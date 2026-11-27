package appcore

import (
	"context"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/alidnscore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/cloudflarecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspodcore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspush"
)

func (s *Service) newDNSHESelfTestClient(provider DNSProviderConfig) *dnshecore.Client {
	client := s.newDNSHEClient(provider)
	// DNSHE defaults to 30 requests/min; a full staged self-test is longer than
	// that, so space calls out instead of failing midway and leaving residue.
	client.SetRequestInterval(2 * time.Second)
	return client
}

func (s *Service) dnsSelfTestAdapter(provider DNSProviderConfig, testName string, dnsheSubdomain *dnshecore.Subdomain) dnsSelfTestAdapter {
	return dnsProviderModuleOrDefault(provider.Provider).selfTest(s, provider, testName, dnsheSubdomain)
}

type cloudflareSelfTestAdapter struct {
	client *cloudflarecore.Client
	cfg    cloudflarecore.Config
	name   string
}

func (a *cloudflareSelfTestAdapter) Read(ctx context.Context) (dnsSelfTestReadResult, error) {
	all, err := a.client.ListRecordsWithOptions(ctx, a.cfg, cloudflarecore.ListOptions{Name: a.name})
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	aRecords, err := a.client.ListRecordsWithOptions(ctx, a.cfg, cloudflarecore.ListOptions{Name: a.name, Type: dnspush.RecordTypeA})
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	aaaaRecords, err := a.client.ListRecordsWithOptions(ctx, a.cfg, cloudflarecore.ListOptions{Name: a.name, Type: dnspush.RecordTypeAAAA})
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	return dnsSelfTestReadResult{
		All:  cloudflareSelfTestRecords(all),
		A:    cloudflareSelfTestRecords(aRecords),
		AAAA: cloudflareSelfTestRecords(aaaaRecords),
	}, nil
}

func (a *cloudflareSelfTestAdapter) Push(ctx context.Context, ips string, ttl int) (dnspush.Summary, []string, error) {
	cfg := a.cfg
	cfg.TTL = ttl
	result, err := cloudflarecore.PushRecords(ctx, a.client, cfg, ips)
	return result.Summary, result.Warnings, err
}

func (a *cloudflareSelfTestAdapter) CreateCNAME(ctx context.Context, target string, ttl int) (dnsSelfTestRecord, error) {
	created, err := a.client.CreateRecord(ctx, a.cfg, cloudflarecore.Record{
		Comment: dnsSelfTestComment,
		Content: target,
		Name:    a.name,
		Proxied: false,
		TTL:     ttl,
		Type:    dnspush.RecordTypeCNAME,
	})
	if err != nil {
		return dnsSelfTestRecord{}, err
	}
	return dnsSelfTestRecord{
		Content: created.Content,
		ID:      created.ID,
		Name:    created.Name,
		TTL:     created.TTL,
		Type:    created.Type,
	}, nil
}

func (a *cloudflareSelfTestAdapter) Delete(ctx context.Context, record dnsSelfTestRecord) error {
	return a.client.DeleteRecord(ctx, a.cfg, record.ID)
}

func cloudflareSelfTestRecords(records []cloudflarecore.Record) []dnsSelfTestRecord {
	result := make([]dnsSelfTestRecord, 0, len(records))
	for _, record := range records {
		result = append(result, dnsSelfTestRecord{
			Content: record.Content,
			ID:      record.ID,
			Name:    record.Name,
			TTL:     record.TTL,
			Type:    record.Type,
		})
	}
	return result
}

type aliyunSelfTestAdapter struct {
	allowCNAME bool
	client     *alidnscore.Client
	name       string
	zone       alidnscore.Zone
	ready      bool
}

func (a *aliyunSelfTestAdapter) ensureZone(ctx context.Context) error {
	if a.ready {
		return nil
	}
	zone, err := a.client.ResolveZone(ctx, a.name)
	if err != nil {
		return err
	}
	a.zone = zone
	a.ready = true
	return nil
}

func (a *aliyunSelfTestAdapter) Read(ctx context.Context) (dnsSelfTestReadResult, error) {
	if err := a.ensureZone(ctx); err != nil {
		return dnsSelfTestReadResult{}, err
	}
	records, err := a.client.ListRecords(ctx, a.zone)
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	all := filterSelfTestRecords(records, a.name)
	return dnsSelfTestReadResult{
		All:  all,
		A:    recordsBySelfTestType(all, dnspush.RecordTypeA),
		AAAA: recordsBySelfTestType(all, dnspush.RecordTypeAAAA),
	}, nil
}

