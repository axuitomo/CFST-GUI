import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// 固定住 Wails 运行时是否就绪的开关与 EventsOn 订阅记录，模拟桌面端启动竞态：
// 首次绑定时尚未注入 _wails.environment（ready=false），运行时就绪后置为 true。
const harness = vi.hoisted(() => ({
  ready: false,
  subscriptions: [] as Array<{ name: string; callback: (payload: unknown) => void }>,
}));

vi.mock("./wailsRuntime", () => ({
  isWailsRuntimeAvailable: () => harness.ready,
  isWailsDesktopHost: () => false,
  waitForWailsRuntime: async () => harness.ready,
  EventsOn: (name: string, callback: (payload: unknown) => void) => {
    const entry = { name, callback };
    harness.subscriptions.push(entry);
    return () => {
      const index = harness.subscriptions.indexOf(entry);
      if (index >= 0) {
        harness.subscriptions.splice(index, 1);
      }
    };
  },
}));

vi.mock("@capacitor/core", () => ({
  Capacitor: { getPlatform: () => "web" },
  registerPlugin: () => ({
    Init: vi.fn(async () => ({})),
    addListener: vi.fn(),
  }),
}));

import { listenToProbeEvents, rebindProbeEventListener } from "./bridge";
import type { ProbeEventEnvelope } from "./bridge/types";

interface FakeEventSourceLike {
  url: string;
  close: ReturnType<typeof vi.fn>;
  onerror: (() => void) | null;
}

function progressEnvelope(seq: number): ProbeEventEnvelope {
  return {
    event: "probe.progress",
    payload: { processed: 1, total: 2, passed: 1, failed: 0 },
    schema_version: "test",
    seq,
    task_id: "task-a",
    ts: "2026-09-19T00:00:00Z",
  };
}

let sources: FakeEventSourceLike[] = [];
let unsubscribe: (() => void) | null = null;

beforeEach(() => {
  harness.ready = false;
  harness.subscriptions.length = 0;
  sources = [];
  unsubscribe = null;

  // webui 通道在绑定前会探测 /api/health 决定是否需要令牌；测试中一律返回“无需令牌”。
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({
      ok: false,
      status: 503,
      headers: { get: () => null },
      json: async () => ({}),
    })),
  );

  class FakeEventSource {
    url: string;
    onmessage: ((message: { data: string }) => void) | null = null;
    onerror: (() => void) | null = null;
    close = vi.fn();

    constructor(url: string) {
      this.url = url;
      sources.push(this);
    }
  }

  vi.stubGlobal("EventSource", FakeEventSource);
});

afterEach(() => {
  unsubscribe?.();
  unsubscribe = null;
  vi.unstubAllGlobals();
});

describe("probe event transport rebinding", () => {
  it("桌面启动竞态误选 WebUI SSE，运行时就绪后 rebind 切回 Wails 事件通道", async () => {
    const received: ProbeEventEnvelope[] = [];
    unsubscribe = await listenToProbeEvents((event) => {
      received.push(event);
    });

    // 竞态输掉：Wails 运行时尚未就绪，误绑到桌面构建中不存在的 SSE 路由。
    expect(sources).toHaveLength(1);
    expect(sources[0].url).toContain("/api/events/probe");
    expect(harness.subscriptions).toHaveLength(0);

    // Wails 运行时延迟就绪后触发校正。
    harness.ready = true;
    await rebindProbeEventListener();

    // 旧 SSE 被关闭，Wails 事件通道被建立，业务 handler 不重新注册。
    expect(sources[0].close).toHaveBeenCalledTimes(1);
    expect(harness.subscriptions).toHaveLength(1);
    expect(harness.subscriptions[0].name).toBe("probe:event");

    // Wails 事件经新通道到达业务 handler。
    harness.subscriptions[0].callback(progressEnvelope(1));
    expect(received).toHaveLength(1);
    expect(received[0].event).toBe("probe.progress");

    // 幂等：通道未变化时再次校正不应重复订阅。
    await rebindProbeEventListener();
    expect(harness.subscriptions).toHaveLength(1);
  });

  it("误绑的 SSE 在 onerror 且 Wails 就绪后自动切回 Wails 事件通道", async () => {
    unsubscribe = await listenToProbeEvents(() => {});
    expect(sources).toHaveLength(1);
    expect(harness.subscriptions).toHaveLength(0);

    harness.ready = true;
    sources[0].onerror?.();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(sources[0].close).toHaveBeenCalledTimes(1);
    expect(harness.subscriptions).toHaveLength(1);
    expect(harness.subscriptions[0].name).toBe("probe:event");
  });
});
