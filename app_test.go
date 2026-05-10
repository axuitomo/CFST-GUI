package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/task"
	"github.com/axuitomo/CFST-GUI/utils"
)

type resolverForTest map[string][]string

func (resolver resolverForTest) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	values, ok := resolver[host]
	if !ok {
		return nil, errors.New("host not found")
	}
	addrs := make([]net.IPAddr, 0, len(values))
	for _, value := range values {
		addrs = append(addrs, net.IPAddr{IP: net.ParseIP(value)})
	}
	return addrs, nil
}

type resolverForTestFunc func(context.Context, string) ([]net.IPAddr, error)

func (fn resolverForTestFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return fn(ctx, host)
}

func TestNormalizeDesktopSourceURLInput(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "bare host", raw: "bestcf.pages.dev/xinyitang3/ipv4.txt", want: "https://bestcf.pages.dev/xinyitang3/ipv4.txt"},
		{name: "https", raw: "https://example.com/ips.txt", want: "https://example.com/ips.txt"},
		{name: "http", raw: "http://example.com/ips.txt", want: "http://example.com/ips.txt"},
		{name: "empty", raw: " ", wantErr: "缺少远程 URL"},
		{name: "unsupported scheme", raw: "ftp://example.com/ips.txt", wantErr: "仅支持 http/https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeDesktopSourceURLInput(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeDesktopSourceURLInput() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeDesktopSourceURLInput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGithubRawJSDelivrURLConversion(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		cdn  string
	}{
		{
			name: "standard raw",
			raw:  "https://raw.githubusercontent.com/HandsomeMJZ/cfip/main/best_ips.txt",
			cdn:  "https://cdn.jsdelivr.net/gh/HandsomeMJZ/cfip@main/best_ips.txt",
		},
		{
			name: "refs heads raw",
			raw:  "https://raw.githubusercontent.com/HandsomeMJZ/cfip/refs/heads/main/best_ips.txt",
			cdn:  "https://cdn.jsdelivr.net/gh/HandsomeMJZ/cfip@main/best_ips.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := githubRawToJSDelivrURL(tt.raw)
			if !ok || got != tt.cdn {
				t.Fatalf("githubRawToJSDelivrURL() = %q, %v; want %q, true", got, ok, tt.cdn)
			}
			raw, ok := jsDelivrToGithubRawURL(tt.cdn)
			wantRaw := "https://raw.githubusercontent.com/HandsomeMJZ/cfip/main/best_ips.txt"
			if !ok || raw != wantRaw {
				t.Fatalf("jsDelivrToGithubRawURL() = %q, %v; want %q, true", raw, ok, wantRaw)
			}
		})
	}
}

func TestLoadDesktopSourceContentFallsBackToJSDelivr(t *testing.T) {
	var hosts []string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Host)
		if req.URL.Host == "raw.githubusercontent.com" {
			return &http.Response{
				Status:     "500 Internal Server Error",
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("raw failed")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("1.1.1.1\n")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	result, err := loadDesktopSourceContent(DesktopSource{
		Kind: "url",
		Name: "HandsomeMJZ",
		URL:  "https://raw.githubusercontent.com/HandsomeMJZ/cfip/refs/heads/main/best_ips.txt",
	}, defaultProbeConfig(), client)
	if err != nil {
		t.Fatalf("loadDesktopSourceContent() error = %v", err)
	}
	if result.Raw != "1.1.1.1\n" {
		t.Fatalf("Raw = %q, want fallback body", result.Raw)
	}
	if len(hosts) != 2 || hosts[0] != "raw.githubusercontent.com" || hosts[1] != "cdn.jsdelivr.net" {
		t.Fatalf("hosts = %#v, want raw then jsDelivr", hosts)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "jsDelivr CDN") {
		t.Fatalf("warnings = %#v, want jsDelivr fallback warning", result.Warnings)
	}
}

func TestLoadDesktopSourceContentNetworkErrorFallsBackToJSDelivr(t *testing.T) {
	var calls int
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls += 1
		if req.URL.Host == "raw.githubusercontent.com" {
			return nil, errors.New("context deadline exceeded")
		}
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("1.0.0.1\n")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	result, err := loadDesktopSourceContent(DesktopSource{
		Kind: "url",
		Name: "HandsomeMJZ",
		URL:  "https://raw.githubusercontent.com/HandsomeMJZ/cfip/main/best_ips.txt",
	}, defaultProbeConfig(), client)
	if err != nil {
		t.Fatalf("loadDesktopSourceContent() error = %v", err)
	}
	if calls != 2 || result.Raw != "1.0.0.1\n" {
		t.Fatalf("calls = %d, raw = %q; want fallback success", calls, result.Raw)
	}
}

func TestLoadDesktopSourceContentDoesNotRetry404(t *testing.T) {
	var calls int
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls += 1
		return &http.Response{
			Status:     "404 Not Found",
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("missing")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	_, err := loadDesktopSourceContent(DesktopSource{
		Kind: "url",
		URL:  "https://raw.githubusercontent.com/HandsomeMJZ/cfip/main/missing.txt",
	}, defaultProbeConfig(), client)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want 404", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want no retry for 404", calls)
	}
}

func TestCSVFloatPtrAllowsZero(t *testing.T) {
	got := csvFloatPtr("0")
	if got == nil || *got != 0 {
		t.Fatalf("csvFloatPtr(0) = %v, want pointer to 0", got)
	}
	if got := csvFloatPtr("-0.1"); got != nil {
		t.Fatalf("csvFloatPtr(-0.1) = %v, want nil", *got)
	}
}

func TestDesktopConfigCSVEncodingNormalizes(t *testing.T) {
	cfg, warnings := desktopConfigToProbeConfig(map[string]any{
		"export": map[string]any{
			"csv_encoding": "utf-8-bom",
			"file_name":    "result.csv",
		},
	})
	if cfg.CSVEncoding != utils.CSVEncodingUTF8BOM {
		t.Fatalf("CSVEncoding = %q, want %q", cfg.CSVEncoding, utils.CSVEncodingUTF8BOM)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}

	cfg, warnings = desktopConfigToProbeConfig(map[string]any{
		"export": map[string]any{
			"csv_encoding": "shift-jis",
			"file_name":    "result.csv",
		},
	})
	if cfg.CSVEncoding != utils.CSVEncodingUTF8 {
		t.Fatalf("CSVEncoding = %q, want %q", cfg.CSVEncoding, utils.CSVEncodingUTF8)
	}
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, "\n"), "未知 CSV 编码") {
		t.Fatalf("warnings = %#v, want unknown CSV encoding warning", warnings)
	}
}

func TestReadProbeResultRowsFromCSVHandlesBOMHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.csv")
	raw := "\xEF\xBB\xBFIP 地址,已发送,已接收,丢包率,TCP延迟(ms),平均速率(MB/s),最高速率(MB/s),地区码,追踪延迟(ms)\n1.1.1.1,3,3,0.00,12.34,56.78,78.90,HKG,34.56\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	rows, err := readProbeResultRowsFromCSV(path)
	if err != nil {
		t.Fatalf("readProbeResultRowsFromCSV returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Address != "1.1.1.1" {
		t.Fatalf("rows = %#v, want one parsed row", rows)
	}
}

func TestConvertProbeRowRoundsResultMetricsToTwoDecimals(t *testing.T) {
	row := convertProbeRow(utils.CloudflareIPData{
		PingData: &utils.PingData{
			IP:       &net.IPAddr{IP: net.ParseIP("1.1.1.1")},
			Sended:   4,
			Received: 4,
			Delay:    12*time.Millisecond + 344*time.Microsecond,
			Colo:     "HKG",
		},
		HeadDelay:        8*time.Millisecond + 345*time.Microsecond,
		DownloadSpeed:    56.785 * 1024 * 1024,
		MaxDownloadSpeed: 78.901 * 1024 * 1024,
	})

	if row.DelayMS != 12.34 {
		t.Fatalf("DelayMS = %v, want 12.34", row.DelayMS)
	}
	if row.TraceDelayMS != 8.35 {
		t.Fatalf("TraceDelayMS = %v, want 8.35", row.TraceDelayMS)
	}
	if row.DownloadSpeedMB != 56.79 {
		t.Fatalf("DownloadSpeedMB = %v, want 56.79", row.DownloadSpeedMB)
	}
	if row.MaxDownloadSpeedMB != 78.9 {
		t.Fatalf("MaxDownloadSpeedMB = %v, want 78.9", row.MaxDownloadSpeedMB)
	}
}

