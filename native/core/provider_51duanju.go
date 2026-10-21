package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// 51短剧：Nuxt 3 站点，列表页与详情页都把数据塞进 <script id="__NUXT_DATA__">。
// 该脚本是 devalue 序列化后的「扁平数组 + 下标引用」，必须先解引用才能拿到真实对象。
//
// 关键点：
//   - 分集播放地址直接在详情页 payload 的 episodeAll[].video_url 里，不用逐集抓页面；
//   - 地址带 auth_key 查询串，截断成 .m3u8 会被 CDN 判 400，必须原样保留；
//   - m3u8 是 AES-128 加密，密钥地址也带 auth_key，交给现有流代理重写即可。
const (
	sourceDuanju51  = "51duanju"
	duanju51BaseURL = "https://blue.xdrgcewe.cc"
)

// 镜像池与 config/source_mirrors.json 的 51duanju 段保持一致。
// 该站换域名很勤（旧域名会 301 到停靠页），新地址优先改远程配置，
// 这份内置列表只保证拉不到远程配置时仍能出内容。
var duanju51BaseMirrors = []string{
	"https://blue.xdrgcewe.cc",
}

var duanju51Categories = []nativeCategory{
	{ID: "home", Name: "首页推荐"},
	{ID: "rank", Name: "排行榜"},
	{ID: "theater", Name: "剧场"},
	{ID: "new", Name: "最新"},
}

var duanju51NuxtPattern = regexp.MustCompile(`(?is)<script[^>]+id=["']__NUXT_DATA__["'][^>]*>(.*?)</script>`)

func duanju51MirrorHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, origin := range duanju51BaseMirrors {
		if strings.TrimPrefix(strings.ToLower(origin), "https://") == host {
			return true
		}
	}
	return false
}

func validDuanju51Category(category string) bool {
	for _, entry := range duanju51Categories {
		if entry.ID == category {
			return true
		}
	}
	return false
}

// duanju51CatalogPath 返回分类列表页路径。page<=1 时用首页形态。
func duanju51CatalogPath(category string, page int) string {
	if page < 1 {
		page = 1
	}
	switch category {
	case "", "home":
		if page > 1 {
			return ""
		}
		return "/"
	case "rank":
		if page == 1 {
			return "/rank/"
		}
		return fmt.Sprintf("/rank/page/%d/", page)
	case "new":
		if page == 1 {
			return "/new-theater-category/0-0-0-0-0-1/"
		}
		return fmt.Sprintf("/new-theater-category/0-0-0-0-0-1/page/%d/", page)
	default:
		if page == 1 {
			return "/new-theater-category/"
		}
		return fmt.Sprintf("/new-theater-category/page/%d/", page)
	}
}

func duanju51SearchPath(query string, page int) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	if page < 1 {
		page = 1
	}
	escaped := url.PathEscape(query)
	if page == 1 {
		return "/search/" + escaped + "/"
	}
	return fmt.Sprintf("/search/%s/page/%d/", escaped, page)
}

func duanju51DetailPath(sourceID string) string {
	return "/play/" + url.PathEscape(strings.TrimSpace(sourceID)) + "/1/"
}

func (d *Downloader) fetchDuanju51Page(ctx context.Context, path string) (string, error) {
	base := d.providerBaseURL(sourceDuanju51)
	return d.fetchProviderText(ctx, strings.TrimRight(base, "/")+path, base+"/")
}

// fetchDuanju51CatalogPage 抓取列表页并解析卡片。
// query 非空时走站内搜索，忽略 category。
func (d *Downloader) fetchDuanju51CatalogPage(ctx context.Context, page int, category, query string) ([]Drama, bool, error) {
	path := ""
	if strings.TrimSpace(query) != "" {
		path = duanju51SearchPath(query, page)
	} else {
		path = duanju51CatalogPath(category, page)
	}
	if path == "" {
		return nil, false, nil
	}
	body, err := d.fetchDuanju51Page(ctx, path)
	if err != nil {
		return nil, false, err
	}
	items := duanju51CardDramas(duanju51DecodeHTML(body))
	// 首页只有一页；分类页每页 20 条，满页即认为还有下一页。
	more := false
	if strings.TrimSpace(query) != "" || (category != "" && category != "home") {
		more = len(items) >= 20
	}
	return items, more, nil
}

