package probecore

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/task"
)

var configSnapshotFieldAliases = map[string][]string{
	"api_token":                         {"apiToken"},
	"api_base_url":                      {"apiBaseUrl", "apiHost"},
	"auto_detect_source_name":           {"autoDetectSourceName"},
	"settings_mode":                     {"settingsMode"},
	"auto_dns_push":                     {"autoDnsPush"},
	"auto_github_export":                {"autoGithubExport"},
	"bot_token":                         {"botToken"},
	"chat_id":                           {"chatId", "targetChatId", "channelChatId", "groupChatId"},
	"cloudflare_enabled":                {"cloudflareEnabled"},
	"backoff_ms":                        {"backoffMs"},
	"colo_filter":                       {"coloFilter"},
	"colo_filter_mode":                  {"coloFilterMode"},
	"commit_message_template":           {"commitMessageTemplate"},
	"consecutive_failures":              {"consecutiveFailures"},
	"cooldown_ms":                       {"cooldownMs"},
	"cooldown_policy":                   {"cooldownPolicy"},
	"csv_encoding":                      {"csvEncoding"},
	"daily_times":                       {"dailyTimes"},
	"debug_capture_address":             {"debugCaptureAddress"},
	"debug_capture_enabled":             {"debugCaptureEnabled"},
	"debug_log_format":                  {"debugLogFormat"},
	"debug_log_mode":                    {"debugLogMode"},
	"debug_log_verbosity":               {"debugLogVerbosity"},
	"download_buffer_kb":                {"downloadBufferKB"},
	"download_success_limit":            {"downloadSuccessLimit"},
	"download_get_concurrency":          {"downloadGetConcurrency"},
	"download_host_header":              {"downloadHostHeader"},
	"download_http_protocol":            {"downloadHTTPProtocol"},
	"download_sni":                      {"downloadSNI", "downloadSni"},
	"download_speed_metric":             {"downloadSpeedMetric"},
	"download_speed_sample_interval_ms": {"downloadSpeedSampleIntervalMs"},
	"download_time_seconds":             {"downloadTimeSeconds"},
	"download_warmup_seconds":           {"downloadWarmupSeconds"},
	"event_throttle_ms":                 {"eventThrottleMs"},
	"file_name":                         {"fileName"},
	"file_name_template":                {"fileNameTemplate"},
	"github_enabled":                    {"githubEnabled"},
	"host_header":                       {"hostHeader"},
	"verify_tls_certificate":            {"verifyTLSCertificate"},
	"trace_cf_colo":                     {"traceCfColo", "traceCFColo", "httping_cf_colo", "httpingCfColo", "httpingCFColo"},
	"trace_cf_colo_mode":                {"traceCfColoMode", "traceCFColoMode", "httping_cf_colo_mode", "httpingCfColoMode", "httpingCFColoMode"},
	"trace_status_code":                 {"traceStatusCode", "httping_status_code", "httpingStatusCode"},
	"include_top_n":                     {"includeTopN", "topNEnabled"},
	"interval_minutes":                  {"intervalMinutes"},
	"ip_limit":                          {"ipLimit"},
	"ip_mode":                           {"ipMode"},
	"kind":                              {"type"},
	"last_backup_at":                    {"lastBackupAt"},
	"last_export_at":                    {"lastExportAt"},
	"last_fetched_at":                   {"lastFetchedAt"},
	"last_fetched_count":                {"lastFetchedCount"},
	"last_restore_at":                   {"lastRestoreAt"},
	"completed_task_retention_days":     {"completedTaskRetentionDays"},
	"max_attempts":                      {"maxAttempts"},
	"max_loss_rate":                     {"maxLossRate"},
	"max_tcp_latency_ms":                {"maxTcpLatencyMs"},
	"min_delay_ms":                      {"minDelayMs"},
	"min_download_mbps":                 {"minDownloadMbps"},
	"name":                              {"label"},
	"ip_version":                        {"ipVersion"},
	"path_template":                     {"pathTemplate"},
	"post_probe_push":                   {"postProbePush", "auto_push", "autoPush"},
	"ping_times":                        {"pingTimes"},
	"print_num":                         {"printNum"},
	"record_name":                       {"recordName"},
	"record_type":                       {"recordType"},
	"recipient_mode":                    {"recipientMode", "targetMode"},
	"remote_path":                       {"remotePath"},
	"request_headers":                   {"requestHeaders"},
	"retry_policy":                      {"retryPolicy"},
	"routing_enabled":                   {"routingEnabled"},
	"routing_rules":                     {"routingRules"},
	"server_url":                        {"serverUrl", "url"},
	"skip_first_latency_sample":         {"skipFirstLatencySample"},
	"skip_if_active":                    {"skipIfActive"},
	"source_colo_filter_phase":          {"sourceColoFilterPhase"},
	"source_pool_limit":                 {"sourcePoolLimit"},
	"doh_url":                           {"dohUrl", "dohURL"},
	"ech":                               {"echEnabled"},
	"ech_config":                        {"echConfig"},
	"ech_domain":                        {"echDomain"},
	"personal_chat_id":                  {"personalChatId", "privateChatId", "userChatId"},
	"stage1_ms":                         {"stage1Ms"},
	"stage2_ms":                         {"stage2Ms"},
	"stage_limits":                      {"stageLimits"},
	"status_text":                       {"statusText"},
	"target_dir":                        {"targetDir"},
	"target_uri":                        {"targetUri"},
	"tcp_port":                          {"tcpPort"},
	"test_all":                          {"testAll"},
	"timeout_seconds":                   {"timeoutSeconds"},
	"telegram":                          {"tg"},
	"top_n_recipient_mode":              {"topNRecipientMode", "topRecipientMode"},
	"trace_colo_mode":                   {"traceColoMode"},
	"trace_url":                         {"traceUrl"},
	"top_n":                             {"topN"},
	"upload_recipient_mode":             {"uploadRecipientMode"},
	"shared_filter":                     {"sharedFilter"},
	"upload":                            {"uploadConfig", "upload_settings"},
	"dingtalk_url":                      {"dingtalkUrl"},
	"dingtalk_secret":                   {"dingtalkSecret"},
	"urls":                              {"url"},
	"use_system_proxy":                  {"useSystemProxy"},
	"use_tls":                           {"useTLS"},
	"user_agent":                        {"userAgent"},
	"utc_offset_minutes":                {"utcOffsetMinutes"},
	"zone_id":                           {"zoneId"},
}

