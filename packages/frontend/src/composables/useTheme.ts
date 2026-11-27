// 主题解析与刷新（重构计划 §4.2）：从 App.vue 原样迁出。
// 解析口径：light/dark 直用；auto_system_time 跟系统偏好；auto_time 按 UTC 偏移的本地时钟窗口。
// 60s 轮询兜底时钟跨窗；安装/拆卸保持 App 生命周期钩子内的原有语句顺序。
import { applySurfaceTheme } from "../lib/app/surfaceTheme";
import type { SettingsForm } from "../lib/config/settingsForm";
import { currentMinutesInUTCOffset } from "../lib/app/time";
import { parseClockMinutes } from "../lib/app/valueCoerce";

export function useTheme(settings: SettingsForm) {
  let themeMediaQuery: MediaQueryList | null = null;
  let themeTimer: number | undefined;

  function timeFallbackTheme() {
    const current = currentMinutesInUTCOffset(settings.utcOffsetMinutes);
    const lightStart = parseClockMinutes(settings.themeLightStart, 7 * 60);
    const darkStart = parseClockMinutes(settings.themeDarkStart, 19 * 60);
    if (lightStart <= darkStart) {
      return current >= darkStart || current < lightStart ? "dark" : "light";
    }
    return current >= darkStart && current < lightStart ? "dark" : "light";
  }

  function resolvedThemeMode() {
    if (settings.themeMode === "light" || settings.themeMode === "dark") {
      return settings.themeMode;
    }
    if (settings.themeMode === "auto_system_time" && themeMediaQuery) {
      return themeMediaQuery.matches ? "dark" : "light";
    }
    return timeFallbackTheme();
  }

  function applyThemeMode() {
    const mode = resolvedThemeMode();
    document.documentElement.dataset.theme = mode;
    document.documentElement.classList.toggle("dark", mode === "dark");
    applySurfaceTheme(mode);
  }

  function scheduleThemeRefresh() {
    applyThemeMode();
    if (themeTimer !== undefined) {
      window.clearTimeout(themeTimer);
    }
    themeTimer = window.setTimeout(scheduleThemeRefresh, 60_000);
  }

  function installThemeTracking() {
    themeMediaQuery = window.matchMedia?.("(prefers-color-scheme: dark)") || null;
    themeMediaQuery?.addEventListener?.("change", applyThemeMode);
    scheduleThemeRefresh();
  }

  function detachThemeMediaListener() {
    themeMediaQuery?.removeEventListener?.("change", applyThemeMode);
  }

  function stopThemeRefreshTimer() {
    if (themeTimer !== undefined) {
      window.clearTimeout(themeTimer);
    }
  }

  return { applyThemeMode, detachThemeMediaListener, installThemeTracking, stopThemeRefreshTimer };
}
