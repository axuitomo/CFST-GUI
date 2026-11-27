package appcore

import (
	"context"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/dns"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/alidnscore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
)

var aliyunDNSProviderModule = dnsProviderModule{
	aliases:       []string{DNSProviderAliyun, "alidns", "ali"},
	configProblem: aliyunConfigProblem,
	defaultTTL:    DefaultAliyunTTL,
	kind:          DNSProviderAliyun,
	label:         "阿里云 DNS",
	listRecords: func(s *Service, ctx context.Context, provider DNSProviderConfig, target dnsReadTarget, recordType string) ([]dnsRecordSnapshot, string, error) {
		return s.listAliyunDNSProviderRecords(ctx, provider, target, recordType)
	},
	normalizeTTL: aliyunNormalizeTTL,
	push: func(s *Service, ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow) CommandResult {
		return s.pushAliyunDNSRecordsContext(ctx, provider, snapshot, rows)
	},
	selfTest: func(s *Service, provider DNSProviderConfig, testName string, _ *dnshecore.Subdomain) dnsSelfTestAdapter {
		return &aliyunSelfTestAdapter{
			allowCNAME: provider.AllowCNAME,
			client:     s.newAliyunClient(provider),
			name:       testName,
		}
	},
	ttlPair: aliyunTTLPair,
}

func aliyunConfigProblem(provider DNSProviderConfig) string {
	if provider.APIKey == "" || dns.IsMaskedSecret(provider.APIKey) {
		return "缺少完整阿里云 AccessKey ID"
	}
	if provider.APISecret == "" || dns.IsMaskedSecret(provider.APISecret) {
		return "缺少完整阿里云 AccessKey Secret"
	}
	if provider.RecordName == "" && !hasUsableRoutingTarget(provider.Routing) {
		return "缺少阿里云 DNS 记录名称（完整域名）"
	}
	return ""
}

func aliyunDNSConfig(provider DNSProviderConfig) alidnscore.Config {
	return alidnscore.Config{
		AccessKeyID:     provider.APIKey,
		AccessKeySecret: provider.APISecret,
		AllowCNAME:      provider.AllowCNAME,
		RecordName:      provider.RecordName,
		RecordType:      provider.RecordType,
		TTL:             provider.TTL,
	}
}

func (s *Service) listAliyunDNSProviderRecords(ctx context.Context, provider DNSProviderConfig, target dnsReadTarget, recordType string) ([]dnsRecordSnapshot, string, error) {
	client := s.newAliyunClient(provider)
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

func aliyunNormalizeTTL(ttl int) int {
	// 阿里云接受 60 起的秒级 TTL；更小的值在控制台也存不住。
	if ttl < 60 {
		return DefaultAliyunTTL
	}
	return ttl
}

func aliyunTTLPair(ttl int) (int, int) {
	base := aliyunNormalizeTTL(ttl)
	// 冷启动时配置里可能是 0/1，自检必须先抬到平台下限才能写成功。
	if base < 600 {
		base = 600
	}
	if base <= 600 {
		return base, 1200
	}
	return base, 600
}
