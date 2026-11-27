import { androidAdapter } from "./android/adapter";
import { isAndroidNativePlatform } from "./android/plugin";
import { desktopAdapter } from "./desktop/adapter";
import { isWailsDesktopHost, isWailsRuntimeAvailable, waitForWailsRuntime } from "./desktop/runtime";
import type { BridgeMode, PlatformAdapter } from "./types";
import { webUIAdapter } from "./webui/adapter";

// WAILS_RUNTIME_WAIT_MS 是等待 Wails 宿主注入运行时的上限：桌面宿主在 navigationCompleted
// 之后立刻 execJS（wails v3 internal/runtime/runtime.go 的 runtimeConfigReady），实测远低于
// 1s，这里只为「宿主比页面脚本晚注入」这一必然发生的竞态留出余量。
const WAILS_RUNTIME_WAIT_MS = 3000;

// bridgeFallback 是 resolveBridgeMode 在挂载前定下来的兜底通道（非宿主页面的情况）。
let bridgeFallback: "wails" | "webui" | null = null;

const adapters: Record<BridgeMode, PlatformAdapter> = {
  native: androidAdapter,
  wails: desktopAdapter,
  webui: webUIAdapter,
};

// currentBridgeMode 是唯一的通道判定入口：宿主能自己表明身份时（Capacitor 原生壳、
// Wails 运行时已注入、或页面由 Wails 资源服务托管）一律以宿主为准；宿主还没就绪时，
// 才用 resolveBridgeMode 事先定下来的兜底结论。
export function currentBridgeMode(): BridgeMode {
  if (isAndroidNativePlatform()) {
    return "native";
  }
  if (isWailsRuntimeAvailable() || isWailsDesktopHost()) {
    return "wails";
  }
  return bridgeFallback ?? "webui";
}

export function getPlatformAdapter(mode: BridgeMode = currentBridgeMode()): PlatformAdapter {
  return adapters[mode];
}

export function getDesktopWindowControls() {
  return getPlatformAdapter().desktop?.windowControls;
}

export function onBridgeRuntimeReady(handler: () => void): () => void {
  if (isAndroidNativePlatform()) {
    return () => {};
  }
  return adapters.wails.desktop?.onRuntimeReady(handler) ?? (() => {});
}

export function subscribeDroppedSources(handler: () => void): () => void {
  return getPlatformAdapter().desktop?.subscribeDroppedSources(handler) ?? (() => {});
}

export function onPlatformSessionRefresh(handler: (() => Promise<void>) | null) {
  for (const adapter of Object.values(adapters)) {
    adapter.setSessionRefreshHandler?.(handler);
  }
}

// resolveBridgeMode 在挂载前把通道定下来，只按「宿主身份」这一件确定的事判断：
//
//   1. Capacitor 原生壳 → native
//   2. Wails 运行时已注入 → wails
//   3. 页面由 Wails 资源服务托管（wails.localhost / wails: 协议）→ 等运行时注入后走 wails
//   4. 其余页面 → webui
//
// 第 3 步的等待必须存在：Wails 桌面宿主的运行时在 navigationCompleted 之后才 execJS 注入
// （wails v3 internal/runtime/runtime.go 的 runtimeConfigReady），页面刚起来时
// isWailsRuntimeAvailable() 必然是 false。
export async function resolveBridgeMode(waitMs = WAILS_RUNTIME_WAIT_MS): Promise<BridgeMode> {
  if (isAndroidNativePlatform()) {
    return "native";
  }
  if (isWailsRuntimeAvailable()) {
    return "wails";
  }
  if (isWailsDesktopHost()) {
    await waitForWailsRuntime(waitMs);
    return currentBridgeMode();
  }
  bridgeFallback = "webui";
  return bridgeFallback;
}
