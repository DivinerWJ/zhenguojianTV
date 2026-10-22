package core

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const ezdmwBaseURL = "https://www.ezdmw.org"
const ezdmwPageSize = 20

// ezdmwSearchPageSize 是 /Index/search_some.html 每页写死的条数，改不了
// （`num`/`limit`/`pagesize`/`size`/`count` 全部无效，实测一律 10 条）。
const ezdmwSearchPageSize = 10

// ezdmwBrowseKeyword 用来把「全站」当一次搜索：站点搜索除了标题也匹配年份字段
// （`2019年1月【连载中】`），而每一部都有年份，所以搜「年」等价于全部番剧。
// 实测结果集约 2080 部，标题里不含「年」的也在内。
const ezdmwBrowseKeyword = "年"

var ezdmwBaseMirrors = []string{
	"https://www.ezdmw.org",
	"https://m.ezdmw.org",
	"https://ezdmw.org",
}

var (
	ezdmwDetailPathPattern  = regexp.MustCompile("(?i)^/(?:show|Index/show|Index/detail|Index/bangumi|Index/video|v)/([0-9]+)\\.html$")
	ezdmwEpisodePathPattern = regexp.MustCompile("(?i)^/(?:Index/bangumi|Index/video|v)/([0-9]+)\\.html$")
	ezdmwAnchorPattern      = regexp.MustCompile("(?is)<a\\b([^>]*)>(.*?)</a>")
	ezdmwParagraphPattern   = regexp.MustCompile("(?is)<p\\b([^>]*)>(.*?)</p>")
	// 集数：`更新到：&nbsp;24` 经 cleanText 变成 `更新到： 24`（全角冒号 + NBSP），
	// 所以数字前要允许 空白 / NBSP(\x{00a0}) / 半角与全角冒号（0.8.9）。
	// search_some 的结果卡片用的是 `更至12话`，简写也要收（0.8.9）。
	ezdmwEpisodeCountPattern = regexp.MustCompile("(?i)(?:更新到|更新至|更至)[\\s\\x{00a0}：:]*0*([0-9]+)|(?:共|全)\\s*0*([0-9]+)\\s*(?:集|话|期)|([0-9]+)\\s*集\\s*完结")
	// 卡片标题后面的元信息尾巴：`标题&nbsp;&nbsp;更至12话`。
	// `共/全 + 数字 + 集/话/期` 要求带数字，否则「全金属狂潮」这类标题会被砍掉。
	ezdmwCardMetaPattern = regexp.MustCompile("(?i)[\\s\\x{00a0}]*(?:更至|更新至|更新到|共\\s*0*[0-9]+\\s*[集话期]|全\\s*0*[0-9]+\\s*[集话期]).*$")
	// 结果卡片右上角的年份/状态，`<span style="float:right;">2019年1月【连载中】`。
	ezdmwFloatRightSpanPattern = regexp.MustCompile("(?is)<span\\b[^>]*float\\s*:\\s*right[^>]*>.*?</span>")
	ezdmwMediaURLPattern       = regexp.MustCompile("(?i)(?:https?:)?//[^\\\"'<>\\s]+?\\.(?:m3u8|mp4)(?:\\?[^\\\"'<>\\s]*)?")
	ezdmwVideoTagPattern       = regexp.MustCompile("(?is)<(?:video|source)\\b[^>]*>")
	ezdmwIframePattern         = regexp.MustCompile("(?is)<iframe\\b[^>]*>")
	ezdmwPlayerDataPattern     = regexp.MustCompile("(?is)(?:player_data|playerData|video_data|videoData)\\s*[:=]\\s*[\\\"']([^\\\"']{16,})[\\\"']")
	ezdmwImageTagPattern       = regexp.MustCompile("(?is)<img\\b[^>]*>")
	ezdmwTitleEpisodeSuffix    = regexp.MustCompile("(?i)\\s*第\\s*0*[0-9]+\\s*集.*$")
	ezdmwTitleSiteSuffix       = regexp.MustCompile("(?i)\\s*[-|_—·]*\\s*(?:E站弹幕网|E站动漫|E站)\\s*$")
	ezdmwHTMLCommentPattern    = regexp.MustCompile("(?s)<!--.*?-->")
	ezdmwNumericIDPattern      = regexp.MustCompile("^[0-9]{1,12}$")
)

