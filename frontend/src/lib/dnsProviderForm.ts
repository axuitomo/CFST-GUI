// DNS 提供商表单 ↔ 快照纯转换（重构计划 §4.2/§4.3）：从 App.vue 原样迁出。
// 无状态依赖；SettingsView 的 DnsProviderCard 后续复用同一单点。
import { isMaskedTokenValue, type DNSProviderSnapshot } from "./bridge";
import {
  DEFAULT_ALIYUN_TTL,
  DEFAULT_CLOUDFLARE_TTL,
  DEFAULT_DNSHE_TTL,
  DEFAULT_DNSPOD_TTL,
  type CloudflareRoutingRuleForm,
  type DNSProviderKind,
  type DnsProviderForm,
} from "./settingsForm";
import { asCount, nonNegativeCount, normalizeCloudflareTTL } from "./valueCoerce";

export function dnsProviderDefaultTTL(provider: DNSProviderKind) {
  switch (provider) {
    case "dnshe":
      return DEFAULT_DNSHE_TTL;
    case "aliyun":
      return DEFAULT_ALIYUN_TTL;
    case "dnspod":
      return DEFAULT_DNSPOD_TTL;
    default:
      return DEFAULT_CLOUDFLARE_TTL;
  }
}

export function dnsProviderPlatformLabel(provider: DNSProviderKind) {
  switch (provider) {
    case "dnshe":
      return "DNSHE";
    case "aliyun":
      return "阿里云 DNS";
    case "dnspod":
      return "DNSPod";
    default:
      return "Cloudflare";
  }
}

export function createDnsProviderForm(provider: DNSProviderKind, index: number): DnsProviderForm {
  return {
    allowCNAME: false,
    apiKey: "",
    apiSecret: "",
    apiToken: "",
    enabled: true,
    id: `${provider}-${index + 1}`,
    label: dnsProviderPlatformLabel(provider),
    maskedKey: "",
    maskedSecret: "",
    maskedToken: "",
    provider,
    recordName: "",
    recordType: "A",
    routingEnabled: false,
    routingRules: [],
    subdomain: "",
    subdomainId: 0,
    topN: 5,
    ttl: dnsProviderDefaultTTL(provider),
    zoneId: "",
  };
}

export function dnsProviderFormFromSnapshot(item: DNSProviderSnapshot, index: number): DnsProviderForm {
  const form = createDnsProviderForm(item.provider, index);
  form.id = item.id || form.id;
  form.label = item.name?.trim() || dnsProviderPlatformLabel(item.provider);
  form.enabled = Boolean(item.enabled);
  form.allowCNAME = Boolean(item.allow_cname);
  form.recordName = item.record_name || "";
  form.recordType = item.record_type === "ALL" ? "ALL" : item.record_type === "AAAA" ? "AAAA" : "A";
  form.topN = nonNegativeCount(item.top_n, 5);
  form.ttl = item.provider === "cloudflare" ? normalizeCloudflareTTL(item.ttl) : Math.max(60, asCount(item.ttl, dnsProviderDefaultTTL(item.provider)));
  if (item.provider !== "cloudflare") {
    form.maskedKey = isMaskedTokenValue(item.api_key || "") ? item.api_key || "" : "";
    form.maskedSecret = isMaskedTokenValue(item.api_secret || "") ? item.api_secret || "" : "";
    form.apiKey = form.maskedKey ? "" : item.api_key || "";
    form.apiSecret = form.maskedSecret ? "" : item.api_secret || "";
    form.subdomain = (item.subdomain || "").trim();
    form.routingEnabled = Boolean(item.routing_enabled);
    form.routingRules = (item.routing_rules ?? []).map((rule, ruleIndex) => ({
      enabled: rule.enabled,
      filterMode: rule.filter_mode,
      filterTokens: rule.filter_tokens,
      id: `${form.id}-route-${ruleIndex}`,
      name: rule.name,
      recordName: rule.record_name,
      recordType: rule.record_type,
      topN: rule.top_n,
    }));
    if (item.provider === "dnshe") {
      form.subdomainId = form.subdomain ? 0 : nonNegativeCount(item.subdomain_id, 0);
    }
    return form;
  }
  form.maskedToken = isMaskedTokenValue(item.api_token || "") ? item.api_token || "" : "";
  form.apiToken = form.maskedToken ? "" : item.api_token || "";
  form.zoneId = item.zone_id || "";
  form.routingEnabled = Boolean(item.routing_enabled);
  form.routingRules = (item.routing_rules ?? []).map((rule, ruleIndex) => ({
    enabled: rule.enabled,
    filterMode: rule.filter_mode,
    filterTokens: rule.filter_tokens,
    id: `${form.id}-route-${ruleIndex}`,
    name: rule.name,
    recordName: rule.record_name,
    recordType: rule.record_type,
    topN: rule.top_n,
  }));
  return form;
}

