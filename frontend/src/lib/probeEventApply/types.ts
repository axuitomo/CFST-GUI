// 探测事件应用域的类型契约（重构计划 §4.2 二次拆解）：deps 与上下文共享，避免各阶段文件重复声明。
import type { Ref } from "vue";
import type { deriveTaskStateFromProbeEvent, ProbeEventEnvelope, ProbeResult, TaskSnapshot, TaskTone } from "../bridge";
import type { DownloadSpeedRow } from "../downloadSpeedRows";
import type { TaskActionKind } from "../labels";
import type { HistoryEntry, ProcessTraceEntry, ToastTone } from "../appTypes";

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

export interface ProbeEventContext {
  event: ProbeEventEnvelope;
  incomingTaskId: string;
  nextTaskState: ReturnType<typeof deriveTaskStateFromProbeEvent>;
  refreshProgressUI: boolean;
  eventDebugLogPath: string;
  eventDebugLogTarget: string;
  traceFailureSummary: string;
}
