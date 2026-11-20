package appcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/alidnscore"
	"github.com/axuitomo/CFST-GUI/internal/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/dnspodcore"
	"github.com/axuitomo/CFST-GUI/internal/dnspush"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

// providerRowsPush runs one provider push against an already-selected IP list.
// Routing targets call it once per target with a different record name.
type providerRowsPush func(ctx context.Context, provider DNSProviderConfig, rows []probecore.ProbeRow) (dnspush.PushResult, error)

// pushProviderWithRoutingContext pushes the provider's main record (when
// configured) plus every enabled routing rule. It is used by zone-scoped
// providers; Cloudflare keeps its own combined path for proxied/summary details.
func (s *Service) pushProviderWithRoutingContext(ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow, push providerRowsPush, label string) CommandResult {
	probeCfg, _ := s.probeConfigFromSnapshot(snapshot)
	selection, err := BuildUploadSelectionWithColoPaths(snapshot, rows, probeCfg.DownloadSpeedMetric, s.ColoPaths())
	if err != nil {
		return NewCommandResult("DNS_CONFIG_INVALID", emptyDNSProviderPushData(), err.Error(), false, nil, nil)
	}
	warnings := append([]string(nil), selection.Warnings...)
	routes, routeWarnings := BuildCloudflareRouteSelections(dnsProviderSnapshot(snapshot, provider), selection.FilteredRows, probeCfg.DownloadSpeedMetric, s.ColoPaths())
	warnings = append(warnings, routeWarnings...)

	pushCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	targets := make([]map[string]any, 0, len(routes)+1)
	totalUpload, successCount, failureCount, skippedCount := 0, 0, 0, 0
	totalSummary := dnspush.Summary{}
	ignored := make([]string, 0)
	recordsAfter := []dnspush.Record{}
	var firstErr error

	pushTarget := func(cfg DNSProviderConfig, targetRows []probecore.ProbeRow, kind, targetLabel string, route *UploadCloudflareRouteSelection) {
		target := map[string]any{
			"kind":         kind,
			"record_name":  cfg.RecordName,
			"record_type":  cfg.RecordType,
			"top_n":        cfg.TopN,
			"summary":      dnspush.SummaryMap(dnspush.Summary{}),
			"upload_count": 0,
		}
		if route != nil {
			target["filtered_count"] = route.FilteredCount
			target["input_count"] = route.InputCount
			target["rule_name"] = route.Rule.Name
			target["selected_count"] = len(route.Rows)
		}
		if route != nil && route.Skipped {
			message := fmt.Sprintf("%s 分流目标 %s 已跳过：筛选后没有匹配 IP。", label, targetLabel)
			target["message"], target["ok"], target["skipped"] = message, false, true
			target["warnings"] = dedupeStrings(route.Warnings)
			targets = append(targets, target)
			skippedCount++
			warnings = append(warnings, route.Warnings...)
			return
		}
		if len(targetRows) == 0 {
			message := fmt.Sprintf("%s 目标 %s：记录类型 %s 没有匹配 IP，已跳过。", label, targetLabel, cfg.RecordType)
			target["message"], target["ok"], target["skipped"], target["warnings"] = message, false, true, []string{message}
			targets = append(targets, target)
			skippedCount++
			warnings = append(warnings, message)
			return
		}
		targetCtx, targetCancel := context.WithTimeout(pushCtx, time.Minute)
		result, pushErr := push(targetCtx, cfg, targetRows)
		targetCancel()
		warnings = append(warnings, result.Warnings...)
		if route != nil && len(route.Warnings) > 0 {
			warnings = append(warnings, route.Warnings...)
		}
		if pushErr != nil {
			message := fmt.Sprintf("%s 目标 %s 推送失败：%v", label, targetLabel, pushErr)
			target["message"], target["ok"], target["skipped"], target["error"] = message, false, false, pushErr.Error()
			target["warnings"] = dedupeStrings(append(routeWarningsFromRoute(route), result.Warnings...))
			targets = append(targets, target)
			failureCount++
			if firstErr == nil {
				firstErr = pushErr
			}
			warnings = append(warnings, message)
			return
		}
		message := fmt.Sprintf("%s 目标 %s 推送完成：创建 %d、更新 %d、删除 %d、忽略 %d。", label, targetLabel, result.Summary.Created, result.Summary.Updated, result.Summary.Deleted, result.Summary.Ignored)
		target["message"], target["ok"], target["skipped"] = message, true, false
		target["summary"], target["upload_count"] = dnspush.SummaryMap(result.Summary), len(targetRows)
		target["records_after"] = result.RecordsAfter
		target["warnings"] = dedupeStrings(append(routeWarningsFromRoute(route), result.Warnings...))
		targets = append(targets, target)
		successCount++
		totalUpload += len(targetRows)
		totalSummary.Created += result.Summary.Created
		totalSummary.Updated += result.Summary.Updated
		totalSummary.Deleted += result.Summary.Deleted
		totalSummary.Ignored += result.Summary.Ignored
		ignored = append(ignored, result.IgnoredEntries...)
		if len(result.RecordsAfter) > 0 {
			recordsAfter = result.RecordsAfter
		}
		warnings = append(warnings, message)
	}

	if dnsProviderHasPrimaryTarget(provider) {
		primaryRows := FilterRowsForCloudflareRecordType(limitUploadRows(selection.FilteredRows, provider.TopN, probeCfg.DownloadSpeedMetric), provider.RecordType)
		pushTarget(provider, primaryRows, "primary", cloudflareTargetLabel("主目标", dnsProviderPrimaryTargetLabel(provider)), nil)
	}
	for index := range routes {
		route := routes[index]
		cfg := provider
		cfg.RecordName, cfg.RecordType, cfg.TopN = route.Rule.RecordName, route.Rule.RecordType, route.Rule.TopN
		routeRows := FilterRowsForCloudflareRecordType(route.Rows, cfg.RecordType)
		pushTarget(cfg, routeRows, "route", cloudflareTargetLabel("分流目标", cloudflareRouteLabel(route.Rule)), &route)
	}

	data := map[string]any{
		"failed_targets":  failureCount,
		"ignored_entries": dedupeStrings(ignored),
		"records_after":   recordsAfter,
		"routing_enabled": true,
		"skipped_targets": skippedCount,
		"success_targets": successCount,
		"summary":         dnspush.SummaryMap(totalSummary),
		"targets":         targets,
		"upload_count":    totalUpload,
	}
	switch {
	case successCount == 0 && failureCount > 0:
		code := dnsProviderErrorCode(provider.Provider, firstErr)
		return NewCommandResult(code, data, fmt.Sprintf("%s 分流推送失败：没有目标完成。", label), false, nil, dedupeStrings(warnings))
	case successCount == 0:
		return NewCommandResult("DNS_INPUT_EMPTY", data, fmt.Sprintf("%s 分流推送已跳过：没有目标可写。", label), false, nil, dedupeStrings(warnings))
	case failureCount > 0:
		return NewCommandResult("DNS_PUSH_PARTIAL", data, fmt.Sprintf("%s 分流推送部分完成：成功 %d 个目标，失败 %d 个，跳过 %d 个。", label, successCount, failureCount, skippedCount), true, nil, dedupeStrings(warnings))
	default:
		return NewCommandResult("DNS_PUSH_COMPLETED", data, fmt.Sprintf("%s 分流推送完成：成功 %d 个目标，跳过 %d 个，共推送 %d 条。", label, successCount, skippedCount, totalUpload), true, nil, dedupeStrings(warnings))
	}
}

