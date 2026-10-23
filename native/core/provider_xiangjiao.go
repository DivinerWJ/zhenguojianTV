package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 香蕉短剧是明文 REST 站点。播放正片必须先创建 guest session，
// 再调用 /api/playback/sessions。
//
// 0.8.10 起：正片会话对游客返回 403 `purchase_required` 时，会再试一次
// `/api/trial/sessions`（站点对游客开放的试看）。拿到试看 manifest 就播
// 试看片段，拿不到才把该集作为受限内容交给 UI —— 客户端没有合法路径
// 拿全集，不做付费绕过。
const (
	sourceXiangjiao     = "xiangjiao"
	xiangjiaoBaseURL    = "https://xiangjiaoai.ai"
	xiangjiaoSessionTTL = 40 * time.Minute
)

var xiangjiaoBaseMirrors = []string{
	"https://xiangjiaoai.ai",
}

type xiangjiaoHTTPError struct {
	Status  int
	Code    string
	Message string
}

func (e *xiangjiaoHTTPError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "请求失败"
	}
	if e.Code != "" {
		return fmt.Sprintf("香蕉接口 %s: %s", e.Code, message)
	}
	return fmt.Sprintf("香蕉接口 HTTP %d: %s", e.Status, message)
}

func xiangjiaoMirrorHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, origin := range xiangjiaoBaseMirrors {
		parsed, err := url.Parse(origin)
		if err == nil && strings.EqualFold(parsed.Hostname(), host) {
			return true
		}
	}
	return false
}

func xiangjiaoText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			if text := strings.TrimSpace(nativeText(value)); text != "" {
				return text
			}
		}
	}
	return ""
}

func xiangjiaoBool(row map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		value, ok := row[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case json.Number:
			return typed.String() != "0", true
		case float64:
			return typed != 0, true
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "1", "true", "yes":
				return true, true
			case "0", "false", "no":
				return false, true
			}
		}
	}
	return false, false
}

func xiangjiaoInt(row map[string]any, keys ...string) int {
	for _, key := range keys {
		if text := xiangjiaoText(row, key); text != "" {
			if value, err := strconv.Atoi(text); err == nil {
				return value
			}
		}
	}
	return 0
}

func xiangjiaoMap(row map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value, ok := row[key].(map[string]any); ok {
			return value
		}
	}
	return nil
}

func xiangjiaoPaymentRequired(apiErr *xiangjiaoHTTPError) bool {
	if apiErr == nil {
		return false
	}
	code := strings.ToLower(strings.TrimSpace(apiErr.Code))
	message := strings.ToLower(strings.TrimSpace(apiErr.Message))
	for _, marker := range []string{"payment_required", "payment required", "paywall", "purchase", "membership", "need_pay", "need pay", "vip"} {
		if strings.Contains(code, marker) || strings.Contains(message, marker) {
			return true
		}
	}
	for _, marker := range []string{"会员", "购买", "付费", "充值", "需要支付"} {
		if strings.Contains(apiErr.Message, marker) {
			return true
		}
	}
	return false
}

