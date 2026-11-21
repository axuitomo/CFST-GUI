// App 级日志 / 进程追踪 / 状态文案工具（重构计划 §4.2）：从 App.vue 原样搬出，
// 只负责写入侧，不参与视图渲染；字段与截断阈值、去重键、时间格式均与迁移前逐字一致。
import { ref, type Ref } from "vue";

import { formatUTCOffsetLabel, formatTimestampWithUTCOffset } from "../lib/time";
import { shouldRefreshProgress } from "../lib/mobileProgressRefresh";
import type { ProcessTraceEntry } from "../lib/appTypes";
import { asCount, asRecord, asString } from "../lib/valueCoerce";
import type { TaskTone } from "../lib/bridge";

const MAX_LOG_ENTRIES = 160;
const MAX_TRACE_ENTRIES = 80;
const DOWNLOAD_SPEED_TRACE_PREFIX = "download-speed:";

export interface AppTraceStatus {
  title: string;
  detail: string;
  tone: TaskTone;
}

export interface AppTraceDeps {
  settings: { utcOffsetMinutes: number };
  configPath: Ref<string>;
  /** 延迟求值：status 由 useProbeTask 持有，构造期还不能直接解引用。 */
  status: () => AppTraceStatus;
  isAndroidApp: () => boolean;
}

export function useAppTrace(deps: AppTraceDeps) {
  // 只写入、不参与渲染：用普通有界数组，避免每个事件都触发响应式追踪。
  const logs: Array<{ event: string; payload: unknown; ts: string }> = [];
  const processTrace = ref<ProcessTraceEntry[]>([]);
  let processTraceId = 0;
  let lastMobileProgressRefreshAt = 0;
  let lastMobileProgressStage = "";

  function appendLog(event: string, payload: unknown) {
    logs.push({ event, payload, ts: new Date().toISOString() });
    if (logs.length > MAX_LOG_ENTRIES) {
      logs.splice(0, logs.length - MAX_LOG_ENTRIES);
    }
  }

  function commandDiagnosticPayload(
    result: { code?: string; data?: unknown; message?: string; warnings?: string[] },
    fallbackConfigPath = deps.configPath.value,
  ) {
    const data = asRecord(result.data);
    const storage = asRecord(data.storage);
    return {
      code: asString(result.code).trim(),
      message: asString(result.message).trim(),
      warnings: Array.isArray(result.warnings) ? result.warnings.map((entry) => asString(entry)).filter(Boolean) : [],
      config_path: asString(data.configPath || data.config_path || fallbackConfigPath).trim(),
      storage_uri: asString(storage.storage_uri || storage.storageUri).trim(),
      runtime_dir: asString(storage.runtime_dir || storage.runtimeDir).trim(),
      last_sync_error: asString(storage.last_sync_error || storage.lastSyncError).trim(),
      permission_ok: storage.permission_ok !== false && storage.permissionOk !== false,
    };
  }

  function clearProcessTrace() {
    processTrace.value = [];
  }

  function clearDownloadSpeedTraceEntries() {
    for (let index = processTrace.value.length - 1; index >= 0; index -= 1) {
      if (processTrace.value[index].key?.startsWith(DOWNLOAD_SPEED_TRACE_PREFIX)) {
        processTrace.value.splice(index, 1);
      }
    }
  }

  function updateDownloadSpeedTrace(ip: string, colo: string, ts: string) {
    const traceKey = `${DOWNLOAD_SPEED_TRACE_PREFIX}${ip || "当前 IP"}`;
    const detail = `${ip || "当前 IP"}${colo ? `(${colo})` : ""} 正在测速中`;
    const entries = processTrace.value;
    const existingIndex = entries.findIndex((entry) => entry.key === traceKey);

    if (existingIndex >= 0) {
      const existing = entries[existingIndex];
      entries[existingIndex] = {
        detail,
        id: existing.id,
        key: traceKey,
        stage: "stage3_get",
        title: "文件测速实时速度",
        tone: "running",
        ts,
      };
      return;
    }

    entries.unshift({
      detail,
      id: processTraceId,
      key: traceKey,
      stage: "stage3_get",
      title: "文件测速实时速度",
      tone: "running",
      ts,
    });
    processTraceId += 1;
    if (entries.length > MAX_TRACE_ENTRIES) {
      entries.splice(MAX_TRACE_ENTRIES);
    }
  }

  function pushProcessTrace(entry: Omit<ProcessTraceEntry, "id">) {
    const entries = processTrace.value;
    const lastEntry = entries[0];
    if (lastEntry && lastEntry.stage === entry.stage && lastEntry.title === entry.title && lastEntry.tone === entry.tone) {
      entries[0] = { ...entry, id: lastEntry.id };
      return;
    }

    entries.unshift({ ...entry, id: processTraceId });
    processTraceId += 1;
    if (entries.length > MAX_TRACE_ENTRIES) {
      entries.splice(MAX_TRACE_ENTRIES);
    }
  }

  function pushWarningTrace(warnings: string[], ts = new Date().toISOString()) {
    if (warnings.length === 0) {
      return;
    }

    pushProcessTrace({
      detail: warnings.join("；"),
      stage: "warning",
      title: "任务提示",
      tone: "warning",
      ts,
    });
  }

  function setStatus(next: AppTraceStatus) {
    const status = deps.status();
    status.title = next.title;
    status.detail = next.detail;
    status.tone = next.tone;
  }

  function summarizeFailureSummary(summaryValue: unknown) {
    if (!summaryValue || typeof summaryValue !== "object") {
      return "";
    }

    const summaryRecord = summaryValue as Record<string, unknown>;
    const duplicateCount = asCount(summaryRecord.duplicate_count);
    const invalidCount = asCount(summaryRecord.invalid_count);
    const parts = [];

    if (duplicateCount > 0) {
      parts.push(`重复 ${duplicateCount}`);
    }

    if (invalidCount > 0) {
      parts.push(`非法 ${invalidCount}`);
    }

    return parts.join("，");
  }

  function formatAppTimestamp(value: string, options?: Parameters<typeof formatTimestampWithUTCOffset>[2]) {
    return formatTimestampWithUTCOffset(value, deps.settings.utcOffsetMinutes, options);
  }

  function utcOffsetLabel() {
    return formatUTCOffsetLabel(deps.settings.utcOffsetMinutes);
  }

  // Android 上进度事件很密，按阶段/进度节流，避免 WebView 每帧重排；桌面/WebUI 不节流。
  function shouldRefreshProgressUI(stage: string, completed: number, total: number) {
    if (!deps.isAndroidApp()) {
      return true;
    }
    const now = performance.now();
    if (!shouldRefreshProgress(lastMobileProgressRefreshAt, lastMobileProgressStage, stage, completed, total, now)) {
      return false;
    }
    lastMobileProgressRefreshAt = now;
    lastMobileProgressStage = stage;
    return true;
  }

  return {
    appendLog,
    clearDownloadSpeedTraceEntries,
    clearProcessTrace,
    commandDiagnosticPayload,
    formatAppTimestamp,
    logs,
    processTrace,
    pushProcessTrace,
    pushWarningTrace,
    setStatus,
    shouldRefreshProgressUI,
    summarizeFailureSummary,
    updateDownloadSpeedTrace,
    utcOffsetLabel,
  };
}
