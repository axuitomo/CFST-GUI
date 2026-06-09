package mobileapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

type mobileResolverForTest map[string][]string

func (resolver mobileResolverForTest) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
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

func parseMobileTestIP(value string) *net.IPAddr {
	return &net.IPAddr{IP: net.ParseIP(value)}
}

func mobileCSVFloatPtrForTest(value string) *float64 {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "N/A") {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return nil
	}
	return &parsed
}

func mobileWeightedResultTestData() []utils.CloudflareIPData {
	return []utils.CloudflareIPData{
		{
			PingData: &utils.PingData{
				IP:       &net.IPAddr{IP: net.ParseIP("1.1.1.1")},
				Sended:   4,
				Received: 4,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 1 * 1024 * 1024,
		},
		{
			PingData: &utils.PingData{
				IP:       &net.IPAddr{IP: net.ParseIP("1.1.1.2")},
				Sended:   4,
				Received: 4,
				Delay:    50 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		},
		{
			PingData: &utils.PingData{
				IP:       &net.IPAddr{IP: net.ParseIP("1.1.1.3")},
				Sended:   4,
				Received: 4,
				Delay:    5 * time.Millisecond,
			},
			DownloadSpeed: 512 * 1024,
		},
		{
			PingData: &utils.PingData{
				IP:       &net.IPAddr{IP: net.ParseIP("1.1.1.4")},
				Sended:   4,
				Received: 4,
				Delay:    100 * time.Millisecond,
			},
			DownloadSpeed: 100 * 1024 * 1024,
		},
	}
}

func TestMobileCSVFloatPtrAllowsZero(t *testing.T) {
	got := mobileCSVFloatPtrForTest("0")
	if got == nil || *got != 0 {
		t.Fatalf("mobileCSVFloatPtrForTest(0) = %v, want pointer to 0", got)
	}
	if got := mobileCSVFloatPtrForTest("-0.1"); got != nil {
		t.Fatalf("mobileCSVFloatPtrForTest(-0.1) = %v, want nil", *got)
	}
}

func TestMobileConfigCSVEncodingNormalizes(t *testing.T) {
	cfg, warnings := configToProbeConfig(map[string]any{
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

	cfg, warnings = configToProbeConfig(map[string]any{
		"export": map[string]any{
			"csv_encoding": "gbk",
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

func TestReadMobileProbeResultRowsFromCSVHandlesBOMHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.csv")
	raw := "\xEF\xBB\xBFIP 地址,已发送,已接收,丢包率,TCP延迟(ms),平均速率(MB/s),最高速率(MB/s),地区码,追踪延迟(ms)\n1.1.1.1,3,3,0.00,12.34,56.78,78.90,HKG,34.56\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	rows, err := readMobileProbeResultRowsFromCSV(path)
	if err != nil {
		t.Fatalf("readMobileProbeResultRowsFromCSV returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Address != "1.1.1.1" {
		t.Fatalf("rows = %#v, want one parsed row", rows)
	}
}

func TestMobileConvertProbeRowRoundsResultMetricsToTwoDecimals(t *testing.T) {
	row := probecore.ConvertProbeRow(utils.CloudflareIPData{
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
	}, 0, 443)

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

func TestMobileSummarizeProbeRowsRoundsAverageDelayToTwoDecimals(t *testing.T) {
	summary := probecore.SummarizeProbeRows([]probeRow{
		{DelayMS: 10.121, DownloadSpeedMB: 1, IP: "1.1.1.1"},
		{DelayMS: 10.123, DownloadSpeedMB: 2, IP: "1.1.1.2"},
	}, 2)

	if summary.AverageDelayMS != 10.12 {
		t.Fatalf("AverageDelayMS = %v, want 10.12", summary.AverageDelayMS)
	}
}

type mobileResolverForTestFunc func(context.Context, string) ([]net.IPAddr, error)

func (fn mobileResolverForTestFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return fn(ctx, host)
}

type mobileRoundTripFunc func(req *http.Request) (*http.Response, error)

func (fn mobileRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestServiceConfigRoundTripUsesMobilePrivatePath(t *testing.T) {
	service := NewService()
	baseDir := t.TempDir()
	decodeCommandForTest(t, service.Init(baseDir))

	load := decodeCommandForTest(t, service.LoadConfig())
	if !boolValue(load["ok"], false) {
		t.Fatalf("load default failed: %#v", load)
	}
	data := mapValue(load["data"])
	if got := stringValue(data["configPath"], ""); got != filepath.Join(baseDir, "mobile-config.json") {
		t.Fatalf("configPath = %q", got)
	}

	snapshot := mapValue(data["config_snapshot"])
	probe := mapValue(snapshot["probe"])
	if got := stringValue(probe["url"], ""); got != defaultFileTestURL {
		t.Fatalf("default probe url = %q, want %q", got, defaultFileTestURL)
	}
	if got := floatValue(probe["max_loss_rate"], 0); got != float64(utils.DefaultMaxLossRate) {
		t.Fatalf("default max_loss_rate = %.2f, want %.2f", got, utils.DefaultMaxLossRate)
	}
	if got := intValue(probe["httping_status_code"], -1); got != 0 {
		t.Fatalf("default httping_status_code = %d, want 0", got)
	}
	if got := intValue(probe["download_warmup_seconds"], -1); got != 5 {
		t.Fatalf("default download_warmup_seconds = %d, want 5", got)
	}
	sources, ok := snapshot["sources"].([]any)
	if !ok || len(sources) != 1 {
		t.Fatalf("default sources = %#v, want one default source", snapshot["sources"])
	}
	if got := intValue(mapValue(sources[0])["ip_limit"], 0); got != defaultMobileSourceIPLimit {
		t.Fatalf("default ip_limit = %d, want %d", got, defaultMobileSourceIPLimit)
	}
	probe["tcp_port"] = 70000
	probe["max_loss_rate"] = 1
	probe["download_warmup_seconds"] = 0
	savePayload := encodeJSON(map[string]any{"config_snapshot": snapshot})
	save := decodeCommandForTest(t, service.SaveConfig(savePayload))
	if !boolValue(save["ok"], false) {
		t.Fatalf("save failed: %#v", save)
	}
	warnings := stringSliceForTest(save["warnings"])
	if !containsForTest(warnings, "测速端口必须在 1-65535") {
		t.Fatalf("warnings = %#v, missing port clamp", warnings)
	}
	if containsForTest(warnings, "TCP 丢包率上限最大支持") {
		t.Fatalf("warnings = %#v, did not expect loss rate clamp", warnings)
	}
}

func TestMCISEngineConfigIgnoresFinalColoFilter(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.HttpingCFColo = "hkg,nrt LAX hkg zzz"

	mcisCfg := buildMCISEngineConfig(cfg, 500)

	if len(mcisCfg.ColoAllow) != 0 {
		t.Fatalf("ColoAllow = %#v, want empty because final COLO filter belongs to stage 2 only", mcisCfg.ColoAllow)
	}
}

func TestMobileConfigDebugCaptureEnabledCompatibility(t *testing.T) {
	cfg, _ := configToProbeConfig(map[string]any{
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

	cfg, _ = configToProbeConfig(map[string]any{
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

func TestMCISProbeConfigOnlySetsDebugDialAddressWhenConfigured(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.Debug = true
	cfg.DebugCaptureAddress = ""

	probeCfg, _ := buildMCISProbeConfig(cfg)
	if probeCfg.DialAddress != "" {
		t.Fatalf("DialAddress = %q, want direct connection when debug capture address is empty", probeCfg.DialAddress)
	}

	cfg.DebugCaptureAddress = "9000"
	cfg.DebugCaptureEnabled = true
	probeCfg, _ = buildMCISProbeConfig(cfg)
	if probeCfg.DialAddress != "127.0.0.1:9000" {
		t.Fatalf("DialAddress = %q, want normalized debug capture address", probeCfg.DialAddress)
	}

	cfg.DebugCaptureEnabled = false
	probeCfg, _ = buildMCISProbeConfig(cfg)
	if probeCfg.DialAddress != "" {
		t.Fatalf("DialAddress = %q, want direct connection when debug capture is disabled", probeCfg.DialAddress)
	}
}

func TestNormalizeProbeConfigRejectsSinglePingTime(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.PingTimes = 1

	normalized, warnings := normalizeProbeConfig(cfg)
	if normalized.PingTimes != task.MinPingTimes {
		t.Fatalf("PingTimes = %d, want %d", normalized.PingTimes, task.MinPingTimes)
	}
	if !containsForTest(warnings, "TCP 发包次数必须至少为 2") {
		t.Fatalf("warnings = %#v, missing minimum ping times warning", warnings)
	}
}

func TestNormalizeProbeConfigDownloadSamplingAndTimingDefaults(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.DownloadSpeedSampleIntervalMS = 0
	cfg.DownloadTimeSeconds = 7
	cfg.DownloadWarmupSeconds = -1

	normalized, warnings := normalizeProbeConfig(cfg)
	if normalized.DownloadSpeedSampleIntervalMS != 500 {
		t.Fatalf("DownloadSpeedSampleIntervalMS = %d, want 500", normalized.DownloadSpeedSampleIntervalMS)
	}
	if normalized.DownloadTimeSeconds != 7 {
		t.Fatalf("DownloadTimeSeconds = %d, want 7", normalized.DownloadTimeSeconds)
	}
	if normalized.DownloadWarmupSeconds != 5 {
		t.Fatalf("DownloadWarmupSeconds = %d, want 5", normalized.DownloadWarmupSeconds)
	}
	if !containsForTest(warnings, "下载速度采样间隔必须大于 0") {
		t.Fatalf("warnings = %#v, missing sample interval warning", warnings)
	}
	if !containsForTest(warnings, "下载预热时间不能为负数") {
		t.Fatalf("warnings = %#v, missing download warmup warning", warnings)
	}

	cfg = defaultProbeConfig()
	cfg.DownloadTimeSeconds = 3
	cfg.DownloadWarmupSeconds = 0
	normalized, warnings = normalizeProbeConfig(cfg)
	if normalized.DownloadTimeSeconds != 3 {
		t.Fatalf("DownloadTimeSeconds = %d, want 3", normalized.DownloadTimeSeconds)
	}
	if normalized.DownloadWarmupSeconds != 0 {
		t.Fatalf("DownloadWarmupSeconds = %d, want 0", normalized.DownloadWarmupSeconds)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}

func TestConfigDownloadSamplingIntervalMSCompatibility(t *testing.T) {
	cfg, _ := configToProbeConfig(map[string]any{
		"probe": map[string]any{
			"download_speed_sample_interval_ms":      750,
			"download_speed_sample_interval_seconds": 9,
		},
	})
	if cfg.DownloadSpeedSampleIntervalMS != 750 {
		t.Fatalf("DownloadSpeedSampleIntervalMS = %d, want ms field priority 750", cfg.DownloadSpeedSampleIntervalMS)
	}

	cfg, _ = configToProbeConfig(map[string]any{
		"probe": map[string]any{
			"download_speed_sample_interval_seconds": 3,
		},
	})
	if cfg.DownloadSpeedSampleIntervalMS != 3000 {
		t.Fatalf("DownloadSpeedSampleIntervalMS = %d, want legacy seconds converted to 3000", cfg.DownloadSpeedSampleIntervalMS)
	}
}

func TestConfigDownloadHTTPFieldsNormalize(t *testing.T) {
	cfg, _ := configToProbeConfig(map[string]any{
		"probe": map[string]any{
			"downloadGetConcurrency": 8,
			"downloadBufferKB":       1024,
			"downloadHTTPProtocol":   "h2",
		},
	})
	normalized, warnings := normalizeProbeConfig(cfg)
	if normalized.DownloadGetConcurrency != 8 {
		t.Fatalf("DownloadGetConcurrency = %d, want 8", normalized.DownloadGetConcurrency)
	}
	if normalized.DownloadBufferKB != 1024 {
		t.Fatalf("DownloadBufferKB = %d, want 1024", normalized.DownloadBufferKB)
	}
	if normalized.DownloadHTTPProtocol != "h2" {
		t.Fatalf("DownloadHTTPProtocol = %q, want h2", normalized.DownloadHTTPProtocol)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}

	cfg.DownloadGetConcurrency = 0
	cfg.DownloadBufferKB = 99999
	cfg.DownloadHTTPProtocol = "bad"
	normalized, warnings = normalizeProbeConfig(cfg)
	if normalized.DownloadGetConcurrency != 4 {
		t.Fatalf("DownloadGetConcurrency = %d, want default 4", normalized.DownloadGetConcurrency)
	}
	if normalized.DownloadBufferKB != task.MaxDownloadBufferKB {
		t.Fatalf("DownloadBufferKB = %d, want max %d", normalized.DownloadBufferKB, task.MaxDownloadBufferKB)
	}
	if normalized.DownloadHTTPProtocol != "auto" {
		t.Fatalf("DownloadHTTPProtocol = %q, want auto", normalized.DownloadHTTPProtocol)
	}
	for _, want := range []string{"GET 分片并发必须大于 0", "下载缓冲最大支持", "未知下载 HTTP 协议"} {
		if !containsForTest(warnings, want) {
			t.Fatalf("warnings = %#v, missing %q", warnings, want)
		}
	}
}

func TestConfigToProbeConfigNormalizesRequestHeaders(t *testing.T) {
	cfg, warnings := configToProbeConfig(map[string]any{
		"probe": map[string]any{
			"requestHeaders": strings.Join([]string{
				"Accept: */*",
				"Connection: close",
				"X-Mobile: ok",
				"bad header: nope",
			}, "\n"),
		},
	})
	if cfg.RequestHeaders != "Accept: */*\nX-Mobile: ok" {
		t.Fatalf("RequestHeaders = %q, want normalized custom headers", cfg.RequestHeaders)
	}
	if len(warnings) < 2 {
		t.Fatalf("warnings = %#v, want reserved and invalid header warnings", warnings)
	}

	cfg, warnings = configToProbeConfig(map[string]any{
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

func TestConfigToProbeConfigNormalizesHTTPingStatusCode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		probe        map[string]any
		want         int
		wantWarnings bool
	}{
		{name: "default", probe: map[string]any{}, want: 0},
		{name: "zero unlimited", probe: map[string]any{"httping_status_code": 0}, want: 0},
		{name: "camel status", probe: map[string]any{"httpingStatusCode": 204}, want: 204},
		{name: "below range", probe: map[string]any{"httping_status_code": 99}, want: 0, wantWarnings: true},
		{name: "above range", probe: map[string]any{"httpingStatusCode": 600}, want: 0, wantWarnings: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, warnings := configToProbeConfig(map[string]any{"probe": tc.probe})
			if cfg.HttpingStatusCode != tc.want {
				t.Fatalf("HttpingStatusCode = %d, want %d", cfg.HttpingStatusCode, tc.want)
			}
			if got := containsForTest(warnings, "追踪有效状态码必须为 0 或 100-599"); got != tc.wantWarnings {
				t.Fatalf("warnings = %#v, contains status warning = %v, want %v", warnings, got, tc.wantWarnings)
			}
		})
	}
}

func TestConfigToProbeConfigMapsStage3Limit(t *testing.T) {
	cfg, _ := configToProbeConfig(map[string]any{
		"probe": map[string]any{
			"strategy":              "full",
			"print_num":             3,
			"download_speed_metric": "highest",
			"stage_limits": map[string]any{
				"stage2": 512,
				"stage3": 7,
			},
		},
	})
	if cfg.Stage3Limit != 7 {
		t.Fatalf("Stage3Limit = %d, want 7", cfg.Stage3Limit)
	}
	if cfg.TestCount != 7 {
		t.Fatalf("TestCount = %d, want legacy mirror 7", cfg.TestCount)
	}
	if cfg.PrintNum != 3 {
		t.Fatalf("PrintNum = %d, want 3", cfg.PrintNum)
	}
	if cfg.DownloadSpeedMetric != utils.DownloadSpeedMetricMax {
		t.Fatalf("DownloadSpeedMetric = %q, want max", cfg.DownloadSpeedMetric)
	}
}

func TestMobileLimitFinalCloudflareResultsUsesWeightedTopN(t *testing.T) {
	data := mobileWeightedResultTestData()
	selected := probecore.LimitFinalResults(data, 2)
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}
	if selected[0].IP.String() != "1.1.1.4" || selected[1].IP.String() != "1.1.1.3" {
		t.Fatalf("selected = %s,%s; want weighted top 1.1.1.4,1.1.1.3", selected[0].IP, selected[1].IP)
	}
	unlimited := probecore.LimitFinalResults(data, 0)
	if len(unlimited) != len(data) || unlimited[0].IP.String() != "1.1.1.1" {
		t.Fatalf("unlimited selection = %#v, want original order and count", unlimited)
	}
}

func TestMobileLimitFinalCloudflareResultsCanUseMaxSpeed(t *testing.T) {
	data := []utils.CloudflareIPData{
		{
			PingData: &utils.PingData{
				IP:       &net.IPAddr{IP: net.ParseIP("1.1.1.1")},
				Sended:   4,
				Received: 4,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed:    5 * 1024 * 1024,
			MaxDownloadSpeed: 100 * 1024 * 1024,
		},
		{
			PingData: &utils.PingData{
				IP:       &net.IPAddr{IP: net.ParseIP("1.1.1.2")},
				Sended:   4,
				Received: 4,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed:    50 * 1024 * 1024,
			MaxDownloadSpeed: 10 * 1024 * 1024,
		},
	}

	selected := probecore.LimitFinalResults(data, 1, utils.DownloadSpeedMetricMax)
	if selected[0].IP.String() != "1.1.1.1" {
		t.Fatalf("selected = %s, want max-speed top 1.1.1.1", selected[0].IP)
	}
}

func TestServiceListResultFileReadsCSVRows(t *testing.T) {
	service := NewService()
	dir := t.TempDir()
	decodeCommandForTest(t, service.Init(dir))
	path := filepath.Join(dir, "exports", "result.csv")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir exports: %v", err)
	}
	body := "IP 地址,已发送,已接收,丢包率,TCP延迟(ms),平均速率(MB/s),最高速率(MB/s),地区码\n1.1.1.1,4,4,0.00,12.34,56.78,78.90,HKG\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	result := decodeCommandForTest(t, service.ListResultFile(encodeJSON(map[string]any{
		"path":    path,
		"task_id": "csv-task",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("ListResultFile failed: %#v", result)
	}
	data := mapValue(result["data"])
	if intValue(data["count"], 0) != 1 {
		t.Fatalf("count = %#v, want 1", data["count"])
	}
}

func TestServiceListResultFileSupportsPaginationFromPersistedTaskResults(t *testing.T) {
	service := NewService()
	dir := t.TempDir()
	decodeCommandForTest(t, service.Init(dir))
	rows := []probeResultRow{
		{Address: "1.1.1.1", ExportStatus: "exported", StageStatus: "completed"},
		{Address: "1.1.1.2", ExportStatus: "exported", StageStatus: "completed"},
		{Address: "1.1.1.3", ExportStatus: "exported", StageStatus: "completed"},
	}
	if err := service.writeTaskResults("task-1", rows); err != nil {
		t.Fatalf("writeTaskResults: %v", err)
	}

	result := decodeCommandForTest(t, service.ListResultFile(encodeJSON(map[string]any{
		"limit":   2,
		"offset":  1,
		"task_id": "task-1",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("ListResultFile failed: %#v", result)
	}
	data := mapValue(result["data"])
	if got := intValue(data["count"], 0); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if got := intValue(data["total_count"], 0); got != 3 {
		t.Fatalf("total_count = %d, want 3", got)
	}
	if got := stringValue(data["source_kind"], ""); got != "persisted" {
		t.Fatalf("source_kind = %q, want persisted", got)
	}
	results, ok := data["results"].([]any)
	if !ok {
		t.Fatalf("results type = %T, want []any", data["results"])
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
}

func TestServiceListResultFileBackfillsEmptyPersistedResultsFromCSV(t *testing.T) {
	service := NewService()
	dir := t.TempDir()
	decodeCommandForTest(t, service.Init(dir))
	csvPath := filepath.Join(dir, "exports", "result.csv")
	if err := os.MkdirAll(filepath.Dir(csvPath), 0o755); err != nil {
		t.Fatalf("mkdir exports: %v", err)
	}
	body := "address,tcp_latency_ms,download_mbps,max_download_mbps,colo\n1.1.1.1,12.34,56.78,78.90,HKG\n"
	if err := os.WriteFile(csvPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := service.writeTaskResults("empty-task", []probeResultRow{}); err != nil {
		t.Fatalf("writeTaskResults: %v", err)
	}

	result := decodeCommandForTest(t, service.ListResultFile(encodeJSON(map[string]any{
		"path":    csvPath,
		"task_id": "empty-task",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("ListResultFile failed: %#v", result)
	}
	data := mapValue(result["data"])
	results, ok := data["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("results = %#v, want CSV row", data["results"])
	}
	if sourcePath := stringValue(data["source_path"], ""); sourcePath != csvPath {
		t.Fatalf("source_path = %q, want %q", sourcePath, csvPath)
	}
	if got := stringValue(data["source_kind"], ""); got != "csv" {
		t.Fatalf("source_kind = %q, want csv", got)
	}
}

func TestServiceListResultFileBackfillsMissingPersistedResultsFromSnapshotCSV(t *testing.T) {
	service := NewService()
	dir := t.TempDir()
	decodeCommandForTest(t, service.Init(dir))
	csvPath := filepath.Join(dir, "exports", "snapshot-result.csv")
	if err := os.MkdirAll(filepath.Dir(csvPath), 0o755); err != nil {
		t.Fatalf("mkdir exports: %v", err)
	}
	body := "address,tcp_latency_ms,download_mbps,max_download_mbps,colo\n1.1.1.2,10.00,20.00,30.00,NRT\n"
	if err := os.WriteFile(csvPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := service.writeTaskSnapshot(taskSnapshot{
		ExportRecord: &exportRecordSnapshot{
			FileName:     filepath.Base(csvPath),
			Format:       "csv",
			SourcePath:   csvPath,
			TargetDir:    filepath.Dir(csvPath),
			TaskID:       "snapshot-task",
			WrittenCount: 1,
		},
		Status: "completed",
		TaskID: "snapshot-task",
	}); err != nil {
		t.Fatalf("writeTaskSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.ListResultFile(encodeJSON(map[string]any{
		"path":    filepath.Join(dir, "missing.csv"),
		"task_id": "snapshot-task",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("ListResultFile failed: %#v", result)
	}
	data := mapValue(result["data"])
	results, ok := data["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("results = %#v, want snapshot CSV row", data["results"])
	}
}

func TestServiceLoadTaskSnapshotReadsPersistedSnapshot(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	if err := service.writeTaskSnapshot(taskSnapshot{
		CurrentStage: "stage2_trace",
		StartedAt:    nowRFC3339(),
		Status:       "running",
		TaskID:       "task-snapshot",
		UpdatedAt:    nowRFC3339(),
	}); err != nil {
		t.Fatalf("writeTaskSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.LoadTaskSnapshot(encodeJSON(map[string]any{
		"task_id": "task-snapshot",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("LoadTaskSnapshot failed: %#v", result)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["status"], ""); got != "failed" {
		t.Fatalf("status = %q, want failed", got)
	}
	if got := stringValue(data["current_stage"], ""); got != "stage2_trace" {
		t.Fatalf("current_stage = %q, want stage2_trace", got)
	}
	if got := stringValue(data["session_state"], ""); got != "persisted_only" {
		t.Fatalf("session_state = %q, want persisted_only", got)
	}
	if got := boolValue(data["runtime_attached"], false); got {
		t.Fatalf("runtime_attached = %v, want false", got)
	}
}

func TestServiceTaskSnapshotCacheKeepsOnlyRuntimeStates(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	if err := service.writeTaskSnapshot(taskSnapshot{
		CurrentStage: "stage1_tcp",
		StartedAt:    nowRFC3339(),
		Status:       "running",
		TaskID:       "active-task",
		UpdatedAt:    nowRFC3339(),
	}); err != nil {
		t.Fatalf("writeTaskSnapshot active-task: %v", err)
	}
	if err := service.writeTaskSnapshot(taskSnapshot{
		CurrentStage: "completed",
		SessionState: "persisted_only",
		StartedAt:    nowRFC3339(),
		Status:       "completed",
		TaskID:       "done-task",
		UpdatedAt:    nowRFC3339(),
	}); err != nil {
		t.Fatalf("writeTaskSnapshot done-task: %v", err)
	}

	if len(service.taskSnapshots) != 1 {
		t.Fatalf("taskSnapshots len = %d, want 1 runtime snapshot", len(service.taskSnapshots))
	}
	if _, ok := service.taskSnapshots["active-task"]; !ok {
		t.Fatalf("runtime snapshot missing from memory cache: %#v", service.taskSnapshots)
	}
	if _, ok := service.taskSnapshots["done-task"]; ok {
		t.Fatalf("terminal snapshot should not stay in memory cache: %#v", service.taskSnapshots)
	}

	snapshot, ok, err := service.loadTaskSnapshot("done-task")
	if err != nil {
		t.Fatalf("loadTaskSnapshot(done-task): %v", err)
	}
	if !ok {
		t.Fatal("loadTaskSnapshot(done-task) = not found, want persisted snapshot")
	}
	if snapshot.Status != "completed" {
		t.Fatalf("snapshot.Status = %q, want completed", snapshot.Status)
	}
	if _, ok := service.taskSnapshots["done-task"]; ok {
		t.Fatalf("terminal snapshot should not be re-cached after load: %#v", service.taskSnapshots)
	}
}

func TestServiceLoadTaskSnapshotKeepsActiveRuntimeState(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	service.currentTaskID = "active-task"
	if err := service.writeTaskSnapshot(taskSnapshot{
		CurrentStage: "stage1_tcp",
		StartedAt:    nowRFC3339(),
		Status:       "running",
		TaskID:       "active-task",
		UpdatedAt:    nowRFC3339(),
	}); err != nil {
		t.Fatalf("writeTaskSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.LoadTaskSnapshot(encodeJSON(map[string]any{
		"task_id": "active-task",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("LoadTaskSnapshot failed: %#v", result)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["status"], ""); got != "running" {
		t.Fatalf("status = %q, want running", got)
	}
	if got := stringValue(data["session_state"], ""); got != "active_runtime" {
		t.Fatalf("session_state = %q, want active_runtime", got)
	}
	if got := boolValue(data["runtime_attached"], false); !got {
		t.Fatalf("runtime_attached = %v, want true", got)
	}
}

func TestServicePipelineAPIsUnsupported(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	for name, response := range map[string]string{
		"LoadPipelineWorkspace": service.LoadPipelineWorkspace(),
		"StartPipeline":         service.StartPipeline(encodeJSON(map[string]any{})),
		"ListPipelineResults":   service.ListPipelineResults(encodeJSON(map[string]any{})),
		"GetPipelineSnapshot":   service.GetPipelineSnapshot(encodeJSON(map[string]any{})),
	} {
		result := decodeCommandForTest(t, response)
		if boolValue(result["ok"], true) || stringValue(result["code"], "") != "PIPELINE_UNSUPPORTED" {
			t.Fatalf("%s = %#v, want PIPELINE_UNSUPPORTED", name, result)
		}
	}
}

func TestTaskSnapshotFromCoolingRecordsSessionState(t *testing.T) {
	paused := taskSnapshotFromEvent("cooling-task", "probe.cooling", map[string]any{
		"recoverable": true,
	})
	if paused.SessionState != "paused_runtime" {
		t.Fatalf("paused SessionState = %q, want paused_runtime", paused.SessionState)
	}
	if !paused.ResumeCapable || !paused.RuntimeAttached {
		t.Fatalf("paused flags = resume:%v runtime:%v, want true/true", paused.ResumeCapable, paused.RuntimeAttached)
	}

	canceled := taskSnapshotFromEvent("cooling-task", "probe.cooling", map[string]any{
		"recoverable": false,
	})
	if canceled.SessionState != "idle" {
		t.Fatalf("canceled SessionState = %q, want idle", canceled.SessionState)
	}
	if canceled.ResumeCapable || canceled.RuntimeAttached {
		t.Fatalf("canceled flags = resume:%v runtime:%v, want false/false", canceled.ResumeCapable, canceled.RuntimeAttached)
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
	if containsForTest(warnings, "追踪 URL 无效") {
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
	if containsForTest(warnings, "追踪 URL 无法从文件测速URL派生") {
		t.Fatalf("warnings = %#v, should derive trace URL from escaped file URL", warnings)
	}
}

func TestServicePreviewSourceNormalizesInlineEntries(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	payload := map[string]any{
		"preview_limit": 2,
		"config":        defaultConfigSnapshot(),
		"source": map[string]any{
			"kind":     "inline",
			"content":  "1.1.1.1\n1.1.1.1\nbad\n1.0.0.1",
			"ip_limit": 8,
			"ip_mode":  "traverse",
			"name":     "test",
		},
	}
	result := decodeCommandForTest(t, service.PreviewSource(encodeJSON(payload)))
	if !boolValue(result["ok"], false) {
		t.Fatalf("preview failed: %#v", result)
	}
	data := mapValue(result["data"])
	entries := stringSliceForTest(data["preview_entries"])
	if len(entries) != 2 || entries[0] != "1.1.1.1" || entries[1] != "1.0.0.1" {
		t.Fatalf("entries = %#v", entries)
	}
	summary := mapValue(data["summary"])
	if intValue(summary["invalid_count"], 0) != 1 {
		t.Fatalf("invalid_count = %#v", summary["invalid_count"])
	}
}

func TestNormalizeMobileSourceURLInput(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "bare host", raw: "bestcf.pages.dev/xinyitang3/ipv4.txt", want: "https://bestcf.pages.dev/xinyitang3/ipv4.txt"},
		{name: "protocol relative", raw: "//bestcf.pages.dev/xinyitang3/ipv4.txt", want: "https://bestcf.pages.dev/xinyitang3/ipv4.txt"},
		{name: "https", raw: "https://example.com/ips.txt", want: "https://example.com/ips.txt"},
		{name: "http", raw: "http://example.com/ips.txt", want: "http://example.com/ips.txt"},
		{name: "empty", raw: " ", wantErr: "缺少远程 URL"},
		{name: "unsupported scheme", raw: "ftp://example.com/ips.txt", wantErr: "仅支持 http/https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMobileSourceURLInput(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeMobileSourceURLInput() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeMobileSourceURLInput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadSourceContentNormalizesBareURLBeforeGET(t *testing.T) {
	var requestedURL string
	client := &http.Client{Transport: mobileRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedURL = req.URL.String()
		if req.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", req.Method)
		}
		if req.URL.Scheme != "https" || req.URL.Host != "bestcf.pages.dev" || req.URL.Path != "/xinyitang3/ipv4.txt" {
			t.Fatalf("request URL = %s, want normalized BestCF URL", req.URL.String())
		}
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("103.44.255.30:443#HK | 103.44.255.30:443\n")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	raw, err := loadSourceContent(desktopSource{
		Kind: "url",
		URL:  "bestcf.pages.dev/xinyitang3/ipv4.txt",
	}, defaultProbeConfig(), client)
	if err != nil {
		t.Fatalf("loadSourceContent() error = %v", err)
	}
	if requestedURL != "https://bestcf.pages.dev/xinyitang3/ipv4.txt" {
		t.Fatalf("requestedURL = %q, want normalized https URL", requestedURL)
	}
	if !strings.Contains(raw, "103.44.255.30") {
		t.Fatalf("raw = %q, want response body", raw)
	}
}

func TestServicePreviewSourceParsesComplexInputAndResolvesDomain(t *testing.T) {
	oldResolver := sourceParseResolver
	sourceParseResolver = mobileResolverForTest(map[string][]string{
		"edge.example.com": {"203.0.113.20"},
	})
	t.Cleanup(func() { sourceParseResolver = oldResolver })

	service := NewService()
	baseDir := t.TempDir()
	decodeCommandForTest(t, service.Init(baseDir))

	payload := map[string]any{
		"preview_limit": 8,
		"source": map[string]any{
			"content": strings.Join([]string{
				"# comment",
				"1.1.1.1 # inline",
				"address=/cf.example.com/1.0.0.1",
				"https://edge.example.com/path/file.txt",
				"bad-token",
			}, "\n"),
			"ip_limit": 8,
			"ip_mode":  "traverse",
			"kind":     "inline",
			"name":     "complex",
		},
	}

	result := decodeCommandForTest(t, service.PreviewSource(encodeJSON(payload)))
	if !boolValue(result["ok"], false) {
		t.Fatalf("preview failed: %#v", result)
	}
	data := mapValue(result["data"])
	entries := stringSliceForTest(data["preview_entries"])
	want := []string{"1.1.1.1", "1.0.0.1", "203.0.113.20"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
	summary := mapValue(data["summary"])
	if got := intValue(summary["invalid_count"], 0); got != 1 {
		t.Fatalf("invalid_count = %d, want 1", got)
	}
}

func TestServicePreviewSourceStopsDomainResolutionAtLimitWithoutColoFilter(t *testing.T) {
	calls := make(map[string]int)
	oldResolver := sourceParseResolver
	sourceParseResolver = mobileResolverForTestFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		calls[host]++
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.60")}}, nil
	})
	t.Cleanup(func() { sourceParseResolver = oldResolver })

	service := NewService()
	baseDir := t.TempDir()
	decodeCommandForTest(t, service.Init(baseDir))

	payload := map[string]any{
		"preview_limit": 8,
		"source": map[string]any{
			"content":  "first.example.com\nsecond.example.com",
			"ip_limit": 1,
			"ip_mode":  "traverse",
			"kind":     "inline",
			"name":     "limited",
		},
	}

	result := decodeCommandForTest(t, service.PreviewSource(encodeJSON(payload)))
	if !boolValue(result["ok"], false) {
		t.Fatalf("preview failed: %#v", result)
	}
	data := mapValue(result["data"])
	if entries := stringSliceForTest(data["preview_entries"]); !reflect.DeepEqual(entries, []string{"203.0.113.60"}) {
		t.Fatalf("entries = %#v, want one resolved IP", entries)
	}
	if calls["first.example.com"] != 1 || calls["second.example.com"] != 0 {
		t.Fatalf("resolver calls = %#v, want only first domain resolved", calls)
	}
}

func TestServiceSourceColoFilterPrefiltersTraverseEntries(t *testing.T) {
	service := newServiceWithMobileColoDictionaryForTest(t)
	source := desktopSource{
		ColoFilter: "SJC",
		Content:    "104.16.0.1\n104.20.0.1\nbad",
		IPLimit:    10,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "mobile-test",
	}
	entries, _, warnings, invalid, err := service.buildSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err != nil {
		t.Fatalf("buildSourceEntriesWithConfig returned error: %v", err)
	}
	if invalid != 1 {
		t.Fatalf("invalid = %d, want 1", invalid)
	}
	if !reflect.DeepEqual(entries, []string{"104.16.0.1"}) {
		t.Fatalf("entries = %#v, want only SJC IP", entries)
	}
	if !containsForTest(warnings, "COLO 白名单 SJC 预筛") {
		t.Fatalf("warnings = %#v, want COLO prefilter warning", warnings)
	}
}

func TestServiceSourceColoFilterIntersectsCIDRBeforeMICS(t *testing.T) {
	service := newServiceWithMobileColoDictionaryForTest(t)
	oldRunner := mobileMCISSearchRunner
	var gotTokens []string
	mobileMCISSearchRunner = func(tokens []string, source desktopSource, cfg probeConfig, limit int) ([]string, []string, error) {
		gotTokens = append([]string(nil), tokens...)
		return []string{"104.16.0.1"}, nil, nil
	}
	t.Cleanup(func() { mobileMCISSearchRunner = oldRunner })

	source := desktopSource{
		ColoFilter: "SJC",
		Content:    "104.0.0.0/8",
		IPLimit:    10,
		IPMode:     "mcis",
		Kind:       "inline",
		Name:       "mobile-mcis",
	}
	entries, _, _, _, err := service.buildSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err != nil {
		t.Fatalf("buildSourceEntriesWithConfig returned error: %v", err)
	}
	if !reflect.DeepEqual(gotTokens, []string{"104.16.0.0/30"}) {
		t.Fatalf("MICS tokens = %#v, want COLO-intersected CIDR", gotTokens)
	}
	if !reflect.DeepEqual(entries, []string{"104.16.0.1"}) {
		t.Fatalf("entries = %#v, want fake MICS result", entries)
	}
}

func TestServiceSourceColoFilterSelectsDictionaryByInputFamily(t *testing.T) {
	service := newServiceWithMobileSplitColoDictionaryForTest(t)

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
			source := desktopSource{
				ColoFilter: "SJC",
				Content:    tc.content,
				IPLimit:    20,
				IPMode:     "traverse",
				Kind:       "inline",
				Name:       tc.name,
			}
			entries, _, _, _, err := service.buildSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
			if err != nil {
				t.Fatalf("buildSourceEntriesWithConfig returned error: %v", err)
			}
			if !reflect.DeepEqual(entries, tc.want) {
				t.Fatalf("entries = %#v, want %#v", entries, tc.want)
			}
		})
	}
}

func TestServiceSourceCountryColoFilterRequiresColoFile(t *testing.T) {
	service := NewService()
	baseDir := t.TempDir()
	decodeCommandForTest(t, service.Init(baseDir))
	if err := os.WriteFile(filepath.Join(baseDir, colodict.GeofeedFileName), []byte("ip_prefix,country,region,city,postal_code\n104.16.0.0/13,US,CA,San Jose,\n"), 0o600); err != nil {
		t.Fatalf("write geofeed file: %v", err)
	}

	source := desktopSource{
		ColoFilter: "JP",
		Content:    "104.16.0.1",
		IPLimit:    10,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "missing-colo",
	}
	_, _, _, _, err := service.buildSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err == nil || !strings.Contains(err.Error(), "COLO 文件不存在") {
		t.Fatalf("err = %v, want missing COLO file error", err)
	}
}

func TestServiceSourceStage2DefersColoFilter(t *testing.T) {
	service := newServiceWithMobileColoDictionaryForTest(t)
	cfg := defaultProbeConfig()
	cfg.SourceColoFilterPhase = sourceColoFilterPhaseStage2
	source := desktopSource{
		ColoFilter: "SJC",
		Content:    "104.16.0.1\n104.20.0.1",
		IPLimit:    10,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "mobile-stage2",
	}

	entries, _, warnings, invalid, err := service.buildSourceEntriesWithConfig(source.Content, source, cfg)
	if err != nil {
		t.Fatalf("buildSourceEntriesWithConfig returned error: %v", err)
	}
	if invalid != 0 {
		t.Fatalf("invalid = %d, want 0", invalid)
	}
	want := []string{"104.16.0.1", "104.20.0.1"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want unfiltered candidates %#v", entries, want)
	}
	if !containsForTest(warnings, "第二阶段起效") {
		t.Fatalf("warnings = %#v, want stage2 warning", warnings)
	}
}

func TestServiceSourceStage2RequiresColoFile(t *testing.T) {
	service := NewService()
	baseDir := t.TempDir()
	decodeCommandForTest(t, service.Init(baseDir))
	if err := os.WriteFile(filepath.Join(baseDir, colodict.GeofeedFileName), []byte("ip_prefix,country,region,city,postal_code\n104.16.0.0/13,US,CA,San Jose,\n"), 0o600); err != nil {
		t.Fatalf("write geofeed file: %v", err)
	}

	cfg := defaultProbeConfig()
	cfg.SourceColoFilterPhase = sourceColoFilterPhaseStage2
	source := desktopSource{
		ColoFilter: "JP",
		Content:    "104.16.0.1",
		IPLimit:    10,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "mobile-stage2-missing",
	}

	_, _, _, _, err := service.buildSourceEntriesWithConfig(source.Content, source, cfg)
	if err == nil || !strings.Contains(err.Error(), "COLO 文件不存在") {
		t.Fatalf("err = %v, want missing COLO file error in stage2 mode", err)
	}
}

func TestServicePrepareSourcesStage2BuildsPassAnySourceColoFilters(t *testing.T) {
	service := newServiceWithMobileColoDictionaryForTest(t)
	cfg := defaultProbeConfig()
	cfg.SourceColoFilterPhase = sourceColoFilterPhaseStage2

	prepared := service.prepareSources(cfg, []desktopSource{
		{
			ColoFilter: "SJC",
			Content:    "104.16.0.1\n104.20.0.1",
			Enabled:    true,
			IPLimit:    10,
			IPMode:     "traverse",
			Kind:       "inline",
			Name:       "mobile-sjc",
		},
		{
			ColoFilter: "LAX",
			Content:    "104.16.0.1",
			Enabled:    true,
			IPLimit:    10,
			IPMode:     "traverse",
			Kind:       "inline",
			Name:       "mobile-lax",
		},
		{
			Content: "104.20.0.1",
			Enabled: true,
			IPLimit: 10,
			IPMode:  "traverse",
			Kind:    "inline",
			Name:    "mobile-unrestricted",
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
	if !containsForTest(prepared.Warnings, "第二阶段起效") {
		t.Fatalf("warnings = %#v, want stage2 warning", prepared.Warnings)
	}
}

func newServiceWithMobileColoDictionaryForTest(t *testing.T) *Service {
	t.Helper()
	service := NewService()
	baseDir := t.TempDir()
	decodeCommandForTest(t, service.Init(baseDir))
	raw := strings.Join([]string{
		"ip_prefix,colo,country,region,city",
		"104.16.0.0/30,SJC,US,CA,San Jose",
		"104.20.0.0/30,LAX,US,CA,Los Angeles",
	}, "\n")
	if err := os.WriteFile(filepath.Join(baseDir, colodict.ColoFileName), []byte(raw), 0o600); err != nil {
		t.Fatalf("write mobile colo file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, colodict.ColoIPv4FileName), []byte(raw), 0o600); err != nil {
		t.Fatalf("write mobile IPv4 colo file: %v", err)
	}
	emptyIPv6Raw, err := colodict.EncodeColoEntries(nil)
	if err != nil {
		t.Fatalf("EncodeColoEntries(empty): %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, colodict.ColoIPv6FileName), emptyIPv6Raw, 0o600); err != nil {
		t.Fatalf("write mobile IPv6 colo file: %v", err)
	}
	return service
}

func newServiceWithMobileSplitColoDictionaryForTest(t *testing.T) *Service {
	t.Helper()
	service := NewService()
	baseDir := t.TempDir()
	decodeCommandForTest(t, service.Init(baseDir))
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
		path := filepath.Join(baseDir, name)
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return service
}

func TestServiceRunProbeReturnsFailureForEmptySources(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	payload := map[string]any{
		"task_id": "mobile-test",
		"config":  defaultConfigSnapshot(),
		"sources": []map[string]any{
			{"kind": "inline", "content": "bad-input", "enabled": true, "id": "source-1", "name": "bad"},
		},
	}
	result := decodeCommandForTest(t, service.RunProbe(encodeJSON(payload)))
	if boolValue(result["ok"], true) {
		t.Fatalf("RunProbe unexpectedly succeeded: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "PROBE_FAILED" {
		t.Fatalf("code = %q", got)
	}
}

func TestServiceRunProbeFailsWhenAnySourceRequiresMissingColoFile(t *testing.T) {
	service := NewService()
	baseDir := t.TempDir()
	decodeCommandForTest(t, service.Init(baseDir))
	if err := os.WriteFile(filepath.Join(baseDir, colodict.GeofeedFileName), []byte("ip_prefix,country,region,city,postal_code\n104.16.0.0/13,US,CA,San Jose,\n"), 0o600); err != nil {
		t.Fatalf("write geofeed file: %v", err)
	}
	cfg := defaultConfigSnapshot()
	probe := mapValue(cfg["probe"])
	probe["source_colo_filter_phase"] = sourceColoFilterPhaseStage2
	payload := map[string]any{
		"task_id": "mobile-missing-colo",
		"config":  cfg,
		"sources": []map[string]any{
			{
				"colo_filter": "JP",
				"content":     "104.16.0.1",
				"enabled":     true,
				"ip_limit":    10,
				"ip_mode":     "traverse",
				"kind":        "inline",
				"name":        "missing-colo",
			},
			{
				"content":  "1.1.1.1",
				"enabled":  true,
				"ip_limit": 10,
				"ip_mode":  "traverse",
				"kind":     "inline",
				"name":     "fallback-source",
			},
		},
	}

	result := decodeCommandForTest(t, service.RunProbe(encodeJSON(payload)))
	if boolValue(result["ok"], true) {
		t.Fatalf("RunProbe unexpectedly succeeded: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "PROBE_FAILED" {
		t.Fatalf("code = %q", got)
	}
	if message := stringValue(result["message"], ""); !strings.Contains(message, "COLO 文件不存在") || !strings.Contains(message, "missing-colo") || !strings.Contains(message, "第二阶段") {
		t.Fatalf("message = %q, want missing COLO file failure", message)
	}
}

func TestServiceRunProbeReportsTCPInputPoolError(t *testing.T) {
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	t.Cleanup(func() {
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
	})

	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return nil, errors.New("ParseCIDR err: invalid CIDR address: bad-input")
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		t.Fatal("trace runner should not run after TCP input pool error")
		return nil
	}

	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	sink := &probeEventSinkForTest{}
	service.SetEventSink(sink)

	cfg := defaultConfigSnapshot()
	probe := mapValue(cfg["probe"])
	probe["disable_download"] = true
	result := decodeCommandForTest(t, service.RunProbe(encodeJSON(map[string]any{
		"config": cfg,
		"sources": []map[string]any{{
			"content":  "1.1.1.1",
			"enabled":  true,
			"ip_limit": 10,
			"ip_mode":  "traverse",
			"kind":     "inline",
			"name":     "valid-source",
		}},
		"task_id": "mobile-tcp-input-error",
	})))
	if boolValue(result["ok"], true) {
		t.Fatalf("RunProbe unexpectedly succeeded: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "PROBE_FAILED" {
		t.Fatalf("code = %q", got)
	}
	if message := stringValue(result["message"], ""); !strings.Contains(message, "ParseCIDR err") {
		t.Fatalf("message = %q, want TCP input error", message)
	}
	event := decodeProbeEventForTest(t, sink.lastEvent)
	if got := stringValue(event["event"], ""); got != "probe.failed" {
		t.Fatalf("event = %q, want probe.failed", got)
	}
	payload := mapValue(event["payload"])
	if got := boolValue(payload["recoverable"], true); got {
		t.Fatalf("recoverable = true, want false")
	}
}

func TestServiceRunProbeRecoversFromPanic(t *testing.T) {
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	t.Cleanup(func() {
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
	})

	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		panic("tcp runner boom")
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		t.Fatal("trace runner should not run after TCP panic")
		return nil
	}

	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	sink := &probeEventSinkForTest{}
	service.SetEventSink(sink)

	cfg := defaultConfigSnapshot()
	probe := mapValue(cfg["probe"])
	probe["disable_download"] = true
	taskID := "mobile-panic"
	result := decodeCommandForTest(t, service.RunProbe(encodeJSON(map[string]any{
		"config": cfg,
		"sources": []map[string]any{{
			"content":  "1.1.1.1",
			"enabled":  true,
			"ip_limit": 10,
			"ip_mode":  "traverse",
			"kind":     "inline",
			"name":     "valid-source",
		}},
		"task_id": taskID,
	})))
	if boolValue(result["ok"], true) {
		t.Fatalf("RunProbe unexpectedly succeeded: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "PROBE_FAILED" {
		t.Fatalf("code = %q", got)
	}
	if message := stringValue(result["message"], ""); !strings.Contains(message, "tcp runner boom") {
		t.Fatalf("message = %q, want panic detail", message)
	}

	event := decodeProbeEventForTest(t, sink.lastEvent)
	if got := stringValue(event["event"], ""); got != "probe.failed" {
		t.Fatalf("event = %q, want probe.failed", got)
	}
	eventPayload := mapValue(event["payload"])
	if got := boolValue(eventPayload["recoverable"], true); got {
		t.Fatalf("recoverable = true, want false")
	}

	snapshot, ok, err := service.loadTaskSnapshot(taskID)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if !ok {
		t.Fatal("snapshot was not persisted")
	}
	if snapshot.Status != "failed" {
		t.Fatalf("snapshot status = %q, want failed", snapshot.Status)
	}
	if snapshot.RuntimeAttached || snapshot.ResumeCapable {
		t.Fatalf("snapshot runtime flags = attached:%v resume:%v, want false", snapshot.RuntimeAttached, snapshot.ResumeCapable)
	}
	service.stateMu.Lock()
	currentTaskID := service.currentTaskID
	service.stateMu.Unlock()
	if currentTaskID != "" {
		t.Fatalf("currentTaskID = %q, want cleared", currentTaskID)
	}
}

func TestServiceEmitRecoversWhenEventSinkPanics(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	service.SetEventSink(probePanicEventSinkForTest{})

	taskID := "mobile-sink-panic"
	service.emit(taskID, "probe.failed", map[string]any{
		"message":     "sink failed",
		"recoverable": false,
	})

	snapshot, ok, err := service.loadTaskSnapshot(taskID)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if !ok {
		t.Fatal("snapshot was not persisted")
	}
	if snapshot.Status != "failed" {
		t.Fatalf("snapshot status = %q, want failed", snapshot.Status)
	}
}

func TestServiceRunProbeGroupsMixedSourcePorts(t *testing.T) {
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	oldDownload := mobileDownloadProbeRunner
	t.Cleanup(func() {
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
		mobileDownloadProbeRunner = oldDownload
	})

	ports := make([]int, 0, 2)
	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		ports = append(ports, task.TCPPort)
		ip := "8.8.8.8"
		if task.TCPPort == 2053 {
			ip = "1.1.1.1"
		}
		return utils.PingDelaySet{{
			PingData: &utils.PingData{
				IP:       parseMobileTestIP(ip),
				Sended:   3,
				Received: 3,
				Delay:    time.Duration(task.TCPPort) * time.Microsecond,
			},
		}}, nil
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet { return input }
	mobileDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	cfg := defaultConfigSnapshot()
	probe := mapValue(cfg["probe"])
	probe["tcp_port"] = 443
	probe["disable_download"] = true
	probe["print_num"] = 0

	result := decodeCommandForTest(t, service.RunProbe(encodeJSON(map[string]any{
		"config":        cfg,
		"config_source": "draft",
		"sources": []map[string]any{{
			"content":  "1.1.1.1:2053\n8.8.8.8",
			"enabled":  true,
			"id":       "mixed-mobile-ports",
			"ip_limit": 10,
			"ip_mode":  "traverse",
			"kind":     "inline",
			"name":     "Mixed Mobile Ports",
		}},
		"task_id": "mobile-mixed-port-test",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunProbe failed: %#v", result)
	}
	if !reflect.DeepEqual(ports, []int{443, 2053}) {
		t.Fatalf("ports = %v, want grouped global 443 and source 2053", ports)
	}
	data := mapValue(result["data"])
	rows, ok := data["results"].([]any)
	if !ok {
		t.Fatalf("results = %#v, want rows", data["results"])
	}
	rowPorts := map[string]int{}
	for _, item := range rows {
		row := mapValue(item)
		rowPorts[stringValue(row["ip"], "")] = int(row["test_port"].(float64))
	}
	if rowPorts["1.1.1.1"] != 2053 || rowPorts["8.8.8.8"] != 443 {
		t.Fatalf("row ports = %#v, want source override and global fallback", rowPorts)
	}
	taskContext := mapValue(data["task_context"])
	if got := stringValue(taskContext["config_source"], ""); got != "draft" {
		t.Fatalf("config_source = %q, want draft", got)
	}
	if got := int(taskContext["global_tcp_port"].(float64)); got != 443 {
		t.Fatalf("global_tcp_port = %d, want 443", got)
	}
	if got := int(taskContext["current_test_port"].(float64)); got != 0 {
		t.Fatalf("current_test_port = %d, want 0 for mixed source/global port groups", got)
	}
}

func TestServiceRunProbeGroupedSummaryUsesStage3Totals(t *testing.T) {
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	oldDownload := mobileDownloadProbeRunner
	t.Cleanup(func() {
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
		mobileDownloadProbeRunner = oldDownload
	})

	downloadInputCounts := make([]int, 0, 2)
	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		ips := []string{"8.8.8.8", "8.8.4.4"}
		if task.TCPPort == 2053 {
			ips = []string{"1.1.1.1", "1.1.1.2"}
		}
		result := make(utils.PingDelaySet, 0, len(ips))
		for _, ip := range ips {
			result = append(result, utils.CloudflareIPData{
				PingData: &utils.PingData{
					IP:       parseMobileTestIP(ip),
					Sended:   3,
					Received: 3,
					Delay:    time.Duration(task.TCPPort) * time.Microsecond,
				},
			})
		}
		return result, nil
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet { return input }
	mobileDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		downloadInputCounts = append(downloadInputCounts, len(input))
		return utils.DownloadSpeedSet{}
	}

	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	cfg := defaultConfigSnapshot()
	probe := mapValue(cfg["probe"])
	probe["tcp_port"] = 443
	probe["strategy"] = "full"
	probe["disable_download"] = false
	probe["print_num"] = 0
	probe["stage_limits"] = map[string]any{"stage3": 1}

	result := decodeCommandForTest(t, service.RunProbe(encodeJSON(map[string]any{
		"config": cfg,
		"sources": []map[string]any{{
			"content":  "1.1.1.1:2053\n1.1.1.2:2053\n8.8.8.8\n8.8.4.4",
			"enabled":  true,
			"id":       "mobile-grouped-summary",
			"ip_limit": 10,
			"ip_mode":  "traverse",
			"kind":     "inline",
			"name":     "Mobile Grouped Summary",
		}},
		"task_id": "mobile-grouped-summary-test",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunProbe failed: %#v", result)
	}
	if !reflect.DeepEqual(downloadInputCounts, []int{1, 1}) {
		t.Fatalf("download input counts = %v, want one stage3 candidate per port group", downloadInputCounts)
	}
	summary := mapValue(mapValue(result["data"])["summary"])
	if got := int(summary["total"].(float64)); got != 2 {
		t.Fatalf("summary total = %d, want 2", got)
	}
	if got := int(summary["passed"].(float64)); got != 0 {
		t.Fatalf("summary passed = %d, want 0", got)
	}
	if got := int(summary["failed"].(float64)); got != 2 {
		t.Fatalf("summary failed = %d, want 2", got)
	}
}

func TestServiceRunProbeCompletedEventKeepsPrivatePathUntilAndroidExportFinishes(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	sink := &probeEventSinkForTest{}
	service.SetEventSink(sink)

	result := probeRunResult{
		OutputFile: "/private/result.csv",
		Results: []probeRow{
			{IP: "1.1.1.1"},
		},
		Summary: probeSummary{Passed: 1, Total: 1},
	}
	service.emitProbeCompleted(
		"task-export-uri",
		result,
		sourceSummary{DuplicateCount: 1},
		2,
		"content://exports/result.csv",
	)

	event := decodeProbeEventForTest(t, sink.lastEvent)
	payload := mapValue(event["payload"])
	if got := stringValue(payload["target_path"], ""); got != "/private/result.csv" {
		t.Fatalf("target_path = %q, want private result path", got)
	}
	if got := stringValue(payload["android_export_uri"], ""); got != "content://exports/result.csv" {
		t.Fatalf("android_export_uri = %q, want SAF URI", got)
	}
	if !boolValue(payload["android_export_pending"], false) {
		t.Fatalf("android_export_pending = false, want true")
	}
	if _, ok := payload["task_context"]; !ok {
		t.Fatalf("payload = %#v, want task_context", payload)
	}
}

func TestServiceRecordAndroidExportResultEmitsAndPersistsExportCompleted(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	sink := &probeEventSinkForTest{}
	service.SetEventSink(sink)

	result := decodeCommandForTest(t, service.RecordAndroidExportResult(encodeJSON(map[string]any{
		"ok":          true,
		"source_path": "/private/result.csv",
		"status":      "written",
		"target_uri":  "content://exports/result.csv",
		"task_id":     "task-export-uri",
		"written":     3,
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RecordAndroidExportResult failed: %#v", result)
	}
	event := decodeProbeEventForTest(t, sink.lastEvent)
	if got := stringValue(event["event"], ""); got != "probe.export_completed" {
		t.Fatalf("event = %q, want probe.export_completed", got)
	}
	payload := mapValue(event["payload"])
	if got := stringValue(payload["target_path"], ""); got != "content://exports/result.csv" {
		t.Fatalf("target_path = %q, want SAF URI", got)
	}

	snapshot := decodeCommandForTest(t, service.LoadTaskSnapshot(encodeJSON(map[string]any{
		"task_id": "task-export-uri",
	})))
	data := mapValue(snapshot["data"])
	exportRecord := mapValue(data["export_record"])
	if got := stringValue(exportRecord["target_dir"], ""); got != "content://exports" {
		t.Fatalf("target_dir = %q, want content://exports", got)
	}
	if got := stringValue(exportRecord["file_name"], ""); got != "result.csv" {
		t.Fatalf("file_name = %q, want result.csv", got)
	}
	if got := intValue(exportRecord["written_count"], 0); got != 3 {
		t.Fatalf("written_count = %d, want 3", got)
	}
}

func TestServiceRecordAndroidExportResultDoesNotMarkProbeFailed(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	sink := &probeEventSinkForTest{}
	service.SetEventSink(sink)

	result := decodeCommandForTest(t, service.RecordAndroidExportResult(encodeJSON(map[string]any{
		"message":    "Android 系统导出目标无法写入。",
		"sourcePath": "/private/result.csv",
		"status":     "failed",
		"targetUri":  "content://exports/result.csv",
		"taskId":     "task-export-failed",
	})))
	if boolValue(result["ok"], true) {
		t.Fatalf("RecordAndroidExportResult succeeded unexpectedly: %#v", result)
	}
	event := decodeProbeEventForTest(t, sink.lastEvent)
	if got := stringValue(event["event"], ""); got != "probe.export_failed" {
		t.Fatalf("event = %q, want probe.export_failed", got)
	}
	snapshot := decodeCommandForTest(t, service.LoadTaskSnapshot(encodeJSON(map[string]any{
		"task_id": "task-export-failed",
	})))
	data := mapValue(snapshot["data"])
	if got := stringValue(data["status"], ""); got != "completed" {
		t.Fatalf("status = %q, want completed", got)
	}
}

func TestMobileTraceDiagnosticsPayloadAndSummary(t *testing.T) {
	diagnostics := newMobileTraceDiagnostics(probeConfig{
		TraceColoMode: "trace_url",
		TraceURL:      "https://trace.example.com/cdn-cgi/trace",
	})
	diagnostics.Record(task.TraceDiagnostic{
		IP:           "1.1.1.1",
		Reason:       "rate_limited",
		RetryAfterMS: 1500,
		StatusCode:   429,
		URL:          "https://trace.example.com/cdn-cgi/trace",
	})
	diagnostics.Record(task.TraceDiagnostic{
		Error:      "read timeout",
		IP:         "1.1.1.2",
		Reason:     "trace_error",
		StatusCode: 504,
		URL:        "https://trace.example.com/cdn-cgi/trace",
	})
	diagnostics.Record(task.TraceDiagnostic{
		IP:         "1.1.1.3",
		Reason:     "rate_limited",
		StatusCode: 429,
		URL:        "https://trace.example.com/cdn-cgi/trace",
	})

	payload := diagnostics.Payload()
	if got := stringValue(payload["trace_colo_mode"], ""); got != "trace_url" {
		t.Fatalf("trace_colo_mode = %q, want trace_url", got)
	}
	if got := stringValue(payload["trace_url"], ""); got != "https://trace.example.com/cdn-cgi/trace" {
		t.Fatalf("trace_url = %q, want configured trace URL", got)
	}
	reasonCounts, ok := payload["reason_counts"].(map[string]int)
	if !ok {
		t.Fatalf("reason_counts = %#v, want map[string]int", payload["reason_counts"])
	}
	if got := reasonCounts["rate_limited"]; got != 2 {
		t.Fatalf("rate_limited count = %d, want 2", got)
	}
	if got := reasonCounts["trace_error"]; got != 1 {
		t.Fatalf("trace_error count = %d, want 1", got)
	}
	statusCounts, ok := payload["status_counts"].(map[string]int)
	if !ok {
		t.Fatalf("status_counts = %#v, want map[string]int", payload["status_counts"])
	}
	if got := statusCounts["429"]; got != 2 {
		t.Fatalf("status 429 count = %d, want 2", got)
	}
	samples, ok := payload["samples"].([]map[string]any)
	if !ok || len(samples) != 3 {
		t.Fatalf("samples = %#v, want 3 structured samples", payload["samples"])
	}
	summary := diagnostics.Summary()
	if !strings.Contains(summary, "服务端限流 2 次") {
		t.Fatalf("summary = %q, want rate limited summary", summary)
	}
	if !strings.Contains(summary, "HTTP 429 2 次") {
		t.Fatalf("summary = %q, want HTTP 429 summary", summary)
	}
}

func TestServiceEmitProbeCompletedIncludesTraceDiagnostics(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	sink := &probeEventSinkForTest{}
	service.SetEventSink(sink)

	service.emitProbeCompleted(
		"task-trace-completed",
		probeRunResult{
			FailureStage: probecore.StageTrace,
			TraceDiagnostics: map[string]any{
				"reason_counts": map[string]any{"rate_limited": 2},
				"status_counts": map[string]any{"429": 2},
				"samples": []map[string]any{
					{
						"ip":          "1.1.1.1",
						"reason":      "rate_limited",
						"status_code": 429,
						"url":         "https://trace.example.com/cdn-cgi/trace",
					},
				},
				"trace_colo_mode": "trace_url",
				"trace_url":       "https://trace.example.com/cdn-cgi/trace",
			},
			Summary: probeSummary{Failed: 1, Total: 1},
		},
		sourceSummary{},
		0,
		"",
	)

	event := decodeProbeEventForTest(t, sink.lastEvent)
	payload := mapValue(event["payload"])
	if got := stringValue(payload["failure_stage"], ""); got != probecore.StageTrace {
		t.Fatalf("failure_stage = %q, want %q", got, probecore.StageTrace)
	}
	traceDiagnostics := mapValue(payload["trace_diagnostics"])
	reasonCounts := mapValue(traceDiagnostics["reason_counts"])
	if got := intValue(reasonCounts["rate_limited"], 0); got != 2 {
		t.Fatalf("trace_diagnostics = %#v, want rate_limited count 2", traceDiagnostics)
	}
}

func TestServiceEmitSpeedIncludesMeasurementMetadata(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	sink := &probeEventSinkForTest{}
	service.SetEventSink(sink)

	service.emitSpeed("speed-task", task.DownloadSpeedSample{
		Stage:             "stage3_get",
		IP:                "1.1.1.1",
		CurrentSpeedMBs:   0,
		CurrentReady:      true,
		AverageSpeedMBs:   0,
		AverageReady:      true,
		BodyRead:          true,
		BytesRead:         4096,
		ElapsedMS:         250,
		MeasuredBytes:     0,
		MeasuredElapsedMS: 0,
		TransferComplete:  false,
	})

	event := decodeProbeEventForTest(t, sink.lastEvent)
	if got := stringValue(event["event"], ""); got != "probe.speed" {
		t.Fatalf("event = %q, want probe.speed", got)
	}
	payload := mapValue(event["payload"])
	if got := intValue(payload["measured_bytes"], -1); got != 0 {
		t.Fatalf("measured_bytes = %d, want 0", got)
	}
	if got := intValue(payload["measured_elapsed_ms"], -1); got != 0 {
		t.Fatalf("measured_elapsed_ms = %d, want 0", got)
	}
	if got := boolValue(payload["average_ready"], false); !got {
		t.Fatal("average_ready = false, want true")
	}
	if got := boolValue(payload["current_ready"], false); !got {
		t.Fatal("current_ready = false, want true")
	}
	if got := boolValue(payload["body_read"], false); !got {
		t.Fatal("body_read = false, want true")
	}
	if got := boolValue(payload["transfer_complete"], true); got {
		t.Fatal("transfer_complete = true, want false")
	}
}

func TestServiceEmitProgressThrottlesSameStage(t *testing.T) {
	service := NewService()
	sink := &probeEventSinkForTest{}
	service.SetEventSink(sink)
	service.configureProgressThrottle(time.Hour)

	service.emitProgress("progress-task", "stage1_tcp", 0, 0, 0, 10)
	service.emitProgress("progress-task", "stage1_tcp", 2, 1, 1, 10)
	service.emitProgress("progress-task", "stage1_tcp", 3, 2, 1, 10)
	service.emitProgress("progress-task", "stage2_trace", 2, 1, 1, 10)
	service.emitProgress("progress-task", "stage2_trace", 3, 2, 1, 10)
	service.emitProgress("progress-task", "stage2_trace", 10, 8, 2, 10)

	if len(sink.events) != 3 {
		t.Fatalf("progress events = %d, want first, stage switch, and final events", len(sink.events))
	}
	first := decodeProbeEventForTest(t, sink.events[0])
	if got := stringValue(mapValue(first["payload"])["stage"], ""); got != "stage1_tcp" {
		t.Fatalf("first progress stage = %q, want stage1_tcp", got)
	}
	switched := decodeProbeEventForTest(t, sink.events[1])
	if got := stringValue(mapValue(switched["payload"])["stage"], ""); got != "stage2_trace" {
		t.Fatalf("stage-switch progress stage = %q, want stage2_trace", got)
	}
	final := decodeProbeEventForTest(t, sink.events[2])
	finalPayload := mapValue(final["payload"])
	if got := intValue(finalPayload["processed"], 0); got != 10 {
		t.Fatalf("final progress processed = %d, want 10", got)
	}
}

func TestServicePreservesPendingCancelForStartingTask(t *testing.T) {
	service := NewService()
	result := decodeCommandForTest(t, service.CancelProbe(encodeJSON(map[string]any{
		"task_id": "pending-task",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("CancelProbe failed: %#v", result)
	}

	service.setCurrentTask("pending-task")
	if !service.isCancelRequested("pending-task") {
		t.Fatal("pending cancel was cleared when the task started")
	}
	service.clearCurrentTask("pending-task")
	if service.isCancelRequested("pending-task") {
		t.Fatal("cancel state was not cleared after the task finished")
	}
}

type probeEventSinkForTest struct {
	lastEvent string
	events    []string
}

func (s *probeEventSinkForTest) OnProbeEvent(eventJSON string) {
	s.lastEvent = eventJSON
	s.events = append(s.events, eventJSON)
}

type probePanicEventSinkForTest struct{}

func (probePanicEventSinkForTest) OnProbeEvent(eventJSON string) {
	panic("event sink boom")
}

func decodeProbeEventForTest(t *testing.T, raw string) map[string]any {
	t.Helper()
	if raw == "" {
		t.Fatal("expected probe event")
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode event %s: %v", raw, err)
	}
	return result
}

func TestServicePendingCancelDoesNotCancelDifferentTask(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.CancelProbe(encodeJSON(map[string]any{
		"task_id": "stale-task",
	})))

	service.setCurrentTask("new-task")
	if service.isCancelRequested("new-task") {
		t.Fatal("stale pending cancel affected a different task")
	}
}

func TestServiceCancelRejectsMissingActiveTaskWithoutTaskID(t *testing.T) {
	service := NewService()
	result := decodeCommandForTest(t, service.CancelProbe(encodeJSON(map[string]any{})))
	if boolValue(result["ok"], true) {
		t.Fatalf("CancelProbe = %#v, want failure", result)
	}
	if got := stringValue(result["code"], ""); got != "PROBE_CANCEL_UNAVAILABLE" {
		t.Fatalf("CancelProbe code = %q, want PROBE_CANCEL_UNAVAILABLE", got)
	}
}

func TestServiceCancelRejectsMismatchedRunningTaskID(t *testing.T) {
	service := NewService()
	service.setCurrentTask("active-task")
	service.stateMu.Lock()
	service.pauseRequested = true
	service.pausedTaskID = "active-task"
	service.stateMu.Unlock()

	result := decodeCommandForTest(t, service.CancelProbe(encodeJSON(map[string]any{
		"task_id": "other-task",
	})))
	if boolValue(result["ok"], true) {
		t.Fatalf("CancelProbe = %#v, want failure", result)
	}
	if got := stringValue(result["code"], ""); got != "PROBE_CANCEL_UNAVAILABLE" {
		t.Fatalf("CancelProbe code = %q, want PROBE_CANCEL_UNAVAILABLE", got)
	}

	service.stateMu.Lock()
	defer service.stateMu.Unlock()
	if service.currentTaskID != "active-task" {
		t.Fatalf("currentTaskID = %q, want active-task", service.currentTaskID)
	}
	if !service.pauseRequested || service.pausedTaskID != "active-task" {
		t.Fatalf("pause state = requested:%v id:%q, want active-task paused", service.pauseRequested, service.pausedTaskID)
	}
	if service.cancelRequested {
		t.Fatal("cancelRequested = true, want false after mismatched cancel")
	}
}

func TestServiceStopInterruptsAllTraceRequests(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload map[string]any
	}{
		{
			name:    "pause",
			payload: map[string]any{"mode": "pause", "task_id": "trace-task"},
		},
		{
			name:    "cancel",
			payload: map[string]any{"task_id": "trace-task"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := NewService()
			service.setCurrentTask("trace-task")
			t.Cleanup(func() {
				service.clearCurrentTask("trace-task")
			})

			interrupts := make(chan string, 2)
			cleanupOne := service.registerTraceInterrupt("trace-task", probecore.StageTrace, "1.1.1.1", func() {
				interrupts <- "one"
			})
			cleanupTwo := service.registerTraceInterrupt("trace-task", probecore.StageTrace, "1.1.1.2", func() {
				interrupts <- "two"
			})
			t.Cleanup(cleanupOne)
			t.Cleanup(cleanupTwo)

			result := decodeCommandForTest(t, service.CancelProbe(encodeJSON(tc.payload)))
			if !boolValue(result["ok"], false) {
				t.Fatalf("CancelProbe = %#v, want ok", result)
			}

			seen := map[string]bool{}
			for range 2 {
				select {
				case label := <-interrupts:
					seen[label] = true
				case <-time.After(time.Second):
					t.Fatalf("interrupted trace requests = %v, want both registered requests", seen)
				}
			}
			if !seen["one"] || !seen["two"] {
				t.Fatalf("interrupted trace requests = %v, want one and two", seen)
			}
		})
	}
}

func TestServiceRunProbeRejectsNewTaskWhilePaused(t *testing.T) {
	service := NewService()
	service.setCurrentTask("active-task")
	service.stateMu.Lock()
	service.pauseRequested = true
	service.pausedTaskID = "active-task"
	service.stateMu.Unlock()

	result := decodeCommandForTest(t, service.RunProbe(encodeJSON(map[string]any{
		"task_id": "new-task",
	})))
	if boolValue(result["ok"], true) {
		t.Fatalf("RunProbe while active = %#v, want failure", result)
	}
	if got := stringValue(result["code"], ""); got != "PROBE_ALREADY_RUNNING" {
		t.Fatalf("RunProbe code = %q, want PROBE_ALREADY_RUNNING", got)
	}

	service.stateMu.Lock()
	defer service.stateMu.Unlock()
	if service.currentTaskID != "active-task" {
		t.Fatalf("currentTaskID = %q, want active-task", service.currentTaskID)
	}
	if !service.pauseRequested || service.pausedTaskID != "active-task" {
		t.Fatalf("pause state = requested:%v id:%q, want active-task paused", service.pauseRequested, service.pausedTaskID)
	}
}

func TestServiceCloudflarePushUpdatesCreatesAndDeletes(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() { cloudflareAPIBaseURL = oldBaseURL })

	records := map[string][]CloudflareDNSRecord{
		"A": {
			{ID: "a-1", Type: "A", Name: "edge.example.com", Content: "1.1.1.1", TTL: 60},
			{ID: "a-2", Type: "A", Name: "edge.example.com", Content: "1.0.0.1", TTL: 60},
		},
		"AAAA": {
			{ID: "aaaa-1", Type: "AAAA", Name: "edge.example.com", Content: "2606:4700:4700::1111", TTL: 60},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		switch r.Method {
		case http.MethodGet:
			recordType := assertCloudflareListQueryForTest(t, r)
			writeCloudflareTestResponse(w, map[string]any{
				"success": true,
				"result":  records[recordType],
				"result_info": map[string]any{
					"page":        1,
					"total_pages": 1,
				},
			})
		case http.MethodPatch:
			id := pathBaseForTest(r.URL.Path)
			var record CloudflareDNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			updateCloudflareRecordForTest(t, records, id, record)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodPost:
			var record CloudflareDNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				t.Fatalf("decode post: %v", err)
			}
			record.ID = strings.ToLower(record.Type) + "-created"
			records[record.Type] = append(records[record.Type], record)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodDelete:
			id := pathBaseForTest(r.URL.Path)
			deleteCloudflareRecordForTest(records, id)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": map[string]string{"id": id}})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	service := NewService()
	result := decodeCommandForTest(t, service.PushCloudflareDNSRecords(encodeJSON(cloudflarePayloadForTest("2.2.2.2\n3.3.3.3\n2606:4700:4700::2222"))))
	if !boolValue(result["ok"], false) {
		t.Fatalf("push failed: %#v", result)
	}
	summary := mapValue(mapValue(result["data"])["summary"])
	if intValue(summary["created"], 0) != 0 || intValue(summary["updated"], 0) != 3 || intValue(summary["deleted"], 0) != 0 {
		t.Fatalf("summary = %#v", summary)
	}
	if !reflect.DeepEqual(recordContentsForTest(records["A"]), []string{"2.2.2.2", "3.3.3.3"}) {
		t.Fatalf("A records = %#v", records["A"])
	}
	if !reflect.DeepEqual(recordContentsForTest(records["AAAA"]), []string{"2606:4700:4700::2222"}) {
		t.Fatalf("AAAA records = %#v", records["AAAA"])
	}
}

func TestServiceCloudflarePushResultsUsesRoutingRules(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() { cloudflareAPIBaseURL = oldBaseURL })

	records := map[string][]CloudflareDNSRecord{
		"A": {
			{ID: "a-1", Type: "A", Name: "us.example.com", Content: "1.1.1.1", TTL: 300},
		},
		"AAAA": {},
	}
	queriedNames := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		switch r.Method {
		case http.MethodGet:
			recordType := r.URL.Query().Get("type")
			recordName := r.URL.Query().Get("name")
			queriedNames = append(queriedNames, recordName)
			if recordName != "us.example.com" || (recordType != "A" && recordType != "AAAA") {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			writeCloudflareTestResponse(w, map[string]any{
				"success":     true,
				"result":      records[recordType],
				"result_info": map[string]any{"page": 1, "total_pages": 1},
			})
		case http.MethodPatch:
			id := pathBaseForTest(r.URL.Path)
			var record CloudflareDNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			updateCloudflareRecordForTest(t, records, id, record)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodDelete:
			id := pathBaseForTest(r.URL.Path)
			deleteCloudflareRecordForTest(records, id)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": map[string]string{"id": id}})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	service := newServiceWithMobileColoDictionaryForTest(t)
	payload := cloudflarePayloadForTest("")
	config := mapValue(payload["config"])
	config["cloudflare"] = map[string]any{
		"api_token":       "test-token",
		"record_name":     "",
		"record_type":     "A",
		"routing_enabled": true,
		"routing_rules": []map[string]any{
			{"enabled": true, "filter_tokens": "US", "name": "us", "record_name": "us.example.com", "record_type": "A", "top_n": 1},
		},
		"ttl":     300,
		"zone_id": "zone-123",
	}
	payload["results"] = []probeRow{
		{Colo: "SJC", DelayMS: 20, DownloadSpeedMB: 5, IP: "104.16.0.1", Received: 4, Sended: 4},
		{Colo: "LAX", DelayMS: 10, DownloadSpeedMB: 15, IP: "104.20.0.1", Received: 4, Sended: 4},
		{Colo: "NRT", DelayMS: 5, DownloadSpeedMB: 30, IP: "203.0.113.10", Received: 4, Sended: 4},
	}
	delete(payload, "ipsRaw")

	result := decodeCommandForTest(t, service.PushCloudflareDNSRecords(encodeJSON(payload)))
	if !boolValue(result["ok"], false) {
		t.Fatalf("push failed: %#v", result)
	}
	data := mapValue(result["data"])
	if !boolValue(data["routing_enabled"], false) {
		t.Fatalf("data = %#v, want routing_enabled", data)
	}
	if intValue(data["upload_count"], 0) != 1 {
		t.Fatalf("upload_count = %#v, want 1", data["upload_count"])
	}
	if !reflect.DeepEqual(recordContentsForTest(records["A"]), []string{"104.20.0.1"}) {
		t.Fatalf("A records = %#v", records["A"])
	}
	if len(queriedNames) == 0 {
		t.Fatal("queried names is empty, want Cloudflare route target queries")
	}
	for _, name := range queriedNames {
		if name != "us.example.com" {
			t.Fatalf("queried names = %#v, want only route target name", queriedNames)
		}
	}
}

func TestServiceCloudflarePushDeletesExistingCNAMEBeforeCreate(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() { cloudflareAPIBaseURL = oldBaseURL })

	records := map[string][]CloudflareDNSRecord{
		"A":    {},
		"AAAA": {},
		"CNAME": {
			{ID: "cname-1", Type: "CNAME", Name: "edge.example.com", Content: "origin.example.net", TTL: 300},
		},
	}
	var createdCount, deletedCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if !strings.Contains(r.URL.Path, "/zones/zone-123/dns_records") {
				t.Fatalf("path = %s", r.URL.Path)
			}
			recordName := r.URL.Query().Get("name")
			recordType := r.URL.Query().Get("type")
			if recordName != "edge.example.com" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			if recordType == "" {
				writeCloudflareTestResponse(w, map[string]any{
					"success":     true,
					"result":      records["CNAME"],
					"result_info": map[string]any{"page": 1, "total_pages": 1},
				})
				return
			}
			if recordType != "A" && recordType != "AAAA" {
				t.Fatalf("unexpected typed query: %s", r.URL.RawQuery)
			}
			writeCloudflareTestResponse(w, map[string]any{
				"success":     true,
				"result":      records[recordType],
				"result_info": map[string]any{"page": 1, "total_pages": 1},
			})
		case http.MethodPost:
			createdCount++
			var record CloudflareDNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				t.Fatalf("decode post: %v", err)
			}
			record.ID = strings.ToLower(record.Type) + "-created"
			records[record.Type] = append(records[record.Type], record)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodDelete:
			deletedCount++
			deleteCloudflareRecordForTest(records, pathBaseForTest(r.URL.Path))
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": map[string]string{"id": "cname-1"}})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	service := NewService()
	result := decodeCommandForTest(t, service.PushCloudflareDNSRecords(encodeJSON(cloudflarePayloadForTest("2.2.2.2"))))
	if !boolValue(result["ok"], false) {
		t.Fatalf("push failed: %#v", result)
	}
	summary := mapValue(mapValue(result["data"])["summary"])
	if intValue(summary["created"], 0) != 1 || intValue(summary["deleted"], 0) != 1 {
		t.Fatalf("summary = %#v, want created 1 deleted 1", summary)
	}
	if createdCount != 1 || deletedCount != 1 {
		t.Fatalf("operation counts = created %d deleted %d, want 1 and 1", createdCount, deletedCount)
	}
	if len(records["CNAME"]) != 0 {
		t.Fatalf("CNAME records = %#v, want empty after delete", records["CNAME"])
	}
}

func TestServiceCloudflareListReadsAAndAAAARecords(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() { cloudflareAPIBaseURL = oldBaseURL })

	queriedNames := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Query().Get("type") != "" {
			t.Fatalf("unexpected type query: %s", r.URL.RawQuery)
		}
		recordName := r.URL.Query().Get("name")
		if recordName != "edge.example.com" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		queriedNames = append(queriedNames, recordName)
		writeCloudflareTestResponse(w, map[string]any{
			"success": true,
			"result": []CloudflareDNSRecord{
				{ID: "a-1", Type: "A", Name: "edge.example.com", Content: "content-A", TTL: 300},
				{ID: "aaaa-1", Type: "AAAA", Name: "edge.example.com", Content: "content-AAAA", TTL: 300},
			},
			"result_info": map[string]any{"page": 1, "total_pages": 1},
		})
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	result := decodeCommandForTest(t, NewService().ListCloudflareDNSRecords(encodeJSON(cloudflarePayloadForTest(""))))
	if !boolValue(result["ok"], false) {
		t.Fatalf("list failed: %#v", result)
	}
	if intValue(mapValue(result["data"])["count"], 0) != 2 {
		t.Fatalf("data = %#v, want 2 records", result["data"])
	}
	if !reflect.DeepEqual(queriedNames, []string{"edge.example.com"}) {
		t.Fatalf("queried names = %#v, want configured record name once", queriedNames)
	}
}

func TestServiceCloudflareListConfiguredScopeRequiresRecordName(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() { cloudflareAPIBaseURL = oldBaseURL })

	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		t.Fatalf("unexpected Cloudflare request: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	payload := cloudflarePayloadForTest("")
	payload["scope"] = "configured"
	cloudflare := mapValue(mapValue(payload["config"])["cloudflare"])
	cloudflare["record_name"] = ""

	result := decodeCommandForTest(t, NewService().ListCloudflareDNSRecords(encodeJSON(payload)))
	if boolValue(result["ok"], true) || stringValue(result["code"], "") != "DNS_CONFIG_INVALID" {
		t.Fatalf("result = %#v, want DNS_CONFIG_INVALID", result)
	}
	if !strings.Contains(stringValue(result["message"], ""), "DNS 记录名称") {
		t.Fatalf("message = %q, want record name error", stringValue(result["message"], ""))
	}
	if requested {
		t.Fatalf("configured scope without record name should not request Cloudflare")
	}
}

func TestServiceCloudflareConfigNormalizesTTLChoices(t *testing.T) {
	for _, tc := range []struct {
		name        string
		ttl         any
		wantTTL     int
		wantWarning bool
	}{
		{name: "missing", ttl: nil, wantTTL: 300},
		{name: "legacy-auto", ttl: 1, wantTTL: 300, wantWarning: true},
		{name: "invalid", ttl: 120, wantTTL: 300, wantWarning: true},
		{name: "one-minute", ttl: 60, wantTTL: 60},
		{name: "five-minutes", ttl: 300, wantTTL: 300},
		{name: "ten-minutes", ttl: 600, wantTTL: 600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := cloudflarePayloadForTest("")
			cloudflare := mapValue(mapValue(payload["config"])["cloudflare"])
			if tc.ttl == nil {
				delete(cloudflare, "ttl")
			} else {
				cloudflare["ttl"] = tc.ttl
			}

			cfg, warnings, err := cloudflareDNSConfigFromPayload(payload)
			if err != nil {
				t.Fatalf("cloudflareDNSConfigFromPayload returned error: %v", err)
			}
			if cfg.TTL != tc.wantTTL {
				t.Fatalf("TTL = %d, want %d", cfg.TTL, tc.wantTTL)
			}
			hasWarning := containsForTest(warnings, "Cloudflare TTL 仅支持 60、300、600 秒")
			if hasWarning != tc.wantWarning {
				t.Fatalf("warnings = %#v, want warning %v", warnings, tc.wantWarning)
			}
		})
	}
}

func cloudflarePayloadForTest(ipsRaw string) map[string]any {
	return map[string]any{
		"config": map[string]any{
			"cloudflare": map[string]any{
				"api_token":   "test-token",
				"record_name": "edge.example.com",
				"record_type": "A",
				"ttl":         300,
				"zone_id":     "zone-123",
			},
		},
		"ipsRaw": ipsRaw,
	}
}

func decodeCommandForTest(t *testing.T, raw string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return result
}

func stringSliceForTest(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, stringValue(item, ""))
	}
	return result
}

func containsForTest(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func writeCloudflareTestResponse(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		panic(err)
	}
}

func assertCloudflareListQueryForTest(t *testing.T, r *http.Request) string {
	t.Helper()
	recordType := r.URL.Query().Get("type")
	if r.URL.Query().Get("name") != "edge.example.com" || (recordType != "A" && recordType != "AAAA") {
		t.Fatalf("unexpected query: %s", r.URL.RawQuery)
	}
	return recordType
}

func updateCloudflareRecordForTest(t *testing.T, records map[string][]CloudflareDNSRecord, id string, record CloudflareDNSRecord) {
	t.Helper()
	for recordType, items := range records {
		for index := range items {
			if items[index].ID == id {
				record.ID = id
				records[recordType][index] = record
				return
			}
		}
	}
	t.Fatalf("unknown record id %s", id)
}

func deleteCloudflareRecordForTest(records map[string][]CloudflareDNSRecord, id string) {
	for recordType, items := range records {
		next := items[:0]
		for _, record := range items {
			if record.ID != id {
				next = append(next, record)
			}
		}
		records[recordType] = next
	}
}

func recordContentsForTest(records []CloudflareDNSRecord) []string {
	contents := make([]string, 0, len(records))
	for _, record := range records {
		contents = append(contents, record.Content)
	}
	return contents
}

func pathBaseForTest(value string) string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	return parts[len(parts)-1]
}
