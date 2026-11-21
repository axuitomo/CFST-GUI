// useAndroidRuntime 行为锁定（重构计划 §4.2）：
// 平台守卫、刷新成功/失败、设置返回去重、各操作的 toast 文案。
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAndroidRuntime, type AndroidRuntimeDeps } from "./useAndroidRuntime";

const { bridge } = vi.hoisted(() => ({
  bridge: {
    checkBatteryOptimization: vi.fn(),
    checkKeepAliveStatus: vi.fn(),
    checkNotificationPermission: vi.fn(),
    openBatteryOptimizationSettings: vi.fn(),
    openNotificationSettings: vi.fn(),
    requestNotificationPermission: vi.fn(),
    setKeepAliveEnabled: vi.fn(),
  },
}));
vi.mock("../lib/bridge", () => bridge);

function makeDeps(overrides: Partial<AndroidRuntimeDeps> = {}) {
  const logs: Array<{ event: string }> = [];
  const toasts: Array<{ message: string; tone?: string }> = [];
  const deps: AndroidRuntimeDeps = {
    appendLog: (event) => logs.push({ event }),
    isAndroidApp: () => true,
    showToast: (message, tone) => toasts.push({ message, tone }),
    ...overrides,
  };
  return { deps, logs, toasts };
}

beforeEach(() => {
  for (const fn of Object.values(bridge)) fn.mockReset();
  vi.stubGlobal("document", { visibilityState: "visible" });
});
afterEach(() => {
  vi.unstubAllGlobals();
});

describe("platform guard", () => {
  it("nulls statuses and skips bridge calls on non-Android", async () => {
    const { deps } = makeDeps({ isAndroidApp: () => false });
    const rt = useAndroidRuntime(deps);
    await rt.refreshAndroidBatteryStatus();
    await rt.refreshAndroidNotificationStatus();
    await rt.refreshAndroidKeepAliveStatus();
    expect(rt.androidBatteryStatus.value).toBeNull();
    expect(rt.androidNotificationStatus.value).toBeNull();
    expect(rt.androidKeepAliveStatus.value).toBeNull();
    expect(bridge.checkBatteryOptimization).not.toHaveBeenCalled();
  });
});

describe("refresh", () => {
  it("stores result.data on ok and logs the bridge event", async () => {
    bridge.checkBatteryOptimization.mockResolvedValue({ ok: true, data: { ignoring: true } });
    const { deps, logs } = makeDeps();
    const rt = useAndroidRuntime(deps);
    await rt.refreshAndroidBatteryStatus();
    expect(rt.androidBatteryStatus.value).toEqual({ ignoring: true });
    expect(logs.some((l) => l.event === "bridge.check_battery_optimization")).toBe(true);
  });

  it("keeps previous value when result not ok; logs failure on throw", async () => {
    bridge.checkKeepAliveStatus.mockResolvedValue({ ok: false, data: null });
    const { deps } = makeDeps();
    const rt = useAndroidRuntime(deps);
    rt.androidKeepAliveStatus.value = { enabled: true } as never;
    await rt.refreshAndroidKeepAliveStatus();
    expect(rt.androidKeepAliveStatus.value).toEqual({ enabled: true });

    bridge.checkKeepAliveStatus.mockRejectedValue(new Error("bridge down"));
    const { deps: deps2, logs: logs2 } = makeDeps();
    const rt2 = useAndroidRuntime(deps2);
    await rt2.refreshAndroidKeepAliveStatus();
    expect(logs2.some((l) => l.event === "bridge.check_keep_alive_status.failed")).toBe(true);
  });
});

