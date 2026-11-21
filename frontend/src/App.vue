<script setup vapor lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { EventsOn, isWailsDesktopHost } from "./lib/wailsRuntime";

import { applyProbeEventFrom, type ProbeEventApplyDeps } from "./lib/probeEventApply";
import {
  backupConfigToWebDAV,
  checkForUpdates,
  checkStorageHealth,
  downloadAndInstallUpdate,
  exportConfigArchive,
  exportDebugLog,
  exportDiagnosticPackage,
  getAndroidRuntimeStatus,
  getTaskSnapshot,
  getAppInfo,
  importConfigArchive,
  listenToProbeEvents,
  listTaskSnapshots,
  listTaskResults,
  loadColoDictionaryStatus,
  loadConfig,
  loadSchedulerStatus,
  normalizeConfigSnapshot,
  openLogDirectory,
  openReleasePage,
  openPath,
  processColoDictionary,
  resumeProbe,
  restoreConfigFromWebDAV,
  rebindProbeEventListener,
  showAndroidNotification,
  requestNotificationPermission,
  selectPath,
  startProbe,
  stopProbe,
  summarizeTraceDiagnostics,
  testWebDAV,
  updateColoDictionary,
  type ColoDictionaryStatus,
  type AppInfo,
  type PathSelectionPayload,
  type ProbeEventEnvelope,
  type SourceIPMode,
  type StorageStatus,
  type SchedulerStatus,
  type TaskSnapshot,
  type TaskTone,
  type UpdateInfo,
} from "./lib/bridge";
import { readDashboardCache, writeDashboardCache } from "./lib/dashboardCache";
import { ACTIVITY_FEED_LIMIT } from "./lib/activityFeed";
import { formatUTCOffsetLabel, formatTimestampWithUTCOffset } from "./lib/time";
import { shouldRefreshProgress } from "./lib/mobileProgressRefresh";
import { taskActionLabel, type TaskActionKind } from "./lib/labels";
import { asCount, asRecord, asString, asBoolean, schedulerDailyTimesFromText } from "./lib/valueCoerce";

import { createDefaultSettingsForm } from "./lib/settingsForm";
import { type ProcessTraceEntry, type ViewName } from "./lib/appTypes";
import { useProbeTask } from "./composables/useProbeTask";
import { useActivityFeed } from "./composables/useActivityFeed";
import { useToasts } from "./composables/useToasts";
import { useTheme } from "./composables/useTheme";
import { useNavigation } from "./composables/useNavigation";
import { useViewport, VIEWPORT_PRESETS } from "./composables/useViewport";
import { useAndroidSelectPicker } from "./composables/useAndroidSelectPicker";
import { useDownloadSpeed } from "./composables/useDownloadSpeed";
import { useAndroidRuntime } from "./composables/useAndroidRuntime";
import { useSources } from "./composables/useSources";
import { useSourceProfiles } from "./composables/useSourceProfiles";
import { useSourceAutoSave } from "./composables/useSourceAutoSave";
import { useConfigPersistence } from "./composables/useConfigPersistence";
import { useDnsProviders } from "./composables/useDnsProviders";
import { dnsProviderSnapshotFromForm } from "./lib/dnsProviderForm";
import { useDnsRecords } from "./composables/useDnsRecords";
import { useDnsSelfTest } from "./composables/useDnsSelfTest";
import { useResults } from "./composables/useResults";
import { useResultExport } from "./composables/useResultExport";
import { buildConfigSnapshot as buildConfigSnapshotFrom } from "./lib/configSnapshotBuild";
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

const ACTIVE_PROBE_MESSAGE = "当前已有探测任务运行或暂停，请完成后再启动新任务。";

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

const selectedView = ref<ViewName>("dashboard");
const { activityFeed, pushActivity } = useActivityFeed();
const configPath = ref("");
const loading = ref(false);
// 只写入、不参与渲染：用普通有界数组，避免每个事件都触发响应式追踪。
const logs: Array<{ event: string; payload: unknown; ts: string }> = [];
const processTrace = ref<ProcessTraceEntry[]>([]);
const probeWarnings = ref<string[]>([]);

