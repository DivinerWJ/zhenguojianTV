package core

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func TestEZDMWCatalogParsesAnimeCards(t *testing.T) {
	body := "<section id=\"some_drama\"><div class=\"item\"><a href=\"/show/123.html\"><img data-src=\"/cover/123.jpg\" /></a><p>番剧甲</p><span>更新到12</span></div></section>"
	items := parseEZDMWCatalogItems(body, ezdmwBaseURL+"/Index/search.html?searchText=x")
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].ID != "ezdmw:123" || items[0].Title != "番剧甲" {
		t.Fatalf("unexpected item: %+v", items[0])
	}
	if items[0].Cover != "https://www.ezdmw.org/cover/123.jpg" {
		t.Fatalf("unexpected cover URL: %q", items[0].Cover)
	}
	if nativeText(items[0].Episodes) != "12" {
		t.Fatalf("unexpected episode count: %v", items[0].Episodes)
	}
}

func TestParseEZDMWEpisodeCountHandlesMissingCount(t *testing.T) {
	if got := parseEZDMWEpisodeCount("暂无集数信息"); got != 0 {
		t.Fatalf("missing episode count = %d, want 0", got)
	}
	if got := parseEZDMWEpisodeCount("更新到12"); got != 12 {
		t.Fatalf("episode count = %d, want 12", got)
	}
}

func TestEZDMWNearbyTitleIgnoresHTMLComments(t *testing.T) {
	raw := "<div><a href=\"/Index/bangumi/123.html\"><img src=\"/cover.jpg\" /></a><p><span><!-- 1.7万 --> </span>&nbsp;番剧甲</p></div>"
	start := strings.Index(raw, "<a")
	end := strings.Index(raw, "</a>") + len("</a>")
	if got := ezdmwNearbyTitle(raw, start, end); got != "番剧甲" {
		t.Fatalf("nearby title = %q, want 番剧甲", got)
	}
}

func TestEZDMWDetailParsesAndDeduplicatesEpisodes(t *testing.T) {
	body := "<section class=\"anthology\"><div class=\"line_button_ban\"></div><a class=\"circuit_switch_ban\" href=\"/Index/bangumi/101.html\">第1集</a><a class=\"circuit_switch_ban\" href=\"/Index/bangumi/201.html\">第1集</a><a class=\"circuit_switch_ban\" href=\"/Index/bangumi/102.html\">第2集</a><a class=\"circuit_switch_ban\" href=\"/v/103.html\">第3集</a></section>"
	chapters := parseEZDMWEpisodes(body, ezdmwBaseURL+"/show/123.html", "123")
	if len(chapters) != 3 {
		t.Fatalf("got %d chapters, want 3: %+v", len(chapters), chapters)
	}
	if chapters[0].ID != "ezdmw:123:101" || chapters[0].PageURL != ezdmwBaseURL+"/Index/video/101.html" {
		t.Fatalf("unexpected first chapter: %+v", chapters[0])
	}
	if chapters[2].PageURL != ezdmwBaseURL+"/Index/video/103.html" {
		t.Fatalf("unexpected canonical episode URL: %q", chapters[2].PageURL)
	}
	if chapters[1].Title != "第2集" || chapters[1].VideoURL != "ezdmw-play://102" {
		t.Fatalf("unexpected second chapter: %+v", chapters[1])
	}
}

func TestEZDMWMediaFromPlayerFrameURL(t *testing.T) {
	direct := "https://player.example.test/danmuku/?src=" + url.QueryEscape("https://cdn.example.test/episode.m3u8?token=abc")
	if got := ezdmwMediaFromPlayerFrameURL(direct); got != "https://cdn.example.test/episode.m3u8?token=abc" {
		t.Fatalf("direct player frame media = %q", got)
	}
	encodedMedia := base64.RawURLEncoding.EncodeToString([]byte("https://cdn.example.test/episode.mp4"))
	encoded := "https://player.example.test/danmuku/?src=" + url.QueryEscape(encodedMedia)
	if got := ezdmwMediaFromPlayerFrameURL(encoded); got != "https://cdn.example.test/episode.mp4" {
		t.Fatalf("encoded player frame media = %q", got)
	}
}

func TestEZDMWListResURLIsRecognizedAsPlaylist(t *testing.T) {
	if !ezdmwPlaylistURL("https://player.ezdmw.com/index/listres/resource_name/test/key/test/sign/abc.webp?lineOne=true") {
		t.Fatal("E站 listres URL with webp suffix should be treated as HLS")
	}
	if ezdmwPlaylistURL("https://cdn.example.test/cover.webp") {
		t.Fatal("ordinary webp image should not be treated as HLS")
	}
}

