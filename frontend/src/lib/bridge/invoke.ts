import { getPlatformAdapter } from "../adapters/registry";

export function invokeCore<T = unknown>(command: string, payload: Record<string, unknown> = {}) {
  const payloadJSON = JSON.stringify(payload ?? {});
  return getPlatformAdapter().invokeCore<T>(command, payloadJSON);
}
