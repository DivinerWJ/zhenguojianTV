package core

import (
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
