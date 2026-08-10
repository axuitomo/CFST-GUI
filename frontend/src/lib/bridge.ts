import { EventsOn } from "../../wailsjs/runtime/runtime";
import { Capacitor, registerPlugin, type PluginListenerHandle } from "@capacitor/core";
import { isObject, toInteger, toNumber, toObjectRecord, toOptionalInteger, toOptionalNumber, toStringValue, toUnknownArray } from "./bridgeValues";
import { commandResult, normalizeCommandResult, SCHEMA_VERSION } from "./bridge/command";
import { normalizeSourceProfileUpdatePayload } from "./bridge/config";

import type {
  TaskTone,
  CommandResult,
  SourcePreviewPayload,
  ColoDictionaryStatus,
  PathSelectionPayload,
  AndroidBatteryStatus,
  AndroidKeepAliveStatus,
  AndroidNotificationPermissionStatus,
  AndroidRuntimeStatus,
  AppInfo,
  UpdateInfo,
  UpdateInstallResult,
  SourceProfileStore,
  SourceProfileUpdatePayload,
  ProbeEventEnvelope,
  DnsRecordSnapshot,
  DerivedTaskState,
  TaskSnapshot,
  TaskSnapshotList,
  TaskResultPage,
  ProbeResult,
  SchedulerStatus,
  ProbeRunResultPayload,
  ProbeResultFilter,
  ProbeResultIPFilter,
  ProbeResultOrder,
  ProbeResultSortBy,
} from "./bridge/types";
export type {
  TaskTone,
  CommandResult,
  ProbeNumericTriple,
  ProbeStageLimits,
  ProbeTimeouts,
  ProbeThresholds,
  ProbeStrategy,
  SourceColoFilterPhase,
  ColoFilterMode,
  TraceColoMode,
  DebugLogMode,
  DebugLogVerbosity,
  DownloadHTTPProtocol,
  DownloadSpeedMetric,
  CSVEncoding,
  SourceKind,
  SourceIPMode,
  ThemeMode,
  SourceConfig,
  SourcePreviewSummary,
  SourcePreviewPayload,
  ColoDictionaryStatus,
  PathSelectionPayload,
  StorageHealth,
  StorageStatus,
  AndroidBatteryStatus,
  AndroidKeepAliveStatus,
  AndroidNotificationPermissionStatus,
  AndroidRuntimeStatus,
  TraceDiagnosticSample,
  TraceDiagnostics,
  AppInfo,
  UpdateInfo,
  UpdateInstallResult,
  SourceProfileItem,
  SourceProfileStore,
  SourceProfileUpdatePayload,
  CloudflareRoutingRuleSnapshot,
  GitHubConfigSnapshot,
  ConfigSnapshot,
  TelegramRecipientMode,
  UploadNotification,
  UploadNotificationTopEntry,
  ProbeEventEnvelope,
  DnsRecordSnapshot,
  DerivedTaskState,
  TaskProgress,
  ExportRecord,
  TaskSnapshot,
  TaskSnapshotList,
  TaskResultPage,
  ProbeResult,
  SchedulerStatus,
  ProbeRunResultPayload,
  ProbeResultFilter,
  ProbeResultIPFilter,
  ProbeResultOrder,
  ProbeResultSortBy,
} from "./bridge/types";

export { normalizeCommandResult, SCHEMA_VERSION } from "./bridge/command";
export { isMaskedTokenValue, normalizeColoFilterMode, normalizeConfigSnapshot, normalizeSourceColoFilterPhase, normalizeSourceProfileStore, normalizeTraceColoMode } from "./bridge/config";

const PROBE_ALREADY_RUNNING_MESSAGE = "当前已有探测任务运行或暂停，请完成后再启动新任务。";

const TRACE_REASON_LABELS = {
  colo_filter: "地区码不匹配",
  rate_limited: "服务端限流",
  request_create_failed: "追踪请求创建失败",
  source_colo_filter: "输入源 COLO 过滤未通过",
  status_mismatch: "状态码不匹配",
  trace_error: "追踪请求失败",
  trace_latency_limit: "追踪延迟超阈值",
  trace_read_error: "追踪响应读取失败",
} as const satisfies Record<string, string>;

const STAGE_LABELS = {
  stage0_pool: "IP池",
  stage1_tcp: "第一阶段",
  stage2_head: "第二阶段",
  stage2_trace: "第二阶段",
  stage3_get: "第三阶段",
  post_probe_push: "自动推送",
} as const satisfies Record<string, string>;

function traceReasonLabel(reason: string) {
  const normalized = reason.trim().toLowerCase();
  return TRACE_REASON_LABELS[normalized as keyof typeof TRACE_REASON_LABELS] || reason.trim() || "未知原因";
}

export function summarizeTraceDiagnostics(value: unknown) {
  const diagnostics = toObjectRecord(value);
  const reasonCounts = toObjectRecord(diagnostics.reason_counts);
  const statusCounts = toObjectRecord(diagnostics.status_counts);
  const samples = toUnknownArray(diagnostics.samples);

  let topReason = "";
  let topReasonCount = 0;
  for (const [reason, rawCount] of Object.entries(reasonCounts)) {
    const count = toInteger(rawCount, 0);
    if (count > topReasonCount) {
      topReason = reason;
      topReasonCount = count;
    }
  }

  const parts: string[] = [];
  if (topReason) {
    parts.push(`${traceReasonLabel(topReason)} ${topReasonCount} 次`);
  }

  const statusEntries = Object.entries(statusCounts)
    .map(([code, rawCount]) => [code, toInteger(rawCount, 0)] as const)
    .filter(([, count]) => count > 0)
    .sort((left, right) => right[1] - left[1]);
  if (statusEntries.length > 0) {
    const [statusCode, count] = statusEntries[0];
    parts.push(`HTTP ${statusCode} ${count} 次`);
  }

  if (samples.length > 0) {
    const sample = toObjectRecord(samples[0]);
    const error = toStringValue(sample.error);
    const ip = toStringValue(sample.ip);
    const url = toStringValue(sample.url);
    if (error) {
      parts.push(error);
    } else if (ip || url) {
      parts.push([ip, url].filter(Boolean).join(" · "));
    }
  }

  return parts.join("；");
}

function stageLabel(stage: string) {
  return STAGE_LABELS[stage as keyof typeof STAGE_LABELS] || stage || "running";
}

