// 探测事件应用域入口（重构计划 §4.2）：只做序号判定、上下文装配与按事件分发；
// 各阶段状态映射在 probeEventApply/ 子模块，deps 契约见 ./types。
import { deriveTaskStateFromProbeEvent, type ProbeEventEnvelope } from "./bridge";
import { classifyProbeEventSequence } from "./probeEventSequence";
import { asCount, asString } from "./valueCoerce";
import { handleMcisProgress, handlePreprocessed, handleProgress, handleResumed } from "./probeEventApply/stage0";
import { handleEdgeRejected, handleSpeed } from "./probeEventApply/speed";
import { handleExportCompleted, handleExportFailed, handlePartialExport } from "./probeEventApply/export";
import { handleCancelled, handleCompleted, handleCooling, handleFailed } from "./probeEventApply/terminal";
import type { ProbeEventApplyDeps, ProbeEventContext } from "./probeEventApply/types";

export type { ProbeEventApplyDeps } from "./probeEventApply/types";

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