describe("settings-return refresh dedup", () => {
  it("does nothing until openAndroidNotificationSettings marks pending", async () => {
    bridge.checkNotificationPermission.mockResolvedValue({ ok: true, data: { granted: true } });
    bridge.checkKeepAliveStatus.mockResolvedValue({ ok: true, data: { enabled: true } });
    const { deps } = makeDeps();
    const rt = useAndroidRuntime(deps);
    rt.handleAndroidNotificationWindowFocus();
    await vi.waitFor(() => {});
    expect(bridge.checkNotificationPermission).not.toHaveBeenCalled();

    bridge.openNotificationSettings.mockResolvedValue({ ok: true, data: { granted: false }, message: "" });
    await rt.openAndroidNotificationSettings();
    rt.handleAndroidNotificationWindowFocus();
    await vi.waitFor(() => expect(bridge.checkKeepAliveStatus).toHaveBeenCalledTimes(1));
    expect(bridge.checkNotificationPermission).toHaveBeenCalledTimes(1);
    // pending consumed: another focus does not re-refresh
    rt.handleAndroidNotificationWindowFocus();
    await vi.waitFor(() => {});
    expect(bridge.checkNotificationPermission).toHaveBeenCalledTimes(1);
  });

  it("skips refresh while document hidden", async () => {
    vi.stubGlobal("document", { visibilityState: "hidden" });
    bridge.openNotificationSettings.mockResolvedValue({ ok: true, data: null, message: "" });
    const { deps } = makeDeps();
    const rt = useAndroidRuntime(deps);
    await rt.openAndroidNotificationSettings();
    rt.handleAndroidNotificationWindowFocus();
    await vi.waitFor(() => {});
    expect(bridge.checkNotificationPermission).not.toHaveBeenCalled();
  });
});

describe("user actions", () => {
  it("battery exemption: request vs settings wording, error path", async () => {
    bridge.openBatteryOptimizationSettings.mockResolvedValue({ ok: true, message: "" });
    bridge.checkBatteryOptimization.mockResolvedValue({ ok: true, data: null });
    const { deps, toasts } = makeDeps();
    const rt = useAndroidRuntime(deps);
    await rt.requestBatteryOptimizationExemption("request");
    expect(toasts[toasts.length - 1]).toEqual({ message: "已打开系统电池优化豁免页", tone: "info" });
    await rt.requestBatteryOptimizationExemption("settings");
    expect(toasts[toasts.length - 1]).toEqual({ message: "已打开电池与后台运行相关设置", tone: "info" });

    bridge.openBatteryOptimizationSettings.mockResolvedValue({ ok: false, message: "被拒绝" });
    await rt.requestBatteryOptimizationExemption();
    expect(toasts[toasts.length - 1]).toEqual({ message: "被拒绝", tone: "error" });
  });

  it("notification permission request stores data, toasts, refreshes both", async () => {
    bridge.requestNotificationPermission.mockResolvedValue({ ok: true, data: { granted: true }, message: "" });
    bridge.checkNotificationPermission.mockResolvedValue({ ok: true, data: { granted: true } });
    bridge.checkKeepAliveStatus.mockResolvedValue({ ok: true, data: { enabled: false } });
    const { deps, toasts } = makeDeps();
    const rt = useAndroidRuntime(deps);
    await rt.requestAndroidNotificationPermission();
    expect(rt.androidNotificationStatus.value).toEqual({ granted: true });
    expect(toasts[0]).toEqual({ message: "通知权限已允许", tone: "success" });
    expect(bridge.checkNotificationPermission).toHaveBeenCalledTimes(1);
    expect(bridge.checkKeepAliveStatus).toHaveBeenCalledTimes(1);
  });

  it("keep-alive toggle wording and failure path", async () => {
    bridge.setKeepAliveEnabled.mockResolvedValue({ ok: true, data: { enabled: true }, message: "" });
    bridge.checkKeepAliveStatus.mockResolvedValue({ ok: true, data: { enabled: true } });
    const { deps, toasts } = makeDeps();
    const rt = useAndroidRuntime(deps);
    await rt.toggleAndroidKeepAlive(true);
    expect(toasts[toasts.length - 1]).toEqual({ message: "通知栏保活已开启", tone: "success" });
    await rt.toggleAndroidKeepAlive(false);
    expect(toasts[toasts.length - 1]).toEqual({ message: "通知栏保活已关闭", tone: "success" });

    bridge.setKeepAliveEnabled.mockResolvedValue({ ok: false, message: "失败原因" });
    await rt.toggleAndroidKeepAlive(true);
    expect(toasts[toasts.length - 1]).toEqual({ message: "失败原因", tone: "error" });
  });
});
