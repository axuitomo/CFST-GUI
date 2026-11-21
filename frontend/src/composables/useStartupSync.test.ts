import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { reactive, ref } from "vue";
import { useStartupSync, type StartupSyncDeps } from "./useStartupSync";

const { loadConfig, listenToProbeEvents, readDashboardCache, writeDashboardCache } = vi.hoisted(() => ({
  loadConfig: vi.fn(),
  listenToProbeEvents: vi.fn(async () => () => {}),
  readDashboardCache: vi.fn((): unknown => null),
  writeDashboardCache: vi.fn(),
}));

vi.mock("../lib/bridge", () => ({
  getAndroidRuntimeStatus: vi.fn(async () => ({ ok: false })),
  listTaskSnapshots: vi.fn(async () => ({ ok: true, data: { items: [] } })),
  listenToProbeEvents,
  loadConfig,
  normalizeConfigSnapshot: vi.fn((value: unknown) => value),
  rebindProbeEventListener: vi.fn(async () => {}),
}));
vi.mock("../lib/dashboardCache", () => ({ readDashboardCache, writeDashboardCache }));
vi.mock("../lib/wailsRuntime", () => ({ EventsOn: vi.fn(() => () => {}), isWailsDesktopHost: vi.fn(() => false) }));

function makeDeps(overrides: Partial<StartupSyncDeps> = {}) {
  return {
    loading: ref(false),
    probeWarnings: ref<string[]>([]),
    configPath: ref(""),
    appInfo: ref({ platform: "windows" }),
    status: reactive({ title: "", detail: "", tone: "idle" }),
    summary: reactive({ processed: 0, total: 0, succeeded: 0, failed: 0, exported: 0 }),
    task: reactive({ taskId: "", active: false, exportPath: "", stage: "" }),
    taskSnapshot: ref(null),
    startupSyncing: ref(false),
    taskSessionState: ref("idle"),
    hasActiveTask: ref(false),
    finishTaskAction: vi.fn(),
    activityFeed: ref([]),
    pushActivity: vi.fn(),
    showToast: vi.fn(),
    navigateTo: vi.fn(),
    setStatus: vi.fn(),
    appendLog: vi.fn(),
    commandDiagnosticPayload: vi.fn((result: unknown) => result),
    applyTaskSnapshot: vi.fn(),
    refreshTaskData: vi.fn(async () => {}),
    reconcileTaskData: vi.fn(async () => {}),
    summarizeFailureSummary: vi.fn(() => ""),
    applyProbeEvent: vi.fn(),
    handleBeforeUnload: vi.fn(),
    prefetchViews: vi.fn(),
    refreshAppInfo: vi.fn(async () => {}),
    applyStorageStatus: vi.fn(),
    statusDetailWithStorage: vi.fn((message: string) => message),
    storageStatus: ref(null),
    applyConfigSnapshot: vi.fn(),
    maybeRestoreDraft: vi.fn(async () => {}),
    markConfigHydrated: vi.fn(),
    setLastSavedSnapshotSignature: vi.fn(),
    currentSnapshotSignature: vi.fn(() => ""),
    flushPendingSettingsSave: vi.fn(),
    cancelPersistenceTimers: vi.fn(),
    flushDraftSave: vi.fn(async () => {}),
    applySourceProfileStore: vi.fn(),
    markSourceSaveBaselines: vi.fn(),
    refreshDroppedSources: vi.fn(async () => {}),
    updateHistory: vi.fn(),
    hydrateResultPushSettings: vi.fn(),
    androidBatteryStatus: ref(null),
    androidKeepAliveStatus: ref(null),
    refreshAndroidBatteryStatus: vi.fn(async () => {}),
    refreshAndroidNotificationStatus: vi.fn(async () => {}),
    refreshAndroidKeepAliveStatus: vi.fn(async () => {}),
    handleAndroidNotificationWindowFocus: vi.fn(),
    handleAndroidNotificationVisibilityChange: vi.fn(),
    refreshColoDictionaryStatus: vi.fn(async () => {}),
    refreshSchedulerStatus: vi.fn(async () => {}),
    ensureAdaptiveViewportOnStartup: vi.fn(async () => {}),
    scheduleViewportSizeRefresh: vi.fn(),
    stopViewportResizeTimer: vi.fn(),
    uninstallAndroidViewportTracking: vi.fn(),
    installThemeTracking: vi.fn(),
    detachThemeMediaListener: vi.fn(),
    stopThemeRefreshTimer: vi.fn(),
    stopDownloadSpeedRowPruneTimer: vi.fn(),
    ...overrides,
  } as unknown as StartupSyncDeps;
}

describe("useStartupSync", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    readDashboardCache.mockReturnValue(null);
    vi.stubGlobal("window", {
      setTimeout: (...args: Parameters<typeof setTimeout>) => setTimeout(...args),
      clearTimeout: (...args: Parameters<typeof clearTimeout>) => clearTimeout(...args),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("fills the first frame from the dashboard cache and keeps actions locked", () => {
    readDashboardCache.mockReturnValue({
      activityFeed: [],
      status: { title: "缓存标题", detail: "缓存详情", tone: "running" },
      summary: { processed: 3, total: 10, succeeded: 2, failed: 1, exported: 0 },
      taskSnapshot: { task_id: "task-cache", status: "running", session_state: "active_runtime", updated_at: "2026-08-06T00:00:00Z" },
      updatedAt: "2026-08-06T00:00:00Z",
    });
    const deps = makeDeps();
    useStartupSync(deps);
    expect(deps.status.title).toBe("缓存标题");
    expect(deps.summary.processed).toBe(3);
    expect(deps.task.taskId).toBe("task-cache");
    expect(deps.startupSyncing.value).toBe(true);
  });

  it("leaves the optimistic period untouched when no cache exists", () => {
    const deps = makeDeps();
    useStartupSync(deps);
    expect(deps.startupSyncing.value).toBe(false);
    expect(deps.status.title).toBe("");
  });

  it("ends the optimistic period and flushes the cache once after finishStartupSync", async () => {
    vi.useFakeTimers();
    const deps = makeDeps({ startupSyncing: ref(true) });
    const api = useStartupSync(deps);
    api.finishStartupSync();
    expect(deps.startupSyncing.value).toBe(false);
    expect(writeDashboardCache).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1000);
    expect(writeDashboardCache).toHaveBeenCalledTimes(1);
  });

  it("surfaces a config read failure without leaving the loading flag set", async () => {
    loadConfig.mockResolvedValueOnce({ ok: false, message: "读取配置失败。", warnings: [] });
    const deps = makeDeps();
    const api = useStartupSync(deps);
    await api.refreshConfig();
    expect(deps.showToast).toHaveBeenCalledWith("读取配置失败。", "error");
    expect(deps.loading.value).toBe(false);
    expect(deps.setStatus).toHaveBeenCalledWith(expect.objectContaining({ tone: "failed" }));
  });
});