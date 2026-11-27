import { getPlatformAdapter } from "../adapters/registry";
import { isObject, toStringValue } from "../bridgeValues";
import type { CommandResult } from "./types";

export function triggerExportDownload(result: CommandResult<unknown>, fallbackName: string, mimeType: string, encoding: "utf8" | "base64" = "base64") {
  const download = getPlatformAdapter().downloadExportedFile;
  if (!download) {
    return result;
  }
  const data = isObject(result.data) ? result.data : {};
  if (encoding === "utf8") {
    const content = toStringValue(data.content);
    if (content) {
      download(toStringValue(data.file_name ?? data.fileName) || fallbackName, content, "utf8", mimeType);
    }
    return result;
  }
  const contentBase64 = toStringValue(data.content_base64 ?? data.contentBase64);
  if (contentBase64) {
    download(toStringValue(data.file_name ?? data.fileName) || fallbackName, contentBase64, "base64", mimeType);
  }
  return result;
}

export function selectPath(payload: Record<string, unknown>) {
  return getPlatformAdapter().selectPath(payload);
}

export function openPath(targetPath: string) {
  const normalized = targetPath.trim();
  if (!normalized) {
    return Promise.resolve();
  }
  return getPlatformAdapter().openPath(normalized);
}
