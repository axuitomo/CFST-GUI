package appcore

import (
	"context"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
)

// dnsProviderModule is the single place that describes one DNS platform.
// Adding a platform = one client package + one dns_provider_<kind>.go file +
// one line in dnsProviderModules; no existing switch needs to change.
type dnsProviderModule struct {
	kind string
	// aliases are accepted when reading config["dns_providers"][i].provider,
	// including the historical misspellings already in user configs.
	aliases    []string
	label      string
	defaultTTL int
	// normalizeTTL clamps a configured TTL to a value the platform accepts.
	normalizeTTL func(ttl int) int
	// ttlPair returns the two TTLs the staged self-test writes, so the
	// attribute-update stage exercises a real change.
	ttlPair func(ttl int) (int, int)
	// configProblem explains why the target cannot be pushed yet, "" when usable.
	configProblem func(provider DNSProviderConfig) string
	push          func(s *Service, ctx context.Context, provider DNSProviderConfig, snapshot map[string]any, rows []probecore.ProbeRow) CommandResult
	listRecords   func(s *Service, ctx context.Context, provider DNSProviderConfig, target dnsReadTarget, recordType string) ([]dnsRecordSnapshot, string, error)
	selfTest      func(s *Service, provider DNSProviderConfig, testName string, dnsheSubdomain *dnshecore.Subdomain) dnsSelfTestAdapter
}

// dnsProviderModules is the platform table in UI order. It is filled in init
// because the module literals call helpers (TTL normalization, display name)
// that read the same table back, which a package-level initializer cannot do.
var dnsProviderModules []dnsProviderModule

func init() {
	dnsProviderModules = []dnsProviderModule{
		cloudflareDNSProviderModule,
		dnsheDNSProviderModule,
		aliyunDNSProviderModule,
		dnspodDNSProviderModule,
	}
}

// dnsProviderModuleFor reports the module for an exact kind.
func dnsProviderModuleFor(kind string) (dnsProviderModule, bool) {
	for _, module := range dnsProviderModules {
		if module.kind == kind {
			return module, true
		}
	}
	return dnsProviderModule{}, false
}

// dnsProviderModuleOrDefault keeps the historical behavior: an unknown or empty
// kind is treated as Cloudflare.
func dnsProviderModuleOrDefault(kind string) dnsProviderModule {
	if module, ok := dnsProviderModuleFor(kind); ok {
		return module
	}
	return cloudflareDNSProviderModule
}

// dnsProviderModuleForAlias resolves a configured provider string, including
// legacy aliases, to its module.
func dnsProviderModuleForAlias(raw string) (dnsProviderModule, bool) {
	name := strings.ToLower(strings.TrimSpace(raw))
	for _, module := range dnsProviderModules {
		for _, alias := range module.aliases {
			if name == alias {
				return module, true
			}
		}
	}
	return dnsProviderModule{}, false
}

// dnsProviderKinds lists the registered kinds in UI order.
func dnsProviderKinds() []string {
	kinds := make([]string, 0, len(dnsProviderModules))
	for _, module := range dnsProviderModules {
		kinds = append(kinds, module.kind)
	}
	return kinds
}
