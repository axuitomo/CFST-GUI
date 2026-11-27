import { Capacitor, registerPlugin, type PluginListenerHandle } from "@capacitor/core";

export interface CapacitorCfstPlugin {
  Invoke: (payload: { command: string; payload_json: string }) => Promise<unknown>;
  CheckBatteryOptimization?: (payload?: Record<string, unknown>) => Promise<unknown>;
  CheckForUpdates: (payload: Record<string, unknown>) => Promise<unknown>;
  CheckKeepAliveStatus?: (payload?: Record<string, unknown>) => Promise<unknown>;
  CheckNotificationPermission?: (payload?: Record<string, unknown>) => Promise<unknown>;
  DownloadAndInstallUpdate: (payload: Record<string, unknown>) => Promise<unknown>;
  GetAppInfo: () => Promise<unknown>;
  GetAndroidRuntimeStatus?: () => Promise<unknown>;
  Init: (payload?: Record<string, unknown>) => Promise<unknown>;
  SetKeepAliveEnabled?: (payload: { enabled: boolean }) => Promise<unknown>;
  SetSurfaceTheme?: (payload: { dark: boolean }) => Promise<unknown>;
  OpenLogDirectory: (payload: Record<string, unknown>) => Promise<unknown>;
  OpenPath: (payload: { targetPath: string }) => Promise<unknown>;
  OpenBatteryOptimizationSettings?: (payload?: Record<string, unknown>) => Promise<unknown>;
  RequestNotificationPermission?: (payload?: Record<string, unknown>) => Promise<unknown>;
  OpenNotificationSettings?: (payload?: Record<string, unknown>) => Promise<unknown>;
  ShowNotification?: (payload: { title: string; body: string; sound?: boolean }) => Promise<unknown>;
  OpenReleasePage: () => Promise<unknown>;
  SelectPath: (payload: Record<string, unknown>) => Promise<unknown>;
  addListener: (eventName: "probe:event", listenerFunc: (event: unknown) => void) => Promise<PluginListenerHandle> & PluginListenerHandle;
}

export const cfstNative = registerPlugin<CapacitorCfstPlugin>("Cfst");

export function isAndroidNativePlatform() {
  return Capacitor.getPlatform() === "android";
}
