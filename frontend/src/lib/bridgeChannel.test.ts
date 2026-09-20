import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// 通道判定只认宿主身份：Wails 宿主地址（运行时稍后注入）、Capacitor 原生壳，以及两者都不是
// 的普通页面（CFST WebUI 或 Vite 直出预览）。判定不得在挂载路径上发起网络请求，也不得为
// 非宿主页面等待运行时注入——那只会让预览页白屏；令牌探测留给首个 WebUI 请求（webUIAuthRequired）。
const harness = vi.hoisted(() => ({
  desktopHost: false,
  platform: "web",
  ready: false,
  waitCalls: 0,
  waitResult: false,
}));

vi.mock("./wailsRuntime", () => ({
  EventsOn: () => () => {},
  isWailsDesktopHost: () => harness.desktopHost,
  isWailsRuntimeAvailable: () => harness.ready,
  // 等待期间宿主完成注入：waitResult 决定这次等待的结论。
  waitForWailsRuntime: async () => {
    harness.waitCalls += 1;
    harness.ready = harness.waitResult;
    return harness.waitResult;
  },
}));

vi.mock("@capacitor/core", () => ({
  Capacitor: { getPlatform: () => harness.platform },
  registerPlugin: () => ({
    Init: vi.fn(async () => ({})),
    addListener: vi.fn(),
  }),
}));

async function loadBridge() {
  vi.resetModules();
  return await import("./bridge");
}

beforeEach(() => {
  harness.desktopHost = false;
  harness.platform = "web";
  harness.ready = false;
  harness.waitCalls = 0;
  harness.waitResult = false;
  // 判定阶段不该用到它；留一个「像 WebUI 服务」的响应，万一被调用也能被断言抓住。
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({
      ok: true,
      status: 200,
      headers: { get: () => "application/json" },
      json: async () => ({ auth_required: false, ok: true, service: "cfst-webui" }),
    })),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("resolveBridgeMode", () => {
  it("非宿主页面直接落 webui 兜底：不等宿主注入，也不探测 WebUI 服务", async () => {
    const bridge = await loadBridge();

    await expect(bridge.resolveBridgeMode(50)).resolves.toBe("webui");
    expect(harness.waitCalls).toBe(0);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("Capacitor 原生壳直接 native，不等待也不探测", async () => {
    harness.platform = "android";
    const bridge = await loadBridge();

    await expect(bridge.resolveBridgeMode(50)).resolves.toBe("native");
    expect(harness.waitCalls).toBe(0);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("Wails 桌面宿主页面只等运行时注入，不探测 WebUI 服务", async () => {
    harness.desktopHost = true;
    harness.waitResult = true;
    const bridge = await loadBridge();

    await expect(bridge.resolveBridgeMode(50)).resolves.toBe("wails");
    expect(harness.waitCalls).toBe(1);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("宿主晚于等待上限注入时仍按宿主判定，绝不退回 HTTP 通道", async () => {
    // 桌面端打到只有 webui 构建才有的 /api/command 会被资产服务回 404（右下角错误提示）。
    harness.desktopHost = true;
    harness.waitResult = false;
    const bridge = await loadBridge();

    await expect(bridge.resolveBridgeMode(50)).resolves.toBe("wails");
    expect(harness.waitCalls).toBe(1);
    expect(fetch).not.toHaveBeenCalled();
  });
});
