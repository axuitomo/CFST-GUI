// Cloudflare 记录与 DNS 平台快照的归一化。
import { nonNegativeInteger, toBoolean, toInteger, toObjectArray, toStringValue } from "../../bridgeValues";
import type { CloudflareRoutingRuleSnapshot, DNSProviderKind, DNSProviderSnapshot } from "../types";
import { DEFAULT_ALIYUN_TTL, DEFAULT_CLOUDFLARE_TTL, DEFAULT_CLOUDFLARE_UPLOAD_TOP_N, DEFAULT_DNSHE_TTL, DEFAULT_DNSPOD_TTL } from "./defaults";

export function normalizeCloudflareTTL(value: unknown) {
  const ttl = toInteger(value, DEFAULT_CLOUDFLARE_TTL);
  return [60, 300, 600].includes(ttl) ? ttl : DEFAULT_CLOUDFLARE_TTL;
}

export function normalizeCloudflareRecordType(value: unknown): "A" | "AAAA" | "ALL" {
  const normalized = toStringValue(value).trim().toUpperCase();
  if (normalized === "ALL" || normalized === "AAAA") {
    return normalized;
  }
  return "A";
}

export function normalizeDNSProviderKind(value: unknown): DNSProviderKind {
  switch (toStringValue(value).trim().toLowerCase()) {
    case "dnshe":
      return "dnshe";
    case "aliyun":
    case "alidns":
    case "ali":
      return "aliyun";
    case "dnspod":
    case "dndps":
    case "tencent":
    case "tencentcloud":
      return "dnspod";
    default:
      return "cloudflare";
  }
}

function normalizePlatformTTL(value: unknown, fallback: number) {
  const ttl = toInteger(value, fallback);
  return ttl >= 60 ? ttl : fallback;
}

function normalizeDNSProviderTTL(provider: DNSProviderKind, value: unknown) {
  if (provider === "dnshe") {
    const ttl = toInteger(value, DEFAULT_DNSHE_TTL);
    return ttl > 0 ? ttl : DEFAULT_DNSHE_TTL;
  }
  if (provider === "aliyun") {
    return normalizePlatformTTL(value, DEFAULT_ALIYUN_TTL);
  }
  if (provider === "dnspod") {
    return normalizePlatformTTL(value, DEFAULT_DNSPOD_TTL);
  }
  return normalizeCloudflareTTL(value);
}

export function normalizeDNSProviders(input: unknown, cloudflare: Record<string, unknown>): DNSProviderSnapshot[] {
  const entries = toObjectArray(input);
  const providers = entries.map((item, index) => normalizeDNSProvider(item, index));
  if (providers.length > 0) {
    return providers;
  }
  // 旧配置只有顶层 cloudflare 段：把它当作一条 Cloudflare 平台。
  return [
    {
      allow_cname: toBoolean(cloudflare.allow_cname ?? cloudflare.allowCNAME ?? cloudflare.allow_modify_cname ?? cloudflare.allowModifyCNAME, false),
      api_token: toStringValue(cloudflare.api_token),
      enabled: toBoolean(cloudflare.enabled ?? cloudflare.cloudflare_enabled ?? cloudflare.cloudflareEnabled, false),
      id: "cloudflare",
      name: "Cloudflare",
      provider: "cloudflare",
      record_name: toStringValue(cloudflare.record_name),
      record_type: normalizeCloudflareRecordType(cloudflare.record_type),
      routing_enabled: toBoolean(cloudflare.routing_enabled ?? cloudflare.routingEnabled, false),
      routing_rules: normalizeCloudflareRoutingRules(cloudflare.routing_rules ?? cloudflare.routingRules),
      top_n: nonNegativeInteger(cloudflare.top_n ?? cloudflare.topN, DEFAULT_CLOUDFLARE_UPLOAD_TOP_N),
      ttl: normalizeCloudflareTTL(cloudflare.ttl),
      zone_id: toStringValue(cloudflare.zone_id),
    },
  ];
}

function normalizeDNSProvider(item: Record<string, unknown>, index: number): DNSProviderSnapshot {
  const provider = normalizeDNSProviderKind(item.provider ?? item.platform ?? item.type);
  const snapshot: DNSProviderSnapshot = {
    allow_cname: toBoolean(item.allow_cname ?? item.allowCname ?? item.allowCNAME ?? item.allow_modify_cname ?? item.allowModifyCNAME, false),
    enabled: toBoolean(item.enabled ?? item.enable, true),
    id: toStringValue(item.id ?? item.provider_id ?? item.providerId).trim() || `${provider}-${index + 1}`,
    name: toStringValue(item.name ?? item.label).trim(),
    provider,
    record_name: toStringValue(item.record_name ?? item.recordName).trim(),
    record_type: normalizeCloudflareRecordType(item.record_type ?? item.recordType),
    top_n: nonNegativeInteger(item.top_n ?? item.topN, DEFAULT_CLOUDFLARE_UPLOAD_TOP_N),
    ttl: normalizeDNSProviderTTL(provider, item.ttl),
  };
  if (provider !== "cloudflare") {
    snapshot.api_key = toStringValue(item.api_key ?? item.apiKey);
    snapshot.api_secret = toStringValue(item.api_secret ?? item.apiSecret);
    snapshot.routing_enabled = toBoolean(item.routing_enabled ?? item.routingEnabled, false);
    snapshot.routing_rules = normalizeCloudflareRoutingRules(item.routing_rules ?? item.routingRules);
    snapshot.subdomain = toStringValue(item.subdomain ?? item.subdomainName).trim();
    if (provider === "dnshe") {
      snapshot.subdomain_id = nonNegativeInteger(item.subdomain_id ?? item.subdomainId, 0);
    }
    return snapshot;
  }
  snapshot.api_token = toStringValue(item.api_token ?? item.apiToken);
  snapshot.zone_id = toStringValue(item.zone_id ?? item.zoneId);
  snapshot.routing_enabled = toBoolean(item.routing_enabled ?? item.routingEnabled, false);
  snapshot.routing_rules = normalizeCloudflareRoutingRules(item.routing_rules ?? item.routingRules);
  return snapshot;
}

export function normalizeCloudflareRoutingRules(input: unknown): CloudflareRoutingRuleSnapshot[] {
  return toObjectArray(input).map((item) => ({
    enabled: toBoolean(item.enabled, true),
    filter_mode: toStringValue(item.filter_mode ?? item.filterMode).toLowerCase() === "deny" ? "deny" : "allow",
    filter_tokens: toStringValue(item.filter_tokens ?? item.filterTokens),
    name: toStringValue(item.name),
    record_name: toStringValue(item.record_name ?? item.recordName),
    record_type: normalizeCloudflareRecordType(item.record_type ?? item.recordType),
    top_n: nonNegativeInteger(item.top_n ?? item.topN, DEFAULT_CLOUDFLARE_UPLOAD_TOP_N),
  }));
}
