import { getPlatformAdapter } from "../../adapters/registry";
import type { PlatformMethodName } from "../../adapters/types";
import { commandResult } from "../command";
import type { AndroidBatteryStatus, AndroidKeepAliveStatus, AndroidNotificationPermissionStatus, AndroidRuntimeStatus, AppInfo, UpdateInfo, UpdateInstallResult } from "../types";

async function invokePlatform<T>(method: PlatformMethodName, payload?: Record<string, unknown>) {
  return getPlatformAdapter().invokePlatform<T>(method, payload);
}

async function invokeAndroidOnly<T>(method: string, args: unknown[] | null, code: string, unsupportedMessage: string, nonAndroidMessage = "当前不是 Android 原生运行环境。") {
  const adapter = getPlatformAdapter();
  if (!adapter.invokeAndroidOnly) {
    return commandResult<T | null>(code, null, { message: nonAndroidMessage, ok: false });
  }
  return adapter.invokeAndroidOnly<T>(method, args, code, unsupportedMessage);
}

export async function getAppInfo() {
  return invokePlatform<AppInfo>("GetAppInfo");
}

export async function getAndroidRuntimeStatus() {
  return invokeAndroidOnly<AndroidRuntimeStatus>("GetAndroidRuntimeStatus", null, "ANDROID_RUNTIME_UNSUPPORTED", "当前环境不支持 Android 运行时状态查询。");
}

export async function checkBatteryOptimization() {
  return invokeAndroidOnly<AndroidBatteryStatus>("CheckBatteryOptimization", [{}], "ANDROID_BATTERY_UNSUPPORTED", "当前环境不支持省电策略检测。");
}

export async function checkKeepAliveStatus() {
  return invokeAndroidOnly<AndroidKeepAliveStatus>("CheckKeepAliveStatus", [{}], "ANDROID_KEEP_ALIVE_UNSUPPORTED", "当前环境不支持通知栏保活状态检测。");
}

export async function setKeepAliveEnabled(enabled: boolean) {
  return invokeAndroidOnly<AndroidKeepAliveStatus>("SetKeepAliveEnabled", [{ enabled }], "ANDROID_KEEP_ALIVE_UNSUPPORTED", "当前环境不支持通知栏保活设置。");
}

/** 把当前主题同步给 Android 原生窗口与 WebView 底色；其他运行环境直接跳过。 */
export async function setAndroidSurfaceTheme(dark: boolean): Promise<void> {
  await getPlatformAdapter().setAndroidSurfaceTheme?.(dark);
}

export async function openBatteryOptimizationSettings(mode = "request") {
  return invokeAndroidOnly<AndroidBatteryStatus>("OpenBatteryOptimizationSettings", [{ mode }], "ANDROID_BATTERY_UNSUPPORTED", "当前环境不支持打开省电策略设置。");
}

export async function checkNotificationPermission() {
  return invokeAndroidOnly<AndroidNotificationPermissionStatus>("CheckNotificationPermission", [{}], "ANDROID_NOTIFICATION_UNSUPPORTED", "当前环境不支持通知权限检测。");
}

export async function requestNotificationPermission() {
  return invokeAndroidOnly<AndroidNotificationPermissionStatus>("RequestNotificationPermission", [{}], "ANDROID_NOTIFICATION_UNSUPPORTED", "当前环境不支持申请通知权限。");
}

export async function showAndroidNotification(title: string, body: string) {
  const adapter = getPlatformAdapter();
  if (adapter.showAndroidNotification) {
    return adapter.showAndroidNotification(title, body);
  }
  return commandResult("ANDROID_NOTIFICATION_UNSUPPORTED", null, { message: "当前环境不支持 Android 通知。", ok: false });
}

export async function openNotificationSettings() {
  return invokeAndroidOnly<AndroidNotificationPermissionStatus>("OpenNotificationSettings", [{}], "ANDROID_NOTIFICATION_UNSUPPORTED", "当前环境不支持打开通知权限设置。");
}

export async function checkForUpdates(payload: Record<string, unknown> = {}) {
  return invokePlatform<UpdateInfo>("CheckForUpdates", payload);
}

export async function downloadAndInstallUpdate(payload: Record<string, unknown> = {}) {
  return invokePlatform<UpdateInstallResult>("DownloadAndInstallUpdate", payload);
}

export async function openReleasePage() {
  return getPlatformAdapter().openReleasePage();
}