// ezdmwRequestURL 把 E站页面里原样出现的地址整理成可以直接发请求的形式（0.8.9）。
//
// E站的播放器 iframe 和 listres 媒体地址都是把值**原样**写进 HTML 属性：
//
//	<iframe src="https://player.ezdmw.com/danmuku/?nk=…&title=关于我转生… 第四季&sign=…">
//	<video  src="//player.ezdmw.com/index/listres/…webp?user=游客&quarterly=2026年4月【全集完】">
//
// 中文的 UTF-8 字节进请求行没问题，但**空格不行**：`url.Parse` 会把 `title=`
// 之后的内容整段塞进 `RawQuery`，Go 的 http 客户端再原样写进请求行，
// 空格把请求行截断 → 服务端直接 400。实测标题里带空格的番剧（"…第四季"、
// "幼女战记 第二季"、"碧蓝之海 第三季"…）全部无法播放，首页 40 部里约一半。
//
// 这里只转义会破坏请求行的字符（空格、控制字符），**已有的 %XX 一律原样保留**：
// 地址里带 `sign=` 签名，整体重新编码会把 `%` 二次编码成 `%25`，签名直接失效。
func ezdmwRequestURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.ContainsAny(raw, " \t\r\n") {
		return raw
	}
	var builder strings.Builder
	builder.Grow(len(raw) + 8)
	for index := 0; index < len(raw); index++ {
		ch := raw[index]
		switch {
		case ch == '%' && index+2 < len(raw) && ezdmwHexDigit(raw[index+1]) && ezdmwHexDigit(raw[index+2]):
			// 已是 %XX，原样保留（签名里的转义不能再编一次）
			builder.WriteString(raw[index : index+3])
			index += 2
		case ch == ' ':
			builder.WriteString("%20")
		case ch == '\t':
			builder.WriteString("%09")
		case ch < 0x20 || ch == 0x7f:
			// 换行之类的控制字符会直接截断请求行，丢掉
		default:
			builder.WriteByte(ch)
		}
	}
	return builder.String()
}

func ezdmwHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

// ezdmwCardTitle 从卡片片段里取干净的番剧名（0.8.9）。
//
// E站首页卡片的 `<a>` 里除了封面还套着整块 `.info-card`：
//
//	<a href="/Index/bangumi/98512.html">
//	  <img src="…/some_img/2500.jpg" title="关于我转生变成史莱姆这档事 第四季" />
//	  <div class="info-card">
//	    <p>关于我转生变成史莱姆这档事 第四季</p>
//	    <p class="info"><span>2星期前</span><span>更新到：&nbsp;24</span>
//	      <!--<span>总追番数：118</span>--></p>
//	  </div>
//	</a>
//
// 直接 `cleanText(整个 <a>)` 会把"更新时间 / 集数 / 总追番数"一起吃进标题，
// 而且 HTML 注释被 `<[^>]+>` 剥掉标签后还会残留一个 `-->`，实际得到
// `"…第四季 2星期前 更新到： 24 总追番数：118 -->"`。
//
// 取法按可靠性排序：封面图的 title/alt → 不带 info class 的 `<p>`（卡片标题行）→ 整块文本兜底。
//
// 首页三种卡片都要覆盖：
//   - 最近更新 `.rec-item`：`<img title>` 或 `<p>`（无 class）+ `<p class="info">`；
//   - 新番 `.anime-ct .item`：`<div class="mask">` 里是类型标签，标题是 `<p class="a-hover">`；
//   - 排行榜 `.a-item`：标题是 `<p class="text-top a-hover">`，没有封面。
func ezdmwCardTitle(inner string) string {
	for _, tag := range ezdmwImageTagPattern.FindAllString(inner, -1) {
		if title := ezdmwCleanTitle(ezdmwAttr(tag, "title", "alt")); title != "" {
			return title
		}
	}
	for _, match := range ezdmwParagraphPattern.FindAllStringSubmatch(inner, -1) {
		// class 含 info 的 <p> 是元信息（更新时间/集数/热度），不是标题。
		// 注意不能"凡带 class 就跳过"：新番区块的标题行是 `<p class="a-hover">`，
		// 排行榜是 `<p class="text-top a-hover">`，跳过了就会退化成整块文本，
		// 把类型标签（"剧场、恋爱、校园…"）和时间一起带进标题。
		if strings.Contains(strings.ToLower(match[1]), "info") {
			continue
		}
		if title := ezdmwCardText(match[2]); title != "" {
			return title
		}
	}
	return ezdmwCardText(inner)
}

