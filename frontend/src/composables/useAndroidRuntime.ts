// Android 运行时状态（重构计划 §4.2）：电池优化 / 通知权限 / 通知栏保活。
// 从 App.vue 原样迁出；restoreAndroidRuntimeState 因耦合任务恢复编排仍留在 App。
import { ref } from "vue";
import { checkBatteryOptimization, checkKeepAliveStatus, checkNotificationPermission, openBatteryOptimizationSettings, openNotificationSettings, requestNotificationPermission, setKeepAliveEnabled, type AndroidBatteryStatus, type AndroidKeepAliveStatus, type AndroidNotificationPermissionStatus } from "../lib/bridge";
import type { ToastTone } from "../lib/appTypes";

export interface AndroidRuntimeDeps {
  appendLog: (event: string, payload: unknown) => void;
  isAndroidApp: () => boolean;
  showToast: (message: string, tone?: ToastTone) => void;
}

export function useAndroidRuntime(deps: AndroidRuntimeDeps) {
  const androidBatteryStatus = ref<AndroidBatteryStatus | null>(null);
  const androidKeepAliveStatus = ref<AndroidKeepAliveStatus | null>(null);
  const androidNotificationStatus = ref<AndroidNotificationPermissionStatus | null>(null);
  let androidNotificationSettingsRefreshPending = false;
  let androidNotificationSettingsRefreshInFlight = false;

  async function refreshAndroidBatteryStatus() {
    if (!deps.isAndroidApp()) {
      androidBatteryStatus.value = null;
      return;
    }
    try {
      const result = await checkBatteryOptimization();
      deps.appendLog("bridge.check_battery_optimization", result);
      if (result.ok && result.data) {
        androidBatteryStatus.value = result.data;
      }
    } catch (error) {
      deps.appendLog("bridge.check_battery_optimization.failed", error instanceof Error ? error.message : String(error));
    }
  }

  async function refreshAndroidNotificationStatus() {
    if (!deps.isAndroidApp()) {
      androidNotificationStatus.value = null;
      return;
    }
    try {
      const result = await checkNotificationPermission();
      deps.appendLog("bridge.check_notification_permission", result);
      if (result.ok && result.data) {
        androidNotificationStatus.value = result.data;
      }
    } catch (error) {
      deps.appendLog("bridge.check_notification_permission.failed", error instanceof Error ? error.message : String(error));
    }
  }

  async function refreshAndroidKeepAliveStatus() {
    if (!deps.isAndroidApp()) {
      androidKeepAliveStatus.value = null;
      return;
    }
    try {
      const result = await checkKeepAliveStatus();
      deps.appendLog("bridge.check_keep_alive_status", result);
      if (result.ok && result.data) {
        androidKeepAliveStatus.value = result.data;
      }
    } catch (error) {
      deps.appendLog("bridge.check_keep_alive_status.failed", error instanceof Error ? error.message : String(error));
    }
  }

  async function refreshAndroidNotificationStatusAfterSettingsReturn() {
    if (!androidNotificationSettingsRefreshPending || androidNotificationSettingsRefreshInFlight || !deps.isAndroidApp()) {
      return;
    }
    if (typeof document !== "undefined" && document.visibilityState === "hidden") {
      return;
    }
    androidNotificationSettingsRefreshPending = false;
    androidNotificationSettingsRefreshInFlight = true;
    try {
      await refreshAndroidNotificationStatus();
      await refreshAndroidKeepAliveStatus();
    } finally {
      androidNotificationSettingsRefreshInFlight = false;
    }
  }

  function handleAndroidNotificationVisibilityChange() {
    if (document.visibilityState === "visible") {
      void refreshAndroidNotificationStatusAfterSettingsReturn();
    }
  }

  function handleAndroidNotificationWindowFocus() {
    void refreshAndroidNotificationStatusAfterSettingsReturn();
  }

  async function requestBatteryOptimizationExemption(mode: "request" | "settings" | "details" = "request") {
    try {
      const result = await openBatteryOptimizationSettings(mode);
      deps.appendLog("bridge.open_battery_optimization_settings", result);
      if (!result.ok) {
        deps.showToast(result.message || "打开省电策略设置失败", "error");
        return;
      }
      deps.showToast(mode === "request" ? "已打开系统电池优化豁免页" : "已打开电池与后台运行相关设置", "info");
      await refreshAndroidBatteryStatus();
    } catch (error) {
      deps.showToast(error instanceof Error ? error.message : "打开省电策略设置失败", "error");
    }
  }

  async function requestAndroidNotificationPermission() {
    try {
      const result = await requestNotificationPermission();
      deps.appendLog("bridge.request_notification_permission", result);
      if (result.data) {
        androidNotificationStatus.value = result.data;
      }
      deps.showToast(result.message || (result.ok ? "通知权限已允许" : "通知权限未允许"), result.ok ? "success" : "error");
      await refreshAndroidNotificationStatus();
      await refreshAndroidKeepAliveStatus();
    } catch (error) {
      deps.showToast(error instanceof Error ? error.message : "申请通知权限失败", "error");
    }
  }

  async function openAndroidNotificationSettings() {
    try {
      const result = await openNotificationSettings();
      deps.appendLog("bridge.open_notification_settings", result);
      if (result.data) {
        androidNotificationStatus.value = result.data;
      }
      if (!result.ok) {
        deps.showToast(result.message || "打开通知权限设置失败", "error");
        return;
      }
      androidNotificationSettingsRefreshPending = true;
      deps.showToast(result.message || "已打开通知权限设置", "info");
    } catch (error) {
      deps.showToast(error instanceof Error ? error.message : "打开通知权限设置失败", "error");
    }
  }

  async function toggleAndroidKeepAlive(enabled: boolean) {
    try {
      const result = await setKeepAliveEnabled(enabled);
      deps.appendLog("bridge.set_keep_alive_enabled", result);
      if (result.data) {
        androidKeepAliveStatus.value = result.data;
      }
      if (!result.ok) {
        deps.showToast(result.message || "更新通知栏保活失败", "error");
        return;
      }
      deps.showToast(result.message || (enabled ? "通知栏保活已开启" : "通知栏保活已关闭"), "success");
      await refreshAndroidKeepAliveStatus();
    } catch (error) {
      deps.showToast(error instanceof Error ? error.message : "更新通知栏保活失败", "error");
    }
  }

  return {
    androidBatteryStatus,
    androidKeepAliveStatus,
    androidNotificationStatus,
    handleAndroidNotificationVisibilityChange,
    handleAndroidNotificationWindowFocus,
    openAndroidNotificationSettings,
    refreshAndroidBatteryStatus,
    refreshAndroidKeepAliveStatus,
    refreshAndroidNotificationStatus,
    requestAndroidNotificationPermission,
    requestBatteryOptimizationExemption,
    toggleAndroidKeepAlive,
  };
}
