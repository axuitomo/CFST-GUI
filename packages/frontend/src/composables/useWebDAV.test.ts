import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { testWebDAV, backupConfigToWebDAV, restoreConfigFromWebDAV } = vi.hoisted(() => ({ testWebDAV: vi.fn(), backupConfigToWebDAV: vi.fn(), restoreConfigFromWebDAV: vi.fn() }));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, testWebDAV, backupConfigToWebDAV, restoreConfigFromWebDAV };
});

import { createDefaultSettingsForm } from "../lib/config/settingsForm";
import { useWebDAV, type WebDAVDeps } from "./useWebDAV";

function makeDeps() {
  const settings = createDefaultSettingsForm();
  const deps: WebDAVDeps = {
    settings,
    buildConfigSnapshot: () => ({}) as never,
    applyImportedConfigData: vi.fn(),
    applyNavigation: vi.fn(),
    appendLog: vi.fn(),
    showToast: vi.fn(),
  };
  return { deps, settings };
}

describe("useWebDAV", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("window", { confirm: () => true });
    testWebDAV.mockResolvedValue({ ok: true, message: "连接正常", warnings: [], data: null });
    backupConfigToWebDAV.mockResolvedValue({ ok: true, message: "", warnings: [], data: null });
    restoreConfigFromWebDAV.mockResolvedValue({ ok: true, message: "", warnings: [], data: { config_snapshot: {} } });
  });

  it("连通性测试：按结果提示", async () => {
    const { deps } = makeDeps();
    const api = useWebDAV(deps);
    await api.testWebDAVSettings();
    expect(deps.showToast).toHaveBeenCalledWith("连接正常", "success");
    testWebDAV.mockResolvedValue({ ok: false, message: "401", warnings: [], data: null });
    await api.testWebDAVSettings();
    expect(deps.showToast).toHaveBeenCalledWith("401", "error");
  });

  it("备份：拒绝确认不请求；成功记录备份时间", async () => {
    vi.stubGlobal("window", { confirm: () => false });
    const { deps, settings } = makeDeps();
    const api = useWebDAV(deps);
    await api.backupToWebDAV();
    expect(backupConfigToWebDAV).not.toHaveBeenCalled();
    vi.stubGlobal("window", { confirm: () => true });
    await api.backupToWebDAV();
    expect(settings.webdavLastBackupAt).not.toBe("");
    expect(deps.showToast).toHaveBeenCalledWith("已备份到 WebDAV", "success");
  });

  it("还原：应用导入数据并跳转设置页", async () => {
    const { deps } = makeDeps();
    const api = useWebDAV(deps);
    await api.restoreFromWebDAV();
    expect(deps.applyImportedConfigData).toHaveBeenCalledWith({ config_snapshot: {} });
    expect(deps.applyNavigation).toHaveBeenCalledWith("settings");
    expect(deps.showToast).toHaveBeenCalledWith("已从 WebDAV 还原配置", "success");
  });
});
