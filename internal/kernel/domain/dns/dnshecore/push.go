package dnshecore

import (
	"context"
	"fmt"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspush"
)

// Push synchronizes the managed record name with ipsRaw through the shared
// provider-agnostic reconcile loop. It manages A and AAAA from the given pool;
// cfg.RecordType is not consulted, so callers must filter the address family.
func Push(ctx context.Context, client *Client, cfg Config, ipsRaw string) (PushResult, error) {
	if client == nil {
		client = NewClient(cfg.APIKey, cfg.APISecret)
	}
	ipGroups, ignored := dnspush.NormalizeIPs(ipsRaw)
	result := newPushResult(ignored)
	result.HasInputIPs = ipGroups.HasIPs()
	if !result.HasInputIPs {
		return result, nil
	}
	subdomain, recordName, err := resolveTarget(ctx, client, cfg)
	if err != nil {
		return result, &dnspush.OperationError{Operation: dnspush.OperationList, Err: err}
	}
	cfg.RecordName = recordName
	return pushResolvedWithGroups(ctx, client, cfg, subdomain, ipGroups, result)
}

// PushResolved is Push with an already resolved subdomain. The self-test uses
// it to avoid repeating GetSubdomain/ListSubdomains on every stage.
func PushResolved(ctx context.Context, client *Client, cfg Config, subdomain Subdomain, ipsRaw string) (PushResult, error) {
	if client == nil {
		client = NewClient(cfg.APIKey, cfg.APISecret)
	}
	ipGroups, ignored := dnspush.NormalizeIPs(ipsRaw)
	result := newPushResult(ignored)
	result.HasInputIPs = ipGroups.HasIPs()
	if !result.HasInputIPs {
		return result, nil
	}
	subdomain.FullDomain = normalizeDomain(subdomain.FullDomain)
	if subdomain.FullDomain == "" {
		return result, fmt.Errorf("DNSHE 子域 %d 缺少完整域名", subdomain.ID)
	}
	recordName := normalizeDomain(cfg.RecordName)
	if recordName != "" && recordName != subdomain.FullDomain && !strings.HasSuffix(recordName, "."+subdomain.FullDomain) {
		return result, fmt.Errorf("DNSHE 记录名 %s 不属于子域名 %s，已拒绝写入以避免覆盖其它子域", recordName, subdomain.FullDomain)
	}
	if recordName == "" {
		cfg.RecordName = subdomain.FullDomain
	}
	return pushResolvedWithGroups(ctx, client, cfg, subdomain, ipGroups, result)
}

func newPushResult(ignored []string) PushResult {
	return PushResult{
		HasInputIPs:    false,
		IgnoredEntries: ignored,
		RecordsAfter:   []Record{},
		Summary:        dnspush.Summary{Ignored: len(ignored)},
		Warnings:       []string{},
	}
}

func pushResolvedWithGroups(ctx context.Context, client *Client, cfg Config, subdomain Subdomain, ipGroups dnspush.IPGroups, result PushResult) (PushResult, error) {
	result.HasInputIPs = ipGroups.HasIPs()
	backend := &pushBackend{client: client, cfg: cfg, subdomain: subdomain}
	desired := make([]dnspush.Record, 0, len(ipGroups.A)+len(ipGroups.AAAA))
	for _, recordType := range []string{RecordTypeA, RecordTypeAAAA} {
		for _, content := range ipGroups.ForType(recordType) {
			desired = append(desired, dnspush.Record{
				Content: content,
				Name:    cfg.RecordName,
				TTL:     cfg.TTL,
				Type:    recordType,
			})
		}
	}
	reconciled, err := dnspush.ReconcileWithOptions(ctx, backend, desired, dnspush.ReconcileOptions{AllowCNAME: cfg.AllowCNAME})
	result.Summary.Created = reconciled.Summary.Created
	result.Summary.Deleted = reconciled.Summary.Deleted
	result.Summary.Updated = reconciled.Summary.Updated
	result.Warnings = append(result.Warnings, reconciled.Warnings...)
	if err != nil {
		return result, err
	}
	recordsAfter, listErr := client.ListRecords(ctx, subdomain.ID)
	if listErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("推送完成但刷新 DNS 记录失败：%v", listErr))
	} else {
		result.RecordsAfter = recordsAfter
	}
	return result, nil
}

