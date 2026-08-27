package probecore

import (
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestNormalizeProbeConfigReturnsDefaultForEmptyConfig(t *testing.T) {
	normalized, warnings := NormalizeProbeConfig(ProbeConfig{}, ProbeConfigNormalizeOptions{})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if normalized.URL != DefaultFileTestURL {
		t.Fatalf("URL = %q, want %q", normalized.URL, DefaultFileTestURL)
	}
	if normalized.TraceURL != "https://speedtest.xyz9923.dpdns.org/cdn-cgi/trace" {
		t.Fatalf("TraceURL = %q, want derived default trace URL", normalized.TraceURL)
	}
	if normalized.Routines != DefaultProbeConfig().Routines {
		t.Fatalf("Routines = %d, want default", normalized.Routines)
	}
	if normalized.HttpingStatusCode != task.DefaultHTTPingStatusCode {
		t.Fatalf("HttpingStatusCode = %d, want %d", normalized.HttpingStatusCode, task.DefaultHTTPingStatusCode)
	}
	if !normalized.VerifyTLSCertificate {
		t.Fatal("VerifyTLSCertificate = false, want secure default")
	}
	if normalized.PortPolicy != PortPolicyFixedGlobal {
		t.Fatalf("PortPolicy = %q, want %q", normalized.PortPolicy, PortPolicyFixedGlobal)
	}
}

func TestNormalizeProbeConfigStrategyAliasesAndUnknownWarning(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{name: "fast alias", in: "latency", want: "fast"},
		{name: "full alias", in: "speed", want: "full"},
		{name: "exhaustive alias", in: "exhaustive", want: "full"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultProbeConfig()
			cfg.Strategy = tc.in
			normalized, warnings := NormalizeProbeConfig(cfg, ProbeConfigNormalizeOptions{})
			if normalized.Strategy != tc.want {
				t.Fatalf("Strategy = %q, want %q", normalized.Strategy, tc.want)
			}
			if len(warnings) != 0 {
				t.Fatalf("warnings = %#v, want none", warnings)
			}
		})
	}

	cfg := DefaultProbeConfig()
	cfg.Strategy = "mystery"
	normalized, warnings := NormalizeProbeConfig(cfg, ProbeConfigNormalizeOptions{})
	if normalized.Strategy != "fast" {
		t.Fatalf("Strategy = %q, want fast", normalized.Strategy)
	}
	if !probeConfigWarningsContain(warnings, "未知探测策略") {
		t.Fatalf("warnings = %#v, missing unknown strategy warning", warnings)
	}
}

