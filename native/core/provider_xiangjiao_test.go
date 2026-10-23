package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestXiangjiaoJSONRetriesForbiddenPrimaryMirror(t *testing.T) {
	resetProviderMirrorCaches()
	var primaryCalls atomic.Int32
	var mirrorCalls atomic.Int32
	downloader := sourceFixtureDownloader(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == providerMirrorConfigURL {
			return sourceFixtureResponse(request, http.StatusOK, `{"version":1,"ttl_seconds":3600,"sources":{"xiangjiao":["https://mirror.example.test"]}}`), nil
		}
		if request.URL.Hostname() == "xiangjiaoai.ai" {
			primaryCalls.Add(1)
			return sourceFixtureResponse(request, http.StatusForbidden, `{"code":"EDGE_BLOCKED","message":"upstream denied"}`), nil
		}
		if request.URL.Hostname() == "mirror.example.test" {
			mirrorCalls.Add(1)
			if got := request.Header.Get("Origin"); got != "https://mirror.example.test" {
				t.Errorf("Origin = %q, want mirror origin", got)
			}
			if got := request.Header.Get("Referer"); got != "https://mirror.example.test/" {
				t.Errorf("Referer = %q, want mirror referer", got)
			}
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"access_token":"fixture-token"}}`), nil
		}
		return nil, fmt.Errorf("unexpected host %q", request.URL.Hostname())
	})
	downloader.providerMu.Lock()
	if downloader.providerHosts == nil {
		downloader.providerHosts = map[string]string{}
	}
	downloader.providerHosts[sourceXiangjiao] = "https://xiangjiaoai.ai"
	downloader.providerMu.Unlock()

	decoded, err := downloader.xiangjiaoJSON(context.Background(), http.MethodPost, "/api/guest-sessions", map[string]any{"device_id": "fixture-device"}, "")
	if err != nil {
		t.Fatalf("xiangjiaoJSON: %v", err)
	}
	data, _ := xiangjiaoPayloadData(decoded).(map[string]any)
	if xiangjiaoText(data, "access_token") != "fixture-token" {
		t.Fatalf("unexpected mirror response: %#v", decoded)
	}
	if primaryCalls.Load() == 0 || mirrorCalls.Load() == 0 {
		t.Fatalf("expected both primary and remote mirror to be tried; primary=%d mirror=%d", primaryCalls.Load(), mirrorCalls.Load())
	}
}

func TestXiangjiaoCatalogKeepsCoinLockedDramasVisible(t *testing.T) {
	resetProviderMirrorCaches()
	var playbackCalls atomic.Int32
	downloader := sourceFixtureDownloader(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == providerMirrorConfigURL {
			return sourceFixtureResponse(request, http.StatusOK, `{"version":1,"ttl_seconds":3600,"sources":{}}`), nil
		}
		if request.Method == http.MethodGet && request.URL.Path == "/api/theater" {
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"total":1,"next_cursor":"","items":[{"id":"coin-drama-fixture","title":"付费短剧","cover_url":"https://media.example.test/cover.jpg","commerce":{"charge_type":"coin"},"episode_count":2,"categories":[{"id":"cat-1","name":"校园青春","slug":"xiaoyuanqingchun"}]}]}}`), nil
		}
		if request.URL.Path == "/api/playback/sessions" {
			playbackCalls.Add(1)
		}
		return nil, fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
	})

	items, more, err := downloader.fetchXiangjiaoCatalogPage(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("fetchXiangjiaoCatalogPage: %v", err)
	}
	if more || len(items) != 1 {
		t.Fatalf("catalog returned more=%v, items=%d; want one visible drama", more, len(items))
	}
	if items[0].Title != "付费短剧" || items[0].VIP == nil || !*items[0].VIP {
		t.Fatalf("coin commerce metadata was not preserved: %#v", items[0])
	}
	if playbackCalls.Load() != 0 {
		t.Fatalf("catalog loading must not probe paid playback sessions, got %d", playbackCalls.Load())
	}
}

