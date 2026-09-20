import { afterEach, describe, expect, it, vi } from "vitest";
import { isWailsDesktopHost, isWailsRuntimeAvailable, waitForWailsRuntime } from "./wailsRuntime";

afterEach(() => {
  vi.unstubAllGlobals();
});

// @wailsio/runtime 会在任何 DOM 环境创建空对象 window._wails，只有 Wails 桌面宿主
// 才会注入 _wails.environment。固定住这个区别，避免 WebUI 再次误走桌面 IPC 通道。
describe("isWailsRuntimeAvailable", () => {
  it("treats a page without the host injection as non desktop", () => {
    vi.stubGlobal("window", { _wails: {} });

    expect(isWailsRuntimeAvailable()).toBe(false);
  });

  it("accepts the Wails desktop host injection", () => {
    vi.stubGlobal("window", { _wails: { environment: { OS: "windows" } } });

    expect(isWailsRuntimeAvailable()).toBe(true);
  });

  it("returns false when no window exists", () => {
    vi.stubGlobal("window", undefined);

    expect(isWailsRuntimeAvailable()).toBe(false);
  });
});

// 宿主地址是启动瞬间唯一可用的身份信号：桌面端永远不是 CFST WebUI 服务，所以这两个
// 地址必须被认成桌面，而 WebUI/浏览器的地址不能被认成桌面。
describe("isWailsDesktopHost", () => {
  it("accepts the Wails asset-server origins", () => {
    vi.stubGlobal("window", { location: { hostname: "wails.localhost", protocol: "http:" } });
    expect(isWailsDesktopHost()).toBe(true);

    vi.stubGlobal("window", { location: { hostname: "localhost", protocol: "wails:" } });
    expect(isWailsDesktopHost()).toBe(true);
  });

  it("rejects browser and WebUI origins", () => {
    vi.stubGlobal("window", { location: { hostname: "127.0.0.1", protocol: "http:" } });
    expect(isWailsDesktopHost()).toBe(false);

    vi.stubGlobal("window", { location: { hostname: "cfst.example.com", protocol: "https:" } });
    expect(isWailsDesktopHost()).toBe(false);
  });

  it("returns false without a window", () => {
    vi.stubGlobal("window", undefined);
    expect(isWailsDesktopHost()).toBe(false);
  });
});

describe("waitForWailsRuntime", () => {
  function stubWindow(location: { hostname: string; protocol: string }) {
    const stub = {
      _wails: {} as { environment?: { OS?: string } },
      location,
      clearInterval: globalThis.clearInterval,
      clearTimeout: globalThis.clearTimeout,
      setInterval: globalThis.setInterval,
      setTimeout: globalThis.setTimeout,
    };
    vi.stubGlobal("window", stub);
    return stub;
  }

  it("resolves true as soon as the host injects _wails.environment", async () => {
    const stub = stubWindow({ hostname: "wails.localhost", protocol: "http:" });

    const wait = waitForWailsRuntime(500);
    stub._wails = { environment: { OS: "windows" } };

    await expect(wait).resolves.toBe(true);
  });

  it("resolves false when the host never injects the runtime", async () => {
    stubWindow({ hostname: "127.0.0.1", protocol: "http:" });

    await expect(waitForWailsRuntime(30)).resolves.toBe(false);
  });
});
