// 应用信息与在线更新域（重构计划 §4.2）：从 App.vue 原样迁出。
import { computed, reactive, ref } from "vue";
import { checkForUpdates, downloadAndInstallUpdate, getAppInfo, openReleasePage, type AppInfo, type UpdateInfo } from "../lib/bridge";
import { asBoolean, asRecord, asString } from "../lib/valueCoerce";
import type { ToastTone } from "../lib/appTypes";

export interface AppUpdateDeps {
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
}

export function useAppUpdate(deps: AppUpdateDeps) {
  const { appendLog, showToast } = deps;
  const appInfo = ref<AppInfo>({
    current_version: "1.0",
    install_mode: "",
    platform: "",
    release_url: "",
  });
  const isAndroidApp = computed(() => appInfo.value.platform === "android");
  const updateState = reactive({
    assetName: "",
    checkedAt: "",
    dockerImage: "",
    downloadPath: "",
    installStarted: false,
    installMode: "",
    installing: false,
    latestVersion: "",
    message: "尚未检查更新。",
    nextAction: "",
    releaseUrl: "",
    status: "idle" as "idle" | "checking" | "available" | "latest" | "installing" | "ready" | "failed",
    updateAvailable: false,
  });

  function applyAppInfo(value: unknown) {
    const source = asRecord(value);
    appInfo.value = {
      current_version: asString(source.current_version || source.currentVersion || "1.0"),
      install_mode: asString(source.install_mode || source.installMode),
      platform: asString(source.platform),
      release_url: asString(source.release_url || source.releaseUrl),
    };
    if (!updateState.releaseUrl) {
      updateState.releaseUrl = appInfo.value.release_url;
    }
  }

  function applyUpdateInfo(value: unknown) {
    const source = asRecord(value) as Partial<UpdateInfo> & Record<string, unknown>;
    updateState.assetName = asString(source.asset_name || source.assetName);
    updateState.checkedAt = new Date().toISOString();
    updateState.dockerImage = asString(source.docker_image || source.dockerImage || updateState.dockerImage);
    updateState.installStarted = asBoolean(source.install_started || source.installStarted, false);
    updateState.installMode = asString(source.install_mode || source.installMode);
    updateState.latestVersion = asString(source.latest_version || source.latestVersion);
    updateState.nextAction = asString(source.next_action || source.nextAction);
    updateState.releaseUrl = asString(source.release_url || source.releaseUrl || appInfo.value.release_url);
    updateState.updateAvailable = source.update_available === true || source.updateAvailable === true;
  }

  async function refreshAppInfo() {
    try {
      const result = await getAppInfo();
      appendLog("bridge.get_app_info", result);
      if (result.ok && result.data) {
        applyAppInfo(result.data);
      }
    } catch (error) {
      appendLog("bridge.get_app_info.failed", error instanceof Error ? error.message : String(error));
    }
  }

  async function checkOnlineUpdate() {
    updateState.status = "checking";
    updateState.message = "正在检查 GitHub Releases。";
    try {
      const result = await checkForUpdates({});
      appendLog("bridge.check_updates", result);
      if (!result.ok) {
        updateState.status = "failed";
        updateState.message = result.message || "检查更新失败。";
        showToast(updateState.message, "error");
        return;
      }
      applyUpdateInfo(result.data || {});
      if (updateState.updateAvailable) {
        updateState.status = "available";
        updateState.message = result.message || `发现新版本 ${updateState.latestVersion}。`;
        showToast("发现新版本", "info");
      } else {
        updateState.status = "latest";
        updateState.message = result.message || "当前已是最新版本。";
        showToast("当前已是最新版本", "success");
      }
    } catch (error) {
      updateState.status = "failed";
      updateState.message = error instanceof Error ? error.message : "检查更新失败。";
      showToast(updateState.message, "error");
    }
  }

  async function installOnlineUpdate() {
    updateState.status = "installing";
    updateState.installing = true;
    updateState.message = "正在下载更新包。";
    try {
      const result = await downloadAndInstallUpdate({});
      appendLog("bridge.download_update", result);
      if (!result.ok) {
        updateState.status = "failed";
        updateState.message = result.message || "下载或安装更新失败。";
        showToast(updateState.message, "error");
        return;
      }
      applyUpdateInfo(result.data || {});
      updateState.downloadPath = asString(asRecord(result.data).downloaded_path || asRecord(result.data).downloadedPath);
      updateState.status = "ready";
      const installStarted = asBoolean(asRecord(result.data).install_started || asRecord(result.data).installStarted, false);
      updateState.message = result.message || (installStarted ? "更新安装流程已启动。" : "更新包已下载，请按当前平台说明手动部署。");
      showToast(installStarted ? "更新安装流程已启动" : "更新包已下载", "success");
    } catch (error) {
      updateState.status = "failed";
      updateState.message = error instanceof Error ? error.message : "下载或安装更新失败。";
      showToast(updateState.message, "error");
    } finally {
      updateState.installing = false;
    }
  }

  async function openOnlineReleasePage() {
    try {
      const result = await openReleasePage();
      appendLog("bridge.open_release_page", result);
      if (!result.ok) {
        showToast(result.message || "打开发行页失败", "error");
      }
    } catch (error) {
      showToast(error instanceof Error ? error.message : "打开发行页失败", "error");
    }
  }

  return { appInfo, isAndroidApp, updateState, applyAppInfo, applyUpdateInfo, refreshAppInfo, checkOnlineUpdate, installOnlineUpdate, openOnlineReleasePage };
}