func (d *Downloader) xiangjiaoJSON(ctx context.Context, method, path string, payload any, bearer string) (any, error) {
	var encodedPayload []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		encodedPayload = encoded
	}
	var lastErr error
	for _, base := range d.providerBaseCandidates(ctx, sourceXiangjiao, xiangjiaoBaseMirrors) {
		address := strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
		// 站点的 IPv6 出口时不时被重置（read: connection reset by peer），
		// 单次失败就放弃会让首页偶发空白，所以网络级错误重试一次。
		//
		// 0.8.10 第二批：用户反馈香蕉报 "443，connection reset by peer"、
		// 列表直接进不去。站点本身是好的（Cloudflare 后面，A 记录都在），
		// 是这条链路的 IPv6 被 RST —— 而 Go 默认 Happy Eyeballs **优先 IPv6**，
		// 不强制的话 IPv4 明明可用也轮不到。所以第一次就强制 IPv4，
		// 第二次回到默认，兜住"IPv4 不通、只有 IPv6 通"的反向情况。
		for attempt := 1; attempt <= 2; attempt++ {
			var body io.Reader
			if encodedPayload != nil {
				body = bytes.NewReader(encodedPayload)
			}
			requestCtx := ctx
			if attempt == 1 {
				requestCtx = withForceIPv4(ctx)
			}
			req, err := http.NewRequestWithContext(requestCtx, method, address, body)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Accept", "application/json")
			req.Header.Set("User-Agent", userAgent)
			req.Header.Set("Origin", strings.TrimRight(base, "/"))
			req.Header.Set("Referer", strings.TrimRight(base, "/")+"/")
			if payload != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			if strings.TrimSpace(bearer) != "" {
				req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearer))
			}
			resp, err := d.doCatalogRequestWithTimeout(req, 20*time.Second)
			if err != nil {
				lastErr = err
				if attempt == 1 && ctx.Err() == nil {
					select {
					case <-time.After(300 * time.Millisecond):
					case <-ctx.Done():
					}
					continue
				}
				break
			}
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
				break
			}
			if len(data) > 4<<20 {
				return nil, fmt.Errorf("香蕉接口响应过大")
			}
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.UseNumber()
			var decoded any
			decodeErr := decoder.Decode(&decoded)
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				apiErr := &xiangjiaoHTTPError{Status: resp.StatusCode}
				if root, ok := decoded.(map[string]any); ok {
					apiErr.Code = firstNonEmpty(xiangjiaoText(root, "code", "error_code", "errorCode"), xiangjiaoText(xiangjiaoMap(root, "error"), "code"))
					apiErr.Message = firstNonEmpty(xiangjiaoText(root, "message", "msg", "detail"), xiangjiaoText(xiangjiaoMap(root, "error"), "message", "msg", "detail"))
				}
				lastErr = apiErr
				normalizedPath := strings.TrimRight(path, "/")
				// 播放接口的 401 交给上层刷新游客 token；403 可能是购买限制，
				// 即使服务端没有返回 JSON 错误体，也不能切换镜像尝试规避访问控制。
				if normalizedPath == "/api/playback/sessions" &&
					(resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
					return nil, apiErr
				}
				break
			}
			if decodeErr != nil {
				lastErr = fmt.Errorf("香蕉接口 JSON 无效: %w", decodeErr)
				break
			}
			d.rememberProviderHost(sourceXiangjiao, base)
			return decoded, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("香蕉接口请求失败")
	}
	return nil, xiangjiaoRequestError(lastErr)
}

// xiangjiaoRequestError 把网络层原始错误换成用户看得懂的说明（0.8.10）。
//
// 用户截图里那行 "443，connection reset by peer" 是 Go 的原文，
// 界面上看只有一个 443 和一句 reset，既分不清是站点挂了还是本地网络问题，
// 也不知道客户端已经试过 IPv4 兜底。这里补一句人话，同时保留原始错误
// （用 %w 包住），排查时仍然能拿到根因。
func xiangjiaoRequestError(err error) error {
	if err == nil {
		return nil
	}
	// 接口层错误（401/403/购买限制）已经有明确语义，不要再包一层。
	var apiErr *xiangjiaoHTTPError
	if errors.As(err, &apiErr) {
		return err
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection reset",
		"broken pipe",
		"unexpected eof",
		"connection refused",
		"i/o timeout",
		"tls handshake timeout",
		"no such host",
		"network is unreachable",
	} {
		if strings.Contains(text, marker) {
			return fmt.Errorf("香蕉站源网络连接被中断（已自动改用 IPv4 重试仍失败，可试试切换网络或关闭代理）: %w", err)
		}
	}
	return err
}

