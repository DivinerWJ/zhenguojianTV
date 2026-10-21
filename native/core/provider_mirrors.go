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

// providerSourceOrigins 记录"镜像域名 → 站源"的映射。
// 硬编码白名单只能认识内置域名，一旦 config/source_mirrors.json 里新增了
// 备用域名（红果、黄剧这类经常换域名的站源尤其需要），providerSourceForURL
// 就会认不出来，导致镜像机制形同虚设。这里把远程配置里的域名动态登记进来。
var providerSourceOrigins struct {
	sync.RWMutex
	hosts map[string]string
}

func rememberProviderOrigins(sources map[string][]string) {
	providerSourceOrigins.Lock()
	defer providerSourceOrigins.Unlock()
	if providerSourceOrigins.hosts == nil {
		providerSourceOrigins.hosts = map[string]string{}
	}
	for source, origins := range sources {
		canonical := canonicalProviderSource(source)
		if canonical == "" {
			canonical = source
		}
		for _, origin := range origins {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Hostname() == "" {
				continue
			}
			providerSourceOrigins.hosts[strings.ToLower(parsed.Hostname())] = canonical
		}
	}
}

func providerSourceForHost(host string) string {
	providerSourceOrigins.RLock()
	defer providerSourceOrigins.RUnlock()
	return providerSourceOrigins.hosts[host]
}

// resetProviderMirrorCaches 清空进程级的远程镜像缓存与"域名 → 站源"映射。
// 这两份缓存都是包级变量、TTL 一小时，会跨用例共享：前一个用例写入的镜像
// 会改变后一个用例里 providerBaseURL / providerSourceForURL 的结果，导致
// 单跑通过、全量失败。测试用它在用例开始前恢复干净状态。
func resetProviderMirrorCaches() {
	providerMirrorCache.Lock()
	providerMirrorCache.expires = time.Time{}
	providerMirrorCache.sources = nil
	providerMirrorCache.Unlock()
	providerSourceOrigins.Lock()
	providerSourceOrigins.hosts = nil
	providerSourceOrigins.Unlock()
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
	rememberProviderOrigins(sources)
	providerMirrorCache.Lock()
	providerMirrorCache.sources = sources
	providerMirrorCache.expires = now.Add(ttl)
	providerMirrorCache.Unlock()
	return sources
}

// cachedProviderMirrorOrigin 只读取已缓存的远程镜像，不触发网络请求。
// 供 providerBaseURL 这类高频同步调用使用。
func cachedProviderMirrorOrigin(source string) string {
	if source == "" {
		return ""
	}
	providerMirrorCache.Lock()
	defer providerMirrorCache.Unlock()
	if !time.Now().Before(providerMirrorCache.expires) {
		return ""
	}
	if origins := providerMirrorCache.sources[source]; len(origins) > 0 {
		return origins[0]
	}
	return ""
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
