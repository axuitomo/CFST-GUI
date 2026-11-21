package appcore

import (
	"testing"
)

// The module table is the only place a DNS platform is described. These tests
// pin that contract: every known kind resolves, the table stays complete, and
// the per-platform metadata keeps the values the old switches produced.
func TestDNSProviderModulesCoverEveryKind(t *testing.T) {
	kinds := dnsProviderKinds()
	if len(kinds) != 4 {
		t.Fatalf("registered kinds = %v, want 4 platforms", kinds)
	}
	seen := map[string]bool{}
	for _, kind := range kinds {
		if seen[kind] {
			t.Fatalf("duplicate kind %q in dnsProviderModules", kind)
		}
		seen[kind] = true
		module, ok := dnsProviderModuleFor(kind)
		if !ok {
			t.Fatalf("kind %q has no module", kind)
		}
		switch {
		case module.label == "":
			t.Fatalf("%s: empty label", kind)
		case module.defaultTTL <= 0:
			t.Fatalf("%s: defaultTTL = %d", kind, module.defaultTTL)
		case module.normalizeTTL == nil || module.ttlPair == nil || module.configProblem == nil:
			t.Fatalf("%s: missing metadata func", kind)
		case module.push == nil || module.listRecords == nil || module.selfTest == nil:
			t.Fatalf("%s: missing dispatch func", kind)
		}
		if _, ok := dnsProviderModuleForAlias(kind); !ok {
			t.Fatalf("%s: own kind is not a resolvable alias", kind)
		}
	}
}

