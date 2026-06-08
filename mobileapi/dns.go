package mobileapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/cloudflarecore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const (
	cloudflareRecordTypeA    = cloudflarecore.RecordTypeA
	cloudflareRecordTypeAAAA = cloudflarecore.RecordTypeAAAA
	cloudflareRecordTypeAll  = "ALL"
	defaultCloudflareTTL     = cloudflarecore.DefaultTTL
)

var cloudflareAPIBaseURL = cloudflarecore.APIBaseURL

type cloudflareDNSConfig = cloudflarecore.Config
type CloudflareDNSRecord = cloudflarecore.Record
type cloudflareDNSPushSummary = cloudflarecore.PushSummary
type cloudflareDNSPushIPGroups = cloudflarecore.PushIPGroups

func (s *Service) ListCloudflareDNSRecords(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	cfg, warnings, err := appcore.CloudflareDNSListConfigFromPayload(payload)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings))
	}
	options, err := cloudflareDNSListOptionsFromPayload(payload, cfg)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	records, err := newCloudflareDNSClient(cfg.APIToken).ListRecordsWithOptions(ctx, cfg, options)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_LIST_FAILED", nil, err.Error(), false, nil, warnings))
	}
	target := "当前 Zone"
	if strings.TrimSpace(options.Name) != "" {
		target = strings.TrimSpace(options.Name)
	}
	return encodeCommand(commandResultFor("DNS_RECORDS_LISTED", map[string]any{
		"count":   len(records),
		"records": records,
	}, fmt.Sprintf("已读取 Cloudflare 中匹配 %s 的 DNS 记录 %d 条。", target, len(records)), true, nil, warnings))
}

func (s *Service) PushCloudflareDNSRecords(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	cfgPayload := mobileCloudflareDNSConfigPayloadForPush(payload)
	cfg, warnings, err := cloudflareDNSConfigFromPayload(cfgPayload)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings))
	}
	cfg.RecordType = cloudflareDNSRecordTypeFromPayload(cfgPayload, cfg.RecordType)

	if rawRows := firstNonNil(payload["results"], payload["rows"]); rawRows != nil {
		rows := mobileProbeRowsFromAny(rawRows)
		if len(rows) == 0 {
			return encodeCommand(commandResultFor("DNS_INPUT_EMPTY", nil, "没有可推送的测速结果。", false, nil, warnings))
		}
		config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
		probeCfg, _ := configToProbeConfig(config)
		selection, selectErr := appcore.BuildUploadSelectionWithColoPaths(config, rows, probeCfg.DownloadSpeedMetric, s.coloDictionaryPaths())
		if selectErr != nil {
			return encodeCommand(commandResultFor("DNS_CONFIG_INVALID", nil, selectErr.Error(), false, nil, warnings))
		}
		warnings = append(warnings, selection.Warnings...)
		if routeSelections, routeWarnings := appcore.BuildCloudflareRouteSelections(config, selection.FilteredRows, probeCfg.DownloadSpeedMetric, s.coloDictionaryPaths()); len(routeSelections) > 0 {
			warnings = append(warnings, routeWarnings...)
			return s.pushCloudflareDNSCombinedSelections(cfg, selection, routeSelections, warnings, mobileCloudflarePayloadHasRecordName(payload))
		}
		rows = appcore.FilterRowsForCloudflareRecordType(selection.CloudflareRows, cfg.RecordType)
		if len(rows) == 0 {
			return encodeCommand(commandResultFor("DNS_INPUT_EMPTY", map[string]any{
				"ignored_entries": []string{},
				"records_after":   []CloudflareDNSRecord{},
				"summary":         cloudflareSummaryMap(cloudflareDNSPushSummary{}),
				"upload_count":    0,
			}, "本次筛选后无匹配 IP，已跳过 DNS 推送。", false, nil, warnings))
		}
		payload["ipsRaw"] = mobileProbeRowsIPList(rows)
	}

	ipsRaw := stringValue(firstNonNil(payload["ipsRaw"], payload["ips_raw"]), "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := appcore.PushCloudflareDNSRecords(ctx, newCloudflareDNSClient(cfg.APIToken), cfg, ipsRaw)
	if err != nil {
		return encodeCommand(commandResultFor(cloudflareDNSErrorCode(err), nil, err.Error(), false, nil, warnings))
	}
	warnings = append(warnings, result.Warnings...)
	if !result.HasInputIPs {
		return encodeCommand(commandResultFor("DNS_INPUT_EMPTY", map[string]any{
			"ignored_entries": result.IgnoredEntries,
			"records_after":   []CloudflareDNSRecord{},
			"summary":         cloudflareSummaryMap(result.Summary),
		}, "没有可推送的有效 IP。", false, nil, warnings))
	}

	return encodeCommand(commandResultFor("DNS_PUSH_COMPLETED", map[string]any{
		"ignored_entries": result.IgnoredEntries,
		"records_after":   result.RecordsAfter,
		"summary":         cloudflareSummaryMap(result.Summary),
	}, fmt.Sprintf("Cloudflare DNS 覆盖推送完成：创建 %d、更新 %d、删除 %d、忽略 %d。", result.Summary.Created, result.Summary.Updated, result.Summary.Deleted, result.Summary.Ignored), true, nil, dedupeStrings(warnings)))
}

