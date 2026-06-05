package app

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

func (a *App) ListCloudflareDNSRecords(payload map[string]any) DesktopCommandResult {
	cfg, warnings, err := appcore.CloudflareDNSListConfigFromPayload(payload)
	if err != nil {
		return desktopCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}

	options, err := cloudflareDNSListOptionsFromPayload(payload, cfg)
	if err != nil {
		return desktopCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	records, err := newCloudflareDNSClient(cfg.APIToken).ListRecordsWithOptions(ctx, cfg, options)
	if err != nil {
		return desktopCommandResult("DNS_LIST_FAILED", nil, err.Error(), false, nil, warnings)
	}

	target := "当前 Zone"
	if strings.TrimSpace(options.Name) != "" {
		target = strings.TrimSpace(options.Name)
	}
	return desktopCommandResult("DNS_RECORDS_LISTED", map[string]any{
		"count":   len(records),
		"records": records,
	}, fmt.Sprintf("已读取 Cloudflare 中匹配 %s 的 DNS 记录 %d 条。", target, len(records)), true, nil, warnings)
}

func (a *App) PushCloudflareDNSRecords(payload map[string]any) DesktopCommandResult {
	cfgPayload := cloudflareDNSConfigPayloadForPush(payload)
	cfg, warnings, err := cloudflareDNSConfigFromPayload(cfgPayload)
	if err != nil {
		return desktopCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}
	cfg.RecordType = cloudflareDNSRecordTypeFromPayload(cfgPayload, cfg.RecordType)
	cfg.Proxied = false

	if rawRows := firstNonNil(payload["results"], payload["rows"]); rawRows != nil {
		rows := probeRowsFromAny(rawRows)
		if len(rows) == 0 {
			return desktopCommandResult("DNS_INPUT_EMPTY", nil, "没有可推送的测速结果。", false, nil, warnings)
		}
		config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
		probeCfg, _ := desktopConfigToProbeConfig(config)
		selection, selectErr := BuildUploadSelection(config, rows, probeCfg.DownloadSpeedMetric)
		if selectErr != nil {
			return desktopCommandResult("DNS_CONFIG_INVALID", nil, selectErr.Error(), false, nil, warnings)
		}
		warnings = append(warnings, selection.Warnings...)
		if routeSelections, routeWarnings := appcore.BuildCloudflareRouteSelections(config, selection.FilteredRows, probeCfg.DownloadSpeedMetric, desktopColoDictionaryPaths()); len(routeSelections) > 0 {
			warnings = append(warnings, routeWarnings...)
			return a.pushCloudflareDNSCombinedSelections(cfg, selection, routeSelections, warnings, cloudflarePayloadHasRecordName(payload))
		}
		rows = filterRowsForCloudflareRecordType(selection.CloudflareRows, cfg.RecordType)
		if len(rows) == 0 {
			return desktopCommandResult("DNS_INPUT_EMPTY", map[string]any{
				"ignored_entries": []string{},
				"records_after":   []CloudflareDNSRecord{},
				"summary":         cloudflareSummaryMap(cloudflareDNSPushSummary{}),
				"upload_count":    0,
			}, "本次筛选后无匹配 IP，已跳过 DNS 推送。", false, nil, warnings)
		}
		payload = cloneMap(payload)
		payload["ipsRaw"] = probeRowsIPList(rows)
	}

	ipsRaw := stringValue(firstNonNil(payload["ipsRaw"], payload["ips_raw"]), "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := appcore.PushCloudflareDNSRecords(ctx, newCloudflareDNSClient(cfg.APIToken), cfg, ipsRaw)
	if err != nil {
		return desktopCommandResult(cloudflareDNSErrorCode(err), nil, err.Error(), false, nil, warnings)
	}
	warnings = append(warnings, result.Warnings...)
	if !result.HasInputIPs {
		return desktopCommandResult("DNS_INPUT_EMPTY", map[string]any{
			"ignored_entries": result.IgnoredEntries,
			"records_after":   []CloudflareDNSRecord{},
			"summary":         cloudflareSummaryMap(result.Summary),
			"upload_count":    0,
		}, "没有可推送的有效 IP。", false, nil, warnings)
	}

	return desktopCommandResult("DNS_PUSH_COMPLETED", map[string]any{
		"ignored_entries": result.IgnoredEntries,
		"records_after":   result.RecordsAfter,
		"summary":         cloudflareSummaryMap(result.Summary),
		"upload_count":    len(normalizeDNSPushIPsForCount(ipsRaw)),
	}, fmt.Sprintf("Cloudflare DNS 覆盖推送完成：创建 %d、更新 %d、删除 %d、忽略 %d。", result.Summary.Created, result.Summary.Updated, result.Summary.Deleted, result.Summary.Ignored), true, nil, dedupeStrings(warnings))
}

func (a *App) pushCloudflareDNSCombinedSelections(baseCfg cloudflareDNSConfig, selection UploadSelectionResult, routes []appcore.UploadCloudflareRouteSelection, warnings []string, includePrimary bool) DesktopCommandResult {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client := newCloudflareDNSClient(baseCfg.APIToken)
	targets := make([]map[string]any, 0, len(routes)+1)
	totalUploadCount := 0
	successCount := 0
	failureCount := 0
	skippedCount := 0

	if includePrimary {
		rows := filterRowsForCloudflareRecordType(selection.CloudflareRows, baseCfg.RecordType)
		target, ok, skipped, uploadCount, targetWarnings := pushCloudflareDNSTarget(ctx, client, baseCfg, rows, cloudflareTargetLabel("主目标", baseCfg.RecordName), "primary")
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
			target := skippedCloudflareDNSTarget(cfg, route.Warnings, cloudflareTargetLabel("分流目标", routeLabel(rule)), "route")
			target["filtered_count"] = route.FilteredCount
			target["input_count"] = route.InputCount
			target["rule_name"] = rule.Name
			target["selected_count"] = len(route.Rows)
			targets = append(targets, target)
			warnings = append(warnings, route.Warnings...)
			skippedCount++
			continue
		}
		rows := filterRowsForCloudflareRecordType(route.Rows, cfg.RecordType)
		label := cloudflareTargetLabel("分流目标", routeLabel(rule))
		target, ok, skipped, uploadCount, targetWarnings := pushCloudflareDNSTarget(ctx, client, cfg, rows, label, "route")
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
		return desktopCommandResult("DNS_INPUT_EMPTY", data, message, false, nil, dedupeStrings(warnings))
	}
	if failureCount > 0 {
		return desktopCommandResult("DNS_PUSH_PARTIAL", data, fmt.Sprintf("Cloudflare 推送部分完成：成功 %d 个目标，失败 %d 个目标，跳过 %d 个目标。", successCount, failureCount, skippedCount), true, nil, dedupeStrings(warnings))
	}
	return desktopCommandResult("DNS_PUSH_COMPLETED", data, fmt.Sprintf("Cloudflare 推送完成：成功 %d 个目标，跳过 %d 个目标。", successCount, skippedCount), true, nil, dedupeStrings(warnings))
}

func skippedCloudflareDNSTarget(cfg cloudflareDNSConfig, routeWarnings []string, label string, targetKind string) map[string]any {
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
	logCloudflarePushTarget("cloudflare.push.target_skipped", targetKind, cfg, 0, message, nil, nil)
	return target
}

func pushCloudflareDNSTarget(ctx context.Context, client *cloudflarecore.Client, cfg cloudflareDNSConfig, rows []ProbeRow, label string, targetKind string) (map[string]any, bool, bool, int, []string) {
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
		logCloudflarePushTarget("cloudflare.push.target_skipped", targetKind, cfg, len(rows), message, nil, nil)
		return target, false, true, 0, []string{message}
	}

	result, err := appcore.PushCloudflareDNSRecords(ctx, client, cfg, probeRowsIPList(rows))
	if err != nil {
		message := fmt.Sprintf("Cloudflare %s推送失败：%s", label, err.Error())
		target["error"] = err.Error()
		target["message"] = message
		target["ok"] = false
		target["skipped"] = false
		target["warnings"] = []string{message}
		logCloudflarePushTarget("cloudflare.push.target_failed", targetKind, cfg, len(rows), message, nil, err)
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
	logCloudflarePushTarget("cloudflare.push.target_completed", targetKind, cfg, len(rows), message, result.Summary, nil)
	return target, true, false, len(rows), warnings
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

func (a *App) pushCloudflareDNSRouteSelections(baseCfg cloudflareDNSConfig, selection UploadSelectionResult, routes []appcore.UploadCloudflareRouteSelection, warnings []string) DesktopCommandResult {
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
		rows := filterRowsForCloudflareRecordType(route.Rows, cfg.RecordType)
		target["upload_count"] = len(rows)
		if len(rows) == 0 {
			skippedCount++
			targetWarnings = append(targetWarnings, fmt.Sprintf("Cloudflare 分流规则「%s」：记录类型 %s 无匹配 IP，已跳过。", routeLabel(rule), cfg.RecordType))
			target["ok"] = false
			target["skipped"] = true
			target["warnings"] = dedupeStrings(targetWarnings)
			targets = append(targets, target)
			warnings = append(warnings, targetWarnings...)
			continue
		}

		result, err := appcore.PushCloudflareDNSRecords(ctx, client, cfg, probeRowsIPList(rows))
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
		return desktopCommandResult("DNS_INPUT_EMPTY", data, message, false, nil, dedupeStrings(warnings))
	}
	if failureCount > 0 {
		return desktopCommandResult("DNS_PUSH_PARTIAL", data, fmt.Sprintf("Cloudflare 分流推送部分完成：成功 %d 个目标，失败 %d 个目标，跳过 %d 个目标。", successCount, failureCount, skippedCount), true, nil, dedupeStrings(warnings))
	}
	return desktopCommandResult("DNS_PUSH_COMPLETED", data, fmt.Sprintf("Cloudflare 分流推送完成：成功 %d 个目标，跳过 %d 个目标。", successCount, skippedCount), true, nil, dedupeStrings(warnings))
}

func cloudflareDNSConfigPayloadForPush(payload map[string]any) map[string]any {
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
		nextPayload := cloneMap(payload)
		nextConfig := cloneMap(config)
		nextCloudflare := cloneMap(cloudflare)
		nextCloudflare["record_name"] = rule.RecordName
		nextCloudflare["record_type"] = rule.RecordType
		nextConfig["cloudflare"] = nextCloudflare
		nextPayload["config"] = nextConfig
		return nextPayload
	}
	return payload
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
	recordType := strings.ToUpper(strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_type"], cloudflare["recordType"]), fallback)))
	switch recordType {
	case cloudflareRecordTypeAll, cloudflareRecordTypeAAAA:
		return recordType
	default:
		return cloudflareRecordTypeA
	}
}

func routeLabel(rule appcore.UploadCloudflareRoutingRule) string {
	if strings.TrimSpace(rule.Name) != "" {
		return strings.TrimSpace(rule.Name)
	}
	if strings.TrimSpace(rule.RecordName) != "" {
		return strings.TrimSpace(rule.RecordName)
	}
	return "未命名规则"
}

func cloudflarePayloadHasRecordName(payload map[string]any) bool {
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

func cloudflareTargetLabel(prefix string, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "「" + prefix + "」"
	}
	return "「" + prefix + " " + name + "」"
}

func logCloudflarePushTarget(event string, targetKind string, cfg cloudflareDNSConfig, uploadCount int, message string, summary any, err error) {
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

func normalizeDNSPushIPsForCount(raw string) []string {
	groups, _ := normalizeDNSPushIPs(raw)
	values := make([]string, 0, len(groups.A)+len(groups.AAAA))
	values = append(values, groups.A...)
	values = append(values, groups.AAAA...)
	return values
}

func cloudflareSummaryMap(summary cloudflareDNSPushSummary) map[string]any {
	return appcore.CloudflareSummaryMap(summary)
}

func cloudflareDNSConfigFromPayload(payload map[string]any) (cloudflareDNSConfig, []string, error) {
	return appcore.CloudflareDNSConfigFromPayload(payload)
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