func SanitizeConfigSnapshot(input map[string]any, options ConfigSnapshotOptions) map[string]any {
	options = normalizeConfigSnapshotOptions(options)
	source := configSnapshotMap(input)
	snapshot := sanitizeConfigSnapshotMap(DefaultConfigSnapshot(options), source, options)
	probeSource := configSnapshotMap(source["probe"])
	applyConfigProbeCompat(snapshot, probeSource)
	applyConfigExportCompat(snapshot, source, probeSource)
	applyConfigUploadCompat(snapshot, source)
	applyConfigTelegramCompat(snapshot, source)
	applyConfigMaintenanceCompat(snapshot)
	// 追踪有效状态码固定 200：旧配置里的 0/204 读入即归一化并写回，避免升级后仍按旧值筛选。
	configSnapshotMap(snapshot["probe"])["trace_status_code"] = task.DefaultTraceStatusCode
	if !hasConfigSnapshotField(source, "sources") {
		if sourceText := legacyConfigSourceText(source, probeSource); sourceText != "" {
			sourceItem := defaultConfigSourceConfig(0, options)
			sourceItem["kind"] = "inline"
			sourceItem["content"] = sourceText
			sourceItem["name"] = "旧版输入源"
			snapshot["sources"] = []map[string]any{sourceItem}
		}
	}
	mode := NormalizeSourceGroupIPMode(configSnapshotStringValue(snapshot["source_ip_mode"], ""))
	snapshot["source_ip_mode"] = mode
	if mode != "" {
		for _, item := range snapshot["sources"].([]map[string]any) {
			item["ip_mode"] = mode
		}
	}
	return snapshot
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
			if len(typedDefault) == 0 {
				// Empty map schemas hold dictionaries such as custom HTTP headers.
				result[key] = cloneConfigSnapshotMap(configSnapshotMap(value))
			} else {
				result[key] = sanitizeConfigSnapshotMap(typedDefault, configSnapshotMap(value), options)
			}
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
	setConfigFieldFromLegacy(stageLimits, "stage2", stageLimitsSource, probeSource, "stage2_limit", "stage2Limit", "headTestCount")
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

	// custom_host_sni 总开关已取消（字段非空即生效）。旧配置显式关闭时清空四个字段，
	// 保持"关闭即完全不生效"的既有行为，避免升级后突然开始覆盖 Host/SNI。
	if value, ok := lookupConfigSnapshotValue(probeSource, "custom_host_sni", "customHostSNI"); ok && !configSnapshotBoolValue(value, true) {
		for _, key := range []string{"host_header", "sni", "download_host_header", "download_sni"} {
			probe[key] = ""
		}
	}
	// 旧配置只有 stage3_ms 时按秒迁移到 download_time_seconds；该字段已不再保存。
	if !hasConfigSnapshotField(probeSource, "download_time_seconds") && !hasConfigSnapshotField(probeSource, "downloadTimeSeconds") {
		if ms := configSnapshotIntValue(firstConfigSnapshotNonNil(timeoutsSource["stage3_ms"], timeoutsSource["stage3Ms"]), 0); ms > 0 {
			probe["download_time_seconds"] = ms / 1000
		}
	}
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
	rootGithub := configSnapshotMap(snapshot["github"])
	rootGithubSource := configSnapshotMap(firstExistingConfigSnapshotValue(snapshotSource, "github"))
	for _, key := range []string{"branch", "commit_message_template", "csv_header_template", "csv_row_template", "enabled", "format", "last_export_at", "owner", "path_template", "repo", "token", "txt_row_template"} {
		setConfigFieldFromLegacy(rootGithub, key, rootGithubSource, githubConfig, key)
	}
	snapshot["github"] = rootGithub
	exportConfig["github"] = cloneConfigSnapshotMap(rootGithub)
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
	setConfigFieldFromLegacy(cloudflareCfg, "routing_enabled", cloudflareSource, uploadSource, "cloudflareRoutingEnabled")
	setConfigFieldFromLegacy(cloudflareCfg, "routing_rules", cloudflareSource, uploadSource, "cloudflareRoutingRules")
	setConfigFieldFromLegacy(cloudflareCfg, "top_n", cloudflareSource, uploadSource, "cloudflareTopN")
	uploadConfig["cloudflare"] = cloudflareCfg
	rootCloudflare := configSnapshotMap(snapshot["cloudflare"])
	rootCloudflareSource := configSnapshotMap(firstExistingConfigSnapshotValue(snapshotSource, "cloudflare"))
	setConfigFieldFromLegacy(rootCloudflare, "enabled", rootCloudflareSource, uploadSource, "cloudflareEnabled")
	setConfigFieldFromLegacy(rootCloudflare, "routing_enabled", rootCloudflareSource, cloudflareCfg, "routing_enabled")
	setConfigFieldFromLegacy(rootCloudflare, "routing_rules", rootCloudflareSource, cloudflareCfg, "routing_rules")
	setConfigFieldFromLegacy(rootCloudflare, "top_n", rootCloudflareSource, cloudflareCfg, "top_n")
	snapshot["cloudflare"] = rootCloudflare

	githubCfg := configSnapshotMap(uploadConfig["github"])
	githubSource := configSnapshotMap(firstExistingConfigSnapshotValue(uploadSource, "github"))
	setConfigFieldFromLegacy(githubCfg, "top_n", githubSource, uploadSource, "githubTopN")
	uploadConfig["github"] = githubCfg
	rootGithub := configSnapshotMap(snapshot["github"])
	rootGithubSource := configSnapshotMap(firstExistingConfigSnapshotValue(snapshotSource, "github"))
	setConfigFieldFromLegacy(rootGithub, "top_n", rootGithubSource, githubCfg, "top_n")
	snapshot["github"] = rootGithub
	exportConfig := configSnapshotMap(snapshot["export"])
	exportConfig["github"] = cloneConfigSnapshotMap(rootGithub)
	snapshot["export"] = exportConfig

	snapshot["upload"] = uploadConfig
}

