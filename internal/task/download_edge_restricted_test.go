package task

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func edgeRestrictedTestEngine(t *testing.T, port int, ignore403, ignore1034 bool, hooks Hooks) *Engine {
	t.Helper()
	config := DefaultConfig()
	config.Disable = false
	config.TestCount = 1
	config.URL = "http://127.0.0.1:" + strconv.Itoa(port) + "/500m"
	config.Timeout = 2 * time.Second
	config.TCPPort = port
	config.DownloadWarmupDuration = 0
	config.DownloadSampleInterval = time.Second
	config.Ignore403 = ignore403
	config.Ignore1034 = ignore1034
	return NewEngine(config, hooks)
}

func edgeRestrictedServer(t *testing.T, body string) (*httptest.Server, int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	port, err := strconv.Atoi(srv.URL[strings.LastIndex(srv.URL, ":")+1:])
	if err != nil {
		t.Fatal(err)
	}
	return srv, port
}

// 1034 与 403 是两个独立开关，且 1034 优先：屏蔽 403 不会连带淘汰被放行的 1034。
// 屏蔽与不屏蔽的差别只在重试和连续失败冷却计账，两者都不会把 IP 留在结果里。
func TestDownload403And1034Matrix(t *testing.T) {
	_, edgePort := edgeRestrictedServer(t, "error code: 1034\n")
	_, forbiddenPort := edgeRestrictedServer(t, "forbidden\n")
	ip := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}

	cases := []struct {
		name          string
		port          int
		ignore403     bool
		ignore1034    bool
		wantReason    string
		wantIgnored   bool
		wantRetryable bool
	}{
		{"1034 blocked", edgePort, true, true, downloadReasonEdgeRestricted, true, false},
		{"1034 kept counts as failure", edgePort, true, false, downloadReasonEdgeRestricted, false, true},
		{"1034 kept wins over ignore 403", edgePort, false, false, downloadReasonEdgeRestricted, false, true},
		{"plain 403 blocked", forbiddenPort, true, false, "", false, true},
		{"plain 403 kept", forbiddenPort, false, false, downloadReasonForbidden, false, true},
	}
	for _, tc := range cases {
		engine := edgeRestrictedTestEngine(t, tc.port, tc.ignore403, tc.ignore1034, Hooks{})
		result, err := engine.downloadHandlerAttemptOnce(ip, 1)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if result.ignored != tc.wantIgnored || result.retryable != tc.wantRetryable {
			t.Fatalf("%s: result = %#v, want ignored=%v retryable=%v", tc.name, result, tc.wantIgnored, tc.wantRetryable)
		}
		if tc.wantReason != "" && result.reason != tc.wantReason {
			t.Fatalf("%s: reason = %q, want %q", tc.name, result.reason, tc.wantReason)
		}
		if result.validMeasurement {
			t.Fatalf("%s: 403 响应不应产生有效速率", tc.name)
		}
	}
}

// 没测到速率的候选一律丢弃：被放行的 1034 IP 也不能进最终结果。
func TestDownloadDropsEdgeRestrictedIPWithoutSpeed(t *testing.T) {
	_, port := edgeRestrictedServer(t, "error code: 1034\n")
	engine := edgeRestrictedTestEngine(t, port, true, false, Hooks{})

	if got := engine.TestDownloadSpeed(makeProbeSetWithIPs("127.0.0.1")); len(got) != 0 {
		t.Fatalf("结果数量 = %d, want 0（没有速率的 1034 IP 不应进入结果）", len(got))
	}
}

// 界面日志流不依赖 debug 开关：被边缘拒绝的 IP 都要发一条带 note 的拒绝事件，
// 这样用户不打开 cfip-log.txt 也能看到 1034 / 403 标识。
func TestDownloadReportsEdgeRejectionToEventStream(t *testing.T) {
	_, edgePort := edgeRestrictedServer(t, "error code: 1034\n")
	_, forbiddenPort := edgeRestrictedServer(t, "forbidden\n")
	ip := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}

	cases := []struct {
		name        string
		port        int
		ignore403   bool
		ignore1034  bool
		wantNote    string
		wantReason  string
		wantIgnored bool
	}{
		{"1034 blocked", edgePort, true, true, EdgeNote1034, downloadReasonEdgeRestricted, true},
		{"1034 kept", edgePort, true, false, EdgeNote1034, downloadReasonEdgeRestricted, false},
		{"plain 403 kept", forbiddenPort, false, false, EdgeNote403, downloadReasonForbidden, false},
		{"plain 403 without note", forbiddenPort, true, false, "", "", false},
	}
	for _, tc := range cases {
		var got []EdgeRejection
		engine := edgeRestrictedTestEngine(t, tc.port, tc.ignore403, tc.ignore1034, Hooks{EdgeRejected: func(rejection EdgeRejection) {
			got = append(got, rejection)
		}})
		if _, err := engine.downloadHandlerAttemptOnce(ip, 1); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if tc.wantNote == "" {
			if len(got) != 0 {
				t.Fatalf("%s: 普通状态码不匹配不应发拒绝事件，got %#v", tc.name, got)
			}
			continue
		}
		if len(got) != 1 {
			t.Fatalf("%s: 拒绝事件数量 = %d, want 1", tc.name, len(got))
		}
		if got[0].Note != tc.wantNote || got[0].Reason != tc.wantReason || got[0].Ignored != tc.wantIgnored || got[0].Stage != "stage3_get" {
			t.Fatalf("%s: 拒绝事件 = %#v", tc.name, got[0])
		}
		if got[0].IP != "127.0.0.1" {
			t.Fatalf("%s: IP = %q", tc.name, got[0].IP)
		}
	}
}
