import { toInteger, toStringValue } from "../../bridgeValues";
import { commandResult } from "../command";
import { invokeCore } from "../invoke";
import { normalizeProbeRows, normalizeResultFilePayload } from "../normalize";
import type { ProbeResult, ProbeResultIPFilter, ProbeResultOrder, ProbeResultSortBy, ProbeRunResultPayload, ResultHistoryMergeResult, ResultHistoryPage, ResultHistoryRecord, TaskResultPage, TaskSnapshot, TaskSnapshotList } from "../types";

const PROBE_ALREADY_RUNNING_MESSAGE = "当前已有探测任务运行或暂停，请完成后再启动新任务。";

function nextTaskId() {
  return `cfst-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
}

function probeStartFailureCode(message: string, code = "") {
  if (code === "PROBE_ALREADY_RUNNING" || message.includes(PROBE_ALREADY_RUNNING_MESSAGE)) {
    return "PROBE_ALREADY_RUNNING";
  }
  return "PROBE_FAILED";
}

export async function startProbe(payload: Record<string, unknown>) {
  const taskId = toStringValue(payload.task_id).trim() || nextTaskId();

  try {
    const requestPayload = {
      ...payload,
      task_id: taskId,
    };
    const desktopResult = await invokeCore<ProbeRunResultPayload>("probe.start", requestPayload);
    return commandResult(desktopResult.code || "PROBE_ACCEPTED", desktopResult.data, {
      message: desktopResult.message || "桌面探测任务已提交。",
      ok: desktopResult.ok,
      taskId: desktopResult.task_id || taskId,
      warnings: desktopResult.warnings,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : toStringValue(error) || "探测任务执行失败。";
    return commandResult(probeStartFailureCode(message), null, {
      message,
      ok: false,
      taskId,
    });
  }
}

export async function stopProbe(payload: Record<string, unknown>) {
  const mode = toStringValue(payload.mode).trim().toLowerCase();
  return invokeCore(mode === "pause" ? "probe.pause" : "probe.cancel", payload);
}

export async function resumeProbe(payload: Record<string, unknown>) {
  return invokeCore("probe.resume", payload);
}

export async function getTaskSnapshot(taskId: string) {
  return invokeCore<TaskSnapshot | null>("task.get", { task_id: taskId });
}

export async function listTaskSnapshots(limit = 20) {
  const payload = { limit: Math.max(1, Math.min(100, Math.floor(limit || 20))) };
  return invokeCore<TaskSnapshotList>("task.list", payload);
}

export async function listTaskResults(taskId: string, sortBy: ProbeResultSortBy, order: ProbeResultOrder, fallbackPayload: Record<string, unknown> = {}, ipFilter: ProbeResultIPFilter = "all", paging: { limit?: number; offset?: number } = {}) {
  const resultFilePayload = normalizeResultFilePayload(fallbackPayload);
  const result = await invokeCore<TaskResultPage>("task.results", {
    ...resultFilePayload,
    ip_filter: ipFilter,
    limit: paging.limit,
    offset: paging.offset,
    order,
    sort_by: sortBy,
    task_id: taskId,
  });
  if (!result.ok || !result.data) {
    return commandResult<TaskResultPage>(result.code || "TASK_RESULTS_LIST_FAILED", { count: 0, results: [], total_count: 0 }, { message: result.message, ok: false, taskId });
  }
  return commandResult<TaskResultPage>(
    result.code || "TASK_RESULTS_LISTED",
    {
      count: toInteger(result.data.count, 0),
      results: normalizeProbeRows(result.data.results),
      source_kind: toStringValue(result.data.source_kind).trim() || null,
      source_path: toStringValue(result.data.source_path).trim() || null,
      total_count: toInteger(result.data.total_count, toInteger(result.data.count, 0)),
    },
    { message: result.message, taskId, warnings: result.warnings },
  );
}

// 结果页跨批次历史存在后端(桌面/WebUI/Android 共用一份):这里只做命令调用,
// 合并、过期清理与上限裁剪都由 appcore 负责。
export async function mergeResultHistoryBatch(batchId: string, rows: ProbeResult[], measuredAt: number) {
  return invokeCore<ResultHistoryMergeResult>("history.merge_batch", {
    at: Math.trunc(measuredAt),
    batch_id: batchId,
    rows,
  });
}

export async function queryResultHistory() {
  const result = await invokeCore<ResultHistoryPage>("history.query", {});
  if (!result.ok || !result.data) {
    return commandResult<ResultHistoryPage>("RESULT_HISTORY_QUERY_FAILED", { records: [], total_count: 0 }, { message: result.message, ok: false });
  }
  return commandResult<ResultHistoryPage>(
    result.code || "HISTORY_LISTED",
    {
      records: Array.isArray(result.data.records) ? result.data.records : [],
      total_count: toInteger(result.data.total_count, 0),
    },
    { message: result.message, warnings: result.warnings },
  );
}

// 一次性迁移入口:旧版历史存在各宿主自己的 localStorage 里,首启灌进后端后删键。
export async function importResultHistory(records: ResultHistoryRecord[]) {
  return invokeCore<ResultHistoryMergeResult>("history.import", { records });
}
