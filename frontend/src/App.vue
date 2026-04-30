<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import {
  deriveTaskStateFromProbeEvent,
  fetchDesktopSource,
  getTaskSnapshot,
  isMaskedTokenValue,
  listenToProbeEvents,
  listDnsRecords,
  listTaskResults,
  loadConfig,
  normalizeConfigSnapshot,
  normalizeDnsRecords,
  openPath,
  previewDesktopSource,
  pushDnsRecords as pushDesktopDnsRecords,
  resumeProbe,
  saveConfig,
  startProbe,
  stopProbe,
  type ConfigSnapshot,
  type DesktopSourceConfig,
  type DnsRecordSnapshot,
  type ProbeEventEnvelope,
  type ProbeResult,
  type ProbeResultFilter,
  type ProbeResultOrder,
  type ProbeResultSortBy,
  type ProbeStrategy,
  type SourcePreviewPayload,
  type SourceIPMode,
  type SourceKind,
  type TaskSnapshot,
  type TaskTone,
} from "./lib/bridge";
import DesktopShell from "./components/layout/DesktopShell.vue";
import MobileShell from "./components/layout/MobileShell.vue";
import ToastStack from "./components/ui/ToastStack.vue";
import DashboardView from "./views/DashboardView.vue";
import DnsView from "./views/DnsView.vue";
import SettingsView from "./views/SettingsView.vue";
import SourcesView from "./views/SourcesView.vue";

type ViewName = "dashboard" | "sources" | "settings" | "dns";
type ToastTone = "success" | "error" | "info";

interface HistoryEntry {
  detail: string;
  exported: number;
  failureSummary: string;
  targetPath: string;
  taskId: string;
  title: string;
  tone: TaskTone;
  updatedAt: string;
}

interface SettingsForm {
  apiToken: string;
  comment: string;
  exportFileName: string;
  exportOverwrite: string;
  exportTargetDir: string;
  maxHttpLatencyMs: number | null;
  maxTcpLatencyMs: number | null;
  maxLossRate: number;
  minDownloadMbps: number;
  minDelayMs: number;
  probeDebug: boolean;
  probeDebugCaptureAddress: string;
  probeDisableDownload: boolean;
  probeConcurrencyStage1: number;
  probeConcurrencyStage2: number;
  probeConcurrencyStage3: number;
  probeCooldownFailures: number;
  probeCooldownMs: number;
  probeDownloadCount: number;
  probeDownloadTimeSeconds: number;
  probeEventThrottleMs: number;
  probeHostHeader: string;
  probeHttping: boolean;
  probeHttpingCfColo: string;
  probeHttpingStatusCode: number;
  probePingTimes: number;
  probePrintNum: number;
  probeRetryBackoffMs: number;
  probeRetryMaxAttempts: number;
  probeSNI: string;
  probeStageLimitStage1: number;
  probeStageLimitStage2: number;
  probeStageLimitStage3: number;
  probeStrategy: ProbeStrategy;
  probeTcpPort: number;
  probeTimeoutStage1Ms: number;
  probeTimeoutStage2Ms: number;
  probeTimeoutStage3Ms: number;
  probeURL: string;
  probeUserAgent: string;
  proxied: boolean;
  recordName: string;
  recordType: "A" | "AAAA";
  ttl: number;
  zoneId: string;
}

const DEFAULT_PROBE_USER_AGENT =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0";

interface SourceDraft extends DesktopSourceConfig {}

interface SourcePreviewState {
  action: string;
  entries: string[];
  invalidCount: number;
  totalCount: number;
  updatedAt: string;
  warnings: string[];
}

interface ProcessTraceEntry {
  detail: string;
  id: number;
  stage: string;
  title: string;
  tone: "success" | "error" | "running" | "info" | "warning";
  ts: string;
}

interface ToastEntry {
  id: number;
  message: string;
  tone: ToastTone;
}

const views: Array<{ id: ViewName; title: string; copy: string; shortLabel: string }> = [
  { id: "dashboard", title: "任务看板", copy: "运行状态、进度条与测试进程", shortLabel: "看板" },
  { id: "sources", title: "输入源", copy: "全局保存的来源、状态与 IP 模式", shortLabel: "来源" },
  { id: "settings", title: "系统配置", copy: "Cloudflare、导出与探测参数", shortLabel: "配置" },
  { id: "dns", title: "DNS 推送", copy: "读取当前记录并执行覆盖推送", shortLabel: "云推" },
];

const routeTitles: Record<ViewName, string> = {
  dashboard: "任务看板",
  dns: "DNS 记录推送",
  settings: "系统配置",
  sources: "输入源管理",
};

const selectedView = ref<ViewName>("dashboard");
const activityFeed = ref<Array<{ detail: string; title: string; ts: string }>>([]);
const configPath = ref("");
const dnsPushText = ref("");
const dnsRecords = ref<DnsRecordSnapshot[]>([]);
const exportHistory = ref<HistoryEntry[]>([]);
const isLoadingDns = ref(false);
const loading = ref(false);
const logs = ref<Array<{ event: string; payload: unknown; ts: string }>>([]);
const maskedTokenHint = ref("");
const processTrace = ref<ProcessTraceEntry[]>([]);
const probeWarnings = ref<string[]>([]);
const resultFilter = ref<ProbeResultFilter>("all");
const resultOrder = ref<ProbeResultOrder>("asc");
const resultRows = ref<ProbeResult[]>([]);
const resultSortBy = ref<ProbeResultSortBy>("address");
const resultsLoading = ref(false);
const showToken = ref(false);
const sourceSeed = ref(0);
const sourcePreviewStates = reactive<Record<string, SourcePreviewState>>({});
const sourceRequestStates = reactive<Record<string, string>>({});
const taskSnapshot = ref<TaskSnapshot | null>(null);
const toasts = ref<ToastEntry[]>([]);

const sources = ref<SourceDraft[]>([createSourceDraft()]);

