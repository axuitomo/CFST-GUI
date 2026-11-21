// 结果落盘与系统导出事件处理器（partial / export_completed / export_failed）。
import { asCount, asString } from "../valueCoerce";
import type { ProbeEventApplyDeps, ProbeEventContext } from "./types";

export function handlePartialExport(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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

export function handleExportCompleted(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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

export function handleExportFailed(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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
