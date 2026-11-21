// 探测事件应用域（重构计划 §4.2）：applyProbeEvent 原样迁出，特征测试锁定输入→输出。
// 任务/进度/测速面板等状态经 deps 注入，本模块只负责事件→状态映射。
import type { Ref } from "vue";
import { deriveTaskStateFromProbeEvent, type ProbeEventEnvelope, type ProbeResult, type TaskSnapshot, type TaskTone } from "./bridge";
import { classifyProbeEventSequence } from "./probeEventSequence";
import { clearDownloadSpeedRows, downloadSpeedRowTtlMs, upsertDownloadSpeedRow, type DownloadSpeedRow } from "./downloadSpeedRows";
import { stageTitle, type TaskActionKind } from "./labels";
import { asBoolean, asCount, asNumber, asString } from "./valueCoerce";
import type { HistoryEntry, ProcessTraceEntry, ToastTone } from "./appTypes";

export interface ProbeEventApplyDeps {
  task: { active: boolean; completedAt: string; exportPath: string; lastEvent: string; lastSeq: number; stage: string; taskId: string };
  taskSessionState: Ref<string>;
  activeTaskSessionState: Ref<string>;
  taskSnapshot: Ref<TaskSnapshot | null>;
  status: { detail: string; title: string; tone: TaskTone };
  summary: { accepted: number; exported: number; failed: number; filtered: number; invalid: number; passed: number; processed: number; total: number };
  mcisProgress: { active: boolean; candidateCount: number; completed: number; failed: number; lastColo: string; lastIP: string; sourceName: string; succeeded: number; total: number };
  finishTaskAction: (kind?: TaskActionKind) => void;
  downloadSpeedState: { active: boolean; averageSpeedMbS: number | null; bytesRead: number; colo: string; currentSpeedMbS: number | null; elapsedMs: number; ip: string };
  downloadSpeedRows: DownloadSpeedRow[];
  activeDownloadConcurrency: Ref<number>;
  activeDownloadSampleIntervalMs: Ref<number>;
  resetDownloadSpeedState: () => void;
  ensureDownloadSpeedRowPruneTimer: () => void;
  stopDownloadSpeedRowPruneTimer: () => void;
  clearDownloadSpeedTraceEntries: () => void;
  updateDownloadSpeedTrace: (ip: string, colo: string, ts: string) => void;
  applySourceStatuses: (statusesValue: unknown) => void;
  resultRows: Ref<ProbeResult[]>;
  taskResultCSVPath: Ref<string>;
  probeWarnings: Ref<string[]>;
  updateHistory: (entry: HistoryEntry) => void;
  shouldRefreshProgressUI: (stage: string, completed: number, total: number) => boolean;
  updateMCISProgress: (payload: Record<string, unknown>, active?: boolean) => void;
  pushProcessTrace: (entry: Omit<ProcessTraceEntry, "id">) => void;
  pushWarningTrace: (warnings: string[], ts?: string) => void;
  summarizeFailureSummary: (summaryValue: unknown) => string;
  edgeRejectedItems: (event: ProbeEventEnvelope) => Array<Record<string, unknown>>;
  notifyUploadResult: (event: ProbeEventEnvelope) => void;
  eventDebugLogDisplayPath: (payload: Record<string, unknown>) => string;
  eventDebugLogOpenTarget: (payload: Record<string, unknown>) => string;
  eventTraceFailureSummary: (payload: Record<string, unknown>) => string;
  reconcileTaskData: (taskId?: string, options?: { switchToResultsOnData?: boolean }) => Promise<void>;
  isAndroidApp: () => boolean;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  pushActivity: (title: string, detail: string) => void;
}

interface ProbeEventContext {
  event: ProbeEventEnvelope;
  incomingTaskId: string;
  nextTaskState: ReturnType<typeof deriveTaskStateFromProbeEvent>;
  refreshProgressUI: boolean;
  eventDebugLogPath: string;
  eventDebugLogTarget: string;
  traceFailureSummary: string;
}