// xiangjiaoCatalogJSON 取目录 / 搜索 / 详情接口，必要时自动补游客会话（0.8.10）。
//
// 用户反馈"香蕉现在列表都加载不出来"。目录这条路径原来一律匿名请求
// （bearer 传空），一旦站点给列表接口加上游客鉴权，就只剩 401/403，
// 整个站源表现为"列表空白、加载失败"，而播放反而还是好的 —— 因为播放
// 自己会去换 guest-sessions。这里让目录也具备同样的自愈能力：
// 匿名请求 401 时补一次游客 token 再重试。
func (d *Downloader) xiangjiaoCatalogJSON(ctx context.Context, path string) (any, error) {
	decoded, err := d.xiangjiaoJSON(ctx, http.MethodGet, path, nil, "")
	if err == nil {
		return decoded, nil
	}
	var apiErr *xiangjiaoHTTPError
	if !errors.As(err, &apiErr) {
		return nil, err
	}
	// 只对 401 补会话：403 会同时给 host 挂上退避（见 requestLimiter.observe），
	// 立刻重试也只会被限流器挡回来，还可能把 Cloudflare 拦得更久。
	if apiErr.Status != http.StatusUnauthorized {
		return nil, err
	}
	token, tokenErr := d.xiangjiaoGuestToken(ctx, true)
	if tokenErr != nil || strings.TrimSpace(token) == "" {
		return nil, err
	}
	return d.xiangjiaoJSON(ctx, http.MethodGet, path, nil, token)
}

func xiangjiaoPayloadData(value any) any {
	if root, ok := value.(map[string]any); ok {
		if data, exists := root["data"]; exists {
			return data
		}
	}
	return value
}

func xiangjiaoDramaFromMap(row map[string]any) Drama {
	// /api/home/hot wraps each item in {id,title,image_url,drama:{...}}.
	// Merge the nested drama fields so commerce metadata (including coin/VIP
	// status) is not lost when the outer item and nested object share the same ID.
	if nested := xiangjiaoMap(row, "drama"); nested != nil {
		merged := make(map[string]any, len(row)+len(nested))
		for key, value := range row {
			merged[key] = value
		}
		for key, value := range nested {
			current, exists := merged[key]
			if !exists || current == nil {
				merged[key] = value
				continue
			}
			if text, ok := current.(string); ok && strings.TrimSpace(text) == "" {
				merged[key] = value
			}
		}
		row = merged
	}
	id := firstNonEmpty(
		xiangjiaoText(row, "dramaId", "drama_id", "shortDramaId", "short_drama_id", "playletId", "playlet_id", "videoId", "video_id"),
		xiangjiaoText(row, "id"),
	)
	title := xiangjiaoText(row, "title", "name", "dramaTitle", "drama_title", "videoTitle", "video_title")
	if id == "" || title == "" {
		return Drama{}
	}
	cover := xiangjiaoText(row, "coverUrl", "cover_url", "cover", "poster")
	charge := strings.ToLower(xiangjiaoText(row, "chargeType", "charge_type"))
	if charge == "" {
		charge = strings.ToLower(xiangjiaoText(xiangjiaoMap(row, "commerce"), "chargeType", "charge_type"))
	}
	var vip *bool
	if charge != "" {
		value := charge != "free" && charge != "public"
		vip = &value
	}
	return Drama{
		VIP:          vip,
		ID:           providerDramaID(sourceXiangjiao, id),
		Source:       sourceXiangjiao,
		SourceID:     id,
		Title:        title,
		Name:         title,
		Desc:         xiangjiaoText(row, "description", "tagline", "intro"),
		Intro:        xiangjiaoText(row, "description", "tagline", "intro"),
		Cover:        cover,
		CoverURL:     cover,
		EpisodeCount: firstNonEmpty(xiangjiaoText(row, "episodeCount", "episode_count"), xiangjiaoText(row, "latestEpisode", "latest_episode")),
		CategoryName: xiangjiaoText(row, "categoryName", "category_name", "kind"),
		ChannelName:  "香蕉",
		Remark:       xiangjiaoText(row, "serialStatus", "serial_status", "badge"),
	}
}

