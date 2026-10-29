package task

import (
	"net"
	"strconv"
	"testing"
)

func httpingEdgeRestrictedPing(t *testing.T, port int, ignore403, ignore1034 bool) *Ping {
	t.Helper()
	config := DefaultConfig()
	config.Httping = true
	config.URL = "http://127.0.0.1:" + strconv.Itoa(port) + "/probe"
	config.TCPPort = port
	config.PingTimes = 2
	config.SkipFirstLatencySample = false
	config.Ignore403 = ignore403
	config.Ignore1034 = ignore1034
	return &Ping{engine: NewEngine(config, Hooks{})}
}

// HEAD 拿不到正文，所以阶段一遇到 403 时要补一次小范围 GET 才能分清 1034 和普通 403：
// 屏蔽的按 reason 淘汰，放行的继续参与后续阶段（但没测到速率仍会被丢弃）。
func TestHTTPing403SniffDecidesRejection(t *testing.T) {
	_, edgePort := edgeRestrictedServer(t, "error code: 1034\n")
	_, forbiddenPort := edgeRestrictedServer(t, "forbidden\n")
	ip := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}

	cases := []struct {
		name         string
		port         int
		ignore403    bool
		ignore1034   bool
		wantReceived bool
	}{
		{"1034 kept", edgePort, true, false, true},
		{"1034 blocked", edgePort, true, true, false},
		{"plain 403 kept", forbiddenPort, false, false, true},
		{"plain 403 blocked", forbiddenPort, true, false, false},
	}
	for _, tc := range cases {
		ping := httpingEdgeRestrictedPing(t, tc.port, tc.ignore403, tc.ignore1034)
		received, _, _ := ping.httping(ip)
		if (received > 0) != tc.wantReceived {
			t.Fatalf("%s: received = %d, want success=%v", tc.name, received, tc.wantReceived)
		}
	}
}
