import { getPlatformAdapter } from "../../adapters/registry";
import { triggerExportDownload } from "../files";
import { invokeCore } from "../invoke";

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
  return triggerExportDownload(await invokeCore("results.export_csv", payload), "result.csv", "text/csv;charset=utf-8", "base64");
}

export async function exportDebugLog(payload: Record<string, unknown>) {
  return triggerExportDownload(await invokeCore("debug.export", payload), "cfip-log.txt", "text/plain;charset=utf-8", "base64");
}

export async function exportDiagnosticPackage(payload: Record<string, unknown>) {
  return triggerExportDownload(await invokeCore("diagnostics.export", payload), "cfst-diagnostics.zip", "application/zip", "base64");
}

export async function openLogDirectory(payload: Record<string, unknown> = {}) {
  return getPlatformAdapter().openLogDirectory(payload);
}
