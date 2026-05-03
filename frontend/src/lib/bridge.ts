import { EventsOn } from "../../wailsjs/runtime/runtime";
import { Capacitor, registerPlugin, type PluginListenerHandle } from "@capacitor/core";

export const SCHEMA_VERSION = "phase1-bridge-v1";
const MIN_PROBE_PING_TIMES = 2;
const MAX_LOSS_RATE = 0.15;
const DEFAULT_SOURCE_IP_LIMIT = 500;
const DEFAULT_CLOUDFLARE_TTL = 300;

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
  colo_filter: string;
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

export interface ColoDictionaryStatus {
  colo_path: string;
  colo_rows: number;
  geofeed_path: string;
  geofeed_rows: number;
  last_updated_at: string;
  matched_rows: number;
  missing_rows: number;
  source_url: string;
  updated: boolean;
  unmatched_rows: number;
}

export interface PathSelectionPayload {
  androidExportUri?: string;
  canceled?: boolean;
  content?: string;
  directory?: string;
  display_name?: string;
  file_name?: string;
  mode?: string;
  path?: string;
  storage_uri?: string;
  target_uri?: string;
  uri?: string;
}

export interface StorageHealth {
  checked_at: string;
  exists: boolean;
  free_bytes: number;
  is_dir: boolean;
  message: string;
  path: string;
  portable_mode: boolean;
  writable: boolean;
}

export interface StorageStatus {
  bootstrap_path: string;
  current_dir: string;
  default_dir: string;
  display_name?: string;
  health?: StorageHealth;
  portable_mode: boolean;
  setup_completed: boolean;
  setup_required: boolean;
  storage_uri?: string;
  writable: boolean;
}

export interface AppInfo {
  current_version: string;
  install_mode: string;
  platform: string;
  release_url: string;
}

export interface UpdateInfo extends AppInfo {
  asset_name: string;
  download_url: string;
  latest_version: string;
  release_name: string;
  sha256: string;
  update_available: boolean;
}

export interface UpdateInstallResult extends UpdateInfo {
  downloaded_path: string;
  install_started: boolean;
  next_action: string;
}

export interface ProfileItem {
  config_snapshot: ConfigSnapshot;
  created_at: string;
  id: string;
  name: string;
  updated_at: string;
}

export interface ProfileStore {
  active_profile_id: string;
  items: ProfileItem[];
  schema_version: string;
  updated_at: string;
}

