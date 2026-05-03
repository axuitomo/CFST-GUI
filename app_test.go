package main

import (
	"encoding/json"
	"net"
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

func TestDesktopConfigToProbeConfigClampsTraceStage(t *testing.T) {
	cfg, warnings := desktopConfigToProbeConfig(map[string]any{
		"probe": map[string]any{
			"strategy": "full",
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
	if cfg.HeadTestCount != 17 {
		t.Fatalf("HeadTestCount = %d, want 17", cfg.HeadTestCount)
	}
	if cfg.TestCount != 5 {
		t.Fatalf("TestCount = %d, want stage3 limit 5", cfg.TestCount)
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
	cfg.DownloadSpeedSampleIntervalSeconds = 0
	cfg.DownloadTimeSeconds = 0
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
		"单 IP 下载测速时间必须至少为 10 秒",
		"测速端口必须在 1-65535",
		"文件测速URL不能为空",
		"追踪延迟上限设置已停用",
		"TCP 丢包率上限最大支持 15%",
		"导出文件路径不能为空",
	} {
		if !warningsContain(warnings, want) {
			t.Fatalf("warnings = %#v, missing %q", warnings, want)
		}
	}
}

func TestNormalizeProbeConfigAllowsTenSecondDownloadTime(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.DownloadTimeSeconds = 10

	normalized, warnings := normalizeProbeConfig(cfg)
	if normalized.DownloadTimeSeconds != 10 {
		t.Fatalf("DownloadTimeSeconds = %d, want 10", normalized.DownloadTimeSeconds)
	}
	if warningsContain(warnings, "单 IP 下载测速时间必须至少为 10 秒") {
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
	if cfg.Stage1Limit != 100 {
		t.Fatalf("Stage1Limit = %d, want 100", cfg.Stage1Limit)
	}
	if cfg.CooldownFailures != 1 || cfg.CooldownMS != 500 {
		t.Fatalf("cooldown = (%d,%d), want (1,500)", cfg.CooldownFailures, cfg.CooldownMS)
	}
	if cfg.RetryBackoffMS != 100 || cfg.RetryMaxAttempts != 2 {
		t.Fatalf("retry = (%d,%d), want (100,2)", cfg.RetryBackoffMS, cfg.RetryMaxAttempts)
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

func TestDesktopMCISEngineConfigIgnoresFinalColoFilter(t *testing.T) {
	cfg := defaultProbeConfig()
	cfg.HttpingCFColo = "hkg,nrt LAX hkg zzz"

	mcisCfg := buildDesktopMCISEngineConfig(cfg, 500)

	if len(mcisCfg.ColoAllow) != 0 {
		t.Fatalf("ColoAllow = %#v, want empty because final COLO filter belongs to stage 2 only", mcisCfg.ColoAllow)
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
			"download_speed_sample_interval_seconds": cfg.DownloadSpeedSampleIntervalSeconds,
			"download_time_seconds":                  cfg.DownloadTimeSeconds,
			"event_throttle_ms":                      cfg.EventThrottleMS,
			"ping_times":                             cfg.PingTimes,
			"strategy":                               cfg.Strategy,
			"tcp_port":                               cfg.TCPPort,
			"trace_url":                              cfg.TraceURL,
			"url":                                    cfg.URL,
			"user_agent":                             cfg.UserAgent,
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
	return path
}

func parseTestIP(value string) *net.IPAddr {
	return &net.IPAddr{IP: net.ParseIP(value)}
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

func TestRunProbeFullUsesAllTraceCandidatesAndDoesNotFallbackOnDownloadFailure(t *testing.T) {
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
	cfg.WriteOutput = false

	app := NewApp()
	result, err := app.runProbe(ProbeRequest{
		Config:     cfg,
		SourceText: "1.1.1.1\n1.1.1.2\n1.1.1.3",
	}, nil)
	if err != nil {
		t.Fatalf("runProbe returned error: %v", err)
	}
	if downloadInputCount != len(sample) {
		t.Fatalf("download input count = %d, want all trace candidates %d", downloadInputCount, len(sample))
	}
	if len(result.Results) != 0 {
		t.Fatalf("result count = %d, want 0 without fallback to trace candidates", len(result.Results))
	}
	if result.Summary.Passed != 0 || result.Summary.Failed != len(sample) {
		t.Fatalf("summary = %#v, want 0 passed and %d failed", result.Summary, len(sample))
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
