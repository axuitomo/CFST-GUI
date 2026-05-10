<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import {
  WindowCenter,
  WindowGetSize,
  WindowIsMaximised,
  WindowMaximise,
  WindowSetSize,
  WindowUnfullscreen,
  WindowUnmaximise,
} from "../wailsjs/runtime/runtime";
import {
  backupConfigArchive,
  backupConfigToWebDAV,
  checkForUpdates,
  checkStorageHealth,
  deleteProfile,
  deleteSourceProfile,
  downloadAndInstallUpdate,
  deriveTaskStateFromProbeEvent,
  exportConfigArchive,
  exportResultsToGitHub,
  fetchDesktopSource,
  getTaskSnapshot,
  getAppInfo,
  importConfigArchive,
  isMaskedTokenValue,
  listenToProbeEvents,
  listDnsRecords,
  listTaskResults,
  loadColoDictionaryStatus,
  loadConfig,
  loadSchedulerStatus,
  normalizeConfigSnapshot,
  normalizeDnsRecords,
  normalizeSourceProfileStore,
  openReleasePage,
  openPath,
  previewDesktopSource,
  processColoDictionary,
  pushDnsRecords as pushDesktopDnsRecords,
  resumeProbe,
  restoreConfigArchive,
  restoreConfigFromWebDAV,
  saveConfig,
  saveCurrentProfile,
  saveSourceProfile,
  selectPath,
  setStorageDirectory,
  startProbe,
  stopProbe,
  switchProfile,
  switchSourceProfile,
  testGitHubExport,
  testWebDAV,
  updateColoDictionary,
  type ColoDictionaryStatus,
  type AppInfo,
  type ColoFilterMode,
  type ConfigSnapshot,
  type CSVEncoding,
  type DebugLogMode,
  type DebugLogVerbosity,
  type DesktopSourceConfig,
  type DownloadSpeedMetric,
  type DnsRecordSnapshot,
  type PathSelectionPayload,
  type ProbeEventEnvelope,
  type ProbeResult,
  type ProbeResultFilter,
  type ProbeResultIPFilter,
  type ProbeResultOrder,
  type ProbeResultSortBy,
  type ProbeStrategy,
  type ProfileStore,
  type SourceProfileStore,
  type SourcePreviewPayload,
  type SourceIPMode,
  type SourceKind,
  type SourceColoFilterPhase,
  type StorageStatus,
  type SchedulerStatus,
  type TaskSnapshot,
  type TaskTone,
  type TraceColoMode,
  type UpdateInfo,
} from "./lib/bridge";
import { detectSourceNameFromUrl, isDefaultSourceName } from "./lib/sourceNames";
import DesktopShell from "./components/layout/DesktopShell.vue";
import MobileShell from "./components/layout/MobileShell.vue";
import ToastStack from "./components/ui/ToastStack.vue";
import DashboardView from "./views/DashboardView.vue";
import DnsView from "./views/DnsView.vue";
import ResultsView from "./views/ResultsView.vue";
import SettingsView from "./views/SettingsView.vue";
import SourcesView from "./views/SourcesView.vue";

type ViewName = "dashboard" | "results" | "sources" | "settings" | "dns";
type ToastTone = "success" | "error" | "info";
type ViewportPresetId = "adaptive" | "phone390" | "tablet768" | "desktop1024" | "desktop1366" | "desktop1920" | "desktop2560";
type FixedViewportPresetId = Exclude<ViewportPresetId, "adaptive">;

interface WailsRuntimeWindow extends Window {
  runtime?: unknown;
}

interface HistoryEntry {
  debugLogPath?: string;
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
  exportFileNameTemplate: string;
  githubBranch: string;
  githubCommitMessageTemplate: string;
  githubExportEnabled: boolean;
  githubLastExportAt: string;
  githubOwner: string;
  githubPathTemplate: string;
  githubRepo: string;
  githubToken: string;
  exportCSVEncoding: CSVEncoding;
  exportOverwrite: string;
  exportTargetDir: string;
  exportTargetUri: string;
  maxHttpLatencyMs: number | null;
  maxTcpLatencyMs: number | null;
  maxLossRate: number;
  minDownloadMbps: number;
  minDelayMs: number;
  probeDebug: boolean;
  probeDebugCaptureAddress: string;
  probeDebugCaptureEnabled: boolean;
  probeDebugLogFormat: string;
  probeDebugLogMode: DebugLogMode;
  probeDebugLogVerbosity: DebugLogVerbosity;
  probeDisableDownload: boolean;
  probeConcurrencyStage1: number;
  probeConcurrencyStage2: number;
  probeConcurrencyStage3: number;
  probeCooldownFailures: number;
  probeCooldownMs: number;
  probeDownloadBufferKB: number;
  probeDownloadCount: number;
  probeDownloadGetConcurrency: number;
  probeDownloadHTTPProtocol: "auto" | "h1" | "h2" | "h3";
  probeDownloadSpeedMetric: DownloadSpeedMetric;
  probeDownloadSpeedSampleIntervalMs: number;
  probeDownloadTimeSeconds: number;
  probeDownloadWarmupSeconds: number;
  probeEventThrottleMs: number;
  probeHostHeader: string;
  probeHttping: boolean;
  probeHttpingCfColo: string;
  probeHttpingCfColoMode: ColoFilterMode;
  probeHttpingStatusCode: number;
  probePingTimes: number;
  probePrintNum: number;
  probeRetryBackoffMs: number;
  probeRetryMaxAttempts: number;
  probeRequestHeaders: string;
  probeSNI: string;
  probeSourceColoFilterPhase: SourceColoFilterPhase;
  probeStageLimitStage3: number;
  probeStrategy: ProbeStrategy;
  probeTcpPort: number;
  probeTimeoutStage1Ms: number;
  probeTimeoutStage2Ms: number;
  probeTimeoutStage3Ms: number;
  probeTraceColoMode: TraceColoMode;
  probeTraceURL: string;
  probeURL: string;
  probeUserAgent: string;
  proxied: boolean;
  recordName: string;
  schedulerAutoDnsPush: boolean;
  schedulerAutoGithubExport: boolean;
  schedulerDailyTimes: string;
  schedulerEnabled: boolean;
  schedulerIntervalMinutes: number;
  schedulerSkipIfActive: boolean;
  sourceAutoDetectName: boolean;
  ttl: number;
  webdavEnabled: boolean;
  webdavLastBackupAt: string;
  webdavLastRestoreAt: string;
  webdavPassword: string;
  webdavRemotePath: string;
  webdavServerURL: string;
  webdavTimeoutSeconds: number;
  webdavUsername: string;
  zoneId: string;
}

const DEFAULT_PROBE_USER_AGENT =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0";
const DEFAULT_FILE_TEST_URL = "https://speed.cloudflare.com/__down?bytes=10000000";
const DEFAULT_DEBUG_LOG_FORMAT = "{ts} [{level}] {event} task={task_id} stage={stage} {message}";
const DEFAULT_SOURCE_IP_LIMIT = 500;
const DEFAULT_CLOUDFLARE_TTL = 300;
const MIN_PROBE_PING_TIMES = 2;
const DEFAULT_MAX_LOSS_RATE = 0.15;
const MAX_LOSS_RATE = 1;
const DEFAULT_HTTPING_STATUS_CODE = 0;
const ACTIVE_PROBE_MESSAGE = "当前已有探测任务运行或暂停，请完成后再启动新任务。";

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

interface DownloadSpeedState {
  active: boolean;
  averageSpeedMbS: number | null;
  bytesRead: number;
  colo: string;
  currentSpeedMbS: number | null;
  elapsedMs: number;
  ip: string;
}

interface ToastEntry {
  id: number;
  message: string;
  tone: ToastTone;
}

interface AdaptiveViewportPreset {
  description: string;
  id: "adaptive";
  label: string;
  mode: "adaptive";
  shell: "desktop";
}

interface FixedViewportPreset {
  description: string;
  height: number;
  id: FixedViewportPresetId;
  label: string;
  mode: "fixed";
  shell: "mobile" | "desktop";
  width: number;
}

type ViewportPreset = AdaptiveViewportPreset | FixedViewportPreset;

interface ViewportSize {
  cssHeight: number;
  cssWidth: number;
  height: number;
  updatedAt: string;
  width: number;
}

const VIEWPORT_PRESETS: ViewportPreset[] = [
  { description: "最大化到当前屏幕可用区域，作为默认 UI 尺寸。", id: "adaptive", label: "自适应", mode: "adaptive", shell: "desktop" },
  { description: "小屏移动端，验证底部导航和触控安全区。", height: 844, id: "phone390", label: "390x844", mode: "fixed", shell: "mobile", width: 390 },
  { description: "平板竖屏，验证移动壳表单和卡片布局。", height: 1024, id: "tablet768", label: "768x1024", mode: "fixed", shell: "mobile", width: 768 },
  { description: "桌面断点起点，验证侧栏和桌面壳切换。", height: 768, id: "desktop1024", label: "1024x768", mode: "fixed", shell: "desktop", width: 1024 },
  { description: "常见 Windows 工作窗口，验证首屏信息密度。", height: 768, id: "desktop1366", label: "1366x768", mode: "fixed", shell: "desktop", width: 1366 },
  { description: "全高清桌面，验证阅读宽度和表格可读性。", height: 1080, id: "desktop1920", label: "1920x1080", mode: "fixed", shell: "desktop", width: 1920 },
  { description: "大屏桌面，验证最大宽度约束和留白。", height: 1440, id: "desktop2560", label: "2560x1440", mode: "fixed", shell: "desktop", width: 2560 },
];

const views: Array<{ id: ViewName; title: string; copy: string; shortLabel: string }> = [
  { id: "dashboard", title: "任务看板", copy: "运行状态、进度条与测试进程", shortLabel: "看板" },
  { id: "results", title: "当前结果", copy: "本次测速结果、排序与导出位置", shortLabel: "结果" },
  { id: "sources", title: "输入源", copy: "全局保存的来源、状态与 IP 模式", shortLabel: "来源" },
  { id: "settings", title: "系统配置", copy: "Cloudflare、导出与探测参数", shortLabel: "配置" },
  { id: "dns", title: "DNS 推送", copy: "读取当前记录并执行覆盖推送", shortLabel: "云推" },
];

