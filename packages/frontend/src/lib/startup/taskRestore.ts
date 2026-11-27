// 启动期任务状态恢复（Android 原生运行时会话 + 最近任务快照），从 useStartupSync 拆出。
import type { Ref } from "vue";
import { getAndroidRuntimeStatus, listTaskSnapshots, type AndroidBatteryStatus, type AndroidKeepAliveStatus, type AppInfo, type TaskSnapshot, type TaskTone } from "../bridge";
import { asString } from "../app/valueCoerce";
import type { HistoryEntry, ViewName } from "../app/types";

export interface AndroidRuntimeRestoreDeps {
  appInfo: Ref<AppInfo>;
  appendLog: (event: string, payload: unknown) => void;
  androidBatteryStatus: Ref<AndroidBatteryStatus | null>;
  androidKeepAliveStatus: Ref<AndroidKeepAliveStatus | null>;
  taskSessionState: Ref<string>;
  applyTaskSnapshot: (snapshot: TaskSnapshot) => void;
  task: { taskId: string };
  finishTaskAction: () => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  pushActivity: (title: string, detail: string) => void;
  navigateTo: (view: ViewName) => void;
  refreshTaskData: (taskId?: string) => Promise<void>;
}

export async function restoreAndroidRuntimeState(deps: AndroidRuntimeRestoreDeps) {
  const { appInfo, appendLog, androidBatteryStatus, androidKeepAliveStatus, taskSessionState, applyTaskSnapshot, task, finishTaskAction, setStatus, pushActivity, navigateTo, refreshTaskData } = deps;
  if (appInfo.value.platform !== "android") {
    return;
  }
  try {
    const result = await getAndroidRuntimeStatus();
    appendLog("bridge.get_android_runtime_status", result);
    if (!result.ok || !result.data) {
      return;
    }
    if (result.data.battery) {
      androidBatteryStatus.value = result.data.battery;
    }
    if (result.data.keep_alive) {
      androidKeepAliveStatus.value = result.data.keep_alive;
    }
    const runtimeTaskId = asString(result.data.task_id).trim();
    const runtimeSnapshot = result.data.task_snapshot;
    taskSessionState.value = asString(result.data.session_state || "idle");
    if (runtimeSnapshot) {
      applyTaskSnapshot(runtimeSnapshot);
      task.taskId = runtimeTaskId || task.taskId;
      if (runtimeSnapshot.runtime_attached === true || taskSessionState.value === "paused_runtime") {
        finishTaskAction();
        setStatus({
          detail: taskSessionState.value === "paused_runtime" ? "已重新接入暂停中的 Android 探测任务，可以继续执行。" : "已重新接入 Android 后台探测任务，界面将继续接收实时进度。",
          title: taskSessionState.value === "paused_runtime" ? "任务已恢复" : "任务重新接入",
          tone: taskSessionState.value === "paused_runtime" ? "cooling" : "running",
        });
        pushActivity("Android 任务已恢复", taskSessionState.value === "paused_runtime" ? "检测到暂停中的原生任务。" : "检测到仍在运行的原生任务。");
        void navigateTo("dashboard");
      } else if (taskSessionState.value === "persisted_only") {
        finishTaskAction();
        setStatus({
          detail: "已恢复上次任务快照和结果文件，但原生运行时会话已经结束，需要重新启动任务。可在「结果」页查看已落盘结果。",
          title: "已恢复历史结果",
          tone: "warning",
        });
        pushActivity("恢复到已落盘结果", "已读取 Android 任务快照，但当前没有可重连的原生活动会话。");
        // 冷启动统一落在任务看板：结果行照旧读取（下面 refreshTaskData），但不再把用户
        // 直接推进结果页；要看历史结果由用户自己在「结果」页打开。
      }
      if (runtimeTaskId) {
        await refreshTaskData(runtimeTaskId);
      }
    }
  } catch (error) {
    appendLog("bridge.get_android_runtime_status.failed", error instanceof Error ? error.message : String(error));
  }
}

export interface LatestTaskRestoreDeps {
  task: { taskId: string; exportPath: string };
  taskSnapshot: Ref<TaskSnapshot | null>;
  hasActiveTask: Ref<boolean>;
  finishTaskAction: () => void;
  applyTaskSnapshot: (snapshot: TaskSnapshot) => void;
  summarizeFailureSummary: (summaryValue: unknown) => string;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  pushActivity: (title: string, detail: string) => void;
  updateHistory: (entry: HistoryEntry) => void;
  summary: { exported: number };
  reconcileTaskData: (taskId?: string, options?: { switchToResultsOnData?: boolean }) => Promise<void>;
  navigateTo: (view: ViewName) => void;
  appendLog: (event: string, payload: unknown) => void;
}

export async function restoreLatestTaskState(deps: LatestTaskRestoreDeps) {
  const { task, taskSnapshot, hasActiveTask, finishTaskAction, applyTaskSnapshot, summarizeFailureSummary, setStatus, pushActivity, updateHistory, summary, reconcileTaskData, navigateTo, appendLog } = deps;
  const result = await listTaskSnapshots(1);
  appendLog("bridge.list_task_snapshots", result);
  const latest = result.ok && result.data && Array.isArray(result.data.items) ? result.data.items[0] : null;
  if (!latest?.task_id) {
    return;
  }
  if (task.taskId === latest.task_id && taskSnapshot.value) {
    return;
  }
  if (hasActiveTask.value && task.taskId && task.taskId !== latest.task_id) {
    return;
  }

  finishTaskAction();
  applyTaskSnapshot(latest);
  const sessionState = asString(latest.session_state || "idle").trim();
  const snapshotStatus = asString(latest.status || "").trim();
  const failureSummary = summarizeFailureSummary(latest.failure_summary);
  let restoredStatus: { detail: string; title: string; tone: TaskTone };
  if (sessionState === "active_runtime" || sessionState === "paused_runtime") {
    restoredStatus = {
      detail: sessionState === "paused_runtime" ? "已重新接入暂停中的探测任务，可以继续执行。" : "已重新接入仍在运行的探测任务，界面将继续接收实时进度。",
      title: sessionState === "paused_runtime" ? "任务已恢复" : "任务重新接入",
      tone: sessionState === "paused_runtime" ? "cooling" : "running",
    };
  } else {
    const tone = (["cancelled", "completed", "failed", "no_results"] as TaskTone[]).includes(snapshotStatus as TaskTone) ? (snapshotStatus as TaskTone) : "warning";
    restoredStatus = {
      detail: failureSummary || (snapshotStatus === "completed" ? "已恢复最近一次任务快照和结果。" : "已恢复最近一次任务快照；原运行会话已结束，不能直接继续。"),
      title: snapshotStatus === "completed" ? "已恢复历史结果" : "已恢复历史任务",
      tone,
    };
  }
  setStatus(restoredStatus);
  pushActivity(restoredStatus.title, restoredStatus.detail);
  updateHistory({
    detail: restoredStatus.detail,
    exported: summary.exported,
    failureSummary,
    targetPath: task.exportPath,
    taskId: latest.task_id,
    title: restoredStatus.title,
    tone: restoredStatus.tone,
    updatedAt: latest.updated_at || latest.completed_at || latest.started_at || new Date().toISOString(),
  });
  await reconcileTaskData(latest.task_id, {
    // 冷启动落在任务看板：结果行仍会随 reconcileTaskData 读进来，但不自动切到结果页。
    switchToResultsOnData: false,
  });
  if (sessionState === "active_runtime" || sessionState === "paused_runtime") {
    void navigateTo("dashboard");
  }
}
