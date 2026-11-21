// 探测任务域编排（重构计划 §4.2）：状态与派生能力在这里装配；
// 启动动作见 probeTaskLaunch，暂停/终止/继续见 probeTaskControl。
import { computed, reactive, ref } from "vue";
import type { TaskSnapshot, TaskTone } from "../lib/bridge";
import { taskActionLabel, taskToneLabel, type TaskActionKind } from "../lib/labels";
import { asBoolean, asCount, asString } from "../lib/valueCoerce";
import { createProbeTaskControl } from "./probeTaskControl";
import { createProbeTaskLaunch } from "./probeTaskLaunch";
import type { MCISProgressState, ProbeTaskDeps } from "./probeTaskDeps";

export type { MCISProgressState, ProbeTaskDeps } from "./probeTaskDeps";

export function useProbeTask(deps: ProbeTaskDeps) {
  const {
    loading,
    probeWarnings,
    pushActivity,
    showToast,
    navigateTo,
    setStatus,
    appendLog,
    pushProcessTrace,
    pushWarningTrace,
    clearProcessTrace,
    buildConfigSnapshot,
    reconcileTaskData,
    resetDownloadSpeedState,
    captureDownloadSpeedTaskConfig,
    clearCurrentTaskResultWorkspace,
    clearResultRows,
    preparedSources,
    sourcePayloads,
    applySourceStatuses,
  } = deps;
  const taskSnapshot = ref<TaskSnapshot | null>(null);
  const taskSessionState = ref("idle");
  const status = reactive({
    detail: "先读取配置，再决定启动探测任务或读取 DNS 记录。",
    title: "就绪",
    tone: "idle" as TaskTone,
  });
  const summary = reactive({
    accepted: 0,
    exported: 0,
    failed: 0,
    filtered: 0,
    invalid: 0,
    passed: 0,
    processed: 0,
    total: 0,
  });
  const mcisProgress = reactive<MCISProgressState>({
    active: false,
    candidateCount: 0,
    completed: 0,
    concurrency: 0,
    elapsedMs: 0,
    failed: 0,
    lastColo: "",
    lastIP: "",
    lastOK: false,
    sourceID: "",
    sourceName: "",
    succeeded: 0,
    total: 0,
  });
  const task = reactive({
    acceptedAt: "",
    active: false,
    completedAt: "",
    exportPath: "",
    lastEvent: "",
    lastSeq: 0,
    stage: "idle",
    taskId: "",
  });
  const taskActionState = reactive<{
    kind: TaskActionKind | "";
    taskId: string;
    target: string;
  }>({
    kind: "",
    target: "",
    taskId: "",
  });
  // 冷启动的乐观 UI 期间为 true：此时看板展示的是上次缓存的展示数据，不是刚读到的运行态。
  // 任务动作全部禁用，避免用户对着过期状态点「启动 / 终止」（真实状态由 App.vue 的启动同步收尾）。
  const startupSyncing = ref(false);

  const dashboardStatusLabel = computed(() => taskToneLabel(status.tone));
  const progressPercent = computed(() => {
    if (mcisProgress.active && task.stage === "stage0_mcis" && mcisProgress.total > 0) {
      return Math.max(0, Math.min(100, Math.round((mcisProgress.completed / mcisProgress.total) * 100)));
    }
    const total = summary.total > 0 ? summary.total : summary.accepted + summary.filtered + summary.invalid > 0 ? summary.accepted + summary.filtered + summary.invalid : summary.accepted;
    if (total <= 0) {
      return 0;
    }
    return Math.max(0, Math.min(100, Math.round((summary.processed / total) * 100)));
  });
  const hasActiveTask = computed(() => Boolean(task.taskId) && task.active);
  const activeTaskSessionState = computed(() => {
    const snapshotState = String(taskSnapshot.value?.session_state || "").trim();
    const runtimeState = String(taskSessionState.value || "").trim();
    if (runtimeState === "active_runtime" || runtimeState === "paused_runtime") {
      return runtimeState;
    }
    return snapshotState || runtimeState || "idle";
  });
  const taskActionInFlight = computed(() => Boolean(taskActionState.kind));
  const hasDetachedTaskSnapshot = computed(() => activeTaskSessionState.value === "persisted_only");
  const hasPausedTask = computed(() => activeTaskSessionState.value === "paused_runtime");
  const canCancelTask = computed(() => !startupSyncing.value && hasActiveTask.value && !taskActionInFlight.value && !hasDetachedTaskSnapshot.value);
  const canPauseTask = computed(() => !startupSyncing.value && hasActiveTask.value && !taskActionInFlight.value && !hasPausedTask.value && task.stage !== "accepted");
  const canResumeTask = computed(() => !startupSyncing.value && Boolean(task.taskId) && !taskActionInFlight.value && !hasDetachedTaskSnapshot.value && (taskSnapshot.value?.resume_capable === true || hasPausedTask.value));
  const canStartTask = computed(() => !startupSyncing.value && !taskActionInFlight.value && (!hasActiveTask.value || hasPausedTask.value));

  function beginTaskAction(kind: TaskActionKind, target = "", taskId = task.taskId) {
    taskActionState.kind = kind;
    taskActionState.target = target;
    taskActionState.taskId = taskId.trim();
  }

  function finishTaskAction(kind?: TaskActionKind) {
    if (kind && taskActionState.kind && taskActionState.kind !== kind) {
      return;
    }
    taskActionState.kind = "";
    taskActionState.target = "";
    taskActionState.taskId = "";
  }

  const ACTIVE_PROBE_MESSAGE = "当前已有探测任务运行或暂停，请完成后再启动新任务。";

  function notifyActiveProbeBlocked(title: string) {
    pushActivity(title, ACTIVE_PROBE_MESSAGE);
    showToast("已有任务运行中", "error");
    void navigateTo("dashboard");
  }

  function notifyTaskActionBlocked(kind: TaskActionKind) {
    const actionLabel = taskActionLabel(kind);
    const blockingLabel = taskActionState.kind ? taskActionLabel(taskActionState.kind as TaskActionKind) : "任务";
    const detail = `${blockingLabel}操作仍在处理中，请等待当前请求完成后再${actionLabel}任务。`;
    setStatus({
      detail,
      title: `${actionLabel}被拦截`,
      tone: "warning",
    });
    pushActivity(`${actionLabel}被拦截`, detail);
    showToast(`请勿重复${actionLabel}`, "info");
    void navigateTo("dashboard");
  }

  function updateMCISProgress(payload: Record<string, unknown>, active = true) {
    mcisProgress.active = active;
    mcisProgress.candidateCount = asCount(payload.candidate_count, mcisProgress.candidateCount);
    mcisProgress.completed = asCount(payload.completed, mcisProgress.completed);
    mcisProgress.concurrency = asCount(payload.concurrency, mcisProgress.concurrency);
    mcisProgress.elapsedMs = asCount(payload.elapsed_ms, mcisProgress.elapsedMs);
    mcisProgress.failed = asCount(payload.failed, mcisProgress.failed);
    mcisProgress.lastColo = payload.last_colo === undefined ? mcisProgress.lastColo : asString(payload.last_colo).trim();
    mcisProgress.lastIP = payload.last_ip === undefined ? mcisProgress.lastIP : asString(payload.last_ip).trim();
    mcisProgress.lastOK = asBoolean(payload.last_ok, mcisProgress.lastOK);
    mcisProgress.sourceID = payload.source_id === undefined ? mcisProgress.sourceID : asString(payload.source_id).trim();
    mcisProgress.sourceName = payload.source_name === undefined ? mcisProgress.sourceName : asString(payload.source_name).trim();
    mcisProgress.succeeded = asCount(payload.succeeded, mcisProgress.succeeded);
    mcisProgress.total = asCount(payload.total, mcisProgress.total);
  }

  function resetMCISProgress() {
    updateMCISProgress(
      {
        candidate_count: 0,
        completed: 0,
        concurrency: 0,
        elapsed_ms: 0,
        failed: 0,
        last_colo: "",
        last_ip: "",
        last_ok: false,
        source_id: "",
        source_name: "",
        succeeded: 0,
        total: 0,
      },
      false,
    );
  }

  function resetProbeSummary() {
    summary.accepted = 0;
    summary.exported = 0;
    summary.failed = 0;
    summary.filtered = 0;
    summary.invalid = 0;
    summary.passed = 0;
    summary.processed = 0;
    summary.total = 0;
    resetMCISProgress();
    clearProcessTrace();
  }

  function allocateTaskId() {
    return `cfst-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
  }

  const coreState = { status, summary, task, taskSessionState, taskSnapshot };
  const taskFlags = { activeTaskSessionState, canCancelTask, hasActiveTask, hasDetachedTaskSnapshot, hasPausedTask, taskActionInFlight };
  const taskHelpers = { beginTaskAction, finishTaskAction, notifyActiveProbeBlocked, notifyTaskActionBlocked };

  const control = createProbeTaskControl({
    ...coreState,
    ...taskFlags,
    ...taskHelpers,
    appendLog,
    loading,
    navigateTo,
    pushActivity,
    reconcileTaskData,
    setStatus,
    showToast,
  });

  const launch = createProbeTaskLaunch({
    ...coreState,
    ...taskFlags,
    ...taskHelpers,
    applySourceStatuses,
    appendLog,
    buildConfigSnapshot,
    captureDownloadSpeedTaskConfig,
    clearCurrentTaskResultWorkspace,
    clearResultRows,
    loading,
    navigateTo,
    preparedSources,
    probeWarnings,
    pushActivity,
    pushProcessTrace,
    pushWarningTrace,
    reconcileTaskData,
    resetDownloadSpeedState,
    resetProbeSummary,
    allocateTaskId,
    setStatus,
    showToast,
    sourcePayloads,
    stopPausedTaskForRestart: control.stopPausedTaskForRestart,
  });

  return {
    ...control,
    ...launch,
    activeTaskSessionState,
    allocateTaskId,
    beginTaskAction,
    canCancelTask,
    canPauseTask,
    canResumeTask,
    canStartTask,
    dashboardStatusLabel,
    finishTaskAction,
    hasActiveTask,
    hasDetachedTaskSnapshot,
    hasPausedTask,
    mcisProgress,
    notifyActiveProbeBlocked,
    notifyTaskActionBlocked,
    progressPercent,
    resetMCISProgress,
    resetProbeSummary,
    startupSyncing,
    status,
    summary,
    task,
    taskActionInFlight,
    taskActionState,
    taskSessionState,
    taskSnapshot,
    updateMCISProgress,
  };
}