func routeWarningsFromRoute(route *UploadCloudflareRouteSelection) []string {
	if route == nil {
		return nil
	}
	return route.Warnings
}

// dnsProviderHasPrimaryTarget reports whether the provider has a main record
// independent of routing. DNSHE can address its apex through a subdomain label
// even when the user leaves record_name empty.
func dnsProviderHasPrimaryTarget(provider DNSProviderConfig) bool {
	if strings.TrimSpace(provider.RecordName) != "" {
		return true
	}
	return provider.Provider == DNSProviderDNSHE && (strings.TrimSpace(provider.Subdomain) != "" || provider.SubdomainID > 0)
}

func dnsProviderPrimaryTargetLabel(provider DNSProviderConfig) string {
	if strings.TrimSpace(provider.RecordName) != "" {
		return provider.RecordName
	}
	if strings.TrimSpace(provider.Subdomain) != "" {
		return fmt.Sprintf("子域名 %s", provider.Subdomain)
	}
	if provider.SubdomainID > 0 {
		return fmt.Sprintf("子域 ID %d", provider.SubdomainID)
	}
	return ""
}

func (s *Service) pushDNSHERows(ctx context.Context, provider DNSProviderConfig, rows []probecore.ProbeRow) (dnspush.PushResult, error) {
	result, err := dnshecore.Push(ctx, s.newDNSHEClient(provider), dnsheDNSConfig(provider), probeRowsIPList(rows))
	converted := dnspush.PushResult{
		HasInputIPs:    result.HasInputIPs,
		IgnoredEntries: result.IgnoredEntries,
		Summary:        result.Summary,
		Warnings:       result.Warnings,
	}
	for _, record := range result.RecordsAfter {
		converted.RecordsAfter = append(converted.RecordsAfter, dnspush.Record{
			Attrs:   map[string]string{"provider_record_id": record.ProviderRecordID},
			Content: record.Content,
			ID:      record.ID,
			Name:    record.Name,
			TTL:     record.TTL,
			Type:    record.Type,
		})
	}
	return converted, err
}

func (s *Service) pushAliyunRows(ctx context.Context, provider DNSProviderConfig, rows []probecore.ProbeRow) (dnspush.PushResult, error) {
	return alidnscore.Push(ctx, s.newAliyunClient(provider), aliyunDNSConfig(provider), probeRowsIPList(rows))
}

func (s *Service) pushDNSPodRows(ctx context.Context, provider DNSProviderConfig, rows []probecore.ProbeRow) (dnspush.PushResult, error) {
	return dnspodcore.Push(ctx, s.newDNSPodClient(provider), dnspodDNSConfig(provider), probeRowsIPList(rows))
}
