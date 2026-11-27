package appcore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/alidnscore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspodcore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspush"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
)

func (s *Service) newAliyunClient(provider DNSProviderConfig) *alidnscore.Client {
	s.mu.RLock()
	baseURL := s.options.AliyunAPIBaseURL
	s.mu.RUnlock()
	return alidnscore.NewClientWithOptions(alidnscore.ClientOptions{
		AccessKeyID:     provider.APIKey,
		AccessKeySecret: provider.APISecret,
		BaseURL:         baseURL,
		HTTPClient:      s.nonStageHTTPClient(),
	})
}

func (s *Service) newDNSPodClient(provider DNSProviderConfig) *dnspodcore.Client {
	s.mu.RLock()
	baseURL := s.options.DNSPodAPIBaseURL
	s.mu.RUnlock()
	return dnspodcore.NewClientWithOptions(dnspodcore.ClientOptions{
		BaseURL:    baseURL,
		HTTPClient: s.nonStageHTTPClient(),
		SecretID:   provider.APIKey,
		SecretKey:  provider.APISecret,
	})
}

func (s *Service) newDNSHEClient(provider DNSProviderConfig) *dnshecore.Client {
	s.mu.RLock()
	baseURL := s.options.DNSHEAPIBaseURL
	s.mu.RUnlock()
	return dnshecore.NewClientWithOptions(dnshecore.ClientOptions{
		APIKey:     provider.APIKey,
		APISecret:  provider.APISecret,
		BaseURL:    baseURL,
		HTTPClient: s.nonStageHTTPClient(),
	})
}

// PushDNSProviders pushes probe rows to every enabled DNS platform.
func (s *Service) PushDNSProviders(ctx context.Context, payload map[string]any) CommandResult {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.pushDNSProvidersContext(ctx, payload)
}

func (s *Service) pushDNSProvidersContext(ctx context.Context, payload map[string]any) CommandResult {
	snapshot := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	onlyID := strings.TrimSpace(stringValue(firstNonNil(payload["provider_id"], payload["providerId"]), ""))
	providers := DNSProvidersFromSnapshot(snapshot)
	rows := ProbeRowsFromAny(firstNonNil(payload["results"], payload["rows"]))

	targets := make([]map[string]any, 0, len(providers))
	warnings := make([]string, 0)
	totalUpload, succeeded, failed, skipped := 0, 0, 0, 0
	anyRouting, anyPartial := false, false
	// provider_id 只用于结果页单平台推送；本地残留的旧 ID 不要变成"什么都不推"。
	matched := false
	for _, provider := range providers {
		if provider.ID == onlyID {
			matched = true
			break
		}
	}
	for _, provider := range providers {
		if matched && provider.ID != onlyID {
			continue
		}
		if !provider.Enabled {
			continue
		}
		target := s.pushDNSProviderTarget(ctx, provider, snapshot, rows)
		targets = append(targets, target)
		warnings = append(warnings, stringSliceFromAny(target["warnings"])...)
		switch {
		case boolValue(target["skipped"], false):
			skipped++
		case boolValue(target["ok"], false):
			succeeded++
			totalUpload += intValue(target["upload_count"], 0)
		default:
			failed++
		}
		if boolValue(target["routing_enabled"], false) {
			anyRouting = true
		}
		if target["status"] == UploadNotificationStatusPartial {
			anyPartial = true
		}
	}

	data := map[string]any{
		"failed_targets":  failed,
		"providers":       targets,
		"routing_enabled": anyRouting,
		"skipped_targets": skipped,
		"success_targets": succeeded,
		"upload_count":    totalUpload,
		"provider_count":  len(targets),
	}
	if len(targets) == 0 {
		return NewCommandResult("DNS_INPUT_EMPTY", data, "没有启用的 DNS 推送平台，已跳过。", false, nil, nil)
	}
	switch {
	case succeeded == 0 && failed > 0:
		return NewCommandResult("DNS_PUSH_FAILED", data, fmt.Sprintf("DNS 推送失败：%d 个平台全部未完成。", failed), false, nil, dedupeStrings(warnings))
	case failed > 0 || anyPartial:
		return NewCommandResult("DNS_PUSH_PARTIAL", data, fmt.Sprintf("DNS 推送部分完成：成功 %d 个平台，失败 %d 个平台，跳过 %d 个平台。", succeeded, failed, skipped), true, nil, dedupeStrings(warnings))
	case succeeded == 0:
		return NewCommandResult("DNS_INPUT_EMPTY", data, "DNS 推送已跳过：没有可推送的 IP。", false, nil, dedupeStrings(warnings))
	default:
		return NewCommandResult("DNS_PUSH_COMPLETED", data, fmt.Sprintf("DNS 推送完成：成功 %d 个平台，跳过 %d 个平台，共推送 %d 条。", succeeded, skipped, totalUpload), true, nil, dedupeStrings(warnings))
	}
}

