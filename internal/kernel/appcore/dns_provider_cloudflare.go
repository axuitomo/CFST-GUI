package appcore

import (
	"context"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/dns"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/cloudflarecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
)

var cloudflareDNSProviderModule = dnsProviderModule{
	aliases:       []string{DNSProviderCloudflare},
	configProblem: cloudflareConfigProblem,
	defaultTTL:    dns.DefaultCloudflareTTL,
	kind:          DNSProviderCloudflare,
	label:         "Cloudflare",
	listRecords: func(s *Service, ctx context.Context, provider DNSProviderConfig, target dnsReadTarget, recordType string) ([]dnsRecordSnapshot, string, error) {
		return s.listCloudflareDNSProviderRecords(ctx, provider, target, recordType)
	},
	normalizeTTL: cloudflareNormalizeTTL,
	push: func(s *Service, ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow) CommandResult {
		return s.pushCloudflareDNSRecordsContext(ctx, map[string]any{
			"config":  dnsProviderSnapshot(snapshot, provider),
			"results": rows,
		})
	},
	selfTest: func(s *Service, provider DNSProviderConfig, testName string, _ *dnshecore.Subdomain) dnsSelfTestAdapter {
		return &cloudflareSelfTestAdapter{
			client: s.newCloudflareClient(provider.APIToken),
			cfg: cloudflarecore.Config{
				AllowCNAME: provider.AllowCNAME,
				APIToken:   provider.APIToken,
				Comment:    dnsSelfTestComment,
				Proxied:    false,
				RecordName: testName,
				TTL:        cloudflareNormalizeTTL(provider.TTL),
				ZoneID:     provider.ZoneID,
			},
			name: testName,
		}
	},
	ttlPair: cloudflareTTLPair,
}

func cloudflareConfigProblem(provider DNSProviderConfig) string {
	if provider.APIToken == "" || dns.IsMaskedSecret(provider.APIToken) {
		return "缺少完整 Cloudflare API Token"
	}
	if provider.ZoneID == "" {
		return "缺少 Cloudflare Zone ID"
	}
	if provider.RecordName == "" && !provider.Routing.Enabled {
		return "缺少 Cloudflare DNS 记录名称"
	}
	return ""
}

func (s *Service) listCloudflareDNSProviderRecords(ctx context.Context, provider DNSProviderConfig, target dnsReadTarget, recordType string) ([]dnsRecordSnapshot, string, error) {
	cfg := cloudflarecore.Config{
		APIToken:   provider.APIToken,
		RecordName: provider.RecordName,
		RecordType: recordType,
		TTL:        provider.TTL,
		ZoneID:     provider.ZoneID,
	}
	// One exact name can be pushed down to the API; a label or a rule list needs
	// the zone listing so the local filter can match every target.
	options := cloudflarecore.ListOptions{Type: recordType}
	if len(target.Names) == 1 && len(target.Labels) == 0 {
		options.Name = target.Names[0]
	}
	records, err := s.newCloudflareClient(provider.APIToken).ListRecordsWithOptions(ctx, cfg, options)
	if err != nil {
		return nil, "", err
	}
	out := make([]dnsRecordSnapshot, 0, len(records))
	zoneName := ""
	for _, record := range records {
		if zoneName == "" {
			zoneName = normalizeDNSRecordName(record.ZoneName)
		}
		out = append(out, dnsRecordSnapshot{
			Comment:    record.Comment,
			Content:    record.Content,
			CreatedOn:  record.CreatedOn,
			ID:         record.ID,
			ModifiedOn: record.ModifiedOn,
			Name:       record.Name,
			Proxied:    record.Proxied,
			TTL:        record.TTL,
			Type:       record.Type,
		})
	}
	return filterDNSRecordSnapshots(out, target, recordType), zoneName, nil
}

func cloudflareNormalizeTTL(ttl int) int {
	if !dns.IsAllowedCloudflareTTL(ttl) {
		return dns.DefaultCloudflareTTL
	}
	return ttl
}

func cloudflareTTLPair(ttl int) (int, int) {
	base := cloudflareNormalizeTTL(ttl)
	if base == 600 {
		return base, 300
	}
	return base, 600
}
