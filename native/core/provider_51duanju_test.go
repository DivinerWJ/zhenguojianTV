package core

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// 51短剧的 payload 是 devalue 扁平数组：整数是「槽位下标」，字符串/布尔是字面量。
// 这里用最小样例锁住解引用逻辑，避免以后改解析时把卡片和分集一起弄丢。
func TestDuanju51DecodePayloadResolvesReferences(t *testing.T) {
	payload := `[
		{"list": 1, "total": 2},
		[2, 3],
		{"video_id": 13679, "title": "剧集甲", "cover": "https://pic.example/a.jpg", "play_count_text": "1.2万播放"},
		{"video_id": 13012, "title": "剧集乙", "cover": "https://pic.example/b.jpg"}
	]`
	root := duanju51DecodePayload(payload)
	if root == nil {
		t.Fatal("payload 解码失败")
	}
	cards := duanju51CardDramas(root)
	if len(cards) != 2 {
		t.Fatalf("卡片数量不对: %d", len(cards))
	}
	if cards[0].ID != "51duanju:13679" || cards[0].Title != "剧集甲" || cards[0].Cover != "https://pic.example/a.jpg" {
		t.Fatalf("首张卡片字段不对: %+v", cards[0])
	}
	if cards[0].Remark != "1.2万播放" {
		t.Fatalf("播放量文案丢了: %q", cards[0].Remark)
	}
}

func TestDuanju51DetailEpisodesAndPaywall(t *testing.T) {
	payload := `[
		{"playlet_id": 13679, "episodeAll": 1, "drama_name": "剧集甲", "cover_img": "https://pic.example/a.jpg", "description": "简介", "update_status": "全2集"},
		[2, 3],
		{"index": 1, "episode_title": "第1集", "video_url": "https://hls.example/1.m3u8?auth_key=aaa", "state": "playable", "unlocked": true},
		{"index": 2, "episode_title": "第2集", "video_url": "https://hls.example/2.m3u8?auth_key=bbb", "state": "locked", "access_reason": "coin"}
	]`
	root := duanju51DecodePayload(payload)
	detail := duanju51DetailObject(root, "13679")
	if detail == nil {
		t.Fatal("没有找到详情对象")
	}
	episodes := duanju51Episodes(detail)
	if len(episodes) != 2 {
		t.Fatalf("分集数量不对: %d", len(episodes))
	}
	if episodes[0].HLS != "https://hls.example/1.m3u8?auth_key=aaa" || episodes[0].Locked {
		t.Fatalf("免费集解析不对: %+v", episodes[0])
	}
	if !episodes[1].Locked {
		t.Fatalf("付费集没有被标记: %+v", episodes[1])
	}
}

// 真实站点冒烟测试，默认跳过；需要时用 DUANJU51_LIVE=1 打开。
func TestDuanju51LiveCatalogAndDetail(t *testing.T) {
	if os.Getenv("DUANJU51_LIVE") == "" {
		t.Skip("set DUANJU51_LIVE=1 to hit the live site")
	}
	d := &Downloader{
		cfg:           Config{Retries: 2, PageSize: 30},
		client:        &http.Client{Timeout: 30 * time.Second},
		limiter:       newRequestLimiter(3, 0),
		providerHosts: map[string]string{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	items, more, err := d.fetchDuanju51CatalogPage(ctx, 1, "home", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("首页没有解析出任何剧集")
	}
	t.Logf("首页 %d 部，hasMore=%v，首部=%s", len(items), more, items[0].Title)

	drama, chapters, err := d.fetchDuanju51Detail(ctx, items[0].SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) == 0 {
		t.Fatal("详情没有分集")
	}
	if !strings.Contains(chapters[0].VideoURL, ".m3u8") {
		t.Fatalf("首集不是 m3u8: %q", chapters[0].VideoURL)
	}
	if !strings.Contains(chapters[0].VideoURL, "auth_key=") {
		t.Fatalf("首集丢了 auth_key: %q", chapters[0].VideoURL)
	}
	t.Logf("详情 %s，%d 集，首集地址长度 %d", drama.Title, len(chapters), len(chapters[0].VideoURL))

	media, err := d.resolveProviderMedia(ctx, Task{DramaID: drama.ID, Chapter: chapters[0], Index: 1, Total: len(chapters)})
	if err != nil {
		t.Fatalf("播放解析失败: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(media.Playlist), "#EXTM3U") {
		t.Fatalf("播放列表无效: %q", media.Playlist[:min(40, len(media.Playlist))])
	}
	if !strings.Contains(media.Playlist, "#EXT-X-KEY") {
		t.Log("该集未使用 AES-128 加密，跳过密钥检查")
	}
	t.Logf("播放列表 %d 字节，含 %d 个分片", len(media.Playlist), strings.Count(media.Playlist, ".ts"))
}