export function dnsProviderFormComplete(entry: DnsProviderForm) {
  const routingTarget = entry.routingEnabled && entry.routingRules.some((rule) => rule.enabled && rule.recordName.trim());
  if (entry.provider !== "cloudflare") {
    const targetReady = Boolean(entry.recordName.trim() || routingTarget || entry.subdomain.trim());
    return Boolean((entry.apiKey.trim() || entry.maskedKey) && (entry.apiSecret.trim() || entry.maskedSecret) && targetReady);
  }
  return Boolean((entry.apiToken.trim() || entry.maskedToken) && entry.zoneId.trim() && (entry.recordName.trim() || routingTarget));
}

export function dnsRoutingRuleSnapshotFromForm(rule: CloudflareRoutingRuleForm) {
  return {
    enabled: Boolean(rule.enabled),
    filter_mode: rule.filterMode === "deny" ? ("deny" as const) : ("allow" as const),
    filter_tokens: rule.filterTokens.trim(),
    name: rule.name.trim(),
    record_name: rule.recordName.trim(),
    record_type: rule.recordType === "ALL" ? ("ALL" as const) : rule.recordType === "AAAA" ? ("AAAA" as const) : ("A" as const),
    top_n: nonNegativeCount(rule.topN, 5),
  };
}

export function dnsProviderSnapshotFromForm(entry: DnsProviderForm): DNSProviderSnapshot {
  if (entry.provider !== "cloudflare") {
    const subdomain = entry.subdomain.trim();
    return {
      ...(entry.apiKey.trim() ? { api_key: entry.apiKey.trim() } : {}),
      ...(entry.apiSecret.trim() ? { api_secret: entry.apiSecret.trim() } : {}),
      allow_cname: entry.allowCNAME,
      enabled: Boolean(entry.enabled && dnsProviderFormComplete(entry)),
      id: entry.id,
      name: entry.label.trim(),
      provider: entry.provider,
      record_name: entry.recordName.trim(),
      record_type: entry.recordType === "AAAA" ? "AAAA" : "A",
      routing_enabled: entry.routingEnabled,
      routing_rules: entry.routingRules.map(dnsRoutingRuleSnapshotFromForm),
      ...(entry.provider === "dnshe" ? (subdomain ? { subdomain } : { subdomain_id: nonNegativeCount(entry.subdomainId, 0) }) : {}),
      top_n: nonNegativeCount(entry.topN, 5),
      ttl: Math.max(60, asCount(entry.ttl, dnsProviderDefaultTTL(entry.provider))),
    };
  }
  return {
    ...(entry.apiToken.trim() ? { api_token: entry.apiToken.trim() } : {}),
    allow_cname: entry.allowCNAME,
    enabled: Boolean(entry.enabled && dnsProviderFormComplete(entry)),
    id: entry.id,
    name: entry.label.trim(),
    provider: "cloudflare",
    record_name: entry.recordName.trim(),
    record_type: entry.recordType,
    routing_enabled: entry.routingEnabled,
    routing_rules: entry.routingRules.map(dnsRoutingRuleSnapshotFromForm),
    top_n: nonNegativeCount(entry.topN, 5),
    ttl: normalizeCloudflareTTL(entry.ttl),
    zone_id: entry.zoneId.trim(),
  };
}