func TestSummarizeProbeRowsRoundsAverageDelayToTwoDecimals(t *testing.T) {
	summary := summarizeProbeRows([]ProbeRow{
		{DelayMS: 10.121, DownloadSpeedMB: 1, IP: "1.1.1.1"},
		{DelayMS: 10.123, DownloadSpeedMB: 2, IP: "1.1.1.2"},
	}, 2)

	if summary.AverageDelayMS != 10.12 {
		t.Fatalf("AverageDelayMS = %v, want 10.12", summary.AverageDelayMS)
	}
}

func TestDesktopConfigToProbeConfigClampsTraceStage(t *testing.T) {
	cfg, warnings := desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"strategy":              "full",
			"print_num":             3,
			"download_speed_metric": "peak",
			"concurrency": map[string]any{
				"stage1": 123,
				"stage2": 99,
				"stage3": 99,
			},
			"stage_limits": map[string]any{
				"stage2": 17,
				"stage3": 5,
			},
			"trace_url": "https://example.com/cdn-cgi/trace",
			"thresholds": map[string]any{
				"max_http_latency_ms": 300,
				"max_tcp_latency_ms":  200,
			},
		},
	})

	if cfg.HeadRoutines != task.MaxTraceRoutines {
		t.Fatalf("HeadRoutines = %d, want %d", cfg.HeadRoutines, task.MaxTraceRoutines)
	}
	if cfg.HeadTestCount != 0 {
		t.Fatalf("HeadTestCount = %d, want unlimited 0", cfg.HeadTestCount)
	}
	if cfg.TestCount != 5 {
		t.Fatalf("TestCount = %d, want stage3 limit 5", cfg.TestCount)
	}
	if cfg.Stage3Limit != 5 {
		t.Fatalf("Stage3Limit = %d, want 5", cfg.Stage3Limit)
	}
	if cfg.PrintNum != 3 {
		t.Fatalf("PrintNum = %d, want 3", cfg.PrintNum)
	}
	if cfg.DownloadSpeedMetric != utils.DownloadSpeedMetricMax {
		t.Fatalf("DownloadSpeedMetric = %q, want max", cfg.DownloadSpeedMetric)
	}
	if cfg.MaxDelayMS != 200 {
		t.Fatalf("MaxDelayMS = %d, want 200", cfg.MaxDelayMS)
	}
	if cfg.HeadMaxDelayMS != 0 {
		t.Fatalf("HeadMaxDelayMS = %d, want disabled 0", cfg.HeadMaxDelayMS)
	}
	if cfg.DisableDownload {
		t.Fatal("full strategy should enable GET download stage")
	}
	if cfg.TraceURL != "https://example.com/cdn-cgi/trace" {
		t.Fatalf("TraceURL = %q, want configured Trace URL", cfg.TraceURL)
	}
	if !warningsContain(warnings, "追踪并发线程最大支持") {
		t.Fatalf("warnings = %#v, want trace concurrency clamp warning", warnings)
	}
	if cfg.Stage3Concurrency != task.MaxDownloadRoutines {
		t.Fatalf("Stage3Concurrency = %d, want %d", cfg.Stage3Concurrency, task.MaxDownloadRoutines)
	}
	if !warningsContain(warnings, "测速并发线程固定为 1") {
		t.Fatalf("warnings = %#v, want fixed stage3 concurrency warning", warnings)
	}
	if !warningsContain(warnings, "追踪延迟上限设置已停用") {
		t.Fatalf("warnings = %#v, want trace latency disabled warning", warnings)
	}
}

func TestNormalizeProbeConfigReportsConstraintWarnings(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.Routines = 5000
	cfg.HeadRoutines = 99
	cfg.PingTimes = 0
	cfg.HeadTestCount = 0
	cfg.TestCount = 0
	cfg.EventThrottleMS = 0
	cfg.DownloadSpeedSampleIntervalMS = 0
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

	normalized, warnings := normalizeProbeConfig(cfg)
	if normalized.Routines != maxDesktopTCPRoutines {
		t.Fatalf("Routines = %d, want %d", normalized.Routines, maxDesktopTCPRoutines)
	}
	if normalized.HeadRoutines != task.MaxTraceRoutines {
		t.Fatalf("HeadRoutines = %d, want %d", normalized.HeadRoutines, task.MaxTraceRoutines)
	}
	if normalized.TCPPort != 443 {
		t.Fatalf("TCPPort = %d, want 443", normalized.TCPPort)
	}
	if normalized.URL != defaultProbeConfig().URL {
		t.Fatalf("URL = %q, want default", normalized.URL)
	}
	if normalized.TraceURL != "https://speed.cloudflare.com/cdn-cgi/trace" {
		t.Fatalf("TraceURL = %q, want derived default trace URL", normalized.TraceURL)
	}
	if normalized.HttpingStatusCode != 0 {
		t.Fatalf("HttpingStatusCode = %d, want 0", normalized.HttpingStatusCode)
	}
	if normalized.MaxLossRate != float64(utils.MaxAllowedLossRate) {
		t.Fatalf("MaxLossRate = %.2f, want %.2f", normalized.MaxLossRate, utils.MaxAllowedLossRate)
	}
	for _, want := range []string{
		"TCP并发线程最大支持",
		"追踪并发线程最大支持",
		"TCP 发包次数必须大于 0",
		"下载速度采样间隔必须大于 0",
		"单 IP 下载测速时间必须大于 0",
		"下载预热时间不能为负数",
		"测速端口必须在 1-65535",
		"文件测速URL不能为空",
		"追踪有效状态码必须为 0 或 100-599",
		"追踪延迟上限设置已停用",
		"TCP 丢包率上限最大支持 100%",
		"导出文件路径不能为空",
	} {
		if !warningsContain(warnings, want) {
			t.Fatalf("warnings = %#v, missing %q", warnings, want)
		}
	}
}

func TestDesktopConfigToProbeConfigNormalizesHTTPingStatusCode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		probe        map[string]any
		want         int
		wantWarnings bool
	}{
		{name: "default", probe: map[string]any{}, want: 0},
		{name: "zero unlimited", probe: map[string]any{"httping_status_code": 0}, want: 0},
		{name: "explicit status", probe: map[string]any{"httpingStatusCode": 200}, want: 200},
		{name: "below range", probe: map[string]any{"httping_status_code": 99}, want: 0, wantWarnings: true},
		{name: "above range", probe: map[string]any{"httping_status_code": 600}, want: 0, wantWarnings: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, warnings := desktopConfigToProbeConfig(map[string]any{"probe": tc.probe})
			if cfg.HttpingStatusCode != tc.want {
				t.Fatalf("HttpingStatusCode = %d, want %d", cfg.HttpingStatusCode, tc.want)
			}
			if got := warningsContain(warnings, "追踪有效状态码必须为 0 或 100-599"); got != tc.wantWarnings {
				t.Fatalf("warnings = %#v, contains status warning = %v, want %v", warnings, got, tc.wantWarnings)
			}
		})
	}
}

