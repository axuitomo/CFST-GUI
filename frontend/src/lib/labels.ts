import type { TaskTone } from "./bridge/types";

export type TaskActionKind = "cancel" | "pause" | "rerun" | "resume" | "start";

/**
 * 动作词只在这里出现一次。
 * verb 用于句子和移动端按钮（"正在终止任务"、"终止"），button 用于桌面按钮（"终止任务"）。
 * 之前三个视图各写各的，同一次操作因此有了"启动 / 开始 / 启动任务"三种说法。
 */
export const taskActionLabels: Record<TaskActionKind, { button: string; verb: string }> = {
  start: { button: "启动任务", verb: "启动" },
  pause: { button: "暂停任务", verb: "暂停" },
  cancel: { button: "终止任务", verb: "终止" },
  resume: { button: "继续任务", verb: "继续" },
  rerun: { button: "重测", verb: "重测" },
};

export function taskActionLabel(kind: TaskActionKind) {
  return taskActionLabels[kind].verb;
}

/**
 * 任务状态统一按 tone 说：看板与结果页读同一张表，同一次任务不会在两个页面得到
 * "部分完成"和"已完成"两种结论。
 */
export const taskToneLabels: Record<TaskTone, string> = {
  cancelled: "已终止",
  completed: "已完成",
  cooling: "冷却中",
  failed: "失败",
  idle: "就绪",
  no_results: "无结果",
  partial: "部分完成",
  preparing: "准备中",
  running: "运行中",
  warning: "警告",
};

export function taskToneLabel(tone: TaskTone) {
  return taskToneLabels[tone];
}

const statusToneMap: Record<string, TaskTone> = {
  accepted: "preparing",
  cancelled: "cancelled",
  completed: "completed",
  cooling: "cooling",
  failed: "failed",
  idle: "idle",
  no_results: "no_results",
  partial: "partial",
  pending: "preparing",
  preprocessed: "preparing",
  preparing: "preparing",
  recovery_required: "warning",
  running: "running",
  waiting: "preparing",
};

// 未登记的状态原样返回：编一个词会让诊断信息变成误导。
export function taskStatusLabel(status: string | null | undefined) {
  const raw = (status || "").trim();
  if (!raw) {
    return taskToneLabels.idle;
  }
  const tone = statusToneMap[raw];
  return tone ? taskToneLabels[tone] : raw;
}

const stageLabels: Record<string, string> = {
  completed: "已完成",
  failed: "失败",
  pending: "待处理",
  stage0_mcis: "MICS 抽样",
  stage0_pool: "IP 池",
  stage1_tcp: "TCP 延迟",
  stage2_head: "追踪探测",
  stage2_trace: "追踪探测",
  stage3_get: "文件测速",
};

export function taskStageLabel(stage: string | null | undefined) {
  const raw = (stage || "").trim();
  if (!raw) {
    return "-";
  }
  return stageLabels[raw] || raw;
}

const sessionLabels: Record<string, string> = {
  active_runtime: "运行中",
  paused_runtime: "已暂停",
  persisted_only: "仅恢复快照",
};

export function taskSessionLabel(session: string | null | undefined) {
  const raw = (session || "").trim();
  if (!raw) {
    return "-";
  }
  return sessionLabels[raw] || raw;
}

// 探测过程轨迹的 stage 标题：与 taskStageLabel 是两套展示口径（措辞不同），各自独立。
export function stageTitle(stage: string) {
  const labels: Record<string, string> = {
    stage0_pool: "IP池",
    stage0_mcis: "MICS 抽样",
    stage1_tcp: "第一阶段",
    stage2_head: "第二阶段",
    stage2_trace: "第二阶段",
    stage3_get: "第三阶段",
    post_probe_push: "自动推送",
  };

  return labels[stage] || stage || "探测";
}

function summaryCount(source: Record<string, unknown>, key: string) {
  const value = Number(source[key]);
  return Number.isFinite(value) && value > 0 ? Math.trunc(value) : 0;
}

function summaryDurationLabel(durationMS: number) {
  const seconds = Math.max(0, Math.round(durationMS / 1000));
  if (seconds < 60) {
    return `${seconds} 秒`;
  }
  const minutes = Math.floor(seconds / 60);
  return minutes < 60 ? `${minutes} 分 ${seconds % 60} 秒` : `${Math.floor(minutes / 60)} 时 ${minutes % 60} 分`;
}

/**
 * 阶段汇总文案：后端把淘汰原因翻译成中文（reasons[].label）后下发，
 * 这里只负责排版，保证进程面板与状态行说的是同一句话。
 */
export function stageSummaryDetail(payload: Record<string, unknown>) {
  const counts = payload.counts && typeof payload.counts === "object" ? (payload.counts as Record<string, unknown>) : {};
  const total = summaryCount(counts, "total") || summaryCount(counts, "input");
  const input = summaryCount(counts, "input") || total;
  const passed = summaryCount(counts, "passed");
  const failed = summaryCount(counts, "failed") || Math.max(0, total - passed);
  const reasons = Array.isArray(payload.reasons) ? payload.reasons : [];
  const reasonEntries = reasons
    .map((entry) => {
      const record = entry && typeof entry === "object" ? (entry as Record<string, unknown>) : {};
      const label = String(record.label ?? record.reason ?? "").trim();
      const count = summaryCount(record, "count");
      return label && count > 0 ? `${label} ${count}` : "";
    })
    .filter(Boolean);
  // 一行放不下所有原因:只列前四项,其余用「等 N 项」表示还有内容。
  const reasonText = reasonEntries.slice(0, 4).join("、") + (reasonEntries.length > 4 ? ` 等 ${reasonEntries.length} 项` : "");
  const durationMS = Number(payload.duration_ms ?? payload.durationMS);

  let line = `输入 ${input || "-"} → 通过 ${passed} / 淘汰 ${failed}`;
  if (reasonText) {
    line += `（${reasonText}）`;
  }
  if (Number.isFinite(durationMS) && durationMS > 0) {
    line += `，耗时 ${summaryDurationLabel(durationMS)}`;
  }
  return line;
}
