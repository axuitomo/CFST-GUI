// 结果导出域（重构计划 §4.2）：CSV/GitHub/Cloudflare 推送、推送偏好持久化与导出历史。
// 从 App.vue 原样迁出；跨域回调经 deps 注入。
import { computed, reactive, ref, watch, type Ref } from "vue";
import {
  exportResultsCSV,
  exportResultsToGitHub,
  normalizeConfigSnapshot,
  openPath,
  pushDnsRecords,
  testGitHubExport,
  testTelegramNotification,
  type ProbeResult,
  type TaskTone,
} from "../lib/bridge";
import { dnsProviderPlatformLabel } from "../lib/dnsProviderForm";
import type { DnsProviderForm, SettingsForm } from "../lib/settingsForm";
import { asBoolean, asCount, asRecord, asString, nonNegativeCount, normalizeResultCloudflareRecordType } from "../lib/valueCoerce";
import type { HistoryEntry, ResultCloudflareRecordType, ToastTone, ViewName } from "../lib/appTypes";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../lib/configSnapshotBuild";

export interface ResultCloudflarePushSettings {
  providerId: string;
  recordName: string;
  recordType: ResultCloudflareRecordType;
  topN: number;
}

export interface ResultExportDeps {
  settings: SettingsForm;
  task: { exportPath: string; taskId: string };
  isAndroidApp: () => boolean;
  enabledDNSProviders: () => DnsProviderForm[];
  primaryCloudflareProvider: () => DnsProviderForm | undefined;
  dnsProviderTargetLabel: (entry: DnsProviderForm) => string;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  currentVisibleResultRows: () => ProbeResult[];
  pathLeafName: (rawPath: string) => string;
  probeWarnings: Ref<string[]>;
  pushWarningTrace: (warnings: string[], ts?: string) => void;
  navigateTo: (view: ViewName) => void;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  pushActivity: (title: string, detail: string) => void;
}

