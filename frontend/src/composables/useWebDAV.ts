// WebDAV 备份/还原域（重构计划 §4.2）：从 App.vue 原样迁出。
import { backupConfigToWebDAV, restoreConfigFromWebDAV, testWebDAV } from "../lib/bridge";
import type { SettingsForm } from "../lib/settingsForm";
import type { ToastTone, ViewName } from "../lib/appTypes";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../lib/configSnapshotBuild";
export interface WebDAVDeps {
  settings: SettingsForm;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  applyImportedConfigData: (value: unknown) => void;
  applyNavigation: (view: ViewName) => void;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
}
export function useWebDAV(deps: WebDAVDeps) {
  const { settings, buildConfigSnapshot, applyImportedConfigData, applyNavigation, appendLog, showToast } = deps;
  async function testWebDAVSettings() {
    try {
      const result = await testWebDAV({ config_snapshot: buildConfigSnapshot() });
      appendLog("bridge.test_webdav", result);
      showToast(result.message || (result.ok ? "WebDAV 连接可用" : "WebDAV 测试失败"), result.ok ? "success" : "error");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "WebDAV 测试失败", "error");
    }
  }

  async function backupToWebDAV() {
    if (!window.confirm("WebDAV 备份会覆盖远端配置压缩包，并包含完整 Cloudflare Token 和 WebDAV 凭据。确认继续？")) {
      return;
    }
    try {
      const result = await backupConfigToWebDAV({ config_snapshot: buildConfigSnapshot() });
      appendLog("bridge.backup_config_webdav", result);
      if (!result.ok) {
        showToast(result.message || "WebDAV 备份失败", "error");
        return;
      }
      settings.webdavLastBackupAt = new Date().toISOString();
      showToast(result.message || "已备份到 WebDAV", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "WebDAV 备份失败", "error");
    }
  }

  async function restoreFromWebDAV() {
    if (!window.confirm("从 WebDAV 还原会替换当前配置，当前配置会先自动备份到本地。确认继续？")) {
      return;
    }
    try {
      const result = await restoreConfigFromWebDAV({
        config_snapshot: buildConfigSnapshot(),
        current_config_snapshot: buildConfigSnapshot(),
      });
      appendLog("bridge.restore_config_webdav", result);
      if (!result.ok) {
        showToast(result.message || "WebDAV 还原失败", "error");
        return;
      }
      applyImportedConfigData(result.data);
      applyNavigation("settings");
      showToast(result.message || "已从 WebDAV 还原配置", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "WebDAV 还原失败", "error");
    }
  }

  return { testWebDAVSettings, backupToWebDAV, restoreFromWebDAV };
}