// ezdmwCardText 把卡片里的一段 HTML 收成干净的标题。
//
// search_some 的结果卡片把标题和元信息塞在同一个 <p> 里（0.8.9）：
//
//	<p>来自深渊剧场版&nbsp;&nbsp;更至PV话<span style="float:right;">2019年1月【连载中】<span></span></p>
//
// 直接 cleanText 会得到「来自深渊剧场版 更至PV话 2019年1月【连载中】」。
// 所以先删注释、删掉右浮动的年份/状态 span，再砍掉 `更至12话` 这类尾巴。
func ezdmwCardText(raw string) string {
	// 注释要**先**整段删掉：卡片里有一段被注释掉的
	// `<!--<span class="play_some"><img/> 1970-01-01</span>-->`，
	// 靠 cleanText 去标签只会把 `<!--` 和 `-->` 留下，标题就变成
	// 「1970-01-01 -->来自深渊剧场版」。
	raw = ezdmwHTMLCommentPattern.ReplaceAllString(raw, " ")
	raw = ezdmwFloatRightSpanPattern.ReplaceAllString(raw, " ")
	title := ezdmwCleanTitle(raw)
	if loc := ezdmwCardMetaPattern.FindStringIndex(title); loc != nil && loc[0] > 0 {
		title = strings.TrimSpace(title[:loc[0]])
	}
	return title
}

func ezdmwMirrorHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "m.ezdmw.org" || host == "www.ezdmw.org" || host == "ezdmw.org"
}

func ezdmwAttr(raw string, names ...string) string {
	for _, name := range names {
		pattern := regexp.MustCompile("(?is)(?:^|\\s)" + regexp.QuoteMeta(name) + "\\s*=\\s*[\\\"']([^\\\"']*)[\\\"']")
		if match := pattern.FindStringSubmatch(raw); len(match) > 1 {
			return strings.TrimSpace(html.UnescapeString(match[1]))
		}
	}
	return ""
}

func ezdmwElementBlock(raw, tag string, matchOpening func(string) bool) string {
	startPattern := regexp.MustCompile("(?is)<" + regexp.QuoteMeta(tag) + "\\b[^>]*>")
	tokenPattern := regexp.MustCompile("(?is)</?" + regexp.QuoteMeta(tag) + "\\b[^>]*>")
	for _, start := range startPattern.FindAllStringIndex(raw, -1) {
		opening := raw[start[0]:start[1]]
		if matchOpening != nil && !matchOpening(opening) {
			continue
		}
		depth := 1
		cursor := start[1]
		for _, token := range tokenPattern.FindAllStringIndex(raw[cursor:], -1) {
			tokenText := raw[cursor+token[0] : cursor+token[1]]
			if strings.HasPrefix(strings.TrimSpace(tokenText), "</") {
				depth--
			} else if !strings.HasSuffix(strings.TrimSpace(tokenText), "/>") {
				depth++
			}
			if depth == 0 {
				return raw[start[0] : cursor+token[1]]
			}
		}
	}
	return ""
}

