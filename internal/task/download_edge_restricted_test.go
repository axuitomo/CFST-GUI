package task

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func edgeRestrictedTestEngine(t *testing.T, port int, hooks Hooks) *Engine {
	t.Helper()
	config := DefaultConfig()
	config.Disable = false
	config.URL = "http://127.0.0.1:" + strconv.Itoa(port) + "/500m"
	config.Timeout = 2 * time.Second
	config.TCPPort = port
	config.DownloadWarmupDuration = 0
	config.DownloadSampleInterval = time.Second
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

// 403 与 1034 一律直接淘汰：跳过该 IP，不重试，也不计入连续失败冷却。
// 1034 只是 403 里的一种，正文带标记时 reason 记为 edge_ip_restricted，否则记 forbidden。
func TestDownload403And1034AlwaysSkipped(t *testing.T) {
	_, edgePort := edgeRestrictedServer(t, "error code: 1034\n")
	_, forbiddenPort := edgeRestrictedServer(t, "forbidden\n")
	ip := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}

	cases := []struct {
		name       string
		port       int
		wantReason string
	}{
		{"1034", edgePort, downloadReasonEdgeRestricted},
		{"plain 403", forbiddenPort, downloadReasonForbidden},
	}
	for _, tc := range cases {
		engine := edgeRestrictedTestEngine(t, tc.port, Hooks{})
		result, err := engine.downloadHandlerAttemptOnce(ip, 1)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !result.ignored || result.retryable {
			t.Fatalf("%s: result = %#v, want ignored=true retryable=false", tc.name, result)
		}
		if result.reason != tc.wantReason {
			t.Fatalf("%s: reason = %q, want %q", tc.name, result.reason, tc.wantReason)
		}
		if result.validMeasurement {
			t.Fatalf("%s: 403 响应不应产生有效速率", tc.name)
		}
	}
}

// 403 / 1034 都不会把 IP 留在结果里：没测到速率的候选一律丢弃。
func TestDownloadDropsEdgeRestrictedIPWithoutSpeed(t *testing.T) {
	_, port := edgeRestrictedServer(t, "error code: 1034\n")
	engine := edgeRestrictedTestEngine(t, port, Hooks{})

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
		name       string
		port       int
		wantNote   string
		wantReason string
	}{
		{"1034", edgePort, EdgeNote1034, downloadReasonEdgeRestricted},
		{"plain 403", forbiddenPort, EdgeNote403, downloadReasonForbidden},
	}
	for _, tc := range cases {
		var got []EdgeRejection
		engine := edgeRestrictedTestEngine(t, tc.port, Hooks{EdgeRejected: func(rejection EdgeRejection) {
			got = append(got, rejection)
		}})
		if _, err := engine.downloadHandlerAttemptOnce(ip, 1); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(got) != 1 {
			t.Fatalf("%s: 拒绝事件数量 = %d, want 1", tc.name, len(got))
		}
		if got[0].Note != tc.wantNote || got[0].Reason != tc.wantReason || !got[0].Ignored || got[0].Stage != "stage3_get" {
			t.Fatalf("%s: 拒绝事件 = %#v", tc.name, got[0])
		}
		if got[0].IP != "127.0.0.1" {
			t.Fatalf("%s: IP = %q", tc.name, got[0].IP)
		}
	}
}

// 阶段 2 追踪也认出 1034：403 + 正文 "error code: 1034" 直接淘汰，
// 不重试也不进文件测速，避免白下一轮下载。
func TestTraceDetects1034(t *testing.T) {
	var requests atomic.Int32
	var mu sync.Mutex
	var events []EdgeRejection
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("error code: 1034\n"))
	}))
	port, err := strconv.Atoi(srv.URL[strings.LastIndex(srv.URL, ":")+1:])
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	config.Disable = false
	config.TraceURL = "http://127.0.0.1:" + strconv.Itoa(port) + "/cdn-cgi/trace"
	config.TCPPort = port
	config.HeadTimeout = 2 * time.Second
	config.TraceColoMode = TraceColoModeStandard
	config.RetryMaxAttempts = 2
	config.RetryBackoff = 0
	engine := NewEngine(config, Hooks{EdgeRejected: func(rejection EdgeRejection) {
		mu.Lock()
		events = append(events, rejection)
		mu.Unlock()
	}})

	probe := engine.runTraceProbeWithRetry(&net.IPAddr{IP: net.ParseIP("127.0.0.1")})
	srv.Close()

	if probe.ok || probe.reason != traceFailureEdgeRestricted || !probe.ignored {
		t.Fatalf("probe = %#v", probe)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("请求次数 = %d, want 1（1034 直接淘汰，不重试）", got)
	}
	if len(events) != 1 {
		t.Fatalf("拒绝事件数量 = %d, want 1", len(events))
	}
	event := events[0]
	if event.Stage != "stage2_trace" || event.Note != EdgeNote1034 || event.Reason != downloadReasonEdgeRestricted || !event.Ignored {
		t.Fatalf("拒绝事件 = %#v", event)
	}
}

