import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const harness = vi.hoisted(() => ({
  platform: "android",
  init: vi.fn(),
  invoke: vi.fn(),
}));
vi.mock("@capacitor/core", () => ({
  Capacitor: { getPlatform: () => harness.platform },
  registerPlugin: () => ({ Init: harness.init, Invoke: harness.invoke, addListener: vi.fn() }),
}));
vi.mock("./wailsRuntime", () => ({
  EventsOn: () => () => {},
  isWailsDesktopHost: () => false,
  isWailsRuntimeAvailable: () => false,
  waitForWailsRuntime: async () => false,
}));

const emptyPage = { ok: true, code: "RESULT_FILE_LISTED", data: { count: 0, results: [], total_count: 0 } };
const cleanups: Array<() => void> = [];

async function loadBridge() {
  vi.resetModules();
  return import("./bridge");
}

beforeEach(() => {
  harness.platform = "android";
  harness.init.mockReset().mockResolvedValue({ ok: true });
  harness.invoke.mockReset().mockResolvedValue(emptyPage);
});
afterEach(() => {
  cleanups.splice(0).forEach((cleanup) => cleanup());
  vi.unstubAllGlobals();
});

describe("native bridge initialization and query boundaries", () => {
  it.each([false, undefined, "true", 1])("rejects ok=%s and retries initialization", async (ok) => {
    harness.init.mockResolvedValueOnce({ ok, message: "init rejected fake-secret" });
    const bridge = await loadBridge();
    await expect(bridge.listTaskResults("task-a", "tcp", "asc")).rejects.toThrow("init rejected");
    await expect(bridge.listTaskResults("task-a", "tcp", "asc")).resolves.toMatchObject({ ok: true });
    expect(harness.init).toHaveBeenCalledTimes(2);
    const traces = harness.invoke.mock.calls.filter(([request]) => request.command === "bridge.trace");
    expect(traces).toHaveLength(1);
    expect(JSON.parse(traces[0][0].payload_json)).toEqual({ source: "frontend", event: "bridge.init_failed" });
  });

  it("retries a rejected native initialization promise", async () => {
    harness.init.mockRejectedValueOnce(new Error("native unavailable"));
    const bridge = await loadBridge();
    await expect(bridge.listTaskResults("task-a", "tcp", "asc")).rejects.toThrow("native unavailable");
    await bridge.listTaskResults("task-a", "tcp", "asc");
    expect(harness.init).toHaveBeenCalledTimes(2);
  });

  it("shares concurrent initialization and reuses success without invocation traces", async () => {
    let finish!: (value: unknown) => void;
    harness.init.mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const bridge = await loadBridge();
    const first = bridge.listTaskResults("task-a", "tcp", "asc");
    const second = bridge.listTaskResults("task-b", "tcp", "asc");
    expect(harness.init).toHaveBeenCalledTimes(1);
    finish({ ok: true });
    await Promise.all([first, second]);
    await bridge.listTaskResults("task-c", "tcp", "asc");
    expect(harness.init).toHaveBeenCalledTimes(1);
    expect(harness.invoke.mock.calls.map(([request]) => request.command)).toEqual(["task.results", "task.results", "task.results"]);
  });

  it.each(["path", "source_path", "sourcePath", "target_path", "targetPath", "export_path", "exportPath"])("projects query fields and accepts %s", async (pathAlias) => {
    const bridge = await loadBridge();
    await bridge.listTaskResults(
      "result-file",
      "tcp",
      "desc",
      {
        [pathAlias]: " result.csv ",
        config: { password: "must-not-cross" },
        arbitrary: "dropped",
      },
      "ipv4",
      { limit: 20, offset: 40 },
    );
    expect(harness.invoke).toHaveBeenCalledTimes(1);
    expect(JSON.parse(harness.invoke.mock.calls[0][0].payload_json)).toEqual({
      task_id: "result-file",
      path: "result.csv",
      sort_by: "tcp",
      order: "desc",
      ip_filter: "ipv4",
      limit: 20,
      offset: 40,
    });
  });
  it("projects result history commands onto the shared core", async () => {
    const bridge = await loadBridge();
    const rows = [{ address: "1.1.1.1", export_status: "", stage_status: "completed" }];
    await bridge.mergeResultHistoryBatch("task-1", rows, 1768910400000.9);
    await bridge.queryResultHistory();
    await bridge.importResultHistory([{ address: "1.1.1.1", batch_id: "batch-old", colo: "", download_mbps: null, max_download_mbps: null, source_port: null, stage_status: "completed", tcp_latency_ms: null, test_port: null, timestamp: 1768910400000, trace_latency_ms: null }]);

    expect(harness.invoke.mock.calls.map(([request]) => request.command)).toEqual(["history.merge_batch", "history.query", "history.import"]);
    expect(JSON.parse(harness.invoke.mock.calls[0][0].payload_json)).toEqual({ at: 1768910400000, batch_id: "task-1", rows });
    expect(JSON.parse(harness.invoke.mock.calls[1][0].payload_json)).toEqual({});
  });
});

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  onmessage: ((message: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  url: string;
  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }
}

