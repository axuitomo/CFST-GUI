import { commandResult, normalizeCommandResult } from "../../bridge/command";
import { normalizeNativePayload, normalizeProbeEvent } from "../../bridge/normalize";
import type { PathSelectionPayload } from "../../bridge/types";
import { toObjectRecord, toStringValue } from "../../bridgeValues";
import type { PlatformAdapter, PlatformMethodName, ProbeEventSubscriptionOptions } from "../types";
import { cfstNative, isAndroidNativePlatform, type CapacitorCfstPlugin } from "./plugin";

const BRIDGE_TRACE_ENABLED = true;
let nativeInitPromise: Promise<void> | null = null;

function bridgeTrace(event: string) {
  if (!BRIDGE_TRACE_ENABLED || !isAndroidNativePlatform()) {
    return;
  }
  const entry: Record<string, unknown> = { source: "frontend", event };
  cfstNative.Invoke({ command: "bridge.trace", payload_json: JSON.stringify(entry) }).catch(() => {
    // Best-effort trace; never breaks the main flow.
  });
}

export async function ensureNativeBridge() {
  if (!isAndroidNativePlatform()) {
    return;
  }
  if (!nativeInitPromise) {
    const currentInit = cfstNative
      .Init({})
      .then((value: unknown) => {
        const result = toObjectRecord(normalizeNativePayload(value));
        if (result.ok !== true) {
          throw new Error(toStringValue(result.message) || "原生桥初始化失败。");
        }
      })
      .catch((error: unknown) => {
        if (nativeInitPromise === currentInit) {
          nativeInitPromise = null;
        }
        bridgeTrace("bridge.init_failed");
        throw error;
      });
    nativeInitPromise = currentInit;
  }
  await nativeInitPromise;
}

export const androidAdapter: PlatformAdapter = {
  mode: "native",

  async invokeCore<T>(command: string, payloadJSON: string) {
    await ensureNativeBridge();
    return normalizeCommandResult<T>(
      normalizeNativePayload(
        await cfstNative.Invoke({
          command,
          payload_json: payloadJSON,
        }),
      ),
    );
  },

  async invokePlatform<T>(method: PlatformMethodName, payload?: Record<string, unknown>) {
    await ensureNativeBridge();
    const fn = cfstNative[method] as (arg?: Record<string, unknown>) => Promise<unknown>;
    return normalizeCommandResult<T>(normalizeNativePayload(payload ? await fn(payload) : await fn()));
  },

  android: {
    async invokeCommand<T>(method: string, args: unknown[] | null, code: string, unsupportedMessage: string) {
      await ensureNativeBridge();
      const fn = cfstNative[method as keyof CapacitorCfstPlugin] as ((...params: unknown[]) => Promise<unknown>) | undefined;
      if (typeof fn === "function") {
        const raw = args ? await fn(...args) : await fn();
        return normalizeCommandResult<T>(normalizeNativePayload(raw));
      }
      return commandResult<T | null>(code, null, { message: unsupportedMessage, ok: false });
    },

    async setSurfaceTheme(dark: boolean) {
      await ensureNativeBridge();
      if (typeof cfstNative.SetSurfaceTheme === "function") {
        await cfstNative.SetSurfaceTheme({ dark });
      }
    },

    async showNotification(title: string, body: string) {
      await ensureNativeBridge();
      if (typeof cfstNative.ShowNotification === "function") {
        return normalizeCommandResult(normalizeNativePayload(await cfstNative.ShowNotification({ title, body, sound: true })));
      }
      return commandResult("ANDROID_NOTIFICATION_UNSUPPORTED", null, { message: "当前环境不支持 Android 通知。", ok: false });
    },
  },

  async subscribeProbeEvents({ onEvent }: ProbeEventSubscriptionOptions) {
    await ensureNativeBridge();
    const handle = await cfstNative.addListener("probe:event", (payload: unknown) => {
      const event = normalizeProbeEvent(normalizeNativePayload(payload));
      if (event) {
        onEvent(event);
      }
    });
    return () => {
      void handle.remove();
    };
  },

  async selectPath(payload: Record<string, unknown>) {
    await ensureNativeBridge();
    return normalizeCommandResult<PathSelectionPayload>(normalizeNativePayload(await cfstNative.SelectPath(payload)));
  },

  async openPath(targetPath: string) {
    await ensureNativeBridge();
    await cfstNative.OpenPath({ targetPath });
  },

  async openLogDirectory(payload: Record<string, unknown>) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.OpenLogDirectory(payload)));
  },

  async openReleasePage() {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.OpenReleasePage()));
  },

  async consumeDroppedSources() {
    return commandResult<{ paths: string[] }>("SOURCES_DROPPED_EMPTY", { paths: [] });
  },
};
