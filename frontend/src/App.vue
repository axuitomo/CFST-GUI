<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { WindowCenter, WindowGetSize, WindowIsMaximised, WindowMaximise, WindowSetSize, WindowUnfullscreen, WindowUnmaximise } from "../wailsjs/runtime/runtime";
import {
  backupConfigToWebDAV,
  checkForUpdates,
  checkBatteryOptimization,
  checkKeepAliveStatus,
  checkNotificationPermission,
  checkStorageHealth,
  clearTaskWorkspaceCache,
  discardDesktopDraft,
  deletePipelineTemplate,
  deleteSourceProfile,
  downloadAndInstallUpdate,
  deriveTaskStateFromProbeEvent,
  exportConfigArchive,
  exportDebugLog,
  exportDiagnosticPackage,
  exportResultsCSV,
  exportResultsToGitHub,
  fetchDesktopSource,
  getAndroidRuntimeStatus,
  getPipelineSnapshot,
  getTaskSnapshot,
  getAppInfo,
  importConfigArchive,
  isMaskedTokenValue,
  listenToProbeEvents,
  listDnsRecords,
  listPipelineResults,
  listTaskResults,
  loadColoDictionaryStatus,
  loadConfig,
  loadSchedulerStatus,
  normalizeConfigSnapshot,
  normalizeDnsRecords,
  defaultPipelineNodeCatalog,
  normalizePipelineProfileStore,
  normalizePipelineRunResult,
  normalizePipelineRunResults,
  normalizePipelineWorkspace,
  pipelineProfileStoreFromWorkspace,
  normalizeSourceProfileStore,
  openLogDirectory,
  openReleasePage,
  openPath,
  openBatteryOptimizationSettings,
  openNotificationSettings,
  previewDesktopSource,
  processColoDictionary,
  pushDnsRecords,
  resumeProbe,
  restoreConfigFromWebDAV,
  requestNotificationPermission,
  saveConfig,
  saveDesktopDraft,
  savePipelineTemplate,
  savePipelineWorkspace,
  saveSourceProfile,
  selectPath,
  setKeepAliveEnabled,
  startProbe,
  startPipeline,
  stopProbe,
  summarizeTraceDiagnostics,
  switchSourceProfile,
  testGitHubExport,
  testTelegramNotification,
  testWebDAV,
  updateColoDictionary,
  updateCurrentSourceProfile,
  type ColoDictionaryStatus,
  type AppInfo,
  type AndroidBatteryStatus,
  type AndroidKeepAliveStatus,
  type AndroidNotificationPermissionStatus,
  type ColoFilterMode,
  type ConfigSnapshot,
  type CSVEncoding,
  type DebugLogMode,
  type DebugLogVerbosity,
  type DesktopSourceConfig,
  type DownloadSpeedMetric,
  type DnsRecordSnapshot,
  type PathSelectionPayload,
  type PipelineProfileStore,
  type PipelineNodeCatalogItem,
  type PipelineTemplate,
  type PipelineWorkspace,
  type PipelineRunResult,
  type ProbeEventEnvelope,
  type ProbeResult,
  type ProbeResultFilter,
  type ProbeResultIPFilter,
  type ProbeResultOrder,
  type ProbeResultSortBy,
  type ProbeStrategy,
  type SourceProfileStore,
  type SourcePreviewPayload,
  type SourceIPMode,
  type SourceKind,
  type SourceColoFilterPhase,
  type StorageStatus,
  type SchedulerRunMode,
  type SchedulerStatus,
  type TaskSnapshot,
  type TaskTone,
  type TelegramRecipientMode,
  type TraceColoMode,
  type UpdateInfo,
} from "./lib/bridge";
import { detectSourceNameFromUrl, isDefaultSourceName } from "./lib/sourceNames";
import { stablePipelineWorkspaceSignature } from "./lib/pipelineStudio";
import { DEFAULT_UTC_OFFSET_MINUTES, currentMinutesInUTCOffset, formatUTCOffsetLabel, formatTimestampWithUTCOffset, normalizeUTCOffsetMinutes } from "./lib/time";
import DesktopShell from "./components/layout/DesktopShell.vue";
import MobileShell from "./components/layout/MobileShell.vue";
import ToastStack from "./components/ui/ToastStack.vue";
import DashboardView from "./views/DashboardView.vue";
import DnsView from "./views/DnsView.vue";
import ResultsView from "./views/ResultsView.vue";
import SettingsView from "./views/SettingsView.vue";
import SourcesView from "./views/SourcesView.vue";
import WorkflowView from "./views/WorkflowView.vue";

type AppMode = "single" | "workflow";
type ViewName = "dashboard" | "results" | "sources" | "settings" | "dns";
type ToastTone = "success" | "error" | "info";
type ViewportPresetId = "adaptive" | "phone390" | "tablet768" | "desktop1024" | "desktop1366" | "desktop1920" | "desktop2560";
type FixedViewportPresetId = Exclude<ViewportPresetId, "adaptive">;
type ResultCloudflareRecordType = "ALL" | "A" | "AAAA";
type SchedulerTriggerMode = "interval" | "daily";

interface WailsRuntimeWindow extends Window {
  runtime?: unknown;
}

interface HistoryEntry {
  debugLogPath?: string;
  debugLogTarget?: string;
  detail: string;
  exported: number;
  failureSummary: string;
  targetPath: string;
  taskId: string;
  title: string;
  tone: TaskTone;
  updatedAt: string;
}

interface ResultCloudflarePushSettings {
  recordName: string;
  recordType: ResultCloudflareRecordType;
  topN: number;
}

interface CloudflareRoutingRuleForm {
  enabled: boolean;
  filterMode: "allow" | "deny";
  filterTokens: string;
  id: string;
  name: string;
  recordName: string;
  recordType: "A" | "AAAA" | "ALL";
  topN: number;
}

interface SettingsForm {
  apiToken: string;
  comment: string;
  cloudflareEnabled: boolean;
  postProbePushCloudflareEnabled: boolean;
  postProbePushGitHubEnabled: boolean;
  telegramBotToken: string;
  telegramChatId: string;
  telegramIncludeTopN: boolean;
  telegramNotificationEnabled: boolean;
  telegramPersonalChatId: string;
  telegramTopNRecipientMode: TelegramRecipientMode;
  telegramTopN: number;
  telegramUploadRecipientMode: TelegramRecipientMode;
  uploadCloudflareRoutingEnabled: boolean;
  uploadCloudflareRoutingRules: CloudflareRoutingRuleForm[];
  uploadCloudflareTopN: number;
  uploadGitHubTopN: number;
  uploadSharedFilterColoAllow: string;
  uploadSharedFilterColoDeny: string;
  uploadSharedFilterEnabled: boolean;
  uploadSharedFilterIPVersion: "any" | "ipv4" | "ipv6";
  uploadSharedFilterMaxLossRate: number | null;
  uploadSharedFilterMaxTcpLatencyMs: number | null;
  uploadSharedFilterMaxTraceLatencyMs: number | null;
  uploadSharedFilterMinDownloadMbps: number;
  uploadSharedFilterStatus: "all" | "passed";
  exportFileName: string;
  exportFileNameTemplate: string;
  githubBranch: string;
  githubCSVHeaderTemplate: string;
  githubCSVRowTemplate: string;
  githubCommitMessageTemplate: string;
  githubExportEnabled: boolean;
  githubFormat: "csv" | "txt";
  githubLastExportAt: string;
  githubOwner: string;
  githubPathTemplate: string;
  githubRepo: string;
  githubToken: string;
  githubTXTRowTemplate: string;
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
  probePortPolicy: "source_override_global" | "fixed_global";
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
  schedulerIntervalMinutesDraft: number;
  schedulerPipelineTemplateId: string;
  schedulerRunMode: SchedulerRunMode;
  schedulerSkipIfActive: boolean;
  schedulerTriggerMode: SchedulerTriggerMode;
  maintenanceCompletedTaskRetentionDays: number;
  sourceAutoDetectName: boolean;
  themeDarkStart: string;
  themeLightStart: string;
  themeMode: "light" | "dark" | "auto_system_time" | "auto_time";
  utcOffsetMinutes: number;
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

const DEFAULT_PROBE_USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0";
const DEFAULT_FILE_TEST_URL = "https://speedtest.xyz9923.dpdns.org/500m";
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
  portSummary: Record<string, unknown> | null;
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

interface AndroidSelectOption {
  disabled: boolean;
  label: string;
  selected: boolean;
  value: string;
}

interface AndroidViewportState {
  keyboardInset: number;
  keyboardOpen: boolean;
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
  { id: "dns", title: "DNS 读取", copy: "读取当前域名与子域名记录", shortLabel: "DNS" },
];

const routeTitles: Record<ViewName, string> = {
  dashboard: "任务看板",
  dns: "DNS 记录读取",
  results: "当前测速结果",
  settings: "系统配置",
  sources: "输入源管理",
};
const RESULT_CLOUDFLARE_PUSH_SETTINGS_KEY = "cfst.result.cloudflarePushSettings.v1";
const RESULT_GITHUB_TOP_N_KEY = "cfst.result.githubTopN.v1";
const EXPORT_HISTORY_LIMIT = 50;

const selectedView = ref<ViewName>("dashboard");
const appMode = ref<AppMode>("single");
const activityFeed = ref<Array<{ detail: string; title: string; ts: string }>>([]);
const configPath = ref("");
const dnsReadName = ref("");
const dnsReadScope = ref<"zone" | "configured" | "custom">("zone");
const dnsRecordType = ref<"all" | "A" | "AAAA">("all");
const dnsRecords = ref<DnsRecordSnapshot[]>([]);
const exportHistory = ref<HistoryEntry[]>([]);
const isLoadingDns = ref(false);
const loading = ref(false);
const logs = ref<Array<{ event: string; payload: unknown; ts: string }>>([]);
const maskedTokenHint = ref("");
const processTrace = ref<ProcessTraceEntry[]>([]);
const workflowFitRequestKey = ref(0);
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
const taskResultCSVPath = ref("");
const resultWorkspaceTaskId = ref("");
const resultSortBy = ref<ProbeResultSortBy>("address");
const resultsLoading = ref(false);
const resultsPageLimit = ref(200);
const resultsTotalCount = ref(0);
const csvExporting = ref(false);
const cloudflarePushing = ref(false);
const githubExporting = ref(false);
const githubTesting = ref(false);
const telegramTesting = ref(false);
const resultGitHubTopN = ref(20);
const resultCloudflarePushSettings = reactive<ResultCloudflarePushSettings>({
  recordName: "",
  recordType: "ALL",
  topN: 5,
});
const showToken = ref(false);
const viewportAdaptiveActive = ref(false);
const viewportSwitching = ref(false);
const sourceSeed = ref(0);
const sourcePreviewStates = reactive<Record<string, SourcePreviewState>>({});
const sourceRequestStates = reactive<Record<string, string>>({});
const coloDictionaryStatus = ref<ColoDictionaryStatus | null>(null);
const coloDictionaryProcessing = ref(false);
const coloDictionaryUpdating = ref(false);
const androidBatteryStatus = ref<AndroidBatteryStatus | null>(null);
const androidKeepAliveStatus = ref<AndroidKeepAliveStatus | null>(null);
const androidNotificationStatus = ref<AndroidNotificationPermissionStatus | null>(null);
const taskSnapshot = ref<TaskSnapshot | null>(null);
const taskSessionState = ref("idle");
const schedulerStatus = ref<SchedulerStatus | null>(null);
const toasts = ref<ToastEntry[]>([]);
const storageStatus = ref<StorageStatus | null>(null);
const pipelineWorkspace = ref<PipelineWorkspace>({
  active_target_id: "",
  active_template_id: "",
  schema_version: "",
  targets: [],
  templates: [],
  updated_at: "",
});
const builtInPipelineTemplateIds = new Set(["pipeline-template-default", "pipeline-template-advanced-upload"]);
const pipelineWorkspaceLastSavedSignature = ref(stablePipelineWorkspaceSignature(pipelineWorkspace.value));
const pipelineProfiles = ref<PipelineProfileStore>({
  active_profile_id: "",
  items: [],
  schema_version: "",
  updated_at: "",
});
const pipelineNodeCatalog = ref<PipelineNodeCatalogItem[]>(defaultPipelineNodeCatalog());
const pipelineResults = ref<PipelineRunResult[]>([]);
const activePipelineId = ref("");
const pipelineWorkspaceDirty = computed(() => stablePipelineWorkspaceSignature(pipelineWorkspace.value) !== pipelineWorkspaceLastSavedSignature.value);
const enabledPipelineProfileCount = computed(() => pipelineProfiles.value.items.filter((item) => item.enabled).length);
const workflowSchedulerState = computed(() => ({
  autoDnsPush: settings.schedulerAutoDnsPush,
  dailyTimes: settings.schedulerDailyTimes,
  enabled: settings.schedulerEnabled,
  intervalMinutes: settings.schedulerIntervalMinutes,
  skipIfActive: settings.schedulerSkipIfActive,
  templateId: settings.schedulerPipelineTemplateId || pipelineWorkspace.value.active_template_id || "",
  triggerMode: settings.schedulerTriggerMode,
}));
const sourceProfiles = ref<SourceProfileStore>({
  active_profile_id: "",
  items: [],
  schema_version: "",
  updated_at: "",
});
const appInfo = ref<AppInfo>({
  current_version: "1.0",
  install_mode: "",
  platform: "",
  release_url: "",
});
const isAndroidApp = computed(() => appInfo.value.platform === "android");
const updateState = reactive({
  assetName: "",
  checkedAt: "",
  dockerImage: "",
  downloadPath: "",
  installStarted: false,
  installMode: "",
  installing: false,
  latestVersion: "",
  message: "尚未检查更新。",
  nextAction: "",
  releaseUrl: "",
  status: "idle" as "idle" | "checking" | "available" | "latest" | "installing" | "ready" | "failed",
  updateAvailable: false,
});
const androidSelectPicker = reactive({
  open: false,
  options: [] as AndroidSelectOption[],
  title: "选择",
  value: "",
});
const viewportSize = reactive<ViewportSize>({
  cssHeight: 0,
  cssWidth: 0,
  height: 0,
  updatedAt: "",
  width: 0,
});
let viewportResizeTimer: number | undefined;
let androidViewportFrame: number | undefined;
let androidViewportTrackingInstalled = false;
let androidNotificationSettingsRefreshPending = false;
let androidNotificationSettingsRefreshInFlight = false;
let androidViewportState: AndroidViewportState | null = null;
let androidSelectElement: HTMLSelectElement | null = null;
const androidSelectCaptureOptions = { capture: true, passive: false } as const;

const sources = ref<SourceDraft[]>([createSourceDraft()]);

const status = reactive({
  detail: "先读取配置，再决定启动探测任务或读取 DNS 记录。",
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
  cloudflareEnabled: false,
  postProbePushCloudflareEnabled: false,
  postProbePushGitHubEnabled: false,
  telegramBotToken: "",
  telegramChatId: "",
  telegramIncludeTopN: false,
  telegramNotificationEnabled: false,
  telegramPersonalChatId: "",
  telegramTopNRecipientMode: "chat",
  telegramTopN: 5,
  telegramUploadRecipientMode: "chat",
  uploadCloudflareRoutingEnabled: false,
  uploadCloudflareRoutingRules: [],
  uploadCloudflareTopN: 5,
  uploadGitHubTopN: 20,
  uploadSharedFilterColoAllow: "",
  uploadSharedFilterColoDeny: "",
  uploadSharedFilterEnabled: false,
  uploadSharedFilterIPVersion: "any",
  uploadSharedFilterMaxLossRate: null,
  uploadSharedFilterMaxTcpLatencyMs: null,
  uploadSharedFilterMaxTraceLatencyMs: null,
  uploadSharedFilterMinDownloadMbps: 0,
  uploadSharedFilterStatus: "passed",
  exportFileName: "",
  exportFileNameTemplate: "",
  githubBranch: "main",
  githubCSVHeaderTemplate: "",
  githubCSVRowTemplate: "",
  githubCommitMessageTemplate: "CFST results {date} {time}",
  githubExportEnabled: false,
  githubFormat: "csv",
  githubLastExportAt: "",
  githubOwner: "axuitomo",
  githubPathTemplate: "cfst-results/{date}/{time}-{task_id}.csv",
  githubRepo: "CFST-GUI",
  githubToken: "",
  githubTXTRowTemplate: "{ip}",
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
  probeDownloadTimeSeconds: 4,
  probeDownloadWarmupSeconds: 1,
  probeEventThrottleMs: 100,
  probeHostHeader: "",
  probeHttping: false,
  probeHttpingCfColo: "",
  probeHttpingCfColoMode: "allow",
  probeHttpingStatusCode: DEFAULT_HTTPING_STATUS_CODE,
  probePingTimes: 4,
  probePrintNum: 0,
  probePortPolicy: "source_override_global",
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
  schedulerIntervalMinutesDraft: 60,
  schedulerPipelineTemplateId: "",
  schedulerRunMode: "probe",
  schedulerSkipIfActive: true,
  schedulerTriggerMode: "interval",
  maintenanceCompletedTaskRetentionDays: 7,
  sourceAutoDetectName: true,
  themeDarkStart: "19:00",
  themeLightStart: "07:00",
  themeMode: "auto_system_time",
  utcOffsetMinutes: DEFAULT_UTC_OFFSET_MINUTES,
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

const activeCloudflareRoutingRuleCount = computed(() => {
  if (!settings.uploadCloudflareRoutingEnabled) {
    return 0;
  }
  return settings.uploadCloudflareRoutingRules.filter((rule) => Boolean(rule.enabled) && rule.recordName.trim()).length;
});
const cloudflareRoutingPushActive = computed(() => activeCloudflareRoutingRuleCount.value > 0);

let removeProbeListener: (() => void) | null = null;
let processTraceId = 0;
let snapshotRefreshInFlight = false;
let snapshotRefreshPending = false;
let snapshotRefreshQueuedTaskId = "";
let toastId = 0;
let draftSaveTimer: number | undefined;
let configHydrated = false;
let draftRestoring = false;
let configSaveInFlight: Promise<boolean> | null = null;
let resultCloudflarePushSettingsHydrated = false;
let lastDraftSnapshotSignature = "";
let lastSavedSnapshotSignature = "";
let lastSavedSourceProfileSignature = "";
let lastSavedSourceSignature = "";
let lastSettingsAutoSaveSkippedSignature = "";
let lastSourceAutoSaveSkippedSignature = "";
let sourceAutoSaveInFlight: Promise<boolean> | null = null;
let sourceLeaveSaveInFlight = false;
let sourceLeaveSaveTarget: { mode: AppMode; view: ViewName } | null = null;
let themeMediaQuery: MediaQueryList | null = null;
let themeTimer: number | undefined;

type TaskActionKind = "cancel" | "pause" | "rerun" | "resume" | "start";

const taskActionState = reactive<{
  kind: TaskActionKind | "";
  taskId: string;
  target: string;
}>({
  kind: "",
  target: "",
  taskId: "",
});

function handleBeforeUnload() {
  void autoSaveSettings("beforeunload");
  void autoSaveSourcePage("beforeunload");
  void flushDraftSave();
}

const dashboardStatusLabel = computed(
  () =>
    (
      ({
        completed: "已完成",
        cooling: "冷却中",
        failed: "失败",
        idle: "就绪",
        no_results: "无结果",
        partial: "部分完成",
        preparing: "准备中",
        running: "运行中",
        warning: "警告",
      }) as Record<TaskTone, string>
    )[status.tone] || status.title,
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
  })),
);
const preparedSources = computed(() => sourcePayloads.value.filter((source) => source.enabled && hasUsableSourceInput(source)));
const progressPercent = computed(() => {
  const total = summary.total > 0 ? summary.total : summary.accepted + summary.filtered + summary.invalid > 0 ? summary.accepted + summary.filtered + summary.invalid : summary.accepted;

  if (total <= 0) {
    return 0;
  }

  return Math.max(0, Math.min(100, Math.round((summary.processed / total) * 100)));
});
const hasActiveTask = computed(() => Boolean(task.taskId) && task.active);
const activeTaskSessionState = computed(() => {
  const snapshotState = asString(taskSnapshot.value?.session_state || "").trim();
  const runtimeState = asString(taskSessionState.value || "").trim();
  if (runtimeState === "active_runtime" || runtimeState === "paused_runtime") {
    return runtimeState;
  }
  return snapshotState || runtimeState || "idle";
});
const taskActionInFlight = computed(() => Boolean(taskActionState.kind));
const hasDetachedTaskSnapshot = computed(() => activeTaskSessionState.value === "persisted_only");
const hasPausedTask = computed(() => activeTaskSessionState.value === "paused_runtime");
const canPauseTask = computed(() => hasActiveTask.value && !taskActionInFlight.value && !hasPausedTask.value && task.stage !== "accepted");
const canResumeTask = computed(() => Boolean(task.taskId) && !taskActionInFlight.value && !hasDetachedTaskSnapshot.value && (taskSnapshot.value?.resume_capable === true || hasPausedTask.value));
const canStartTask = computed(() => !taskActionInFlight.value && (!hasActiveTask.value || hasPausedTask.value));
const canStartPipeline = computed(() => !taskActionInFlight.value && !activePipelineId.value && (!hasActiveTask.value || hasPausedTask.value));
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

