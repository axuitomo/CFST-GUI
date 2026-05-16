package mobileapi

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

func (s *Service) ListCloudflareDNSRecords(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	cfg, warnings, err := cloudflareDNSConfigFromPayload(payload)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	records, err := newCloudflareDNSClient(cfg.APIToken).ListRecords(ctx, cfg)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_LIST_FAILED", nil, err.Error(), false, nil, warnings))
	}
	return encodeCommand(commandResultFor("DNS_RECORDS_LISTED", map[string]any{
		"count":   len(records),
		"records": records,
	}, fmt.Sprintf("已读取 Cloudflare 中匹配 %s 的 A/AAAA 记录 %d 条。", cfg.RecordName, len(records)), true, nil, warnings))
}

func (s *Service) PushCloudflareDNSRecords(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	cfg, warnings, err := cloudflareDNSConfigFromPayload(payload)
	if err != nil {
		return encodeCommand(commandResultFor("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings))
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
