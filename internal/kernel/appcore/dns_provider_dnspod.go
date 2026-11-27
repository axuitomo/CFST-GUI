package appcore

import (
	"context"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspodcore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
)

var dnspodDNSProviderModule = dnsProviderModule{
	aliases:       []string{DNSProviderDNSPod, "dndps", "tencent", "tencentcloud"},
	configProblem: dnspodConfigProblem,
	defaultTTL:    DefaultDNSPodTTL,
	kind:          DNSProviderDNSPod,
	label:         "DNSPod",
	listRecords: func(s *Service, ctx context.Context, provider DNSProviderConfig, target dnsReadTarget, recordType string) ([]dnsRecordSnapshot, string, error) {
		return s.listDNSPodDNSProviderRecords(ctx, provider, target, recordType)
	},
	normalizeTTL: dnspodNormalizeTTL,
	push: func(s *Service, ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow) CommandResult {
		return s.pushDNSPodDNSRecordsContext(ctx, provider, snapshot, rows)
	},
	selfTest: func(s *Service, provider DNSProviderConfig, testName string, _ *dnshecore.Subdomain) dnsSelfTestAdapter {
		return &dnspodSelfTestAdapter{
			allowCNAME: provider.AllowCNAME,
			client:     s.newDNSPodClient(provider),
			name:       testName,
		}
	},
	ttlPair: dnspodTTLPair,
}

func dnspodConfigProblem(provider DNSProviderConfig) string {
	if provider.APIKey == "" || IsMaskedSecret(provider.APIKey) {
		return "缺少完整 DNSPod SecretId"
	}
	if provider.APISecret == "" || IsMaskedSecret(provider.APISecret) {
		return "缺少完整 DNSPod SecretKey"
	}
	if provider.RecordName == "" && !hasUsableRoutingTarget(provider.Routing) {
		return "缺少 DNSPod 记录名称（完整域名）"
	}
	return ""
}

func dnspodDNSConfig(provider DNSProviderConfig) dnspodcore.Config {
	return dnspodcore.Config{
		AllowCNAME: provider.AllowCNAME,
		RecordName: provider.RecordName,
		RecordType: provider.RecordType,
		SecretID:   provider.APIKey,
		SecretKey:  provider.APISecret,
		TTL:        provider.TTL,
	}
}

func (s *Service) listDNSPodDNSProviderRecords(ctx context.Context, provider DNSProviderConfig, target dnsReadTarget, recordType string) ([]dnsRecordSnapshot, string, error) {
	client := s.newDNSPodClient(provider)
	zone, err := client.ResolveZone(ctx, dnsZoneLookupName(provider, target))
	if err != nil {
		return nil, "", err
	}
	records, err := client.ListRecords(ctx, zone)
	if err != nil {
		return nil, "", err
	}
	return filterDNSRecordSnapshots(dnsSnapshotsFromPushRecords(records), target, recordType), normalizeDNSRecordName(zone.Name), nil
}

func dnspodNormalizeTTL(ttl int) int {
	// DNSPod 接受 60 起的秒级 TTL；更小的值在控制台也存不住。
	if ttl < 60 {
		return DefaultDNSPodTTL
	}
	return ttl
}

func dnspodTTLPair(ttl int) (int, int) {
	base := dnspodNormalizeTTL(ttl)
	// 冷启动时配置里可能是 0/1，自检必须先抬到平台下限才能写成功。
	if base < 600 {
		base = 600
	}
	if base <= 600 {
		return base, 1200
	}
	return base, 600
}