func (a *aliyunSelfTestAdapter) Push(ctx context.Context, ips string, ttl int) (dnspush.Summary, []string, error) {
	result, err := alidnscore.Push(ctx, a.client, alidnscore.Config{
		AllowCNAME: a.allowCNAME,
		RecordName: a.name,
		TTL:        ttl,
	}, ips)
	return result.Summary, result.Warnings, err
}

func (a *aliyunSelfTestAdapter) CreateCNAME(ctx context.Context, target string, ttl int) (dnsSelfTestRecord, error) {
	if err := a.ensureZone(ctx); err != nil {
		return dnsSelfTestRecord{}, err
	}
	created, err := a.client.CreateRecord(ctx, a.zone, dnspush.Record{
		Content: target,
		Name:    a.name,
		TTL:     ttl,
		Type:    dnspush.RecordTypeCNAME,
	})
	if err != nil {
		return dnsSelfTestRecord{}, err
	}
	return dnsSelfTestRecord{Content: created.Content, ID: created.ID, Name: created.Name, TTL: created.TTL, Type: created.Type}, nil
}

func (a *aliyunSelfTestAdapter) Delete(ctx context.Context, record dnsSelfTestRecord) error {
	return a.client.DeleteRecord(ctx, a.zone, dnspush.Record{
		Content: record.Content,
		ID:      record.ID,
		Name:    record.Name,
		TTL:     record.TTL,
		Type:    record.Type,
	})
}

type dnspodSelfTestAdapter struct {
	allowCNAME bool
	client     *dnspodcore.Client
	name       string
	zone       dnspodcore.Zone
	ready      bool
}

func (a *dnspodSelfTestAdapter) ensureZone(ctx context.Context) error {
	if a.ready {
		return nil
	}
	zone, err := a.client.ResolveZone(ctx, a.name)
	if err != nil {
		return err
	}
	a.zone = zone
	a.ready = true
	return nil
}

func (a *dnspodSelfTestAdapter) Read(ctx context.Context) (dnsSelfTestReadResult, error) {
	if err := a.ensureZone(ctx); err != nil {
		return dnsSelfTestReadResult{}, err
	}
	records, err := a.client.ListRecords(ctx, a.zone)
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	all := filterSelfTestRecords(records, a.name)
	return dnsSelfTestReadResult{
		All:  all,
		A:    recordsBySelfTestType(all, dnspush.RecordTypeA),
		AAAA: recordsBySelfTestType(all, dnspush.RecordTypeAAAA),
	}, nil
}

func (a *dnspodSelfTestAdapter) Push(ctx context.Context, ips string, ttl int) (dnspush.Summary, []string, error) {
	result, err := dnspodcore.Push(ctx, a.client, dnspodcore.Config{
		AllowCNAME: a.allowCNAME,
		RecordName: a.name,
		TTL:        ttl,
	}, ips)
	return result.Summary, result.Warnings, err
}

func (a *dnspodSelfTestAdapter) CreateCNAME(ctx context.Context, target string, ttl int) (dnsSelfTestRecord, error) {
	if err := a.ensureZone(ctx); err != nil {
		return dnsSelfTestRecord{}, err
	}
	created, err := a.client.CreateRecord(ctx, a.zone, dnspush.Record{
		Content: target,
		Name:    a.name,
		TTL:     ttl,
		Type:    dnspush.RecordTypeCNAME,
	})
	if err != nil {
		return dnsSelfTestRecord{}, err
	}
	return dnsSelfTestRecord{Content: created.Content, ID: created.ID, Name: created.Name, TTL: created.TTL, Type: created.Type}, nil
}

func (a *dnspodSelfTestAdapter) Delete(ctx context.Context, record dnsSelfTestRecord) error {
	return a.client.DeleteRecord(ctx, a.zone, dnspush.Record{
		Content: record.Content,
		ID:      record.ID,
		Name:    record.Name,
		TTL:     record.TTL,
		Type:    record.Type,
	})
}

type dnsheSelfTestAdapter struct {
	allowCNAME    bool
	client        *dnshecore.Client
	name          string
	sourceName    string
	subdomain     dnshecore.Subdomain
	subdomainID   int
	subdomainName string
	ready         bool
}

func (a *dnsheSelfTestAdapter) ensureSubdomain(ctx context.Context) error {
	if a.ready {
		return nil
	}
	var (
		subdomain dnshecore.Subdomain
		err       error
	)
	switch {
	case strings.TrimSpace(a.subdomainName) != "":
		subdomain, _, err = a.client.ResolveSubdomainTarget(ctx, a.subdomainName, a.sourceName)
	case a.subdomainID > 0:
		subdomain, err = a.client.GetSubdomain(ctx, a.subdomainID)
	default:
		subdomain, err = a.client.ResolveSubdomain(ctx, a.sourceName)
	}
	if err != nil {
		return err
	}
	a.subdomain = subdomain
	a.ready = true
	return nil
}

