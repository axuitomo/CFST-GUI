import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";

const { runDnsSelfTest, testDoH } = vi.hoisted(() => ({ runDnsSelfTest: vi.fn(), testDoH: vi.fn() }));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, runDnsSelfTest, testDoH };
});

import { createDnsProviderForm } from "../lib/dns/providerForm";
import { createDefaultSettingsForm, type DnsProviderForm } from "../lib/config/settingsForm";
import { useDnsSelfTest, type DnsSelfTestDeps } from "./useDnsSelfTest";

function makeDeps() {
  const settings = createDefaultSettingsForm();
  const provider: DnsProviderForm = {
    ...createDnsProviderForm("cloudflare", 0),
    id: "cf-1",
    label: "主平台",
    apiToken: "tok",
    zoneId: "zone-1",
    recordName: "probe.example.com",
  };
  settings.dnsProviders = [provider];
  const probeWarnings = ref<string[]>([]);
  const deps: DnsSelfTestDeps = {
    settings,
    buildConfigSnapshot: () => ({}) as never,
    enabledDNSProviders: () => settings.dnsProviders.filter((entry) => entry.enabled),
    targetLabel: () => "example.com",
    resolvedRecordName: () => "probe.example.com",
    zoneName: () => "example.com",
    probeWarnings,
    pushWarningTrace: vi.fn(),
    appendLog: vi.fn(),
    showToast: vi.fn(),
    setStatus: vi.fn(),
    pushActivity: vi.fn(),
  };
  return { deps, probeWarnings };
}

describe("useDnsSelfTest", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    runDnsSelfTest.mockResolvedValue({ ok: true, message: "", warnings: ["w1"], data: { passed: 3, failed: 0, skipped: 1, stages: [{ id: "s1", title: "写入", status: "passed", message: "", duration_ms: 10, details: {} }] } });
    testDoH.mockResolvedValue({ ok: true, message: "DoH 可用。", warnings: [], data: null });
  });

  it("无启用平台时拒绝自检", async () => {
    const { deps } = makeDeps();
    deps.settings.dnsProviders = [];
    const api = useDnsSelfTest(deps);
    await api.runDNSSelfTest({ providerId: "cf-1", testName: "basic" });
    expect(runDnsSelfTest).not.toHaveBeenCalled();
    expect(deps.showToast).toHaveBeenCalledWith("请先在设置页添加并启用 DNS 推送平台", "error");
  });

  it("自检成功：skipped>0 走部分通过措辞，warnings 入 trace", async () => {
    const { deps, probeWarnings } = makeDeps();
    const api = useDnsSelfTest(deps);
    await api.runDNSSelfTest({ providerId: "cf-1", testName: "basic" });
    expect(api.dnsSelfTestResult.value?.passed).toBe(3);
    expect(api.dnsSelfTestResult.value?.stages[0].status).toBe("passed");
    expect(probeWarnings.value).toEqual(["w1"]);
    expect(deps.pushWarningTrace).toHaveBeenCalledWith(["w1"]);
    expect(deps.setStatus).toHaveBeenCalledWith(expect.objectContaining({ title: "Cloudflare 自检部分通过", tone: "completed" }));
    expect(api.dnsSelfTesting.value).toBe(false);
    // 自检平台下拉：label 回退平台名 + 目标。
    expect(api.dnsSelfTestProviders.value[0]).toMatchObject({ id: "cf-1", label: "主平台", recordName: "probe.example.com", zoneName: "example.com" });
  });

  it("自检失败：状态 failed + 错误 toast", async () => {
    runDnsSelfTest.mockResolvedValue({ ok: false, message: "token 无效", warnings: [], data: {} });
    const { deps } = makeDeps();
    const api = useDnsSelfTest(deps);
    await api.runDNSSelfTest({ providerId: "cf-1", testName: "basic" });
    expect(deps.setStatus).toHaveBeenCalledWith(expect.objectContaining({ title: "Cloudflare 自检失败", tone: "failed" }));
    expect(deps.showToast).toHaveBeenCalledWith("Cloudflare 自检失败", "error");
  });

  it("DoH 测试失败：结果与 toast 同步", async () => {
    testDoH.mockResolvedValue({ ok: false, message: "端点无响应", warnings: ["慢"], data: null });
    const { deps } = makeDeps();
    deps.settings.probeDoHURL = "https://doh.example/dns-query";
    deps.settings.probeURL = "https://probe.example/path";
    const api = useDnsSelfTest(deps);
    await api.testDohConnection();
    expect(testDoH).toHaveBeenCalledWith(expect.objectContaining({ doh_url: "https://doh.example/dns-query", host: "probe.example" }));
    expect(api.dohTestResult.value).toEqual({ message: "端点无响应 慢", ok: false });
    expect(deps.showToast).toHaveBeenCalledWith("DoH 测试失败", "error");
    expect(api.dohTesting.value).toBe(false);
  });
});