function sourceDraftsFromProfileSources(value: unknown): SourceDraft[] {
  const nextSources = Array.isArray(value) ? value : [];
  return normalizeConfigSnapshot({ sources: nextSources }).sources.map((source) => ({ ...source }));
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
  const bridge = runtimeWindow.go?.app?.App ?? runtimeWindow.go?.main?.App;
  return Boolean(runtimeWindow.runtime && bridge);
}

function applyViewportSize(runtimeSize?: { h: number; w: number }) {
  const cssWidth = typeof window === "undefined" ? 0 : Math.round(window.innerWidth);
  const cssHeight = typeof window === "undefined" ? 0 : Math.round(window.innerHeight);
  viewportSize.cssWidth = cssWidth;
  viewportSize.cssHeight = cssHeight;
  viewportSize.width = runtimeSize ? Math.round(runtimeSize.w) : cssWidth;
  viewportSize.height = runtimeSize ? Math.round(runtimeSize.h) : cssHeight;
  viewportSize.updatedAt = new Date().toISOString();
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

function applyAndroidViewportState() {
  if (typeof window === "undefined" || !isAndroidApp.value) {
    return;
  }
  const visualViewport = window.visualViewport;
  const viewportHeight = visualViewport?.height && Number.isFinite(visualViewport.height) ? visualViewport.height : window.innerHeight;
  const viewportTop = visualViewport?.offsetTop && Number.isFinite(visualViewport.offsetTop) ? visualViewport.offsetTop : 0;
  const keyboardInset = Math.max(0, Math.round(window.innerHeight - viewportHeight - viewportTop));
  const nextState = {
    keyboardInset,
    keyboardOpen: keyboardInset > 48,
  };
  document.documentElement.dataset.cfstAndroid = "true";
  if (androidViewportState?.keyboardInset !== nextState.keyboardInset) {
    document.documentElement.style.setProperty("--cfst-keyboard-inset-bottom", `${nextState.keyboardInset}px`);
  }
  if (androidViewportState?.keyboardOpen !== nextState.keyboardOpen) {
    document.documentElement.dataset.cfstAndroidKeyboard = nextState.keyboardOpen ? "open" : "closed";
  }
  androidViewportState = nextState;
}

function closeAndroidSelectPicker() {
  androidSelectPicker.open = false;
  androidSelectPicker.options = [];
  androidSelectPicker.value = "";
  androidSelectElement = null;
}

function androidSelectTitle(selectElement: HTMLSelectElement) {
  const labelElement = selectElement.closest("label");
  const labelText = labelElement?.querySelector(".ui-label")?.textContent || "";
  return labelText.trim() || selectElement.getAttribute("aria-label") || selectElement.name || "选择";
}

function openAndroidSelectPicker(selectElement: HTMLSelectElement) {
  if (selectElement.disabled) {
    return;
  }
  androidSelectElement = selectElement;
  androidSelectPicker.title = androidSelectTitle(selectElement);
  androidSelectPicker.value = selectElement.value;
  androidSelectPicker.options = Array.from(selectElement.options).map((option) => ({
    disabled: option.disabled,
    label: option.label || option.textContent?.trim() || option.value,
    selected: option.selected,
    value: option.value,
  }));
  androidSelectPicker.open = true;
}

function handleAndroidSelectPointerDown(event: PointerEvent) {
  if (!isAndroidApp.value) {
    return;
  }
  const target = event.target;
  if (!(target instanceof Element)) {
    return;
  }
  const selectElement = target.closest("select");
  if (!(selectElement instanceof HTMLSelectElement)) {
    return;
  }
  event.preventDefault();
  event.stopPropagation();
  selectElement.blur();
  openAndroidSelectPicker(selectElement);
}

function handleAndroidSelectTouchStart(event: TouchEvent) {
  if (!isAndroidApp.value) {
    return;
  }
  const target = event.target;
  if (!(target instanceof Element)) {
    return;
  }
  const selectElement = target.closest("select");
  if (!(selectElement instanceof HTMLSelectElement)) {
    return;
  }
  event.preventDefault();
  event.stopPropagation();
  selectElement.blur();
  openAndroidSelectPicker(selectElement);
}

function handleAndroidSelectClick(event: MouseEvent) {
  if (!isAndroidApp.value) {
    return;
  }
  const target = event.target;
  if (!(target instanceof Element)) {
    return;
  }
  const selectElement = target.closest("select");
  if (!(selectElement instanceof HTMLSelectElement)) {
    return;
  }
  event.preventDefault();
  event.stopPropagation();
  openAndroidSelectPicker(selectElement);
}

function chooseAndroidSelectOption(option: AndroidSelectOption) {
  const selectElement = androidSelectElement;
  if (!selectElement || option.disabled) {
    return;
  }
  selectElement.value = option.value;
  selectElement.dispatchEvent(new Event("input", { bubbles: true }));
  selectElement.dispatchEvent(new Event("change", { bubbles: true }));
  closeAndroidSelectPicker();
}

function handleAndroidSelectKeydown(event: KeyboardEvent) {
  if (event.key === "Escape" && androidSelectPicker.open) {
    closeAndroidSelectPicker();
  }
}

function scheduleAndroidViewportState() {
  if (androidViewportFrame !== undefined) {
    window.cancelAnimationFrame(androidViewportFrame);
  }
  androidViewportFrame = window.requestAnimationFrame(() => {
    androidViewportFrame = undefined;
    applyAndroidViewportState();
  });
}

function handleAndroidControlFocus() {
  scheduleAndroidViewportState();
}

function installAndroidViewportTracking() {
  if (androidViewportTrackingInstalled || typeof window === "undefined") {
    return;
  }
  androidViewportTrackingInstalled = true;
  applyAndroidViewportState();
  window.addEventListener("resize", scheduleAndroidViewportState);
  window.addEventListener("focusin", handleAndroidControlFocus);
  window.addEventListener("focusout", scheduleAndroidViewportState);
  document.addEventListener("touchstart", handleAndroidSelectTouchStart, androidSelectCaptureOptions);
  document.addEventListener("pointerdown", handleAndroidSelectPointerDown, true);
  document.addEventListener("click", handleAndroidSelectClick, true);
  document.addEventListener("keydown", handleAndroidSelectKeydown);
  window.visualViewport?.addEventListener("resize", scheduleAndroidViewportState);
}

function uninstallAndroidViewportTracking() {
  if (!androidViewportTrackingInstalled || typeof window === "undefined") {
    return;
  }
  androidViewportTrackingInstalled = false;
  window.removeEventListener("resize", scheduleAndroidViewportState);
  window.removeEventListener("focusin", handleAndroidControlFocus);
  window.removeEventListener("focusout", scheduleAndroidViewportState);
  document.removeEventListener("touchstart", handleAndroidSelectTouchStart, androidSelectCaptureOptions);
  document.removeEventListener("pointerdown", handleAndroidSelectPointerDown, true);
  document.removeEventListener("click", handleAndroidSelectClick, true);
  document.removeEventListener("keydown", handleAndroidSelectKeydown);
  window.visualViewport?.removeEventListener("resize", scheduleAndroidViewportState);
  if (androidViewportFrame !== undefined) {
    window.cancelAnimationFrame(androidViewportFrame);
    androidViewportFrame = undefined;
  }
  document.documentElement.style.removeProperty("--cfst-keyboard-inset-bottom");
  androidViewportState = null;
  delete document.documentElement.dataset.cfstAndroid;
  delete document.documentElement.dataset.cfstAndroidKeyboard;
  closeAndroidSelectPicker();
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
    backend: (asString(source.backend) || undefined) as StorageStatus["backend"],
    bootstrap_path: asString(source.bootstrap_path || source.bootstrapPath),
    current_dir: asString(source.current_dir || source.currentDir),
    default_dir: asString(source.default_dir || source.defaultDir),
    display_name: asString(source.display_name || source.displayName),
    health: asRecord(source.health) as unknown as StorageStatus["health"],
    last_sync_at: asString(source.last_sync_at || source.lastSyncAt),
    last_sync_error: asString(source.last_sync_error || source.lastSyncError),
    log_uri: asString(source.log_uri || source.logUri),
    permission_ok: source.permission_ok !== false && source.permissionOk !== false,
    portable_mode: Boolean(source.portable_mode || source.portableMode),
    runtime_dir: asString(source.runtime_dir || source.runtimeDir),
    setup_completed: Boolean(source.setup_completed || source.setupCompleted),
    setup_required: Boolean(source.setup_required || source.setupRequired),
    storage_uri: asString(source.storage_uri || source.storageUri),
    writable: source.writable !== false,
  };
}

function applyPipelineProfileStore(value: unknown) {
  pipelineProfiles.value = normalizePipelineProfileStore(value);
}

function applyPipelineWorkspace(value: unknown) {
  pipelineWorkspace.value = normalizePipelineWorkspace(value);
  pipelineWorkspaceLastSavedSignature.value = stablePipelineWorkspaceSignature(pipelineWorkspace.value);
  pipelineProfiles.value = pipelineProfileStoreFromWorkspace(pipelineWorkspace.value);
}

function applyPipelineResults(value: unknown) {
  pipelineResults.value = normalizePipelineRunResults(value)
    .sort((left, right) => (right.started_at || "").localeCompare(left.started_at || ""))
    .slice(0, 1);
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
  updateState.dockerImage = asString(source.docker_image || source.dockerImage || updateState.dockerImage);
  updateState.installStarted = asBoolean(source.install_started || source.installStarted, false);
  updateState.installMode = asString(source.install_mode || source.installMode);
  updateState.latestVersion = asString(source.latest_version || source.latestVersion);
  updateState.nextAction = asString(source.next_action || source.nextAction);
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

function schedulerTriggerModeFromSnapshot(scheduler: ConfigSnapshot["scheduler"], fallback: SchedulerTriggerMode): SchedulerTriggerMode {
  if (scheduler.daily_times.length > 0) {
    return "daily";
  }
  if (scheduler.interval_minutes > 0) {
    return "interval";
  }
  return fallback;
}

function schedulerDailyTimesFromText(value: string) {
  return value
    .split(/[,\s;，；、]+/)
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function normalizeResultCloudflareRecordType(value: unknown): ResultCloudflareRecordType {
  const normalized = asString(value).trim().toUpperCase();
  if (normalized === "A" || normalized === "AAAA") {
    return normalized;
  }
  return "ALL";
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
    stage1_tcp: "第一阶段",
    stage2_head: "第二阶段",
    stage2_trace: "第二阶段",
    stage3_get: "第三阶段",
    post_probe_push: "自动推送",
  };

  return labels[stage] || stage || "探测";
}

function pipelineTraceTargetLabel(payload: Record<string, unknown>, fallback = "当前目标") {
  const profileName = asString(payload.profile_name || payload.pipeline_profile_name).trim();
  const region = asString(payload.region || payload.pipeline_region).trim();
  const domain = asString(payload.domain || payload.pipeline_domain).trim();
  return [profileName || fallback, region, domain].filter(Boolean).join(" / ");
}

function pipelineTraceNodeLabel(payload: Record<string, unknown>) {
  return asString(payload.node_name).trim() || asString(payload.action).trim() || asString(payload.node_type).trim() || asString(payload.node_id).trim() || "当前节点";
}

function pipelineTraceNodeStatusLabel(status: string) {
  const labels: Record<string, string> = {
    completed: "完成",
    failed: "失败",
    manual_review: "等待复核",
    partial: "部分完成",
    skipped: "跳过",
  };
  return labels[status] || status || "完成";
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

function formatAppTimestamp(value: string, options?: Parameters<typeof formatTimestampWithUTCOffset>[2]) {
  return formatTimestampWithUTCOffset(value, settings.utcOffsetMinutes, options);
}

function utcOffsetLabel() {
  return formatUTCOffsetLabel(settings.utcOffsetMinutes);
}

function appendLog(event: string, payload: unknown) {
  logs.value = [...logs.value, { event, payload, ts: new Date().toISOString() }].slice(-160);
}

function storageContextDetail(storage: StorageStatus | null | undefined) {
  if (!storage) {
    return "";
  }
  const details: string[] = [];
  const current = storage.current_dir?.trim() || "";
  const runtime = storage.runtime_dir?.trim() || "";

  if (current) {
    details.push(`应用数据目录：${current}`);
  }

  if (runtime && runtime !== current) {
    details.push(`运行时目录：${runtime}`);
  }

  return details.join("；");
}

function statusDetailWithStorage(message: string, storage: StorageStatus | null | undefined) {
  return [message.trim(), storageContextDetail(storage)].filter(Boolean).join(" ");
}

function storageStatusTone(storage: StorageStatus | null | undefined, fallback: TaskTone = "idle"): TaskTone {
  if (!storage) {
    return fallback;
  }
  if (storage.permission_ok === false || storage.writable === false) {
    return "failed";
  }
  if ((storage.last_sync_error || "").trim()) {
    return "warning";
  }
  return fallback;
}

function commandDiagnosticPayload(result: { code?: string; data?: unknown; message?: string; warnings?: string[] }, fallbackConfigPath = configPath.value) {
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

function notifyActiveProbeBlocked(title: string) {
  pushActivity(title, ACTIVE_PROBE_MESSAGE);
  showToast("已有任务运行中", "error");
  void navigateTo({ mode: "single", view: "dashboard" });
}

function taskActionLabel(kind: TaskActionKind) {
  return (
    {
      cancel: "终止",
      pause: "暂停",
      rerun: "重测",
      resume: "继续",
      start: "启动",
    } as Record<TaskActionKind, string>
  )[kind];
}

function beginTaskAction(kind: TaskActionKind, target = "", taskId = task.taskId) {
  taskActionState.kind = kind;
  taskActionState.target = target;
  taskActionState.taskId = taskId.trim();
}

function finishTaskAction(kind?: TaskActionKind) {
  if (kind && taskActionState.kind && taskActionState.kind !== kind) {
    return;
  }
  taskActionState.kind = "";
  taskActionState.target = "";
  taskActionState.taskId = "";
}

function notifyTaskActionBlocked(kind: TaskActionKind) {
  const actionLabel = taskActionLabel(kind);
  const blockingLabel = taskActionState.kind ? taskActionLabel(taskActionState.kind as TaskActionKind) : "任务";
  const detail = `${blockingLabel}操作仍在处理中，请等待当前请求完成后再${actionLabel}任务。`;
  setStatus({
    detail,
    title: `${actionLabel}被拦截`,
    tone: "warning",
  });
  pushActivity(`${actionLabel}被拦截`, detail);
  showToast(`请勿重复${actionLabel}`, "info");
  void navigateTo({ mode: "single", view: "dashboard" });
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

  exportHistory.value = nextEntries.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt)).slice(0, EXPORT_HISTORY_LIMIT);
}

function clearCurrentTaskResultWorkspace(options: { preserveSnapshot?: boolean } = {}) {
  clearTaskWorkspaceCache();
  resultRows.value = [];
  taskResultCSVPath.value = "";
  resultsTotalCount.value = 0;
  resultWorkspaceTaskId.value = "";
  if (!options.preserveSnapshot) {
    taskSnapshot.value = null;
  }
}

function applyCurrentTaskResultWorkspace(taskId: string, rows: ProbeResult[], totalCount: number) {
  resultWorkspaceTaskId.value = taskId.trim();
  resultRows.value = rows;
  resultsTotalCount.value = asCount(totalCount, rows.length);
}

function resetCurrentResultPage() {
  resultRows.value = [];
  resultsTotalCount.value = 0;
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
      .filter((entry): entry is readonly [string, { last_fetched_at: string; last_fetched_count: number; status_text: string }] => Boolean(entry)),
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
    portSummary: Object.keys(asRecord(data.port_summary)).length > 0 ? asRecord(data.port_summary) : null,
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
  settings.cloudflareEnabled = Boolean(normalized.cloudflare.enabled);
  settings.postProbePushCloudflareEnabled = Boolean(normalized.post_probe_push.cloudflare_enabled);
  settings.postProbePushGitHubEnabled = Boolean(normalized.post_probe_push.github_enabled);
  settings.telegramBotToken = normalized.notifications.telegram.bot_token;
  settings.telegramChatId = normalized.notifications.telegram.chat_id;
  settings.telegramIncludeTopN = Boolean(normalized.notifications.telegram.include_top_n);
  settings.telegramNotificationEnabled = Boolean(normalized.notifications.telegram.enabled);
  settings.telegramPersonalChatId = normalized.notifications.telegram.personal_chat_id;
  settings.telegramTopNRecipientMode = normalized.notifications.telegram.top_n_recipient_mode;
  settings.telegramTopN = nonNegativeCount(normalized.notifications.telegram.top_n, 5);
  settings.telegramUploadRecipientMode = normalized.notifications.telegram.upload_recipient_mode || normalized.notifications.telegram.recipient_mode;
  settings.uploadCloudflareRoutingEnabled = Boolean(normalized.cloudflare.routing_enabled);
  settings.uploadCloudflareRoutingRules = normalized.cloudflare.routing_rules.map((rule, index) => ({
    enabled: rule.enabled,
    filterMode: rule.filter_mode,
    filterTokens: rule.filter_tokens,
    id: `cf-route-${index}-${Date.now()}`,
    name: rule.name,
    recordName: rule.record_name,
    recordType: rule.record_type,
    topN: rule.top_n,
  }));
  settings.uploadCloudflareTopN = normalized.cloudflare.top_n;
  settings.uploadGitHubTopN = normalized.github.top_n ?? 20;
  settings.uploadSharedFilterColoAllow = normalized.upload.shared_filter.colo_allow || "";
  settings.uploadSharedFilterColoDeny = normalized.upload.shared_filter.colo_deny || "";
  settings.uploadSharedFilterEnabled = Boolean(normalized.upload.shared_filter.enabled);
  settings.uploadSharedFilterIPVersion = normalized.upload.shared_filter.ip_version;
  settings.uploadSharedFilterMaxLossRate = asNullableNumber(normalized.upload.shared_filter.max_loss_rate);
  settings.uploadSharedFilterMaxTcpLatencyMs = asNullableNumber(normalized.upload.shared_filter.max_tcp_latency_ms);
  settings.uploadSharedFilterMaxTraceLatencyMs = asNullableNumber(normalized.upload.shared_filter.max_trace_latency_ms);
  settings.uploadSharedFilterMinDownloadMbps = asNumber(normalized.upload.shared_filter.min_download_mbps, 0);
  settings.uploadSharedFilterStatus = normalized.upload.shared_filter.status;
  settings.exportFileName = normalized.export.file_name || "";
  settings.exportFileNameTemplate = normalized.export.file_name_template || "";
  settings.githubBranch = normalized.github.branch || "main";
  settings.githubCSVHeaderTemplate = normalized.github.csv_header_template || "";
  settings.githubCSVRowTemplate = normalized.github.csv_row_template || "";
  settings.githubCommitMessageTemplate = normalized.github.commit_message_template || "CFST results {date} {time}";
  settings.githubExportEnabled = Boolean(normalized.github.enabled);
  settings.githubFormat = normalized.github.format === "txt" ? "txt" : "csv";
  settings.githubLastExportAt = normalized.github.last_export_at || "";
  settings.githubOwner = normalized.github.owner || "axuitomo";
  settings.githubPathTemplate = normalized.github.path_template || "cfst-results/{date}/{time}-{task_id}.csv";
  settings.githubRepo = normalized.github.repo || "CFST-GUI";
  settings.githubToken = normalized.github.token || "";
  settings.githubTXTRowTemplate = normalized.github.txt_row_template || "{ip}";
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
  settings.probePortPolicy = normalized.probe.port_policy === "fixed_global" ? "fixed_global" : "source_override_global";
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
  settings.proxied = false;
  settings.recordName = normalized.cloudflare.record_name || "";
  settings.maintenanceCompletedTaskRetentionDays = nonNegativeCount(normalized.maintenance.completed_task_retention_days, 7);
  settings.schedulerAutoDnsPush = normalized.scheduler.auto_dns_push;
  settings.schedulerAutoGithubExport = normalized.scheduler.auto_github_export;
  settings.schedulerDailyTimes = normalized.scheduler.daily_times.join("\n");
  settings.schedulerEnabled = normalized.scheduler.enabled;
  settings.schedulerIntervalMinutes = normalized.scheduler.interval_minutes;
  if (normalized.scheduler.interval_minutes > 0) {
    settings.schedulerIntervalMinutesDraft = normalized.scheduler.interval_minutes;
  } else if (!Number.isFinite(settings.schedulerIntervalMinutesDraft) || settings.schedulerIntervalMinutesDraft <= 0) {
    settings.schedulerIntervalMinutesDraft = 60;
  }
  settings.schedulerPipelineTemplateId = isAndroidApp.value ? "" : normalized.scheduler.pipeline_template_id || "";
  settings.schedulerRunMode = isAndroidApp.value ? "probe" : normalized.scheduler.run_mode;
  settings.schedulerSkipIfActive = normalized.scheduler.skip_if_active;
  settings.schedulerTriggerMode = schedulerTriggerModeFromSnapshot(normalized.scheduler, settings.schedulerTriggerMode);
  settings.sourceAutoDetectName = normalized.ui.auto_detect_source_name;
  settings.themeDarkStart = normalized.ui.theme_dark_start || "19:00";
  settings.themeLightStart = normalized.ui.theme_light_start || "07:00";
  settings.themeMode = normalized.ui.theme_mode || "auto_system_time";
  settings.utcOffsetMinutes = normalizeUTCOffsetMinutes(normalized.ui.utc_offset_minutes);
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

function applyResultCloudflarePushSettings(input: Partial<ResultCloudflarePushSettings> = {}) {
  resultCloudflarePushSettings.recordName = asString(input.recordName).trim() || settings.recordName.trim();
  resultCloudflarePushSettings.recordType = normalizeResultCloudflareRecordType(input.recordType);
  resultCloudflarePushSettings.topN = nonNegativeCount(input.topN, settings.uploadCloudflareTopN);
}

function loadResultCloudflarePushSettings() {
  if (typeof window === "undefined") {
    applyResultCloudflarePushSettings();
    return;
  }
  try {
    const raw = window.localStorage.getItem(RESULT_CLOUDFLARE_PUSH_SETTINGS_KEY);
    applyResultCloudflarePushSettings(raw ? (JSON.parse(raw) as Partial<ResultCloudflarePushSettings>) : {});
  } catch (error) {
    appendLog("result.cloudflare_push_settings.load_failed", error instanceof Error ? error.message : String(error));
    applyResultCloudflarePushSettings();
  }
}

function saveResultCloudflarePushSettings() {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(
      RESULT_CLOUDFLARE_PUSH_SETTINGS_KEY,
      JSON.stringify({
        recordName: resultCloudflarePushSettings.recordName.trim(),
        recordType: normalizeResultCloudflareRecordType(resultCloudflarePushSettings.recordType),
        topN: nonNegativeCount(resultCloudflarePushSettings.topN, 0),
      }),
    );
  } catch (error) {
    appendLog("result.cloudflare_push_settings.save_failed", error instanceof Error ? error.message : String(error));
  }
}

function updateResultCloudflarePushSettings(next: Partial<ResultCloudflarePushSettings>) {
  if ("recordName" in next) {
    resultCloudflarePushSettings.recordName = asString(next.recordName).trim();
  }
  if ("recordType" in next) {
    resultCloudflarePushSettings.recordType = normalizeResultCloudflareRecordType(next.recordType);
  }
  if ("topN" in next) {
    resultCloudflarePushSettings.topN = nonNegativeCount(next.topN, 0);
  }
}

function loadResultGitHubTopN() {
  if (typeof window === "undefined") {
    resultGitHubTopN.value = nonNegativeCount(settings.uploadGitHubTopN, 20);
    return;
  }
  try {
    const raw = window.localStorage.getItem(RESULT_GITHUB_TOP_N_KEY);
    resultGitHubTopN.value = raw === null ? nonNegativeCount(settings.uploadGitHubTopN, 20) : nonNegativeCount(JSON.parse(raw), settings.uploadGitHubTopN);
  } catch (error) {
    appendLog("result.github_top_n.load_failed", error instanceof Error ? error.message : String(error));
    resultGitHubTopN.value = nonNegativeCount(settings.uploadGitHubTopN, 20);
  }
}

function saveResultGitHubTopN() {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(RESULT_GITHUB_TOP_N_KEY, JSON.stringify(nonNegativeCount(resultGitHubTopN.value, 0)));
  } catch (error) {
    appendLog("result.github_top_n.save_failed", error instanceof Error ? error.message : String(error));
  }
}

