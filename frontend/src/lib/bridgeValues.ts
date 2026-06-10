export function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

export function toObjectRecord(value: unknown): Record<string, unknown> {
  return isObject(value) ? value : {};
}

export function toUnknownArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

export function toObjectArray(value: unknown): Record<string, unknown>[] {
  return toUnknownArray(value).filter(isObject);
}

export function toStringArray(value: unknown, options: { trim?: boolean } = {}): string[] {
  return toUnknownArray(value)
    .map((entry) => {
      const text = toStringValue(entry);
      return options.trim ? text.trim() : text;
    })
    .filter(Boolean);
}

export function toStringValue(value: unknown) {
  return typeof value === "string" ? value : value == null ? "" : String(value);
}

export function toInteger(value: unknown, fallback = 0) {
  const parsed = Number.parseInt(String(value ?? ""), 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

export function toNumber(value: unknown, fallback = 0) {
  const parsed = Number.parseFloat(String(value ?? ""));
  return Number.isFinite(parsed) ? parsed : fallback;
}

export function toOptionalNumber(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return null;
  }

  const parsed = Number.parseFloat(String(value));
  return Number.isFinite(parsed) ? parsed : null;
}

export function toOptionalInteger(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return null;
  }

  const parsed = Number.parseInt(String(value), 10);
  return Number.isFinite(parsed) ? parsed : null;
}

export function clampInteger(value: unknown, fallback: number, min: number, max: number) {
  return Math.max(min, Math.min(max, toInteger(value, fallback)));
}

export function positiveInteger(value: unknown, fallback: number, max?: number) {
  const parsed = toInteger(value, fallback);
  const normalized = parsed > 0 ? parsed : fallback;
  return typeof max === "number" ? Math.min(normalized, max) : normalized;
}

export function nonNegativeInteger(value: unknown, fallback: number) {
  const parsed = toInteger(value, fallback);
  return parsed >= 0 ? parsed : fallback;
}

export function nonNegativeNumber(value: unknown, fallback: number) {
  const parsed = toNumber(value, fallback);
  return parsed >= 0 ? parsed : fallback;
}

export function clampNumber(value: unknown, fallback: number, min: number, max: number) {
  return Math.max(min, Math.min(max, toNumber(value, fallback)));
}

export function toBoolean(value: unknown, fallback = false) {
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
