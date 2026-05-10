package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/axuitomo/CFST-GUI/task"
)

var desktopConfigFieldAliases = map[string][]string{
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

func sanitizeDesktopConfigSnapshot(input map[string]any) map[string]any {
	source := desktopConfigMap(input)
	snapshot := sanitizeDesktopConfigMap(defaultDesktopConfigSnapshot(), source)
	probeSource := desktopConfigMap(source["probe"])
	applyDesktopProbeCompat(snapshot, probeSource)
	applyDesktopExportCompat(snapshot, source, probeSource)
	if !desktopHasConfigField(source, "sources") {
		if sourceText := desktopLegacySourceText(source, probeSource); sourceText != "" {
			sourceItem := defaultDesktopSourceConfig(0)
			sourceItem["kind"] = "inline"
			sourceItem["content"] = sourceText
			sourceItem["name"] = "旧版输入源"
			snapshot["sources"] = []map[string]any{sourceItem}
		}
	}
	return snapshot
}

func sanitizeDesktopConfigMap(schema map[string]any, source map[string]any) map[string]any {
	result := make(map[string]any, len(schema))
	for key, defaultValue := range schema {
		value, exists := desktopConfigFieldValue(source, key)
		if !exists || value == nil {
			result[key] = cloneDesktopConfigValue(defaultValue)
			continue
		}
		switch typedDefault := defaultValue.(type) {
		case map[string]any:
			result[key] = sanitizeDesktopConfigMap(typedDefault, desktopConfigMap(value))
		case []map[string]any:
			if key == "sources" {
				result[key] = sanitizeDesktopSources(value)
			} else {
				result[key] = cloneDesktopConfigValue(defaultValue)
			}
		case []string:
			result[key] = desktopStringSlice(value)
		default:
			result[key] = value
		}
	}
	return result
}

func applyDesktopProbeCompat(snapshot map[string]any, probeSource map[string]any) {
	probe := desktopConfigMap(snapshot["probe"])
	concurrency := desktopConfigMap(probe["concurrency"])
	concurrencySource := desktopConfigMap(firstExistingDesktopConfigValue(probeSource, "concurrency"))
	setDesktopFieldFromLegacy(concurrency, "stage1", concurrencySource, probeSource, "routines")
	setDesktopFieldFromLegacy(concurrency, "stage2", concurrencySource, probeSource, "headRoutines")
	setDesktopFieldFromLegacy(concurrency, "stage3", concurrencySource, probeSource, "stage3Concurrency")
	probe["concurrency"] = concurrency

	stageLimits := desktopConfigMap(probe["stage_limits"])
	stageLimitsSource := desktopConfigMap(firstExistingDesktopConfigValue(probeSource, "stage_limits"))
	setDesktopFieldFromLegacy(stageLimits, "stage3", stageLimitsSource, probeSource, "stage3_limit", "stage3Limit", "download_count", "downloadCount", "testCount")
	probe["stage_limits"] = stageLimits

	timeouts := desktopConfigMap(probe["timeouts"])
	timeoutsSource := desktopConfigMap(firstExistingDesktopConfigValue(probeSource, "timeouts"))
	setDesktopFieldFromLegacy(timeouts, "stage1_ms", timeoutsSource, probeSource, "stage1TimeoutMs", "stage1TimeoutMS")
	setDesktopFieldFromLegacy(timeouts, "stage2_ms", timeoutsSource, probeSource, "stage2TimeoutMs", "stage2TimeoutMS")
	probe["timeouts"] = timeouts

	thresholds := desktopConfigMap(probe["thresholds"])
	thresholdsSource := desktopConfigMap(firstExistingDesktopConfigValue(probeSource, "thresholds"))
	setDesktopFieldFromLegacy(thresholds, "max_tcp_latency_ms", thresholdsSource, probeSource, "maxDelayMS", "maxDelayMs")
	setDesktopFieldFromLegacy(thresholds, "max_http_latency_ms", thresholdsSource, probeSource, "headMaxDelayMS", "headMaxDelayMs")
	setDesktopFieldFromLegacy(thresholds, "min_download_mbps", thresholdsSource, probeSource, "minSpeedMB", "minSpeedMb")
	probe["thresholds"] = thresholds

	retryPolicy := desktopConfigMap(probe["retry_policy"])
	retrySource := desktopConfigMap(firstExistingDesktopConfigValue(probeSource, "retry_policy"))
	setDesktopFieldFromLegacy(retryPolicy, "max_attempts", retrySource, probeSource, "retryMaxAttempts")
	setDesktopFieldFromLegacy(retryPolicy, "backoff_ms", retrySource, probeSource, "retryBackoffMs", "retryBackoffMS")
	probe["retry_policy"] = retryPolicy

	cooldownPolicy := desktopConfigMap(probe["cooldown_policy"])
	cooldownSource := desktopConfigMap(firstExistingDesktopConfigValue(probeSource, "cooldown_policy"))
	setDesktopFieldFromLegacy(cooldownPolicy, "consecutive_failures", cooldownSource, probeSource, "cooldownFailures")
	setDesktopFieldFromLegacy(cooldownPolicy, "cooldown_ms", cooldownSource, probeSource, "cooldownMs", "cooldownMS")
	probe["cooldown_policy"] = cooldownPolicy

	snapshot["probe"] = probe
}

func applyDesktopExportCompat(snapshot map[string]any, snapshotSource map[string]any, probeSource map[string]any) {
	exportConfig := desktopConfigMap(snapshot["export"])
	exportSource := desktopConfigMap(firstExistingDesktopConfigValue(snapshotSource, "export"))
	setDesktopFieldFromLegacy(exportConfig, "csv_encoding", exportSource, probeSource, "csvEncoding")
	if !desktopHasConfigField(exportSource, "file_name") {
		if outputFile, ok := desktopLookupConfigValue(probeSource, "outputFile"); ok {
			if fileName := strings.TrimSpace(filepath.Base(stringValue(outputFile, ""))); fileName != "" && fileName != "." {
				exportConfig["file_name"] = fileName
			}
		}
	}
	if !desktopHasConfigField(exportSource, "overwrite") {
		if appendValue, ok := desktopLookupConfigValue(probeSource, "exportAppend"); ok && boolValue(appendValue, false) {
			exportConfig["overwrite"] = "append"
		}
	}
	snapshot["export"] = exportConfig
}

func sanitizeDesktopSources(value any) []map[string]any {
	items, ok := desktopConfigSlice(value)
	if !ok {
		return []map[string]any{}
	}
	sources := make([]map[string]any, 0, len(items))
	for index, item := range items {
		source := desktopConfigMap(item)
		sources = append(sources, sanitizeDesktopConfigMap(defaultDesktopSourceConfig(index), source))
	}
	return sources
}

func defaultDesktopSourceConfig(index int) map[string]any {
	return map[string]any{
		"content":            "",
		"colo_filter":        "",
		"colo_filter_mode":   task.ColoFilterModeAllow,
		"enabled":            true,
		"id":                 fmt.Sprintf("source-%d", index+1),
		"ip_limit":           defaultDesktopSourceIPLimit,
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

func desktopLegacySourceText(snapshot map[string]any, probe map[string]any) string {
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

func setDesktopFieldFromLegacy(target map[string]any, key string, currentSource map[string]any, legacySource map[string]any, legacyKeys ...string) {
	if desktopHasConfigField(currentSource, key) {
		return
	}
	if value, ok := desktopLookupConfigValue(legacySource, legacyKeys...); ok && value != nil {
		target[key] = value
	}
}

func firstExistingDesktopConfigValue(source map[string]any, key string) any {
	value, _ := desktopConfigFieldValue(source, key)
	return value
}

func desktopHasConfigField(source map[string]any, key string) bool {
	_, ok := desktopConfigFieldValue(source, key)
	return ok
}

func desktopConfigFieldValue(source map[string]any, key string) (any, bool) {
	keys := append([]string{key}, desktopConfigFieldAliases[key]...)
	return desktopLookupConfigValue(source, keys...)
}

func desktopLookupConfigValue(source map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, exists := source[key]; exists {
			return value, true
		}
	}
	return nil, false
}

func desktopConfigMap(value any) map[string]any {
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

func desktopConfigSlice(value any) ([]any, bool) {
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

func desktopStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case string:
		return splitDesktopConfigStrings(typed)
	}
	items, ok := desktopConfigSlice(value)
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

func splitDesktopConfigStrings(value string) []string {
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

func cloneDesktopConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneDesktopConfigMap(typed)
	case []map[string]any:
		cloned := make([]map[string]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneDesktopConfigMap(item)
		}
		return cloned
	case []string:
		return append([]string{}, typed...)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneDesktopConfigValue(item)
		}
		return cloned
	default:
		return typed
	}
}

func cloneDesktopConfigMap(value map[string]any) map[string]any {
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = cloneDesktopConfigValue(item)
	}
	return cloned
}
