// useSourceProfiles 行为锁定（重构计划 §4.2）：档案应用/签名/更新期变更保护/切换守卫。
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { useSourceProfiles, type SourceProfilesDeps } from "./useSourceProfiles";
import type { SourceDraft } from "./useSources";
import type { SourceGroupIPMode } from "../lib/sourceGroups";
import type { ColoFilterMode } from "../lib/bridge";

const { bridge } = vi.hoisted(() => ({
  bridge: {
    deleteSourceProfile: vi.fn(),
    saveSourceProfile: vi.fn(),
    switchSourceProfile: vi.fn(),
    updateCurrentSourceProfile: vi.fn(),
  },
}));
vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, ...bridge };
});

const STORE = {
  active_profile_id: "p1",
  items: [
    {
      id: "p1",
      name: "组一",
      ip_mode: "mcis",
      colo_filter: "LAX",
      colo_filter_mode: "deny",
      sources: [{ id: "s1", kind: "url", url: "https://a.example", enabled: true, name: "A", content: "", path: "", colo_filter: "", colo_filter_mode: "allow", ip_mode: "mcis", ip_limit: -1, last_fetched_at: "", last_fetched_count: 0, status_text: " " }],
      created_at: "",
      updated_at: "",
    },
  ],
  schema_version: "1",
  updated_at: "",
};

function makeDeps(overrides: Partial<SourceProfilesDeps> = {}) {
  const sources = ref<SourceDraft[]>([]);
  const sourceGroupIPMode = ref<SourceGroupIPMode>("traverse");
  const sourceGroupColoFilter = ref("");
  const sourceGroupColoFilterMode = ref<ColoFilterMode | "">("allow");
  const configPath = ref("");
  const logs: string[] = [];
  const toasts: Array<{ message: string; tone?: string }> = [];
  const baselines: string[] = [];
  const deps: SourceProfilesDeps = {
    sources,
    sourceGroupIPMode,
    sourceGroupColoFilter,
    sourceGroupColoFilterMode,
    sourcePayloads: () => [],
    currentSourceSignature: () => "sig",
    sourcePageHasUnsavedChanges: () => false,
    autoSaveSourcePage: vi.fn(async () => true),
    selectedView: () => "sources",
    configPath,
    appendLog: (event) => logs.push(event),
    showToast: (message, tone) => toasts.push({ message, tone }),
    setLastSavedSourceProfileSignature: (sig) => baselines.push(sig),
    ...overrides,
  };
  return { deps, sources, sourceGroupIPMode, sourceGroupColoFilter, sourceGroupColoFilterMode, configPath, logs, toasts, baselines };
}

beforeEach(() => {
  for (const fn of Object.values(bridge)) fn.mockReset();
  vi.stubGlobal("window", { confirm: vi.fn(() => true) });
});
afterEach(() => {
  vi.unstubAllGlobals();
});

describe("applySourceProfileStore", () => {
  it("mirrors active profile colo settings into group refs", () => {
    const { deps, sourceGroupColoFilter, sourceGroupColoFilterMode } = makeDeps();
    const api = useSourceProfiles(deps);
    api.applySourceProfileStore(STORE);
    expect(api.sourceProfiles.value.active_profile_id).toBe("p1");
    expect(sourceGroupColoFilter.value).toBe("LAX");
    expect(sourceGroupColoFilterMode.value).toBe("deny");
    api.applySourceProfileStore({ active_profile_id: "missing", items: [] });
    expect(sourceGroupColoFilter.value).toBe("");
    expect(sourceGroupColoFilterMode.value).toBe("allow");
  });

  it("activeSourceProfileSourceSignature normalizes status_text", () => {
    const { deps } = makeDeps();
    const api = useSourceProfiles(deps);
    api.applySourceProfileStore(STORE);
    const sig = api.activeSourceProfileSourceSignature();
    expect(sig).toContain('"status_text":""');
    expect(sig).toContain('"ip_mode":"mcis"');
  });
});

describe("updateActiveSourceProfile", () => {
  it("replaces sources from response and records baseline on success", async () => {
    bridge.updateCurrentSourceProfile.mockResolvedValue({ ok: true, data: { source_profiles: STORE, sources: STORE.items[0].sources }, message: "" });
    const { deps, sources, toasts, baselines } = makeDeps();
    const api = useSourceProfiles(deps);
    api.applySourceProfileStore(STORE);
    const ok = await api.updateActiveSourceProfile();
    expect(ok).toBe(true);
    expect(sources.value).toHaveLength(1);
    expect(sources.value[0].status_text).toBe(" ");
    expect(baselines).toHaveLength(1);
    expect(toasts[toasts.length - 1]).toEqual({ message: "输入组已更新并保存", tone: "success" });
  });

  it("preserves latest colo filter when sources changed during save", async () => {
    bridge.updateCurrentSourceProfile.mockResolvedValue({ ok: true, data: { source_profiles: STORE, sources: STORE.items[0].sources }, message: "" });
    let call = 0;
    const { deps, sourceGroupColoFilter, sourceGroupColoFilterMode, sources } = makeDeps({
      currentSourceSignature: () => (call++ === 0 ? "before" : "after"),
    });
    const api = useSourceProfiles(deps);
    api.applySourceProfileStore(STORE);
    sourceGroupColoFilter.value = "SJC";
    sourceGroupColoFilterMode.value = "allow";
    const ok = await api.updateActiveSourceProfile();
    expect(ok).toBe(true);
    expect(sourceGroupColoFilter.value).toBe("SJC");
    expect(sourceGroupColoFilterMode.value).toBe("allow");
    expect(sources.value).toHaveLength(0);
  });

  it("quiet variant logs failure instead of toasting", async () => {
    bridge.updateCurrentSourceProfile.mockResolvedValue({ ok: false, message: "后端拒绝" });
    const { deps, logs, toasts } = makeDeps();
    const api = useSourceProfiles(deps);
    const ok = await api.updateActiveSourceProfileQuietly();
    expect(ok).toBe(false);
    expect(logs).toContain("source.profile_quiet_save.failed");
    expect(toasts).toHaveLength(0);
  });
});

describe("switch / remove guards", () => {
  it("switch cancels when auto-save fails on unsaved sources page", async () => {
    const { deps, toasts } = makeDeps({
      sourcePageHasUnsavedChanges: () => true,
      autoSaveSourcePage: vi.fn(async () => false),
    });
    const api = useSourceProfiles(deps);
    await api.switchToSourceProfile("p2");
    expect(bridge.switchSourceProfile).not.toHaveBeenCalled();
    expect(toasts[toasts.length - 1]).toEqual({ message: "输入源保存失败，已取消切换", tone: "error" });
  });

  it("switch applies store, sources and config path on success", async () => {
    bridge.switchSourceProfile.mockResolvedValue({ ok: true, data: { source_profiles: STORE, sources: STORE.items[0].sources, config_snapshot: {}, configPath: "/new/config.json" }, message: "" });
    const { deps, sources, sourceGroupIPMode, configPath } = makeDeps();
    const api = useSourceProfiles(deps);
    await api.switchToSourceProfile("p1");
    expect(sources.value).toHaveLength(1);
    expect(sourceGroupIPMode.value).toBe("mcis");
    expect(configPath.value).toBe("/new/config.json");
  });

  it("remove respects confirm dialog", async () => {
    (window.confirm as ReturnType<typeof vi.fn>).mockReturnValue(false);
    const { deps } = makeDeps();
    const api = useSourceProfiles(deps);
    await api.removeSourceProfile("p1");
    expect(bridge.deleteSourceProfile).not.toHaveBeenCalled();
  });
});