function updateResultGitHubTopN(value: number) {
  resultGitHubTopN.value = nonNegativeCount(value, 0);
  saveResultGitHubTopN();
}

function limitRowsForQuickPush(rows: ProbeResult[], topN: number) {
  const normalizedTopN = nonNegativeCount(topN, 0);
  return normalizedTopN > 0 ? rows.slice(0, normalizedTopN) : rows;
}

function buildConfigSnapshot() {
  const normalizedStrategy: ProbeStrategy = settings.probeStrategy === "full" ? "full" : "fast";
  const normalizedGitHubToken = settings.githubToken.trim();
  const githubProviderEnabled = Boolean(settings.githubOwner.trim() && settings.githubRepo.trim() && settings.githubBranch.trim() && settings.githubPathTemplate.trim() && normalizedGitHubToken && !isMaskedTokenValue(normalizedGitHubToken));
  const cloudflareRoutingRules = settings.uploadCloudflareRoutingRules.map((rule) => ({
    enabled: Boolean(rule.enabled),
    filter_mode: rule.filterMode === "deny" ? "deny" : "allow",
    filter_tokens: rule.filterTokens.trim(),
    name: rule.name.trim(),
    record_name: rule.recordName.trim(),
    record_type: rule.recordType === "ALL" ? "ALL" : rule.recordType === "AAAA" ? "AAAA" : "A",
    top_n: nonNegativeCount(rule.topN, 5),
  }));
  const hasCloudflareRoutingTarget = settings.uploadCloudflareRoutingEnabled && cloudflareRoutingRules.some((rule) => rule.enabled && rule.record_name.trim());
  const cloudflareProviderEnabled = Boolean((settings.apiToken.trim() || maskedTokenHint.value) && settings.zoneId.trim() && (settings.recordName.trim() || hasCloudflareRoutingTarget));
  const githubConfig = {
    branch: settings.githubBranch.trim() || "main",
    csv_header_template: settings.githubCSVHeaderTemplate,
    csv_row_template: settings.githubCSVRowTemplate,
    commit_message_template: settings.githubCommitMessageTemplate.trim() || "CFST results {date} {time}",
    enabled: githubProviderEnabled,
    format: settings.githubFormat,
    last_export_at: settings.githubLastExportAt.trim(),
    owner: settings.githubOwner.trim(),
    path_template: settings.githubPathTemplate.trim() || "cfst-results/{date}/{time}-{task_id}.csv",
    repo: settings.githubRepo.trim(),
    token: normalizedGitHubToken,
    top_n: nonNegativeCount(settings.uploadGitHubTopN, 20),
    txt_row_template: settings.githubTXTRowTemplate || "{ip}",
  };

  return {
    cloudflare: {
      ...(settings.apiToken.trim() ? { api_token: settings.apiToken.trim() } : {}),
      comment: settings.comment.trim(),
      enabled: cloudflareProviderEnabled,
      proxied: false,
      record_name: settings.recordName.trim(),
      routing_enabled: settings.uploadCloudflareRoutingEnabled,
      routing_rules: cloudflareRoutingRules,
      top_n: nonNegativeCount(settings.uploadCloudflareTopN, 5),
      ttl: normalizeCloudflareTTL(settings.ttl),
      zone_id: settings.zoneId.trim(),
    },
    github: githubConfig,
    post_probe_push: {
      cloudflare_enabled: settings.postProbePushCloudflareEnabled,
      github_enabled: settings.postProbePushGitHubEnabled,
    },
    upload: {
      cloudflare: {
        routing_enabled: settings.uploadCloudflareRoutingEnabled,
        routing_rules: cloudflareRoutingRules,
        top_n: nonNegativeCount(settings.uploadCloudflareTopN, 5),
      },
      github: {
        top_n: nonNegativeCount(settings.uploadGitHubTopN, 20),
      },
      shared_filter: {
        colo_allow: settings.uploadSharedFilterColoAllow.trim(),
        colo_deny: settings.uploadSharedFilterColoDeny.trim(),
        enabled: settings.uploadSharedFilterEnabled,
        ip_version: settings.uploadSharedFilterIPVersion,
        max_loss_rate: optionalNumberForPayload(settings.uploadSharedFilterMaxLossRate),
        max_tcp_latency_ms: optionalNumberForPayload(settings.uploadSharedFilterMaxTcpLatencyMs),
        max_trace_latency_ms: optionalNumberForPayload(settings.uploadSharedFilterMaxTraceLatencyMs),
        min_download_mbps: nonNegativeNumber(settings.uploadSharedFilterMinDownloadMbps, 0),
        status: settings.uploadSharedFilterStatus,
      },
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
      github: githubConfig,
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
      debug_log_format: settings.probeDebugLogMode === "freeform" ? settings.probeDebugLogFormat.trim() || DEFAULT_DEBUG_LOG_FORMAT : "",
      debug_log_mode: settings.probeDebugLogMode === "freeform" ? "freeform" : "structured",
      debug_log_verbosity: settings.probeDebugLogVerbosity === "simple" ? "simple" : "detailed",
      disable_download: normalizedStrategy === "fast",
      download_buffer_kb: boundedCount(settings.probeDownloadBufferKB, 256, 64, 4096),
      download_get_concurrency: boundedCount(settings.probeDownloadGetConcurrency, 4, 1, 32),
      download_http_protocol: normalizeDownloadHTTPProtocol(settings.probeDownloadHTTPProtocol),
      download_speed_metric: settings.probeDownloadSpeedMetric === "max" ? "max" : "average",
      download_speed_sample_interval_ms: positiveCount(settings.probeDownloadSpeedSampleIntervalMs, 500),
      download_time_seconds: positiveCount(settings.probeDownloadTimeSeconds, 4),
      download_warmup_seconds: nonNegativeCount(settings.probeDownloadWarmupSeconds, 1),
      event_throttle_ms: positiveCount(settings.probeEventThrottleMs, 100),
      host_header: settings.probeHostHeader.trim(),
      httping: false,
      httping_cf_colo: settings.probeHttpingCfColo.trim(),
      httping_cf_colo_mode: settings.probeHttpingCfColoMode === "deny" ? "deny" : "allow",
      httping_status_code: settings.probeHttpingStatusCode === 0 || (settings.probeHttpingStatusCode >= 100 && settings.probeHttpingStatusCode <= 599) ? settings.probeHttpingStatusCode : DEFAULT_HTTPING_STATUS_CODE,
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
      port_policy: settings.probePortPolicy,
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
        stage3_ms: positiveCount(settings.probeDownloadTimeSeconds, 4) * 1000,
      },
      trace_colo_mode: settings.probeTraceColoMode,
      trace_url: settings.probeTraceURL.trim(),
      url: settings.probeURL.trim(),
      user_agent: settings.probeUserAgent.trim() || DEFAULT_PROBE_USER_AGENT,
    },
    ui: {
      auto_detect_source_name: settings.sourceAutoDetectName,
      theme_dark_start: settings.themeDarkStart.trim() || "19:00",
      theme_light_start: settings.themeLightStart.trim() || "07:00",
      theme_mode: settings.themeMode,
      utc_offset_minutes: normalizeUTCOffsetMinutes(settings.utcOffsetMinutes),
    },
    maintenance: {
      completed_task_retention_days: nonNegativeCount(settings.maintenanceCompletedTaskRetentionDays, 7),
    },
    notifications: {
      telegram: {
        bot_token: settings.telegramBotToken.trim(),
        chat_id: settings.telegramChatId.trim(),
        enabled: settings.telegramNotificationEnabled,
        include_top_n: settings.telegramIncludeTopN,
        personal_chat_id: settings.telegramPersonalChatId.trim(),
        recipient_mode: settings.telegramUploadRecipientMode,
        top_n: positiveCount(settings.telegramTopN, 5, 50),
        top_n_recipient_mode: settings.telegramTopNRecipientMode,
        upload_recipient_mode: settings.telegramUploadRecipientMode,
      },
    },
    scheduler: {
      auto_dns_push: settings.schedulerAutoDnsPush,
      auto_github_export: settings.schedulerRunMode === "pipeline" ? false : settings.schedulerAutoGithubExport,
      config_source: "draft_preferred",
      daily_times: settings.schedulerTriggerMode === "daily" ? schedulerDailyTimesFromText(settings.schedulerDailyTimes) : [],
      enabled: settings.schedulerEnabled,
      interval_minutes: settings.schedulerTriggerMode === "interval" ? (settings.schedulerEnabled ? positiveCount(settings.schedulerIntervalMinutes || settings.schedulerIntervalMinutesDraft, 60) : nonNegativeCount(settings.schedulerIntervalMinutes || settings.schedulerIntervalMinutesDraft, 0)) : 0,
      pipeline_template_id: isAndroidApp.value ? "" : settings.schedulerPipelineTemplateId.trim(),
      post_run_source_profile_action: "update_recent_run_source_profile",
      run_mode: isAndroidApp.value ? "probe" : settings.schedulerRunMode,
      skip_if_active: settings.schedulerSkipIfActive,
    },
    sources: sourcePayloads.value.map((source) => ({
      ...source,
      status_text: source.status_text.trim(),
    })),
  };
}

function stableSnapshotValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((entry) => stableSnapshotValue(entry));
  }
  if (value && typeof value === "object") {
    return Object.keys(value as Record<string, unknown>)
      .sort()
      .reduce<Record<string, unknown>>((acc, key) => {
        acc[key] = stableSnapshotValue((value as Record<string, unknown>)[key]);
        return acc;
      }, {});
  }
  return value;
}

function snapshotSignature(snapshot: unknown) {
  return JSON.stringify(stableSnapshotValue(snapshot));
}

function currentSnapshotSignature() {
  return snapshotSignature(buildConfigSnapshot());
}

function currentSourceSignature() {
  return snapshotSignature(sourcePayloads.value.map((source) => ({ ...source, status_text: source.status_text.trim() })));
}

function currentSourceProfileSignature() {
  return snapshotSignature(sourceProfiles.value);
}

function activeSourceProfileSourceSignature() {
  const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
  if (!active) {
    return snapshotSignature([]);
  }
  return snapshotSignature(
    normalizeConfigSnapshot({ sources: active.sources }).sources.map((source) => ({
      ...source,
      status_text: source.status_text.trim(),
    })),
  );
}

function markSourceSaveBaselines() {
  lastSavedSourceSignature = currentSourceSignature();
  lastSavedSourceProfileSignature = currentSourceProfileSignature();
}

function sourcePageHasUnsavedChanges() {
  return currentSourceSignature() !== lastSavedSourceSignature || currentSourceProfileSignature() !== lastSavedSourceProfileSignature || currentSourceSignature() !== activeSourceProfileSourceSignature();
}

function currentSourceAutoSaveSignature() {
  return snapshotSignature({
    active_sources: currentSourceSignature(),
    source_profiles: currentSourceProfileSignature(),
    saved_sources: lastSavedSourceSignature,
    saved_source_profiles: lastSavedSourceProfileSignature,
  });
}

async function persistSourceConfigQuietly() {
  const saved = await persistConfig({
    redirectOnMaskedToken: false,
    silentFailure: true,
    silentMaskedToken: true,
    silentSuccess: true,
    skipIfUnchanged: true,
  });
  if (saved) {
    markSourceSaveBaselines();
  }
  return saved;
}

async function saveSourcePageQuietly() {
  try {
    if (currentSourceSignature() !== activeSourceProfileSourceSignature()) {
      const profileSaved = await updateActiveSourceProfileQuietly();
      if (!profileSaved) {
        return false;
      }
    }

    if (currentSourceSignature() !== lastSavedSourceSignature || currentSourceProfileSignature() !== lastSavedSourceProfileSignature) {
      return persistSourceConfigQuietly();
    }

    return true;
  } catch (error) {
    appendLog("source.auto_save.failed", error instanceof Error ? error.message : String(error));
    return false;
  }
}

async function autoSaveSourcePage(reason: string) {
  if (!configHydrated || draftRestoring) {
    return true;
  }
  if (sourceAutoSaveInFlight) {
    return sourceAutoSaveInFlight;
  }
  if (!sourcePageHasUnsavedChanges()) {
    lastSourceAutoSaveSkippedSignature = "";
    return true;
  }

  sourceAutoSaveInFlight = (async () => {
    while (sourcePageHasUnsavedChanges()) {
      const saved = await saveSourcePageQuietly();
      if (!saved) {
        const skippedSignature = currentSourceAutoSaveSignature();
        if (skippedSignature !== lastSourceAutoSaveSkippedSignature) {
          appendLog("source.auto_save.skipped", { reason });
          lastSourceAutoSaveSkippedSignature = skippedSignature;
        }
        return false;
      }
    }
    lastSourceAutoSaveSkippedSignature = "";
    return true;
  })();

  try {
    return await sourceAutoSaveInFlight;
  } finally {
    sourceAutoSaveInFlight = null;
  }
}

async function saveSourcePageBeforeLeave() {
  try {
    if (sourceAutoSaveInFlight) {
      await sourceAutoSaveInFlight;
      if (!sourcePageHasUnsavedChanges()) {
        return true;
      }
    }

    if (currentSourceSignature() !== activeSourceProfileSourceSignature()) {
      const profileSaved = await updateActiveSourceProfile();
      if (!profileSaved) {
        return false;
      }
    }

    if (currentSourceSignature() !== lastSavedSourceSignature || currentSourceProfileSignature() !== lastSavedSourceProfileSignature) {
      return persistConfig({ redirectOnMaskedToken: false });
    }

    return true;
  } catch (error) {
    showToast(error instanceof Error ? error.message : "输入源保存失败", "error");
    return false;
  }
}

function applyNavigation(target: { mode?: AppMode; view?: ViewName }) {
  if (target.mode) {
    appMode.value = target.mode;
  }
  if (target.view) {
    selectedView.value = target.view;
  }
}

function shouldSaveSourcesBeforeNavigation(target: { mode: AppMode; view: ViewName }) {
  if (appMode.value !== "single" || selectedView.value !== "sources") {
    return false;
  }
  if (target.mode === "single" && target.view === "sources") {
    return false;
  }
  return sourcePageHasUnsavedChanges();
}

async function navigateTo(target: { mode?: AppMode; view?: ViewName }) {
  const nextTarget = {
    mode: target.mode || appMode.value,
    view: target.view || selectedView.value,
  };
  if (shouldSaveSourcesBeforeNavigation(nextTarget)) {
    sourceLeaveSaveTarget = nextTarget;
    if (sourceLeaveSaveInFlight) {
      return false;
    }

    sourceLeaveSaveInFlight = true;
    try {
      const saved = await saveSourcePageBeforeLeave();
      if (saved && sourceLeaveSaveTarget) {
        applyNavigation(sourceLeaveSaveTarget);
      }
      return saved;
    } finally {
      sourceLeaveSaveInFlight = false;
      sourceLeaveSaveTarget = null;
    }
  }

  if (appMode.value === "single" && selectedView.value === "settings" && (nextTarget.mode !== "single" || nextTarget.view !== "settings")) {
    await autoSaveSettings("navigation");
  }

  applyNavigation(nextTarget);
  return true;
}

function changeSingleView(nextView: ViewName) {
  void navigateTo({ mode: "single", view: nextView });
}

function changeAppMode(nextMode: AppMode) {
  void navigateTo({
    mode: isAndroidApp.value && nextMode === "workflow" ? "single" : nextMode,
  });
}

function parseClockMinutes(value: string, fallback: number) {
  const [hourRaw, minuteRaw] = value.trim().split(":");
  const hour = Number.parseInt(hourRaw || "", 10);
  const minute = Number.parseInt(minuteRaw || "", 10);
  if (!Number.isFinite(hour) || !Number.isFinite(minute) || hour < 0 || hour > 23 || minute < 0 || minute > 59) {
    return fallback;
  }
  return hour * 60 + minute;
}

function timeFallbackTheme() {
  const current = currentMinutesInUTCOffset(settings.utcOffsetMinutes);
  const lightStart = parseClockMinutes(settings.themeLightStart, 7 * 60);
  const darkStart = parseClockMinutes(settings.themeDarkStart, 19 * 60);
  if (lightStart <= darkStart) {
    return current >= darkStart || current < lightStart ? "dark" : "light";
  }
  return current >= darkStart && current < lightStart ? "dark" : "light";
}

function resolvedThemeMode() {
  if (settings.themeMode === "light" || settings.themeMode === "dark") {
    return settings.themeMode;
  }
  if (settings.themeMode === "auto_system_time" && themeMediaQuery) {
    return themeMediaQuery.matches ? "dark" : "light";
  }
  return timeFallbackTheme();
}

function applyThemeMode() {
  const mode = resolvedThemeMode();
  document.documentElement.dataset.theme = mode;
  document.documentElement.classList.toggle("dark", mode === "dark");
}

function scheduleThemeRefresh() {
  applyThemeMode();
  if (themeTimer !== undefined) {
    window.clearTimeout(themeTimer);
  }
  themeTimer = window.setTimeout(scheduleThemeRefresh, 60_000);
}