func (s *Service) pushCloudflareDNSCombinedSelections(baseCfg cloudflareDNSConfig, selection appcore.UploadSelectionResult, routes []appcore.UploadCloudflareRouteSelection, warnings []string, includePrimary bool) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client := newCloudflareDNSClient(baseCfg.APIToken)
	targets := make([]map[string]any, 0, len(routes)+1)
	totalUploadCount := 0
	successCount := 0
	failureCount := 0
	skippedCount := 0

	if includePrimary {
		rows := appcore.FilterRowsForCloudflareRecordType(selection.CloudflareRows, baseCfg.RecordType)
		target, ok, skipped, uploadCount, targetWarnings := mobilePushCloudflareDNSTarget(ctx, client, baseCfg, rows, mobileCloudflareTargetLabel("主目标", baseCfg.RecordName), "primary")
		targets = append(targets, target)
		warnings = append(warnings, targetWarnings...)
		if ok {
			successCount++
			totalUploadCount += uploadCount
		} else if skipped {
			skippedCount++
		} else {
			failureCount++
		}
	}

	for _, route := range routes {
		rule := route.Rule
		cfg := baseCfg
		cfg.RecordName = rule.RecordName
		cfg.RecordType = rule.RecordType
		if route.Skipped {
			target := mobileSkippedCloudflareDNSTarget(cfg, route.Warnings, mobileCloudflareTargetLabel("分流目标", mobileCloudflareRouteLabel(rule)), "route")
			target["filtered_count"] = route.FilteredCount
			target["input_count"] = route.InputCount
			target["rule_name"] = rule.Name
			target["selected_count"] = len(route.Rows)
			targets = append(targets, target)
			warnings = append(warnings, route.Warnings...)
			skippedCount++
			continue
		}
		rows := appcore.FilterRowsForCloudflareRecordType(route.Rows, cfg.RecordType)
		label := mobileCloudflareTargetLabel("分流目标", mobileCloudflareRouteLabel(rule))
		target, ok, skipped, uploadCount, targetWarnings := mobilePushCloudflareDNSTarget(ctx, client, cfg, rows, label, "route")
		target["filtered_count"] = route.FilteredCount
		target["input_count"] = route.InputCount
		target["rule_name"] = rule.Name
		target["selected_count"] = len(route.Rows)
		if len(route.Warnings) > 0 {
			targetWarnings = append(route.Warnings, targetWarnings...)
			target["warnings"] = dedupeStrings(targetWarnings)
		}
		targets = append(targets, target)
		warnings = append(warnings, targetWarnings...)
		if ok {
			successCount++
			totalUploadCount += uploadCount
		} else if skipped {
			skippedCount++
		} else {
			failureCount++
		}
	}

	data := map[string]any{
		"filtered_count":   len(selection.FilteredRows),
		"input_count":      len(selection.InputRows),
		"routing_enabled":  len(routes) > 0,
		"skipped_targets":  skippedCount,
		"success_targets":  successCount,
		"failed_targets":   failureCount,
		"targets":          targets,
		"upload_count":     totalUploadCount,
		"cloudflare_count": totalUploadCount,
	}
	if successCount == 0 {
		message := "Cloudflare 推送未执行：所有目标均无可上传 IP。"
		if failureCount > 0 {
			message = "Cloudflare 推送失败：所有目标均未完成。"
		}
		return encodeCommand(commandResultFor("DNS_INPUT_EMPTY", data, message, false, nil, dedupeStrings(warnings)))
	}
	if failureCount > 0 {
		return encodeCommand(commandResultFor("DNS_PUSH_PARTIAL", data, fmt.Sprintf("Cloudflare 推送部分完成：成功 %d 个目标，失败 %d 个目标，跳过 %d 个目标。", successCount, failureCount, skippedCount), true, nil, dedupeStrings(warnings)))
	}
	return encodeCommand(commandResultFor("DNS_PUSH_COMPLETED", data, fmt.Sprintf("Cloudflare 推送完成：成功 %d 个目标，跳过 %d 个目标。", successCount, skippedCount), true, nil, dedupeStrings(warnings)))
}

