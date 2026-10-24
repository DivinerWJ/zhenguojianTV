package core

import (
	"strings"
	"testing"
)

const huangguoAIDetailFixture = `<!doctype html>
<html>
<head>
<title>测试剧 第一季 - 短剧视频在线观看 | 黄果短剧</title>
<meta property="og:description" content="测试剧 第一季全4集，8月19日更新。截断版简介…免费在线观看。">
</head>
<body>
<h1>测试剧 第一季</h1>
<script id="videoInitialData">{"id":"12","ep":1,"title":"测试剧 第一季","description":"这是完整的剧情简介，不是截断版。","tags":["悬疑","古风"],"tagLinks":[{"name":"悬疑","url":"/tag/xuanyi/"},{"name":"古风","url":"/tag/gufeng/"}],"videoSrc":"https://cdn.example/ep1.m3u8","epPlaySrcs":{"1":"https://cdn.example/ep1.m3u8?auth_key=1","2":"https://cdn.example/ep2.m3u8?auth_key=2"}}</script>
<div class="hg-web-play"><div class="hg-web-play__ep-grid" data-web-play-ep-grid="">
<a class="hg-web-play__ep is-active" href="/video/12/" data-ep-id="1">01</a><a class="hg-web-play__ep" href="/video/12/ep-2/" data-ep-id="2">02</a><a class="hg-web-play__ep" href="/video/12/ep-3/" data-ep-id="3">03</a><a class="hg-web-play__ep" href="/video/12/ep-4/" data-ep-id="4">04</a>
</div></div>
<div class="hg-web-recommend"><div class="hg-drama-card" data-track-id="99"><span class="hg-tag">别剧标签</span></div></div>
</body></html>`

func TestBuildAIEpisodesPrefersGridOverShortEpPlaySrcs(t *testing.T) {
	episodes := buildAIEpisodes(huangguoAIDetailFixture, "https://huangguoai.com/detail/12/", "12")
	if len(episodes) != 4 {
		t.Fatalf("episode count = %d, want 4 (epPlaySrcs only has 2)", len(episodes))
	}
	for i, episode := range episodes {
		if episode.Index != i+1 {
			t.Fatalf("episodes[%d].Index = %d", i, episode.Index)
		}
	}
	if episodes[0].Title != "01" || episodes[3].Title != "04" {
		t.Fatalf("episode titles should come from the grid text, got %q %q", episodes[0].Title, episodes[3].Title)
	}
	if episodes[0].HLS == "" || episodes[1].HLS == "" {
		t.Fatal("covered episodes lost their play URLs", episodes[0].HLS, episodes[1].HLS)
	}
	if episodes[2].HLS != "" || episodes[3].HLS != "" {
		t.Fatal("uncovered episodes should resolve at playback time")
	}
	if episodes[3].URL != "https://huangguoai.com/video/12/ep-4/" {
		t.Fatal("episode page URL mismatch", episodes[3].URL)
	}
	if episodes[0].URL != "https://huangguoai.com/video/12/" {
		t.Fatal("active episode page URL mismatch", episodes[0].URL)
	}
}

func TestBuildAIEpisodesStillWorksWithoutGrid(t *testing.T) {
	body := `<html><head><title>旧剧</title></head><body>
	<script id="videoInitialData">{"id":"7","ep":1,"videoSrc":"https://cdn.example/ep1.m3u8","epPlaySrcs":{"1":"https://cdn.example/ep1.m3u8","2":"https://cdn.example/ep2.m3u8"}}</script>
	</body></html>`
	episodes := buildAIEpisodes(body, "https://huangguoai.com/detail/7/", "7")
	if len(episodes) != 2 {
		t.Fatalf("fallback episode count = %d, want 2", len(episodes))
	}
	if episodes[0].Index != 1 || episodes[1].Index != 2 {
		t.Fatal("fallback episodes misnumbered", episodes)
	}
}

func TestHuangguoDetailMetadataUsesInitialData(t *testing.T) {
	drama := huangguoDetailMetadata(huangguoAIDetailFixture, "https://huangguoai.com/detail/12/", sourceHuangguoAI, "12")
	if drama.Title != "测试剧 第一季" {
		t.Fatal("title should come from videoInitialData", drama.Title)
	}
	if !strings.Contains(drama.Desc, "完整的剧情简介") || strings.Contains(drama.Desc, "免费在线观看") {
		t.Fatal("description should use the full JSON text", drama.Desc)
	}
	if len(drama.Tags) != 2 || drama.Tags[0] != "悬疑" || drama.Tags[1] != "古风" {
		t.Fatal("tags should come from videoInitialData, not page hg-tag blocks", drama.Tags)
	}
}