func TestDesktopConfigDownloadSamplingIntervalMSCompatibility(t *testing.T) {
	cfg, _ := desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"download_speed_sample_interval_ms":      750,
			"download_speed_sample_interval_seconds": 9,
		},
	})
	if cfg.DownloadSpeedSampleIntervalMS != 750 {
		t.Fatalf("DownloadSpeedSampleIntervalMS = %d, want ms field priority 750", cfg.DownloadSpeedSampleIntervalMS)
	}

	cfg, _ = desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"download_speed_sample_interval_seconds": 3,
		},
	})
	if cfg.DownloadSpeedSampleIntervalMS != 3000 {
		t.Fatalf("DownloadSpeedSampleIntervalMS = %d, want legacy seconds converted to 3000", cfg.DownloadSpeedSampleIntervalMS)
	}
}

func TestDesktopConfigDownloadHTTPFieldsNormalize(t *testing.T) {
	cfg, _ := desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"download_get_concurrency": 12,
			"download_buffer_kb":       512,
			"download_http_protocol":   "h3",
		},
	})
	normalized, warnings := normalizeProbeConfig(cfg)
	if normalized.DownloadGetConcurrency != 12 {
		t.Fatalf("DownloadGetConcurrency = %d, want 12", normalized.DownloadGetConcurrency)
	}
	if normalized.DownloadBufferKB != 512 {
		t.Fatalf("DownloadBufferKB = %d, want 512", normalized.DownloadBufferKB)
	}
	if normalized.DownloadHTTPProtocol != "h3" {
		t.Fatalf("DownloadHTTPProtocol = %q, want h3", normalized.DownloadHTTPProtocol)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}

	cfg.DownloadGetConcurrency = 99
	cfg.DownloadBufferKB = 1
	cfg.DownloadHTTPProtocol = "bad"
	normalized, warnings = normalizeProbeConfig(cfg)
	if normalized.DownloadGetConcurrency != task.MaxDownloadGetConcurrency {
		t.Fatalf("DownloadGetConcurrency = %d, want clamp %d", normalized.DownloadGetConcurrency, task.MaxDownloadGetConcurrency)
	}
	if normalized.DownloadBufferKB != task.MinDownloadBufferKB {
		t.Fatalf("DownloadBufferKB = %d, want min %d", normalized.DownloadBufferKB, task.MinDownloadBufferKB)
	}
	if normalized.DownloadHTTPProtocol != "auto" {
		t.Fatalf("DownloadHTTPProtocol = %q, want auto", normalized.DownloadHTTPProtocol)
	}
	for _, want := range []string{"GET 分片并发最大支持", "下载缓冲最小支持", "未知下载 HTTP 协议"} {
		if !warningsContain(warnings, want) {
			t.Fatalf("warnings = %#v, missing %q", warnings, want)
		}
	}
}

func TestDesktopConfigDebugCaptureEnabledCompatibility(t *testing.T) {
	cfg, _ := desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"debug":                 true,
			"debug_capture_address": "9000",
		},
	})
	if !cfg.DebugCaptureEnabled {
		t.Fatal("legacy debug capture address should enable capture by default")
	}
	if got := effectiveDebugCaptureAddress(cfg); got != "127.0.0.1:9000" {
		t.Fatalf("effective capture address = %q, want normalized address", got)
	}

	cfg, _ = desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"debug":                 true,
			"debug_capture_address": "9000",
			"debug_capture_enabled": false,
		},
	})
	if cfg.DebugCaptureEnabled {
		t.Fatal("explicit disabled debug capture should be preserved")
	}
	if got := effectiveDebugCaptureAddress(cfg); got != "" {
		t.Fatalf("effective capture address = %q, want disabled capture", got)
	}
}

func TestNormalizeProbeConfigAllowsShortDownloadTime(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.DownloadTimeSeconds = 3

	normalized, warnings := normalizeProbeConfig(cfg)
	if normalized.DownloadTimeSeconds != 3 {
		t.Fatalf("DownloadTimeSeconds = %d, want 3", normalized.DownloadTimeSeconds)
	}
	if warningsContain(warnings, "单 IP 下载测速时间") {
		t.Fatalf("warnings = %#v, did not expect download time warning", warnings)
	}
}

func TestConfigureCLITraceURLUsesFileURLAndNewDefault(t *testing.T) {
	oldURL := task.URL
	oldTraceURL := task.TraceURL
	t.Cleanup(func() {
		task.URL = oldURL
		task.TraceURL = oldTraceURL
	})

	task.URL = "https://download.example.net/__down?bytes=1"
	task.TraceURL = ""
	configureCLITraceURL()
	if task.TraceURL != "https://download.example.net/cdn-cgi/trace" {
		t.Fatalf("TraceURL = %q, want derived trace URL", task.TraceURL)
	}

	task.URL = "://bad"
	task.TraceURL = ""
	configureCLITraceURL()
	if task.TraceURL != "https://speed.cloudflare.com/cdn-cgi/trace" {
		t.Fatalf("TraceURL fallback = %q, want new default trace URL", task.TraceURL)
	}
}

func TestNormalizeProbeConfigUnescapesTraceURLSlashes(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.URL = `https:\/\/download.example.net\/__down?bytes=1`
	cfg.TraceURL = `https:\/\/trace.example.net\/cdn-cgi\/trace`

	normalized, warnings := normalizeProbeConfig(cfg)
	if normalized.URL != "https://download.example.net/__down?bytes=1" {
		t.Fatalf("URL = %q, want unescaped file URL", normalized.URL)
	}
	if normalized.TraceURL != "https://trace.example.net/cdn-cgi/trace" {
		t.Fatalf("TraceURL = %q, want unescaped trace URL", normalized.TraceURL)
	}
	if warningsContain(warnings, "追踪 URL 无效") {
		t.Fatalf("warnings = %#v, should not reject escaped trace URL", warnings)
	}
}

func TestNormalizeProbeConfigDerivesTraceURLFromEscapedFileURL(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.URL = `https:\/\/download.example.net\/__down?bytes=1`
	cfg.TraceURL = ""

	normalized, warnings := normalizeProbeConfig(cfg)
	if normalized.TraceURL != "https://download.example.net/cdn-cgi/trace" {
		t.Fatalf("TraceURL = %q, want trace URL derived from unescaped file URL", normalized.TraceURL)
	}
	if warningsContain(warnings, "追踪 URL 无法从文件测速URL派生") {
		t.Fatalf("warnings = %#v, should derive trace URL from escaped file URL", warnings)
	}
}

func TestNormalizeProbeConfigRejectsSinglePingTime(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.PingTimes = 1

	normalized, warnings := normalizeProbeConfig(cfg)
	if normalized.PingTimes != task.MinPingTimes {
		t.Fatalf("PingTimes = %d, want %d", normalized.PingTimes, task.MinPingTimes)
	}
	if !warningsContain(warnings, "TCP 发包次数必须至少为 2") {
		t.Fatalf("warnings = %#v, missing minimum ping times warning", warnings)
	}
}

func TestDesktopConfigToProbeConfigAppliesAdvancedFields(t *testing.T) {
	cfg, warnings := desktopConfigToProbeConfig(map[string]any{
		"export": map[string]any{
			"overwrite": "append",
		},
		"probe": map[string]any{
			"concurrency": map[string]any{
				"stage3": 3,
			},
			"cooldown_policy": map[string]any{
				"consecutive_failures": 1,
				"cooldown_ms":          500,
			},
			"retry_policy": map[string]any{
				"backoff_ms":   100,
				"max_attempts": 2,
			},
			"downloadWarmupSeconds": 0,
			"stage_limits": map[string]any{
				"stage1": 100,
			},
			"timeouts": map[string]any{
				"stage1_ms": 250,
				"stage2_ms": 500,
			},
		},
	})
	if cfg.Strategy != "fast" {
		t.Fatalf("Strategy = %q, want fast", cfg.Strategy)
	}
	if !cfg.ExportAppend {
		t.Fatal("ExportAppend = false, want true")
	}
	if cfg.Stage3Concurrency != 1 {
		t.Fatalf("Stage3Concurrency = %d, want forced 1", cfg.Stage3Concurrency)
	}
	if cfg.Stage1Limit != 0 {
		t.Fatalf("Stage1Limit = %d, want disabled 0", cfg.Stage1Limit)
	}
	if cfg.CooldownFailures != 1 || cfg.CooldownMS != 500 {
		t.Fatalf("cooldown = (%d,%d), want (1,500)", cfg.CooldownFailures, cfg.CooldownMS)
	}
	if cfg.RetryBackoffMS != 100 || cfg.RetryMaxAttempts != 2 {
		t.Fatalf("retry = (%d,%d), want (100,2)", cfg.RetryBackoffMS, cfg.RetryMaxAttempts)
	}
	if cfg.DownloadWarmupSeconds != 0 {
		t.Fatalf("DownloadWarmupSeconds = %d, want 0", cfg.DownloadWarmupSeconds)
	}
	if cfg.Stage1TimeoutMS != 250 || cfg.Stage2TimeoutMS != 500 {
		t.Fatalf("timeouts = (%d,%d), want (250,500)", cfg.Stage1TimeoutMS, cfg.Stage2TimeoutMS)
	}
	for _, warning := range warnings {
		if strings.Contains(warning, "暂未") {
			t.Fatalf("warnings = %#v, should not contain reserved-field warnings", warnings)
		}
	}
}

