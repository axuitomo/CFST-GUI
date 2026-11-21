// 任务终态与冷却事件处理器（completed / failed / cancelled / cooling）。
import { asBoolean, asCount, asString } from "../valueCoerce";
import { clearDownloadSpeedRows } from "../downloadSpeedRows";
import type { ProbeEventApplyDeps, ProbeEventContext } from "./types";

export function handleCompleted(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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

export function handleFailed(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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

export function handleCancelled(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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

export function handleCooling(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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
