import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import * as wails from "../lib/wailsRuntime";
import { VIEWPORT_PRESETS, useViewport } from "./useViewport";

vi.mock("../lib/wailsRuntime", () => ({
  WindowCenter: vi.fn(),
  WindowGetSize: vi.fn(async () => ({ h: 900, w: 1440 })),
  WindowIsMaximised: vi.fn(async () => true),
  WindowMaximise: vi.fn(),
  WindowSetSize: vi.fn(),
  WindowUnfullscreen: vi.fn(),
  WindowUnmaximise: vi.fn(),
  isWailsRuntimeAvailable: vi.fn(() => true),
}));

function makeDeps() {
  return { appendLog: vi.fn(), showToast: vi.fn() };
}

describe("useViewport", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal("window", {
      clearTimeout: (id: number) => clearTimeout(id),
      innerHeight: 800,
      innerWidth: 1280,
      setTimeout: (fn: () => void, ms: number) => setTimeout(fn, ms),
    });
    vi.mocked(wails.isWailsRuntimeAvailable).mockReturnValue(true);
    vi.mocked(wails.WindowIsMaximised).mockResolvedValue(true);
    vi.mocked(wails.WindowGetSize).mockResolvedValue({ h: 900, w: 1440 });
    vi.clearAllMocks();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("无运行时回退 inner 尺寸并标记自适应", async () => {
    vi.mocked(wails.isWailsRuntimeAvailable).mockReturnValue(false);
    const { refreshViewportSize, viewportAdaptiveActive, viewportSize, desktopLayout } = useViewport(makeDeps());
    await refreshViewportSize();
    expect(viewportSize.cssWidth).toBe(1280);
    expect(viewportSize.width).toBe(1280);
    expect(viewportSize.height).toBe(800);
    expect(viewportAdaptiveActive.value).toBe(true);
    expect(desktopLayout.value).toBe(true);
    expect(Number.isNaN(Date.parse(viewportSize.updatedAt))).toBe(false);
  });

  it("桌面运行时读原生窗口尺寸与最大化状态", async () => {
    const { refreshViewportSize, viewportAdaptiveActive, viewportSize } = useViewport(makeDeps());
    await refreshViewportSize();
    expect(viewportSize.width).toBe(1440);
    expect(viewportSize.height).toBe(900);
    expect(viewportSize.cssWidth).toBe(1280);
    expect(viewportAdaptiveActive.value).toBe(true);
  });

  it("窗口尺寸读取失败时记日志并回退", async () => {
    vi.mocked(wails.WindowGetSize).mockRejectedValue(new Error("boom"));
    const deps = makeDeps();
    const { refreshViewportSize, viewportAdaptiveActive, viewportSize } = useViewport(deps);
    await refreshViewportSize();
    expect(deps.appendLog).toHaveBeenCalledWith("runtime.window_size.failed", "boom");
    expect(viewportAdaptiveActive.value).toBe(false);
    expect(viewportSize.width).toBe(1280);
  });

  it("resize 调度 120ms 防抖", async () => {
    const { scheduleViewportSizeRefresh } = useViewport(makeDeps());
    scheduleViewportSizeRefresh();
    scheduleViewportSizeRefresh();
    expect(wails.WindowGetSize).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(120);
    expect(wails.WindowGetSize).toHaveBeenCalledTimes(1);
  });

  it("未知预设报错不切换", async () => {
    const deps = makeDeps();
    const { applyViewportPreset, viewportSwitching } = useViewport(deps);
    await applyViewportPreset("nope");
    expect(deps.showToast).toHaveBeenCalledWith("未知的验收尺寸", "error");
    expect(viewportSwitching.value).toBe(false);
  });

  it("浏览器端自适应预设只刷新，固定预设拒绝", async () => {
    vi.mocked(wails.isWailsRuntimeAvailable).mockReturnValue(false);
    const deps = makeDeps();
    const { applyViewportPreset } = useViewport(deps);
    await applyViewportPreset("adaptive");
    expect(deps.showToast).toHaveBeenCalledWith("已刷新浏览器自适应尺寸", "success");
    await applyViewportPreset("phone390");
    expect(deps.showToast).toHaveBeenCalledWith("固定验收尺寸仅 Wails 桌面支持", "error");
  });

  it("桌面端固定预设走 Unfullscreen/Unmaximise/SetSize/Center", async () => {
    const deps = makeDeps();
    const { applyViewportPreset, viewportSwitching } = useViewport(deps);
    const pending = applyViewportPreset("phone390");
    expect(viewportSwitching.value).toBe(true);
    await vi.advanceTimersByTimeAsync(160);
    await pending;
    expect(wails.WindowUnfullscreen).toHaveBeenCalled();
    expect(wails.WindowUnmaximise).toHaveBeenCalled();
    expect(wails.WindowSetSize).toHaveBeenCalledWith(390, 844);
    expect(wails.WindowCenter).toHaveBeenCalled();
    expect(deps.showToast).toHaveBeenCalledWith("已切换到 390x844", "success");
    expect(viewportSwitching.value).toBe(false);
  });

  it("桌面端自适应预设走最大化", async () => {
    const deps = makeDeps();
    const { applyViewportPreset } = useViewport(deps);
    const pending = applyViewportPreset("adaptive");
    await vi.advanceTimersByTimeAsync(160);
    await pending;
    expect(wails.WindowMaximise).toHaveBeenCalled();
    expect(deps.showToast).toHaveBeenCalledWith("已切换到 自适应", "success");
  });

  it("预设切换失败记日志并复位 switching", async () => {
    vi.mocked(wails.WindowSetSize).mockImplementation(() => {
      throw new Error("size failed");
    });
    const deps = makeDeps();
    const { applyViewportPreset, viewportSwitching } = useViewport(deps);
    await applyViewportPreset("phone390");
    expect(deps.appendLog).toHaveBeenCalledWith("runtime.viewport_preset.failed", expect.objectContaining({ message: "size failed" }));
    expect(deps.showToast).toHaveBeenCalledWith("size failed", "error");
    expect(viewportSwitching.value).toBe(false);
  });

  it("启动时已最大化只刷新，未最大化走最大化", async () => {
    const deps = makeDeps();
    const { ensureAdaptiveViewportOnStartup } = useViewport(deps);
    await ensureAdaptiveViewportOnStartup();
    expect(wails.WindowMaximise).not.toHaveBeenCalled();
    vi.mocked(wails.WindowIsMaximised).mockResolvedValue(false);
    const pending = ensureAdaptiveViewportOnStartup();
    await vi.advanceTimersByTimeAsync(160);
    await pending;
    expect(wails.WindowMaximise).toHaveBeenCalledTimes(1);
  });

  it("预设表保持 7 项且首项自适应", () => {
    expect(VIEWPORT_PRESETS).toHaveLength(7);
    expect(VIEWPORT_PRESETS[0]).toMatchObject({ id: "adaptive", mode: "adaptive", shell: "desktop" });
  });
});