func TestDesktopConfigToProbeConfigAppliesDebugLogFields(t *testing.T) {
	cfg, warnings := desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"debug_log_format":    "{event} {message}",
			"debug_log_mode":      utils.DebugLogModeFreeform,
			"debug_log_verbosity": utils.DebugLogVerbositySimple,
		},
	})
	if cfg.DebugLogMode != utils.DebugLogModeFreeform {
		t.Fatalf("DebugLogMode = %q, want %q", cfg.DebugLogMode, utils.DebugLogModeFreeform)
	}
	if cfg.DebugLogFormat != "{event} {message}" {
		t.Fatalf("DebugLogFormat = %q, want custom template", cfg.DebugLogFormat)
	}
	if cfg.DebugLogVerbosity != utils.DebugLogVerbositySimple {
		t.Fatalf("DebugLogVerbosity = %q, want %q", cfg.DebugLogVerbosity, utils.DebugLogVerbositySimple)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}

	cfg, warnings = desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"debug_log_mode":      "bad-mode",
			"debug_log_verbosity": "bad-verbosity",
		},
	})
	if cfg.DebugLogMode != utils.DebugLogModeStructured {
		t.Fatalf("DebugLogMode = %q, want %q", cfg.DebugLogMode, utils.DebugLogModeStructured)
	}
	if !warningsContain(warnings, "未知调试日志模式") {
		t.Fatalf("warnings = %#v, want invalid log mode warning", warnings)
	}
	if cfg.DebugLogVerbosity != utils.DebugLogVerbosityDetailed {
		t.Fatalf("DebugLogVerbosity = %q, want %q", cfg.DebugLogVerbosity, utils.DebugLogVerbosityDetailed)
	}
	if !warningsContain(warnings, "未知调试日志粒度") {
		t.Fatalf("warnings = %#v, want invalid log verbosity warning", warnings)
	}

	cfg, warnings = desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"debug_log_format": "   ",
			"debug_log_mode":   utils.DebugLogModeFreeform,
		},
	})
	if cfg.DebugLogFormat != utils.DefaultDebugLogFormat {
		t.Fatalf("DebugLogFormat = %q, want default template", cfg.DebugLogFormat)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none for empty freeform template fallback", warnings)
	}
}

func TestDesktopConfigToProbeConfigNormalizesRequestHeaders(t *testing.T) {
	cfg, warnings := desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"requestHeaders": strings.Join([]string{
				"Accept: */*",
				"Host: example.com",
				"X-Test: ok",
				"bad header: nope",
			}, "\n"),
		},
	})
	if cfg.RequestHeaders != "Accept: */*\nX-Test: ok" {
		t.Fatalf("RequestHeaders = %q, want normalized custom headers", cfg.RequestHeaders)
	}
	if len(warnings) < 2 {
		t.Fatalf("warnings = %#v, want reserved and invalid header warnings", warnings)
	}

	cfg, warnings = desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"request_headers": "X-Snake: yes",
		},
	})
	if cfg.RequestHeaders != "X-Snake: yes" {
		t.Fatalf("RequestHeaders = %q, want snake_case value", cfg.RequestHeaders)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}

func TestDesktopMCISEngineConfigIgnoresFinalColoFilter(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.HttpingCFColo = "hkg,nrt LAX hkg zzz"

	mcisCfg := buildDesktopMCISEngineConfig(cfg, 500)

	if len(mcisCfg.ColoAllow) != 0 {
		t.Fatalf("ColoAllow = %#v, want empty because final COLO filter belongs to stage 2 only", mcisCfg.ColoAllow)
	}
}

func TestDesktopMCISProbeConfigOnlySetsDebugDialAddressWhenConfigured(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.Debug = true
	cfg.DebugCaptureAddress = ""

	probeCfg, _ := buildDesktopMCISProbeConfig(cfg)
	if probeCfg.DialAddress != "" {
		t.Fatalf("DialAddress = %q, want direct connection when debug capture address is empty", probeCfg.DialAddress)
	}

	cfg.DebugCaptureAddress = "9000"
	cfg.DebugCaptureEnabled = true
	probeCfg, _ = buildDesktopMCISProbeConfig(cfg)
	if probeCfg.DialAddress != "127.0.0.1:9000" {
		t.Fatalf("DialAddress = %q, want normalized debug capture address", probeCfg.DialAddress)
	}

	cfg.DebugCaptureEnabled = false
	probeCfg, _ = buildDesktopMCISProbeConfig(cfg)
	if probeCfg.DialAddress != "" {
		t.Fatalf("DialAddress = %q, want direct connection when debug capture is disabled", probeCfg.DialAddress)
	}
}