function scheduleDraftSave() {
  if (!configHydrated || draftRestoring || hasActiveTask.value) {
    return;
  }
  const signature = currentSnapshotSignature();
  if (signature === lastSavedSnapshotSignature || signature === lastDraftSnapshotSignature) {
    return;
  }
  if (draftSaveTimer !== undefined) {
    window.clearTimeout(draftSaveTimer);
  }
  draftSaveTimer = window.setTimeout(() => {
    draftSaveTimer = undefined;
    void saveDraftNow();
  }, 5000);
}

async function saveDraftNow() {
  if (!configHydrated || draftRestoring) {
    return;
  }
  const snapshot = buildConfigSnapshot();
  const signature = snapshotSignature(snapshot);
  if (signature === lastSavedSnapshotSignature || signature === lastDraftSnapshotSignature) {
    return;
  }
  try {
    const result = await saveDesktopDraft({ config_snapshot: snapshot });
    appendLog("bridge.save_desktop_draft", result);
    if (result.ok) {
      lastDraftSnapshotSignature = signature;
    }
  } catch (error) {
    appendLog("bridge.save_desktop_draft.failed", error instanceof Error ? error.message : String(error));
  }
}

async function flushDraftSave() {
  if (draftSaveTimer !== undefined) {
    window.clearTimeout(draftSaveTimer);
    draftSaveTimer = undefined;
  }
  await saveDraftNow();
}

