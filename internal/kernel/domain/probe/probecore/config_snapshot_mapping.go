package probecore

import (
	"path/filepath"
	"strings"
	"time"
)

func ConfigSnapshotToProbeConfig(config map[string]any, options ConfigSnapshotOptions) (ProbeConfig, []string) {
	options = normalizeConfigSnapshotOptions(options)
	cfg := DefaultProbeConfig()
	probe := configSnapshotMap(config["probe"])
	exportCfg := configSnapshotMap(config["export"])
	mcis := configSnapshotMap(probe["mcis"])
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
	case "latency", "fast":
		strategy = "fast"
	default:
		strategy = "fast"
	}

	cfg.Strategy = strategy
	cfg.Routines = configSnapshotIntValue(concurrency["stage1"], cfg.Routines)
	cfg.HeadRoutines = configSnapshotIntValue(concurrency["stage2"], cfg.HeadRoutines)
	cfg.MCISBudget = configSnapshotIntValue(firstConfigSnapshotNonNil(mcis["budget"], mcis["budgetOverride"]), cfg.MCISBudget)
	cfg.MCISConcurrency = configSnapshotIntValue(firstConfigSnapshotNonNil(mcis["concurrency"], mcis["concurrencyOverride"]), cfg.MCISConcurrency)
	cfg.PingTimes = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["ping_times"], probe["pingTimes"]), cfg.PingTimes)
	cfg.SkipFirstLatency = configSnapshotBoolValue(firstConfigSnapshotNonNil(probe["skip_first_latency_sample"], probe["skipFirstLatencySample"]), true)
	cfg.EventThrottleMS = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["event_throttle_ms"], probe["eventThrottleMs"]), cfg.EventThrottleMS)
	cfg.DownloadSpeedSampleIntervalMS = ProbeDownloadSpeedSampleIntervalMS(probe, cfg)
	cfg.DownloadGetConcurrency = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["download_get_concurrency"], probe["downloadGetConcurrency"]), cfg.DownloadGetConcurrency)
	cfg.DownloadBufferKB = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["download_buffer_kb"], probe["downloadBufferKB"]), cfg.DownloadBufferKB)
	cfg.DownloadHTTPProtocol = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["download_http_protocol"], probe["downloadHTTPProtocol"]), cfg.DownloadHTTPProtocol)
	cfg.DownloadSpeedMetric = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["download_speed_metric"], probe["downloadSpeedMetric"]), cfg.DownloadSpeedMetric)
	cfg.HeadTestCount = configSnapshotIntValue(firstConfigSnapshotNonNil(stageLimits["stage2"], probe["stage2_limit"], probe["stage2Limit"], probe["headTestCount"]), cfg.HeadTestCount)
	cfg.Stage3Limit = configSnapshotIntValue(firstConfigSnapshotNonNil(stageLimits["stage3"], probe["stage3_limit"], probe["stage3Limit"], probe["download_count"], probe["downloadCount"]), cfg.Stage3Limit)
	cfg.DownloadSuccessLimit = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["download_success_limit"], probe["downloadSuccessLimit"]), cfg.DownloadSuccessLimit)
	cfg.Stage3Concurrency = configSnapshotIntValue(concurrency["stage3"], cfg.Stage3Concurrency)
	cfg.Stage1TimeoutMS = configSnapshotIntValue(firstConfigSnapshotNonNil(timeouts["stage1_ms"], timeouts["stage1Ms"]), cfg.Stage1TimeoutMS)
	cfg.Stage2TimeoutMS = configSnapshotIntValue(firstConfigSnapshotNonNil(timeouts["stage2_ms"], timeouts["stage2Ms"]), cfg.Stage2TimeoutMS)
	cfg.DownloadTimeSeconds = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["download_time_seconds"], probe["downloadTimeSeconds"]), cfg.DownloadTimeSeconds)
	cfg.DownloadWarmupSeconds = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["download_warmup_seconds"], probe["downloadWarmupSeconds"]), cfg.DownloadWarmupSeconds)
	cfg.TCPPort = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["tcp_port"], probe["tcpPort"]), cfg.TCPPort)
	cfg.PortPolicy = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["port_policy"], probe["portPolicy"]), cfg.PortPolicy)
	cfg.URL = configSnapshotStringValue(probe["url"], cfg.URL)
	cfg.TraceURL = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["trace_url"], probe["traceUrl"]), cfg.TraceURL)
	cfg.TraceColoMode = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["trace_colo_mode"], probe["traceColoMode"]), cfg.TraceColoMode)
	cfg.SourceColoFilterPhase = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["source_colo_filter_phase"], probe["sourceColoFilterPhase"]), cfg.SourceColoFilterPhase)
	cfg.SourcePoolLimit = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["source_pool_limit"], probe["sourcePoolLimit"]), cfg.SourcePoolLimit)
	cfg.DoHURL = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["doh_url"], probe["dohUrl"]), cfg.DoHURL)
	cfg.ECH = configSnapshotBoolValue(firstConfigSnapshotNonNil(probe["ech"], probe["echEnabled"]), cfg.ECH)
	cfg.ECHConfigList = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["ech_config"], probe["echConfig"]), cfg.ECHConfigList)
	cfg.ECHDomain = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["ech_domain"], probe["echDomain"]), cfg.ECHDomain)
	cfg.UserAgent = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["user_agent"], probe["userAgent"]), cfg.UserAgent)
	cfg.HostHeader = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["host_header"], probe["hostHeader"]), cfg.HostHeader)
	cfg.SNI = configSnapshotStringValue(probe["sni"], cfg.SNI)
	cfg.DownloadHostHeader = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["download_host_header"], probe["downloadHostHeader"]), cfg.DownloadHostHeader)
	cfg.DownloadSNI = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["download_sni"], probe["downloadSNI"], probe["downloadSni"]), cfg.DownloadSNI)
	cfg.VerifyTLSCertificate = configSnapshotBoolValue(firstConfigSnapshotNonNil(probe["verify_tls_certificate"], probe["verifyTLSCertificate"]), cfg.VerifyTLSCertificate)
	cfg.RequestHeaders = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["request_headers"], probe["requestHeaders"]), cfg.RequestHeaders)
	cfg.TraceStatusCode = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["trace_status_code"], probe["traceStatusCode"]), cfg.TraceStatusCode)
	cfg.TraceCFColo = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["trace_cf_colo"], probe["traceCfColo"]), cfg.TraceCFColo)
	cfg.TraceCFColoMode = configSnapshotStringValue(firstConfigSnapshotNonNil(probe["trace_cf_colo_mode"], probe["traceCfColoMode"]), cfg.TraceCFColoMode)
	cfg.MaxDelayMS = configSnapshotIntValue(thresholds["max_tcp_latency_ms"], cfg.MaxDelayMS)
	cfg.MinDelayMS = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["min_delay_ms"], probe["minDelayMs"]), cfg.MinDelayMS)
	cfg.MaxLossRate = configSnapshotFloatValue(firstConfigSnapshotNonNil(probe["max_loss_rate"], probe["maxLossRate"]), cfg.MaxLossRate)
	cfg.MinSpeedMB = configSnapshotFloatValue(thresholds["min_download_mbps"], cfg.MinSpeedMB)
	cfg.PrintNum = configSnapshotIntValue(firstConfigSnapshotNonNil(probe["print_num"], probe["printNum"]), cfg.PrintNum)
	cfg.DisableDownload = strategy == "fast"
	cfg.TestAll = configSnapshotBoolValue(firstConfigSnapshotNonNil(probe["test_all"], probe["testAll"]), false)
	cfg.RetryMaxAttempts = configSnapshotIntValue(firstConfigSnapshotNonNil(retryPolicy["max_attempts"], retryPolicy["maxAttempts"]), cfg.RetryMaxAttempts)
	cfg.RetryBackoffMS = configSnapshotIntValue(firstConfigSnapshotNonNil(retryPolicy["backoff_ms"], retryPolicy["backoffMs"]), cfg.RetryBackoffMS)
	cfg.CooldownFailures = configSnapshotIntValue(firstConfigSnapshotNonNil(cooldownPolicy["consecutive_failures"], cooldownPolicy["consecutiveFailures"]), cfg.CooldownFailures)
	cfg.CooldownMS = configSnapshotIntValue(firstConfigSnapshotNonNil(cooldownPolicy["cooldown_ms"], cooldownPolicy["cooldownMs"]), cfg.CooldownMS)
	cfg.Debug = configSnapshotBoolValue(probe["debug"], options.DefaultProbeDebug)
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
	} else if hasConfigSnapshotField(exportCfg, "file_name") || hasConfigSnapshotField(exportCfg, "file_name_template") {
		cfg.OutputFile = ""
		cfg.WriteOutput = false
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