func TestEZDMWPlaybackParserHandlesDirectMediaAndIframe(t *testing.T) {
	pageURL := ezdmwBaseURL + "/Index/bangumi/101.html"
	raw := "<video><source src=\"https://cdn.example.test/episode.m3u8?token=one&amp;part=two\"></video>"
	if got := ezdmwDirectMedia(raw, pageURL); got != "https://cdn.example.test/episode.m3u8?token=one&part=two" {
		t.Fatalf("direct media URL = %q", got)
	}
	if got := parseEZDMWPlayerFrame("<iframe src=\"//player.example.test/embed/101\"></iframe>", pageURL); got != "https://player.example.test/embed/101" {
		t.Fatalf("iframe URL = %q", got)
	}
}

func TestEZDMWRejectsUnsafePlayerPageAddresses(t *testing.T) {
	for _, address := range []string{"http://127.0.0.1/player", "http://192.168.1.2/player", "https://localhost/player"} {
		if ezdmwSafePageURL(address) {
			t.Fatalf("unsafe address accepted: %s", address)
		}
	}
	if !ezdmwSafePageURL("https://player.example.test/embed/101") {
		t.Fatal("public HTTPS player address rejected")
	}
	if !strings.Contains(ezdmwCleanTitle("番剧甲第1集在线观看&下载-E站弹幕网"), "番剧甲") {
		t.Fatal("episode title cleanup lost the anime title")
	}
}

// E站把播放器 iframe 的 src 原样写进 HTML，`title=` 里带中文和空格。
// 空格进请求行会让服务端回 400，标题带空格的番剧因此全部播不了（0.8.9 修）。
func TestEZDMWRequestURLEscapesSpacesAndKeepsSignatures(t *testing.T) {
	raw := "https://player.ezdmw.com/danmuku/?nk=clksdysjsh4/clksdysjsh4_19&title=幼女战记 第二季&sign=d56ad252bb0af6037bc4c3da7296c096&quarterly=2026年4月【全集完】"
	got := ezdmwRequestURL(raw)
	if strings.ContainsAny(got, " \t\r\n") {
		t.Fatalf("空格未转义：%q", got)
	}
	if !strings.Contains(got, "title=幼女战记%20第二季") {
		t.Fatalf("title 参数转义结果不对：%q", got)
	}
	// 签名与中文必须原样保留，否则播放器鉴权失败
	if !strings.Contains(got, "sign=d56ad252bb0af6037bc4c3da7296c096") || !strings.Contains(got, "【全集完】") {
		t.Fatalf("签名/中文被破坏：%q", got)
	}

	// 已有的 %XX 不能再编码一次（会把 % 变成 %25）
	already := "https://example.test/a?x=%E4%B8%AD&t=a b"
	if got := ezdmwRequestURL(already); !strings.Contains(got, "x=%E4%B8%AD") || !strings.Contains(got, "t=a%20b") {
		t.Fatalf("已有转义被破坏：%q", got)
	}

	// 没有空格时原样返回，不做任何改写
	plain := "https://example.test/a?x=1&y=%2F"
	if got := ezdmwRequestURL(plain); got != plain {
		t.Fatalf("无空格地址被改动：%q -> %q", plain, got)
	}
}

func TestEZDMWPlayerFrameEscapesSpacesInTitle(t *testing.T) {
	pageURL := ezdmwBaseURL + "/Index/video/102593.html"
	raw := `<iframe class="iframe" src="https://player.ezdmw.com/danmuku/?nk=a/b&name=a/b&title=关于我转生变成史莱姆这档事 第四季&sign=504e0a6c" frameborder="0"></iframe>`
	got := parseEZDMWPlayerFrame(raw, pageURL)
	if got == "" {
		t.Fatal("未解析到播放器地址")
	}
	if strings.Contains(got, " ") {
		t.Fatalf("播放器地址仍带空格（会 400）：%q", got)
	}
	if !strings.Contains(got, "title=关于我转生变成史莱姆这档事%20第四季") || !strings.Contains(got, "sign=504e0a6c") {
		t.Fatalf("播放器地址转义结果不对：%q", got)
	}
}

// 首页卡片把"更新时间 / 集数 / 总追番数"放在 <a> 里的 .info-card，
// 直接取整块文本会把它们当成标题，HTML 注释还会残留 `-->`（0.8.9 修）。
func TestEZDMWCatalogCardTitleIgnoresInfoBlock(t *testing.T) {
	body := `<div class="rec-ct"><div class="rec-item">` +
		`<a href="/Index/bangumi/98512.html" target="_blank">` +
		`<img src="https://www.ezdmw.org/Public/atlas_oss/Public/images/some_img/2500.jpg" title="关于我转生变成史莱姆这档事 第四季" style="width:100%;height:100%;" />` +
		`<div class="info-card">` +
		`<p>关于我转生变成史莱姆这档事 第四季</p>` +
		`<p class="info"><span>2星期前</span><span>更新到：&nbsp;24</span><!--<span>总追番数：118</span>--></p>` +
		`</div></a></div></div>`
	items := parseEZDMWCatalogItems(body, ezdmwBaseURL+"/")
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	item := items[0]
	if item.Title != "关于我转生变成史莱姆这档事 第四季" {
		t.Fatalf("标题仍带脏尾巴：%q", item.Title)
	}
	if strings.Contains(item.Title, "更新到") || strings.Contains(item.Title, "总追番数") || strings.Contains(item.Title, "-->") {
		t.Fatalf("标题混入了 info 区内容：%q", item.Title)
	}
	if nativeText(item.Episodes) != "24" {
		t.Fatalf("集数 = %v, want 24", item.Episodes)
	}
	if item.Cover != "https://www.ezdmw.org/Public/atlas_oss/Public/images/some_img/2500.jpg" {
		t.Fatalf("封面 = %q", item.Cover)
	}
}

