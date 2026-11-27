// 视口尺寸与验收预设（重构计划 §4.2）：从 App.vue 原样迁出。
// 桌面端通过窗口适配器读取原生窗口尺寸并支持固定验收预设；浏览器 / WebUI 回退 innerWidth/innerHeight。
import { reactive, ref } from "vue";
import type { ToastTone, ViewportPreset, ViewportSize } from "../lib/appTypes";
import { getDesktopWindowControls } from "../lib/bridge";

export const VIEWPORT_PRESETS: ViewportPreset[] = [
  { description: "最大化到当前屏幕可用区域，作为默认 UI 尺寸。", id: "adaptive", label: "自适应", mode: "adaptive", shell: "desktop" },
  { description: "小屏移动端，验证底部导航和触控安全区。", height: 844, id: "phone390", label: "390x844", mode: "fixed", shell: "mobile", width: 390 },
  { description: "平板竖屏，验证移动壳表单和卡片布局。", height: 1024, id: "tablet768", label: "768x1024", mode: "fixed", shell: "mobile", width: 768 },
  { description: "桌面断点起点，验证侧栏和桌面壳切换。", height: 768, id: "desktop1024", label: "1024x768", mode: "fixed", shell: "desktop", width: 1024 },
  { description: "常见 Windows 工作窗口，验证首屏信息密度。", height: 768, id: "desktop1366", label: "1366x768", mode: "fixed", shell: "desktop", width: 1366 },
  { description: "全高清桌面，验证阅读宽度和表格可读性。", height: 1080, id: "desktop1920", label: "1920x1080", mode: "fixed", shell: "desktop", width: 1920 },
  { description: "大屏桌面，验证最大宽度约束和留白。", height: 1440, id: "desktop2560", label: "2560x1440", mode: "fixed", shell: "desktop", width: 2560 },
];

export interface ViewportDeps {
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
}

export function useViewport(deps: ViewportDeps) {
  const viewportAdaptiveActive = ref(false);
  const viewportSwitching = ref(false);
  const desktopLayout = ref(typeof window === "undefined" ? true : Math.round(window.innerWidth) >= 1024);
  const viewportSize = reactive<ViewportSize>({
    cssHeight: 0,
    cssWidth: 0,
    height: 0,
    updatedAt: "",
    width: 0,
  });
  let viewportResizeTimer: number | undefined;

  function isViewportRuntimeSupported() {
    return Boolean(getDesktopWindowControls()?.isAvailable());
  }

  function applyViewportSize(runtimeSize?: { h: number; w: number }) {
    const cssWidth = typeof window === "undefined" ? 0 : Math.round(window.innerWidth);
    desktopLayout.value = cssWidth >= 1024;
    const cssHeight = typeof window === "undefined" ? 0 : Math.round(window.innerHeight);
    viewportSize.cssWidth = cssWidth;
    viewportSize.cssHeight = cssHeight;
    viewportSize.width = runtimeSize ? Math.round(runtimeSize.w) : cssWidth;
    viewportSize.height = runtimeSize ? Math.round(runtimeSize.h) : cssHeight;
    viewportSize.updatedAt = new Date().toISOString();
  }

  async function refreshViewportSize() {
    const controls = getDesktopWindowControls();
    if (!controls?.isAvailable()) {
      applyViewportSize();
      viewportAdaptiveActive.value = true;
      return;
    }

    try {
      const runtimeSize = await controls.getSize();
      applyViewportSize(runtimeSize);
      viewportAdaptiveActive.value = await controls.isMaximised();
    } catch (error) {
      deps.appendLog("runtime.window_size.failed", error instanceof Error ? error.message : String(error));
      viewportAdaptiveActive.value = false;
      applyViewportSize();
    }
  }

  async function maximizeViewportToAdaptive() {
    const controls = getDesktopWindowControls();
    if (!controls) {
      return;
    }
    controls.unfullscreen();
    controls.maximise();
    await new Promise((resolve) => window.setTimeout(resolve, 160));
    await refreshViewportSize();
  }

  async function ensureAdaptiveViewportOnStartup() {
    const controls = getDesktopWindowControls();
    if (!controls?.isAvailable()) {
      await refreshViewportSize();
      return;
    }

    try {
      const maximised = await controls.isMaximised();
      if (maximised) {
        await refreshViewportSize();
        return;
      }
      await maximizeViewportToAdaptive();
    } catch (error) {
      deps.appendLog("runtime.viewport_adaptive_startup.failed", error instanceof Error ? error.message : String(error));
      await refreshViewportSize();
    }
  }

  function scheduleViewportSizeRefresh() {
    applyViewportSize();
    if (viewportResizeTimer !== undefined) {
      window.clearTimeout(viewportResizeTimer);
    }
    viewportResizeTimer = window.setTimeout(() => {
      void refreshViewportSize();
    }, 120);
  }

  function stopViewportResizeTimer() {
    if (viewportResizeTimer !== undefined) {
      window.clearTimeout(viewportResizeTimer);
    }
  }

  async function applyViewportPreset(presetId: string) {
    const preset = VIEWPORT_PRESETS.find((entry) => entry.id === presetId);
    if (!preset) {
      deps.showToast("未知的验收尺寸", "error");
      return;
    }

    const controls = getDesktopWindowControls();
    if (!controls?.isAvailable()) {
      await refreshViewportSize();
      if (preset.mode === "adaptive") {
        deps.showToast("已刷新浏览器自适应尺寸", "success");
        return;
      }
      deps.showToast("固定验收尺寸仅 Wails 桌面支持", "error");
      return;
    }

    viewportSwitching.value = true;
    try {
      if (preset.mode === "adaptive") {
        await maximizeViewportToAdaptive();
      } else {
        controls.unfullscreen();
        controls.unmaximise();
        controls.setSize(preset.width, preset.height);
        controls.center();
        await new Promise((resolve) => window.setTimeout(resolve, 160));
        await refreshViewportSize();
      }
      deps.showToast(`已切换到 ${preset.label}`, "success");
    } catch (error) {
      const message = error instanceof Error ? error.message : "切换验收尺寸失败";
      deps.appendLog("runtime.viewport_preset.failed", { message, preset });
      deps.showToast(message, "error");
    } finally {
      viewportSwitching.value = false;
    }
  }

  return {
    applyViewportPreset,
    desktopLayout,
    ensureAdaptiveViewportOnStartup,
    isViewportRuntimeSupported,
    refreshViewportSize,
    scheduleViewportSizeRefresh,
    stopViewportResizeTimer,
    viewportAdaptiveActive,
    viewportSize,
    viewportSwitching,
  };
}
