// 任务快照 / 结果刷新（重构计划 §4.2）：从 App.vue 原样迁出。
// 负责把快照写回任务/统计（含 MCIS 进度与导出记录），并串行合并并发的刷新请求；
// 语义与迁移前一致：同任务号去重、排队重放、结果工作区随任务切换清空。
import type { Ref } from "vue";

import { getTaskSnapshot, listTaskResults, type ProbeResult, type ProbeResultIPFilter, type ProbeResultOrder, type ProbeResultSortBy, type TaskSnapshot } from "../lib/bridge";
import { asCount, asString } from "../lib/valueCoerce";
import type { ViewName } from "../lib/appTypes";

export interface TaskSyncTaskState {
  taskId: string;
  active: boolean;
  exportPath: string;
  stage: string;
  completedAt: string;
}

export interface TaskSyncSummaryState {
  downloadTarget: number;
  failed: number;
  passed: number;
  processed: number;
  total: number;
  exported: number;
}

export interface TaskSyncDeps {
  task: TaskSyncTaskState;
  taskSnapshot: Ref<TaskSnapshot | null>;
  taskSessionState: Ref<string>;
  summary: TaskSyncSummaryState;
  updateMCISProgress: (payload: Record<string, unknown>, active?: boolean) => void;
  taskResultCSVPath: Ref<string>;
  resultsLoading: Ref<boolean>;
  resultWorkspaceTaskId: Ref<string>;
  resultRows: Ref<ProbeResult[]>;
  resultSortBy: Ref<ProbeResultSortBy>;
  resultOrder: Ref<ProbeResultOrder>;
  resultIpFilter: Ref<ProbeResultIPFilter>;
  resultsPageLimit: Ref<number>;
  clearCurrentTaskResultWorkspace: (options?: { preserveSnapshot?: boolean }) => void;
  applyCurrentTaskResultWorkspace: (taskId: string, rows: ProbeResult[], totalCount: number) => void;
  navigateTo: (view: ViewName) => void;
  appendLog: (event: string, payload: unknown) => void;
}

