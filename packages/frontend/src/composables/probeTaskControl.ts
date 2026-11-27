// 任务控制动作（暂停 / 终止 / 继续 / 重启前清理暂停任务）。
import type { ComputedRef, Ref } from "vue";
import { resumeProbe, stopProbe, type TaskTone } from "../lib/bridge";
import type { TaskActionKind } from "../lib/app/labels";
import type { ViewName } from "../lib/app/types";
import type { ProbeTaskCoreState } from "./probeTaskDeps";

export interface ProbeTaskControlOptions extends ProbeTaskCoreState {
  loading: Ref<boolean>;
  appendLog: (event: string, payload: unknown) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  pushActivity: (title: string, detail: string) => void;
  navigateTo: (view: ViewName) => void;
  reconcileTaskData: (taskId?: string, options?: { switchToResultsOnData?: boolean }) => Promise<void>;
  canCancelTask: ComputedRef<boolean>;
  hasDetachedTaskSnapshot: ComputedRef<boolean>;
  hasPausedTask: ComputedRef<boolean>;
  activeTaskSessionState: ComputedRef<string>;
  taskActionInFlight: ComputedRef<boolean>;
  beginTaskAction: (kind: TaskActionKind, target?: string, taskId?: string) => void;
  finishTaskAction: (kind?: TaskActionKind) => void;
  notifyTaskActionBlocked: (kind: TaskActionKind) => void;
}

export function createProbeTaskControl(options: ProbeTaskControlOptions) {
  const { task, taskSessionState, status, loading, appendLog, setStatus, showToast, pushActivity, navigateTo, reconcileTaskData, canCancelTask, hasDetachedTaskSnapshot, hasPausedTask, activeTaskSessionState, taskActionInFlight, beginTaskAction, finishTaskAction, notifyTaskActionBlocked } = options;

  async function pauseProbe() {
    if (!task.taskId) {
      return;
    }

    if (taskActionInFlight.value) {
      notifyTaskActionBlocked("pause");
      return;
    }

    beginTaskAction("pause", "", task.taskId);

    loading.value = true;

    try {
      const result = await stopProbe({
        mode: "pause",
        task_id: task.taskId,
      });
      appendLog("bridge.stop_probe", result);

      if (!result.ok) {
        finishTaskAction("pause");
        setStatus({
          detail: result.message || "暂停失败。",
          title: "暂停失败",
          tone: "failed",
        });
        showToast("暂停失败", "error");
        return;
      }

      setStatus({
        detail: result.message || "已请求暂停，等待 cooling 事件确认。",
        title: "暂停请求已发送",
        tone: "running",
      });
      pushActivity("请求暂停", result.message || "已向桌面端发送暂停请求。");
      showToast("已请求暂停", "info");
    } finally {
      if (status.tone === "failed") {
        finishTaskAction("pause");
      }
      loading.value = false;
    }
  }

  async function cancelProbe() {
    if (!task.taskId || !canCancelTask.value) {
      return;
    }
    if (!window.confirm("确定要停止这次测速吗？已经得到的结果会保留，但未完成的项目不会继续测速。")) {
      return;
    }

    beginTaskAction("cancel", "", task.taskId);
    loading.value = true;

    try {
      const result = await stopProbe({
        mode: "cancel",
        task_id: task.taskId,
      });
      appendLog("bridge.stop_probe.cancel", result);
      const pending = result.code === "PROBE_CANCEL_PENDING";

      if (!result.ok && !pending) {
        finishTaskAction("cancel");
        setStatus({
          detail: result.message || "终止任务失败。",
          title: "终止失败",
          tone: "failed",
        });
        showToast("终止任务失败", "error");
        return;
      }

      setStatus({
        detail: result.message || "终止请求已发送，正在等待任务退出。",
        title: "正在终止任务",
        tone: "warning",
      });
      pushActivity("请求终止", result.message || "终止请求已发送，正在等待任务退出。");
      showToast("已请求终止任务", "info");
    } catch (error) {
      finishTaskAction("cancel");
      const message = error instanceof Error ? error.message : "终止任务失败。";
      setStatus({ detail: message, title: "终止失败", tone: "failed" });
      showToast(message, "error");
    } finally {
      loading.value = false;
    }
  }

  async function stopPausedTaskForRestart(reason: string) {
    if (!task.taskId || !hasPausedTask.value) {
      return true;
    }
    if (taskActionInFlight.value) {
      notifyTaskActionBlocked("cancel");
      return false;
    }

    beginTaskAction("cancel", "", task.taskId);
    loading.value = true;

    try {
      const result = await stopProbe({
        mode: "cancel",
        task_id: task.taskId,
      });
      appendLog("bridge.stop_probe.cancel", result);

      if (!result.ok) {
        finishTaskAction("cancel");
        setStatus({
          detail: result.message || "终止暂停任务失败。",
          title: "终止失败",
          tone: "failed",
        });
        showToast("终止暂停任务失败", "error");
        return false;
      }

      await reconcileTaskData(task.taskId);
      if (task.active || ["active_runtime", "paused_runtime"].includes(activeTaskSessionState.value)) {
        finishTaskAction("cancel");
        setStatus({
          detail: "终止请求已发送，但旧任务尚未完全退出，请稍后重试。",
          title: "等待旧任务退出",
          tone: "warning",
        });
        pushActivity("终止等待中", "旧任务仍在退出流程中，本次重启已取消。");
        showToast("旧任务仍在退出，请稍后重试", "info");
        return false;
      }

      finishTaskAction("cancel");
      setStatus({
        detail: result.message || reason,
        title: "已终止暂停任务",
        tone: "warning",
      });
      pushActivity("已终止暂停任务", result.message || reason);
      return true;
    } finally {
      loading.value = false;
    }
  }

  async function continueProbe() {
    if (!task.taskId) {
      return;
    }

    if (taskActionInFlight.value) {
      notifyTaskActionBlocked("resume");
      return;
    }

    if (hasDetachedTaskSnapshot.value) {
      setStatus({
        detail: "当前只恢复了历史快照和结果文件，原生运行时会话已经结束，不能直接继续，请重新启动任务。",
        title: "无法继续历史快照",
        tone: "warning",
      });
      pushActivity("继续被阻止", "仅恢复了历史结果，当前没有可继续的运行时会话。");
      showToast("历史快照不能直接继续", "info");
      void navigateTo("results");
      return;
    }

    beginTaskAction("resume", "", task.taskId);

    loading.value = true;

    try {
      const result = await resumeProbe({
        task_id: task.taskId,
      });
      appendLog("bridge.resume_probe", result);

      if (!result.ok) {
        finishTaskAction("resume");
        setStatus({
          detail: result.message || "继续失败。",
          title: "继续失败",
          tone: "failed",
        });
        showToast("继续失败", "error");
        return;
      }

      setStatus({
        detail: result.message || "已请求继续，等待新的 progress 事件。",
        title: "继续请求已发送",
        tone: "running",
      });
      taskSessionState.value = "active_runtime";
      task.active = true;
      pushActivity("请求继续", result.message || "已向桌面端发送继续请求。");
      showToast("已请求继续", "info");
      await reconcileTaskData(task.taskId);
    } finally {
      if (status.tone === "failed") {
        finishTaskAction("resume");
      }
      loading.value = false;
    }
  }

  return { pauseProbe, cancelProbe, continueProbe, stopPausedTaskForRestart };
}