export interface ConfigSnapshot {
  cloudflare: {
    api_token: string;
    comment: string;
    proxied: boolean;
    record_name: string;
    record_type?: "A" | "AAAA";
    ttl: number;
    zone_id: string;
  };
  export: {
    file_name?: string;
    file_name_template?: string;
    format?: string;
    overwrite?: string;
    target_dir: string;
    target_uri?: string;
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
    download_speed_sample_interval_seconds: number;
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
    trace_url: string;
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
  last_error_code?: string | null;
  stage_status: string;
  tcp_latency_ms?: number | null;
  trace_latency_ms?: number | null;
}

interface ProbeRunResultPayload extends Record<string, unknown> {
  outputFile?: unknown;
  results?: unknown;
  source?: unknown;
  sourceStatuses?: unknown;
  startedAt?: unknown;
  summary?: unknown;
  warnings?: unknown;
}

export type ProbeResultFilter = "all" | "exported" | "pending" | "failed";
export type ProbeResultOrder = "asc" | "desc";
export type ProbeResultSortBy = "address" | "stage" | "tcp" | "trace" | "download" | "export_status";

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

function clampInteger(value: unknown, fallback: number, min: number, max: number) {
  return Math.max(min, Math.min(max, toInteger(value, fallback)));
}

function positiveInteger(value: unknown, fallback: number, max?: number) {
  const parsed = toInteger(value, fallback);
  const normalized = parsed > 0 ? parsed : fallback;
  return typeof max === "number" ? Math.min(normalized, max) : normalized;
}

function minimumInteger(value: unknown, fallback: number, min: number, max?: number) {
  return Math.max(min, positiveInteger(value, fallback, max));
}

function nonNegativeInteger(value: unknown, fallback: number) {
  const parsed = toInteger(value, fallback);
  return parsed >= 0 ? parsed : fallback;
}

function nonNegativeNumber(value: unknown, fallback: number) {
  const parsed = toNumber(value, fallback);
  return parsed >= 0 ? parsed : fallback;
}

function clampNumber(value: unknown, fallback: number, min: number, max: number) {
  return Math.max(min, Math.min(max, toNumber(value, fallback)));
}

function toOptionalPositiveInteger(value: unknown) {
  const parsed = toOptionalInteger(value);
  return parsed !== null && parsed > 0 ? parsed : null;
}

function normalizeHTTPStatusCode(value: unknown) {
  const parsed = toInteger(value, 0);
  return parsed === 0 || (parsed >= 100 && parsed <= 599) ? parsed : 0;
}

function normalizeExportOverwrite(value: unknown) {
  return toStringValue(value) === "append" ? "append" : "replace_on_start";
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

function stageLabel(stage: string) {
  const labels: Record<string, string> = {
    stage0_pool: "IP池",
    stage1_tcp: "TCP测延迟",
    stage2_head: "追踪探测",
    stage2_trace: "追踪探测",
    stage3_get: "文件测速",
  };

  return labels[stage] || stage || "running";
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

function normalizeCloudflareTTL(value: unknown) {
  const ttl = toInteger(value, DEFAULT_CLOUDFLARE_TTL);
  return [60, 300, 600].includes(ttl) ? ttl : DEFAULT_CLOUDFLARE_TTL;
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
    colo_filter: toStringValue(source.colo_filter ?? source.coloFilter),
    content: toStringValue(source.content),
    enabled: toBoolean(source.enabled, true),
    id: toStringValue(source.id) || `source-${index + 1}`,
    ip_limit: Math.max(1, toInteger(source.ip_limit ?? source.ipLimit, DEFAULT_SOURCE_IP_LIMIT)),
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
  const strategy = normalizeStrategy(probe.strategy);
  const testAll = toBoolean(probe.test_all ?? probe.testAll, false);

  return {
    cloudflare: {
      api_token: toStringValue(cloudflare.api_token),
      comment: toStringValue(cloudflare.comment),
      proxied: Boolean(cloudflare.proxied),
      record_name: toStringValue(cloudflare.record_name),
      record_type: toStringValue(cloudflare.record_type).toUpperCase() === "AAAA" ? "AAAA" : "A",
      ttl: normalizeCloudflareTTL(cloudflare.ttl),
      zone_id: toStringValue(cloudflare.zone_id),
    },
    export: {
      file_name: toStringValue(exportConfig.file_name),
      file_name_template: toStringValue(exportConfig.file_name_template ?? exportConfig.fileNameTemplate),
      format: toStringValue(exportConfig.format),
      overwrite: normalizeExportOverwrite(exportConfig.overwrite),
      target_dir: toStringValue(exportConfig.target_dir),
      target_uri: toStringValue(exportConfig.target_uri ?? exportConfig.targetUri),
    },
    probe: {
      concurrency: {
        stage1: positiveInteger(concurrency.stage1, 200, 1000),
        stage2: clampInteger(concurrency.stage2, 6, 1, 20),
        stage3: 1,
      },
      cooldown_policy: {
        consecutive_failures: nonNegativeInteger(cooldownPolicy.consecutive_failures ?? cooldownPolicy.consecutiveFailures, 3),
        cooldown_ms: nonNegativeInteger(cooldownPolicy.cooldown_ms ?? cooldownPolicy.cooldownMs, 250),
      },
      debug: toBoolean(probe.debug, false),
      debug_capture_address: toStringValue(probe.debug_capture_address ?? probe.debugCaptureAddress),
      disable_download: strategy === "fast",
      download_count: positiveInteger(probe.download_count ?? probe.downloadCount ?? stageLimits.stage3, 10),
      download_speed_sample_interval_seconds: positiveInteger(
        probe.download_speed_sample_interval_seconds ?? probe.downloadSpeedSampleIntervalSeconds,
        2,
      ),
      download_time_seconds: minimumInteger(probe.download_time_seconds ?? probe.downloadTimeSeconds, 10, 10),
      event_throttle_ms: positiveInteger(probe.event_throttle_ms ?? probe.eventThrottleMs, 100),
      host_header: toStringValue(probe.host_header ?? probe.hostHeader),
      httping: false,
      httping_cf_colo: toStringValue(probe.httping_cf_colo ?? probe.httpingCfColo),
      httping_status_code: normalizeHTTPStatusCode(probe.httping_status_code ?? probe.httpingStatusCode),
      max_loss_rate: clampNumber(probe.max_loss_rate ?? probe.maxLossRate, MAX_LOSS_RATE, 0, MAX_LOSS_RATE),
      min_delay_ms: nonNegativeInteger(probe.min_delay_ms ?? probe.minDelayMs, 0),
      ping_times: minimumInteger(probe.ping_times ?? probe.pingTimes, 4, MIN_PROBE_PING_TIMES),
      print_num: nonNegativeInteger(probe.print_num ?? probe.printNum, 10),
      retry_policy: {
        backoff_ms: nonNegativeInteger(retryPolicy.backoff_ms ?? retryPolicy.backoffMs, 0),
        max_attempts: nonNegativeInteger(retryPolicy.max_attempts ?? retryPolicy.maxAttempts, 0),
      },
      skip_first_latency_sample: toBoolean(probe.skip_first_latency_sample ?? probe.skipFirstLatencySample, true),
      stage_limits: {
        stage1: positiveInteger(stageLimits.stage1, 512),
        stage2: positiveInteger(stageLimits.stage2, 64),
        stage3: positiveInteger(stageLimits.stage3, 10),
      },
      strategy,
      sni: toStringValue(probe.sni),
      tcp_port: clampInteger(probe.tcp_port ?? probe.tcpPort, 443, 1, 65535),
      test_all: testAll,
      thresholds: {
        max_http_latency_ms: null,
        max_tcp_latency_ms: toOptionalPositiveInteger(thresholds.max_tcp_latency_ms ?? thresholds.maxTcpLatencyMs),
        min_download_mbps: nonNegativeNumber(thresholds.min_download_mbps ?? thresholds.minDownloadMbps, 0),
      },
      timeouts: {
        stage1_ms: positiveInteger(timeouts.stage1_ms ?? timeouts.stage1Ms, 1000),
        stage2_ms: positiveInteger(timeouts.stage2_ms ?? timeouts.stage2Ms, 1000),
        stage3_ms: positiveInteger(timeouts.stage3_ms ?? timeouts.stage3Ms, 10000),
      },
      trace_url: toStringValue(probe.trace_url ?? probe.traceUrl),
      url: toStringValue(probe.url) || "https://speed.cloudflare.com/__down?bytes=10000000",
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
      title: accepted > 0 ? "IP池已完成" : "IP池没有可用结果",
      tone: accepted > 0 ? "preparing" : "no_results",
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

  if (event.event === "probe.speed") {
    const ip = toStringValue(event.payload.ip);
    const currentSpeed = toNumber(event.payload.current_speed_mb_s, 0);
    const averageSpeed = toNumber(event.payload.average_speed_mb_s, 0);
    const bytesRead = toInteger(event.payload.bytes_read, 0);

    return {
      detail: `${ip || "当前 IP"} 当前 ${currentSpeed.toFixed(2)} MB/s，平均 ${averageSpeed.toFixed(2)} MB/s，已读取 ${bytesRead} bytes。`,
      title: "文件测速实时速度",
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
      detail: `${reason}${event.payload.recoverable ? " 可以点击继续任务。" : ""}`,
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

interface WailsAppBridge {
  BackupCurrentConfig: (payload: Record<string, unknown>) => Promise<unknown>;
  CheckForUpdates: (payload: Record<string, unknown>) => Promise<unknown>;
  CheckStorageHealth: (payload: Record<string, unknown>) => Promise<unknown>;
  DeleteProfile: (payload: Record<string, unknown>) => Promise<unknown>;
  DownloadAndInstallUpdate: (payload: Record<string, unknown>) => Promise<unknown>;
  ExportConfig: (payload: Record<string, unknown>) => Promise<unknown>;
  FetchDesktopSource: (payload: Record<string, unknown>) => Promise<unknown>;
  GetAppInfo: () => Promise<unknown>;
  ListCloudflareDNSRecords: (payload: Record<string, unknown>) => Promise<unknown>;
  LoadColoDictionaryStatus: () => Promise<unknown>;
  LoadDesktopConfig: () => Promise<unknown>;
  LoadProfiles: () => Promise<unknown>;
  ProcessColoDictionary: (payload: Record<string, unknown>) => Promise<unknown>;
  OpenPath: (targetPath: string) => Promise<void>;
  OpenReleasePage: () => Promise<unknown>;
  PreviewDesktopSource: (payload: Record<string, unknown>) => Promise<unknown>;
  PushCloudflareDNSRecords: (payload: Record<string, unknown>) => Promise<unknown>;
  RunDesktopProbe: (payload: Record<string, unknown>) => Promise<Record<string, unknown>>;
  CancelProbe: (payload: Record<string, unknown>) => Promise<unknown>;
  ResumeProbe: (payload: Record<string, unknown>) => Promise<unknown>;
  SaveDesktopConfig: (payload: Record<string, unknown>) => Promise<unknown>;
  SaveCurrentProfile: (payload: Record<string, unknown>) => Promise<unknown>;
  SelectPath: (payload: Record<string, unknown>) => Promise<unknown>;
  SetStorageDirectory: (payload: Record<string, unknown>) => Promise<unknown>;
  SwitchProfile: (payload: Record<string, unknown>) => Promise<unknown>;
  UpdateColoDictionary: (payload: Record<string, unknown>) => Promise<unknown>;
}

interface NativeJSONResult {
  value?: string;
}

interface CapacitorCfstPlugin {
  BackupCurrentConfig: (payload: Record<string, unknown>) => Promise<unknown>;
  CheckForUpdates: (payload: Record<string, unknown>) => Promise<unknown>;
  CheckStorageHealth: (payload: Record<string, unknown>) => Promise<unknown>;
  DeleteProfile: (payload: Record<string, unknown>) => Promise<unknown>;
  DownloadAndInstallUpdate: (payload: Record<string, unknown>) => Promise<unknown>;
  ExportConfig: (payload: Record<string, unknown>) => Promise<unknown>;
  GetAppInfo: () => Promise<unknown>;
  Init: (payload?: Record<string, unknown>) => Promise<unknown>;
  LoadConfig: () => Promise<unknown>;
  LoadProfiles: () => Promise<unknown>;
  SaveConfig: (payload: Record<string, unknown>) => Promise<unknown>;
  SaveCurrentProfile: (payload: Record<string, unknown>) => Promise<unknown>;
  SetStorageDirectory: (payload: Record<string, unknown>) => Promise<unknown>;
  SwitchProfile: (payload: Record<string, unknown>) => Promise<unknown>;
  PreviewSource: (payload: Record<string, unknown>) => Promise<unknown>;
  FetchSource: (payload: Record<string, unknown>) => Promise<unknown>;
  LoadColoDictionaryStatus: () => Promise<unknown>;
  ProcessColoDictionary: (payload: Record<string, unknown>) => Promise<unknown>;
  UpdateColoDictionary: (payload: Record<string, unknown>) => Promise<unknown>;
  RunProbe: (payload: Record<string, unknown>) => Promise<unknown>;
  CancelProbe: (payload: Record<string, unknown>) => Promise<unknown>;
  ListCloudflareDNSRecords: (payload: Record<string, unknown>) => Promise<unknown>;
  PushCloudflareDNSRecords: (payload: Record<string, unknown>) => Promise<unknown>;
  OpenPath: (payload: { targetPath: string }) => Promise<unknown>;
  OpenReleasePage: () => Promise<unknown>;
  SelectPath: (payload: Record<string, unknown>) => Promise<unknown>;
  addListener: (
    eventName: "desktop:probe",
    listenerFunc: (event: unknown) => void,
  ) => Promise<PluginListenerHandle> & PluginListenerHandle;
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
const cfstNative = registerPlugin<CapacitorCfstPlugin>("Cfst");
let disposeRuntimeProbeListener: (() => void) | null = null;
let nativeInitPromise: Promise<void> | null = null;

function appBridge() {
  const bridge = window.go?.main?.App;

  if (!bridge) {
    throw new Error("Wails bridge is not ready.");
  }

  return bridge;
}

function shouldUseNativeBridge() {
  return !window.go?.main?.App && Capacitor.isNativePlatform() && Capacitor.getPlatform() === "android";
}

async function ensureNativeBridge() {
  if (!shouldUseNativeBridge()) {
    return;
  }
  if (!nativeInitPromise) {
    nativeInitPromise = cfstNative.Init({}).then(() => undefined);
  }
  await nativeInitPromise;
}

function normalizeNativePayload(input: unknown): unknown {
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
    const traceDelayMs = toNumber(source.traceDelayMs ?? source.trace_delay_ms, 0);
    const downloadMbps = toNumber(source.downloadSpeedMb ?? source.download_mbps, 0);

    return {
      address: toStringValue(source.ip ?? source.address),
      colo: toStringValue(source.colo) || null,
      download_mbps: downloadMbps > 0 ? downloadMbps : null,
      export_status: "exported",
      last_error_code: null,
      stage_status: "completed",
      tcp_latency_ms: delayMs > 0 ? delayMs : null,
      trace_latency_ms: traceDelayMs > 0 ? traceDelayMs : null,
    };
  });
}

function sortResults(rows: ProbeResult[], sortBy: ProbeResultSortBy, order: ProbeResultOrder) {
  const factor = order === "desc" ? -1 : 1;
  const valueOf = (row: ProbeResult) => {
    if (sortBy === "download") {
      return row.download_mbps ?? -1;
    }

    if (sortBy === "trace") {
      return row.trace_latency_ms ?? Number.MAX_SAFE_INTEGER;
    }

    if (sortBy === "tcp") {
      return row.tcp_latency_ms ?? Number.MAX_SAFE_INTEGER;
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
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.LoadConfig()));
  }
  return normalizeCommandResult(await appBridge().LoadDesktopConfig());
}

export async function getAppInfo() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<AppInfo>(normalizeNativePayload(await cfstNative.GetAppInfo()));
  }
  return normalizeCommandResult<AppInfo>(await appBridge().GetAppInfo());
}

export async function checkForUpdates(payload: Record<string, unknown> = {}) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<UpdateInfo>(normalizeNativePayload(await cfstNative.CheckForUpdates(payload)));
  }
  return normalizeCommandResult<UpdateInfo>(await appBridge().CheckForUpdates(payload));
}

export async function downloadAndInstallUpdate(payload: Record<string, unknown> = {}) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<UpdateInstallResult>(normalizeNativePayload(await cfstNative.DownloadAndInstallUpdate(payload)));
  }
  return normalizeCommandResult<UpdateInstallResult>(await appBridge().DownloadAndInstallUpdate(payload));
}