// 列表项里嵌套的 categories[] 是分类对象，不能被当成剧集。
func TestXiangjiaoCatalogIgnoresNestedCategories(t *testing.T) {
	resetProviderMirrorCaches()
	downloader := sourceFixtureDownloader(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == providerMirrorConfigURL {
			return sourceFixtureResponse(request, http.StatusOK, `{"version":1,"ttl_seconds":3600,"sources":{}}`), nil
		}
		if request.Method == http.MethodGet && request.URL.Path == "/api/theater" {
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"total":2,"next_cursor":"","items":[
				{"id":"drama-a","title":"剧A","commerce":{"charge_type":"free"},"categories":[{"id":"cat-1","name":"校园青春","slug":"xiaoyuanqingchun"}],"primary_category":{"id":"cat-2","name":"成人动漫","slug":"chengrendongman"}},
				{"id":"drama-b","title":"剧B","commerce":{"charge_type":"coin"},"categories":[{"id":"cat-1","name":"校园青春","slug":"xiaoyuanqingchun"}]}
			]}}`), nil
		}
		return nil, fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
	})

	items, _, err := downloader.fetchXiangjiaoCatalogPage(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("fetchXiangjiaoCatalogPage: %v", err)
	}
	if len(items) != 2 {
		titles := make([]string, 0, len(items))
		for _, item := range items {
			titles = append(titles, item.Title)
		}
		t.Fatalf("got %d items %v, want exactly the 2 dramas", len(items), titles)
	}
	for _, item := range items {
		if item.Title == "校园青春" || item.Title == "成人动漫" {
			t.Fatalf("category leaked into catalog: %q", item.Title)
		}
	}
}

// 搜索必须走 /api/search?q=，且查询串要 URL 编码（直接塞中文站点回 400）。
func TestXiangjiaoSearchUsesEncodedQuery(t *testing.T) {
	resetProviderMirrorCaches()
	var gotQuery, gotPath string
	downloader := sourceFixtureDownloader(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == providerMirrorConfigURL {
			return sourceFixtureResponse(request, http.StatusOK, `{"version":1,"ttl_seconds":3600,"sources":{}}`), nil
		}
		if request.Method == http.MethodGet && request.URL.Path == "/api/search" {
			gotPath = request.URL.Path
			gotQuery = request.URL.RawQuery
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"items":[{"id":"search-hit","title":"总裁的替身新娘","type":"drama"}]}}`), nil
		}
		return nil, fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
	})

	items, _, err := downloader.fetchXiangjiaoCatalogPage(context.Background(), 1, "总裁")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if gotPath != "/api/search" {
		t.Fatalf("search path = %q, want /api/search", gotPath)
	}
	if !strings.Contains(gotQuery, "q=%E6%80%BB%E8%A3%81") {
		t.Fatalf("search query = %q, want percent-encoded 总裁", gotQuery)
	}
	if !strings.Contains(gotQuery, "limit=30") {
		t.Fatalf("search query = %q, want limit=30", gotQuery)
	}
	if len(items) != 1 || items[0].Title != "总裁的替身新娘" {
		t.Fatalf("unexpected search result: %#v", items)
	}
}

// 剧场只有 cursor 分页：第 2 页必须带第 1 页返回的 next_cursor。
func TestXiangjiaoTheaterPaginationFollowsCursor(t *testing.T) {
	resetProviderMirrorCaches()
	xiangjiaoCursors.mu.Lock()
	xiangjiaoCursors.pages = map[int]xiangjiaoCursorEntry{}
	xiangjiaoCursors.mu.Unlock()
	var cursors []string
	downloader := sourceFixtureDownloader(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == providerMirrorConfigURL {
			return sourceFixtureResponse(request, http.StatusOK, `{"version":1,"ttl_seconds":3600,"sources":{}}`), nil
		}
		if request.Method == http.MethodGet && request.URL.Path == "/api/theater" {
			cursor := request.URL.Query().Get("cursor")
			cursors = append(cursors, cursor)
			if cursor == "" {
				return sourceFixtureResponse(request, http.StatusOK, `{"data":{"total":2,"next_cursor":"cursor-page-2","items":[{"id":"drama-a","title":"剧A","commerce":{"charge_type":"free"}}]}}`), nil
			}
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"total":2,"next_cursor":"","items":[{"id":"drama-b","title":"剧B","commerce":{"charge_type":"free"}}]}}`), nil
		}
		return nil, fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
	})

	if _, _, err := downloader.fetchXiangjiaoCatalogPage(context.Background(), 1, ""); err != nil {
		t.Fatalf("page 1: %v", err)
	}
	items, more, err := downloader.fetchXiangjiaoCatalogPage(context.Background(), 2, "")
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if more {
		t.Fatalf("page 2 should be the last page")
	}
	if len(items) != 1 || items[0].Title != "剧B" {
		t.Fatalf("unexpected page 2 items: %#v", items)
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "cursor-page-2" {
		t.Fatalf("cursor chain = %v, want [\"\" \"cursor-page-2\"]", cursors)
	}
}

func TestXiangjiaoDetailMarksCoinEpisodesLocked(t *testing.T) {
	resetProviderMirrorCaches()
	const sourceID = "detail-fixture"
	var playbackCalls atomic.Int32
	downloader := sourceFixtureDownloader(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == providerMirrorConfigURL {
			return sourceFixtureResponse(request, http.StatusOK, `{"version":1,"ttl_seconds":3600,"sources":{}}`), nil
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/dramas/"+sourceID:
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"id":"detail-fixture","title":"香蕉测试剧","commerce":{"charge_type":"coin"}}}`), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/dramas/"+sourceID+"/episodes":
			return sourceFixtureResponse(request, http.StatusOK, `{"data":[
				{"id":"ep-locked","episode_number":1,"drama_id":"detail-fixture","title":"第1集","commerce":{"charge_type":"coin","can_play":false,"owned":false,"reason":"purchase_required"}},
				{"id":"ep-free","episode_number":2,"drama_id":"detail-fixture","title":"第2集","commerce":{"charge_type":"free","can_play":true,"owned":false}}
			]}`), nil
		case request.URL.Path == "/api/playback/sessions":
			playbackCalls.Add(1)
			return sourceFixtureResponse(request, http.StatusForbidden, ""), nil
		default:
			return nil, fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	})

	drama, chapters, err := downloader.fetchXiangjiaoDetail(context.Background(), sourceID)
	if err != nil {
		t.Fatalf("fetchXiangjiaoDetail: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("got %d chapters, want 2", len(chapters))
	}
	if !chapters[0].VIP || chapters[1].VIP {
		t.Fatalf("episode lock flags incorrect: first=%v second=%v", chapters[0].VIP, chapters[1].VIP)
	}
	if drama.VIP == nil || *drama.VIP {
		t.Fatalf("a drama with at least one free episode should not be marked entirely VIP: %#v", drama.VIP)
	}
	if playbackCalls.Load() != 0 {
		t.Fatalf("detail loading must not call playback sessions for locked episodes, got %d", playbackCalls.Load())
	}
}

