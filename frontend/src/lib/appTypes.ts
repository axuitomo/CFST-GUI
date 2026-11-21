// App 级共享 UI 类型（重构计划 §4.1）：App.vue / 壳层 / 视图 / Toast 的唯一类型来源。
// 原本 App.vue 与 DesktopShell / MobileShell / DashboardView / ToastStack / SettingsView
// 各自持有副本，现统一从这里导入；字段契约与迁移前逐字一致。
import type { TaskTone } from "./bridge";

export type ViewName = "dashboard" | "results" | "sources" | "settings" | "dns";

export type ToastTone = "success" | "error" | "info";

export interface ToastEntry {
  id: number;
  message: string;
  tone: ToastTone;
}

export interface HistoryEntry {
  debugLogPath?: string;
  debugLogTarget?: string;
  detail: string;
  exported: number;
  failureSummary: string;
  targetPath: string;
  taskId: string;
  title: string;
  tone: TaskTone;
  updatedAt: string;
}

export interface ProcessTraceEntry {
  detail: string;
  id: number;
  /** 可选的去重键：同一颗 IP 的实时测速行只更新一条，不再全表只留最近一条。 */
  key?: string;
  stage: string;
  title: string;
  tone: "success" | "error" | "running" | "info" | "warning";
  ts: string;
}

export type ViewportPresetId = "adaptive" | "phone390" | "tablet768" | "desktop1024" | "desktop1366" | "desktop1920" | "desktop2560";

export type FixedViewportPresetId = Exclude<ViewportPresetId, "adaptive">;

interface AdaptiveViewportPreset {
  description: string;
  id: "adaptive";
  label: string;
  mode: "adaptive";
  shell: "desktop";
}

interface FixedViewportPreset {
  description: string;
  height: number;
  id: FixedViewportPresetId;
  label: string;
  mode: "fixed";
  shell: "mobile" | "desktop";
  width: number;
}

export type ViewportPreset = AdaptiveViewportPreset | FixedViewportPreset;

export interface ViewportSize {
  cssHeight: number;
  cssWidth: number;
  height: number;
  updatedAt: string;
  width: number;
}