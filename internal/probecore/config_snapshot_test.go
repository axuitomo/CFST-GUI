package probecore

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfigSnapshotPlatformOptions(t *testing.T) {
	desktop := DefaultConfigSnapshot(ConfigSnapshotOptions{
		IncludePortPolicy:        true,
		IncludeSchedulerWorkflow: true,
		IncludeTheme:             true,
	})
	desktopProbe := testConfigMap(t, desktop["probe"])
	if got := desktopProbe["port_policy"]; got != PortPolicySourceOverrideGlobal {
		t.Fatalf("desktop port_policy = %#v, want %q", got, PortPolicySourceOverrideGlobal)
	}
	desktopScheduler := testConfigMap(t, desktop["scheduler"])
	if got := desktopScheduler["config_source"]; got != DefaultSchedulerConfigSource {
		t.Fatalf("desktop scheduler config_source = %#v, want %q", got, DefaultSchedulerConfigSource)
	}
	desktopUI := testConfigMap(t, desktop["ui"])
	if got := desktopUI["theme_mode"]; got != DefaultThemeMode {
		t.Fatalf("desktop theme_mode = %#v, want %q", got, DefaultThemeMode)
	}
	if got := desktopUI["utc_offset_minutes"]; got != DefaultUTCOffsetMinutes {
		t.Fatalf("desktop utc_offset_minutes = %#v, want %d", got, DefaultUTCOffsetMinutes)
	}

	mobile := DefaultConfigSnapshot(ConfigSnapshotOptions{})
	mobileProbe := testConfigMap(t, mobile["probe"])
	if _, ok := mobileProbe["port_policy"]; ok {
		t.Fatalf("mobile default unexpectedly contains port_policy")
	}
	mobileScheduler := testConfigMap(t, mobile["scheduler"])
	if _, ok := mobileScheduler["config_source"]; ok {
		t.Fatalf("mobile default unexpectedly contains scheduler config_source")
	}
	mobileUI := testConfigMap(t, mobile["ui"])
	if _, ok := mobileUI["theme_mode"]; ok {
		t.Fatalf("mobile default unexpectedly contains theme_mode")
	}
	if _, ok := mobileUI["utc_offset_minutes"]; ok {
		t.Fatalf("mobile default unexpectedly contains utc_offset_minutes")
	}
}

func TestSanitizeConfigSnapshotLegacySourceText(t *testing.T) {
	snapshot := SanitizeConfigSnapshot(map[string]any{
		"sourceText": "1.1.1.1\n8.8.8.8",
	}, ConfigSnapshotOptions{
		IncludePortPolicy: true,
		PortPolicy:        PortPolicySourceOverrideGlobal,
	})

	sources, ok := snapshot["sources"].([]map[string]any)
	if !ok || len(sources) != 1 {
		t.Fatalf("sources = %#v, want one source", snapshot["sources"])
	}
	source := sources[0]
	if source["kind"] != "inline" {
		t.Fatalf("source kind = %#v, want inline", source["kind"])
	}
	if source["content"] != "1.1.1.1\n8.8.8.8" {
		t.Fatalf("source content = %#v", source["content"])
	}
}

func TestConfigSnapshotToProbeConfigMapsLegacySanitizedFields(t *testing.T) {
	snapshot := SanitizeConfigSnapshot(map[string]any{
		"export": map[string]any{
			"fileName":    "legacy.csv",
			"csvEncoding": "utf-8-bom",
		},
		"probe": map[string]any{
			"downloadCount":          7,
			"downloadSpeedMetric":    "max",
			"eventThrottleMs":        250,
			"headMaxDelayMS":         123,
			"headRoutines":           3,
			"maxDelayMS":             456,
			"minSpeedMB":             1.5,
			"pingTimes":              5,
			"routines":               321,
			"skipFirstLatencySample": true,
			"stage1TimeoutMS":        1500,
			"stage2TimeoutMS":        2500,
			"strategy":               "speed",
			"tcpPort":                2053,
			"port_policy":            PortPolicyFixedGlobal,
			"url":                    "https://download.example.com/file.bin",
		},
	}, ConfigSnapshotOptions{
		IncludePortPolicy: true,
		PortPolicy:        PortPolicySourceOverrideGlobal,
	})

	cfg, warnings := ConfigSnapshotToProbeConfig(snapshot, ConfigSnapshotOptions{
		DefaultExportTargetDir: "/tmp/cfst",
	})
	if cfg.Strategy != "full" || cfg.DisableDownload {
		t.Fatalf("strategy = %q disable=%v, want full with download", cfg.Strategy, cfg.DisableDownload)
	}
	if cfg.Routines != 321 || cfg.HeadRoutines != 3 {
		t.Fatalf("routines = %d/%d, want 321/3", cfg.Routines, cfg.HeadRoutines)
	}
	if cfg.Stage3Limit != 7 || cfg.TestCount != 7 {
		t.Fatalf("stage3/test count = %d/%d, want 7/7", cfg.Stage3Limit, cfg.TestCount)
	}
	if cfg.MaxDelayMS != 456 || cfg.HeadMaxDelayMS != 0 || cfg.MinSpeedMB != 1.5 {
		t.Fatalf("thresholds = %#v", cfg)
	}
	if cfg.TCPPort != 2053 || cfg.EventThrottleMS != 250 {
		t.Fatalf("tcp/event = %d/%d, want 2053/250", cfg.TCPPort, cfg.EventThrottleMS)
	}
	if cfg.PortPolicy != PortPolicyFixedGlobal {
		t.Fatalf("PortPolicy = %q, want %q", cfg.PortPolicy, PortPolicyFixedGlobal)
	}
	if cfg.CSVEncoding != "utf-8-bom" {
		t.Fatalf("CSVEncoding = %q, want utf-8-bom", cfg.CSVEncoding)
	}
	if cfg.OutputFile != filepath.Join("/tmp/cfst", "legacy.csv") {
		t.Fatalf("OutputFile = %q", cfg.OutputFile)
	}
	if !configSnapshotWarningsContain(warnings, "追踪延迟上限设置已停用") {
		t.Fatalf("warnings = %#v, want disabled trace latency warning", warnings)
	}
}

