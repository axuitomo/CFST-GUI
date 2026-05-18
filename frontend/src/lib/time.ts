const DEFAULT_UTC_OFFSET_MINUTES = 8 * 60;
const MIN_UTC_OFFSET_MINUTES = -12 * 60;
const MAX_UTC_OFFSET_MINUTES = 14 * 60;

function pad2(value: number) {
  return String(value).padStart(2, "0");
}

export function normalizeUTCOffsetMinutes(value: unknown, fallback = DEFAULT_UTC_OFFSET_MINUTES) {
  const parsed = Number.parseInt(String(value ?? ""), 10);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return Math.max(MIN_UTC_OFFSET_MINUTES, Math.min(MAX_UTC_OFFSET_MINUTES, parsed));
}

export function formatUTCOffsetLabel(offsetMinutes: number) {
  const normalized = normalizeUTCOffsetMinutes(offsetMinutes);
  const sign = normalized >= 0 ? "+" : "-";
  const absoluteMinutes = Math.abs(normalized);
  const hours = Math.floor(absoluteMinutes / 60);
  const minutes = absoluteMinutes % 60;
  return `UTC${sign}${pad2(hours)}:${pad2(minutes)}`;
}

function datePartsInUTCOffset(ts: string, offsetMinutes: number) {
  if (!ts.trim()) {
    return null;
  }
  const parsed = new Date(ts);
  if (Number.isNaN(parsed.getTime())) {
    return null;
  }
  const shifted = new Date(parsed.getTime() + normalizeUTCOffsetMinutes(offsetMinutes) * 60_000);
  return {
    year: shifted.getUTCFullYear(),
    month: shifted.getUTCMonth() + 1,
    day: shifted.getUTCDate(),
    hours: shifted.getUTCHours(),
    minutes: shifted.getUTCMinutes(),
    seconds: shifted.getUTCSeconds(),
  };
}

interface TimestampFormatOptions {
  includeDate?: boolean;
  includeOffset?: boolean;
  includeSeconds?: boolean;
  fallback?: string;
}

export function formatTimestampWithUTCOffset(ts: string, offsetMinutes: number, options: TimestampFormatOptions = {}) {
  const {
    fallback = "-",
    includeDate = true,
    includeOffset = false,
    includeSeconds = true,
  } = options;
  const parts = datePartsInUTCOffset(ts, offsetMinutes);
  if (!parts) {
    return ts.trim() || fallback;
  }

  const dateText = `${parts.year}-${pad2(parts.month)}-${pad2(parts.day)}`;
  const timeText = includeSeconds
    ? `${pad2(parts.hours)}:${pad2(parts.minutes)}:${pad2(parts.seconds)}`
    : `${pad2(parts.hours)}:${pad2(parts.minutes)}`;
  const offsetText = includeOffset ? ` ${formatUTCOffsetLabel(offsetMinutes)}` : "";
  return `${includeDate ? `${dateText} ` : ""}${timeText}${offsetText}`.trim();
}

export function currentMinutesInUTCOffset(offsetMinutes: number) {
  const now = new Date();
  const utcMinutes = now.getUTCHours() * 60 + now.getUTCMinutes();
  const normalized = normalizeUTCOffsetMinutes(offsetMinutes);
  const minutesPerDay = 24 * 60;
  return ((utcMinutes + normalized) % minutesPerDay + minutesPerDay) % minutesPerDay;
}

export { DEFAULT_UTC_OFFSET_MINUTES };
