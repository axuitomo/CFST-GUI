<script setup vapor lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { EventsOn, isWailsDesktopHost } from "./lib/wailsRuntime";

import { clearDownloadSpeedRows, downloadSpeedRowTtlMs, upsertDownloadSpeedRow } from "./lib/downloadSpeedRows";
import {
  backupConfigToWebDAV,
  checkForUpdates,
  checkStorageHealth,
  consumeDroppedSources,
  discardDraft,
  deleteSourceProfile,
  downloadAndInstallUpdate,
  deriveTaskStateFromProbeEvent,
  exportConfigArchive,
  exportDebugLog,
  exportDiagnosticPackage,
  exportResultsCSV,
  exportResultsToGitHub,
  getAndroidRuntimeStatus,
  getTaskSnapshot,
  getAppInfo,
  importConfigArchive,
  isMaskedTokenValue,
  listenToProbeEvents,
  listDnsRecords,
  listDnsSubdomains,
  listTaskSnapshots,
  listTaskResults,
  loadColoDictionaryStatus,
  loadConfig,
  loadSchedulerStatus,
  normalizeConfigSnapshot,
  normalizeDnsRecords,
  normalizeSourceProfileStore,
  openLogDirectory,
  openReleasePage,
  openPath,
  previewSource,
  processColoDictionary,
  pushDnsRecords,
  resumeProbe,
  restoreConfigFromWebDAV,
  rebindProbeEventListener,
  runDnsSelfTest,
  showAndroidNotification,
  requestNotificationPermission,
  saveConfig,
  saveDraft,
  saveSourceProfile,
  selectPath,
  startProbe,
  stopProbe,
  summarizeTraceDiagnostics,
  switchSourceProfile,
  testDoH,
  testGitHubExport,
  testTelegramNotification,
  testWebDAV,
  updateColoDictionary,
  updateCurrentSourceProfile,
  type ColoDictionaryStatus,
  type AppInfo,
  type ColoFilterMode,
  type ConfigSnapshot,
  type SourceConfig,
  type DnsReadProviderOption,
  type DnsRecordSnapshot,
  type DnsSelfTestProviderOption,
  type DnsSelfTestResult,
  type DNSProviderSnapshot,
  type DNSSubdomainSnapshot,
  type PathSelectionPayload,
  type ProbeEventEnvelope,
  type ProbeResult,
  type ProbeResultIPFilter,
  type ProbeResultOrder,
  type ProbeResultSortBy,
  type ProbeStrategy,
  type SourceProfileStore,
  type SourcePreviewPayload,
  type SourceIPMode,
  type SourceKind,
  type StorageStatus,
  type SchedulerStatus,
  type TaskSnapshot,
  type TaskTone,
  type UpdateInfo,
} from "./lib/bridge";
import { detectSourceNameFromUrl, isDefaultSourceName } from "./lib/sourceNames";
import { resolveSourceGroupIPMode, sourceInputIssue, sourcePreviewKey, type SourceGroupIPMode } from "./lib/sourceGroups";
import { readDashboardCache, writeDashboardCache } from "./lib/dashboardCache";
import { ACTIVITY_FEED_LIMIT } from "./lib/activityFeed";
import { classifyProbeEventSequence } from "./lib/probeEventSequence";
import { formatUTCOffsetLabel, formatTimestampWithUTCOffset, normalizeUTCOffsetMinutes } from "./lib/time";
import { shouldRefreshProgress } from "./lib/mobileProgressRefresh";
import { createSettingsAutoSaver } from "./lib/settingsAutoSave";
import { stageTitle, taskActionLabel, type TaskActionKind } from "./lib/labels";
import { asCount, asRecord, asString, asBoolean, asNumber, positiveCount, normalizeCloudflareTTL, minimumCount, boundedCount, normalizeDownloadHTTPProtocol, nonNegativeCount, schedulerTriggerModeFromSnapshot, schedulerDailyTimesFromText, normalizeResultCloudflareRecordType, nonNegativeNumber, clampedNumber, asNullableNumber, optionalNumberForPayload } from "./lib/valueCoerce";

import { createDefaultSettingsForm, DEFAULT_ALIYUN_TTL, DEFAULT_CLOUDFLARE_TTL, DEFAULT_DEBUG_LOG_FORMAT, DEFAULT_DNSHE_TTL, DEFAULT_DNSPOD_TTL, DEFAULT_DOH_URL, DEFAULT_ECH_DOMAIN, DEFAULT_FILE_TEST_URL, DEFAULT_HTTPING_STATUS_CODE, DEFAULT_MAX_LOSS_RATE, DEFAULT_PROBE_USER_AGENT, DEFAULT_SOURCE_POOL_LIMIT, MAX_LOSS_RATE, MAX_SOURCE_POOL_LIMIT, MIN_PROBE_PING_TIMES, type CloudflareRoutingRuleForm, type DNSProviderKind, type DnsProviderForm } from "./lib/settingsForm";
import { type HistoryEntry, type ProcessTraceEntry, type ResultCloudflareRecordType, type ViewName } from "./lib/appTypes";
import { useProbeTask } from "./composables/useProbeTask";
import { useActivityFeed } from "./composables/useActivityFeed";
import { useToasts } from "./composables/useToasts";
import { useTheme } from "./composables/useTheme";
import { useNavigation } from "./composables/useNavigation";
import { useViewport, VIEWPORT_PRESETS } from "./composables/useViewport";
import { useAndroidSelectPicker } from "./composables/useAndroidSelectPicker";
import { useDownloadSpeed } from "./composables/useDownloadSpeed";
import { useAndroidRuntime } from "./composables/useAndroidRuntime";
import DesktopShell from "./components/layout/DesktopShell.vue";
import MobileShell from "./components/layout/MobileShell.vue";
import ToastStack from "./components/ui/ToastStack.vue";
import DashboardView from "./views/DashboardView.vue";
// 视图分包：首屏只解析看板，其余视图按需加载并在空闲时预取，避免一次解析全部模板。
const dnsViewImport = () => import("./views/DnsView.vue");
const resultsViewImport = () => import("./views/ResultsView.vue");
const settingsViewImport = () => import("./views/SettingsView.vue");
const sourcesViewImport = () => import("./views/SourcesView.vue");
const DnsView = defineAsyncComponent(dnsViewImport);
const ResultsView = defineAsyncComponent(resultsViewImport);
const SettingsView = defineAsyncComponent(settingsViewImport);
const SourcesView = defineAsyncComponent(sourcesViewImport);

interface ResultCloudflarePushSettings {
  providerId: string;
  recordName: string;
  recordType: ResultCloudflareRecordType;
  topN: number;
}

const ACTIVE_PROBE_MESSAGE = "当前已有探测任务运行或暂停，请完成后再启动新任务。";

interface SourceDraft extends SourceConfig {}

interface SourcePreviewState {
  action: string;
  entries: string[];
  invalidCount: number;
  portSummary: Record<string, unknown> | null;
  totalCount: number;
  updatedAt: string;
  warnings: string[];
}

const views: Array<{ id: ViewName; title: string; copy: string; shortLabel: string }> = [
  { id: "dashboard", title: "任务看板", copy: "运行状态、进度条与测试进程", shortLabel: "看板" },
  { id: "results", title: "当前结果", copy: "本次测速结果、排序与导出位置", shortLabel: "结果" },
  { id: "sources", title: "输入源", copy: "本次任务要读的输入源、启用状态与 IP 模式", shortLabel: "来源" },
  { id: "settings", title: "系统配置", copy: "Cloudflare、导出与探测参数", shortLabel: "配置" },
  { id: "dns", title: "DNS 读取", copy: "读取当前域名与子域名记录", shortLabel: "DNS" },
];

const routeTitles: Record<ViewName, string> = {
  dashboard: "任务看板",
  dns: "DNS 记录读取",
  results: "当前测速结果",
  settings: "系统配置",
  sources: "输入源",
};
const RESULT_CLOUDFLARE_PUSH_SETTINGS_KEY = "cfst.result.cloudflarePushSettings.v1";
const RESULT_GITHUB_TOP_N_KEY = "cfst.result.githubTopN.v1";
const EXPORT_HISTORY_LIMIT = 50;

const selectedView = ref<ViewName>("dashboard");
const { activityFeed, pushActivity } = useActivityFeed();
const configPath = ref("");
const dnsReadName = ref("");
const dnsReadProviderId = ref("");
const dnsReadScope = ref<"zone" | "configured" | "custom">("zone");
const dnsRecordType = ref<"all" | "A" | "AAAA">("all");
const dnsRecords = ref<DnsRecordSnapshot[]>([]);
const dnsSubdomainErrors = ref<Record<string, string>>({});
const dnsSubdomainsByProvider = ref<Record<string, DNSSubdomainSnapshot[]>>({});
const dnsSubdomainsLoading = ref<Record<string, boolean>>({});
const dnsSelfTesting = ref(false);
const dnsSelfTestResult = ref<DnsSelfTestResult | null>(null);
const exportHistory = ref<HistoryEntry[]>([]);
const isLoadingDns = ref(false);
const dohTesting = ref(false);
const dohTestResult = ref<{ message: string; ok: boolean } | null>(null);
const loading = ref(false);
// 只写入、不参与渲染：用普通有界数组，避免每个事件都触发响应式追踪。
const logs: Array<{ event: string; payload: unknown; ts: string }> = [];
const maskedTokenHint = ref("");
const processTrace = ref<ProcessTraceEntry[]>([]);
const probeWarnings = ref<string[]>([]);

const resultIpFilter = ref<ProbeResultIPFilter>("all");
const resultOrder = ref<ProbeResultOrder>("desc");
const resultRows = ref<ProbeResult[]>([]);
const taskResultCSVPath = ref("");
const resultWorkspaceTaskId = ref("");
const resultSortBy = ref<ProbeResultSortBy>("download");
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
  providerId: "",
  recordName: "",
  recordType: "ALL",
  topN: 5,
});
const showToken = ref(false);
// The two shells are CSS-hidden duplicates; mounting only the active one keeps
// every view (and its DOM) from existing twice.
const sourceSeed = ref(0);
const sourcePreviewStates = reactive<Record<string, SourcePreviewState>>({});
const sourcePreviewErrors = reactive<Record<string, string>>({});
const sourceRequestStates = reactive<Record<string, string>>({});
const coloDictionaryStatus = ref<ColoDictionaryStatus | null>(null);
const coloDictionaryProcessing = ref(false);
const coloDictionaryUpdating = ref(false);
const {
  activeTaskSessionState,
  beginTaskAction,
  canCancelTask,
  canPauseTask,
  canResumeTask,
  canStartTask,
  dashboardStatusLabel,
  finishTaskAction,
  hasActiveTask,
  hasDetachedTaskSnapshot,
  hasPausedTask,
  mcisProgress,
  progressPercent,
  status,
  summary,
  task,
  taskActionInFlight,
  taskActionState,
  taskSessionState,
  taskSnapshot,
  startupSyncing,
} = useProbeTask();

// 乐观 UI：挂载前用上次缓存的看板展示状态填充首帧，冷启动时不再先出现一块空看板。
// 只填展示数据（状态、统计、动态、任务快照），不填运行态：任务动作在 startupSyncing 期间
// 保持禁用，缓存值不需要专门清理：真实状态到达时会逐项覆盖它。
let dashboardCacheTimer: number | undefined;
let dashboardCachePending = false;

function restoreDashboardCache() {
  const cached = readDashboardCache();
  if (!cached) {
    return;
  }
  Object.assign(status, cached.status);
  Object.assign(summary, cached.summary);
  activityFeed.value = cached.activityFeed;
  // 快照用于展示任务上下文（端口等）；任务号只从它里面取，没有快照就不显示任务号，
  // 不让缓存里的任务号比快照活得更久。
  taskSnapshot.value = cached.taskSnapshot;
  task.taskId = cached.taskSnapshot?.task_id || "";
  startupSyncing.value = true;
}

// 桥调用永久挂起时的兜底：到点就结束乐观期，不留一个永远点不动的界面。
const startupTimedOut = ref(false);
const STARTUP_SYNC_TIMEOUT_MS = 8000;

// 启动同步收尾：乐观期结束，任务动作按真实状态重新计算。缓存的展示值不需要在这里清空，
// 真实状态到达时已经逐项覆盖（状态由配置加载覆盖，任务与统计由任务快照覆盖）。
// 乐观期内所有变化都被节流写入跳过，这里补一次，让缓存与同步结束后的看板一致。
function finishStartupSync() {
  startupSyncing.value = false;

  scheduleDashboardCacheSave();
}

// 启动与手动重试共用同一条兜底：乐观期内到点没结束，就结束乐观期并重新给出重试入口。
// 乐观期已经正常结束时 startupSyncing 是 false，这次兜底让位，避免同步结束后误报「读取配置时间较长」。
function armStartupFallback() {
  window.setTimeout(() => {
    if (!startupSyncing.value) {
      return;
    }
    startupTimedOut.value = true;
    finishStartupSync();
    void refreshConfig();
  }, STARTUP_SYNC_TIMEOUT_MS);
}

async function retryStartupSync() {
  startupTimedOut.value = false;
  startupSyncing.value = true;
  armStartupFallback();
  await Promise.all([refreshAppInfo(), refreshConfig()]);
  finishStartupSync();
}

// 写缓存要节流：探测事件会高频改动 task 与 activityFeed，逐次写 localStorage 会拖慢主线程。
function scheduleDashboardCacheSave() {
  if (startupSyncing.value || dashboardCachePending) {
    return;
  }
  dashboardCachePending = true;
  dashboardCacheTimer = window.setTimeout(() => {
    dashboardCacheTimer = undefined;
    dashboardCachePending = false;
    writeDashboardCache({
      activityFeed: activityFeed.value.slice(0, ACTIVITY_FEED_LIMIT),
      status: { ...status },
      summary: { ...summary },
      taskSnapshot: taskSnapshot.value,
      updatedAt: new Date().toISOString(),
    });
  }, 1000);
}

restoreDashboardCache();
const schedulerStatus = ref<SchedulerStatus | null>(null);
const { showToast, toasts } = useToasts();
const storageStatus = ref<StorageStatus | null>(null);
const androidLogExportTargetUri = ref("");
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

const sourceGroupIPMode = ref<SourceGroupIPMode>("traverse");
const sourceGroupColoFilter = ref("");
const sourceGroupColoFilterMode = ref<ColoFilterMode | "">("allow");
const sources = ref<SourceDraft[]>([]);