func TestXiangjiaoPlayback403IsTreatedAsPaymentDenial(t *testing.T) {
	resetProviderMirrorCaches()
	const sourceID = "payment-fixture"
	var playbackCalls atomic.Int32
	var mirrorPlaybackCalls atomic.Int32
	downloader := sourceFixtureDownloader(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == providerMirrorConfigURL {
			return sourceFixtureResponse(request, http.StatusOK, `{"version":1,"ttl_seconds":3600,"sources":{}}`), nil
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/guest-sessions":
			return sourceFixtureResponse(request, http.StatusCreated, `{"data":{"access_token":"fixture-token","expires_at":"2099-01-01T00:00:00Z"}}`), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/playback/sessions" && request.URL.Hostname() == "xiangjiaoai.ai":
			playbackCalls.Add(1)
			return sourceFixtureResponse(request, http.StatusForbidden, ""), nil
		case request.URL.Path == "/api/playback/sessions":
			mirrorPlaybackCalls.Add(1)
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"play_mode":"full","media":{"manifest_url":"https://media.example.test/ep.m3u8"}}}`), nil
		case request.URL.Path == "/api/trial/sessions" || request.URL.Path == "/api/preview/sessions":
			// 站点没有试看接口时也不能影响"需要会员"这个结论。
			return sourceFixtureResponse(request, http.StatusNotFound, ""), nil
		default:
			return nil, fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	})

	_, err := downloader.xiangjiaoFullPlaybackManifest(context.Background(), sourceID, "ep-locked")
	if err == nil || !strings.Contains(err.Error(), "会员") {
		t.Fatalf("expected a clear payment-denial error, got %v", err)
	}
	if playbackCalls.Load() != 1 || mirrorPlaybackCalls.Load() != 0 {
		t.Fatalf("403 must not retry playback on another host; primary=%d mirror=%d", playbackCalls.Load(), mirrorPlaybackCalls.Load())
	}
}

// 0.9.0：正片 403 时退一步拿试看会话，让"需要付费"的剧至少能看前几分钟。
func TestXiangjiaoPlaybackFallsBackToTrialPreview(t *testing.T) {
	resetProviderMirrorCaches()
	downloader := sourceFixtureDownloader(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == providerMirrorConfigURL {
			return sourceFixtureResponse(request, http.StatusOK, `{"version":1,"ttl_seconds":3600,"sources":{}}`), nil
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/guest-sessions":
			return sourceFixtureResponse(request, http.StatusCreated, `{"data":{"access_token":"fixture-token","expires_at":"2099-01-01T00:00:00Z"}}`), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/playback/sessions":
			return sourceFixtureResponse(request, http.StatusForbidden, `{"code":"purchase_required","message":"need purchase"}`), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/trial/sessions":
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"play_mode":"trial","media":{"manifest_url":"https://media.example.test/trial.m3u8"}}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	})

	manifest, err := downloader.xiangjiaoFullPlaybackManifest(context.Background(), "payment-fixture", "ep-locked")
	if err != nil {
		t.Fatalf("expected the trial preview to be used, got error %v", err)
	}
	if manifest != "https://media.example.test/trial.m3u8" {
		t.Fatalf("manifest = %q, want the trial playlist", manifest)
	}
}

// 0.8.10：目录接口匿名 401 时，补一次游客会话再重试。
//
// 用户反馈"香蕉现在列表都加载不出来"：目录这条路径一直匿名请求，
// 站点给列表接口加上游客鉴权后整条链路就只剩 401。
func TestXiangjiaoCatalogRetriesWithGuestTokenOnUnauthorized(t *testing.T) {
	resetProviderMirrorCaches()
	var theaterCalls, sessionCalls atomic.Int32
	downloader := sourceFixtureDownloader(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == providerMirrorConfigURL {
			return sourceFixtureResponse(request, http.StatusOK, `{"version":1,"ttl_seconds":3600,"sources":{}}`), nil
		}
		switch request.URL.Path {
		case "/api/theater":
			if theaterCalls.Add(1) == 1 {
				return sourceFixtureResponse(request, http.StatusUnauthorized, `{"code":"unauthorized","message":"guest session required"}`), nil
			}
			if got := request.Header.Get("Authorization"); got != "Bearer fixture-guest" {
				t.Errorf("Authorization = %q, want the guest token", got)
			}
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"total":1,"next_cursor":"","items":[{"id":"guest-only","title":"游客可见"}]}}`), nil
		case "/api/guest-sessions":
			sessionCalls.Add(1)
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"access_token":"fixture-guest"}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	})

	items, _, err := downloader.fetchXiangjiaoCatalogPage(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("fetchXiangjiaoCatalogPage: %v", err)
	}
	if len(items) != 1 || items[0].Title != "游客可见" {
		t.Fatalf("catalog did not recover after the guest session: %#v", items)
	}
	if sessionCalls.Load() != 1 || theaterCalls.Load() != 2 {
		t.Fatalf("expected one guest session and one retry; sessions=%d theater=%d", sessionCalls.Load(), theaterCalls.Load())
	}
}