const showToken = ref(false);
// The two shells are CSS-hidden duplicates; mounting only the active one keeps
// every view (and its DOM) from existing twice.
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

const settings = reactive(createDefaultSettingsForm());
const { maskedTokenHint, applyDNSProvidersFromSnapshot, primaryCloudflareProvider, enabledDNSProviders } = useDnsProviders({ settings });
const { applyThemeMode, detachThemeMediaListener, installThemeTracking, stopThemeRefreshTimer } = useTheme(settings);
const { applyNavigation, changeSingleView, navigateTo } = useNavigation({
  autoSaveSettings: (reason) => configPersistenceApi.autoSaveSettings(reason),
  configHydrated: () => configPersistenceApi.configHydrated(),
  draftRestoring: () => configPersistenceApi.draftRestoring(),
  isAndroidApp: () => isAndroidApp.value,
  saveSourcePageBeforeLeave: () => saveSourcePageBeforeLeave(),
  selectedView,
  sourcePageHasUnsavedChanges: () => sourcePageHasUnsavedChanges(),
});
const dnsRecordsApi = useDnsRecords({ settings, buildConfigSnapshot, appendLog, showToast, setStatus, pushActivity, navigateTo, selectedView: () => selectedView.value });
const { dnsReadName, dnsReadProviderId, dnsReadScope, dnsRecordType, dnsRecords, dnsSubdomainErrors, dnsSubdomainsByProvider, dnsSubdomainsLoading, isLoadingDns, dnsReadProviders, dnsProviderTargetLabel, dnsProviderResolvedRecordName, fetchDnsRecords, loadDnsProviderSubdomains } = dnsRecordsApi;
const dnsSelfTestApi = useDnsSelfTest({ settings, buildConfigSnapshot, enabledDNSProviders, targetLabel: dnsProviderTargetLabel, resolvedRecordName: dnsProviderResolvedRecordName, zoneName: dnsRecordsApi.dnsProviderZoneName, probeWarnings, pushWarningTrace, appendLog, showToast, setStatus, pushActivity });
const { dnsSelfTesting, dnsSelfTestResult, dohTesting, dohTestResult, dnsSelfTestProviders, runDNSSelfTest, testDohConnection } = dnsSelfTestApi;
const resultsApi = useResults({ task, taskSnapshot, buildResultFileFallbackPayload, refreshTaskData, appendLog });
const { resultIpFilter, resultOrder, resultRows, taskResultCSVPath, resultWorkspaceTaskId, resultSortBy, resultsLoading, resultsPageLimit, resultsTotalCount, resultIpFilterOptions, resultSortOptions, clearCurrentTaskResultWorkspace, applyCurrentTaskResultWorkspace, updateResultSort, updateResultIpFilter, updateResultOrder, currentVisibleResultRows, refreshCurrentTaskData, loadMoreResults } = resultsApi;
const resultExportApi = useResultExport({ settings, task, isAndroidApp: () => isAndroidApp.value, enabledDNSProviders, primaryCloudflareProvider, dnsProviderTargetLabel, buildConfigSnapshot, currentVisibleResultRows, pathLeafName, probeWarnings, pushWarningTrace, navigateTo, appendLog, showToast, setStatus, pushActivity });
const { exportHistory, resultGitHubTopN, resultCloudflarePushSettings, csvExporting, cloudflarePushing, githubExporting, githubTesting, telegramTesting, dnsPushTargets, updateHistory, updateResultCloudflarePushSettings, updateResultGitHubTopN, testGitHubExportSettings, testTelegramNotificationSettings, exportCurrentResultsToGitHub, pushCurrentResultsToCloudflare, exportCurrentResultsCSV, openHistoryTarget, hydrateResultPushSettings, resultPushTargetProvider } = resultExportApi;
const { applyViewportPreset, desktopLayout, ensureAdaptiveViewportOnStartup, isViewportRuntimeSupported, scheduleViewportSizeRefresh, stopViewportResizeTimer, viewportAdaptiveActive, viewportSize, viewportSwitching } = useViewport({ appendLog, showToast });
const { androidSelectPicker, chooseAndroidSelectOption, closeAndroidSelectPicker, installAndroidViewportTracking, uninstallAndroidViewportTracking } = useAndroidSelectPicker({ isAndroidApp: () => isAndroidApp.value });
const { activeDownloadConcurrency, activeDownloadEnabled, activeDownloadSampleIntervalMs, captureDownloadSpeedTaskConfig, downloadSpeedRows, downloadSpeedState, ensureDownloadSpeedRowPruneTimer, isDownloadSpeedTaskConfigCaptured, resetDownloadSpeedState, stopDownloadSpeedRowPruneTimer } = useDownloadSpeed({ clearDownloadSpeedTraceEntries });
const { androidBatteryStatus, androidKeepAliveStatus, androidNotificationStatus, handleAndroidNotificationVisibilityChange, handleAndroidNotificationWindowFocus, openAndroidNotificationSettings, refreshAndroidBatteryStatus, refreshAndroidKeepAliveStatus, refreshAndroidNotificationStatus, requestAndroidNotificationPermission, requestBatteryOptimizationExemption, toggleAndroidKeepAlive } = useAndroidRuntime({ appendLog, isAndroidApp: () => isAndroidApp.value, showToast });
const sourcesApi = useSources({
  appendLog,
  showToast,
  autoDetectSourceName: () => settings.sourceAutoDetectName,
  buildConfigSnapshot,
  commandDiagnosticPayload,
  currentSourceProfileSignature: () => sourceProfilesApi.currentSourceProfileSignature(),
  activeSourceProfileSourceSignature: () => sourceProfilesApi.activeSourceProfileSourceSignature(),
});
const { sources, sourceGroupIPMode, sourceGroupColoFilter, sourceGroupColoFilterMode, sourcePreviewStates, sourcePreviewErrors, sourceRequestStates, sourcePayloads, preparedSources, noSources, addSource, setSourceGroupIPMode, setSourceGroupColoFilter, setSourceGroupColoFilterMode, removeSource, refreshDroppedSources, applyDetectedSourceName, applySourceStatuses, inspectSource, currentSourceSignature, markSourceSaveBaselines, sourcePageHasUnsavedChanges } = sourcesApi;
const sourceProfilesApi = useSourceProfiles({
  sources,
  sourceGroupIPMode,
  sourceGroupColoFilter,
  sourceGroupColoFilterMode,
  sourcePayloads: () => sourcePayloads.value,
  currentSourceSignature,
  sourcePageHasUnsavedChanges,
  autoSaveSourcePage: (reason) => sourceAutoSaveApi.autoSaveSourcePage(reason),
  selectedView: () => selectedView.value,
  configPath,
  appendLog,
  showToast,
  setLastSavedSourceProfileSignature: sourcesApi.setLastSavedSourceProfileSignature,
});
const { sourceProfiles, applySourceProfileStore, saveCurrentSourceProfile, copyCurrentSourceProfile, updateActiveSourceProfile, updateActiveSourceProfileQuietly, switchToSourceProfile, removeSourceProfile } = sourceProfilesApi;
const sourceAutoSaveApi = useSourceAutoSave({
  persistConfig: (options) => configPersistenceApi.persistConfig(options),
  currentSourceSignature,
  currentSourceProfileSignature: sourceProfilesApi.currentSourceProfileSignature,
  activeSourceProfileSourceSignature: sourceProfilesApi.activeSourceProfileSourceSignature,
  sourcePageHasUnsavedChanges,
  lastSavedSourceSignature: sourcesApi.lastSavedSourceSignature,
  lastSavedSourceProfileSignature: sourcesApi.lastSavedSourceProfileSignature,
  markSourceSaveBaselines,
  updateActiveSourceProfile,
  updateActiveSourceProfileQuietly,
  configHydrated: () => configPersistenceApi.configHydrated(),
  draftRestoring: () => configPersistenceApi.draftRestoring(),
  appendLog,
  showToast,
});
const { autoSaveSourcePage, saveSourcePageBeforeLeave } = sourceAutoSaveApi;
const configPersistenceApi = useConfigPersistence({
  settings,
  loading,
  probeWarnings,
  configPath,
  sources,
  sourceGroupIPMode,
  buildConfigSnapshot,
  currentSourceSignature,
  markSourceSaveBaselines,
  applySourceProfileStore,
  applyStorageStatus,
  refreshSchedulerStatus,
  applyDNSProvidersFromSnapshot,
  isTaskActive: () => task.active,
  hasActiveTask: () => hasActiveTask.value,
  isDownloadSpeedTaskConfigCaptured,
  captureDownloadSpeedTaskConfig,
  appendLog,
  showToast,
  pushActivity,
  setStatus,
  applyNavigation,
});
const { applyConfigSnapshot, autoSaveSettings, cancelPersistenceTimers, currentSnapshotSignature, flushDraftSave, flushPendingSettingsSave, maybeRestoreDraft, persistCurrentConfig, saveState, scheduleDraftSave, markConfigHydrated, setLastSavedSnapshotSignature, lastSavedSnapshotSignature, lastDraftSnapshotSignature } = configPersistenceApi;

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

