// 探测阶段0（MICS 抽样 / IP 池 / 进度 / 恢复）事件处理器。
import { asCount, asString } from "../valueCoerce";
import { stageTitle } from "../labels";
import type { ProbeEventApplyDeps, ProbeEventContext } from "./types";

export function handleMcisProgress(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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

export function handlePreprocessed(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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

export function handleProgress(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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
    summary.downloadTarget = asCount(event.payload.qualified_target);
    if (task.stage !== "stage3_get") {
      resetDownloadSpeedState();
    }
    const progressPrefix = task.stage === "stage3_get" ? "文件测速" : "";
    // 有成功 IP 配额时进度条按配额走，这一行把两个口径都写清楚，免得数字看起来对不上。
    const quotaSuffix = summary.downloadTarget > 0 ? `，目标 ${summary.passed}/${summary.downloadTarget}` : "";
    pushProcessTrace({
      detail: `${progressPrefix ? `${progressPrefix}，` : ""}已处理 ${summary.processed}/${summary.total || "-"}，通过 ${summary.passed}，失败 ${summary.failed}${quotaSuffix}。`,
      stage: task.stage,
      title: `${stageTitle(task.stage)}进行中`,
      tone: "running",
      ts: event.ts,
    });
  }
}

export function handleResumed(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
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