func ezdmwDramaIDFromURL(raw string) string {
	parsed, err := url.Parse(html.UnescapeString(strings.TrimSpace(raw)))
	if err != nil {
		return ""
	}
	match := ezdmwDetailPathPattern.FindStringSubmatch(parsed.Path)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func ezdmwEpisodeIDFromURL(raw string) string {
	parsed, err := url.Parse(html.UnescapeString(strings.TrimSpace(raw)))
	if err != nil {
		return ""
	}
	match := ezdmwEpisodePathPattern.FindStringSubmatch(parsed.Path)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func ezdmwCleanTitle(raw string) string {
	title := cleanText(raw)
	title = ezdmwTitleEpisodeSuffix.ReplaceAllString(title, "")
	title = ezdmwTitleSiteSuffix.ReplaceAllString(title, "")
	return strings.Trim(title, " \t\r\n-|_—·")
}

func parseEZDMWEpisodeCount(raw string) int {
	match := ezdmwEpisodeCountPattern.FindStringSubmatch(raw)
	if len(match) < 2 {
		return 0
	}
	for _, group := range match[1:] {
		if group == "" {
			continue
		}
		if count, err := strconv.Atoi(group); err == nil && count > 0 {
			return count
		}
	}
	return 0
}

func ezdmwNearbyTitle(raw string, start, end int) string {
	pPattern := regexp.MustCompile("(?is)<p\\b[^>]*>(.*?)</p>")
	bestDistance := int(^uint(0) >> 1)
	bestTitle := ""
	for _, match := range pPattern.FindAllStringSubmatchIndex(raw, -1) {
		if len(match) < 4 {
			continue
		}
		distance := 0
		if match[1] < start {
			distance = start - match[1]
		} else if match[0] > end {
			distance = match[0] - end
		}
		if distance > 600 || distance >= bestDistance {
			continue
		}
		paragraph := ezdmwHTMLCommentPattern.ReplaceAllString(raw[match[2]:match[3]], " ")
		title := ezdmwCleanTitle(cleanText(paragraph))
		if title != "" {
			bestDistance = distance
			bestTitle = title
		}
	}
	return bestTitle
}

func parseEZDMWCatalogItems(raw, pageURL string) []Drama {
	scope := ezdmwElementBlock(raw, "section", func(opening string) bool {
		return strings.EqualFold(extractAttr(opening, "id"), "some_drama")
	})
	if scope == "" {
		scope = raw
	}
	positions := map[string]int{}
	items := make([]Drama, 0, ezdmwPageSize)
	for _, match := range ezdmwAnchorPattern.FindAllStringSubmatchIndex(scope, -1) {
		attributes := scope[match[2]:match[3]]
		inner := scope[match[4]:match[5]]
		href := extractAttr(attributes, "href")
		sourceID := ezdmwDramaIDFromURL(href)
		if sourceID == "" {
			continue
		}
		// 0.8.9：标题必须走 ezdmwCardTitle。首页卡片的 <a> 里套着 .info-card，
		// 直接 cleanText(inner) 会把"2星期前 更新到： 24 总追番数：118 -->"当标题。
		title := ezdmwCardTitle(inner)
		if title == "" {
			title = ezdmwCleanTitle(extractAttr(attributes, "title", "aria-label"))
		}
		if title == "" {
			title = ezdmwNearbyTitle(scope, match[0], match[1])
		}
		cover := ""
		for _, tag := range ezdmwImageTagPattern.FindAllString(inner, -1) {
			cover = firstNonEmpty(ezdmwAttr(tag, "data-src", "data-original", "src"))
			if cover != "" {
				break
			}
		}
		coverURL := ""
		if cover != "" {
			resolved := resolveProviderURL(pageURL, cover)
			if isProviderHTTPMediaURL(resolved) {
				coverURL = resolved
			}
		}
		// search_some 的结果卡片把 `更至12话`、`【连载中】` 放在 <a> 内部，
		// 首页卡片则在 .info-card 里；两处都要看，所以拼起来再解析（0.8.9）。
		contextText := cleanText(contextBlock(scope, match[0], match[1], 180))
		metaText := cleanText(inner) + " " + contextText
		count := parseEZDMWEpisodeCount(metaText)
		releaseStatus := ""
		// 站点的完结标记是「【全集完】」，不是「完结」——只查「完结」会漏掉，
		// 于是所有完结番都被当成未知状态（0.8.9）。
		if strings.Contains(metaText, "全集完") || strings.Contains(metaText, "完结") {
			releaseStatus = "finished"
		} else if strings.Contains(metaText, "连载") || strings.Contains(metaText, "更新") {
			releaseStatus = "ongoing"
		}
		drama := Drama{
			ID: providerDramaID(sourceEZDMW, sourceID), Source: sourceEZDMW, SourceID: sourceID,
			Title: title, Name: title, Cover: coverURL, CoverURL: coverURL,
			CategoryName: "动漫", Episodes: count, TotalEpisode: count, EpisodeCount: count,
			ReleaseStatus: releaseStatus,
		}
		if index, exists := positions[sourceID]; exists {
			existing := items[index]
			if existing.Title == "" && title != "" {
				existing.Title, existing.Name = title, title
			}
			if existing.Cover == "" && coverURL != "" {
				existing.Cover, existing.CoverURL = coverURL, coverURL
			}
			if (existing.Episodes == nil || nativeText(existing.Episodes) == "0") && count > 0 {
				existing.Episodes, existing.TotalEpisode, existing.EpisodeCount = count, count, count
			}
			if existing.ReleaseStatus == "" && releaseStatus != "" {
				existing.ReleaseStatus = releaseStatus
			}
			items[index] = existing
			continue
		}
		positions[sourceID] = len(items)
		items = append(items, drama)
	}
	out := make([]Drama, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Title) != "" {
			out = append(out, item)
		}
	}
	return out
}

// ezdmwSearchSomeURL 构造 /Index/search_some.html 的列表地址。
//
// 这个入口的 `page` 参数是**条目偏移量**，不是页码：站点直接把 page 当 offset 用，
// 每页固定 10 条。实测 page=1 与 page=2 的结果重叠 9 条、与 page=11 零重叠，
// 步长正好 1 条，所以取第 n 页必须传 (n-1)*10。
//
// 注意站点的 /Index/search.html 是另一个控制器：结果固定截断在 5 条，而且完全不认
// page —— 0.8.9 之前用的就是它，所以搜索永远只有 5 条、也没有"加载更多"。
func ezdmwSearchSomeURL(base, keyword string, offset int) string {
	if offset < 0 {
		offset = 0
	}
	values := url.Values{}
	values.Set("searchText", keyword)
	values.Set("page", strconv.Itoa(offset))
	return base + "/Index/search_some.html?" + values.Encode()
}

// fetchEZDMWListPage 取一页 search_some 结果。
//
// 空页不算错误：它表示已经翻到底，返回 hasMore=false 让客户端收手。报错反而更糟——
// catalog_browser.dart 里 `hasMore = result.hasMore || result.warning.isNotEmpty`，
// 一旦带 warning，"加载更多"会变成无限重试。
func (d *Downloader) fetchEZDMWListPage(ctx context.Context, address string) ([]Drama, bool, error) {
	base := strings.TrimRight(d.providerBaseURL(sourceEZDMW), "/")
	body, err := d.fetchProviderText(ctx, address, base+"/")
	if err != nil {
		return nil, false, err
	}
	items := parseEZDMWCatalogItems(body, address)
	return items, len(items) >= ezdmwSearchPageSize, nil
}

func (d *Downloader) fetchEZDMWCatalogPage(ctx context.Context, page int, category, query string) ([]Drama, bool, error) {
	if strings.TrimSpace(category) != "" {
		return nil, false, errors.New("E站不支持该分类")
	}
	if page < 1 {
		page = 1
	}
	base := strings.TrimRight(d.providerBaseURL(sourceEZDMW), "/")
	query = strings.TrimSpace(query)

	// 搜索：走 search_some 的偏移分页，能把整个结果集翻完。
	// 站点对空结果会输出「搜索结果为空」，此时返回空页而不是报错，
	// 否则"搜不到"会被渲染成"站点结构变了"。
	if query != "" {
		return d.fetchEZDMWListPage(ctx, ezdmwSearchSomeURL(base, query, (page-1)*ezdmwSearchPageSize))
	}

	// 目录首页仍然用站点编排好的首页：最近更新 20 + 新番 9 + 排行榜 11，一次 40 部。
	// 站点对 `/` 和 `/Index/anime.html` 的 `page` 参数一律忽略，没有真正的翻页接口，
	// 所以"加载更多"只能靠 search_some 的偏移继续往下走（0.8.9）。
	if page == 1 {
		homeURL := base + "/"
		body, err := d.fetchProviderText(ctx, homeURL, homeURL)
		if err != nil {
			return nil, false, err
		}
		items := parseEZDMWCatalogItems(body, homeURL)
		if len(items) == 0 {
			return nil, false, errors.New("E站目录未解析到番剧，站点页面结构可能已变化")
		}
		return items, true, nil
	}

	// 后续页从偏移 0 开始，而不是从首页条数（40）接着走：首页是按"最近更新"编排的，
	// 和 search_some 默认的 ID 倒序只重叠 11/40，跳过前 40 条会漏掉约 29 部。
	// 重叠部分由 catalog_browser 按 item.id 建 Map 合并时自然去重。
	offset := (page - 2) * ezdmwSearchPageSize
	return d.fetchEZDMWListPage(ctx, ezdmwSearchSomeURL(base, ezdmwBrowseKeyword, offset))
}

func ezdmwEpisodeTitle(raw string, index int) string {
	title := ezdmwCleanTitle(raw)
	if title == "" || regexp.MustCompile("^[0-9]+$").MatchString(title) {
		return fmt.Sprintf("第%d集", index)
	}
	if episodeIndex(title, 0) > 0 {
		return title
	}
	return fmt.Sprintf("第%d集", index)
}

func parseEZDMWEpisodes(raw, detailURL, sourceID string) []Chapter {
	scope := ezdmwElementBlock(raw, "section", func(opening string) bool {
		for _, class := range strings.Fields(extractAttr(opening, "class")) {
			if class == "anthology" {
				return true
			}
		}
		return false
	})
	if scope == "" {
		scope = raw
	}
	seenIDs := map[string]bool{}
	seenIndexes := map[int]bool{}
	chapters := make([]Chapter, 0, 24)
	for _, match := range ezdmwAnchorPattern.FindAllStringSubmatchIndex(scope, -1) {
		attributes := scope[match[2]:match[3]]
		inner := scope[match[4]:match[5]]
		href := extractAttr(attributes, "href")
		pageURL := resolveProviderURL(detailURL, href)
		episodeID := ezdmwEpisodeIDFromURL(pageURL)
		if episodeID == "" || seenIDs[episodeID] || !isProviderHTTPMediaURL(pageURL) {
			continue
		}
		// E站的 /Index/bangumi/ 页面是番剧/分集信息页，真正的播放线路
		// 使用 /Index/video/{id}.html；旧 /v/ 链接也统一映射到该播放路由。
		if parsedPage, parseErr := url.Parse(pageURL); parseErr == nil {
			parsedPage.Path = "/Index/video/" + episodeID + ".html"
			pageURL = parsedPage.String()
		}
		title := cleanText(inner)
		if title == "" {
			title = extractAttr(attributes, "title", "aria-label")
		}
		index := episodeIndex(title, 0)
		if index <= 0 {
			index = episodeIndex(href, 0)
		}
		if index <= 0 {
			index = len(chapters) + 1
		}
		if seenIndexes[index] {
			continue
		}
		title = ezdmwEpisodeTitle(title, index)
		seenIDs[episodeID], seenIndexes[index] = true, true
		chapters = append(chapters, Chapter{
			ID:     providerChapterID(sourceEZDMW, sourceID, episodeID),
			Source: sourceEZDMW, Title: title,
			VideoURL: "ezdmw-play://" + episodeID, PageURL: pageURL,
			Referer: detailURL, CurrentEpisode: rawEpisode(index),
		})
	}
	sortProviderChapters(chapters)
	return uniqueChapters(chapters)
}

func ezdmwCoverURL(raw, pageURL string) string {
	metaPattern := regexp.MustCompile("(?is)<meta\\b[^>]*(?:property|name)=[\\\"'](?:og:image|twitter:image)[\\\"'][^>]*>")
	for _, tag := range metaPattern.FindAllString(raw, -1) {
		if address := ezdmwAttr(tag, "content"); address != "" {
			resolved := resolveProviderURL(pageURL, address)
			if isProviderHTTPMediaURL(resolved) {
				return resolved
			}
		}
	}
	// 0.8.9：E站详情页没有 og:image，且页面头部第一个 <img> 是站点 logo
	// （/Public/images/icon.png），原来直接取"第一个 img"会把 logo 当海报。
	// 真实海报在 Public/atlas_oss/Public/images/some_img/<id>.jpg，优先取它；
	// 注意海报路径本身也含 `/Public/images/`，所以必须先判 some_img 再排除 UI 图标。
	var fallback string
	for _, tag := range ezdmwImageTagPattern.FindAllString(raw, -1) {
		address := ezdmwAttr(tag, "data-src", "data-original", "src")
		if address == "" {
			continue
		}
		resolved := resolveProviderURL(pageURL, address)
		if !isProviderHTTPMediaURL(resolved) {
			continue
		}
		lower := strings.ToLower(resolved)
		if strings.Contains(lower, "some_img") || strings.Contains(lower, "atlas_oss") {
			return resolved
		}
		// /Public/images/、/src/images/ 下都是 UI 图标（logo、播放键、收藏…）
		if strings.Contains(lower, "/public/images/") || strings.Contains(lower, "/src/images/") || strings.Contains(lower, "logo") {
			continue
		}
		if fallback == "" {
			fallback = resolved
		}
	}
	return fallback
}

func (d *Downloader) fetchEZDMWDetail(ctx context.Context, sourceID string) (Drama, []Chapter, error) {
	sourceID = strings.TrimSpace(sourceID)
	if !ezdmwNumericIDPattern.MatchString(sourceID) {
		return Drama{}, nil, errors.New("E站番剧 ID 无效")
	}
	base := strings.TrimRight(d.providerBaseURL(sourceEZDMW), "/")
	// E站的列表入口在不同页面/版本中可能链接到番剧页或某一集的
	// /Index/bangumi/、/v/ 页面；这些页面都可能包含完整选集列表。
	paths := []string{
		"/Index/bangumi/" + sourceID + ".html",
		"/show/" + sourceID + ".html",
		"/Index/show/" + sourceID + ".html",
		"/Index/detail/" + sourceID + ".html",
		"/v/" + sourceID + ".html",
	}
	var body, detailURL string
	var chapters []Chapter
	var lastErr error
	for _, path := range paths {
		candidateURL := base + path
		candidateBody, err := d.fetchProviderText(ctx, candidateURL, base+"/")
		if err != nil {
			lastErr = err
			continue
		}
		candidateChapters := parseEZDMWEpisodes(candidateBody, candidateURL, sourceID)
		if len(candidateChapters) == 0 {
			lastErr = errors.New("页面未解析到分集")
			continue
		}
		body, detailURL, chapters = candidateBody, candidateURL, candidateChapters
		break
	}
	if len(chapters) == 0 {
		if lastErr != nil {
			return Drama{}, nil, fmt.Errorf("E站未找到包含选集的详情页：%w", lastErr)
		}
		return Drama{}, nil, errors.New("E站未找到包含选集的详情页")
	}
	title := ezdmwCleanTitle(extractPageTitle(body))
	if title == "" {
		title = "番剧 " + sourceID
	}
	count := len(chapters)
	releaseStatus := "ongoing"
	if strings.Contains(body, "完结") {
		releaseStatus = "finished"
	}
	cover := ezdmwCoverURL(body, detailURL)
	description := extractDescription(body)
	drama := Drama{
		ID: providerDramaID(sourceEZDMW, sourceID), Source: sourceEZDMW, SourceID: sourceID,
		Title: title, Name: title, Desc: description, Intro: description,
		Cover: cover, CoverURL: cover, CategoryName: "动漫",
		Episodes: count, TotalEpisode: count, EpisodeCount: count, ReleaseStatus: releaseStatus,
	}
	return drama, chapters, nil
}

func (d *Downloader) fetchEZDMWChapters(ctx context.Context, sourceID string) (string, []Chapter, error) {
	drama, chapters, err := d.fetchEZDMWDetail(ctx, sourceID)
	if err != nil {
		return "", nil, err
	}
	return drama.DisplayTitle(), chapters, nil
}

func ezdmwUnescapePlayerText(raw string) string {
	return strings.NewReplacer(
		"\\/", "/",
		"\\u0026", "&",
		"\\u002f", "/",
		"\\u003a", ":",
		"\\x26", "&",
		"&amp;", "&",
	).Replace(html.UnescapeString(raw))
}

func ezdmwPlaylistURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && strings.Contains(strings.ToLower(parsed.Path), "/index/listres/")
}

