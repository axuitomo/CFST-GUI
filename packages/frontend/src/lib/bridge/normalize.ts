import { isObject, toInteger, toNumber, toObjectRecord, toOptionalInteger, toOptionalNumber, toStringValue, toUnknownArray } from "./values";
import { stageSummaryDetail } from "../app/labels";
import { SCHEMA_VERSION } from "./command";
import type { DerivedTaskState, DnsRecordSnapshot, ProbeEventEnvelope, ProbeResult, TaskTone } from "./types";

interface NativeJSONResult {
  value?: string;
}

const TRACE_REASON_LABELS = {
  colo_filter: "地区码不匹配",
  edge_ip_restricted: "边缘 IP 受限（1034）",
  rate_limited: "服务端限流",
  request_create_failed: "追踪请求创建失败",
  source_colo_filter: "输入源 COLO 过滤未通过",
  status_mismatch: "状态码不匹配",
  trace_error: "追踪请求失败",
  trace_latency_limit: "追踪延迟超阈值",
  trace_read_error: "追踪响应读取失败",
} as const satisfies Record<string, string>;

const STAGE_LABELS = {
  stage0_pool: "IP池",
  stage1_tcp: "第一阶段",
  stage2_head: "第二阶段",
  stage2_trace: "第二阶段",
  stage3_get: "第三阶段",
  post_probe_push: "自动推送",
} as const satisfies Record<string, string>;

function traceReasonLabel(reason: string) {
  const normalized = reason.trim().toLowerCase();
  return TRACE_REASON_LABELS[normalized as keyof typeof TRACE_REASON_LABELS] || reason.trim() || "未知原因";
}

function stageLabel(stage: string) {
  return STAGE_LABELS[stage as keyof typeof STAGE_LABELS] || stage || "running";
}

export function summarizeTraceDiagnostics(value: unknown) {
  const diagnostics = toObjectRecord(value);
  const reasonCounts = toObjectRecord(diagnostics.reason_counts);
  const statusCounts = toObjectRecord(diagnostics.status_counts);
  const samples = toUnknownArray(diagnostics.samples);

  let topReason = "";
  let topReasonCount = 0;
  for (const [reason, rawCount] of Object.entries(reasonCounts)) {
    const count = toInteger(rawCount, 0);
    if (count > topReasonCount) {
      topReason = reason;
      topReasonCount = count;
    }
  }

  const parts: string[] = [];
  if (topReason) {
    parts.push(`${traceReasonLabel(topReason)} ${topReasonCount} 次`);
  }

  const statusEntries = Object.entries(statusCounts)
    .map(([code, rawCount]) => [code, toInteger(rawCount, 0)] as const)
    .filter(([, count]) => count > 0)
    .sort((left, right) => right[1] - left[1]);
  if (statusEntries.length > 0) {
    const [statusCode, count] = statusEntries[0];
    parts.push(`HTTP ${statusCode} ${count} 次`);
  }

  if (samples.length > 0) {
    const sample = toObjectRecord(samples[0]);
    const error = toStringValue(sample.error);
    const ip = toStringValue(sample.ip);
    const url = toStringValue(sample.url);
    if (error) {
      parts.push(error);
    } else if (ip || url) {
      parts.push([ip, url].filter(Boolean).join(" · "));
    }
  }

  return parts.join("；");
}

export function normalizeProbeEvent(input: unknown): ProbeEventEnvelope | null {
  if (!isObject(input)) {
    return null;
  }

  return {
    event: toStringValue(input.event),
    payload: toObjectRecord(input.payload),
    schema_version: toStringValue(input.schema_version) || SCHEMA_VERSION,
    seq: toInteger(input.seq, 0),
    task_id: toStringValue(input.task_id),
    ts: toStringValue(input.ts) || new Date().toISOString(),
  };
}

export function normalizeDnsRecord(input: unknown): DnsRecordSnapshot {
  const source = toObjectRecord(input);

  return {
    comment: toStringValue(source.comment),
    content: toStringValue(source.content),
    created_on: toStringValue(source.created_on),
    id: toStringValue(source.id),
    modified_on: toStringValue(source.modified_on),
    name: toStringValue(source.name),
    proxied: Boolean(source.proxied),
    ttl: toInteger(source.ttl, 1),
    type: toStringValue(source.type) || "A",
  };
}

export function normalizeDnsRecords(input: unknown): DnsRecordSnapshot[] {
  return toUnknownArray(input).map((entry) => normalizeDnsRecord(entry));
}

