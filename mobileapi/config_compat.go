package mobileapi

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/axuitomo/CFST-GUI/task"
)

var mobileConfigFieldAliases = map[string][]string{
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
	"user_agent":                             {"userAgent"},
	"zone_id":                                {"zoneId"},
}

func sanitizeMobileConfigSnapshot(input map[string]any) map[string]any {
	source := mobileConfigMap(input)
	snapshot := sanitizeMobileConfigMap(defaultConfigSnapshot(), source)
	probeSource := mobileConfigMap(source["probe"])
	applyMobileProbeCompat(snapshot, probeSource)
	applyMobileExportCompat(snapshot, source, probeSource)
	if !mobileHasConfigField(source, "sources") {
		if sourceText := mobileLegacySourceText(source, probeSource); sourceText != "" {
			sourceItem := defaultMobileSourceConfig(0)
			sourceItem["kind"] = "inline"
			sourceItem["content"] = sourceText
			sourceItem["name"] = "旧版输入源"
			snapshot["sources"] = []map[string]any{sourceItem}
		}
	}
	return snapshot
}

func sanitizeMobileConfigMap(schema map[string]any, source map[string]any) map[string]any {
	result := make(map[string]any, len(schema))
	for key, defaultValue := range schema {
		value, exists := mobileConfigFieldValue(source, key)
		if !exists || value == nil {
			result[key] = cloneMobileConfigValue(defaultValue)
			continue
		}
		switch typedDefault := defaultValue.(type) {
		case map[string]any:
			result[key] = sanitizeMobileConfigMap(typedDefault, mobileConfigMap(value))
		case []map[string]any:
			if key == "sources" {
				result[key] = sanitizeMobileSources(value)
			} else {
				result[key] = cloneMobileConfigValue(defaultValue)
			}
		case []string:
			result[key] = mobileStringSlice(value)
		default:
			result[key] = value
		}
	}
	return result
}

func applyMobileProbeCompat(snapshot map[string]any, probeSource map[string]any) {
	probe := mobileConfigMap(snapshot["probe"])
	concurrency := mobileConfigMap(probe["concurrency"])
	concurrencySource := mobileConfigMap(firstExistingMobileConfigValue(probeSource, "concurrency"))
	setMobileFieldFromLegacy(concurrency, "stage1", concurrencySource, probeSource, "routines")
	setMobileFieldFromLegacy(concurrency, "stage2", concurrencySource, probeSource, "headRoutines")
	setMobileFieldFromLegacy(concurrency, "stage3", concurrencySource, probeSource, "stage3Concurrency")
	probe["concurrency"] = concurrency

	stageLimits := mobileConfigMap(probe["stage_limits"])
	stageLimitsSource := mobileConfigMap(firstExistingMobileConfigValue(probeSource, "stage_limits"))
	setMobileFieldFromLegacy(stageLimits, "stage3", stageLimitsSource, probeSource, "stage3_limit", "stage3Limit", "download_count", "downloadCount", "testCount")
	probe["stage_limits"] = stageLimits

	timeouts := mobileConfigMap(probe["timeouts"])
	timeoutsSource := mobileConfigMap(firstExistingMobileConfigValue(probeSource, "timeouts"))
	setMobileFieldFromLegacy(timeouts, "stage1_ms", timeoutsSource, probeSource, "stage1TimeoutMs", "stage1TimeoutMS")
	setMobileFieldFromLegacy(timeouts, "stage2_ms", timeoutsSource, probeSource, "stage2TimeoutMs", "stage2TimeoutMS")
	probe["timeouts"] = timeouts

	thresholds := mobileConfigMap(probe["thresholds"])
	thresholdsSource := mobileConfigMap(firstExistingMobileConfigValue(probeSource, "thresholds"))
	setMobileFieldFromLegacy(thresholds, "max_tcp_latency_ms", thresholdsSource, probeSource, "maxDelayMS", "maxDelayMs")
	setMobileFieldFromLegacy(thresholds, "max_http_latency_ms", thresholdsSource, probeSource, "headMaxDelayMS", "headMaxDelayMs")
	setMobileFieldFromLegacy(thresholds, "min_download_mbps", thresholdsSource, probeSource, "minSpeedMB", "minSpeedMb")
	probe["thresholds"] = thresholds

	retryPolicy := mobileConfigMap(probe["retry_policy"])
	retrySource := mobileConfigMap(firstExistingMobileConfigValue(probeSource, "retry_policy"))
	setMobileFieldFromLegacy(retryPolicy, "max_attempts", retrySource, probeSource, "retryMaxAttempts")
	setMobileFieldFromLegacy(retryPolicy, "backoff_ms", retrySource, probeSource, "retryBackoffMs", "retryBackoffMS")
	probe["retry_policy"] = retryPolicy

	cooldownPolicy := mobileConfigMap(probe["cooldown_policy"])
	cooldownSource := mobileConfigMap(firstExistingMobileConfigValue(probeSource, "cooldown_policy"))
	setMobileFieldFromLegacy(cooldownPolicy, "consecutive_failures", cooldownSource, probeSource, "cooldownFailures")
	setMobileFieldFromLegacy(cooldownPolicy, "cooldown_ms", cooldownSource, probeSource, "cooldownMs", "cooldownMS")
	probe["cooldown_policy"] = cooldownPolicy

	snapshot["probe"] = probe
}