func TestDesktopSourceColoFilterPrefiltersTraverseEntries(t *testing.T) {
	writeDesktopColoDictionaryForTest(t)

	source := DesktopSource{
		ColoFilter: "SJC",
		Content:    "104.16.0.1\n104.20.0.1\nbad",
		IPLimit:    10,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "test",
	}
	entries, warnings, invalid, err := buildDesktopSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err != nil {
		t.Fatalf("buildDesktopSourceEntriesWithConfig returned error: %v", err)
	}
	if invalid != 1 {
		t.Fatalf("invalid = %d, want 1", invalid)
	}
	want := []string{"104.16.0.1"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
	if !warningsContain(warnings, "COLO 白名单 SJC 预筛") {
		t.Fatalf("warnings = %#v, want COLO prefilter warning", warnings)
	}
}

func TestDesktopSourceColoFilterDenyModePrefiltersTraverseEntries(t *testing.T) {
	writeDesktopColoDictionaryForTest(t)

	source := DesktopSource{
		ColoFilter:     "SJC",
		ColoFilterMode: task.ColoFilterModeDeny,
		Content:        "104.16.0.1\n104.20.0.1\n203.0.113.1",
		IPLimit:        10,
		IPMode:         "traverse",
		Kind:           "inline",
		Name:           "deny-test",
	}
	entries, warnings, invalid, err := buildDesktopSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err != nil {
		t.Fatalf("buildDesktopSourceEntriesWithConfig returned error: %v", err)
	}
	if invalid != 0 {
		t.Fatalf("invalid = %d, want 0", invalid)
	}
	want := []string{"104.20.0.1", "203.0.113.1"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
	if !warningsContain(warnings, "COLO 黑名单 SJC 预筛") {
		t.Fatalf("warnings = %#v, want deny COLO prefilter warning", warnings)
	}
}

func TestDesktopSourceParsesComplexInputAndResolvesDomain(t *testing.T) {
	oldResolver := sourceParseResolver
	sourceParseResolver = resolverForTest(map[string][]string{
		"edge.example.com": {"203.0.113.10", "2001:db8::10"},
	})
	t.Cleanup(func() { sourceParseResolver = oldResolver })

	source := DesktopSource{
		Content: strings.Join([]string{
			"# comment",
			"1.1.1.1 # keep only the address",
			"address=/cf.example.com/1.0.0.1",
			"https://edge.example.com/path/file.txt",
			"bad-token",
		}, "\n"),
		IPLimit: 10,
		IPMode:  "traverse",
		Kind:    "inline",
		Name:    "complex",
	}

	entries, warnings, invalid, err := buildDesktopSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err != nil {
		t.Fatalf("buildDesktopSourceEntriesWithConfig returned error: %v", err)
	}
	if invalid != 1 {
		t.Fatalf("invalid = %d, want 1", invalid)
	}
	want := []string{"1.1.1.1", "1.0.0.1", "203.0.113.10", "2001:db8::10"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
	if !warningsContain(warnings, "IP/CIDR/域名") {
		t.Fatalf("warnings = %#v, want IP/CIDR/domain warning", warnings)
	}
}

func TestDesktopSourceStopsDomainResolutionAtLimitWithoutColoFilter(t *testing.T) {
	calls := make(map[string]int)
	oldResolver := sourceParseResolver
	sourceParseResolver = resolverForTestFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		calls[host]++
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.50")}}, nil
	})
	t.Cleanup(func() { sourceParseResolver = oldResolver })

	source := DesktopSource{
		Content: "first.example.com\nsecond.example.com",
		IPLimit: 1,
		IPMode:  "traverse",
		Kind:    "inline",
		Name:    "limited",
	}

	entries, _, _, err := buildDesktopSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err != nil {
		t.Fatalf("buildDesktopSourceEntriesWithConfig returned error: %v", err)
	}
	if !reflect.DeepEqual(entries, []string{"203.0.113.50"}) {
		t.Fatalf("entries = %#v, want one resolved IP", entries)
	}
	if calls["first.example.com"] != 1 || calls["second.example.com"] != 0 {
		t.Fatalf("resolver calls = %#v, want only first domain resolved", calls)
	}
}

func TestDesktopSourceKeepsFullDomainResolutionWithColoFilter(t *testing.T) {
	writeDesktopColoDictionaryForTest(t)
	calls := make(map[string]int)
	oldResolver := sourceParseResolver
	sourceParseResolver = resolverForTestFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		calls[host]++
		if host == "first.example.com" {
			return []net.IPAddr{{IP: net.ParseIP("104.20.0.1")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("104.16.0.1")}}, nil
	})
	t.Cleanup(func() { sourceParseResolver = oldResolver })

	source := DesktopSource{
		ColoFilter: "SJC",
		Content:    "first.example.com\nsecond.example.com",
		IPLimit:    1,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "colo-domain",
	}

	entries, _, _, err := buildDesktopSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err != nil {
		t.Fatalf("buildDesktopSourceEntriesWithConfig returned error: %v", err)
	}
	if !reflect.DeepEqual(entries, []string{"104.16.0.1"}) {
		t.Fatalf("entries = %#v, want COLO-matched second domain IP", entries)
	}
	if calls["first.example.com"] != 1 || calls["second.example.com"] != 1 {
		t.Fatalf("resolver calls = %#v, want both domains resolved before COLO filter", calls)
	}
}

func TestDesktopSourceColoFilterIntersectsCIDRBeforeTraverse(t *testing.T) {
	writeDesktopColoDictionaryForTest(t)

	source := DesktopSource{
		ColoFilter: "SJC",
		Content:    "104.0.0.0/8",
		IPLimit:    10,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "cidr",
	}
	entries, _, _, err := buildDesktopSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err != nil {
		t.Fatalf("buildDesktopSourceEntriesWithConfig returned error: %v", err)
	}
	want := []string{"104.16.0.0", "104.16.0.1", "104.16.0.2", "104.16.0.3"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
}

func TestDesktopSourceColoFilterPrefiltersMICSInput(t *testing.T) {
	writeDesktopColoDictionaryForTest(t)

	oldRunner := desktopMCISSearchRunner
	var gotTokens []string
	desktopMCISSearchRunner = func(tokens []string, source DesktopSource, cfg ProbeConfig, limit int) ([]string, []string, error) {
		gotTokens = append([]string(nil), tokens...)
		return []string{"104.16.0.1"}, nil, nil
	}
	t.Cleanup(func() { desktopMCISSearchRunner = oldRunner })

	source := DesktopSource{
		ColoFilter: "SJC",
		Content:    "104.0.0.0/8",
		IPLimit:    10,
		IPMode:     "mcis",
		Kind:       "inline",
		Name:       "mcis",
	}
	entries, _, _, err := buildDesktopSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err != nil {
		t.Fatalf("buildDesktopSourceEntriesWithConfig returned error: %v", err)
	}
	if !reflect.DeepEqual(gotTokens, []string{"104.16.0.0/30"}) {
		t.Fatalf("MICS tokens = %#v, want COLO-intersected CIDR", gotTokens)
	}
	if !reflect.DeepEqual(entries, []string{"104.16.0.1"}) {
		t.Fatalf("entries = %#v, want fake MICS result", entries)
	}
}

func TestDesktopSourceColoFilterSelectsDictionaryByInputFamily(t *testing.T) {
	writeDesktopSplitColoDictionaryForTest(t)

	for _, tc := range []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "ipv4 only uses ipv4 dictionary",
			content: "104.0.0.0/8",
			want:    []string{"104.16.0.0", "104.16.0.1", "104.16.0.2", "104.16.0.3"},
		},
		{
			name:    "ipv6 only uses ipv6 dictionary",
			content: "2400:cb00::/32",
			want:    []string{"2400:cb00::", "2400:cb00::1", "2400:cb00::2", "2400:cb00::3"},
		},
		{
			name:    "mixed input uses comprehensive dictionary",
			content: "104.0.0.0/8\n2400:cb00::/32",
			want: []string{
				"104.24.0.0", "104.24.0.1", "104.24.0.2", "104.24.0.3",
				"2400:cb00:ffff::", "2400:cb00:ffff::1", "2400:cb00:ffff::2", "2400:cb00:ffff::3",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := DesktopSource{
				ColoFilter: "SJC",
				Content:    tc.content,
				IPLimit:    20,
				IPMode:     "traverse",
				Kind:       "inline",
				Name:       tc.name,
			}
			entries, _, _, err := buildDesktopSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
			if err != nil {
				t.Fatalf("buildDesktopSourceEntriesWithConfig returned error: %v", err)
			}
			if !reflect.DeepEqual(entries, tc.want) {
				t.Fatalf("entries = %#v, want %#v", entries, tc.want)
			}
		})
	}
}

func TestDesktopSourceColoFilterRequiresColoFile(t *testing.T) {
	configDir := configureDesktopConfigDirForTest(t)
	if err := os.WriteFile(filepath.Join(configDir, colodict.GeofeedFileName), []byte("ip_prefix,country,region,city,postal_code\n104.16.0.0/13,US,CA,San Jose,\n"), 0o600); err != nil {
		t.Fatalf("write geofeed file: %v", err)
	}

	source := DesktopSource{
		ColoFilter: "SJC",
		Content:    "104.16.0.1",
		IPLimit:    10,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "missing-colo",
	}
	_, _, _, err := buildDesktopSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err == nil || !strings.Contains(err.Error(), "COLO 文件不存在") {
		t.Fatalf("err = %v, want missing COLO file error", err)
	}
}

func TestDesktopSourceStage2DefersColoFilter(t *testing.T) {
	writeDesktopColoDictionaryForTest(t)

	cfg := defaultProbeConfig()
	cfg.SourceColoFilterPhase = sourceColoFilterPhaseStage2
	source := DesktopSource{
		ColoFilter: "SJC",
		Content:    "104.16.0.1\n104.20.0.1",
		IPLimit:    10,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "stage2",
	}

	entries, warnings, invalid, err := buildDesktopSourceEntriesWithConfig(source.Content, source, cfg)
	if err != nil {
		t.Fatalf("buildDesktopSourceEntriesWithConfig returned error: %v", err)
	}
	if invalid != 0 {
		t.Fatalf("invalid = %d, want 0", invalid)
	}
	want := []string{"104.16.0.1", "104.20.0.1"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want unfiltered candidates %#v", entries, want)
	}
	if !warningsContain(warnings, "第二阶段起效") {
		t.Fatalf("warnings = %#v, want stage2 warning", warnings)
	}
}

