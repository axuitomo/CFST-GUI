<script setup vapor lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { EventsOn, isWailsDesktopHost } from "./lib/wailsRuntime";

import { applyProbeEventFrom, type ProbeEventApplyDeps } from "./lib/probeEventApply";
import {
  getAndroidRuntimeStatus,
  getTaskSnapshot,
  listenToProbeEvents,
  listTaskSnapshots,
  listTaskResults,
  loadConfig,
  normalizeConfigSnapshot,
  rebindProbeEventListener,
  showAndroidNotification,
  requestNotificationPermission,
  summarizeTraceDiagnostics,
  type ProbeEventEnvelope,
  type TaskSnapshot,
  type TaskTone,
} from "./lib/bridge";
import { readDashboardCache, writeDashboardCache } from "./lib/dashboardCache";
import { ACTIVITY_FEED_LIMIT } from "./lib/activityFeed";
import { formatUTCOffsetLabel, formatTimestampWithUTCOffset } from "./lib/time";
import { shouldRefreshProgress } from "./lib/mobileProgressRefresh";
import {  } from "./lib/labels";
import { asCount, asRecord, asString } from "./lib/valueCoerce";

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
import { useStorageHealth } from "./composables/useStorageHealth";
import { useColoDictionary } from "./composables/useColoDictionary";
import { useScheduler } from "./composables/useScheduler";
import { useAppUpdate } from "./composables/useAppUpdate";
import { useFileExport } from "./composables/useFileExport";
import { useWebDAV } from "./composables/useWebDAV";
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
const {
  activeTaskSessionState,
  canCancelTask,
  canPauseTask,
  canResumeTask,
  canStartTask,
  dashboardStatusLabel,
  finishTaskAction,
  hasActiveTask,
  mcisProgress,
  progressPercent,
  status,
  summary,
  task,
  taskSessionState,
  taskSnapshot,
  startupSyncing,
  launchProbe,
  rerunSingleAddress,
  copyAddress,
  pauseProbe,
  cancelProbe,
  continueProbe,
  updateMCISProgress,
} = useProbeTask({
  loading,
  probeWarnings,
  pushActivity,
  showToast: (message, tone) => showToast(message, tone),
  navigateTo: (view) => navigateTo(view),
  setStatus,
  appendLog,
  pushProcessTrace,
  pushWarningTrace,
  clearProcessTrace,
  buildConfigSnapshot,
  reconcileTaskData,
  resetDownloadSpeedState: () => resetDownloadSpeedState(),
  captureDownloadSpeedTaskConfig: (config) => captureDownloadSpeedTaskConfig(config),
  clearCurrentTaskResultWorkspace: (options) => clearCurrentTaskResultWorkspace(options),
  clearResultRows: () => { resultRows.value = []; },
  preparedSources: () => preparedSources.value,
  sourcePayloads: () => sourcePayloads.value,
  applySourceStatuses: (value) => applySourceStatuses(value),
});

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
const { showToast, toasts } = useToasts();
const appUpdateApi = useAppUpdate({ appendLog, showToast });
const { appInfo, isAndroidApp, updateState, refreshAppInfo, checkOnlineUpdate, installOnlineUpdate, openOnlineReleasePage } = appUpdateApi;

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
const resultExportApi = useResultExport({ settings, task, isAndroidApp: () => isAndroidApp.value, enabledDNSProviders, primaryCloudflareProvider, dnsProviderTargetLabel, buildConfigSnapshot, currentVisibleResultRows, pathLeafName: (rawPath) => fileExportApi.pathLeafName(rawPath), probeWarnings, pushWarningTrace, navigateTo, appendLog, showToast, setStatus, pushActivity });
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
const storageHealthApi = useStorageHealth({ appendLog, showToast, setStatus, commandDiagnosticPayload });
const { storageStatus, applyStorageStatus, statusDetailWithStorage, checkCurrentStorageHealth, openStorageDirectory } = storageHealthApi;
const coloDictionaryApi = useColoDictionary({ appendLog, showToast, setStatus });
const { coloDictionaryStatus, coloDictionaryProcessing, coloDictionaryUpdating, refreshColoDictionaryStatus, refreshColoDictionary, processLocalColoDictionary } = coloDictionaryApi;
const schedulerApi = useScheduler({ settings, appendLog, showToast, pushActivity, flushDraftSave: () => configPersistenceApi.flushDraftSave(), persistCurrentConfig: (options) => configPersistenceApi.persistCurrentConfig(options) });
const { schedulerStatus, refreshSchedulerStatus, handleSchedulerDailyTimesBlur } = schedulerApi;
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
const { applyConfigSnapshot, autoSaveSettings, cancelPersistenceTimers, currentSnapshotSignature, flushDraftSave, flushPendingSettingsSave, maybeRestoreDraft, saveState, scheduleDraftSave, markConfigHydrated, setLastSavedSnapshotSignature, lastSavedSnapshotSignature, lastDraftSnapshotSignature } = configPersistenceApi;
const fileExportApi = useFileExport({ settings, sources, configPath, storageStatus, isAndroidApp: () => isAndroidApp.value, buildConfigSnapshot, applyConfigSnapshot, applySourceProfileStore, markSourceSaveBaselines, applyStorageStatus, applyNavigation, appendLog, showToast, setStatus });
const { selectSourceFile, selectExportTarget, importConfigFromFile, exportCurrentDebugLog, exportCurrentDiagnosticPackage, openCurrentLogDirectory, exportConfigToFile } = fileExportApi;
const webdavApi = useWebDAV({ settings, buildConfigSnapshot, applyImportedConfigData: fileExportApi.applyImportedConfigData, applyNavigation, appendLog, showToast });
const { testWebDAVSettings, backupToWebDAV, restoreFromWebDAV } = webdavApi;

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

function eventDebugLogDisplayPath(payload: Record<string, unknown>) {
  return asString(payload.debug_log_path || payload.debugLogPath).trim();
}

function eventDebugLogOpenTarget(payload: Record<string, unknown>) {
  return asString(payload.log_uri || payload.logUri || payload.debug_log_path || payload.debugLogPath).trim();
}

function eventTraceFailureSummary(payload: Record<string, unknown>) {
  return summarizeTraceDiagnostics(payload.trace_diagnostics || payload.traceDiagnostics);
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

// 设置页的点击/失焦会连发，逐次保存要重建整份配置快照并稳定化；合并到一次。

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