const settings = reactive(createDefaultSettingsForm());
const { applyThemeMode, detachThemeMediaListener, installThemeTracking, stopThemeRefreshTimer } = useTheme(settings);
const { applyNavigation, changeSingleView, navigateTo } = useNavigation({
  autoSaveSettings,
  configHydrated: () => configHydrated,
  draftRestoring: () => draftRestoring,
  isAndroidApp: () => isAndroidApp.value,
  saveSourcePageBeforeLeave,
  selectedView,
  sourcePageHasUnsavedChanges,
});
const { applyViewportPreset, desktopLayout, ensureAdaptiveViewportOnStartup, isViewportRuntimeSupported, scheduleViewportSizeRefresh, stopViewportResizeTimer, viewportAdaptiveActive, viewportSize, viewportSwitching } = useViewport({ appendLog, showToast });
const { androidSelectPicker, chooseAndroidSelectOption, closeAndroidSelectPicker, installAndroidViewportTracking, uninstallAndroidViewportTracking } = useAndroidSelectPicker({ isAndroidApp: () => isAndroidApp.value });
const { activeDownloadConcurrency, activeDownloadEnabled, activeDownloadSampleIntervalMs, captureDownloadSpeedTaskConfig, downloadSpeedRows, downloadSpeedState, ensureDownloadSpeedRowPruneTimer, isDownloadSpeedTaskConfigCaptured, resetDownloadSpeedState, stopDownloadSpeedRowPruneTimer } = useDownloadSpeed({ clearDownloadSpeedTraceEntries });
const { androidBatteryStatus, androidKeepAliveStatus, androidNotificationStatus, handleAndroidNotificationVisibilityChange, handleAndroidNotificationWindowFocus, openAndroidNotificationSettings, refreshAndroidBatteryStatus, refreshAndroidKeepAliveStatus, refreshAndroidNotificationStatus, requestAndroidNotificationPermission, requestBatteryOptimizationExemption, toggleAndroidKeepAlive } = useAndroidRuntime({ appendLog, isAndroidApp: () => isAndroidApp.value, showToast });

// 结果页按当前选中的推送平台展示分流状态；Cloudflare 只是其中一个平台。
const activeCloudflareRoutingRuleCount = computed(() => {
  const target = resultPushTargetProvider();
  if (!target?.routingEnabled) {
    return 0;
  }
  return target.routingRules.filter((rule) => Boolean(rule.enabled) && rule.recordName.trim()).length;
});
const cloudflareRoutingPushActive = computed(() => activeCloudflareRoutingRuleCount.value > 0);

let removeProbeListener: (() => void) | null = null;
let removeDroppedSourcesListener: (() => void) | null = null;
let processTraceId = 0;
let lastMobileProgressRefreshAt = 0;
let lastMobileProgressStage = "";