function handleMcisProgress(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, mcisProgress, finishTaskAction, resetDownloadSpeedState, updateMCISProgress, pushProcessTrace } = deps;
  const { event, refreshProgressUI } = ctx;
  finishTaskAction("start");
  finishTaskAction("rerun");
  taskSessionState.value = "active_runtime";
  task.stage = "stage0_mcis";
  if (refreshProgressUI) {
    resetDownloadSpeedState();
    updateMCISProgress(event.payload);
    const sourceLabel = mcisProgress.sourceName || "当前输入源";
    const latestLabel = mcisProgress.lastIP ? `，最近 ${mcisProgress.lastIP}${mcisProgress.lastColo ? `(${mcisProgress.lastColo})` : ""}` : "";
    pushProcessTrace({
      detail: `${sourceLabel}，已完成 ${mcisProgress.completed}/${mcisProgress.total || "-"}，成功 ${mcisProgress.succeeded}，失败/超时 ${mcisProgress.failed}，候选 ${mcisProgress.candidateCount}${latestLabel}。`,
      stage: "stage0_mcis",
      title: "MICS 抽样进行中",
      tone: "running",
      ts: event.ts,
    });
  }
}

function handlePreprocessed(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, summary, mcisProgress, finishTaskAction, resetDownloadSpeedState, applySourceStatuses, pushProcessTrace } = deps;
  const { event } = ctx;
  finishTaskAction("start");
  finishTaskAction("rerun");
  taskSessionState.value = "active_runtime";
  resetDownloadSpeedState();
  mcisProgress.active = false;
  summary.accepted = asCount(event.payload.accepted);
  summary.filtered = asCount(event.payload.filtered);
  summary.invalid = asCount(event.payload.invalid);
  summary.processed = 0;
  summary.passed = 0;
  summary.failed = 0;
  summary.total = asCount(event.payload.total, summary.accepted);
  task.stage = "stage0_pool";
  applySourceStatuses(event.payload.source_statuses);
  pushProcessTrace({
    detail: `候选 ${summary.total} 条，接受 ${summary.accepted} 条，过滤 ${summary.filtered} 条，非法 ${summary.invalid} 条。`,
    stage: "stage0_pool",
    title: "阶段0 IP池完成",
    tone: summary.accepted > 0 ? "success" : "warning",
    ts: event.ts,
  });
}

function handleProgress(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, summary, mcisProgress, finishTaskAction, resetDownloadSpeedState, pushProcessTrace } = deps;
  const { event, refreshProgressUI } = ctx;
  finishTaskAction("start");
  finishTaskAction("rerun");
  finishTaskAction("resume");
  taskSessionState.value = "active_runtime";
  mcisProgress.active = false;
  task.stage = asString(event.payload.stage) || "running";
  const completed = asCount(event.payload.processed);
  if (refreshProgressUI) {
    summary.failed = asCount(event.payload.failed);
    summary.passed = asCount(event.payload.passed);
    summary.processed = completed;
    summary.total = asCount(event.payload.total, summary.total);
    if (task.stage !== "stage3_get") {
      resetDownloadSpeedState();
    }
    const progressPrefix = task.stage === "stage3_get" ? "文件测速" : "";
    pushProcessTrace({
      detail: `${progressPrefix ? `${progressPrefix}，` : ""}已处理 ${summary.processed}/${summary.total || "-"}，通过 ${summary.passed}，失败 ${summary.failed}。`,
      stage: task.stage,
      title: `${stageTitle(task.stage)}进行中`,
      tone: "running",
      ts: event.ts,
    });
  }
}

function handleResumed(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, finishTaskAction, pushProcessTrace } = deps;
  const { event } = ctx;
  finishTaskAction("resume");
  taskSessionState.value = "active_runtime";
  task.active = true;
  task.stage = asString(event.payload.stage || event.payload.current_stage || task.stage) || task.stage;
  pushProcessTrace({
    detail: asString(event.payload.message || "任务已恢复执行。"),
    stage: task.stage || "running",
    title: "任务继续执行",
    tone: "running",
    ts: event.ts,
  });
}