// 0.8.10：目录拉取失败后自动重试一次，不再等用户去设置里点「检查链接」。
func TestXiangjiaoCatalogRecoversFromTransientServerError(t *testing.T) {
	resetProviderMirrorCaches()
	var theaterCalls atomic.Int32
	engine := sourceFixtureEngine(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == providerMirrorConfigURL {
			return sourceFixtureResponse(request, http.StatusOK, `{"version":1,"ttl_seconds":3600,"sources":{}}`), nil
		}
		if request.Method == http.MethodGet && request.URL.Path == "/api/theater" {
			if theaterCalls.Add(1) == 1 {
				return sourceFixtureResponse(request, http.StatusInternalServerError, `{"code":"internal","message":"boom"}`), nil
			}
			return sourceFixtureResponse(request, http.StatusOK, `{"data":{"total":1,"next_cursor":"","items":[{"id":"recovered","title":"重试后可见"}]}}`), nil
		}
		return nil, fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
	})

	result, err := engine.nativeCatalog(context.Background(), nativeInput{Source: sourceXiangjiao, Page: 1})
	if err != nil {
		t.Fatalf("nativeCatalog: %v", err)
	}
	if len(result.Items) != 1 || theaterCalls.Load() < 2 {
		t.Fatalf("catalog did not retry after a transient failure: items=%d calls=%d", len(result.Items), theaterCalls.Load())
	}
}

// 0.8.10：用户看到的 "443，connection reset by peer" 要换成人话，
// 同时不能把接口层错误（401/403/购买限制）也包掉 —— 上层靠它决定补游客会话。
func TestXiangjiaoRequestErrorKeepsAPIErrorsAndExplainsNetworkFailures(t *testing.T) {
	resetErr := fmt.Errorf("Get \"https://xiangjiaoai.ai/api/theater\": read tcp 10.0.0.2:5321->104.21.24.81:443: read: connection reset by peer")
	wrapped := xiangjiaoRequestError(resetErr)
	if !strings.Contains(wrapped.Error(), "已自动改用 IPv4 重试仍失败") {
		t.Fatalf("network error was not explained: %v", wrapped)
	}
	if !errors.Is(wrapped, resetErr) {
		t.Fatalf("original error lost, cannot inspect root cause: %v", wrapped)
	}

	apiErr := &xiangjiaoHTTPError{Status: http.StatusUnauthorized, Code: "unauthorized"}
	if got := xiangjiaoRequestError(apiErr); got != error(apiErr) {
		t.Fatalf("api error should pass through untouched, got %v", got)
	}
	if got := xiangjiaoRequestError(nil); got != nil {
		t.Fatalf("nil should stay nil, got %v", got)
	}
}
