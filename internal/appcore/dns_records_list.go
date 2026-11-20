package appcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/cloudflarecore"
	"github.com/axuitomo/CFST-GUI/internal/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/dnspush"
)

// dnsRecordSnapshot is the read-only record shape the DNS page renders. It
// mirrors the Cloudflare listing fields so legacy callers keep working.
type dnsRecordSnapshot struct {
	Comment    string `json:"comment"`
	Content    string `json:"content"`
	CreatedOn  string `json:"created_on,omitempty"`
	ID         string `json:"id"`
	ModifiedOn string `json:"modified_on,omitempty"`
	Name       string `json:"name"`
	Proxied    bool   `json:"proxied"`
	TTL        int    `json:"ttl"`
	Type       string `json:"type"`
}

// invokeDNSRecordsList serves the DNS page read. Without provider_id it keeps
// the legacy Cloudflare-only path; with provider_id it reads from any saved
// platform entry so the page can switch targets without a second command.
func (s *Service) invokeDNSRecordsList(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	providerID := strings.TrimSpace(stringValue(firstNonNil(payload["provider_id"], payload["providerId"]), ""))
	if providerID == "" {
		return s.invokeCloudflareList(payloadJSON)
	}

	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	provider, ok := selectDNSProvider(DNSProvidersFromSnapshot(config), providerID)
	if !ok {
		return NewCommandResult("DNS_PROVIDER_NOT_FOUND", nil, "没有找到该 DNS 平台，请先在设置页保存后再读取。", false, nil, nil)
	}
	if problem := dnsProviderConfigProblem(provider); problem != "" {
		return NewCommandResult("DNS_CONFIG_INVALID", dnsProviderSelectionData(provider), "平台配置未完成："+problem, false, nil, nil)
	}

	name, scope, err := dnsReadTargetName(payload, provider)
	if err != nil {
		return NewCommandResult("DNS_CONFIG_INVALID", dnsProviderSelectionData(provider), err.Error(), false, nil, nil)
	}
	recordType := dnsReadRecordType(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	records, err := s.listDNSProviderRecords(ctx, provider, name, recordType)
	if err != nil {
		return NewCommandResult("DNS_LIST_FAILED", dnsProviderSelectionData(provider), err.Error(), false, nil, nil)
	}

	target := name
	if target == "" {
		target = provider.RecordName
	}
	return NewCommandResult("DNS_RECORDS_LISTED", map[string]any{
		"count":          len(records),
		"provider":       provider.Provider,
		"provider_id":    provider.ID,
		"provider_label": dnsProviderDisplayLabel(provider),
		"record_name":    provider.RecordName,
		"records":        records,
		"scope":          scope,
	}, fmt.Sprintf("Read %d %s DNS records matching %s.", len(records), provider.Provider, target), true, nil, nil)
}

func dnsProviderSelectionData(provider DNSProviderConfig) map[string]any {
	return map[string]any{
		"provider":       provider.Provider,
		"provider_id":    provider.ID,
		"provider_label": dnsProviderDisplayLabel(provider),
		"record_name":    provider.RecordName,
	}
}

func dnsProviderDisplayLabel(provider DNSProviderConfig) string {
	if label := strings.TrimSpace(provider.Label); label != "" {
		return label
	}
	return provider.Provider
}

// dnsReadTargetName maps the page's scope switch onto one record name. An empty
// name means "the whole zone/domain" for every platform.
func dnsReadTargetName(payload map[string]any, provider DNSProviderConfig) (string, string, error) {
	scope := strings.ToLower(strings.TrimSpace(stringValue(firstNonNil(payload["scope"], payload["filter"], payload["mode"]), "")))
	name := normalizeDNSRecordName(stringValue(firstNonNil(payload["name"], payload["record_name"], payload["recordName"]), ""))
	switch scope {
	case "zone", "all", "domain":
		return "", "zone", nil
	case "custom", "subdomain", "name":
		if name == "" {
			return "", "custom", errorsNew("请输入要读取的子域名或记录名")
		}
		return name, "custom", nil
	default:
		name = normalizeDNSRecordName(provider.RecordName)
		if name == "" {
			if provider.Provider == DNSProviderDNSHE && strings.TrimSpace(provider.Subdomain) != "" {
				return strings.TrimSpace(provider.Subdomain), "configured", nil
			}
			return "", "configured", errorsNew("当前平台没有填写记录名，无法读取记录")
		}
		return name, "configured", nil
	}
}

func dnsReadRecordType(payload map[string]any) string {
	raw := strings.ToUpper(strings.TrimSpace(stringValue(firstNonNil(payload["record_type"], payload["recordType"], payload["type"]), "")))
	switch raw {
	case dnspush.RecordTypeA, dnspush.RecordTypeAAAA:
		return raw
	}
	return ""
}

func normalizeDNSRecordName(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

func (s *Service) listDNSProviderRecords(ctx context.Context, provider DNSProviderConfig, name, recordType string) ([]dnsRecordSnapshot, error) {
	switch provider.Provider {
	case DNSProviderCloudflare:
		cfg := cloudflarecore.Config{
			APIToken:   provider.APIToken,
			RecordName: provider.RecordName,
			RecordType: recordType,
			TTL:        provider.TTL,
			ZoneID:     provider.ZoneID,
		}
		records, err := s.newCloudflareClient(provider.APIToken).ListRecordsWithOptions(ctx, cfg, cloudflarecore.ListOptions{Name: name, Type: recordType})
		if err != nil {
			return nil, err
		}
		out := make([]dnsRecordSnapshot, 0, len(records))
		for _, record := range records {
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
		return out, nil
	case DNSProviderAliyun:
		zoneName := name
		if zoneName == "" {
			zoneName = provider.RecordName
		}
		client := s.newAliyunClient(provider)
		zone, err := client.ResolveZone(ctx, zoneName)
		if err != nil {
			return nil, err
		}
		records, err := client.ListRecords(ctx, zone)
		if err != nil {
			return nil, err
		}
		return filterDNSRecordSnapshots(dnsSnapshotsFromPushRecords(records), name, recordType), nil
	case DNSProviderDNSPod:
		zoneName := name
		if zoneName == "" {
			zoneName = provider.RecordName
		}
		client := s.newDNSPodClient(provider)
		zone, err := client.ResolveZone(ctx, zoneName)
		if err != nil {
			return nil, err
		}
		records, err := client.ListRecords(ctx, zone)
		if err != nil {
			return nil, err
		}
		return filterDNSRecordSnapshots(dnsSnapshotsFromPushRecords(records), name, recordType), nil
	case DNSProviderDNSHE:
		client := s.newDNSHEClient(provider)
		var (
			subdomain dnshecore.Subdomain
			err       error
		)
		switch {
		case strings.TrimSpace(provider.Subdomain) != "":
			subdomain, err = client.ResolveSubdomainByName(ctx, provider.Subdomain)
		case provider.SubdomainID > 0:
			subdomain, err = client.GetSubdomain(ctx, provider.SubdomainID)
		default:
			subdomain, err = client.ResolveSubdomain(ctx, provider.RecordName)
		}
		if err != nil {
			return nil, err
		}
		// A bare subdomain label (from subdomain-only config) addresses the
		// apex of that subdomain; expand it to the full name for filtering.
		if name == "" || (name == normalizeDNSRecordName(provider.Subdomain) && !strings.Contains(name, ".")) {
			name = normalizeDNSRecordName(subdomain.FullDomain)
		}
		records, err := client.ListRecords(ctx, subdomain.ID)
		if err != nil {
			return nil, err
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
		return filterDNSRecordSnapshots(snapshots, name, recordType), nil
	}
	return nil, errorsNew("该平台暂不支持读取 DNS 记录")
}

func dnsSnapshotsFromPushRecords(records []dnspush.Record) []dnsRecordSnapshot {
	snapshots := make([]dnsRecordSnapshot, 0, len(records))
	for _, record := range records {
		snapshots = append(snapshots, dnsRecordSnapshot{
			Content: record.Content,
			ID:      record.ID,
			Name:    record.Name,
			TTL:     record.TTL,
			Type:    record.Type,
		})
	}
	return snapshots
}

// filterDNSRecordSnapshots keeps the page's name/type filter identical across
// platforms: providers that cannot filter server-side still return only what
// the page asked for.
func filterDNSRecordSnapshots(records []dnsRecordSnapshot, name, recordType string) []dnsRecordSnapshot {
	filtered := make([]dnsRecordSnapshot, 0, len(records))
	for _, record := range records {
		if name != "" && !strings.EqualFold(normalizeDNSRecordName(record.Name), name) {
			continue
		}
		if recordType != "" && !strings.EqualFold(strings.TrimSpace(record.Type), recordType) {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}
