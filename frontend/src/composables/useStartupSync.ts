// 启动同步编排（重构计划 §4.2）：兜底计时、生命周期挂载与步骤串联；
// 配置读取、任务恢复与看板缓存分别见 lib/startup/*。
import { onBeforeUnmount, onMounted, ref } from "vue";
import { listenToProbeEvents, rebindProbeEventListener } from "../lib/bridge";
import { EventsOn, isWailsDesktopHost } from "../lib/wailsRuntime";
import { createDashboardCacheSync } from "../lib/startup/dashboardCacheSync";
import { refreshConfig as refreshConfigImpl } from "../lib/startup/configRefresh";
import { restoreAndroidRuntimeState, restoreLatestTaskState } from "../lib/startup/taskRestore";
import type { StartupSyncDeps } from "./startupSyncDeps";

export type { StartupSyncDeps } from "./startupSyncDeps";

export function useStartupSync(deps: StartupSyncDeps) {
  const {
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
    prefetchViews,
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
  } = deps;
  let removeProbeListener: (() => void) | null = null;
  let removeDroppedSourcesListener: (() => void) | null = null;

  const dashboardCache = createDashboardCacheSync({ activityFeed, status, startupSyncing, summary, task, taskSnapshot });
  const { restoreDashboardCache, scheduleDashboardCacheSave, disposeDashboardCacheTimer } = dashboardCache;

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

  restoreDashboardCache();

  async function refreshConfig() {
    await refreshConfigImpl({
      appendLog,
      applyConfigSnapshot,
      applySourceProfileStore,
      applyStorageStatus,
      commandDiagnosticPayload,
      configPath,
      currentSnapshotSignature,
      loading,
      markConfigHydrated,
      markSourceSaveBaselines,
      maybeRestoreDraft,
      probeWarnings,
      pushActivity,
      refreshDroppedSources,
      setLastSavedSnapshotSignature,
      setStatus,
      showToast,
      startupTimedOut,
      statusDetailWithStorage,
      storageStatus,
    });
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

  async function restoreAndroidRuntime() {
    await restoreAndroidRuntimeState({
      androidBatteryStatus,
      androidKeepAliveStatus,
      appInfo,
      appendLog,
      applyTaskSnapshot,
      finishTaskAction,
      navigateTo,
      pushActivity,
      refreshTaskData,
      setStatus,
      task,
      taskSessionState,
    });
  }

  async function restoreTaskHistory() {
    await restoreLatestTaskState({
      appendLog,
      applyTaskSnapshot,
      finishTaskAction,
      hasActiveTask,
      navigateTo,
      pushActivity,
      reconcileTaskData,
      setStatus,
      summary,
      summarizeFailureSummary,
      task,
      taskSnapshot,
      updateHistory,
    });
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
      await runStartupStep("android_runtime.restore", restoreAndroidRuntime);
      await runStartupStep("task_history.restore", restoreTaskHistory);
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
    disposeDashboardCacheTimer();
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