// xiangjiaoListItems 取出列表响应里的数组。站点的列表都在 data.items 下
// （/api/home/hot、/api/search、/api/theater 同构），/api/categories 则是 data 数组。
func xiangjiaoListItems(value any) []any {
	data := xiangjiaoPayloadData(value)
	if list, ok := data.([]any); ok {
		return list
	}
	if row, ok := data.(map[string]any); ok {
		if list, ok := row["items"].([]any); ok {
			return list
		}
	}
	return nil
}

// xiangjiaoIsDramaItem 判断列表项是不是一部剧。列表项里嵌套的 categories[] /
// primary_category 都是 {id,name,slug} 的分类对象，没有 commerce；分类接口的项
// 还带 drama_count / definition。0.8.9 之前这里泛走整棵树，把这些分类对象当成
// 剧集，于是 32 部热播变成 68 部，还混进「校园青春」「成人动漫」「明星换脸」。
func xiangjiaoIsDramaItem(row map[string]any) bool {
	if kind := strings.ToLower(xiangjiaoText(row, "content_type", "contentType", "type")); kind != "" && kind != "drama" {
		return false
	}
	if _, ok := row["drama_count"]; ok {
		return false
	}
	if _, ok := row["definition"]; ok {
		return false
	}
	if _, ok := row["commerce"]; !ok {
		if _, ok := row["slug"]; ok {
			return false
		}
	}
	return xiangjiaoText(row, "title", "name", "dramaTitle", "drama_title", "videoTitle", "video_title") != ""
}

func xiangjiaoDramas(value any) []Drama {
	seen := map[string]bool{}
	var items []Drama
	for _, raw := range xiangjiaoListItems(value) {
		row, ok := raw.(map[string]any)
		if !ok || !xiangjiaoIsDramaItem(row) {
			continue
		}
		drama := xiangjiaoDramaFromMap(row)
		if drama.ID == "" || seen[drama.ID] {
			continue
		}
		seen[drama.ID] = true
		items = append(items, drama)
	}
	return items
}

const (
	xiangjiaoTheaterLimit = 50
	xiangjiaoSearchLimit  = 30
	xiangjiaoCursorTTL    = 10 * time.Minute
)

type xiangjiaoCursorEntry struct {
	next    string
	expires time.Time
}

// xiangjiaoCursors 记录"第 N 页之后的光标"。/api/theater 只认 cursor，没有
// page/offset（实测都无效），所以要拿第 5 页必须依次走前 4 页；缓存下来后
// 用户上下翻页就不用重复拉取。
var xiangjiaoCursors = struct {
	mu    sync.Mutex
	pages map[int]xiangjiaoCursorEntry
}{pages: map[int]xiangjiaoCursorEntry{}}

func xiangjiaoCursorAfter(page int) (string, bool) {
	xiangjiaoCursors.mu.Lock()
	defer xiangjiaoCursors.mu.Unlock()
	entry, ok := xiangjiaoCursors.pages[page]
	if !ok || time.Now().After(entry.expires) {
		return "", false
	}
	return entry.next, true
}

func xiangjiaoStoreCursor(page int, next string) {
	xiangjiaoCursors.mu.Lock()
	defer xiangjiaoCursors.mu.Unlock()
	xiangjiaoCursors.pages[page] = xiangjiaoCursorEntry{next: next, expires: time.Now().Add(xiangjiaoCursorTTL)}
}

func xiangjiaoNextCursor(value any) string {
	data := xiangjiaoPayloadData(value)
	row, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	return xiangjiaoText(row, "next_cursor", "nextCursor")
}

