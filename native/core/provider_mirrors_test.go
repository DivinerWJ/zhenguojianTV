package core

import (
	"context"
	"strings"
	"testing"
)

func TestProviderMirrorConfigUsesJsDelivr(t *testing.T) {
	if !strings.HasPrefix(providerMirrorConfigURL, "https://cdn.jsdelivr.net/gh/ai0by/zhenguojianTV@main/") {
		t.Fatalf("remote provider config must use jsDelivr GitHub CDN, got %q", providerMirrorConfigURL)
	}
	if !strings.HasSuffix(providerMirrorConfigURL, "/config/source_mirrors.json") {
		t.Fatalf("unexpected remote provider config path: %q", providerMirrorConfigURL)
	}
}

func TestHuangguoVideoBuiltinUsesCurrentTheaterEndpoint(t *testing.T) {
	if huangguoVideoBaseURL != "https://huangguo.video" {
		t.Fatalf("unexpected Huangguo theater fallback: %q", huangguoVideoBaseURL)
	}
}

func TestParseProviderMirrorDocument(t *testing.T) {
	body := []byte(`{"version":1,"ttlSeconds":3600,"sources":{"huangdou":["https://a.example","https://a.example/","http://bad.example","https://b.example/path"]}}`)
	sources, ttl := parseProviderMirrorDocument(body)
	if ttl <= 0 {
		t.Fatal("expected ttl")
	}
	got := sources["huangdou"]
	if len(got) != 1 || got[0] != "https://a.example" {
		t.Fatalf("unexpected mirrors: %#v", got)
	}
}

func TestInvalidProviderMirrorDocumentFallsBack(t *testing.T) {
	sources, _ := parseProviderMirrorDocument([]byte(`{"version":9,"sources":{"huangdou":["https://a.example"]}}`))
	if sources != nil {
		t.Fatalf("unexpected sources: %#v", sources)
	}
}

func TestApplyLocalProviderMirrors(t *testing.T) {
	resetProviderMirrorCaches()
	t.Cleanup(resetProviderMirrorCaches)

	body := []byte(`{"version":1,"ttlSeconds":3600,"sources":{"huangguo-video":["https://huangguo.video","https://huangguoai.fit"]}}`)
	count, err := applyLocalProviderMirrors(body)
	if err != nil {
		t.Fatalf("apply local mirrors: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 source, got %d", count)
	}
	d := &Downloader{}
	if got := cachedProviderMirrorOrigin("huangguo-video"); got != "https://huangguo.video" {
		t.Fatalf("unexpected cached origin: %q", got)
	}
	// 本地注入后，远程拉取不应覆盖本地内容（不发起请求且返回本地列表）。
	if got := d.remoteProviderMirrors(context.Background()); len(got["huangguo-video"]) != 2 {
		t.Fatalf("local override lost after remote call: %#v", got)
	}
	// 域名登记生效：providerSourceForURL 能认出新镜像归属。
	if source := providerSourceForURL("https://huangguoai.fit/video/x/"); source != sourceHuangguoVideo {
		t.Fatalf("unexpected source for mirror host: %q", source)
	}

	// 无效内容应报错且不清空已有缓存。
	if _, err := applyLocalProviderMirrors([]byte(`{"version":9}`)); err == nil {
		t.Fatal("expected error for invalid document")
	}
	if got := cachedProviderMirrorOrigin("huangguo-video"); got != "https://huangguo.video" {
		t.Fatalf("cache clobbered by invalid apply: %q", got)
	}
}
