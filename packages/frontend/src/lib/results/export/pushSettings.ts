// 结果推送偏好（Cloudflare 推送记录名/类型/TopN 与 GitHub TopN）的本地持久化。
import { reactive, ref, watch } from "vue";
import { asString, nonNegativeCount, normalizeResultCloudflareRecordType } from "../../app/valueCoerce";
import type { DnsProviderForm, SettingsForm } from "../../config/settingsForm";
import type { ResultCloudflarePushSettings } from "./types";

export const RESULT_CLOUDFLARE_PUSH_SETTINGS_KEY = "cfst.result.cloudflarePushSettings.v1";
export const RESULT_GITHUB_TOP_N_KEY = "cfst.result.githubTopN.v1";

export interface ResultPushSettingsOptions {
  settings: SettingsForm;
  appendLog: (event: string, payload: unknown) => void;
  enabledDNSProviders: () => DnsProviderForm[];
  primaryCloudflareProvider: () => DnsProviderForm | undefined;
}

export function createResultPushSettings(options: ResultPushSettingsOptions) {
  const { settings, appendLog, enabledDNSProviders, primaryCloudflareProvider } = options;

  const resultGitHubTopN = ref(20);
  const resultCloudflarePushSettings = reactive<ResultCloudflarePushSettings>({
    providerId: "",
    recordName: "",
    recordType: "ALL",
    topN: 5,
  });
  let resultCloudflarePushSettingsHydrated = false;

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

  return {
    resultCloudflarePushSettings,
    resultGitHubTopN,
    applyResultCloudflarePushSettings,
    loadResultCloudflarePushSettings,
    saveResultCloudflarePushSettings,
    updateResultCloudflarePushSettings,
    loadResultGitHubTopN,
    saveResultGitHubTopN,
    updateResultGitHubTopN,
    hydrateResultPushSettings,
  };
}
