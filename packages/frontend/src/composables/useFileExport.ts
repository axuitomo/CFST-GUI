// 文件导出/导入域（重构计划 §4.2）：路径选择、日志/诊断包/配置压缩包导出与配置导入。
// 从 App.vue 原样迁出；跨域回调经 deps 注入。
import { ref, type Ref } from "vue";
import { exportConfigArchive, exportDebugLog, exportDiagnosticPackage, importConfigArchive, normalizeConfigSnapshot, openLogDirectory, openPath, selectPath, type PathSelectionPayload, type StorageStatus, type TaskTone } from "../lib/bridge";
import { asRecord, asString } from "../lib/app/valueCoerce";
import type { SettingsForm } from "../lib/config/settingsForm";
import type { ToastTone, ViewName } from "../lib/app/types";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../lib/config/snapshotBuild";
import type { SourceDraft } from "./useSources";
export interface FileExportDeps {
  settings: SettingsForm;
  sources: Ref<SourceDraft[]>;
  configPath: Ref<string>;
  storageStatus: Ref<StorageStatus | null>;
  isAndroidApp: () => boolean;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  applyConfigSnapshot: (snapshot: ReturnType<typeof normalizeConfigSnapshot>) => void;
  applySourceProfileStore: (value: unknown) => void;
  markSourceSaveBaselines: () => void;
  applyStorageStatus: (value: unknown) => void;
  applyNavigation: (view: ViewName) => void;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
}
export function useFileExport(deps: FileExportDeps) {
  const { settings, sources, configPath, storageStatus, isAndroidApp, buildConfigSnapshot, applyConfigSnapshot, applySourceProfileStore, markSourceSaveBaselines, applyStorageStatus, applyNavigation, appendLog, showToast, setStatus } = deps;
  const androidLogExportTargetUri = ref("");

  function selectedPathValue(data: PathSelectionPayload) {
    return asString(data.path || data.directory || data.uri || data.target_uri).trim();
  }

  function pathLeafName(rawPath: string) {
    const normalized = rawPath
      .trim()
      .replace(/[?#].*$/, "")
      .replace(/[\\/]+$/, "");
    if (!normalized) {
      return "";
    }
    const browserDownloadPrefix = "browser-download:";
    const value = normalized.startsWith(browserDownloadPrefix) ? normalized.slice(browserDownloadPrefix.length) : normalized;
    const parts = value.split(/[\\/]/).filter(Boolean);
    return parts.length > 0 ? parts[parts.length - 1] : "";
  }

  async function selectAndroidLogExportTarget(defaultFileName: string, title: string): Promise<string | null> {
    try {
      const result = await selectPath({
        current_path: androidLogExportTargetUri.value,
        default_file_name: defaultFileName,
        mode: "export_target",
        title,
      });
      appendLog("bridge.select_log_export_target", result);
      const data = asRecord(result.data) as PathSelectionPayload;
      if (!result.ok) {
        showToast(result.message || "选择日志导出目录失败", "error");
        return null;
      }
      if (data.canceled) {
        return null;
      }
      const targetUri = asString(data.target_uri || data.uri || data.androidExportUri).trim();
      if (!targetUri) {
        showToast("未获取到日志导出目录", "error");
        return null;
      }
      androidLogExportTargetUri.value = targetUri;
      return targetUri;
    } catch (error) {
      showToast(error instanceof Error ? error.message : "选择日志导出目录失败", "error");
      return null;
    }
  }

  async function selectSourceFile(sourceId: string) {
    const source = sources.value.find((entry) => entry.id === sourceId);
    if (!source) {
      return;
    }

    try {
      const result = await selectPath({
        current_path: source.path,
        mode: "source_file",
        title: "选择输入源文件",
      });
      appendLog("bridge.select_source_file", result);
      const data = asRecord(result.data) as PathSelectionPayload;
      if (!result.ok) {
        showToast(result.message || "选择输入源文件失败", "error");
        return;
      }
      if (data.canceled) {
        return;
      }

      const uploadedContent = asString(data.content).trim();
      const path = selectedPathValue(data);
      if (!path && !uploadedContent) {
        showToast("未获取到文件路径", "error");
        return;
      }
      if (uploadedContent) {
        source.kind = "inline";
        source.content = uploadedContent;
        source.path = "";
        source.status_text = `已读取文件：${asString(data.display_name || data.file_name).trim() || "浏览器文件"}`;
        showToast("已读取输入源文件", "success");
        return;
      }
      source.kind = "file";
      source.path = path;
      if (data.display_name) {
        source.status_text = `已选择文件：${data.display_name}`;
      }
      showToast("已选择输入源文件", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "选择输入源文件失败", "error");
    }
  }

  async function selectExportTarget() {
    try {
      const result = await selectPath({
        current_path: settings.exportTargetUri.trim() || settings.exportTargetDir,
        default_file_name: settings.exportFileName.trim() || "result.csv",
        mode: "export_target",
        title: isAndroidApp() ? "选择 Android SAF 导出目录" : "选择导出目录",
      });
      appendLog("bridge.select_export_target", result);
      const data = asRecord(result.data) as PathSelectionPayload;
      if (!result.ok) {
        showToast(result.message || "选择导出位置失败", "error");
        return;
      }
      if (data.canceled) {
        return;
      }

      const targetUri = asString(data.target_uri || data.uri).trim();
      if (targetUri) {
        settings.exportTargetUri = targetUri;
        settings.exportTargetDir = "";
        showToast("已选择 Android SAF 导出目录", "success");
        return;
      }

      const selected = selectedPathValue(data);
      if (!selected) {
        showToast("未获取到导出位置", "error");
        return;
      }
      settings.exportTargetDir = asString(data.directory || data.path || selected).trim();
      if (data.file_name) {
        settings.exportFileName = data.file_name;
      }
      settings.exportTargetUri = "";
      showToast("已选择导出位置", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "选择导出位置失败", "error");
    }
  }

  async function importConfigFromFile() {
    try {
      const result = await selectPath({
        current_path: configPath.value,
        mode: "config_archive_import",
        title: "加载配置压缩包",
      });
      appendLog("bridge.import_config", result);
      const data = asRecord(result.data) as PathSelectionPayload;
      if (!result.ok) {
        showToast(result.message || "导入配置失败", "error");
        return;
      }
      if (data.canceled) {
        return;
      }

      const imported = await importConfigArchive({
        content: data.content,
        content_base64: data.content_base64,
        current_config_snapshot: buildConfigSnapshot(),
        path: selectedPathValue(data),
      });
      appendLog("bridge.import_config_archive", imported);
      if (!imported.ok) {
        showToast(imported.message || "导入配置失败", "error");
        return;
      }
      applyImportedConfigData(imported.data);
      applyNavigation("settings");
      showToast(imported.message || "配置已导入，原配置已备份", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "导入配置失败", "error");
    }
  }

  async function exportCurrentDebugLog() {
    const targetUri = isAndroidApp() ? await selectAndroidLogExportTarget("cfip-log.txt", "选择日志导出目录") : "";
    if (isAndroidApp() && !targetUri) {
      return;
    }
    try {
      const result = await exportDebugLog({
        config: buildConfigSnapshot(),
        ...(targetUri ? { target_uri: targetUri } : {}),
      });
      appendLog("bridge.export_debug_log", result);
      if (!result.ok) {
        showToast(result.message || "调试日志导出失败", "error");
        return;
      }
      const data = asRecord(result.data);
      const target = asString(data.path || data.target_uri || data.targetUri || data.file_name || data.fileName).trim();
      setStatus({
        detail: target ? `调试日志已导出到 ${target}。` : result.message || "调试日志已导出。",
        title: "调试日志已导出",
        tone: "completed",
      });
      showToast(result.message || "调试日志已导出", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "调试日志导出失败", "error");
    }
  }

  async function exportCurrentDiagnosticPackage() {
    const targetUri = isAndroidApp() ? await selectAndroidLogExportTarget("cfst-diagnostics.zip", "选择诊断包导出目录") : "";
    if (isAndroidApp() && !targetUri) {
      return;
    }
    try {
      const result = await exportDiagnosticPackage({
        config: buildConfigSnapshot(),
        ...(targetUri ? { target_uri: targetUri } : {}),
      });
      appendLog("bridge.export_diagnostic_package", result);
      if (!result.ok) {
        showToast(result.message || "诊断包导出失败", "error");
        return;
      }
      const data = asRecord(result.data);
      const target = asString(data.path || data.target_uri || data.targetUri || data.file_name || data.fileName).trim();
      setStatus({
        detail: target ? `诊断包已导出到 ${target}。` : result.message || "诊断包已导出。",
        title: "诊断包已导出",
        tone: "completed",
      });
      showToast(result.message || "诊断包已导出", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "诊断包导出失败", "error");
    }
  }

  async function openCurrentLogDirectory() {
    if (isAndroidApp()) {
      const targetUri = androidLogExportTargetUri.value;
      if (!targetUri) {
        showToast("请先导出调试日志或诊断包以定位日志目录。", "info");
        return;
      }
      try {
        await openPath(targetUri);
        setStatus({
          detail: `Android 日志导出目录：${targetUri}`,
          title: "日志导出目录",
          tone: "completed",
        });
        showToast("已打开 Android 日志导出目录", "success");
      } catch (error) {
        showToast(error instanceof Error ? error.message : "打开导出目录失败", "error");
      }
      return;
    }
    try {
      const result = await openLogDirectory({
        config: buildConfigSnapshot(),
      });
      appendLog("bridge.open_log_directory", result);
      const data = asRecord(result.data);
      const target = asString(data.path || data.directory).trim();
      if (!result.ok) {
        showToast(result.message || "打开日志目录失败", "error");
        return;
      }
      setStatus({
        detail: target ? `${result.message || "日志目录已定位。"} ${target}` : result.message || "日志目录已定位。",
        title: "日志目录",
        tone: "completed",
      });
      showToast(result.message || (target ? `日志目录：${target}` : "日志目录已定位"), "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "打开日志目录失败", "error");
    }
  }

  async function exportConfigToFile() {
    if (!window.confirm("导出的配置压缩包包含完整 Cloudflare Token 和 WebDAV 凭据。请确认目标位置可信。")) {
      return;
    }
    try {
      const result = await selectPath({
        current_path: storageStatus.value?.current_dir || configPath.value,
        default_file_name: `cfst-gui-config-${new Date().toISOString().slice(0, 10).replace(/-/g, "")}.zip`,
        mode: "config_archive_export",
        title: "导出配置压缩包",
      });
      appendLog("bridge.select_config_archive_export", result);
      const data = asRecord(result.data) as PathSelectionPayload;
      if (!result.ok || data.canceled) {
        return;
      }
      const targetUri = asString(data.target_uri || data.uri).trim();
      const targetPath = targetUri ? "" : selectedPathValue(data);
      const exported = await exportConfigArchive({
        config_snapshot: buildConfigSnapshot(),
        path: targetPath,
        target_uri: targetUri,
      });
      appendLog("bridge.export_config_archive", exported);
      if (!exported.ok) {
        showToast(exported.message || "配置导出失败", "error");
        return;
      }
      showToast("配置压缩包已导出", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "配置导出失败", "error");
    }
  }

  function applyImportedConfigData(value: unknown) {
    const data = asRecord(value);
    applyConfigSnapshot(normalizeConfigSnapshot(data.config_snapshot || data.configSnapshot || {}));
    if (data.source_profiles || data.sourceProfiles) {
      applySourceProfileStore(data.source_profiles || data.sourceProfiles);
    }
    markSourceSaveBaselines();
    if (data.storage) {
      applyStorageStatus(data.storage);
    }
    configPath.value = asString(data.configPath || data.config_path || configPath.value);
  }

  return { androidLogExportTargetUri, selectedPathValue, pathLeafName, selectAndroidLogExportTarget, selectSourceFile, selectExportTarget, importConfigFromFile, exportCurrentDebugLog, exportCurrentDiagnosticPackage, openCurrentLogDirectory, exportConfigToFile, applyImportedConfigData };
}
