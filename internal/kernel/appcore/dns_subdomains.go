package appcore

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"
)

// invokeDNSHEListSubdomains backs the settings subdomain picker: it reads the
// subdomains available to the saved DNSHE credentials so the UI can show labels
// and full domains instead of asking for a numeric subdomain ID.
func (s *Service) invokeDNSHEListSubdomains(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	providerID := strings.TrimSpace(stringValue(firstNonNil(payload["provider_id"], payload["providerId"]), ""))
	providers := DNSProvidersFromSnapshot(config)
	provider, ok := selectDNSProvider(providers, providerID)
	if !ok {
		if providerID != "" {
			return NewCommandResult("DNS_PROVIDER_NOT_FOUND", nil, "没有找到该 DNS 平台，请先在设置页保存后再读取。", false, nil, nil)
		}
		raw := mapValue(firstNonNil(payload["dns_provider"], payload["dnsProvider"], payload["provider_config"], payload["providerConfig"]))
		if len(raw) == 0 {
			return NewCommandResult("DNS_PROVIDER_NOT_FOUND", nil, "缺少 DNSHE 平台配置。", false, nil, nil)
		}
		provider = dnsProviderConfigFromMap(DNSProviderDNSHE, raw, 1)
	}
	if provider.Provider != DNSProviderDNSHE {
		return NewCommandResult("DNS_CONFIG_INVALID", nil, "只有 DNSHE 平台支持读取子域名列表。", false, nil, nil)
	}
	if provider.APIKey == "" || IsMaskedSecret(provider.APIKey) {
		return NewCommandResult("DNS_CONFIG_INVALID", dnsProviderSelectionData(provider), "缺少完整 DNSHE API Key", false, nil, nil)
	}
	if provider.APISecret == "" || IsMaskedSecret(provider.APISecret) {
		return NewCommandResult("DNS_CONFIG_INVALID", dnsProviderSelectionData(provider), "缺少完整 DNSHE API Secret", false, nil, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	items, err := s.newDNSHEClient(provider).ListSubdomains(ctx)
	if err != nil {
		return NewCommandResult(dnsheErrorCode(err), dnsProviderSelectionData(provider), "读取 DNSHE 子域名失败："+err.Error(), false, nil, nil)
	}
	subdomains := make([]map[string]any, 0, len(items))
	for _, item := range items {
		fullDomain := strings.TrimSpace(item.FullDomain)
		if fullDomain == "" {
			fullDomain = strings.TrimSpace(item.Subdomain + "." + item.RootDomain)
		}
		subdomains = append(subdomains, map[string]any{
			"id":          item.ID,
			"subdomain":   item.Subdomain,
			"rootdomain":  item.RootDomain,
			"full_domain": fullDomain,
			"status":      item.Status,
		})
	}
	sort.Slice(subdomains, func(i, j int) bool {
		return stringValue(subdomains[i]["full_domain"], "") < stringValue(subdomains[j]["full_domain"], "")
	})
	return NewCommandResult("DNS_SUBDOMAINS_LISTED", map[string]any{
		"count":       len(subdomains),
		"provider":    provider.Provider,
		"provider_id": provider.ID,
		"subdomains":  subdomains,
	}, "读取到 DNSHE 子域名 "+strconv.Itoa(len(subdomains))+" 个。", true, nil, nil)
}
