package probecore

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/task"
	"github.com/axuitomo/CFST-GUI/utils"
)

const (
	DefaultCloudflareTTL                  = 300
	DefaultConfigArchiveName              = "cfst-gui-config.zip"
	DefaultGitHubExportBranch             = "main"
	DefaultGitHubExportCommitMessage      = "CFST results {date} {time}"
	DefaultGitHubExportOwner              = "axuitomo"
	DefaultGitHubExportPathTemplate       = "cfst-results/{date}/{time}-{task_id}.csv"
	DefaultGitHubExportRepo               = "CFST-GUI"
	DefaultThemeMode                      = "auto_system_time"
	DefaultThemeLightStart                = "07:00"
	DefaultThemeDarkStart                 = "19:00"
	DefaultUTCOffsetMinutes               = 8 * 60
	DefaultSchedulerConfigSource          = "draft_preferred"
	DefaultSchedulerProfileAction         = "update_recent_run_profile"
	DefaultSchedulerSourceProfileAction   = "update_recent_run_source_profile"
	DefaultConfigSnapshotSourceIPLimit    = 500
	DefaultConfigSnapshotExportTargetFile = "result.csv"
)

type ConfigSnapshotOptions struct {
	CloudflareTTL                int
	DefaultExportTargetDir       string
	DefaultSourceIPLimit         int
	GitHubBranch                 string
	GitHubCommitMessageTemplate  string
	GitHubOwner                  string
	GitHubPathTemplate           string
	GitHubRepo                   string
	IncludePortPolicy            bool
	IncludeSchedulerWorkflow     bool
	IncludeTheme                 bool
	Now                          time.Time
	PortPolicy                   string
	ProfileName                  string
	SchedulerConfigSource        string
	SchedulerProfileAction       string
	SchedulerSourceProfileAction string
	ThemeDarkStart               string
	ThemeLightStart              string
	ThemeMode                    string
	ProbeNormalizeOptions        ProbeConfigNormalizeOptions
}

var configSnapshotFieldAliases = map[string][]string{
	"api_token":                              {"apiToken"},
	"auto_detect_source_name":                {"autoDetectSourceName"},
	"auto_dns_push":                          {"autoDnsPush"},
	"auto_github_export":                     {"autoGithubExport"},
	"backoff_ms":                             {"backoffMs"},
	"colo_filter":                            {"coloFilter"},
	"colo_filter_mode":                       {"coloFilterMode"},
	"commit_message_template":                {"commitMessageTemplate"},
	"consecutive_failures":                   {"consecutiveFailures"},
	"cooldown_ms":                            {"cooldownMs"},
	"cooldown_policy":                        {"cooldownPolicy"},
	"csv_encoding":                           {"csvEncoding"},
	"daily_times":                            {"dailyTimes"},
	"debug_capture_address":                  {"debugCaptureAddress"},
	"debug_capture_enabled":                  {"debugCaptureEnabled"},
	"debug_log_format":                       {"debugLogFormat"},
	"debug_log_mode":                         {"debugLogMode"},
	"debug_log_verbosity":                    {"debugLogVerbosity"},
	"download_buffer_kb":                     {"downloadBufferKB"},
	"download_count":                         {"downloadCount", "testCount"},
	"download_get_concurrency":               {"downloadGetConcurrency"},
	"download_http_protocol":                 {"downloadHTTPProtocol"},
	"download_speed_metric":                  {"downloadSpeedMetric"},
	"download_speed_sample_interval_ms":      {"downloadSpeedSampleIntervalMs"},
	"download_speed_sample_interval_seconds": {"downloadSpeedSampleIntervalSeconds"},
	"download_time_seconds":                  {"downloadTimeSeconds"},
	"download_warmup_seconds":                {"downloadWarmupSeconds"},
	"event_throttle_ms":                      {"eventThrottleMs"},
	"file_name":                              {"fileName"},
	"file_name_template":                     {"fileNameTemplate"},
	"host_header":                            {"hostHeader"},
	"httping_cf_colo":                        {"httpingCfColo", "httpingCFColo"},
	"httping_cf_colo_mode":                   {"httpingCfColoMode", "httpingCFColoMode"},
	"httping_status_code":                    {"httpingStatusCode"},
	"interval_minutes":                       {"intervalMinutes"},
	"ip_limit":                               {"ipLimit"},
	"ip_mode":                                {"ipMode"},
	"kind":                                   {"type"},
	"last_backup_at":                         {"lastBackupAt"},
	"last_export_at":                         {"lastExportAt"},
	"last_fetched_at":                        {"lastFetchedAt"},
	"last_fetched_count":                     {"lastFetchedCount"},
	"last_restore_at":                        {"lastRestoreAt"},
	"max_attempts":                           {"maxAttempts"},
	"max_http_latency_ms":                    {"maxHttpLatencyMs"},
	"max_loss_rate":                          {"maxLossRate"},
	"max_tcp_latency_ms":                     {"maxTcpLatencyMs"},
	"min_delay_ms":                           {"minDelayMs"},
	"min_download_mbps":                      {"minDownloadMbps"},
	"name":                                   {"label"},
	"ip_version":                             {"ipVersion"},
	"path_template":                          {"pathTemplate"},
	"ping_times":                             {"pingTimes"},
	"print_num":                              {"printNum"},
	"record_name":                            {"recordName"},
	"record_type":                            {"recordType"},
	"remote_path":                            {"remotePath"},
	"request_headers":                        {"requestHeaders"},
	"retry_policy":                           {"retryPolicy"},
	"server_url":                             {"serverUrl", "url"},
	"skip_first_latency_sample":              {"skipFirstLatencySample"},
	"skip_if_active":                         {"skipIfActive"},
	"source_colo_filter_phase":               {"sourceColoFilterPhase"},
	"stage1_ms":                              {"stage1Ms"},
	"stage2_ms":                              {"stage2Ms"},
	"stage3_ms":                              {"stage3Ms"},
	"stage_limits":                           {"stageLimits"},
	"status_text":                            {"statusText"},
	"target_dir":                             {"targetDir"},
	"target_uri":                             {"targetUri"},
	"tcp_port":                               {"tcpPort"},
	"test_all":                               {"testAll"},
	"timeout_seconds":                        {"timeoutSeconds"},
	"trace_colo_mode":                        {"traceColoMode"},
	"trace_url":                              {"traceUrl"},
	"top_n":                                  {"topN"},
	"shared_filter":                          {"sharedFilter"},
	"upload":                                 {"uploadConfig", "upload_settings"},
	"user_agent":                             {"userAgent"},
	"utc_offset_minutes":                     {"utcOffsetMinutes"},
	"zone_id":                                {"zoneId"},
}

