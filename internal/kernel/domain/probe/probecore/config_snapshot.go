package probecore

import (
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/task"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/utils"
)

const (
	DefaultCloudflareTTL                  = 300
	DefaultConfigArchiveName              = "cfst-gui-config.zip"
	DefaultCloudflareUploadTopN           = 5
	DefaultGitHubExportBranch             = "main"
	DefaultGitHubExportCommitMessage      = "CFST results {date} {time}"
	DefaultGitHubExportOwner              = ""
	DefaultGitHubExportPathTemplate       = "cfst-results/{date}/{time}-{task_id}.csv"
	DefaultGitHubExportRepo               = ""
	DefaultGitHubUploadTopN               = 20
	DefaultThemeMode                      = "auto_system_time"
	DefaultThemeLightStart                = "07:00"
	DefaultThemeDarkStart                 = "19:00"
	DefaultUTCOffsetMinutes               = 8 * 60
	DefaultSchedulerConfigSource          = "draft_preferred"
	DefaultSchedulerSourceProfileAction   = "update_recent_run_source_profile"
	DefaultConfigSnapshotSourceIPLimit    = 500
	DefaultConfigSnapshotExportTargetFile = "result.csv"
	DefaultCompletedTaskRetentionDays     = 7
)

type ConfigSnapshotOptions struct {
	CloudflareTTL                int
	DefaultProbeDebug            bool
	DefaultExportTargetDir       string
	DefaultSourceIPLimit         int
	GitHubBranch                 string
	GitHubCommitMessageTemplate  string
	GitHubOwner                  string
	GitHubPathTemplate           string
	GitHubRepo                   string
	IncludePortPolicy            bool
	IncludeSchedulerMetadata     bool
	IncludeTheme                 bool
	Now                          time.Time
	PortPolicy                   string
	ProfileName                  string
	SchedulerConfigSource        string
	SchedulerSourceProfileAction string
	ThemeDarkStart               string
	ThemeLightStart              string
	ThemeMode                    string
	ProbeNormalizeOptions        ProbeConfigNormalizeOptions
	DefaultProbeRoutines         int
}

