// 任务启动动作（批量启动 / 单条重测 / 复制地址）。
import type { ComputedRef, Ref } from "vue";
import { startProbe, type SourceConfig, type SourceIPMode, type TaskTone } from "../lib/bridge";
import { asRecord, asString } from "../lib/app/valueCoerce";
import type { TaskActionKind } from "../lib/app/labels";
import type { ViewName } from "../lib/app/types";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../lib/config/snapshotBuild";
import type { ProbeTaskCoreState } from "./probeTaskDeps";

export interface ProbeTaskLaunchOptions extends ProbeTaskCoreState {
  loading: Ref<boolean>;
  probeWarnings: Ref<string[]>;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  navigateTo: (view: ViewName) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  appendLog: (event: string, payload: unknown) => void;
  pushProcessTrace: (entry: { detail: string; stage: string; title: string; tone: "success" | "error" | "running" | "info" | "warning"; ts: string }) => void;
  pushWarningTrace: (warnings: string[], ts?: string) => void;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  reconcileTaskData: (taskId?: string, options?: { switchToResultsOnData?: boolean }) => Promise<void>;
  resetDownloadSpeedState: () => void;
  captureDownloadSpeedTaskConfig: (config: ReturnType<typeof buildConfigSnapshotFrom>) => void;
  clearCurrentTaskResultWorkspace: (options?: { preserveSnapshot?: boolean }) => void;
  clearResultRows: () => void;
  preparedSources: () => SourceConfig[];
  sourcePayloads: () => SourceConfig[];
  applySourceStatuses: (statusesValue: unknown) => void;
  pushActivity: (title: string, detail: string) => void;
  taskActionInFlight: ComputedRef<boolean>;
  hasActiveTask: ComputedRef<boolean>;
  hasPausedTask: ComputedRef<boolean>;
  beginTaskAction: (kind: TaskActionKind, target?: string, taskId?: string) => void;
  finishTaskAction: (kind?: TaskActionKind) => void;
  resetProbeSummary: () => void;
  allocateTaskId: () => string;
  notifyActiveProbeBlocked: (title: string) => void;
  notifyTaskActionBlocked: (kind: TaskActionKind) => void;
  stopPausedTaskForRestart: (reason: string) => Promise<boolean>;
}

