import { onBeforeUnmount, onMounted, ref, watch, type Ref } from "vue";
import {
  getAndroidRuntimeStatus,
  listenToProbeEvents,
  listTaskSnapshots,
  loadConfig,
  normalizeConfigSnapshot,
  rebindProbeEventListener,
  type AppInfo,
  type ProbeEventEnvelope,
  type TaskSnapshot,
  type TaskTone,
} from "../lib/bridge";
import { EventsOn, isWailsDesktopHost } from "../lib/wailsRuntime";
import { readDashboardCache, writeDashboardCache } from "../lib/dashboardCache";
import { ACTIVITY_FEED_LIMIT } from "../lib/activityFeed";
import { asRecord, asString } from "../lib/valueCoerce";
import type { ToastTone, ViewName } from "../lib/appTypes";
import type { useActivityFeed } from "./useActivityFeed";
import type { useAndroidRuntime } from "./useAndroidRuntime";
import type { useAndroidSelectPicker } from "./useAndroidSelectPicker";
import type { useAppUpdate } from "./useAppUpdate";
import type { useColoDictionary } from "./useColoDictionary";
import type { useConfigPersistence } from "./useConfigPersistence";
import type { useDownloadSpeed } from "./useDownloadSpeed";
import type { useProbeTask } from "./useProbeTask";
import type { useResultExport } from "./useResultExport";
import type { useScheduler } from "./useScheduler";
import type { useSourceProfiles } from "./useSourceProfiles";
import type { useSources } from "./useSources";
import type { useStorageHealth } from "./useStorageHealth";
import type { useTheme } from "./useTheme";
import type { useViewport } from "./useViewport";

type ProbeTaskState = ReturnType<typeof useProbeTask>;
type ConfigPersistenceState = ReturnType<typeof useConfigPersistence>;
type AndroidRuntimeState = ReturnType<typeof useAndroidRuntime>;
type ViewportState = ReturnType<typeof useViewport>;
type ThemeState = ReturnType<typeof useTheme>;