func (a *dnsheSelfTestAdapter) Read(ctx context.Context) (dnsSelfTestReadResult, error) {
	if err := a.ensureSubdomain(ctx); err != nil {
		return dnsSelfTestReadResult{}, err
	}
	records, err := a.client.ListRecords(ctx, a.subdomain.ID)
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	all := make([]dnsSelfTestRecord, 0, len(records))
	for _, record := range records {
		full := dnsheSelfTestFullName(a.subdomain.FullDomain, record.Name)
		if !strings.EqualFold(full, a.name) {
			continue
		}
		all = append(all, dnsSelfTestRecord{
			Content:          record.Content,
			ID:               record.ID,
			Name:             full,
			ProviderRecordID: record.ProviderRecordID,
			TTL:              record.TTL,
			Type:             record.Type,
		})
	}
	return dnsSelfTestReadResult{
		All:  all,
		A:    recordsBySelfTestType(all, dnspush.RecordTypeA),
		AAAA: recordsBySelfTestType(all, dnspush.RecordTypeAAAA),
	}, nil
}

func (a *dnsheSelfTestAdapter) Push(ctx context.Context, ips string, ttl int) (dnspush.Summary, []string, error) {
	if err := a.ensureSubdomain(ctx); err != nil {
		return dnspush.Summary{}, nil, err
	}
	result, err := dnshecore.PushResolved(ctx, a.client, dnshecore.Config{
		AllowCNAME: a.allowCNAME,
		RecordName: a.name,
		TTL:        ttl,
	}, a.subdomain, ips)
	return result.Summary, result.Warnings, err
}

func (a *dnsheSelfTestAdapter) CreateCNAME(ctx context.Context, target string, ttl int) (dnsSelfTestRecord, error) {
	if err := a.ensureSubdomain(ctx); err != nil {
		return dnsSelfTestRecord{}, err
	}
	created, err := a.client.CreateRecord(ctx, a.subdomain.ID, dnshecore.Record{
		Content: target,
		Name:    dnsheSelfTestRelativeName(a.subdomain.FullDomain, a.name),
		TTL:     ttl,
		Type:    dnspush.RecordTypeCNAME,
	})
	if err != nil {
		return dnsSelfTestRecord{}, err
	}
	return dnsSelfTestRecord{
		Content:          created.Content,
		ID:               created.ID,
		Name:             a.name,
		ProviderRecordID: created.ProviderRecordID,
		TTL:              created.TTL,
		Type:             created.Type,
	}, nil
}

func (a *dnsheSelfTestAdapter) Delete(ctx context.Context, record dnsSelfTestRecord) error {
	return a.client.DeleteRecord(ctx, dnshecore.Record{
		Content:          record.Content,
		ID:               record.ID,
		Name:             dnsheSelfTestRelativeName(a.subdomain.FullDomain, a.name),
		ProviderRecordID: record.ProviderRecordID,
		TTL:              record.TTL,
		Type:             record.Type,
	})
}

func filterSelfTestRecords(records []dnspush.Record, name string) []dnsSelfTestRecord {
	normalized := normalizeSelfTestName(name)
	result := make([]dnsSelfTestRecord, 0)
	for _, record := range records {
		if normalizeSelfTestName(record.Name) != normalized {
			continue
		}
		result = append(result, dnsSelfTestRecord{
			Content: record.Content,
			ID:      record.ID,
			Name:    record.Name,
			TTL:     record.TTL,
			Type:    record.Type,
		})
	}
	return result
}

func normalizeSelfTestName(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

func dnsheSelfTestRelativeName(subdomainFullDomain, recordName string) string {
	subdomain := normalizeSelfTestName(subdomainFullDomain)
	name := normalizeSelfTestName(recordName)
	if subdomain == "" || name == "" || name == subdomain {
		return "@"
	}
	if strings.HasSuffix(name, "."+subdomain) {
		if prefix := strings.TrimSuffix(name, "."+subdomain); prefix != "" {
			return prefix
		}
	}
	return "@"
}

func dnsheSelfTestFullName(subdomainFullDomain, name string) string {
	subdomain := normalizeSelfTestName(subdomainFullDomain)
	normalized := normalizeSelfTestName(name)
	switch {
	case normalized == "" || normalized == "@":
		return subdomain
	case normalized == subdomain || strings.HasSuffix(normalized, "."+subdomain):
		return normalized
	default:
		// DNSHE returns direct labels as bare names; a multi-label relative
		// name such as "cfst-selftest.www" must still be expanded.
		return normalized + "." + subdomain
	}
}
