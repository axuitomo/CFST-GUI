// 下载测速展示状态（重构计划 §4.2）：从 App.vue 原样迁出。
// 任务启动时用 captureDownloadSpeedTaskConfig 锁定展示口径，运行中改设置不影响当前任务。
import { reactive, ref } from "vue";
import type { ProbeStrategy } from "../lib/bridge";
import { clearDownloadSpeedRows, downloadSpeedRowTtlMs, pruneDownloadSpeedRows, type DownloadSpeedRow } from "../lib/downloadSpeedRows";
import { positiveCount } from "../lib/valueCoerce";

export interface DownloadSpeedState {
  active: boolean;
  averageSpeedMbS: number | null;
  bytesRead: number;
  colo: string;
  currentSpeedMbS: number | null;
  elapsedMs: number;
  ip: string;
}

export interface DownloadSpeedDeps {
  clearDownloadSpeedTraceEntries: () => void;
}

export function useDownloadSpeed(deps: DownloadSpeedDeps) {
  const downloadSpeedState = reactive<DownloadSpeedState>({
    active: false,
    averageSpeedMbS: null,
    bytesRead: 0,
    colo: "",
    currentSpeedMbS: null,
    elapsedMs: 0,
    ip: "",
  });
  const downloadSpeedRows = reactive<DownloadSpeedRow[]>([]);
  // 任务启动时从配置快照里锁定，运行中改设置不影响当前任务的展示与行 TTL。
  const activeDownloadConcurrency = ref(1);
  const activeDownloadSampleIntervalMs = ref(1000);
  const activeDownloadEnabled = ref(false);
  // 启动时已用 launchConfig 锁定过一次展示口径；之后只有空闲态重新加载配置才允许覆盖。
  let downloadSpeedTaskConfigCaptured = false;
  const DOWNLOAD_SPEED_ROW_PRUNE_INTERVAL_MS = 1000;
  let downloadSpeedRowPruneTimer: number | undefined;

  function stopDownloadSpeedRowPruneTimer() {
    if (downloadSpeedRowPruneTimer !== undefined) {
      window.clearInterval(downloadSpeedRowPruneTimer);
      downloadSpeedRowPruneTimer = undefined;
    }
  }

  // 没有新采样时也要按 TTL 移除旧行，界面文案承诺的“采样停止后自动移除”靠这个定时器兑现。
  function ensureDownloadSpeedRowPruneTimer() {
    if (downloadSpeedRowPruneTimer !== undefined) {
      return;
    }
    downloadSpeedRowPruneTimer = window.setInterval(() => {
      pruneDownloadSpeedRows(downloadSpeedRows, Date.now(), downloadSpeedRowTtlMs(activeDownloadSampleIntervalMs.value));
      if (downloadSpeedRows.length === 0) {
        stopDownloadSpeedRowPruneTimer();
      }
    }, DOWNLOAD_SPEED_ROW_PRUNE_INTERVAL_MS);
  }

  function resetDownloadSpeedState(clearIdentity = true) {
    downloadSpeedState.active = false;
    downloadSpeedState.averageSpeedMbS = null;
    downloadSpeedState.bytesRead = 0;
    downloadSpeedState.currentSpeedMbS = null;
    downloadSpeedState.elapsedMs = 0;
    clearDownloadSpeedRows(downloadSpeedRows);
    stopDownloadSpeedRowPruneTimer();
    deps.clearDownloadSpeedTraceEntries();
    if (clearIdentity) {
      downloadSpeedState.colo = "";
      downloadSpeedState.ip = "";
    }
  }

  function captureDownloadSpeedTaskConfig(snapshot: {
    probe: {
      concurrency: { stage3: number };
      download_speed_sample_interval_ms: number;
      strategy: ProbeStrategy;
    };
  }) {
    activeDownloadConcurrency.value = positiveCount(snapshot.probe.concurrency.stage3, 1, 16);
    activeDownloadSampleIntervalMs.value = positiveCount(snapshot.probe.download_speed_sample_interval_ms, 1000);
    activeDownloadEnabled.value = snapshot.probe.strategy === "full";
    downloadSpeedTaskConfigCaptured = true;
  }

  function isDownloadSpeedTaskConfigCaptured() {
    return downloadSpeedTaskConfigCaptured;
  }

  return {
    activeDownloadConcurrency,
    activeDownloadEnabled,
    activeDownloadSampleIntervalMs,
    captureDownloadSpeedTaskConfig,
    downloadSpeedRows,
    downloadSpeedState,
    ensureDownloadSpeedRowPruneTimer,
    isDownloadSpeedTaskConfigCaptured,
    resetDownloadSpeedState,
    stopDownloadSpeedRowPruneTimer,
  };
}