function setupWebUI() {
  harness.platform = "web";
  FakeEventSource.instances = [];
  const tokens = new Map([["cfst-webui-token", "old-token"]]);
  const prompt = vi.fn(() => "new-token");
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => tokens.get(key) ?? null,
    setItem: (key: string, value: string) => tokens.set(key, value),
    removeItem: (key: string) => tokens.delete(key),
  });
  vi.stubGlobal("window", { prompt });
  vi.stubGlobal("EventSource", FakeEventSource);
  const fetchMock = vi.fn(async (path: string, init?: RequestInit) => {
    if (path === "/api/health") {
      return new Response(JSON.stringify({ service: "cfst-webui", auth_required: true }), { headers: { "content-type": "application/json" } });
    }
    if (new Headers(init?.headers).get("Authorization") === "Bearer old-token") {
      return new Response("{}", { status: 401 });
    }
    return new Response(JSON.stringify(emptyPage), { headers: { "content-type": "application/json" } });
  });
  vi.stubGlobal("fetch", fetchMock);
  return { fetchMock, prompt };
}

describe("WebUI event token lifecycle", () => {
  it("replaces an old-token stream after 401 and keeps subscriptions and cleanup idempotent", async () => {
    const { fetchMock, prompt } = setupWebUI();
    const bridge = await loadBridge();
    const received = vi.fn();
    const firstOff = await bridge.listenToProbeEvents(received);
    const secondOff = await bridge.listenToProbeEvents(() => {});
    cleanups.push(firstOff, secondOff);
    expect(FakeEventSource.instances).toHaveLength(1);
    expect(FakeEventSource.instances[0].url).toContain("token=old-token");
    await bridge.listTaskResults("task-a", "tcp", "asc");
    expect(prompt).toHaveBeenCalledTimes(1);
    expect(FakeEventSource.instances).toHaveLength(2);
    expect(FakeEventSource.instances[0].close).toHaveBeenCalledTimes(1);
    const current = FakeEventSource.instances[1];
    expect(current.url).toContain("token=new-token");
    current.onmessage?.({ data: JSON.stringify({ event: "probe.progress", task_id: "task-a", seq: 1, payload: { processed: 1 } }) });
    expect(received).toHaveBeenCalledTimes(1);
    await bridge.rebindProbeEventListener();
    expect(FakeEventSource.instances).toHaveLength(2);
    expect(fetchMock.mock.calls.filter(([path]) => path.includes("/api/command/"))).toHaveLength(2);
    firstOff();
    expect(current.close).not.toHaveBeenCalled();
    secondOff();
    secondOff();
    expect(current.close).toHaveBeenCalledTimes(1);
  });

  it("does not recreate SSE if the last listener leaves while auth refresh awaits health", async () => {
    const { fetchMock } = setupWebUI();
    const bridge = await loadBridge();
    const off = await bridge.listenToProbeEvents(() => {});
    cleanups.push(off);
    let finishHealth!: (response: Response) => void;
    fetchMock
      .mockImplementationOnce(async () => new Response("{}", { status: 401 }))
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finishHealth = resolve;
          }),
      );
    const query = bridge.listTaskResults("task-a", "tcp", "asc");
    await vi.waitFor(() => expect(finishHealth).toBeTypeOf("function"));
    off();
    finishHealth(new Response(JSON.stringify({ service: "cfst-webui", auth_required: true }), { headers: { "content-type": "application/json" } }));
    await query;
    expect(FakeEventSource.instances).toHaveLength(1);
    expect(FakeEventSource.instances[0].close).toHaveBeenCalledTimes(1);
  });

  it("removes a listener whose initial binding failed", async () => {
    const { prompt } = setupWebUI();
    localStorage.removeItem("cfst-webui-token");
    prompt.mockReturnValue("");
    const bridge = await loadBridge();
    await expect(bridge.listenToProbeEvents(() => {})).rejects.toThrow("缺少 WebUI");
    prompt.mockReturnValue("new-token");
    await bridge.rebindProbeEventListener();
    expect(FakeEventSource.instances).toHaveLength(0);
  });
});
