import { EventsOn } from "../../wailsjs/runtime/runtime";

export const SCHEMA_VERSION = "phase1-bridge-v1";

export type TaskTone = "idle" | "preparing" | "running" | "partial" | "cooling" | "completed" | "no_results" | "failed";

export interface CommandResult<T = Record<string, unknown> | null> {
  code: string;
  data: T | null;
  message: string;
  ok: boolean;
  schema_version: string;
  task_id: string | null;
  warnings: string[];
}

export interface ProbeNumericTriple {
  stage1: number;
  stage2: number;
  stage3: number;
}

export interface ProbeTimeouts {
  stage1_ms: number;
  stage2_ms: number;
  stage3_ms: number;
}

export interface ProbeThresholds {
  max_http_latency_ms: number | null;
  max_tcp_latency_ms: number | null;
  min_download_mbps: number;
}

export type ProbeStrategy = "fast" | "full";
export type SourceKind = "inline" | "file" | "url";
export type SourceIPMode = "traverse" | "mcis";

export interface DesktopSourceConfig {
  content: string;
  enabled: boolean;
  id: string;
  ip_limit: number;
  ip_mode: SourceIPMode;
  kind: SourceKind;
  last_fetched_at: string;
  last_fetched_count: number;
  name: string;
  path: string;
  status_text: string;
  url: string;
}

export interface SourcePreviewSummary {
  action: string;
  invalid_count: number;
  mode: SourceIPMode;
  name: string;
  total_count: number;
}

export interface SourcePreviewPayload {
  preview_entries: string[];
  source_status: Partial<DesktopSourceConfig> | null;
  summary: SourcePreviewSummary | null;
}

export interface ConfigSnapshot {
  cloudflare: {
    api_token: string;
    comment: string;
    proxied: boolean;
    record_name: string;
    record_type: "A" | "AAAA";
    ttl: number;
    zone_id: string;
  };
  export: {
    file_name?: string;
    format?: string;
    overwrite?: string;
    target_dir: string;
  };
	  probe: {
	    concurrency: ProbeNumericTriple;
	    cooldown_policy: {
	      consecutive_failures: number;
	      cooldown_ms: number;
	    };
	    debug: boolean;
	    debug_capture_address: string;
	    disable_download: boolean;
	    download_count: number;
	    download_time_seconds: number;
	    event_throttle_ms: number;
	    host_header: string;
	    httping: boolean;
	    httping_cf_colo: string;
	    httping_status_code: number;
    max_loss_rate: number;
    min_delay_ms: number;
    ping_times: number;
    print_num: number;
    retry_policy: {
      backoff_ms: number;
      max_attempts: number;
    };
	    skip_first_latency_sample: boolean;
	    stage_limits: ProbeNumericTriple;
	    strategy: ProbeStrategy;
	    sni: string;
	    tcp_port: number;
	    test_all: boolean;
	    thresholds: ProbeThresholds;
	    timeouts: ProbeTimeouts;
	    url: string;
	    user_agent: string;
	  };
  sources: DesktopSourceConfig[];
}

export interface ProbeEventEnvelope {
  event: string;
  payload: Record<string, unknown>;
  schema_version: string;
  seq: number;
  task_id: string;
  ts: string;
}

export interface DnsRecordSnapshot {
  comment: string;
  content: string;
  created_on?: string;
  id: string;
  modified_on?: string;
  name: string;
  proxied: boolean;
  ttl: number;
  type: string;
}

export interface DerivedTaskState {
  detail: string;
  title: string;
  tone: TaskTone;
}

export interface TaskProgress {
  failed: number;
  passed: number;
  processed: number;
  stage: string;
  total?: number | null;
}

export interface ExportRecord {
  file_name: string;
  format: string;
  last_write_at?: string | null;
  target_dir: string;
  task_id: string;
  written_count: number;
}

export interface TaskSnapshot {
  completed_at?: string | null;
  config_digest?: string | null;
  current_stage?: string | null;
  export_record?: ExportRecord | null;
  failure_summary?: Record<string, unknown> | null;
  progress?: TaskProgress | null;
  started_at?: string | null;
  status: string;
  task_id: string;
  updated_at: string;
}

