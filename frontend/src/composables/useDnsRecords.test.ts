import { beforeEach, describe, expect, it, vi } from "vitest";

const { listDnsRecords, listDnsSubdomains } = vi.hoisted(() => ({ listDnsRecords: vi.fn(), listDnsSubdomains: vi.fn() }));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, listDnsRecords, listDnsSubdomains };
});

import { createDnsProviderForm } from "../lib/dnsProviderForm";
import { createDefaultSettingsForm } from "../lib/settingsForm";
import { useDnsRecords, type DnsRecordsDeps } from "./useDnsRecords";

function makeDeps(overrides: Partial<DnsRecordsDeps> = {}) {
  const settings = createDefaultSettingsForm();
  settings.dnsProviders = [
    {
      ...createDnsProviderForm("cloudflare", 0),
      id: "cf-1",
      label: "",
      apiToken: "tok",
      zoneId: "zone-1",
      recordName: "probe.example.com",
    },
  ];
  return {
    deps: {
      settings,
      buildConfigSnapshot: () => ({}) as never,
      appendLog: vi.fn(),
      showToast: vi.fn(),
      setStatus: vi.fn(),
      pushActivity: vi.fn(),
      navigateTo: vi.fn(),
      selectedView: () => "dns",
      ...overrides,
    } as DnsRecordsDeps,
    settings,
  };
}

describe("useDnsRecords", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listDnsRecords.mockResolvedValue({ ok: true, message: "", warnings: [], data: { records: [{ name: "a.example.com", type: "A", value: "1.2.3.4", ttl: 60 }] } });
    listDnsSubdomains.mockResolvedValue({ ok: true, message: "", warnings: [], data: { subdomains: [] } });
  });

  it("custom 作用域空名称：跳转 DNS 页并提示，不发请求", async () => {
    const { deps } = makeDeps();
    const api = useDnsRecords(deps);
    api.dnsReadScope.value = "custom";
    api.dnsReadName.value = "  ";
    await api.fetchDnsRecords();
    expect(deps.navigateTo).toHaveBeenCalledWith("dns");
    expect(listDnsRecords).not.toHaveBeenCalled();
  });

  it("读取成功：payload 条件字段 + 记录归一 + 条数提示", async () => {
    const { deps } = makeDeps();
    const api = useDnsRecords(deps);
    api.dnsReadScope.value = "custom";
    api.dnsReadName.value = " probe.example.com ";
    api.dnsRecordType.value = "A";
    await api.fetchDnsRecords();
    const payload = listDnsRecords.mock.calls[0][0];
    expect(payload.scope).toBe("custom");
    expect(payload.name).toBe("probe.example.com");
    expect(payload.record_type).toBe("A");
    expect(payload.provider_id).toBe("cf-1"); // watch 回退选中唯一平台
    expect(api.dnsRecords.value.length).toBe(1);
    expect(deps.showToast).toHaveBeenCalledWith("已同步 1 条记录", "info");
    expect(api.isLoadingDns.value).toBe(false);
  });

  it("非 dnshe 平台不触发子域名读取", async () => {
    const { deps } = makeDeps();
    const api = useDnsRecords(deps);
    await api.loadDnsProviderSubdomains("cf-1", true);
    expect(listDnsSubdomains).not.toHaveBeenCalled();
  });

  it("DNSHE 子域名最长根域匹配解析记录名", () => {
    const { deps, settings } = makeDeps();
    settings.dnsProviders = [{ ...createDnsProviderForm("dnshe", 0), id: "he-1", label: "", apiToken: "tok", subdomain: "free.hr", recordName: "ignored" }];
    const api = useDnsRecords(deps);
    api.dnsSubdomainsByProvider.value = {
      "he-1": [
        { full_domain: "a.free.hr", id: 1, rootdomain: "free.hr", status: "active", subdomain: "a" },
        { full_domain: "b.free.hr", id: 2, rootdomain: "free.hr", status: "active", subdomain: "b" },
      ],
    };
    const entry = settings.dnsProviders[0];
    expect(api.dnsProviderZoneName(entry)).toBe("free.hr");
    // 子域名已是完整域名时，记录名以表单 recordName 优先。
    expect(api.dnsProviderResolvedRecordName(entry)).toBe("ignored");
    // DNSHE 无子域名数据时同样回退表单值。
    api.dnsSubdomainsByProvider.value = {};
    expect(api.dnsProviderResolvedRecordName(entry)).toBe("ignored");
  });

  it("读取平台 watch：当前选中失效时回退到首个已就绪平台", () => {
    const { deps, settings } = makeDeps();
    settings.dnsProviders.push({ ...settings.dnsProviders[0], id: "cf-2", enabled: false });
    const api = useDnsRecords(deps);
    expect(api.dnsReadProviderId.value).toBe("cf-1");
  });
});
