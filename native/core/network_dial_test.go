package core

import (
	"context"
	"testing"
)

// 0.8.10：香蕉站源的 IPv6 出口在部分网络下被 RST，必须能按请求退回 IPv4。
func TestForceIPv4DialNetwork(t *testing.T) {
	plain := context.Background()
	if got := dialNetworkFor(plain, "tcp"); got != "tcp" {
		t.Fatalf("dialNetworkFor(tcp) = %q, want unchanged", got)
	}
	if got := dialNetworkFor(plain, "tcp6"); got != "tcp6" {
		t.Fatalf("dialNetworkFor(tcp6) = %q, want unchanged", got)
	}
	forced := withForceIPv4(plain)
	for _, network := range []string{"tcp", "tcp6"} {
		if got := dialNetworkFor(forced, network); got != "tcp4" {
			t.Fatalf("dialNetworkFor(%s) with forceIPv4 = %q, want tcp4", network, got)
		}
	}
	// 非 TCP 一律不动，避免误伤 UDP 类流量。
	if got := dialNetworkFor(forced, "udp"); got != "udp" {
		t.Fatalf("dialNetworkFor(udp) with forceIPv4 = %q, want unchanged", got)
	}
}