func xiangjiaoTheaterPath(cursor string) string {
	values := url.Values{}
	values.Set("limit", strconv.Itoa(xiangjiaoTheaterLimit))
	if cursor != "" {
		values.Set("cursor", cursor)
	}
	return "/api/theater?" + values.Encode()
}

// xiangjiaoTheaterPage 取剧场第 page 页。站点没有页码参数，只能顺着
// next_cursor 往前走。
func (d *Downloader) xiangjiaoTheaterPage(ctx context.Context, page int) ([]Drama, bool, error) {
	if page < 1 {
		page = 1
	}
	cursor := ""
	for current := 1; current < page; current++ {
		if next, ok := xiangjiaoCursorAfter(current); ok {
			if next == "" {
				return nil, false, nil
			}
			cursor = next
			continue
		}
		decoded, err := d.xiangjiaoCatalogJSON(ctx, xiangjiaoTheaterPath(cursor))
		if err != nil {
			return nil, false, err
		}
		next := xiangjiaoNextCursor(decoded)
		xiangjiaoStoreCursor(current, next)
		if next == "" {
			return nil, false, nil
		}
		cursor = next
	}
	decoded, err := d.xiangjiaoCatalogJSON(ctx, xiangjiaoTheaterPath(cursor))
	if err != nil {
		return nil, false, err
	}
	items := xiangjiaoDramas(decoded)
	next := xiangjiaoNextCursor(decoded)
	xiangjiaoStoreCursor(page, next)
	return items, next != "" && len(items) > 0, nil
}

func xiangjiaoSearchPath(query string) string {
	values := url.Values{}
	values.Set("q", query)
	values.Set("limit", strconv.Itoa(xiangjiaoSearchLimit))
	return "/api/search?" + values.Encode()
}

func (d *Downloader) fetchXiangjiaoCatalogPage(ctx context.Context, page int, query string) ([]Drama, bool, error) {
	if page < 1 {
		page = 1
	}
	if strings.TrimSpace(query) != "" {
		// /api/search 只认 q（keyword/query/term 一律 400），而且查询串必须做
		// URL 编码——直接塞中文会 400。返回结构同样是 data.items。
		if page > 1 {
			return nil, false, nil
		}
		decoded, err := d.xiangjiaoCatalogJSON(ctx, xiangjiaoSearchPath(query))
		if err != nil {
			return nil, false, err
		}
		items := xiangjiaoDramas(decoded)
		return items, len(items) >= xiangjiaoSearchLimit, nil
	}
	// 目录走 /api/theater（total 7000+，cursor 分页），不再用只有 32 部的
	// /api/home/hot，也不再在本地对首页做关键词过滤。
	return d.xiangjiaoTheaterPage(ctx, page)
}

func xiangjiaoEpisodeLocked(row map[string]any) bool {
	commerce := xiangjiaoMap(row, "commerce")
	if commerce == nil {
		commerce = row
	}
	if owned, ok := xiangjiaoBool(commerce, "owned"); ok && owned {
		return false
	}
	if canPlay, ok := xiangjiaoBool(commerce, "can_play", "canPlay"); ok {
		return !canPlay
	}
	charge := strings.ToLower(xiangjiaoText(commerce, "charge_type", "chargeType"))
	switch charge {
	case "", "free", "public":
		return false
	default:
		return true
	}
}

func xiangjiaoEpisodeRecord(row map[string]any) (string, int, bool) {
	explicitID := xiangjiaoText(row, "episode_id", "episodeId", "video_id", "videoId")
	id := firstNonEmpty(explicitID, xiangjiaoText(row, "id"))
	if id == "" {
		return "", 0, false
	}
	index := xiangjiaoInt(row, "episode_number", "episodeNumber", "episode_no", "episodeNo", "sort", "index")
	if _, hasNestedEpisodes := row["episodes"]; hasNestedEpisodes && index <= 0 && explicitID == "" {
		return "", 0, false
	}
	if index <= 0 && explicitID == "" && xiangjiaoText(row, "drama_id", "dramaId") == "" {
		return "", 0, false
	}
	return id, index, true
}