func mobileSkippedCloudflareDNSTarget(cfg cloudflareDNSConfig, routeWarnings []string, label string, targetKind string) map[string]any {
	message := "Cloudflare " + label + "已跳过。"
	if len(routeWarnings) > 0 {
		message = routeWarnings[len(routeWarnings)-1]
	}
	target := map[string]any{
		"kind":         targetKind,
		"message":      message,
		"ok":           false,
		"record_name":  cfg.RecordName,
		"record_type":  cfg.RecordType,
		"skipped":      true,
		"summary":      cloudflareSummaryMap(cloudflareDNSPushSummary{}),
		"upload_count": 0,
		"warnings":     dedupeStrings(routeWarnings),
	}
	mobileLogCloudflarePushTarget("cloudflare.push.target_skipped", targetKind, cfg, 0, message, nil, nil)
	return target
}

func mobilePushCloudflareDNSTarget(ctx context.Context, client *cloudflarecore.Client, cfg cloudflareDNSConfig, rows []probeRow, label string, targetKind string) (map[string]any, bool, bool, int, []string) {
	target := map[string]any{
		"kind":         targetKind,
		"record_name":  cfg.RecordName,
		"record_type":  cfg.RecordType,
		"summary":      cloudflareSummaryMap(cloudflareDNSPushSummary{}),
		"upload_count": len(rows),
	}
	if len(rows) == 0 {
		message := fmt.Sprintf("Cloudflare %s：记录类型 %s 无匹配 IP，已跳过。", label, cfg.RecordType)
		target["message"] = message
		target["ok"] = false
		target["skipped"] = true
		target["warnings"] = []string{message}
		mobileLogCloudflarePushTarget("cloudflare.push.target_skipped", targetKind, cfg, len(rows), message, nil, nil)
		return target, false, true, 0, []string{message}
	}

	result, err := appcore.PushCloudflareDNSRecords(ctx, client, cfg, mobileProbeRowsIPList(rows))
	if err != nil {
		message := fmt.Sprintf("Cloudflare %s推送失败：%s", label, err.Error())
		target["error"] = err.Error()
		target["message"] = message
		target["ok"] = false
		target["skipped"] = false
		target["warnings"] = []string{message}
		mobileLogCloudflarePushTarget("cloudflare.push.target_failed", targetKind, cfg, len(rows), message, nil, err)
		return target, false, false, 0, []string{message}
	}
	message := fmt.Sprintf("Cloudflare %s推送成功：创建 %d、更新 %d、删除 %d、忽略 %d。", label, result.Summary.Created, result.Summary.Updated, result.Summary.Deleted, result.Summary.Ignored)
	warnings := append([]string{message}, result.Warnings...)
	target["ignored_entries"] = result.IgnoredEntries
	target["message"] = message
	target["ok"] = true
	target["records_after"] = result.RecordsAfter
	target["skipped"] = false
	target["summary"] = cloudflareSummaryMap(result.Summary)
	target["warnings"] = dedupeStrings(warnings)
	mobileLogCloudflarePushTarget("cloudflare.push.target_completed", targetKind, cfg, len(rows), message, result.Summary, nil)
	return target, true, false, len(rows), warnings
}

func mobileCloudflareDNSConfigPayloadForPush(payload map[string]any) map[string]any {
	if firstNonNil(payload["results"], payload["rows"]) == nil {
		return payload
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(config) == 0 {
		return payload
	}
	cloudflare := mapValue(config["cloudflare"])
	if strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_name"], cloudflare["recordName"]), "")) != "" {
		return payload
	}
	routing := appcore.CloudflareRoutingConfigFromSnapshot(config)
	if !routing.Enabled {
		return payload
	}
	for _, rule := range routing.Rules {
		if !rule.Enabled || strings.TrimSpace(rule.RecordName) == "" {
			continue
		}
		nextPayload := mobileDeepCloneMap(payload)
		nextConfig := mapValue(nextPayload["config"])
		if len(nextConfig) == 0 {
			nextConfig = mapValue(nextPayload["config_snapshot"])
			nextPayload["config"] = nextConfig
		}
		nextCloudflare := mapValue(nextConfig["cloudflare"])
		nextCloudflare["record_name"] = rule.RecordName
		nextCloudflare["record_type"] = rule.RecordType
		nextConfig["cloudflare"] = nextCloudflare
		return nextPayload
	}
	return payload
}