export function useResultExport(deps: ResultExportDeps) {
  const { settings, task, isAndroidApp, enabledDNSProviders, primaryCloudflareProvider, dnsProviderTargetLabel, buildConfigSnapshot, currentVisibleResultRows, pathLeafName, probeWarnings, pushWarningTrace, navigateTo, appendLog, showToast, setStatus, pushActivity } = deps;
  const RESULT_CLOUDFLARE_PUSH_SETTINGS_KEY = "cfst.result.cloudflarePushSettings.v1";
  const RESULT_GITHUB_TOP_N_KEY = "cfst.result.githubTopN.v1";
  const EXPORT_HISTORY_LIMIT = 50;

  const exportHistory = ref<HistoryEntry[]>([]);

  const csvExporting = ref(false);
  const cloudflarePushing = ref(false);
  const githubExporting = ref(false);
  const githubTesting = ref(false);
  const telegramTesting = ref(false);
  const resultGitHubTopN = ref(20);
  const resultCloudflarePushSettings = reactive<ResultCloudflarePushSettings>({
    providerId: "",
    recordName: "",
    recordType: "ALL",
    topN: 5,
  });

  let resultCloudflarePushSettingsHydrated = false;

  function updateHistory(entry: HistoryEntry) {
    const nextEntries = [...exportHistory.value];
    const currentIndex = nextEntries.findIndex((item) => item.taskId === entry.taskId);

    if (currentIndex >= 0) {
      nextEntries[currentIndex] = entry;
    } else {
      nextEntries.unshift(entry);
    }

    exportHistory.value = nextEntries.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt)).slice(0, EXPORT_HISTORY_LIMIT);
  }

  function applyResultCloudflarePushSettings(input: Partial<ResultCloudflarePushSettings> = {}) {
    const primary = enabledDNSProviders()[0] ?? primaryCloudflareProvider();
    resultCloudflarePushSettings.providerId = asString(input.providerId).trim() || primary?.id || "";
    resultCloudflarePushSettings.recordName = asString(input.recordName).trim() || primary?.recordName.trim() || "";
    resultCloudflarePushSettings.recordType = normalizeResultCloudflareRecordType(input.recordType);
    resultCloudflarePushSettings.topN = nonNegativeCount(input.topN, primary?.topN ?? 5);
  }

  function loadResultCloudflarePushSettings() {
    if (typeof window === "undefined") {
      applyResultCloudflarePushSettings();
      return;
    }
    try {
      const raw = window.localStorage.getItem(RESULT_CLOUDFLARE_PUSH_SETTINGS_KEY);
      applyResultCloudflarePushSettings(raw ? (JSON.parse(raw) as Partial<ResultCloudflarePushSettings>) : {});
    } catch (error) {
      appendLog("result.cloudflare_push_settings.load_failed", error instanceof Error ? error.message : String(error));
      applyResultCloudflarePushSettings();
    }
  }

  function saveResultCloudflarePushSettings() {
    if (typeof window === "undefined") {
      return;
    }
    try {
      window.localStorage.setItem(
        RESULT_CLOUDFLARE_PUSH_SETTINGS_KEY,
        JSON.stringify({
          recordName: resultCloudflarePushSettings.recordName.trim(),
          recordType: normalizeResultCloudflareRecordType(resultCloudflarePushSettings.recordType),
          topN: nonNegativeCount(resultCloudflarePushSettings.topN, 0),
        }),
      );
    } catch (error) {
      appendLog("result.cloudflare_push_settings.save_failed", error instanceof Error ? error.message : String(error));
    }
  }

  function updateResultCloudflarePushSettings(next: Partial<ResultCloudflarePushSettings>) {
    if ("recordName" in next) {
      resultCloudflarePushSettings.recordName = asString(next.recordName).trim();
    }
    if ("recordType" in next) {
      resultCloudflarePushSettings.recordType = normalizeResultCloudflareRecordType(next.recordType);
    }
    if ("topN" in next) {
      resultCloudflarePushSettings.topN = nonNegativeCount(next.topN, 0);
    }
  }

  function loadResultGitHubTopN() {
    if (typeof window === "undefined") {
      resultGitHubTopN.value = nonNegativeCount(settings.uploadGitHubTopN, 20);
      return;
    }
    try {
      const raw = window.localStorage.getItem(RESULT_GITHUB_TOP_N_KEY);
      resultGitHubTopN.value = raw === null ? nonNegativeCount(settings.uploadGitHubTopN, 20) : nonNegativeCount(JSON.parse(raw), settings.uploadGitHubTopN);
    } catch (error) {
      appendLog("result.github_top_n.load_failed", error instanceof Error ? error.message : String(error));
      resultGitHubTopN.value = nonNegativeCount(settings.uploadGitHubTopN, 20);
    }
  }

  function saveResultGitHubTopN() {
    if (typeof window === "undefined") {
      return;
    }
    try {
      window.localStorage.setItem(RESULT_GITHUB_TOP_N_KEY, JSON.stringify(nonNegativeCount(resultGitHubTopN.value, 0)));
    } catch (error) {
      appendLog("result.github_top_n.save_failed", error instanceof Error ? error.message : String(error));
    }
  }

  function updateResultGitHubTopN(value: number) {
    resultGitHubTopN.value = nonNegativeCount(value, 0);
    saveResultGitHubTopN();
  }

  function limitRowsForQuickPush(rows: ProbeResult[], topN: number) {
    const normalizedTopN = nonNegativeCount(topN, 0);
    return normalizedTopN > 0 ? rows.slice(0, normalizedTopN) : rows;
  }

  function defaultResultsCSVFileName() {
    return pathLeafName(task.exportPath) || settings.exportFileName.trim() || "result.csv";
  }

  async function testGitHubExportSettings() {
    githubTesting.value = true;
    try {
      const result = await testGitHubExport({ config: buildConfigSnapshot() });
      appendLog("bridge.test_github_export", result);
      showToast(result.message || (result.ok ? "GitHub 仓库访问可用" : "GitHub 导出测试失败"), result.ok ? "success" : "error");
      if (!result.ok) {
        pushActivity("GitHub 导出测试失败", result.message || "请检查 owner、repo、branch 与 PAT 权限。");
        return;
      }
      pushActivity("GitHub 导出测试通过", result.message || "目标仓库 Contents 权限可用。");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "GitHub 导出测试失败", "error");
    } finally {
      githubTesting.value = false;
    }
  }

  async function testTelegramNotificationSettings() {
    telegramTesting.value = true;
    try {
      const result = await testTelegramNotification({ config: buildConfigSnapshot() });
      appendLog("bridge.test_telegram_notification", result);
      showToast(result.message || (result.ok ? "Telegram 通知可用" : "Telegram 通知测试失败"), result.ok ? "success" : "error");
      if (!result.ok) {
        pushActivity("Telegram 通知测试失败", result.message || "请检查 Bot Token 与 Chat ID。");
        return;
      }
      pushActivity("Telegram 通知测试通过", result.message || "上传通知渠道可用。");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Telegram 通知测试失败", "error");
    } finally {
      telegramTesting.value = false;
    }
  }

  async function exportCurrentResultsToGitHub() {
    const visibleRows = currentVisibleResultRows();
    if (visibleRows.length === 0) {
      void navigateTo("results");
      showToast("没有可导出的测速结果", "error");
      return;
    }

    const githubTopN = nonNegativeCount(resultGitHubTopN.value, 0);

    githubExporting.value = true;
    try {
      const config = normalizeConfigSnapshot(buildConfigSnapshot());
      config.github.top_n = githubTopN;
      config.upload.github.top_n = githubTopN;
      const result = await exportResultsToGitHub({
        config,
        export_path: task.exportPath,
        notification_trigger: "manual_push",
        results: visibleRows,
        task_id: task.taskId,
      });
      const data = asRecord(result.data);
      appendLog("bridge.export_results_github", result);
      if (!result.ok) {
        setStatus({
          detail: result.message || "导出到 GitHub 失败。",
          title: "GitHub 导出失败",
          tone: "failed",
        });
        pushActivity("GitHub 导出失败", result.message || "未能写入目标仓库。");
        showToast("GitHub 导出失败", "error");
        return;
      }

      settings.githubLastExportAt = asString(data.exported_at || data.exportedAt).trim() || new Date().toISOString();
      const targetPath = asString(data.path).trim();
      const htmlURL = asString(data.html_url || data.htmlURL).trim();
      setStatus({
        detail: targetPath ? `已写入 GitHub：${targetPath}` : result.message || "测速结果已导出到 GitHub。",
        title: "GitHub 导出完成",
        tone: "completed",
      });
      pushActivity("GitHub 导出完成", htmlURL || targetPath || result.message || "测速结果 CSV 已写入目标仓库。");
      showToast("已导出到 GitHub", "success");
    } finally {
      githubExporting.value = false;
    }
  }

  function resultPushTargetProvider() {
    const enabled = enabledDNSProviders();
    return enabled.find((entry) => entry.id === resultCloudflarePushSettings.providerId) ?? enabled[0];
  }

  async function pushCurrentResultsToCloudflare() {
    const visibleRows = currentVisibleResultRows();
    if (visibleRows.length === 0) {
      void navigateTo("results");
      showToast("没有可推送的测速结果", "error");
      return;
    }

    const target = resultPushTargetProvider();
    if (!target) {
      void navigateTo("settings");
      showToast("请先在设置页添加并启用 DNS 推送平台", "error");
      return;
    }
    const platformLabel = dnsProviderPlatformLabel(target.provider);
    const routingActive = Boolean(target.routingEnabled && target.routingRules.some((rule) => rule.enabled && rule.recordName.trim()));
    const recordName = resultCloudflarePushSettings.recordName.trim() || target.recordName.trim();
    const subdomainTarget = target.provider === "dnshe" && Boolean(target.subdomain.trim());
    if (!routingActive && !recordName && !subdomainTarget) {
      void navigateTo("results");
      showToast("请先填写 DNS 记录名称", "error");
      return;
    }

    const topN = nonNegativeCount(resultCloudflarePushSettings.topN, 0);
    const selectedRows = limitRowsForQuickPush(visibleRows, topN);
    if (selectedRows.length === 0) {
      void navigateTo("results");
      showToast("当前筛选结果没有可推送 IP", "error");
      return;
    }

    resultCloudflarePushSettings.providerId = target.id;
    saveResultCloudflarePushSettings();
    cloudflarePushing.value = true;
    try {
      const config = normalizeConfigSnapshot(buildConfigSnapshot());
      const entry = config.dns_providers.find((item) => item.id === target.id);
      if (entry) {
        entry.top_n = topN;
        if (!routingActive) {
          entry.record_name = recordName;
          entry.record_type = normalizeResultCloudflareRecordType(resultCloudflarePushSettings.recordType);
          entry.routing_enabled = false;
          entry.routing_rules = [];
        }
      }
      if (target.provider === "cloudflare") {
        config.cloudflare.top_n = topN;
        config.upload.cloudflare.top_n = topN;
        if (!routingActive) {
          config.cloudflare.record_name = recordName;
          config.cloudflare.record_type = normalizeResultCloudflareRecordType(resultCloudflarePushSettings.recordType);
          config.cloudflare.routing_enabled = false;
          config.cloudflare.routing_rules = [];
          config.upload.cloudflare.routing_enabled = false;
          config.upload.cloudflare.routing_rules = [];
        }
      }

      const result = await pushDnsRecords({
        config,
        notification_trigger: "manual_push",
        provider_id: target.id,
        results: selectedRows,
        task_id: task.taskId,
      });
      const data = asRecord(result.data);
      const providerReport = Array.isArray(data.providers) ? asRecord((data.providers as unknown[])[0]) : {};
      appendLog("bridge.push_results_dns", result);
      probeWarnings.value = result.warnings || [];
      pushWarningTrace(probeWarnings.value);
      if (!result.ok) {
        setStatus({
          detail: result.message || `当前结果推送到 ${platformLabel} 失败。`,
          title: `${platformLabel} 推送失败`,
          tone: "failed",
        });
        pushActivity(`${platformLabel} 推送失败`, result.message || "未能覆盖目标 DNS 记录。");
        showToast(`${platformLabel} 推送失败`, "error");
        return;
      }

      if (asBoolean(providerReport.routing_enabled || data.routing_enabled || data.routingEnabled, false)) {
        const detail = asString(providerReport.message) || result.message || `${platformLabel} 分流推送完成。`;
        const partial = asString(providerReport.status) === "partial";
        setStatus({
          detail,
          title: partial ? `${platformLabel} 推送部分完成` : `${platformLabel} 推送完成`,
          tone: partial ? "partial" : "completed",
        });
        pushActivity(partial ? `${platformLabel} 推送部分完成` : `${platformLabel} 推送完成`, detail);
        showToast(partial ? `${platformLabel} 部分目标推送完成` : `已推送到 ${platformLabel}`, partial ? "info" : "success");
        return;
      }

      const summaryRecord = asRecord(providerReport.summary ?? data.summary);
      const created = asCount(summaryRecord.created);
      const updated = asCount(summaryRecord.updated);
      const deleted = asCount(summaryRecord.deleted);
      const ignored = asCount(summaryRecord.ignored);
      const uploadCount = asCount(providerReport.upload_count || data.upload_count || data.uploadCount, selectedRows.length);
      const detail = `已推送 ${uploadCount} 条到 ${platformLabel} ${recordName}：创建 ${created}、更新 ${updated}、删除 ${deleted}、忽略 ${ignored}。`;
      setStatus({
        detail,
        title: `${platformLabel} 推送完成`,
        tone: "completed",
      });
      pushActivity(`${platformLabel} 推送完成`, result.message || detail);
      showToast(`已推送到 ${platformLabel}`, "success");
    } finally {
      cloudflarePushing.value = false;
    }
  }

  async function exportCurrentResultsCSV() {
    const visibleRows = currentVisibleResultRows();
    if (visibleRows.length === 0) {
      void navigateTo("results");
      showToast("没有可导出的测速结果", "error");
      return;
    }

    csvExporting.value = true;
    try {
      const defaultFileName = defaultResultsCSVFileName();
      const targetUri = settings.exportTargetUri.trim();
      const targetDir = settings.exportTargetDir.trim();
      if (isAndroidApp() && !targetUri) {
        showToast("请先选择 Android SAF 导出目录", "error");
        return;
      }

      const result = await exportResultsCSV({
        config: buildConfigSnapshot(),
        file_name: defaultFileName,
        results: visibleRows,
        task_id: task.taskId,
        ...(targetDir ? { target_dir: targetDir } : {}),
        ...(targetUri ? { target_uri: targetUri } : {}),
      });
      appendLog("bridge.export_results_csv", result);
      const data = asRecord(result.data);
      if (!result.ok) {
        setStatus({
          detail: result.message || "导出当前测速结果 CSV 失败。",
          title: "CSV 导出失败",
          tone: "failed",
        });
        pushActivity("CSV 导出失败", result.message || "未能写入导出目标。");
        showToast("CSV 导出失败", "error");
        return;
      }

      const writtenCount = asCount(data.written_count || data.writtenCount) || visibleRows.length;
      const resolvedFileName = asString(data.file_name || data.fileName).trim() || defaultFileName;
      const resolvedPath = asString(data.path).trim();
      const resolvedTargetURI = asString(data.target_uri || data.targetUri).trim();
      const targetLabel = resolvedPath || resolvedTargetURI || resolvedFileName;
      setStatus({
        detail: targetLabel ? `已导出 ${writtenCount} 条结果到 ${targetLabel}。` : result.message || `已导出 ${writtenCount} 条结果。`,
        title: "CSV 导出完成",
        tone: "completed",
      });
      pushActivity("CSV 导出完成", targetLabel ? `已导出 ${writtenCount} 条结果到 ${targetLabel}。` : `已导出 ${writtenCount} 条结果。`);
      showToast("已导出当前测速结果 CSV", "success");
    } catch (error) {
      setStatus({
        detail: error instanceof Error ? error.message : "导出当前测速结果 CSV 失败。",
        title: "CSV 导出失败",
        tone: "failed",
      });
      showToast(error instanceof Error ? error.message : "CSV 导出失败", "error");
    } finally {
      csvExporting.value = false;
    }
  }

  async function openHistoryTarget(targetPath: string) {
    if (!targetPath) {
      return;
    }

    await openPath(targetPath);
  }

  // DNSHE 自检固定落在主域名（子域 zone 顶点）下的 cfst-selftest 子域。读取过账号
  // 子域名时按最长后缀求主域名；没读取过时退回完整域名本身。

  // DNSHE 的裸子域名标签按 <标签>.<完整域名> 解释；只有填写完整域名，或账号下已读取到
  // 同名子域时，才按子域选择处理。自检默认测试名与读取目标都要跟着同一结果。

  const dnsPushTargets = computed(() =>
    enabledDNSProviders().map((entry) => ({
      id: entry.id,
      label: entry.label.trim() || `${dnsProviderPlatformLabel(entry.provider)}${dnsProviderTargetLabel(entry) ? ` · ${dnsProviderTargetLabel(entry)}` : ""}`,
      recordTarget: Boolean(entry.recordName.trim() || entry.subdomain.trim()),
    })),
  );

  watch(
    resultCloudflarePushSettings,
    () => {
      if (resultCloudflarePushSettingsHydrated) {
        saveResultCloudflarePushSettings();
      }
    },
    { deep: true },
  );

  function hydrateResultPushSettings() {
    loadResultCloudflarePushSettings();
    loadResultGitHubTopN();
    resultCloudflarePushSettingsHydrated = true;
  }

  return { exportHistory, resultGitHubTopN, resultCloudflarePushSettings, csvExporting, cloudflarePushing, githubExporting, githubTesting, telegramTesting, dnsPushTargets, updateHistory, applyResultCloudflarePushSettings, loadResultCloudflarePushSettings, saveResultCloudflarePushSettings, updateResultCloudflarePushSettings, loadResultGitHubTopN, saveResultGitHubTopN, updateResultGitHubTopN, limitRowsForQuickPush, defaultResultsCSVFileName, testGitHubExportSettings, testTelegramNotificationSettings, exportCurrentResultsToGitHub, resultPushTargetProvider, pushCurrentResultsToCloudflare, exportCurrentResultsCSV, openHistoryTarget, hydrateResultPushSettings };
}
