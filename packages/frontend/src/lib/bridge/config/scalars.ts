// 标量字段（枚举、时间、数值区间）的归一化工具。
import { clampInteger, positiveInteger, toInteger, toOptionalInteger, toStringArray, toStringValue } from "../values";
import type { ColoFilterMode, CSVEncoding, DebugLogMode, DebugLogVerbosity, DownloadHTTPProtocol, DownloadSpeedMetric, ProbeStrategy, SourceColoFilterPhase, SourceIPMode, SourceKind, TelegramRecipientMode, ThemeMode, TraceColoMode } from "../types";
import { DEFAULT_FILE_TEST_URL, DEFAULT_HTTPING_STATUS_CODE, DEFAULT_UTC_OFFSET_MINUTES, LEGACY_FILE_TEST_URLS } from "./defaults";

export function downloadSpeedSampleIntervalMs(probe: Record<string, unknown>) {
  const msValue = probe.download_speed_sample_interval_ms ?? probe.downloadSpeedSampleIntervalMs;
  if (msValue !== null && msValue !== undefined && msValue !== "") {
    return positiveInteger(msValue, 1000);
  }

  const secondsValue = probe.download_speed_sample_interval_seconds ?? probe.downloadSpeedSampleIntervalSeconds;
  if (secondsValue !== null && secondsValue !== undefined && secondsValue !== "") {
    return positiveInteger(secondsValue, 1) * 1000;
  }

  return 1000;
}

// 旧配置只保存过 stage3_ms，按秒迁移到 download_time_seconds。
export function downloadTimeSeconds(probe: Record<string, unknown>, timeouts: Record<string, unknown>) {
  const seconds = probe.download_time_seconds ?? probe.downloadTimeSeconds;
  if (seconds !== null && seconds !== undefined && seconds !== "") {
    return positiveInteger(seconds, 4);
  }
  const legacyMs = positiveInteger(timeouts.stage3_ms ?? timeouts.stage3Ms, 0);
  return legacyMs > 0 ? Math.max(1, Math.round(legacyMs / 1000)) : 4;
}

export function normalizeDebugLogMode(value: unknown): DebugLogMode {
  return toStringValue(value).toLowerCase() === "freeform" ? "freeform" : "structured";
}

export function normalizeDownloadSpeedMetric(value: unknown): DownloadSpeedMetric {
  const normalized = toStringValue(value).trim().toLowerCase();
  return normalized === "max" || normalized === "peak" || normalized === "highest" ? "max" : "average";
}

export function normalizeDebugLogVerbosity(value: unknown): DebugLogVerbosity {
  return toStringValue(value).toLowerCase() === "simple" ? "simple" : "detailed";
}

export function normalizeThemeMode(value: unknown): ThemeMode {
  const normalized = toStringValue(value).toLowerCase().trim();
  if (normalized === "light") {
    return "light";
  }
  if (normalized === "dark") {
    return "dark";
  }
  if (normalized === "auto_time") {
    return "auto_time";
  }
  return "auto_system_time";
}

export function normalizeUTCOffsetMinutes(value: unknown) {
  return clampInteger(value, DEFAULT_UTC_OFFSET_MINUTES, -12 * 60, 14 * 60);
}

export function normalizeSchedulerDailyTime(value: string) {
  const normalized = value.replace(/：/g, ":").trim();
  const match = normalized.match(/^(\d{1,2}):(\d{1,2})(?::(\d{1,2}))?$/);
  if (!match) {
    return "";
  }
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  const second = match[3] === undefined ? 0 : Number(match[3]);
  if (!Number.isInteger(hour) || !Number.isInteger(minute) || !Number.isInteger(second) || hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59) {
    return "";
  }
  const hh = String(hour).padStart(2, "0");
  const mm = String(minute).padStart(2, "0");
  if (match[3] === undefined) {
    return `${hh}:${mm}`;
  }
  return `${hh}:${mm}:${String(second).padStart(2, "0")}`;
}

export function normalizeSchedulerDailyTimes(value: unknown) {
  const entries = Array.isArray(value)
    ? toStringArray(value, { trim: true })
    : toStringValue(value)
        .split(/[,\s;，；、]+/)
        .map((entry) => entry.trim());
  return entries.map(normalizeSchedulerDailyTime).filter(Boolean);
}

export function normalizeFileTestURL(value: unknown): string {
  const url = toStringValue(value);
  if (!url || LEGACY_FILE_TEST_URLS.has(url)) {
    return DEFAULT_FILE_TEST_URL;
  }
  return url;
}

