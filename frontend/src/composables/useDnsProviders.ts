// DNS 提供商设置绑定（重构计划 §4.2）：水合、主 Cloudflare 派生、可用性过滤。
// 从 App.vue 原样迁出；纯表单转换复用 lib/dnsProviderForm。
import { ref } from "vue";
import type { ConfigSnapshot } from "../lib/bridge";
import { dnsProviderFormComplete, dnsProviderFormFromSnapshot } from "../lib/dnsProviderForm";
import type { DnsProviderForm, SettingsForm } from "../lib/settingsForm";

export interface DnsProvidersDeps {
  settings: SettingsForm;
}

export function useDnsProviders(deps: DnsProvidersDeps) {
  const { settings } = deps;
  const maskedTokenHint = ref("");

  function primaryCloudflareProvider(): DnsProviderForm | undefined {
    return settings.dnsProviders.find((entry) => entry.provider === "cloudflare");
  }

  // 旧字段（记录名、Token 脱敏提示、分流规则计数）仍被结果页和设置页摘要读取，
  // 统一从主 Cloudflare 平台派生，避免两套状态互相覆盖。
  function syncCloudflareLegacyMirror() {
    maskedTokenHint.value = primaryCloudflareProvider()?.maskedToken ?? "";
  }

  function applyDNSProvidersFromSnapshot(normalized: ConfigSnapshot) {
    settings.dnsProviders = normalized.dns_providers.map((item, index) => dnsProviderFormFromSnapshot(item, index));
    syncCloudflareLegacyMirror();
  }

  function enabledDNSProviders() {
    return settings.dnsProviders.filter((entry) => entry.enabled && dnsProviderFormComplete(entry));
  }

  return {
    maskedTokenHint,
    applyDNSProvidersFromSnapshot,
    primaryCloudflareProvider,
    syncCloudflareLegacyMirror,
    enabledDNSProviders,
  };
}