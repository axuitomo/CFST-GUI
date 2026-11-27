// @ts-expect-error V3 generator emits JavaScript bindings for this project.
import * as WailsAppBinding from "../../../../bindings/github.com/axuitomo/CFST-GUI/internal/adapters/desktop/app.js";
import { normalizeCommandResult } from "../../bridge/command";
import { normalizeNativePayload, normalizeProbeEvent } from "../../bridge/normalize";
import type { PathSelectionPayload } from "../../bridge/types";
import type { PlatformAdapter, PlatformMethodName, ProbeEventSubscriptionOptions } from "../types";
import { desktopWindowControls, EventsOn, isWailsDesktopHost, isWailsRuntimeAvailable } from "./runtime";

export interface WailsAppBridge {
  Invoke: (command: string, payloadJSON: string) => Promise<string>;
  CheckForUpdates: (payload: Record<string, unknown>) => Promise<unknown>;
  DownloadAndInstallUpdate: (payload: Record<string, unknown>) => Promise<unknown>;
  GetAppInfo: () => Promise<unknown>;
  OpenLogDirectory: (payload: Record<string, unknown>) => Promise<unknown>;
  OpenPath: (targetPath: string) => Promise<void>;
  OpenReleasePage: () => Promise<unknown>;
  SelectPath: (payload: Record<string, unknown>) => Promise<unknown>;
}

function appBridge(): WailsAppBridge {
  if (!isWailsRuntimeAvailable()) {
    throw new Error("Wails bridge is not ready.");
  }
  return WailsAppBinding as unknown as WailsAppBridge;
}

export const desktopAdapter: PlatformAdapter = {
  mode: "wails",
  desktop: {
    windowControls: desktopWindowControls,

    subscribeDroppedSources(handler: () => void) {
      if (!isWailsDesktopHost()) {
        return () => {};
      }
      return EventsOn("sources:dropped", handler);
    },

    onRuntimeReady(handler: () => void) {
      if (typeof window === "undefined") {
        return () => {};
      }
      window.addEventListener("wails:runtime-config-ready", handler);
      return () => {
        window.removeEventListener("wails:runtime-config-ready", handler);
      };
    },
  },

  async invokeCore<T>(command: string, payloadJSON: string) {
    return normalizeCommandResult<T>(normalizeNativePayload(await appBridge().Invoke(command, payloadJSON)));
  },

  async invokePlatform<T>(method: PlatformMethodName, payload?: Record<string, unknown>) {
    const bridge = appBridge();
    const fn = bridge[method] as (arg?: Record<string, unknown>) => Promise<unknown>;
    return normalizeCommandResult<T>(payload ? await fn.call(bridge, payload) : await bridge.GetAppInfo());
  },

  async subscribeProbeEvents({ onEvent }: ProbeEventSubscriptionOptions) {
    return EventsOn("probe:event", (payload: unknown) => {
      const event = normalizeProbeEvent(payload);
      if (event) {
        onEvent(event);
      }
    });
  },

  async selectPath(payload: Record<string, unknown>) {
    return normalizeCommandResult<PathSelectionPayload>(await appBridge().SelectPath(payload));
  },

  async openPath(targetPath: string) {
    await appBridge().OpenPath(targetPath);
  },

  async openLogDirectory(payload: Record<string, unknown>) {
    return normalizeCommandResult(await appBridge().OpenLogDirectory(payload));
  },

  async openReleasePage() {
    return normalizeCommandResult(await appBridge().OpenReleasePage());
  },

  async consumeDroppedSources() {
    return this.invokeCore<{ paths: string[] }>("sources.dropped.consume", "{}");
  },
};
