// DNS 平台描述表：平台名、默认 TTL、凭据文案、完成判定只在这里定义。
// 新增平台 = 一个 spec 条目 + DNSProviderKind 补值；SettingsView 与
// dnsProviderForm 都从这里读，不再各自 switch。
import type { DNSProviderKind } from "./bridge";
import { DEFAULT_ALIYUN_TTL, DEFAULT_CLOUDFLARE_TTL, DEFAULT_DNSHE_TTL, DEFAULT_DNSPOD_TTL, type DnsProviderForm } from "./settingsForm";

export interface DnsProviderCredentials {
  idLabel: string;
  idPlaceholder: string;
  secretHint: string;
  secretLabel: string;
}

export interface DnsProviderSpec {
  complete: (form: DnsProviderForm) => boolean;
  credentials: DnsProviderCredentials;
  defaultTTL: number;
  label: string;
  recordNameHint: string;
  supportsRecordTypeAll: boolean;
  supportsSubdomain: boolean;
  /** true = 只填 API Token + Zone ID；false = API Key + API Secret。 */
  tokenOnly: boolean;
}

/** 新增平台按钮的顺序。 */
export const DNS_PROVIDER_KINDS: DNSProviderKind[] = ["cloudflare", "dnshe", "aliyun", "dnspod"];

function hasRoutingTarget(entry: DnsProviderForm) {
  return entry.routingEnabled && entry.routingRules.some((rule) => rule.enabled && rule.recordName.trim());
}

function tokenFormComplete(entry: DnsProviderForm) {
  return Boolean((entry.apiToken.trim() || entry.maskedToken) && entry.zoneId.trim() && (entry.recordName.trim() || hasRoutingTarget(entry)));
}

function keySecretFormComplete(entry: DnsProviderForm) {
  return Boolean((entry.apiKey.trim() || entry.maskedKey) && (entry.apiSecret.trim() || entry.maskedSecret) && (entry.recordName.trim() || hasRoutingTarget(entry) || entry.subdomain.trim()));
}

const DNS_PROVIDER_SPECS: Record<DNSProviderKind, DnsProviderSpec> = {
  cloudflare: {
    complete: tokenFormComplete,
    credentials: {
      idLabel: "API Token",
      idPlaceholder: "重新输入完整 Token 以保存",
      secretHint: "重新输入完整 Token 以保存",
      secretLabel: "API Token",
    },
    defaultTTL: DEFAULT_CLOUDFLARE_TTL,
    label: "Cloudflare",
    recordNameHint: "",
    supportsRecordTypeAll: true,
    supportsSubdomain: false,
    tokenOnly: true,
  },
  dnshe: {
    complete: keySecretFormComplete,
    credentials: {
      idLabel: "API Key",
      idPlaceholder: "cfsd_xxxxxxxxxx",
      secretHint: "创建后只显示一次",
      secretLabel: "API Secret",
    },
    defaultTTL: DEFAULT_DNSHE_TTL,
    label: "DNSHE",
    recordNameHint: "填完整域名（如 test.example.com）；也可只填上级域名，再在下方子域名填 test，程序会解析为 test.<完整域名>。",
    supportsRecordTypeAll: false,
    supportsSubdomain: true,
    tokenOnly: false,
  },
  aliyun: {
    complete: keySecretFormComplete,
    credentials: {
      idLabel: "AccessKey ID",
      idPlaceholder: "LTAI5t...",
      secretHint: "控制台 AccessKey Secret",
      secretLabel: "AccessKey Secret",
    },
    defaultTTL: DEFAULT_ALIYUN_TTL,
    label: "阿里云 DNS",
    recordNameHint: "填完整域名，程序按阿里云云解析域名列表自动匹配所属域名。",
    supportsRecordTypeAll: false,
    supportsSubdomain: false,
    tokenOnly: false,
  },
  dnspod: {
    complete: keySecretFormComplete,
    credentials: {
      idLabel: "SecretId",
      idPlaceholder: "AKID...",
      secretHint: "腾讯云 API 密钥 SecretKey",
      secretLabel: "SecretKey",
    },
    defaultTTL: DEFAULT_DNSPOD_TTL,
    label: "DNSPod",
    recordNameHint: "填完整域名，程序按 DNSPod 域名列表自动匹配所属域名。",
    supportsRecordTypeAll: false,
    supportsSubdomain: false,
    tokenOnly: false,
  },
};

export function dnsProviderSpec(provider: DNSProviderKind): DnsProviderSpec {
  return DNS_PROVIDER_SPECS[provider] ?? DNS_PROVIDER_SPECS.cloudflare;
}

export function dnsProviderPlatformLabel(provider: DNSProviderKind) {
  return dnsProviderSpec(provider).label;
}

export function dnsProviderDefaultTTL(provider: DNSProviderKind) {
  return dnsProviderSpec(provider).defaultTTL;
}

export function dnsProviderCredentialLabels(provider: DNSProviderKind) {
  return dnsProviderSpec(provider).credentials;
}

export function dnsProviderRecordNameHint(provider: DNSProviderKind) {
  return dnsProviderSpec(provider).recordNameHint;
}