function handleSpeed(ctx: ProbeEventContext, deps: ProbeEventApplyDeps): boolean {
  const { task, taskSessionState, finishTaskAction, downloadSpeedState, downloadSpeedRows, activeDownloadConcurrency, activeDownloadSampleIntervalMs, resetDownloadSpeedState, ensureDownloadSpeedRowPruneTimer, updateDownloadSpeedTrace } = deps;
  const { event, refreshProgressUI } = ctx;
  finishTaskAction("start");
  finishTaskAction("rerun");
  finishTaskAction("resume");
  taskSessionState.value = "active_runtime";
  task.stage = asString(event.payload.stage) || "stage3_get";
  const ip = asString(event.payload.ip).trim() || "当前 IP";
  const currentSpeed = asNumber(event.payload.current_speed_mb_s, 0);
  const averageSpeed = asNumber(event.payload.average_speed_mb_s, 0);
  const currentReadyValue = event.payload.current_ready ?? event.payload.currentReady;
  const averageReadyValue = event.payload.average_ready ?? event.payload.averageReady;
  const currentReady = currentReadyValue === undefined ? true : asBoolean(currentReadyValue, false);
  const averageReady = averageReadyValue === undefined ? true : asBoolean(averageReadyValue, false);
  const bytesRead = asCount(event.payload.bytes_read, 0);
  const measuredBytes = asCount(event.payload.measured_bytes ?? event.payload.measuredBytes, 0);
  const bodyReadValue = event.payload.body_read ?? event.payload.bodyRead;
  const transferCompleteValue = event.payload.transfer_complete ?? event.payload.transferComplete;
  const bodyRead = bodyReadValue === undefined ? bytesRead > 0 : asBoolean(bodyReadValue, false);
  const transferComplete = transferCompleteValue === undefined ? false : asBoolean(transferCompleteValue, false);
  const elapsedMs = asCount(event.payload.elapsed_ms, 0);
  const colo = asString(event.payload.colo).trim();
  const hasReadyFlag = currentReadyValue !== undefined || averageReadyValue !== undefined;
  const isInitialEmptySample = elapsedMs === 0 && bytesRead === 0 && ((hasReadyFlag && !currentReady && !averageReady) || (!hasReadyFlag && currentSpeed === 0 && averageSpeed === 0 && !downloadSpeedState.ip));
  if (isInitialEmptySample) {
    return false;
  }
  // 单并发时保留原来“换 IP 就重置面板”的行为；多并发时所有 IP 共用一张表，不能互相清空。
  if (activeDownloadConcurrency.value <= 1 && downloadSpeedState.ip && downloadSpeedState.ip !== ip) {
    resetDownloadSpeedState();
  }
  // Android 的 200ms 进度节流只限制状态文案与进程日志；每 IP 的实时行必须逐采样更新。
  // 否则同一秒内并发采样的其它 IP 会全部落在节流窗口里，表里只剩第一颗 IP。
  downloadSpeedState.active = true;
  const averageDisplayReady = averageReady && (averageSpeed !== 0 || bytesRead > 0 || measuredBytes > 0 || bodyRead || transferComplete);
  const currentDisplaySpeed = currentReady ? currentSpeed : null;
  const averageDisplaySpeed = averageDisplayReady ? averageSpeed : null;
  downloadSpeedState.averageSpeedMbS = averageDisplaySpeed;
  downloadSpeedState.bytesRead = bytesRead;
  downloadSpeedState.colo = colo;
  downloadSpeedState.currentSpeedMbS = currentDisplaySpeed;
  downloadSpeedState.elapsedMs = elapsedMs;
  downloadSpeedState.ip = ip;
  const rowNow = Date.now();
  upsertDownloadSpeedRow(
    downloadSpeedRows,
    {
      averageSpeedMbS: averageDisplaySpeed,
      bytesRead,
      colo,
      currentSpeedMbS: currentDisplaySpeed,
      elapsedMs,
      ip,
      updatedAt: rowNow,
    },
    { now: rowNow, ttlMs: downloadSpeedRowTtlMs(activeDownloadSampleIntervalMs.value) },
  );
  ensureDownloadSpeedRowPruneTimer();
  if (refreshProgressUI) {
    updateDownloadSpeedTrace(ip, colo, event.ts);
  }
  return true;
}

function handleEdgeRejected(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { pushProcessTrace, edgeRejectedItems } = deps;
  const { event } = ctx;
  // 403 / 1034 淘汰不受 debug 开关约束：进程面板要能直接看到被边缘拒绝的 IP。
  // 后端按 250ms 窗口把同一批拒绝合并进 payload.items，早先的单条形态照样能读。
  for (const item of edgeRejectedItems(event)) {
    const note = asString(item.note).trim() || "403";
    const rejectedIp = asString(item.ip).trim();
    pushProcessTrace({
      detail: asString(item.message).trim() || `${rejectedIp || "当前 IP"} 被边缘拒绝（${note}）。`,
      stage: asString(item.stage).trim() || "stage3_get",
      title: note === "1034" ? "边缘 IP 受限 1034" : "HTTP 403 拒绝",
      tone: "warning",
      ts: event.ts,
    });
  }
}

