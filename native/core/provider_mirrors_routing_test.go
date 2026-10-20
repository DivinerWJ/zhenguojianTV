package core

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func withProviderMirrorCacheForTest(t *testing.T, sources map[string][]string) {
	t.Helper()
	providerMirrorCache.Lock()
	oldExpires := providerMirrorCache.expires
	oldSources := providerMirrorCache.sources
	providerMirrorCache.sources = sources
	providerMirrorCache.expires = time.Now().Add(time.Hour)
	providerMirrorCache.Unlock()
	t.Cleanup(func() {
		providerMirrorCache.Lock()
		providerMirrorCache.expires = oldExpires
		providerMirrorCache.sources = oldSources
		providerMirrorCache.Unlock()
	})
}

func TestValidProviderMirrorOriginRejectsUnsafeAddresses(t *testing.T) {
	cases := map[string]string{
		"https://mirror.example/":          "https://mirror.example",
		"http://mirror.example":            "",
		"https://u:p@mirror.example":       "",
		"https://mirror.example/path":      "",
		"https://mirror.example/?q=1":      "",
		"https://mirror.example/#fragment": "",
	}
	for input, want := range cases {
		if got := validProviderMirrorOrigin(input); got != want {
			t.Fatalf("validProviderMirrorOrigin(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestProviderBaseCandidatesPrefersRememberedThenRemoteThenBuiltin(t *testing.T) {
	withProviderMirrorCacheForTest(t, map[string][]string{
		sourceHuangdou: {"https://remote-a.example", "https://remote-b.example"},
	})
	d := &Downloader{providerHosts: map[string]string{
		sourceHuangdou: "https://remembered.example",
	}}
	got := d.providerBaseCandidates(
		context.Background(),
		sourceHuangdou,
		[]string{"https://builtin.example", "https://remote-a.example/"},
	)
	want := []string{
		"https://remembered.example",
		"https://remote-a.example",
		"https://remote-b.example",
		"https://builtin.example",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates = %#v, want %#v", got, want)
	}
}

func TestProviderURLCandidatesDynamicPreservesPathAndQuery(t *testing.T) {
	withProviderMirrorCacheForTest(t, map[string][]string{
		sourceHuangguoAI: {"https://mirror.example"},
	})
	d := &Downloader{providerHosts: map[string]string{}}
	got := d.providerURLCandidatesDynamic(
		context.Background(),
		"https://huangguoai.com/drama/123?episode=2",
	)
	if len(got) == 0 || got[0] != "https://mirror.example/drama/123?episode=2" {
		t.Fatalf("unexpected dynamic candidates: %#v", got)
	}
}