export interface ProbeResult {
  address: string;
  colo?: string | null;
  download_mbps?: number | null;
  export_status: string;
  http_latency_ms?: number | null;
  last_error_code?: string | null;
  stage_status: string;
  tcp_latency_ms?: number | null;
  tls_latency_ms?: number | null;
}

export type ProbeResultFilter = "all" | "exported" | "pending" | "failed";
export type ProbeResultOrder = "asc" | "desc";
export type ProbeResultSortBy = "address" | "stage" | "tcp" | "http" | "download" | "export_status";

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function toStringValue(value: unknown) {
  return typeof value === "string" ? value : value == null ? "" : String(value);
}

function toInteger(value: unknown, fallback = 0) {
  const parsed = Number.parseInt(String(value ?? ""), 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function toNumber(value: unknown, fallback = 0) {
  const parsed = Number.parseFloat(String(value ?? ""));
  return Number.isFinite(parsed) ? parsed : fallback;
}

function toOptionalInteger(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return null;
  }

  const parsed = Number.parseInt(String(value), 10);
  return Number.isFinite(parsed) ? parsed : null;
}

function toBoolean(value: unknown, fallback = false) {
  if (typeof value === "boolean") {
    return value;
  }

  if (typeof value === "number") {
    return value !== 0;
  }

  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    if (["1", "true", "yes", "on"].includes(normalized)) {
      return true;
    }
    if (["0", "false", "no", "off"].includes(normalized)) {
      return false;
    }
  }

  return fallback;
}

function normalizeStrategy(value: unknown): ProbeStrategy {
  const normalized = toStringValue(value).toLowerCase();
  if (normalized === "fast" || normalized === "latency" || normalized === "http-colo") {
    return "fast";
  }
  if (normalized === "full" || normalized === "speed" || normalized === "exhaustive") {
    return "full";
  }
  return "fast";
}

function normalizeSourceKind(value: unknown): SourceKind {
  const normalized = toStringValue(value).toLowerCase();
  if (normalized === "inline" || normalized === "file") {
    return normalized;
  }
  return "url";
}

function normalizeSourceIPMode(value: unknown): SourceIPMode {
  return toStringValue(value).toLowerCase() === "mcis" ? "mcis" : "traverse";
}

function normalizeSourceConfig(input: unknown, index: number): DesktopSourceConfig {
  const source = isObject(input) ? input : {};

  return {
    content: toStringValue(source.content),
    enabled: toBoolean(source.enabled, true),
    id: toStringValue(source.id) || `source-${index + 1}`,
    ip_limit: Math.max(1, toInteger(source.ip_limit ?? source.ipLimit, 3000)),
    ip_mode: normalizeSourceIPMode(source.ip_mode ?? source.ipMode),
    kind: normalizeSourceKind(source.kind ?? source.type),
    last_fetched_at: toStringValue(source.last_fetched_at ?? source.lastFetchedAt),
    last_fetched_count: Math.max(0, toInteger(source.last_fetched_count ?? source.lastFetchedCount, 0)),
    name: toStringValue(source.name) || `输入源 ${index + 1}`,
    path: toStringValue(source.path),
    status_text: toStringValue(source.status_text ?? source.statusText),
    url: toStringValue(source.url),
  };
}

export function isMaskedTokenValue(value: string) {
  return value.includes("...") || value.includes("***") || /^\*+$/.test(value);
}