func resolveTarget(ctx context.Context, client *Client, cfg Config) (Subdomain, string, error) {
	var (
		subdomain  Subdomain
		recordName string
		err        error
	)
	switch {
	case strings.TrimSpace(cfg.Subdomain) != "":
		subdomain, recordName, err = client.ResolveSubdomainTarget(ctx, cfg.Subdomain, cfg.RecordName)
	case cfg.SubdomainID > 0:
		subdomain, err = client.GetSubdomain(ctx, cfg.SubdomainID)
		recordName = cfg.RecordName
	default:
		subdomain, err = client.ResolveSubdomain(ctx, cfg.RecordName)
		recordName = cfg.RecordName
	}
	if err != nil {
		return Subdomain{}, "", err
	}
	subdomain.FullDomain = normalizeDomain(subdomain.FullDomain)
	if subdomain.FullDomain == "" {
		return Subdomain{}, "", fmt.Errorf("DNSHE 子域 %d 缺少完整域名", subdomain.ID)
	}
	recordName = normalizeDomain(recordName)
	if recordName == "" {
		recordName = subdomain.FullDomain
	}
	if recordName != subdomain.FullDomain && !strings.HasSuffix(recordName, "."+subdomain.FullDomain) {
		return Subdomain{}, "", fmt.Errorf("DNSHE 记录名 %s 不属于子域名 %s，已拒绝写入以避免覆盖其它子域", recordName, subdomain.FullDomain)
	}
	return subdomain, recordName, nil
}

type pushBackend struct {
	client    *Client
	cfg       Config
	subdomain Subdomain
}

func (b *pushBackend) ListRecords(ctx context.Context) ([]dnspush.Record, error) {
	records, err := b.client.ListRecords(ctx, b.subdomain.ID)
	if err != nil {
		return nil, err
	}
	name := normalizeDomain(b.cfg.RecordName)
	converted := make([]dnspush.Record, 0, len(records))
	for _, record := range records {
		full := recordFullName(b.subdomain.FullDomain, record.Name)
		if full != name {
			continue
		}
		record.Name = full
		converted = append(converted, toPushRecord(record))
	}
	return converted, nil
}

func (b *pushBackend) CreateRecord(ctx context.Context, record dnspush.Record) (dnspush.Record, error) {
	created, err := b.client.CreateRecord(ctx, b.subdomain.ID, fromPushRecord(b.subdomain, record))
	if err != nil {
		return record, err
	}
	created.Name = record.Name
	return toPushRecord(created), nil
}

func (b *pushBackend) UpdateRecord(ctx context.Context, record dnspush.Record) (dnspush.Record, error) {
	updated, err := b.client.UpdateRecord(ctx, fromPushRecord(b.subdomain, record))
	if err != nil {
		return record, err
	}
	updated.Name = record.Name
	return toPushRecord(updated), nil
}

func (b *pushBackend) DeleteRecord(ctx context.Context, record dnspush.Record) error {
	return b.client.DeleteRecord(ctx, fromPushRecord(b.subdomain, record))
}

func (b *pushBackend) EquivalentRecord(existing, desired dnspush.Record) bool {
	return strings.EqualFold(strings.TrimSpace(existing.Type), strings.TrimSpace(desired.Type)) &&
		normalizeDomain(existing.Name) == normalizeDomain(desired.Name) &&
		strings.TrimSpace(existing.Content) == strings.TrimSpace(desired.Content) &&
		existing.TTL == desired.TTL
}

func toPushRecord(record Record) dnspush.Record {
	return dnspush.Record{
		Attrs:   map[string]string{"provider_record_id": record.ProviderRecordID},
		Content: record.Content,
		ID:      record.ID,
		Name:    record.Name,
		TTL:     record.TTL,
		Type:    record.Type,
	}
}

func fromPushRecord(subdomain Subdomain, record dnspush.Record) Record {
	return Record{
		Content:          record.Content,
		ID:               record.ID,
		Name:             relativeRecordName(subdomain.FullDomain, record.Name),
		ProviderRecordID: record.Attrs["provider_record_id"],
		TTL:              record.TTL,
		Type:             record.Type,
	}
}

// relativeRecordName converts a full record name into the value DNSHE expects:
// the subdomain itself is addressed as "@", child records by their prefix.
func relativeRecordName(subdomainFullDomain, recordName string) string {
	subdomainFullDomain = normalizeDomain(subdomainFullDomain)
	recordName = normalizeDomain(recordName)
	if subdomainFullDomain == "" || recordName == "" || recordName == subdomainFullDomain {
		return "@"
	}
	if strings.HasSuffix(recordName, "."+subdomainFullDomain) {
		prefix := strings.TrimSuffix(recordName, "."+subdomainFullDomain)
		if prefix != "" {
			return prefix
		}
	}
	return "@"
}

// recordFullName expands the record name DNSHE returns ("@", a bare label or a
// full domain) into the absolute record name.
func recordFullName(subdomainFullDomain, name string) string {
	subdomainFullDomain = normalizeDomain(subdomainFullDomain)
	normalized := normalizeDomain(name)
	switch {
	case normalized == "" || normalized == "@":
		return subdomainFullDomain
	case !strings.Contains(normalized, "."):
		return normalized + "." + subdomainFullDomain
	default:
		return normalized
	}
}

func normalizeDomain(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}