func DefaultConfigSnapshot(options ConfigSnapshotOptions) map[string]any {
	options = normalizeConfigSnapshotOptions(options)
	probe := map[string]any{
		"concurrency": map[string]any{
			"stage1": 200,
			"stage2": task.MaxTraceRoutines,
			"stage3": 1,
		},
		"cooldown_policy": map[string]any{
			"consecutive_failures": 3,
			"cooldown_ms":          250,
		},
		"debug":                                  false,
		"debug_capture_address":                  "",
		"debug_capture_enabled":                  false,
		"debug_log_format":                       "",
		"debug_log_mode":                         utils.DebugLogModeStructured,
		"debug_log_verbosity":                    utils.DebugLogVerbosityDetailed,
		"disable_download":                       true,
		"download_buffer_kb":                     256,
		"download_count":                         10,
		"download_get_concurrency":               4,
		"download_http_protocol":                 "auto",
		"download_speed_metric":                  utils.DownloadSpeedMetricAverage,
		"download_speed_sample_interval_ms":      500,
		"download_speed_sample_interval_seconds": 0,
		"download_time_seconds":                  10,
		"download_warmup_seconds":                5,
		"event_throttle_ms":                      100,
		"host_header":                            "",
		"httping":                                false,
		"httping_cf_colo":                        "",
		"httping_cf_colo_mode":                   task.ColoFilterModeAllow,
		"httping_status_code":                    0,
		"max_loss_rate":                          float64(utils.DefaultMaxLossRate),
		"min_delay_ms":                           0,
		"ping_times":                             4,
		"print_num":                              0,
		"request_headers":                        "",
		"retry_policy": map[string]any{
			"backoff_ms":   0,
			"max_attempts": 0,
		},
		"skip_first_latency_sample": true,
		"stage_limits": map[string]any{
			"stage3": 10,
		},
		"strategy": "fast",
		"sni":      "",
		"tcp_port": 443,
		"test_all": false,
		"thresholds": map[string]any{
			"max_http_latency_ms": nil,
			"max_tcp_latency_ms":  nil,
			"min_download_mbps":   0,
		},
		"timeouts": map[string]any{
			"stage1_ms": 1000,
			"stage2_ms": 1000,
			"stage3_ms": 10000,
		},
		"trace_colo_mode":          task.TraceColoModeStandard,
		"trace_url":                "",
		"source_colo_filter_phase": SourceColoFilterPhasePrecheck,
		"url":                      DefaultFileTestURL,
		"user_agent":               httpcfg.DefaultUserAgent,
	}
	if options.IncludePortPolicy {
		probe["port_policy"] = options.PortPolicy
	}
	ui := map[string]any{"auto_detect_source_name": true}
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
	if options.IncludeSchedulerWorkflow {
		scheduler["config_source"] = options.SchedulerConfigSource
		scheduler["post_run_profile_action"] = options.SchedulerProfileAction
		scheduler["post_run_source_profile_action"] = options.SchedulerSourceProfileAction
	}
	return map[string]any{
		"cloudflare": map[string]any{
			"api_token":   "",
			"comment":     "",
			"proxied":     false,
			"record_name": "",
			"record_type": "A",
			"ttl":         options.CloudflareTTL,
			"zone_id":     "",
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
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"top_n": 0,
			},
			"github": map[string]any{
				"top_n": 0,
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
		"probe": probe,
		"sources": []map[string]any{
			defaultConfigSourceConfig(0, options),
		},
		"ui":        ui,
		"scheduler": scheduler,
	}
}

func SanitizeConfigSnapshot(input map[string]any, options ConfigSnapshotOptions) map[string]any {
	options = normalizeConfigSnapshotOptions(options)
	source := configSnapshotMap(input)
	snapshot := sanitizeConfigSnapshotMap(DefaultConfigSnapshot(options), source, options)
	probeSource := configSnapshotMap(source["probe"])
	applyConfigProbeCompat(snapshot, probeSource)
	applyConfigExportCompat(snapshot, source, probeSource)
	applyConfigUploadCompat(snapshot, source)
	if !hasConfigSnapshotField(source, "sources") {
		if sourceText := legacyConfigSourceText(source, probeSource); sourceText != "" {
			sourceItem := defaultConfigSourceConfig(0, options)
			sourceItem["kind"] = "inline"
			sourceItem["content"] = sourceText
			sourceItem["name"] = "旧版输入源"
			snapshot["sources"] = []map[string]any{sourceItem}
		}
	}
	return snapshot
}

func ConfigSnapshotToProbeConfig(config map[string]any, options ConfigSnapshotOptions) (ProbeConfig, []string) {
	options = normalizeConfigSnapshotOptions(options)
	cfg := DefaultProbeConfig()
	probe := configSnapshotMap(config["probe"])
	exportCfg := configSnapshotMap(config["export"])
	concurrency := configSnapshotMap(probe["concurrency"])
	stageLimits := configSnapshotMap(firstConfigSnapshotNonNil(probe["stage_limits"], probe["stageLimits"]))
	thresholds := configSnapshotMap(probe["thresholds"])
	timeouts := configSnapshotMap(probe["timeouts"])
	cooldownPolicy := configSnapshotMap(firstConfigSnapshotNonNil(probe["cooldown_policy"], probe["cooldownPolicy"]))
	retryPolicy := configSnapshotMap(firstConfigSnapshotNonNil(probe["retry_policy"], probe["retryPolicy"]))
	warnings := make([]string, 0)
	rawStrategy := strings.ToLower(strings.TrimSpace(configSnapshotStringValue(probe["strategy"], cfg.Strategy)))
	strategy := rawStrategy
	switch strategy {
	case "speed", "exhaustive", "full":
		strategy = "full"
	case "latency", "http-colo", "fast":
		strategy = "fast"
	default:
		strategy = "fast"
	}

	cfg.Strategy = strategy
	cfg.Routines = configSnapshotIntValue(concurrency["stage1"], cfg.Routines)
	cfg.HeadRoutines = configSnapshotIntValue(concurrency["stage2"], cfg.HeadRoutines)
	cfg.PingTimes = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["ping_times"], probe["pingTimes"]), cfg.PingTimes)
	cfg.SkipFirstLatency = configSnapshotBoolValue(firstConfigSnapshotNonNil(probe["skip_first_latency_sample"], probe["skipFirstLatencySample"]), true)
	cfg.EventThrottleMS = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["event_throttle_ms"], probe["eventThrottleMs"]), cfg.EventThrottleMS)
	cfg.DownloadSpeedSampleIntervalMS = ProbeDownloadSpeedSampleIntervalMS(probe, cfg)
	cfg.DownloadGetConcurrency = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["download_get_concurrency"], probe["downloadGetConcurrency"]), cfg.DownloadGetConcurrency)
	cfg.DownloadBufferKB = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["download_buffer_kb"], probe["downloadBufferKB"]), cfg.DownloadBufferKB)
	cfg.DownloadHTTPProtocol = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["download_http_protocol"], probe["downloadHTTPProtocol"]), cfg.DownloadHTTPProtocol)
	cfg.DownloadSpeedMetric = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["download_speed_metric"], probe["downloadSpeedMetric"]), cfg.DownloadSpeedMetric)
	cfg.Stage1Limit = 0
	cfg.HeadTestCount = 0
	cfg.Stage3Limit = configSnapshotIntValue(firstConfigSnapshotNonNil(stageLimits["stage3"], probe["stage3_limit"], probe["stage3Limit"], probe["download_count"], probe["downloadCount"]), cfg.Stage3Limit)
	cfg.TestCount = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["download_count"], probe["downloadCount"], cfg.Stage3Limit), cfg.TestCount)
	cfg.Stage3Concurrency = configSnapshotIntValue(concurrency["stage3"], cfg.Stage3Concurrency)
	cfg.Stage1TimeoutMS = configSnapshotIntValue(firstConfigSnapshotNonNil(timeouts["stage1_ms"], timeouts["stage1Ms"]), cfg.Stage1TimeoutMS)
	cfg.Stage2TimeoutMS = configSnapshotIntValue(firstConfigSnapshotNonNil(timeouts["stage2_ms"], timeouts["stage2Ms"]), cfg.Stage2TimeoutMS)
	downloadTimeSeconds := configSnapshotIntValue(firstConfigSnapshotNonNil(probe["download_time_seconds"], probe["downloadTimeSeconds"]), cfg.DownloadTimeSeconds)
	if downloadTimeSeconds <= 0 {
		cfg.DownloadTimeSeconds = configSnapshotIntValue(timeouts["stage3_ms"], cfg.DownloadTimeSeconds*1000) / 1000
	} else {
		cfg.DownloadTimeSeconds = downloadTimeSeconds
	}
	cfg.DownloadWarmupSeconds = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["download_warmup_seconds"], probe["downloadWarmupSeconds"]), cfg.DownloadWarmupSeconds)
	cfg.TCPPort = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["tcp_port"], probe["tcpPort"]), cfg.TCPPort)
	cfg.PortPolicy = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["port_policy"], probe["portPolicy"]), cfg.PortPolicy)
	cfg.URL = configSnapshotStringValue(probe["url"], cfg.URL)
	cfg.TraceURL = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["trace_url"], probe["traceUrl"]), cfg.TraceURL)
	cfg.TraceColoMode = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["trace_colo_mode"], probe["traceColoMode"]), cfg.TraceColoMode)
	cfg.SourceColoFilterPhase = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["source_colo_filter_phase"], probe["sourceColoFilterPhase"]), cfg.SourceColoFilterPhase)
	cfg.UserAgent = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["user_agent"], probe["userAgent"]), cfg.UserAgent)
	cfg.HostHeader = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["host_header"], probe["hostHeader"]), cfg.HostHeader)
	cfg.SNI = configSnapshotStringValue(probe["sni"], cfg.SNI)
	cfg.RequestHeaders = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["request_headers"], probe["requestHeaders"]), cfg.RequestHeaders)
	cfg.Httping = configSnapshotBoolValue(probe["httping"], rawStrategy == "http-colo")
	cfg.HttpingStatusCode = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["httping_status_code"], probe["httpingStatusCode"]), cfg.HttpingStatusCode)
	cfg.HttpingCFColo = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["httping_cf_colo"], probe["httpingCfColo"]), cfg.HttpingCFColo)
	cfg.HttpingCFColoMode = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["httping_cf_colo_mode"], probe["httpingCfColoMode"]), cfg.HttpingCFColoMode)
	cfg.MaxDelayMS = configSnapshotIntValue(thresholds["max_tcp_latency_ms"], cfg.MaxDelayMS)
	cfg.HeadMaxDelayMS = configSnapshotIntValue(thresholds["max_http_latency_ms"], cfg.HeadMaxDelayMS)
	cfg.MinDelayMS = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["min_delay_ms"], probe["minDelayMs"]), cfg.MinDelayMS)
	cfg.MaxLossRate = configSnapshotFloatValue(firstConfigSnapshotNonNil(probe["max_loss_rate"], probe["maxLossRate"]), cfg.MaxLossRate)
	cfg.MinSpeedMB = configSnapshotFloatValue(thresholds["min_download_mbps"], cfg.MinSpeedMB)
	cfg.PrintNum = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["print_num"], probe["printNum"]), cfg.PrintNum)
	cfg.DisableDownload = strategy == "fast"
	cfg.TestAll = false
	cfg.RetryMaxAttempts = configSnapshotIntValue(firstConfigSnapshotNonNil(retryPolicy["max_attempts"], retryPolicy["maxAttempts"]), cfg.RetryMaxAttempts)
	cfg.RetryBackoffMS = configSnapshotIntValue(firstConfigSnapshotNonNil(retryPolicy["backoff_ms"], retryPolicy["backoffMs"]), cfg.RetryBackoffMS)
	cfg.CooldownFailures = configSnapshotIntValue(firstConfigSnapshotNonNil(cooldownPolicy["consecutive_failures"], cooldownPolicy["consecutiveFailures"]), cfg.CooldownFailures)
	cfg.CooldownMS = configSnapshotIntValue(firstConfigSnapshotNonNil(cooldownPolicy["cooldown_ms"], cooldownPolicy["cooldownMs"]), cfg.CooldownMS)
	cfg.Debug = configSnapshotBoolValue(probe["debug"], cfg.Debug)
	cfg.DebugCaptureAddress = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["debug_capture_address"], probe["debugCaptureAddress"]), cfg.DebugCaptureAddress)
	cfg.DebugCaptureEnabled = configSnapshotBoolValue(firstConfigSnapshotNonNil(probe["debug_capture_enabled"], probe["debugCaptureEnabled"]), strings.TrimSpace(cfg.DebugCaptureAddress) != "")
	cfg.DebugLogMode = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["debug_log_mode"], probe["debugLogMode"]), cfg.DebugLogMode)
	cfg.DebugLogFormat = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["debug_log_format"], probe["debugLogFormat"]), cfg.DebugLogFormat)
	cfg.DebugLogVerbosity = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["debug_log_verbosity"], probe["debugLogVerbosity"]), cfg.DebugLogVerbosity)

	if strategy == "fast" {
		cfg.MinSpeedMB = 0
	} else {
		cfg.DisableDownload = false
	}
	if fileName := ExportFileName(exportCfg, "", options.ProfileName, options.now()); fileName != "" {
		cfg.OutputFile = ExportPath(exportCfg, fileName, options.DefaultExportTargetDir)
		cfg.WriteOutput = true
	}
	cfg.ExportAppend = strings.EqualFold(strings.TrimSpace(configSnapshotStringValue(exportCfg["overwrite"], "")), "append")
	cfg.CSVEncoding = configSnapshotStringValue(firstConfigSnapshotNonNil(exportCfg["csv_encoding"], exportCfg["csvEncoding"]), cfg.CSVEncoding)

	normalized, normalizeWarnings := NormalizeProbeConfig(cfg, options.ProbeNormalizeOptions)
	warnings = append(warnings, normalizeWarnings...)
	return normalized, DedupeStrings(warnings)
}