func TestNormalizeProbeConfigConstraintsAndWarnings(t *testing.T) {
	cfg := DefaultProbeConfig()
	cfg.Routines = 5000
	cfg.HeadRoutines = 99
	cfg.PingTimes = 1
	cfg.TestCount = 0
	cfg.Stage3Limit = 0
	cfg.Stage1TimeoutMS = 0
	cfg.Stage2TimeoutMS = 0
	cfg.Stage3Concurrency = 7
	cfg.EventThrottleMS = 0
	cfg.DownloadSpeedSampleIntervalMS = 0
	cfg.DownloadGetConcurrency = 99
	cfg.DownloadBufferKB = 1
	cfg.DownloadHTTPProtocol = "bad"
	cfg.DownloadTimeSeconds = 0
	cfg.DownloadWarmupSeconds = -1
	cfg.TCPPort = 70000
	cfg.URL = " "
	cfg.TraceURL = " "
	cfg.UserAgent = " "
	cfg.HttpingStatusCode = 99
	cfg.MaxDelayMS = 0
	cfg.HeadMaxDelayMS = -1
	cfg.MinDelayMS = -1
	cfg.MaxLossRate = 2
	cfg.MinSpeedMB = -1
	cfg.PrintNum = -1
	cfg.IPFile = " "
	cfg.OutputFile = " "
	cfg.CSVEncoding = "bad"
	cfg.DebugLogMode = "bad"
	cfg.DebugLogVerbosity = "bad"

	normalized, warnings := NormalizeProbeConfig(cfg, ProbeConfigNormalizeOptions{
		MaxTCPRoutines:    300,
		MaxStage3Routines: 9,
	})
	if normalized.Routines != 300 {
		t.Fatalf("Routines = %d, want custom max 300", normalized.Routines)
	}
	if normalized.HeadRoutines != task.MaxTraceRoutines {
		t.Fatalf("HeadRoutines = %d, want %d", normalized.HeadRoutines, task.MaxTraceRoutines)
	}
	if normalized.PingTimes != task.MinPingTimes {
		t.Fatalf("PingTimes = %d, want %d", normalized.PingTimes, task.MinPingTimes)
	}
	if normalized.Stage3Concurrency != DefaultProbeConfig().Stage3Concurrency {
		t.Fatalf("Stage3Concurrency = %d, want default", normalized.Stage3Concurrency)
	}
	if normalized.DownloadGetConcurrency != task.MaxDownloadGetConcurrency {
		t.Fatalf("DownloadGetConcurrency = %d, want %d", normalized.DownloadGetConcurrency, task.MaxDownloadGetConcurrency)
	}
	if normalized.DownloadBufferKB != task.MinDownloadBufferKB {
		t.Fatalf("DownloadBufferKB = %d, want %d", normalized.DownloadBufferKB, task.MinDownloadBufferKB)
	}
	if normalized.DownloadHTTPProtocol != string(httpclient.ProtocolAuto) {
		t.Fatalf("DownloadHTTPProtocol = %q, want auto", normalized.DownloadHTTPProtocol)
	}
	if normalized.DownloadTimeSeconds != 4 {
		t.Fatalf("DownloadTimeSeconds = %d, want 4", normalized.DownloadTimeSeconds)
	}
	if normalized.DownloadWarmupSeconds != 1 {
		t.Fatalf("DownloadWarmupSeconds = %d, want 1", normalized.DownloadWarmupSeconds)
	}
	if normalized.TCPPort != 443 {
		t.Fatalf("TCPPort = %d, want 443", normalized.TCPPort)
	}
	if normalized.TraceURL != "https://speedtest.xyz9923.dpdns.org/cdn-cgi/trace" {
		t.Fatalf("TraceURL = %q, want derived default trace URL", normalized.TraceURL)
	}
	if normalized.HttpingStatusCode != task.DefaultHTTPingStatusCode {
		t.Fatalf("HttpingStatusCode = %d, want %d", normalized.HttpingStatusCode, task.DefaultHTTPingStatusCode)
	}
	if normalized.MaxLossRate != float64(utils.MaxAllowedLossRate) {
		t.Fatalf("MaxLossRate = %.2f, want %.2f", normalized.MaxLossRate, utils.MaxAllowedLossRate)
	}
	if normalized.CSVEncoding != utils.CSVEncodingUTF8 {
		t.Fatalf("CSVEncoding = %q, want %q", normalized.CSVEncoding, utils.CSVEncodingUTF8)
	}
	if normalized.DebugLogMode != utils.DebugLogModeStructured {
		t.Fatalf("DebugLogMode = %q, want structured", normalized.DebugLogMode)
	}
	if normalized.DebugLogVerbosity != utils.DebugLogVerbosityDetailed {
		t.Fatalf("DebugLogVerbosity = %q, want detailed", normalized.DebugLogVerbosity)
	}
	for _, want := range []string{
		"TCP并发线程最大支持 300",
		"追踪并发线程最大支持",
		"TCP 发包次数必须至少为 2",
		"测速并发线程固定为 9",
		"下载速度采样间隔必须大于 0",
		"GET 分片并发最大支持",
		"下载缓冲最小支持",
		"未知下载 HTTP 协议",
		"单 IP 下载测速时间必须大于 0",
		"下载预热时间不能为负数",
		"测速端口必须在 1-65535",
		"文件测速URL不能为空",
		"追踪有效状态码必须为 0 或 100-599",
		"追踪延迟上限设置已停用",
		"TCP 丢包率上限最大支持 100%",
		"未知 CSV 编码",
		"未知调试日志模式",
		"未知调试日志粒度",
	} {
		if !probeConfigWarningsContain(warnings, want) {
			t.Fatalf("warnings = %#v, missing %q", warnings, want)
		}
	}
}

func TestNormalizeProbeConfigURLsHeadersAndModes(t *testing.T) {
	cfg := DefaultProbeConfig()
	cfg.Strategy = "full"
	cfg.DisableDownload = false
	cfg.URL = `https:\/\/download.example.net\/cdn-cgi\/trace`
	cfg.TraceURL = "://bad"
	cfg.DownloadHostHeader = "  download.example.net  "
	cfg.DownloadSNI = "  download-tls.example.net  "
	cfg.SourceColoFilterPhase = "second-stage"
	cfg.TraceColoMode = "trace-url"
	cfg.RequestHeaders = strings.Join([]string{
		"X-Test: one",
		"bad-header",
	}, "\n")
	cfg.HttpingCFColoMode = "deny"
	cfg.DebugLogMode = utils.DebugLogModeFreeform
	cfg.DebugLogFormat = ""
	cfg.DebugLogVerbosity = utils.DebugLogVerbositySimple

	normalized, warnings := NormalizeProbeConfig(cfg, ProbeConfigNormalizeOptions{})
	if normalized.URL != "https://download.example.net/cdn-cgi/trace" {
		t.Fatalf("URL = %q, want unescaped trace URL", normalized.URL)
	}
	if normalized.TraceURL != "https://download.example.net/cdn-cgi/trace" {
		t.Fatalf("TraceURL = %q, want derived trace URL", normalized.TraceURL)
	}
	if normalized.SourceColoFilterPhase != SourceColoFilterPhaseStage2 {
		t.Fatalf("SourceColoFilterPhase = %q, want stage2", normalized.SourceColoFilterPhase)
	}
	if normalized.TraceColoMode != task.TraceColoModeTraceURL {
		t.Fatalf("TraceColoMode = %q, want trace_url", normalized.TraceColoMode)
	}
	if normalized.DownloadHostHeader != "download.example.net" || normalized.DownloadSNI != "download-tls.example.net" {
		t.Fatalf("download request identity = %q/%q", normalized.DownloadHostHeader, normalized.DownloadSNI)
	}
	if normalized.HttpingCFColoMode != task.ColoFilterModeDeny {
		t.Fatalf("HttpingCFColoMode = %q, want deny", normalized.HttpingCFColoMode)
	}
	if normalized.DebugLogFormat != utils.DefaultDebugLogFormat {
		t.Fatalf("DebugLogFormat = %q, want default freeform format", normalized.DebugLogFormat)
	}
	for _, want := range []string{
		"追踪 URL 无效",
		"文件测速URL当前指向 /cdn-cgi/trace",
		"格式无效，已忽略",
	} {
		if !probeConfigWarningsContain(warnings, want) {
			t.Fatalf("warnings = %#v, missing %q", warnings, want)
		}
	}
}

