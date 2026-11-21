// GitHub 导出与连通性测试（结果页快捷推送）。
import type { Ref } from "vue";
import { exportResultsToGitHub, normalizeConfigSnapshot, testGitHubExport, type ProbeResult, type TaskTone } from "../bridge";
import type { SettingsForm } from "../settingsForm";
import { asRecord, asString, nonNegativeCount } from "../valueCoerce";
import type { ToastTone, ViewName } from "../appTypes";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../configSnapshotBuild";

export interface GithubExportOptions {
  settings: SettingsForm;
  task: { exportPath: string; taskId: string };
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  currentVisibleResultRows: () => ProbeResult[];
  navigateTo: (view: ViewName) => void;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  pushActivity: (title: string, detail: string) => void;
  resultGitHubTopN: Ref<number>;
  githubTesting: Ref<boolean>;
  githubExporting: Ref<boolean>;
}

export async function testGitHubExportSettings(options: GithubExportOptions) {
  const { buildConfigSnapshot, appendLog, showToast, pushActivity, githubTesting } = options;
  githubTesting.value = true;
  try {
    const result = await testGitHubExport({ config: buildConfigSnapshot() });
    appendLog("bridge.test_github_export", result);
    showToast(result.message || (result.ok ? "GitHub 仓库访问可用" : "GitHub 导出测试失败"), result.ok ? "success" : "error");
    if (!result.ok) {
      pushActivity("GitHub 导出测试失败", result.message || "请检查 owner、repo、branch 与 PAT 权限。");
      return;
    }
    pushActivity("GitHub 导出测试通过", result.message || "目标仓库 Contents 权限可用。");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "GitHub 导出测试失败", "error");
  } finally {
    githubTesting.value = false;
  }
}

export async function exportCurrentResultsToGitHub(options: GithubExportOptions) {
  const { settings, task, buildConfigSnapshot, currentVisibleResultRows, navigateTo, appendLog, showToast, setStatus, pushActivity, resultGitHubTopN, githubExporting } = options;
  const visibleRows = currentVisibleResultRows();
  if (visibleRows.length === 0) {
    void navigateTo("results");
    showToast("没有可导出的测速结果", "error");
    return;
  }

  const githubTopN = nonNegativeCount(resultGitHubTopN.value, 0);

  githubExporting.value = true;
  try {
    const config = normalizeConfigSnapshot(buildConfigSnapshot());
    config.github.top_n = githubTopN;
    config.upload.github.top_n = githubTopN;
    const result = await exportResultsToGitHub({
      config,
      export_path: task.exportPath,
      notification_trigger: "manual_push",
      results: visibleRows,
      task_id: task.taskId,
    });
    const data = asRecord(result.data);
    appendLog("bridge.export_results_github", result);
    if (!result.ok) {
      setStatus({
        detail: result.message || "导出到 GitHub 失败。",
        title: "GitHub 导出失败",
        tone: "failed",
      });
      pushActivity("GitHub 导出失败", result.message || "未能写入目标仓库。");
      showToast("GitHub 导出失败", "error");
      return;
    }

    settings.githubLastExportAt = asString(data.exported_at || data.exportedAt).trim() || new Date().toISOString();
    const targetPath = asString(data.path).trim();
    const htmlURL = asString(data.html_url || data.htmlURL).trim();
    setStatus({
      detail: targetPath ? `已写入 GitHub：${targetPath}` : result.message || "测速结果已导出到 GitHub。",
      title: "GitHub 导出完成",
      tone: "completed",
    });
    pushActivity("GitHub 导出完成", htmlURL || targetPath || result.message || "测速结果 CSV 已写入目标仓库。");
    showToast("已导出到 GitHub", "success");
  } finally {
    githubExporting.value = false;
  }
}