// 1034 是整个 IP 在边缘被限制，换追踪端点还是同一个边缘：首端点返回 1034 后
// 剩余端点不再请求，重试预算也不消耗。
func TestTrace1034AbortsRemainingEndpoints(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("error code: 1034\n"))
	}))
	t.Cleanup(srv.Close)
	port, err := strconv.Atoi(srv.URL[strings.LastIndex(srv.URL, ":")+1:])
	if err != nil {
		t.Fatal(err)
	}

	config := DefaultConfig()
	config.Disable = false
	config.TCPPort = port
	// 标准模式下端点 0 是 IP 字面量 URL（http://127.0.0.1/cdn-cgi/trace，不带端口），
	// 端点 1 是这里配置的 TraceURL；host 写法不同，所以两个端点都会留在列表里。
	config.TraceURL = "http://127.0.0.1:" + strconv.Itoa(port) + "/cdn-cgi/trace"
	config.HeadTimeout = 2 * time.Second
	config.TraceColoMode = TraceColoModeStandard
	config.RetryMaxAttempts = 3
	config.RetryBackoff = 0
	var events []EdgeRejection
	engine := NewEngine(config, Hooks{EdgeRejected: func(rejection EdgeRejection) {
		events = append(events, rejection)
	}})
	ip := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}

	if endpoints := engine.traceEndpointsForIP(ip); len(endpoints) != 2 {
		t.Fatalf("端点数量 = %d, want 2（本用例验证首端点 1034 后不再走次端点）", len(endpoints))
	}
	probe := engine.runTraceProbeWithRetry(ip)
	if probe.ok || probe.reason != traceFailureEdgeRestricted || !probe.ignored {
		t.Fatalf("probe = %#v", probe)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("请求次数 = %d, want 1（1034 后不再请求剩余端点，也不重试）", got)
	}
	if len(events) != 1 || events[0].Note != EdgeNote1034 {
		t.Fatalf("拒绝事件 = %#v", events)
	}
}

// 追踪阶段的 1034 必须整颗丢弃：不进入追踪结果（也就不会进文件测速），
// 不被软放行或追踪全失败回退捡回来，也不计入连续失败冷却。
func TestTraceAvailabilityDrops1034Candidate(t *testing.T) {
	_, port := edgeRestrictedServer(t, "error code: 1034\n")
	config := DefaultConfig()
	config.Disable = false
	config.TraceURL = "http://127.0.0.1:" + strconv.Itoa(port) + "/cdn-cgi/trace"
	config.TCPPort = port
	config.HeadTimeout = 2 * time.Second
	config.TraceColoMode = TraceColoModeStandard
	config.RetryMaxAttempts = 2
	config.RetryBackoff = 0
	// 阈值 1 意味着只要被计一次失败就会触发冷却；1034 不该走到那里。
	config.CooldownFailures = 1
	config.CooldownDuration = time.Millisecond
	var events []EdgeRejection
	var debugEvents []string
	engine := NewEngine(config, Hooks{
		EdgeRejected: func(rejection EdgeRejection) {
			events = append(events, rejection)
		},
		DebugEvent: func(event string, payload map[string]any) {
			reason, _ := payload["reason"].(string)
			debugEvents = append(debugEvents, event+"|"+reason)
		},
	})

	if got := engine.TestTraceAvailability(makeProbeSetWithIPs("127.0.0.1")); len(got) != 0 {
		t.Fatalf("追踪结果数量 = %d, want 0（1034 IP 必须丢弃）", len(got))
	}
	if len(events) != 1 {
		t.Fatalf("拒绝事件数量 = %d, want 1", len(events))
	}
	if event := events[0]; event.Stage != "stage2_trace" || event.Note != EdgeNote1034 || !event.Ignored {
		t.Fatalf("拒绝事件 = %#v", event)
	}
	for _, event := range debugEvents {
		switch {
		case strings.HasPrefix(event, "stage.cooldown"):
			t.Fatalf("%s: 1034 不应计入连续失败冷却", event)
		case strings.HasPrefix(event, "stage.fallback"):
			t.Fatalf("%s: 1034 不应被追踪全失败回退重新保留", event)
		case strings.HasPrefix(event, "stage.detail") && strings.Contains(event, "trace_soft_pass"):
			t.Fatalf("%s: 1034 不应被软放行", event)
		}
	}
}

// 文件测速的 1034 同样不吃重试预算：RetryMaxAttempts 设得再大也只有一次尝试。
func TestDownload1034IgnoresRetryBudget(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("error code: 1034\n"))
	}))
	t.Cleanup(srv.Close)
	port, err := strconv.Atoi(srv.URL[strings.LastIndex(srv.URL, ":")+1:])
	if err != nil {
		t.Fatal(err)
	}

	config := DefaultConfig()
	config.Disable = false
	config.URL = "http://127.0.0.1:" + strconv.Itoa(port) + "/500m"
	config.Timeout = 2 * time.Second
	config.TCPPort = port
	config.DownloadWarmupDuration = 0
	config.DownloadSampleInterval = time.Second
	config.RetryMaxAttempts = 3
	config.RetryBackoff = 0
	engine := NewEngine(config, Hooks{})
	ip := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}

	// 先量出单次尝试的请求数，避免把“探测 + 正文嗅探”的请求数写死在断言里。
	if _, err := engine.downloadHandlerAttemptOnce(ip, 1); err != nil {
		t.Fatal(err)
	}
	baseline := requests.Load()
	requests.Store(0)

	result := engine.runDownloadHandlerWithRetry(ip)
	if !result.ignored || result.retryable || result.validMeasurement || result.reason != downloadReasonEdgeRestricted {
		t.Fatalf("result = %#v, want ignored=true retryable=false", result)
	}
	if got := requests.Load(); got != baseline {
		t.Fatalf("请求次数 = %d, want %d（1034 不重试）", got, baseline)
	}
}
