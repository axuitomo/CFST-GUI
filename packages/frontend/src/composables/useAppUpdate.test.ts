import { beforeEach, describe, expect, it, vi } from "vitest";

const { checkForUpdates, downloadAndInstallUpdate, getAppInfo, openReleasePage } = vi.hoisted(() => ({ checkForUpdates: vi.fn(), downloadAndInstallUpdate: vi.fn(), getAppInfo: vi.fn(), openReleasePage: vi.fn() }));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, checkForUpdates, downloadAndInstallUpdate, getAppInfo, openReleasePage };
});

import { useAppUpdate, type AppUpdateDeps } from "./useAppUpdate";

function makeDeps() {
  const deps: AppUpdateDeps = { appendLog: vi.fn(), showToast: vi.fn() };
  return deps;
}

describe("useAppUpdate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getAppInfo.mockResolvedValue({ ok: true, message: "", warnings: [], data: { current_version: "2.0", platform: "android", release_url: "https://rel" } });
    checkForUpdates.mockResolvedValue({ ok: true, message: "", warnings: [], data: { latest_version: "2.1", update_available: true } });
    downloadAndInstallUpdate.mockResolvedValue({ ok: true, message: "", warnings: [], data: { downloaded_path: "C:/up.zip", install_started: true } });
    openReleasePage.mockResolvedValue({ ok: true, message: "", warnings: [], data: null });
  });

  it("refreshAppInfo：回填版本与平台，isAndroidApp 派生", async () => {
    const api = useAppUpdate(makeDeps());
    expect(api.isAndroidApp.value).toBe(false);
    await api.refreshAppInfo();
    expect(api.appInfo.value.current_version).toBe("2.0");
    expect(api.isAndroidApp.value).toBe(true);
    expect(api.updateState.releaseUrl).toBe("https://rel");
  });

  it("checkOnlineUpdate：发现新版本走 available 与 info 提示", async () => {
    const deps = makeDeps();
    const api = useAppUpdate(deps);
    await api.checkOnlineUpdate();
    expect(api.updateState.status).toBe("available");
    expect(api.updateState.latestVersion).toBe("2.1");
    expect(deps.showToast).toHaveBeenCalledWith("发现新版本", "info");
  });

  it("checkOnlineUpdate：已是最新走 latest；失败走 failed", async () => {
    checkForUpdates.mockResolvedValue({ ok: true, message: "", warnings: [], data: { update_available: false } });
    const deps = makeDeps();
    const api = useAppUpdate(deps);
    await api.checkOnlineUpdate();
    expect(api.updateState.status).toBe("latest");
    expect(deps.showToast).toHaveBeenCalledWith("当前已是最新版本", "success");
    checkForUpdates.mockResolvedValue({ ok: false, message: "网络错误", warnings: [], data: null });
    await api.checkOnlineUpdate();
    expect(api.updateState.status).toBe("failed");
    expect(deps.showToast).toHaveBeenCalledWith("网络错误", "error");
  });

  it("installOnlineUpdate：记录下载路径并复位 installing", async () => {
    const deps = makeDeps();
    const api = useAppUpdate(deps);
    await api.installOnlineUpdate();
    expect(api.updateState.status).toBe("ready");
    expect(api.updateState.downloadPath).toBe("C:/up.zip");
    expect(api.updateState.installing).toBe(false);
    expect(deps.showToast).toHaveBeenCalledWith("更新安装流程已启动", "success");
  });

  it("openOnlineReleasePage：失败提示错误", async () => {
    openReleasePage.mockResolvedValue({ ok: false, message: "无浏览器", warnings: [], data: null });
    const deps = makeDeps();
    const api = useAppUpdate(deps);
    await api.openOnlineReleasePage();
    expect(deps.showToast).toHaveBeenCalledWith("无浏览器", "error");
  });
});
