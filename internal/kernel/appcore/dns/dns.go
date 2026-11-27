package dns

import (
	"context"
	"errors"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/alidnscore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/cloudflarecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspodcore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspush"
)

type CloudflareDNSConfig = cloudflarecore.Config
type CloudflareDNSRecord = cloudflarecore.Record
type CloudflareDNSPushSummary = cloudflarecore.PushSummary
type CloudflareDNSPushIPGroups = cloudflarecore.PushIPGroups
type CloudflareDNSClient = cloudflarecore.Client

const (
	CloudflareRecordTypeA    = cloudflarecore.RecordTypeA
	CloudflareRecordTypeAAAA = cloudflarecore.RecordTypeAAAA
	DefaultCloudflareTTL     = cloudflarecore.DefaultTTL
)

var CloudflareAPIBaseURL = cloudflarecore.APIBaseURL

var DNSHEAPIBaseURL = dnshecore.APIBaseURL

var AliyunAPIBaseURL = alidnscore.APIBaseURL

var DNSPodAPIBaseURL = dnspodcore.APIBaseURL

func CloudflareDNSConfigFromPayload(payload map[string]any) (CloudflareDNSConfig, []string, error) {
	return cloudflarecore.ParseConfigFromPayload(payload)
}

func CloudflareDNSListConfigFromPayload(payload map[string]any) (CloudflareDNSConfig, []string, error) {
	return cloudflarecore.ParseListConfigFromPayload(payload)
}

func CloudflareSummaryMap(summary CloudflareDNSPushSummary) map[string]any {
	return cloudflarecore.SummaryMap(summary)
}

func NormalizeDNSPushIPs(raw string) (CloudflareDNSPushIPGroups, []string) {
	return cloudflarecore.NormalizePushIPs(raw)
}

func IsAllowedCloudflareTTL(ttl int) bool {
	return cloudflarecore.IsAllowedTTL(ttl)
}

func IsMaskedSecret(value string) bool {
	return cloudflarecore.IsMaskedSecret(value)
}

func PushCloudflareDNSRecords(ctx context.Context, client *CloudflareDNSClient, cfg CloudflareDNSConfig, ipsRaw string) (cloudflarecore.PushResult, error) {
	return cloudflarecore.PushRecords(ctx, client, cfg, ipsRaw)
}

func CloudflareDNSErrorCode(err error) string {
	if errors.Is(err, dnspush.ErrCNAMEProtected) {
		return "DNS_CNAME_PROTECTED"
	}
	switch cloudflarecore.OperationFromError(err) {
	case cloudflarecore.OperationUpdate:
		return "DNS_UPDATE_FAILED"
	case cloudflarecore.OperationCreate:
		return "DNS_CREATE_FAILED"
	case cloudflarecore.OperationDelete:
		return "DNS_DELETE_FAILED"
	default:
		return "DNS_LIST_FAILED"
	}
}
