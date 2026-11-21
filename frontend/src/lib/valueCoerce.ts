// 值归一化纯函数（重构计划 §4.2）：从 App.vue 原样迁出，零状态、零副作用。
// 配置快照构建 / 应用快照 / 事件处理共用这一套 coercion 口径，避免各写各的边界判定。
import type { ConfigSnapshot } from "./bridge";
import type { ResultCloudflareRecordType } from "./appTypes";
import { DEFAULT_CLOUDFLARE_TTL, type SchedulerTriggerMode, type SettingsForm } from "./settingsForm";

export function asCount(value: unknown, fallback = 0) {
  const parsed = Number.parseInt(String(value ?? ""), 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

export function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

export function asString(value: unknown) {
  return typeof value === "string" ? value : value == null ? "" : String(value);
}

export function asBoolean(value: unknown, fallback = false) {
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "number") {
    return value !== 0;
  }
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    if (["1", "true", "yes", "on"].includes(normalized)) {
      return true;
    }
    if (["0", "false", "no", "off"].includes(normalized)) {
      return false;
    }
  }
  return fallback;
}

export function asNumber(value: unknown, fallback = 0) {
  const parsed = Number.parseFloat(String(value ?? ""));
  return Number.isFinite(parsed) ? parsed : fallback;
}

export function positiveCount(value: unknown, fallback: number, max?: number) {
  const parsed = asCount(value, fallback);
  const normalized = parsed > 0 ? parsed : fallback;
  return typeof max === "number" ? Math.min(normalized, max) : normalized;
}

export function normalizeCloudflareTTL(value: unknown) {
  const ttl = asCount(value, DEFAULT_CLOUDFLARE_TTL);
  return [60, 300, 600].includes(ttl) ? ttl : DEFAULT_CLOUDFLARE_TTL;
}

export function minimumCount(value: unknown, fallback: number, min: number, max?: number) {
  return Math.max(min, positiveCount(value, fallback, max));
}

export function boundedCount(value: unknown, fallback: number, min: number, max: number) {
  return Math.max(min, Math.min(max, asCount(value, fallback)));
}

export function normalizeDownloadHTTPProtocol(value: unknown): SettingsForm["probeDownloadHTTPProtocol"] {
  const normalized = asString(value).toLowerCase();
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

export function nonNegativeCount(value: unknown, fallback = 0) {
  const parsed = asCount(value, fallback);
  return parsed >= 0 ? parsed : fallback;
}

export function schedulerTriggerModeFromSnapshot(scheduler: ConfigSnapshot["scheduler"], fallback: SchedulerTriggerMode): SchedulerTriggerMode {
  if (scheduler.daily_times.length > 0) {
    return "daily";
  }
  if (scheduler.interval_minutes > 0) {
    return "interval";
  }
  return fallback;
}

export function schedulerDailyTimesFromText(value: string) {
  return value
    .split(/[,\s;，；、]+/)
    .map((entry) => entry.trim())
    .map(normalizeSchedulerDailyTime)
    .filter(Boolean);
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

export function normalizeResultCloudflareRecordType(value: unknown): ResultCloudflareRecordType {
  const normalized = asString(value).trim().toUpperCase();
  if (normalized === "A" || normalized === "AAAA") {
    return normalized;
  }
  return "ALL";
}

export function nonNegativeNumber(value: unknown, fallback = 0) {
  const parsed = asNumber(value, fallback);
  return parsed >= 0 ? parsed : fallback;
}

export function clampedNumber(value: unknown, fallback: number, min: number, max: number) {
  const parsed = asNumber(value, fallback);
  return Math.max(min, Math.min(max, parsed));
}

export function asNullableNumber(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return null;
  }

  const parsed = Number.parseFloat(String(value));
  return Number.isFinite(parsed) ? parsed : null;
}

export function optionalNumberForPayload(value: number | null) {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : null;
}

export function parseClockMinutes(value: string, fallback: number) {
  const [hourRaw, minuteRaw] = value.trim().split(":");
  const hour = Number.parseInt(hourRaw || "", 10);
  const minute = Number.parseInt(minuteRaw || "", 10);
  if (!Number.isFinite(hour) || !Number.isFinite(minute) || hour < 0 || hour > 23 || minute < 0 || minute > 59) {
    return fallback;
  }
  return hour * 60 + minute;
}