func ezdmwDirectMedia(rawHTML, pageURL string) string {
	rawHTML = ezdmwUnescapePlayerText(rawHTML)
	for _, tag := range ezdmwVideoTagPattern.FindAllString(rawHTML, -1) {
		if address := ezdmwAttr(tag, "src", "data-src", "data-url"); address != "" {
			// E站播放器的 listres 接口使用 .webp 后缀，但页面脚本明确交给 Hls.js 加载。
			if strings.Contains(strings.ToLower(address), "/index/listres/") || strings.Contains(strings.ToLower(address), "/listres/") {
				if media := resolveProviderURL(pageURL, address); isProviderHTTPMediaURL(media) {
					// 0.8.9：查询串里带 `user=游客&quarterly=…`，可能含空格，
					// 必须转义后再交给代理，否则请求行非法（详见 ezdmwRequestURL）。
					return ezdmwRequestURL(media)
				}
			}
			if media := normalizeProviderMediaURL(pageURL, address); media != "" && isProviderHTTPMediaURL(media) {
				return ezdmwRequestURL(media)
			}
		}
	}
	if media := parseDataHLS(rawHTML, pageURL); media != "" && isProviderHTTPMediaURL(media) {
		return ezdmwRequestURL(media)
	}
	if media := parseAIVideoURL(rawHTML, pageURL); media != "" && isProviderHTTPMediaURL(media) {
		return ezdmwRequestURL(media)
	}
	for _, address := range ezdmwMediaURLPattern.FindAllString(rawHTML, -1) {
		address = strings.TrimRight(address, ".,);]")
		if strings.HasPrefix(address, "//") {
			address = "https:" + address
		}
		if media := normalizeProviderMediaURL(pageURL, address); media != "" && isProviderHTTPMediaURL(media) {
			return ezdmwRequestURL(media)
		}
	}
	return ""
}