func TestDesktopSourceStage2RequiresColoFile(t *testing.T) {
	configDir := configureDesktopConfigDirForTest(t)
	if err := os.WriteFile(filepath.Join(configDir, colodict.GeofeedFileName), []byte("ip_prefix,country,region,city,postal_code\n104.16.0.0/13,US,CA,San Jose,\n"), 0o600); err != nil {
		t.Fatalf("write geofeed file: %v", err)
	}

	cfg := defaultProbeConfig()
	cfg.SourceColoFilterPhase = sourceColoFilterPhaseStage2
	source := DesktopSource{
		ColoFilter: "SJC",
		Content:    "104.16.0.1",
		IPLimit:    10,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "stage2-missing-colo",
	}

	_, _, _, err := buildDesktopSourceEntriesWithConfig(source.Content, source, cfg)
	if err == nil || !strings.Contains(err.Error(), "COLO 文件不存在") {
		t.Fatalf("err = %v, want missing COLO file error in stage2 mode", err)
	}
}

func TestPrepareDesktopSourcesStage2BuildsPassAnySourceColoFilters(t *testing.T) {
	writeDesktopColoDictionaryForTest(t)

	cfg := defaultProbeConfig()
	cfg.SourceColoFilterPhase = sourceColoFilterPhaseStage2
	prepared := prepareDesktopSources(cfg, []DesktopSource{
		{
			ColoFilter: "SJC",
			Content:    "104.16.0.1\n104.20.0.1",
			Enabled:    true,
			IPLimit:    10,
			IPMode:     "traverse",
			Kind:       "inline",
			Name:       "sjc",
		},
		{
			ColoFilter: "LAX",
			Content:    "104.16.0.1",
			Enabled:    true,
			IPLimit:    10,
			IPMode:     "traverse",
			Kind:       "inline",
			Name:       "lax",
		},
		{
			Content: "104.20.0.1",
			Enabled: true,
			IPLimit: 10,
			IPMode:  "traverse",
			Kind:    "inline",
			Name:    "unrestricted",
		},
	})

	if prepared.SourceColoFilters == nil {
		t.Fatal("SourceColoFilters = nil, want stage2 source filter map")
	}
	filter := prepared.SourceColoFilters["104.16.0.1"]
	if filter.Unrestricted || len(filter.Allowed) != 2 {
		t.Fatalf("filter for duplicate allowlisted IP = %#v, want SJC/LAX pass-any", filter)
	}
	if _, ok := filter.Allowed["SJC"]; !ok {
		t.Fatalf("filter for 104.16.0.1 = %#v, missing SJC", filter)
	}
	if _, ok := filter.Allowed["LAX"]; !ok {
		t.Fatalf("filter for 104.16.0.1 = %#v, missing LAX", filter)
	}
	if filter := prepared.SourceColoFilters["104.20.0.1"]; !filter.Unrestricted {
		t.Fatalf("filter for unrestricted duplicate IP = %#v, want unrestricted", filter)
	}
	if !warningsContain(prepared.Warnings, "第二阶段起效") {
		t.Fatalf("warnings = %#v, want stage2 warning", prepared.Warnings)
	}
}

func TestRunDesktopProbeFailsWhenAnySourceRequiresMissingColoFile(t *testing.T) {
	configDir := configureDesktopConfigDirForTest(t)
	if err := os.WriteFile(filepath.Join(configDir, colodict.GeofeedFileName), []byte("ip_prefix,country,region,city,postal_code\n104.16.0.0/13,US,CA,San Jose,\n"), 0o600); err != nil {
		t.Fatalf("write geofeed file: %v", err)
	}

	app := NewApp()
	_, err := app.RunDesktopProbe(DesktopProbePayload{
		Config: map[string]any{
			"probe": map[string]any{
				"source_colo_filter_phase": sourceColoFilterPhaseStage2,
			},
		},
		Sources: []DesktopSource{
			{
				ColoFilter: "SJC",
				Content:    "104.16.0.1",
				Enabled:    true,
				IPLimit:    10,
				IPMode:     "traverse",
				Kind:       "inline",
				Name:       "missing-colo",
			},
			{
				Content: "1.1.1.1",
				Enabled: true,
				IPLimit: 10,
				IPMode:  "traverse",
				Kind:    "inline",
				Name:    "fallback-source",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "COLO 文件不存在") {
		t.Fatalf("err = %v, want startup failure for missing COLO file", err)
	}
}

func TestDefaultDesktopSourceIPLimitIsFiveHundred(t *testing.T) {
	snapshot := defaultDesktopConfigSnapshot()
	sources, ok := snapshot["sources"].([]map[string]any)
	if !ok || len(sources) != 1 {
		t.Fatalf("sources = %#v, want one default source", snapshot["sources"])
	}
	if got := intValue(sources[0]["ip_limit"], 0); got != defaultDesktopSourceIPLimit {
		t.Fatalf("default ip_limit = %d, want %d", got, defaultDesktopSourceIPLimit)
	}
}

func warningsContain(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}

func desktopConfigSnapshotForTest(cfg ProbeConfig) map[string]any {
	return map[string]any{
		"export": map[string]any{
			"overwrite": "replace_on_start",
		},
		"probe": map[string]any{
			"concurrency": map[string]any{
				"stage1": cfg.Routines,
				"stage2": cfg.HeadRoutines,
				"stage3": cfg.Stage3Concurrency,
			},
			"debug_log_format":                  cfg.DebugLogFormat,
			"debug_log_mode":                    cfg.DebugLogMode,
			"debug_log_verbosity":               cfg.DebugLogVerbosity,
			"download_speed_sample_interval_ms": cfg.DownloadSpeedSampleIntervalMS,
			"download_time_seconds":             cfg.DownloadTimeSeconds,
			"download_warmup_seconds":           cfg.DownloadWarmupSeconds,
			"event_throttle_ms":                 cfg.EventThrottleMS,
			"ping_times":                        cfg.PingTimes,
			"strategy":                          cfg.Strategy,
			"tcp_port":                          cfg.TCPPort,
			"trace_url":                         cfg.TraceURL,
			"url":                               cfg.URL,
			"user_agent":                        cfg.UserAgent,
			"thresholds": map[string]any{
				"max_tcp_latency_ms": cfg.MaxDelayMS,
				"min_download_mbps":  cfg.MinSpeedMB,
			},
			"timeouts": map[string]any{
				"stage1_ms": cfg.Stage1TimeoutMS,
				"stage2_ms": cfg.Stage2TimeoutMS,
				"stage3_ms": cfg.DownloadTimeSeconds * 1000,
			},
		},
	}
}

func configureDesktopConfigDirForTest(t *testing.T) string {
	t.Helper()
	baseDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", baseDir)
	configDir := filepath.Join(baseDir, "CFST-GUI")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", configDir, err)
	}
	return configDir
}

func writeDesktopColoDictionaryForTest(t *testing.T) string {
	t.Helper()
	configDir := configureDesktopConfigDirForTest(t)
	path := filepath.Join(configDir, colodict.ColoFileName)
	raw := strings.Join([]string{
		"ip_prefix,colo,country,region,city",
		"104.16.0.0/30,SJC,US,CA,San Jose",
		"104.20.0.0/30,LAX,US,CA,Los Angeles",
	}, "\n")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	if err := os.WriteFile(filepath.Join(configDir, colodict.ColoIPv4FileName), []byte(raw), 0o600); err != nil {
		t.Fatalf("write desktop IPv4 colo file: %v", err)
	}
	emptyIPv6Raw, err := colodict.EncodeColoEntries(nil)
	if err != nil {
		t.Fatalf("EncodeColoEntries(empty): %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, colodict.ColoIPv6FileName), emptyIPv6Raw, 0o600); err != nil {
		t.Fatalf("write desktop IPv6 colo file: %v", err)
	}
	return path
}

func writeDesktopSplitColoDictionaryForTest(t *testing.T) string {
	t.Helper()
	configDir := configureDesktopConfigDirForTest(t)
	files := map[string]string{
		colodict.ColoFileName: strings.Join([]string{
			"ip_prefix,colo,country,region,city",
			"104.24.0.0/30,SJC,US,CA,San Jose",
			"2400:cb00:ffff::/126,SJC,US,CA,San Jose",
		}, "\n"),
		colodict.ColoIPv4FileName: strings.Join([]string{
			"ip_prefix,colo,country,region,city",
			"104.16.0.0/30,SJC,US,CA,San Jose",
		}, "\n"),
		colodict.ColoIPv6FileName: strings.Join([]string{
			"ip_prefix,colo,country,region,city",
			"2400:cb00::/126,SJC,US,CA,San Jose",
		}, "\n"),
	}
	for name, raw := range files {
		path := filepath.Join(configDir, name)
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}
	return configDir
}

func parseTestIP(value string) *net.IPAddr {
	return &net.IPAddr{IP: net.ParseIP(value)}
}

func weightedResultTestData() []utils.CloudflareIPData {
	return []utils.CloudflareIPData{
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   4,
				Received: 4,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 1 * 1024 * 1024,
		},
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.2"),
				Sended:   4,
				Received: 4,
				Delay:    50 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		},
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.3"),
				Sended:   4,
				Received: 4,
				Delay:    5 * time.Millisecond,
			},
			DownloadSpeed: 512 * 1024,
		},
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.4"),
				Sended:   4,
				Received: 4,
				Delay:    100 * time.Millisecond,
			},
			DownloadSpeed: 100 * 1024 * 1024,
		},
	}
}

