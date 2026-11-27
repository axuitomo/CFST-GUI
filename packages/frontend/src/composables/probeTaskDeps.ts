// 探测任务域的类型契约（重构计划 §4.2 二次拆解）。
import type { Ref } from "vue";
import type { SourceConfig, TaskSnapshot, TaskTone } from "../lib/bridge";
import type { ProcessTraceEntry, ToastTone, ViewName } from "../lib/app/types";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../lib/config/snapshotBuild";

export interface MCISProgressState {
  active: boolean;
  candidateCount: number;
  completed: number;
  concurrency: number;
  elapsedMs: number;
  failed: number;
  lastColo: string;
  lastIP: string;
  lastOK: boolean;
  sourceID: string;
  sourceName: string;
  succeeded: number;
  total: number;
}

export interface ProbeTaskDeps {
  loading: Ref<boolean>;
  probeWarnings: Ref<string[]>;
  pushActivity: (title: string, detail: string) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  navigateTo: (view: ViewName) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  appendLog: (event: string, payload: unknown) => void;
  pushProcessTrace: (entry: Omit<ProcessTraceEntry, "id">) => void;
  pushWarningTrace: (warnings: string[], ts?: string) => void;
  clearProcessTrace: () => void;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  reconcileTaskData: (taskId?: string, options?: { switchToResultsOnData?: boolean }) => Promise<void>;
  resetDownloadSpeedState: () => void;
  captureDownloadSpeedTaskConfig: (config: ReturnType<typeof buildConfigSnapshotFrom>) => void;
  clearCurrentTaskResultWorkspace: (options?: { preserveSnapshot?: boolean }) => void;
  clearResultRows: () => void;
  preparedSources: () => SourceConfig[];
  sourcePayloads: () => SourceConfig[];
  applySourceStatuses: (statusesValue: unknown) => void;
}

/** 任务状态的可变视图，由 useProbeTask 持有、动作模块共享。 */
export interface ProbeTaskCoreState {
  task: {
    acceptedAt: string;
    active: boolean;
    completedAt: string;
    exportPath: string;
    lastEvent: string;
    lastSeq: number;
    stage: string;
    taskId: string;
  };
  taskSnapshot: Ref<TaskSnapshot | null>;
  taskSessionState: Ref<string>;
  summary: { accepted: number; downloadTarget: number; exported: number; failed: number; filtered: number; invalid: number; passed: number; processed: number; total: number };
  status: { detail: string; title: string; tone: TaskTone };
}
