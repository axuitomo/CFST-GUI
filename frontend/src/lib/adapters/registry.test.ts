import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const harness = vi.hoisted(() => ({
  desktopHost: false,
  platform: "web",
  ready: false,
  droppedHandler: null as null | (() => void),
  eventsOnCalls: [] as string[],
}));

vi.mock("./desktop/runtime", () => ({
  desktopWindowControls: { isAvailable: () => harness.ready },
  EventsOn: (name: string, cb: () => void) => {
    harness.eventsOnCalls.push(name);
    if (name === "sources:dropped") {
      harness.droppedHandler = cb;
    }
    return () => {
      if (name === "sources:dropped") {
        harness.droppedHandler = null;
      }
    };
  },
  isWailsDesktopHost: () => harness.desktopHost,
  isWailsRuntimeAvailable: () => harness.ready,
  waitForWailsRuntime: async () => harness.ready,
}));

vi.mock("@capacitor/core", () => ({
  Capacitor: { getPlatform: () => harness.platform },
  registerPlugin: () => ({
    Init: vi.fn(async () => ({ ok: true })),
    Invoke: vi.fn(async () => ({ ok: true, code: "OK", data: {} })),
    addListener: vi.fn(),
  }),
}));

async function loadRegistry() {
  vi.resetModules();
  return import("./registry");
}

beforeEach(() => {
  harness.desktopHost = false;
  harness.platform = "web";
  harness.ready = false;
  harness.droppedHandler = null;
  harness.eventsOnCalls = [];
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("platform adapter registry", () => {
  it("selects distinct adapters per host mode and exposes desktop window controls only on wails", async () => {
    const reg1 = await loadRegistry();
    expect(reg1.getPlatformAdapter("webui").mode).toBe("webui");
    expect(reg1.getPlatformAdapter("webui").windowControls).toBeUndefined();
    expect(reg1.getPlatformAdapter("native").mode).toBe("native");
    expect(reg1.getPlatformAdapter("native").windowControls).toBeUndefined();
    expect(reg1.getPlatformAdapter("wails").mode).toBe("wails");
    expect(reg1.getPlatformAdapter("wails").windowControls).toBeDefined();

    harness.desktopHost = true;
    harness.ready = true;
    const reg2 = await loadRegistry();
    expect(reg2.currentBridgeMode()).toBe("wails");
    expect(reg2.getDesktopWindowControls()?.isAvailable()).toBe(true);
  });

  it("subscribes dropped sources only on desktop hosts and wires runtime ready events", async () => {
    const listeners = new Map<string, Set<() => void>>();
    vi.stubGlobal("window", {
      addEventListener: (name: string, fn: () => void) => {
        if (!listeners.has(name)) listeners.set(name, new Set());
        listeners.get(name)?.add(fn);
      },
      removeEventListener: (name: string, fn: () => void) => {
        listeners.get(name)?.delete(fn);
      },
    });

    harness.desktopHost = true;
    const reg = await loadRegistry();
    const dropped = vi.fn();
    const offDropped = reg.subscribeDroppedSources(dropped);
    expect(harness.eventsOnCalls).toEqual(["sources:dropped"]);
    harness.droppedHandler?.();
    expect(dropped).toHaveBeenCalledTimes(1);
    offDropped();
    expect(harness.droppedHandler).toBeNull();

    const onReady = vi.fn();
    const offReady = reg.onBridgeRuntimeReady(onReady);
    listeners.get("wails:runtime-config-ready")?.forEach((fn) => fn());
    expect(onReady).toHaveBeenCalledTimes(1);
    offReady();
    expect(listeners.get("wails:runtime-config-ready")?.size).toBe(0);
  });
});
