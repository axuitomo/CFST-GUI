package appcore

import (
	"context"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/dnspush"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

var dnsheDNSProviderModule = dnsProviderModule{
	aliases:       []string{DNSProviderDNSHE},
	configProblem: dnsheConfigProblem,
	defaultTTL:    DefaultDNSHETTL,
	kind:          DNSProviderDNSHE,
	label:         "DNSHE",
	listRecords: func(s *Service, ctx context.Context, provider DNSProviderConfig, target dnsReadTarget, recordType string) ([]dnsRecordSnapshot, string, error) {
		return s.listDNSHEDNSProviderRecords(ctx, provider, target, recordType)
	},
	normalizeTTL: dnsheNormalizeTTL,
	push: func(s *Service, ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow) CommandResult {
		return s.pushDNSHERecordsContext(ctx, provider, snapshot, rows)
	},
	selfTest: func(s *Service, provider DNSProviderConfig, testName string, dnsheSubdomain *dnshecore.Subdomain) dnsSelfTestAdapter {
		adapter := &dnsheSelfTestAdapter{
			allowCNAME:    provider.AllowCNAME,
			client:        s.newDNSHESelfTestClient(provider),
			name:          testName,
			sourceName:    provider.RecordName,
			subdomainName: provider.Subdomain,
			subdomainID:   provider.SubdomainID,
		}
		if dnsheSubdomain != nil {
			adapter.subdomain = *dnsheSubdomain
			adapter.ready = true
		}
		return adapter
	},
	ttlPair: dnsheTTLPair,
}

func dnsheConfigProblem(provider DNSProviderConfig) string {
	if provider.APIKey == "" || IsMaskedSecret(provider.APIKey) {
		return "缺少完整 DNSHE API Key"
	}
	if provider.APISecret == "" || IsMaskedSecret(provider.APISecret) {
		return "缺少完整 DNSHE API Secret"
	}
	if provider.RecordName == "" && strings.TrimSpace(provider.Subdomain) == "" && provider.SubdomainID <= 0 && !hasUsableRoutingTarget(provider.Routing) {
		return "缺少 DNSHE 记录名称（完整域名或子域名）"
	}
	return ""
}

func dnsheDNSConfig(provider DNSProviderConfig) dnshecore.Config {
	return dnshecore.Config{
		APIKey:      provider.APIKey,
		APISecret:   provider.APISecret,
		AllowCNAME:  provider.AllowCNAME,
		RecordName:  provider.RecordName,
		RecordType:  provider.RecordType,
		Subdomain:   provider.Subdomain,
		SubdomainID: provider.SubdomainID,
		TTL:         provider.TTL,
	}
}

func (s *Service) listDNSHEDNSProviderRecords(ctx context.Context, provider DNSProviderConfig, target dnsReadTarget, recordType string) ([]dnsRecordSnapshot, string, error) {
	client := s.newDNSHEClient(provider)
	subdomain, targetName, err := resolveDNSHETarget(ctx, client, provider)
	if err != nil {
		return nil, "", err
	}
	// An empty target is the page's whole-zone scope; the configured target and a
	// bare subdomain label both map onto the resolved record name.
	resolved := normalizeDNSRecordName(targetName)
	if len(target.Names) == 1 && target.Names[0] == normalizeDNSRecordName(provider.RecordName) {
		target = dnsReadTarget{Names: []string{resolved}, Scope: target.Scope}
	} else if len(target.Labels) == 1 && target.Labels[0] == normalizeDNSRecordName(provider.Subdomain) {
		target = dnsReadTarget{Names: []string{resolved}, Scope: target.Scope}
	}
	records, err := client.ListRecords(ctx, subdomain.ID)
	if err != nil {
		return nil, "", err
	}
	snapshots := make([]dnsRecordSnapshot, 0, len(records))
	for _, record := range records {
		snapshots = append(snapshots, dnsRecordSnapshot{
			Content: record.Content,
			ID:      record.ID,
			Name:    dnspush.AbsoluteName(subdomain.FullDomain, record.Name),
			TTL:     record.TTL,
			Type:    record.Type,
		})
	}
	return filterDNSRecordSnapshots(snapshots, target, recordType), normalizeDNSRecordName(subdomain.FullDomain), nil
}

func dnsheNormalizeTTL(ttl int) int {
	if ttl <= 0 {
		return DefaultDNSHETTL
	}
	return ttl
}

func dnsheTTLPair(ttl int) (int, int) {
	base := dnsheNormalizeTTL(ttl)
	// DNSHE rejects TTLs below 600, so a low configured value would fail the
	// upload before it could test anything.
	if base < 600 {
		base = 600
	}
	if base <= 600 {
		return base, 1200
	}
	return base, 600
}