func applyMobileExportCompat(snapshot map[string]any, snapshotSource map[string]any, probeSource map[string]any) {
	exportConfig := mobileConfigMap(snapshot["export"])
	exportSource := mobileConfigMap(firstExistingMobileConfigValue(snapshotSource, "export"))
	setMobileFieldFromLegacy(exportConfig, "csv_encoding", exportSource, probeSource, "csvEncoding")
	if !mobileHasConfigField(exportSource, "file_name") {
		if outputFile, ok := mobileLookupConfigValue(probeSource, "outputFile"); ok {
			if fileName := strings.TrimSpace(filepath.Base(stringValue(outputFile, ""))); fileName != "" && fileName != "." {
				exportConfig["file_name"] = fileName
			}
		}
	}
	if !mobileHasConfigField(exportSource, "overwrite") {
		if appendValue, ok := mobileLookupConfigValue(probeSource, "exportAppend"); ok && boolValue(appendValue, false) {
			exportConfig["overwrite"] = "append"
		}
	}
	snapshot["export"] = exportConfig
}

func sanitizeMobileSources(value any) []map[string]any {
	items, ok := mobileConfigSlice(value)
	if !ok {
		return []map[string]any{}
	}
	sources := make([]map[string]any, 0, len(items))
	for index, item := range items {
		source := mobileConfigMap(item)
		sources = append(sources, sanitizeMobileConfigMap(defaultMobileSourceConfig(index), source))
	}
	return sources
}

func defaultMobileSourceConfig(index int) map[string]any {
	return map[string]any{
		"content":            "",
		"colo_filter":        "",
		"colo_filter_mode":   task.ColoFilterModeAllow,
		"enabled":            true,
		"id":                 fmt.Sprintf("source-%d", index+1),
		"ip_limit":           defaultMobileSourceIPLimit,
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

func mobileLegacySourceText(snapshot map[string]any, probe map[string]any) string {
	for _, candidate := range []any{
		snapshot["sourceText"],
		snapshot["source_text"],
		probe["ipText"],
		probe["ip_text"],
	} {
		if text := strings.TrimSpace(stringValue(candidate, "")); text != "" {
			return text
		}
	}
	return ""
}

func setMobileFieldFromLegacy(target map[string]any, key string, currentSource map[string]any, legacySource map[string]any, legacyKeys ...string) {
	if mobileHasConfigField(currentSource, key) {
		return
	}
	if value, ok := mobileLookupConfigValue(legacySource, legacyKeys...); ok && value != nil {
		target[key] = value
	}
}

func firstExistingMobileConfigValue(source map[string]any, key string) any {
	value, _ := mobileConfigFieldValue(source, key)
	return value
}

func mobileHasConfigField(source map[string]any, key string) bool {
	_, ok := mobileConfigFieldValue(source, key)
	return ok
}

func mobileConfigFieldValue(source map[string]any, key string) (any, bool) {
	keys := append([]string{key}, mobileConfigFieldAliases[key]...)
	return mobileLookupConfigValue(source, keys...)
}

func mobileLookupConfigValue(source map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, exists := source[key]; exists {
			return value, true
		}
	}
	return nil, false
}

func mobileConfigMap(value any) map[string]any {
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

func mobileConfigSlice(value any) ([]any, bool) {
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

func mobileStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case string:
		return splitMobileConfigStrings(typed)
	}
	items, ok := mobileConfigSlice(value)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(stringValue(item, "")); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func splitMobileConfigStrings(value string) []string {
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

func cloneMobileConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMobileConfigMap(typed)
	case []map[string]any:
		cloned := make([]map[string]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneMobileConfigMap(item)
		}
		return cloned
	case []string:
		return append([]string{}, typed...)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneMobileConfigValue(item)
		}
		return cloned
	default:
		return typed
	}
}

func cloneMobileConfigMap(value map[string]any) map[string]any {
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = cloneMobileConfigValue(item)
	}
	return cloned
}
