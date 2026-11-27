import { invokeCore } from "../invoke";
import type { DNSSubdomainListResult, DnsSelfTestResult } from "../types";

export async function testDoH(payload: Record<string, unknown>) {
  return invokeCore("doh.test", payload);
}

export async function listDnsRecords(payload: Record<string, unknown>) {
  return invokeCore("cloudflare.list", payload);
}

export async function listDnsSubdomains(payload: Record<string, unknown>) {
  return invokeCore<DNSSubdomainListResult>("dns.subdomains", payload);
}

export async function runDnsSelfTest(payload: Record<string, unknown>) {
  return invokeCore<DnsSelfTestResult>("dns.selftest", payload);
}

export async function pushDnsRecords(payload: Record<string, unknown>) {
  return invokeCore("cloudflare.push", payload);
}