export function normalizeConfigSnapshot(input: unknown): ConfigSnapshot {
  const source = isObject(input) ? input : {};
  const cloudflare = isObject(source.cloudflare) ? source.cloudflare : {};
  const exportConfig = isObject(source.export) ? source.export : {};
  const probe = isObject(source.probe) ? source.probe : {};
  const sources = Array.isArray(source.sources) ? source.sources : [];
  const timeouts = isObject(probe.timeouts) ? probe.timeouts : {};
  const concurrency = isObject(probe.concurrency) ? probe.concurrency : {};
  const stageLimits = isObject(probe.stage_limits) ? probe.stage_limits : isObject(probe.stageLimits) ? probe.stageLimits : {};
  const cooldownPolicy = isObject(probe.cooldown_policy)
    ? probe.cooldown_policy
    : isObject(probe.cooldownPolicy)
      ? probe.cooldownPolicy
      : {};
  const retryPolicy = isObject(probe.retry_policy) ? probe.retry_policy : isObject(probe.retryPolicy) ? probe.retryPolicy : {};
  const thresholds = isObject(probe.thresholds) ? probe.thresholds : {};
  const rawStrategy = toStringValue(probe.strategy).toLowerCase();
  const strategy = normalizeStrategy(probe.strategy);
  const testAll = toBoolean(probe.test_all ?? probe.testAll, false);

  return {
    cloudflare: {
      api_token: toStringValue(cloudflare.api_token),
      comment: toStringValue(cloudflare.comment),
      proxied: Boolean(cloudflare.proxied),
      record_name: toStringValue(cloudflare.record_name),
      record_type: toStringValue(cloudflare.record_type).toUpperCase() === "AAAA" ? "AAAA" : "A",
      ttl: toInteger(cloudflare.ttl, 1),
      zone_id: toStringValue(cloudflare.zone_id),
    },
    export: {
      file_name: toStringValue(exportConfig.file_name),
      format: toStringValue(exportConfig.format),
      overwrite: toStringValue(exportConfig.overwrite),
      target_dir: toStringValue(exportConfig.target_dir),
    },
    probe: {
      concurrency: {
        stage1: toInteger(concurrency.stage1, 200),
        stage2: toInteger(concurrency.stage2, 10),
        stage3: toInteger(concurrency.stage3, 1),
      },
	      cooldown_policy: {
	        consecutive_failures: toInteger(cooldownPolicy.consecutive_failures ?? cooldownPolicy.consecutiveFailures, 3),
	        cooldown_ms: toInteger(cooldownPolicy.cooldown_ms ?? cooldownPolicy.cooldownMs, 250),
	      },
	      debug: toBoolean(probe.debug, false),
	      debug_capture_address: toStringValue(probe.debug_capture_address ?? probe.debugCaptureAddress),
	      disable_download: strategy === "fast",
	      download_count: toInteger(probe.download_count ?? probe.downloadCount ?? stageLimits.stage3, 10),
	      download_time_seconds: toInteger(probe.download_time_seconds ?? probe.downloadTimeSeconds, 10),
	      event_throttle_ms: toInteger(probe.event_throttle_ms ?? probe.eventThrottleMs, 100),
	      host_header: toStringValue(probe.host_header ?? probe.hostHeader),
	      httping: toBoolean(probe.httping, rawStrategy === "http-colo"),
      httping_cf_colo: toStringValue(probe.httping_cf_colo ?? probe.httpingCfColo),
      httping_status_code: toInteger(probe.httping_status_code ?? probe.httpingStatusCode, 0),
      max_loss_rate: toNumber(probe.max_loss_rate ?? probe.maxLossRate, 1),
      min_delay_ms: toInteger(probe.min_delay_ms ?? probe.minDelayMs, 0),
      ping_times: toInteger(probe.ping_times ?? probe.pingTimes, 4),
      print_num: toInteger(probe.print_num ?? probe.printNum, 10),
      retry_policy: {
        backoff_ms: toInteger(retryPolicy.backoff_ms ?? retryPolicy.backoffMs, 0),
        max_attempts: toInteger(retryPolicy.max_attempts ?? retryPolicy.maxAttempts, 0),
      },
      skip_first_latency_sample: toBoolean(probe.skip_first_latency_sample ?? probe.skipFirstLatencySample, true),
	      stage_limits: {
	        stage1: toInteger(stageLimits.stage1, 512),
	        stage2: toInteger(stageLimits.stage2, 64),
	        stage3: toInteger(stageLimits.stage3, 10),
	      },
	      strategy,
	      sni: toStringValue(probe.sni),
	      tcp_port: toInteger(probe.tcp_port ?? probe.tcpPort, 443),
      test_all: testAll,
      thresholds: {
        max_http_latency_ms: toOptionalInteger(thresholds.max_http_latency_ms ?? thresholds.maxHttpLatencyMs),
        max_tcp_latency_ms: toOptionalInteger(thresholds.max_tcp_latency_ms ?? thresholds.maxTcpLatencyMs),
        min_download_mbps: toNumber(thresholds.min_download_mbps ?? thresholds.minDownloadMbps, 0),
      },
	      timeouts: {
	        stage1_ms: toInteger(timeouts.stage1_ms ?? timeouts.stage1Ms, 1000),
	        stage2_ms: toInteger(timeouts.stage2_ms ?? timeouts.stage2Ms, 1000),
	        stage3_ms: toInteger(timeouts.stage3_ms ?? timeouts.stage3Ms, 10000),
	      },
	      url: toStringValue(probe.url) || "https://cf.xiu2.xyz/url",
	      user_agent:
	        toStringValue(probe.user_agent ?? probe.userAgent) ||
	        "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0",
	    },
    sources: sources.map((entry, index) => normalizeSourceConfig(entry, index)),
  };
}

