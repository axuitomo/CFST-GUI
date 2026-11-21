// useDnsProviders 行为锁定（重构计划 §4.2）：水合镜像、主平台派生、可用性过滤。
import { describe, expect, it } from "vitest";
import { reactive } from "vue";
import { normalizeConfigSnapshot } from "../lib/bridge";
import { createDefaultSettingsForm } from "../lib/settingsForm";
import { useDnsProviders } from "./useDnsProviders";

function setup() {
  const settings = reactive(createDefaultSettingsForm());
  const api = useDnsProviders({ settings });
  return { settings, api };
}

describe("applyDNSProvidersFromSnapshot", () => {
  it("hydrates forms and mirrors masked token hint", () => {
    const { settings, api } = setup();
    api.applyDNSProvidersFromSnapshot(normalizeConfigSnapshot({
      dns_providers: [{ provider: "cloudflare", api_token: "tok***", zone_id: "z", record_name: "r.example", enabled: true }],
    }));
    expect(settings.dnsProviders).toHaveLength(1);
    expect(settings.dnsProviders[0].maskedToken).toBe("tok***");
    expect(api.maskedTokenHint.value).toBe("tok***");
    api.applyDNSProvidersFromSnapshot(normalizeConfigSnapshot({ dns_providers: [] }));
    expect(api.maskedTokenHint.value).toBe("");
  });
});

describe("primary / enabled", () => {
  it("primaryCloudflareProvider finds the cloudflare entry only", () => {
    const { settings, api } = setup();
    expect(api.primaryCloudflareProvider()).toBeUndefined();
    api.applyDNSProvidersFromSnapshot(normalizeConfigSnapshot({
      dns_providers: [
        { provider: "dnshe", api_key: "k", api_secret: "s", subdomain: "h.example.com", enabled: true },
        { provider: "cloudflare", api_token: "t", zone_id: "z", record_name: "r.example", enabled: true },
      ],
    }));
    expect(api.primaryCloudflareProvider()?.provider).toBe("cloudflare");
    const enabled = api.enabledDNSProviders();
    expect(enabled).toHaveLength(2);
    settings.dnsProviders[0].enabled = false;
    expect(api.enabledDNSProviders()).toHaveLength(1);
    settings.dnsProviders[1].apiToken = "";
    settings.dnsProviders[1].maskedToken = "";
    expect(api.enabledDNSProviders()).toHaveLength(0);
  });
});