func ProbeDownloadSpeedSampleIntervalMS(probe map[string]any, fallback ProbeConfig) int {
	if value := firstConfigSnapshotNonNil(probe["download_speed_sample_interval_ms"], probe["downloadSpeedSampleIntervalMs"]); value != nil {
		return configSnapshotIntValue(value, fallback.DownloadSpeedSampleIntervalMS)
	}
	if value := firstConfigSnapshotNonNil(probe["download_speed_sample_interval_seconds"], probe["downloadSpeedSampleIntervalSeconds"]); value != nil {
		return configSnapshotIntValue(value, 0) * 1000
	}
	return fallback.DownloadSpeedSampleIntervalMS
}

func ExportFileName(exportCfg map[string]any, taskID, profileName string, now time.Time) string {
	if template := strings.TrimSpace(configSnapshotStringValue(firstConfigSnapshotNonNil(exportCfg["file_name_template"], exportCfg["fileNameTemplate"]), "")); template != "" {
		if fileName := RenderExportFileTemplate(template, taskID, profileName, now); fileName != "" {
			return fileName
		}
	}
	return SanitizeTemplateFileName(configSnapshotStringValue(firstConfigSnapshotNonNil(exportCfg["file_name"], exportCfg["fileName"]), ""))
}

func ExportPath(exportCfg map[string]any, fileName string, fallbackTargetDir string) string {
	targetDir := strings.TrimSpace(configSnapshotStringValue(firstConfigSnapshotNonNil(exportCfg["target_dir"], exportCfg["targetDir"]), ""))
	if targetDir == "" {
		targetDir = fallbackTargetDir
	}
	if targetDir == "" {
		return fileName
	}
	return filepath.Join(targetDir, fileName)
}

func SanitizeTemplateFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	value = replacer.Replace(value)
	value = strings.TrimSpace(value)
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	return value
}

func RenderExportFileTemplate(template, taskID, profileName string, now time.Time) string {
	template = strings.TrimSpace(template)
	if template == "" {
		return ""
	}
	if profileName == "" {
		profileName = "default"
	}
	replacements := map[string]string{
		"{date}":    now.Format("20060102"),
		"{profile}": SanitizeTemplateFileName(profileName),
		"{task_id}": SanitizeTemplateFileName(taskID),
		"{time}":    now.Format("150405"),
	}
	for key, value := range replacements {
		template = strings.ReplaceAll(template, key, value)
	}
	return SanitizeTemplateFileName(template)
}

func sanitizeConfigSnapshotMap(schema map[string]any, source map[string]any, options ConfigSnapshotOptions) map[string]any {
	result := make(map[string]any, len(schema))
	for key, defaultValue := range schema {
		value, exists := configSnapshotFieldValue(source, key)
		if !exists || value == nil {
			result[key] = cloneConfigSnapshotValue(defaultValue)
			continue
		}
		switch typedDefault := defaultValue.(type) {
		case map[string]any:
			result[key] = sanitizeConfigSnapshotMap(typedDefault, configSnapshotMap(value), options)
		case []map[string]any:
			if key == "sources" {
				result[key] = sanitizeConfigSnapshotSources(value, options)
			} else {
				result[key] = cloneConfigSnapshotValue(defaultValue)
			}
		case []string:
			result[key] = configSnapshotStringSlice(value)
		default:
			result[key] = value
		}
	}
	return result
}