var xiangjiaoGuestRefreshMu sync.Mutex

func xiangjiaoEpisodeMarker(episodeID string) string {
	return "xiangjiao-play://episode/" + url.PathEscape(strings.TrimSpace(episodeID))
}

func xiangjiaoMarkerEpisode(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "xiangjiao-play" || parsed.Host != "episode" {
		return ""
	}
	value, _ := url.PathUnescape(strings.Trim(parsed.Path, "/"))
	return strings.TrimSpace(value)
}

func (d *Downloader) fetchXiangjiaoDetail(ctx context.Context, sourceID string) (Drama, []Chapter, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return Drama{}, nil, fmt.Errorf("香蕉剧集 ID 为空")
	}
	detailRaw, err := d.xiangjiaoCatalogJSON(ctx, "/api/dramas/"+url.PathEscape(sourceID))
	if err != nil {
		return Drama{}, nil, err
	}
	episodesRaw, err := d.xiangjiaoCatalogJSON(ctx, "/api/dramas/"+url.PathEscape(sourceID)+"/episodes")
	if err != nil {
		return Drama{}, nil, err
	}

	var drama Drama
	if row, ok := xiangjiaoPayloadData(detailRaw).(map[string]any); ok {
		drama = xiangjiaoDramaFromMap(row)
	}
	if drama.ID == "" {
		drama = Drama{ID: providerDramaID(sourceXiangjiao, sourceID), Source: sourceXiangjiao, SourceID: sourceID, Title: "香蕉 " + sourceID, Name: "香蕉 " + sourceID, ChannelName: "香蕉"}
	}

	type episodeRow struct {
		id     string
		index  int
		title  string
		locked bool
	}
	seen := map[string]bool{}
	var episodes []episodeRow
	// 分集在 data 数组里，每项是 {id, drama_id, episode_number, title, commerce}。
	// 不要泛走整棵树：每项里嵌套的 primary_category / categories 也带 id，
	// 会被当成"分集"混进来。
	for _, raw := range xiangjiaoListItems(episodesRaw) {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, hasCategory := row["slug"]; hasCategory {
			continue
		}
		if _, hasCount := row["drama_count"]; hasCount {
			continue
		}
		id, index, ok := xiangjiaoEpisodeRecord(row)
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		episodes = append(episodes, episodeRow{
			id:     id,
			index:  index,
			title:  xiangjiaoText(row, "title", "name", "episodeTitle", "episode_title"),
			locked: xiangjiaoEpisodeLocked(row),
		})
	}
	if len(episodes) == 0 {
		return Drama{}, nil, fmt.Errorf("香蕉详情未返回分集")
	}
	for index := range episodes {
		if episodes[index].index <= 0 {
			episodes[index].index = index + 1
		}
	}
	sort.SliceStable(episodes, func(i, j int) bool { return episodes[i].index < episodes[j].index })
	base := d.providerBaseURL(sourceXiangjiao)
	chapters := make([]Chapter, 0, len(episodes))
	freeFound := false
	for _, episode := range episodes {
		title := strings.TrimSpace(episode.title)
		if title == "" {
			title = fmt.Sprintf("第%d集", episode.index)
		}
		chapter := Chapter{
			VIP:            episode.locked,
			ID:             providerChapterID(sourceXiangjiao, sourceID, episode.id),
			Source:         sourceXiangjiao,
			Title:          title,
			PageURL:        strings.TrimRight(base, "/") + "/dramas/" + url.PathEscape(sourceID),
			Referer:        strings.TrimRight(base, "/") + "/",
			CurrentEpisode: rawEpisode(episode.index),
		}
		chapter.VideoURL = xiangjiaoEpisodeMarker(episode.id)
		if !episode.locked {
			freeFound = true
		}
		chapters = append(chapters, chapter)
	}
	vip := !freeFound
	drama.VIP = &vip
	drama.EpisodeCount = strconv.Itoa(len(chapters))
	drama.TotalEpisode = strconv.Itoa(len(chapters))
	return drama, chapters, nil
}

