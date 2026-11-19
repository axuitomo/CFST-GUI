import { describe, expect, it } from "vitest";
import { normalizeConfigSnapshot } from "../bridge/config";

describe("normalizeConfigSnapshot dns_providers", () => {
  it("derives one Cloudflare platform from the legacy cloudflare block", () => {
    const snapshot = normalizeConfigSnapshot({
      cloudflare: { api_token: "token-1", enabled: true, record_name: "edge.example.com", record_type: "AAAA", ttl: 60, zone_id: "zone-1" },
    });
    expect(snapshot.dns_providers).toHaveLength(1);
    const provider = snapshot.dns_providers[0];
    expect(provider.provider).toBe("cloudflare");
    expect(provider.id).toBe("cloudflare");
    expect(provider.enabled).toBe(true);
    expect(provider.record_name).toBe("edge.example.com");
    expect(provider.record_type).toBe("AAAA");
    expect(provider.ttl).toBe(60);
    expect(provider.zone_id).toBe("zone-1");
  });

  it("keeps cloudflare and dnshe entries with their own credentials", () => {
    const snapshot = normalizeConfigSnapshot({
      dns_providers: [
        { enabled: true, id: "cf", name: "主站", provider: "cloudflare", api_token: "token-1", zone_id: "zone-1", record_name: "edge.example.com", record_type: "A", ttl: 300, top_n: 3 },
        { api_key: "key-1", api_secret: "s1", id: "he", provider: "dnshe", record_name: "api.example.com", ttl: 0 },
      ],
    });
    expect(snapshot.dns_providers).toHaveLength(2);
    expect(snapshot.dns_providers[0]).toMatchObject({ id: "cf", name: "主站", top_n: 3, ttl: 300 });
    expect(snapshot.dns_providers[1]).toMatchObject({ api_key: "key-1", api_secret: "s1", enabled: true, provider: "dnshe", record_name: "api.example.com", subdomain_id: 0, ttl: 600 });
    expect(snapshot.dns_providers[1].api_token).toBeUndefined();
  });

  it("keeps aliyun and dnspod entries with their own credentials and TTL defaults", () => {
    const snapshot = normalizeConfigSnapshot({
      dns_providers: [
        { api_key: "LTAI", api_secret: "a1", id: "ali", provider: "alidns", record_name: "api.example.com", ttl: 10 },
        { api_key: "i2", api_secret: "s2", id: "dp", provider: "dndps", record_name: "edge.example.com", record_type: "AAAA", ttl: 1200 },
        { api_key: "AKID", api_secret: "t1", id: "tc", provider: "tencentcloud", record_name: "www.example.com" },
      ],
    });
    expect(snapshot.dns_providers).toHaveLength(3);
    expect(snapshot.dns_providers[0]).toMatchObject({ api_key: "LTAI", api_secret: "a1", provider: "aliyun", ttl: 600 });
    expect(snapshot.dns_providers[1]).toMatchObject({ id: "dp", provider: "dnspod", record_type: "AAAA", ttl: 1200 });
    expect(snapshot.dns_providers[2]).toMatchObject({ provider: "dnspod", ttl: 600 });
    expect(snapshot.dns_providers[0].subdomain_id).toBeUndefined();
  });
});

describe("normalizeConfigSnapshot probe concurrency", () => {
  it("keeps the configured stage3 download concurrency", () => {
    const snapshot = normalizeConfigSnapshot({ probe: { concurrency: { stage3: 6 } } });
    expect(snapshot.probe.concurrency.stage3).toBe(6);
  });

  it("clamps stage3 download concurrency to 1-16", () => {
    expect(normalizeConfigSnapshot({ probe: { concurrency: { stage3: 0 } } }).probe.concurrency.stage3).toBe(1);
    expect(normalizeConfigSnapshot({ probe: { concurrency: { stage3: 99 } } }).probe.concurrency.stage3).toBe(16);
  });
});
