import { afterEach, describe, expect, it, vi } from "vitest";
import { isWailsRuntimeAvailable } from "./wailsRuntime";

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