func applyConfigProbeCompat(snapshot map[string]any, probeSource map[string]any) {
	probe := configSnapshotMap(snapshot["probe"])
	concurrency := configSnapshotMap(probe["concurrency"])
	concurrencySource := configSnapshotMap(firstExistingConfigSnapshotValue(probeSource, "concurrency"))
	setConfigFieldFromLegacy(concurrency, "stage1", concurrencySource, probeSource, "routines")
	setConfigFieldFromLegacy(concurrency, "stage2", concurrencySource, probeSource, "headRoutines")
	setConfigFieldFromLegacy(concurrency, "stage3", concurrencySource, probeSource, "stage3Concurrency")
	probe["concurrency"] = concurrency

	stageLimits := configSnapshotMap(probe["stage_limits"])
	stageLimitsSource := configSnapshotMap(firstExistingConfigSnapshotValue(probeSource, "stage_limits"))
	setConfigFieldFromLegacy(stageLimits, "stage3", stageLimitsSource, probeSource, "stage3_limit", "stage3Limit", "download_count", "downloadCount", "testCount")
	probe["stage_limits"] = stageLimits

	timeouts := configSnapshotMap(probe["timeouts"])
	timeoutsSource := configSnapshotMap(firstExistingConfigSnapshotValue(probeSource, "timeouts"))
	setConfigFieldFromLegacy(timeouts, "stage1_ms", timeoutsSource, probeSource, "stage1TimeoutMs", "stage1TimeoutMS")
	setConfigFieldFromLegacy(timeouts, "stage2_ms", timeoutsSource, probeSource, "stage2TimeoutMs", "stage2TimeoutMS")
	probe["timeouts"] = timeouts

	thresholds := configSnapshotMap(probe["thresholds"])
	thresholdsSource := configSnapshotMap(firstExistingConfigSnapshotValue(probeSource, "thresholds"))
	setConfigFieldFromLegacy(thresholds, "max_tcp_latency_ms", thresholdsSource, probeSource, "maxDelayMS", "maxDelayMs")
	setConfigFieldFromLegacy(thresholds, "max_http_latency_ms", thresholdsSource, probeSource, "headMaxDelayMS", "headMaxDelayMs")
	setConfigFieldFromLegacy(thresholds, "min_download_mbps", thresholdsSource, probeSource, "minSpeedMB", "minSpeedMb")
	probe["thresholds"] = thresholds

	retryPolicy := configSnapshotMap(probe["retry_policy"])
	retrySource := configSnapshotMap(firstExistingConfigSnapshotValue(probeSource, "retry_policy"))
	setConfigFieldFromLegacy(retryPolicy, "max_attempts", retrySource, probeSource, "retryMaxAttempts")
	setConfigFieldFromLegacy(retryPolicy, "backoff_ms", retrySource, probeSource, "retryBackoffMs", "retryBackoffMS")
	probe["retry_policy"] = retryPolicy

	cooldownPolicy := configSnapshotMap(probe["cooldown_policy"])
	cooldownSource := configSnapshotMap(firstExistingConfigSnapshotValue(probeSource, "cooldown_policy"))
	setConfigFieldFromLegacy(cooldownPolicy, "consecutive_failures", cooldownSource, probeSource, "cooldownFailures")
	setConfigFieldFromLegacy(cooldownPolicy, "cooldown_ms", cooldownSource, probeSource, "cooldownMs", "cooldownMS")
	probe["cooldown_policy"] = cooldownPolicy

	if !hasConfigSnapshotField(probeSource, "download_speed_sample_interval_ms") {
		if value, ok := lookupConfigSnapshotValue(probeSource, "download_speed_sample_interval_seconds", "downloadSpeedSampleIntervalSeconds"); ok {
			seconds := configSnapshotIntValue(value, 0)
			if seconds > 0 {
				probe["download_speed_sample_interval_ms"] = seconds * 1000
			}
		}
	}

	snapshot["probe"] = probe
}