func DefaultConfigSnapshot(options ConfigSnapshotOptions) map[string]any {
	options = normalizeConfigSnapshotOptions(options)
	probe := map[string]any{
		"concurrency": map[string]any{
			"stage1": options.DefaultProbeRoutines,
			"stage2": task.MaxTraceRoutines,
			"stage3": 1,
		},
		"mcis": map[string]any{
			"budget":      0,
			"concurrency": 0,
		},
		"cooldown_policy": map[string]any{
			"consecutive_failures": 3,
			"cooldown_ms":          250,
		},
		"debug":                             options.DefaultProbeDebug,
		"debug_capture_address":             "",
		"debug_capture_enabled":             false,
		"debug_log_format":                  "",
		"debug_log_mode":                    utils.DebugLogModeStructured,
		"debug_log_verbosity":               utils.DebugLogVerbosityDetailed,
		"disable_download":                  true,
		"download_buffer_kb":                256,
		"download_success_limit":            0,
		"download_get_concurrency":          4,
		"download_host_header":              "",
		"download_http_protocol":            "auto",
		"download_sni":                      "",
		"download_speed_metric":             utils.DownloadSpeedMetricAverage,
		"download_speed_sample_interval_ms": 1000,
		"download_time_seconds":             4,
		"download_warmup_seconds":           1,
		"event_throttle_ms":                 100,
		"host_header":                       "",
		"verify_tls_certificate":            false,
		"trace_cf_colo":                     "",
		"trace_cf_colo_mode":                task.ColoFilterModeAllow,
		"trace_status_code":                 task.DefaultTraceStatusCode,
		"max_loss_rate":                     float64(utils.DefaultMaxLossRate),
		"min_delay_ms":                      0,
		"ping_times":                        4,
		"print_num":                         0,
		"request_headers":                   "",
		"retry_policy": map[string]any{
			"backoff_ms":   0,
			"max_attempts": 0,
		},
		"skip_first_latency_sample": true,
		"stage_limits": map[string]any{
			"stage2": 0,
			"stage3": 100,
		},
		"strategy": "fast",
		"sni":      "",
		"tcp_port": 443,
		"test_all": false,
		"thresholds": map[string]any{
			"max_tcp_latency_ms": nil,
			"min_download_mbps":  0,
		},
		"timeouts": map[string]any{
			"stage1_ms": 1000,
			"stage2_ms": 1000,
		},
		"trace_colo_mode":          task.TraceColoModeStandard,
		"trace_url":                "",
		"source_colo_filter_phase": SourceColoFilterPhasePrecheck,
		"source_pool_limit":        DefaultSourcePoolLimit,
		"doh_url":                  DefaultDoHURL,
		"ech":                      true,
		"ech_config":               "",
		"ech_domain":               DefaultECHDomain,
		"url":                      DefaultFileTestURL,
		"user_agent":               httpcfg.DefaultUserAgent,
	}
	if options.IncludePortPolicy {
		probe["port_policy"] = options.PortPolicy
	}
	ui := map[string]any{"auto_detect_source_name": true, "settings_mode": "basic"}
	if options.IncludeTheme {
		ui["theme_dark_start"] = options.ThemeDarkStart
		ui["theme_light_start"] = options.ThemeLightStart
		ui["theme_mode"] = options.ThemeMode
		ui["utc_offset_minutes"] = DefaultUTCOffsetMinutes
	}
	scheduler := map[string]any{
		"auto_dns_push":      true,
		"auto_github_export": true,
		"daily_times":        []string{},
		"enabled":            false,
		"interval_minutes":   0,
		"skip_if_active":     true,
	}
	if options.IncludeSchedulerMetadata {
		scheduler["config_source"] = options.SchedulerConfigSource
		scheduler["post_run_source_profile_action"] = options.SchedulerSourceProfileAction
	}
	return map[string]any{
		"dns_providers": []any{},
		"cloudflare": map[string]any{
			"api_token":       "",
			"comment":         "",
			"enabled":         false,
			"proxied":         false,
			"record_name":     "",
			"record_type":     "A",
			"routing_enabled": false,
			"routing_rules":   []any{},
			"top_n":           DefaultCloudflareUploadTopN,
			"ttl":             options.CloudflareTTL,
			"zone_id":         "",
		},
		"github": map[string]any{
			"branch":                  options.GitHubBranch,
			"commit_message_template": options.GitHubCommitMessageTemplate,
			"csv_header_template":     "",
			"csv_row_template":        "",
			"enabled":                 false,
			"format":                  "csv",
			"last_export_at":          "",
			"owner":                   options.GitHubOwner,
			"path_template":           options.GitHubPathTemplate,
			"repo":                    options.GitHubRepo,
			"token":                   "",
			"top_n":                   DefaultGitHubUploadTopN,
			"txt_row_template":        "{ip}",
		},
		"export": map[string]any{
			"file_name":          DefaultConfigSnapshotExportTargetFile,
			"file_name_template": "",
			"format":             "csv",
			"github": map[string]any{
				"branch":                  options.GitHubBranch,
				"commit_message_template": options.GitHubCommitMessageTemplate,
				"csv_header_template":     "",
				"csv_row_template":        "",
				"enabled":                 false,
				"format":                  "csv",
				"last_export_at":          "",
				"owner":                   options.GitHubOwner,
				"path_template":           options.GitHubPathTemplate,
				"repo":                    options.GitHubRepo,
				"token":                   "",
				"txt_row_template":        "{ip}",
			},
			"csv_encoding": utils.CSVEncodingUTF8,
			"overwrite":    "replace_on_start",
			"target_dir":   "",
			"target_uri":   "",
		},
		"post_probe_push": map[string]any{
			"cloudflare_enabled": true,
			"github_enabled":     false,
		},
		"backup": map[string]any{
			"webdav": map[string]any{
				"enabled":         false,
				"last_backup_at":  "",
				"last_restore_at": "",
				"password":        "",
				"remote_path":     DefaultConfigArchiveName,
				"server_url":      "",
				"timeout_seconds": 30,
				"username":        "",
			},
		},
		"maintenance": map[string]any{
			"completed_task_retention_days": DefaultCompletedTaskRetentionDays,
		},
		"notifications": map[string]any{
			"telegram": map[string]any{
				"api_base_url":          "",
				"bot_token":             "",
				"chat_id":               "",
				"enabled":               false,
				"include_top_n":         false,
				"personal_chat_id":      "",
				"recipient_mode":        "chat",
				"top_n":                 5,
				"top_n_recipient_mode":  "chat",
				"upload_recipient_mode": "chat",
				"use_system_proxy":      false,
			},
			"webhook": map[string]any{
				"enabled": false, "urls": []string{},
				"dingtalk_url": "", "dingtalk_secret": "",
				"wecom_urls": []string{}, "wecom_corp_id": "", "wecom_agent_id": "",
				"wecom_secret": "", "wecom_mobiles": []string{},
				"headers": map[string]any{}, "use_system_proxy": false,
			},
			"email": map[string]any{
				"enabled": false, "host": "", "port": 587,
				"username": "", "password": "", "from": "",
				"to": []string{}, "use_tls": true,
			},
		},
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"routing_enabled": false,
				"routing_rules":   []any{},
				"top_n":           DefaultCloudflareUploadTopN,
			},
			"github": map[string]any{
				"top_n": DefaultGitHubUploadTopN,
			},
			"shared_filter": map[string]any{
				"colo_allow":           "",
				"colo_deny":            "",
				"enabled":              false,
				"ip_version":           "any",
				"max_loss_rate":        nil,
				"max_tcp_latency_ms":   nil,
				"max_trace_latency_ms": nil,
				"min_download_mbps":    0,
				"status":               "passed",
			},
		},
		"probe":          probe,
		"source_ip_mode": "",
		"sources":        []map[string]any{},
		"ui":             ui,
		"scheduler":      scheduler,
	}
}