export function useTaskSync(deps: TaskSyncDeps) {
  const { task, taskSnapshot, taskSessionState, summary } = deps;
  let snapshotRefreshInFlight = false;
  let snapshotRefreshPending = false;
  let snapshotRefreshQueuedTaskId = "";

  function applyTaskSnapshot(snapshot: TaskSnapshot) {
    taskSnapshot.value = snapshot;
    const snapshotSessionState = asString(snapshot.session_state || "").trim();
    taskSessionState.value = snapshotSessionState || asString(taskSessionState.value || "idle");
    task.taskId = snapshot.task_id || task.taskId;
    task.stage = snapshot.current_stage || snapshot.progress?.stage || task.stage;
    task.completedAt = snapshot.completed_at || "";
    const inactiveSession = ["idle", "persisted_only"].includes(snapshotSessionState);
    task.active = !["cancelled", "completed", "failed", "no_results"].includes(snapshot.status || "") && !inactiveSession && (snapshot.runtime_attached !== false || snapshot.session_state === "paused_runtime");

    if (snapshot.mcis_progress) {
      deps.updateMCISProgress({ ...snapshot.mcis_progress }, snapshot.current_stage === "stage0_mcis");
    }
    if (snapshot.progress) {
      summary.failed = asCount(snapshot.progress.failed, summary.failed);
      summary.passed = asCount(snapshot.progress.passed, summary.passed);
      summary.processed = asCount(snapshot.progress.processed, summary.processed);
      summary.total = asCount(snapshot.progress.total, summary.total);
      summary.downloadTarget = asCount(snapshot.progress.qualified_target, summary.downloadTarget);
    }

    if (snapshot.export_record) {
      summary.exported = asCount(snapshot.export_record.written_count, summary.exported);
      task.exportPath = [snapshot.export_record.target_dir, snapshot.export_record.file_name].filter(Boolean).join("/");
      deps.taskResultCSVPath.value = asString(snapshot.export_record.source_path || deps.taskResultCSVPath.value).trim();
    }
  }

  function taskResultCSVFallbackPath(snapshot?: TaskSnapshot | null) {
    const snapshotSourcePath = asString(snapshot?.export_record?.source_path || "").trim();
    if (snapshotSourcePath) {
      return snapshotSourcePath;
    }
    const cachedSourcePath = deps.taskResultCSVPath.value.trim();
    if (cachedSourcePath) {
      return cachedSourcePath;
    }
    return task.exportPath.trim();
  }

  function buildResultFileFallbackPayload(exportPath: string, snapshot?: TaskSnapshot | null) {
    const normalizedExportPath = (taskResultCSVFallbackPath(snapshot) || exportPath).trim();
    return { path: normalizedExportPath };
  }

  async function reconcileTaskData(taskId = task.taskId, options: { switchToResultsOnData?: boolean } = {}) {
    const normalizedTaskId = taskId.trim();
    if (!normalizedTaskId) {
      return;
    }
    if (deps.resultWorkspaceTaskId.value && deps.resultWorkspaceTaskId.value !== normalizedTaskId) {
      deps.clearCurrentTaskResultWorkspace();
    }
    await refreshTaskData(normalizedTaskId);
    if (options.switchToResultsOnData && deps.resultRows.value.length > 0) {
      void deps.navigateTo("results");
    }
  }

  async function refreshTaskData(taskId = task.taskId) {
    const normalizedTaskId = taskId.trim() || task.taskId.trim() || "result-file";

    if (snapshotRefreshInFlight) {
      snapshotRefreshPending = true;
      snapshotRefreshQueuedTaskId = normalizedTaskId;
      return;
    }

    snapshotRefreshInFlight = true;
    snapshotRefreshQueuedTaskId = normalizedTaskId;
    deps.resultsLoading.value = true;

    try {
      do {
        const currentTaskId = snapshotRefreshQueuedTaskId || normalizedTaskId;
        snapshotRefreshPending = false;
        snapshotRefreshQueuedTaskId = "";
        const snapshotResult = await getTaskSnapshot(currentTaskId);

        deps.appendLog("bridge.get_task_snapshot", snapshotResult);

        let snapshotForResults: TaskSnapshot | null = null;
        if (snapshotResult.ok && snapshotResult.data) {
          snapshotForResults = snapshotResult.data;
        } else if (taskSnapshot.value && asString(taskSnapshot.value.task_id || "").trim() === currentTaskId) {
          snapshotForResults = taskSnapshot.value;
        }

        if (snapshotResult.ok && snapshotResult.data) {
          applyTaskSnapshot(snapshotResult.data);
        }

        const resultsResult = await listTaskResults(currentTaskId, deps.resultSortBy.value, deps.resultOrder.value, buildResultFileFallbackPayload(task.exportPath, snapshotForResults), deps.resultIpFilter.value, {
          limit: deps.resultsPageLimit.value,
          offset: 0,
        });
        deps.appendLog("bridge.list_task_results", resultsResult);

        if (resultsResult.ok && resultsResult.data) {
          const nextRows = Array.isArray(resultsResult.data.results) ? resultsResult.data.results : [];
          const totalCount = asCount(resultsResult.data.total_count, nextRows.length);
          deps.applyCurrentTaskResultWorkspace(currentTaskId, nextRows, totalCount);
        }
      } while (snapshotRefreshPending);
    } finally {
      snapshotRefreshInFlight = false;
      deps.resultsLoading.value = false;
      snapshotRefreshQueuedTaskId = "";
    }
  }

  return {
    applyTaskSnapshot,
    buildResultFileFallbackPayload,
    reconcileTaskData,
    refreshTaskData,
    taskResultCSVFallbackPath,
  };
}
