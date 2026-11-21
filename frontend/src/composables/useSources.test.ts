// useSources 行为锁定（重构计划 §4.2）：草稿创建/分组覆盖/状态映射/预览/签名基线。
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useSources, type SourcesDeps } from "./useSources";

const { bridge } = vi.hoisted(() => ({
  bridge: {
    consumeDroppedSources: vi.fn(),
    previewSource: vi.fn(),
  },
}));
vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, ...bridge };
});

function makeDeps(overrides: Partial<SourcesDeps> = {}) {
  const logs: Array<{ event: string }> = [];
  const toasts: Array<{ message: string; tone?: string }> = [];
  const deps: SourcesDeps = {
    appendLog: (event) => logs.push({ event }),
    showToast: (message, tone) => toasts.push({ message, tone }),
    autoDetectSourceName: () => false,
    buildConfigSnapshot: () => ({}) as never,
    commandDiagnosticPayload: (result) => result,
    currentSourceProfileSignature: () => "profile-sig",
    activeSourceProfileSourceSignature: () => "active-sig",
    ...overrides,
  };
  return { deps, logs, toasts };
}

beforeEach(() => {
  bridge.consumeDroppedSources.mockReset();
  bridge.previewSource.mockReset();
  vi.stubGlobal("window", { confirm: vi.fn(() => true) });
});
afterEach(() => {
  vi.unstubAllGlobals();
});

describe("drafts and group overrides", () => {
  it("addSource seeds names and inherits group ip mode", () => {
    const { deps } = makeDeps();
    const api = useSources(deps);
    api.addSource("url");
    api.addSource("file");
    expect(api.sources.value.map((s) => s.name)).toEqual(["输入源 1", "输入源 2"]);
    expect(api.sources.value.every((s) => s.ip_mode === "traverse")).toBe(true);
    api.setSourceGroupIPMode("mcis");
    expect(api.sources.value.every((s) => s.ip_mode === "mcis")).toBe(true);
  });

  it("sourcePayloads applies group colo filter and trims fields", () => {
    const { deps } = makeDeps();
    const api = useSources(deps);
    api.addSource("url");
    api.sources.value[0].url = "  https://a.example  ";
    api.sources.value[0].colo_filter = "LAX";
    api.setSourceGroupColoFilter("  SJC  ");
    api.setSourceGroupColoFilterMode("deny");
    const payload = api.sourcePayloads.value[0];
    expect(payload.url).toBe("https://a.example");
    expect(payload.colo_filter).toBe("SJC");
    expect(payload.colo_filter_mode).toBe("deny");
    expect(payload.ip_limit).toBe(-1);
  });

  it("preparedSources/noSources reflect enabled + usable input", () => {
    const { deps } = makeDeps();
    const api = useSources(deps);
    expect(api.noSources.value).toBe(true);
    api.addSource("inline");
    expect(api.noSources.value).toBe(true);
    api.sources.value[0].content = "1.1.1.1";
    expect(api.noSources.value).toBe(false);
    api.sources.value[0].enabled = false;
    expect(api.noSources.value).toBe(true);
  });

  it("removeSource respects confirm and cleans preview state", () => {
    const { deps } = makeDeps();
    const api = useSources(deps);
    api.addSource("url");
    const id = api.sources.value[0].id;
    api.applySourcePreview(id, { summary: { total_count: 2 }, preview_entries: ["a", "b"] }, []);
    (window.confirm as ReturnType<typeof vi.fn>).mockReturnValue(false);
    api.removeSource(id);
    expect(api.sources.value).toHaveLength(1);
    (window.confirm as ReturnType<typeof vi.fn>).mockReturnValue(true);
    api.removeSource(id);
    expect(api.sources.value).toHaveLength(0);
    expect(api.sourcePreviewStates[id]).toBeUndefined();
  });
});

describe("statuses and dropped paths", () => {
  it("applySourceStatuses maps by id with trimming and camelCase fallback", () => {
    const { deps } = makeDeps();
    const api = useSources(deps);
    api.addSource("url");
    const id = api.sources.value[0].id;
    api.applySourceStatuses([{ id, lastFetchedAt: " 2026-11-21 ", lastFetchedCount: "7", statusText: " 就绪 " }]);
    expect(api.sources.value[0].last_fetched_at).toBe("2026-11-21");
    expect(api.sources.value[0].last_fetched_count).toBe(7);
    expect(api.sources.value[0].status_text).toBe("就绪");
    api.applySourceStatuses([]);
    expect(api.sources.value[0].status_text).toBe("就绪");
  });

  it("applyDroppedSourcePaths dedupes by path and names from filename", () => {
    const { deps, toasts } = makeDeps();
    const api = useSources(deps);
    api.applyDroppedSourcePaths(["C:\\lists\\a.txt", "C:\\lists\\a.txt", "/lists/b.txt"]);
    expect(api.sources.value).toHaveLength(2);
    expect(api.sources.value.map((s) => s.name)).toEqual(["a.txt", "b.txt"]);
    expect(toasts[0].message).toContain("2 个拖放文件");
    api.applyDroppedSourcePaths(["/lists/b.txt"]);
    expect(api.sources.value).toHaveLength(2);
  });
});

describe("signatures and baselines", () => {
  it("sourcePageHasUnsavedChanges compares against baselines and active profile", () => {
    const { deps } = makeDeps();
    const api = useSources(deps);
    // empty sources vs empty active profile signature differ -> unsaved
    expect(api.sourcePageHasUnsavedChanges()).toBe(true);
    api.markSourceSaveBaselines();
    // baselines now match current, but active profile sig is still "active-sig"
    expect(api.sourcePageHasUnsavedChanges()).toBe(true);
    const { deps: deps2 } = makeDeps({ activeSourceProfileSourceSignature: () => api.currentSourceSignature() });
    const api2 = useSources(deps2);
    api2.markSourceSaveBaselines();
    expect(api2.sourcePageHasUnsavedChanges()).toBe(false);
    api2.addSource("url");
    expect(api2.sourcePageHasUnsavedChanges()).toBe(true);
  });

  it("currentSourceSignature is key-order invariant and content sensitive", () => {
    const { deps } = makeDeps();
    const api = useSources(deps);
    api.addSource("url");
    const before = api.currentSourceSignature();
    api.sources.value[0].url = "https://x.example";
    expect(api.currentSourceSignature()).not.toBe(before);
  });
});

describe("inspectSource", () => {
  it("records input issue without calling bridge", async () => {
    const { deps } = makeDeps();
    const api = useSources(deps);
    api.addSource("url");
    const id = api.sources.value[0].id;
    await api.inspectSource(id);
    expect(api.sourcePreviewErrors[id]).toBeTruthy();
    expect(bridge.previewSource).not.toHaveBeenCalled();
  });

  it("stores preview on ok and clears request state", async () => {
    bridge.previewSource.mockResolvedValue({ ok: true, data: { summary: { total_count: 1 }, preview_entries: ["1.1.1.1"] }, warnings: [] });
    const { deps } = makeDeps();
    const api = useSources(deps);
    api.addSource("url");
    api.sources.value[0].url = "https://a.example";
    const id = api.sources.value[0].id;
    await api.inspectSource(id);
    expect(api.sourcePreviewStates[id].entries).toEqual(["1.1.1.1"]);
    expect(api.sourceRequestStates[id]).toBeUndefined();
  });
});