func newXiangjiaoDeviceID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		data[0:4], data[4:6], data[6:8], data[8:10], data[10:16]), nil
}

func (d *Downloader) xiangjiaoGuestToken(ctx context.Context, force bool) (string, error) {
	xiangjiaoGuestRefreshMu.Lock()
	defer xiangjiaoGuestRefreshMu.Unlock()
	d.xiangjiaoSessionMu.Lock()
	if !force && d.xiangjiaoAccessToken != "" && time.Now().Add(time.Minute).Before(d.xiangjiaoAccessExpires) {
		token := d.xiangjiaoAccessToken
		d.xiangjiaoSessionMu.Unlock()
		return token, nil
	}
	deviceID := d.xiangjiaoDeviceID
	d.xiangjiaoSessionMu.Unlock()
	if deviceID == "" {
		var err error
		deviceID, err = newXiangjiaoDeviceID()
		if err != nil {
			return "", err
		}
	}

	decoded, err := d.xiangjiaoJSON(ctx, http.MethodPost, "/api/guest-sessions", map[string]any{"device_id": deviceID}, "")
	if err != nil {
		return "", err
	}
	data, _ := xiangjiaoPayloadData(decoded).(map[string]any)
	token := xiangjiaoText(data, "access_token", "accessToken")
	if token == "" {
		return "", fmt.Errorf("香蕉游客会话未返回 access_token")
	}
	expires := time.Now().Add(xiangjiaoSessionTTL)
	if raw := xiangjiaoText(data, "expires_at", "expiresAt"); raw != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, raw); parseErr == nil {
			expires = parsed
		}
	}
	d.xiangjiaoSessionMu.Lock()
	d.xiangjiaoDeviceID = deviceID
	d.xiangjiaoAccessToken = token
	d.xiangjiaoAccessExpires = expires
	d.xiangjiaoSessionMu.Unlock()
	return token, nil
}

func (d *Downloader) clearXiangjiaoGuestToken() {
	d.xiangjiaoSessionMu.Lock()
	d.xiangjiaoAccessToken = ""
	d.xiangjiaoAccessExpires = time.Time{}
	d.xiangjiaoSessionMu.Unlock()
}

func (d *Downloader) xiangjiaoFullPlaybackManifest(ctx context.Context, dramaID, episodeID string) (string, error) {
	for attempt := 0; attempt < 2; attempt++ {
		token, err := d.xiangjiaoGuestToken(ctx, attempt > 0)
		if err != nil {
			return "", err
		}
		payload := map[string]any{"episode_id": episodeID}
		if strings.TrimSpace(dramaID) != "" {
			payload["drama_id"] = strings.TrimSpace(dramaID)
		}
		decoded, err := d.xiangjiaoJSON(ctx, http.MethodPost, "/api/playback/sessions", payload, token)
		if err != nil {
			if apiErr, ok := err.(*xiangjiaoHTTPError); ok {
				if apiErr.Status == http.StatusUnauthorized && attempt == 0 {
					d.clearXiangjiaoGuestToken()
					continue
				}
				if apiErr.Status == http.StatusForbidden || xiangjiaoPaymentRequired(apiErr) {
					// 0.8.10：正片被拒时退一步拿试看会话。
					//
					// 香蕉的正片对游客一律 403 `purchase_required`，客户端没有
					// 合法路径拿全集。但站点对游客开放了 trial 会话（前几分钟），
					// 原来这里直接报错，用户看到的就是"香蕉完全不能播放"。
					// 现在先试试看：能拿到就播（时长由服务端截断），拿不到再报错。
					if preview, previewErr := d.xiangjiaoTrialManifest(ctx, dramaID, episodeID, token); previewErr == nil && preview != "" {
						return preview, nil
					}
					return "", fmt.Errorf("香蕉该集需要会员或购买，服务端拒绝游客正片播放")
				}
			}
			return "", err
		}
		data, _ := xiangjiaoPayloadData(decoded).(map[string]any)
		mode := strings.ToLower(xiangjiaoText(data, "play_mode", "playMode"))
		// 0.8.10：trial / preview 会话同样可用（就是试看片段），不再直接判失败。
		if mode != "full" && mode != "trial" && mode != "preview" {
			return "", fmt.Errorf("香蕉服务端未返回可用播放会话（%s）", firstNonEmpty(mode, "空"))
		}
		media := xiangjiaoMap(data, "media")
		manifest := xiangjiaoText(media, "manifest_url", "manifestUrl", "url")
		if manifest == "" {
			return "", fmt.Errorf("香蕉正片会话未返回 manifest_url")
		}
		if strings.HasPrefix(manifest, "/") {
			manifest = strings.TrimRight(d.providerBaseURL(sourceXiangjiao), "/") + manifest
		}
		return manifest, nil
	}
	return "", fmt.Errorf("香蕉游客会话刷新失败")
}