// SharedConfigSnapshotOptions returns the cross-platform snapshot defaults.
// Desktop and mobile adapters should start from this and only set platform deltas.
func SharedConfigSnapshotOptions() ConfigSnapshotOptions {
	return ConfigSnapshotOptions{
		DefaultProbeRoutines: 200,
		IncludePortPolicy:    true,
		IncludeTheme:         true,
		ProbeNormalizeOptions: ProbeConfigNormalizeOptions{
			MaxTCPRoutines:    DefaultMaxProbeTCPRoutines,
			MaxStage3Routines: DefaultMaxProbeStage3Routines,
		},
	}
}

// DesktopConfigSnapshotOptions adds desktop-only scheduler metadata to the shared defaults.
func DesktopConfigSnapshotOptions() ConfigSnapshotOptions {
	options := SharedConfigSnapshotOptions()
	options.IncludeSchedulerMetadata = true
	return options
}

// MobileConfigSnapshotOptions enables diagnostic logging by default on Android.
func MobileConfigSnapshotOptions() ConfigSnapshotOptions {
	options := SharedConfigSnapshotOptions()
	options.DefaultProbeDebug = true
	options.DefaultProbeRoutines = 64
	return options
}

func normalizeConfigSnapshotOptions(options ConfigSnapshotOptions) ConfigSnapshotOptions {
	if options.DefaultProbeRoutines <= 0 {
		options.DefaultProbeRoutines = 200
	}
	if options.CloudflareTTL <= 0 {
		options.CloudflareTTL = DefaultCloudflareTTL
	}
	if options.DefaultSourceIPLimit <= 0 {
		options.DefaultSourceIPLimit = DefaultConfigSnapshotSourceIPLimit
	}
	if strings.TrimSpace(options.GitHubBranch) == "" {
		options.GitHubBranch = DefaultGitHubExportBranch
	}
	if strings.TrimSpace(options.GitHubCommitMessageTemplate) == "" {
		options.GitHubCommitMessageTemplate = DefaultGitHubExportCommitMessage
	}
	if strings.TrimSpace(options.GitHubOwner) == "" {
		options.GitHubOwner = DefaultGitHubExportOwner
	}
	if strings.TrimSpace(options.GitHubPathTemplate) == "" {
		options.GitHubPathTemplate = DefaultGitHubExportPathTemplate
	}
	if strings.TrimSpace(options.GitHubRepo) == "" {
		options.GitHubRepo = DefaultGitHubExportRepo
	}
	if strings.TrimSpace(options.PortPolicy) == "" {
		options.PortPolicy = PortPolicyFixedGlobal
	}
	if strings.TrimSpace(options.SchedulerConfigSource) == "" {
		options.SchedulerConfigSource = DefaultSchedulerConfigSource
	}
	if strings.TrimSpace(options.SchedulerSourceProfileAction) == "" {
		options.SchedulerSourceProfileAction = DefaultSchedulerSourceProfileAction
	}
	if strings.TrimSpace(options.ThemeMode) == "" {
		options.ThemeMode = DefaultThemeMode
	}
	if strings.TrimSpace(options.ThemeLightStart) == "" {
		options.ThemeLightStart = DefaultThemeLightStart
	}
	if strings.TrimSpace(options.ThemeDarkStart) == "" {
		options.ThemeDarkStart = DefaultThemeDarkStart
	}
	options.ProbeNormalizeOptions = normalizeProbeConfigOptions(options.ProbeNormalizeOptions)
	return options
}

func (options ConfigSnapshotOptions) now() time.Time {
	if options.Now.IsZero() {
		return time.Now()
	}
	return options.Now
}

// HasCustomRequestAuthority 判断是否填过自定义 Host/SNI：字段非空即生效，
// 不再有独立的总开关。
func HasCustomRequestAuthority(cfg ProbeConfig) bool {
	return strings.TrimSpace(cfg.HostHeader) != "" || strings.TrimSpace(cfg.SNI) != "" ||
		strings.TrimSpace(cfg.DownloadHostHeader) != "" || strings.TrimSpace(cfg.DownloadSNI) != ""
}