function handlePartialExport(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, status, summary, taskResultCSVPath, updateHistory, pushProcessTrace } = deps;
  const { event, eventDebugLogPath } = ctx;
  taskSessionState.value = "active_runtime";
  summary.exported = asCount(event.payload.written, summary.exported);
  taskResultCSVPath.value = asString(event.payload.source_path || taskResultCSVPath.value).trim();
  task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
  updateHistory({
    debugLogPath: eventDebugLogPath,
    detail: status.detail,
    exported: summary.exported,
    failureSummary: "",
    targetPath: task.exportPath,
    taskId: task.taskId,
    title: status.title,
    tone: "partial",
    updatedAt: event.ts,
  });
  pushProcessTrace({
    detail: task.exportPath ? `已写出 ${summary.exported} 条结果到 ${task.exportPath}。` : `已整理 ${summary.exported} 条结果。`,
    stage: "export",
    title: "结果已落盘",
    tone: "success",
    ts: event.ts,
  });
}

function handleExportCompleted(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, summary, finishTaskAction, taskResultCSVPath, updateHistory, pushProcessTrace, showToast } = deps;
  const { event, nextTaskState, eventDebugLogPath, eventDebugLogTarget } = ctx;
  finishTaskAction();
  taskSessionState.value = "idle";
  task.active = false;
  summary.exported = asCount(event.payload.written, summary.exported);
  taskResultCSVPath.value = asString(event.payload.source_path || taskResultCSVPath.value).trim();
  task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
  updateHistory({
    debugLogPath: eventDebugLogPath,
    debugLogTarget: eventDebugLogTarget,
    detail: nextTaskState.detail,
    exported: summary.exported,
    failureSummary: "",
    targetPath: task.exportPath,
    taskId: task.taskId,
    title: nextTaskState.title,
    tone: "completed",
    updatedAt: event.ts,
  });
  pushProcessTrace({
    detail: task.exportPath ? `Android 系统导出已写出 ${summary.exported} 条结果到 ${task.exportPath}。` : `Android 系统导出已写出 ${summary.exported} 条结果。`,
    stage: "export",
    title: "系统导出完成",
    tone: "success",
    ts: event.ts,
  });
  showToast("Android 系统导出完成", "success");
}

function handleExportFailed(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, summary, finishTaskAction, updateHistory, pushProcessTrace, showToast } = deps;
  const { event, nextTaskState, eventDebugLogPath, eventDebugLogTarget } = ctx;
  finishTaskAction();
  taskSessionState.value = "idle";
  task.active = false;
  const exportFailureMessage = asString(event.payload.message || nextTaskState.detail).trim() || "Android 系统导出失败。";
  updateHistory({
    debugLogPath: eventDebugLogPath,
    debugLogTarget: eventDebugLogTarget,
    detail: exportFailureMessage,
    exported: summary.exported,
    failureSummary: "",
    targetPath: task.exportPath,
    taskId: task.taskId,
    title: "系统导出失败",
    tone: "warning",
    updatedAt: event.ts,
  });
  pushProcessTrace({
    detail: exportFailureMessage,
    stage: "export",
    title: "系统导出失败",
    tone: "warning",
    ts: event.ts,
  });
  showToast(exportFailureMessage, "error");
}