export function deriveTaskStateFromProbeEvent(event: ProbeEventEnvelope): DerivedTaskState {
  if (event.event === "probe.preprocessed") {
    const accepted = toInteger(event.payload.accepted, 0);
    const filtered = toInteger(event.payload.filtered, 0);
    const invalid = toInteger(event.payload.invalid, 0);
    const total = toInteger(event.payload.total, accepted + filtered + invalid);

    return {
      detail: `候选 ${total} 条，接受 ${accepted} 条，过滤 ${filtered} 条，非法 ${invalid} 条。`,
      title: accepted > 0 ? "IP池已完成" : "IP池没有可用结果",
      tone: accepted > 0 ? "preparing" : "no_results",
    };
  }

  if (event.event === "probe.mcis.progress") {
    const sourceName = toStringValue(event.payload.source_name) || "当前输入源";
    const completed = toInteger(event.payload.completed, 0);
    const total = toInteger(event.payload.total, 0);
    const succeeded = toInteger(event.payload.succeeded, 0);
    const failed = toInteger(event.payload.failed, 0);
    const candidates = toInteger(event.payload.candidate_count, 0);

    return {
      detail: `${sourceName}，已完成 ${completed}/${total || "-"}，成功 ${succeeded}，失败/超时 ${failed}，候选 ${candidates}。`,
      title: "MICS 抽样进行中",
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.progress") {
    const stage = toStringValue(event.payload.stage) || "running";
    const processed = toInteger(event.payload.processed, 0);
    const passed = toInteger(event.payload.passed, 0);
    const failed = toInteger(event.payload.failed, 0);
    const prefix = stage === "stage3_get" ? "文件测速" : stageLabel(stage);

    return {
      detail: `${prefix}，已处理 ${processed}，通过 ${passed}，失败 ${failed}。`,
      title: `${stageLabel(stage)}进行中`,
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.stage_summary") {
    const stage = toStringValue(event.payload.stage) || "running";

    // 阶段结束的汇总行：任务仍在继续，所以 tone 保持 running，不能在这里判成 no_results。
    return {
      detail: stageSummaryDetail(event.payload),
      title: `${stageLabel(stage)}汇总`,
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.resumed") {
    const stage = toStringValue(event.payload.stage ?? event.payload.current_stage) || "running";
    return {
      detail: toStringValue(event.payload.message) || "任务已恢复执行。",
      title: `${stageLabel(stage)}继续中`,
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.speed") {
    const ip = toStringValue(event.payload.ip);
    const colo = toStringValue(event.payload.colo);

    return {
      detail: `${ip || "当前 IP"}${colo ? `(${colo})` : ""} 正在测速中`,
      title: "文件测速实时速度",
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.partial_export") {
    const written = toInteger(event.payload.written, 0);
    const targetPath = toStringValue(event.payload.target_path);

    return {
      detail: targetPath ? `已导出 ${written} 条结果到 ${targetPath}。` : `已导出 ${written} 条结果。`,
      title: "结果已落盘",
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.export_completed") {
    const written = toInteger(event.payload.written, 0);
    const targetPath = toStringValue(event.payload.target_path);

    return {
      detail: targetPath ? `Android 系统导出已写入 ${written} 条结果到 ${targetPath}。` : `Android 系统导出已写入 ${written} 条结果。`,
      title: "系统导出完成",
      tone: "completed" as TaskTone,
    };
  }

  if (event.event === "probe.export_failed") {
    const message = toStringValue(event.payload.message) || "Android 系统导出失败。";

    return {
      detail: message,
      title: "系统导出失败",
      tone: "warning" as TaskTone,
    };
  }

  if (event.event === "probe.edge_rejected") {
    const note = toStringValue(event.payload.note) || "403";
    const stage = toStringValue(event.payload.stage) || "stage3_get";

    return {
      detail: toStringValue(event.payload.message) || `当前 IP 被边缘拒绝（${note}）。`,
      title: `${stageLabel(stage)}遇到 ${note}`,
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.cooling") {
    const reason = toStringValue(event.payload.reason) || "冷却中";

    return {
      detail: `${reason}${event.payload.recoverable ? " 可以点击继续任务。" : ""}`,
      title: "任务进入冷却",
      tone: "cooling" as TaskTone,
    };
  }

  if (event.event === "probe.completed") {
    const resultCount = Math.max(toInteger(event.payload.result_count, 0), toInteger(event.payload.passed, 0), toInteger(event.payload.exported, 0));
    const exported = toInteger(event.payload.exported, 0);
    const targetPath = toStringValue(event.payload.target_path);
    const hasResults = resultCount > 0;
    const traceSummary = summarizeTraceDiagnostics(event.payload.trace_diagnostics);
    const traceStageFailure = toStringValue(event.payload.failure_stage) === "stage2_trace" && traceSummary;

    return {
      detail: hasResults
        ? exported > 0
          ? targetPath
            ? `任务完成，可用结果 ${resultCount} 条，已导出 ${exported} 条到 ${targetPath}。`
            : `任务完成，可用结果 ${resultCount} 条，已导出 ${exported} 条。`
          : `任务完成，可用结果 ${resultCount} 条。`
        : traceStageFailure
          ? `追踪阶段未找到可用结果：${traceSummary}`
          : "任务已完成，但当前没有可用结果。",
      title: hasResults ? "任务完成" : traceStageFailure ? "追踪阶段无可用结果" : "没有可用结果",
      tone: hasResults ? ("completed" as TaskTone) : ("no_results" as TaskTone),
    };
  }

  if (event.event === "probe.cancelled") {
    return {
      detail: toStringValue(event.payload.message) || "测速任务已终止。",
      title: "任务已终止",
      tone: "cancelled" as TaskTone,
    };
  }

  if (event.event === "probe.failed") {
    const traceSummary = summarizeTraceDiagnostics(event.payload.trace_diagnostics);
    const message = toStringValue(event.payload.failure_stage) === "stage2_trace" && traceSummary ? `追踪阶段失败：${traceSummary}` : toStringValue(event.payload.message) || "任务失败。";

    return {
      detail: event.payload.recoverable ? `${message} 可以尝试继续或重试。` : message,
      title: "任务失败",
      tone: "failed" as TaskTone,
    };
  }

  return {
    detail: event.event,
    title: "收到新事件",
    tone: "running" as TaskTone,
  };
}

export function normalizeNativePayload(input: unknown): unknown {
  if (typeof input === "string") {
    try {
      return JSON.parse(input);
    } catch {
      return input;
    }
  }
  if (isObject(input)) {
    const value = (input as NativeJSONResult).value;
    if (typeof value === "string") {
      try {
        return JSON.parse(value);
      } catch {
        return value;
      }
    }
  }
  return input;
}

export function normalizeProbeRows(rows: unknown): ProbeResult[] {
  return toUnknownArray(rows).map((row) => {
    const source = toObjectRecord(row);
    const delayMs = toNumber(source.delayMs ?? source.delay_ms ?? source.tcp_latency_ms ?? source.tcpLatencyMs, 0);
    const traceDelayMs = toNumber(source.traceDelayMs ?? source.trace_delay_ms ?? source.trace_latency_ms ?? source.traceLatencyMs, 0);
    const downloadMbps = toOptionalNumber(source.downloadSpeedMb ?? source.download_mbps);
    const maxDownloadMbps = toOptionalNumber(source.maxDownloadSpeedMb ?? source.max_download_speed_mb ?? source.max_download_mbps ?? source.maxDownloadMbps);
    const normalizedDownloadMbps = downloadMbps !== null && downloadMbps >= 0 ? downloadMbps : null;
    const normalizedMaxDownloadMbps = maxDownloadMbps !== null && maxDownloadMbps >= 0 ? maxDownloadMbps : normalizedDownloadMbps;
    const sourcePort = toOptionalInteger(source.source_port ?? source.sourcePort);
    const testPort = toOptionalInteger(source.test_port ?? source.testPort);

    return {
      address: toStringValue(source.ip ?? source.address),
      colo: toStringValue(source.colo) || null,
      download_mbps: normalizedDownloadMbps,
      export_status: toStringValue(source.export_status ?? source.exportStatus) || "exported",
      last_error_code: toStringValue(source.last_error_code ?? source.lastErrorCode) || null,
      max_download_mbps: normalizedMaxDownloadMbps,
      note: toStringValue(source.note) || null,
      source_port: sourcePort !== null && sourcePort > 0 ? sourcePort : null,
      stage_status: toStringValue(source.stage_status ?? source.stageStatus) || "completed",
      tcp_latency_ms: delayMs > 0 ? delayMs : null,
      test_port: testPort !== null && testPort > 0 ? testPort : null,
      trace_latency_ms: traceDelayMs > 0 ? traceDelayMs : null,
    };
  });
}

export function normalizeResultFilePayload(payload: Record<string, unknown>) {
  const resultPath = [payload.path, payload.source_path, payload.sourcePath, payload.target_path, payload.targetPath, payload.export_path, payload.exportPath].map((value) => toStringValue(value).trim()).find((value) => value.length > 0) || "";
  return { path: resultPath };
}
