package core

import (
	"context"
	"net"
	"strings"
	"time"
)

// forceIPv4Key 让本次请求的 TCP 拨号只解析 / 连接 IPv4（0.8.10 第二批）。
//
// 用户反馈：香蕉站源报 "443，connection reset by peer"，列表进不去，播放也不确定。
// 站点本身是好的（Cloudflare 后面，A 记录 104.21.24.81 / 172.67.218.16 都在），
// 但它在部分网络下的 **IPv6 出口会被直接 RST** —— 这正是 0.8.9 时在
// `xiangjiaoJSON` 里写下"IPv6 出口时不时被重置"那条注释的来源。
//
// Go 默认走 Happy Eyeballs 且优先 IPv6，于是同一条链路 IPv4 明明可用也轮不到，
// 客户端看到的就只有 `dial tcp [2606:4700:...]:443: read: connection reset by peer`。
// 这里给需要兜底的请求打个标记，由 [newForceableDialer] 把 network 改成 tcp4。
type forceIPv4Key struct{}

func withForceIPv4(ctx context.Context) context.Context {
	return context.WithValue(ctx, forceIPv4Key{}, true)
}

func forceIPv4Requested(ctx context.Context) bool {
	requested, _ := ctx.Value(forceIPv4Key{}).(bool)
	return requested
}

// dialNetworkFor 决定本次拨号用哪个 network。抽出来是为了能单测，
// 真正的拨号在 [newForceableDialer] 里。
func dialNetworkFor(ctx context.Context, network string) string {
	if forceIPv4Requested(ctx) && strings.HasPrefix(network, "tcp") {
		// 只改 tcp/tcp6 → tcp4；udp 之类保持原样。
		return "tcp4"
	}
	return network
}

// newForceableDialer 包装默认拨号行为，仅在上下文要求时强制 IPv4。
//
// 没有标记时完全等价于 `http.Transport` 自带的默认拨号（含 Happy Eyeballs），
// 所以对其它站源零影响；这也是为什么必须显式设置 DialContext 而不能留 nil
// —— 留 nil 就拿不到上下文，没法按请求区分。
func newForceableDialer() func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, dialNetworkFor(ctx, network), address)
	}
}
