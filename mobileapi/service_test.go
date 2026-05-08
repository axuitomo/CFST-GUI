package mobileapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/internal/colodict"
	"github.com/XIU2/CloudflareSpeedTest/task"
	"github.com/XIU2/CloudflareSpeedTest/utils"
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

func TestMobileCSVFloatPtrAllowsZero(t *testing.T) {
	got := mobileCSVFloatPtr("0")
	if got == nil || *got != 0 {
		t.Fatalf("mobileCSVFloatPtr(0) = %v, want pointer to 0", got)
	}
	if got := mobileCSVFloatPtr("-0.1"); got != nil {
		t.Fatalf("mobileCSVFloatPtr(-0.1) = %v, want nil", *got)
	}
}

type mobileResolverForTestFunc func(context.Context, string) ([]net.IPAddr, error)

func (fn mobileResolverForTestFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return fn(ctx, host)
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
	if got := intValue(probe["httping_status_code"], 0); got != 200 {
		t.Fatalf("default httping_status_code = %d, want 200", got)
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

func TestConfigToProbeConfigMapsStage3Limit(t *testing.T) {
	cfg, _ := configToProbeConfig(map[string]any{
		"probe": map[string]any{
			"strategy": "full",
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
}

func TestServiceListResultFileReadsCSVRows(t *testing.T) {
	service := NewService()
	dir := t.TempDir()
	decodeCommandForTest(t, service.Init(dir))
	path := filepath.Join(dir, "exports", "result.csv")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir exports: %v", err)
	}
	body := "IP 地址,已发送,已接收,丢包率,TCP延迟(ms),下载速度(MB/s),地区码\n1.1.1.1,4,4,0.00,12.34,56.78,HKG\n"
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
	entries, warnings, invalid, err := service.buildSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
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
	entries, _, _, err := service.buildSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
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
			entries, _, _, err := service.buildSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
			if err != nil {
				t.Fatalf("buildSourceEntriesWithConfig returned error: %v", err)
			}
			if !reflect.DeepEqual(entries, tc.want) {
				t.Fatalf("entries = %#v, want %#v", entries, tc.want)
			}
		})
	}
}

func TestServiceSourceColoFilterRequiresColoFile(t *testing.T) {
	service := NewService()
	baseDir := t.TempDir()
	decodeCommandForTest(t, service.Init(baseDir))
	if err := os.WriteFile(filepath.Join(baseDir, colodict.GeofeedFileName), []byte("ip_prefix,country,region,city,postal_code\n104.16.0.0/13,US,CA,San Jose,\n"), 0o600); err != nil {
		t.Fatalf("write geofeed file: %v", err)
	}

	source := desktopSource{
		ColoFilter: "SJC",
		Content:    "104.16.0.1",
		IPLimit:    10,
		IPMode:     "traverse",
		Kind:       "inline",
		Name:       "missing-colo",
	}
	_, _, _, err := service.buildSourceEntriesWithConfig(source.Content, source, defaultProbeConfig())
	if err == nil || !strings.Contains(err.Error(), "COLO 文件不存在") {
		t.Fatalf("err = %v, want missing COLO file error", err)
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

func TestServiceRunProbeCompletedEventUsesAndroidExportURI(t *testing.T) {
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
	if got := stringValue(payload["target_path"], ""); got != "content://exports/result.csv" {
		t.Fatalf("target_path = %q, want SAF URI", got)
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

func TestServiceCloudflareListReadsAAndAAAARecords(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() { cloudflareAPIBaseURL = oldBaseURL })

	queriedTypes := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		recordType := assertCloudflareListQueryForTest(t, r)
		queriedTypes = append(queriedTypes, recordType)
		writeCloudflareTestResponse(w, map[string]any{
			"success": true,
			"result": []CloudflareDNSRecord{
				{ID: strings.ToLower(recordType) + "-1", Type: recordType, Name: "edge.example.com", Content: "content-" + recordType, TTL: 300},
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
	if !reflect.DeepEqual(queriedTypes, []string{"A", "AAAA"}) {
		t.Fatalf("queried types = %#v, want A and AAAA", queriedTypes)
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