export function normalizeProbeEvent(input: unknown): ProbeEventEnvelope | null {
  if (!isObject(input)) {
    return null;
  }

  return {
    event: toStringValue(input.event),
    payload: toObjectRecord(input.payload),
    schema_version: toStringValue(input.schema_version) || SCHEMA_VERSION,
    seq: toInteger(input.seq, 0),
    task_id: toStringValue(input.task_id),
    ts: toStringValue(input.ts) || new Date().toISOString(),
  };
}

export function normalizeDnsRecord(input: unknown): DnsRecordSnapshot {
  const source = toObjectRecord(input);

  return {
    comment: toStringValue(source.comment),
    content: toStringValue(source.content),
    created_on: toStringValue(source.created_on),
    id: toStringValue(source.id),
    modified_on: toStringValue(source.modified_on),
    name: toStringValue(source.name),
    proxied: Boolean(source.proxied),
    ttl: toInteger(source.ttl, 1),
    type: toStringValue(source.type) || "A",
  };
}

export function normalizeDnsRecords(input: unknown): DnsRecordSnapshot[] {
  return toUnknownArray(input).map((entry) => normalizeDnsRecord(entry));
}

export function deriveTaskStateFromProbeEvent(event: ProbeEventEnvelope): DerivedTaskState {
  if (event.event === "probe.preprocessed") {
    const accepted = toInteger(event.payload.accepted, 0);
    const filtered = toInteger(event.payload.filtered, 0);
    const invalid = toInteger(event.payload.invalid, 0);
    const total = toInteger(event.payload.total, accepted + filtered + invalid);

    return {
      detail: `候选 ${total} 条，接受 ${accepted} 条，过滤 ${filtered} 条，非法 ${invalid} 条。`,
      title: accepted > 0 ? "IP池已完成" : "IP池没有可用结果",
      tone: accepted > 0 ? "preparing" : "no_results",
    };
  }

  if (event.event === "probe.progress") {
    const stage = toStringValue(event.payload.stage) || "running";
    const processed = toInteger(event.payload.processed, 0);
    const passed = toInteger(event.payload.passed, 0);
    const failed = toInteger(event.payload.failed, 0);
    const prefix = stage === "stage3_get" ? "文件测速" : stageLabel(stage);

    return {
      detail: `${prefix}，已处理 ${processed}，通过 ${passed}，失败 ${failed}。`,
      title: `${stageLabel(stage)}进行中`,
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.resumed") {
    const stage = toStringValue(event.payload.stage ?? event.payload.current_stage) || "running";
    return {
      detail: toStringValue(event.payload.message) || "任务已恢复执行。",
      title: `${stageLabel(stage)}继续中`,
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.speed") {
    const ip = toStringValue(event.payload.ip);
    const colo = toStringValue(event.payload.colo);

    return {
      detail: `${ip || "当前 IP"}${colo ? `(${colo})` : ""} 正在测速中`,
      title: "文件测速实时速度",
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.partial_export") {
    const written = toInteger(event.payload.written, 0);
    const targetPath = toStringValue(event.payload.target_path);

    return {
      detail: targetPath ? `已导出 ${written} 条结果到 ${targetPath}。` : `已导出 ${written} 条结果。`,
      title: "结果已落盘",
      tone: "running" as TaskTone,
    };
  }

  if (event.event === "probe.export_completed") {
    const written = toInteger(event.payload.written, 0);
    const targetPath = toStringValue(event.payload.target_path);

    return {
      detail: targetPath ? `Android 系统导出已写入 ${written} 条结果到 ${targetPath}。` : `Android 系统导出已写入 ${written} 条结果。`,
      title: "系统导出完成",
      tone: "completed" as TaskTone,
    };
  }

  if (event.event === "probe.export_failed") {
    const message = toStringValue(event.payload.message) || "Android 系统导出失败。";

    return {
      detail: message,
      title: "系统导出失败",
      tone: "warning" as TaskTone,
    };
  }

  if (event.event === "probe.cooling") {
    const reason = toStringValue(event.payload.reason) || "冷却中";

    return {
      detail: `${reason}${event.payload.recoverable ? " 可以点击继续任务。" : ""}`,
      title: "任务进入冷却",
      tone: "cooling" as TaskTone,
    };
  }

  if (event.event === "probe.completed") {
    const resultCount = Math.max(toInteger(event.payload.result_count, 0), toInteger(event.payload.passed, 0), toInteger(event.payload.exported, 0));
    const exported = toInteger(event.payload.exported, 0);
    const targetPath = toStringValue(event.payload.target_path);
    const hasResults = resultCount > 0;
    const traceSummary = summarizeTraceDiagnostics(event.payload.trace_diagnostics);
    const traceStageFailure = toStringValue(event.payload.failure_stage) === "stage2_trace" && traceSummary;

    return {
      detail: hasResults
        ? exported > 0
          ? targetPath
            ? `任务完成，可用结果 ${resultCount} 条，已导出 ${exported} 条到 ${targetPath}。`
            : `任务完成，可用结果 ${resultCount} 条，已导出 ${exported} 条。`
          : `任务完成，可用结果 ${resultCount} 条。`
        : traceStageFailure
          ? `追踪阶段未找到可用结果：${traceSummary}`
          : "任务已完成，但当前没有可用结果。",
      title: hasResults ? "任务完成" : traceStageFailure ? "追踪阶段无可用结果" : "没有可用结果",
      tone: hasResults ? ("completed" as TaskTone) : ("no_results" as TaskTone),
    };
  }

  if (event.event === "probe.cancelled") {
    return {
      detail: toStringValue(event.payload.message) || "测速任务已终止。",
      title: "任务已终止",
      tone: "cancelled" as TaskTone,
    };
  }

  if (event.event === "probe.failed") {
    const traceSummary = summarizeTraceDiagnostics(event.payload.trace_diagnostics);
    const message = toStringValue(event.payload.failure_stage) === "stage2_trace" && traceSummary ? `追踪阶段失败：${traceSummary}` : toStringValue(event.payload.message) || "任务失败。";

    return {
      detail: event.payload.recoverable ? `${message} 可以尝试继续或重试。` : message,
      title: "任务失败",
      tone: "failed" as TaskTone,
    };
  }

  return {
    detail: event.event,
    title: "收到新事件",
    tone: "running" as TaskTone,
  };
}

interface WailsAppBridge {
	Invoke: (command: string, payloadJSON: string) => Promise<string>;
  CheckForUpdates: (payload: Record<string, unknown>) => Promise<unknown>;
  DownloadAndInstallUpdate: (payload: Record<string, unknown>) => Promise<unknown>;
  GetAppInfo: () => Promise<unknown>;
  OpenLogDirectory: (payload: Record<string, unknown>) => Promise<unknown>;
  OpenPath: (targetPath: string) => Promise<void>;
  OpenReleasePage: () => Promise<unknown>;
  SelectPath: (payload: Record<string, unknown>) => Promise<unknown>;
}

interface NativeJSONResult {
  value?: string;
}

interface CapacitorCfstPlugin {
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
  OpenLogDirectory: (payload: Record<string, unknown>) => Promise<unknown>;
  OpenPath: (payload: { targetPath: string }) => Promise<unknown>;
  OpenBatteryOptimizationSettings?: (payload?: Record<string, unknown>) => Promise<unknown>;
  RequestNotificationPermission?: (payload?: Record<string, unknown>) => Promise<unknown>;
  OpenNotificationSettings?: (payload?: Record<string, unknown>) => Promise<unknown>;
  OpenReleasePage: () => Promise<unknown>;
  SelectPath: (payload: Record<string, unknown>) => Promise<unknown>;
  addListener: (eventName: "probe:event", listenerFunc: (event: unknown) => void) => Promise<PluginListenerHandle> & PluginListenerHandle;
}

declare global {
  interface Window {
    go?: {
      app?: {
        App?: WailsAppBridge;
      };
      main?: {
        App?: WailsAppBridge;
      };
    };
  }
}

const probeListeners = new Set<(event: ProbeEventEnvelope) => void>();
const cfstNative = registerPlugin<CapacitorCfstPlugin>("Cfst");
let disposeRuntimeProbeListener: (() => void) | null = null;
let nativeInitPromise: Promise<void> | null = null;
let webUIAuthRequiredPromise: Promise<boolean> | null = null;

const WEBUI_TOKEN_STORAGE_KEY = "cfst-webui-token";

function wailsBridge() {
  return window.go?.app?.App ?? window.go?.main?.App;
}

function appBridge() {
  const bridge = wailsBridge();

  if (!bridge) {
    throw new Error("Wails bridge is not ready.");
  }

  return bridge;
}

function shouldUseNativeBridge() {
  return !wailsBridge() && Capacitor.isNativePlatform() && Capacitor.getPlatform() === "android";
}

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
  if (!disposeRuntimeProbeListener) {
    return;
  }
  const dispose = disposeRuntimeProbeListener;
  disposeRuntimeProbeListener = null;
  dispose();
}

export function clearTaskWorkspaceCache(taskId = "") {
  void taskId;
}

async function ensureNativeBridge() {
  if (!shouldUseNativeBridge()) {
    return;
  }
  if (!nativeInitPromise) {
    nativeInitPromise = cfstNative.Init({}).then(() => undefined);
  }
  await nativeInitPromise;
}

function shouldUseWebUIBridge() {
  return !wailsBridge() && !shouldUseNativeBridge();
}

async function webUIAuthRequired() {
  if (!webUIAuthRequiredPromise) {
    webUIAuthRequiredPromise = fetch("/api/health", { cache: "no-store" })
      .then((response) => response.json())
      .then((payload) => Boolean(isObject(payload) && payload.auth_required))
      .catch(() => false);
  }
  return webUIAuthRequiredPromise;
}

async function ensureWebUIToken() {
  if (!(await webUIAuthRequired())) {
    return "";
  }
  let token = localStorage.getItem(WEBUI_TOKEN_STORAGE_KEY) || "";
  while (!token.trim()) {
    token = window.prompt("请输入 CFST WebUI 访问令牌") || "";
    if (!token.trim()) {
      throw new Error("缺少 WebUI 访问令牌。");
    }
  }
  localStorage.setItem(WEBUI_TOKEN_STORAGE_KEY, token.trim());
  return token.trim();
}

async function webUIFetch(path: string, init: RequestInit = {}, retry = true) {
  const headers = new Headers(init.headers || {});
  if (!headers.has("Content-Type") && init.body) {
    headers.set("Content-Type", "application/json");
  }
  const token = await ensureWebUIToken();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  const response = await fetch(path, {
    ...init,
    headers,
  });
  if (response.status === 401 && retry) {
    localStorage.removeItem(WEBUI_TOKEN_STORAGE_KEY);
    await ensureWebUIToken();
    return webUIFetch(path, init, false);
  }
  if (!response.ok) {
    let message = `WebUI 请求失败 (${response.status})`;
    try {
      const body = await response.json();
      if (isObject(body) && body.message) {
        message = toStringValue(body.message);
      }
    } catch {
      // Keep the status-based message when the response is not JSON.
    }
    throw new Error(message);
  }
  return response;
}

async function webUIApp<T = unknown>(method: string, payload: Record<string, unknown> = {}) {
  const response = await webUIFetch(`/api/platform/${encodeURIComponent(method)}`, {
    body: JSON.stringify(payload),
    method: "POST",
  });
  return (await response.json()) as T;
}

async function invokeCore<T = unknown>(command: string, payload: Record<string, unknown> = {}) {
  const payloadJSON = JSON.stringify(payload ?? {});
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<T>(
      normalizeNativePayload(
        await cfstNative.Invoke({
          command,
          payload_json: payloadJSON,
        }),
      ),
    );
  }
  if (shouldUseWebUIBridge()) {
    const response = await webUIFetch(`/api/command/${encodeURIComponent(command)}`, {
      body: payloadJSON,
      method: "POST",
    });
    return normalizeCommandResult<T>(await response.json());
  }
  return normalizeCommandResult<T>(normalizeNativePayload(await appBridge().Invoke(command, payloadJSON)));
}

function webUITokenQuery(token: string) {
  return token ? `?token=${encodeURIComponent(token)}` : "";
}

function arrayBufferToBase64(buffer: ArrayBuffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  const chunkSize = 0x8000;
  for (let index = 0; index < bytes.length; index += chunkSize) {
    const chunk = bytes.subarray(index, index + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
}

function downloadBase64File(fileName: string, contentBase64: string, mimeType = "application/octet-stream") {
  const link = document.createElement("a");
  link.href = `data:${mimeType};base64,${contentBase64}`;
  link.download = fileName || "download";
  document.body.appendChild(link);
  link.click();
  link.remove();
}

function downloadBlobFile(fileName: string, content: string, mimeType = "application/octet-stream") {
  const link = document.createElement("a");
  link.href = URL.createObjectURL(new Blob([content], { type: mimeType }));
  link.download = fileName || "download";
  document.body.appendChild(link);
  link.click();
  URL.revokeObjectURL(link.href);
  link.remove();
}

async function selectBrowserFile(mode: string): Promise<CommandResult<PathSelectionPayload>> {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = mode === "config_archive_import" ? ".zip,.json,application/zip,application/json" : ".txt,.csv,text/plain,text/csv,*/*";
  const file = await new Promise<File | null>((resolve) => {
    input.onchange = () => resolve(input.files?.[0] || null);
    input.click();
  });
  if (!file) {
    return commandResult("PATH_SELECTION_CANCELED", { canceled: true, mode }, { message: "已取消选择文件。" });
  }
  if (mode === "config_archive_import") {
    return commandResult(
      "PATH_SELECTED",
      {
        canceled: false,
        content_base64: arrayBufferToBase64(await file.arrayBuffer()),
        display_name: file.name,
        file_name: file.name,
        mode,
        path: `browser-upload:${file.name}`,
      },
      { message: "已选择配置压缩包。" },
    );
  }
  return commandResult(
    "PATH_SELECTED",
    {
      canceled: false,
      content: await file.text(),
      display_name: file.name,
      file_name: file.name,
      mode,
      path: `browser-upload:${file.name}`,
    },
    { message: "已选择输入源文件。" },
  );
}

async function fetchWebUIFileList(path: string) {
  const query = path ? `?path=${encodeURIComponent(path)}` : "";
  const response = await webUIFetch(`/api/files/list${query}`, { method: "GET" });
  return (await response.json()) as {
    entries: Array<{ is_dir: boolean; name: string; path: string; size: number }>;
    path: string;
    roots: string[];
  };
}

async function browseWebUIDirectory(startPath: string, title: string): Promise<string | null> {
  const overlay = document.createElement("div");
  overlay.style.cssText = "position:fixed;inset:0;z-index:9999;background:rgba(15,23,42,.48);display:flex;align-items:center;justify-content:center;padding:24px;";
  const panel = document.createElement("div");
  panel.style.cssText = "width:min(760px,100%);max-height:min(720px,90vh);background:#fff;border-radius:8px;box-shadow:0 24px 80px rgba(15,23,42,.28);display:flex;flex-direction:column;overflow:hidden;font:14px system-ui,sans-serif;";

  const header = document.createElement("div");
  header.style.cssText = "padding:16px 18px;border-bottom:1px solid #e2e8f0;font-weight:700;color:#0f172a;";
  header.textContent = title || "选择服务端目录";

  const pathRow = document.createElement("div");
  pathRow.style.cssText = "display:flex;gap:8px;padding:12px 18px;border-bottom:1px solid #e2e8f0;";
  const input = document.createElement("input");
  input.style.cssText = "flex:1;border:1px solid #cbd5e1;border-radius:6px;padding:8px 10px;font-family:monospace;";
  const goButton = document.createElement("button");
  goButton.style.cssText = "border:1px solid #0f172a;border-radius:6px;background:#0f172a;color:white;padding:8px 12px;";
  goButton.textContent = "打开";
  pathRow.append(input, goButton);

  const roots = document.createElement("div");
  roots.style.cssText = "display:flex;gap:8px;flex-wrap:wrap;padding:10px 18px;border-bottom:1px solid #e2e8f0;";

  const list = document.createElement("div");
  list.style.cssText = "min-height:280px;max-height:420px;overflow:auto;padding:8px 10px;";

  const actionRow = document.createElement("div");
  actionRow.style.cssText = "display:flex;justify-content:space-between;gap:8px;padding:14px 18px;border-top:1px solid #e2e8f0;";
  const parentButton = document.createElement("button");
  parentButton.style.cssText = "border:1px solid #cbd5e1;border-radius:6px;background:white;padding:8px 12px;";
  parentButton.textContent = "上一级";
  const spacer = document.createElement("span");
  spacer.style.cssText = "flex:1";
  const cancelButton = document.createElement("button");
  cancelButton.style.cssText = "border:1px solid #cbd5e1;border-radius:6px;background:white;padding:8px 12px;";
  cancelButton.textContent = "取消";
  const chooseButton = document.createElement("button");
  chooseButton.style.cssText = "border:1px solid #2563eb;border-radius:6px;background:#2563eb;color:white;padding:8px 12px;";
  chooseButton.textContent = "选择当前目录";
  actionRow.append(parentButton, spacer, cancelButton, chooseButton);

  panel.append(header, pathRow, roots, list, actionRow);
  overlay.appendChild(panel);
  document.body.appendChild(overlay);

  let currentPath = startPath || "";

  const render = async (path: string) => {
    list.textContent = "正在读取目录...";
    const data = await fetchWebUIFileList(path);
    currentPath = data.path;
    input.value = currentPath;
    roots.replaceChildren();
    data.roots.forEach((root) => {
      const button = document.createElement("button");
      button.textContent = root;
      button.style.cssText = "border:1px solid #cbd5e1;border-radius:999px;background:white;padding:5px 10px;font-family:monospace;font-size:12px;";
      button.onclick = () => void render(root);
      roots.appendChild(button);
    });
    list.replaceChildren();
    data.entries.forEach((entry) => {
      const row = document.createElement("button");
      row.type = "button";
      row.disabled = !entry.is_dir;
      row.textContent = `${entry.is_dir ? "[D]" : "[F]"} ${entry.name}`;
      row.style.cssText = `width:100%;display:block;text-align:left;border:0;border-radius:6px;background:${entry.is_dir ? "white" : "#f8fafc"};padding:9px 10px;color:${entry.is_dir ? "#0f172a" : "#64748b"};`;
      row.ondblclick = () => entry.is_dir && void render(entry.path);
      row.onclick = () => {
        if (entry.is_dir) {
          input.value = entry.path;
        }
      };
      list.appendChild(row);
    });
    if (data.entries.length === 0) {
      list.textContent = "目录为空。";
    }
  };

  return new Promise<string | null>((resolve) => {
    const close = (value: string | null) => {
      overlay.remove();
      resolve(value);
    };
    goButton.onclick = () => void render(input.value);
    parentButton.onclick = () => {
      const parent = currentPath.replace(/[\\/]+$/, "").replace(/[\\/][^\\/]*$/, "") || currentPath;
      void render(parent);
    };
    cancelButton.onclick = () => close(null);
    chooseButton.onclick = () => close(input.value || currentPath);
    void render(currentPath).catch((error) => {
      list.textContent = error instanceof Error ? error.message : "读取目录失败。";
    });
  });
}

async function selectWebUIPath(payload: Record<string, unknown>) {
  const mode = toStringValue(payload.mode ?? payload.kind).trim() || "source_file";
  const title = toStringValue(payload.title);
  const defaultFileName = toStringValue(payload.default_file_name ?? payload.defaultFileName).trim() || "result.csv";
  if (mode === "source_file" || mode === "config_archive_import" || mode === "config_import" || mode === "import_config") {
    return selectBrowserFile(mode === "config_archive_import" || mode === "config_import" || mode === "import_config" ? "config_archive_import" : mode);
  }
  if (mode === "config_archive_export" || mode === "config_export" || mode === "save_file") {
    return commandResult<PathSelectionPayload>(
      "PATH_SELECTED",
      {
        canceled: false,
        file_name: defaultFileName,
        mode,
        target_uri: `browser-download:${defaultFileName}`,
      },
      { message: "已选择浏览器下载。" },
    );
  }
  const selected = await browseWebUIDirectory(toStringValue(payload.current_path ?? payload.currentPath), title || "选择服务端目录");
  if (!selected) {
    return commandResult<PathSelectionPayload>("PATH_SELECTION_CANCELED", { canceled: true, mode }, { message: "已取消选择目录。" });
  }
  return commandResult<PathSelectionPayload>(
    "PATH_SELECTED",
    {
      canceled: false,
      directory: selected,
      mode,
      path: selected,
    },
    { message: "已选择服务端目录。" },
  );
}

async function openWebUIPath(targetPath: string) {
  if (/^https?:\/\//i.test(targetPath)) {
    window.open(targetPath, "_blank", "noopener,noreferrer");
    return;
  }
  try {
    await fetchWebUIFileList(targetPath);
    await browseWebUIDirectory(targetPath, "浏览服务端目录");
    return;
  } catch {
    const token = await ensureWebUIToken();
    window.open(`/api/files/download?path=${encodeURIComponent(targetPath)}${token ? `&token=${encodeURIComponent(token)}` : ""}`, "_blank", "noopener,noreferrer");
  }
}

function normalizeNativePayload(input: unknown): unknown {
  if (typeof input === "string") {
    try {
      return JSON.parse(input);
    } catch {
      return input;
    }
  }
  if (isObject(input)) {
    const value = (input as NativeJSONResult).value;
    if (typeof value === "string") {
      try {
        return JSON.parse(value);
      } catch {
        return value;
      }
    }
  }
  return input;
}

function nextTaskId() {
  return `cfst-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
}

function emitProbeEvent(event: ProbeEventEnvelope) {
  probeListeners.forEach((listener) => listener(event));
}

function probeStartFailureCode(message: string, code = "") {
  if (code === "PROBE_ALREADY_RUNNING" || message.includes(PROBE_ALREADY_RUNNING_MESSAGE)) {
    return "PROBE_ALREADY_RUNNING";
  }
  return "PROBE_FAILED";
}

function normalizeProbeRows(rows: unknown): ProbeResult[] {
  return toUnknownArray(rows).map((row) => {
    const source = toObjectRecord(row);
    const delayMs = toNumber(source.delayMs ?? source.delay_ms ?? source.tcp_latency_ms ?? source.tcpLatencyMs, 0);
    const traceDelayMs = toNumber(source.traceDelayMs ?? source.trace_delay_ms ?? source.trace_latency_ms ?? source.traceLatencyMs, 0);
    const downloadMbps = toOptionalNumber(source.downloadSpeedMb ?? source.download_mbps);
    const maxDownloadMbps = toOptionalNumber(source.maxDownloadSpeedMb ?? source.max_download_speed_mb ?? source.max_download_mbps ?? source.maxDownloadMbps);
    const normalizedDownloadMbps = downloadMbps !== null && downloadMbps >= 0 ? downloadMbps : null;
    const normalizedMaxDownloadMbps = maxDownloadMbps !== null && maxDownloadMbps >= 0 ? maxDownloadMbps : normalizedDownloadMbps;
    const sourcePort = toOptionalInteger(source.source_port ?? source.sourcePort);
    const testPort = toOptionalInteger(source.test_port ?? source.testPort);

    return {
      address: toStringValue(source.ip ?? source.address),
      colo: toStringValue(source.colo) || null,
      download_mbps: normalizedDownloadMbps,
      export_status: toStringValue(source.export_status ?? source.exportStatus) || "exported",
      last_error_code: toStringValue(source.last_error_code ?? source.lastErrorCode) || null,
      max_download_mbps: normalizedMaxDownloadMbps,
      source_port: sourcePort !== null && sourcePort > 0 ? sourcePort : null,
      stage_status: toStringValue(source.stage_status ?? source.stageStatus) || "completed",
      tcp_latency_ms: delayMs > 0 ? delayMs : null,
      test_port: testPort !== null && testPort > 0 ? testPort : null,
      trace_latency_ms: traceDelayMs > 0 ? traceDelayMs : null,
    };
  });
}

export async function loadConfig() {
  return invokeCore("config.load");
}

export async function saveDraft(payload: Record<string, unknown>) {
  return invokeCore("draft.save", payload);
}

export async function discardDraft(payload: Record<string, unknown> = {}) {
  return invokeCore("draft.discard", payload);
}

export async function getAppInfo() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<AppInfo>(normalizeNativePayload(await cfstNative.GetAppInfo()));
  }
  if (shouldUseWebUIBridge()) {
    return normalizeCommandResult<AppInfo>(await webUIApp("GetAppInfo"));
  }
  return normalizeCommandResult<AppInfo>(await appBridge().GetAppInfo());
}

export async function getAndroidRuntimeStatus() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    if (typeof cfstNative.GetAndroidRuntimeStatus === "function") {
      return normalizeCommandResult<AndroidRuntimeStatus>(normalizeNativePayload(await cfstNative.GetAndroidRuntimeStatus()));
    }
    return commandResult<AndroidRuntimeStatus | null>("ANDROID_RUNTIME_UNSUPPORTED", null, {
      message: "当前环境不支持 Android 运行时状态查询。",
      ok: false,
    });
  }
  return commandResult<AndroidRuntimeStatus | null>("ANDROID_RUNTIME_UNSUPPORTED", null, {
    message: "当前不是 Android 原生运行环境。",
    ok: false,
  });
}

export async function checkBatteryOptimization() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    if (typeof cfstNative.CheckBatteryOptimization === "function") {
      return normalizeCommandResult<AndroidBatteryStatus>(normalizeNativePayload(await cfstNative.CheckBatteryOptimization({})));
    }
    return commandResult<AndroidBatteryStatus | null>("ANDROID_BATTERY_UNSUPPORTED", null, {
      message: "当前环境不支持省电策略检测。",
      ok: false,
    });
  }
  return commandResult<AndroidBatteryStatus | null>("ANDROID_BATTERY_UNSUPPORTED", null, {
    message: "当前不是 Android 原生运行环境。",
    ok: false,
  });
}

export async function checkKeepAliveStatus() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    if (typeof cfstNative.CheckKeepAliveStatus === "function") {
      return normalizeCommandResult<AndroidKeepAliveStatus>(normalizeNativePayload(await cfstNative.CheckKeepAliveStatus({})));
    }
    return commandResult<AndroidKeepAliveStatus | null>("ANDROID_KEEP_ALIVE_UNSUPPORTED", null, {
      message: "当前环境不支持通知栏保活状态检测。",
      ok: false,
    });
  }
  return commandResult<AndroidKeepAliveStatus | null>("ANDROID_KEEP_ALIVE_UNSUPPORTED", null, {
    message: "当前不是 Android 原生运行环境。",
    ok: false,
  });
}

export async function setKeepAliveEnabled(enabled: boolean) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    if (typeof cfstNative.SetKeepAliveEnabled === "function") {
      return normalizeCommandResult<AndroidKeepAliveStatus>(normalizeNativePayload(await cfstNative.SetKeepAliveEnabled({ enabled })));
    }
    return commandResult<AndroidKeepAliveStatus | null>("ANDROID_KEEP_ALIVE_UNSUPPORTED", null, {
      message: "当前环境不支持通知栏保活设置。",
      ok: false,
    });
  }
  return commandResult<AndroidKeepAliveStatus | null>("ANDROID_KEEP_ALIVE_UNSUPPORTED", null, {
    message: "当前不是 Android 原生运行环境。",
    ok: false,
  });
}

export async function openBatteryOptimizationSettings(mode = "request") {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    if (typeof cfstNative.OpenBatteryOptimizationSettings === "function") {
      return normalizeCommandResult<AndroidBatteryStatus>(normalizeNativePayload(await cfstNative.OpenBatteryOptimizationSettings({ mode })));
    }
    return commandResult<AndroidBatteryStatus | null>("ANDROID_BATTERY_UNSUPPORTED", null, {
      message: "当前环境不支持打开省电策略设置。",
      ok: false,
    });
  }
  return commandResult<AndroidBatteryStatus | null>("ANDROID_BATTERY_UNSUPPORTED", null, {
    message: "当前不是 Android 原生运行环境。",
    ok: false,
  });
}

export async function checkNotificationPermission() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    if (typeof cfstNative.CheckNotificationPermission === "function") {
      return normalizeCommandResult<AndroidNotificationPermissionStatus>(normalizeNativePayload(await cfstNative.CheckNotificationPermission({})));
    }
    return commandResult<AndroidNotificationPermissionStatus | null>("ANDROID_NOTIFICATION_UNSUPPORTED", null, {
      message: "当前环境不支持通知权限检测。",
      ok: false,
    });
  }
  return commandResult<AndroidNotificationPermissionStatus | null>("ANDROID_NOTIFICATION_UNSUPPORTED", null, {
    message: "当前不是 Android 原生运行环境。",
    ok: false,
  });
}

export async function requestNotificationPermission() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    if (typeof cfstNative.RequestNotificationPermission === "function") {
      return normalizeCommandResult<AndroidNotificationPermissionStatus>(normalizeNativePayload(await cfstNative.RequestNotificationPermission({})));
    }
    return commandResult<AndroidNotificationPermissionStatus | null>("ANDROID_NOTIFICATION_UNSUPPORTED", null, {
      message: "当前环境不支持申请通知权限。",
      ok: false,
    });
  }
  return commandResult<AndroidNotificationPermissionStatus | null>("ANDROID_NOTIFICATION_UNSUPPORTED", null, {
    message: "当前不是 Android 原生运行环境。",
    ok: false,
  });
}

export async function openNotificationSettings() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    if (typeof cfstNative.OpenNotificationSettings === "function") {
      return normalizeCommandResult<AndroidNotificationPermissionStatus>(normalizeNativePayload(await cfstNative.OpenNotificationSettings({})));
    }
    return commandResult<AndroidNotificationPermissionStatus | null>("ANDROID_NOTIFICATION_UNSUPPORTED", null, {
      message: "当前环境不支持打开通知权限设置。",
      ok: false,
    });
  }
  return commandResult<AndroidNotificationPermissionStatus | null>("ANDROID_NOTIFICATION_UNSUPPORTED", null, {
    message: "当前不是 Android 原生运行环境。",
    ok: false,
  });
}

export async function checkForUpdates(payload: Record<string, unknown> = {}) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<UpdateInfo>(normalizeNativePayload(await cfstNative.CheckForUpdates(payload)));
  }
  if (shouldUseWebUIBridge()) {
    return normalizeCommandResult<UpdateInfo>(await webUIApp("CheckForUpdates", payload));
  }
  return normalizeCommandResult<UpdateInfo>(await appBridge().CheckForUpdates(payload));
}

export async function downloadAndInstallUpdate(payload: Record<string, unknown> = {}) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<UpdateInstallResult>(normalizeNativePayload(await cfstNative.DownloadAndInstallUpdate(payload)));
  }
  if (shouldUseWebUIBridge()) {
    return normalizeCommandResult<UpdateInstallResult>(await webUIApp("DownloadAndInstallUpdate", payload));
  }
  return normalizeCommandResult<UpdateInstallResult>(await appBridge().DownloadAndInstallUpdate(payload));
}

export async function openReleasePage() {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.OpenReleasePage()));
  }
  if (shouldUseWebUIBridge()) {
    const result = normalizeCommandResult(await webUIApp("OpenReleasePage"));
    const releaseUrl = toStringValue(isObject(result.data) ? result.data.release_url : "");
    if (releaseUrl) {
      window.open(releaseUrl, "_blank", "noopener,noreferrer");
    }
    return result;
  }
  return normalizeCommandResult(await appBridge().OpenReleasePage());
}

export async function listDnsRecords(payload: Record<string, unknown>) {
  return invokeCore("cloudflare.list", payload);
}

export async function saveConfig(payload: Record<string, unknown>) {
  return invokeCore("config.save", payload);
}

export async function setStorageDirectory(payload: Record<string, unknown>) {
  return invokeCore("storage.set", payload);
}

export async function checkStorageHealth(payload: Record<string, unknown> = {}) {
  return invokeCore("storage.health", payload);
}

export async function exportConfig(payload: Record<string, unknown>) {
  const result = await invokeCore("config.export", payload);
  if (shouldUseWebUIBridge()) {
    const data = isObject(result.data) ? result.data : {};
    const content = toStringValue(data.content);
    const fileName = toStringValue(data.file_name ?? data.fileName) || "cfst-gui-config.json";
    if (content) {
      downloadBlobFile(fileName, content, "application/json");
    }
  }
  return result;
}

export async function exportConfigArchive(payload: Record<string, unknown>) {
  const result = await invokeCore("archive.export", payload);
  if (shouldUseWebUIBridge()) {
    const data = isObject(result.data) ? result.data : {};
    const contentBase64 = toStringValue(data.content_base64 ?? data.contentBase64);
    if (contentBase64) {
      downloadBase64File(toStringValue(data.file_name ?? data.fileName) || "cfst-gui-config.zip", contentBase64, "application/zip");
    }
  }
  return result;
}

export async function importConfigArchive(payload: Record<string, unknown>) {
  return invokeCore("archive.import", payload);
}

export async function testWebDAV(payload: Record<string, unknown>) {
  return invokeCore("webdav.test", payload);
}

export async function backupConfigToWebDAV(payload: Record<string, unknown>) {
  return invokeCore("webdav.backup", payload);
}

export async function restoreConfigFromWebDAV(payload: Record<string, unknown>) {
  return invokeCore("webdav.restore", payload);
}

export async function backupCurrentConfig(payload: Record<string, unknown>) {
  return invokeCore("config.backup", payload);
}

export async function loadSourceProfiles() {
  return invokeCore<SourceProfileStore>("source_profiles.load");
}

export async function saveSourceProfile(payload: Record<string, unknown>) {
  return invokeCore<SourceProfileStore>("source_profiles.save", payload);
}

export async function updateCurrentSourceProfile(payload: Record<string, unknown>) {
  const result = await invokeCore("source_profiles.update_current", payload);
  return {
    ...result,
    data: result.data ? normalizeSourceProfileUpdatePayload(result.data) : null,
  } as CommandResult<SourceProfileUpdatePayload | null>;
}

export async function saveSourceProfileStore(payload: Record<string, unknown>) {
  return invokeCore<SourceProfileStore>("source_profiles.save_store", payload);
}

export async function switchSourceProfile(payload: Record<string, unknown>) {
  return invokeCore("source_profiles.switch", payload);
}

export async function deleteSourceProfile(payload: Record<string, unknown>) {
  return invokeCore<SourceProfileStore>("source_profiles.delete", payload);
}

export async function selectPath(payload: Record<string, unknown>) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult<PathSelectionPayload>(normalizeNativePayload(await cfstNative.SelectPath(payload)));
  }
  if (shouldUseWebUIBridge()) {
    return selectWebUIPath(payload);
  }
  return normalizeCommandResult<PathSelectionPayload>(await appBridge().SelectPath(payload));
}

export async function previewSource(payload: Record<string, unknown>) {
  return invokeCore<SourcePreviewPayload>("source.preview", payload);
}

export async function fetchSource(payload: Record<string, unknown>) {
  return invokeCore<SourcePreviewPayload>("source.fetch", payload);
}

export async function loadColoDictionaryStatus() {
  return invokeCore<ColoDictionaryStatus>("colo.status");
}

export async function updateColoDictionary(payload: Record<string, unknown> = {}) {
  return invokeCore<ColoDictionaryStatus>("colo.update", payload);
}

export async function processColoDictionary(payload: Record<string, unknown> = {}) {
  return invokeCore<ColoDictionaryStatus>("colo.process", payload);
}

export async function loadSchedulerStatus() {
  return invokeCore<SchedulerStatus>("scheduler.status");
}

export async function testGitHubExport(payload: Record<string, unknown>) {
  return invokeCore("github.test", payload);
}

export async function testTelegramNotification(payload: Record<string, unknown>) {
  return invokeCore("telegram.test", payload);
}

export async function exportResultsToGitHub(payload: Record<string, unknown>) {
  return invokeCore("github.export", payload);
}

export async function exportResultsCSV(payload: Record<string, unknown>) {
  const result = await invokeCore("results.export_csv", payload);
  if (shouldUseWebUIBridge()) {
    const data = isObject(result.data) ? result.data : {};
    const contentBase64 = toStringValue(data.content_base64 ?? data.contentBase64);
    if (contentBase64) {
      downloadBase64File(toStringValue(data.file_name ?? data.fileName) || "result.csv", contentBase64, "text/csv;charset=utf-8");
    }
  }
  return result;
}

export async function exportDebugLog(payload: Record<string, unknown>) {
  const result = await invokeCore("debug.export", payload);
  if (shouldUseWebUIBridge()) {
    const data = isObject(result.data) ? result.data : {};
    const contentBase64 = toStringValue(data.content_base64 ?? data.contentBase64);
    if (contentBase64) {
      downloadBase64File(toStringValue(data.file_name ?? data.fileName) || "cfip-log.txt", contentBase64, "text/plain;charset=utf-8");
    }
  }
  return result;
}

export async function exportDiagnosticPackage(payload: Record<string, unknown>) {
  const result = await invokeCore("diagnostics.export", payload);
  if (shouldUseWebUIBridge()) {
    const data = isObject(result.data) ? result.data : {};
    const contentBase64 = toStringValue(data.content_base64 ?? data.contentBase64);
    if (contentBase64) {
      downloadBase64File(toStringValue(data.file_name ?? data.fileName) || "cfst-diagnostics.zip", contentBase64, "application/zip");
    }
  }
  return result;
}

export async function openLogDirectory(payload: Record<string, unknown> = {}) {
  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    return normalizeCommandResult(normalizeNativePayload(await cfstNative.OpenLogDirectory(payload)));
  }
  if (shouldUseWebUIBridge()) {
    return normalizeCommandResult(await webUIApp("OpenLogDirectory", payload));
  }
  return normalizeCommandResult(await appBridge().OpenLogDirectory(payload));
}

export async function pushDnsRecords(payload: Record<string, unknown>) {
  return invokeCore("cloudflare.push", payload);
}

export async function startProbe(payload: Record<string, unknown>) {
  const taskId = toStringValue(payload.task_id).trim() || nextTaskId();
  clearTaskWorkspaceCache();

  try {
    const requestPayload = {
      ...payload,
      task_id: taskId,
    };
    const desktopResult = await invokeCore<ProbeRunResultPayload>("probe.start", requestPayload);
    return commandResult(desktopResult.code || "PROBE_ACCEPTED", desktopResult.data, {
      message: desktopResult.message || "桌面探测任务已提交。",
      ok: desktopResult.ok,
      taskId: desktopResult.task_id || taskId,
      warnings: desktopResult.warnings,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : toStringValue(error) || "探测任务执行失败。";
    return commandResult(probeStartFailureCode(message), null, {
      message,
      ok: false,
      taskId,
    });
  }
}

export async function stopProbe(payload: Record<string, unknown>) {
  const mode = toStringValue(payload.mode).trim().toLowerCase();
  return invokeCore(mode === "pause" ? "probe.pause" : "probe.cancel", payload);
}

export async function resumeProbe(payload: Record<string, unknown>) {
  return invokeCore("probe.resume", payload);
}

export async function getTaskSnapshot(taskId: string) {
  return invokeCore<TaskSnapshot | null>("task.get", { task_id: taskId });
}

export async function listTaskSnapshots(limit = 20) {
  const payload = { limit: Math.max(1, Math.min(100, Math.floor(limit || 20))) };
  return invokeCore<TaskSnapshotList>("task.list", payload);
}

export async function listTaskResults(taskId: string, sortBy: ProbeResultSortBy, order: ProbeResultOrder, filter: ProbeResultFilter, fallbackPayload: Record<string, unknown> = {}, ipFilter: ProbeResultIPFilter = "all", paging: { limit?: number; offset?: number } = {}, options: { allowFileFallback?: boolean } = {}) {
  const resultFilePayload = normalizeResultFilePayload(fallbackPayload);
  void options;
  const result = await invokeCore<TaskResultPage>("task.results", {
    ...resultFilePayload,
    filter,
    ip_filter: ipFilter,
    limit: paging.limit,
    offset: paging.offset,
    order,
    sort_by: sortBy,
    task_id: taskId,
  });
  if (!result.ok || !result.data) {
    return commandResult<TaskResultPage>(result.code || "TASK_RESULTS_LIST_FAILED", { count: 0, results: [], total_count: 0 }, { message: result.message, ok: false, taskId });
  }
  return commandResult<TaskResultPage>(result.code || "TASK_RESULTS_LISTED", {
    count: toInteger(result.data.count, 0),
    results: normalizeProbeRows(result.data.results),
    source_kind: toStringValue(result.data.source_kind).trim() || null,
    source_path: toStringValue(result.data.source_path).trim() || null,
    total_count: toInteger(result.data.total_count, toInteger(result.data.count, 0)),
  }, { message: result.message, taskId, warnings: result.warnings });
}

function normalizeResultFilePayload(payload: Record<string, unknown>) {
  const normalized = { ...payload };
  const resultPath = [payload.path, payload.source_path, payload.sourcePath, payload.target_path, payload.targetPath, payload.export_path, payload.exportPath].map((value) => toStringValue(value).trim()).find((value) => value.length > 0) || "";

  if (resultPath) {
    normalized.path = resultPath;
    normalized.source_path = resultPath;
    normalized.target_path = resultPath;
    normalized.export_path = resultPath;
  }

  return normalized;
}

export async function listenToProbeEvents(handler: (event: ProbeEventEnvelope) => void) {
  probeListeners.add(handler);

  if (!disposeRuntimeProbeListener) {
    if (shouldUseNativeBridge()) {
      await ensureNativeBridge();
      const handle = await cfstNative.addListener("probe:event", (payload: unknown) => {
        const event = normalizeProbeEvent(normalizeNativePayload(payload));
        if (event) {
          emitProbeEvent(event);
        }
      });
      disposeRuntimeProbeListener = buildIdempotentDisposer(() => {
        void handle.remove();
      });
    } else if (shouldUseWebUIBridge()) {
      const token = await ensureWebUIToken();
      const source = new EventSource(`/api/events/probe${webUITokenQuery(token)}`);
      source.onmessage = (message) => {
        try {
          const event = normalizeProbeEvent(JSON.parse(message.data));
          if (event) {
            emitProbeEvent(event);
          }
        } catch {
          // Ignore malformed frames; the next valid event or snapshot reconciliation repairs state.
        }
      };
      disposeRuntimeProbeListener = buildIdempotentDisposer(() => source.close());
    } else {
      disposeRuntimeProbeListener = buildIdempotentDisposer(
        EventsOn("probe:event", (payload: unknown) => {
          const event = normalizeProbeEvent(payload);
          if (event) {
            emitProbeEvent(event);
          }
        }),
      );
    }
  }

  return () => {
    probeListeners.delete(handler);
    if (probeListeners.size === 0) {
      clearProbeRuntimeListener();
    }
  };
}

export async function openPath(targetPath: string) {
  const normalized = targetPath.trim();

  if (!normalized) {
    return;
  }

  if (shouldUseNativeBridge()) {
    await ensureNativeBridge();
    await cfstNative.OpenPath({ targetPath: normalized });
    return;
  }

  if (shouldUseWebUIBridge()) {
    await openWebUIPath(normalized);
    return;
  }

  await appBridge().OpenPath(normalized);
}
