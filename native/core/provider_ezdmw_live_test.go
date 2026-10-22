package core

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// resolveEZDMWLiveTestURL resolves playlist references without applying the
// production helper's HTTP-to-HTTPS upgrade. The native playback gateway is
// intentionally served over plain HTTP on loopback.
func resolveEZDMWLiveTestURL(base, reference string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return reference
	}
	referenceURL, err := url.Parse(reference)
	if err != nil {
		return reference
	}
	return baseURL.ResolveReference(referenceURL).String()
}

func firstEZDMWHLSResource(playlist string) string {
	for _, line := range strings.Split(playlist, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

// TestEZDMWLiveCatalogDetailPlayback is an opt-in end-to-end smoke test against
// the real E站 site. Set EZDMW_LIVE_TEST=1 to run it; normal unit tests stay deterministic.
func TestEZDMWLiveCatalogDetailPlayback(t *testing.T) {
	if os.Getenv("EZDMW_LIVE_TEST") != "1" {
		t.Skip("set EZDMW_LIVE_TEST=1 to test the live E站 catalog and playback")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	engine, err := newNativeEngine(t.TempDir())
	if err != nil {
		t.Fatalf("initialize native engine: %v", err)
	}
	d := engine.downloader

	items, _, err := d.fetchEZDMWCatalogPage(ctx, 1, "", "学生会也有洞")
	if err != nil {
		t.Fatalf("live E站 search/catalog failed: %v", err)
	}
	var selected Drama
	for _, item := range items {
		if strings.Contains(item.Title, "学生会也有洞") {
			selected = item
			break
		}
	}
	if selected.ID == "" {
		t.Fatalf("live E站 search returned %d results but not the expected title", len(items))
	}

	drama, chapters, err := d.fetchEZDMWDetail(ctx, selected.SourceID)
	if err != nil {
		t.Fatalf("live E站 detail parse failed: %v", err)
	}
	if !strings.Contains(drama.DisplayTitle(), "学生会也有洞") {
		t.Fatalf("live E站 detail title mismatch: %q", drama.DisplayTitle())
	}
	if len(chapters) == 0 {
		t.Fatalf("live E站 detail %q returned no episodes", drama.DisplayTitle())
	}

	task := Task{DramaID: drama.ID, DramaTitle: drama.DisplayTitle(), Chapter: chapters[0], Index: 0}
	media, err := d.resolveProviderMedia(ctx, task)
	if err != nil {
		t.Fatalf("live E站 playback resolution failed: %v", err)
	}
	parsed, err := url.Parse(media.URL)
	if err != nil || parsed.Host == "" {
		t.Fatal("live E站 playback resolver returned an invalid media URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, media.URL, nil)
	if err != nil {
		t.Fatalf("create playback probe: %v", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", media.Referer)
	req.Header.Set("Range", "bytes=0-1023")
	resp, err := d.client.Do(req)
	if err != nil {
		t.Fatalf("live E站 media endpoint request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("live E站 media endpoint returned HTTP %d", resp.StatusCode)
	}
	n, readErr := io.CopyN(io.Discard, resp.Body, 1)
	if n == 0 || (readErr != nil && readErr != io.EOF) {
		t.Fatalf("live E站 media endpoint returned no readable bytes (read=%d, err=%v)", n, readErr)
	}
	plan, err := engine.nativeOpenPlayback(ctx, nativePlaybackChoices(media, 0))
	if err != nil {
		t.Fatalf("open E站 through the native playback gateway: %v", err)
	}
	localClient := &http.Client{Timeout: 15 * time.Second}
	localReq, err := http.NewRequestWithContext(ctx, http.MethodGet, plan.URL, nil)
	if err != nil {
		t.Fatalf("create local playback request: %v", err)
	}
	localResp, err := localClient.Do(localReq)
	if err != nil {
		t.Fatalf("native playback gateway request failed: %v", err)
	}
	playlistBytes, readErr := io.ReadAll(io.LimitReader(localResp.Body, 1<<20))
	localResp.Body.Close()
	if readErr != nil {
		t.Fatalf("read native HLS playlist: %v", readErr)
	}
	if localResp.StatusCode < 200 || localResp.StatusCode >= 300 || !strings.HasPrefix(strings.TrimSpace(string(playlistBytes)), "#EXTM3U") {
		t.Fatalf("native playback gateway did not return a valid HLS playlist (HTTP %d)", localResp.StatusCode)
	}
	resource := firstEZDMWHLSResource(string(playlistBytes))
	if resource == "" {
		t.Fatal("native HLS playlist contains no variant or segment URL")
	}
	resourceURL := resolveEZDMWLiveTestURL(plan.URL, resource)
	resourceReq, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, nil)
	if err != nil {
		t.Fatalf("create native HLS resource request: %v", err)
	}
	resourceResp, err := localClient.Do(resourceReq)
	if err != nil {
		t.Fatalf("native HLS resource request failed: %v", err)
	}
	resourceBytes, resourceReadErr := io.ReadAll(io.LimitReader(resourceResp.Body, 1<<20))
	resourceResp.Body.Close()
	if resourceReadErr != nil {
		t.Fatalf("read native HLS resource: %v", resourceReadErr)
	}
	if resourceResp.StatusCode < 200 || resourceResp.StatusCode >= 300 || len(resourceBytes) == 0 {
		t.Fatalf("native HLS resource unavailable (HTTP %d, bytes=%d)", resourceResp.StatusCode, len(resourceBytes))
	}
	if strings.HasPrefix(strings.TrimSpace(string(resourceBytes)), "#EXTM3U") {
		segment := firstEZDMWHLSResource(string(resourceBytes))
		if segment == "" {
			t.Fatal("variant HLS playlist contains no media segment")
		}
		segmentURL := resolveEZDMWLiveTestURL(resourceURL, segment)
		segmentReq, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, segmentURL, nil)
		if reqErr != nil {
			t.Fatalf("create segment request: %v", reqErr)
		}
		segmentResp, reqErr := localClient.Do(segmentReq)
		if reqErr != nil {
			t.Fatalf("native media segment request failed: %v", reqErr)
		}
		n, _ := io.CopyN(io.Discard, segmentResp.Body, 1)
		segmentResp.Body.Close()
		if segmentResp.StatusCode < 200 || segmentResp.StatusCode >= 300 || n == 0 {
			t.Fatalf("native media segment unavailable (HTTP %d, bytes=%d)", segmentResp.StatusCode, n)
		}
	}
	t.Logf("live E站 playback passed: %d search results, %d episodes, upstream=%s, native HLS gateway and first resource verified", len(items), len(chapters), parsed.Host)
}

// TestEZDMWLivePlaybackSpacedTitle 专门盯"标题里带空格"的番剧（0.8.9）。
//
// 之前 0.8.9 的冒烟测试挑的是「学生会也有洞」——标题没有空格，正好绕过了
// iframe 地址未转义的坑，于是测试全绿而线上首页约一半番剧播不了。
// 这个用例从真实目录里**只挑标题含空格**的番剧，确保转义逻辑不会退化。
func TestEZDMWLivePlaybackSpacedTitle(t *testing.T) {
	if os.Getenv("EZDMW_LIVE_TEST") != "1" {
		t.Skip("set EZDMW_LIVE_TEST=1 to test the live E站 spaced-title playback")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	engine, err := newNativeEngine(t.TempDir())
	if err != nil {
		t.Fatalf("initialize native engine: %v", err)
	}
	d := engine.downloader

	items, _, err := d.fetchEZDMWCatalogPage(ctx, 1, "", "")
	if err != nil {
		t.Fatalf("live E站 catalog failed: %v", err)
	}
	var selected Drama
	for _, item := range items {
		if strings.ContainsAny(item.Title, " \u00a0") {
			selected = item
			break
		}
	}
	if selected.ID == "" {
		t.Skipf("live E站 catalog has no spaced title among %d items", len(items))
	}

	drama, chapters, err := d.fetchEZDMWDetail(ctx, selected.SourceID)
	if err != nil {
		t.Fatalf("live E站 detail for %q failed: %v", drama.DisplayTitle(), err)
	}
	if len(chapters) == 0 {
		t.Fatalf("live E站 detail %q returned no episodes", drama.DisplayTitle())
	}
	task := Task{DramaID: drama.ID, DramaTitle: drama.DisplayTitle(), Chapter: chapters[0]}
	media, err := d.resolveProviderMedia(ctx, task)
	if err != nil {
		t.Fatalf("live E站 spaced-title playback failed (%q): %v", drama.DisplayTitle(), err)
	}
	if strings.ContainsAny(media.URL, " \t\r\n") {
		t.Fatalf("resolved media URL still contains raw whitespace: %q", media.URL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, media.URL, nil)
	if err != nil {
		t.Fatalf("create media probe: %v", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", media.Referer)
	req.Header.Set("Range", "bytes=0-1023")
	resp, err := d.client.Do(req)
	if err != nil {
		t.Fatalf("live E站 media request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("live E站 media endpoint returned HTTP %d", resp.StatusCode)
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		t.Fatalf("read media body: %v", readErr)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(body)), "#EXTM3U") {
		t.Fatalf("live E站 media is not an HLS playlist (%q)", string(body[:min(len(body), 40)]))
	}
	t.Logf("live E站 spaced-title playback passed: %q (%d episodes) -> %s", drama.DisplayTitle(), len(chapters), media.URL)
}

// TestEZDMWLiveCatalogPagination 盯"能不能枚举整个站"（0.8.9）。
//
// 站点的 `/` 和 `/Index/anime.html` 都忽略 `page`，目录长期只能拿首页 40 部；
// 真正的翻页只有 /Index/search_some.html，而且它的 page 是**条目偏移量**。
// 这个用例验证三件事：目录第 2 页之后确实在往下走、搜索能翻页、翻到底不报错。
func TestEZDMWLiveCatalogPagination(t *testing.T) {
	if os.Getenv("EZDMW_LIVE_TEST") != "1" {
		t.Skip("set EZDMW_LIVE_TEST=1 to test the live E站 pagination")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	engine, err := newNativeEngine(t.TempDir())
	if err != nil {
		t.Fatalf("initialize native engine: %v", err)
	}
	d := engine.downloader

	first, more, err := d.fetchEZDMWCatalogPage(ctx, 1, "", "")
	if err != nil {
		t.Fatalf("live E站 catalog page 1 failed: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("live E站 catalog page 1 is empty")
	}
	if !more {
		t.Fatal("目录第 1 页之后必须能继续翻（hasMore=false 会让首页永远只有 40 部）")
	}
	seen := map[string]bool{}
	for _, item := range first {
		seen[item.ID] = true
	}

	second, more, err := d.fetchEZDMWCatalogPage(ctx, 2, "", "")
	if err != nil {
		t.Fatalf("live E站 catalog page 2 failed: %v", err)
	}
	if len(second) != ezdmwSearchPageSize {
		t.Fatalf("目录第 2 页拿到 %d 条，want %d", len(second), ezdmwSearchPageSize)
	}
	if !more {
		t.Fatal("目录第 2 页之后仍应有下一页")
	}
	fresh := 0
	for _, item := range second {
		if !seen[item.ID] {
			fresh++
		}
	}
	if fresh == 0 {
		t.Fatalf("目录第 2 页没有带来任何新番剧，分页参数可能又退化成页码了：%v", second)
	}

	third, _, err := d.fetchEZDMWCatalogPage(ctx, 3, "", "")
	if err != nil {
		t.Fatalf("live E站 catalog page 3 failed: %v", err)
	}
	for _, item := range second {
		seen[item.ID] = true
	}
	thirdFresh := 0
	for _, item := range third {
		if !seen[item.ID] {
			thirdFresh++
		}
	}
	if thirdFresh != len(third) {
		t.Fatalf("目录第 3 页与第 2 页重叠了 %d 条，偏移步长不对", len(third)-thirdFresh)
	}

	// 搜索：第 1、2 页必须是不相交的两批（站点每页 10 条）
	searchFirst, searchMore, err := d.fetchEZDMWCatalogPage(ctx, 1, "", "之")
	if err != nil {
		t.Fatalf("live E站 search page 1 failed: %v", err)
	}
	if len(searchFirst) == 0 {
		t.Skip("live E站 search for 之 returned nothing")
	}
	if !searchMore {
		t.Fatalf("搜索结果只有 %d 条就报 hasMore=false，分页没生效", len(searchFirst))
	}
	searchSecond, _, err := d.fetchEZDMWCatalogPage(ctx, 2, "", "之")
	if err != nil {
		t.Fatalf("live E站 search page 2 failed: %v", err)
	}
	overlap := 0
	for _, a := range searchFirst {
		for _, b := range searchSecond {
			if a.ID == b.ID {
				overlap++
			}
		}
	}
	if overlap != 0 {
		t.Fatalf("搜索第 1、2 页重叠 %d 条，偏移步长不对", overlap)
	}

	// 搜不到时必须是"空页"而不是错误，否则 UI 会显示成站点结构变了
	empty, emptyMore, err := d.fetchEZDMWCatalogPage(ctx, 1, "", "zzzzqqqzzzz")
	if err != nil {
		t.Fatalf("无结果搜索不应该报错：%v", err)
	}
	if len(empty) != 0 || emptyMore {
		t.Fatalf("无结果搜索返回了 %d 条（hasMore=%v）", len(empty), emptyMore)
	}
	t.Logf("live E站 pagination passed: page1=%d, page2 新增 %d/%d, page3 全部新, 搜索两页无重叠",
		len(first), fresh, len(second))
}
