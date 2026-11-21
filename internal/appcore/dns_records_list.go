package appcore

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

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

	target, err := resolveDNSReadTarget(payload, provider)
	if err != nil {
		return NewCommandResult("DNS_CONFIG_INVALID", dnsProviderSelectionData(provider), err.Error(), false, nil, nil)
	}
	recordType := dnsReadRecordType(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	records, zoneName, err := s.listDNSProviderRecords(ctx, provider, target, recordType)
	if err != nil {
		return NewCommandResult("DNS_LIST_FAILED", dnsProviderSelectionData(provider), err.Error(), false, nil, nil)
	}

	data := map[string]any{
		"count":          len(records),
		"provider":       provider.Provider,
		"provider_id":    provider.ID,
		"provider_label": dnsProviderDisplayLabel(provider),
		"record_name":    provider.RecordName,
		"records":        records,
		"scope":          target.Scope,
	}
	if zoneName != "" {
		data["zone_name"] = zoneName
	}
	return NewCommandResult("DNS_RECORDS_LISTED", data, fmt.Sprintf("Read %d %s DNS records matching %s.", len(records), provider.Provider, target.describe(provider)), true, nil, nil)
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

// dnsReadTarget is the resolved read filter. Names hold exact record names,
// Labels hold bare first-level subdomain labels, and an empty target reads the
// whole zone/domain.
type dnsReadTarget struct {
	Labels []string
	Names  []string
	Scope  string
}

// dnsReadTarget maps the page's scope switch onto record names. The configured
// scope keeps record_name first and falls back to the first enabled split-rule
// target, so a routing-only config can still be read; the routes scope reads
// every enabled split-rule target.
func resolveDNSReadTarget(payload map[string]any, provider DNSProviderConfig) (dnsReadTarget, error) {
	scope := strings.ToLower(strings.TrimSpace(stringValue(firstNonNil(payload["scope"], payload["filter"], payload["mode"]), "")))
	name := normalizeDNSRecordName(stringValue(firstNonNil(payload["name"], payload["record_name"], payload["recordName"]), ""))
	switch scope {
	case "zone", "all", "domain":
		return dnsReadTarget{Scope: "zone"}, nil
	case "custom", "subdomain", "name":
		if name == "" {
			return dnsReadTarget{}, errorsNew("请输入要读取的子域名或记录名")
		}
		return dnsTargetForName(name, "custom"), nil
	case "routes", "routing":
		if names := dnsRoutingRecordNames(provider); len(names) > 0 {
			return dnsReadTarget{Names: names, Scope: "routes"}, nil
		}
		return dnsReadTarget{}, errorsNew("当前平台没有启用的分流规则记录名")
	default:
		if name = normalizeDNSRecordName(provider.RecordName); name != "" {
			return dnsTargetForName(name, "configured"), nil
		}
		if provider.Provider == DNSProviderDNSHE && strings.TrimSpace(provider.Subdomain) != "" {
			return dnsTargetForName(normalizeDNSRecordName(provider.Subdomain), "configured"), nil
		}
		if names := dnsRoutingRecordNames(provider); len(names) > 0 {
			return dnsReadTarget{Names: names[:1], Scope: "configured"}, nil
		}
		return dnsReadTarget{}, errorsNew("当前平台没有填写记录名，无法读取记录")
	}
}

// dnsTargetForName keeps a bare first-level label fuzzy so "test" can match the
// zone's own records without the page knowing the zone name.
func dnsTargetForName(name, scope string) dnsReadTarget {
	if strings.Contains(name, ".") {
		return dnsReadTarget{Names: []string{name}, Scope: scope}
	}
	return dnsReadTarget{Labels: []string{name}, Scope: scope}
}

// dnsRoutingRecordNames lists enabled split-rule record names in rule order.
func dnsRoutingRecordNames(provider DNSProviderConfig) []string {
	names := make([]string, 0, len(provider.Routing.Rules))
	for _, rule := range provider.Routing.Rules {
		if !rule.Enabled {
			continue
		}
		name := normalizeDNSRecordName(rule.RecordName)
		if name == "" || slices.Contains(names, name) {
			continue
		}
		names = append(names, name)
	}
	return names
}

func (target dnsReadTarget) describe(provider DNSProviderConfig) string {
	if len(target.Names) > 0 {
		return strings.Join(target.Names, ", ")
	}
	if len(target.Labels) > 0 {
		return strings.Join(target.Labels, ", ") + " (any zone record named by this first label)"
	}
	if name := normalizeDNSRecordName(provider.RecordName); name != "" {
		return name
	}
	return "current zone"
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

func (s *Service) listDNSProviderRecords(ctx context.Context, provider DNSProviderConfig, target dnsReadTarget, recordType string) ([]dnsRecordSnapshot, string, error) {
	module, ok := dnsProviderModuleFor(provider.Provider)
	if !ok {
		return nil, "", errorsNew("该平台暂不支持读取 DNS 记录")
	}
	return module.listRecords(s, ctx, provider, target, recordType)
}

// dnsZoneLookupName picks the name used to resolve the owning zone: a target
// record name when there is one, otherwise the configured record name.
func dnsZoneLookupName(provider DNSProviderConfig, target dnsReadTarget) string {
	if len(target.Names) > 0 {
		return target.Names[0]
	}
	return normalizeDNSRecordName(provider.RecordName)
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
func filterDNSRecordSnapshots(records []dnsRecordSnapshot, target dnsReadTarget, recordType string) []dnsRecordSnapshot {
	filtered := make([]dnsRecordSnapshot, 0, len(records))
	for _, record := range records {
		if !dnsRecordMatchesTarget(record.Name, target) {
			continue
		}
		if recordType != "" && !strings.EqualFold(strings.TrimSpace(record.Type), recordType) {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}

// dnsRecordMatchesTarget matches exact names and bare first-level labels; an
// empty target keeps every record.
func dnsRecordMatchesTarget(name string, target dnsReadTarget) bool {
	if len(target.Names) == 0 && len(target.Labels) == 0 {
		return true
	}
	normalized := normalizeDNSRecordName(name)
	if slices.Contains(target.Names, normalized) {
		return true
	}
	label := normalized
	if index := strings.Index(label, "."); index >= 0 {
		label = label[:index]
	}
	return slices.Contains(target.Labels, label)
}