func applyConfigMaintenanceCompat(snapshot map[string]any) {
	maintenance := configSnapshotMap(snapshot["maintenance"])
	retentionDays := configSnapshotIntValue(maintenance["completed_task_retention_days"], DefaultCompletedTaskRetentionDays)
	if retentionDays < 0 {
		retentionDays = DefaultCompletedTaskRetentionDays
	}
	maintenance["completed_task_retention_days"] = retentionDays
	snapshot["maintenance"] = maintenance
}

func applyConfigTelegramCompat(snapshot map[string]any, snapshotSource map[string]any) {
	notifications := configSnapshotMap(snapshot["notifications"])
	telegram := configSnapshotMap(notifications["telegram"])
	notificationSource := configSnapshotMap(firstExistingConfigSnapshotValue(snapshotSource, "notifications"))
	telegramSource := configSnapshotMap(firstExistingConfigSnapshotValue(notificationSource, "telegram"))
	if len(telegramSource) == 0 {
		telegramSource = configSnapshotMap(firstExistingConfigSnapshotValue(snapshotSource, "telegram"))
	}
	applyConfigTelegramLegacyRecipientMode(telegram, telegramSource)
	notifications["telegram"] = telegram
	snapshot["notifications"] = notifications
}

func applyConfigTelegramLegacyRecipientMode(telegram map[string]any, telegramSource map[string]any) {
	legacyMode, ok := lookupConfigSnapshotValue(telegramSource, "recipient_mode", "recipientMode", "target_mode", "targetMode")
	if !ok || legacyMode == nil {
		return
	}
	if !hasConfigSnapshotField(telegramSource, "upload_recipient_mode") {
		telegram["upload_recipient_mode"] = legacyMode
	}
	if !hasConfigSnapshotField(telegramSource, "top_n_recipient_mode") {
		telegram["top_n_recipient_mode"] = legacyMode
	}
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
