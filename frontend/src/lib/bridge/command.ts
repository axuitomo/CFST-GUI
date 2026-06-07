import { isObject, toStringValue } from "../bridgeValues";
import type { CommandResult } from "./types";

export const SCHEMA_VERSION = "phase1-bridge-v1";

export function normalizeCommandResult<T = Record<string, unknown> | null>(input: unknown): CommandResult<T> {
  const source = isObject(input) ? input : {};
  return {
    code: toStringValue(source.code) || "UNKNOWN",
    data: (source.data as T | null) ?? null,
    message: toStringValue(source.message),
    ok: source.ok !== false,
    schema_version: toStringValue(source.schema_version) || SCHEMA_VERSION,
    task_id: toStringValue(source.task_id) || null,
    warnings: Array.isArray(source.warnings) ? source.warnings.map((entry) => toStringValue(entry)).filter(Boolean) : [],
  };
}

export function commandResult<T = Record<string, unknown> | null>(
  code: string,
  data: T,
  options: {
    message?: string;
    ok?: boolean;
    taskId?: string | null;
    warnings?: string[];
  } = {},
): CommandResult<T> {
  return {
    code,
    data,
    message: options.message || "",
    ok: options.ok !== false,
    schema_version: SCHEMA_VERSION,
    task_id: options.taskId || null,
    warnings: options.warnings || [],
  };
}
