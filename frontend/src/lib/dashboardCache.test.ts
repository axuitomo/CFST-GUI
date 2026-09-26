import { afterEach, describe, expect, it, vi } from "vitest";
import { DASHBOARD_CACHE_STORAGE_KEY, readDashboardCache, writeDashboardCache, type DashboardCache } from "./dashboardCache";

function stubBrowserStorage() {
  const store = new Map<string, string>();
  vi.stubGlobal("window", {
    localStorage: {
      getItem: (key: string) => store.get(key) ?? null,
      removeItem: (key: string) => store.delete(key),
      setItem: (key: string, value: string) => {
        store.set(key, value);
      },
    },
  });
  return store;
}

const sample: DashboardCache = {
  activityFeed: [{ detail: "已恢复最近一次任务快照。", title: "已恢复历史结果", ts: "2026-08-06T00:00:00Z" }],
  status: { detail: "已恢复最近一次任务快照和结果。", title: "已恢复历史结果", tone: "completed" },
  summary: { accepted: 300, exported: 12, failed: 4, filtered: 9, invalid: 2, passed: 12, processed: 300, total: 300 },
  taskSnapshot: { status: "completed", task_id: "task-a", updated_at: "2026-08-06T00:00:00Z" },
  updatedAt: "2026-08-06T00:00:00Z",
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("dashboardCache", () => {
  it("round-trips the last known board state", () => {
    stubBrowserStorage();

    writeDashboardCache(sample);

    expect(readDashboardCache()).toEqual(sample);
  });

  it("ignores malformed, foreign and value-less payloads", () => {
    const store = stubBrowserStorage();

    store.set(DASHBOARD_CACHE_STORAGE_KEY, "{not json");
    expect(readDashboardCache()).toBeNull();

    store.set(DASHBOARD_CACHE_STORAGE_KEY, JSON.stringify({ status: { title: "x" } }));
    expect(readDashboardCache()).toBeNull();

    store.set(DASHBOARD_CACHE_STORAGE_KEY, JSON.stringify({ activityFeed: [], status: {}, summary: {}, updatedAt: "2026-08-06T00:00:00Z" }));
    expect(readDashboardCache()).toBeNull();

    store.delete(DASHBOARD_CACHE_STORAGE_KEY);
    expect(readDashboardCache()).toBeNull();
  });

  it("coerces partial payloads into a usable board state", () => {
    const store = stubBrowserStorage();
    store.set(
      DASHBOARD_CACHE_STORAGE_KEY,
      JSON.stringify({
        activityFeed: [{ title: "保留" }, { detail: "" }, "drop", { title: "x", detail: "y", ts: 7 }],
        status: { title: "运行中", tone: "not-a-tone" },
        summary: { processed: 12.9, total: -4, failed: "3" },
        taskSnapshot: { status: "running" },
        updatedAt: "2026-08-06T00:00:00Z",
      }),
    );

    expect(readDashboardCache()).toEqual({
      activityFeed: [
        { detail: "", title: "保留", ts: "" },
        { detail: "y", title: "x", ts: "" },
      ],
      status: { detail: "", title: "运行中", tone: "idle" },
      summary: { accepted: 0, exported: 0, failed: 3, filtered: 0, invalid: 0, passed: 0, processed: 12, total: 0 },
      taskSnapshot: null,
      updatedAt: "2026-08-06T00:00:00Z",
    });
  });

  it("survives a page without web storage", () => {
    vi.stubGlobal("window", {});

    expect(readDashboardCache()).toBeNull();
    expect(() => writeDashboardCache(sample)).not.toThrow();
  });
});
