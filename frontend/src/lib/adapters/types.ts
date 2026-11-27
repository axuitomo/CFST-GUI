import type { CommandResult, PathSelectionPayload, ProbeEventEnvelope } from "../bridge/types";

export type BridgeMode = "native" | "wails" | "webui";
export type PlatformMethodName = "GetAppInfo" | "CheckForUpdates" | "DownloadAndInstallUpdate";

export interface ProbeEventSubscriptionOptions {
  onEvent: (event: ProbeEventEnvelope) => void;
  onTransportDrift?: () => void;
}

export interface WindowControls {
  isAvailable: () => boolean;
  center: () => void;
  getSize: () => Promise<{ h: number; w: number }>;
  isMaximised: () => Promise<boolean>;
  maximise: () => void;
  minimise: () => void;
  quit: () => void;
  setSize: (width: number, height: number) => void;
  toggleMaximise: () => Promise<unknown>;
  unfullscreen: () => void;
  unmaximise: () => void;
}

export interface DesktopPlatformCapabilities {
  readonly windowControls: WindowControls;
  subscribeDroppedSources(handler: () => void): () => void;
  onRuntimeReady(handler: () => void): () => void;
}

export interface AndroidPlatformCapabilities {
  invokeCommand<T = unknown>(method: string, args: unknown[] | null, code: string, unsupportedMessage: string): Promise<CommandResult<T | null>>;
  setSurfaceTheme(dark: boolean): Promise<void>;
  showNotification(title: string, body: string): Promise<CommandResult<unknown>>;
}

export interface WebUIPlatformCapabilities {
  getEventSubscriptionKey(): Promise<string>;
  downloadExportedFile(fileName: string, content: string, encoding: "utf8" | "base64", mimeType: string): void;
}

export interface PlatformAdapter {
  readonly mode: BridgeMode;
  invokeCore<T = unknown>(command: string, payloadJSON: string): Promise<CommandResult<T>>;
  invokePlatform<T = unknown>(method: PlatformMethodName, payload?: Record<string, unknown>): Promise<CommandResult<T>>;
  subscribeProbeEvents(options: ProbeEventSubscriptionOptions): Promise<() => void>;
  selectPath(payload: Record<string, unknown>): Promise<CommandResult<PathSelectionPayload>>;
  openPath(targetPath: string): Promise<void>;
  openLogDirectory(payload: Record<string, unknown>): Promise<CommandResult<unknown>>;
  openReleasePage(): Promise<CommandResult<unknown>>;
  consumeDroppedSources?(): Promise<CommandResult<{ paths: string[] }>>;
  readonly desktop?: DesktopPlatformCapabilities;
  readonly android?: AndroidPlatformCapabilities;
  readonly webui?: WebUIPlatformCapabilities;
}
