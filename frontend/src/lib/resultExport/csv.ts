// 当前测速结果 CSV 导出（含 Android SAF 目标与文件名回退）。
import type { Ref } from "vue";
import { exportResultsCSV, type ProbeResult, type TaskTone } from "../bridge";
import type { SettingsForm } from "../settingsForm";
import { asCount, asRecord, asString } from "../valueCoerce";
import type { ToastTone, ViewName } from "../appTypes";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../configSnapshotBuild";

export interface CsvExportOptions {
  settings: SettingsForm;
  task: { exportPath: string; taskId: string };
  isAndroidApp: () => boolean;
  pathLeafName: (rawPath: string) => string;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  currentVisibleResultRows: () => ProbeResult[];
  navigateTo: (view: ViewName) => void;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  pushActivity: (title: string, detail: string) => void;
  csvExporting: Ref<boolean>;
}

export function defaultResultsCSVFileName(options: CsvExportOptions) {
  const { settings, task, pathLeafName } = options;
  return pathLeafName(task.exportPath) || settings.exportFileName.trim() || "result.csv";
}

export async function exportCurrentResultsCSV(options: CsvExportOptions) {
  const { settings, task, isAndroidApp, buildConfigSnapshot, currentVisibleResultRows, navigateTo, appendLog, showToast, setStatus, pushActivity, csvExporting } = options;
  const visibleRows = currentVisibleResultRows();
  if (visibleRows.length === 0) {
    void navigateTo("results");
    showToast("没有可导出的测速结果", "error");
    return;
  }

  csvExporting.value = true;
  try {
    const defaultFileName = defaultResultsCSVFileName(options);
    const targetUri = settings.exportTargetUri.trim();
    const targetDir = settings.exportTargetDir.trim();
    if (isAndroidApp() && !targetUri) {
      showToast("请先选择 Android SAF 导出目录", "error");
      return;
    }

    const result = await exportResultsCSV({
      config: buildConfigSnapshot(),
      file_name: defaultFileName,
      results: visibleRows,
      task_id: task.taskId,
      ...(targetDir ? { target_dir: targetDir } : {}),
      ...(targetUri ? { target_uri: targetUri } : {}),
    });
    appendLog("bridge.export_results_csv", result);
    const data = asRecord(result.data);
    if (!result.ok) {
      setStatus({
        detail: result.message || "导出当前测速结果 CSV 失败。",
        title: "CSV 导出失败",
        tone: "failed",
      });
      pushActivity("CSV 导出失败", result.message || "未能写入导出目标。");
      showToast("CSV 导出失败", "error");
      return;
    }

    const writtenCount = asCount(data.written_count || data.writtenCount) || visibleRows.length;
    const resolvedFileName = asString(data.file_name || data.fileName).trim() || defaultFileName;
    const resolvedPath = asString(data.path).trim();
    const resolvedTargetURI = asString(data.target_uri || data.targetUri).trim();
    const targetLabel = resolvedPath || resolvedTargetURI || resolvedFileName;
    setStatus({
      detail: targetLabel ? `已导出 ${writtenCount} 条结果到 ${targetLabel}。` : result.message || `已导出 ${writtenCount} 条结果。`,
      title: "CSV 导出完成",
      tone: "completed",
    });
    pushActivity("CSV 导出完成", targetLabel ? `已导出 ${writtenCount} 条结果到 ${targetLabel}。` : `已导出 ${writtenCount} 条结果。`);
    showToast("已导出当前测速结果 CSV", "success");
  } catch (error) {
    setStatus({
      detail: error instanceof Error ? error.message : "导出当前测速结果 CSV 失败。",
      title: "CSV 导出失败",
      tone: "failed",
    });
    showToast(error instanceof Error ? error.message : "CSV 导出失败", "error");
  } finally {
    csvExporting.value = false;
  }
}
