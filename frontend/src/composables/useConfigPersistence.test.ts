// useConfigPersistence 特征测试（重构计划 §4.2）：
// skipIfUnchanged、脱敏 Token 阻断、成功/失败路径、草稿调度与恢复。
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { reactive, ref } from "vue";
import { useConfigPersistence, type ConfigPersistenceDeps } from "./useConfigPersistence";
import { createDefaultSettingsForm } from "../lib/settingsForm";
import type { SourceDraft } from "./useSources";
import type { SourceGroupIPMode } from "../lib/sourceGroups";

const { bridge } = vi.hoisted(() => ({
  bridge: {
    saveConfig: vi.fn(),
    saveDraft: vi.fn(),
    discardDraft: vi.fn(),
  },
}));
vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, ...bridge };
});

function makeDeps(overrides: Partial<ConfigPersistenceDeps> = {}) {
  const settings = reactive(createDefaultSettingsForm());
  const loading = ref(false);
  const probeWarnings = ref<string[]>([]);
  const configPath = ref("");
  const sources = ref<SourceDraft[]>([]);
  const sourceGroupIPMode = ref<SourceGroupIPMode>("traverse");
  const toasts: Array<{ message: string; tone?: string }> = [];
  const statuses: Array<{ title: string; tone: string }> = [];
  const navigations: string[] = [];
  const activities: string[] = [];
  const snapshot = { cloudflare: { comment: "" } };
  const deps: ConfigPersistenceDeps = {
    settings,
    loading,
    probeWarnings,
    configPath,
    sources,
    sourceGroupIPMode,
    buildConfigSnapshot: () => snapshot as never,
    currentSourceSignature: () => "src-sig",
    markSourceSaveBaselines: vi.fn(),
    applySourceProfileStore: vi.fn(),
    applyStorageStatus: vi.fn(),
    refreshSchedulerStatus: vi.fn(async () => undefined),
    applyDNSProvidersFromSnapshot: vi.fn(),
    isTaskActive: () => false,
    hasActiveTask: () => false,
    isDownloadSpeedTaskConfigCaptured: () => false,
    captureDownloadSpeedTaskConfig: vi.fn(),
    appendLog: vi.fn(),
    showToast: (message, tone) => toasts.push({ message, tone }),
    pushActivity: (title) => activities.push(title),
    setStatus: (next) => statuses.push({ title: next.title, tone: next.tone }),
    applyNavigation: (view) => navigations.push(view),
    ...overrides,
  };
  return { deps, settings, loading, probeWarnings, configPath, toasts, statuses, navigations, activities, snapshot };
}