func cloudflareDNSListOptionsFromPayload(payload map[string]any, cfg cloudflareDNSConfig) (cloudflarecore.ListOptions, error) {
	scope := strings.ToLower(strings.TrimSpace(stringValue(firstNonNil(payload["scope"], payload["filter"], payload["mode"]), "")))
	name := strings.TrimSpace(stringValue(firstNonNil(payload["name"], payload["record_name"], payload["recordName"]), ""))
	recordType := strings.TrimSpace(stringValue(firstNonNil(payload["record_type"], payload["recordType"], payload["type"]), ""))
	switch scope {
	case "zone", "all", "domain":
		name = ""
	case "custom", "subdomain", "name":
		if name == "" {
			return cloudflarecore.ListOptions{}, fmt.Errorf("缺少要读取的 Cloudflare DNS 记录名称")
		}
	case "configured", "config", "":
		if name == "" {
			name = strings.TrimSpace(cfg.RecordName)
		}
		if name == "" {
			return cloudflarecore.ListOptions{}, fmt.Errorf("缺少 Cloudflare DNS 记录名称")
		}
	default:
		if name == "" {
			name = strings.TrimSpace(cfg.RecordName)
		}
		if name == "" {
			return cloudflarecore.ListOptions{}, fmt.Errorf("缺少 Cloudflare DNS 记录名称")
		}
	}
	return cloudflarecore.ListOptions{Name: name, Type: recordType}, nil
}

func (s *Service) pushCloudflareDNSRouteSelections(baseCfg cloudflareDNSConfig, selection appcore.UploadSelectionResult, routes []appcore.UploadCloudflareRouteSelection, warnings []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client := newCloudflareDNSClient(baseCfg.APIToken)
	targets := make([]map[string]any, 0, len(routes))
	totalUploadCount := 0
	successCount := 0
	failureCount := 0
	skippedCount := 0

	for _, route := range routes {
		rule := route.Rule
		targetWarnings := append([]string{}, route.Warnings...)
		target := map[string]any{
			"filtered_count": route.FilteredCount,
			"input_count":    route.InputCount,
			"record_name":    rule.RecordName,
			"record_type":    rule.RecordType,
			"rule_name":      rule.Name,
			"selected_count": len(route.Rows),
			"summary":        cloudflareSummaryMap(cloudflareDNSPushSummary{}),
			"upload_count":   0,
		}
		if route.Skipped {
			skippedCount++
			target["ok"] = false
			target["skipped"] = true
			target["warnings"] = dedupeStrings(targetWarnings)
			targets = append(targets, target)
			continue
		}

		cfg := baseCfg
		cfg.RecordName = rule.RecordName
		cfg.RecordType = rule.RecordType
		rows := appcore.FilterRowsForCloudflareRecordType(route.Rows, cfg.RecordType)
		target["upload_count"] = len(rows)
		if len(rows) == 0 {
			skippedCount++
			targetWarnings = append(targetWarnings, fmt.Sprintf("Cloudflare 分流规则「%s」：记录类型 %s 无匹配 IP，已跳过。", mobileCloudflareRouteLabel(rule), cfg.RecordType))
			target["ok"] = false
			target["skipped"] = true
			target["warnings"] = dedupeStrings(targetWarnings)
			targets = append(targets, target)
			warnings = append(warnings, targetWarnings...)
			continue
		}

		result, err := appcore.PushCloudflareDNSRecords(ctx, client, cfg, mobileProbeRowsIPList(rows))
		if err != nil {
			failureCount++
			targetWarnings = append(targetWarnings, err.Error())
			target["error"] = err.Error()
			target["ok"] = false
			target["skipped"] = false
			target["warnings"] = dedupeStrings(targetWarnings)
			targets = append(targets, target)
			warnings = append(warnings, targetWarnings...)
			continue
		}
		targetWarnings = append(targetWarnings, result.Warnings...)
		successCount++
		totalUploadCount += len(rows)
		target["ignored_entries"] = result.IgnoredEntries
		target["ok"] = true
		target["records_after"] = result.RecordsAfter
		target["skipped"] = false
		target["summary"] = cloudflareSummaryMap(result.Summary)
		target["warnings"] = dedupeStrings(targetWarnings)
		targets = append(targets, target)
		warnings = append(warnings, targetWarnings...)
	}

	data := map[string]any{
		"filtered_count":   len(selection.FilteredRows),
		"input_count":      len(selection.InputRows),
		"routing_enabled":  true,
		"skipped_targets":  skippedCount,
		"success_targets":  successCount,
		"failed_targets":   failureCount,
		"targets":          targets,
		"upload_count":     totalUploadCount,
		"cloudflare_count": totalUploadCount,
	}
	if successCount == 0 {
		message := "Cloudflare 分流推送未执行：所有规则均无可上传 IP。"
		if failureCount > 0 {
			message = "Cloudflare 分流推送失败：所有目标均未完成。"
		}
		return encodeCommand(commandResultFor("DNS_INPUT_EMPTY", data, message, false, nil, dedupeStrings(warnings)))
	}
	if failureCount > 0 {
		return encodeCommand(commandResultFor("DNS_PUSH_PARTIAL", data, fmt.Sprintf("Cloudflare 分流推送部分完成：成功 %d 个目标，失败 %d 个目标，跳过 %d 个目标。", successCount, failureCount, skippedCount), true, nil, dedupeStrings(warnings)))
	}
	return encodeCommand(commandResultFor("DNS_PUSH_COMPLETED", data, fmt.Sprintf("Cloudflare 分流推送完成：成功 %d 个目标，跳过 %d 个目标。", successCount, skippedCount), true, nil, dedupeStrings(warnings)))
}

