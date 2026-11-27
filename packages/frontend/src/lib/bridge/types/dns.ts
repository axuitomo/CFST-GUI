import type { DNSProviderKind } from "./enums";

export interface CloudflareRoutingRuleSnapshot {
  enabled: boolean;
  filter_mode: "allow" | "deny";
  filter_tokens: string;
  name: string;
  record_name: string;
  record_type: "A" | "AAAA" | "ALL";
  top_n: number;
}

export interface DNSSubdomainSnapshot {
  full_domain: string;
  id: number;
  rootdomain: string;
  status: string;
  subdomain: string;
}

export interface DNSSubdomainListResult {
  count: number;
  provider: string;
  provider_id: string;
  subdomains: DNSSubdomainSnapshot[];
}

export interface DNSProviderSnapshot {
  allow_cname?: boolean;
  api_key?: string;
  api_secret?: string;
  api_token?: string;
  enabled: boolean;
  id: string;
  name: string;
  provider: DNSProviderKind;
  record_name: string;
  record_type: "A" | "AAAA" | "ALL";
  routing_enabled?: boolean;
  routing_rules?: CloudflareRoutingRuleSnapshot[];
  subdomain?: string;
  subdomain_id?: number;
  top_n: number;
  ttl: number;
  zone_id?: string;
}

export interface DnsRecordSnapshot {
  comment: string;
  content: string;
  created_on?: string;
  id: string;
  modified_on?: string;
  name: string;
  proxied: boolean;
  ttl: number;
  type: string;
}

export interface DnsSelfTestStage {
  details?: Record<string, unknown>;
  duration_ms: number;
  id: string;
  message: string;
  status: "passed" | "failed" | "skipped";
  title: string;
}

export interface DnsSelfTestResult {
  alt_ttl?: number;
  base_ttl?: number;
  cleanup_ok: boolean;
  failed: number;
  passed: number;
  provider: string;
  provider_id: string;
  record_name: string;
  skipped: number;
  stages: DnsSelfTestStage[];
  test_name: string;
  warnings: string[];
  wrote_records: boolean;
}

export interface DnsReadProviderOption {
  enabled: boolean;
  id: string;
  label: string;
  provider: string;
  recordName: string;
  status: string;
}

export interface DnsSelfTestProviderOption {
  id: string;
  label: string;
  provider: string;
  recordName: string;
  zoneName: string;
}