function shouldRefreshProgressUI(stage: string, completed: number, total: number) {
  if (!isAndroidApp.value) {
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
let snapshotRefreshInFlight = false;
let snapshotRefreshPending = false;
let snapshotRefreshQueuedTaskId = "";
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

// Android WebView 未必触发 beforeunload，挂起的合并保存要在 pagehide/隐藏时补发。
function flushPendingSettingsSave() {
  if (settingsAutoSaver.takePending()) void runAutoSaveSettings("pagehide");
}

function handleBeforeUnload(event: BeforeUnloadEvent) {
  void autoSaveSettings("beforeunload");
  void autoSaveSourcePage("beforeunload");
  void flushDraftSave();
  if (currentSnapshotSignature() !== lastSavedSnapshotSignature && currentSnapshotSignature() !== lastDraftSnapshotSignature) {
    event.preventDefault();
    event.returnValue = "";
  }
}

const sourcePayloads = computed(() =>
  sources.value.map((source, index) => ({
    colo_filter: sourceGroupColoFilterMode.value ? sourceGroupColoFilter.value.trim() : source.colo_filter.trim(),
    colo_filter_mode: sourceGroupColoFilterMode.value || source.colo_filter_mode,
    content: source.content.trim(),
    enabled: source.enabled,
    id: source.id,
    ip_limit: -1,
    ip_mode: sourceGroupIPMode.value || source.ip_mode,
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
const noSources = computed(() => preparedSources.value.length === 0);
// 配置写盘的可感知回显：自动保存走 silentSuccess，状态栏不动，头部这个角标是唯一的回声。
const saveState = ref<"error" | "idle" | "saved" | "saving">("idle");
let saveStateTimer: number | undefined;

// "已保存"只留几秒：来源页是失焦才存，一直挂着会在用户继续编辑时撒谎。失败状态常驻。
function markSaveState(state: "error" | "saved" | "saving") {
  if (saveStateTimer !== undefined) {
    window.clearTimeout(saveStateTimer);
    saveStateTimer = undefined;
  }
  saveState.value = state;
  if (state === "saved") {
    saveStateTimer = window.setTimeout(() => {
      saveState.value = "idle";
      saveStateTimer = undefined;
    }, 4000);
  }
}
const saveBlockedByMaskedToken = computed(() =>
  settings.dnsProviders.some((entry) => {
    if (entry.provider === "dnshe") {
      return Boolean(entry.maskedKey || entry.maskedSecret) && (!entry.apiKey.trim() || !entry.apiSecret.trim());
    }
    return Boolean(entry.maskedToken) && !entry.apiToken.trim();
  }),
);
const viewportRuntimeSupported = computed(() => isViewportRuntimeSupported());
const resultIpFilterOptions: Array<{ label: string; value: ProbeResultIPFilter }> = [
  { label: "全部 IP", value: "all" },
  { label: "仅 IPv4", value: "ipv4" },
  { label: "仅 IPv6", value: "ipv6" },
];
const resultSortOptions: Array<{ label: string; value: ProbeResultSortBy }> = [
  { label: "TCP 延迟", value: "tcp" },
  { label: "追踪延迟", value: "trace" },
  { label: "平均速率", value: "download" },
  { label: "最高速率", value: "max_download" },
];

function createSourceDraft(kind: SourceKind): SourceDraft {
  sourceSeed.value += 1;
  return {
    content: "",
    colo_filter: "",
    colo_filter_mode: "allow",
    enabled: true,
    id: `source-${Date.now()}-${sourceSeed.value}`,
    ip_limit: -1,
    ip_mode: sourceGroupIPMode.value || "traverse",
    kind,
    last_fetched_at: "",
    last_fetched_count: 0,
    name: `输入源 ${sourceSeed.value}`,
    path: "",
    status_text: "",
    url: "",
  };
}

function applyDroppedSourcePaths(value: unknown) {
  const paths = Array.isArray(value) ? value.map((item) => asString(item)).filter((item) => item !== "") : [];
  let added = 0;
  for (const path of paths) {
    if (sources.value.some((source) => source.kind === "file" && source.path === path)) {
      continue;
    }
    const draft = createSourceDraft("file");
    draft.path = path;
    const fileName = path.split(/[\\/]/).pop() || "";
    if (fileName) {
      draft.name = fileName;
    }
    sources.value.push(draft);
    added += 1;
  }
  if (added > 0) {
    showToast(`已把 ${added} 个拖放文件加入输入源，保存配置后生效。`, "success");
  }
}

async function refreshDroppedSources() {
  try {
    const result = await consumeDroppedSources();
    appendLog("bridge.consume_dropped_sources", commandDiagnosticPayload(result));
    if (!result.ok) {
      return;
    }
    applyDroppedSourcePaths(asRecord(result.data).paths);
  } catch (error) {
    appendLog("bridge.consume_dropped_sources_failed", {
      message: error instanceof Error ? error.message : String(error),
    });
  }
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

function applySourceProfileStore(value: unknown) {
  sourceProfiles.value = normalizeSourceProfileStore(value);
  const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
  sourceGroupColoFilter.value = active?.colo_filter || "";
  sourceGroupColoFilterMode.value = active ? active.colo_filter_mode : "allow";
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

function schedulerIntervalMinutesForSnapshot(enabled: boolean) {
  const value = Number.isFinite(settings.schedulerIntervalMinutes) && settings.schedulerIntervalMinutes > 0 ? settings.schedulerIntervalMinutes : settings.schedulerIntervalMinutesDraft;
  return enabled ? positiveCount(value, 60) : nonNegativeCount(value, 0);
}

function formatAppTimestamp(value: string, options?: Parameters<typeof formatTimestampWithUTCOffset>[2]) {
  return formatTimestampWithUTCOffset(value, settings.utcOffsetMinutes, options);
}

function utcOffsetLabel() {
  return formatUTCOffsetLabel(settings.utcOffsetMinutes);
}

function appendLog(event: string, payload: unknown) {
  logs.push({ event, payload, ts: new Date().toISOString() });
  if (logs.length > 160) {
    logs.splice(0, logs.length - 160);
  }
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
  void navigateTo("dashboard");
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
  void navigateTo("dashboard");
}

function clearProcessTrace() {
  processTrace.value = [];
}

function clearDownloadSpeedTraceEntries() {
  for (let index = processTrace.value.length - 1; index >= 0; index -= 1) {
    if (processTrace.value[index].key?.startsWith("download-speed:")) {
      processTrace.value.splice(index, 1);
    }
  }
}

function updateDownloadSpeedTrace(ip: string, colo: string, ts: string) {
  const traceKey = `download-speed:${ip || "当前 IP"}`;
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
  if (entries.length > 80) {
    entries.splice(80);
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

function pushProcessTrace(entry: Omit<ProcessTraceEntry, "id">) {
  const entries = processTrace.value;
  const lastEntry = entries[0];
  if (lastEntry && lastEntry.stage === entry.stage && lastEntry.title === entry.title && lastEntry.tone === entry.tone) {
    entries[0] = { ...entry, id: lastEntry.id };
    return;
  }

  entries.unshift({ ...entry, id: processTraceId });
  processTraceId += 1;
  if (entries.length > 80) {
    entries.splice(80);
  }
}

function setStatus(next: { title: string; detail: string; tone: TaskTone }) {
  status.title = next.title;
  status.detail = next.detail;
  status.tone = next.tone;
}

function updateMCISProgress(payload: Record<string, unknown>, active = true) {
  mcisProgress.active = active;
  mcisProgress.candidateCount = asCount(payload.candidate_count, mcisProgress.candidateCount);
  mcisProgress.completed = asCount(payload.completed, mcisProgress.completed);
  mcisProgress.concurrency = asCount(payload.concurrency, mcisProgress.concurrency);
  mcisProgress.elapsedMs = asCount(payload.elapsed_ms, mcisProgress.elapsedMs);
  mcisProgress.failed = asCount(payload.failed, mcisProgress.failed);
  mcisProgress.lastColo = payload.last_colo === undefined ? mcisProgress.lastColo : asString(payload.last_colo).trim();
  mcisProgress.lastIP = payload.last_ip === undefined ? mcisProgress.lastIP : asString(payload.last_ip).trim();
  mcisProgress.lastOK = asBoolean(payload.last_ok, mcisProgress.lastOK);
  mcisProgress.sourceID = payload.source_id === undefined ? mcisProgress.sourceID : asString(payload.source_id).trim();
  mcisProgress.sourceName = payload.source_name === undefined ? mcisProgress.sourceName : asString(payload.source_name).trim();
  mcisProgress.succeeded = asCount(payload.succeeded, mcisProgress.succeeded);
  mcisProgress.total = asCount(payload.total, mcisProgress.total);
}

function resetMCISProgress() {
  updateMCISProgress(
    {
      candidate_count: 0,
      completed: 0,
      concurrency: 0,
      elapsed_ms: 0,
      failed: 0,
      last_colo: "",
      last_ip: "",
      last_ok: false,
      source_id: "",
      source_name: "",
      succeeded: 0,
      total: 0,
    },
    false,
  );
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
  resetMCISProgress();
  clearProcessTrace();
}

function addSource(kind: SourceKind) {
  sources.value.push(createSourceDraft(kind));
}

function setSourceGroupIPMode(mode: SourceIPMode) {
  sourceGroupIPMode.value = mode;
  for (const source of sources.value) source.ip_mode = mode;
}

function setSourceGroupColoFilter(filter: string) {
  sourceGroupColoFilter.value = filter;
}

function setSourceGroupColoFilterMode(mode: ColoFilterMode) {
  sourceGroupColoFilterMode.value = mode;
}

function removeSource(sourceId: string) {
  const source = sources.value.find((item) => item.id === sourceId);
  if (source && !window.confirm(`确定删除“${source.name || "未命名输入源"}”吗？删除后需要重新添加。`)) {
    return;
  }
  sources.value = sources.value.filter((source) => source.id !== sourceId);
  delete sourcePreviewStates[sourceId];
  delete sourceRequestStates[sourceId];
  delete sourcePreviewErrors[sourceId];
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

function dnsProviderDefaultTTL(provider: DNSProviderKind) {
  switch (provider) {
    case "dnshe":
      return DEFAULT_DNSHE_TTL;
    case "aliyun":
      return DEFAULT_ALIYUN_TTL;
    case "dnspod":
      return DEFAULT_DNSPOD_TTL;
    default:
      return DEFAULT_CLOUDFLARE_TTL;
  }
}

function createDnsProviderForm(provider: DNSProviderKind, index: number): DnsProviderForm {
  return {
    allowCNAME: false,
    apiKey: "",
    apiSecret: "",
    apiToken: "",
    enabled: true,
    id: `${provider}-${index + 1}`,
    label: dnsProviderPlatformLabel(provider),
    maskedKey: "",
    maskedSecret: "",
    maskedToken: "",
    provider,
    recordName: "",
    recordType: "A",
    routingEnabled: false,
    routingRules: [],
    subdomain: "",
    subdomainId: 0,
    topN: 5,
    ttl: dnsProviderDefaultTTL(provider),
    zoneId: "",
  };
}

function dnsProviderFormFromSnapshot(item: DNSProviderSnapshot, index: number): DnsProviderForm {
  const form = createDnsProviderForm(item.provider, index);
  form.id = item.id || form.id;
  form.label = item.name?.trim() || dnsProviderPlatformLabel(item.provider);
  form.enabled = Boolean(item.enabled);
  form.allowCNAME = Boolean(item.allow_cname);
  form.recordName = item.record_name || "";
  form.recordType = item.record_type === "ALL" ? "ALL" : item.record_type === "AAAA" ? "AAAA" : "A";
  form.topN = nonNegativeCount(item.top_n, 5);
  form.ttl = item.provider === "cloudflare" ? normalizeCloudflareTTL(item.ttl) : Math.max(60, asCount(item.ttl, dnsProviderDefaultTTL(item.provider)));
  if (item.provider !== "cloudflare") {
    form.maskedKey = isMaskedTokenValue(item.api_key || "") ? item.api_key || "" : "";
    form.maskedSecret = isMaskedTokenValue(item.api_secret || "") ? item.api_secret || "" : "";
    form.apiKey = form.maskedKey ? "" : item.api_key || "";
    form.apiSecret = form.maskedSecret ? "" : item.api_secret || "";
    form.subdomain = (item.subdomain || "").trim();
    form.routingEnabled = Boolean(item.routing_enabled);
    form.routingRules = (item.routing_rules ?? []).map((rule, ruleIndex) => ({
      enabled: rule.enabled,
      filterMode: rule.filter_mode,
      filterTokens: rule.filter_tokens,
      id: `${form.id}-route-${ruleIndex}`,
      name: rule.name,
      recordName: rule.record_name,
      recordType: rule.record_type,
      topN: rule.top_n,
    }));
    if (item.provider === "dnshe") {
      form.subdomainId = form.subdomain ? 0 : nonNegativeCount(item.subdomain_id, 0);
    }
    return form;
  }
  form.maskedToken = isMaskedTokenValue(item.api_token || "") ? item.api_token || "" : "";
  form.apiToken = form.maskedToken ? "" : item.api_token || "";
  form.zoneId = item.zone_id || "";
  form.routingEnabled = Boolean(item.routing_enabled);
  form.routingRules = (item.routing_rules ?? []).map((rule, ruleIndex) => ({
    enabled: rule.enabled,
    filterMode: rule.filter_mode,
    filterTokens: rule.filter_tokens,
    id: `${form.id}-route-${ruleIndex}`,
    name: rule.name,
    recordName: rule.record_name,
    recordType: rule.record_type,
    topN: rule.top_n,
  }));
  return form;
}

function applyDNSProvidersFromSnapshot(normalized: ConfigSnapshot) {
  settings.dnsProviders = normalized.dns_providers.map((item, index) => dnsProviderFormFromSnapshot(item, index));
  syncCloudflareLegacyMirror();
}

function primaryCloudflareProvider(): DnsProviderForm | undefined {
  return settings.dnsProviders.find((entry) => entry.provider === "cloudflare");
}

// 旧字段（记录名、Token 脱敏提示、分流规则计数）仍被结果页和设置页摘要读取，
// 统一从主 Cloudflare 平台派生，避免两套状态互相覆盖。
function syncCloudflareLegacyMirror() {
  maskedTokenHint.value = primaryCloudflareProvider()?.maskedToken ?? "";
}

function dnsProviderFormComplete(entry: DnsProviderForm) {
  const routingTarget = entry.routingEnabled && entry.routingRules.some((rule) => rule.enabled && rule.recordName.trim());
  if (entry.provider !== "cloudflare") {
    const targetReady = Boolean(entry.recordName.trim() || routingTarget || entry.subdomain.trim());
    return Boolean((entry.apiKey.trim() || entry.maskedKey) && (entry.apiSecret.trim() || entry.maskedSecret) && targetReady);
  }
  return Boolean((entry.apiToken.trim() || entry.maskedToken) && entry.zoneId.trim() && (entry.recordName.trim() || routingTarget));
}

function dnsRoutingRuleSnapshotFromForm(rule: CloudflareRoutingRuleForm) {
  return {
    enabled: Boolean(rule.enabled),
    filter_mode: rule.filterMode === "deny" ? ("deny" as const) : ("allow" as const),
    filter_tokens: rule.filterTokens.trim(),
    name: rule.name.trim(),
    record_name: rule.recordName.trim(),
    record_type: rule.recordType === "ALL" ? ("ALL" as const) : rule.recordType === "AAAA" ? ("AAAA" as const) : ("A" as const),
    top_n: nonNegativeCount(rule.topN, 5),
  };
}

function dnsProviderSnapshotFromForm(entry: DnsProviderForm): DNSProviderSnapshot {
  if (entry.provider !== "cloudflare") {
    const subdomain = entry.subdomain.trim();
    return {
      ...(entry.apiKey.trim() ? { api_key: entry.apiKey.trim() } : {}),
      ...(entry.apiSecret.trim() ? { api_secret: entry.apiSecret.trim() } : {}),
      allow_cname: entry.allowCNAME,
      enabled: Boolean(entry.enabled && dnsProviderFormComplete(entry)),
      id: entry.id,
      name: entry.label.trim(),
      provider: entry.provider,
      record_name: entry.recordName.trim(),
      record_type: entry.recordType === "AAAA" ? "AAAA" : "A",
      routing_enabled: entry.routingEnabled,
      routing_rules: entry.routingRules.map(dnsRoutingRuleSnapshotFromForm),
      ...(entry.provider === "dnshe" ? (subdomain ? { subdomain } : { subdomain_id: nonNegativeCount(entry.subdomainId, 0) }) : {}),
      top_n: nonNegativeCount(entry.topN, 5),
      ttl: Math.max(60, asCount(entry.ttl, dnsProviderDefaultTTL(entry.provider))),
    };
  }
  return {
    ...(entry.apiToken.trim() ? { api_token: entry.apiToken.trim() } : {}),
    allow_cname: entry.allowCNAME,
    enabled: Boolean(entry.enabled && dnsProviderFormComplete(entry)),
    id: entry.id,
    name: entry.label.trim(),
    provider: "cloudflare",
    record_name: entry.recordName.trim(),
    record_type: entry.recordType,
    routing_enabled: entry.routingEnabled,
    routing_rules: entry.routingRules.map(dnsRoutingRuleSnapshotFromForm),
    top_n: nonNegativeCount(entry.topN, 5),
    ttl: normalizeCloudflareTTL(entry.ttl),
    zone_id: entry.zoneId.trim(),
  };
}

function enabledDNSProviders() {
  return settings.dnsProviders.filter((entry) => entry.enabled && dnsProviderFormComplete(entry));
}

const dnsSelfTestProviders = computed<DnsSelfTestProviderOption[]>(() =>
  enabledDNSProviders().map((entry) => ({
    id: entry.id,
    label: entry.label.trim() || `${dnsProviderPlatformLabel(entry.provider)}${dnsProviderTargetLabel(entry) ? ` · ${dnsProviderTargetLabel(entry)}` : ""}`,
    provider: entry.provider,
    recordName: dnsProviderResolvedRecordName(entry),
    zoneName: dnsProviderZoneName(entry),
  })),
);

// 读取平台列出全部已保存的条目（含停用与待补全），让用户能看到自己在设置页保存了什么；
// 真正的可用性由状态文案和后端校验一起说明。
const dnsReadProviders = computed<DnsReadProviderOption[]>(() =>
  settings.dnsProviders.map((entry) => ({
    enabled: entry.enabled,
    id: entry.id,
    label: entry.label.trim() || `${dnsProviderPlatformLabel(entry.provider)}${dnsProviderTargetLabel(entry) ? ` · ${dnsProviderTargetLabel(entry)}` : ""}`,
    provider: entry.provider,
    recordName: dnsProviderResolvedRecordName(entry),
    status: entry.enabled ? (dnsProviderFormComplete(entry) ? "已就绪" : "待补全") : "已停用",
  })),
);

watch(
  dnsReadProviders,
  (providers) => {
    if (providers.some((provider) => provider.id === dnsReadProviderId.value)) {
      return;
    }
    const ready = providers.find((provider) => provider.status === "已就绪");
    dnsReadProviderId.value = ready?.id || providers[0]?.id || "";
  },
  { immediate: true },
);

// 打开 DNS 页时静默补齐 DNSHE 子域名列表，自检才能自动取到主域名（子域 zone 顶点）。
watch(
  () =>
    selectedView.value === "dns"
      ? settings.dnsProviders
          .filter((entry) => entry.enabled && entry.provider === "dnshe")
          .map((entry) => entry.id)
          .join(",")
      : "",
  (ids) => {
    if (!ids) {
      return;
    }
    for (const id of ids.split(",")) {
      if (!dnsSubdomainsByProvider.value[id] && !dnsSubdomainsLoading.value[id]) {
        void loadDnsProviderSubdomains(id, true);
      }
    }
  },
  { immediate: true },
);

// 切换读取平台后旧记录已经没有意义，先清空等用户重新读取。
watch(dnsReadProviderId, () => {
  dnsRecords.value = [];
});

function applyConfigSnapshot(snapshot: ConfigSnapshot) {
  const normalized = normalizeConfigSnapshot(snapshot);

  // 备注没有独立 UI，随兼容 cloudflare 段原样往返，避免保存时丢字段。
  settings.comment = normalized.cloudflare.comment || "";
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
  settings.telegramUseSystemProxy = Boolean(normalized.notifications.telegram.use_system_proxy);
  settings.webhookEnabled = Boolean(normalized.notifications.webhook.enabled);
  settings.webhookURLs = normalized.notifications.webhook.urls.join("\n");
  settings.webhookUseSystemProxy = Boolean(normalized.notifications.webhook.use_system_proxy);
  settings.webhookDingTalkURL = normalized.notifications.webhook.dingtalk_url;
  settings.webhookDingTalkSecret = normalized.notifications.webhook.dingtalk_secret;
  settings.webhookWeComURLs = normalized.notifications.webhook.wecom_urls.join("\n");
  settings.webhookWeComCorpID = normalized.notifications.webhook.wecom_corp_id;
  settings.webhookWeComAgentID = normalized.notifications.webhook.wecom_agent_id;
  settings.webhookWeComSecret = normalized.notifications.webhook.wecom_secret;
  settings.webhookWeComMobiles = normalized.notifications.webhook.wecom_mobiles.join("\n");
  settings.webhookHeadersJSON = JSON.stringify(normalized.notifications.webhook.headers, null, 2);
  settings.emailEnabled = Boolean(normalized.notifications.email.enabled);
  settings.emailHost = normalized.notifications.email.host;
  settings.emailPort = normalized.notifications.email.port;
  settings.emailUsername = normalized.notifications.email.username;
  settings.emailPassword = normalized.notifications.email.password;
  settings.emailFrom = normalized.notifications.email.from;
  settings.emailTo = normalized.notifications.email.to.join("\n");
  settings.emailUseTLS = Boolean(normalized.notifications.email.use_tls);
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
  settings.githubOwner = normalized.github.owner || "";
  settings.githubPathTemplate = normalized.github.path_template || "cfst-results/{date}/{time}-{task_id}.csv";
  settings.githubRepo = normalized.github.repo || "";
  settings.githubToken = normalized.github.token || "";
  settings.githubTXTRowTemplate = normalized.github.txt_row_template || "{ip}";
  settings.exportCSVEncoding = normalized.export.csv_encoding || "utf-8";
  settings.exportOverwrite = normalized.export.overwrite || "replace_on_start";
  settings.exportTargetDir = normalized.export.target_dir || "";
  settings.exportTargetUri = normalized.export.target_uri || "";
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
  settings.probeConcurrencyStage3 = positiveCount(normalized.probe.concurrency.stage3, 1, 16);
  settings.probeMCISBudget = normalized.probe.mcis.budget;
  settings.probeMCISConcurrency = normalized.probe.mcis.concurrency;
  settings.probeCooldownFailures = normalized.probe.cooldown_policy.consecutive_failures;
  settings.probeCooldownMs = normalized.probe.cooldown_policy.cooldown_ms;
  settings.probeDownloadBufferKB = normalized.probe.download_buffer_kb;
  settings.probeDownloadSuccessLimit = normalized.probe.download_success_limit;
  settings.probeDownloadGetConcurrency = normalized.probe.download_get_concurrency;
  settings.probeDownloadHostHeader = normalized.probe.download_host_header || "";
  settings.probeDownloadHTTPProtocol = normalized.probe.download_http_protocol;
  settings.probeDownloadSNI = normalized.probe.download_sni || "";
  settings.probeDownloadSpeedMetric = normalized.probe.download_speed_metric;
  settings.probeDownloadSpeedSampleIntervalMs = normalized.probe.download_speed_sample_interval_ms;
  settings.probeDownloadTimeSeconds = normalized.probe.download_time_seconds;
  settings.probeDownloadWarmupSeconds = normalized.probe.download_warmup_seconds;
  settings.probeEventThrottleMs = normalized.probe.event_throttle_ms;
  settings.probeHostHeader = normalized.probe.host_header || "";
  settings.probeTraceCfColo = normalized.probe.trace_cf_colo || "";
  settings.probeTraceCfColoMode = normalized.probe.trace_cf_colo_mode || "allow";
  settings.probeTraceStatusCode = normalized.probe.trace_status_code;
  settings.probePingTimes = Math.max(MIN_PROBE_PING_TIMES, normalized.probe.ping_times);
  settings.probePrintNum = normalized.probe.print_num;
  settings.probePortPolicy = normalized.probe.port_policy === "source_override_global" ? "source_override_global" : "fixed_global";
  settings.probeRetryBackoffMs = normalized.probe.retry_policy.backoff_ms;
  settings.probeRetryMaxAttempts = normalized.probe.retry_policy.max_attempts;
  settings.probeRequestHeaders = normalized.probe.request_headers || "";
  settings.probeSNI = normalized.probe.sni || "";
  settings.probeVerifyTLSCertificate = Boolean(normalized.probe.verify_tls_certificate);
  settings.probeSourceColoFilterPhase = normalized.probe.source_colo_filter_phase;
  settings.probeSourcePoolLimit = boundedCount(normalized.probe.source_pool_limit, DEFAULT_SOURCE_POOL_LIMIT, 1, MAX_SOURCE_POOL_LIMIT);
  settings.probeStageLimitStage2 = normalized.probe.stage_limits.stage2;
  settings.probeStageLimitStage3 = normalized.probe.stage_limits.stage3;
  settings.probeStrategy = normalized.probe.strategy;
  settings.probeTcpPort = normalized.probe.tcp_port;
  settings.probeTimeoutStage1Ms = normalized.probe.timeouts.stage1_ms;
  settings.probeTimeoutStage2Ms = normalized.probe.timeouts.stage2_ms;
  settings.probeTraceColoMode = normalized.probe.trace_colo_mode;
  settings.probeTraceURL = normalized.probe.trace_url || "";
  settings.probeDoHURL = normalized.probe.doh_url || DEFAULT_DOH_URL;
  settings.probeECH = normalized.probe.ech;
  settings.probeECHConfig = normalized.probe.ech_config || "";
  settings.probeECHDomain = normalized.probe.ech_domain || DEFAULT_ECH_DOMAIN;
  settings.probeURL = normalized.probe.url || DEFAULT_FILE_TEST_URL;
  settings.probeUserAgent = normalized.probe.user_agent || DEFAULT_PROBE_USER_AGENT;
  settings.proxied = false;
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
  settings.schedulerSkipIfActive = normalized.scheduler.skip_if_active;
  settings.schedulerTriggerMode = schedulerTriggerModeFromSnapshot(normalized.scheduler, settings.schedulerTriggerMode);
  settings.sourceAutoDetectName = normalized.ui.auto_detect_source_name;
  settings.themeDarkStart = normalized.ui.theme_dark_start || "19:00";
  settings.themeLightStart = normalized.ui.theme_light_start || "07:00";
  settings.themeMode = normalized.ui.theme_mode || "auto_system_time";
  settings.utcOffsetMinutes = normalizeUTCOffsetMinutes(normalized.ui.utc_offset_minutes);
  settings.webdavEnabled = normalized.backup.webdav.enabled;
  settings.webdavLastBackupAt = normalized.backup.webdav.last_backup_at;
  settings.webdavLastRestoreAt = normalized.backup.webdav.last_restore_at;
  settings.webdavPassword = normalized.backup.webdav.password;
  settings.webdavRemotePath = normalized.backup.webdav.remote_path || "cfst-gui-config.zip";
  settings.webdavServerURL = normalized.backup.webdav.server_url;
  settings.webdavTimeoutSeconds = normalized.backup.webdav.timeout_seconds || 30;
  settings.webdavUsername = normalized.backup.webdav.username;
  applyDNSProvidersFromSnapshot(normalized);
  sourceGroupIPMode.value = normalized.source_ip_mode;
  sources.value = normalized.sources.map((source) => ({ ...source }));
  if (!task.active || !isDownloadSpeedTaskConfigCaptured()) {
    captureDownloadSpeedTaskConfig(normalized);
  }
}

function applyResultCloudflarePushSettings(input: Partial<ResultCloudflarePushSettings> = {}) {
  const primary = enabledDNSProviders()[0] ?? primaryCloudflareProvider();
  resultCloudflarePushSettings.providerId = asString(input.providerId).trim() || primary?.id || "";
  resultCloudflarePushSettings.recordName = asString(input.recordName).trim() || primary?.recordName.trim() || "";
  resultCloudflarePushSettings.recordType = normalizeResultCloudflareRecordType(input.recordType);
  resultCloudflarePushSettings.topN = nonNegativeCount(input.topN, primary?.topN ?? 5);
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

function parseWebhookHeaders(value: string) {
  try {
    const parsed = JSON.parse(value);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
}

function buildConfigSnapshot() {
  const normalizedStrategy: ProbeStrategy = settings.probeStrategy === "full" ? "full" : "fast";
  const normalizedGitHubToken = settings.githubToken.trim();
  const githubProviderEnabled = Boolean(settings.githubOwner.trim() && settings.githubRepo.trim() && settings.githubBranch.trim() && settings.githubPathTemplate.trim() && normalizedGitHubToken && !isMaskedTokenValue(normalizedGitHubToken));
  const primaryCloudflare = primaryCloudflareProvider();
  const primaryCloudflareSnapshot = primaryCloudflare ? dnsProviderSnapshotFromForm(primaryCloudflare) : null;
  const cloudflareRoutingRules = primaryCloudflareSnapshot?.routing_rules ?? [];
  const cloudflareProviderEnabled = Boolean(primaryCloudflareSnapshot?.enabled);
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
    dns_providers: settings.dnsProviders.map((entry) => dnsProviderSnapshotFromForm(entry)),
    cloudflare: {
      ...(primaryCloudflareSnapshot?.api_token ? { api_token: primaryCloudflareSnapshot.api_token } : {}),
      allow_cname: primaryCloudflareSnapshot?.allow_cname ?? false,
      comment: settings.comment.trim(),
      enabled: cloudflareProviderEnabled,
      proxied: false,
      record_name: primaryCloudflareSnapshot?.record_name ?? "",
      routing_enabled: Boolean(primaryCloudflareSnapshot?.routing_enabled),
      routing_rules: cloudflareRoutingRules,
      top_n: primaryCloudflareSnapshot?.top_n ?? 5,
      ttl: primaryCloudflareSnapshot?.ttl ?? DEFAULT_CLOUDFLARE_TTL,
      zone_id: primaryCloudflareSnapshot?.zone_id ?? "",
    },
    github: githubConfig,
    post_probe_push: {
      cloudflare_enabled: settings.postProbePushCloudflareEnabled,
      github_enabled: settings.postProbePushGitHubEnabled,
    },
    upload: {
      cloudflare: {
        routing_enabled: Boolean(primaryCloudflareSnapshot?.routing_enabled),
        routing_rules: cloudflareRoutingRules,
        top_n: primaryCloudflareSnapshot?.top_n ?? 5,
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
        stage3: positiveCount(settings.probeConcurrencyStage3, 1, 16),
      },
      mcis: {
        budget: nonNegativeCount(settings.probeMCISBudget, 0),
        concurrency: nonNegativeCount(settings.probeMCISConcurrency, 0),
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
      download_success_limit: nonNegativeCount(settings.probeDownloadSuccessLimit, 0),
      download_get_concurrency: boundedCount(settings.probeDownloadGetConcurrency, 4, 1, 32),
      download_host_header: settings.probeDownloadHostHeader.trim(),
      download_http_protocol: normalizeDownloadHTTPProtocol(settings.probeDownloadHTTPProtocol),
      download_sni: settings.probeDownloadSNI.trim(),
      download_speed_metric: settings.probeDownloadSpeedMetric === "max" ? "max" : "average",
      download_speed_sample_interval_ms: positiveCount(settings.probeDownloadSpeedSampleIntervalMs, 1000),
      download_time_seconds: positiveCount(settings.probeDownloadTimeSeconds, 4),
      download_warmup_seconds: nonNegativeCount(settings.probeDownloadWarmupSeconds, 1),
      event_throttle_ms: positiveCount(settings.probeEventThrottleMs, 100),
      host_header: settings.probeHostHeader.trim(),
      trace_cf_colo: settings.probeTraceCfColo.trim(),
      trace_cf_colo_mode: settings.probeTraceCfColoMode === "deny" ? "deny" : "allow",
      trace_status_code: settings.probeTraceStatusCode === 0 || (settings.probeTraceStatusCode >= 100 && settings.probeTraceStatusCode <= 599) ? settings.probeTraceStatusCode : DEFAULT_HTTPING_STATUS_CODE,
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
      source_pool_limit: boundedCount(settings.probeSourcePoolLimit, DEFAULT_SOURCE_POOL_LIMIT, 1, MAX_SOURCE_POOL_LIMIT),
      stage_limits: {
        stage2: nonNegativeCount(settings.probeStageLimitStage2, 0),
        stage3: positiveCount(settings.probeStageLimitStage3, 10),
      },
      port_policy: settings.probePortPolicy,
      strategy: normalizedStrategy,
      sni: settings.probeSNI.trim(),
      verify_tls_certificate: settings.probeVerifyTLSCertificate,
      tcp_port: positiveCount(settings.probeTcpPort, 443, 65535),
      test_all: false,
      thresholds: {
        max_tcp_latency_ms: optionalNumberForPayload(settings.maxTcpLatencyMs),
        min_download_mbps: nonNegativeNumber(settings.minDownloadMbps, 0),
      },
      timeouts: {
        stage1_ms: positiveCount(settings.probeTimeoutStage1Ms, 1000),
        stage2_ms: positiveCount(settings.probeTimeoutStage2Ms, 1000),
      },
      trace_colo_mode: settings.probeTraceColoMode,
      trace_url: settings.probeTraceURL.trim(),
      doh_url: settings.probeDoHURL.trim(),
      ech: settings.probeECH,
      ech_config: settings.probeECHConfig.trim(),
      ech_domain: settings.probeECHDomain.trim(),
      url: settings.probeURL.trim(),
      user_agent: settings.probeUserAgent.trim() || DEFAULT_PROBE_USER_AGENT,
    },
    ui: {
      auto_detect_source_name: settings.sourceAutoDetectName,
      // 普通/高级设置已合并；固定写 advanced，旧配置的 basic 值读入后忽略。
      settings_mode: "advanced",
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
        use_system_proxy: settings.telegramUseSystemProxy,
      },
      webhook: {
        enabled: settings.webhookEnabled,
        urls: settings.webhookURLs
          .split(/[\n\r,;]+/)
          .map((value) => value.trim())
          .filter(Boolean),
        dingtalk_url: settings.webhookDingTalkURL.trim(),
        dingtalk_secret: settings.webhookDingTalkSecret.trim(),
        wecom_urls: settings.webhookWeComURLs
          .split(/[\n\r,;]+/)
          .map((value) => value.trim())
          .filter(Boolean),
        use_system_proxy: settings.webhookUseSystemProxy,
        headers: parseWebhookHeaders(settings.webhookHeadersJSON),
        wecom_corp_id: settings.webhookWeComCorpID.trim(),
        wecom_agent_id: settings.webhookWeComAgentID.trim(),
        wecom_secret: settings.webhookWeComSecret.trim(),
        wecom_mobiles: settings.webhookWeComMobiles
          .split(/[\n\r,;]+/)
          .map((value) => value.trim())
          .filter(Boolean),
      },
      email: {
        enabled: settings.emailEnabled,
        host: settings.emailHost.trim(),
        port: positiveCount(settings.emailPort, 587, 65535),
        username: settings.emailUsername.trim(),
        password: settings.emailPassword,
        from: settings.emailFrom.trim(),
        to: settings.emailTo
          .split(/[\n\r,;]+/)
          .map((value) => value.trim())
          .filter(Boolean),
        use_tls: settings.emailUseTLS,
      },
    },
    scheduler: {
      auto_dns_push: settings.schedulerAutoDnsPush,
      auto_github_export: settings.schedulerAutoGithubExport,
      config_source: "draft_preferred",
      daily_times: settings.schedulerTriggerMode === "daily" ? schedulerDailyTimesFromText(settings.schedulerDailyTimes) : [],
      enabled: settings.schedulerEnabled,
      interval_minutes: settings.schedulerTriggerMode === "interval" ? schedulerIntervalMinutesForSnapshot(settings.schedulerEnabled) : 0,
      post_run_source_profile_action: "update_recent_run_source_profile",
      skip_if_active: settings.schedulerSkipIfActive,
    },
    source_ip_mode: sourceGroupIPMode.value,
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
  return snapshotSignature({ ip_mode: sourceGroupIPMode.value, colo_filter: sourceGroupColoFilter.value, colo_filter_mode: sourceGroupColoFilterMode.value, sources: sourcePayloads.value.map((source) => ({ ...source, status_text: source.status_text.trim() })) });
}

function currentSourceProfileSignature() {
  return snapshotSignature(sourceProfiles.value);
}

function activeSourceProfileSourceSignature() {
  const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
  const snapshot = normalizeConfigSnapshot({ sources: active?.sources || [], source_ip_mode: active?.ip_mode });
  return snapshotSignature({
    ip_mode: snapshot.source_ip_mode,
    colo_filter: active?.colo_filter || "",
    colo_filter_mode: active?.colo_filter_mode || "",
    sources: snapshot.sources.map((source) => ({ ...source, status_text: source.status_text.trim() })),
  });
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
  }, 500);
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
    const result = await saveDraft({ config_snapshot: snapshot });
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

async function maybeRestoreDraft(statusValue: unknown) {
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
    const discarded = await discardDraft();
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

async function selectAndroidLogExportTarget(defaultFileName: string, title: string): Promise<string | null> {
  try {
    const result = await selectPath({
      current_path: androidLogExportTargetUri.value,
      default_file_name: defaultFileName,
      mode: "export_target",
      title,
    });
    appendLog("bridge.select_log_export_target", result);
    const data = asRecord(result.data) as PathSelectionPayload;
    if (!result.ok) {
      showToast(result.message || "选择日志导出目录失败", "error");
      return null;
    }
    if (data.canceled) {
      return null;
    }
    const targetUri = asString(data.target_uri || data.uri || data.androidExportUri).trim();
    if (!targetUri) {
      showToast("未获取到日志导出目录", "error");
      return null;
    }
    androidLogExportTargetUri.value = targetUri;
    return targetUri;
  } catch (error) {
    showToast(error instanceof Error ? error.message : "选择日志导出目录失败", "error");
    return null;
  }
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
    applyNavigation("settings");
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
  const target = storageStatus.value?.current_dir || "";
  if (!target) {
    showToast("尚未读取应用数据目录", "info");
    return;
  }
  try {
    await openPath(target);
  } catch (error) {
    showToast(error instanceof Error ? error.message : "打开应用数据目录失败", "error");
  }
}

async function exportCurrentDebugLog() {
  const targetUri = appInfo.value.platform === "android" ? await selectAndroidLogExportTarget("cfip-log.txt", "选择日志导出目录") : "";
  if (appInfo.value.platform === "android" && !targetUri) {
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
  const targetUri = appInfo.value.platform === "android" ? await selectAndroidLogExportTarget("cfst-diagnostics.zip", "选择诊断包导出目录") : "";
  if (appInfo.value.platform === "android" && !targetUri) {
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
    const targetUri = androidLogExportTargetUri.value;
    if (!targetUri) {
      showToast("请先导出调试日志或诊断包以定位日志目录。", "info");
      return;
    }
    try {
      await openPath(targetUri);
      setStatus({
        detail: `Android 日志导出目录：${targetUri}`,
        title: "日志导出目录",
        tone: "completed",
      });
      showToast("已打开 Android 日志导出目录", "success");
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
  if (settings.schedulerTriggerMode === "daily") {
    const dailyTimes = schedulerDailyTimesFromText(settings.schedulerDailyTimes);
    if (dailyTimes.length === 0) {
      showToast("请填写有效的每日定时时间", "error");
      return;
    }
    settings.schedulerDailyTimes = dailyTimes.join("\n");
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
    applyNavigation("settings");
    showToast(result.message || "已从 WebDAV 还原配置", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "WebDAV 还原失败", "error");
  }
}

async function saveCurrentSourceProfile(name: string, profileId = "", profileSources?: unknown, setActive = true, ipMode?: SourceGroupIPMode, coloFilter = sourceGroupColoFilter.value, coloFilterMode = sourceGroupColoFilterMode.value) {
  try {
    if (setActive && profileId !== sourceProfiles.value.active_profile_id && sourcePageHasUnsavedChanges()) {
      if (!(await autoSaveSourcePage("create_source_profile"))) {
        showToast("当前输入组保存失败，已取消创建或复制", "error");
        return false;
      }
    }
    const payloadMode = resolveSourceGroupIPMode(ipMode ?? sourceGroupIPMode.value, Array.isArray(profileSources) ? profileSources : sources.value);
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
      ip_mode: payloadMode,
      colo_filter: coloFilter.trim(),
      colo_filter_mode: coloFilterMode,
      sources: payloadSources,
    });
    appendLog("bridge.save_source_profile", result);
    if (!result.ok) {
      showToast(result.message || "保存输入组失败", "error");
      return false;
    }
    applySourceProfileStore(result.data);
    if (setActive) {
      sourceGroupIPMode.value = payloadMode;
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

async function copyCurrentSourceProfile() {
  const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
  await saveCurrentSourceProfile(
    `${active?.name || "当前输入源"} 副本`,
    "",
    sourcePayloads.value.map((source) => ({ ...source })),
    true,
    sourceGroupIPMode.value,
  );
}
async function updateActiveSourceProfile() {
  try {
    const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
    const submittedSignature = currentSourceSignature();
    const result = await updateCurrentSourceProfile({
      name: active?.name || "当前输入源",
      profile_id: active?.id || "",
      ip_mode: sourceGroupIPMode.value,
      colo_filter: sourceGroupColoFilter.value.trim(),
      colo_filter_mode: sourceGroupColoFilterMode.value,
      sources: sourcePayloads.value,
    });
    appendLog("bridge.update_current_source_profile", result);
    if (!result.ok) {
      showToast(result.message || "更新输入组失败", "error");
      return false;
    }
    const changedDuringSave = currentSourceSignature() !== submittedSignature;
    const latestColoFilter = sourceGroupColoFilter.value;
    const latestColoFilterMode = sourceGroupColoFilterMode.value;
    applySourceProfileStore(result.data?.source_profiles);
    if (changedDuringSave) {
      sourceGroupColoFilter.value = latestColoFilter;
      sourceGroupColoFilterMode.value = latestColoFilterMode;
    } else {
      sources.value = sourceDraftsFromProfileSources(result.data?.sources || sourcePayloads.value);
    }
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
    const submittedSignature = currentSourceSignature();
    const result = await updateCurrentSourceProfile({
      name: active?.name || "当前输入源",
      profile_id: active?.id || "",
      ip_mode: sourceGroupIPMode.value,
      colo_filter: sourceGroupColoFilter.value.trim(),
      colo_filter_mode: sourceGroupColoFilterMode.value,
      sources: sourcePayloads.value,
    });
    appendLog("bridge.update_current_source_profile", result);
    if (!result.ok) {
      appendLog("source.profile_quiet_save.failed", result.message || "更新输入组失败");
      return false;
    }
    const changedDuringSave = currentSourceSignature() !== submittedSignature;
    const latestColoFilter = sourceGroupColoFilter.value;
    const latestColoFilterMode = sourceGroupColoFilterMode.value;
    applySourceProfileStore(result.data?.source_profiles);
    if (changedDuringSave) {
      sourceGroupColoFilter.value = latestColoFilter;
      sourceGroupColoFilterMode.value = latestColoFilterMode;
    } else {
      sources.value = sourceDraftsFromProfileSources(result.data?.sources || sourcePayloads.value);
    }
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
    sourceGroupIPMode.value = resolveSourceGroupIPMode(sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id)?.ip_mode, sourceDraftsFromProfileSources(nextSources));
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
    applySourceProfileStore(nextStore);
    if (deletedActiveProfile) {
      const activeProfile = nextStore.items.find((profile) => profile.id === nextStore.active_profile_id) || null;
      const nextSources = Array.isArray(data.sources) ? data.sources : activeProfile?.sources || [];
      sourceGroupIPMode.value = resolveSourceGroupIPMode(activeProfile?.ip_mode, sourceDraftsFromProfileSources(nextSources));
      sources.value = sourceDraftsFromProfileSources(nextSources);
    }
    lastSavedSourceProfileSignature = currentSourceProfileSignature();
    showToast("输入组已删除", "success");
  } catch (error) {
    showToast(error instanceof Error ? error.message : "删除输入组失败", "error");
  }
}

function applyTaskSnapshot(snapshot: TaskSnapshot) {
  taskSnapshot.value = snapshot;
  const snapshotSessionState = asString(snapshot.session_state || "").trim();
  taskSessionState.value = snapshotSessionState || asString(taskSessionState.value || "idle");
  task.taskId = snapshot.task_id || task.taskId;
  task.stage = snapshot.current_stage || snapshot.progress?.stage || task.stage;
  task.completedAt = snapshot.completed_at || "";
  const inactiveSession = ["idle", "persisted_only"].includes(snapshotSessionState);
  task.active = !["cancelled", "completed", "failed", "no_results"].includes(snapshot.status || "") && !inactiveSession && (snapshot.runtime_attached !== false || snapshot.session_state === "paused_runtime");

  if (snapshot.mcis_progress) {
    updateMCISProgress({ ...snapshot.mcis_progress }, snapshot.current_stage === "stage0_mcis");
  }
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
  return { path: normalizedExportPath };
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
    void navigateTo("results");
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

      const resultsResult = await listTaskResults(currentTaskId, resultSortBy.value, resultOrder.value, buildResultFileFallbackPayload(task.exportPath, snapshotForResults), resultIpFilter.value, {
        limit: resultsPageLimit.value,
        offset: 0,
      });
      appendLog("bridge.list_task_results", resultsResult);

      if (resultsResult.ok && resultsResult.data) {
        const nextRows = Array.isArray(resultsResult.data.results) ? resultsResult.data.results : [];
        const totalCount = asCount(resultsResult.data.total_count, nextRows.length);
        applyCurrentTaskResultWorkspace(currentTaskId, nextRows, totalCount);
      }
    } while (snapshotRefreshPending);
  } finally {
    snapshotRefreshInFlight = false;
    resultsLoading.value = false;
    snapshotRefreshQueuedTaskId = "";
  }
}

function notifyUploadResult(event: ProbeEventEnvelope) {
  if (event.event !== "upload.notification") return;
  const status = asString(event.payload.status) || "completed";
  const body = asString(event.payload.message) || `上传结论：${status}`;
  if (appInfo.value.platform === "android") {
    void (async () => {
      if (!androidNotificationStatus.value?.granted) {
        const permission = await requestNotificationPermission();
        androidNotificationStatus.value = permission.data || androidNotificationStatus.value;
      }
      if (androidNotificationStatus.value?.granted) await showAndroidNotification("CFST 上传结论", body);
    })();
    return;
  }
  // 桌面端不在这里发通知：Go 侧在同一个 upload.notification 事件上补发系统通知，
  // 这样主窗口隐藏在托盘时也能收到（见 internal/app/desktop_notify.go）。
}

// 403 / 1034 淘汰事件的明细：后端把同一合并窗口内的拒绝放进 payload.items，
// 老形态（单条 payload）为了兼容仍然照单条读。
function edgeRejectedItems(event: ProbeEventEnvelope): Array<Record<string, unknown>> {
  const items = event.payload.items;
  if (Array.isArray(items) && items.length > 0) {
    return items.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === "object");
  }
  return [event.payload];
}

function applyProbeEvent(event: ProbeEventEnvelope) {
  const incomingTaskId = asString(event.task_id).trim();
  const currentTaskId = task.taskId.trim();
  const sequenceDecision = classifyProbeEventSequence(currentTaskId, task.lastSeq, event);
  if (sequenceDecision === "foreign") {
    appendLog(`${event.event}.ignored`, {
      current_task_id: currentTaskId,
      event_task_id: incomingTaskId,
      payload: event.payload,
    });
    return;
  }
  if (sequenceDecision === "duplicate") {
    appendLog(`${event.event}.duplicate`, {
      event_seq: event.seq,
      last_seq: task.lastSeq,
      task_id: incomingTaskId || currentTaskId,
    });
    return;
  }
  if (sequenceDecision === "gap") {
    appendLog(`${event.event}.sequence_gap`, {
      event_seq: event.seq,
      last_seq: task.lastSeq,
      task_id: incomingTaskId || currentTaskId,
    });
    void reconcileTaskData(incomingTaskId || currentTaskId);
  }

  appendLog(event.event, event.payload);
  notifyUploadResult(event);
  const nextTaskState = deriveTaskStateFromProbeEvent(event);
  const progressStage = event.event === "probe.mcis.progress" ? "stage0_mcis" : event.event === "probe.progress" || event.event === "probe.speed" ? asString(event.payload.stage) || "running" : "";
  const refreshProgressUI = !progressStage || shouldRefreshProgressUI(progressStage, asCount(event.payload.completed ?? event.payload.processed), asCount(event.payload.total));

  if (refreshProgressUI) {
    setStatus(nextTaskState);
  }
  task.active = !["cancelled", "completed", "failed", "no_results"].includes(nextTaskState.tone);
  if (["cancelled", "completed", "failed", "no_results"].includes(nextTaskState.tone)) {
    mcisProgress.active = false;
  }
  task.lastEvent = event.event;
  task.lastSeq = event.seq;
  task.taskId = event.task_id || task.taskId;
  const eventDebugLogPath = eventDebugLogDisplayPath(event.payload);
  const eventDebugLogTarget = eventDebugLogOpenTarget(event.payload);
  const traceFailureSummary = eventTraceFailureSummary(event.payload);

  if (event.event === "probe.mcis.progress") {
    finishTaskAction("start");
    finishTaskAction("rerun");
    taskSessionState.value = "active_runtime";
    task.stage = "stage0_mcis";
    if (refreshProgressUI) {
      resetDownloadSpeedState();
      updateMCISProgress(event.payload);
      const sourceLabel = mcisProgress.sourceName || "当前输入源";
      const latestLabel = mcisProgress.lastIP ? `，最近 ${mcisProgress.lastIP}${mcisProgress.lastColo ? `(${mcisProgress.lastColo})` : ""}` : "";
      pushProcessTrace({
        detail: `${sourceLabel}，已完成 ${mcisProgress.completed}/${mcisProgress.total || "-"}，成功 ${mcisProgress.succeeded}，失败/超时 ${mcisProgress.failed}，候选 ${mcisProgress.candidateCount}${latestLabel}。`,
        stage: "stage0_mcis",
        title: "MICS 抽样进行中",
        tone: "running",
        ts: event.ts,
      });
    }
  }
  if (event.event === "probe.preprocessed") {
    finishTaskAction("start");
    finishTaskAction("rerun");
    taskSessionState.value = "active_runtime";
    resetDownloadSpeedState();
    mcisProgress.active = false;
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
    mcisProgress.active = false;
    task.stage = asString(event.payload.stage) || "running";
    const completed = asCount(event.payload.processed);
    if (refreshProgressUI) {
      summary.failed = asCount(event.payload.failed);
      summary.passed = asCount(event.payload.passed);
      summary.processed = completed;
      summary.total = asCount(event.payload.total, summary.total);
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
    // 单并发时保留原来“换 IP 就重置面板”的行为；多并发时所有 IP 共用一张表，不能互相清空。
    if (activeDownloadConcurrency.value <= 1 && downloadSpeedState.ip && downloadSpeedState.ip !== ip) {
      resetDownloadSpeedState();
    }
    // Android 的 200ms 进度节流只限制状态文案与进程日志；每 IP 的实时行必须逐采样更新。
    // 否则同一秒内并发采样的其它 IP 会全部落在节流窗口里，表里只剩第一颗 IP。
    downloadSpeedState.active = true;
    const averageDisplayReady = averageReady && (averageSpeed !== 0 || bytesRead > 0 || measuredBytes > 0 || bodyRead || transferComplete);
    const currentDisplaySpeed = currentReady ? currentSpeed : null;
    const averageDisplaySpeed = averageDisplayReady ? averageSpeed : null;
    downloadSpeedState.averageSpeedMbS = averageDisplaySpeed;
    downloadSpeedState.bytesRead = bytesRead;
    downloadSpeedState.colo = colo;
    downloadSpeedState.currentSpeedMbS = currentDisplaySpeed;
    downloadSpeedState.elapsedMs = elapsedMs;
    downloadSpeedState.ip = ip;
    const rowNow = Date.now();
    upsertDownloadSpeedRow(
      downloadSpeedRows,
      {
        averageSpeedMbS: averageDisplaySpeed,
        bytesRead,
        colo,
        currentSpeedMbS: currentDisplaySpeed,
        elapsedMs,
        ip,
        updatedAt: rowNow,
      },
      { now: rowNow, ttlMs: downloadSpeedRowTtlMs(activeDownloadSampleIntervalMs.value) },
    );
    ensureDownloadSpeedRowPruneTimer();
    if (refreshProgressUI) {
      updateDownloadSpeedTrace(ip, colo, event.ts);
    }
  }

  if (event.event === "probe.edge_rejected") {
    // 403 / 1034 淘汰不受 debug 开关约束：进程面板要能直接看到被边缘拒绝的 IP。
    // 后端按 250ms 窗口把同一批拒绝合并进 payload.items，早先的单条形态照样能读。
    for (const item of edgeRejectedItems(event)) {
      const note = asString(item.note).trim() || "403";
      const rejectedIp = asString(item.ip).trim();
      pushProcessTrace({
        detail: asString(item.message).trim() || `${rejectedIp || "当前 IP"} 被边缘拒绝（${note}）。`,
        stage: asString(item.stage).trim() || "stage3_get",
        title: note === "1034" ? "边缘 IP 受限 1034" : "HTTP 403 拒绝",
        tone: "warning",
        ts: event.ts,
      });
    }
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
    finishTaskAction();
    taskSessionState.value = "idle";
    task.active = false;
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
    showToast(hasResults ? "探测任务已完成" : "任务结束但没有可用结果", hasResults ? "success" : "info");
  }

  if (event.event === "probe.failed") {
    finishTaskAction();
    taskSessionState.value = activeTaskSessionState.value === "persisted_only" || taskSnapshot.value?.session_state === "persisted_only" ? "persisted_only" : "idle";
    task.active = false;
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
    showToast(failureMessage, "error");
  }

  if (event.event === "probe.cancelled") {
    finishTaskAction();
    taskSessionState.value = "persisted_only";
    task.active = false;
    task.completedAt = event.ts;
    resetDownloadSpeedState();
    const cancelMessage = asString(event.payload.message || nextTaskState.detail).trim() || "测速任务已终止。";
    updateHistory({
      debugLogPath: eventDebugLogPath,
      debugLogTarget: eventDebugLogTarget,
      detail: cancelMessage,
      exported: summary.exported,
      failureSummary: "",
      targetPath: task.exportPath,
      taskId: task.taskId,
      title: "任务已终止",
      tone: "cancelled",
      updatedAt: event.ts,
    });
    pushProcessTrace({
      detail: cancelMessage,
      stage: "cancelled",
      title: "探测任务已终止",
      tone: "warning",
      ts: event.ts,
    });
    showToast("探测任务已终止", "info");
  }

  if (event.event === "probe.cooling") {
    const recoverable = asBoolean(event.payload.recoverable, true);
    if (recoverable) {
      finishTaskAction("pause");
      taskSessionState.value = "paused_runtime";
    } else {
      taskSessionState.value = "active_runtime";
    }
    task.active = true;
    task.stage = "cooling";
    downloadSpeedState.active = false;
    clearDownloadSpeedRows(downloadSpeedRows);
    stopDownloadSpeedRowPruneTimer();
    clearDownloadSpeedTraceEntries();
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

async function inspectSource(sourceId: string) {
  if (sourceRequestStates[sourceId]) return;
  applyDetectedSourceName(sourceId);
  const sourceIndex = sources.value.findIndex((entry) => entry.id === sourceId);
  const source = sources.value[sourceIndex];
  if (!source) return;
  const issue = sourceInputIssue(source);
  if (issue) {
    sourcePreviewErrors[sourceId] = issue;
    return;
  }
  const inputKey = sourcePreviewKey(source, sourceGroupColoFilter.value, sourceGroupColoFilterMode.value);
  const isCurrent = () => {
    const current = sources.value.find((entry) => entry.id === sourceId);
    return current && sourcePreviewKey(current, sourceGroupColoFilter.value, sourceGroupColoFilterMode.value) === inputKey;
  };
  sourceRequestStates[sourceId] = "preview";
  delete sourcePreviewErrors[sourceId];
  try {
    const result = await previewSource({
      config: buildConfigSnapshot(),
      preview_limit: 16,
      source: { ...sourcePayloads.value[sourceIndex], content: source.content.trim(), name: sourceNameForPayload(source, sourceIndex), path: source.path.trim(), url: source.url.trim() },
    });
    appendLog("bridge.source_preview", result);
    if (!isCurrent()) return;
    if (!result.ok) {
      sourcePreviewErrors[sourceId] = result.message || "读取来源失败。";
      return;
    }
    applySourcePreview(sourceId, asRecord(result.data as SourcePreviewPayload | null), result.warnings || []);
  } catch (error) {
    if (isCurrent()) sourcePreviewErrors[sourceId] = error instanceof Error ? error.message : "读取来源失败。";
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
    if (data.source_profiles || data.sourceProfiles) {
      applySourceProfileStore(data.source_profiles || data.sourceProfiles);
    }
    markSourceSaveBaselines();
    await maybeRestoreDraft(data.draft_status || data.draftStatus);
    lastSavedSnapshotSignature = currentSnapshotSignature();
    configHydrated = true;
    await refreshDroppedSources();
    startupTimedOut.value = false;
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

// Wails 在 navigationCompleted 之后才注入宿主运行时标记（见 lib/wailsRuntime.ts）并派发
// "wails:runtime-config-ready"（见 wails internal/runtime/runtime.go 的 runtimeConfigReady）。
// 挂载前已由 resolveBridgeMode 定好通道（见 lib/bridge.ts），而它正是等到宿主运行时标记
// 出现才挂载，所以正常路径下这个事件一定早于本监听器注册，下面这段只在「宿主比等待上限更晚
// 注入」时才生效：就绪后重拉一次 appInfo，让版本号等字段从占位值回填，并把可能误绑到
// WebUI SSE 的事件通道校正回 Wails。
function handleWailsRuntimeReady() {
  void refreshAppInfo();
  // 运行时就绪后，若启动竞态使 probe 事件通道误绑到 WebUI SSE，则在此校正回真实的 Wails 事件通道。
  void rebindProbeEventListener().catch(() => {
    // 重绑失败时保留现有通道；SSE onerror 与下一次就绪事件仍会再次触发校正。
  });
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

const settingsAutoSaver = createSettingsAutoSaver(() => {
  void runAutoSaveSettings("interaction");
});

// 设置页的点击/失焦会连发，逐次保存要重建整份配置快照并稳定化；合并到一次。
async function autoSaveSettings(reason: string) {
  if (reason === "interaction") {
    settingsAutoSaver.schedule(reason);
    return true;
  }
  settingsAutoSaver.takePending();
  return runAutoSaveSettings(reason);
}

async function runAutoSaveSettings(reason: string) {
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
  const requestedSourceSignature = currentSourceSignature();
  if (options.skipIfUnchanged && requestedSignature === lastSavedSnapshotSignature) {
    return true;
  }

  if (saveBlockedByMaskedToken.value) {
    markSaveState("error");
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
      applyNavigation("settings");
    }
    showToast("需要重新输入完整 Token", "error");
    return false;
  }

  markSaveState("saving");
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
      markSaveState("error");
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
    const currentStillMatchesRequest = currentSnapshotSignature() === requestedSignature && currentSourceSignature() === requestedSourceSignature;
    if (currentStillMatchesRequest) {
      applyConfigSnapshot(normalizedSnapshot);
    }
    applyStorageStatus(data.storage);
    if (currentStillMatchesRequest) {
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
    markSaveState("saved");
    await refreshSchedulerStatus();
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    appendLog("bridge.save_config.failed", message);
    markSaveState("error");
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
      detail: "当前还不能开始测速：请先到“输入源”添加一个已启用的来源，并填写 URL、文件路径或 IP 内容。",
      title: "缺少输入源",
      tone: "failed",
    });
    showToast("请先配置至少一个输入源", "error");
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
  void navigateTo("dashboard");

  try {
    const launchConfig = buildConfigSnapshot();
    captureDownloadSpeedTaskConfig(launchConfig);
    const result = await startProbe({
      config: launchConfig,
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
  void navigateTo("dashboard");

  try {
    const launchConfig = buildConfigSnapshot();
    captureDownloadSpeedTaskConfig(launchConfig);
    const result = await startProbe({
      config: launchConfig,
      sources: [
        {
          content: trimmedAddress,
          enabled: true,
          id: `rerun-${Date.now()}`,
          ip_limit: -1,
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
        void navigateTo("dashboard");
      } else if (taskSessionState.value === "persisted_only") {
        finishTaskAction();
        setStatus({
          detail: "已恢复上次任务快照和结果文件，但原生运行时会话已经结束，需要重新启动任务。可在「结果」页查看已落盘结果。",
          title: "已恢复历史结果",
          tone: "warning",
        });
        pushActivity("恢复到已落盘结果", "已读取 Android 任务快照，但当前没有可重连的原生活动会话。");
        // 冷启动统一落在任务看板：结果行照旧读取（下面 refreshTaskData），但不再把用户
        // 直接推进结果页；要看历史结果由用户自己在「结果」页打开。
      }
      if (runtimeTaskId) {
        await refreshTaskData(runtimeTaskId);
      }
    }
  } catch (error) {
    appendLog("bridge.get_android_runtime_status.failed", error instanceof Error ? error.message : String(error));
  }
}

async function restoreLatestTaskState() {
  const result = await listTaskSnapshots(1);
  appendLog("bridge.list_task_snapshots", result);
  const latest = result.ok && result.data && Array.isArray(result.data.items) ? result.data.items[0] : null;
  if (!latest?.task_id) {
    return;
  }
  if (task.taskId === latest.task_id && taskSnapshot.value) {
    return;
  }
  if (hasActiveTask.value && task.taskId && task.taskId !== latest.task_id) {
    return;
  }

  finishTaskAction();
  applyTaskSnapshot(latest);
  const sessionState = asString(latest.session_state || "idle").trim();
  const snapshotStatus = asString(latest.status || "").trim();
  const failureSummary = summarizeFailureSummary(latest.failure_summary);
  let restoredStatus: { detail: string; title: string; tone: TaskTone };
  if (sessionState === "active_runtime" || sessionState === "paused_runtime") {
    restoredStatus = {
      detail: sessionState === "paused_runtime" ? "已重新接入暂停中的探测任务，可以继续执行。" : "已重新接入仍在运行的探测任务，界面将继续接收实时进度。",
      title: sessionState === "paused_runtime" ? "任务已恢复" : "任务重新接入",
      tone: sessionState === "paused_runtime" ? "cooling" : "running",
    };
  } else {
    const tone = (["cancelled", "completed", "failed", "no_results"] as TaskTone[]).includes(snapshotStatus as TaskTone) ? (snapshotStatus as TaskTone) : "warning";
    restoredStatus = {
      detail: failureSummary || (snapshotStatus === "completed" ? "已恢复最近一次任务快照和结果。" : "已恢复最近一次任务快照；原运行会话已结束，不能直接继续。"),
      title: snapshotStatus === "completed" ? "已恢复历史结果" : "已恢复历史任务",
      tone,
    };
  }
  setStatus(restoredStatus);
  pushActivity(restoredStatus.title, restoredStatus.detail);
  updateHistory({
    detail: restoredStatus.detail,
    exported: summary.exported,
    failureSummary,
    targetPath: task.exportPath,
    taskId: latest.task_id,
    title: restoredStatus.title,
    tone: restoredStatus.tone,
    updatedAt: latest.updated_at || latest.completed_at || latest.started_at || new Date().toISOString(),
  });
  await reconcileTaskData(latest.task_id, {
    // 冷启动落在任务看板：结果行仍会随 reconcileTaskData 读进来，但不自动切到结果页。
    switchToResultsOnData: false,
  });
  if (sessionState === "active_runtime" || sessionState === "paused_runtime") {
    void navigateTo("dashboard");
  }
}

async function loadMoreResults() {
  const normalizedTaskId = task.taskId.trim();
  if (!normalizedTaskId || resultRows.value.length >= resultsTotalCount.value) {
    return;
  }
  resultsLoading.value = true;
  try {
    const result = await listTaskResults(normalizedTaskId, resultSortBy.value, resultOrder.value, buildResultFileFallbackPayload(task.exportPath, taskSnapshot.value), resultIpFilter.value, {
      limit: resultsPageLimit.value,
      offset: resultRows.value.length,
    });
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

async function cancelProbe() {
  if (!task.taskId || !canCancelTask.value) {
    return;
  }
  if (!window.confirm("确定要停止这次测速吗？已经得到的结果会保留，但未完成的项目不会继续测速。")) {
    return;
  }

  beginTaskAction("cancel", "", task.taskId);
  loading.value = true;

  try {
    const result = await stopProbe({
      mode: "cancel",
      task_id: task.taskId,
    });
    appendLog("bridge.stop_probe.cancel", result);
    const pending = result.code === "PROBE_CANCEL_PENDING";

    if (!result.ok && !pending) {
      finishTaskAction("cancel");
      setStatus({
        detail: result.message || "终止任务失败。",
        title: "终止失败",
        tone: "failed",
      });
      showToast("终止任务失败", "error");
      return;
    }

    setStatus({
      detail: result.message || "终止请求已发送，正在等待任务退出。",
      title: "正在终止任务",
      tone: "warning",
    });
    pushActivity("请求终止", result.message || "终止请求已发送，正在等待任务退出。");
    showToast("已请求终止任务", "info");
  } catch (error) {
    finishTaskAction("cancel");
    const message = error instanceof Error ? error.message : "终止任务失败。";
    setStatus({ detail: message, title: "终止失败", tone: "failed" });
    showToast(message, "error");
  } finally {
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
    void navigateTo("results");
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

async function loadDnsProviderSubdomains(providerId: string, silent = false) {
  const provider = settings.dnsProviders.find((entry) => entry.id === providerId);
  if (!provider || provider.provider !== "dnshe") {
    return;
  }
  dnsSubdomainsLoading.value = { ...dnsSubdomainsLoading.value, [providerId]: true };
  dnsSubdomainErrors.value = { ...dnsSubdomainErrors.value, [providerId]: "" };
  try {
    const result = await listDnsSubdomains({
      config: buildConfigSnapshot(),
      provider_id: providerId,
    });
    appendLog("bridge.list_dns_subdomains", result);
    if (!result.ok) {
      dnsSubdomainErrors.value = { ...dnsSubdomainErrors.value, [providerId]: result.message || "读取 DNSHE 子域名失败。" };
      if (!silent) {
        showToast(result.message || "读取 DNSHE 子域名失败", "error");
      }
      return;
    }
    const data = asRecord(result.data);
    const items = Array.isArray(data.subdomains)
      ? data.subdomains.map((item) => {
          const record = asRecord(item);
          return {
            full_domain: asString(record.full_domain),
            id: asCount(record.id, 0),
            rootdomain: asString(record.rootdomain),
            status: asString(record.status),
            subdomain: asString(record.subdomain),
          };
        })
      : [];
    dnsSubdomainsByProvider.value = { ...dnsSubdomainsByProvider.value, [providerId]: items };
    if (!silent) {
      showToast(items.length > 0 ? `已读取 ${items.length} 个 DNSHE 子域名` : "该账号下没有可用子域名", items.length > 0 ? "success" : "info");
    }
  } finally {
    dnsSubdomainsLoading.value = { ...dnsSubdomainsLoading.value, [providerId]: false };
  }
}

async function fetchDnsRecords() {
  if (dnsReadScope.value === "custom" && !dnsReadName.value.trim()) {
    void navigateTo("dns");
    showToast("请输入要读取的子域名或记录名", "error");
    return;
  }

  isLoadingDns.value = true;

  try {
    const payload: Record<string, unknown> = {
      config: buildConfigSnapshot(),
      scope: dnsReadScope.value,
    };
    if (dnsReadProviderId.value.trim()) {
      payload.provider_id = dnsReadProviderId.value.trim();
    }
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
      pushActivity("读取 DNS 失败", result.message || "未能读取所选平台的 DNS 记录。");
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

function normalizeDnsSelfTestStatus(value: unknown): "passed" | "failed" | "skipped" {
  const status = asString(value).trim();
  return status === "passed" || status === "failed" ? status : "skipped";
}

function normalizeDnsSelfTestResult(data: Record<string, unknown>): DnsSelfTestResult {
  const stages = Array.isArray(data.stages)
    ? data.stages.map((item) => {
        const stage = asRecord(item);
        return {
          details: asRecord(stage.details),
          duration_ms: asCount(stage.duration_ms),
          id: asString(stage.id),
          message: asString(stage.message),
          status: normalizeDnsSelfTestStatus(stage.status),
          title: asString(stage.title),
        };
      })
    : [];
  return {
    alt_ttl: asCount(data.alt_ttl),
    base_ttl: asCount(data.base_ttl),
    cleanup_ok: asBoolean(data.cleanup_ok, true),
    failed: asCount(data.failed),
    passed: asCount(data.passed),
    provider: asString(data.provider),
    provider_id: asString(data.provider_id),
    record_name: asString(data.record_name),
    skipped: asCount(data.skipped),
    stages,
    test_name: asString(data.test_name),
    warnings: Array.isArray(data.warnings) ? data.warnings.map((warning) => asString(warning)).filter(Boolean) : [],
    wrote_records: asBoolean(data.wrote_records, true),
  };
}

async function runDNSSelfTest(payload: { providerId: string; testName: string }) {
  if (dnsSelfTesting.value) {
    return;
  }
  const provider = enabledDNSProviders().find((entry) => entry.id === payload.providerId);
  if (!provider) {
    showToast("请先在设置页添加并启用 DNS 推送平台", "error");
    return;
  }
  const platformLabel = dnsProviderPlatformLabel(provider.provider);

  dnsSelfTesting.value = true;
  dnsSelfTestResult.value = null;
  try {
    const result = await runDnsSelfTest({
      config: buildConfigSnapshot(),
      provider_id: payload.providerId,
      test_name: payload.testName,
    });
    appendLog("bridge.dns_selftest", result);
    const normalized = normalizeDnsSelfTestResult(asRecord(result.data));
    dnsSelfTestResult.value = normalized;
    probeWarnings.value = result.warnings || [];
    pushWarningTrace(probeWarnings.value);

    if (!result.ok) {
      const detail = result.message || `${platformLabel} 自检未通过。`;
      setStatus({
        detail,
        title: `${platformLabel} 自检失败`,
        tone: "failed",
      });
      pushActivity(`${platformLabel} 自检失败`, detail);
      showToast(`${platformLabel} 自检失败`, "error");
      return;
    }

    const detail = result.message || `${platformLabel} 自检通过。`;
    setStatus({
      detail,
      title: normalized.skipped > 0 ? `${platformLabel} 自检部分通过` : `${platformLabel} 自检通过`,
      tone: "completed",
    });
    pushActivity(normalized.skipped > 0 ? `${platformLabel} 自检部分通过` : `${platformLabel} 自检通过`, detail);
    showToast(normalized.skipped > 0 ? `${platformLabel} 自检部分通过` : `${platformLabel} 自检通过`, "success");
  } catch (error) {
    const message = error instanceof Error ? error.message : "DNS 自检失败";
    setStatus({ detail: message, title: `${platformLabel} 自检失败`, tone: "failed" });
    pushActivity(`${platformLabel} 自检失败`, message);
    showToast(`${platformLabel} 自检失败`, "error");
  } finally {
    dnsSelfTesting.value = false;
  }
}

function probeURLHostName() {
  const raw = settings.probeURL.trim();
  if (!raw) {
    return "";
  }
  try {
    return new URL(raw).hostname;
  } catch {
    return "";
  }
}

async function testDohConnection() {
  if (dohTesting.value) {
    return;
  }
  dohTesting.value = true;
  try {
    const result = await testDoH({
      doh_url: settings.probeDoHURL.trim(),
      ech: settings.probeECH,
      ech_config: settings.probeECHConfig.trim(),
      ech_domain: settings.probeECHDomain.trim(),
      host: probeURLHostName(),
    });
    appendLog("bridge.doh_test", result);
    const warnings = (result.warnings || []).join(" ");
    dohTestResult.value = {
      message: [result.message || (result.ok ? "DoH 可用。" : "DoH 测试失败。"), warnings].filter(Boolean).join(" "),
      ok: result.ok,
    };
    if (!result.ok) {
      setStatus({ detail: result.message || "DoH 端点没有返回有效答案。", title: "DoH 测试失败", tone: "failed" });
      pushActivity("DoH 测试失败", result.message || "DoH 端点没有返回有效答案。");
      showToast("DoH 测试失败", "error");
      return;
    }
    pushActivity("DoH 测试通过", result.message || "DoH 端点可用。");
    showToast("DoH 测试通过", "info");
  } catch (error) {
    const message = error instanceof Error ? error.message : "DoH 测试失败";
    dohTestResult.value = { message, ok: false };
    showToast(message, "error");
  } finally {
    dohTesting.value = false;
  }
}

async function exportCurrentResultsToGitHub() {
  const visibleRows = currentVisibleResultRows();
  if (visibleRows.length === 0) {
    void navigateTo("results");
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

function dnsProviderTargetLabel(entry: DnsProviderForm) {
  const recordName = entry.recordName.trim();
  if (recordName) {
    return dnsProviderResolvedRecordName(entry);
  }
  if (entry.provider === "dnshe" && entry.subdomain.trim()) {
    return `子域名 ${entry.subdomain.trim()}`;
  }
  return "";
}

// DNSHE 自检固定落在主域名（子域 zone 顶点）下的 cfst-selftest 子域。读取过账号
// 子域名时按最长后缀求主域名；没读取过时退回完整域名本身。
function dnsProviderZoneName(entry: DnsProviderForm) {
  const target = dnsProviderResolvedRecordName(entry);
  if (entry.provider !== "dnshe") {
    return target;
  }
  const subdomain = entry.subdomain.trim().replace(/\.$/, "");
  if (subdomain.includes(".")) {
    return subdomain.toLowerCase();
  }
  const known = dnsSubdomainsByProvider.value[entry.id] ?? [];
  let best = "";
  for (const item of known) {
    const full = (item.full_domain || `${item.subdomain}.${item.rootdomain}`).trim().replace(/\.$/, "").toLowerCase();
    if (!full) {
      continue;
    }
    if (full === target.toLowerCase() || target.toLowerCase().endsWith(`.${full}`)) {
      if (full.length > best.length) {
        best = full;
      }
    }
  }
  return best || target;
}

// DNSHE 的裸子域名标签按 <标签>.<完整域名> 解释；只有填写完整域名，或账号下已读取到
// 同名子域时，才按子域选择处理。自检默认测试名与读取目标都要跟着同一结果。
function dnsProviderResolvedRecordName(entry: DnsProviderForm) {
  const recordName = entry.recordName.trim().replace(/\.$/, "");
  const subdomain = entry.subdomain.trim();
  if (entry.provider !== "dnshe" || subdomain === "" || subdomain.includes(".")) {
    return recordName || subdomain;
  }
  const known = dnsSubdomainsByProvider.value[entry.id] ?? [];
  if (known.some((item) => item.subdomain.toLowerCase() === subdomain.toLowerCase() || item.full_domain.toLowerCase() === subdomain.toLowerCase())) {
    return recordName || subdomain;
  }
  if (!recordName) {
    return subdomain;
  }
  if (recordName.toLowerCase().startsWith(`${subdomain.toLowerCase()}.`)) {
    return recordName;
  }
  return `${subdomain}.${recordName}`;
}

const dnsPushTargets = computed(() =>
  enabledDNSProviders().map((entry) => ({
    id: entry.id,
    label: entry.label.trim() || `${dnsProviderPlatformLabel(entry.provider)}${dnsProviderTargetLabel(entry) ? ` · ${dnsProviderTargetLabel(entry)}` : ""}`,
    recordTarget: Boolean(entry.recordName.trim() || entry.subdomain.trim()),
  })),
);

function resultPushTargetProvider() {
  const enabled = enabledDNSProviders();
  return enabled.find((entry) => entry.id === resultCloudflarePushSettings.providerId) ?? enabled[0];
}

function dnsProviderPlatformLabel(provider: DNSProviderKind) {
  switch (provider) {
    case "dnshe":
      return "DNSHE";
    case "aliyun":
      return "阿里云 DNS";
    case "dnspod":
      return "DNSPod";
    default:
      return "Cloudflare";
  }
}

async function pushCurrentResultsToCloudflare() {
  const visibleRows = currentVisibleResultRows();
  if (visibleRows.length === 0) {
    void navigateTo("results");
    showToast("没有可推送的测速结果", "error");
    return;
  }

  const target = resultPushTargetProvider();
  if (!target) {
    void navigateTo("settings");
    showToast("请先在设置页添加并启用 DNS 推送平台", "error");
    return;
  }
  const platformLabel = dnsProviderPlatformLabel(target.provider);
  const routingActive = Boolean(target.routingEnabled && target.routingRules.some((rule) => rule.enabled && rule.recordName.trim()));
  const recordName = resultCloudflarePushSettings.recordName.trim() || target.recordName.trim();
  const subdomainTarget = target.provider === "dnshe" && Boolean(target.subdomain.trim());
  if (!routingActive && !recordName && !subdomainTarget) {
    void navigateTo("results");
    showToast("请先填写 DNS 记录名称", "error");
    return;
  }

  const topN = nonNegativeCount(resultCloudflarePushSettings.topN, 0);
  const selectedRows = limitRowsForQuickPush(visibleRows, topN);
  if (selectedRows.length === 0) {
    void navigateTo("results");
    showToast("当前筛选结果没有可推送 IP", "error");
    return;
  }

  resultCloudflarePushSettings.providerId = target.id;
  saveResultCloudflarePushSettings();
  cloudflarePushing.value = true;
  try {
    const config = normalizeConfigSnapshot(buildConfigSnapshot());
    const entry = config.dns_providers.find((item) => item.id === target.id);
    if (entry) {
      entry.top_n = topN;
      if (!routingActive) {
        entry.record_name = recordName;
        entry.record_type = normalizeResultCloudflareRecordType(resultCloudflarePushSettings.recordType);
        entry.routing_enabled = false;
        entry.routing_rules = [];
      }
    }
    if (target.provider === "cloudflare") {
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
    }

    const result = await pushDnsRecords({
      config,
      notification_trigger: "manual_push",
      provider_id: target.id,
      results: selectedRows,
      task_id: task.taskId,
    });
    const data = asRecord(result.data);
    const providerReport = Array.isArray(data.providers) ? asRecord((data.providers as unknown[])[0]) : {};
    appendLog("bridge.push_results_dns", result);
    probeWarnings.value = result.warnings || [];
    pushWarningTrace(probeWarnings.value);
    if (!result.ok) {
      setStatus({
        detail: result.message || `当前结果推送到 ${platformLabel} 失败。`,
        title: `${platformLabel} 推送失败`,
        tone: "failed",
      });
      pushActivity(`${platformLabel} 推送失败`, result.message || "未能覆盖目标 DNS 记录。");
      showToast(`${platformLabel} 推送失败`, "error");
      return;
    }

    if (asBoolean(providerReport.routing_enabled || data.routing_enabled || data.routingEnabled, false)) {
      const detail = asString(providerReport.message) || result.message || `${platformLabel} 分流推送完成。`;
      const partial = asString(providerReport.status) === "partial";
      setStatus({
        detail,
        title: partial ? `${platformLabel} 推送部分完成` : `${platformLabel} 推送完成`,
        tone: partial ? "partial" : "completed",
      });
      pushActivity(partial ? `${platformLabel} 推送部分完成` : `${platformLabel} 推送完成`, detail);
      showToast(partial ? `${platformLabel} 部分目标推送完成` : `已推送到 ${platformLabel}`, partial ? "info" : "success");
      return;
    }

    const summaryRecord = asRecord(providerReport.summary ?? data.summary);
    const created = asCount(summaryRecord.created);
    const updated = asCount(summaryRecord.updated);
    const deleted = asCount(summaryRecord.deleted);
    const ignored = asCount(summaryRecord.ignored);
    const uploadCount = asCount(providerReport.upload_count || data.upload_count || data.uploadCount, selectedRows.length);
    const detail = `已推送 ${uploadCount} 条到 ${platformLabel} ${recordName}：创建 ${created}、更新 ${updated}、删除 ${deleted}、忽略 ${ignored}。`;
    setStatus({
      detail,
      title: `${platformLabel} 推送完成`,
      tone: "completed",
    });
    pushActivity(`${platformLabel} 推送完成`, result.message || detail);
    showToast(`已推送到 ${platformLabel}`, "success");
  } finally {
    cloudflarePushing.value = false;
  }
}

async function exportCurrentResultsCSV() {
  const visibleRows = currentVisibleResultRows();
  if (visibleRows.length === 0) {
    void navigateTo("results");
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
  () => sources.value.map((source) => [source.id, sourcePreviewKey(source, sourceGroupColoFilter.value, sourceGroupColoFilterMode.value)] as const),
  (current, previous) => {
    const previousKeys = new Map(previous);
    const currentKeys = new Map(current);
    for (const [id, key] of previousKeys) {
      if (currentKeys.get(id) !== key) {
        delete sourcePreviewStates[id];
        delete sourcePreviewErrors[id];
      }
    }
  },
);
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
  isAndroidApp,
  (android) => {
    if (!android) {
      uninstallAndroidViewportTracking();
      return;
    }
    installAndroidViewportTracking();
  },
  { immediate: true },
);

watch(selectedView, (view, previousView) => {
  if (previousView === "results" && view !== "results") {
    clearCurrentTaskResultWorkspace({ preserveSnapshot: true });
  }
});

// 看板展示状态的回写：状态、统计、任务身份与最近动态任一变化都会调度一次节流写入。
watch([() => status.title, () => status.detail, () => status.tone, summary, task, taskSnapshot, activityFeed], scheduleDashboardCacheSave, { deep: true });

onMounted(async () => {
  window.addEventListener("resize", scheduleViewportSizeRefresh);
  window.addEventListener("beforeunload", handleBeforeUnload);
  window.addEventListener("pagehide", flushPendingSettingsSave);
  document.addEventListener("visibilitychange", flushPendingSettingsSave);
  window.addEventListener("focus", handleAndroidNotificationWindowFocus);
  window.addEventListener("wails:runtime-config-ready", handleWailsRuntimeReady);
  document.addEventListener("visibilitychange", handleAndroidNotificationVisibilityChange);
  installThemeTracking();
  await runStartupStep("viewport.startup", ensureAdaptiveViewportOnStartup);
  if (startupSyncing.value) {
    armStartupFallback();
  }
  appendLog("system.boot", { message: "桌面端调用链已初始化。" });
  pushActivity("应用已启动", "正在读取配置与任务状态。");
  const subscribeProbeEvents = async () => {
    removeProbeListener = await listenToProbeEvents(applyProbeEvent);
  };
  await runStartupStep("probe.listen", subscribeProbeEvents);
  if (isWailsDesktopHost()) {
    removeDroppedSourcesListener = EventsOn("sources:dropped", () => {
      void refreshDroppedSources();
    });
  }
  // 首帧优先：到这里挂载已经完成、探测与拖放监听都已注册，但浏览器还没机会把界面画出来。
  // 立刻跑下面的启动 IPC 链会把首帧一起拖后（慢机上是几百毫秒的骨架屏），因此先让出一帧，
  // 让用户先看到用缓存回填的看板，再开始读配置与任务快照。让位只推迟一帧，监听器已就位，
  // 探测事件的最终状态本来就由任务快照校正，不会因此漏掉结论。
  // 监听器必须先注册，避免后台任务在首屏恢复期间漏事件；应用信息与配置彼此独立，随后并行读取。
  await Promise.all([runStartupStep("app_info.refresh", refreshAppInfo), runStartupStep("config.refresh", refreshConfig)]);
  if (!removeProbeListener) {
    await runStartupStep("probe.listen.retry", subscribeProbeEvents);
  }
  loadResultCloudflarePushSettings();
  loadResultGitHubTopN();
  resultCloudflarePushSettingsHydrated = true;
  // 上面几步决定桥通道与配置，必须先落地。其余启动步骤互不依赖，并行推进：每个结果一到
  // 就更新看板，冷启动的等待时间从「各步耗时之和」变成「最慢的一步」。
  // 任务状态两步保持原有先后顺序，避免快照与原生运行时会话互相覆盖。
  const restoreTaskState = async () => {
    await runStartupStep("android_runtime.restore", restoreAndroidRuntimeState);
    await runStartupStep("task_history.restore", restoreLatestTaskState);
  };
  await Promise.all([
    runStartupStep("android_battery.refresh", refreshAndroidBatteryStatus),
    runStartupStep("android_notification.refresh", refreshAndroidNotificationStatus),
    runStartupStep("android_keep_alive.refresh", refreshAndroidKeepAliveStatus),
    restoreTaskState(),
    runStartupStep("colo_dictionary.refresh", refreshColoDictionaryStatus),
    runStartupStep("scheduler.refresh", refreshSchedulerStatus),
  ]);
  finishStartupSync();
  // 启动同步完成后空闲预取其余视图片段，首次切换不必等下载与解析。
  const prefetchViews = () => {
    void Promise.all([resultsViewImport(), sourcesViewImport(), settingsViewImport(), dnsViewImport()]);
  };
  const requestIdle = window.requestIdleCallback?.bind(window);
  if (requestIdle) {
    requestIdle(prefetchViews, { timeout: 3000 });
  } else {
    globalThis.setTimeout(prefetchViews, 1200);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", scheduleViewportSizeRefresh);
  window.removeEventListener("beforeunload", handleBeforeUnload);
  window.removeEventListener("pagehide", flushPendingSettingsSave);
  document.removeEventListener("visibilitychange", flushPendingSettingsSave);
  window.removeEventListener("focus", handleAndroidNotificationWindowFocus);
  window.removeEventListener("wails:runtime-config-ready", handleWailsRuntimeReady);
  document.removeEventListener("visibilitychange", handleAndroidNotificationVisibilityChange);
  detachThemeMediaListener();
  stopViewportResizeTimer();
  if (saveStateTimer !== undefined) {
    window.clearTimeout(saveStateTimer);
  }
  if (draftSaveTimer !== undefined) {
    window.clearTimeout(draftSaveTimer);
  }
  stopThemeRefreshTimer();
  if (dashboardCacheTimer !== undefined) {
    window.clearTimeout(dashboardCacheTimer);
  }
  stopDownloadSpeedRowPruneTimer();
  uninstallAndroidViewportTracking();
  void flushDraftSave();
  removeProbeListener?.();
  removeDroppedSourcesListener?.();
});
</script>

<template>
  <DesktopShell v-if="desktopLayout" :current-version="appInfo.current_version" :route-title="routeTitles[selectedView]" :save-state="saveState" :selected-view="selectedView" :views="views" @change-view="changeSingleView">
    <DashboardView
      v-if="selectedView === 'dashboard'"
      :activity-feed="activityFeed"
      :can-cancel-task="canCancelTask"
      :can-pause-task="canPauseTask"
      :can-resume-task="canResumeTask"
      :can-start-task="canStartTask"
      :download-speed-concurrency="activeDownloadConcurrency"
      :download-speed-enabled="activeDownloadEnabled"
      :download-speed-rows="downloadSpeedRows"
      :download-speed-state="downloadSpeedState"
      :export-history="exportHistory"
      :format-timestamp="formatAppTimestamp"
      :has-active-task="hasActiveTask"
      :startup-timed-out="startupTimedOut"
      :loading="loading"
      :mcis-progress="mcisProgress"
      :no-sources="noSources"
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
      @cancel="cancelProbe"
      @open-history-target="openHistoryTarget"
      @pause="pauseProbe"
      @resume="continueProbe"
      @start="launchProbe"
      @go-sources="navigateTo('sources')"
      @retry-startup="retryStartupSync"
    />

    <ResultsView
      v-else-if="selectedView === 'results'"
      :can-rerun-task="canStartTask"
      :has-active-task="hasActiveTask"
      :loading="loading"
      :format-timestamp="formatAppTimestamp"
      platform="desktop"
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
      :dns-push-targets="dnsPushTargets"
      :cloudflare-routing-active="cloudflareRoutingPushActive"
      :cloudflare-routing-rule-count="activeCloudflareRoutingRuleCount"
      :summary="summary"
      :task="task"
      :task-snapshot="taskSnapshot"
      @copy-address="copyAddress"
      @export-current-results-csv="exportCurrentResultsCSV"
      @go-sources="navigateTo('sources')"
      @export-github="exportCurrentResultsToGitHub"
      @push-cloudflare="pushCurrentResultsToCloudflare"
      @load-more-results="loadMoreResults"
      @refresh-results="refreshCurrentTaskData"
      @rerun-address="rerunSingleAddress"
      @update-cloudflare-push-settings="updateResultCloudflarePushSettings"
      @update-github-top-n="updateResultGitHubTopN"
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
      :preview-states="sourcePreviewStates"
      :request-states="sourceRequestStates"
      :source-profiles="sourceProfiles"
      :group-ip-mode="sourceGroupIPMode"
      :group-colo-filter="sourceGroupColoFilter"
      :group-colo-filter-mode="sourceGroupColoFilterMode"
      :preview-errors="sourcePreviewErrors"
      @group-mode-change="setSourceGroupIPMode"
      @group-colo-filter-change="setSourceGroupColoFilter"
      @group-colo-filter-mode-change="setSourceGroupColoFilterMode"
      @copy-current-source-profile="copyCurrentSourceProfile"
      :sources="sources"
      :task-stage="task.stage"
      @add="addSource"
      @auto-save="autoSaveSourcePage('interaction')"
      @delete-source-profile="removeSourceProfile"
      @detect-source-name="applyDetectedSourceName"
      @preview="inspectSource($event)"
      @preview-request="inspectSource($event)"
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
      :colo-dictionary-processing="coloDictionaryProcessing"
      :colo-dictionary-status="coloDictionaryStatus"
      :colo-dictionary-updating="coloDictionaryUpdating"
      :android-battery-status="androidBatteryStatus"
      :android-keep-alive-status="androidKeepAliveStatus"
      :android-notification-status="androidNotificationStatus"
      :format-timestamp="formatAppTimestamp"
      :masked-token-hint="maskedTokenHint"
      :utc-offset-label="utcOffsetLabel()"
      platform="desktop"
      :settings="settings"
      :dns-subdomain-errors="dnsSubdomainErrors"
      :dns-subdomains="dnsSubdomainsByProvider"
      :dns-subdomains-loading="dnsSubdomainsLoading"
      :show-token="showToken"
      :github-testing="githubTesting"
      :telegram-testing="telegramTesting"
      :doh-testing="dohTesting"
      :doh-test-result="dohTestResult"
      :scheduler-status="schedulerStatus"
      :storage="storageStatus"
      :update-state="updateState"
      :viewport-adaptive-active="viewportAdaptiveActive"
      :viewport-presets="VIEWPORT_PRESETS"
      :viewport-runtime-supported="viewportRuntimeSupported"
      :viewport-size="viewportSize"
      :viewport-switching="viewportSwitching"
      @auto-save="autoSaveSettings('interaction')"
      @load-dns-subdomains="loadDnsProviderSubdomains"
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
      @process-colo-dictionary="processLocalColoDictionary"
      @refresh-colo-dictionary="refreshColoDictionary"
      @open-storage-dir="openStorageDirectory"
      @open-release-page="openOnlineReleasePage"
      @scheduler-daily-times-blur="handleSchedulerDailyTimesBlur"
      @select-export-target="selectExportTarget"
      @restore-config-webdav="restoreFromWebDAV"
      @install-update="installOnlineUpdate"
      @test-github-export="testGitHubExportSettings"
      @test-telegram-notification="testTelegramNotificationSettings"
      @test-doh="testDohConnection"
      @test-webdav="testWebDAVSettings"
      @toggle-token="showToken = !showToken"
    />

    <DnsView
      v-else
      :dns-records="dnsRecords"
      :read-providers="dnsReadProviders"
      :is-loading-dns="isLoadingDns"
      platform="desktop"
      :self-test-providers="dnsSelfTestProviders"
      :self-test-result="dnsSelfTestResult"
      :self-test-running="dnsSelfTesting"
      v-model:dns-read-name="dnsReadName"
      v-model:dns-read-provider-id="dnsReadProviderId"
      v-model:dns-read-scope="dnsReadScope"
      v-model:dns-record-type="dnsRecordType"
      @fetch="fetchDnsRecords"
      @go-settings="navigateTo('settings')"
      @run-selftest="runDNSSelfTest"
    />
  </DesktopShell>

  <MobileShell v-else :route-title="routeTitles[selectedView]" :save-state="saveState" :selected-view="selectedView" :views="views" @change-view="changeSingleView">
    <DashboardView
      v-if="selectedView === 'dashboard'"
      :activity-feed="activityFeed"
      :can-cancel-task="canCancelTask"
      :can-pause-task="canPauseTask"
      :can-resume-task="canResumeTask"
      :can-start-task="canStartTask"
      :download-speed-concurrency="activeDownloadConcurrency"
      :download-speed-enabled="activeDownloadEnabled"
      :download-speed-rows="downloadSpeedRows"
      :download-speed-state="downloadSpeedState"
      :export-history="exportHistory"
      :format-timestamp="formatAppTimestamp"
      :has-active-task="hasActiveTask"
      :startup-timed-out="startupTimedOut"
      :loading="loading"
      :mcis-progress="mcisProgress"
      :no-sources="noSources"
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
      @cancel="cancelProbe"
      @open-history-target="openHistoryTarget"
      @pause="pauseProbe"
      @resume="continueProbe"
      @start="launchProbe"
      @go-sources="navigateTo('sources')"
      @retry-startup="retryStartupSync"
    />

    <ResultsView
      v-else-if="selectedView === 'results'"
      :can-rerun-task="canStartTask"
      :has-active-task="hasActiveTask"
      :loading="loading"
      :format-timestamp="formatAppTimestamp"
      platform="mobile"
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
      :dns-push-targets="dnsPushTargets"
      :cloudflare-routing-active="cloudflareRoutingPushActive"
      :cloudflare-routing-rule-count="activeCloudflareRoutingRuleCount"
      :summary="summary"
      :task="task"
      :task-snapshot="taskSnapshot"
      @copy-address="copyAddress"
      @export-current-results-csv="exportCurrentResultsCSV"
      @go-sources="navigateTo('sources')"
      @export-github="exportCurrentResultsToGitHub"
      @push-cloudflare="pushCurrentResultsToCloudflare"
      @load-more-results="loadMoreResults"
      @refresh-results="refreshCurrentTaskData"
      @rerun-address="rerunSingleAddress"
      @update-cloudflare-push-settings="updateResultCloudflarePushSettings"
      @update-github-top-n="updateResultGitHubTopN"
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
      :preview-states="sourcePreviewStates"
      :request-states="sourceRequestStates"
      :source-profiles="sourceProfiles"
      :group-ip-mode="sourceGroupIPMode"
      :group-colo-filter="sourceGroupColoFilter"
      :group-colo-filter-mode="sourceGroupColoFilterMode"
      :preview-errors="sourcePreviewErrors"
      @group-mode-change="setSourceGroupIPMode"
      @group-colo-filter-change="setSourceGroupColoFilter"
      @group-colo-filter-mode-change="setSourceGroupColoFilterMode"
      @copy-current-source-profile="copyCurrentSourceProfile"
      :sources="sources"
      :task-stage="task.stage"
      @add="addSource"
      @auto-save="autoSaveSourcePage('interaction')"
      @delete-source-profile="removeSourceProfile"
      @detect-source-name="applyDetectedSourceName"
      @preview="inspectSource($event)"
      @preview-request="inspectSource($event)"
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
      :colo-dictionary-processing="coloDictionaryProcessing"
      :colo-dictionary-status="coloDictionaryStatus"
      :colo-dictionary-updating="coloDictionaryUpdating"
      :android-battery-status="androidBatteryStatus"
      :android-keep-alive-status="androidKeepAliveStatus"
      :android-notification-status="androidNotificationStatus"
      :format-timestamp="formatAppTimestamp"
      :masked-token-hint="maskedTokenHint"
      :utc-offset-label="utcOffsetLabel()"
      platform="mobile"
      :settings="settings"
      :dns-subdomain-errors="dnsSubdomainErrors"
      :dns-subdomains="dnsSubdomainsByProvider"
      :dns-subdomains-loading="dnsSubdomainsLoading"
      :show-token="showToken"
      :github-testing="githubTesting"
      :telegram-testing="telegramTesting"
      :doh-testing="dohTesting"
      :doh-test-result="dohTestResult"
      :scheduler-status="schedulerStatus"
      :storage="storageStatus"
      :update-state="updateState"
      :viewport-adaptive-active="viewportAdaptiveActive"
      :viewport-presets="VIEWPORT_PRESETS"
      :viewport-runtime-supported="viewportRuntimeSupported"
      :viewport-size="viewportSize"
      :viewport-switching="viewportSwitching"
      @auto-save="autoSaveSettings('interaction')"
      @load-dns-subdomains="loadDnsProviderSubdomains"
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
      @process-colo-dictionary="processLocalColoDictionary"
      @refresh-colo-dictionary="refreshColoDictionary"
      @open-storage-dir="openStorageDirectory"
      @open-release-page="openOnlineReleasePage"
      @scheduler-daily-times-blur="handleSchedulerDailyTimesBlur"
      @select-export-target="selectExportTarget"
      @restore-config-webdav="restoreFromWebDAV"
      @install-update="installOnlineUpdate"
      @test-github-export="testGitHubExportSettings"
      @test-telegram-notification="testTelegramNotificationSettings"
      @test-doh="testDohConnection"
      @test-webdav="testWebDAVSettings"
      @toggle-token="showToken = !showToken"
    />

    <DnsView
      v-else
      :dns-records="dnsRecords"
      :read-providers="dnsReadProviders"
      :is-loading-dns="isLoadingDns"
      platform="mobile"
      :self-test-providers="dnsSelfTestProviders"
      :self-test-result="dnsSelfTestResult"
      :self-test-running="dnsSelfTesting"
      v-model:dns-read-name="dnsReadName"
      v-model:dns-read-provider-id="dnsReadProviderId"
      v-model:dns-read-scope="dnsReadScope"
      v-model:dns-record-type="dnsRecordType"
      @fetch="fetchDnsRecords"
      @go-settings="navigateTo('settings')"
      @run-selftest="runDNSSelfTest"
    />
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