func applyConfigExportCompat(snapshot map[string]any, snapshotSource map[string]any, probeSource map[string]any) {
	exportConfig := configSnapshotMap(snapshot["export"])
	exportSource := configSnapshotMap(firstExistingConfigSnapshotValue(snapshotSource, "export"))
	setConfigFieldFromLegacy(exportConfig, "csv_encoding", exportSource, probeSource, "csvEncoding")
	if !hasConfigSnapshotField(exportSource, "file_name") {
		if outputFile, ok := lookupConfigSnapshotValue(probeSource, "outputFile"); ok {
			if fileName := strings.TrimSpace(filepath.Base(configSnapshotStringValue(outputFile, ""))); fileName != "" && fileName != "." {
				exportConfig["file_name"] = fileName
			}
		}
	}
	if !hasConfigSnapshotField(exportSource, "overwrite") {
		if appendValue, ok := lookupConfigSnapshotValue(probeSource, "exportAppend"); ok && configSnapshotBoolValue(appendValue, false) {
			exportConfig["overwrite"] = "append"
		}
	}
	githubConfig := configSnapshotMap(exportConfig["github"])
	githubSource := configSnapshotMap(firstExistingConfigSnapshotValue(exportSource, "github"))
	setConfigFieldFromLegacy(githubConfig, "format", githubSource, exportSource, "githubFormat")
	setConfigFieldFromLegacy(githubConfig, "csv_header_template", githubSource, exportSource, "csvHeaderTemplate", "githubCSVHeaderTemplate")
	setConfigFieldFromLegacy(githubConfig, "csv_row_template", githubSource, exportSource, "csvRowTemplate", "githubCSVRowTemplate")
	setConfigFieldFromLegacy(githubConfig, "txt_row_template", githubSource, exportSource, "txtRowTemplate", "githubTXTRowTemplate")
	exportConfig["github"] = githubConfig
	snapshot["export"] = exportConfig
}

