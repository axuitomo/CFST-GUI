import { getPlatformAdapter } from "../../adapters/registry";
import type { PlatformMethodName } from "../../adapters/types";
import type { AppInfo, UpdateInfo, UpdateInstallResult } from "../../bridge/types";

async function invokePlatform<T>(method: PlatformMethodName, payload?: Record<string, unknown>) {
  return getPlatformAdapter().invokePlatform<T>(method, payload);
}

export async function getAppInfo() {
  return invokePlatform<AppInfo>("GetAppInfo");
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