async function maybeRestoreDesktopDraft(statusValue: unknown) {
  const statusRecord = asRecord(statusValue);
  if (!statusRecord.exists || !statusRecord.is_newer_than_saved) {
    return;
  }
  const snapshot = asRecord(statusRecord.config_snapshot);
  if (Object.keys(snapshot).length === 0) {
    return;
  }
  const restore = window.confirm("检测到比正式配置更新的未保存草稿，是否恢复？取消将保留正式配置并丢弃草稿。");
  if (!restore) {
    const discarded = await discardDesktopDraft();
    appendLog("bridge.discard_desktop_draft", discarded);
    if (discarded.ok) {
      lastDraftSnapshotSignature = "";
    }
    return;
  }
  draftRestoring = true;
  applyConfigSnapshot(normalizeConfigSnapshot(snapshot));
  draftRestoring = false;
  lastDraftSnapshotSignature = currentSnapshotSignature();
  pushActivity("草稿已恢复", "已恢复上次未保存的桌面配置草稿。");
  showToast("已恢复未保存草稿", "success");
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

async function refreshPipelineResults(pipelineId = activePipelineId.value) {
  if (isAndroidApp.value) {
    pipelineResults.value = [];
    return;
  }
  try {
    const result = await listPipelineResults(pipelineId ? { pipeline_id: pipelineId } : {});
    appendLog("bridge.list_pipeline_results", result);
    if (result.ok && result.data) {
      applyPipelineResults(result.data);
    }
  } catch (error) {
    appendLog("bridge.list_pipeline_results.failed", error instanceof Error ? error.message : String(error));
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

function pathLeafName(rawPath: string) {
  const normalized = rawPath
    .trim()
    .replace(/[?#].*$/, "")
    .replace(/[\\/]+$/, "");
  if (!normalized) {
    return "";
  }
  const browserDownloadPrefix = "browser-download:";
  const value = normalized.startsWith(browserDownloadPrefix) ? normalized.slice(browserDownloadPrefix.length) : normalized;
  const parts = value.split(/[\\/]/).filter(Boolean);
  return parts.length > 0 ? parts[parts.length - 1] : "";
}

function defaultResultsCSVFileName() {
  return pathLeafName(task.exportPath) || settings.exportFileName.trim() || "result.csv";
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
      current_path: settings.exportTargetUri.trim() || settings.exportTargetDir,
      default_file_name: settings.exportFileName.trim() || "result.csv",
      mode: "export_target",
      title: appInfo.value.platform === "android" ? "选择 Android SAF 导出目录" : "选择导出目录",
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
      showToast("已选择 Android SAF 导出目录", "success");
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
    applyNavigation({ mode: "single", view: "settings" });
    showToast(imported.message || "配置已导入，原配置已备份", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "导入配置失败", "error");
  }
}

async function checkCurrentStorageHealth() {
  try {
    const result = await checkStorageHealth({ path: storageStatus.value?.current_dir || "" });
    appendLog("bridge.storage_health", commandDiagnosticPayload(result));
    if (!result.ok) {
      const message = result.message || "应用数据目录健康检查失败";
      setStatus({
        detail: statusDetailWithStorage(message, storageStatus.value),
        title: "应用数据目录健康检查失败",
        tone: "failed",
      });
      showToast(message, "error");
      return;
    }
    applyStorageStatus(asRecord(asRecord(result.data).storage));
    const health = asRecord(asRecord(result.data).health);
    const message = asString(health.message) || result.message || "应用数据目录健康检查完成";
    setStatus({
      detail: statusDetailWithStorage(message, storageStatus.value),
      title: storageStatusTone(storageStatus.value) === "failed" ? "应用数据目录异常" : storageStatusTone(storageStatus.value) === "warning" ? "存储状态异常" : "应用数据目录已检查",
      tone: storageStatusTone(storageStatus.value),
    });
    showToast(message, storageStatusTone(storageStatus.value) === "failed" ? "error" : storageStatusTone(storageStatus.value) === "warning" ? "info" : "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "应用数据目录健康检查失败", "error");
  }
}

async function openStorageDirectory() {
  const target = appInfo.value.platform === "android" ? "" : storageStatus.value?.current_dir || "";
  if (!target) {
    if (appInfo.value.platform === "android") {
      showToast("Android 应用私有目录不支持直接打开，请使用导出目录导出文件。", "info");
    }
    return;
  }
  try {
    await openPath(target);
  } catch (error) {
    showToast(error instanceof Error ? error.message : "打开应用数据目录失败", "error");
  }
}

async function exportCurrentDebugLog() {
  const targetUri = appInfo.value.platform === "android" ? settings.exportTargetUri.trim() : "";
  if (appInfo.value.platform === "android" && !targetUri) {
    showToast("请先选择 Android SAF 导出目录", "error");
    return;
  }
  try {
    const result = await exportDebugLog({
      config: buildConfigSnapshot(),
      ...(targetUri ? { target_uri: targetUri } : {}),
    });
    appendLog("bridge.export_debug_log", result);
    if (!result.ok) {
      showToast(result.message || "调试日志导出失败", "error");
      return;
    }
    const data = asRecord(result.data);
    const target = asString(data.path || data.target_uri || data.targetUri || data.file_name || data.fileName).trim();
    setStatus({
      detail: target ? `调试日志已导出到 ${target}。` : result.message || "调试日志已导出。",
      title: "调试日志已导出",
      tone: "completed",
    });
    showToast(result.message || "调试日志已导出", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "调试日志导出失败", "error");
  }
}

async function exportCurrentDiagnosticPackage() {
  const targetUri = appInfo.value.platform === "android" ? settings.exportTargetUri.trim() : "";
  if (appInfo.value.platform === "android" && !targetUri) {
    showToast("请先选择 Android SAF 导出目录", "error");
    return;
  }
  try {
    const result = await exportDiagnosticPackage({
      config: buildConfigSnapshot(),
      ...(targetUri ? { target_uri: targetUri } : {}),
    });
    appendLog("bridge.export_diagnostic_package", result);
    if (!result.ok) {
      showToast(result.message || "诊断包导出失败", "error");
      return;
    }
    const data = asRecord(result.data);
    const target = asString(data.path || data.target_uri || data.targetUri || data.file_name || data.fileName).trim();
    setStatus({
      detail: target ? `诊断包已导出到 ${target}。` : result.message || "诊断包已导出。",
      title: "诊断包已导出",
      tone: "completed",
    });
    showToast(result.message || "诊断包已导出", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "诊断包导出失败", "error");
  }
}

async function openCurrentLogDirectory() {
  if (appInfo.value.platform === "android") {
    const targetUri = settings.exportTargetUri.trim();
    if (!targetUri) {
      showToast("请先选择 Android SAF 导出目录或导出诊断包。", "info");
      return;
    }
    try {
      await openPath(targetUri);
      setStatus({
        detail: `Android SAF 导出目录：${targetUri}`,
        title: "导出目录",
        tone: "completed",
      });
      showToast("已打开 Android SAF 导出目录", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "打开导出目录失败", "error");
    }
    return;
  }
  try {
    const result = await openLogDirectory({
      config: buildConfigSnapshot(),
    });
    appendLog("bridge.open_log_directory", result);
    const data = asRecord(result.data);
    const target = asString(data.path || data.directory).trim();
    if (!result.ok) {
      showToast(result.message || "打开日志目录失败", "error");
      return;
    }
    setStatus({
      detail: target ? `${result.message || "日志目录已定位。"} ${target}` : result.message || "日志目录已定位。",
      title: "日志目录",
      tone: "completed",
    });
    showToast(result.message || (target ? `日志目录：${target}` : "日志目录已定位"), "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "打开日志目录失败", "error");
  }
}

async function handleSchedulerDailyTimesBlur() {
  await flushDraftSave();
  if (!settings.schedulerEnabled) {
    return;
  }
  const saved = await persistCurrentConfig({
    redirectOnMaskedToken: false,
    silentFailure: false,
    silentSuccess: true,
    skipIfUnchanged: true,
  });
  if (saved) {
    await refreshSchedulerStatus();
  }
}

function eventDebugLogDisplayPath(payload: Record<string, unknown>) {
  return asString(payload.debug_log_path || payload.debugLogPath).trim();
}

function eventDebugLogOpenTarget(payload: Record<string, unknown>) {
  return asString(payload.log_uri || payload.logUri || payload.debug_log_path || payload.debugLogPath).trim();
}

function eventTraceFailureSummary(payload: Record<string, unknown>) {
  return summarizeTraceDiagnostics(payload.trace_diagnostics || payload.traceDiagnostics);
}

async function saveDirtyPipelineWorkspaceBeforeArchive(actionLabel: string) {
  if (!pipelineWorkspaceDirty.value) {
    return true;
  }
  const saved = await savePipelineWorkspaceFromView({ silentSuccess: true });
  if (!saved) {
    showToast(`工作流未保存，已取消${actionLabel}`, "error");
    return false;
  }
  return true;
}

async function exportConfigToFile() {
  if (!window.confirm("导出的配置压缩包包含完整 Cloudflare Token 和 WebDAV 凭据。请确认目标位置可信。")) {
    return;
  }
  if (!(await saveDirtyPipelineWorkspaceBeforeArchive("配置导出"))) {
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
  if (data.pipeline_workspace || data.pipelineWorkspace) {
    applyPipelineWorkspace(data.pipeline_workspace || data.pipelineWorkspace);
  }
  if (data.pipeline_profiles || data.pipelineProfiles) {
    applyPipelineProfileStore(data.pipeline_profiles || data.pipelineProfiles);
  }
  if (data.source_profiles || data.sourceProfiles) {
    applySourceProfileStore(data.source_profiles || data.sourceProfiles);
  }
  markSourceSaveBaselines();
  if (data.storage) {
    applyStorageStatus(data.storage);
  }
  configPath.value = asString(data.configPath || data.config_path || configPath.value);
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

async function testTelegramNotificationSettings() {
  telegramTesting.value = true;
  try {
    const result = await testTelegramNotification({ config: buildConfigSnapshot() });
    appendLog("bridge.test_telegram_notification", result);
    showToast(result.message || (result.ok ? "Telegram 通知可用" : "Telegram 通知测试失败"), result.ok ? "success" : "error");
    if (!result.ok) {
      pushActivity("Telegram 通知测试失败", result.message || "请检查 Bot Token 与 Chat ID。");
      return;
    }
    pushActivity("Telegram 通知测试通过", result.message || "上传通知渠道可用。");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "Telegram 通知测试失败", "error");
  } finally {
    telegramTesting.value = false;
  }
}

async function backupToWebDAV() {
  if (!window.confirm("WebDAV 备份会覆盖远端配置压缩包，并包含完整 Cloudflare Token 和 WebDAV 凭据。确认继续？")) {
    return;
  }
  if (!(await saveDirtyPipelineWorkspaceBeforeArchive(" WebDAV 备份"))) {
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
    applyNavigation({ mode: "single", view: "settings" });
    showToast(result.message || "已从 WebDAV 还原配置", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "WebDAV 还原失败", "error");
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
      showToast(result.message || "保存输入组失败", "error");
      return false;
    }
    applySourceProfileStore(result.data);
    if (setActive) {
      sources.value = sourceDraftsFromProfileSources(payloadSources);
    }
    lastSavedSourceProfileSignature = currentSourceProfileSignature();
    showToast("输入组已保存", "success");
    return true;
  } catch (error) {
    showToast(error instanceof Error ? error.message : "保存输入组失败", "error");
    return false;
  }
}

async function updateActiveSourceProfile() {
  try {
    const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
    const result = await updateCurrentSourceProfile({
      name: active?.name || "当前输入源",
      profile_id: active?.id || "",
      sources: sourcePayloads.value,
    });
    appendLog("bridge.update_current_source_profile", result);
    if (!result.ok) {
      showToast(result.message || "更新输入组失败", "error");
      return false;
    }
    applySourceProfileStore(result.data?.source_profiles);
    sources.value = sourceDraftsFromProfileSources(result.data?.sources || sourcePayloads.value);
    lastSavedSourceProfileSignature = currentSourceProfileSignature();
    showToast("输入组已更新并保存", "success");
    return true;
  } catch (error) {
    showToast(error instanceof Error ? error.message : "更新输入组失败", "error");
    return false;
  }
}

async function updateActiveSourceProfileQuietly() {
  try {
    const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
    const result = await updateCurrentSourceProfile({
      name: active?.name || "当前输入源",
      profile_id: active?.id || "",
      sources: sourcePayloads.value,
    });
    appendLog("bridge.update_current_source_profile", result);
    if (!result.ok) {
      appendLog("source.profile_quiet_save.failed", result.message || "更新输入组失败");
      return false;
    }
    applySourceProfileStore(result.data?.source_profiles);
    sources.value = sourceDraftsFromProfileSources(result.data?.sources || sourcePayloads.value);
    lastSavedSourceProfileSignature = currentSourceProfileSignature();
    return true;
  } catch (error) {
    appendLog("source.profile_quiet_save.failed", error instanceof Error ? error.message : String(error));
    return false;
  }
}

async function switchToSourceProfile(profileId: string) {
  if (selectedView.value === "sources" && sourcePageHasUnsavedChanges()) {
    const saved = await autoSaveSourcePage("switch_source_profile");
    if (!saved) {
      showToast("输入源保存失败，已取消切换", "error");
      return;
    }
  }

  try {
    const result = await switchSourceProfile({ profile_id: profileId });
    appendLog("bridge.switch_source_profile", result);
    const data = asRecord(result.data);
    if (!result.ok) {
      showToast(result.message || "切换输入组失败", "error");
      return;
    }
    applySourceProfileStore(data.source_profiles || data.sourceProfiles);
    const nextSources = Array.isArray(data.sources) ? data.sources : [];
    sources.value = sourceDraftsFromProfileSources(nextSources);
    lastSavedSourceProfileSignature = currentSourceProfileSignature();
    if (data.config_snapshot || data.configSnapshot) {
      configPath.value = asString(data.configPath || data.config_path || configPath.value);
    }
    showToast("输入组已切换", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "切换输入组失败", "error");
  }
}

async function removeSourceProfile(profileId: string) {
  const deletedActiveProfile = sourceProfiles.value.active_profile_id === profileId;
  if (!window.confirm("删除输入组后无法恢复。若删除当前输入组，当前输入源会切换到新的当前输入组。")) {
    return;
  }
  if (deletedActiveProfile && selectedView.value === "sources" && sourcePageHasUnsavedChanges()) {
    const saved = await autoSaveSourcePage("delete_active_source_profile");
    if (!saved) {
      showToast("输入源保存失败，已取消删除", "error");
      return;
    }
  }
  try {
    const result = await deleteSourceProfile({ profile_id: profileId });
    appendLog("bridge.delete_source_profile", result);
    if (!result.ok) {
      showToast(result.message || "删除输入组失败", "error");
      return;
    }
    const data = asRecord(result.data);
    const nextStore = normalizeSourceProfileStore(data.source_profiles || data.sourceProfiles || result.data);
    sourceProfiles.value = nextStore;
    if (deletedActiveProfile) {
      const activeProfile = nextStore.items.find((profile) => profile.id === nextStore.active_profile_id) || null;
      const nextSources = Array.isArray(data.sources) ? data.sources : activeProfile?.sources || [];
      sources.value = sourceDraftsFromProfileSources(nextSources);
    }
    lastSavedSourceProfileSignature = currentSourceProfileSignature();
    showToast("输入组已删除", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "删除输入组失败", "error");
  }
}

function pipelineProbeTemplateConfig() {
  return {
    ...(defaultPipelineNodeCatalog().find((item) => item.action === "probe_tcp")?.default_config || {}),
  };
}

function defaultPipelineTemplateDraft(name = "默认工作流", boundSnapshot: ConfigSnapshot = normalizeConfigSnapshot(buildConfigSnapshot())): PipelineTemplate {
  const template = uploadRecoveryPipelineTemplateDraft(name, boundSnapshot);
  template.description = "默认流程：筛选结果后，有结果自动 DNS 推送并导出 GitHub；无结果进入人工复核。";
  return template;
}

function uploadRecoveryPipelineTemplateDraft(name = "高级上传工作流", boundSnapshot: ConfigSnapshot = normalizeConfigSnapshot(buildConfigSnapshot())): PipelineTemplate {
  const now = new Date().toISOString();
  const probeConfig = pipelineProbeTemplateConfig();
  return {
    bound_config_snapshot: normalizeConfigSnapshot(boundSnapshot),
    created_at: now,
    description: "输入源组 -> 输入源筛选 -> TCP 延迟测速 -> 追踪测试 -> 下载测速 -> 结果筛选 -> 结果检查；有结果时 DNS 推送并导出 GitHub，无结果时进入人工复核。",
    enabled: true,
    entry_node_id: "source-group-main",
    edges: [
      { id: "edge-source-filter", label: "", outcome: "", source_node_id: "source-group-main", target_node_id: "source-filter" },
      { id: "edge-filter-tcp", label: "", outcome: "", source_node_id: "source-filter", target_node_id: "probe-tcp" },
      { id: "edge-tcp-trace", label: "", outcome: "", source_node_id: "probe-tcp", target_node_id: "probe-trace" },
      { id: "edge-trace-download", label: "", outcome: "", source_node_id: "probe-trace", target_node_id: "probe-download" },
      { id: "edge-download-filter", label: "", outcome: "", source_node_id: "probe-download", target_node_id: "filter-results" },
      { id: "edge-filter-branch", label: "", outcome: "", source_node_id: "filter-results", target_node_id: "branch-has-results" },
      { id: "edge-branch-dns", label: "有结果", outcome: "true", source_node_id: "branch-has-results", target_node_id: "deliver-dns" },
      { id: "edge-dns-github", label: "", outcome: "", source_node_id: "deliver-dns", target_node_id: "deliver-github" },
      { id: "edge-github-end", label: "", outcome: "", source_node_id: "deliver-github", target_node_id: "end-completed" },
      { id: "edge-branch-recovery", label: "无结果", outcome: "false", source_node_id: "branch-has-results", target_node_id: "recovery-manual-review" },
      { id: "edge-recovery-end", label: "", outcome: "", source_node_id: "recovery-manual-review", target_node_id: "end-manual-review" },
    ],
    id: "",
    name,
    nodes: [
      {
        action: "select_sources",
        config: { source_ids: [], source_profile_id: "", source_selection: "enabled" },
        id: "source-group-main",
        name: "输入源组",
        node_type: "source",
        ui: { collapsed: false, position: { x: 60, y: 180 }, width: 320 },
        updated_at: now,
      },
      {
        action: "filter_sources",
        config: { source_colo_filter: "", source_colo_filter_mode: "allow", source_ip_limit: 500, source_ip_mode: "traverse" },
        id: "source-filter",
        name: "输入源筛选",
        node_type: "source",
        ui: { collapsed: false, position: { x: 420, y: 180 }, width: 320 },
        updated_at: now,
      },
      {
        action: "probe_tcp",
        config: { ...probeConfig },
        id: "probe-tcp",
        name: "TCP 延迟测速",
        node_type: "probe",
        ui: { collapsed: false, position: { x: 780, y: 180 }, width: 320 },
        updated_at: now,
      },
      {
        action: "probe_trace",
        config: { ...probeConfig },
        id: "probe-trace",
        name: "追踪测试",
        node_type: "probe",
        ui: { collapsed: false, position: { x: 1140, y: 180 }, width: 320 },
        updated_at: now,
      },
      {
        action: "probe_download",
        config: { ...probeConfig },
        id: "probe-download",
        name: "下载测速",
        node_type: "probe",
        ui: { collapsed: false, position: { x: 1500, y: 180 }, width: 320 },
        updated_at: now,
      },
      {
        action: "filter_results",
        config: { source: "probe_results", status: "passed" },
        id: "filter-results",
        name: "结果筛选",
        node_type: "filter",
        ui: { collapsed: false, position: { x: 1860, y: 180 }, width: 320 },
        updated_at: now,
      },
      {
        action: "branch_has_results",
        config: { source: "filtered_rows" },
        id: "branch-has-results",
        name: "结果检查",
        node_type: "branch",
        ui: { collapsed: false, position: { x: 2220, y: 180 }, width: 320 },
        updated_at: now,
      },
      {
        action: "deliver_dns",
        config: { source: "filtered_rows", top_n: 0 },
        id: "deliver-dns",
        name: "DNS 推送",
        node_type: "deliver",
        ui: { collapsed: false, position: { x: 2580, y: 70 }, width: 320 },
        updated_at: now,
      },
      {
        action: "deliver_github",
        config: { source: "filtered_rows", top_n: 0 },
        id: "deliver-github",
        name: "GitHub 导出",
        node_type: "deliver",
        ui: { collapsed: false, position: { x: 2940, y: 70 }, width: 320 },
        updated_at: now,
      },
      {
        action: "end",
        config: { message: "上传流程已完成。", status: "completed" },
        id: "end-completed",
        name: "结束：完成",
        node_type: "end",
        ui: { collapsed: false, position: { x: 2220, y: 70 }, width: 320 },
        updated_at: now,
      },
      {
        action: "recovery_mark",
        config: { message: "筛选后没有可投递结果，需要人工复核。", status: "manual_review" },
        id: "recovery-manual-review",
        name: "人工复核标记",
        node_type: "recovery",
        ui: { collapsed: false, position: { x: 1500, y: 300 }, width: 320 },
        updated_at: now,
      },
      {
        action: "end",
        config: { message: "没有可投递结果，已转入人工复核。", status: "manual_review" },
        id: "end-manual-review",
        name: "结束：人工复核",
        node_type: "end",
        ui: { collapsed: false, position: { x: 1860, y: 300 }, width: 320 },
        updated_at: now,
      },
    ],
    ui: {
      viewport: {
        x: 0,
        y: 0,
        zoom: 0.72,
      },
    },
    updated_at: now,
    version: 1,
  };
}

function pipelineTemplatePayload(template: PipelineTemplate) {
  return {
    bound_config_snapshot: normalizeConfigSnapshot(template.bound_config_snapshot || {}),
    created_at: template.created_at,
    description: template.description.trim(),
    enabled: template.enabled,
    entry_node_id: template.entry_node_id,
    edges: template.edges.map((edge) => ({ ...edge })),
    id: template.id,
    name: template.name.trim() || "工作流",
    nodes: template.nodes.map((node) => ({
      ...node,
      config: { ...node.config },
      ui: node.ui
        ? {
            ...node.ui,
            position: node.ui.position ? { ...node.ui.position } : undefined,
          }
        : undefined,
    })),
    ui: template.ui
      ? {
          ...template.ui,
          viewport: template.ui.viewport ? { ...template.ui.viewport } : undefined,
        }
      : undefined,
    updated_at: template.updated_at,
    version: template.version,
  };
}

interface CreatePipelineTemplatePayload {
  preset?: "default" | "upload_recovery";
}

async function createPipelineTemplate(payload?: CreatePipelineTemplatePayload) {
  try {
    const preset = payload?.preset || "default";
    const name = preset === "upload_recovery" ? `高级上传工作流 ${pipelineWorkspace.value.templates.length + 1}` : `工作流 ${pipelineWorkspace.value.templates.length + 1}`;
    const template = preset === "upload_recovery" ? uploadRecoveryPipelineTemplateDraft(name) : defaultPipelineTemplateDraft(name);
    const result = await savePipelineTemplate({
      set_active: true,
      template: pipelineTemplatePayload(template),
    });
    appendLog("bridge.create_pipeline_template", result);
    if (!result.ok) {
      showToast(result.message || "新建工作流失败", "error");
      return;
    }
    applyPipelineWorkspace(result.data);
    workflowFitRequestKey.value += 1;
    showToast(preset === "upload_recovery" ? "高级上传工作流已创建" : "工作流已创建", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "新建工作流失败", "error");
  }
}

async function removePipelineTemplate(templateId: string) {
  if (builtInPipelineTemplateIds.has(templateId)) {
    showToast("内置工作流不能删除", "error");
    return;
  }
  if (!window.confirm("删除工作流后，绑定它的目标会切回默认工作流。")) {
    return;
  }
  try {
    const result = await deletePipelineTemplate({ template_id: templateId });
    appendLog("bridge.delete_pipeline_template", result);
    if (!result.ok) {
      showToast(result.message || "删除工作流失败", "error");
      return;
    }
    applyPipelineWorkspace(result.data);
    showToast("工作流已删除", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "删除工作流失败", "error");
  }
}

async function savePipelineWorkspaceFromView(options: { silentSuccess?: boolean } = {}) {
  try {
    const result = await savePipelineWorkspace({ workspace: pipelineWorkspace.value });
    appendLog("bridge.save_pipeline_workspace", result);
    if (!result.ok) {
      showToast(result.message || "保存失败", "error");
      return false;
    }
    applyPipelineWorkspace(result.data);
    if (!options.silentSuccess) {
      showToast("已保存", "success");
    }
    return true;
  } catch (error) {
    showToast(error instanceof Error ? error.message : "保存失败", "error");
    return false;
  }
}

function openDashboardView() {
  void navigateTo({ mode: "single", view: "dashboard" });
}

async function setActivePipelineTemplate(templateId: string) {
  if (!templateId || templateId === pipelineWorkspace.value.active_template_id) {
    return;
  }
  pipelineWorkspace.value = {
    ...pipelineWorkspace.value,
    active_template_id: templateId,
  };
  await savePipelineWorkspaceFromView();
}

async function refreshActivePipelineSnapshot(pipelineId: string) {
  const id = pipelineId.trim();
  if (!id) {
    return;
  }
  try {
    const result = await getPipelineSnapshot({ pipeline_id: id });
    appendLog("bridge.get_pipeline_snapshot", result);
    if (result.ok && result.data) {
      const normalized = normalizePipelineRunResult(result.data);
      pipelineResults.value = [normalized];
    }
  } catch (error) {
    appendLog("bridge.get_pipeline_snapshot.failed", error instanceof Error ? error.message : String(error));
  }
}

async function launchPipeline(templateId = pipelineWorkspace.value.active_template_id) {
  if (taskActionInFlight.value) {
    notifyTaskActionBlocked("start");
    return;
  }
  if (hasActiveTask.value && !hasPausedTask.value) {
    notifyActiveProbeBlocked("启动工作流被拦截");
    return;
  }
  if (hasPausedTask.value) {
    const stopped = await stopPausedTaskForRestart("启动工作流前需要先停止暂停中的任务。");
    if (!stopped) {
      return;
    }
  }
  if (pipelineWorkspaceDirty.value) {
    const saved = await savePipelineWorkspaceFromView({ silentSuccess: true });
    if (!saved) {
      return;
    }
  }
  const activeTemplate = pipelineWorkspace.value.templates.find((template) => template.id === templateId);
  if (!activeTemplate) {
    showToast("未找到工作流模板", "error");
    return;
  }
  if (Object.keys(normalizeConfigSnapshot(activeTemplate.bound_config_snapshot || {})).length === 0) {
    showToast("请先把当前设置绑定到这个工作流", "error");
    return;
  }

  const pipelineId = allocateTaskId();
  beginTaskAction("start", "pipeline", pipelineId);
  loading.value = true;
  activePipelineId.value = pipelineId;
  pipelineResults.value = [];
  resetProbeSummary();
  resetDownloadSpeedState();
  clearCurrentTaskResultWorkspace();
  taskSessionState.value = "active_runtime";
  task.acceptedAt = new Date().toISOString();
  task.active = true;
  task.completedAt = "";
  task.exportPath = "";
  task.lastEvent = "pipeline.accepted";
  task.lastSeq = 0;
  task.stage = "pipeline";
  task.taskId = pipelineId;
  summary.total = 1;
  setStatus({
    detail: `工作流 ${pipelineId} 已提交，等待后端执行绑定配置。`,
    title: "工作流提交中",
    tone: "preparing",
  });
  pushProcessTrace({
    detail: "将执行当前工作流绑定的单套配置。",
    stage: "pipeline",
    title: "工作流已提交",
    tone: "info",
    ts: task.acceptedAt,
  });

  try {
    const result = await startPipeline({
      config_source: "pipeline",
      pipeline_id: pipelineId,
      target_ids: [],
      task_id: pipelineId,
      template_id: templateId,
    });
    appendLog("bridge.start_pipeline", result);
    probeWarnings.value = result.warnings || [];
    pushWarningTrace(probeWarnings.value);
    if (!result.ok) {
      activePipelineId.value = "";
      task.active = false;
      task.completedAt = new Date().toISOString();
      setStatus({
        detail: result.message || "工作流未被接受。",
        title: "工作流启动失败",
        tone: "failed",
      });
      pushProcessTrace({
        detail: result.message || "工作流未被接受。",
        stage: "failed",
        title: "工作流启动失败",
        tone: "error",
        ts: task.completedAt,
      });
      showToast(result.message || "工作流启动失败", "error");
      return;
    }
    pushActivity("工作流已提交", result.message || `工作流 ${pipelineId} 已进入执行队列。`);
    showToast("工作流已提交", "success");
  } finally {
    finishTaskAction("start");
    loading.value = false;
  }
}

async function saveWorkflowSchedulerFromView(payload: { autoDnsPush: boolean; dailyTimes: string; enabled: boolean; intervalMinutes: number; skipIfActive: boolean; templateId: string; triggerMode: SchedulerTriggerMode }) {
  settings.schedulerAutoDnsPush = payload.autoDnsPush;
  settings.schedulerDailyTimes = payload.dailyTimes;
  settings.schedulerEnabled = payload.enabled;
  settings.schedulerIntervalMinutes = payload.intervalMinutes;
  settings.schedulerPipelineTemplateId = payload.templateId.trim();
  settings.schedulerRunMode = "pipeline";
  settings.schedulerSkipIfActive = payload.skipIfActive;
  settings.schedulerTriggerMode = payload.triggerMode;
  settings.schedulerAutoGithubExport = false;
  await persistCurrentConfig();
  await refreshSchedulerStatus();
}

function applyTaskSnapshot(snapshot: TaskSnapshot) {
  taskSnapshot.value = snapshot;
  const snapshotSessionState = asString(snapshot.session_state || "").trim();
  taskSessionState.value = snapshotSessionState || asString(taskSessionState.value || "idle");
  task.taskId = snapshot.task_id || task.taskId;
  task.stage = snapshot.current_stage || snapshot.progress?.stage || task.stage;
  task.completedAt = snapshot.completed_at || "";
  const inactiveSession = ["idle", "persisted_only"].includes(snapshotSessionState);
  task.active = !["completed", "failed", "no_results"].includes(snapshot.status || "") && !inactiveSession && (snapshot.runtime_attached !== false || snapshot.session_state === "paused_runtime");

  if (snapshot.progress) {
    summary.failed = asCount(snapshot.progress.failed, summary.failed);
    summary.passed = asCount(snapshot.progress.passed, summary.passed);
    summary.processed = asCount(snapshot.progress.processed, summary.processed);
    summary.total = asCount(snapshot.progress.total, summary.total);
  }

  if (snapshot.export_record) {
    summary.exported = asCount(snapshot.export_record.written_count, summary.exported);
    task.exportPath = [snapshot.export_record.target_dir, snapshot.export_record.file_name].filter(Boolean).join("/");
    taskResultCSVPath.value = asString(snapshot.export_record.source_path || taskResultCSVPath.value).trim();
  }
}

function shouldAllowTaskResultFileFallback(snapshot: TaskSnapshot | null | undefined, taskId: string) {
  const normalizedTaskId = taskId.trim();
  const snapshotTaskId = asString(snapshot?.task_id || "").trim();
  if (snapshotTaskId && normalizedTaskId && snapshotTaskId !== normalizedTaskId) {
    return false;
  }
  if (snapshot?.export_record) {
    return true;
  }

  const snapshotStatus = asString(snapshot?.status || "").trim();
  if (snapshotStatus === "completed") {
    return true;
  }

  if (snapshotStatus === "no_results") {
    return Boolean(task.exportPath.trim());
  }

  if (normalizedTaskId && normalizedTaskId === task.taskId.trim()) {
    if (task.exportPath.trim()) {
      return true;
    }
    if (summary.passed > 0 || summary.exported > 0) {
      return true;
    }
  }

  return false;
}

function shouldRetryTaskResultCSVFallback(snapshot: TaskSnapshot | null | undefined, taskId: string) {
  if (!shouldAllowTaskResultFileFallback(snapshot, taskId)) {
    return false;
  }
  const snapshotStatus = asString(snapshot?.status || "").trim();
  if (["completed", "partial", "cooling", "no_results"].includes(snapshotStatus)) {
    return Boolean(taskResultCSVFallbackPath(snapshot).trim());
  }
  return Boolean(taskResultCSVPath.value.trim());
}

function taskResultCSVFallbackPath(snapshot?: TaskSnapshot | null) {
  const snapshotSourcePath = asString(snapshot?.export_record?.source_path || "").trim();
  if (snapshotSourcePath) {
    return snapshotSourcePath;
  }
  const cachedSourcePath = taskResultCSVPath.value.trim();
  if (cachedSourcePath) {
    return cachedSourcePath;
  }
  return task.exportPath.trim();
}

function buildResultFileFallbackPayload(exportPath: string, snapshot?: TaskSnapshot | null) {
  const normalizedExportPath = (taskResultCSVFallbackPath(snapshot) || exportPath).trim();
  return {
    config: buildConfigSnapshot(),
    export_path: normalizedExportPath,
    path: normalizedExportPath,
    target_path: normalizedExportPath,
  };
}

async function reconcileTaskData(taskId = task.taskId, options: { switchToResultsOnData?: boolean } = {}) {
  const normalizedTaskId = taskId.trim();
  if (!normalizedTaskId) {
    return;
  }
  if (resultWorkspaceTaskId.value && resultWorkspaceTaskId.value !== normalizedTaskId) {
    clearCurrentTaskResultWorkspace();
  }
  await refreshTaskData(normalizedTaskId);
  if (options.switchToResultsOnData && resultRows.value.length > 0) {
    void navigateTo({ mode: "single", view: "results" });
  }
}

async function refreshTaskData(taskId = task.taskId) {
  const normalizedTaskId = taskId.trim() || task.taskId.trim() || "result-file";

  if (snapshotRefreshInFlight) {
    snapshotRefreshPending = true;
    snapshotRefreshQueuedTaskId = normalizedTaskId;
    return;
  }

  snapshotRefreshInFlight = true;
  snapshotRefreshQueuedTaskId = normalizedTaskId;
  resultsLoading.value = true;

  try {
    do {
      const currentTaskId = snapshotRefreshQueuedTaskId || normalizedTaskId;
      snapshotRefreshPending = false;
      snapshotRefreshQueuedTaskId = "";
      const snapshotResult = await getTaskSnapshot(currentTaskId);

      appendLog("bridge.get_task_snapshot", snapshotResult);

      let snapshotForResults: TaskSnapshot | null = null;
      if (snapshotResult.ok && snapshotResult.data) {
        snapshotForResults = snapshotResult.data;
      } else if (taskSnapshot.value && asString(taskSnapshot.value.task_id || "").trim() === currentTaskId) {
        snapshotForResults = taskSnapshot.value;
      }

      if (snapshotResult.ok && snapshotResult.data) {
        applyTaskSnapshot(snapshotResult.data);
      }

      const resultsResult = await listTaskResults(
        currentTaskId,
        resultSortBy.value,
        resultOrder.value,
        resultFilter.value,
        buildResultFileFallbackPayload(task.exportPath, snapshotForResults),
        resultIpFilter.value,
        {
          limit: resultsPageLimit.value,
          offset: 0,
        },
        {
          allowFileFallback: shouldAllowTaskResultFileFallback(snapshotForResults, currentTaskId),
        },
      );
      appendLog("bridge.list_task_results", resultsResult);

      if (resultsResult.ok && resultsResult.data) {
        let nextRows = Array.isArray(resultsResult.data.results) ? resultsResult.data.results : [];
        let totalCount = asCount(resultsResult.data.total_count, nextRows.length);
        if (nextRows.length === 0 && shouldRetryTaskResultCSVFallback(snapshotForResults, currentTaskId)) {
          const csvResult = await listTaskResults(
            currentTaskId,
            resultSortBy.value,
            resultOrder.value,
            resultFilter.value,
            buildResultFileFallbackPayload(task.exportPath, snapshotForResults),
            resultIpFilter.value,
            {
              limit: resultsPageLimit.value,
              offset: 0,
            },
            {
              allowFileFallback: true,
            },
          );
          appendLog("bridge.list_task_results.csv_fallback", csvResult);
          if (csvResult.ok && csvResult.data) {
            nextRows = Array.isArray(csvResult.data.results) ? csvResult.data.results : [];
            totalCount = asCount(csvResult.data.total_count, nextRows.length);
          }
        }
        applyCurrentTaskResultWorkspace(currentTaskId, nextRows, totalCount);
      }
    } while (snapshotRefreshPending);
  } finally {
    snapshotRefreshInFlight = false;
    resultsLoading.value = false;
    snapshotRefreshQueuedTaskId = "";
  }
}

function applyProbeEvent(event: ProbeEventEnvelope) {
  const incomingTaskId = asString(event.task_id).trim();
  const currentTaskId = task.taskId.trim();
  const eventPipelineId = asString(event.payload.pipeline_id || event.payload.pipelineId).trim();
  const pipelineMatches = Boolean(activePipelineId.value && (eventPipelineId === activePipelineId.value || incomingTaskId === activePipelineId.value));
  const pipelineChildEvent = Boolean(activePipelineId.value && eventPipelineId === activePipelineId.value && incomingTaskId !== activePipelineId.value && event.event.startsWith("probe."));
  if (incomingTaskId && currentTaskId && incomingTaskId !== currentTaskId && !pipelineMatches) {
    appendLog(`${event.event}.ignored`, {
      current_task_id: currentTaskId,
      event_task_id: incomingTaskId,
      active_pipeline_id: activePipelineId.value,
      event_pipeline_id: eventPipelineId,
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
  const eventDebugLogPath = eventDebugLogDisplayPath(event.payload);
  const eventDebugLogTarget = eventDebugLogOpenTarget(event.payload);
  const traceFailureSummary = eventTraceFailureSummary(event.payload);

  if (event.event === "pipeline.started") {
    activePipelineId.value = eventPipelineId || incomingTaskId || activePipelineId.value;
    taskSessionState.value = "active_runtime";
    task.active = true;
    task.stage = "pipeline";
    task.taskId = activePipelineId.value || task.taskId;
    summary.total = asCount(event.payload.total, summary.total);
    summary.processed = 0;
    summary.passed = 0;
    summary.failed = 0;
    pushProcessTrace({
      detail: `工作流开始执行，共 ${summary.total || "-"} 个目标。`,
      stage: "pipeline",
      title: "工作流已启动",
      tone: "running",
      ts: event.ts,
    });
  }

  if (event.event === "pipeline.profile_started") {
    taskSessionState.value = "active_runtime";
    task.active = true;
    task.stage = "pipeline_profile";
    const profileName = asString(event.payload.profile_name || event.payload.pipeline_profile_name || "当前目标");
    const region = asString(event.payload.region || event.payload.pipeline_region);
    const domain = asString(event.payload.domain || event.payload.pipeline_domain);
    pushProcessTrace({
      detail: `${profileName}${region ? ` / ${region}` : ""}${domain ? ` / ${domain}` : ""} 已开始。`,
      stage: "pipeline_profile",
      title: "目标开始执行",
      tone: "running",
      ts: event.ts,
    });
  }

  if (event.event === "pipeline.profile_skipped") {
    summary.processed += 1;
    task.stage = "pipeline_profile";
    pushProcessTrace({
      detail: asString(event.payload.message || "目标未启用，已跳过。"),
      stage: "pipeline_profile",
      title: "目标已跳过",
      tone: "warning",
      ts: event.ts,
    });
  }

  if (event.event === "pipeline.profile_completed") {
    summary.processed = Math.min(summary.total || summary.processed + 1, summary.processed + 1);
    const resultCount = asCount(event.payload.result_count, 0);
    summary.passed += resultCount > 0 ? resultCount : 0;
    const statusValue = asString(event.payload.status);
    if (statusValue === "dns_failed") {
      summary.failed += 1;
    }
    pushProcessTrace({
      detail: `${asString(event.payload.profile_name || event.payload.pipeline_profile_name || "目标")} 完成，可用结果 ${resultCount} 条${statusValue === "dns_failed" ? "，DNS 推送失败" : ""}。`,
      stage: "pipeline_profile",
      title: statusValue === "dns_failed" ? "目标部分完成" : "目标完成",
      tone: statusValue === "dns_failed" ? "warning" : "success",
      ts: event.ts,
    });
    void refreshActivePipelineSnapshot(eventPipelineId || activePipelineId.value);
  }

  if (event.event === "pipeline.profile_failed") {
    summary.processed = Math.min(summary.total || summary.processed + 1, summary.processed + 1);
    summary.failed += 1;
    pushProcessTrace({
      detail: asString(event.payload.message || "目标执行失败。"),
      stage: "pipeline_profile",
      title: "目标失败",
      tone: "error",
      ts: event.ts,
    });
  }

  if (event.event === "pipeline.completed") {
    const completedStatus = asString(event.payload.status);
    const pipelineCancelled = completedStatus === "cancelled";
    finishTaskAction();
    taskSessionState.value = "idle";
    task.active = false;
    task.stage = pipelineCancelled ? "cancelled" : "completed";
    task.completedAt = event.ts;
    summary.total = asCount(event.payload.total, summary.total);
    summary.processed = summary.total;
    summary.failed = asCount(event.payload.failed, summary.failed);
    activePipelineId.value = "";
    resetDownloadSpeedState();
    pushProcessTrace({
      detail: nextTaskState.detail,
      stage: pipelineCancelled ? "cancelled" : "completed",
      title: nextTaskState.title,
      tone: pipelineCancelled || summary.failed > 0 ? "warning" : "success",
      ts: event.ts,
    });
    void refreshPipelineResults(eventPipelineId || incomingTaskId);
    showToast(nextTaskState.title, pipelineCancelled || summary.failed > 0 ? "info" : "success");
  }

  if (event.event === "pipeline.failed") {
    finishTaskAction();
    taskSessionState.value = "idle";
    task.active = false;
    task.stage = "failed";
    task.completedAt = event.ts;
    activePipelineId.value = "";
    resetDownloadSpeedState();
    pushProcessTrace({
      detail: nextTaskState.detail,
      stage: "failed",
      title: "工作流失败",
      tone: "error",
      ts: event.ts,
    });
    void refreshPipelineResults(eventPipelineId || incomingTaskId);
    showToast(nextTaskState.detail, "error");
  }

  if (event.event === "pipeline.node_started") {
    taskSessionState.value = "active_runtime";
    task.active = true;
    task.stage = asString(event.payload.node_id || event.payload.action || "pipeline_node");
    const target = pipelineTraceTargetLabel(event.payload);
    const node = pipelineTraceNodeLabel(event.payload);
    pushProcessTrace({
      detail: `${target} / ${node} 开始执行。`,
      stage: task.stage || "pipeline_node",
      title: "节点开始执行",
      tone: "running",
      ts: event.ts,
    });
  }

  if (event.event === "pipeline.node_completed") {
    taskSessionState.value = "active_runtime";
    task.active = true;
    task.stage = asString(event.payload.node_id || event.payload.action || "pipeline_node");
    const target = pipelineTraceTargetLabel(event.payload);
    const node = pipelineTraceNodeLabel(event.payload);
    const statusValue = asString(event.payload.status);
    const message = asString(event.payload.message);
    const outputSummary = asString(event.payload.output_summary);
    const statusLabel = pipelineTraceNodeStatusLabel(statusValue);
    pushProcessTrace({
      detail: `${target} / ${node} ${statusLabel}${message ? `：${message}` : outputSummary ? `：${outputSummary}` : "。"}`,
      stage: task.stage || "pipeline_node",
      title: statusValue === "failed" ? "节点执行失败" : statusValue === "skipped" ? "节点已跳过" : "节点执行完成",
      tone: statusValue === "failed" || statusValue === "skipped" ? "warning" : "success",
      ts: event.ts,
    });
  }

  if (event.event === "pipeline.branch_taken") {
    taskSessionState.value = "active_runtime";
    task.active = true;
    task.stage = asString(event.payload.node_id || event.payload.action || "pipeline_branch");
    const target = pipelineTraceTargetLabel(event.payload);
    const node = pipelineTraceNodeLabel(event.payload);
    const branch = asString(event.payload.branch_taken || event.payload.outcome || "-");
    const resultCount = asCount(event.payload.result_count, 0);
    pushProcessTrace({
      detail: `${target} / ${node} 命中分支 ${branch}，当前结果 ${resultCount} 条。`,
      stage: task.stage || "pipeline_branch",
      title: "分支已命中",
      tone: "running",
      ts: event.ts,
    });
  }

  if (event.event === "probe.preprocessed") {
    finishTaskAction("start");
    finishTaskAction("rerun");
    taskSessionState.value = "active_runtime";
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
    finishTaskAction("start");
    finishTaskAction("rerun");
    finishTaskAction("resume");
    taskSessionState.value = "active_runtime";
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

  if (event.event === "probe.resumed") {
    finishTaskAction("resume");
    taskSessionState.value = "active_runtime";
    task.active = true;
    task.stage = asString(event.payload.stage || event.payload.current_stage || task.stage) || task.stage;
    pushProcessTrace({
      detail: asString(event.payload.message || "任务已恢复执行。"),
      stage: task.stage || "running",
      title: "任务继续执行",
      tone: "running",
      ts: event.ts,
    });
  }

  if (event.event === "probe.speed") {
    finishTaskAction("start");
    finishTaskAction("rerun");
    finishTaskAction("resume");
    taskSessionState.value = "active_runtime";
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
    const transferComplete = transferCompleteValue === undefined ? false : asBoolean(transferCompleteValue, false);
    const elapsedMs = asCount(event.payload.elapsed_ms, 0);
    const colo = asString(event.payload.colo).trim();
    const hasReadyFlag = currentReadyValue !== undefined || averageReadyValue !== undefined;
    const isInitialEmptySample = elapsedMs === 0 && bytesRead === 0 && ((hasReadyFlag && !currentReady && !averageReady) || (!hasReadyFlag && currentSpeed === 0 && averageSpeed === 0 && !downloadSpeedState.ip));
    if (isInitialEmptySample) {
      return;
    }
    if (downloadSpeedState.ip && downloadSpeedState.ip !== ip) {
      resetDownloadSpeedState();
    }
    downloadSpeedState.active = true;
    const averageDisplayReady = averageReady && (averageSpeed !== 0 || bytesRead > 0 || measuredBytes > 0 || bodyRead || transferComplete);
    downloadSpeedState.averageSpeedMbS = averageDisplayReady ? averageSpeed : null;
    downloadSpeedState.bytesRead = bytesRead;
    downloadSpeedState.colo = colo;
    downloadSpeedState.currentSpeedMbS = currentReady ? currentSpeed : null;
    downloadSpeedState.elapsedMs = elapsedMs;
    downloadSpeedState.ip = ip;
    updateDownloadSpeedTrace(ip, colo, event.ts);
  }

  if (event.event === "probe.partial_export") {
    taskSessionState.value = "active_runtime";
    summary.exported = asCount(event.payload.written, summary.exported);
    taskResultCSVPath.value = asString(event.payload.source_path || taskResultCSVPath.value).trim();
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

  if (event.event === "probe.export_completed") {
    finishTaskAction();
    taskSessionState.value = "idle";
    task.active = false;
    summary.exported = asCount(event.payload.written, summary.exported);
    taskResultCSVPath.value = asString(event.payload.source_path || taskResultCSVPath.value).trim();
    task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
    updateHistory({
      debugLogPath: eventDebugLogPath,
      debugLogTarget: eventDebugLogTarget,
      detail: nextTaskState.detail,
      exported: summary.exported,
      failureSummary: "",
      targetPath: task.exportPath,
      taskId: task.taskId,
      title: nextTaskState.title,
      tone: "completed",
      updatedAt: event.ts,
    });
    pushProcessTrace({
      detail: task.exportPath ? `Android 系统导出已写出 ${summary.exported} 条结果到 ${task.exportPath}。` : `Android 系统导出已写出 ${summary.exported} 条结果。`,
      stage: "export",
      title: "系统导出完成",
      tone: "success",
      ts: event.ts,
    });
    showToast("Android 系统导出完成", "success");
  }

  if (event.event === "probe.export_failed") {
    finishTaskAction();
    taskSessionState.value = "idle";
    task.active = false;
    const exportFailureMessage = asString(event.payload.message || nextTaskState.detail).trim() || "Android 系统导出失败。";
    updateHistory({
      debugLogPath: eventDebugLogPath,
      debugLogTarget: eventDebugLogTarget,
      detail: exportFailureMessage,
      exported: summary.exported,
      failureSummary: "",
      targetPath: task.exportPath,
      taskId: task.taskId,
      title: "系统导出失败",
      tone: "warning",
      updatedAt: event.ts,
    });
    pushProcessTrace({
      detail: exportFailureMessage,
      stage: "export",
      title: "系统导出失败",
      tone: "warning",
      ts: event.ts,
    });
    showToast(exportFailureMessage, "error");
  }

  if (event.event === "probe.completed") {
    if (pipelineChildEvent) {
      taskSessionState.value = "active_runtime";
      task.active = true;
    } else {
      finishTaskAction();
      taskSessionState.value = "idle";
      task.active = false;
    }
    task.completedAt = event.ts;
    resetDownloadSpeedState();
    summary.exported = asCount(event.payload.exported, summary.exported);
    summary.failed = Math.max(summary.failed, asCount(event.payload.failed, summary.failed));
    const resultCount = Math.max(asCount(event.payload.result_count, 0), asCount(event.payload.passed, 0), summary.passed, summary.exported, resultRows.value.length);
    summary.passed = Math.max(summary.passed, resultCount);
    taskResultCSVPath.value = asString(event.payload.source_path || taskResultCSVPath.value).trim();
    task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
    const completedWarnings = Array.isArray(event.payload.warnings) ? event.payload.warnings.map((entry) => asString(entry)).filter(Boolean) : [];
    if (completedWarnings.length > 0) {
      probeWarnings.value = completedWarnings;
      pushWarningTrace(completedWarnings, event.ts);
    }
    const hasResults = resultCount > 0;
    updateHistory({
      debugLogPath: eventDebugLogPath,
      debugLogTarget: eventDebugLogTarget,
      detail: status.detail,
      exported: summary.exported,
      failureSummary: summarizeFailureSummary(event.payload.failure_summary) || traceFailureSummary,
      targetPath: task.exportPath,
      taskId: task.taskId,
      title: status.title,
      tone: hasResults ? "completed" : "no_results",
      updatedAt: event.ts,
    });
    pushProcessTrace({
      detail: hasResults ? `任务完成，可用结果 ${resultCount} 条${task.exportPath ? `，导出路径 ${task.exportPath}` : ""}。` : "任务执行完成，但当前筛选条件下没有可用结果。",
      stage: "completed",
      title: hasResults ? "探测任务完成" : "任务完成但无结果",
      tone: hasResults ? "success" : "warning",
      ts: event.ts,
    });
    void reconcileTaskData(task.taskId || incomingTaskId, {
      switchToResultsOnData: hasResults || appInfo.value.platform === "android",
    });
    if (!pipelineChildEvent) {
      showToast(hasResults ? "探测任务已完成" : "任务结束但没有可用结果", hasResults ? "success" : "info");
    }
  }

  if (event.event === "probe.failed") {
    if (pipelineChildEvent) {
      taskSessionState.value = "active_runtime";
      task.active = true;
    } else {
      finishTaskAction();
      taskSessionState.value = activeTaskSessionState.value === "persisted_only" || taskSnapshot.value?.session_state === "persisted_only" ? "persisted_only" : "idle";
      task.active = false;
    }
    task.completedAt = event.ts;
    resetDownloadSpeedState();
    taskResultCSVPath.value = asString(event.payload.source_path || taskResultCSVPath.value).trim();
    task.exportPath = asString(event.payload.target_path || task.exportPath).trim();
    const failureMessage = asString(status.detail || event.payload.message).trim() || "探测任务失败。";
    updateHistory({
      debugLogPath: eventDebugLogPath,
      debugLogTarget: eventDebugLogTarget,
      detail: failureMessage,
      exported: summary.exported,
      failureSummary: traceFailureSummary,
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
    if (!pipelineChildEvent) {
      showToast(failureMessage, "error");
    }
  }

  if (event.event === "probe.cooling") {
    finishTaskAction("pause");
    finishTaskAction("cancel");
    taskSessionState.value = asBoolean(event.payload.recoverable, true) ? "paused_runtime" : "idle";
    task.active = asBoolean(event.payload.recoverable, true);
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
    const result = action === "fetch" ? await fetchDesktopSource(payload) : await previewDesktopSource(payload);
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
    appendLog("bridge.load_config", commandDiagnosticPayload(result));
    probeWarnings.value = result.warnings || [];
    applyStorageStatus(data.storage);
    configPath.value = asString(data.configPath || data.config_path || configPath.value);

    if (!result.ok) {
      const failureMessage = result.message || "读取配置失败。";
      setStatus({
        detail: statusDetailWithStorage(failureMessage, storageStatus.value),
        title: storageStatus.value?.permission_ok === false ? "权限失效" : "读取失败",
        tone: "failed",
      });
      pushActivity("读取配置失败", failureMessage);
      showToast(failureMessage, "error");
      return;
    }

    applyConfigSnapshot(normalizeConfigSnapshot(data.config_snapshot || {}));
    if (data.pipeline_workspace || data.pipelineWorkspace) {
      applyPipelineWorkspace(data.pipeline_workspace || data.pipelineWorkspace);
    }
    if (data.pipeline_profiles || data.pipelineProfiles) {
      applyPipelineProfileStore(data.pipeline_profiles || data.pipelineProfiles);
    }
    if (data.source_profiles || data.sourceProfiles) {
      applySourceProfileStore(data.source_profiles || data.sourceProfiles);
    }
    markSourceSaveBaselines();
    await maybeRestoreDesktopDraft(data.draft_status || data.draftStatus);
    lastSavedSnapshotSignature = currentSnapshotSignature();
    configHydrated = true;
    configPath.value = asString(data.configPath || data.config_path || configPath.value);
    const successMessage = result.message || "配置已加载。";
    const syncWarning = storageStatus.value?.last_sync_error?.trim() || "";
    const successDetail = syncWarning ? statusDetailWithStorage(`${successMessage} 但存储状态异常：${syncWarning}`, storageStatus.value) : statusDetailWithStorage(successMessage, storageStatus.value);
    setStatus({
      detail: successDetail,
      title: syncWarning ? "存储状态异常" : "配置已加载",
      tone: syncWarning ? "warning" : "idle",
    });
    pushActivity(syncWarning ? "配置已加载（存储状态异常）" : "配置已加载", syncWarning || successMessage || "已读取当前配置快照。");
    showToast(syncWarning ? `配置已加载，但存储状态异常：${syncWarning}` : successMessage || "配置已加载", syncWarning ? "info" : "success");
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
    const installStarted = asBoolean(asRecord(result.data).install_started || asRecord(result.data).installStarted, false);
    updateState.message = result.message || (installStarted ? "更新安装流程已启动。" : "更新包已下载，请按当前平台说明手动部署。");
    showToast(installStarted ? "更新安装流程已启动" : "更新包已下载", "success");
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

interface PersistConfigOptions {
  redirectOnMaskedToken?: boolean;
  silentFailure?: boolean;
  silentMaskedToken?: boolean;
  silentSuccess?: boolean;
  skipIfUnchanged?: boolean;
}

async function autoSaveSettings(reason: string) {
  if (!configHydrated || draftRestoring) {
    return true;
  }
  while (currentSnapshotSignature() !== lastSavedSnapshotSignature) {
    const saved = await persistConfig({
      redirectOnMaskedToken: false,
      silentFailure: true,
      silentMaskedToken: true,
      silentSuccess: true,
      skipIfUnchanged: true,
    });
    if (!saved) {
      const skippedSignature = currentSnapshotSignature();
      if (skippedSignature !== lastSettingsAutoSaveSkippedSignature) {
        appendLog("settings.auto_save.skipped", { reason });
        lastSettingsAutoSaveSkippedSignature = skippedSignature;
      }
      return false;
    }
  }
  lastSettingsAutoSaveSkippedSignature = "";
  return true;
}

async function persistConfig(options: PersistConfigOptions = {}) {
  if (configSaveInFlight) {
    return configSaveInFlight;
  }

  configSaveInFlight = persistConfigNow(options);
  try {
    return await configSaveInFlight;
  } finally {
    configSaveInFlight = null;
  }
}

async function persistCurrentConfig(options: PersistConfigOptions = {}) {
  while (currentSnapshotSignature() !== lastSavedSnapshotSignature) {
    const joinedExistingSave = configSaveInFlight !== null;
    const saved = await persistConfig(options);
    if (!saved) {
      if (joinedExistingSave && currentSnapshotSignature() !== lastSavedSnapshotSignature) {
        continue;
      }
      return false;
    }
  }
  return true;
}

async function persistConfigNow(options: PersistConfigOptions = {}) {
  const snapshot = buildConfigSnapshot();
  const requestedSignature = snapshotSignature(snapshot);
  if (options.skipIfUnchanged && requestedSignature === lastSavedSnapshotSignature) {
    return true;
  }

  if (saveBlockedByMaskedToken.value) {
    if (options.silentMaskedToken) {
      return false;
    }
    setStatus({
      detail: "当前只拿到了脱敏 Token。请重新输入完整 API Token 后再保存。",
      title: "需要完整 Token",
      tone: "failed",
    });
    pushActivity("保存被阻止", "检测到脱敏 Token，占位值不能直接回写。");
    if (options.redirectOnMaskedToken !== false) {
      applyNavigation({ mode: "single", view: "settings" });
    }
    showToast("需要重新输入完整 Token", "error");
    return false;
  }

  if (!options.silentSuccess) {
    loading.value = true;
  }

  try {
    const result = await saveConfig({
      config_snapshot: snapshot,
    });
    const data = asRecord(result.data);
    appendLog("bridge.save_config", result);
    probeWarnings.value = result.warnings || [];

    if (!result.ok) {
      if (!options.silentFailure) {
        setStatus({
          detail: result.message || "保存配置失败。",
          title: "保存失败",
          tone: "failed",
        });
        pushActivity("保存失败", result.message || "保存配置失败。");
        showToast("保存配置失败", "error");
      }
      return false;
    }

    const normalizedSnapshot = normalizeConfigSnapshot(data.config_snapshot || data.configSnapshot || snapshot);
    const currentStillMatchesRequest = currentSnapshotSignature() === requestedSignature;
    if (currentStillMatchesRequest) {
      applyConfigSnapshot(normalizedSnapshot);
    }
    applyStorageStatus(data.storage);
    if (currentStillMatchesRequest) {
      if (data.pipeline_workspace || data.pipelineWorkspace) {
        applyPipelineWorkspace(data.pipeline_workspace || data.pipelineWorkspace);
      }
      if (data.pipeline_profiles || data.pipelineProfiles) {
        applyPipelineProfileStore(data.pipeline_profiles || data.pipelineProfiles);
      }
      if (data.source_profiles || data.sourceProfiles) {
        applySourceProfileStore(data.source_profiles || data.sourceProfiles);
      }
    }
    if (draftSaveTimer !== undefined) {
      window.clearTimeout(draftSaveTimer);
      draftSaveTimer = undefined;
    }
    lastSavedSnapshotSignature = currentStillMatchesRequest ? currentSnapshotSignature() : snapshotSignature(normalizedSnapshot);
    lastDraftSnapshotSignature = "";
    if (currentStillMatchesRequest) {
      markSourceSaveBaselines();
    }
    configPath.value = asString(data.configPath || data.config_path || configPath.value);
    if (!options.silentSuccess) {
      setStatus({
        detail: result.message || "配置已保存。",
        title: "配置已保存",
        tone: "idle",
      });
      pushActivity("配置已保存", result.message || "设置已保存并可用于后续任务。");
      showToast("配置已保存");
    }
    await refreshSchedulerStatus();
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    appendLog("bridge.save_config.failed", message);
    if (!options.silentFailure) {
      setStatus({
        detail: message || "保存配置失败。",
        title: "保存失败",
        tone: "failed",
      });
      pushActivity("保存失败", message || "保存配置失败。");
      showToast("保存配置失败", "error");
    }
    return false;
  } finally {
    if (!options.silentSuccess) {
      loading.value = false;
    }
  }
}

async function launchProbe() {
  if (taskActionInFlight.value) {
    notifyTaskActionBlocked("start");
    return;
  }
  if (hasActiveTask.value && !hasPausedTask.value) {
    notifyActiveProbeBlocked("启动任务被拦截");
    return;
  }

  if (preparedSources.value.length === 0) {
    setStatus({
      detail: "至少需要一个已启用且内容完整的输入源，支持手动输入、本地文件或远程 URL。",
      title: "缺少输入源",
      tone: "failed",
    });
    void navigateTo({ mode: "single", view: "sources" });
    showToast("请先配置至少一个来源", "error");
    return;
  }
  if (hasPausedTask.value) {
    const stopped = await stopPausedTaskForRestart("启动新任务前需要先终止暂停中的任务。");
    if (!stopped) {
      return;
    }
  }

  beginTaskAction("start");
  loading.value = true;
  activePipelineId.value = "";
  resetProbeSummary();
  resetDownloadSpeedState();
  clearCurrentTaskResultWorkspace();
  taskSessionState.value = "active_runtime";
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
  void navigateTo({ mode: "single", view: "dashboard" });

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
      finishTaskAction("start");
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
    void reconcileTaskData(task.taskId || taskId);
  } finally {
    finishTaskAction("start");
    loading.value = false;
  }
}

async function rerunSingleAddress(address: string) {
  const trimmedAddress = address.trim();

  if (!trimmedAddress) {
    return;
  }

  if (taskActionInFlight.value) {
    notifyTaskActionBlocked("rerun");
    return;
  }

  if (hasActiveTask.value && !hasPausedTask.value) {
    notifyActiveProbeBlocked("单条重测被拦截");
    return;
  }
  if (hasPausedTask.value) {
    const stopped = await stopPausedTaskForRestart("单条重测前需要先终止暂停中的任务。");
    if (!stopped) {
      return;
    }
  }

  beginTaskAction("rerun", trimmedAddress);
  loading.value = true;
  resetProbeSummary();
  resetDownloadSpeedState();
  resultRows.value = [];
  taskSnapshot.value = null;
  taskSessionState.value = "active_runtime";
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
  void navigateTo({ mode: "single", view: "dashboard" });

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
      finishTaskAction("rerun");
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
    void reconcileTaskData(task.taskId || taskId);
  } finally {
    finishTaskAction("rerun");
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

  resetCurrentResultPage();
  void refreshTaskData();
}

function updateResultFilter(filter: ProbeResultFilter) {
  if (resultFilter.value === filter) {
    return;
  }
  resultFilter.value = filter;
  resetCurrentResultPage();
  void refreshTaskData();
}

function updateResultIpFilter(filter: ProbeResultIPFilter) {
  if (resultIpFilter.value === filter) {
    return;
  }
  resultIpFilter.value = filter;
  resetCurrentResultPage();
  void refreshTaskData();
}

function updateResultOrder(order: ProbeResultOrder) {
  if (resultOrder.value === order) {
    return;
  }
  resultOrder.value = order;
  resetCurrentResultPage();
  void refreshTaskData();
}

function currentVisibleResultRows() {
  return [...resultRows.value];
}

function refreshCurrentTaskData() {
  void refreshTaskData();
}

async function refreshAndroidBatteryStatus() {
  if (appInfo.value.platform !== "android") {
    androidBatteryStatus.value = null;
    return;
  }
  try {
    const result = await checkBatteryOptimization();
    appendLog("bridge.check_battery_optimization", result);
    if (result.ok && result.data) {
      androidBatteryStatus.value = result.data;
    }
  } catch (error) {
    appendLog("bridge.check_battery_optimization.failed", error instanceof Error ? error.message : String(error));
  }
}

async function refreshAndroidNotificationStatus() {
  if (appInfo.value.platform !== "android") {
    androidNotificationStatus.value = null;
    return;
  }
  try {
    const result = await checkNotificationPermission();
    appendLog("bridge.check_notification_permission", result);
    if (result.ok && result.data) {
      androidNotificationStatus.value = result.data;
    }
  } catch (error) {
    appendLog("bridge.check_notification_permission.failed", error instanceof Error ? error.message : String(error));
  }
}

async function refreshAndroidKeepAliveStatus() {
  if (appInfo.value.platform !== "android") {
    androidKeepAliveStatus.value = null;
    return;
  }
  try {
    const result = await checkKeepAliveStatus();
    appendLog("bridge.check_keep_alive_status", result);
    if (result.ok && result.data) {
      androidKeepAliveStatus.value = result.data;
    }
  } catch (error) {
    appendLog("bridge.check_keep_alive_status.failed", error instanceof Error ? error.message : String(error));
  }
}

async function refreshAndroidNotificationStatusAfterSettingsReturn() {
  if (!androidNotificationSettingsRefreshPending || androidNotificationSettingsRefreshInFlight || appInfo.value.platform !== "android") {
    return;
  }
  if (typeof document !== "undefined" && document.visibilityState === "hidden") {
    return;
  }
  androidNotificationSettingsRefreshPending = false;
  androidNotificationSettingsRefreshInFlight = true;
  try {
    await refreshAndroidNotificationStatus();
    await refreshAndroidKeepAliveStatus();
  } finally {
    androidNotificationSettingsRefreshInFlight = false;
  }
}

function handleAndroidNotificationVisibilityChange() {
  if (document.visibilityState === "visible") {
    void refreshAndroidNotificationStatusAfterSettingsReturn();
  }
}

function handleAndroidNotificationWindowFocus() {
  void refreshAndroidNotificationStatusAfterSettingsReturn();
}

async function restoreAndroidRuntimeState() {
  if (appInfo.value.platform !== "android") {
    return;
  }
  try {
    const result = await getAndroidRuntimeStatus();
    appendLog("bridge.get_android_runtime_status", result);
    if (!result.ok || !result.data) {
      return;
    }
    if (result.data.battery) {
      androidBatteryStatus.value = result.data.battery;
    }
    if (result.data.keep_alive) {
      androidKeepAliveStatus.value = result.data.keep_alive;
    }
    const runtimeTaskId = asString(result.data.task_id).trim();
    const runtimeSnapshot = result.data.task_snapshot;
    taskSessionState.value = asString(result.data.session_state || "idle");
    if (runtimeSnapshot) {
      applyTaskSnapshot(runtimeSnapshot);
      task.taskId = runtimeTaskId || task.taskId;
      if (runtimeSnapshot.runtime_attached === true || taskSessionState.value === "paused_runtime") {
        finishTaskAction();
        setStatus({
          detail: taskSessionState.value === "paused_runtime" ? "已重新接入暂停中的 Android 探测任务，可以继续执行。" : "已重新接入 Android 后台探测任务，界面将继续接收实时进度。",
          title: taskSessionState.value === "paused_runtime" ? "任务已恢复" : "任务重新接入",
          tone: taskSessionState.value === "paused_runtime" ? "cooling" : "running",
        });
        pushActivity("Android 任务已恢复", taskSessionState.value === "paused_runtime" ? "检测到暂停中的原生任务。" : "检测到仍在运行的原生任务。");
        void navigateTo({ mode: "single", view: "dashboard" });
      } else if (taskSessionState.value === "persisted_only") {
        finishTaskAction();
        setStatus({
          detail: "已恢复上次任务快照和结果文件，但原生运行时会话已经结束，需要重新启动任务。",
          title: "已恢复历史结果",
          tone: "warning",
        });
        pushActivity("恢复到已落盘结果", "已读取 Android 任务快照，但当前没有可重连的原生活动会话。");
        void navigateTo({ mode: "single", view: "results" });
      }
      if (runtimeTaskId) {
        await refreshTaskData(runtimeTaskId);
      }
    }
  } catch (error) {
    appendLog("bridge.get_android_runtime_status.failed", error instanceof Error ? error.message : String(error));
  }
}

async function requestBatteryOptimizationExemption(mode: "request" | "settings" | "details" = "request") {
  try {
    const result = await openBatteryOptimizationSettings(mode);
    appendLog("bridge.open_battery_optimization_settings", result);
    if (!result.ok) {
      showToast(result.message || "打开省电策略设置失败", "error");
      return;
    }
    showToast(mode === "request" ? "已打开系统电池优化豁免页" : "已打开电池与后台运行相关设置", "info");
    await refreshAndroidBatteryStatus();
  } catch (error) {
    showToast(error instanceof Error ? error.message : "打开省电策略设置失败", "error");
  }
}

async function requestAndroidNotificationPermission() {
  try {
    const result = await requestNotificationPermission();
    appendLog("bridge.request_notification_permission", result);
    if (result.data) {
      androidNotificationStatus.value = result.data;
    }
    showToast(result.message || (result.ok ? "通知权限已允许" : "通知权限未允许"), result.ok ? "success" : "error");
    await refreshAndroidNotificationStatus();
    await refreshAndroidKeepAliveStatus();
  } catch (error) {
    showToast(error instanceof Error ? error.message : "申请通知权限失败", "error");
  }
}

async function openAndroidNotificationSettings() {
  try {
    const result = await openNotificationSettings();
    appendLog("bridge.open_notification_settings", result);
    if (result.data) {
      androidNotificationStatus.value = result.data;
    }
    if (!result.ok) {
      showToast(result.message || "打开通知权限设置失败", "error");
      return;
    }
    androidNotificationSettingsRefreshPending = true;
    showToast(result.message || "已打开通知权限设置", "info");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "打开通知权限设置失败", "error");
  }
}

async function toggleAndroidKeepAlive(enabled: boolean) {
  try {
    const result = await setKeepAliveEnabled(enabled);
    appendLog("bridge.set_keep_alive_enabled", result);
    if (result.data) {
      androidKeepAliveStatus.value = result.data;
    }
    if (!result.ok) {
      showToast(result.message || "更新通知栏保活失败", "error");
      return;
    }
    showToast(result.message || (enabled ? "通知栏保活已开启" : "通知栏保活已关闭"), "success");
    await refreshAndroidKeepAliveStatus();
  } catch (error) {
    showToast(error instanceof Error ? error.message : "更新通知栏保活失败", "error");
  }
}

async function loadMoreResults() {
  const normalizedTaskId = task.taskId.trim();
  if (!normalizedTaskId || resultRows.value.length >= resultsTotalCount.value) {
    return;
  }
  resultsLoading.value = true;
  try {
    const result = await listTaskResults(
      normalizedTaskId,
      resultSortBy.value,
      resultOrder.value,
      resultFilter.value,
      buildResultFileFallbackPayload(task.exportPath, taskSnapshot.value),
      resultIpFilter.value,
      {
        limit: resultsPageLimit.value,
        offset: resultRows.value.length,
      },
      {
        allowFileFallback: shouldAllowTaskResultFileFallback(taskSnapshot.value, normalizedTaskId),
      },
    );
    appendLog("bridge.list_task_results.more", result);
    if (!result.ok || !result.data) {
      return;
    }
    const nextRows = Array.isArray(result.data.results) ? result.data.results : [];
    resultWorkspaceTaskId.value = normalizedTaskId;
    resultRows.value = [...resultRows.value, ...nextRows];
    resultsTotalCount.value = asCount(result.data.total_count, resultRows.value.length);
  } finally {
    resultsLoading.value = false;
  }
}

async function pauseProbe() {
  if (!task.taskId) {
    return;
  }

  if (taskActionInFlight.value) {
    notifyTaskActionBlocked("pause");
    return;
  }

  beginTaskAction("pause", "", task.taskId);

  loading.value = true;

  try {
    const result = await stopProbe({
      mode: "pause",
      task_id: task.taskId,
    });
    appendLog("bridge.stop_probe", result);

    if (!result.ok) {
      finishTaskAction("pause");
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
    if (status.tone === "failed") {
      finishTaskAction("pause");
    }
    loading.value = false;
  }
}

async function stopPausedTaskForRestart(reason: string) {
  if (!task.taskId || !hasPausedTask.value) {
    return true;
  }
  if (taskActionInFlight.value) {
    notifyTaskActionBlocked("cancel");
    return false;
  }

  beginTaskAction("cancel", "", task.taskId);
  loading.value = true;

  try {
    const result = await stopProbe({
      mode: "cancel",
      task_id: task.taskId,
    });
    appendLog("bridge.stop_probe.cancel", result);

    if (!result.ok) {
      finishTaskAction("cancel");
      setStatus({
        detail: result.message || "终止暂停任务失败。",
        title: "终止失败",
        tone: "failed",
      });
      showToast("终止暂停任务失败", "error");
      return false;
    }

    await reconcileTaskData(task.taskId);
    if (task.active || ["active_runtime", "paused_runtime"].includes(activeTaskSessionState.value)) {
      finishTaskAction("cancel");
      setStatus({
        detail: "终止请求已发送，但旧任务尚未完全退出，请稍后重试。",
        title: "等待旧任务退出",
        tone: "warning",
      });
      pushActivity("终止等待中", "旧任务仍在退出流程中，本次重启已取消。");
      showToast("旧任务仍在退出，请稍后重试", "info");
      return false;
    }

    finishTaskAction("cancel");
    setStatus({
      detail: result.message || reason,
      title: "已终止暂停任务",
      tone: "warning",
    });
    pushActivity("已终止暂停任务", result.message || reason);
    return true;
  } finally {
    loading.value = false;
  }
}

async function continueProbe() {
  if (!task.taskId) {
    return;
  }

  if (taskActionInFlight.value) {
    notifyTaskActionBlocked("resume");
    return;
  }

  if (hasDetachedTaskSnapshot.value) {
    setStatus({
      detail: "当前只恢复了历史快照和结果文件，原生运行时会话已经结束，不能直接继续，请重新启动任务。",
      title: "无法继续历史快照",
      tone: "warning",
    });
    pushActivity("继续被阻止", "仅恢复了历史结果，当前没有可继续的运行时会话。");
    showToast("历史快照不能直接继续", "info");
    void navigateTo({ mode: "single", view: "results" });
    return;
  }

  beginTaskAction("resume", "", task.taskId);

  loading.value = true;

  try {
    const result = await resumeProbe({
      task_id: task.taskId,
    });
    appendLog("bridge.resume_probe", result);

    if (!result.ok) {
      finishTaskAction("resume");
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
    taskSessionState.value = "active_runtime";
    task.active = true;
    pushActivity("请求继续", result.message || "已向桌面端发送继续请求。");
    showToast("已请求继续", "info");
    await reconcileTaskData(task.taskId);
  } finally {
    if (status.tone === "failed") {
      finishTaskAction("resume");
    }
    loading.value = false;
  }
}

async function fetchDnsRecords() {
  if (dnsReadScope.value === "custom" && !dnsReadName.value.trim()) {
    void navigateTo({ mode: "single", view: "dns" });
    showToast("请输入要读取的子域名或记录名", "error");
    return;
  }

  isLoadingDns.value = true;

  try {
    const payload: Record<string, unknown> = {
      config: buildConfigSnapshot(),
      scope: dnsReadScope.value,
    };
    if (dnsReadScope.value === "custom") {
      payload.name = dnsReadName.value.trim();
    }
    if (dnsRecordType.value !== "all") {
      payload.record_type = dnsRecordType.value;
    }

    const result = await listDnsRecords(payload);
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

async function exportCurrentResultsToGitHub() {
  const visibleRows = currentVisibleResultRows();
  if (visibleRows.length === 0) {
    void navigateTo({ mode: "single", view: "results" });
    showToast("没有可导出的测速结果", "error");
    return;
  }

  const githubTopN = nonNegativeCount(resultGitHubTopN.value, 0);

  githubExporting.value = true;
  try {
    const config = normalizeConfigSnapshot(buildConfigSnapshot());
    config.github.top_n = githubTopN;
    config.upload.github.top_n = githubTopN;
    const result = await exportResultsToGitHub({
      config,
      export_path: task.exportPath,
      notification_trigger: "manual_push",
      results: visibleRows,
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

async function pushCurrentResultsToCloudflare() {
  const visibleRows = currentVisibleResultRows();
  if (visibleRows.length === 0) {
    void navigateTo({ mode: "single", view: "results" });
    showToast("没有可推送的测速结果", "error");
    return;
  }

  const recordName = resultCloudflarePushSettings.recordName.trim();
  const routingActive = cloudflareRoutingPushActive.value;
  if (!routingActive && !recordName) {
    void navigateTo({ mode: "single", view: "results" });
    showToast("请先填写 Cloudflare 记录名称", "error");
    return;
  }

  const topN = nonNegativeCount(resultCloudflarePushSettings.topN, 0);
  const selectedRows = limitRowsForQuickPush(visibleRows, topN);
  if (selectedRows.length === 0) {
    void navigateTo({ mode: "single", view: "results" });
    showToast("当前筛选结果没有可推送 IP", "error");
    return;
  }

  saveResultCloudflarePushSettings();
  cloudflarePushing.value = true;
  try {
    const config = normalizeConfigSnapshot(buildConfigSnapshot());
    config.cloudflare.top_n = topN;
    config.upload.cloudflare.top_n = topN;
    if (!routingActive) {
      config.cloudflare.record_name = recordName;
      config.cloudflare.record_type = normalizeResultCloudflareRecordType(resultCloudflarePushSettings.recordType);
      config.cloudflare.routing_enabled = false;
      config.cloudflare.routing_rules = [];
      config.upload.cloudflare.routing_enabled = false;
      config.upload.cloudflare.routing_rules = [];
    }

    const result = await pushDnsRecords({
      config,
      notification_trigger: "manual_push",
      results: selectedRows,
      task_id: task.taskId,
    });
    const data = asRecord(result.data);
    appendLog("bridge.push_results_cloudflare", result);
    probeWarnings.value = result.warnings || [];
    pushWarningTrace(probeWarnings.value);
    if (!result.ok) {
      setStatus({
        detail: result.message || "当前结果推送到 Cloudflare 失败。",
        title: "Cloudflare 推送失败",
        tone: "failed",
      });
      pushActivity("Cloudflare 推送失败", result.message || "未能覆盖目标 DNS 记录。");
      showToast("Cloudflare 推送失败", "error");
      return;
    }

    if (asBoolean(data.routing_enabled || data.routingEnabled, false)) {
      const successTargets = asCount(data.success_targets || data.successTargets);
      const skippedTargets = asCount(data.skipped_targets || data.skippedTargets);
      const failedTargets = asCount(data.failed_targets || data.failedTargets);
      const uploadCount = asCount(data.upload_count || data.uploadCount, selectedRows.length);
      const detail = result.message || `Cloudflare 推送完成：成功 ${successTargets} 个目标，失败 ${failedTargets} 个目标，跳过 ${skippedTargets} 个目标，共推送 ${uploadCount} 条。`;
      const partial = failedTargets > 0;
      setStatus({
        detail,
        title: partial ? "Cloudflare 推送部分完成" : "Cloudflare 推送完成",
        tone: partial ? "partial" : "completed",
      });
      pushActivity(partial ? "Cloudflare 推送部分完成" : "Cloudflare 推送完成", detail);
      showToast(partial ? "Cloudflare 部分目标推送完成" : "已推送到 Cloudflare", partial ? "info" : "success");
      return;
    }

    const summaryRecord = asRecord(data.summary);
    const created = asCount(summaryRecord.created);
    const updated = asCount(summaryRecord.updated);
    const deleted = asCount(summaryRecord.deleted);
    const ignored = asCount(summaryRecord.ignored);
    const uploadCount = asCount(data.upload_count || data.uploadCount, selectedRows.length);
    const detail = `已推送 ${uploadCount} 条到 ${recordName}：创建 ${created}、更新 ${updated}、删除 ${deleted}、忽略 ${ignored}。`;
    setStatus({
      detail,
      title: "Cloudflare 推送完成",
      tone: "completed",
    });
    pushActivity("Cloudflare 推送完成", result.message || detail);
    showToast("已推送到 Cloudflare", "success");
  } finally {
    cloudflarePushing.value = false;
  }
}

async function exportCurrentResultsCSV() {
  const visibleRows = currentVisibleResultRows();
  if (visibleRows.length === 0) {
    void navigateTo({ mode: "single", view: "results" });
    showToast("没有可导出的测速结果", "error");
    return;
  }

  csvExporting.value = true;
  try {
    const defaultFileName = defaultResultsCSVFileName();
    const targetUri = settings.exportTargetUri.trim();
    const targetDir = settings.exportTargetDir.trim();
    if (appInfo.value.platform === "android" && !targetUri) {
      showToast("请先选择 Android SAF 导出目录", "error");
      return;
    }

    const result = await exportResultsCSV({
      config: buildConfigSnapshot(),
      file_name: defaultFileName,
      results: visibleRows,
      task_id: task.taskId,
      ...(targetDir ? { target_dir: targetDir } : {}),
      ...(targetUri ? { target_uri: targetUri } : {}),
    });
    appendLog("bridge.export_results_csv", result);
    const data = asRecord(result.data);
    if (!result.ok) {
      setStatus({
        detail: result.message || "导出当前测速结果 CSV 失败。",
        title: "CSV 导出失败",
        tone: "failed",
      });
      pushActivity("CSV 导出失败", result.message || "未能写入导出目标。");
      showToast("CSV 导出失败", "error");
      return;
    }

    const writtenCount = asCount(data.written_count || data.writtenCount) || visibleRows.length;
    const resolvedFileName = asString(data.file_name || data.fileName).trim() || defaultFileName;
    const resolvedPath = asString(data.path).trim();
    const resolvedTargetURI = asString(data.target_uri || data.targetUri).trim();
    const targetLabel = resolvedPath || resolvedTargetURI || resolvedFileName;
    setStatus({
      detail: targetLabel ? `已导出 ${writtenCount} 条结果到 ${targetLabel}。` : result.message || `已导出 ${writtenCount} 条结果。`,
      title: "CSV 导出完成",
      tone: "completed",
    });
    pushActivity("CSV 导出完成", targetLabel ? `已导出 ${writtenCount} 条结果到 ${targetLabel}。` : `已导出 ${writtenCount} 条结果。`);
    showToast("已导出当前测速结果 CSV", "success");
  } catch (error) {
    setStatus({
      detail: error instanceof Error ? error.message : "导出当前测速结果 CSV 失败。",
      title: "CSV 导出失败",
      tone: "failed",
    });
    showToast(error instanceof Error ? error.message : "CSV 导出失败", "error");
  } finally {
    csvExporting.value = false;
  }
}

async function openHistoryTarget(targetPath: string) {
  if (!targetPath) {
    return;
  }

  await openPath(targetPath);
}

async function runStartupStep(label: string, step: () => Promise<void>) {
  try {
    await step();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    appendLog(`${label}.failed`, message);
    pushActivity("启动步骤失败", message);
    showToast(message || "启动步骤失败", "error");
  }
}

watch(
  () => buildConfigSnapshot(),
  () => {
    scheduleDraftSave();
    applyThemeMode();
  },
  { deep: true },
);

watch(
  resultCloudflarePushSettings,
  () => {
    if (resultCloudflarePushSettingsHydrated) {
      saveResultCloudflarePushSettings();
    }
  },
  { deep: true },
);

watch(
  () => settings.schedulerRunMode,
  (mode) => {
    if (mode === "pipeline") {
      settings.schedulerAutoGithubExport = false;
    }
  },
);

watch(
  isAndroidApp,
  (android) => {
    if (!android) {
      uninstallAndroidViewportTracking();
      return;
    }
    installAndroidViewportTracking();
    applyNavigation({ mode: "single" });
    settings.schedulerRunMode = "probe";
    settings.schedulerPipelineTemplateId = "";
  },
  { immediate: true },
);

watch([() => appMode.value, () => selectedView.value], ([mode, view], [previousMode, previousView]) => {
  if (previousMode === "single" && previousView === "results" && (mode !== "single" || view !== "results")) {
    clearCurrentTaskResultWorkspace({ preserveSnapshot: true });
  }
});

onMounted(async () => {
  window.addEventListener("resize", scheduleViewportSizeRefresh);
  window.addEventListener("beforeunload", handleBeforeUnload);
  window.addEventListener("focus", handleAndroidNotificationWindowFocus);
  document.addEventListener("visibilitychange", handleAndroidNotificationVisibilityChange);
  themeMediaQuery = window.matchMedia?.("(prefers-color-scheme: dark)") || null;
  themeMediaQuery?.addEventListener?.("change", applyThemeMode);
  scheduleThemeRefresh();
  await runStartupStep("viewport.startup", ensureAdaptiveViewportOnStartup);
  appendLog("system.boot", { message: "桌面端调用链已初始化。" });
  pushActivity("桌面端已启动", "等待桌面端返回配置与任务状态。");
  await runStartupStep("probe.listen", async () => {
    removeProbeListener = await listenToProbeEvents((event) => {
      applyProbeEvent(event);
    });
  });
  await runStartupStep("app_info.refresh", refreshAppInfo);
  await runStartupStep("config.refresh", refreshConfig);
  loadResultCloudflarePushSettings();
  loadResultGitHubTopN();
  resultCloudflarePushSettingsHydrated = true;
  await runStartupStep("android_battery.refresh", refreshAndroidBatteryStatus);
  await runStartupStep("android_notification.refresh", refreshAndroidNotificationStatus);
  await runStartupStep("android_keep_alive.refresh", refreshAndroidKeepAliveStatus);
  await runStartupStep("android_runtime.restore", restoreAndroidRuntimeState);
  await runStartupStep("colo_dictionary.refresh", refreshColoDictionaryStatus);
  await runStartupStep("scheduler.refresh", refreshSchedulerStatus);
  await runStartupStep("pipeline_results.refresh", refreshPipelineResults);
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", scheduleViewportSizeRefresh);
  window.removeEventListener("beforeunload", handleBeforeUnload);
  window.removeEventListener("focus", handleAndroidNotificationWindowFocus);
  document.removeEventListener("visibilitychange", handleAndroidNotificationVisibilityChange);
  themeMediaQuery?.removeEventListener?.("change", applyThemeMode);
  if (viewportResizeTimer !== undefined) {
    window.clearTimeout(viewportResizeTimer);
  }
  if (draftSaveTimer !== undefined) {
    window.clearTimeout(draftSaveTimer);
  }
  if (themeTimer !== undefined) {
    window.clearTimeout(themeTimer);
  }
  uninstallAndroidViewportTracking();
  void flushDraftSave();
  removeProbeListener?.();
});
</script>

<template>
  <DesktopShell :app-mode="appMode" :route-title="appMode === 'workflow' ? '工作流' : routeTitles[selectedView]" :selected-view="selectedView" :views="views" @change-app-mode="changeAppMode" @change-view="changeSingleView">
    <WorkflowView
      v-if="!isAndroidApp && appMode === 'workflow'"
      :active-pipeline-id="activePipelineId"
      :can-start-pipeline="canStartPipeline"
      :current-result-rows="resultRows"
      :format-timestamp="formatAppTimestamp"
      :fit-request-key="workflowFitRequestKey"
      :loading="loading"
      :node-catalog="pipelineNodeCatalog"
      :pipeline-results="pipelineResults"
      :pipeline-workspace="pipelineWorkspace"
      platform="desktop"
      :process-trace="processTrace"
      :scheduler-state="workflowSchedulerState"
      :scheduler-status="schedulerStatus"
      :source-profiles="sourceProfiles"
      :workspace-dirty="pipelineWorkspaceDirty"
      @activate-template="setActivePipelineTemplate"
      @clear-process="clearProcessTrace"
      @create-template="createPipelineTemplate"
      @delete-template="removePipelineTemplate"
      @open-dashboard="openDashboardView"
      @save-scheduler="saveWorkflowSchedulerFromView"
      @save-workspace="savePipelineWorkspaceFromView"
      @start-pipeline="launchPipeline"
    />

    <DashboardView
      v-else-if="selectedView === 'dashboard'"
      :activity-feed="activityFeed"
      :can-pause-task="canPauseTask"
      :can-resume-task="canResumeTask"
      :can-start-task="canStartTask"
      :download-speed-state="downloadSpeedState"
      :export-history="exportHistory"
      :format-timestamp="formatAppTimestamp"
      :has-active-task="hasActiveTask"
      :loading="loading"
      platform="desktop"
      :process-trace="processTrace"
      :probe-config="{ portPolicy: settings.probePortPolicy, tcpPort: settings.probeTcpPort }"
      :progress-percent="progressPercent"
      :status-label="dashboardStatusLabel"
      :status-tone="status.tone"
      :summary="summary"
      :task="task"
      :task-snapshot="taskSnapshot"
      @clear-process="clearProcessTrace"
      @open-history-target="openHistoryTarget"
      @pause="pauseProbe"
      @resume="continueProbe"
      @start="launchProbe"
    />

    <ResultsView
      v-else-if="selectedView === 'results'"
      :can-rerun-task="canStartTask"
      :has-active-task="hasActiveTask"
      :loading="loading"
      :format-timestamp="formatAppTimestamp"
      platform="desktop"
      :result-filter="resultFilter"
      :result-filter-options="resultFilterOptions"
      :result-ip-filter="resultIpFilter"
      :result-ip-filter-options="resultIpFilterOptions"
      :result-order="resultOrder"
      :result-rows="resultRows"
      :results-total-count="resultsTotalCount"
      :result-sort-by="resultSortBy"
      :result-sort-options="resultSortOptions"
      :results-loading="resultsLoading"
      :csv-exporting="csvExporting"
      :cloudflare-pushing="cloudflarePushing"
      :github-exporting="githubExporting"
      :github-top-n="resultGitHubTopN"
      :cloudflare-push-settings="resultCloudflarePushSettings"
      :cloudflare-routing-active="cloudflareRoutingPushActive"
      :cloudflare-routing-rule-count="activeCloudflareRoutingRuleCount"
      :summary="summary"
      :task="task"
      :task-snapshot="taskSnapshot"
      @copy-address="copyAddress"
      @export-current-results-csv="exportCurrentResultsCSV"
      @export-github="exportCurrentResultsToGitHub"
      @push-cloudflare="pushCurrentResultsToCloudflare"
      @load-more-results="loadMoreResults"
      @refresh-results="refreshCurrentTaskData"
      @rerun-address="rerunSingleAddress"
      @update-cloudflare-push-settings="updateResultCloudflarePushSettings"
      @update-github-top-n="updateResultGitHubTopN"
      @update-filter="updateResultFilter"
      @update-ip-filter="updateResultIpFilter"
      @update-order="updateResultOrder"
      @update-sort="updateResultSort"
    />

    <SourcesView
      v-else-if="selectedView === 'sources'"
      :accepted="summary.accepted"
      :invalid="summary.invalid"
      :format-timestamp="formatAppTimestamp"
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
      @auto-save="autoSaveSourcePage('interaction')"
      @delete-source-profile="removeSourceProfile"
      @detect-source-name="applyDetectedSourceName"
      @process-colo-dictionary="processLocalColoDictionary"
      @refresh-colo-dictionary="refreshColoDictionary"
      @fetch-source="inspectSource($event, 'fetch')"
      @preview="inspectSource($event, 'preview')"
      @preview-request="inspectSource($event, 'preview')"
      @remove="removeSource"
      @save-source-profile="saveCurrentSourceProfile"
      @select-file="selectSourceFile"
      @switch-source-profile="switchToSourceProfile"
      @update-current-source-profile="updateActiveSourceProfile"
    />

    <SettingsView
      v-else-if="selectedView === 'settings'"
      :loading="loading"
      :app-info="appInfo"
      :android-battery-status="androidBatteryStatus"
      :android-keep-alive-status="androidKeepAliveStatus"
      :android-notification-status="androidNotificationStatus"
      :format-timestamp="formatAppTimestamp"
      :masked-token-hint="maskedTokenHint"
      :utc-offset-label="utcOffsetLabel()"
      platform="desktop"
      :pipeline-workspace="pipelineWorkspace"
      :settings="settings"
      :show-token="showToken"
      :github-testing="githubTesting"
      :telegram-testing="telegramTesting"
      :enabled-pipeline-profile-count="enabledPipelineProfileCount"
      :pipeline-profile-count="pipelineProfiles.items.length"
      :scheduler-status="schedulerStatus"
      :storage="storageStatus"
      :update-state="updateState"
      :viewport-adaptive-active="viewportAdaptiveActive"
      :viewport-presets="VIEWPORT_PRESETS"
      :viewport-runtime-supported="viewportRuntimeSupported"
      :viewport-size="viewportSize"
      :viewport-switching="viewportSwitching"
      @auto-save="autoSaveSettings('interaction')"
      @apply-viewport-preset="applyViewportPreset"
      @open-battery-settings="requestBatteryOptimizationExemption"
      @set-keep-alive-enabled="toggleAndroidKeepAlive"
      @open-notification-settings="openAndroidNotificationSettings"
      @request-notification-permission="requestAndroidNotificationPermission"
      @backup-config-webdav="backupToWebDAV"
      @check-storage-health="checkCurrentStorageHealth"
      @check-update="checkOnlineUpdate"
      @export-config="exportConfigToFile"
      @export-diagnostic-package="exportCurrentDiagnosticPackage"
      @export-debug-log="exportCurrentDebugLog"
      @import-config="importConfigFromFile"
      @open-log-directory="openCurrentLogDirectory"
      @open-storage-dir="openStorageDirectory"
      @open-release-page="openOnlineReleasePage"
      @scheduler-daily-times-blur="handleSchedulerDailyTimesBlur"
      @select-export-target="selectExportTarget"
      @restore-config-webdav="restoreFromWebDAV"
      @install-update="installOnlineUpdate"
      @test-github-export="testGitHubExportSettings"
      @test-telegram-notification="testTelegramNotificationSettings"
      @test-webdav="testWebDAVSettings"
      @toggle-token="showToken = !showToken"
    />

    <DnsView v-else :dns-records="dnsRecords" :is-loading-dns="isLoadingDns" platform="desktop" v-model:dns-read-name="dnsReadName" v-model:dns-read-scope="dnsReadScope" v-model:dns-record-type="dnsRecordType" @fetch="fetchDnsRecords" />
  </DesktopShell>

  <MobileShell :app-mode="appMode" :hide-workflow="isAndroidApp" :route-title="appMode === 'workflow' ? '工作流' : routeTitles[selectedView]" :selected-view="selectedView" :views="views" @change-app-mode="changeAppMode" @change-view="changeSingleView">
    <WorkflowView
      v-if="!isAndroidApp && appMode === 'workflow'"
      :active-pipeline-id="activePipelineId"
      :can-start-pipeline="canStartPipeline"
      :current-result-rows="resultRows"
      :format-timestamp="formatAppTimestamp"
      :fit-request-key="workflowFitRequestKey"
      :loading="loading"
      :node-catalog="pipelineNodeCatalog"
      :pipeline-results="pipelineResults"
      :pipeline-workspace="pipelineWorkspace"
      platform="mobile"
      :process-trace="processTrace"
      :scheduler-state="workflowSchedulerState"
      :scheduler-status="schedulerStatus"
      :source-profiles="sourceProfiles"
      :workspace-dirty="pipelineWorkspaceDirty"
      @activate-template="setActivePipelineTemplate"
      @clear-process="clearProcessTrace"
      @create-template="createPipelineTemplate"
      @delete-template="removePipelineTemplate"
      @open-dashboard="openDashboardView"
      @save-scheduler="saveWorkflowSchedulerFromView"
      @save-workspace="savePipelineWorkspaceFromView"
      @start-pipeline="launchPipeline"
    />

    <DashboardView
      v-else-if="selectedView === 'dashboard'"
      :activity-feed="activityFeed"
      :can-pause-task="canPauseTask"
      :can-resume-task="canResumeTask"
      :can-start-task="canStartTask"
      :download-speed-state="downloadSpeedState"
      :export-history="exportHistory"
      :format-timestamp="formatAppTimestamp"
      :has-active-task="hasActiveTask"
      :loading="loading"
      platform="mobile"
      :process-trace="processTrace"
      :probe-config="{ portPolicy: settings.probePortPolicy, tcpPort: settings.probeTcpPort }"
      :progress-percent="progressPercent"
      :status-label="dashboardStatusLabel"
      :status-tone="status.tone"
      :summary="summary"
      :task="task"
      :task-snapshot="taskSnapshot"
      @clear-process="clearProcessTrace"
      @open-history-target="openHistoryTarget"
      @pause="pauseProbe"
      @resume="continueProbe"
      @start="launchProbe"
    />

    <ResultsView
      v-else-if="selectedView === 'results'"
      :can-rerun-task="canStartTask"
      :has-active-task="hasActiveTask"
      :loading="loading"
      :format-timestamp="formatAppTimestamp"
      platform="mobile"
      :result-filter="resultFilter"
      :result-filter-options="resultFilterOptions"
      :result-ip-filter="resultIpFilter"
      :result-ip-filter-options="resultIpFilterOptions"
      :result-order="resultOrder"
      :result-rows="resultRows"
      :results-total-count="resultsTotalCount"
      :result-sort-by="resultSortBy"
      :result-sort-options="resultSortOptions"
      :results-loading="resultsLoading"
      :csv-exporting="csvExporting"
      :cloudflare-pushing="cloudflarePushing"
      :github-exporting="githubExporting"
      :github-top-n="resultGitHubTopN"
      :cloudflare-push-settings="resultCloudflarePushSettings"
      :cloudflare-routing-active="cloudflareRoutingPushActive"
      :cloudflare-routing-rule-count="activeCloudflareRoutingRuleCount"
      :summary="summary"
      :task="task"
      :task-snapshot="taskSnapshot"
      @copy-address="copyAddress"
      @export-current-results-csv="exportCurrentResultsCSV"
      @export-github="exportCurrentResultsToGitHub"
      @push-cloudflare="pushCurrentResultsToCloudflare"
      @load-more-results="loadMoreResults"
      @refresh-results="refreshCurrentTaskData"
      @rerun-address="rerunSingleAddress"
      @update-cloudflare-push-settings="updateResultCloudflarePushSettings"
      @update-github-top-n="updateResultGitHubTopN"
      @update-filter="updateResultFilter"
      @update-ip-filter="updateResultIpFilter"
      @update-order="updateResultOrder"
      @update-sort="updateResultSort"
    />

    <SourcesView
      v-else-if="selectedView === 'sources'"
      :accepted="summary.accepted"
      :invalid="summary.invalid"
      :format-timestamp="formatAppTimestamp"
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
      @auto-save="autoSaveSourcePage('interaction')"
      @delete-source-profile="removeSourceProfile"
      @detect-source-name="applyDetectedSourceName"
      @process-colo-dictionary="processLocalColoDictionary"
      @refresh-colo-dictionary="refreshColoDictionary"
      @fetch-source="inspectSource($event, 'fetch')"
      @preview="inspectSource($event, 'preview')"
      @preview-request="inspectSource($event, 'preview')"
      @remove="removeSource"
      @save-source-profile="saveCurrentSourceProfile"
      @select-file="selectSourceFile"
      @switch-source-profile="switchToSourceProfile"
      @update-current-source-profile="updateActiveSourceProfile"
    />

    <SettingsView
      v-else-if="selectedView === 'settings'"
      :loading="loading"
      :app-info="appInfo"
      :android-battery-status="androidBatteryStatus"
      :android-keep-alive-status="androidKeepAliveStatus"
      :android-notification-status="androidNotificationStatus"
      :format-timestamp="formatAppTimestamp"
      :masked-token-hint="maskedTokenHint"
      :utc-offset-label="utcOffsetLabel()"
      platform="mobile"
      :pipeline-workspace="pipelineWorkspace"
      :settings="settings"
      :show-token="showToken"
      :github-testing="githubTesting"
      :telegram-testing="telegramTesting"
      :enabled-pipeline-profile-count="enabledPipelineProfileCount"
      :pipeline-profile-count="pipelineProfiles.items.length"
      :scheduler-status="schedulerStatus"
      :storage="storageStatus"
      :update-state="updateState"
      :viewport-adaptive-active="viewportAdaptiveActive"
      :viewport-presets="VIEWPORT_PRESETS"
      :viewport-runtime-supported="viewportRuntimeSupported"
      :viewport-size="viewportSize"
      :viewport-switching="viewportSwitching"
      @auto-save="autoSaveSettings('interaction')"
      @apply-viewport-preset="applyViewportPreset"
      @open-battery-settings="requestBatteryOptimizationExemption"
      @set-keep-alive-enabled="toggleAndroidKeepAlive"
      @open-notification-settings="openAndroidNotificationSettings"
      @request-notification-permission="requestAndroidNotificationPermission"
      @backup-config-webdav="backupToWebDAV"
      @check-storage-health="checkCurrentStorageHealth"
      @check-update="checkOnlineUpdate"
      @export-config="exportConfigToFile"
      @export-diagnostic-package="exportCurrentDiagnosticPackage"
      @export-debug-log="exportCurrentDebugLog"
      @import-config="importConfigFromFile"
      @open-log-directory="openCurrentLogDirectory"
      @open-storage-dir="openStorageDirectory"
      @open-release-page="openOnlineReleasePage"
      @scheduler-daily-times-blur="handleSchedulerDailyTimesBlur"
      @select-export-target="selectExportTarget"
      @restore-config-webdav="restoreFromWebDAV"
      @install-update="installOnlineUpdate"
      @test-github-export="testGitHubExportSettings"
      @test-telegram-notification="testTelegramNotificationSettings"
      @test-webdav="testWebDAVSettings"
      @toggle-token="showToken = !showToken"
    />

    <DnsView v-else :dns-records="dnsRecords" :is-loading-dns="isLoadingDns" platform="mobile" v-model:dns-read-name="dnsReadName" v-model:dns-read-scope="dnsReadScope" v-model:dns-record-type="dnsRecordType" @fetch="fetchDnsRecords" />
  </MobileShell>

  <div v-if="androidSelectPicker.open" class="android-select-overlay" role="presentation" @click.self="closeAndroidSelectPicker">
    <section class="android-select-sheet" role="dialog" aria-modal="true" :aria-label="androidSelectPicker.title">
      <div class="android-select-header">
        <h2 class="truncate text-base font-semibold">{{ androidSelectPicker.title }}</h2>
        <button type="button" class="android-select-close" aria-label="关闭" @click="closeAndroidSelectPicker">×</button>
      </div>
      <div class="android-select-options" role="listbox">
        <button
          v-for="option in androidSelectPicker.options"
          :key="`${option.value}:${option.label}`"
          type="button"
          class="android-select-option"
          :class="option.selected || option.value === androidSelectPicker.value ? 'android-select-option-active' : ''"
          :disabled="option.disabled"
          role="option"
          :aria-selected="option.selected || option.value === androidSelectPicker.value"
          @click="chooseAndroidSelectOption(option)"
        >
          <span class="truncate">{{ option.label }}</span>
          <span v-if="option.selected || option.value === androidSelectPicker.value" class="android-select-check">✓</span>
        </button>
      </div>
    </section>
  </div>

  <ToastStack :toasts="toasts" />
</template>