func ezdmwMediaFromPlayerFrameURL(frameURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(frameURL))
	if err != nil {
		return ""
	}
	for _, key := range []string{"src", "url", "file", "video", "media", "play"} {
		candidate := strings.TrimSpace(parsed.Query().Get(key))
		for depth := 0; depth < 4 && candidate != ""; depth++ {
			if media := ezdmwDirectMedia(candidate, frameURL); media != "" {
				return media
			}
			var decoded []byte
			var decodeErr error
			decoded, decodeErr = base64.RawURLEncoding.DecodeString(candidate)
			if decodeErr != nil {
				decoded, decodeErr = base64.URLEncoding.DecodeString(candidate)
			}
			if decodeErr != nil {
				decoded, decodeErr = base64.RawStdEncoding.DecodeString(candidate)
			}
			if decodeErr != nil {
				decoded, decodeErr = base64.StdEncoding.DecodeString(candidate)
			}
			if decodeErr == nil {
				candidate = strings.TrimSpace(string(decoded))
				continue
			}
			unescaped, unescapeErr := url.QueryUnescape(candidate)
			if unescapeErr == nil && unescaped != candidate {
				candidate = strings.TrimSpace(unescaped)
				continue
			}
			break
		}
	}
	return ""
}

func parseEZDMWPlayerMedia(rawHTML, pageURL string) string {
	if media := ezdmwDirectMedia(rawHTML, pageURL); media != "" {
		return media
	}
	for _, match := range ezdmwPlayerDataPattern.FindAllStringSubmatch(rawHTML, -1) {
		candidate := strings.TrimSpace(html.UnescapeString(match[1]))
		for depth := 0; depth < 3; depth++ {
			if media := ezdmwDirectMedia(candidate, pageURL); media != "" {
				return media
			}
			decoded, err := base64.StdEncoding.DecodeString(candidate)
			if err != nil {
				decoded, err = base64.RawStdEncoding.DecodeString(candidate)
			}
			if err != nil {
				break
			}
			candidate = string(decoded)
		}
		if decoded, err := url.QueryUnescape(candidate); err == nil {
			if media := ezdmwDirectMedia(decoded, pageURL); media != "" {
				return media
			}
		}
	}
	return ""
}

func ezdmwSafePageURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !isProviderHTTPMediaURL(raw) {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "0.0.0.0" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
		return false
	}
	return true
}

func parseEZDMWPlayerFrame(rawHTML, pageURL string) string {
	var fallback string
	for _, tag := range ezdmwIframePattern.FindAllString(rawHTML, -1) {
		address := ezdmwAttr(tag, "src", "data-src")
		if address == "" {
			continue
		}
		resolved := resolveProviderURL(pageURL, address)
		if !ezdmwSafePageURL(resolved) {
			continue
		}
		// 0.8.9：iframe 的 src 属性里 `title=` 带中文和空格（"… 第四季"），
		// 不转义会让请求行非法、服务端回 400——标题带空格的番剧全都播不了。
		resolved = ezdmwRequestURL(resolved)
		if fallback == "" {
			fallback = resolved
		}
		lower := strings.ToLower(address)
		if strings.Contains(lower, "player") || strings.Contains(lower, "play") || strings.Contains(lower, "embed") {
			return resolved
		}
	}
	return fallback
}

func (d *Downloader) resolveEZDMWPlayback(ctx context.Context, task Task) (providerMedia, error) {
	source, _, valid := splitProviderDramaID(task.DramaID)
	if !valid || source != sourceEZDMW {
		return providerMedia{}, errors.New("E站番剧信息无效，请刷新详情")
	}
	episodeID := strings.TrimPrefix(strings.TrimSpace(task.Chapter.VideoURL), "ezdmw-play://")
	base := strings.TrimRight(d.providerBaseURL(sourceEZDMW), "/")
	pageURL := strings.TrimSpace(task.Chapter.PageURL)
	if pageURL == "" {
		if !ezdmwNumericIDPattern.MatchString(episodeID) {
			return providerMedia{}, errors.New("E站分集播放标识无效，请刷新详情")
		}
		pageURL = base + "/Index/bangumi/" + episodeID + ".html"
	}
	pageURL = resolveProviderURL(base+"/", pageURL)
	parsed, parseErr := url.Parse(pageURL)
	if parseErr != nil || !ezdmwMirrorHost(parsed.Hostname()) || !ezdmwSafePageURL(pageURL) {
		return providerMedia{}, errors.New("E站分集页面地址无效")
	}
	referer := base + "/"
	visited := map[string]bool{}
	for depth := 0; depth < 3; depth++ {
		if visited[pageURL] {
			return providerMedia{}, errors.New("E站播放线路出现循环跳转")
		}
		visited[pageURL] = true
		body, err := d.fetchProviderText(ctx, pageURL, referer)
		if err != nil {
			return providerMedia{}, fmt.Errorf("获取E站播放页面失败：%w", err)
		}
		if media := parseEZDMWPlayerMedia(body, pageURL); media != "" {
			return providerMedia{URL: media, Referer: pageURL}, nil
		}
		next := parseEZDMWPlayerFrame(body, pageURL)
		if next == "" || visited[next] {
			break
		}
		if media := ezdmwMediaFromPlayerFrameURL(next); media != "" {
			return providerMedia{URL: media, Referer: pageURL}, nil
		}
		referer, pageURL = pageURL, next
	}
	return providerMedia{}, errors.New("E站未解析到可播放的媒体地址，可能线路失效或需要浏览器验证")
}