func TestRunProbeStagePlanFastAndFull(t *testing.T) {
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
	})

	sample := utils.PingDelaySet{
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
		},
	}

	for _, tc := range []struct {
		name               string
		strategy           string
		disableDownload    bool
		expectedStageCalls []string
	}{
		{
			name:               "fast",
			strategy:           "fast",
			disableDownload:    true,
			expectedStageCalls: []string{"tcp", "trace"},
		},
		{
			name:               "full",
			strategy:           "full",
			disableDownload:    false,
			expectedStageCalls: []string{"tcp", "trace", "get"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := make([]string, 0, 3)
			desktopTCPProbeRunner = func() utils.PingDelaySet {
				calls = append(calls, "tcp")
				return sample
			}
			desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
				calls = append(calls, "trace")
				return input
			}
			desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
				calls = append(calls, "get")
				return utils.DownloadSpeedSet(input)
			}

			cfg := defaultProbeConfig()
			cfg.Strategy = tc.strategy
			cfg.DisableDownload = tc.disableDownload
			cfg.WriteOutput = false

			app := NewApp()
			_, err := app.runProbe(ProbeRequest{
				Config:     cfg,
				SourceText: "1.1.1.1",
			}, nil)
			if err != nil {
				t.Fatalf("runProbe returned error: %v", err)
			}
			if !reflect.DeepEqual(calls, tc.expectedStageCalls) {
				t.Fatalf("stage calls = %v, want %v", calls, tc.expectedStageCalls)
			}
		})
	}
}

func TestRunProbePrintNumLimitsFinalResultsAndCSV(t *testing.T) {
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
	})

	weightedData := weightedResultTestData()
	desktopTCPProbeRunner = func() utils.PingDelaySet {
		return utils.PingDelaySet(weightedData)
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(weightedData)
	}

	outputPath := filepath.Join(t.TempDir(), "result.csv")
	cfg := defaultProbeConfig()
	cfg.Strategy = "full"
	cfg.DisableDownload = false
	cfg.OutputFile = outputPath
	cfg.PrintNum = 2
	cfg.Stage3Limit = len(weightedData)
	cfg.TestCount = len(weightedData)

	app := NewApp()
	result, err := app.runProbe(ProbeRequest{
		Config:     cfg,
		SourceText: "1.1.1.1\n1.1.1.2\n1.1.1.3\n1.1.1.4",
	}, nil)
	if err != nil {
		t.Fatalf("runProbe returned error: %v", err)
	}
	if len(result.Results) != 2 {
		t.Fatalf("result count = %d, want 2", len(result.Results))
	}
	if result.Results[0].IP != "1.1.1.4" || result.Results[1].IP != "1.1.1.3" {
		t.Fatalf("result order = %s,%s; want weighted top 1.1.1.4,1.1.1.3", result.Results[0].IP, result.Results[1].IP)
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("csv line count = %d, want header + 2 rows; body=%q", len(lines), string(raw))
	}
	if !strings.HasPrefix(lines[1], "1.1.1.4,") || !strings.HasPrefix(lines[2], "1.1.1.3,") {
		t.Fatalf("csv rows = %q, %q; want weighted top rows", lines[1], lines[2])
	}
}

func TestLimitFinalCloudflareResultsUnlimitedKeepsOrder(t *testing.T) {
	data := weightedResultTestData()
	selected := limitFinalCloudflareResults(data, 0)
	if len(selected) != len(data) {
		t.Fatalf("selected count = %d, want %d", len(selected), len(data))
	}
	if selected[0].IP.String() != "1.1.1.1" || selected[1].IP.String() != "1.1.1.2" {
		t.Fatalf("selected order = %s,%s; want original order for unlimited", selected[0].IP, selected[1].IP)
	}
}

func TestLimitFinalCloudflareResultsCanUseMaxSpeed(t *testing.T) {
	data := []utils.CloudflareIPData{
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   4,
				Received: 4,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed:    5 * 1024 * 1024,
			MaxDownloadSpeed: 100 * 1024 * 1024,
		},
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.2"),
				Sended:   4,
				Received: 4,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed:    50 * 1024 * 1024,
			MaxDownloadSpeed: 10 * 1024 * 1024,
		},
	}

	averageSelected := limitFinalCloudflareResults(data, 1, utils.DownloadSpeedMetricAverage)
	if averageSelected[0].IP.String() != "1.1.1.2" {
		t.Fatalf("average selected = %s, want 1.1.1.2", averageSelected[0].IP)
	}
	maxSelected := limitFinalCloudflareResults(data, 1, utils.DownloadSpeedMetricMax)
	if maxSelected[0].IP.String() != "1.1.1.1" {
		t.Fatalf("max selected = %s, want 1.1.1.1", maxSelected[0].IP)
	}
}

func TestRunProbeFullLimitsStage3CandidatesAndDoesNotFallbackOnDownloadFailure(t *testing.T) {
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
	})

	sample := utils.PingDelaySet{
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
		},
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.2"),
				Sended:   3,
				Received: 3,
				Delay:    20 * time.Millisecond,
			},
		},
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.3"),
				Sended:   3,
				Received: 3,
				Delay:    30 * time.Millisecond,
			},
		},
	}
	downloadInputCount := 0
	desktopTCPProbeRunner = func() utils.PingDelaySet {
		return sample
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		downloadInputCount = len(input)
		return utils.DownloadSpeedSet{}
	}

	cfg := defaultProbeConfig()
	cfg.Strategy = "full"
	cfg.DisableDownload = false
	cfg.TestCount = 1
	cfg.Stage3Limit = 1
	cfg.WriteOutput = false

	app := NewApp()
	result, err := app.runProbe(ProbeRequest{
		Config:     cfg,
		SourceText: "1.1.1.1\n1.1.1.2\n1.1.1.3",
	}, nil)
	if err != nil {
		t.Fatalf("runProbe returned error: %v", err)
	}
	if downloadInputCount != 1 {
		t.Fatalf("download input count = %d, want stage3 limit 1", downloadInputCount)
	}
	if len(result.Results) != 0 {
		t.Fatalf("result count = %d, want 0 without fallback to trace candidates", len(result.Results))
	}
	if result.Summary.Passed != 0 || result.Summary.Failed != 1 {
		t.Fatalf("summary = %#v, want 0 passed and 1 failed", result.Summary)
	}
}

func TestListResultFileReadsCSVRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.csv")
	body := "IP 地址,已发送,已接收,丢包率,TCP延迟(ms),平均速率(MB/s),最高速率(MB/s),地区码\n1.1.1.1,4,4,0.00,12.34,56.78,78.90,HKG\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	app := NewApp()
	result := app.ListResultFile(map[string]any{"path": path, "task_id": "csv-task"})
	if !result.OK {
		t.Fatalf("ListResultFile = %#v, want ok", result)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map", result.Data)
	}
	rows, ok := data["results"].([]ProbeResultRow)
	if !ok || len(rows) != 1 {
		t.Fatalf("rows = %#v, want one ProbeResultRow", data["results"])
	}
	if rows[0].Address != "1.1.1.1" || rows[0].TCPLatencyMS == nil || *rows[0].TCPLatencyMS != 12.34 {
		t.Fatalf("row = %#v, want parsed values", rows[0])
	}
	if rows[0].DownloadMbps == nil || *rows[0].DownloadMbps != 56.78 || rows[0].MaxDownloadMbps == nil || *rows[0].MaxDownloadMbps != 78.90 {
		t.Fatalf("download speeds = avg %v max %v, want 56.78/78.90", rows[0].DownloadMbps, rows[0].MaxDownloadMbps)
	}
}

func TestDesktopProbePauseAndResumeControlsRunningTask(t *testing.T) {
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
	})

	sample := utils.PingDelaySet{
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
		},
	}
	tcpEntered := make(chan struct{})
	allowCheckpoint := make(chan struct{})
	desktopTCPProbeRunner = func() utils.PingDelaySet {
		close(tcpEntered)
		<-allowCheckpoint
		task.CheckProbePause("stage1_tcp", "1.1.1.1")
		return sample
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	app := NewApp()
	cfg := defaultProbeConfig()
	cfg.WriteOutput = false
	taskID := "pause-task"
	done := make(chan error, 1)

	go func() {
		_, err := app.RunDesktopProbe(DesktopProbePayload{
			Config:  desktopConfigSnapshotForTest(cfg),
			Sources: []DesktopSource{{Content: "1.1.1.1", Enabled: true, ID: "source-1", Kind: "inline", Name: "inline", IPMode: "traverse"}},
			TaskID:  taskID,
		})
		done <- err
	}()

	select {
	case <-tcpEntered:
	case err := <-done:
		t.Fatalf("runProbe finished before pause: %v", err)
	case <-time.After(time.Second):
		t.Fatal("runProbe did not enter TCP stage")
	}
	pauseResult := app.CancelProbe(map[string]any{"task_id": taskID})
	if !pauseResult.OK {
		t.Fatalf("CancelProbe = %#v, want ok", pauseResult)
	}
	close(allowCheckpoint)
	select {
	case err := <-done:
		t.Fatalf("runProbe finished while paused: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	_, secondErr := app.RunDesktopProbe(DesktopProbePayload{
		Config:  desktopConfigSnapshotForTest(cfg),
		Sources: []DesktopSource{{Content: "1.1.1.2", Enabled: true, ID: "source-2", Kind: "inline", Name: "inline-2", IPMode: "traverse"}},
		TaskID:  "second-task",
	})
	if secondErr == nil || !strings.Contains(secondErr.Error(), probeAlreadyRunningMessage) {
		t.Fatalf("second RunDesktopProbe error = %v, want already-running error", secondErr)
	}
	wrongResume := app.ResumeProbe(map[string]any{"task_id": "other-task"})
	if wrongResume.OK {
		t.Fatalf("ResumeProbe with wrong task = %#v, want failure", wrongResume)
	}
	resumeResult := app.ResumeProbe(map[string]any{"task_id": taskID})
	if !resumeResult.OK {
		t.Fatalf("ResumeProbe = %#v, want ok", resumeResult)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runProbe returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runProbe did not finish after resume")
	}
}

func TestRunProbeDebugLogStagesFastAndFull(t *testing.T) {
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	oldDebug := utils.Debug
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
		utils.Debug = oldDebug
		_ = utils.CloseDebugLog()
	})

	sample := utils.PingDelaySet{
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
		},
	}
	desktopTCPProbeRunner = func() utils.PingDelaySet {
		return sample
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	for _, tc := range []struct {
		name            string
		disableDownload bool
		wantStage3      bool
	}{
		{name: "fast", disableDownload: true},
		{name: "full", disableDownload: false, wantStage3: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := defaultProbeConfig()
			cfg.Debug = true
			cfg.DisableDownload = tc.disableDownload
			cfg.WriteOutput = false
			if tc.wantStage3 {
				cfg.Strategy = "full"
			}
			taskID := "task-" + tc.name

			app := NewApp()
			_, err := app.runProbe(ProbeRequest{
				Config:     cfg,
				SourceText: "1.1.1.1",
				TaskID:     taskID,
			}, nil)
			if err != nil {
				t.Fatalf("runProbe returned error: %v", err)
			}

			entries := readDebugLogEntries(t, debugLogFilePath())
			events := make(map[string]int)
			stages := make(map[string]int)
			for _, entry := range entries {
				if entry["task_id"] != taskID {
					t.Fatalf("task_id = %v, want %s in entry %#v", entry["task_id"], taskID, entry)
				}
				events[stringValue(entry["event"], "")]++
				if stage := stringValue(entry["stage"], ""); stage != "" {
					stages[stage]++
				}
			}
			for _, event := range []string{"probe.start", "stage.start", "stage.complete", "probe.complete"} {
				if events[event] == 0 {
					t.Fatalf("missing debug event %s in %#v", event, events)
				}
			}
			for _, stage := range []string{"stage0_pool", "stage1_tcp", "stage2_trace"} {
				if stages[stage] == 0 {
					t.Fatalf("missing debug stage %s in %#v", stage, stages)
				}
			}
			if gotStage3 := stages["stage3_get"] > 0; gotStage3 != tc.wantStage3 {
				t.Fatalf("stage3 logged = %v, want %v; stages=%#v", gotStage3, tc.wantStage3, stages)
			}
		})
	}
}

func TestRunProbeDebugLogSimpleVerbosityOmitsStageStart(t *testing.T) {
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	oldDebug := utils.Debug
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
		utils.Debug = oldDebug
		_ = utils.CloseDebugLog()
	})

	sample := utils.PingDelaySet{
		{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
		},
	}
	desktopTCPProbeRunner = func() utils.PingDelaySet {
		return sample
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultProbeConfig()
	cfg.Debug = true
	cfg.DebugLogVerbosity = utils.DebugLogVerbositySimple
	cfg.DisableDownload = true
	cfg.WriteOutput = false
	taskID := "task-simple-verbosity"

	app := NewApp()
	_, err := app.runProbe(ProbeRequest{
		Config:     cfg,
		SourceText: "1.1.1.1",
		TaskID:     taskID,
	}, nil)
	if err != nil {
		t.Fatalf("runProbe returned error: %v", err)
	}

	entries := readDebugLogEntries(t, debugLogFilePath())
	events := make(map[string]int)
	for _, entry := range entries {
		events[stringValue(entry["event"], "")]++
	}
	for _, event := range []string{"probe.start", "stage.complete", "probe.complete"} {
		if events[event] == 0 {
			t.Fatalf("missing debug event %s in %#v", event, events)
		}
	}
	for _, event := range []string{"stage.start", "stage.detail"} {
		if events[event] != 0 {
			t.Fatalf("unexpected debug event %s in %#v", event, events)
		}
	}
}

func readDebugLogEntries(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("debug log line is not JSON: %v\n%s", err, line)
		}
		entries = append(entries, entry)
	}
	return entries
}
