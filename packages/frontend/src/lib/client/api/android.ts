import { getPlatformAdapter } from "../../adapters/registry";
import { commandResult } from "../../bridge/command";
import type { AndroidBatteryStatus, AndroidKeepAliveStatus, AndroidNotificationPermissionStatus, AndroidRuntimeStatus } from "../../bridge/types";

async function invokeAndroidCommand<T>(method: string, args: unknown[] | null, code: string, unsupportedMessage: string, nonAndroidMessage = "当前不是 Android 原生运行环境。") {
  const android = getPlatformAdapter().android;
  if (!android) {
    return commandResult<T | null>(code, null, { message: nonAndroidMessage, ok: false });
  }
  return android.invokeCommand<T>(method, args, code, unsupportedMessage);
}

export async function getAndroidRuntimeStatus() {
  return invokeAndroidCommand<AndroidRuntimeStatus>("GetAndroidRuntimeStatus", null, "ANDROID_RUNTIME_UNSUPPORTED", "当前环境不支持 Android 运行时状态查询。");
}

export async function checkBatteryOptimization() {
  return invokeAndroidCommand<AndroidBatteryStatus>("CheckBatteryOptimization", [{}], "ANDROID_BATTERY_UNSUPPORTED", "当前环境不支持省电策略检测。");
}

export async function checkKeepAliveStatus() {
  return invokeAndroidCommand<AndroidKeepAliveStatus>("CheckKeepAliveStatus", [{}], "ANDROID_KEEP_ALIVE_UNSUPPORTED", "当前环境不支持通知栏保活状态检测。");
}

export async function setKeepAliveEnabled(enabled: boolean) {
  return invokeAndroidCommand<AndroidKeepAliveStatus>("SetKeepAliveEnabled", [{ enabled }], "ANDROID_KEEP_ALIVE_UNSUPPORTED", "当前环境不支持通知栏保活设置。");
}

/** 把当前主题同步给 Android 原生窗口与 WebView 底色；其他运行环境直接跳过。 */
export async function setAndroidSurfaceTheme(dark: boolean): Promise<void> {
  await getPlatformAdapter().android?.setSurfaceTheme(dark);
}

export async function openBatteryOptimizationSettings(mode = "request") {
  return invokeAndroidCommand<AndroidBatteryStatus>("OpenBatteryOptimizationSettings", [{ mode }], "ANDROID_BATTERY_UNSUPPORTED", "当前环境不支持打开省电策略设置。");
}

export async function checkNotificationPermission() {
  return invokeAndroidCommand<AndroidNotificationPermissionStatus>("CheckNotificationPermission", [{}], "ANDROID_NOTIFICATION_UNSUPPORTED", "当前环境不支持通知权限检测。");
}

export async function requestNotificationPermission() {
  return invokeAndroidCommand<AndroidNotificationPermissionStatus>("RequestNotificationPermission", [{}], "ANDROID_NOTIFICATION_UNSUPPORTED", "当前环境不支持申请通知权限。");
}

export async function showAndroidNotification(title: string, body: string) {
  const android = getPlatformAdapter().android;
  if (android) {
    return android.showNotification(title, body);
  }
  return commandResult("ANDROID_NOTIFICATION_UNSUPPORTED", null, { message: "当前环境不支持 Android 通知。", ok: false });
}

export async function openNotificationSettings() {
  return invokeAndroidCommand<AndroidNotificationPermissionStatus>("OpenNotificationSettings", [{}], "ANDROID_NOTIFICATION_UNSUPPORTED", "当前环境不支持打开通知权限设置。");
}