func TestConfigSnapshotToProbeConfigExportTemplateAndSampleInterval(t *testing.T) {
	now := time.Date(2026, 5, 14, 1, 2, 3, 0, time.UTC)
	snapshot := SanitizeConfigSnapshot(map[string]any{
		"export": map[string]any{
			"file_name_template": "result/{date}/{time}-{task_id}-{profile}.csv",
			"target_dir":         "/exports",
		},
		"probe": map[string]any{
			"download_speed_sample_interval_seconds": 2,
			"event_throttle_ms":                      150,
		},
	}, ConfigSnapshotOptions{})

	cfg, warnings := ConfigSnapshotToProbeConfig(snapshot, ConfigSnapshotOptions{
		Now:         now,
		ProfileName: "A:B",
	})
	if cfg.DownloadSpeedSampleIntervalMS != 2000 {
		t.Fatalf("DownloadSpeedSampleIntervalMS = %d, want 2000", cfg.DownloadSpeedSampleIntervalMS)
	}
	if cfg.EventThrottleMS != 150 {
		t.Fatalf("EventThrottleMS = %d, want 150", cfg.EventThrottleMS)
	}
	wantOutput := filepath.Join("/exports", "result_20260514_010203--A_B.csv")
	if cfg.OutputFile != wantOutput {
		t.Fatalf("OutputFile = %q, want %q", cfg.OutputFile, wantOutput)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}

func TestSanitizeConfigSnapshotAddsGitHubTemplateDefaults(t *testing.T) {
	snapshot := SanitizeConfigSnapshot(map[string]any{
		"export": map[string]any{
			"github": map[string]any{
				"format":              "txt",
				"csv_row_template":    "{ip},{test_port}",
				"txt_row_template":    "{ip}:{test_port}",
				"csv_header_template": "IP,PORT",
			},
		},
	}, ConfigSnapshotOptions{})
	exportCfg := testConfigMap(t, snapshot["export"])
	githubCfg := testConfigMap(t, exportCfg["github"])
	if got := githubCfg["format"]; got != "txt" {
		t.Fatalf("format = %#v, want txt", got)
	}
	if got := githubCfg["csv_header_template"]; got != "IP,PORT" {
		t.Fatalf("csv_header_template = %#v", got)
	}
	if got := githubCfg["csv_row_template"]; got != "{ip},{test_port}" {
		t.Fatalf("csv_row_template = %#v", got)
	}
	if got := githubCfg["txt_row_template"]; got != "{ip}:{test_port}" {
		t.Fatalf("txt_row_template = %#v", got)
	}
}

func TestSanitizeConfigSnapshotMigratesProviderConfigs(t *testing.T) {
	snapshot := SanitizeConfigSnapshot(map[string]any{
		"export": map[string]any{
			"github": map[string]any{
				"enabled":       true,
				"owner":         "legacy-owner",
				"repo":          "legacy-repo",
				"token":         "legacy-token",
				"path_template": "legacy/{task_id}.csv",
			},
		},
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"routing_enabled": true,
				"routing_rules": []map[string]any{
					{"enabled": true, "record_name": "jp.example.com"},
				},
				"top_n": 3,
			},
			"github": map[string]any{
				"top_n": 5,
			},
		},
	}, ConfigSnapshotOptions{})

	github := testConfigMap(t, snapshot["github"])
	if github["owner"] != "legacy-owner" || github["repo"] != "legacy-repo" || github["top_n"] != 5 {
		t.Fatalf("github = %#v, want migrated legacy export/upload github", github)
	}
	export := testConfigMap(t, snapshot["export"])
	exportGithub := testConfigMap(t, export["github"])
	if !reflect.DeepEqual(exportGithub, github) {
		t.Fatalf("export.github = %#v, want mirror of root github %#v", exportGithub, github)
	}

	cloudflare := testConfigMap(t, snapshot["cloudflare"])
	if cloudflare["routing_enabled"] != true || cloudflare["top_n"] != 3 {
		t.Fatalf("cloudflare = %#v, want migrated routing/top_n", cloudflare)
	}
	upload := testConfigMap(t, snapshot["upload"])
	uploadCloudflare := testConfigMap(t, upload["cloudflare"])
	if uploadCloudflare["routing_enabled"] != true || uploadCloudflare["top_n"] != 3 {
		t.Fatalf("upload.cloudflare = %#v, want legacy mirror retained", uploadCloudflare)
	}
}

