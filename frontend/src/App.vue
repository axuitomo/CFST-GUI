<script setup vapor lang="ts">
import { computed, defineAsyncComponent, reactive, ref, watch } from "vue";

import { applyProbeEventFrom, type ProbeEventApplyDeps } from "./lib/probeEventApply";
import {
  showAndroidNotification,
  requestNotificationPermission,
  summarizeTraceDiagnostics,
  type ProbeEventEnvelope,
} from "./lib/bridge";
import { asString } from "./lib/valueCoerce";

import { createDefaultSettingsForm } from "./lib/settingsForm";
import { type ViewName } from "./lib/appTypes";
import { useAppTrace } from "./composables/useAppTrace";
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
import { useTaskSync } from "./composables/useTaskSync";
import { useResultExport } from "./composables/useResultExport";
import { useStorageHealth } from "./composables/useStorageHealth";
import { useColoDictionary } from "./composables/useColoDictionary";
import { useScheduler } from "./composables/useScheduler";
import { useAppUpdate } from "./composables/useAppUpdate";
import { useFileExport } from "./composables/useFileExport";
import { useWebDAV } from "./composables/useWebDAV";
import { useStartupSync } from "./composables/useStartupSync";
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
const probeWarnings = ref<string[]>([]);
const settings = reactive(createDefaultSettingsForm());
const {
  appendLog,
  clearDownloadSpeedTraceEntries,
  clearProcessTrace,
  commandDiagnosticPayload,
  formatAppTimestamp,
  processTrace,
  pushProcessTrace,
  pushWarningTrace,
  setStatus,
  shouldRefreshProgressUI,
  summarizeFailureSummary,
  updateDownloadSpeedTrace,
  utcOffsetLabel,
} = useAppTrace({
  settings,
  configPath,
  status: () => status,
  isAndroidApp: () => isAndroidApp.value,
});

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
  reconcileTaskData: (taskId, options) => taskSyncApi.reconcileTaskData(taskId, options),
  resetDownloadSpeedState: () => resetDownloadSpeedState(),
  captureDownloadSpeedTaskConfig: (config) => captureDownloadSpeedTaskConfig(config),
  clearCurrentTaskResultWorkspace: (options) => clearCurrentTaskResultWorkspace(options),
  clearResultRows: () => { resultRows.value = []; },
  preparedSources: () => preparedSources.value,
  sourcePayloads: () => sourcePayloads.value,
  applySourceStatuses: (value) => applySourceStatuses(value),
});

const { showToast, toasts } = useToasts();
const appUpdateApi = useAppUpdate({ appendLog, showToast });
const { appInfo, isAndroidApp, updateState, refreshAppInfo, checkOnlineUpdate, installOnlineUpdate, openOnlineReleasePage } = appUpdateApi;

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
const resultsApi = useResults({
  task,
  taskSnapshot,
  buildResultFileFallbackPayload: (exportPath, snapshot) => taskSyncApi.buildResultFileFallbackPayload(exportPath, snapshot),
  refreshTaskData: (taskId) => taskSyncApi.refreshTaskData(taskId),
  appendLog,
});
const { resultIpFilter, resultOrder, resultRows, taskResultCSVPath, resultWorkspaceTaskId, resultSortBy, resultsLoading, resultsPageLimit, resultsTotalCount, resultIpFilterOptions, resultSortOptions, clearCurrentTaskResultWorkspace, applyCurrentTaskResultWorkspace, updateResultSort, updateResultIpFilter, updateResultOrder, currentVisibleResultRows, refreshCurrentTaskData, loadMoreResults } = resultsApi;
const taskSyncApi = useTaskSync({
  task,
  taskSnapshot,
  taskSessionState,
  summary,
  updateMCISProgress,
  taskResultCSVPath,
  resultsLoading,
  resultWorkspaceTaskId,
  resultRows,
  resultSortBy,
  resultOrder,
  resultIpFilter,
  resultsPageLimit,
  clearCurrentTaskResultWorkspace,
  applyCurrentTaskResultWorkspace,
  navigateTo,
  appendLog,
});
const { applyTaskSnapshot, reconcileTaskData, refreshTaskData } = taskSyncApi;
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

// 设置页的点击/失焦会连发，逐次保存要重建整份配置快照并稳定化；合并到一次。

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

const { startupTimedOut, retryStartupSync } = useStartupSync({
  loading,
  probeWarnings,
  configPath,
  appInfo,
  status,
  summary,
  task,
  taskSnapshot,
  startupSyncing,
  taskSessionState,
  hasActiveTask,
  finishTaskAction,
  activityFeed,
  pushActivity,
  showToast,
  navigateTo,
  setStatus,
  appendLog,
  commandDiagnosticPayload,
  applyTaskSnapshot,
  refreshTaskData,
  reconcileTaskData,
  summarizeFailureSummary,
  applyProbeEvent,
  handleBeforeUnload,
  prefetchViews: () => {
    void Promise.all([resultsViewImport(), sourcesViewImport(), settingsViewImport(), dnsViewImport()]);
  },
  refreshAppInfo,
  applyStorageStatus,
  statusDetailWithStorage,
  storageStatus,
  applyConfigSnapshot,
  maybeRestoreDraft,
  markConfigHydrated,
  setLastSavedSnapshotSignature,
  currentSnapshotSignature,
  flushPendingSettingsSave,
  cancelPersistenceTimers,
  flushDraftSave,
  applySourceProfileStore,
  markSourceSaveBaselines,
  refreshDroppedSources,
  updateHistory,
  hydrateResultPushSettings,
  androidBatteryStatus,
  androidKeepAliveStatus,
  refreshAndroidBatteryStatus,
  refreshAndroidNotificationStatus,
  refreshAndroidKeepAliveStatus,
  handleAndroidNotificationWindowFocus,
  handleAndroidNotificationVisibilityChange,
  refreshColoDictionaryStatus,
  refreshSchedulerStatus,
  ensureAdaptiveViewportOnStartup,
  scheduleViewportSizeRefresh,
  stopViewportResizeTimer,
  uninstallAndroidViewportTracking,
  installThemeTracking,
  detachThemeMediaListener,
  stopThemeRefreshTimer,
  stopDownloadSpeedRowPruneTimer,
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