// xiangjiaoTrialManifest 取"试看会话"的播放列表（0.8.10）。
//
// 只在正片会话被 403 时调用：站点对游客开放试看，能拿到 manifest 就说明
// 这一集至少能看前几分钟，比直接报"不能播放"有用。
// 返回空字符串表示站点没有试看接口或不给试看。
func (d *Downloader) xiangjiaoTrialManifest(ctx context.Context, dramaID, episodeID, token string) (string, error) {
	// 正片那次 403 会给 host 挂上退避，紧接着的试看请求会被限流器直接挡回
	// （连请求都发不出去），所以这里跳过源级门/退避 —— 只是一次性的兜底请求。
	ctx = context.WithValue(ctx, skipCatalogGateKey{}, true)
	payload := map[string]any{"episode_id": episodeID}
	if strings.TrimSpace(dramaID) != "" {
		payload["drama_id"] = strings.TrimSpace(dramaID)
	}
	for _, path := range []string{"/api/trial/sessions", "/api/preview/sessions"} {
		decoded, err := d.xiangjiaoJSON(ctx, http.MethodPost, path, payload, token)
		if err != nil {
			continue
		}
		data, _ := xiangjiaoPayloadData(decoded).(map[string]any)
		manifest := xiangjiaoText(xiangjiaoMap(data, "media"), "manifest_url", "manifestUrl", "url")
		if manifest == "" {
			manifest = xiangjiaoText(data, "manifest_url", "manifestUrl", "trial_url", "url")
		}
		if manifest == "" {
			continue
		}
		if strings.HasPrefix(manifest, "/") {
			manifest = strings.TrimRight(d.providerBaseURL(sourceXiangjiao), "/") + manifest
		}
		return manifest, nil
	}
	return "", nil
}

func (d *Downloader) resolveXiangjiaoPlayback(ctx context.Context, task Task) (providerMedia, error) {
	episodeID := xiangjiaoMarkerEpisode(task.Chapter.VideoURL)
	if episodeID == "" {
		return providerMedia{}, fmt.Errorf("香蕉分集播放标识无效，请刷新详情")
	}
	_, dramaID, validDrama := splitProviderDramaID(task.DramaID)
	if !validDrama {
		dramaID = ""
	}
	manifest, err := d.xiangjiaoFullPlaybackManifest(ctx, dramaID, episodeID)
	if err != nil {
		return providerMedia{}, err
	}
	return providerMedia{URL: manifest, Referer: firstNonEmpty(task.Chapter.Referer, d.providerBaseURL(sourceXiangjiao)+"/")}, nil
}
