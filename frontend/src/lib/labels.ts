import type { TaskTone } from "./bridge";

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
  stage0_mcis: "随机抽查",
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
