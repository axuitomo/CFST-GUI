import { getPlatformAdapter, onPlatformSessionRefresh } from "../adapters/registry";
import type { BridgeMode } from "../adapters/types";
import type { ProbeEventEnvelope } from "../bridge/types";

const probeListeners = new Set<(event: ProbeEventEnvelope) => void>();
let disposeRuntimeProbeListener: (() => void) | null = null;
let runtimeProbeMode: BridgeMode | null = null;
let runtimeProbeSubscriptionKey = "";
let runtimeProbeBindPromise: Promise<void> | null = null;

function buildIdempotentDisposer(dispose: () => void) {
  let disposed = false;
  return () => {
    if (disposed) {
      return;
    }
    disposed = true;
    dispose();
  };
}

function clearProbeRuntimeListener() {
  runtimeProbeMode = null;
  runtimeProbeSubscriptionKey = "";
  if (!disposeRuntimeProbeListener) {
    return;
  }
  const dispose = disposeRuntimeProbeListener;
  disposeRuntimeProbeListener = null;
  dispose();
}

function emitProbeEvent(event: ProbeEventEnvelope) {
  probeListeners.forEach((listener) => listener(event));
}

// 绑定（或校正）底层适配器事件通道。通道选择依赖宿主身份（如桌面运行时延迟注入），
// 仅当通道或适配器订阅键（如 WebUI 访问令牌）发生变化时重绑，业务侧监听器集合保持不变。
function bindRuntimeProbeListener(): Promise<void> {
  if (runtimeProbeBindPromise) {
    return runtimeProbeBindPromise.then(() => bindRuntimeProbeListener());
  }
  runtimeProbeBindPromise = (async () => {
    const adapter = getPlatformAdapter();
    const mode = adapter.mode;
    const subscriptionKey = adapter.getEventSubscriptionKey ? await adapter.getEventSubscriptionKey() : "";
    if (probeListeners.size === 0) {
      clearProbeRuntimeListener();
      return;
    }
    if (disposeRuntimeProbeListener && runtimeProbeMode === mode && runtimeProbeSubscriptionKey === subscriptionKey) {
      return;
    }
    clearProbeRuntimeListener();

    const unsubscribe = await adapter.subscribeProbeEvents({
      onEvent: emitProbeEvent,
      onTransportDrift: () => {
        if (getPlatformAdapter().mode !== mode) {
          void bindRuntimeProbeListener();
        }
      },
    });
    disposeRuntimeProbeListener = buildIdempotentDisposer(unsubscribe);
    runtimeProbeMode = mode;
    runtimeProbeSubscriptionKey = subscriptionKey;
  })();

  const currentBind = runtimeProbeBindPromise;
  currentBind
    .catch(() => {
      // 绑定失败（如 WebUI 令牌被取消）时清理半绑定状态，允许后续调用重新绑定。
      clearProbeRuntimeListener();
    })
    .finally(() => {
      if (runtimeProbeBindPromise === currentBind) {
        runtimeProbeBindPromise = null;
      }
    });
  return currentBind;
}

onPlatformSessionRefresh(async () => {
  if (probeListeners.size > 0) {
    await bindRuntimeProbeListener();
  }
});

export async function listenToProbeEvents(handler: (event: ProbeEventEnvelope) => void) {
  probeListeners.add(handler);
  try {
    await bindRuntimeProbeListener();
  } catch (error) {
    probeListeners.delete(handler);
    throw error;
  }

  return () => {
    probeListeners.delete(handler);
    if (probeListeners.size === 0) {
      clearProbeRuntimeListener();
    }
  };
}

// 在桌面运行时延迟就绪后调用：若启动竞态使事件通道误绑到 WebUI SSE，则在此切回真实的桌面事件通道。
// 幂等，通道无变化时为空操作。
export function rebindProbeEventListener() {
  return bindRuntimeProbeListener();
}