func applyConfigUploadCompat(snapshot map[string]any, snapshotSource map[string]any) {
	uploadConfig := configSnapshotMap(snapshot["upload"])
	uploadSource := configSnapshotMap(firstExistingConfigSnapshotValue(snapshotSource, "upload"))
	sharedFilter := configSnapshotMap(uploadConfig["shared_filter"])
	sharedFilterSource := configSnapshotMap(firstExistingConfigSnapshotValue(uploadSource, "shared_filter"))
	setConfigFieldFromLegacy(sharedFilter, "enabled", sharedFilterSource, uploadSource, "enabled")
	setConfigFieldFromLegacy(sharedFilter, "status", sharedFilterSource, uploadSource, "status")
	setConfigFieldFromLegacy(sharedFilter, "ip_version", sharedFilterSource, uploadSource, "ipVersion")
	setConfigFieldFromLegacy(sharedFilter, "colo_allow", sharedFilterSource, uploadSource, "coloAllow")
	setConfigFieldFromLegacy(sharedFilter, "colo_deny", sharedFilterSource, uploadSource, "coloDeny")
	setConfigFieldFromLegacy(sharedFilter, "max_tcp_latency_ms", sharedFilterSource, uploadSource, "maxTcpLatencyMs")
	setConfigFieldFromLegacy(sharedFilter, "max_trace_latency_ms", sharedFilterSource, uploadSource, "maxTraceLatencyMs")
	setConfigFieldFromLegacy(sharedFilter, "min_download_mbps", sharedFilterSource, uploadSource, "minDownloadMbps")
	setConfigFieldFromLegacy(sharedFilter, "max_loss_rate", sharedFilterSource, uploadSource, "maxLossRate")
	uploadConfig["shared_filter"] = sharedFilter

	cloudflareCfg := configSnapshotMap(uploadConfig["cloudflare"])
	cloudflareSource := configSnapshotMap(firstExistingConfigSnapshotValue(uploadSource, "cloudflare"))
	setConfigFieldFromLegacy(cloudflareCfg, "top_n", cloudflareSource, uploadSource, "cloudflareTopN")
	uploadConfig["cloudflare"] = cloudflareCfg

	githubCfg := configSnapshotMap(uploadConfig["github"])
	githubSource := configSnapshotMap(firstExistingConfigSnapshotValue(uploadSource, "github"))
	setConfigFieldFromLegacy(githubCfg, "top_n", githubSource, uploadSource, "githubTopN")
	uploadConfig["github"] = githubCfg

	snapshot["upload"] = uploadConfig
}

