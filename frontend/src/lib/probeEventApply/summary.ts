// 阶段汇总事件处理器（probe.stage_summary）：每个阶段结束时把漏斗与淘汰原因落到进程面板。
import { stageSummaryDetail, stageTitle } from "../labels";
import { asCount, asString } from "../valueCoerce";
import type { ProbeEventApplyDeps, ProbeEventContext } from "./types";

export function handleStageSummary(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { task, summary, pushProcessTrace } = deps;
  const { event } = ctx;
  const stage = asString(event.payload.stage).trim() || task.stage;
  const counts = event.payload.counts && typeof event.payload.counts === "object" ? (event.payload.counts as Record<string, unknown>) : {};
  const total = asCount(counts.total) || asCount(counts.input);
  const passed = asCount(counts.passed);
  const failed = asCount(counts.failed);

  // 对齐看板数字：进度事件被节流时，阶段汇总才是该阶段的最终计数。
  task.stage = stage || task.stage;
  if (total > 0) {
    summary.total = total;
    summary.processed = total;
  }
  summary.passed = passed;
  summary.failed = failed;

  pushProcessTrace({
    detail: stageSummaryDetail(event.payload),
    stage,
    title: `${stageTitle(stage)}汇总`,
    tone: passed > 0 ? "success" : "warning",
    ts: event.ts,
  });
}
