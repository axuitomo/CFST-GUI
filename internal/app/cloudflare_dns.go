package app

import (
	"context"
	"fmt"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/cloudflarecore"
)

const (
	cloudflareRecordTypeA    = cloudflarecore.RecordTypeA
	cloudflareRecordTypeAAAA = cloudflarecore.RecordTypeAAAA
	defaultCloudflareTTL     = cloudflarecore.DefaultTTL
)

var cloudflareAPIBaseURL = cloudflarecore.APIBaseURL

type cloudflareDNSConfig = cloudflarecore.Config
type CloudflareDNSRecord = cloudflarecore.Record
type cloudflareDNSPushSummary = cloudflarecore.PushSummary
type cloudflareDNSPushIPGroups = cloudflarecore.PushIPGroups

func (a *App) ListCloudflareDNSRecords(payload map[string]any) DesktopCommandResult {
	cfg, warnings, err := cloudflareDNSConfigFromPayload(payload)
	if err != nil {
		return desktopCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	records, err := newCloudflareDNSClient(cfg.APIToken).ListRecords(ctx, cfg)
	if err != nil {
		return desktopCommandResult("DNS_LIST_FAILED", nil, err.Error(), false, nil, warnings)
	}

	return desktopCommandResult("DNS_RECORDS_LISTED", map[string]any{
		"count":   len(records),
		"records": records,
	}, fmt.Sprintf("已读取 Cloudflare 中匹配 %s 的 A/AAAA 记录 %d 条。", cfg.RecordName, len(records)), true, nil, warnings)
}

func (a *App) PushCloudflareDNSRecords(payload map[string]any) DesktopCommandResult {
	cfg, warnings, err := cloudflareDNSConfigFromPayload(payload)
	if err != nil {
		return desktopCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}

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
		rows = filterRowsForCloudflareRecordType(selection.CloudflareRows, cfg.RecordType)
		if len(rows) == 0 {
			return desktopCommandResult("DNS_INPUT_EMPTY", map[string]any{
				"ignored_entries": []string{},
				"records_after":   []CloudflareDNSRecord{},
				"summary":         cloudflareSummaryMap(cloudflareDNSPushSummary{}),
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
		}, "没有可推送的有效 IP。", false, nil, warnings)
	}

	return desktopCommandResult("DNS_PUSH_COMPLETED", map[string]any{
		"ignored_entries": result.IgnoredEntries,
		"records_after":   result.RecordsAfter,
		"summary":         cloudflareSummaryMap(result.Summary),
	}, fmt.Sprintf("Cloudflare DNS 覆盖推送完成：创建 %d、更新 %d、删除 %d、忽略 %d。", result.Summary.Created, result.Summary.Updated, result.Summary.Deleted, result.Summary.Ignored), true, nil, dedupeStrings(warnings))
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
