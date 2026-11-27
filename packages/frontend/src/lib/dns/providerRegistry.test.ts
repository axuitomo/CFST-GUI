// DNS 平台描述表契约：新增平台必须补齐 DNSProviderKind 与 spec，否则这里失败。
import { describe, expect, it } from "vitest";
import { DNS_PROVIDER_KINDS, dnsProviderSpec } from "./providerRegistry";

describe("dnsProviderRegistry", () => {
  it("describes every kind with a usable spec", () => {
    expect(DNS_PROVIDER_KINDS).toEqual(["cloudflare", "dnshe", "aliyun", "dnspod"]);
    for (const kind of DNS_PROVIDER_KINDS) {
      const spec = dnsProviderSpec(kind);
      expect(spec.label.length).toBeGreaterThan(0);
      expect(spec.defaultTTL).toBeGreaterThan(0);
      expect(typeof spec.complete).toBe("function");
      expect(spec.credentials.idLabel.length).toBeGreaterThan(0);
    }
  });

  it("keeps platform-specific switches on the spec", () => {
    expect(dnsProviderSpec("cloudflare").tokenOnly).toBe(true);
    expect(dnsProviderSpec("cloudflare").supportsRecordTypeAll).toBe(true);
    expect(dnsProviderSpec("dnshe").supportsSubdomain).toBe(true);
    for (const kind of ["dnshe", "aliyun", "dnspod"] as const) {
      expect(dnsProviderSpec(kind).tokenOnly).toBe(false);
      expect(dnsProviderSpec(kind).supportsRecordTypeAll).toBe(false);
      expect(dnsProviderSpec(kind).recordNameHint.length).toBeGreaterThan(0);
    }
  });
});