// fetchDuanju51Detail 从详情页 payload 取剧集元信息与全部分集地址。
func (d *Downloader) fetchDuanju51Detail(ctx context.Context, sourceID string) (Drama, []Chapter, error) {
	sourceID = strings.Trim(strings.TrimSpace(sourceID), "/")
	if sourceID == "" {
		return Drama{}, nil, fmt.Errorf("empty 51duanju sourceID")
	}
	base := d.providerBaseURL(sourceDuanju51)
	pagePath := duanju51DetailPath(sourceID)
	body, err := d.fetchDuanju51Page(ctx, pagePath)
	if err != nil {
		return Drama{}, nil, err
	}
	root := duanju51DecodeHTML(body)
	detail := duanju51DetailObject(root, sourceID)
	if detail == nil {
		return Drama{}, nil, fmt.Errorf("51短剧详情解析失败，站点结构可能已变更")
	}

	title := firstNonEmpty(mapString(detail, "drama_name"), mapString(detail, "video_title"), "短剧"+sourceID)
	cover := firstNonEmpty(mapString(detail, "cover_img"), mapString(detail, "cover"))
	if strings.HasPrefix(cover, "//") {
		cover = "https:" + cover
	}
	description := firstNonEmpty(mapString(detail, "description"), mapString(detail, "video_title"))
	remark := firstNonEmpty(mapString(detail, "update_status"), mapString(detail, "serialize_status_text"))

	episodes := duanju51Episodes(detail)
	if len(episodes) == 0 {
		return Drama{}, nil, fmt.Errorf("51短剧该剧暂无可用分集")
	}

	var chapters []Chapter
	freeFound := false
	for position, ep := range episodes {
		index := ep.Index
		if index <= 0 {
			index = position + 1
		}
		chapterTitle := strings.TrimSpace(ep.Title)
		if chapterTitle == "" {
			chapterTitle = fmt.Sprintf("第%d集", index)
		}
		key := ep.Key
		if key == "" {
			key = strconv.Itoa(index)
		}
		chapter := Chapter{
			ID:             providerChapterID(sourceDuanju51, sourceID, key),
			Source:         sourceDuanju51,
			Title:          chapterTitle,
			VideoURL:       ep.HLS,
			PageURL:        strings.TrimRight(base, "/") + duanju51DetailPath(sourceID),
			Referer:        strings.TrimRight(base, "/") + "/",
			CurrentEpisode: rawEpisode(index),
		}
		if ep.Locked {
			chapter.VIP = true
		} else if ep.HLS != "" {
			freeFound = true
		}
		chapters = append(chapters, chapter)
	}
	sortProviderChapters(chapters)
	chapters = uniqueChapters(chapters)

	vip := !freeFound
	drama := Drama{
		VIP:          &vip,
		ID:           providerDramaID(sourceDuanju51, sourceID),
		Source:       sourceDuanju51,
		SourceID:     sourceID,
		Title:        title,
		Name:         title,
		Desc:         description,
		Intro:        description,
		Cover:        cover,
		CoverURL:     cover,
		TotalEpisode: strconv.Itoa(len(chapters)),
		EpisodeCount: strconv.Itoa(len(chapters)),
		ChannelName:  "51短剧",
		CategoryName: "短剧",
		Remark:       remark,
	}
	return drama, chapters, nil
}

// duanju51Episodes 把 episodeAll 转成 providerEpisode。
func duanju51Episodes(detail map[string]any) []providerEpisode {
	raw, ok := detail["episodeAll"].([]any)
	if !ok {
		return nil
	}
	var out []providerEpisode
	for position, entry := range raw {
		ep, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if isAdv, _ := ep["is_adv"].(bool); isAdv {
			continue
		}
		index := 0
		for _, key := range []string{"index", "sort"} {
			if value, err := strconv.Atoi(mapString(ep, key)); err == nil && value > 0 {
				index = value
				break
			}
		}
		media := mapString(ep, "video_url")
		if media == "" {
			media = mapString(ep, "video_url_h265")
		}
		if strings.HasPrefix(media, "//") {
			media = "https:" + media
		}
		key := mapString(ep, "index", "sort", "id", "episode_id")
		if key == "" {
			key = strconv.Itoa(position + 1)
		}
		out = append(out, providerEpisode{
			Key:    key,
			Title:  firstNonEmpty(mapString(ep, "episode_title"), mapString(ep, "title")),
			Index:  index,
			HLS:    media,
			Locked: !duanju51EpisodeFree(ep),
		})
	}
	return out
}

// duanju51EpisodeFree 判断单集是否可直接播放。
// 站点用 unlocked / access_reason / state 三个字段描述权益，任一命中即视为免费。
func duanju51EpisodeFree(ep map[string]any) bool {
	if unlocked, ok := ep["unlocked"].(bool); ok && unlocked {
		return true
	}
	if unlocked, ok := ep["web_unlocked"].(bool); ok && unlocked {
		return true
	}
	if reason := strings.ToLower(mapString(ep, "access_reason")); reason == "free" {
		return true
	}
	if state := strings.ToLower(mapString(ep, "state")); state == "playable" {
		return true
	}
	return false
}

func duanju51DetailObject(root any, sourceID string) map[string]any {
	var found map[string]any
	duanju51WalkObjects(root, func(item map[string]any) {
		if found != nil {
			return
		}
		if mapString(item, "playlet_id") != sourceID {
			return
		}
		if _, ok := item["episodeAll"].([]any); !ok {
			return
		}
		found = item
	})
	return found
}