const status = reactive({
  detail: "先读取配置，再决定启动探测任务还是执行 DNS 推送。",
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

const dnsPushSummary = reactive({
  created: 0,
  deleted: 0,
  hasRun: false,
  ignored: 0,
  message: "尚未执行推送。",
  updated: 0,
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

const settings = reactive<SettingsForm>({
  apiToken: "",
  comment: "",
  exportFileName: "",
  exportOverwrite: "replace_on_start",
  exportTargetDir: "",
  maxHttpLatencyMs: null,
  maxTcpLatencyMs: null,
  maxLossRate: 1,
  minDownloadMbps: 0,
  minDelayMs: 0,
  probeDebug: false,
  probeDebugCaptureAddress: "",
  probeDisableDownload: true,
  probeConcurrencyStage1: 200,
  probeConcurrencyStage2: 10,
  probeConcurrencyStage3: 1,
  probeCooldownFailures: 3,
  probeCooldownMs: 250,
  probeDownloadCount: 10,
  probeDownloadTimeSeconds: 10,
  probeEventThrottleMs: 100,
  probeHostHeader: "",
  probeHttping: false,
  probeHttpingCfColo: "",
  probeHttpingStatusCode: 0,
  probePingTimes: 4,
  probePrintNum: 10,
  probeRetryBackoffMs: 0,
  probeRetryMaxAttempts: 0,
  probeSNI: "",
  probeStageLimitStage1: 512,
  probeStageLimitStage2: 64,
  probeStageLimitStage3: 10,
  probeStrategy: "fast",
  probeTcpPort: 443,
  probeTimeoutStage1Ms: 1000,
  probeTimeoutStage2Ms: 1000,
  probeTimeoutStage3Ms: 10000,
  probeURL: "https://cf.xiu2.xyz/url",
  probeUserAgent: DEFAULT_PROBE_USER_AGENT,
  proxied: false,
  recordName: "",
  recordType: "A",
  ttl: 1,
  zoneId: "",
});

let removeProbeListener: (() => void) | null = null;
let processTraceId = 0;
let snapshotRefreshInFlight = false;
let snapshotRefreshPending = false;
let toastId = 0;

const dashboardStatusLabel = computed(
  () =>
    (
      {
        completed: "已完成",
        cooling: "冷却中",
        failed: "失败",
        idle: "就绪",
        no_results: "无结果",
        partial: "部分完成",
        preparing: "准备中",
        running: "运行中",
      } as Record<TaskTone, string>
    )[status.tone] || status.title
);
const sourcePayloads = computed(() =>
  sources.value.map((source, index) => ({
    content: source.content.trim(),
    enabled: source.enabled,
    id: source.id,
    ip_limit: Math.max(1, Math.floor(source.ip_limit || 1)),
    ip_mode: source.ip_mode,
    kind: source.kind,
    last_fetched_at: source.last_fetched_at,
    last_fetched_count: source.last_fetched_count,
    name: source.name.trim() || `输入源 ${index + 1}`,
    path: source.path.trim(),
    status_text: source.status_text,
    url: source.url.trim(),
  }))
);
const preparedSources = computed(() =>
  sourcePayloads.value.filter((source) => source.enabled && hasUsableSourceInput(source))
);
const progressPercent = computed(() => {
  const total =
    summary.total > 0
      ? summary.total
      : summary.accepted + summary.filtered + summary.invalid > 0
        ? summary.accepted + summary.filtered + summary.invalid
        : summary.accepted;

  if (total <= 0) {
    return 0;
  }

  return Math.max(0, Math.min(100, Math.round((summary.processed / total) * 100)));
});
const hasActiveTask = computed(() => Boolean(task.taskId) && task.active);
const lastHistoryEntry = computed(() => exportHistory.value[0] || null);
const saveBlockedByMaskedToken = computed(() => Boolean(maskedTokenHint.value) && !settings.apiToken.trim());
const resultFilterOptions: Array<{ label: string; value: ProbeResultFilter }> = [
  { label: "全部", value: "all" },
  { label: "已导出", value: "exported" },
  { label: "待处理", value: "pending" },
  { label: "失败", value: "failed" },
];
const resultSortOptions: Array<{ label: string; value: ProbeResultSortBy }> = [
  { label: "地址", value: "address" },
  { label: "阶段", value: "stage" },
  { label: "TCP", value: "tcp" },
  { label: "HTTP", value: "http" },
  { label: "下载", value: "download" },
  { label: "导出", value: "export_status" },
];

function createSourceDraft(kind: SourceKind = "url"): SourceDraft {
  sourceSeed.value += 1;
  return {
    content: "",
    enabled: true,
    id: `source-${Date.now()}-${sourceSeed.value}`,
    ip_limit: 3000,
    ip_mode: "traverse",
    kind,
    last_fetched_at: "",
    last_fetched_count: 0,
    name: `输入源 ${sourceSeed.value}`,
    path: "",
    status_text: "",
    url: "",
  };
}

function hasUsableSourceInput(source: Pick<SourceDraft, "content" | "kind" | "path" | "url">) {
  if (source.kind === "inline") {
    return Boolean(source.content.trim());
  }

  if (source.kind === "file") {
    return Boolean(source.path.trim());
  }

  return Boolean(source.url.trim());
}

function asCount(value: unknown, fallback = 0) {
  const parsed = Number.parseInt(String(value ?? ""), 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function asString(value: unknown) {
  return typeof value === "string" ? value : value == null ? "" : String(value);
}

function asNumber(value: unknown, fallback = 0) {
  const parsed = Number.parseFloat(String(value ?? ""));
  return Number.isFinite(parsed) ? parsed : fallback;
}

function asNullableNumber(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return null;
  }

  const parsed = Number.parseFloat(String(value));
  return Number.isFinite(parsed) ? parsed : null;
}

function optionalNumberForPayload(value: number | null) {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function showToast(message: string, tone: ToastTone = "success") {
  const nextId = toastId;
  toastId += 1;
  toasts.value.push({
    id: nextId,
    message,
    tone,
  });
  setTimeout(() => {
    toasts.value = toasts.value.filter((toast) => toast.id !== nextId);
  }, 3200);
}

function appendLog(event: string, payload: unknown) {
  logs.value = [...logs.value, { event, payload, ts: new Date().toISOString() }].slice(-160);
}

function clearProcessTrace() {
  processTrace.value = [];
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

function pushProcessTrace(entry: Omit<ProcessTraceEntry, "id">) {
  const nextEntry: ProcessTraceEntry = {
    ...entry,
    id: processTraceId,
  };
  processTraceId += 1;

  const current = [...processTrace.value];
  const lastEntry = current[0];
  if (lastEntry && lastEntry.stage === nextEntry.stage && lastEntry.title === nextEntry.title && lastEntry.tone === nextEntry.tone) {
    current[0] = nextEntry;
    processTrace.value = current;
    return;
  }

  processTrace.value = [nextEntry, ...current].slice(0, 80);
}

function pushActivity(title: string, detail: string) {
  activityFeed.value = [
    {
      detail,
      title,
      ts: new Date().toISOString(),
    },
    ...activityFeed.value,
  ].slice(0, 10);
}

function setStatus(next: { title: string; detail: string; tone: TaskTone }) {
  status.title = next.title;
  status.detail = next.detail;
  status.tone = next.tone;
}

function resetProbeSummary() {
  summary.accepted = 0;
  summary.exported = 0;
  summary.failed = 0;
  summary.filtered = 0;
  summary.invalid = 0;
  summary.passed = 0;
  summary.processed = 0;
  summary.total = 0;
  clearProcessTrace();
}

function addSource() {
  sources.value.push(createSourceDraft());
}

function removeSource(sourceId: string) {
  sources.value = sources.value.filter((source) => source.id !== sourceId);
  delete sourcePreviewStates[sourceId];
  delete sourceRequestStates[sourceId];
}

function allocateTaskId() {
  return `cfst-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
}

function updateHistory(entry: HistoryEntry) {
  const nextEntries = [...exportHistory.value];
  const currentIndex = nextEntries.findIndex((item) => item.taskId === entry.taskId);

  if (currentIndex >= 0) {
    nextEntries[currentIndex] = entry;
  } else {
    nextEntries.unshift(entry);
  }

  exportHistory.value = nextEntries.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt));
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

function applySourceStatuses(statusesValue: unknown) {
  if (!Array.isArray(statusesValue) || statusesValue.length === 0) {
    return;
  }

  const nextStatusMap = new Map(
    statusesValue
      .map((entry) => {
        const source = asRecord(entry);
        const sourceId = asString(source.id).trim();

        if (!sourceId) {
          return null;
        }

        return [
          sourceId,
          {
            last_fetched_at: asString(source.last_fetched_at || source.lastFetchedAt).trim(),
            last_fetched_count: Math.max(0, asCount(source.last_fetched_count || source.lastFetchedCount, 0)),
            status_text: asString(source.status_text || source.statusText).trim(),
          },
        ] as const;
      })
      .filter((entry): entry is readonly [string, { last_fetched_at: string; last_fetched_count: number; status_text: string }] => Boolean(entry))
  );

  if (nextStatusMap.size === 0) {
    return;
  }

  sources.value = sources.value.map((source) => {
    const statusEntry = nextStatusMap.get(source.id);
    if (!statusEntry) {
      return source;
    }

    return {
      ...source,
      last_fetched_at: statusEntry.last_fetched_at,
      last_fetched_count: statusEntry.last_fetched_count,
      status_text: statusEntry.status_text,
    };
  });
}

function applySourceStatus(statusValue: unknown) {
  if (!statusValue) {
    return;
  }
  applySourceStatuses([statusValue]);
}

function applySourcePreview(sourceId: string, dataValue: unknown, warnings: string[]) {
  const data = asRecord(dataValue);
  const summaryRecord = asRecord(data.summary);
  const entries = Array.isArray(data.preview_entries) ? data.preview_entries.map((entry) => asString(entry).trim()).filter(Boolean) : [];

  sourcePreviewStates[sourceId] = {
    action: asString(summaryRecord.action || "预览"),
    entries,
    invalidCount: asCount(summaryRecord.invalid_count, 0),
    totalCount: asCount(summaryRecord.total_count, entries.length),
    updatedAt: new Date().toISOString(),
    warnings,
  };
}

function applyConfigSnapshot(snapshot: ConfigSnapshot) {
  const normalized = normalizeConfigSnapshot(snapshot);
  const apiToken = normalized.cloudflare.api_token || "";

  maskedTokenHint.value = isMaskedTokenValue(apiToken) ? apiToken : "";
  settings.apiToken = maskedTokenHint.value ? "" : apiToken;
  settings.comment = normalized.cloudflare.comment || "";
  settings.exportFileName = normalized.export.file_name || "";
  settings.exportOverwrite = normalized.export.overwrite || "replace_on_start";
  settings.exportTargetDir = normalized.export.target_dir || "";
  settings.maxHttpLatencyMs = asNullableNumber(normalized.probe.thresholds.max_http_latency_ms);
  settings.maxTcpLatencyMs = asNullableNumber(normalized.probe.thresholds.max_tcp_latency_ms);
  settings.maxLossRate = asNumber(normalized.probe.max_loss_rate, 1);
  settings.minDownloadMbps = asNumber(normalized.probe.thresholds.min_download_mbps, 0);
  settings.minDelayMs = asCount(normalized.probe.min_delay_ms, 0);
  settings.probeDebug = Boolean(normalized.probe.debug);
  settings.probeDebugCaptureAddress = normalized.probe.debug_capture_address || "";
  settings.probeDisableDownload = normalized.probe.strategy === "fast";
  settings.probeConcurrencyStage1 = normalized.probe.concurrency.stage1;
  settings.probeConcurrencyStage2 = normalized.probe.concurrency.stage2;
  settings.probeConcurrencyStage3 = normalized.probe.concurrency.stage3;
  settings.probeCooldownFailures = normalized.probe.cooldown_policy.consecutive_failures;
  settings.probeCooldownMs = normalized.probe.cooldown_policy.cooldown_ms;
  settings.probeDownloadCount = normalized.probe.download_count;
  settings.probeDownloadTimeSeconds = normalized.probe.download_time_seconds;
  settings.probeEventThrottleMs = normalized.probe.event_throttle_ms;
  settings.probeHostHeader = normalized.probe.host_header || "";
  settings.probeHttping = Boolean(normalized.probe.httping);
  settings.probeHttpingCfColo = normalized.probe.httping_cf_colo || "";
  settings.probeHttpingStatusCode = normalized.probe.httping_status_code;
  settings.probePingTimes = normalized.probe.ping_times;
  settings.probePrintNum = normalized.probe.print_num;
  settings.probeRetryBackoffMs = normalized.probe.retry_policy.backoff_ms;
  settings.probeRetryMaxAttempts = normalized.probe.retry_policy.max_attempts;
  settings.probeSNI = normalized.probe.sni || "";
  settings.probeStageLimitStage1 = normalized.probe.stage_limits.stage1;
  settings.probeStageLimitStage2 = normalized.probe.stage_limits.stage2;
  settings.probeStageLimitStage3 = normalized.probe.stage_limits.stage3;
  settings.probeStrategy = normalized.probe.strategy;
  settings.probeTcpPort = normalized.probe.tcp_port;
  settings.probeTimeoutStage1Ms = normalized.probe.timeouts.stage1_ms;
  settings.probeTimeoutStage2Ms = normalized.probe.timeouts.stage2_ms;
  settings.probeTimeoutStage3Ms = normalized.probe.timeouts.stage3_ms;
  settings.probeURL = normalized.probe.url || "https://cf.xiu2.xyz/url";
  settings.probeUserAgent = normalized.probe.user_agent || DEFAULT_PROBE_USER_AGENT;
  settings.proxied = Boolean(normalized.cloudflare.proxied);
  settings.recordName = normalized.cloudflare.record_name || "";
  settings.recordType = normalized.cloudflare.record_type === "AAAA" ? "AAAA" : "A";
  settings.ttl = asCount(normalized.cloudflare.ttl, 1) || 1;
  settings.zoneId = normalized.cloudflare.zone_id || "";
  sources.value = normalized.sources.length > 0 ? normalized.sources.map((source) => ({ ...source })) : [createSourceDraft()];
}

function buildConfigSnapshot() {
  const normalizedStrategy: ProbeStrategy = settings.probeStrategy === "full" ? "full" : "fast";

  return {
    cloudflare: {
      ...(settings.apiToken.trim() ? { api_token: settings.apiToken.trim() } : {}),
      comment: settings.comment.trim(),
      proxied: settings.proxied,
      record_name: settings.recordName.trim(),
      record_type: settings.recordType,
      ttl: settings.ttl,
      zone_id: settings.zoneId.trim(),
    },
    export: {
      ...(settings.exportFileName.trim() ? { file_name: settings.exportFileName.trim() } : {}),
      ...(settings.exportOverwrite.trim() ? { overwrite: settings.exportOverwrite.trim() } : {}),
      ...(settings.exportTargetDir.trim() ? { target_dir: settings.exportTargetDir.trim() } : {}),
    },
    probe: {
      concurrency: {
        stage1: settings.probeConcurrencyStage1,
        stage2: settings.probeConcurrencyStage2,
        stage3: settings.probeConcurrencyStage3,
      },
      cooldown_policy: {
        consecutive_failures: settings.probeCooldownFailures,
        cooldown_ms: settings.probeCooldownMs,
      },
      debug: settings.probeDebug,
      debug_capture_address: settings.probeDebugCaptureAddress.trim(),
      disable_download: normalizedStrategy === "fast",
      download_count: settings.probeDownloadCount,
      download_time_seconds: settings.probeDownloadTimeSeconds,
      event_throttle_ms: settings.probeEventThrottleMs,
      host_header: settings.probeHostHeader.trim(),
      httping: settings.probeHttping,
      httping_cf_colo: settings.probeHttpingCfColo.trim(),
      httping_status_code: settings.probeHttpingStatusCode,
      max_loss_rate: settings.maxLossRate,
      min_delay_ms: settings.minDelayMs,
      ping_times: settings.probePingTimes,
      print_num: settings.probePrintNum,
      retry_policy: {
        backoff_ms: settings.probeRetryBackoffMs,
        max_attempts: settings.probeRetryMaxAttempts,
      },
      skip_first_latency_sample: true,
      stage_limits: {
        stage1: settings.probeStageLimitStage1,
        stage2: settings.probeStageLimitStage2,
        stage3: settings.probeStageLimitStage3,
      },
      strategy: normalizedStrategy,
      sni: settings.probeSNI.trim(),
      tcp_port: settings.probeTcpPort,
      test_all: false,
      thresholds: {
        max_http_latency_ms: optionalNumberForPayload(settings.maxHttpLatencyMs),
        max_tcp_latency_ms: optionalNumberForPayload(settings.maxTcpLatencyMs),
        min_download_mbps: settings.minDownloadMbps,
      },
      timeouts: {
        stage1_ms: settings.probeTimeoutStage1Ms,
        stage2_ms: settings.probeTimeoutStage2Ms,
        stage3_ms: settings.probeDownloadTimeSeconds * 1000,
      },
      url: settings.probeURL.trim(),
      user_agent: settings.probeUserAgent.trim() || DEFAULT_PROBE_USER_AGENT,
    },
    sources: sourcePayloads.value.map((source) => ({
      ...source,
      status_text: source.status_text.trim(),
    })),
  };
}

function applyTaskSnapshot(snapshot: TaskSnapshot) {
  taskSnapshot.value = snapshot;
  task.taskId = snapshot.task_id || task.taskId;
  task.stage = snapshot.current_stage || snapshot.progress?.stage || task.stage;
  task.completedAt = snapshot.completed_at || "";
  task.active = !["completed", "failed", "no_results"].includes(snapshot.status || "");

  if (snapshot.progress) {
    summary.failed = asCount(snapshot.progress.failed, summary.failed);
    summary.passed = asCount(snapshot.progress.passed, summary.passed);
    summary.processed = asCount(snapshot.progress.processed, summary.processed);
    summary.total = asCount(snapshot.progress.total, summary.total);
  }

  if (snapshot.export_record) {
    summary.exported = asCount(snapshot.export_record.written_count, summary.exported);
    task.exportPath = [snapshot.export_record.target_dir, snapshot.export_record.file_name].filter(Boolean).join("/");
  }
}

async function refreshTaskData(taskId = task.taskId) {
  const normalizedTaskId = taskId.trim();

  if (!normalizedTaskId) {
    return;
  }

  if (snapshotRefreshInFlight) {
    snapshotRefreshPending = true;
    return;
  }

  snapshotRefreshInFlight = true;
  resultsLoading.value = true;

  try {
    do {
      snapshotRefreshPending = false;
      const [snapshotResult, resultsResult] = await Promise.all([
        getTaskSnapshot(normalizedTaskId),
        listTaskResults(normalizedTaskId, resultSortBy.value, resultOrder.value, resultFilter.value),
      ]);

      appendLog("bridge.get_task_snapshot", snapshotResult);
      appendLog("bridge.list_task_results", resultsResult);

      if (snapshotResult.ok && snapshotResult.data) {
        applyTaskSnapshot(snapshotResult.data);
      }

      if (resultsResult.ok && resultsResult.data) {
        resultRows.value = Array.isArray(resultsResult.data.results) ? resultsResult.data.results : [];
      }
    } while (snapshotRefreshPending);
  } finally {
    snapshotRefreshInFlight = false;
    resultsLoading.value = false;
  }
}

function applyProbeEvent(event: ProbeEventEnvelope) {
  appendLog(event.event, event.payload);
  const nextTaskState = deriveTaskStateFromProbeEvent(event);

  setStatus(nextTaskState);
  task.active = !["completed", "failed", "no_results"].includes(nextTaskState.tone);
  task.lastEvent = event.event;
  task.lastSeq = event.seq;
  task.taskId = event.task_id || task.taskId;

  if (event.event === "probe.preprocessed") {
    summary.accepted = asCount(event.payload.accepted);
    summary.filtered = asCount(event.payload.filtered);
    summary.invalid = asCount(event.payload.invalid);
    summary.processed = 0;
    summary.passed = 0;
    summary.failed = 0;
    summary.total = asCount(event.payload.total, summary.accepted);
    task.stage = "preprocessed";
    applySourceStatuses(event.payload.source_statuses);
    pushProcessTrace({
      detail: `候选 ${summary.total} 条，接受 ${summary.accepted} 条，过滤 ${summary.filtered} 条，非法 ${summary.invalid} 条。`,
      stage: "preprocessed",
      title: "输入源预处理完成",
      tone: summary.accepted > 0 ? "success" : "warning",
      ts: event.ts,
    });
  }

  if (event.event === "probe.progress") {
    summary.failed = asCount(event.payload.failed);
    summary.passed = asCount(event.payload.passed);
    summary.processed = asCount(event.payload.processed);
    summary.total = asCount(event.payload.total, summary.total);
    task.stage = asString(event.payload.stage) || "running";
    pushProcessTrace({
      detail: `已处理 ${summary.processed}/${summary.total || "-"}，通过 ${summary.passed}，失败 ${summary.failed}。`,
      stage: task.stage,
      title: task.stage === "download" ? "下载测速进行中" : "延迟测速进行中",
      tone: "running",
      ts: event.ts,
    });
  }

  if (event.event === "probe.partial_export") {
    summary.exported = asCount(event.payload.written, summary.exported);
    task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
    updateHistory({
      detail: status.detail,
      exported: summary.exported,
      failureSummary: "",
      targetPath: task.exportPath,
      taskId: task.taskId,
      title: status.title,
      tone: "partial",
      updatedAt: event.ts,
    });
    pushProcessTrace({
      detail: task.exportPath ? `已写出 ${summary.exported} 条结果到 ${task.exportPath}。` : `已整理 ${summary.exported} 条结果。`,
      stage: "export",
      title: "结果已落盘",
      tone: "success",
      ts: event.ts,
    });
  }

  if (event.event === "probe.completed") {
    task.active = false;
    task.completedAt = event.ts;
    summary.exported = asCount(event.payload.exported, summary.exported);
    summary.failed = Math.max(summary.failed, asCount(event.payload.failed, summary.failed));
    const resultCount = Math.max(
      asCount(event.payload.result_count, 0),
      asCount(event.payload.passed, 0),
      summary.passed,
      summary.exported,
      resultRows.value.length,
    );
    summary.passed = Math.max(summary.passed, resultCount);
    task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
    const hasResults = resultCount > 0;
    updateHistory({
      detail: status.detail,
      exported: summary.exported,
      failureSummary: summarizeFailureSummary(event.payload.failure_summary),
      targetPath: task.exportPath,
      taskId: task.taskId,
      title: status.title,
      tone: hasResults ? "completed" : "no_results",
      updatedAt: event.ts,
    });
    pushProcessTrace({
      detail: hasResults
        ? `任务完成，可用结果 ${resultCount} 条${task.exportPath ? `，导出路径 ${task.exportPath}` : ""}。`
        : "任务执行完成，但当前筛选条件下没有可用结果。",
      stage: "completed",
      title: hasResults ? "探测任务完成" : "任务完成但无结果",
      tone: hasResults ? "success" : "warning",
      ts: event.ts,
    });
    showToast(hasResults ? "探测任务已完成" : "任务结束但没有可用结果", hasResults ? "success" : "info");
  }

  if (event.event === "probe.failed") {
    task.active = false;
    task.completedAt = event.ts;
    task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
    const failureMessage = asString(event.payload.message || status.detail).trim() || "探测任务失败。";
    updateHistory({
      detail: failureMessage,
      exported: summary.exported,
      failureSummary: "",
      targetPath: task.exportPath,
      taskId: task.taskId,
      title: status.title,
      tone: "failed",
      updatedAt: event.ts,
    });
    pushProcessTrace({
      detail: failureMessage,
      stage: "failed",
      title: "探测任务失败",
      tone: "error",
      ts: event.ts,
    });
    showToast("探测任务失败", "error");
  }

  if (event.event === "probe.cooling") {
    task.stage = "cooling";
    pushProcessTrace({
      detail: asString(event.payload.reason || "当前任务进入冷却阶段。"),
      stage: "cooling",
      title: "任务进入冷却",
      tone: "warning",
      ts: event.ts,
    });
  }

  pushActivity(nextTaskState.title, nextTaskState.detail);
}

async function inspectSource(sourceId: string, action: "preview" | "fetch") {
  const source = sources.value.find((entry) => entry.id === sourceId);
  if (!source) {
    return;
  }

  sourceRequestStates[sourceId] = action;
  try {
    const payload = {
      config: buildConfigSnapshot(),
      preview_limit: 16,
      source: {
        ...source,
        content: source.content.trim(),
        name: source.name.trim(),
        path: source.path.trim(),
        url: source.url.trim(),
      },
    };
    const result =
      action === "fetch" ? await fetchDesktopSource(payload) : await previewDesktopSource(payload);
    const data = asRecord(result.data as SourcePreviewPayload | null);
    appendLog(`bridge.source_${action}`, result);

    if (!result.ok) {
      setStatus({
        detail: result.message || `${action === "fetch" ? "抓取" : "预览"}输入源失败。`,
        title: `${action === "fetch" ? "抓取" : "预览"}失败`,
        tone: "failed",
      });
      showToast(`${action === "fetch" ? "抓取" : "预览"}失败`, "error");
      return;
    }

    applySourcePreview(sourceId, data, result.warnings || []);
    if (action === "fetch") {
      applySourceStatus(data.source_status);
    }

    setStatus({
      detail: result.message || `${action === "fetch" ? "抓取" : "预览"}输入源成功。`,
      title: `${action === "fetch" ? "抓取" : "预览"}已完成`,
      tone: "idle",
    });
    showToast(action === "fetch" ? "来源抓取已完成" : "来源预览已更新", "success");
  } finally {
    delete sourceRequestStates[sourceId];
  }
}

async function refreshConfig() {
  loading.value = true;

  try {
    const result = await loadConfig();
    const data = asRecord(result.data);
    appendLog("bridge.load_config", result);
    probeWarnings.value = result.warnings || [];

    if (!result.ok) {
      setStatus({
        detail: result.message || "读取配置失败。",
        title: "读取失败",
        tone: "failed",
      });
      pushActivity("读取配置失败", result.message || "读取配置失败。");
      showToast("读取配置失败", "error");
      return;
    }

    applyConfigSnapshot(normalizeConfigSnapshot(data.config_snapshot || {}));
    configPath.value = asString(data.configPath || data.config_path || "");
    setStatus({
      detail: result.message || "配置已加载。",
      title: "配置已加载",
      tone: "idle",
    });
    pushActivity("配置已加载", result.message || "已读取当前配置快照。");
    showToast("配置已加载");
  } finally {
    loading.value = false;
  }
}

async function persistConfig() {
  if (saveBlockedByMaskedToken.value) {
    setStatus({
      detail: "当前只拿到了脱敏 Token。请重新输入完整 API Token 后再保存。",
      title: "需要完整 Token",
      tone: "failed",
    });
    pushActivity("保存被阻止", "检测到脱敏 Token，占位值不能直接回写。");
    selectedView.value = "settings";
    showToast("需要重新输入完整 Token", "error");
    return;
  }

  loading.value = true;

  try {
    const result = await saveConfig({
      config_snapshot: buildConfigSnapshot(),
    });
    const data = asRecord(result.data);
    appendLog("bridge.save_config", result);
    probeWarnings.value = result.warnings || [];

    if (!result.ok) {
      setStatus({
        detail: result.message || "保存配置失败。",
        title: "保存失败",
        tone: "failed",
      });
      pushActivity("保存失败", result.message || "保存配置失败。");
      showToast("保存配置失败", "error");
      return;
    }

    applyConfigSnapshot(normalizeConfigSnapshot(data.config_snapshot || {}));
    configPath.value = asString(data.configPath || data.config_path || configPath.value);
    setStatus({
      detail: result.message || "配置已保存。",
      title: "配置已保存",
      tone: "idle",
    });
    pushActivity("配置已保存", result.message || "设置已保存并可用于后续任务。");
    showToast("配置已保存");
  } finally {
    loading.value = false;
  }
}

async function launchProbe() {
  if (preparedSources.value.length === 0) {
    setStatus({
      detail: "至少需要一个已启用且内容完整的输入源，支持手动输入、本地文件或远程 URL。",
      title: "缺少输入源",
      tone: "failed",
    });
    selectedView.value = "sources";
    showToast("请先配置至少一个来源", "error");
    return;
  }

  loading.value = true;
  resetProbeSummary();
  resultRows.value = [];
  taskSnapshot.value = null;
  const taskId = allocateTaskId();
  task.acceptedAt = new Date().toISOString();
  task.active = true;
  task.completedAt = "";
  task.exportPath = "";
  task.lastEvent = "probe.accepted";
  task.lastSeq = 0;
  task.stage = "accepted";
  task.taskId = taskId;
  setStatus({
    detail: "正在准备输入源并启动桌面探测任务。",
    title: "任务提交中",
    tone: "preparing",
  });
  pushProcessTrace({
    detail: `任务 ${taskId} 已提交，等待原生探测引擎开始执行。`,
    stage: "accepted",
    title: "探测任务已提交",
    tone: "info",
    ts: task.acceptedAt,
  });
  pushActivity("任务提交中", `${taskId} 正在等待原生探测引擎处理。`);
  selectedView.value = "dashboard";

  try {
    const result = await startProbe({
      config: buildConfigSnapshot(),
      sources: sourcePayloads.value,
      task_id: taskId,
    });
    const data = asRecord(result.data);
    appendLog("bridge.start_probe", result);
    probeWarnings.value = result.warnings || [];
    pushWarningTrace(probeWarnings.value);
    if (!result.ok) {
      task.active = false;
      task.completedAt = new Date().toISOString();
      pushProcessTrace({
        detail: result.message || "桌面端未接受探测任务。",
        stage: "failed",
        title: "任务启动失败",
        tone: "error",
        ts: task.completedAt,
      });
      if (status.tone !== "failed") {
        setStatus({
          detail: result.message || "启动探测失败。",
          title: "启动失败",
          tone: "failed",
        });
      }
      pushActivity("启动失败", result.message || "桌面端未接受探测任务。");
      if (status.tone !== "failed") {
        showToast("启动探测失败", "error");
      }
      return;
    }

    applySourceStatuses(data.source_statuses || data.sourceStatuses);
    task.exportPath = asString(data.export_path || task.exportPath).trim();
    task.taskId = asString(result.task_id || data.task_id || task.taskId).trim();
    void refreshTaskData(task.taskId || taskId);
  } finally {
    loading.value = false;
  }
}

async function rerunSingleAddress(address: string) {
  const trimmedAddress = address.trim();

  if (!trimmedAddress) {
    return;
  }

  loading.value = true;
  resetProbeSummary();
  resultRows.value = [];
  taskSnapshot.value = null;
  const taskId = allocateTaskId();
  task.acceptedAt = new Date().toISOString();
  task.active = true;
  task.completedAt = "";
  task.exportPath = "";
  task.lastEvent = "probe.accepted";
  task.lastSeq = 0;
  task.stage = "accepted";
  task.taskId = taskId;
  setStatus({
    detail: `${trimmedAddress} 正在提交到桌面探测引擎。`,
    title: "单条重测提交中",
    tone: "preparing",
  });
  pushProcessTrace({
    detail: `${trimmedAddress} 已提交为单条重测任务。`,
    stage: "accepted",
    title: "单条重测已提交",
    tone: "info",
    ts: task.acceptedAt,
  });
  selectedView.value = "dashboard";

  try {
    const result = await startProbe({
      config: buildConfigSnapshot(),
      sources: [
        {
          content: trimmedAddress,
          enabled: true,
          id: `rerun-${Date.now()}`,
          ip_limit: 1,
          ip_mode: "traverse" as SourceIPMode,
          kind: "inline",
          last_fetched_at: "",
          last_fetched_count: 0,
          name: `单条重测 ${trimmedAddress}`,
          path: "",
          status_text: "",
          url: "",
        },
      ],
      task_id: taskId,
    });
    const data = asRecord(result.data);
    appendLog("bridge.start_probe.single", result);
    probeWarnings.value = result.warnings || [];
    pushWarningTrace(probeWarnings.value);

    if (!result.ok) {
      task.active = false;
      task.completedAt = new Date().toISOString();
      pushProcessTrace({
        detail: result.message || "单条重测未能启动。",
        stage: "failed",
        title: "单条重测失败",
        tone: "error",
        ts: task.completedAt,
      });
      if (status.tone !== "failed") {
        setStatus({
          detail: result.message || "单条重测启动失败。",
          title: "重测失败",
          tone: "failed",
        });
        showToast("单条重测失败", "error");
      }
      return;
    }

    task.exportPath = asString(data.export_path || task.exportPath).trim();
    task.taskId = asString(result.task_id || data.task_id || task.taskId).trim();
    void refreshTaskData(task.taskId || taskId);
  } finally {
    loading.value = false;
  }
}

async function copyAddress(address: string) {
  const trimmedAddress = address.trim();

  if (!trimmedAddress) {
    return;
  }

  try {
    await navigator.clipboard.writeText(trimmedAddress);
    showToast("IP 已复制", "success");
  } catch (_error) {
    showToast("复制失败，请手动选择该地址", "error");
  }
}

function updateResultSort(sortBy: ProbeResultSortBy) {
  if (resultSortBy.value === sortBy) {
    resultOrder.value = resultOrder.value === "asc" ? "desc" : "asc";
  } else {
    resultSortBy.value = sortBy;
    resultOrder.value = sortBy === "download" ? "desc" : "asc";
  }

  void refreshTaskData();
}

function updateResultFilter(filter: ProbeResultFilter) {
  resultFilter.value = filter;
  void refreshTaskData();
}

function updateResultOrder(order: ProbeResultOrder) {
  resultOrder.value = order;
  void refreshTaskData();
}

function refreshCurrentTaskData() {
  void refreshTaskData();
}

async function pauseProbe() {
  if (!task.taskId) {
    return;
  }

  loading.value = true;

  try {
    const result = await stopProbe({
      mode: "pause",
      task_id: task.taskId,
    });
    appendLog("bridge.stop_probe", result);

    if (!result.ok) {
      setStatus({
        detail: result.message || "暂停失败。",
        title: "暂停失败",
        tone: "failed",
      });
      showToast("暂停失败", "error");
      return;
    }

    setStatus({
      detail: result.message || "已请求暂停，等待 cooling 事件确认。",
      title: "暂停请求已发送",
      tone: "running",
    });
    pushActivity("请求暂停", result.message || "已向桌面端发送暂停请求。");
    showToast("已请求暂停", "info");
  } finally {
    loading.value = false;
  }
}

async function continueProbe() {
  if (!task.taskId) {
    return;
  }

  loading.value = true;

  try {
    const result = await resumeProbe({
      task_id: task.taskId,
    });
    appendLog("bridge.resume_probe", result);

    if (!result.ok) {
      setStatus({
        detail: result.message || "恢复失败。",
        title: "恢复失败",
        tone: "failed",
      });
      showToast("恢复失败", "error");
      return;
    }

    setStatus({
      detail: result.message || "已请求恢复，等待新的 progress 事件。",
      title: "恢复请求已发送",
      tone: "running",
    });
    pushActivity("请求恢复", result.message || "已向桌面端发送恢复请求。");
    showToast("已请求恢复", "info");
  } finally {
    loading.value = false;
  }
}

async function fetchDnsRecords() {
  isLoadingDns.value = true;

  try {
    const result = await listDnsRecords();
    const data = asRecord(result.data);
    appendLog("bridge.list_dns_records", result);
    if (!result.ok) {
      setStatus({
        detail: result.message || "读取 DNS 记录失败。",
        title: "DNS 读取失败",
        tone: "failed",
      });
      pushActivity("读取 DNS 失败", result.message || "未能读取当前 Cloudflare 记录。");
      showToast("读取 DNS 失败", "error");
      return;
    }

    dnsRecords.value = normalizeDnsRecords(data.records);
    pushActivity("DNS 已同步", result.message || `共加载 ${dnsRecords.value.length} 条记录。`);
    showToast(dnsRecords.value.length > 0 ? `已同步 ${dnsRecords.value.length} 条记录` : "当前没有匹配记录", "info");
  } finally {
    isLoadingDns.value = false;
  }
}

async function pushToDns() {
  if (!dnsPushText.value.trim()) {
    selectedView.value = "dns";
    showToast("没有可推送的 IP", "error");
    return;
  }

  loading.value = true;

  try {
    const result = await pushDesktopDnsRecords({
      ipsRaw: dnsPushText.value,
    });
    const data = asRecord(result.data);
    const pushSummary = asRecord(data.summary);
    const ignoredEntries = Array.isArray(data.ignored_entries) ? data.ignored_entries.map((entry) => asString(entry)).filter(Boolean) : [];
    appendLog("bridge.push_dns", result);
    if (!result.ok) {
      setStatus({
        detail: result.message || "DNS 推送失败。",
        title: "推送失败",
        tone: "failed",
      });
      pushActivity("DNS 推送失败", result.message || "未能执行 Cloudflare 覆盖推送。");
      showToast("DNS 推送失败", "error");
      return;
    }

    dnsPushSummary.created = asCount(pushSummary.created);
    dnsPushSummary.deleted = asCount(pushSummary.deleted);
    dnsPushSummary.hasRun = true;
    dnsPushSummary.ignored = ignoredEntries.length;
    dnsPushSummary.message =
      ignoredEntries.length > 0
        ? `推送已完成，但忽略了 ${ignoredEntries.length} 个无效或不匹配输入项。`
        : result.message || "Cloudflare DNS 覆盖推送已完成。";
    dnsPushSummary.updated = asCount(pushSummary.updated);
    dnsRecords.value = normalizeDnsRecords(data.records_after);

    setStatus({
      detail: dnsPushSummary.message,
      title: ignoredEntries.length > 0 ? "推送部分完成" : "推送完成",
      tone: ignoredEntries.length > 0 ? "partial" : "completed",
    });
    pushActivity(
      ignoredEntries.length > 0 ? "DNS 推送部分完成" : "DNS 推送完成",
      `创建 ${dnsPushSummary.created}、更新 ${dnsPushSummary.updated}、删除 ${dnsPushSummary.deleted}${
        ignoredEntries.length > 0 ? `，忽略 ${ignoredEntries.length} 项。` : "。"
      }`
    );
    showToast(ignoredEntries.length > 0 ? `推送完成，忽略 ${ignoredEntries.length} 项` : "DNS 推送成功");
  } finally {
    loading.value = false;
  }
}

async function openHistoryTarget(targetPath: string) {
  if (!targetPath) {
    return;
  }

  await openPath(targetPath);
}

onMounted(async () => {
  appendLog("system.boot", { message: "桌面端调用链已初始化。" });
  pushActivity("桌面端已启动", "等待桌面端返回配置与任务状态。");
  removeProbeListener = await listenToProbeEvents((event) => {
    applyProbeEvent(event);
  });
  await refreshConfig();
});

onBeforeUnmount(() => {
  removeProbeListener?.();
});
</script>

<template>
  <DesktopShell
    :config-path="configPath"
    :loading="loading"
    :route-title="routeTitles[selectedView]"
    :selected-view="selectedView"
    :status-detail="status.detail"
    :status-label="dashboardStatusLabel"
    :status-tone="status.tone"
    :views="views"
    @change-view="selectedView = $event"
    @persist-config="persistConfig"
    @refresh-config="refreshConfig"
  >
    <DashboardView
      v-if="selectedView === 'dashboard'"
      :activity-feed="activityFeed"
      :export-history="exportHistory"
      :has-active-task="hasActiveTask"
      :last-history-entry="lastHistoryEntry"
      :loading="loading"
      platform="desktop"
      :process-trace="processTrace"
      :probe-warnings="probeWarnings"
      :progress-percent="progressPercent"
      :result-filter="resultFilter"
      :result-filter-options="resultFilterOptions"
      :result-order="resultOrder"
      :result-rows="resultRows"
      :result-sort-by="resultSortBy"
      :result-sort-options="resultSortOptions"
      :results-loading="resultsLoading"
      :status-label="dashboardStatusLabel"
      :status-tone="status.tone"
      :summary="summary"
      :task="task"
      :task-snapshot="taskSnapshot"
      @clear-process="clearProcessTrace"
      @copy-address="copyAddress"
      @open-history-target="openHistoryTarget"
      @pause="pauseProbe"
      @refresh-results="refreshCurrentTaskData"
      @rerun-address="rerunSingleAddress"
      @resume="continueProbe"
      @start="launchProbe"
      @update-filter="updateResultFilter"
      @update-order="updateResultOrder"
      @update-sort="updateResultSort"
    />

    <SourcesView
      v-else-if="selectedView === 'sources'"
      :accepted="summary.accepted"
      :invalid="summary.invalid"
      platform="desktop"
      :prepared-count="preparedSources.length"
      :preview-states="sourcePreviewStates"
      :request-states="sourceRequestStates"
      :sources="sources"
      :task-stage="task.stage"
      @add="addSource"
      @fetch-source="inspectSource($event, 'fetch')"
      @preview="inspectSource($event, 'preview')"
      @remove="removeSource"
    />

    <SettingsView
      v-else-if="selectedView === 'settings'"
      :loading="loading"
      :masked-token-hint="maskedTokenHint"
      platform="desktop"
      :save-blocked-by-masked-token="saveBlockedByMaskedToken"
      :settings="settings"
      :show-token="showToken"
      @save="persistConfig"
      @toggle-token="showToken = !showToken"
    />

    <DnsView
      v-else
      :dns-push-summary="dnsPushSummary"
      :dns-push-text="dnsPushText"
      :dns-records="dnsRecords"
      :is-loading-dns="isLoadingDns"
      :loading="loading"
      platform="desktop"
      @fetch="fetchDnsRecords"
      @push="pushToDns"
      @update:dnsPushText="dnsPushText = $event"
    />
  </DesktopShell>

  <MobileShell
    :route-title="routeTitles[selectedView]"
    :selected-view="selectedView"
    :views="views"
    @change-view="selectedView = $event"
  >
    <DashboardView
      v-if="selectedView === 'dashboard'"
      :activity-feed="activityFeed"
      :export-history="exportHistory"
      :has-active-task="hasActiveTask"
      :last-history-entry="lastHistoryEntry"
      :loading="loading"
      platform="mobile"
      :process-trace="processTrace"
      :probe-warnings="probeWarnings"
      :progress-percent="progressPercent"
      :result-filter="resultFilter"
      :result-filter-options="resultFilterOptions"
      :result-order="resultOrder"
      :result-rows="resultRows"
      :result-sort-by="resultSortBy"
      :result-sort-options="resultSortOptions"
      :results-loading="resultsLoading"
      :status-label="dashboardStatusLabel"
      :status-tone="status.tone"
      :summary="summary"
      :task="task"
      :task-snapshot="taskSnapshot"
      @clear-process="clearProcessTrace"
      @copy-address="copyAddress"
      @open-history-target="openHistoryTarget"
      @pause="pauseProbe"
      @refresh-results="refreshCurrentTaskData"
      @rerun-address="rerunSingleAddress"
      @resume="continueProbe"
      @start="launchProbe"
      @update-filter="updateResultFilter"
      @update-order="updateResultOrder"
      @update-sort="updateResultSort"
    />

    <SourcesView
      v-else-if="selectedView === 'sources'"
      :accepted="summary.accepted"
      :invalid="summary.invalid"
      platform="mobile"
      :prepared-count="preparedSources.length"
      :preview-states="sourcePreviewStates"
      :request-states="sourceRequestStates"
      :sources="sources"
      :task-stage="task.stage"
      @add="addSource"
      @fetch-source="inspectSource($event, 'fetch')"
      @preview="inspectSource($event, 'preview')"
      @remove="removeSource"
    />

    <SettingsView
      v-else-if="selectedView === 'settings'"
      :loading="loading"
      :masked-token-hint="maskedTokenHint"
      platform="mobile"
      :save-blocked-by-masked-token="saveBlockedByMaskedToken"
      :settings="settings"
      :show-token="showToken"
      @save="persistConfig"
      @toggle-token="showToken = !showToken"
    />

    <DnsView
      v-else
      :dns-push-summary="dnsPushSummary"
      :dns-push-text="dnsPushText"
      :dns-records="dnsRecords"
      :is-loading-dns="isLoadingDns"
      :loading="loading"
      platform="mobile"
      @fetch="fetchDnsRecords"
      @push="pushToDns"
      @update:dnsPushText="dnsPushText = $event"
    />
  </MobileShell>

  <ToastStack :toasts="toasts" />
</template>