export function normalizeDownloadHTTPProtocol(value: unknown): DownloadHTTPProtocol {
  const normalized = toStringValue(value).toLowerCase();
  if (normalized === "h1" || normalized === "h1.1" || normalized === "http/1.1") {
    return "h1";
  }
  if (normalized === "h2" || normalized === "http/2") {
    return "h2";
  }
  if (normalized === "h3" || normalized === "http/3") {
    return "h3";
  }
  return "auto";
}

export function minimumInteger(value: unknown, fallback: number, min: number, max?: number) {
  return Math.max(min, positiveInteger(value, fallback, max));
}

export function toOptionalPositiveInteger(value: unknown) {
  const parsed = toOptionalInteger(value);
  return parsed !== null && parsed > 0 ? parsed : null;
}

export function normalizeHTTPStatusCode(value: unknown) {
  const parsed = toInteger(value, DEFAULT_HTTPING_STATUS_CODE);
  return parsed === 0 || (parsed >= 100 && parsed <= 599) ? parsed : DEFAULT_HTTPING_STATUS_CODE;
}

export function normalizeExportOverwrite(value: unknown) {
  return toStringValue(value) === "append" ? "append" : "replace_on_start";
}

export function normalizeCSVEncoding(value: unknown): CSVEncoding {
  const normalized = toStringValue(value)
    .trim()
    .toLowerCase()
    .replace(/[_\s]+/g, "-");
  if (normalized === "utf-8-bom" || normalized === "utf8-bom" || normalized === "utf-8-with-bom" || normalized === "utf8-with-bom" || normalized === "utf-8-sig" || normalized === "bom") {
    return "utf-8-bom";
  }
  return "utf-8";
}

export function normalizeStrategy(value: unknown): ProbeStrategy {
  const normalized = toStringValue(value).toLowerCase();
  if (normalized === "fast" || normalized === "latency") {
    return "fast";
  }
  if (normalized === "full" || normalized === "speed" || normalized === "exhaustive") {
    return "full";
  }
  return "fast";
}

export function normalizeSourceColoFilterPhase(value: unknown): SourceColoFilterPhase {
  const normalized = toStringValue(value).toLowerCase().trim();
  if (normalized === "stage2" || normalized === "stage-2" || normalized === "second_stage" || normalized === "second-stage") {
    return "stage2";
  }
  return "precheck";
}

export function normalizeTraceColoMode(value: unknown): TraceColoMode {
  const normalized = toStringValue(value).toLowerCase().trim();
  if (normalized === "trace_url" || normalized === "trace-url" || normalized === "traceurl") {
    return "trace_url";
  }
  return "standard";
}

export function normalizeColoFilterMode(value: unknown): ColoFilterMode {
  const normalized = toStringValue(value).toLowerCase().trim();
  if (normalized === "deny" || normalized === "blacklist" || normalized === "black-list" || normalized === "black_list" || normalized === "blocklist" || normalized === "block-list" || normalized === "block_list") {
    return "deny";
  }
  return "allow";
}

export function normalizeTelegramRecipientMode(value: unknown): TelegramRecipientMode {
  const normalized = toStringValue(value).trim().toLowerCase();
  if (["personal", "private", "direct", "user", "me"].includes(normalized)) {
    return "personal";
  }
  if (["both", "all", "chat_personal", "chat_and_personal", "personal_and_chat"].includes(normalized)) {
    return "both";
  }
  return "chat";
}

export function normalizeSourceKind(value: unknown): SourceKind {
  const normalized = toStringValue(value).toLowerCase();
  if (normalized === "inline" || normalized === "file") {
    return normalized;
  }
  return "url";
}

export function normalizeSourceIPMode(value: unknown): SourceIPMode {
  return toStringValue(value).toLowerCase() === "mcis" ? "mcis" : "traverse";
}

export function isMaskedTokenValue(value: string) {
  return value.includes("...") || value.includes("***") || /^\*+$/.test(value);
}

export function normalizeUploadStatus(value: unknown): "all" | "passed" {
  return toStringValue(value).trim().toLowerCase() === "all" ? "all" : "passed";
}

export function normalizeGitHubFormat(value: unknown): "csv" | "txt" {
  return toStringValue(value).trim().toLowerCase() === "txt" ? "txt" : "csv";
}

export function normalizePortPolicy(value: unknown): "source_override_global" | "fixed_global" {
  return toStringValue(value).trim().toLowerCase() === "source_override_global" ? "source_override_global" : "fixed_global";
}

export function normalizeUploadIPVersion(value: unknown): "any" | "ipv4" | "ipv6" {
  const normalized = toStringValue(value).trim().toLowerCase();
  if (normalized === "ipv4" || normalized === "ipv6") {
    return normalized;
  }
  return "any";
}

export function toOptionalNonNegativeNumber(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  const parsed = Number.parseFloat(String(value));
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }
  return parsed;
}