func (s *Service) pushDNSProviderTarget(ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow) map[string]any {
	target := map[string]any{
		"enabled":      true,
		"id":           provider.ID,
		"label":        dnsProviderDisplayName(provider),
		"provider":     provider.Provider,
		"record_name":  provider.RecordName,
		"record_type":  provider.RecordType,
		"top_n":        provider.TopN,
		"ttl":          provider.TTL,
		"upload_count": 0,
	}
	var result CommandResult
	switch problem := dnsProviderConfigProblem(provider); {
	case problem != "":
		result = NewCommandResult("DNS_CONFIG_INVALID", emptyDNSProviderPushData(), dnsProviderDisplayName(provider)+" 配置未完成："+problem, false, nil, []string{dnsProviderDisplayName(provider) + " 配置未完成：" + problem})
	case len(rows) == 0:
		result = NewCommandResult("DNS_INPUT_EMPTY", emptyDNSProviderPushData(), "没有可推送的探测结果。", false, nil, nil)
	default:
		result = dnsProviderModuleOrDefault(provider.Provider).push(s, ctx, provider, snapshot, rows)
	}

	inner := mapValue(result.Data)
	target["code"] = result.Code
	target["message"] = result.Message
	target["status"] = dnsProviderTargetStatus(result.Code, result.OK)
	target["ok"] = result.OK
	target["upload_count"] = intValue(firstNonNil(inner["upload_count"], inner["cloudflare_count"]), 0)
	target["warnings"] = result.Warnings
	for _, key := range []string{"summary", "records_after", "ignored_entries", "routing_enabled", "success_targets", "failed_targets", "skipped_targets", "targets"} {
		if value, exists := inner[key]; exists {
			target[key] = value
		}
	}
	if result.Code == "DNS_INPUT_EMPTY" {
		target["skipped"] = true
		target["ok"] = false
	}
	return target
}