export async function openReleasePage() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.OpenReleasePage()));
  }
  return normalizeCommandResult(await appBridge().OpenReleasePage());
}

export async function listDnsRecords(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.ListCloudflareDNSRecords(payload)));
  }
  return normalizeCommandResult(await appBridge().ListCloudflareDNSRecords(payload));
}

export async function saveConfig(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.SaveConfig(payload)));
  }
  return normalizeCommandResult(await appBridge().SaveDesktopConfig(payload));
}

export async function setStorageDirectory(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.SetStorageDirectory(payload)));
  }
  return normalizeCommandResult(await appBridge().SetStorageDirectory(payload));
}

export async function checkStorageHealth(payload: Record<string, unknown> = {}) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.CheckStorageHealth(payload)));
  }
  return normalizeCommandResult(await appBridge().CheckStorageHealth(payload));
}

export async function exportConfig(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.ExportConfig(payload)));
  }
  return normalizeCommandResult(await appBridge().ExportConfig(payload));
}

export async function backupCurrentConfig(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.BackupCurrentConfig(payload)));
  }
  return normalizeCommandResult(await appBridge().BackupCurrentConfig(payload));
}

export async function loadProfiles() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<ProfileStore>(normalizeNativePayload(await cfstNative.LoadProfiles()));
  }
  return normalizeCommandResult<ProfileStore>(await appBridge().LoadProfiles());
}