beforeEach(() => {
  for (const fn of Object.values(bridge)) fn.mockReset();
  vi.useFakeTimers();
  vi.stubGlobal("window", {
    setTimeout: (fn: () => void, ms: number) => setTimeout(fn, ms),
    clearTimeout: (id: number) => clearTimeout(id),
    confirm: vi.fn(() => true),
  });
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("persistConfigNow", () => {
  it("skipIfUnchanged short-circuits the second identical save", async () => {
    bridge.saveConfig.mockResolvedValue({ ok: true, data: { config_snapshot: {} }, message: "" });
    const { deps } = makeDeps();
    const api = useConfigPersistence(deps);
    expect(await api.persistConfig({ skipIfUnchanged: true })).toBe(true);
    expect(await api.persistConfig({ skipIfUnchanged: true })).toBe(true);
    expect(bridge.saveConfig).toHaveBeenCalledTimes(1);
  });

  it("blocks masked token with toast + navigation unless silent", async () => {
    const { deps, toasts, navigations } = makeDeps();
    deps.settings.dnsProviders = [{ provider: "cloudflare", maskedToken: "***", apiToken: "  " } as never];
    const api = useConfigPersistence(deps);
    expect(await api.persistConfig({})).toBe(false);
    expect(bridge.saveConfig).not.toHaveBeenCalled();
    expect(toasts[toasts.length - 1]).toEqual({ message: "需要重新输入完整 Token", tone: "error" });
    expect(navigations).toContain("settings");
    expect(await api.persistConfig({ silentMaskedToken: true })).toBe(false);
    expect(toasts).toHaveLength(1);
  });

  it("success hydrates snapshot, records signature and warnings", async () => {
    bridge.saveConfig.mockResolvedValue({ ok: true, data: { config_snapshot: { cloudflare: { comment: "测试备注" } }, configPath: "/p.json" }, warnings: ["w1"], message: "" });
    const { deps, settings, configPath, probeWarnings } = makeDeps();
    const api = useConfigPersistence(deps);
    expect(await api.persistConfig({})).toBe(true);
    expect(settings.comment).toBe("测试备注");
    expect(configPath.value).toBe("/p.json");
    expect(probeWarnings.value).toEqual(["w1"]);
    expect(api.lastSavedSnapshotSignature()).toBe(api.currentSnapshotSignature());
    expect(api.saveState.value).toBe("saved");
    vi.advanceTimersByTime(4000);
    expect(api.saveState.value).toBe("idle");
  });

  it("failure surfaces status and keeps saveState error", async () => {
    bridge.saveConfig.mockResolvedValue({ ok: false, message: "磁盘已满" });
    const { deps, statuses, toasts } = makeDeps();
    const api = useConfigPersistence(deps);
    expect(await api.persistConfig({})).toBe(false);
    expect(statuses[statuses.length - 1]).toEqual({ title: "保存失败", tone: "failed" });
    expect(toasts[toasts.length - 1]).toEqual({ message: "保存配置失败", tone: "error" });
    expect(api.saveState.value).toBe("error");
  });

  it("silent failure suppresses user-facing feedback", async () => {
    bridge.saveConfig.mockResolvedValue({ ok: false, message: "x" });
    const { deps, statuses, toasts } = makeDeps();
    const api = useConfigPersistence(deps);
    expect(await api.persistConfig({ silentFailure: true })).toBe(false);
    expect(statuses).toHaveLength(0);
    expect(toasts).toHaveLength(0);
  });
});

describe("draft lifecycle", () => {
  it("scheduleDraftSave waits for hydration and debounces 500ms", async () => {
    bridge.saveDraft.mockResolvedValue({ ok: true });
    const { deps } = makeDeps();
    const api = useConfigPersistence(deps);
    api.scheduleDraftSave();
    vi.advanceTimersByTime(600);
    expect(bridge.saveDraft).not.toHaveBeenCalled();
    api.markConfigHydrated();
    api.scheduleDraftSave();
    api.scheduleDraftSave();
    vi.advanceTimersByTime(499);
    expect(bridge.saveDraft).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(bridge.saveDraft).toHaveBeenCalledTimes(1);
    expect(api.lastDraftSnapshotSignature()).toBe(api.currentSnapshotSignature());
  });

  it("maybeRestoreDraft discards on cancel and hydrates on confirm", async () => {
    bridge.discardDraft.mockResolvedValue({ ok: true });
    const { deps, settings, toasts } = makeDeps();
    const api = useConfigPersistence(deps);
    const status = { exists: true, is_newer_than_saved: true, config_snapshot: { cloudflare: { comment: "草稿备注" } } };
    (window.confirm as ReturnType<typeof vi.fn>).mockReturnValue(false);
    await api.maybeRestoreDraft(status);
    expect(bridge.discardDraft).toHaveBeenCalledTimes(1);
    expect(settings.comment).toBe("");
    (window.confirm as ReturnType<typeof vi.fn>).mockReturnValue(true);
    await api.maybeRestoreDraft(status);
    expect(settings.comment).toBe("草稿备注");
    expect(api.draftRestoring()).toBe(false);
    expect(toasts[toasts.length - 1]).toEqual({ message: "已恢复未保存草稿", tone: "success" });
  });

  it("maybeRestoreDraft ignores stale or empty drafts", async () => {
    const { deps } = makeDeps();
    const api = useConfigPersistence(deps);
    await api.maybeRestoreDraft({ exists: false });
    await api.maybeRestoreDraft({ exists: true, is_newer_than_saved: false });
    await api.maybeRestoreDraft({ exists: true, is_newer_than_saved: true, config_snapshot: {} });
    expect(window.confirm).not.toHaveBeenCalled();
  });
});

describe("autoSaveSettings", () => {
  it("interaction reason merges through the auto saver timer", async () => {
    bridge.saveConfig.mockResolvedValue({ ok: true, data: { config_snapshot: {} }, message: "" });
    const { deps } = makeDeps();
    const api = useConfigPersistence(deps);
    api.markConfigHydrated();
    expect(await api.autoSaveSettings("interaction")).toBe(true);
    expect(bridge.saveConfig).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1000);
    await vi.waitFor(() => expect(bridge.saveConfig).toHaveBeenCalled());
  });
});