function handleCompleted(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, status, summary, finishTaskAction, resetDownloadSpeedState, resultRows, taskResultCSVPath, probeWarnings, updateHistory, pushProcessTrace, pushWarningTrace, summarizeFailureSummary, reconcileTaskData, isAndroidApp, showToast } = deps;
  const { event, incomingTaskId, eventDebugLogPath, eventDebugLogTarget, traceFailureSummary } = ctx;
  finishTaskAction();
  taskSessionState.value = "idle";
  task.active = false;
  task.completedAt = event.ts;
  resetDownloadSpeedState();
  summary.exported = asCount(event.payload.exported, summary.exported);
  summary.failed = Math.max(summary.failed, asCount(event.payload.failed, summary.failed));
  const resultCount = Math.max(asCount(event.payload.result_count, 0), asCount(event.payload.passed, 0), summary.passed, summary.exported, resultRows.value.length);
  summary.passed = Math.max(summary.passed, resultCount);
  taskResultCSVPath.value = asString(event.payload.source_path || taskResultCSVPath.value).trim();
  task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
  const completedWarnings = Array.isArray(event.payload.warnings) ? event.payload.warnings.map((entry) => asString(entry)).filter(Boolean) : [];
  if (completedWarnings.length > 0) {
    probeWarnings.value = completedWarnings;
    pushWarningTrace(completedWarnings, event.ts);
  }
  const hasResults = resultCount > 0;
  updateHistory({
    debugLogPath: eventDebugLogPath,
    debugLogTarget: eventDebugLogTarget,
    detail: status.detail,
    exported: summary.exported,
    failureSummary: summarizeFailureSummary(event.payload.failure_summary) || traceFailureSummary,
    targetPath: task.exportPath,
    taskId: task.taskId,
    title: status.title,
    tone: hasResults ? "completed" : "no_results",
    updatedAt: event.ts,
  });
  pushProcessTrace({
    detail: hasResults ? `任务完成，可用结果 ${resultCount} 条${task.exportPath ? `，导出路径 ${task.exportPath}` : ""}。` : "任务执行完成，但当前筛选条件下没有可用结果。",
    stage: "completed",
    title: hasResults ? "探测任务完成" : "任务完成但无结果",
    tone: hasResults ? "success" : "warning",
    ts: event.ts,
  });
  void reconcileTaskData(task.taskId || incomingTaskId, {
    switchToResultsOnData: hasResults || isAndroidApp(),
  });
  showToast(hasResults ? "探测任务已完成" : "任务结束但没有可用结果", hasResults ? "success" : "info");
}

function handleFailed(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, activeTaskSessionState, taskSnapshot, status, summary, finishTaskAction, resetDownloadSpeedState, taskResultCSVPath, updateHistory, pushProcessTrace, showToast } = deps;
  const { event, eventDebugLogPath, eventDebugLogTarget, traceFailureSummary } = ctx;
  finishTaskAction();
  taskSessionState.value = activeTaskSessionState.value === "persisted_only" || taskSnapshot.value?.session_state === "persisted_only" ? "persisted_only" : "idle";
  task.active = false;
  task.completedAt = event.ts;
  resetDownloadSpeedState();
  taskResultCSVPath.value = asString(event.payload.source_path || taskResultCSVPath.value).trim();
  task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
  const failureMessage = asString(status.detail || event.payload.message).trim() || "探测任务失败。";
  updateHistory({
    debugLogPath: eventDebugLogPath,
    debugLogTarget: eventDebugLogTarget,
    detail: failureMessage,
    exported: summary.exported,
    failureSummary: traceFailureSummary,
    targetPath: task.exportPath,
    taskId: task.taskId,
    title: status.title,
    tone: "failed",
    updatedAt: event.ts,
  });
  pushProcessTrace({
    detail: failureMessage,
    stage: "failed",
    title: "探测任务失败",
    tone: "error",
    ts: event.ts,
  });
  showToast(failureMessage, "error");
}

function handleCancelled(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, summary, finishTaskAction, resetDownloadSpeedState, updateHistory, pushProcessTrace, showToast } = deps;
  const { event, nextTaskState, eventDebugLogPath, eventDebugLogTarget } = ctx;
  finishTaskAction();
  taskSessionState.value = "persisted_only";
  task.active = false;
  task.completedAt = event.ts;
  resetDownloadSpeedState();
  const cancelMessage = asString(event.payload.message || nextTaskState.detail).trim() || "测速任务已终止。";
  updateHistory({
    debugLogPath: eventDebugLogPath,
    debugLogTarget: eventDebugLogTarget,
    detail: cancelMessage,
    exported: summary.exported,
    failureSummary: "",
    targetPath: task.exportPath,
    taskId: task.taskId,
    title: "任务已终止",
    tone: "cancelled",
    updatedAt: event.ts,
  });
  pushProcessTrace({
    detail: cancelMessage,
    stage: "cancelled",
    title: "探测任务已终止",
    tone: "warning",
    ts: event.ts,
  });
  showToast("探测任务已终止", "info");
}

function handleCooling(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, taskSessionState, finishTaskAction, downloadSpeedState, downloadSpeedRows, stopDownloadSpeedRowPruneTimer, clearDownloadSpeedTraceEntries, pushProcessTrace } = deps;
  const { event } = ctx;
  const recoverable = asBoolean(event.payload.recoverable, true);
  if (recoverable) {
    finishTaskAction("pause");
    taskSessionState.value = "paused_runtime";
  } else {
    taskSessionState.value = "active_runtime";
  }
  task.active = true;
  task.stage = "cooling";
  downloadSpeedState.active = false;
  clearDownloadSpeedRows(downloadSpeedRows);
  stopDownloadSpeedRowPruneTimer();
  clearDownloadSpeedTraceEntries();
  pushProcessTrace({
    detail: asString(event.payload.reason || "当前任务进入冷却阶段。"),
    stage: "cooling",
    title: "任务进入冷却",
    tone: "warning",
    ts: event.ts,
  });
}