export interface StartupSyncDeps {
  loading: Ref<boolean>;
  probeWarnings: Ref<string[]>;
  configPath: Ref<string>;
  appInfo: Ref<AppInfo>;
  status: ProbeTaskState["status"];
  summary: ProbeTaskState["summary"];
  task: ProbeTaskState["task"];
  taskSnapshot: ProbeTaskState["taskSnapshot"];
  startupSyncing: ProbeTaskState["startupSyncing"];
  taskSessionState: ProbeTaskState["taskSessionState"];
  hasActiveTask: ProbeTaskState["hasActiveTask"];
  finishTaskAction: ProbeTaskState["finishTaskAction"];
  activityFeed: ReturnType<typeof useActivityFeed>["activityFeed"];
  pushActivity: ReturnType<typeof useActivityFeed>["pushActivity"];
  showToast: (message: string, tone?: ToastTone) => void;
  navigateTo: (view: ViewName) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  appendLog: (event: string, payload: unknown) => void;
  commandDiagnosticPayload: (result: { code?: string; data?: unknown; message?: string; warnings?: string[] }) => unknown;
  applyTaskSnapshot: (snapshot: TaskSnapshot) => void;
  refreshTaskData: (taskId?: string) => Promise<void>;
  reconcileTaskData: (taskId?: string, options?: { switchToResultsOnData?: boolean }) => Promise<void>;
  summarizeFailureSummary: (summaryValue: unknown) => string;
  applyProbeEvent: (event: ProbeEventEnvelope) => void;
  handleBeforeUnload: (event: BeforeUnloadEvent) => void;
  prefetchViews: () => void;
  refreshAppInfo: ReturnType<typeof useAppUpdate>["refreshAppInfo"];
  applyStorageStatus: ReturnType<typeof useStorageHealth>["applyStorageStatus"];
  statusDetailWithStorage: ReturnType<typeof useStorageHealth>["statusDetailWithStorage"];
  storageStatus: ReturnType<typeof useStorageHealth>["storageStatus"];
  applyConfigSnapshot: ConfigPersistenceState["applyConfigSnapshot"];
  maybeRestoreDraft: ConfigPersistenceState["maybeRestoreDraft"];
  markConfigHydrated: ConfigPersistenceState["markConfigHydrated"];
  setLastSavedSnapshotSignature: ConfigPersistenceState["setLastSavedSnapshotSignature"];
  currentSnapshotSignature: ConfigPersistenceState["currentSnapshotSignature"];
  flushPendingSettingsSave: ConfigPersistenceState["flushPendingSettingsSave"];
  cancelPersistenceTimers: ConfigPersistenceState["cancelPersistenceTimers"];
  flushDraftSave: ConfigPersistenceState["flushDraftSave"];
  applySourceProfileStore: ReturnType<typeof useSourceProfiles>["applySourceProfileStore"];
  markSourceSaveBaselines: ReturnType<typeof useSources>["markSourceSaveBaselines"];
  refreshDroppedSources: ReturnType<typeof useSources>["refreshDroppedSources"];
  updateHistory: ReturnType<typeof useResultExport>["updateHistory"];
  hydrateResultPushSettings: ReturnType<typeof useResultExport>["hydrateResultPushSettings"];
  androidBatteryStatus: AndroidRuntimeState["androidBatteryStatus"];
  androidKeepAliveStatus: AndroidRuntimeState["androidKeepAliveStatus"];
  refreshAndroidBatteryStatus: AndroidRuntimeState["refreshAndroidBatteryStatus"];
  refreshAndroidNotificationStatus: AndroidRuntimeState["refreshAndroidNotificationStatus"];
  refreshAndroidKeepAliveStatus: AndroidRuntimeState["refreshAndroidKeepAliveStatus"];
  handleAndroidNotificationWindowFocus: AndroidRuntimeState["handleAndroidNotificationWindowFocus"];
  handleAndroidNotificationVisibilityChange: AndroidRuntimeState["handleAndroidNotificationVisibilityChange"];
  refreshColoDictionaryStatus: ReturnType<typeof useColoDictionary>["refreshColoDictionaryStatus"];
  refreshSchedulerStatus: ReturnType<typeof useScheduler>["refreshSchedulerStatus"];
  ensureAdaptiveViewportOnStartup: ViewportState["ensureAdaptiveViewportOnStartup"];
  scheduleViewportSizeRefresh: ViewportState["scheduleViewportSizeRefresh"];
  stopViewportResizeTimer: ViewportState["stopViewportResizeTimer"];
  uninstallAndroidViewportTracking: ReturnType<typeof useAndroidSelectPicker>["uninstallAndroidViewportTracking"];
  installThemeTracking: ThemeState["installThemeTracking"];
  detachThemeMediaListener: ThemeState["detachThemeMediaListener"];
  stopThemeRefreshTimer: ThemeState["stopThemeRefreshTimer"];
  stopDownloadSpeedRowPruneTimer: ReturnType<typeof useDownloadSpeed>["stopDownloadSpeedRowPruneTimer"];
}

export function useStartupSync(deps: StartupSyncDeps) {
  const {
    loading, probeWarnings, configPath, appInfo, status, summary, task, taskSnapshot, startupSyncing,
    taskSessionState, hasActiveTask, finishTaskAction, activityFeed, pushActivity, showToast, navigateTo,
    setStatus, appendLog, commandDiagnosticPayload, applyTaskSnapshot, refreshTaskData, reconcileTaskData,
    summarizeFailureSummary, applyProbeEvent, handleBeforeUnload, prefetchViews, refreshAppInfo,
    applyStorageStatus, statusDetailWithStorage, storageStatus, applyConfigSnapshot, maybeRestoreDraft,
    markConfigHydrated, setLastSavedSnapshotSignature, currentSnapshotSignature, flushPendingSettingsSave,
    cancelPersistenceTimers, flushDraftSave, applySourceProfileStore, markSourceSaveBaselines,
    refreshDroppedSources, updateHistory, hydrateResultPushSettings, androidBatteryStatus,
    androidKeepAliveStatus, refreshAndroidBatteryStatus, refreshAndroidNotificationStatus,
    refreshAndroidKeepAliveStatus, handleAndroidNotificationWindowFocus, handleAndroidNotificationVisibilityChange,
    refreshColoDictionaryStatus, refreshSchedulerStatus, ensureAdaptiveViewportOnStartup,
    scheduleViewportSizeRefresh, stopViewportResizeTimer, uninstallAndroidViewportTracking,
    installThemeTracking, detachThemeMediaListener, stopThemeRefreshTimer, stopDownloadSpeedRowPruneTimer,
  } = deps;
  let removeProbeListener: (() => void) | null = null;
  let removeDroppedSourcesListener: (() => void) | null = null;

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
  return {
    startupTimedOut,
    retryStartupSync,
    refreshConfig,
    scheduleDashboardCacheSave,
    finishStartupSync,
  };
}
