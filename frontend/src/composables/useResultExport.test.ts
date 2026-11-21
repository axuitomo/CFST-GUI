import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick, reactive, ref } from "vue";

function stubBrowserStorage() {
  const store = new Map<string, string>();
  vi.stubGlobal("window", {
    localStorage: {
      getItem: (key: string) => store.get(key) ?? null,
      removeItem: (key: string) => store.delete(key),
      setItem: (key: string, value: string) => {
        store.set(key, value);
      },
    },
  });
  return store;
}

const mocks = vi.hoisted(() => ({
  exportResultsCSV: vi.fn(),
  exportResultsToGitHub: vi.fn(),
  pushDnsRecords: vi.fn(),
  testGitHubExport: vi.fn(),
  testTelegramNotification: vi.fn(),
  openPath: vi.fn(),
}));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, ...mocks };
});

import type { ProbeResult } from "../lib/bridge";
import type { HistoryEntry } from "../lib/appTypes";
import { createDnsProviderForm } from "../lib/dnsProviderForm";
import { createDefaultSettingsForm } from "../lib/settingsForm";
import { useResultExport, type ResultExportDeps } from "./useResultExport";

const row = (address: string) => ({ address, export_status: "", stage_status: "" }) as ProbeResult;
const hist = (taskId: string, updatedAt: string): HistoryEntry => ({ detail: "", exported: 0, failureSummary: "", targetPath: "", taskId, title: "", tone: "completed", updatedAt });

function makeDeps() {
  const settings = createDefaultSettingsForm();
  settings.dnsProviders = [{ ...createDnsProviderForm("cloudflare", 0), id: "cf-1", label: "", apiToken: "tok", zoneId: "zone-1", recordName: "probe.example.com", topN: 7 }];
  const rows = [row("1.1.1.1"), row("2.2.2.2"), row("3.3.3.3")];
  const deps: ResultExportDeps = {
    settings,
    task: reactive({ exportPath: "C:/out/result.csv", taskId: "task-1" }),
    isAndroidApp: () => false,
    enabledDNSProviders: () => settings.dnsProviders.filter((entry) => entry.enabled),
    primaryCloudflareProvider: () => settings.dnsProviders.find((entry) => entry.provider === "cloudflare"),
    dnsProviderTargetLabel: (entry) => entry.recordName,
    buildConfigSnapshot: () => ({}) as never,
    currentVisibleResultRows: () => [...rows],
    pathLeafName: (rawPath) => rawPath.split("/").pop() || "",
    probeWarnings: ref<string[]>([]),
    pushWarningTrace: vi.fn(),
    navigateTo: vi.fn(),
    appendLog: vi.fn(),
    showToast: vi.fn(),
    setStatus: vi.fn(),
    pushActivity: vi.fn(),
  };
  return { deps, settings, rows };
}

