package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Mutable source addresses live in the repository so an APK does not need to be
// rebuilt just because a provider changes its domain. Use jsDelivr instead of
// raw.githubusercontent.com because the latter is frequently slow or blocked on
// some networks. If this document cannot be fetched, providerBaseCandidates
// simply falls back to the built-in addresses compiled into the APK.
const providerMirrorConfigURL = "https://cdn.jsdelivr.net/gh/ai0by/zhenguojianTV@main/config/source_mirrors.json"

type providerMirrorDocument struct {
	Version    int                 `json:"version"`
	TTLSeconds int                 `json:"ttlSeconds"`
	Sources    map[string][]string `json:"sources"`
}

var providerMirrorCache struct {
	sync.Mutex
	expires time.Time
	sources map[string][]string
}

func validProviderMirrorOrigin(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return ""
	}
	if len(parsed.Host) > 253 {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func parseProviderMirrorDocument(body []byte) (map[string][]string, time.Duration) {
	var doc providerMirrorDocument
	if len(body) == 0 || len(body) > 64<<10 || json.Unmarshal(body, &doc) != nil || doc.Version != 1 {
		return nil, 0
	}
	ttl := time.Duration(doc.TTLSeconds) * time.Second
	if ttl < 10*time.Minute || ttl > 24*time.Hour {
		ttl = 6 * time.Hour
	}
	sources := make(map[string][]string)
	for source, values := range doc.Sources {
		if len(source) == 0 || len(source) > 64 {
			continue
		}
		seen := map[string]bool{}
		for _, raw := range values {
			origin := validProviderMirrorOrigin(raw)
			if origin == "" || seen[origin] {
				continue
			}
			seen[origin] = true
			sources[source] = append(sources[source], origin)
			if len(sources[source]) >= 16 {
				break
			}
		}
	}
	return sources, ttl
}

func (d *Downloader) remoteProviderMirrors(ctx context.Context) map[string][]string {
	now := time.Now()
	providerMirrorCache.Lock()
	if now.Before(providerMirrorCache.expires) {
		cached := providerMirrorCache.sources
		providerMirrorCache.Unlock()
		return cached
	}
	providerMirrorCache.Unlock()

	fetchCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, providerMirrorConfigURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := d.doCatalogRequestWithTimeout(req, 2500*time.Millisecond)
	if err != nil {
		providerMirrorCache.Lock()
		providerMirrorCache.expires = now.Add(10 * time.Minute)
		providerMirrorCache.Unlock()
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		providerMirrorCache.Lock()
		providerMirrorCache.expires = now.Add(10 * time.Minute)
		providerMirrorCache.Unlock()
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 {
		return nil
	}
	sources, ttl := parseProviderMirrorDocument(body)
	if len(sources) == 0 {
		providerMirrorCache.Lock()
		providerMirrorCache.expires = now.Add(10 * time.Minute)
		providerMirrorCache.Unlock()
		return nil
	}
	providerMirrorCache.Lock()
	providerMirrorCache.sources = sources
	providerMirrorCache.expires = now.Add(ttl)
	providerMirrorCache.Unlock()
	return sources
}

func (d *Downloader) providerBaseCandidates(ctx context.Context, source string, builtins []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(builtins)+8)
	add := func(raw string) {
		origin := validProviderMirrorOrigin(raw)
		if origin == "" || seen[origin] {
			return
		}
		seen[origin] = true
		result = append(result, origin)
	}
	d.providerMu.Lock()
	add(d.providerHosts[source])
	d.providerMu.Unlock()
	if remote := d.remoteProviderMirrors(ctx); remote != nil {
		for _, origin := range remote[source] {
			add(origin)
		}
	}
	for _, origin := range builtins {
		add(origin)
	}
	return result
}

func (d *Downloader) rememberProviderHost(source, origin string) {
	origin = validProviderMirrorOrigin(origin)
	if source == "" || origin == "" {
		return
	}
	d.providerMu.Lock()
	d.providerHosts[source] = origin
	d.providerMu.Unlock()
}

func (d *Downloader) providerURLCandidatesDynamic(ctx context.Context, rawURL string) []string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return []string{rawURL}
	}
	source := providerSourceForURL(rawURL)
	fixed := providerURLCandidates(rawURL)
	if source == "" {
		return fixed
	}
	origins := make([]string, 0, len(fixed)+4)
	for _, candidate := range fixed {
		if value, err := url.Parse(candidate); err == nil && value.Host != "" {
			origins = append(origins, value.Scheme+"://"+value.Host)
		}
	}
	origins = d.providerBaseCandidates(ctx, source, origins)
	seen := map[string]bool{}
	result := make([]string, 0, len(origins)+len(fixed))
	add := func(candidate string) {
		if candidate == "" || seen[candidate] {
			return
		}
		seen[candidate] = true
		result = append(result, candidate)
	}
	for _, origin := range origins {
		add(rehostProviderURL(parsed, origin))
	}
	for _, candidate := range fixed {
		add(candidate)
	}
	if len(result) == 0 {
		return []string{rawURL}
	}
	return result
}