func duanju51CardDramas(root any) []Drama {
	var out []Drama
	seen := map[string]bool{}
	duanju51WalkObjects(root, func(item map[string]any) {
		videoID := mapString(item, "video_id")
		if videoID == "" || seen[videoID] {
			return
		}
		title := firstNonEmpty(mapString(item, "title"), mapString(item, "video_title"), mapString(item, "drama_name"))
		cover := firstNonEmpty(mapString(item, "cover"), mapString(item, "cover_img"))
		if title == "" || cover == "" {
			return
		}
		if strings.HasPrefix(cover, "//") {
			cover = "https:" + cover
		}
		seen[videoID] = true
		remark := firstNonEmpty(mapString(item, "play_count_text"), mapString(item, "remark_text"), mapString(item, "update_status"))
		out = append(out, Drama{
			ID:           providerDramaID(sourceDuanju51, videoID),
			Source:       sourceDuanju51,
			SourceID:     videoID,
			Title:        title,
			Name:         title,
			Desc:         mapString(item, "description"),
			Intro:        mapString(item, "description"),
			Cover:        cover,
			CoverURL:     cover,
			ChannelName:  "51短剧",
			CategoryName: firstNonEmpty(mapString(item, "type_text"), "短剧"),
			Remark:       remark,
			TotalEpisode: mapString(item, "total_serial"),
			EpisodeCount: mapString(item, "total_serial"),
			Tags:         mapStringSlice(item, "tags"),
			Views:        normalizeViews(mapString(item, "play_count")),
		})
	})
	return out
}

func duanju51WalkObjects(root any, visit func(map[string]any)) {
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			visit(typed)
			for _, item := range typed {
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(root)
}

// duanju51DecodeHTML 从页面里取出 __NUXT_DATA__ 并解引用。
func duanju51DecodeHTML(rawHTML string) any {
	match := duanju51NuxtPattern.FindStringSubmatch(rawHTML)
	if len(match) < 2 {
		return nil
	}
	return duanju51DecodePayload(match[1])
}

var duanju51DevalueSpecial = map[string]bool{
	"ShallowReactive": true, "Reactive": true, "Ref": true, "ShallowRef": true,
	"ToRaw": true, "Readonly": true, "ShallowReadonly": true,
	"Set": true, "Map": true, "Date": true, "RegExp": true,
}

// duanju51DecodePayload 还原 devalue 的扁平数组。
//
// 序列化格式：整个 payload 是一个数组，数组下标即「槽位」。
// 槽位里的整数代表另一个槽位的下标，字符串/布尔/浮点等是字面量。
// 形如 ["Ref", 12] 的二元组表示「取 12 号槽位的值」。
func duanju51DecodePayload(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var slots []any
	if err := json.Unmarshal([]byte(raw), &slots); err != nil {
		return nil
	}
	if len(slots) == 0 {
		return nil
	}

	memo := make(map[int]any, len(slots))
	done := make(map[int]bool, len(slots))
	active := make(map[int]bool, 8)

	var resolve func(int) any
	var read func(any) any

	read = func(value any) any {
		switch typed := value.(type) {
		case []any:
			out := make([]any, len(typed))
			for i, item := range typed {
				out[i] = read(item)
			}
			return out
		case map[string]any:
			out := make(map[string]any, len(typed))
			for key, item := range typed {
				out[key] = read(item)
			}
			return out
		case float64:
			if typed == float64(int64(typed)) {
				return resolve(int(typed))
			}
			return typed
		default:
			return value
		}
	}

	resolve = func(index int) any {
		if index < 0 || index >= len(slots) {
			return index
		}
		if done[index] {
			return memo[index]
		}
		if active[index] {
			// 循环引用：返回 nil，避免无限递归。
			return nil
		}
		active[index] = true
		value := slots[index]
		var result any
		if list, ok := value.([]any); ok {
			switch {
			case len(list) == 2 && duanju51DevalueMarker(list[0]) != "" && duanju51DevalueSpecial[duanju51DevalueMarker(list[0])]:
				if target, ok := list[1].(float64); ok {
					result = resolve(int(target))
				} else {
					result = read(list[1])
				}
			case len(list) >= 3 && duanju51DevalueMarker(list[0]) == "Set":
				items := make([]any, 0, len(list)-1)
				for _, item := range list[1:] {
					items = append(items, read(item))
				}
				result = items
			case len(list) >= 3 && duanju51DevalueMarker(list[0]) == "Map":
				pairs := make(map[string]any, (len(list)-1)/2)
				for i := 1; i+1 < len(list); i += 2 {
					pairs[fmt.Sprint(read(list[i]))] = read(list[i+1])
				}
				result = pairs
			default:
				result = read(list)
			}
		} else if object, ok := value.(map[string]any); ok {
			converted := make(map[string]any, len(object))
			for key, item := range object {
				converted[key] = read(item)
			}
			result = converted
		} else {
			result = value
		}
		delete(active, index)
		memo[index] = result
		done[index] = true
		return result
	}

	return resolve(0)
}

func duanju51DevalueMarker(value any) string {
	text, _ := value.(string)
	return text
}