export function createProbeTaskLaunch(options: ProbeTaskLaunchOptions) {
  const {
    task,
    taskSnapshot,
    taskSessionState,
    status,
    loading,
    probeWarnings,
    showToast,
    navigateTo,
    setStatus,
    appendLog,
    pushProcessTrace,
    pushWarningTrace,
    buildConfigSnapshot,
    reconcileTaskData,
    resetDownloadSpeedState,
    captureDownloadSpeedTaskConfig,
    clearCurrentTaskResultWorkspace,
    clearResultRows,
    preparedSources,
    sourcePayloads,
    applySourceStatuses,
    pushActivity,
    taskActionInFlight,
    hasActiveTask,
    hasPausedTask,
    beginTaskAction,
    finishTaskAction,
    resetProbeSummary,
    allocateTaskId,
    notifyActiveProbeBlocked,
    notifyTaskActionBlocked,
    stopPausedTaskForRestart,
  } = options;

  async function launchProbe() {
    if (taskActionInFlight.value) {
      notifyTaskActionBlocked("start");
      return;
    }
    if (hasActiveTask.value && !hasPausedTask.value) {
      notifyActiveProbeBlocked("启动任务被拦截");
      return;
    }

    if (preparedSources().length === 0) {
      setStatus({
        detail: "当前还不能开始测速：请先到“输入源”添加一个已启用的来源，并填写 URL、文件路径或 IP 内容。",
        title: "缺少输入源",
        tone: "failed",
      });
      showToast("请先配置至少一个输入源", "error");
      return;
    }
    if (hasPausedTask.value) {
      const stopped = await stopPausedTaskForRestart("启动新任务前需要先终止暂停中的任务。");
      if (!stopped) {
        return;
      }
    }

    beginTaskAction("start");
    loading.value = true;
    resetProbeSummary();
    resetDownloadSpeedState();
    clearCurrentTaskResultWorkspace();
    taskSessionState.value = "active_runtime";
    const taskId = allocateTaskId();
    task.acceptedAt = new Date().toISOString();
    task.active = true;
    task.completedAt = "";
    task.exportPath = "";
    task.lastEvent = "probe.accepted";
    task.lastSeq = 0;
    task.stage = "accepted";
    task.taskId = taskId;
    setStatus({
      detail: "正在准备输入源并启动桌面探测任务。",
      title: "任务提交中",
      tone: "preparing",
    });
    pushProcessTrace({
      detail: `任务 ${taskId} 已提交，等待原生探测引擎开始执行。`,
      stage: "accepted",
      title: "探测任务已提交",
      tone: "info",
      ts: task.acceptedAt,
    });
    pushActivity("任务提交中", `${taskId} 正在等待原生探测引擎处理。`);
    void navigateTo("dashboard");

    try {
      const launchConfig = buildConfigSnapshot();
      captureDownloadSpeedTaskConfig(launchConfig);
      const result = await startProbe({
        config: launchConfig,
        sources: sourcePayloads(),
        task_id: taskId,
      });
      const data = asRecord(result.data);
      appendLog("bridge.start_probe", result);
      probeWarnings.value = result.warnings || [];
      pushWarningTrace(probeWarnings.value);
      if (!result.ok) {
        finishTaskAction("start");
        task.active = false;
        task.completedAt = new Date().toISOString();
        pushProcessTrace({
          detail: result.message || "桌面端未接受探测任务。",
          stage: "failed",
          title: "任务启动失败",
          tone: "error",
          ts: task.completedAt,
        });
        if (status.tone !== "failed") {
          setStatus({
            detail: result.message || "启动探测失败。",
            title: "启动失败",
            tone: "failed",
          });
        }
        pushActivity("启动失败", result.message || "桌面端未接受探测任务。");
        if (status.tone !== "failed") {
          showToast("启动探测失败", "error");
        }
        return;
      }

      applySourceStatuses(data.source_statuses || data.sourceStatuses);
      task.exportPath = asString(data.export_path || task.exportPath).trim();
      task.taskId = asString(result.task_id || data.task_id || task.taskId).trim();
      void reconcileTaskData(task.taskId || taskId);
    } finally {
      finishTaskAction("start");
      loading.value = false;
    }
  }

  async function rerunSingleAddress(address: string) {
    const trimmedAddress = address.trim();

    if (!trimmedAddress) {
      return;
    }

    if (taskActionInFlight.value) {
      notifyTaskActionBlocked("rerun");
      return;
    }

    if (hasActiveTask.value && !hasPausedTask.value) {
      notifyActiveProbeBlocked("单条重测被拦截");
      return;
    }
    if (hasPausedTask.value) {
      const stopped = await stopPausedTaskForRestart("单条重测前需要先终止暂停中的任务。");
      if (!stopped) {
        return;
      }
    }

    beginTaskAction("rerun", trimmedAddress);
    loading.value = true;
    resetProbeSummary();
    resetDownloadSpeedState();
    clearResultRows();
    taskSnapshot.value = null;
    taskSessionState.value = "active_runtime";
    const taskId = allocateTaskId();
    task.acceptedAt = new Date().toISOString();
    task.active = true;
    task.completedAt = "";
    task.exportPath = "";
    task.lastEvent = "probe.accepted";
    task.lastSeq = 0;
    task.stage = "accepted";
    task.taskId = taskId;
    setStatus({
      detail: `${trimmedAddress} 正在提交到桌面探测引擎。`,
      title: "单条重测提交中",
      tone: "preparing",
    });
    pushProcessTrace({
      detail: `${trimmedAddress} 已提交为单条重测任务。`,
      stage: "accepted",
      title: "单条重测已提交",
      tone: "info",
      ts: task.acceptedAt,
    });
    void navigateTo("dashboard");

    try {
      const launchConfig = buildConfigSnapshot();
      captureDownloadSpeedTaskConfig(launchConfig);
      const result = await startProbe({
        config: launchConfig,
        sources: [
          {
            content: trimmedAddress,
            enabled: true,
            id: `rerun-${Date.now()}`,
            ip_limit: -1,
            ip_mode: "traverse" as SourceIPMode,
            kind: "inline",
            last_fetched_at: "",
            last_fetched_count: 0,
            name: `单条重测 ${trimmedAddress}`,
            path: "",
            status_text: "",
            url: "",
          },
        ],
        task_id: taskId,
      });
      const data = asRecord(result.data);
      appendLog("bridge.start_probe.single", result);
      probeWarnings.value = result.warnings || [];
      pushWarningTrace(probeWarnings.value);

      if (!result.ok) {
        finishTaskAction("rerun");
        task.active = false;
        task.completedAt = new Date().toISOString();
        pushProcessTrace({
          detail: result.message || "单条重测未能启动。",
          stage: "failed",
          title: "单条重测失败",
          tone: "error",
          ts: task.completedAt,
        });
        if (status.tone !== "failed") {
          setStatus({
            detail: result.message || "单条重测启动失败。",
            title: "重测失败",
            tone: "failed",
          });
          showToast("单条重测失败", "error");
        }
        return;
      }

      task.exportPath = asString(data.export_path || task.exportPath).trim();
      task.taskId = asString(result.task_id || data.task_id || task.taskId).trim();
      void reconcileTaskData(task.taskId || taskId);
    } finally {
      finishTaskAction("rerun");
      loading.value = false;
    }
  }

  async function copyAddress(address: string) {
    const trimmedAddress = address.trim();

    if (!trimmedAddress) {
      return;
    }

    try {
      await navigator.clipboard.writeText(trimmedAddress);
      showToast("IP 已复制", "success");
    } catch (_error) {
      showToast("复制失败，请手动选择该地址", "error");
    }
  }

  return { launchProbe, rerunSingleAddress, copyAddress };
}