const routeTitles: Record<ViewName, string> = {
  dashboard: "任务看板",
  dns: "DNS 记录推送",
  results: "当前测速结果",
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
const downloadSpeedState = reactive<DownloadSpeedState>({
  active: false,
  averageSpeedMbS: null,
  bytesRead: 0,
  colo: "",
  currentSpeedMbS: null,
  elapsedMs: 0,
  ip: "",
});
const resultFilter = ref<ProbeResultFilter>("all");
const resultIpFilter = ref<ProbeResultIPFilter>("all");
const resultOrder = ref<ProbeResultOrder>("asc");
const resultRows = ref<ProbeResult[]>([]);
const resultSortBy = ref<ProbeResultSortBy>("address");
const resultsLoading = ref(false);
const githubExporting = ref(false);
const githubTesting = ref(false);
const showToken = ref(false);
const viewportAdaptiveActive = ref(false);
const viewportSwitching = ref(false);
const sourceSeed = ref(0);
const sourcePreviewStates = reactive<Record<string, SourcePreviewState>>({});
const sourceRequestStates = reactive<Record<string, string>>({});
const coloDictionaryStatus = ref<ColoDictionaryStatus | null>(null);
const coloDictionaryProcessing = ref(false);
const coloDictionaryUpdating = ref(false);
const taskSnapshot = ref<TaskSnapshot | null>(null);
const schedulerStatus = ref<SchedulerStatus | null>(null);
const toasts = ref<ToastEntry[]>([]);
const storageStatus = ref<StorageStatus | null>(null);
const profiles = ref<ProfileStore>({
  active_profile_id: "",
  items: [],
  schema_version: "",
  updated_at: "",
});
const sourceProfiles = ref<SourceProfileStore>({
  active_profile_id: "",
  items: [],
  schema_version: "",
  updated_at: "",
});
const storageSetupVisible = ref(false);
const storageSetupDismissed = ref(false);
const appInfo = ref<AppInfo>({
  current_version: "1.0",
  install_mode: "",
  platform: "",
  release_url: "",
});
const updateState = reactive({
  assetName: "",
  checkedAt: "",
  downloadPath: "",
  installMode: "",
  installing: false,
  latestVersion: "",
  message: "尚未检查更新。",
  releaseUrl: "",
  status: "idle" as "idle" | "checking" | "available" | "latest" | "installing" | "ready" | "failed",
  updateAvailable: false,
});
const viewportSize = reactive<ViewportSize>({
  cssHeight: 0,
  cssWidth: 0,
  height: 0,
  updatedAt: "",
  width: 0,
});
let viewportResizeTimer: number | undefined;

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
  exportFileNameTemplate: "",
  githubBranch: "main",
  githubCommitMessageTemplate: "CFST results {date} {time}",
  githubExportEnabled: false,
  githubLastExportAt: "",
  githubOwner: "axuitomo",
  githubPathTemplate: "cfst-results/{date}/{time}-{task_id}.csv",
  githubRepo: "CFST-GUI",
  githubToken: "",
  exportCSVEncoding: "utf-8",
  exportOverwrite: "replace_on_start",
  exportTargetDir: "",
  exportTargetUri: "",
  maxHttpLatencyMs: null,
  maxTcpLatencyMs: null,
  maxLossRate: DEFAULT_MAX_LOSS_RATE,
  minDownloadMbps: 0,
  minDelayMs: 0,
  probeDebug: false,
  probeDebugCaptureAddress: "",
  probeDebugCaptureEnabled: false,
  probeDebugLogFormat: "",
  probeDebugLogMode: "structured",
  probeDebugLogVerbosity: "detailed",
  probeDisableDownload: true,
  probeConcurrencyStage1: 200,
  probeConcurrencyStage2: 6,
  probeConcurrencyStage3: 1,
  probeCooldownFailures: 3,
  probeCooldownMs: 250,
  probeDownloadBufferKB: 256,
  probeDownloadCount: 10,
  probeDownloadGetConcurrency: 4,
  probeDownloadHTTPProtocol: "auto",
  probeDownloadSpeedMetric: "average",
  probeDownloadSpeedSampleIntervalMs: 500,
  probeDownloadTimeSeconds: 10,
  probeDownloadWarmupSeconds: 5,
  probeEventThrottleMs: 100,
  probeHostHeader: "",
  probeHttping: false,
  probeHttpingCfColo: "",
  probeHttpingCfColoMode: "allow",
  probeHttpingStatusCode: DEFAULT_HTTPING_STATUS_CODE,
  probePingTimes: 4,
  probePrintNum: 0,
  probeRetryBackoffMs: 0,
  probeRetryMaxAttempts: 0,
  probeRequestHeaders: "",
  probeSNI: "",
  probeSourceColoFilterPhase: "precheck",
  probeStageLimitStage3: 10,
  probeStrategy: "fast",
  probeTcpPort: 443,
  probeTimeoutStage1Ms: 1000,
  probeTimeoutStage2Ms: 1000,
  probeTimeoutStage3Ms: 10000,
  probeTraceColoMode: "standard",
  probeTraceURL: "",
  probeURL: DEFAULT_FILE_TEST_URL,
  probeUserAgent: DEFAULT_PROBE_USER_AGENT,
  proxied: false,
  recordName: "",
  schedulerAutoDnsPush: true,
  schedulerAutoGithubExport: true,
  schedulerDailyTimes: "",
  schedulerEnabled: false,
  schedulerIntervalMinutes: 0,
  schedulerSkipIfActive: true,
  sourceAutoDetectName: true,
  ttl: DEFAULT_CLOUDFLARE_TTL,
  webdavEnabled: false,
  webdavLastBackupAt: "",
  webdavLastRestoreAt: "",
  webdavPassword: "",
  webdavRemotePath: "cfst-gui-config.zip",
  webdavServerURL: "",
  webdavTimeoutSeconds: 30,
  webdavUsername: "",
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
    colo_filter: source.colo_filter.trim(),
    colo_filter_mode: source.colo_filter_mode === "deny" ? "deny" : "allow",
    content: source.content.trim(),
    enabled: source.enabled,
    id: source.id,
    ip_limit: Math.max(1, Math.floor(source.ip_limit || 1)),
    ip_mode: source.ip_mode,
    kind: source.kind,
    last_fetched_at: source.last_fetched_at,
    last_fetched_count: source.last_fetched_count,
    name: sourceNameForPayload(source, index),
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
const canResumeTask = computed(() => Boolean(task.taskId) && (status.tone === "cooling" || task.stage === "cooling"));
const lastHistoryEntry = computed(() => exportHistory.value[0] || null);
const saveBlockedByMaskedToken = computed(() => Boolean(maskedTokenHint.value) && !settings.apiToken.trim());
const viewportRuntimeSupported = computed(() => isViewportRuntimeSupported());
const resultFilterOptions: Array<{ label: string; value: ProbeResultFilter }> = [
  { label: "全部", value: "all" },
  { label: "已导出", value: "exported" },
  { label: "待处理", value: "pending" },
  { label: "失败", value: "failed" },
];
const resultIpFilterOptions: Array<{ label: string; value: ProbeResultIPFilter }> = [
  { label: "全部 IP", value: "all" },
  { label: "仅 IPv4", value: "ipv4" },
  { label: "仅 IPv6", value: "ipv6" },
];
const resultSortOptions: Array<{ label: string; value: ProbeResultSortBy }> = [
  { label: "IP 地址", value: "address" },
  { label: "阶段状态", value: "stage" },
  { label: "TCP 延迟", value: "tcp" },
  { label: "追踪延迟", value: "trace" },
  { label: "平均速率", value: "download" },
  { label: "最高速率", value: "max_download" },
  { label: "导出状态", value: "export_status" },
];

function createSourceDraft(kind: SourceKind = "url"): SourceDraft {
  sourceSeed.value += 1;
  return {
    content: "",
    colo_filter: "",
    colo_filter_mode: "allow",
    enabled: true,
    id: `source-${Date.now()}-${sourceSeed.value}`,
    ip_limit: DEFAULT_SOURCE_IP_LIMIT,
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

function detectedSourceName(source: Pick<SourceDraft, "kind" | "url">) {
  if (source.kind !== "url") {
    return "";
  }
  return detectSourceNameFromUrl(source.url);
}

function shouldAutoFillSourceName(source: Pick<SourceDraft, "kind" | "name" | "url">) {
  return settings.sourceAutoDetectName && source.kind === "url" && isDefaultSourceName(source.name) && Boolean(source.url.trim());
}

function sourceNameForPayload(source: SourceDraft, index: number) {
  if (shouldAutoFillSourceName(source)) {
    const detected = detectedSourceName(source);
    if (detected) {
      return detected;
    }
  }
  return source.name.trim() || `输入源 ${index + 1}`;
}

function applyDetectedSourceName(sourceId: string) {
  const source = sources.value.find((entry) => entry.id === sourceId);
  if (!source || !shouldAutoFillSourceName(source)) {
    return;
  }
  const detected = detectedSourceName(source);
  if (detected) {
    source.name = detected;
  }
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

function asBoolean(value: unknown, fallback = false) {
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

function isViewportRuntimeSupported() {
  if (typeof window === "undefined") {
    return false;
  }
  const runtimeWindow = window as WailsRuntimeWindow;
  return Boolean(runtimeWindow.runtime && runtimeWindow.go?.main?.App);
}

function applyViewportSize(runtimeSize?: { h: number; w: number }) {
  const cssWidth = typeof window === "undefined" ? 0 : Math.round(window.innerWidth);
  const cssHeight = typeof window === "undefined" ? 0 : Math.round(window.innerHeight);
  viewportSize.cssWidth = cssWidth;
  viewportSize.cssHeight = cssHeight;
  viewportSize.width = runtimeSize ? Math.round(runtimeSize.w) : cssWidth;
  viewportSize.height = runtimeSize ? Math.round(runtimeSize.h) : cssHeight;
  viewportSize.updatedAt = new Date().toLocaleTimeString();
}

async function refreshViewportSize() {
  if (!isViewportRuntimeSupported()) {
    applyViewportSize();
    viewportAdaptiveActive.value = true;
    return;
  }

  try {
    const runtimeSize = await WindowGetSize();
    applyViewportSize(runtimeSize);
    viewportAdaptiveActive.value = await WindowIsMaximised();
  } catch (error) {
    appendLog("runtime.window_size.failed", error instanceof Error ? error.message : String(error));
    viewportAdaptiveActive.value = false;
    applyViewportSize();
  }
}

async function maximizeViewportToAdaptive() {
  WindowUnfullscreen();
  WindowMaximise();
  await new Promise((resolve) => window.setTimeout(resolve, 160));
  await refreshViewportSize();
}

async function ensureAdaptiveViewportOnStartup() {
  if (!isViewportRuntimeSupported()) {
    await refreshViewportSize();
    return;
  }

  try {
    const maximised = await WindowIsMaximised();
    if (maximised) {
      await refreshViewportSize();
      return;
    }
    await maximizeViewportToAdaptive();
  } catch (error) {
    appendLog("runtime.viewport_adaptive_startup.failed", error instanceof Error ? error.message : String(error));
    await refreshViewportSize();
  }
}

function scheduleViewportSizeRefresh() {
  applyViewportSize();
  if (viewportResizeTimer !== undefined) {
    window.clearTimeout(viewportResizeTimer);
  }
  viewportResizeTimer = window.setTimeout(() => {
    void refreshViewportSize();
  }, 120);
}

async function applyViewportPreset(presetId: string) {
  const preset = VIEWPORT_PRESETS.find((entry) => entry.id === presetId);
  if (!preset) {
    showToast("未知的验收尺寸", "error");
    return;
  }

  if (!isViewportRuntimeSupported()) {
    await refreshViewportSize();
    if (preset.mode === "adaptive") {
      showToast("已刷新浏览器自适应尺寸", "success");
      return;
    }
    showToast("固定验收尺寸仅 Wails 桌面支持", "error");
    return;
  }

  viewportSwitching.value = true;
  try {
    if (preset.mode === "adaptive") {
      await maximizeViewportToAdaptive();
    } else {
      WindowUnfullscreen();
      WindowUnmaximise();
      WindowSetSize(preset.width, preset.height);
      WindowCenter();
      await new Promise((resolve) => window.setTimeout(resolve, 160));
      await refreshViewportSize();
    }
    showToast(`已切换到 ${preset.label}`, "success");
  } catch (error) {
    const message = error instanceof Error ? error.message : "切换验收尺寸失败";
    appendLog("runtime.viewport_preset.failed", { message, preset });
    showToast(message, "error");
  } finally {
    viewportSwitching.value = false;
  }
}

function applyStorageStatus(value: unknown) {
  const source = asRecord(value);
  if (Object.keys(source).length === 0) {
    return;
  }
  storageStatus.value = {
    bootstrap_path: asString(source.bootstrap_path || source.bootstrapPath),
    current_dir: asString(source.current_dir || source.currentDir),
    default_dir: asString(source.default_dir || source.defaultDir),
    display_name: asString(source.display_name || source.displayName),
    health: asRecord(source.health) as unknown as StorageStatus["health"],
    portable_mode: Boolean(source.portable_mode || source.portableMode),
    setup_completed: Boolean(source.setup_completed || source.setupCompleted),
    setup_required: Boolean(source.setup_required || source.setupRequired),
    storage_uri: asString(source.storage_uri || source.storageUri),
    writable: source.writable !== false,
  };
  storageSetupVisible.value = Boolean(storageStatus.value.setup_required) && !storageSetupDismissed.value;
}

function applyProfileStore(value: unknown) {
  const source = asRecord(value);
  profiles.value = {
    active_profile_id: asString(source.active_profile_id || source.activeProfileId),
    items: Array.isArray(source.items)
      ? source.items.map((entry) => {
          const item = asRecord(entry);
          return {
            config_snapshot: normalizeConfigSnapshot(item.config_snapshot || item.configSnapshot || {}),
            created_at: asString(item.created_at || item.createdAt),
            id: asString(item.id),
            name: asString(item.name) || "未命名档案",
            updated_at: asString(item.updated_at || item.updatedAt),
          };
        })
      : [],
    schema_version: asString(source.schema_version || source.schemaVersion),
    updated_at: asString(source.updated_at || source.updatedAt),
  };
}

function applySourceProfileStore(value: unknown) {
  sourceProfiles.value = normalizeSourceProfileStore(value);
}

function applyAppInfo(value: unknown) {
  const source = asRecord(value);
  appInfo.value = {
    current_version: asString(source.current_version || source.currentVersion || "1.0"),
    install_mode: asString(source.install_mode || source.installMode),
    platform: asString(source.platform),
    release_url: asString(source.release_url || source.releaseUrl),
  };
  if (!updateState.releaseUrl) {
    updateState.releaseUrl = appInfo.value.release_url;
  }
}

function applyUpdateInfo(value: unknown) {
  const source = asRecord(value) as Partial<UpdateInfo> & Record<string, unknown>;
  updateState.assetName = asString(source.asset_name || source.assetName);
  updateState.checkedAt = new Date().toISOString();
  updateState.installMode = asString(source.install_mode || source.installMode);
  updateState.latestVersion = asString(source.latest_version || source.latestVersion);
  updateState.releaseUrl = asString(source.release_url || source.releaseUrl || appInfo.value.release_url);
  updateState.updateAvailable = source.update_available === true || source.updateAvailable === true;
}

function asNumber(value: unknown, fallback = 0) {
  const parsed = Number.parseFloat(String(value ?? ""));
  return Number.isFinite(parsed) ? parsed : fallback;
}

function positiveCount(value: unknown, fallback: number, max?: number) {
  const parsed = asCount(value, fallback);
  const normalized = parsed > 0 ? parsed : fallback;
  return typeof max === "number" ? Math.min(normalized, max) : normalized;
}

function normalizeCloudflareTTL(value: unknown) {
  const ttl = asCount(value, DEFAULT_CLOUDFLARE_TTL);
  return [60, 300, 600].includes(ttl) ? ttl : DEFAULT_CLOUDFLARE_TTL;
}

function minimumCount(value: unknown, fallback: number, min: number, max?: number) {
  return Math.max(min, positiveCount(value, fallback, max));
}

function boundedCount(value: unknown, fallback: number, min: number, max: number) {
  return Math.max(min, Math.min(max, asCount(value, fallback)));
}

function normalizeDownloadHTTPProtocol(value: unknown): SettingsForm["probeDownloadHTTPProtocol"] {
  const normalized = asString(value).toLowerCase();
  if (normalized === "h1" || normalized === "h1.1" || normalized === "http/1.1") {
    return "h1";
  }
  if (normalized === "h2" || normalized === "http/2") {
    return "h2";
  }
  if (normalized === "h3" || normalized === "http/3") {
    return "h3";
  }
  return "auto";
}

function nonNegativeCount(value: unknown, fallback = 0) {
  const parsed = asCount(value, fallback);
  return parsed >= 0 ? parsed : fallback;
}

function nonNegativeNumber(value: unknown, fallback = 0) {
  const parsed = asNumber(value, fallback);
  return parsed >= 0 ? parsed : fallback;
}

function clampedNumber(value: unknown, fallback: number, min: number, max: number) {
  const parsed = asNumber(value, fallback);
  return Math.max(min, Math.min(max, parsed));
}

function asNullableNumber(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return null;
  }

  const parsed = Number.parseFloat(String(value));
  return Number.isFinite(parsed) ? parsed : null;
}

function stageTitle(stage: string) {
  const labels: Record<string, string> = {
    stage0_pool: "IP池",
    stage1_tcp: "TCP测延迟",
    stage2_head: "追踪探测",
    stage2_trace: "追踪探测",
    stage3_get: "文件测速",
  };

  return labels[stage] || stage || "探测";
}

function optionalNumberForPayload(value: number | null) {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : null;
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

function notifyActiveProbeBlocked(title: string) {
  pushActivity(title, ACTIVE_PROBE_MESSAGE);
  showToast("已有任务运行中", "error");
  selectedView.value = "dashboard";
}

function clearProcessTrace() {
  processTrace.value = [];
}

function resetDownloadSpeedState(clearIdentity = true) {
  downloadSpeedState.active = false;
  downloadSpeedState.averageSpeedMbS = null;
  downloadSpeedState.bytesRead = 0;
  downloadSpeedState.currentSpeedMbS = null;
  downloadSpeedState.elapsedMs = 0;
  if (clearIdentity) {
    downloadSpeedState.colo = "";
    downloadSpeedState.ip = "";
  }
}

function updateDownloadSpeedTrace(ip: string, colo: string, ts: string) {
  const detail = `${ip || "当前 IP"}${colo ? `(${colo})` : ""} 正在测速中`;
  const current = [...processTrace.value];
  const existingIndex = current.findIndex((entry) => entry.stage === "stage3_get" && entry.title === "文件测速实时速度");
  const nextEntry: ProcessTraceEntry = {
    detail,
    id: existingIndex >= 0 ? current[existingIndex].id : processTraceId,
    stage: "stage3_get",
    title: "文件测速实时速度",
    tone: "running",
    ts,
  };

  if (existingIndex >= 0) {
    current.splice(existingIndex, 1);
    processTrace.value = [nextEntry, ...current].slice(0, 80);
    return;
  }

  processTraceId += 1;
  processTrace.value = [nextEntry, ...current].slice(0, 80);
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
  settings.exportFileNameTemplate = normalized.export.file_name_template || "";
  settings.githubBranch = normalized.export.github.branch || "main";
  settings.githubCommitMessageTemplate = normalized.export.github.commit_message_template || "CFST results {date} {time}";
  settings.githubExportEnabled = Boolean(normalized.export.github.enabled);
  settings.githubLastExportAt = normalized.export.github.last_export_at || "";
  settings.githubOwner = normalized.export.github.owner || "axuitomo";
  settings.githubPathTemplate = normalized.export.github.path_template || "cfst-results/{date}/{time}-{task_id}.csv";
  settings.githubRepo = normalized.export.github.repo || "CFST-GUI";
  settings.githubToken = normalized.export.github.token || "";
  settings.exportCSVEncoding = normalized.export.csv_encoding || "utf-8";
  settings.exportOverwrite = normalized.export.overwrite || "replace_on_start";
  settings.exportTargetDir = normalized.export.target_dir || "";
  settings.exportTargetUri = normalized.export.target_uri || "";
  settings.maxHttpLatencyMs = null;
  settings.maxTcpLatencyMs = asNullableNumber(normalized.probe.thresholds.max_tcp_latency_ms);
  settings.maxLossRate = asNumber(normalized.probe.max_loss_rate, DEFAULT_MAX_LOSS_RATE);
  settings.minDownloadMbps = asNumber(normalized.probe.thresholds.min_download_mbps, 0);
  settings.minDelayMs = asCount(normalized.probe.min_delay_ms, 0);
  settings.probeDebug = Boolean(normalized.probe.debug);
  settings.probeDebugCaptureAddress = normalized.probe.debug_capture_address || "";
  settings.probeDebugCaptureEnabled = Boolean(normalized.probe.debug_capture_enabled);
  settings.probeDebugLogFormat = normalized.probe.debug_log_format || "";
  settings.probeDebugLogMode = normalized.probe.debug_log_mode || "structured";
  settings.probeDebugLogVerbosity = normalized.probe.debug_log_verbosity || "detailed";
  settings.probeDisableDownload = normalized.probe.strategy === "fast";
  settings.probeConcurrencyStage1 = normalized.probe.concurrency.stage1;
  settings.probeConcurrencyStage2 = normalized.probe.concurrency.stage2;
  settings.probeConcurrencyStage3 = 1;
  settings.probeCooldownFailures = normalized.probe.cooldown_policy.consecutive_failures;
  settings.probeCooldownMs = normalized.probe.cooldown_policy.cooldown_ms;
  settings.probeDownloadBufferKB = normalized.probe.download_buffer_kb;
  settings.probeDownloadCount = normalized.probe.download_count;
  settings.probeDownloadGetConcurrency = normalized.probe.download_get_concurrency;
  settings.probeDownloadHTTPProtocol = normalized.probe.download_http_protocol;
  settings.probeDownloadSpeedMetric = normalized.probe.download_speed_metric;
  settings.probeDownloadSpeedSampleIntervalMs = normalized.probe.download_speed_sample_interval_ms;
  settings.probeDownloadTimeSeconds = normalized.probe.download_time_seconds;
  settings.probeDownloadWarmupSeconds = normalized.probe.download_warmup_seconds;
  settings.probeEventThrottleMs = normalized.probe.event_throttle_ms;
  settings.probeHostHeader = normalized.probe.host_header || "";
  settings.probeHttping = Boolean(normalized.probe.httping);
  settings.probeHttpingCfColo = normalized.probe.httping_cf_colo || "";
  settings.probeHttpingCfColoMode = normalized.probe.httping_cf_colo_mode || "allow";
  settings.probeHttpingStatusCode = normalized.probe.httping_status_code;
  settings.probePingTimes = Math.max(MIN_PROBE_PING_TIMES, normalized.probe.ping_times);
  settings.probePrintNum = normalized.probe.print_num;
  settings.probeRetryBackoffMs = normalized.probe.retry_policy.backoff_ms;
  settings.probeRetryMaxAttempts = normalized.probe.retry_policy.max_attempts;
  settings.probeRequestHeaders = normalized.probe.request_headers || "";
  settings.probeSNI = normalized.probe.sni || "";
  settings.probeSourceColoFilterPhase = normalized.probe.source_colo_filter_phase;
  settings.probeStageLimitStage3 = normalized.probe.stage_limits.stage3;
  settings.probeStrategy = normalized.probe.strategy;
  settings.probeTcpPort = normalized.probe.tcp_port;
  settings.probeTimeoutStage1Ms = normalized.probe.timeouts.stage1_ms;
  settings.probeTimeoutStage2Ms = normalized.probe.timeouts.stage2_ms;
  settings.probeTimeoutStage3Ms = normalized.probe.timeouts.stage3_ms;
  settings.probeTraceColoMode = normalized.probe.trace_colo_mode;
  settings.probeTraceURL = normalized.probe.trace_url || "";
  settings.probeURL = normalized.probe.url || DEFAULT_FILE_TEST_URL;
  settings.probeUserAgent = normalized.probe.user_agent || DEFAULT_PROBE_USER_AGENT;
  settings.proxied = Boolean(normalized.cloudflare.proxied);
  settings.recordName = normalized.cloudflare.record_name || "";
  settings.schedulerAutoDnsPush = normalized.scheduler.auto_dns_push;
  settings.schedulerAutoGithubExport = normalized.scheduler.auto_github_export;
  settings.schedulerDailyTimes = normalized.scheduler.daily_times.join("\n");
  settings.schedulerEnabled = normalized.scheduler.enabled;
  settings.schedulerIntervalMinutes = normalized.scheduler.interval_minutes;
  settings.schedulerSkipIfActive = normalized.scheduler.skip_if_active;
  settings.sourceAutoDetectName = normalized.ui.auto_detect_source_name;
  settings.ttl = normalizeCloudflareTTL(normalized.cloudflare.ttl);
  settings.webdavEnabled = normalized.backup.webdav.enabled;
  settings.webdavLastBackupAt = normalized.backup.webdav.last_backup_at;
  settings.webdavLastRestoreAt = normalized.backup.webdav.last_restore_at;
  settings.webdavPassword = normalized.backup.webdav.password;
  settings.webdavRemotePath = normalized.backup.webdav.remote_path || "cfst-gui-config.zip";
  settings.webdavServerURL = normalized.backup.webdav.server_url;
  settings.webdavTimeoutSeconds = normalized.backup.webdav.timeout_seconds || 30;
  settings.webdavUsername = normalized.backup.webdav.username;
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
      ttl: normalizeCloudflareTTL(settings.ttl),
      zone_id: settings.zoneId.trim(),
    },
    backup: {
      webdav: {
        enabled: settings.webdavEnabled,
        last_backup_at: settings.webdavLastBackupAt.trim(),
        last_restore_at: settings.webdavLastRestoreAt.trim(),
        password: settings.webdavPassword,
        remote_path: settings.webdavRemotePath.trim() || "cfst-gui-config.zip",
        server_url: settings.webdavServerURL.trim(),
        timeout_seconds: positiveCount(settings.webdavTimeoutSeconds, 30),
        username: settings.webdavUsername.trim(),
      },
    },
    export: {
      csv_encoding: settings.exportCSVEncoding === "utf-8-bom" ? "utf-8-bom" : "utf-8",
      ...(settings.exportFileName.trim() ? { file_name: settings.exportFileName.trim() } : {}),
      ...(settings.exportFileNameTemplate.trim() ? { file_name_template: settings.exportFileNameTemplate.trim() } : {}),
      github: {
        branch: settings.githubBranch.trim() || "main",
        commit_message_template: settings.githubCommitMessageTemplate.trim() || "CFST results {date} {time}",
        enabled: settings.githubExportEnabled,
        last_export_at: settings.githubLastExportAt.trim(),
        owner: settings.githubOwner.trim(),
        path_template: settings.githubPathTemplate.trim() || "cfst-results/{date}/{time}-{task_id}.csv",
        repo: settings.githubRepo.trim(),
        token: settings.githubToken.trim(),
      },
      ...(settings.exportOverwrite.trim() ? { overwrite: settings.exportOverwrite.trim() } : {}),
      ...(settings.exportTargetDir.trim() ? { target_dir: settings.exportTargetDir.trim() } : {}),
      ...(settings.exportTargetUri.trim() ? { target_uri: settings.exportTargetUri.trim() } : {}),
    },
    probe: {
      concurrency: {
        stage1: positiveCount(settings.probeConcurrencyStage1, 200, 1000),
        stage2: Math.max(1, Math.min(30, settings.probeConcurrencyStage2)),
        stage3: 1,
      },
      cooldown_policy: {
        consecutive_failures: nonNegativeCount(settings.probeCooldownFailures, 3),
        cooldown_ms: nonNegativeCount(settings.probeCooldownMs, 250),
      },
      debug: settings.probeDebug,
      debug_capture_address: settings.probeDebugCaptureAddress.trim(),
      debug_capture_enabled: settings.probeDebugCaptureEnabled,
      debug_log_format:
        settings.probeDebugLogMode === "freeform" ? settings.probeDebugLogFormat.trim() || DEFAULT_DEBUG_LOG_FORMAT : "",
      debug_log_mode: settings.probeDebugLogMode === "freeform" ? "freeform" : "structured",
      debug_log_verbosity: settings.probeDebugLogVerbosity === "simple" ? "simple" : "detailed",
      disable_download: normalizedStrategy === "fast",
      download_buffer_kb: boundedCount(settings.probeDownloadBufferKB, 256, 64, 4096),
      download_get_concurrency: boundedCount(settings.probeDownloadGetConcurrency, 4, 1, 32),
      download_http_protocol: normalizeDownloadHTTPProtocol(settings.probeDownloadHTTPProtocol),
      download_speed_metric: settings.probeDownloadSpeedMetric === "max" ? "max" : "average",
      download_speed_sample_interval_ms: positiveCount(settings.probeDownloadSpeedSampleIntervalMs, 500),
      download_time_seconds: positiveCount(settings.probeDownloadTimeSeconds, 10),
      download_warmup_seconds: nonNegativeCount(settings.probeDownloadWarmupSeconds, 5),
      event_throttle_ms: positiveCount(settings.probeEventThrottleMs, 100),
      host_header: settings.probeHostHeader.trim(),
      httping: false,
      httping_cf_colo: settings.probeHttpingCfColo.trim(),
      httping_cf_colo_mode: settings.probeHttpingCfColoMode === "deny" ? "deny" : "allow",
      httping_status_code:
        settings.probeHttpingStatusCode === 0 ||
        (settings.probeHttpingStatusCode >= 100 && settings.probeHttpingStatusCode <= 599)
          ? settings.probeHttpingStatusCode
          : DEFAULT_HTTPING_STATUS_CODE,
      max_loss_rate: clampedNumber(settings.maxLossRate, DEFAULT_MAX_LOSS_RATE, 0, MAX_LOSS_RATE),
      min_delay_ms: nonNegativeCount(settings.minDelayMs, 0),
      ping_times: minimumCount(settings.probePingTimes, 4, MIN_PROBE_PING_TIMES),
      print_num: nonNegativeCount(settings.probePrintNum, 0),
      retry_policy: {
        backoff_ms: nonNegativeCount(settings.probeRetryBackoffMs, 0),
        max_attempts: nonNegativeCount(settings.probeRetryMaxAttempts, 0),
      },
      request_headers: settings.probeRequestHeaders.trim(),
      skip_first_latency_sample: true,
      source_colo_filter_phase: settings.probeSourceColoFilterPhase,
      stage_limits: {
        stage3: positiveCount(settings.probeStageLimitStage3, 10),
      },
      strategy: normalizedStrategy,
      sni: settings.probeSNI.trim(),
      tcp_port: positiveCount(settings.probeTcpPort, 443, 65535),
      test_all: false,
      thresholds: {
        max_http_latency_ms: null,
        max_tcp_latency_ms: optionalNumberForPayload(settings.maxTcpLatencyMs),
        min_download_mbps: nonNegativeNumber(settings.minDownloadMbps, 0),
      },
      timeouts: {
        stage1_ms: positiveCount(settings.probeTimeoutStage1Ms, 1000),
        stage2_ms: positiveCount(settings.probeTimeoutStage2Ms, 1000),
        stage3_ms: positiveCount(settings.probeDownloadTimeSeconds, 10) * 1000,
      },
      trace_colo_mode: settings.probeTraceColoMode,
      trace_url: settings.probeTraceURL.trim(),
      url: settings.probeURL.trim(),
      user_agent: settings.probeUserAgent.trim() || DEFAULT_PROBE_USER_AGENT,
    },
    ui: {
      auto_detect_source_name: settings.sourceAutoDetectName,
    },
    scheduler: {
      auto_dns_push: settings.schedulerAutoDnsPush,
      auto_github_export: settings.schedulerAutoGithubExport,
      daily_times: settings.schedulerDailyTimes
        .split(/[,\s;]+/)
        .map((entry) => entry.trim())
        .filter(Boolean),
      enabled: settings.schedulerEnabled,
      interval_minutes: nonNegativeCount(settings.schedulerIntervalMinutes, 0),
      skip_if_active: settings.schedulerSkipIfActive,
    },
    sources: sourcePayloads.value.map((source) => ({
      ...source,
      status_text: source.status_text.trim(),
    })),
  };
}

async function refreshColoDictionaryStatus() {
  try {
    const result = await loadColoDictionaryStatus();
    appendLog("bridge.load_colo_dictionary_status", result);
    if (!result.ok) {
      showToast(result.message || "读取 COLO 词典状态失败", "error");
      return;
    }
    coloDictionaryStatus.value = result.data || null;
  } catch (error) {
    showToast(error instanceof Error ? error.message : "读取 COLO 词典状态失败", "error");
  }
}

async function refreshSchedulerStatus() {
  try {
    const result = await loadSchedulerStatus();
    appendLog("bridge.load_scheduler_status", result);
    if (!result.ok) {
      schedulerStatus.value = null;
      if (result.code !== "SCHEDULER_UNSUPPORTED") {
        pushActivity("读取定时任务状态失败", result.message || "无法读取桌面定时任务状态。");
      }
      return;
    }
    schedulerStatus.value = result.data || null;
  } catch (error) {
    schedulerStatus.value = null;
    appendLog("bridge.load_scheduler_status.failed", error instanceof Error ? error.message : String(error));
  }
}

async function refreshColoDictionary() {
  coloDictionaryUpdating.value = true;
  try {
    const result = await updateColoDictionary({});
    appendLog("bridge.update_colo_dictionary", result);
    if (!result.ok) {
      setStatus({
        detail: result.message || "拉取 COLO 原始词典失败。",
        title: "词典拉取失败",
        tone: "failed",
      });
      showToast("拉取 COLO 原始词典失败", "error");
      return;
    }
    coloDictionaryStatus.value = result.data || null;
    setStatus({
      detail: result.message || "COLO 原始词典已拉取，可继续本地处理。",
      title: "COLO 原始词典已拉取",
      tone: "idle",
    });
    showToast("COLO 原始词典已拉取", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "拉取 COLO 原始词典失败", "error");
  } finally {
    coloDictionaryUpdating.value = false;
  }
}

async function processLocalColoDictionary() {
  coloDictionaryProcessing.value = true;
  try {
    const result = await processColoDictionary({});
    appendLog("bridge.process_colo_dictionary", result);
    if (!result.ok) {
      setStatus({
        detail: result.message || "处理 COLO 词典失败。",
        title: "词典处理失败",
        tone: "failed",
      });
      showToast("处理 COLO 词典失败", "error");
      return;
    }
    coloDictionaryStatus.value = result.data || null;
    setStatus({
      detail: result.message || "COLO 词典已本地处理。",
      title: "COLO 词典已处理",
      tone: "idle",
    });
    showToast("COLO 词典已处理", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "处理 COLO 词典失败", "error");
  } finally {
    coloDictionaryProcessing.value = false;
  }
}

function selectedPathValue(data: PathSelectionPayload) {
  return asString(data.path || data.directory || data.uri || data.target_uri).trim();
}

async function selectSourceFile(sourceId: string) {
  const source = sources.value.find((entry) => entry.id === sourceId);
  if (!source) {
    return;
  }

  try {
    const result = await selectPath({
      current_path: source.path,
      mode: "source_file",
      title: "选择输入源文件",
    });
    appendLog("bridge.select_source_file", result);
    const data = asRecord(result.data) as PathSelectionPayload;
    if (!result.ok) {
      showToast(result.message || "选择输入源文件失败", "error");
      return;
    }
    if (data.canceled) {
      return;
    }

    const uploadedContent = asString(data.content).trim();
    const path = selectedPathValue(data);
    if (!path && !uploadedContent) {
      showToast("未获取到文件路径", "error");
      return;
    }
    if (uploadedContent) {
      source.kind = "inline";
      source.content = uploadedContent;
      source.path = "";
      source.status_text = `已读取文件：${asString(data.display_name || data.file_name).trim() || "浏览器文件"}`;
      showToast("已读取输入源文件", "success");
      return;
    }
    source.kind = "file";
    source.path = path;
    if (data.display_name) {
      source.status_text = `已选择文件：${data.display_name}`;
    }
    showToast("已选择输入源文件", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "选择输入源文件失败", "error");
  }
}

async function selectExportTarget() {
  try {
    const result = await selectPath({
      current_path: settings.exportTargetDir,
      default_file_name: settings.exportFileName.trim() || "result.csv",
      mode: "export_target",
      title: "选择导出位置",
    });
    appendLog("bridge.select_export_target", result);
    const data = asRecord(result.data) as PathSelectionPayload;
    if (!result.ok) {
      showToast(result.message || "选择导出位置失败", "error");
      return;
    }
    if (data.canceled) {
      return;
    }

    const targetUri = asString(data.target_uri || data.uri).trim();
    if (targetUri) {
      settings.exportTargetUri = targetUri;
      settings.exportTargetDir = "";
      if (data.display_name || data.file_name) {
        settings.exportFileName = asString(data.display_name || data.file_name).trim();
      }
      showToast("已选择导出文件", "success");
      return;
    }

    const selected = selectedPathValue(data);
    if (!selected) {
      showToast("未获取到导出位置", "error");
      return;
    }
    settings.exportTargetDir = asString(data.directory || data.path || selected).trim();
    if (data.file_name) {
      settings.exportFileName = data.file_name;
    }
    settings.exportTargetUri = "";
    showToast("已选择导出位置", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "选择导出位置失败", "error");
  }
}

async function importConfigFromFile() {
  try {
    const result = await selectPath({
      current_path: configPath.value,
      mode: "config_archive_import",
      title: "加载配置压缩包",
    });
    appendLog("bridge.import_config", result);
    const data = asRecord(result.data) as PathSelectionPayload;
    if (!result.ok) {
      showToast(result.message || "导入配置失败", "error");
      return;
    }
    if (data.canceled) {
      return;
    }

    const imported = await importConfigArchive({
      content: data.content,
      content_base64: data.content_base64,
      current_config_snapshot: buildConfigSnapshot(),
      path: selectedPathValue(data),
    });
    appendLog("bridge.import_config_archive", imported);
    if (!imported.ok) {
      showToast(imported.message || "导入配置失败", "error");
      return;
    }
    applyImportedConfigData(imported.data);
    selectedView.value = "settings";
    showToast(imported.message || "配置已导入，原配置已备份", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "导入配置失败", "error");
  }
}

async function selectStorageDirectory() {
  try {
    const result = await selectPath({
      current_path: storageStatus.value?.current_dir || "",
      mode: "storage_dir",
      title: "选择储存目录",
    });
    appendLog("bridge.select_storage_dir", result);
    const data = asRecord(result.data) as PathSelectionPayload;
    if (!result.ok) {
      showToast(result.message || "选择储存目录失败", "error");
      return;
    }
    if (data.canceled) {
      return;
    }
    const selected = selectedPathValue(data);
    const storageUri = asString(data.storage_uri || data.target_uri || data.uri).trim();
    const update = await setStorageDirectory({
      display_name: asString(data.display_name || data.file_name).trim(),
      migrate: true,
      storage_dir: storageUri ? "" : selected,
      storage_uri: storageUri,
    });
    appendLog("bridge.set_storage_dir", update);
    const updateData = asRecord(update.data);
    if (!update.ok) {
      showToast(update.message || "储存目录更新失败", "error");
      return;
    }
    applyStorageStatus(asRecord(updateData.storage));
    storageSetupVisible.value = false;
    showToast("储存目录已更新", "success");
    await refreshConfig();
  } catch (error) {
    showToast(error instanceof Error ? error.message : "储存目录更新失败", "error");
  }
}

async function useDefaultStorageDirectory() {
  try {
    const result = await setStorageDirectory({ migrate: true, use_default: true });
    appendLog("bridge.set_storage_default", result);
    if (!result.ok) {
      showToast(result.message || "使用默认储存目录失败", "error");
      return;
    }
    applyStorageStatus(asRecord(asRecord(result.data).storage));
    storageSetupVisible.value = false;
    showToast("已使用默认储存目录", "success");
    await refreshConfig();
  } catch (error) {
    showToast(error instanceof Error ? error.message : "使用默认储存目录失败", "error");
  }
}

async function checkCurrentStorageHealth() {
  try {
    const result = await checkStorageHealth({ path: storageStatus.value?.current_dir || "" });
    appendLog("bridge.storage_health", result);
    if (!result.ok) {
      showToast(result.message || "储存目录健康检查失败", "error");
      return;
    }
    applyStorageStatus(asRecord(asRecord(result.data).storage));
    const health = asRecord(asRecord(result.data).health);
    showToast(asString(health.message) || "储存目录健康检查完成", health.writable === false ? "error" : "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "储存目录健康检查失败", "error");
  }
}

async function openStorageDirectory() {
  const target = storageStatus.value?.current_dir || "";
  if (!target) {
    return;
  }
  await openPath(target);
}

async function exportConfigToFile() {
  if (!window.confirm("导出的配置压缩包包含完整 Cloudflare Token 和 WebDAV 凭据。请确认目标位置可信。")) {
    return;
  }
  try {
    const result = await selectPath({
      current_path: storageStatus.value?.current_dir || configPath.value,
      default_file_name: `cfst-gui-config-${new Date().toISOString().slice(0, 10).replace(/-/g, "")}.zip`,
      mode: "config_archive_export",
      title: "导出配置压缩包",
    });
    appendLog("bridge.select_config_archive_export", result);
    const data = asRecord(result.data) as PathSelectionPayload;
    if (!result.ok || data.canceled) {
      return;
    }
    const targetUri = asString(data.target_uri || data.uri).trim();
    const targetPath = targetUri ? "" : selectedPathValue(data);
    const exported = await exportConfigArchive({
      config_snapshot: buildConfigSnapshot(),
      path: targetPath,
      target_uri: targetUri,
    });
    appendLog("bridge.export_config_archive", exported);
    if (!exported.ok) {
      showToast(exported.message || "配置导出失败", "error");
      return;
    }
    showToast("配置压缩包已导出", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "配置导出失败", "error");
  }
}

function applyImportedConfigData(value: unknown) {
  const data = asRecord(value);
  applyConfigSnapshot(normalizeConfigSnapshot(data.config_snapshot || data.configSnapshot || {}));
  if (data.profiles) {
    applyProfileStore(data.profiles);
  }
  if (data.source_profiles || data.sourceProfiles) {
    applySourceProfileStore(data.source_profiles || data.sourceProfiles);
  }
  if (data.storage) {
    applyStorageStatus(data.storage);
  }
  configPath.value = asString(data.configPath || data.config_path || configPath.value);
}

async function backupConfigToLocal() {
  if (!window.confirm("本地备份压缩包会包含完整 Cloudflare Token 和 WebDAV 凭据。请确认储存目录可信。")) {
    return;
  }
  try {
    const result = await backupConfigArchive({ config_snapshot: buildConfigSnapshot() });
    appendLog("bridge.backup_config_archive", result);
    if (!result.ok) {
      showToast(result.message || "本地备份失败", "error");
      return;
    }
    showToast(result.message || "配置压缩包已备份到本地", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "本地备份失败", "error");
  }
}

async function restoreConfigFromLocal() {
  try {
    const result = await selectPath({
      current_path: storageStatus.value?.current_dir || configPath.value,
      mode: "config_archive_import",
      title: "从本地还原配置压缩包",
    });
    appendLog("bridge.select_config_archive_restore", result);
    const data = asRecord(result.data) as PathSelectionPayload;
    if (!result.ok || data.canceled) {
      return;
    }
    const restored = await restoreConfigArchive({
      content: data.content,
      content_base64: data.content_base64,
      current_config_snapshot: buildConfigSnapshot(),
      path: selectedPathValue(data),
    });
    appendLog("bridge.restore_config_archive", restored);
    if (!restored.ok) {
      showToast(restored.message || "本地还原失败", "error");
      return;
    }
    applyImportedConfigData(restored.data);
    selectedView.value = "settings";
    showToast(restored.message || "已从本地还原配置", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "本地还原失败", "error");
  }
}

async function testWebDAVSettings() {
  try {
    const result = await testWebDAV({ config_snapshot: buildConfigSnapshot() });
    appendLog("bridge.test_webdav", result);
    showToast(result.message || (result.ok ? "WebDAV 连接可用" : "WebDAV 测试失败"), result.ok ? "success" : "error");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "WebDAV 测试失败", "error");
  }
}

async function testGitHubExportSettings() {
  githubTesting.value = true;
  try {
    const result = await testGitHubExport({ config: buildConfigSnapshot() });
    appendLog("bridge.test_github_export", result);
    showToast(result.message || (result.ok ? "GitHub 仓库访问可用" : "GitHub 导出测试失败"), result.ok ? "success" : "error");
    if (!result.ok) {
      pushActivity("GitHub 导出测试失败", result.message || "请检查 owner、repo、branch 与 PAT 权限。");
      return;
    }
    pushActivity("GitHub 导出测试通过", result.message || "目标仓库 Contents 权限可用。");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "GitHub 导出测试失败", "error");
  } finally {
    githubTesting.value = false;
  }
}

async function backupToWebDAV() {
  if (!window.confirm("WebDAV 备份会覆盖远端配置压缩包，并包含完整 Cloudflare Token 和 WebDAV 凭据。确认继续？")) {
    return;
  }
  try {
    const result = await backupConfigToWebDAV({ config_snapshot: buildConfigSnapshot() });
    appendLog("bridge.backup_config_webdav", result);
    if (!result.ok) {
      showToast(result.message || "WebDAV 备份失败", "error");
      return;
    }
    settings.webdavLastBackupAt = new Date().toISOString();
    showToast(result.message || "已备份到 WebDAV", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "WebDAV 备份失败", "error");
  }
}

async function restoreFromWebDAV() {
  if (!window.confirm("从 WebDAV 还原会替换当前配置，当前配置会先自动备份到本地。确认继续？")) {
    return;
  }
  try {
    const result = await restoreConfigFromWebDAV({
      config_snapshot: buildConfigSnapshot(),
      current_config_snapshot: buildConfigSnapshot(),
    });
    appendLog("bridge.restore_config_webdav", result);
    if (!result.ok) {
      showToast(result.message || "WebDAV 还原失败", "error");
      return;
    }
    applyImportedConfigData(result.data);
    selectedView.value = "settings";
    showToast(result.message || "已从 WebDAV 还原配置", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "WebDAV 还原失败", "error");
  }
}

async function saveProfile(name: string, profileId = "", configSnapshot?: unknown, setActive = true) {
  try {
    const snapshot = asRecord(configSnapshot);
    const result = await saveCurrentProfile({
      config_snapshot: Object.keys(snapshot).length > 0 ? normalizeConfigSnapshot(snapshot) : buildConfigSnapshot(),
      name: name.trim() || "当前配置",
      profile_id: profileId,
      set_active: setActive,
    });
    appendLog("bridge.save_profile", result);
    if (!result.ok) {
      showToast(result.message || "保存配置档案失败", "error");
      return;
    }
    applyProfileStore(result.data);
    showToast("配置档案已保存", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "保存配置档案失败", "error");
  }
}

async function switchToProfile(profileId: string) {
  try {
    const result = await switchProfile({ profile_id: profileId });
    appendLog("bridge.switch_profile", result);
    const data = asRecord(result.data);
    if (!result.ok) {
      showToast(result.message || "切换配置档案失败", "error");
      return;
    }
    applyConfigSnapshot(normalizeConfigSnapshot(data.config_snapshot || {}));
    applyProfileStore(data.profiles);
    if (data.source_profiles || data.sourceProfiles) {
      applySourceProfileStore(data.source_profiles || data.sourceProfiles);
    }
    applyStorageStatus(data.storage);
    configPath.value = asString(data.configPath || data.config_path || configPath.value);
    showToast("配置档案已切换", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "切换配置档案失败", "error");
  }
}

async function removeProfile(profileId: string) {
  if (!window.confirm("删除配置档案不会删除当前配置文件，但该档案无法恢复。")) {
    return;
  }
  try {
    const result = await deleteProfile({ profile_id: profileId });
    appendLog("bridge.delete_profile", result);
    if (!result.ok) {
      showToast(result.message || "删除配置档案失败", "error");
      return;
    }
    applyProfileStore(result.data);
    showToast("配置档案已删除", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "删除配置档案失败", "error");
  }
}

async function saveCurrentSourceProfile(name: string, profileId = "", profileSources?: unknown, setActive = true) {
  try {
    const payloadSources = Array.isArray(profileSources)
      ? profileSources.map((entry, index) => ({
          ...entry,
          name: asString(asRecord(entry).name).trim() || `输入源 ${index + 1}`,
        }))
      : sourcePayloads.value;
    const result = await saveSourceProfile({
      name: name.trim() || "当前输入源",
      profile_id: profileId,
      set_active: setActive,
      sources: payloadSources,
    });
    appendLog("bridge.save_source_profile", result);
    if (!result.ok) {
      showToast(result.message || "保存输入源档案失败", "error");
      return;
    }
    applySourceProfileStore(result.data);
    showToast("输入源档案已保存", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "保存输入源档案失败", "error");
  }
}

async function switchToSourceProfile(profileId: string) {
  try {
    const result = await switchSourceProfile({ profile_id: profileId });
    appendLog("bridge.switch_source_profile", result);
    const data = asRecord(result.data);
    if (!result.ok) {
      showToast(result.message || "切换输入源档案失败", "error");
      return;
    }
    applySourceProfileStore(data.source_profiles || data.sourceProfiles);
    const nextSources = Array.isArray(data.sources) ? data.sources : [];
    sources.value = nextSources.length > 0 ? normalizeConfigSnapshot({ sources: nextSources }).sources.map((source) => ({ ...source })) : [createSourceDraft()];
    if (data.config_snapshot || data.configSnapshot) {
      configPath.value = asString(data.configPath || data.config_path || configPath.value);
    }
    showToast("输入源档案已切换", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "切换输入源档案失败", "error");
  }
}

async function removeSourceProfile(profileId: string) {
  if (!window.confirm("删除输入源档案后无法恢复。当前输入源列表不会被删除。")) {
    return;
  }
  try {
    const result = await deleteSourceProfile({ profile_id: profileId });
    appendLog("bridge.delete_source_profile", result);
    if (!result.ok) {
      showToast(result.message || "删除输入源档案失败", "error");
      return;
    }
    applySourceProfileStore(result.data);
    showToast("输入源档案已删除", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "删除输入源档案失败", "error");
  }
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
  const normalizedTaskId = taskId.trim() || "result-file";

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
        listTaskResults(
          normalizedTaskId,
          resultSortBy.value,
          resultOrder.value,
          resultFilter.value,
          {
            config: buildConfigSnapshot(),
            export_path: task.exportPath,
          },
          resultIpFilter.value,
        ),
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
  const incomingTaskId = asString(event.task_id).trim();
  const currentTaskId = task.taskId.trim();
  if (incomingTaskId && currentTaskId && incomingTaskId !== currentTaskId) {
    appendLog(`${event.event}.ignored`, {
      current_task_id: currentTaskId,
      event_task_id: incomingTaskId,
      payload: event.payload,
    });
    return;
  }

  appendLog(event.event, event.payload);
  const nextTaskState = deriveTaskStateFromProbeEvent(event);

  setStatus(nextTaskState);
  task.active = !["completed", "failed", "no_results"].includes(nextTaskState.tone);
  task.lastEvent = event.event;
  task.lastSeq = event.seq;
  task.taskId = event.task_id || task.taskId;
  const eventDebugLogPath = asString(event.payload.debug_log_path || event.payload.debugLogPath).trim();

  if (event.event === "probe.preprocessed") {
    resetDownloadSpeedState();
    summary.accepted = asCount(event.payload.accepted);
    summary.filtered = asCount(event.payload.filtered);
    summary.invalid = asCount(event.payload.invalid);
    summary.processed = 0;
    summary.passed = 0;
    summary.failed = 0;
    summary.total = asCount(event.payload.total, summary.accepted);
    task.stage = "stage0_pool";
    applySourceStatuses(event.payload.source_statuses);
    pushProcessTrace({
      detail: `候选 ${summary.total} 条，接受 ${summary.accepted} 条，过滤 ${summary.filtered} 条，非法 ${summary.invalid} 条。`,
      stage: "stage0_pool",
      title: "阶段0 IP池完成",
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
    if (task.stage !== "stage3_get") {
      resetDownloadSpeedState();
    }
    const progressPrefix = task.stage === "stage3_get" ? "文件测速" : "";
    pushProcessTrace({
      detail: `${progressPrefix ? `${progressPrefix}，` : ""}已处理 ${summary.processed}/${summary.total || "-"}，通过 ${summary.passed}，失败 ${summary.failed}。`,
      stage: task.stage,
      title: `${stageTitle(task.stage)}进行中`,
      tone: "running",
      ts: event.ts,
    });
  }

  if (event.event === "probe.speed") {
    task.stage = asString(event.payload.stage) || "stage3_get";
    const ip = asString(event.payload.ip).trim() || "当前 IP";
    const currentSpeed = asNumber(event.payload.current_speed_mb_s, 0);
    const averageSpeed = asNumber(event.payload.average_speed_mb_s, 0);
    const currentReadyValue = event.payload.current_ready ?? event.payload.currentReady;
    const averageReadyValue = event.payload.average_ready ?? event.payload.averageReady;
    const currentReady = currentReadyValue === undefined ? true : asBoolean(currentReadyValue, false);
    const averageReady = averageReadyValue === undefined ? true : asBoolean(averageReadyValue, false);
    const bytesRead = asCount(event.payload.bytes_read, 0);
    const measuredBytes = asCount(event.payload.measured_bytes ?? event.payload.measuredBytes, 0);
    const bodyReadValue = event.payload.body_read ?? event.payload.bodyRead;
    const transferCompleteValue = event.payload.transfer_complete ?? event.payload.transferComplete;
    const bodyRead = bodyReadValue === undefined ? bytesRead > 0 : asBoolean(bodyReadValue, false);
    const transferComplete =
      transferCompleteValue === undefined ? false : asBoolean(transferCompleteValue, false);
    const elapsedMs = asCount(event.payload.elapsed_ms, 0);
    const colo = asString(event.payload.colo).trim();
    const hasReadyFlag = currentReadyValue !== undefined || averageReadyValue !== undefined;
    const isInitialEmptySample =
      elapsedMs === 0 &&
      bytesRead === 0 &&
      ((hasReadyFlag && !currentReady && !averageReady) ||
        (!hasReadyFlag && currentSpeed === 0 && averageSpeed === 0 && !downloadSpeedState.ip));
    if (isInitialEmptySample) {
      return;
    }
    if (downloadSpeedState.ip && downloadSpeedState.ip !== ip) {
      resetDownloadSpeedState();
    }
    downloadSpeedState.active = true;
    const averageDisplayReady =
      averageReady && (averageSpeed !== 0 || bytesRead > 0 || measuredBytes > 0 || bodyRead || transferComplete);
    downloadSpeedState.averageSpeedMbS = averageDisplayReady ? averageSpeed : null;
    downloadSpeedState.bytesRead = bytesRead;
    downloadSpeedState.colo = colo;
    downloadSpeedState.currentSpeedMbS = currentReady ? currentSpeed : null;
    downloadSpeedState.elapsedMs = elapsedMs;
    downloadSpeedState.ip = ip;
    updateDownloadSpeedTrace(ip, colo, event.ts);
  }

  if (event.event === "probe.partial_export") {
    summary.exported = asCount(event.payload.written, summary.exported);
    task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
    updateHistory({
      debugLogPath: eventDebugLogPath,
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
    resetDownloadSpeedState();
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
      debugLogPath: eventDebugLogPath,
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
    resetDownloadSpeedState();
    task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
    const failureMessage = asString(event.payload.message || status.detail).trim() || "探测任务失败。";
    updateHistory({
      debugLogPath: eventDebugLogPath,
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
    downloadSpeedState.active = false;
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
  applyDetectedSourceName(sourceId);
  const sourceIndex = sources.value.findIndex((entry) => entry.id === sourceId);
  const source = sourceIndex >= 0 ? sources.value[sourceIndex] : null;
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
        name: sourceNameForPayload(source, sourceIndex),
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
    applyStorageStatus(data.storage);
    applyProfileStore(data.profiles);
    if (data.source_profiles || data.sourceProfiles) {
      applySourceProfileStore(data.source_profiles || data.sourceProfiles);
    }
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

async function refreshAppInfo() {
  try {
    const result = await getAppInfo();
    appendLog("bridge.get_app_info", result);
    if (result.ok && result.data) {
      applyAppInfo(result.data);
    }
  } catch (error) {
    appendLog("bridge.get_app_info.failed", error instanceof Error ? error.message : String(error));
  }
}

async function checkOnlineUpdate() {
  updateState.status = "checking";
  updateState.message = "正在检查 GitHub Releases。";
  try {
    const result = await checkForUpdates({});
    appendLog("bridge.check_updates", result);
    if (!result.ok) {
      updateState.status = "failed";
      updateState.message = result.message || "检查更新失败。";
      showToast(updateState.message, "error");
      return;
    }
    applyUpdateInfo(result.data || {});
    if (updateState.updateAvailable) {
      updateState.status = "available";
      updateState.message = result.message || `发现新版本 ${updateState.latestVersion}。`;
      showToast("发现新版本", "info");
    } else {
      updateState.status = "latest";
      updateState.message = result.message || "当前已是最新版本。";
      showToast("当前已是最新版本", "success");
    }
  } catch (error) {
    updateState.status = "failed";
    updateState.message = error instanceof Error ? error.message : "检查更新失败。";
    showToast(updateState.message, "error");
  }
}

async function installOnlineUpdate() {
  updateState.status = "installing";
  updateState.installing = true;
  updateState.message = "正在下载更新包。";
  try {
    const result = await downloadAndInstallUpdate({});
    appendLog("bridge.download_update", result);
    if (!result.ok) {
      updateState.status = "failed";
      updateState.message = result.message || "下载或安装更新失败。";
      showToast(updateState.message, "error");
      return;
    }
    applyUpdateInfo(result.data || {});
    updateState.downloadPath = asString(asRecord(result.data).downloaded_path || asRecord(result.data).downloadedPath);
    updateState.status = "ready";
    updateState.message = result.message || "更新安装流程已启动。";
    showToast("更新安装流程已启动", "success");
  } catch (error) {
    updateState.status = "failed";
    updateState.message = error instanceof Error ? error.message : "下载或安装更新失败。";
    showToast(updateState.message, "error");
  } finally {
    updateState.installing = false;
  }
}

async function openOnlineReleasePage() {
  try {
    const result = await openReleasePage();
    appendLog("bridge.open_release_page", result);
    if (!result.ok) {
      showToast(result.message || "打开发行页失败", "error");
    }
  } catch (error) {
    showToast(error instanceof Error ? error.message : "打开发行页失败", "error");
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
    applyStorageStatus(data.storage);
    applyProfileStore(data.profiles);
    if (data.source_profiles || data.sourceProfiles) {
      applySourceProfileStore(data.source_profiles || data.sourceProfiles);
    }
    configPath.value = asString(data.configPath || data.config_path || configPath.value);
    setStatus({
      detail: result.message || "配置已保存。",
      title: "配置已保存",
      tone: "idle",
    });
    pushActivity("配置已保存", result.message || "设置已保存并可用于后续任务。");
    showToast("配置已保存");
    await refreshSchedulerStatus();
  } finally {
    loading.value = false;
  }
}

async function launchProbe() {
  if (hasActiveTask.value) {
    notifyActiveProbeBlocked("启动任务被拦截");
    return;
  }

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
  resetDownloadSpeedState();
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

  if (hasActiveTask.value) {
    notifyActiveProbeBlocked("单条重测被拦截");
    return;
  }

  loading.value = true;
  resetProbeSummary();
  resetDownloadSpeedState();
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
    resultOrder.value = sortBy === "download" || sortBy === "max_download" ? "desc" : "asc";
  }

  void refreshTaskData();
}

function updateResultFilter(filter: ProbeResultFilter) {
  resultFilter.value = filter;
  void refreshTaskData();
}

function updateResultIpFilter(filter: ProbeResultIPFilter) {
  resultIpFilter.value = filter;
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
        detail: result.message || "继续失败。",
        title: "继续失败",
        tone: "failed",
      });
      showToast("继续失败", "error");
      return;
    }

    setStatus({
      detail: result.message || "已请求继续，等待新的 progress 事件。",
      title: "继续请求已发送",
      tone: "running",
    });
    pushActivity("请求继续", result.message || "已向桌面端发送继续请求。");
    showToast("已请求继续", "info");
  } finally {
    loading.value = false;
  }
}

async function fetchDnsRecords() {
  isLoadingDns.value = true;

  try {
    const result = await listDnsRecords({
      config: buildConfigSnapshot(),
    });
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
      config: buildConfigSnapshot(),
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

async function exportCurrentResultsToGitHub() {
  if (resultRows.value.length === 0) {
    selectedView.value = "results";
    showToast("没有可导出的测速结果", "error");
    return;
  }

  githubExporting.value = true;
  try {
    const result = await exportResultsToGitHub({
      config: buildConfigSnapshot(),
      export_path: task.exportPath,
      results: resultRows.value,
      task_id: task.taskId,
    });
    const data = asRecord(result.data);
    appendLog("bridge.export_results_github", result);
    if (!result.ok) {
      setStatus({
        detail: result.message || "导出到 GitHub 失败。",
        title: "GitHub 导出失败",
        tone: "failed",
      });
      pushActivity("GitHub 导出失败", result.message || "未能写入目标仓库。");
      showToast("GitHub 导出失败", "error");
      return;
    }

    settings.githubLastExportAt = asString(data.exported_at || data.exportedAt).trim() || new Date().toISOString();
    const targetPath = asString(data.path).trim();
    const htmlURL = asString(data.html_url || data.htmlURL).trim();
    setStatus({
      detail: targetPath ? `已写入 GitHub：${targetPath}` : result.message || "测速结果已导出到 GitHub。",
      title: "GitHub 导出完成",
      tone: "completed",
    });
    pushActivity("GitHub 导出完成", htmlURL || targetPath || result.message || "测速结果 CSV 已写入目标仓库。");
    showToast("已导出到 GitHub", "success");
  } finally {
    githubExporting.value = false;
  }
}

async function openHistoryTarget(targetPath: string) {
  if (!targetPath) {
    return;
  }

  await openPath(targetPath);
}

onMounted(async () => {
  window.addEventListener("resize", scheduleViewportSizeRefresh);
  await ensureAdaptiveViewportOnStartup();
  appendLog("system.boot", { message: "桌面端调用链已初始化。" });
  pushActivity("桌面端已启动", "等待桌面端返回配置与任务状态。");
  removeProbeListener = await listenToProbeEvents((event) => {
    applyProbeEvent(event);
  });
  await refreshConfig();
  await refreshAppInfo();
  await refreshColoDictionaryStatus();
  await refreshSchedulerStatus();
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", scheduleViewportSizeRefresh);
  if (viewportResizeTimer !== undefined) {
    window.clearTimeout(viewportResizeTimer);
  }
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
      :can-resume-task="canResumeTask"
      :download-speed-state="downloadSpeedState"
      :export-history="exportHistory"
      :has-active-task="hasActiveTask"
      :last-history-entry="lastHistoryEntry"
      :loading="loading"
      platform="desktop"
      :process-trace="processTrace"
      :probe-warnings="probeWarnings"
      :progress-percent="progressPercent"
      :status-label="dashboardStatusLabel"
      :status-tone="status.tone"
      :summary="summary"
      :task="task"
      @clear-process="clearProcessTrace"
      @open-history-target="openHistoryTarget"
      @pause="pauseProbe"
      @resume="continueProbe"
      @start="launchProbe"
    />

    <ResultsView
      v-else-if="selectedView === 'results'"
      :has-active-task="hasActiveTask"
      :loading="loading"
      platform="desktop"
      :result-filter="resultFilter"
      :result-filter-options="resultFilterOptions"
      :result-ip-filter="resultIpFilter"
      :result-ip-filter-options="resultIpFilterOptions"
      :result-order="resultOrder"
      :result-rows="resultRows"
      :result-sort-by="resultSortBy"
      :result-sort-options="resultSortOptions"
      :results-loading="resultsLoading"
      :github-exporting="githubExporting"
      :summary="summary"
      :task="task"
      :task-snapshot="taskSnapshot"
      @copy-address="copyAddress"
      @export-github="exportCurrentResultsToGitHub"
      @refresh-results="refreshCurrentTaskData"
      @rerun-address="rerunSingleAddress"
      @update-filter="updateResultFilter"
      @update-ip-filter="updateResultIpFilter"
      @update-order="updateResultOrder"
      @update-sort="updateResultSort"
    />

    <SourcesView
      v-else-if="selectedView === 'sources'"
      :accepted="summary.accepted"
      :invalid="summary.invalid"
      platform="desktop"
      :prepared-count="preparedSources.length"
      :colo-dictionary-status="coloDictionaryStatus"
      :colo-dictionary-processing="coloDictionaryProcessing"
      :colo-dictionary-updating="coloDictionaryUpdating"
      :preview-states="sourcePreviewStates"
      :request-states="sourceRequestStates"
      :source-profiles="sourceProfiles"
      :sources="sources"
      :task-stage="task.stage"
      @add="addSource"
      @delete-source-profile="removeSourceProfile"
      @detect-source-name="applyDetectedSourceName"
      @process-colo-dictionary="processLocalColoDictionary"
      @refresh-colo-dictionary="refreshColoDictionary"
      @fetch-source="inspectSource($event, 'fetch')"
      @preview="inspectSource($event, 'preview')"
      @preview-request="inspectSource($event, 'fetch')"
      @remove="removeSource"
      @save="persistConfig"
      @save-source-profile="saveCurrentSourceProfile"
      @select-file="selectSourceFile"
      @switch-source-profile="switchToSourceProfile"
    />

    <SettingsView
      v-else-if="selectedView === 'settings'"
      :loading="loading"
      :app-info="appInfo"
      :masked-token-hint="maskedTokenHint"
      platform="desktop"
      :profiles="profiles"
      :save-blocked-by-masked-token="saveBlockedByMaskedToken"
      :settings="settings"
      :show-token="showToken"
      :github-testing="githubTesting"
      :scheduler-status="schedulerStatus"
      :storage="storageStatus"
      :update-state="updateState"
      :viewport-adaptive-active="viewportAdaptiveActive"
      :viewport-presets="VIEWPORT_PRESETS"
      :viewport-runtime-supported="viewportRuntimeSupported"
      :viewport-size="viewportSize"
      :viewport-switching="viewportSwitching"
      @apply-viewport-preset="applyViewportPreset"
      @backup-config-local="backupConfigToLocal"
      @backup-config-webdav="backupToWebDAV"
      @check-storage-health="checkCurrentStorageHealth"
      @check-update="checkOnlineUpdate"
      @delete-profile="removeProfile"
      @export-config="exportConfigToFile"
      @import-config="importConfigFromFile"
      @open-storage-dir="openStorageDirectory"
      @open-release-page="openOnlineReleasePage"
      @save="persistConfig"
      @save-profile="saveProfile"
      @select-export-target="selectExportTarget"
      @select-storage-dir="selectStorageDirectory"
      @restore-config-local="restoreConfigFromLocal"
      @restore-config-webdav="restoreFromWebDAV"
      @install-update="installOnlineUpdate"
      @switch-profile="switchToProfile"
      @test-github-export="testGitHubExportSettings"
      @test-webdav="testWebDAVSettings"
      @toggle-token="showToken = !showToken"
      @use-default-storage-dir="useDefaultStorageDirectory"
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
      :can-resume-task="canResumeTask"
      :download-speed-state="downloadSpeedState"
      :export-history="exportHistory"
      :has-active-task="hasActiveTask"
      :last-history-entry="lastHistoryEntry"
      :loading="loading"
      platform="mobile"
      :process-trace="processTrace"
      :probe-warnings="probeWarnings"
      :progress-percent="progressPercent"
      :status-label="dashboardStatusLabel"
      :status-tone="status.tone"
      :summary="summary"
      :task="task"
      @clear-process="clearProcessTrace"
      @open-history-target="openHistoryTarget"
      @pause="pauseProbe"
      @resume="continueProbe"
      @start="launchProbe"
    />

    <ResultsView
      v-else-if="selectedView === 'results'"
      :has-active-task="hasActiveTask"
      :loading="loading"
      platform="mobile"
      :result-filter="resultFilter"
      :result-filter-options="resultFilterOptions"
      :result-ip-filter="resultIpFilter"
      :result-ip-filter-options="resultIpFilterOptions"
      :result-order="resultOrder"
      :result-rows="resultRows"
      :result-sort-by="resultSortBy"
      :result-sort-options="resultSortOptions"
      :results-loading="resultsLoading"
      :github-exporting="githubExporting"
      :summary="summary"
      :task="task"
      :task-snapshot="taskSnapshot"
      @copy-address="copyAddress"
      @export-github="exportCurrentResultsToGitHub"
      @refresh-results="refreshCurrentTaskData"
      @rerun-address="rerunSingleAddress"
      @update-filter="updateResultFilter"
      @update-ip-filter="updateResultIpFilter"
      @update-order="updateResultOrder"
      @update-sort="updateResultSort"
    />

    <SourcesView
      v-else-if="selectedView === 'sources'"
      :accepted="summary.accepted"
      :invalid="summary.invalid"
      platform="mobile"
      :prepared-count="preparedSources.length"
      :colo-dictionary-status="coloDictionaryStatus"
      :colo-dictionary-processing="coloDictionaryProcessing"
      :colo-dictionary-updating="coloDictionaryUpdating"
      :preview-states="sourcePreviewStates"
      :request-states="sourceRequestStates"
      :source-profiles="sourceProfiles"
      :sources="sources"
      :task-stage="task.stage"
      @add="addSource"
      @delete-source-profile="removeSourceProfile"
      @detect-source-name="applyDetectedSourceName"
      @process-colo-dictionary="processLocalColoDictionary"
      @refresh-colo-dictionary="refreshColoDictionary"
      @fetch-source="inspectSource($event, 'fetch')"
      @preview="inspectSource($event, 'preview')"
      @preview-request="inspectSource($event, 'fetch')"
      @remove="removeSource"
      @save="persistConfig"
      @save-source-profile="saveCurrentSourceProfile"
      @select-file="selectSourceFile"
      @switch-source-profile="switchToSourceProfile"
    />

    <SettingsView
      v-else-if="selectedView === 'settings'"
      :loading="loading"
      :app-info="appInfo"
      :masked-token-hint="maskedTokenHint"
      platform="mobile"
      :profiles="profiles"
      :save-blocked-by-masked-token="saveBlockedByMaskedToken"
      :settings="settings"
      :show-token="showToken"
      :github-testing="githubTesting"
      :scheduler-status="schedulerStatus"
      :storage="storageStatus"
      :update-state="updateState"
      :viewport-adaptive-active="viewportAdaptiveActive"
      :viewport-presets="VIEWPORT_PRESETS"
      :viewport-runtime-supported="viewportRuntimeSupported"
      :viewport-size="viewportSize"
      :viewport-switching="viewportSwitching"
      @apply-viewport-preset="applyViewportPreset"
      @backup-config-local="backupConfigToLocal"
      @backup-config-webdav="backupToWebDAV"
      @check-storage-health="checkCurrentStorageHealth"
      @check-update="checkOnlineUpdate"
      @delete-profile="removeProfile"
      @export-config="exportConfigToFile"
      @import-config="importConfigFromFile"
      @open-storage-dir="openStorageDirectory"
      @open-release-page="openOnlineReleasePage"
      @save="persistConfig"
      @save-profile="saveProfile"
      @select-export-target="selectExportTarget"
      @select-storage-dir="selectStorageDirectory"
      @restore-config-local="restoreConfigFromLocal"
      @restore-config-webdav="restoreFromWebDAV"
      @install-update="installOnlineUpdate"
      @switch-profile="switchToProfile"
      @test-github-export="testGitHubExportSettings"
      @test-webdav="testWebDAVSettings"
      @toggle-token="showToken = !showToken"
      @use-default-storage-dir="useDefaultStorageDirectory"
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

  <div v-if="storageSetupVisible" class="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/50 px-4">
    <section class="w-full max-w-xl rounded-2xl bg-white p-6 shadow-2xl">
      <p class="text-sm font-semibold text-primary">首次储存设置</p>
      <h2 class="mt-2 text-2xl font-bold text-slate-900">选择 CFST-GUI 的储存目录</h2>
      <p class="mt-3 text-sm leading-6 text-slate-500">
        配置、COLO 词典、日志和默认结果文件会放在这里。可以继续使用默认目录，之后也能在全局设置里调整。
      </p>
      <div class="mt-4 rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
        <p class="font-medium text-slate-700">默认目录</p>
        <p class="mt-1 break-all font-mono text-xs">{{ storageStatus?.default_dir || "等待后端返回默认目录" }}</p>
      </div>
      <div class="mt-6 flex flex-wrap justify-end gap-3">
        <button type="button" class="ui-button ui-button-ghost" @click="storageSetupDismissed = true; storageSetupVisible = false">稍后</button>
        <button type="button" class="ui-button ui-button-ghost" @click="useDefaultStorageDirectory">使用默认目录</button>
        <button type="button" class="ui-button ui-button-primary" @click="selectStorageDirectory">选择目录</button>
      </div>
    </section>
  </div>

  <ToastStack :toasts="toasts" />
</template>