export async function saveCurrentProfile(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<ProfileStore>(normalizeNativePayload(await cfstNative.SaveCurrentProfile(payload)));
  }
  return normalizeCommandResult<ProfileStore>(await appBridge().SaveCurrentProfile(payload));
}

export async function switchProfile(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.SwitchProfile(payload)));
  }
  return normalizeCommandResult(await appBridge().SwitchProfile(payload));
}

export async function deleteProfile(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<ProfileStore>(normalizeNativePayload(await cfstNative.DeleteProfile(payload)));
  }
  return normalizeCommandResult<ProfileStore>(await appBridge().DeleteProfile(payload));
}

export async function selectPath(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<PathSelectionPayload>(normalizeNativePayload(await cfstNative.SelectPath(payload)));
  }
  return normalizeCommandResult<PathSelectionPayload>(await appBridge().SelectPath(payload));
}

export async function previewDesktopSource(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<SourcePreviewPayload>(normalizeNativePayload(await cfstNative.PreviewSource(payload)));
  }
  return normalizeCommandResult<SourcePreviewPayload>(await appBridge().PreviewDesktopSource(payload));
}

export async function fetchDesktopSource(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<SourcePreviewPayload>(normalizeNativePayload(await cfstNative.FetchSource(payload)));
  }
  return normalizeCommandResult<SourcePreviewPayload>(await appBridge().FetchDesktopSource(payload));
}

