package core

import "testing"

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