func TestDNSProviderAliasesResolve(t *testing.T) {
	cases := map[string]string{
		"cloudflare": "cloudflare",
		" DNSHE ":    "dnshe",
		"aliyun":     "aliyun",
		"alidns":     "aliyun",
		"ali":        "aliyun",
		"dnspod":     "dnspod",
		"dndps":      "dnspod",
		"tencent":    "dnspod",
		"mystery":    "",
		"":           "",
	}
	for raw, want := range cases {
		if got := normalizeDNSProviderName(raw); got != want {
			t.Errorf("normalizeDNSProviderName(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestDNSProviderModuleMetadata(t *testing.T) {
	labels := map[string]string{
		DNSProviderCloudflare: "Cloudflare",
		DNSProviderDNSHE:      "DNSHE",
		DNSProviderAliyun:     "阿里云 DNS",
		DNSProviderDNSPod:     "DNSPod",
	}
	defaults := map[string]int{
		DNSProviderCloudflare: 300,
		DNSProviderDNSHE:      600,
		DNSProviderAliyun:     600,
		DNSProviderDNSPod:     600,
	}
	for kind, want := range labels {
		if got := dnsProviderPlatformName(kind); got != want {
			t.Errorf("dnsProviderPlatformName(%q) = %q, want %q", kind, got, want)
		}
		if got := dnsProviderDefaultTTL(kind); got != defaults[kind] {
			t.Errorf("dnsProviderDefaultTTL(%q) = %d, want %d", kind, got, defaults[kind])
		}
	}
	// An unknown kind keeps the historical Cloudflare default.
	if got := dnsProviderPlatformName("mystery"); got != "Cloudflare" {
		t.Errorf("unknown kind label = %q, want Cloudflare", got)
	}
}

func TestNormalizeDNSProviderTTLPerPlatform(t *testing.T) {
	cases := []struct {
		provider string
		ttl      int
		want     int
	}{
		{DNSProviderCloudflare, 0, 300},
		{DNSProviderCloudflare, 60, 60},
		{DNSProviderCloudflare, 600, 600},
		{DNSProviderCloudflare, 7, 300},
		{DNSProviderDNSHE, 0, 600},
		{DNSProviderDNSHE, 1, 1},
		{DNSProviderAliyun, 0, 600},
		{DNSProviderAliyun, 59, 600},
		{DNSProviderAliyun, 60, 60},
		{DNSProviderDNSPod, 59, 600},
		{DNSProviderDNSPod, 900, 900},
	}
	for _, item := range cases {
		if got := normalizeDNSProviderTTL(item.provider, item.ttl); got != item.want {
			t.Errorf("normalizeDNSProviderTTL(%q, %d) = %d, want %d", item.provider, item.ttl, got, item.want)
		}
	}
}

func TestDNSProviderConfigProblemPerPlatform(t *testing.T) {
	cases := []struct {
		name     string
		provider DNSProviderConfig
		want     string
	}{
		{"cloudflare token", DNSProviderConfig{Provider: DNSProviderCloudflare}, "缺少完整 Cloudflare API Token"},
		{"cloudflare zone", DNSProviderConfig{Provider: DNSProviderCloudflare, APIToken: "tok"}, "缺少 Cloudflare Zone ID"},
		{"cloudflare name", DNSProviderConfig{Provider: DNSProviderCloudflare, APIToken: "tok", ZoneID: "z"}, "缺少 Cloudflare DNS 记录名称"},
		{"cloudflare ok", DNSProviderConfig{Provider: DNSProviderCloudflare, APIToken: "tok", ZoneID: "z", RecordName: "a.example"}, ""},
		{"dnshe key", DNSProviderConfig{Provider: DNSProviderDNSHE}, "缺少完整 DNSHE API Key"},
		{"dnshe secret", DNSProviderConfig{Provider: DNSProviderDNSHE, APIKey: "k"}, "缺少完整 DNSHE API Secret"},
		{"dnshe target", DNSProviderConfig{Provider: DNSProviderDNSHE, APIKey: "k", APISecret: "s"}, "缺少 DNSHE 记录名称（完整域名或子域名）"},
		{"dnshe ok", DNSProviderConfig{Provider: DNSProviderDNSHE, APIKey: "k", APISecret: "s", Subdomain: "test"}, ""},
		{"aliyun key", DNSProviderConfig{Provider: DNSProviderAliyun}, "缺少完整阿里云 AccessKey ID"},
		{"aliyun secret", DNSProviderConfig{Provider: DNSProviderAliyun, APIKey: "k"}, "缺少完整阿里云 AccessKey Secret"},
		{"aliyun target", DNSProviderConfig{Provider: DNSProviderAliyun, APIKey: "k", APISecret: "s"}, "缺少阿里云 DNS 记录名称（完整域名）"},
		{"aliyun ok", DNSProviderConfig{Provider: DNSProviderAliyun, APIKey: "k", APISecret: "s", RecordName: "a.example"}, ""},
		{"dnspod key", DNSProviderConfig{Provider: DNSProviderDNSPod}, "缺少完整 DNSPod SecretId"},
		{"dnspod secret", DNSProviderConfig{Provider: DNSProviderDNSPod, APIKey: "k"}, "缺少完整 DNSPod SecretKey"},
		{"dnspod target", DNSProviderConfig{Provider: DNSProviderDNSPod, APIKey: "k", APISecret: "s"}, "缺少 DNSPod 记录名称（完整域名）"},
		{"dnspod ok", DNSProviderConfig{Provider: DNSProviderDNSPod, APIKey: "k", APISecret: "s", RecordName: "a.example"}, ""},
	}
	for _, item := range cases {
		if got := dnsProviderConfigProblem(item.provider); got != item.want {
			t.Errorf("%s: problem = %q, want %q", item.name, got, item.want)
		}
	}
}

func TestDNSProviderSelfTestTTLPairPerPlatform(t *testing.T) {
	cases := []struct {
		provider DNSProviderConfig
		base     int
		alt      int
	}{
		{DNSProviderConfig{Provider: DNSProviderCloudflare, TTL: 0}, 300, 600},
		{DNSProviderConfig{Provider: DNSProviderCloudflare, TTL: 600}, 600, 300},
		{DNSProviderConfig{Provider: DNSProviderDNSHE, TTL: 0}, 600, 1200},
		{DNSProviderConfig{Provider: DNSProviderAliyun, TTL: 1}, 600, 1200},
		{DNSProviderConfig{Provider: DNSProviderDNSPod, TTL: 900}, 900, 600},
	}
	for _, item := range cases {
		if base, alt := dnsSelfTestTTLPair(item.provider); base != item.base || alt != item.alt {
			t.Errorf("%s ttl = %d: pair = (%d, %d), want (%d, %d)", item.provider.Provider, item.provider.TTL, base, alt, item.base, item.alt)
		}
	}
}

func TestDNSProviderModulesWireSelfTestAdapter(t *testing.T) {
	for _, kind := range dnsProviderKinds() {
		module, _ := dnsProviderModuleFor(kind)
		if module.selfTest(&Service{}, DNSProviderConfig{Provider: kind}, "selftest.example.com", nil) == nil {
			t.Fatalf("%s: selfTest returned no adapter", kind)
		}
	}
}