export function normalizeCommandResult<T = Record<string, unknown> | null>(input: unknown): CommandResult<T> {
  const source = isObject(input) ? input : {};
  return {
    code: toStringValue(source.code) || "UNKNOWN",
    data: (source.data as T | null) ?? null,
    message: toStringValue(source.message),
    ok: source.ok !== false,
    schema_version: toStringValue(source.schema_version) || SCHEMA_VERSION,
    task_id: toStringValue(source.task_id) || null,
    warnings: Array.isArray(source.warnings) ? source.warnings.map((entry) => toStringValue(entry)).filter(Boolean) : [],
  };
}

export function normalizeProbeEvent(input: unknown): ProbeEventEnvelope | null {
  if (!isObject(input)) {
    return null;
  }

  return {
    event: toStringValue(input.event),
    payload: isObject(input.payload) ? input.payload : {},
    schema_version: toStringValue(input.schema_version) || SCHEMA_VERSION,
    seq: toInteger(input.seq, 0),
    task_id: toStringValue(input.task_id),
    ts: toStringValue(input.ts) || new Date().toISOString(),
  };
}

export function normalizeDnsRecord(input: unknown): DnsRecordSnapshot {
  const source = isObject(input) ? input : {};

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
  return Array.isArray(input) ? input.map((entry) => normalizeDnsRecord(entry)) : [];
}