describe("useResultExport", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    stubBrowserStorage();
    mocks.exportResultsToGitHub.mockResolvedValue({ ok: true, message: "", warnings: [], data: { exported_at: "2026-11-21T00:00:00Z", path: "results.csv", html_url: "https://github.com/x" } });
    mocks.exportResultsCSV.mockResolvedValue({ ok: true, message: "", warnings: [], data: { written_count: 3, path: "C:/out/result.csv" } });
    mocks.pushDnsRecords.mockResolvedValue({ ok: true, message: "", warnings: [], data: { providers: [{ summary: { created: 2, updated: 1, deleted: 0, ignored: 0 }, upload_count: 3 }] } });
    mocks.testGitHubExport.mockResolvedValue({ ok: true, message: "", warnings: [], data: null });
    mocks.testTelegramNotification.mockResolvedValue({ ok: true, message: "", warnings: [], data: null });
  });

  it("推送偏好：无缓存时回退主平台默认值，保存后可回读", async () => {
    const { deps } = makeDeps();
    const api = useResultExport(deps);
    api.hydrateResultPushSettings();
    expect(api.resultCloudflarePushSettings.providerId).toBe("cf-1");
    expect(api.resultCloudflarePushSettings.recordName).toBe("probe.example.com");
    expect(api.resultCloudflarePushSettings.topN).toBe(7);
    expect(api.resultGitHubTopN.value).toBe(20);
    api.updateResultCloudflarePushSettings({ recordName: " a.example.com ", topN: 3 });
    api.updateResultGitHubTopN(-5);
    expect(api.resultGitHubTopN.value).toBe(0);
    await nextTick(); // deep watch 异步落盘
    const fresh = useResultExport(makeDeps().deps);
    fresh.hydrateResultPushSettings();
    expect(fresh.resultCloudflarePushSettings.recordName).toBe("a.example.com");
    expect(fresh.resultCloudflarePushSettings.topN).toBe(3);
    expect(fresh.resultGitHubTopN.value).toBe(0);
  });

  it("limitRowsForQuickPush：topN=0 全量，正数截断", () => {
    const { deps, rows } = makeDeps();
    const api = useResultExport(deps);
    expect(api.limitRowsForQuickPush(rows, 0)).toHaveLength(3);
    expect(api.limitRowsForQuickPush(rows, 2)).toHaveLength(2);
  });

  it("导出历史：按任务号去重、时间倒序、上限 50", () => {
    const { deps } = makeDeps();
    const api = useResultExport(deps);
    api.updateHistory(hist("t1", "2026-11-20T00:00:00Z"));
    api.updateHistory(hist("t2", "2026-11-21T00:00:00Z"));
    api.updateHistory(hist("t1", "2026-11-22T00:00:00Z"));
    expect(api.exportHistory.value.map((entry) => entry.taskId)).toEqual(["t1", "t2"]);
    for (let i = 0; i < 60; i += 1) {
      api.updateHistory(hist(`bulk-${i}`, `2026-12-${String(i + 1).padStart(2, "0")}T00:00:00Z`));
    }
    expect(api.exportHistory.value.length).toBe(50);
  });

  it("GitHub 导出：空结果拦截；成功写入最近导出时间", async () => {
    const { deps, settings, rows } = makeDeps();
    const api = useResultExport(deps);
    rows.splice(0, rows.length);
    await api.exportCurrentResultsToGitHub();
    expect(mocks.exportResultsToGitHub).not.toHaveBeenCalled();
    expect(deps.navigateTo).toHaveBeenCalledWith("results");
    rows.push(row("1.1.1.1"));
    await api.exportCurrentResultsToGitHub();
    expect(settings.githubLastExportAt).toBe("2026-11-21T00:00:00Z");
    expect(deps.showToast).toHaveBeenCalledWith("已导出到 GitHub", "success");
    expect(api.githubExporting.value).toBe(false);
  });

  it("Cloudflare 推送：无启用平台跳转设置页；成功覆盖记录名与 topN", async () => {
    const { deps, settings } = makeDeps();
    const api = useResultExport(deps);
    settings.dnsProviders = [];
    await api.pushCurrentResultsToCloudflare();
    expect(deps.navigateTo).toHaveBeenCalledWith("settings");
    expect(mocks.pushDnsRecords).not.toHaveBeenCalled();
    settings.dnsProviders = [{ ...createDnsProviderForm("cloudflare", 0), id: "cf-1", label: "", apiToken: "tok", zoneId: "zone-1", recordName: "probe.example.com" }];
    api.hydrateResultPushSettings();
    await api.pushCurrentResultsToCloudflare();
    const payload = mocks.pushDnsRecords.mock.calls[0][0];
    expect(payload.provider_id).toBe("cf-1");
    expect(payload.config.cloudflare.record_name).toBe("probe.example.com");
    expect(payload.config.cloudflare.top_n).toBe(5);
    expect(deps.setStatus).toHaveBeenCalledWith(expect.objectContaining({ title: "Cloudflare 推送完成", tone: "completed" }));
    expect(deps.showToast).toHaveBeenCalledWith("已推送到 Cloudflare", "success");
    expect(api.cloudflarePushing.value).toBe(false);
  });

  it("CSV 导出：Android 未选 SAF 目录时拦截", async () => {
    const { deps } = makeDeps();
    deps.isAndroidApp = () => true;
    const api = useResultExport(deps);
    await api.exportCurrentResultsCSV();
    expect(mocks.exportResultsCSV).not.toHaveBeenCalled();
    expect(deps.showToast).toHaveBeenCalledWith("请先选择 Android SAF 导出目录", "error");
    expect(api.csvExporting.value).toBe(false);
  });

  it("GitHub 连通性测试：失败走错误 toast 与动态", async () => {
    mocks.testGitHubExport.mockResolvedValue({ ok: false, message: "401", warnings: [], data: null });
    const { deps } = makeDeps();
    const api = useResultExport(deps);
    await api.testGitHubExportSettings();
    expect(deps.showToast).toHaveBeenCalledWith("401", "error");
    expect(deps.pushActivity).toHaveBeenCalledWith("GitHub 导出测试失败", "401");
    expect(api.githubTesting.value).toBe(false);
  });

  it("推送平台下拉：label 为空时回退平台名与目标", () => {
    const { deps } = makeDeps();
    const api = useResultExport(deps);
    expect(api.dnsPushTargets.value[0]).toEqual({ id: "cf-1", label: "Cloudflare · probe.example.com", recordTarget: true });
  });
});