export async function loadColoDictionaryStatus() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<ColoDictionaryStatus>(normalizeNativePayload(await cfstNative.LoadColoDictionaryStatus()));
  }
  return normalizeCommandResult<ColoDictionaryStatus>(await appBridge().LoadColoDictionaryStatus());
}

export async function updateColoDictionary(payload: Record<string, unknown> = {}) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<ColoDictionaryStatus>(normalizeNativePayload(await cfstNative.UpdateColoDictionary(payload)));
  }
  return normalizeCommandResult<ColoDictionaryStatus>(await appBridge().UpdateColoDictionary(payload));
}

export async function processColoDictionary(payload: Record<string, unknown> = {}) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<ColoDictionaryStatus>(normalizeNativePayload(await cfstNative.ProcessColoDictionary(payload)));
  }
  return normalizeCommandResult<ColoDictionaryStatus>(await appBridge().ProcessColoDictionary(payload));
}

export async function pushDnsRecords(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.PushCloudflareDNSRecords(payload)));
  }
  const result = normalizeCommandResult(await appBridge().PushCloudflareDNSRecords(payload));
  return result;
}

export async function startProbe(payload: Record<string, unknown>) {
  const taskId = toStringValue(payload.task_id).trim() || nextTaskId();

  try {
    let result: ProbeRunResultPayload;
    if (shouldUseNativeBridge()) {
      await ensureNativeBridge();
      const nativeResult = normalizeCommandResult<ProbeRunResultPayload>(
        normalizeNativePayload(
          await cfstNative.RunProbe({
            ...payload,
            task_id: taskId,
          }),
        ),
      );
      if (!nativeResult.ok) {
        return commandResult("PROBE_FAILED", null, {
          message: nativeResult.message || "移动端探测任务执行失败。",
          ok: false,
          taskId,
          warnings: nativeResult.warnings,
        });
      }
      result = nativeResult.data || {};
    } else {
      result = await appBridge().RunDesktopProbe({
        ...payload,
        task_id: taskId,
      });
    }
    const rows = normalizeProbeRows(result.results);

    taskResults.set(taskId, rows);
    taskSnapshots.set(taskId, buildTaskSnapshot(taskId, result, rows));

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
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.CancelProbe(payload)));
  }
  return normalizeCommandResult(await appBridge().CancelProbe(payload));
}

export async function resumeProbe(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    return commandResult(
      "PROBE_RESUME_UNSUPPORTED",
      null,
      {
        message: "Android 端当前不支持继续已暂停的底层测速任务，请重新启动任务。",
        ok: false,
        taskId: toStringValue(payload.task_id),
      },
    );
  }
  return normalizeCommandResult(await appBridge().ResumeProbe(payload));
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
    if (shouldUseNativeBridge()) {
      await ensureNativeBridge();
      const handle = await cfstNative.addListener("desktop:probe", (payload: unknown) => {
        const event = normalizeProbeEvent(normalizeNativePayload(payload));
        if (event) {
          emitProbeEvent(event);
        }
      });
      disposeRuntimeProbeListener = () => {
        void handle.remove();
      };
    } else {
      disposeRuntimeProbeListener = EventsOn("desktop:probe", (payload: unknown) => {
        const event = normalizeProbeEvent(payload);
        if (event) {
          emitProbeEvent(event);
        }
      });
    }
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

  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    await cfstNative.OpenPath({ targetPath: normalized });
    return;
  }

  await appBridge().OpenPath(normalized);
}