export function deriveTaskStateFromProbeEvent(event: ProbeEventEnvelope): DerivedTaskState {
  if (event.event === "probe.preprocessed") {
    const accepted = toInteger(event.payload.accepted, 0);
    const filtered = toInteger(event.payload.filtered, 0);
    const invalid = toInteger(event.payload.invalid, 0);
    const total = toInteger(event.payload.total, accepted + filtered + invalid);

    return {
      detail: `候选 ${total} 条，接受 ${accepted} 条，过滤 ${filtered} 条，非法 ${invalid} 条。`,
      title: accepted > 0 ? "预处理已完成" : "预处理没有可用结果",
      tone: accepted > 0 ? "preparing" : "no_results",
    };
  }

  if (event.event === "probe.progress") {
    const stage = toStringValue(event.payload.stage) || "running";
    const processed = toInteger(event.payload.processed, 0);
    const passed = toInteger(event.payload.passed, 0);
    const failed = toInteger(event.payload.failed, 0);

    return {
      detail: `阶段 ${stage}，已处理 ${processed}，通过 ${passed}，失败 ${failed}。`,
      title: "任务运行中",
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.partial_export") {
    const written = toInteger(event.payload.written, 0);
    const targetPath = toStringValue(event.payload.target_path);

    return {
      detail: targetPath ? `已导出 ${written} 条结果到 ${targetPath}。` : `已导出 ${written} 条结果。`,
      title: "已有部分结果可用",
      tone: "partial" as TaskTone,
    };
  }

  if (event.event === "probe.cooling") {
    const reason = toStringValue(event.payload.reason) || "冷却中";

    return {
      detail: `${reason}${event.payload.recoverable ? " 可以恢复后继续。" : ""}`,
      title: "任务进入冷却",
      tone: "cooling" as TaskTone,
    };
  }

  if (event.event === "probe.completed") {
    const resultCount = Math.max(
      toInteger(event.payload.result_count, 0),
      toInteger(event.payload.passed, 0),
      toInteger(event.payload.exported, 0),
    );
    const exported = toInteger(event.payload.exported, 0);
    const targetPath = toStringValue(event.payload.target_path);
    const hasResults = resultCount > 0;

    return {
      detail:
        hasResults
          ? exported > 0
            ? targetPath
              ? `任务完成，可用结果 ${resultCount} 条，已导出 ${exported} 条到 ${targetPath}。`
              : `任务完成，可用结果 ${resultCount} 条，已导出 ${exported} 条。`
            : `任务完成，可用结果 ${resultCount} 条。`
          : "任务已完成，但当前没有可用结果。",
      title: hasResults ? "任务完成" : "没有可用结果",
      tone: hasResults ? ("completed" as TaskTone) : ("no_results" as TaskTone),
    };
  }

  if (event.event === "probe.failed") {
    const message = toStringValue(event.payload.message) || "任务失败。";

    return {
      detail: event.payload.recoverable ? `${message} 可以尝试恢复或重试。` : message,
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

interface WailsAppBridge {
  FetchDesktopSource: (payload: Record<string, unknown>) => Promise<unknown>;
  LoadDesktopConfig: () => Promise<unknown>;
  OpenPath: (targetPath: string) => Promise<void>;
  PreviewDesktopSource: (payload: Record<string, unknown>) => Promise<unknown>;
  RunDesktopProbe: (payload: Record<string, unknown>) => Promise<Record<string, unknown>>;
  SaveDesktopConfig: (payload: Record<string, unknown>) => Promise<unknown>;
}

declare global {
  interface Window {
    go?: {
      main?: {
        App?: WailsAppBridge;
      };
    };
  }
}

const probeListeners = new Set<(event: ProbeEventEnvelope) => void>();
const taskSnapshots = new Map<string, TaskSnapshot>();
const taskResults = new Map<string, ProbeResult[]>();
let dnsRecordCache: DnsRecordSnapshot[] = [];
let disposeRuntimeProbeListener: (() => void) | null = null;

function appBridge() {
  const bridge = window.go?.main?.App;

  if (!bridge) {
    throw new Error("Wails bridge is not ready.");
  }

  return bridge;
}

function nowIso() {
  return new Date().toISOString();
}

function nextTaskId() {
  return `cfst-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
}

function emitProbeEvent(event: ProbeEventEnvelope) {
  probeListeners.forEach((listener) => listener(event));
}

function commandResult<T = Record<string, unknown> | null>(
  code: string,
  data: T,
  options: {
    message?: string;
    ok?: boolean;
    taskId?: string | null;
    warnings?: string[];
  } = {},
): CommandResult<T> {
  return {
    code,
    data,
    message: options.message || "",
    ok: options.ok !== false,
    schema_version: SCHEMA_VERSION,
    task_id: options.taskId || null,
    warnings: options.warnings || [],
  };
}

function asArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function normalizeProbeRows(rows: unknown): ProbeResult[] {
  return asArray(rows).map((row) => {
    const source = isObject(row) ? row : {};
    const delayMs = toNumber(source.delayMs ?? source.delay_ms, 0);
    const downloadMbps = toNumber(source.downloadSpeedMb ?? source.download_mbps, 0);

    return {
      address: toStringValue(source.ip ?? source.address),
      colo: toStringValue(source.colo) || null,
      download_mbps: downloadMbps > 0 ? downloadMbps : null,
      export_status: "exported",
      http_latency_ms: null,
      last_error_code: null,
      stage_status: "completed",
      tcp_latency_ms: delayMs > 0 ? delayMs : null,
      tls_latency_ms: null,
    };
  });
}

function sortResults(rows: ProbeResult[], sortBy: ProbeResultSortBy, order: ProbeResultOrder) {
  const factor = order === "desc" ? -1 : 1;
  const valueOf = (row: ProbeResult) => {
    if (sortBy === "download") {
      return row.download_mbps ?? -1;
    }

    if (sortBy === "tcp") {
      return row.tcp_latency_ms ?? Number.MAX_SAFE_INTEGER;
    }

    if (sortBy === "http") {
      return row.http_latency_ms ?? Number.MAX_SAFE_INTEGER;
    }

    if (sortBy === "stage") {
      return row.stage_status;
    }

    if (sortBy === "export_status") {
      return row.export_status;
    }

    return row.address;
  };

  return [...rows].sort((left, right) => {
    const leftValue = valueOf(left);
    const rightValue = valueOf(right);

    if (typeof leftValue === "number" && typeof rightValue === "number") {
      return (leftValue - rightValue) * factor;
    }

    return String(leftValue).localeCompare(String(rightValue)) * factor;
  });
}

function filterResults(rows: ProbeResult[], filter: ProbeResultFilter) {
  if (filter === "exported") {
    return rows.filter((row) => row.export_status === "exported");
  }

  if (filter === "failed") {
    return rows.filter((row) => row.stage_status === "failed" || Boolean(row.last_error_code));
  }

  if (filter === "pending") {
    return rows.filter((row) => row.export_status !== "exported" && row.stage_status !== "failed");
  }

  return rows;
}

function rowsToDnsRecords(rows: ProbeResult[]): DnsRecordSnapshot[] {
  return rows.map((row, index) => ({
    comment: row.colo ? `CFST ${row.colo}` : "CFST result",
    content: row.address,
    created_on: nowIso(),
    id: `local-${index}-${row.address}`,
    modified_on: nowIso(),
    name: "local.cfst-gui",
    proxied: false,
    ttl: 1,
    type: row.address.includes(":") ? "AAAA" : "A",
  }));
}

function buildTaskSnapshot(taskId: string, result: Record<string, unknown>, rows: ProbeResult[]): TaskSnapshot {
  const summary = isObject(result.summary) ? result.summary : {};
  const outputFile = toStringValue(result.outputFile);
  const completedAt = nowIso();
  const passed = toInteger(summary.passed, rows.length);
  const failed = toInteger(summary.failed, 0);
  const total = toInteger(summary.total, passed + failed);

  return {
    completed_at: completedAt,
    config_digest: null,
    current_stage: "completed",
    export_record: {
      file_name: outputFile.split(/[\\/]/).pop() || outputFile || "result.csv",
      format: "csv",
      last_write_at: completedAt,
      target_dir: outputFile.includes("/") || outputFile.includes("\\") ? outputFile.replace(/[\\/][^\\/]+$/, "") : "",
      task_id: taskId,
      written_count: rows.length,
    },
    failure_summary: {
      invalid_count: toInteger(isObject(result.source) ? result.source.invalidCount : 0, 0),
    },
    progress: {
      failed,
      passed,
      processed: Math.max(passed+failed, rows.length),
      stage: "completed",
      total: Math.max(total, passed+failed, rows.length),
    },
    started_at: toStringValue(result.startedAt) || completedAt,
    status: passed > 0 ? "completed" : "no_results",
    task_id: taskId,
    updated_at: completedAt,
  };
}

export async function loadConfig() {
  return normalizeCommandResult(await appBridge().LoadDesktopConfig());
}

export async function listDnsRecords() {
  return commandResult(
    "RECORDS_LISTED",
    {
      count: dnsRecordCache.length,
      records: dnsRecordCache,
    },
    {
      message: dnsRecordCache.length > 0 ? "已读取本地 CFST 结果快照。" : "当前没有本地结果可作为 DNS 记录预览。",
    },
  );
}

export async function saveConfig(payload: Record<string, unknown>) {
  return normalizeCommandResult(await appBridge().SaveDesktopConfig(payload));
}

export async function previewDesktopSource(payload: Record<string, unknown>) {
  return normalizeCommandResult<SourcePreviewPayload>(await appBridge().PreviewDesktopSource(payload));
}

export async function fetchDesktopSource(payload: Record<string, unknown>) {
  return normalizeCommandResult<SourcePreviewPayload>(await appBridge().FetchDesktopSource(payload));
}

export async function pushDnsRecords(payload: Record<string, unknown>) {
  const ipsRaw = toStringValue(payload.ipsRaw);
  const ips = ipsRaw
    .split(/[,\s]+/)
    .map((entry) => entry.trim())
    .filter(Boolean);

  dnsRecordCache = ips.map((ip, index) => ({
    comment: "CFST local push preview",
    content: ip,
    created_on: nowIso(),
    id: `push-${index}-${ip}`,
    modified_on: nowIso(),
    name: "local.cfst-gui",
    proxied: false,
    ttl: 1,
    type: ip.includes(":") ? "AAAA" : "A",
  }));

  return commandResult(
    "PUSH_COMPLETED",
    {
      ignored_entries: [],
      records_after: dnsRecordCache,
      summary: {
        created: 0,
        deleted: 0,
        updated: dnsRecordCache.length,
      },
    },
    {
      message: "当前 Wails 适配将推送映射为本地结果预览，未写入 Cloudflare DNS。",
    },
  );
}

export async function startProbe(payload: Record<string, unknown>) {
  const taskId = toStringValue(payload.task_id).trim() || nextTaskId();

  try {
    const result = await appBridge().RunDesktopProbe({
      ...payload,
      task_id: taskId,
    });
    const rows = normalizeProbeRows(result.results);

    taskResults.set(taskId, rows);
    taskSnapshots.set(taskId, buildTaskSnapshot(taskId, result, rows));
    dnsRecordCache = rowsToDnsRecords(rows);

    return commandResult(
      "PROBE_COMPLETED",
      {
        accepted: true,
        export_path: toStringValue(result.outputFile),
        source_statuses: Array.isArray(result.sourceStatuses) ? result.sourceStatuses : [],
        task_id: taskId,
      },
      {
        message: rows.length > 0 ? "CFST 探测已完成，结果已同步到桌面 UI。" : "CFST 探测完成，但没有可用结果。",
        taskId,
        warnings: asArray(result.warnings).map((entry) => toStringValue(entry)).filter(Boolean),
      },
    );
  } catch (error) {
    return commandResult(
      "PROBE_FAILED",
      null,
      {
        message: error instanceof Error ? error.message : toStringValue(error) || "探测任务执行失败。",
        ok: false,
        taskId,
      },
    );
  }
}

export async function stopProbe(payload: Record<string, unknown>) {
  return commandResult(
    "PROBE_STOP_REQUESTED",
    null,
    {
      message: "当前 CFST 后端任务为同步执行，暂停请求已记录但不会中断已完成任务。",
      taskId: toStringValue(payload.task_id),
    },
  );
}

export async function resumeProbe(payload: Record<string, unknown>) {
  return commandResult(
    "PROBE_RESUME_REQUESTED",
    null,
    {
      message: "当前没有处于冷却状态的可恢复 CFST 任务。",
      taskId: toStringValue(payload.task_id),
    },
  );
}

export async function getTaskSnapshot(taskId: string) {
  return commandResult<TaskSnapshot | null>(
    taskSnapshots.has(taskId) ? "TASK_SNAPSHOT" : "TASK_NOT_FOUND",
    taskSnapshots.get(taskId) || null,
    {
      ok: taskSnapshots.has(taskId),
      taskId,
      message: taskSnapshots.has(taskId) ? "任务快照已读取。" : "任务不存在。",
    },
  );
}

export async function listTaskResults(
  taskId: string,
  sortBy: ProbeResultSortBy,
  order: ProbeResultOrder,
  filter: ProbeResultFilter,
) {
  const rows = filterResults(taskResults.get(taskId) || [], filter);
  const results = sortResults(rows, sortBy, order);

  return commandResult<{ count: number; results: ProbeResult[] }>(
    "TASK_RESULTS_LISTED",
    {
      count: results.length,
      results,
    },
    {
      taskId,
    },
  );
}

export async function listenToProbeEvents(handler: (event: ProbeEventEnvelope) => void) {
  probeListeners.add(handler);

  if (!disposeRuntimeProbeListener) {
    disposeRuntimeProbeListener = EventsOn("desktop:probe", (payload: unknown) => {
      const event = normalizeProbeEvent(payload);
      if (event) {
        emitProbeEvent(event);
      }
    });
  }

  return () => {
    probeListeners.delete(handler);
    if (probeListeners.size === 0 && disposeRuntimeProbeListener) {
      disposeRuntimeProbeListener();
      disposeRuntimeProbeListener = null;
    }
  };
}

export async function openPath(targetPath: string) {
  const normalized = targetPath.trim();

  if (!normalized) {
    return;
  }

  await appBridge().OpenPath(normalized);
}
