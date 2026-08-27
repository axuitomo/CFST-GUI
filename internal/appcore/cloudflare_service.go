package appcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/cloudflarecore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func (s *Service) invokeCloudflareList(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	cfg, warnings, err := CloudflareDNSListConfigFromPayload(payload)
	if err != nil {
		return NewCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}
	options, err := cloudflareDNSListOptionsFromPayload(payload, cfg)
	if err != nil {
		return NewCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	records, err := s.newCloudflareClient(cfg.APIToken).ListRecordsWithOptions(ctx, cfg, options)
	if err != nil {
		return NewCommandResult("DNS_LIST_FAILED", nil, err.Error(), false, nil, warnings)
	}
	target := "current zone"
	if strings.TrimSpace(options.Name) != "" {
		target = strings.TrimSpace(options.Name)
	}
	return NewCommandResult("DNS_RECORDS_LISTED", map[string]any{
		"count": len(records), "records": records,
	}, fmt.Sprintf("Read %d Cloudflare DNS records matching %s.", len(records), target), true, nil, warnings)
}

func (s *Service) invokeCloudflarePush(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	result := s.pushCloudflareDNSRecordsContext(context.Background(), payload)
	return s.attachManualUploadNotification(payload, UploadNotificationProviderCloudflare, result)
}

func (s *Service) PushCloudflareDNS(ctx context.Context, payload map[string]any) CommandResult {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.pushCloudflareDNSRecordsContext(ctx, payload)
}

func (s *Service) pushCloudflareDNSRecordsContext(ctx context.Context, payload map[string]any) CommandResult {
	cfgPayload := cloudflareDNSConfigPayloadForPush(payload)
	cfg, warnings, err := CloudflareDNSConfigFromPayload(cfgPayload)
	if err != nil {
		return NewCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}
	cfg.RecordType = cloudflareDNSRecordTypeFromPayload(cfgPayload, cfg.RecordType)
	cfg.Proxied = false

	if rawRows := firstNonNil(payload["results"], payload["rows"]); rawRows != nil {
		rows := ProbeRowsFromAny(rawRows)
		if len(rows) == 0 {
			return NewCommandResult("DNS_INPUT_EMPTY", nil, "没有可推送的探测结果。", false, nil, warnings)
		}
		config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
		probeCfg, _ := s.probeConfigFromSnapshot(config)
		selection, selectErr := BuildUploadSelectionWithColoPaths(config, rows, probeCfg.DownloadSpeedMetric, s.ColoPaths())
		if selectErr != nil {
			return NewCommandResult("DNS_CONFIG_INVALID", nil, selectErr.Error(), false, nil, warnings)
		}
		warnings = append(warnings, selection.Warnings...)
		if routes, routeWarnings := BuildCloudflareRouteSelections(config, selection.FilteredRows, probeCfg.DownloadSpeedMetric, s.ColoPaths()); len(routes) > 0 {
			warnings = append(warnings, routeWarnings...)
			return s.pushCloudflareDNSCombinedSelections(ctx, cfg, selection, routes, warnings, cloudflarePayloadHasRecordName(payload))
		}
		rows = FilterRowsForCloudflareRecordType(selection.CloudflareRows, cfg.RecordType)
		if len(rows) == 0 {
			return NewCommandResult("DNS_INPUT_EMPTY", emptyCloudflarePushData(), "本次筛选后没有匹配 IP，已跳过 DNS 推送。", false, nil, warnings)
		}
		payload = cloneAnyMap(payload)
		payload["ipsRaw"] = probeRowsIPList(rows)
	}

	ipsRaw := stringValue(firstNonNil(payload["ipsRaw"], payload["ips_raw"]), "")
	requestCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	pushResult, err := PushCloudflareDNSRecords(requestCtx, s.newCloudflareClient(cfg.APIToken), cfg, ipsRaw)
	if err != nil {
		warnings = append(warnings, pushResult.Warnings...)
		return NewCommandResult(CloudflareDNSErrorCode(err), map[string]any{
			"ignored_entries": pushResult.IgnoredEntries,
			"records_before":  pushResult.RecordsAfter,
			"summary":         CloudflareSummaryMap(pushResult.Summary),
		}, err.Error(), false, nil, dedupeStrings(warnings))
	}
	warnings = append(warnings, pushResult.Warnings...)
	if !pushResult.HasInputIPs {
		data := emptyCloudflarePushData()
		data["ignored_entries"] = pushResult.IgnoredEntries
		data["summary"] = CloudflareSummaryMap(pushResult.Summary)
		return NewCommandResult("DNS_INPUT_EMPTY", data, "没有可推送的有效 IP。", false, nil, warnings)
	}
	return NewCommandResult("DNS_PUSH_COMPLETED", map[string]any{
		"ignored_entries": pushResult.IgnoredEntries, "records_after": pushResult.RecordsAfter,
		"summary": CloudflareSummaryMap(pushResult.Summary), "upload_count": len(normalizeDNSPushIPsForCount(ipsRaw)),
	}, fmt.Sprintf("Cloudflare DNS replacement completed: created %d, updated %d, deleted %d, ignored %d.", pushResult.Summary.Created, pushResult.Summary.Updated, pushResult.Summary.Deleted, pushResult.Summary.Ignored), true, nil, dedupeStrings(warnings))
}

func (s *Service) pushCloudflareDNSCombinedSelections(parent context.Context, baseCfg CloudflareDNSConfig, selection UploadSelectionResult, routes []UploadCloudflareRouteSelection, warnings []string, includePrimary bool) CommandResult {
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel()
	client := s.newCloudflareClient(baseCfg.APIToken)
	targets := make([]map[string]any, 0, len(routes)+1)
	totalUploadCount, successCount, failureCount, skippedCount := 0, 0, 0, 0

	addTarget := func(cfg CloudflareDNSConfig, rows []probecore.ProbeRow, label, kind string, route *UploadCloudflareRouteSelection) {
		var target map[string]any
		var ok, skipped bool
		var uploadCount int
		var targetWarnings []string
		if route != nil && route.Skipped {
			target = s.skippedCloudflareDNSTarget(cfg, route.Warnings, label, kind)
			skipped = true
		} else {
			target, ok, skipped, uploadCount, targetWarnings = s.pushCloudflareDNSTarget(ctx, client, cfg, rows, label, kind)
		}
		if route != nil {
			target["filtered_count"] = route.FilteredCount
			target["input_count"] = route.InputCount
			target["rule_name"] = route.Rule.Name
			target["selected_count"] = len(route.Rows)
			if len(route.Warnings) > 0 && !route.Skipped {
				targetWarnings = append(route.Warnings, targetWarnings...)
				target["warnings"] = dedupeStrings(targetWarnings)
			}
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

	if includePrimary {
		rows := FilterRowsForCloudflareRecordType(selection.CloudflareRows, baseCfg.RecordType)
		addTarget(baseCfg, rows, cloudflareTargetLabel("主目标", baseCfg.RecordName), "primary", nil)
	}
	for index := range routes {
		route := routes[index]
		cfg := baseCfg
		cfg.RecordName, cfg.RecordType = route.Rule.RecordName, route.Rule.RecordType
		rows := FilterRowsForCloudflareRecordType(route.Rows, cfg.RecordType)
		addTarget(cfg, rows, cloudflareTargetLabel("分流目标", cloudflareRouteLabel(route.Rule)), "route", &route)
	}
	data := map[string]any{
		"cloudflare_count": totalUploadCount, "failed_targets": failureCount, "filtered_count": len(selection.FilteredRows),
		"input_count": len(selection.InputRows), "routing_enabled": len(routes) > 0, "skipped_targets": skippedCount,
		"success_targets": successCount, "targets": targets, "upload_count": totalUploadCount,
	}
	if successCount == 0 {
		if failureCount > 0 {
			return NewCommandResult("DNS_PUSH_FAILED", data, "Cloudflare push failed: no target completed.", false, nil, dedupeStrings(warnings))
		}
		return NewCommandResult("DNS_INPUT_EMPTY", data, "Cloudflare push skipped: no target had uploadable IPs.", false, nil, dedupeStrings(warnings))
	}
	if failureCount > 0 {
		return NewCommandResult("DNS_PUSH_PARTIAL", data, fmt.Sprintf("Cloudflare 推送部分完成：成功 %d 个目标，失败 %d 个目标，跳过 %d 个目标。", successCount, failureCount, skippedCount), true, nil, dedupeStrings(warnings))
	}
	return NewCommandResult("DNS_PUSH_COMPLETED", data, fmt.Sprintf("Cloudflare push completed: %d succeeded, %d skipped.", successCount, skippedCount), true, nil, dedupeStrings(warnings))
}

func (s *Service) skippedCloudflareDNSTarget(cfg CloudflareDNSConfig, routeWarnings []string, label, kind string) map[string]any {
	message := "Cloudflare " + label + " skipped."
	if len(routeWarnings) > 0 {
		message = routeWarnings[len(routeWarnings)-1]
	}
	target := map[string]any{
		"kind": kind, "message": message, "ok": false, "record_name": cfg.RecordName, "record_type": cfg.RecordType,
		"skipped": true, "summary": CloudflareSummaryMap(CloudflareDNSPushSummary{}), "upload_count": 0,
		"warnings": dedupeStrings(routeWarnings),
	}
	s.logCloudflarePushTarget("cloudflare.push.target_skipped", kind, cfg, 0, message, nil, nil)
	return target
}

func (s *Service) pushCloudflareDNSTarget(ctx context.Context, client *cloudflarecore.Client, cfg CloudflareDNSConfig, rows []probecore.ProbeRow, label, kind string) (map[string]any, bool, bool, int, []string) {
	target := map[string]any{
		"kind": kind, "record_name": cfg.RecordName, "record_type": cfg.RecordType,
		"summary": CloudflareSummaryMap(CloudflareDNSPushSummary{}), "upload_count": len(rows),
	}
	if len(rows) == 0 {
		message := fmt.Sprintf("Cloudflare %s: record type %s has no matching IPs and was skipped.", label, cfg.RecordType)
		target["message"], target["ok"], target["skipped"], target["warnings"] = message, false, true, []string{message}
		s.logCloudflarePushTarget("cloudflare.push.target_skipped", kind, cfg, 0, message, nil, nil)
		return target, false, true, 0, []string{message}
	}
	result, err := PushCloudflareDNSRecords(ctx, client, cfg, probeRowsIPList(rows))
	if err != nil {
		message := fmt.Sprintf("Cloudflare %s推送失败：%s", label, err)
		target["error"], target["message"], target["ok"], target["skipped"], target["warnings"] = err.Error(), message, false, false, []string{message}
		s.logCloudflarePushTarget("cloudflare.push.target_failed", kind, cfg, len(rows), message, nil, err)
		return target, false, false, 0, []string{message}
	}
	message := fmt.Sprintf("Cloudflare %s push completed: created %d, updated %d, deleted %d, ignored %d.", label, result.Summary.Created, result.Summary.Updated, result.Summary.Deleted, result.Summary.Ignored)
	warnings := append([]string{message}, result.Warnings...)
	target["ignored_entries"], target["message"], target["ok"] = result.IgnoredEntries, message, true
	target["records_after"], target["skipped"], target["summary"], target["warnings"] = result.RecordsAfter, false, CloudflareSummaryMap(result.Summary), dedupeStrings(warnings)
	s.logCloudflarePushTarget("cloudflare.push.target_completed", kind, cfg, len(rows), message, result.Summary, nil)
	return target, true, false, len(rows), warnings
}

func cloudflareDNSListOptionsFromPayload(payload map[string]any, cfg CloudflareDNSConfig) (cloudflarecore.ListOptions, error) {
	scope := strings.ToLower(strings.TrimSpace(stringValue(firstNonNil(payload["scope"], payload["filter"], payload["mode"]), "")))
	name := strings.TrimSpace(stringValue(firstNonNil(payload["name"], payload["record_name"], payload["recordName"]), ""))
	recordType := strings.TrimSpace(stringValue(firstNonNil(payload["record_type"], payload["recordType"], payload["type"]), ""))
	switch scope {
	case "zone", "all", "domain":
		name = ""
	case "custom", "subdomain", "name":
		if name == "" {
			return cloudflarecore.ListOptions{}, errorsNew("缺少要读取的 Cloudflare DNS 记录名称")
		}
	default:
		if name == "" {
			name = strings.TrimSpace(cfg.RecordName)
		}
		if name == "" {
			return cloudflarecore.ListOptions{}, errorsNew("缺少 Cloudflare DNS 记录名称")
		}
	}
	return cloudflarecore.ListOptions{Name: name, Type: recordType}, nil
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
	routing := CloudflareRoutingConfigFromSnapshot(config)
	if !routing.Enabled {
		return payload
	}
	for _, rule := range routing.Rules {
		if !rule.Enabled || strings.TrimSpace(rule.RecordName) == "" {
			continue
		}
		nextPayload, nextConfig, nextCloudflare := cloneAnyMap(payload), cloneAnyMap(config), cloneAnyMap(cloudflare)
		nextCloudflare["record_name"], nextCloudflare["record_type"] = rule.RecordName, rule.RecordType
		nextConfig["cloudflare"], nextPayload["config"] = nextCloudflare, nextConfig
		return nextPayload
	}
	return payload
}

func cloudflareDNSRecordTypeFromPayload(payload map[string]any, fallback string) string {
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"]))
	if len(config) == 0 {
		config = payload
	}
	cloudflare := mapValue(config["cloudflare"])
	if len(cloudflare) == 0 {
		cloudflare = config
	}
	switch strings.ToUpper(strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_type"], cloudflare["recordType"]), fallback))) {
	case cloudflareRecordTypeAll:
		return cloudflareRecordTypeAll
	case CloudflareRecordTypeAAAA:
		return CloudflareRecordTypeAAAA
	default:
		return CloudflareRecordTypeA
	}
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

func cloudflareRouteLabel(rule UploadCloudflareRoutingRule) string {
	if strings.TrimSpace(rule.Name) != "" {
		return strings.TrimSpace(rule.Name)
	}
	if strings.TrimSpace(rule.RecordName) != "" {
		return strings.TrimSpace(rule.RecordName)
	}
	return "unnamed rule"
}

func cloudflareTargetLabel(prefix, name string) string {
	if strings.TrimSpace(name) == "" {
		return prefix
	}
	return prefix + " " + strings.TrimSpace(name)
}

func (s *Service) logCloudflarePushTarget(event, kind string, cfg CloudflareDNSConfig, uploadCount int, message string, summary any, err error) {
	fields := map[string]any{"message": message, "record_name": cfg.RecordName, "record_type": cfg.RecordType, "target_kind": kind, "upload_count": uploadCount}
	if summary != nil {
		fields["summary"] = summary
	}
	if err != nil {
		fields["error"], fields["level"] = err.Error(), "error"
	}
	s.DebugLogger().Event(event, fields)
}

func normalizeDNSPushIPsForCount(raw string) []string {
	groups, _ := NormalizeDNSPushIPs(raw)
	return append(append(make([]string, 0, len(groups.A)+len(groups.AAAA)), groups.A...), groups.AAAA...)
}

func probeRowsIPList(rows []probecore.ProbeRow) string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if ip := strings.TrimSpace(row.IP); ip != "" {
			values = append(values, ip)
		}
	}
	return strings.Join(values, "\n")
}

func emptyCloudflarePushData() map[string]any {
	return map[string]any{
		"ignored_entries": []string{}, "records_after": []CloudflareDNSRecord{},
		"summary": CloudflareSummaryMap(CloudflareDNSPushSummary{}), "upload_count": 0,
	}
}

func errorsNew(message string) error { return fmt.Errorf("%s", message) }

func (s *Service) newCloudflareClient(token string) *cloudflarecore.Client {
	s.mu.RLock()
	baseURL, client := s.options.CloudflareAPIBaseURL, s.options.HTTPClient
	s.mu.RUnlock()
	return cloudflarecore.NewClientWithOptions(cloudflarecore.ClientOptions{BaseURL: baseURL, HTTPClient: client, Token: token})
}
