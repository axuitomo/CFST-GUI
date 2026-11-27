import { triggerExportDownload } from "../files";
import { invokeCore } from "../invoke";
import type { ColoDictionaryStatus, SchedulerStatus } from "../types";

export async function loadConfig() {
  return invokeCore("config.load");
}

export async function saveDraft(payload: Record<string, unknown>) {
  return invokeCore("draft.save", payload);
}

export async function discardDraft(payload: Record<string, unknown> = {}) {
  return invokeCore("draft.discard", payload);
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
  return triggerExportDownload(await invokeCore("config.export", payload), "cfst-gui-config.json", "application/json", "utf8");
}

export async function exportConfigArchive(payload: Record<string, unknown>) {
  return triggerExportDownload(await invokeCore("archive.export", payload), "cfst-gui-config.zip", "application/zip", "base64");
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
