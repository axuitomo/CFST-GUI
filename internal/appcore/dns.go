package appcore

import (
	"context"
	"github.com/axuitomo/CFST-GUI/internal/cloudflarecore"
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

func CloudflareDNSConfigFromPayload(payload map[string]any) (CloudflareDNSConfig, []string, error) {
	return cloudflarecore.ParseConfigFromPayload(payload)
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

func NewCloudflareDNSClient(token string) *CloudflareDNSClient {
	return NewCloudflareDNSClientWithBaseURL(token, CloudflareAPIBaseURL)
}

func NewCloudflareDNSClientWithBaseURL(token string, baseURL string) *CloudflareDNSClient {
	return cloudflarecore.NewClientWithOptions(cloudflarecore.ClientOptions{
		BaseURL: baseURL,
		Token:   token,
	})
}

func IsMaskedSecret(value string) bool {
	return cloudflarecore.IsMaskedSecret(value)
}

func PushCloudflareDNSRecords(ctx context.Context, client *CloudflareDNSClient, cfg CloudflareDNSConfig, ipsRaw string) (cloudflarecore.PushResult, error) {
	return cloudflarecore.PushRecords(ctx, client, cfg, ipsRaw)
}

func CloudflareDNSErrorCode(err error) string {
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