func TestSanitizeConfigSnapshotRootProviderConfigTakesPriority(t *testing.T) {
	snapshot := SanitizeConfigSnapshot(map[string]any{
		"github": map[string]any{
			"enabled": true,
			"owner":   "root-owner",
			"repo":    "root-repo",
			"token":   "root-token",
			"top_n":   9,
		},
		"export": map[string]any{
			"github": map[string]any{
				"enabled": false,
				"owner":   "legacy-owner",
				"repo":    "legacy-repo",
				"token":   "legacy-token",
			},
		},
		"cloudflare": map[string]any{
			"enabled":         true,
			"routing_enabled": true,
			"top_n":           7,
		},
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"routing_enabled": false,
				"top_n":           1,
			},
			"github": map[string]any{
				"top_n": 2,
			},
		},
	}, ConfigSnapshotOptions{})

	github := testConfigMap(t, snapshot["github"])
	if github["owner"] != "root-owner" || github["repo"] != "root-repo" || github["enabled"] != true || github["top_n"] != 9 {
		t.Fatalf("github = %#v, want root provider priority", github)
	}
	export := testConfigMap(t, snapshot["export"])
	if exportGithub := testConfigMap(t, export["github"]); exportGithub["owner"] != "root-owner" || exportGithub["top_n"] != 9 {
		t.Fatalf("export.github = %#v, want mirror of root github", exportGithub)
	}
	cloudflare := testConfigMap(t, snapshot["cloudflare"])
	if cloudflare["enabled"] != true || cloudflare["routing_enabled"] != true || cloudflare["top_n"] != 7 {
		t.Fatalf("cloudflare = %#v, want root cloudflare priority", cloudflare)
	}
}

func TestExportTemplateHelpers(t *testing.T) {
	now := time.Date(2026, 5, 14, 1, 2, 3, 0, time.UTC)
	fileName := ExportFileName(map[string]any{
		"fileNameTemplate": `res/{date}/{time}-{task_id}-{profile}.csv`,
	}, "task/1", "P:1", now)
	if fileName != "res_20260514_010203-task_1-P_1.csv" {
		t.Fatalf("fileName = %q", fileName)
	}

	got := ExportPath(map[string]any{"targetDir": "/tmp/out"}, fileName, "/fallback")
	if got != filepath.Join("/tmp/out", fileName) {
		t.Fatalf("ExportPath = %q", got)
	}
	got = ExportPath(map[string]any{}, fileName, "/fallback")
	if got != filepath.Join("/fallback", fileName) {
		t.Fatalf("ExportPath fallback = %q", got)
	}
}

func TestConfigSnapshotStringListsAndAliases(t *testing.T) {
	snapshot := SanitizeConfigSnapshot(map[string]any{
		"scheduler": map[string]any{
			"dailyTimes": "01:00,02:00; 03:00",
		},
	}, ConfigSnapshotOptions{})
	scheduler := testConfigMap(t, snapshot["scheduler"])
	got, ok := scheduler["daily_times"].([]string)
	if !ok {
		t.Fatalf("daily_times = %#v, want []string", scheduler["daily_times"])
	}
	want := []string{"01:00", "02:00", "03:00"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("daily_times = %#v, want %#v", got, want)
	}
}

func testConfigMap(t *testing.T, value any) map[string]any {
	t.Helper()
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value = %#v, want map[string]any", value)
	}
	return result
}

func configSnapshotWarningsContain(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}
