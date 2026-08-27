import { computed, reactive, ref } from "vue";
import type { TaskSnapshot, TaskTone } from "../lib/bridge";

export type TaskActionKind = "cancel" | "pause" | "rerun" | "resume" | "start";

export function taskActionLabel(kind: TaskActionKind) {
  return (
    {
      cancel: "终止",
      pause: "暂停",
      rerun: "重测",
      resume: "继续",
      start: "启动",
    } as Record<TaskActionKind, string>
  )[kind];
}

export function useProbeTask() {
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

  const dashboardStatusLabel = computed(
    () =>
      (
        ({
          completed: "已完成",
          cancelled: "已终止",
          cooling: "冷却中",
          failed: "失败",
          idle: "就绪",
          no_results: "无结果",
          partial: "部分完成",
          preparing: "准备中",
          running: "运行中",
          warning: "警告",
        }) as Record<TaskTone, string>
      )[status.tone] || status.title,
  );
  const progressPercent = computed(() => {
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
  const canCancelTask = computed(() => hasActiveTask.value && !taskActionInFlight.value && !hasDetachedTaskSnapshot.value);
  const canPauseTask = computed(() => hasActiveTask.value && !taskActionInFlight.value && !hasPausedTask.value && task.stage !== "accepted");
  const canResumeTask = computed(() => Boolean(task.taskId) && !taskActionInFlight.value && !hasDetachedTaskSnapshot.value && (taskSnapshot.value?.resume_capable === true || hasPausedTask.value));
  const canStartTask = computed(() => !taskActionInFlight.value && (!hasActiveTask.value || hasPausedTask.value));

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
    progressPercent,
    status,
    summary,
    task,
    taskActionInFlight,
    taskActionState,
    taskSessionState,
    taskSnapshot,
  };
}