export function applyProbeEventFrom(event: ProbeEventEnvelope, deps: ProbeEventApplyDeps) {
  const { task, mcisProgress, shouldRefreshProgressUI, notifyUploadResult, eventDebugLogDisplayPath, eventDebugLogOpenTarget, eventTraceFailureSummary, reconcileTaskData, appendLog, setStatus, pushActivity } = deps;
  const incomingTaskId = asString(event.task_id).trim();
  const currentTaskId = task.taskId.trim();
  const sequenceDecision = classifyProbeEventSequence(currentTaskId, task.lastSeq, event);
  if (sequenceDecision === "foreign") {
    appendLog(`${event.event}.ignored`, {
      current_task_id: currentTaskId,
      event_task_id: incomingTaskId,
      payload: event.payload,
    });
    return;
  }
  if (sequenceDecision === "duplicate") {
    appendLog(`${event.event}.duplicate`, {
      event_seq: event.seq,
      last_seq: task.lastSeq,
      task_id: incomingTaskId || currentTaskId,
    });
    return;
  }
  if (sequenceDecision === "gap") {
    appendLog(`${event.event}.sequence_gap`, {
      event_seq: event.seq,
      last_seq: task.lastSeq,
      task_id: incomingTaskId || currentTaskId,
    });
    void reconcileTaskData(incomingTaskId || currentTaskId);
  }

  appendLog(event.event, event.payload);
  notifyUploadResult(event);
  const nextTaskState = deriveTaskStateFromProbeEvent(event);
  const progressStage = event.event === "probe.mcis.progress" ? "stage0_mcis" : event.event === "probe.progress" || event.event === "probe.speed" ? asString(event.payload.stage) || "running" : "";
  const refreshProgressUI = !progressStage || shouldRefreshProgressUI(progressStage, asCount(event.payload.completed ?? event.payload.processed), asCount(event.payload.total));

  if (refreshProgressUI) {
    setStatus(nextTaskState);
  }
  task.active = !["cancelled", "completed", "failed", "no_results"].includes(nextTaskState.tone);
  if (["cancelled", "completed", "failed", "no_results"].includes(nextTaskState.tone)) {
    mcisProgress.active = false;
  }
  task.lastEvent = event.event;
  task.lastSeq = event.seq;
  task.taskId = event.task_id || task.taskId;
  const eventDebugLogPath = eventDebugLogDisplayPath(event.payload);
  const eventDebugLogTarget = eventDebugLogOpenTarget(event.payload);
  const traceFailureSummary = eventTraceFailureSummary(event.payload);
  const ctx: ProbeEventContext = { event, incomingTaskId, nextTaskState, refreshProgressUI, eventDebugLogPath, eventDebugLogTarget, traceFailureSummary };

  switch (event.event) {
    case "probe.mcis.progress":
      handleMcisProgress(ctx, deps);
      break;
    case "probe.preprocessed":
      handlePreprocessed(ctx, deps);
      break;
    case "probe.progress":
      handleProgress(ctx, deps);
      break;
    case "probe.resumed":
      handleResumed(ctx, deps);
      break;
    case "probe.speed":
      if (!handleSpeed(ctx, deps)) {
        return;
      }
      break;
    case "probe.edge_rejected":
      handleEdgeRejected(ctx, deps);
      break;
    case "probe.partial_export":
      handlePartialExport(ctx, deps);
      break;
    case "probe.export_completed":
      handleExportCompleted(ctx, deps);
      break;
    case "probe.export_failed":
      handleExportFailed(ctx, deps);
      break;
    case "probe.completed":
      handleCompleted(ctx, deps);
      break;
    case "probe.failed":
      handleFailed(ctx, deps);
      break;
    case "probe.cancelled":
      handleCancelled(ctx, deps);
      break;
    case "probe.cooling":
      handleCooling(ctx, deps);
      break;
  }

  pushActivity(nextTaskState.title, nextTaskState.detail);
}
