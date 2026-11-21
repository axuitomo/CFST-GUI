package appcore

import (
	"strconv"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/alidnscore"
	"github.com/axuitomo/CFST-GUI/internal/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/dnspodcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

const (
	DNSProviderCloudflare = "cloudflare"
	DNSProviderDNSHE      = "dnshe"
	DNSProviderAliyun     = "aliyun"
	DNSProviderDNSPod     = "dnspod"

	DefaultDNSProviderTopN = probecore.DefaultCloudflareUploadTopN
	DefaultDNSHETTL        = dnshecore.DefaultTTL
	DefaultAliyunTTL       = alidnscore.DefaultTTL
	DefaultDNSPodTTL       = dnspodcore.DefaultTTL
)

// DNSProviderConfig is one DNS upload target. Every supported platform is
// described by the same fields so the push pipeline never branches on the
// platform before it has to.
type DNSProviderConfig struct {
	APISecret   string
	APIToken    string
	APIKey      string
	AllowCNAME  bool
	Enabled     bool
	ID          string
	Label       string
	Provider    string
	RecordName  string
	RecordType  string
	Routing     UploadCloudflareRoutingConfig
	Subdomain   string
	SubdomainID int
	TTL         int
	TopN        int
	ZoneID      string
}

// DNSProvidersFromSnapshot reads the DNS upload targets. The legacy top-level
// cloudflare block is still honored when no dns_providers list is configured.
func DNSProvidersFromSnapshot(snapshot map[string]any) []DNSProviderConfig {
	items := snapshot["dns_providers"]
	if items == nil {
		items = snapshot["dnsProviders"]
	}
	providers := make([]DNSProviderConfig, 0)
	index := 0
	for _, item := range configItemMaps(items) {
		provider := normalizeDNSProviderName(stringValue(firstNonNil(item["provider"], item["platform"], item["type"]), ""))
		if provider == "" {
			continue
		}
		index++
		providers = append(providers, dnsProviderConfigFromMap(provider, item, index))
	}
	if len(providers) > 0 {
		return providers
	}
	return []DNSProviderConfig{legacyCloudflareDNSProvider(snapshot)}
}

func legacyCloudflareDNSProvider(snapshot map[string]any) DNSProviderConfig {
	cloudflare := mapValue(snapshot["cloudflare"])
	upload := mapValue(firstNonNil(mapValue(snapshot["upload"])["cloudflare"], cloudflare))
	return DNSProviderConfig{
		APIToken:   strings.TrimSpace(stringValue(firstNonNil(cloudflare["api_token"], cloudflare["apiToken"]), "")),
		AllowCNAME: boolValue(firstNonNil(cloudflare["allow_cname"], cloudflare["allowCNAME"], cloudflare["allow_modify_cname"], cloudflare["allowModifyCNAME"]), false),
		Enabled:    CloudflareProviderEnabledFromSnapshot(snapshot),
		ID:         DNSProviderCloudflare,
		Label:      "Cloudflare",
		Provider:   DNSProviderCloudflare,
		RecordName: strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_name"], cloudflare["recordName"]), "")),
		RecordType: normalizeDNSProviderRecordType(stringValue(firstNonNil(cloudflare["record_type"], cloudflare["recordType"]), cloudflareRecordTypeA)),
		Routing:    CloudflareRoutingConfigFromSnapshot(snapshot),
		TTL:        normalizeDNSProviderTTL(DNSProviderCloudflare, intValue(cloudflare["ttl"], DefaultCloudflareTTL)),
		TopN:       max(0, intValue(firstNonNil(cloudflare["top_n"], cloudflare["topN"], upload["top_n"], upload["topN"]), DefaultDNSProviderTopN)),
		ZoneID:     strings.TrimSpace(stringValue(firstNonNil(cloudflare["zone_id"], cloudflare["zoneId"]), "")),
	}
}

func dnsProviderConfigFromMap(provider string, raw map[string]any, index int) DNSProviderConfig {
	cfg := DNSProviderConfig{
		APISecret:   strings.TrimSpace(stringValue(firstNonNil(raw["api_secret"], raw["apiSecret"]), "")),
		APIToken:    strings.TrimSpace(stringValue(firstNonNil(raw["api_token"], raw["apiToken"]), "")),
		APIKey:      strings.TrimSpace(stringValue(firstNonNil(raw["api_key"], raw["apiKey"]), "")),
		AllowCNAME:  boolValue(firstNonNil(raw["allow_cname"], raw["allowCNAME"], raw["allow_modify_cname"], raw["allowModifyCNAME"]), false),
		Enabled:     boolValue(firstNonNil(raw["enabled"], raw["enable"]), true),
		ID:          strings.TrimSpace(stringValue(firstNonNil(raw["id"], raw["provider_id"], raw["providerId"]), "")),
		Label:       strings.TrimSpace(stringValue(firstNonNil(raw["name"], raw["label"]), "")),
		Provider:    provider,
		RecordName:  strings.TrimSpace(stringValue(firstNonNil(raw["record_name"], raw["recordName"]), "")),
		RecordType:  normalizeDNSProviderRecordType(stringValue(firstNonNil(raw["record_type"], raw["recordType"]), cloudflareRecordTypeA)),
		Subdomain:   strings.TrimSpace(stringValue(firstNonNil(raw["subdomain"], raw["subdomain_name"], raw["subdomainName"]), "")),
		SubdomainID: max(0, intValue(firstNonNil(raw["subdomain_id"], raw["subdomainId"]), 0)),
		TTL:         normalizeDNSProviderTTL(provider, intValue(raw["ttl"], dnsProviderDefaultTTL(provider))),
		TopN:        max(0, intValue(firstNonNil(raw["top_n"], raw["topN"]), DefaultDNSProviderTopN)),
		ZoneID:      strings.TrimSpace(stringValue(firstNonNil(raw["zone_id"], raw["zoneId"]), "")),
	}
	if cfg.ID == "" {
		cfg.ID = provider + "-" + strconv.Itoa(index)
	}
	cfg.Routing = UploadCloudflareRoutingConfig{
		Enabled: boolValue(firstNonNil(raw["routing_enabled"], raw["routingEnabled"]), false),
		Rules:   uploadCloudflareRoutingRulesFromAny(firstNonNil(raw["routing_rules"], raw["routingRules"])),
	}
	return cfg
}

// DNSProviderEnabledFromSnapshot reports whether any DNS upload target is enabled.
func DNSProviderEnabledFromSnapshot(snapshot map[string]any) bool {
	for _, provider := range DNSProvidersFromSnapshot(snapshot) {
		if provider.Enabled {
			return true
		}
	}
	return false
}

func normalizeDNSProviderName(raw string) string {
	module, ok := dnsProviderModuleForAlias(raw)
	if !ok {
		return ""
	}
	return module.kind
}

func dnsProviderDefaultTTL(provider string) int {
	return dnsProviderModuleOrDefault(provider).defaultTTL
}

func normalizeDNSProviderTTL(provider string, ttl int) int {
	return dnsProviderModuleOrDefault(provider).normalizeTTL(ttl)
}

func normalizeDNSProviderRecordType(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case cloudflareRecordTypeAll:
		return cloudflareRecordTypeAll
	case cloudflareRecordTypeAAAA:
		return cloudflareRecordTypeAAAA
	default:
		return cloudflareRecordTypeA
	}
}

