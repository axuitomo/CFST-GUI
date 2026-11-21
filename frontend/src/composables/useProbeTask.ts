import { computed, reactive, ref, type Ref } from "vue";
import { resumeProbe, startProbe, stopProbe, type SourceConfig, type SourceIPMode, type TaskSnapshot, type TaskTone } from "../lib/bridge";
import { taskActionLabel, taskToneLabel, type TaskActionKind } from "../lib/labels";
import { asBoolean, asCount, asRecord, asString } from "../lib/valueCoerce";
import type { ProcessTraceEntry, ToastTone, ViewName } from "../lib/appTypes";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../lib/configSnapshotBuild";

export interface MCISProgressState {
  active: boolean;
  candidateCount: number;
  completed: number;
  concurrency: number;
  elapsedMs: number;
  failed: number;
  lastColo: string;
  lastIP: string;
  lastOK: boolean;
  sourceID: string;
  sourceName: string;
  succeeded: number;
  total: number;
}

export interface ProbeTaskDeps {
  loading: Ref<boolean>;
  probeWarnings: Ref<string[]>;
  pushActivity: (title: string, detail: string) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  navigateTo: (view: ViewName) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  appendLog: (event: string, payload: unknown) => void;
  pushProcessTrace: (entry: Omit<ProcessTraceEntry, "id">) => void;
  pushWarningTrace: (warnings: string[], ts?: string) => void;
  clearProcessTrace: () => void;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  reconcileTaskData: (taskId?: string, options?: { switchToResultsOnData?: boolean }) => Promise<void>;
  resetDownloadSpeedState: () => void;
  captureDownloadSpeedTaskConfig: (config: ReturnType<typeof buildConfigSnapshotFrom>) => void;
  clearCurrentTaskResultWorkspace: (options?: { preserveSnapshot?: boolean }) => void;
  clearResultRows: () => void;
  preparedSources: () => SourceConfig[];
  sourcePayloads: () => SourceConfig[];
  applySourceStatuses: (statusesValue: unknown) => void;
}

export function useProbeTask(deps: ProbeTaskDeps) {
  const { loading, probeWarnings, pushActivity, showToast, navigateTo, setStatus, appendLog, pushProcessTrace, pushWarningTrace, clearProcessTrace, buildConfigSnapshot, reconcileTaskData, resetDownloadSpeedState, captureDownloadSpeedTaskConfig, clearCurrentTaskResultWorkspace, clearResultRows, preparedSources, sourcePayloads, applySourceStatuses } = deps;
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

  return {
    activeTaskSessionState,
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
    progressPercent,
    status,
    summary,
    task,
    taskActionInFlight,
    taskActionState,
    startupSyncing,
    taskSessionState,
    taskSnapshot,
    launchProbe,
    rerunSingleAddress,
    copyAddress,
    pauseProbe,
    cancelProbe,
    continueProbe,
    stopPausedTaskForRestart,
    updateMCISProgress,
    resetMCISProgress,
    resetProbeSummary,
    allocateTaskId,
    notifyActiveProbeBlocked,
    notifyTaskActionBlocked,
  };
}