function handleBeforeUnload(event: BeforeUnloadEvent) {
  void autoSaveSettings("beforeunload");
  void autoSaveSourcePage("beforeunload");
  void flushDraftSave();
  if (currentSnapshotSignature() !== lastSavedSnapshotSignature() && currentSnapshotSignature() !== lastDraftSnapshotSignature()) {
    event.preventDefault();
    event.returnValue = "";
  }
}

const viewportRuntimeSupported = computed(() => isViewportRuntimeSupported());

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

function allocateTaskId() {
  return `cfst-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
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

function buildConfigSnapshot() {
  const primaryCloudflare = primaryCloudflareProvider();
  return buildConfigSnapshotFrom({
    settings,
    dnsProviderSnapshots: settings.dnsProviders.map((entry) => dnsProviderSnapshotFromForm(entry)),
    primaryCloudflareSnapshot: primaryCloudflare ? dnsProviderSnapshotFromForm(primaryCloudflare) : null,
    sourceIPMode: sourceGroupIPMode.value,
    sources: sourcePayloads.value,
  });
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

const probeEventApplyDeps: ProbeEventApplyDeps = {
  task,
  taskSessionState,
  activeTaskSessionState,
  taskSnapshot,
  status,
  summary,
  mcisProgress,
  finishTaskAction,
  downloadSpeedState,
  downloadSpeedRows,
  activeDownloadConcurrency,
  activeDownloadSampleIntervalMs,
  resetDownloadSpeedState,
  ensureDownloadSpeedRowPruneTimer,
  stopDownloadSpeedRowPruneTimer,
  clearDownloadSpeedTraceEntries,
  updateDownloadSpeedTrace,
  applySourceStatuses,
  resultRows,
  taskResultCSVPath,
  probeWarnings,
  updateHistory,
  shouldRefreshProgressUI,
  updateMCISProgress,
  pushProcessTrace,
  pushWarningTrace,
  summarizeFailureSummary,
  edgeRejectedItems,
  notifyUploadResult,
  eventDebugLogDisplayPath,
  eventDebugLogOpenTarget,
  eventTraceFailureSummary,
  reconcileTaskData,
  isAndroidApp: () => isAndroidApp.value,
  appendLog,
  showToast,
  setStatus,
  pushActivity,
};

function applyProbeEvent(event: ProbeEventEnvelope) {
  applyProbeEventFrom(event, probeEventApplyDeps);
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
    setLastSavedSnapshotSignature(currentSnapshotSignature());
    markConfigHydrated();
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

// 设置页的点击/失焦会连发，逐次保存要重建整份配置快照并稳定化；合并到一次。

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
  hydrateResultPushSettings();
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
  cancelPersistenceTimers();
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