func TestParseEZDMWEpisodeCountHandlesFullWidthColon(t *testing.T) {
	cases := map[string]int{
		"更新到12":        12,
		"更新到： 24":      24,
		"更新到:\u00a024": 24, // cleanText 把 &nbsp; 变成 U+00A0
		"2星期前 更新到： 16 总追番数：66 -->": 16,
		"全12集":   12,
		"暂无集数信息": 0,
	}
	for raw, want := range cases {
		if got := parseEZDMWEpisodeCount(raw); got != want {
			t.Fatalf("parseEZDMWEpisodeCount(%q) = %d, want %d", raw, got, want)
		}
	}
}

// 详情页没有 og:image，第一个 <img> 是站点 logo；真实海报在 some_img 下（0.8.9 修）。
func TestEZDMWCoverPrefersPosterOverSiteIcon(t *testing.T) {
	pageURL := ezdmwBaseURL + "/Index/bangumi/98512.html"
	raw := `<img src="/Public/images/icon.png" style="position:relative;left:28px;top:4px;" />` +
		`<img src="/src/images/header/top-logo.png" />` +
		`<img src="https://www.ezdmw.org/Public/atlas_oss/Public/images/some_img/2500.jpg" />`
	if got := ezdmwCoverURL(raw, pageURL); got != "https://www.ezdmw.org/Public/atlas_oss/Public/images/some_img/2500.jpg" {
		t.Fatalf("封面取到了站点图标：%q", got)
	}
	// 只剩 UI 图标时不能把 logo 当海报，宁可留空
	iconsOnly := `<img src="/Public/images/icon.png" />`
	if got := ezdmwCoverURL(iconsOnly, pageURL); got != "" {
		t.Fatalf("只有 UI 图标时不应返回封面：%q", got)
	}
}

// 首页还有两种卡片：新番区块的标题在 <p class="a-hover"> 里，类型标签在
// <div class="mask"> 中；排行榜的标题在 <p class="text-top a-hover"> 里。
// 曾经"凡带 class 的 <p> 都跳过"，结果退化成整块文本，把
// "剧场、恋爱、校园、少女、治愈" 和时间一起当成标题（0.8.9 修）。
func TestEZDMWCatalogTitleHandlesOtherCardLayouts(t *testing.T) {
	animeBlock := `<div class="anime-ct"><div class="item a-hover">` +
		`<a href="/Index/bangumi/101390.html" target="_blank">` +
		`<div class="pic"><img src="https://www.ezdmw.org/Public/atlas_oss/Public/images/some_img/2590.jpg" style="width:100%;height:100%;" />` +
		`<div class="mask"><span class="time" style="height:50px;line-height:25px;">剧场、恋爱、校园、少女、治愈</span></div></div>` +
		`<p class="a-hover">我心中的野兽 剧场版</p>` +
		`<p class="info"><span class="audience">2个月前</span></p>` +
		`</a></div></div>`
	items := parseEZDMWCatalogItems(animeBlock, ezdmwBaseURL+"/")
	if len(items) != 1 {
		t.Fatalf("新番卡片解析出 %d 条", len(items))
	}
	if items[0].Title != "我心中的野兽 剧场版" {
		t.Fatalf("新番标题 = %q，want 我心中的野兽 剧场版", items[0].Title)
	}

	rankBlock := `<div class="a-item"><span class="number num-top">7</span>` +
		`<a href="/Index/bangumi/76634.html" target="_blank">` +
		`<div class="title"><p class="text-top a-hover" style="color:#000;">葬送的芙莉莲</p></div>` +
		`</a></div>`
	ranked := parseEZDMWCatalogItems(rankBlock, ezdmwBaseURL+"/")
	if len(ranked) != 1 {
		t.Fatalf("排行榜卡片解析出 %d 条", len(ranked))
	}
	if ranked[0].Title != "葬送的芙莉莲" {
		t.Fatalf("排行榜标题 = %q，want 葬送的芙莉莲", ranked[0].Title)
	}
}