func sanitizeConfigSnapshotSources(value any, options ConfigSnapshotOptions) []map[string]any {
	items, ok := configSnapshotSlice(value)
	if !ok {
		return []map[string]any{}
	}
	sources := make([]map[string]any, 0, len(items))
	for index, item := range items {
		source := configSnapshotMap(item)
		sources = append(sources, sanitizeConfigSnapshotMap(defaultConfigSourceConfig(index, options), source, options))
	}
	return sources
}

func defaultConfigSourceConfig(index int, options ConfigSnapshotOptions) map[string]any {
	return map[string]any{
		"content":            "",
		"colo_filter":        "",
		"colo_filter_mode":   task.ColoFilterModeAllow,
		"enabled":            true,
		"id":                 fmt.Sprintf("source-%d", index+1),
		"ip_limit":           options.DefaultSourceIPLimit,
		"ip_mode":            "traverse",
		"kind":               "url",
		"last_fetched_at":    "",
		"last_fetched_count": 0,
		"name":               fmt.Sprintf("输入源 %d", index+1),
		"path":               "",
		"status_text":        "",
		"url":                "",
	}
}

func legacyConfigSourceText(snapshot map[string]any, probe map[string]any) string {
	for _, candidate := range []any{
		snapshot["sourceText"],
		snapshot["source_text"],
		probe["ipText"],
		probe["ip_text"],
	} {
		if text := strings.TrimSpace(configSnapshotStringValue(candidate, "")); text != "" {
			return text
		}
	}
	return ""
}

func setConfigFieldFromLegacy(target map[string]any, key string, currentSource map[string]any, legacySource map[string]any, legacyKeys ...string) {
	if hasConfigSnapshotField(currentSource, key) {
		return
	}
	if value, ok := lookupConfigSnapshotValue(legacySource, legacyKeys...); ok && value != nil {
		target[key] = value
	}
}

func firstExistingConfigSnapshotValue(source map[string]any, key string) any {
	value, _ := configSnapshotFieldValue(source, key)
	return value
}

func hasConfigSnapshotField(source map[string]any, key string) bool {
	_, ok := configSnapshotFieldValue(source, key)
	return ok
}

func configSnapshotFieldValue(source map[string]any, key string) (any, bool) {
	keys := append([]string{key}, configSnapshotFieldAliases[key]...)
	return lookupConfigSnapshotValue(source, keys...)
}

func lookupConfigSnapshotValue(source map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, exists := source[key]; exists {
			return value, true
		}
	}
	return nil, false
}

func configSnapshotMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if value == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]any{}
	}
	if result == nil {
		return map[string]any{}
	}
	return result
}

func configSnapshotSlice(value any) ([]any, bool) {
	if value == nil {
		return nil, false
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var result []any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, false
	}
	if result == nil {
		return []any{}, true
	}
	return result, true
}

func configSnapshotStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case string:
		return splitConfigSnapshotStrings(typed)
	}
	items, ok := configSnapshotSlice(value)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(configSnapshotStringValue(item, "")); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func splitConfigSnapshotStrings(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if text := strings.TrimSpace(part); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func cloneConfigSnapshotValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneConfigSnapshotMap(typed)
	case []map[string]any:
		cloned := make([]map[string]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneConfigSnapshotMap(item)
		}
		return cloned
	case []string:
		return append([]string{}, typed...)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneConfigSnapshotValue(item)
		}
		return cloned
	default:
		return typed
	}
}

func cloneConfigSnapshotMap(value map[string]any) map[string]any {
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = cloneConfigSnapshotValue(item)
	}
	return cloned
}

func firstConfigSnapshotNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func configSnapshotIntValue(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func configSnapshotFloatValue(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err == nil {
			return parsed
		}
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func configSnapshotBoolValue(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	}
	return fallback
}

func configSnapshotStringValue(value any, fallback string) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case nil:
		return fallback
	default:
		return fmt.Sprint(value)
	}
}

func normalizeConfigSnapshotOptions(options ConfigSnapshotOptions) ConfigSnapshotOptions {
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
		options.PortPolicy = PortPolicySourceOverrideGlobal
	}
	if strings.TrimSpace(options.SchedulerConfigSource) == "" {
		options.SchedulerConfigSource = DefaultSchedulerConfigSource
	}
	if strings.TrimSpace(options.SchedulerProfileAction) == "" {
		options.SchedulerProfileAction = DefaultSchedulerProfileAction
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
