// dnsProviderForm 特征测试（重构计划 §4.2）：表单↔快照往返、脱敏、可用性门控。
import { describe, expect, it } from "vitest";
import { createDnsProviderForm, dnsProviderDefaultTTL, dnsProviderFormComplete, dnsProviderFormFromSnapshot, dnsProviderPlatformLabel, dnsProviderSnapshotFromForm } from "./dnsProviderForm";

describe("defaults", () => {
  it("per-provider TTL and platform labels", () => {
    expect(dnsProviderDefaultTTL("cloudflare")).toBe(300);
    expect(dnsProviderDefaultTTL("dnshe")).toBe(600);
    expect(dnsProviderDefaultTTL("aliyun")).toBe(600);
    expect(dnsProviderDefaultTTL("dnspod")).toBe(600);
    expect(dnsProviderPlatformLabel("dnshe")).toBe("DNSHE");
    expect(dnsProviderPlatformLabel("aliyun")).toBe("阿里云 DNS");
    expect(dnsProviderPlatformLabel("dnspod")).toBe("DNSPod");
    expect(dnsProviderPlatformLabel("cloudflare")).toBe("Cloudflare");
    const form = createDnsProviderForm("cloudflare", 0);
    expect(form.id).toBe("cloudflare-1");
    expect(form.enabled).toBe(true);
    expect(form.recordType).toBe("A");
    expect(form.topN).toBe(5);
  });
});

describe("snapshot -> form", () => {
  it("cloudflare masked token is kept as hint, not as writable token", () => {
    const form = dnsProviderFormFromSnapshot({ provider: "cloudflare", api_token: "abc***def", zone_id: "z1", record_name: "r.example", enabled: true } as never, 0);
    expect(form.maskedToken).toBe("abc***def");
    expect(form.apiToken).toBe("");
    expect(form.zoneId).toBe("z1");
  });

  it("dnshe masked key/secret split and subdomain id fallback", () => {
    const form = dnsProviderFormFromSnapshot({ provider: "dnshe", api_key: "***", api_secret: "real-secret", subdomain_id: 42, enabled: true } as never, 1);
    expect(form.maskedKey).toBe("***");
    expect(form.apiKey).toBe("");
    expect(form.apiSecret).toBe("real-secret");
    expect(form.subdomainId).toBe(42);
    const withSubdomain = dnsProviderFormFromSnapshot({ provider: "dnshe", subdomain: "home.example.com", subdomain_id: 42 } as never, 0);
    expect(withSubdomain.subdomainId).toBe(0);
  });

  it("maps routing rules with stable ids and clamps ttl", () => {
    const form = dnsProviderFormFromSnapshot(
      {
        provider: "cloudflare",
        ttl: 1,
        routing_enabled: true,
        routing_rules: [{ enabled: true, filter_mode: "deny", filter_tokens: "LAX", name: "r1", record_name: "a.example", record_type: "AAAA", top_n: 3 }],
      } as never,
      2,
    );
    expect(form.ttl).toBe(300);
    expect(form.routingRules[0]).toEqual({ enabled: true, filterMode: "deny", filterTokens: "LAX", id: "cloudflare-3-route-0", name: "r1", recordName: "a.example", recordType: "AAAA", topN: 3 });
  });
});

describe("form -> snapshot", () => {
  it("enabled requires completeness", () => {
    const incomplete = createDnsProviderForm("cloudflare", 0);
    expect(dnsProviderFormComplete(incomplete)).toBe(false);
    expect(dnsProviderSnapshotFromForm(incomplete).enabled).toBe(false);
    incomplete.apiToken = "tok";
    incomplete.zoneId = "zone";
    incomplete.recordName = "r.example";
    expect(dnsProviderFormComplete(incomplete)).toBe(true);
    const snap = dnsProviderSnapshotFromForm(incomplete);
    expect(snap.enabled).toBe(true);
    expect(snap.api_token).toBe("tok");
    expect(snap.ttl).toBe(300);
  });

  it("omits empty credentials and picks dnshe subdomain vs id", () => {
    const form = createDnsProviderForm("dnshe", 0);
    form.apiKey = "k";
    form.apiSecret = "s";
    form.subdomain = "home.example.com";
    const snap = dnsProviderSnapshotFromForm(form);
    expect(snap.subdomain).toBe("home.example.com");
    expect("subdomain_id" in snap).toBe(false);
    expect(snap.api_key).toBe("k");
    form.subdomain = "";
    form.subdomainId = 7;
    const snap2 = dnsProviderSnapshotFromForm(form);
    expect(snap2.subdomain_id).toBe(7);
    expect("subdomain" in snap2).toBe(false);
    const empty = createDnsProviderForm("cloudflare", 0);
    expect("api_token" in dnsProviderSnapshotFromForm(empty)).toBe(false);
  });

  it("routing target alone satisfies completeness", () => {
    const form = createDnsProviderForm("cloudflare", 0);
    form.apiToken = "tok";
    form.zoneId = "zone";
    form.routingEnabled = true;
    form.routingRules = [{ enabled: true, filterMode: "allow", filterTokens: "", id: "x", name: "", recordName: "a.example", recordType: "A", topN: 5 }];
    expect(dnsProviderFormComplete(form)).toBe(true);
    const snap = dnsProviderSnapshotFromForm(form);
    expect(snap.routing_rules?.[0].record_name).toBe("a.example");
    expect(snap.routing_rules?.[0].filter_mode).toBe("allow");
  });
});