func mobileCloudflareRouteLabel(rule appcore.UploadCloudflareRoutingRule) string {
	if strings.TrimSpace(rule.Name) != "" {
		return strings.TrimSpace(rule.Name)
	}
	if strings.TrimSpace(rule.RecordName) != "" {
		return strings.TrimSpace(rule.RecordName)
	}
	return "未命名规则"
}

func mobileCloudflarePayloadHasRecordName(payload map[string]any) bool {
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(config) == 0 {
		config = payload
	}
	cloudflare := mapValue(config["cloudflare"])
	if len(cloudflare) == 0 {
		cloudflare = config
	}
	return strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_name"], cloudflare["recordName"]), "")) != ""
}

func mobileCloudflareTargetLabel(prefix string, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "「" + prefix + "」"
	}
	return "「" + prefix + " " + name + "」"
}

func mobileLogCloudflarePushTarget(event string, targetKind string, cfg cloudflareDNSConfig, uploadCount int, message string, summary any, err error) {
	fields := map[string]any{
		"message":      message,
		"record_name":  cfg.RecordName,
		"record_type":  cfg.RecordType,
		"target_kind":  targetKind,
		"upload_count": uploadCount,
	}
	if summary != nil {
		fields["summary"] = summary
	}
	if err != nil {
		fields["error"] = err.Error()
		fields["level"] = "error"
	}
	utils.DebugEvent(event, fields)
}

func cloudflareSummaryMap(summary cloudflareDNSPushSummary) map[string]any {
	return appcore.CloudflareSummaryMap(summary)
}

func cloudflareDNSConfigFromPayload(payload map[string]any) (cloudflareDNSConfig, []string, error) {
	return appcore.CloudflareDNSConfigFromPayload(payload)
}

func cloudflareDNSRecordTypeFromPayload(payload map[string]any, fallback string) string {
	config := mapValue(payload["config"])
	if len(config) == 0 {
		config = mapValue(payload["config_snapshot"])
	}
	if len(config) == 0 {
		config = payload
	}
	cloudflare := mapValue(config["cloudflare"])
	if len(cloudflare) == 0 {
		cloudflare = config
	}
	recordType := stringValue(firstNonNil(cloudflare["record_type"], cloudflare["recordType"]), fallback)
	return normalizeCloudflareRecordType(recordType)
}

func normalizeCloudflareRecordType(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case cloudflareRecordTypeAll:
		return cloudflareRecordTypeAll
	case cloudflareRecordTypeAAAA:
		return cloudflareRecordTypeAAAA
	default:
		return cloudflareRecordTypeA
	}
}

func isAllowedCloudflareTTL(ttl int) bool {
	return appcore.IsAllowedCloudflareTTL(ttl)
}

func normalizeDNSPushIPs(raw string) (cloudflareDNSPushIPGroups, []string) {
	return appcore.NormalizeDNSPushIPs(raw)
}

func newCloudflareDNSClient(token string) *cloudflarecore.Client {
	return appcore.NewCloudflareDNSClientWithBaseURL(token, cloudflareAPIBaseURL)
}

func isMaskedSecret(value string) bool {
	return appcore.IsMaskedSecret(value)
}

func cloudflareDNSErrorCode(err error) string {
	return appcore.CloudflareDNSErrorCode(err)
}