func dnsProviderDisplayName(provider DNSProviderConfig) string {
	if label := strings.TrimSpace(provider.Label); label != "" {
		return label
	}
	name := dnsProviderPlatformName(provider.Provider)
	if record := strings.TrimSpace(provider.RecordName); record != "" {
		return name + " (" + record + ")"
	}
	return name
}

func dnsProviderPlatformName(provider string) string {
	return dnsProviderModuleOrDefault(provider).label
}

// dnsProviderSnapshot returns a snapshot whose legacy cloudflare block describes
// this provider, so the Cloudflare push path keeps a single code path.
func dnsProviderSnapshot(snapshot map[string]any, provider DNSProviderConfig) map[string]any {
	clone := cloneAnyMap(snapshot)
	cloudflare := map[string]any{
		"allow_cname":     provider.AllowCNAME,
		"api_token":       provider.APIToken,
		"enabled":         provider.Enabled,
		"record_name":     provider.RecordName,
		"record_type":     provider.RecordType,
		"routing_enabled": provider.Routing.Enabled,
		"top_n":           provider.TopN,
		"ttl":             provider.TTL,
		"zone_id":         provider.ZoneID,
	}
	rules := make([]any, 0, len(provider.Routing.Rules))
	for _, rule := range provider.Routing.Rules {
		rules = append(rules, map[string]any{
			"enabled":       rule.Enabled,
			"filter_mode":   rule.FilterMode,
			"filter_tokens": rule.FilterTokens,
			"name":          rule.Name,
			"record_name":   rule.RecordName,
			"record_type":   rule.RecordType,
			"top_n":         rule.TopN,
		})
	}
	cloudflare["routing_rules"] = rules
	clone["cloudflare"] = cloudflare
	upload := mapValue(clone["upload"])
	uploadCloudflare := cloneAnyMap(mapValue(upload["cloudflare"]))
	uploadCloudflare["routing_enabled"] = provider.Routing.Enabled
	uploadCloudflare["routing_rules"] = rules
	uploadCloudflare["top_n"] = provider.TopN
	upload["cloudflare"] = uploadCloudflare
	clone["upload"] = upload
	return clone
}

func hasUsableRoutingTarget(routing UploadCloudflareRoutingConfig) bool {
	if !routing.Enabled {
		return false
	}
	for _, rule := range routing.Rules {
		if rule.Enabled && strings.TrimSpace(rule.RecordName) != "" {
			return true
		}
	}
	return false
}

// dnsProviderConfigProblem reports why a provider cannot be pushed yet.
func dnsProviderConfigProblem(provider DNSProviderConfig) string {
	return dnsProviderModuleOrDefault(provider.Provider).configProblem(provider)
}

func configItemMaps(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if mapped := mapValue(item); len(mapped) > 0 {
				result = append(result, mapped)
			}
		}
		return result
	default:
		return nil
	}
}