func TestPlatformDownloadAutoFallback(t *testing.T) {
	tests := []struct {
		goarch string
		goos   string
		want   httpclient.Protocol
	}{
		{goos: "linux", goarch: "arm", want: httpclient.ProtocolTCP},
		{goos: "linux", goarch: "arm64", want: httpclient.ProtocolTCP},
		{goos: "linux", goarch: "amd64", want: ""},
		{goos: "android", goarch: "arm64", want: httpclient.ProtocolTCP},
		{goos: "android", goarch: "arm", want: httpclient.ProtocolTCP},
		{goos: "windows", goarch: "amd64", want: ""},
	}

	for _, tt := range tests {
		if got := platformDownloadAutoFallback(tt.goos, tt.goarch); got != tt.want {
			t.Fatalf("platformDownloadAutoFallback(%q, %q) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
		}
	}
}

func TestResolveDownloadHTTPProtocolForPlatform(t *testing.T) {
	protocol, warnings := ResolveDownloadHTTPProtocolForPlatform("auto", "android", "arm64")
	if protocol != string(httpclient.ProtocolTCP) {
		t.Fatalf("android auto protocol = %q, want tcp", protocol)
	}
	if !probeConfigWarningsContain(warnings, "当前平台 android/arm64 默认将下载 HTTP 协议 auto 调整为 tcp") {
		t.Fatalf("warnings = %#v, missing android auto fallback", warnings)
	}

	protocol, warnings = ResolveDownloadHTTPProtocolForPlatform("h3", "android", "arm64")
	if protocol != string(httpclient.ProtocolH3) {
		t.Fatalf("explicit h3 protocol = %q, want h3", protocol)
	}
	if len(warnings) != 0 {
		t.Fatalf("explicit h3 warnings = %#v, want none", warnings)
	}

	protocol, warnings = ResolveDownloadHTTPProtocolForPlatform("mystery", "windows", "amd64")
	if protocol != string(httpclient.ProtocolAuto) {
		t.Fatalf("unknown protocol = %q, want auto", protocol)
	}
	if !probeConfigWarningsContain(warnings, "未知下载 HTTP 协议") {
		t.Fatalf("warnings = %#v, missing unknown protocol warning", warnings)
	}
}

func TestCleartextDownloadURLWarning(t *testing.T) {
	if got := CleartextDownloadURLWarning("android", "http://speed.example.test/500m"); got == "" {
		t.Fatal("expected android cleartext download URL warning")
	}
	if got := CleartextDownloadURLWarning("android", "https://speed.example.test/500m"); got != "" {
		t.Fatalf("https warning = %q, want empty", got)
	}
	if got := CleartextDownloadURLWarning("windows", "http://speed.example.test/500m"); got != "" {
		t.Fatalf("desktop warning = %q, want empty", got)
	}
}

func TestProbeURLHelpers(t *testing.T) {
	normalized := NormalizeProbeURLInput(`https:\/\/download.example.net\/__down?bytes=1`)
	if normalized != "https://download.example.net/__down?bytes=1" {
		t.Fatalf("NormalizeProbeURLInput = %q", normalized)
	}
	traceURL, ok := DeriveTraceURL(normalized)
	if !ok || traceURL != "https://download.example.net/cdn-cgi/trace" {
		t.Fatalf("DeriveTraceURL = %q, %v", traceURL, ok)
	}
	if !IsValidProbeURL(traceURL) {
		t.Fatalf("IsValidProbeURL(%q) = false", traceURL)
	}
	if !IsTraceProbeURL(traceURL) {
		t.Fatalf("IsTraceProbeURL(%q) = false", traceURL)
	}
	if _, ok := DeriveTraceURL("://bad"); ok {
		t.Fatal("DeriveTraceURL accepted invalid URL")
	}
}

func probeConfigWarningsContain(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}