func (s *Service) pushDNSHERecordsContext(ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow) CommandResult {
	label := dnsProviderDisplayName(provider)
	if hasUsableRoutingTarget(provider.Routing) {
		return s.pushProviderWithRoutingContext(ctx, provider, snapshot, rows, s.pushDNSHERows, label)
	}
	probeCfg, _ := s.probeConfigFromSnapshot(snapshot)
	selection, err := BuildUploadSelectionWithColoPaths(snapshot, rows, probeCfg.DownloadSpeedMetric, s.ColoPaths())
	if err != nil {
		return NewCommandResult("DNS_CONFIG_INVALID", emptyDNSProviderPushData(), err.Error(), false, nil, nil)
	}
	warnings := append([]string(nil), selection.Warnings...)
	selected := FilterRowsForCloudflareRecordType(limitUploadRows(selection.FilteredRows, provider.TopN, probeCfg.DownloadSpeedMetric), provider.RecordType)
	if len(selected) == 0 {
		warnings = append(warnings, label+"：本次筛选后没有匹配 IP，已跳过 DNS 推送。")
		return NewCommandResult("DNS_INPUT_EMPTY", emptyDNSProviderPushData(), label+" 本次筛选后没有匹配 IP，已跳过。", false, nil, dedupeStrings(warnings))
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	pushResult, pushErr := dnshecore.Push(requestCtx, s.newDNSHEClient(provider), dnsheDNSConfig(provider), probeRowsIPList(selected))
	warnings = append(warnings, pushResult.Warnings...)
	if pushErr != nil {
		return NewCommandResult(dnsheErrorCode(pushErr), map[string]any{
			"ignored_entries": pushResult.IgnoredEntries,
			"summary":         dnsheSummaryMap(pushResult.Summary),
			"upload_count":    0,
		}, fmt.Sprintf("%s 推送失败：%v", label, pushErr), false, nil, dedupeStrings(warnings))
	}
	if !pushResult.HasInputIPs {
		data := emptyDNSProviderPushData()
		data["ignored_entries"] = pushResult.IgnoredEntries
		data["summary"] = dnsheSummaryMap(pushResult.Summary)
		return NewCommandResult("DNS_INPUT_EMPTY", data, "没有可推送的有效 IP。", false, nil, warnings)
	}
	return NewCommandResult("DNS_PUSH_COMPLETED", map[string]any{
		"ignored_entries": pushResult.IgnoredEntries,
		"records_after":   pushResult.RecordsAfter,
		"summary":         dnsheSummaryMap(pushResult.Summary),
		"upload_count":    len(selected),
	}, fmt.Sprintf("%s 推送完成：创建 %d、更新 %d、删除 %d、忽略 %d。", label, pushResult.Summary.Created, pushResult.Summary.Updated, pushResult.Summary.Deleted, pushResult.Summary.Ignored), true, nil, dedupeStrings(warnings))
}

// zonePushFunc runs one provider push for the selected IP list.
type zonePushFunc func(ctx context.Context, ips string) (dnspush.PushResult, error)

func (s *Service) pushAliyunDNSRecordsContext(ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow) CommandResult {
	label := dnsProviderDisplayName(provider)
	if hasUsableRoutingTarget(provider.Routing) {
		return s.pushProviderWithRoutingContext(ctx, provider, snapshot, rows, s.pushAliyunRows, label)
	}
	client := s.newAliyunClient(provider)
	cfg := aliyunDNSConfig(provider)
	return s.pushZoneDNSRecordsContext(ctx, provider, snapshot, rows, func(pushCtx context.Context, ips string) (dnspush.PushResult, error) {
		return alidnscore.Push(pushCtx, client, cfg, ips)
	})
}

func (s *Service) pushDNSPodDNSRecordsContext(ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow) CommandResult {
	label := dnsProviderDisplayName(provider)
	if hasUsableRoutingTarget(provider.Routing) {
		return s.pushProviderWithRoutingContext(ctx, provider, snapshot, rows, s.pushDNSPodRows, label)
	}
	client := s.newDNSPodClient(provider)
	cfg := dnspodDNSConfig(provider)
	return s.pushZoneDNSRecordsContext(ctx, provider, snapshot, rows, func(pushCtx context.Context, ips string) (dnspush.PushResult, error) {
		return dnspodcore.Push(pushCtx, client, cfg, ips)
	})
}

// pushZoneDNSRecordsContext is the shared path for providers that resolve their
// own zone from the record name and reconcile A/AAAA records inside it.
func (s *Service) pushZoneDNSRecordsContext(ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow, push zonePushFunc) CommandResult {
	label := dnsProviderDisplayName(provider)
	probeCfg, _ := s.probeConfigFromSnapshot(snapshot)
	selection, err := BuildUploadSelectionWithColoPaths(snapshot, rows, probeCfg.DownloadSpeedMetric, s.ColoPaths())
	if err != nil {
		return NewCommandResult("DNS_CONFIG_INVALID", emptyDNSProviderPushData(), err.Error(), false, nil, nil)
	}
	warnings := append([]string(nil), selection.Warnings...)
	selected := FilterRowsForCloudflareRecordType(limitUploadRows(selection.FilteredRows, provider.TopN, probeCfg.DownloadSpeedMetric), provider.RecordType)
	if len(selected) == 0 {
		warnings = append(warnings, label+"：本次筛选后没有匹配 IP，已跳过 DNS 推送。")
		return NewCommandResult("DNS_INPUT_EMPTY", emptyDNSProviderPushData(), label+" 本次筛选后没有匹配 IP，已跳过。", false, nil, dedupeStrings(warnings))
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	pushResult, pushErr := push(requestCtx, probeRowsIPList(selected))
	warnings = append(warnings, pushResult.Warnings...)
	summary := dnspush.SummaryMap(pushResult.Summary)
	if pushErr != nil {
		return NewCommandResult(dnsProviderErrorCode(provider.Provider, pushErr), map[string]any{
			"ignored_entries": pushResult.IgnoredEntries,
			"records_after":   pushResult.RecordsAfter,
			"summary":         summary,
			"upload_count":    0,
		}, fmt.Sprintf("%s 推送失败：%v", label, pushErr), false, nil, dedupeStrings(warnings))
	}
	if !pushResult.HasInputIPs {
		data := emptyDNSProviderPushData()
		data["ignored_entries"] = pushResult.IgnoredEntries
		data["summary"] = summary
		return NewCommandResult("DNS_INPUT_EMPTY", data, "没有可推送的有效 IP。", false, nil, warnings)
	}
	return NewCommandResult("DNS_PUSH_COMPLETED", map[string]any{
		"ignored_entries": pushResult.IgnoredEntries,
		"records_after":   pushResult.RecordsAfter,
		"summary":         summary,
		"upload_count":    len(selected),
	}, fmt.Sprintf("%s 推送完成：创建 %d、更新 %d、删除 %d、忽略 %d。", label, pushResult.Summary.Created, pushResult.Summary.Updated, pushResult.Summary.Deleted, pushResult.Summary.Ignored), true, nil, dedupeStrings(warnings))
}

// dnsProviderErrorCode maps a provider API error code into the DNS_* result code
// so the UI can show one stable code per failure kind.
func dnsProviderErrorCode(provider string, err error) string {
	if errors.Is(err, dnspush.ErrCNAMEProtected) {
		return "DNS_CNAME_PROTECTED"
	}
	code := ""
	switch provider {
	case DNSProviderAliyun:
		code = alidnscore.ErrorCode(err)
	case DNSProviderDNSPod:
		code = dnspodcore.ErrorCode(err)
	case DNSProviderDNSHE:
		code = dnshecore.ErrorCode(err)
	}
	if code == "" {
		return "DNS_PUSH_FAILED"
	}
	var builder strings.Builder
	for _, char := range strings.ToUpper(code) {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('_')
	}
	return "DNS_" + builder.String()
}

func emptyDNSProviderPushData() map[string]any {
	return map[string]any{
		"ignored_entries": []string{},
		"records_after":   []dnshecore.Record{},
		"summary":         dnspush.SummaryMap(dnspush.Summary{}),
		"upload_count":    0,
	}
}

func dnsheErrorCode(err error) string {
	if errors.Is(err, dnspush.ErrCNAMEProtected) {
		return "DNS_CNAME_PROTECTED"
	}
	if code := dnshecore.ErrorCode(err); code != "" {
		return "DNS_" + strings.ToUpper(strings.ReplaceAll(code, "-", "_"))
	}
	return "DNS_PUSH_FAILED"
}

func dnsheSummaryMap(summary dnspush.Summary) map[string]any {
	return dnspush.SummaryMap(summary)
}

func dnsProviderTargetStatus(code string, ok bool) string {
	switch {
	case ok && strings.Contains(code, "PARTIAL"):
		return UploadNotificationStatusPartial
	case ok:
		return UploadNotificationStatusCompleted
	case strings.Contains(code, "INPUT_EMPTY"):
		return UploadNotificationStatusSkipped
	default:
		return UploadNotificationStatusFailed
	}
}

func (s *Service) attachManualDNSNotification(payload map[string]any, result CommandResult) CommandResult {
	if UploadNotificationTriggerFromPayload(payload) != UploadNotificationSourceManualPush {
		return result
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	notification := BuildUploadNotification(UploadNotificationInput{
		CreatedAt:  s.now(),
		Message:    result.Message,
		Providers:  DNSProviderReportsFromCommandData(result.Data),
		Source:     UploadNotificationSourceManualPush,
		TaskID:     CommandResultTaskID(payload, result),
		TopEntries: s.manualUploadNotificationTopEntries(payload),
	})
	warnings := s.sendUploadNotification(context.Background(), config, notification)
	result.Data = CommandResultDataWithUploadNotification(result.Data, notification)
	result.Warnings = dedupeStrings(append(result.Warnings, warnings...))
	return result
}

// DNSProviderReportsFromCommandData maps an aggregate push result into the
// provider list used by upload notifications.
func DNSProviderReportsFromCommandData(data any) []UploadProviderResult {
	mapped := commandResultDataMap(data)
	results := make([]UploadProviderResult, 0)
	for _, item := range configItemMaps(mapped["providers"]) {
		status := normalizeUploadNotificationStatus(stringValue(item["status"], ""))
		if status == "" {
			continue
		}
		results = append(results, UploadProviderResult{
			ID:          stringValue(item["id"], ""),
			Label:       dnsProviderReportLabel(item),
			Provider:    stringValue(item["provider"], ""),
			Status:      status,
			UploadCount: max(0, intValue(item["upload_count"], 0)),
		})
	}
	return results
}

func dnsProviderReportLabel(item map[string]any) string {
	if label := strings.TrimSpace(stringValue(item["label"], "")); label != "" {
		return label
	}
	name := strings.TrimSpace(stringValue(item["record_name"], ""))
	platform := dnsProviderPlatformName(stringValue(item["provider"], ""))
	if name == "" {
		return platform
	}
	return platform + " (" + name + ")"
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(stringValue(item, "")); text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}
