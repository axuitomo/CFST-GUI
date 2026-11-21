// 结果导出域编排（重构计划 §4.2）：状态与返回值在这里装配，CSV/GitHub/DNS 推送实现见 lib/resultExport/*。
import { computed, ref } from "vue";
import { openPath, testTelegramNotification } from "../lib/bridge";
import { dnsProviderPlatformLabel } from "../lib/dnsProviderForm";
import type { HistoryEntry } from "../lib/appTypes";
import { createResultPushSettings } from "../lib/resultExport/pushSettings";
import { exportCurrentResultsToGitHub as exportCurrentResultsToGitHubImpl, testGitHubExportSettings as testGitHubExportSettingsImpl, type GithubExportOptions } from "../lib/resultExport/github";
import { pushCurrentResultsToCloudflare as pushCurrentResultsToCloudflareImpl, resultPushTargetProvider as resultPushTargetProviderImpl, type CloudflarePushOptions } from "../lib/resultExport/cloudflare";
import { defaultResultsCSVFileName as defaultResultsCSVFileNameImpl, exportCurrentResultsCSV as exportCurrentResultsCSVImpl, type CsvExportOptions } from "../lib/resultExport/csv";
import { limitRowsForQuickPush } from "../lib/resultExport/rows";
import type { ResultExportDeps } from "../lib/resultExport/types";

export type { ResultCloudflarePushSettings, ResultExportDeps } from "../lib/resultExport/types";

export function useResultExport(deps: ResultExportDeps) {
  const { settings, task, isAndroidApp, enabledDNSProviders, primaryCloudflareProvider, dnsProviderTargetLabel, buildConfigSnapshot, currentVisibleResultRows, pathLeafName, probeWarnings, pushWarningTrace, navigateTo, appendLog, showToast, setStatus, pushActivity } = deps;

  const pushSettings = createResultPushSettings({ appendLog, enabledDNSProviders, primaryCloudflareProvider, settings });
  const { resultCloudflarePushSettings, resultGitHubTopN } = pushSettings;

  const EXPORT_HISTORY_LIMIT = 50;
  const exportHistory = ref<HistoryEntry[]>([]);

  const csvExporting = ref(false);
  const cloudflarePushing = ref(false);
  const githubExporting = ref(false);
  const githubTesting = ref(false);
  const telegramTesting = ref(false);

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

  const dnsPushTargets = computed(() =>
    enabledDNSProviders().map((entry) => ({
      id: entry.id,
      label: entry.label.trim() || `${dnsProviderPlatformLabel(entry.provider)}${dnsProviderTargetLabel(entry) ? ` · ${dnsProviderTargetLabel(entry)}` : ""}`,
      recordTarget: Boolean(entry.recordName.trim() || entry.subdomain.trim()),
    })),
  );

  const csvOptions: CsvExportOptions = { appendLog, buildConfigSnapshot, csvExporting, currentVisibleResultRows, isAndroidApp, navigateTo, pathLeafName, pushActivity, setStatus, settings, showToast, task };
  const githubOptions: GithubExportOptions = { appendLog, buildConfigSnapshot, currentVisibleResultRows, githubExporting, githubTesting, navigateTo, pushActivity, resultGitHubTopN, setStatus, settings, showToast, task };
  const cloudflareOptions: CloudflarePushOptions = {
    appendLog,
    buildConfigSnapshot,
    cloudflarePushing,
    currentVisibleResultRows,
    enabledDNSProviders,
    navigateTo,
    probeWarnings,
    pushActivity,
    pushWarningTrace,
    resultCloudflarePushSettings,
    saveResultCloudflarePushSettings: pushSettings.saveResultCloudflarePushSettings,
    setStatus,
    showToast,
    task,
  };

  async function openHistoryTarget(targetPath: string) {
    if (!targetPath) {
      return;
    }

    await openPath(targetPath);
  }

  return {
    exportHistory,
    resultGitHubTopN,
    resultCloudflarePushSettings,
    csvExporting,
    cloudflarePushing,
    githubExporting,
    githubTesting,
    telegramTesting,
    dnsPushTargets,
    updateHistory,
    applyResultCloudflarePushSettings: pushSettings.applyResultCloudflarePushSettings,
    loadResultCloudflarePushSettings: pushSettings.loadResultCloudflarePushSettings,
    saveResultCloudflarePushSettings: pushSettings.saveResultCloudflarePushSettings,
    updateResultCloudflarePushSettings: pushSettings.updateResultCloudflarePushSettings,
    loadResultGitHubTopN: pushSettings.loadResultGitHubTopN,
    saveResultGitHubTopN: pushSettings.saveResultGitHubTopN,
    updateResultGitHubTopN: pushSettings.updateResultGitHubTopN,
    limitRowsForQuickPush,
    defaultResultsCSVFileName: () => defaultResultsCSVFileNameImpl(csvOptions),
    testGitHubExportSettings: () => testGitHubExportSettingsImpl(githubOptions),
    testTelegramNotificationSettings,
    exportCurrentResultsToGitHub: () => exportCurrentResultsToGitHubImpl(githubOptions),
    resultPushTargetProvider: () => resultPushTargetProviderImpl(cloudflareOptions),
    pushCurrentResultsToCloudflare: () => pushCurrentResultsToCloudflareImpl(cloudflareOptions),
    exportCurrentResultsCSV: () => exportCurrentResultsCSVImpl(csvOptions),
    openHistoryTarget,
    hydrateResultPushSettings: pushSettings.hydrateResultPushSettings,
  };
}
