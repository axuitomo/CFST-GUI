package appcore

import "strings"

type PostProbePushConfig struct {
	CloudflareEnabled bool `json:"cloudflare_enabled"`
	GitHubEnabled     bool `json:"github_enabled"`
}

func PostProbePushConfigFromSnapshot(snapshot map[string]any) PostProbePushConfig {
	raw := mapValue(snapshot["post_probe_push"])
	return PostProbePushConfig{
		CloudflareEnabled: boolValue(firstNonNil(raw["cloudflare_enabled"], raw["cloudflareEnabled"]), false),
		GitHubEnabled:     boolValue(firstNonNil(raw["github_enabled"], raw["githubEnabled"]), false),
	}
}

func CloudflareProviderEnabledFromSnapshot(snapshot map[string]any) bool {
	cloudflare := mapValue(snapshot["cloudflare"])
	if boolValue(firstNonNil(cloudflare["enabled"], cloudflare["cloudflare_enabled"], cloudflare["cloudflareEnabled"]), false) {
		return true
	}
	return cloudflareRoutingProviderReady(snapshot)
}

func GitHubProviderEnabledFromSnapshot(snapshot map[string]any) bool {
	github := mapValue(snapshot["github"])
	if len(github) == 0 {
		export := mapValue(snapshot["export"])
		github = mapValue(export["github"])
	}
	return boolValue(firstNonNil(github["enabled"], github["github_enabled"], github["githubEnabled"]), false)
}

func cloudflareRoutingProviderReady(snapshot map[string]any) bool {
	cloudflare := mapValue(snapshot["cloudflare"])
	if len(cloudflare) == 0 {
		cloudflare = snapshot
	}
	apiToken := strings.TrimSpace(stringValue(firstNonNil(cloudflare["api_token"], cloudflare["apiToken"]), ""))
	zoneID := strings.TrimSpace(stringValue(firstNonNil(cloudflare["zone_id"], cloudflare["zoneId"]), ""))
	if apiToken == "" || IsMaskedSecret(apiToken) || zoneID == "" {
		return false
	}

	routing := CloudflareRoutingConfigFromSnapshot(snapshot)
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
