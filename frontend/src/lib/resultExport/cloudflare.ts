// DNS 推送（结果页快捷推送，多平台）：目标选择、配置覆盖与结果汇报。
import type { Ref } from "vue";
import { normalizeConfigSnapshot, pushDnsRecords, type ProbeResult, type TaskTone } from "../bridge";
import { dnsProviderPlatformLabel } from "../dnsProviderForm";
import type { DnsProviderForm } from "../settingsForm";
import { asBoolean, asCount, asRecord, asString, nonNegativeCount, normalizeResultCloudflareRecordType } from "../valueCoerce";
import type { ToastTone, ViewName } from "../appTypes";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../configSnapshotBuild";
import { limitRowsForQuickPush } from "./rows";
import type { ResultCloudflarePushSettings } from "./types";

export interface CloudflarePushOptions {
  task: { exportPath: string; taskId: string };
  enabledDNSProviders: () => DnsProviderForm[];
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  currentVisibleResultRows: () => ProbeResult[];
  navigateTo: (view: ViewName) => void;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  pushActivity: (title: string, detail: string) => void;
  probeWarnings: Ref<string[]>;
  pushWarningTrace: (warnings: string[], ts?: string) => void;
  resultCloudflarePushSettings: ResultCloudflarePushSettings;
  saveResultCloudflarePushSettings: () => void;
  cloudflarePushing: Ref<boolean>;
}

export function resultPushTargetProvider(options: CloudflarePushOptions) {
  const enabled = options.enabledDNSProviders();
  return enabled.find((entry) => entry.id === options.resultCloudflarePushSettings.providerId) ?? enabled[0];
}

export async function pushCurrentResultsToCloudflare(options: CloudflarePushOptions) {
  const { task, buildConfigSnapshot, currentVisibleResultRows, navigateTo, appendLog, showToast, setStatus, pushActivity, probeWarnings, pushWarningTrace, resultCloudflarePushSettings, saveResultCloudflarePushSettings, cloudflarePushing } = options;
  const visibleRows = currentVisibleResultRows();
  if (visibleRows.length === 0) {
    void navigateTo("results");
    showToast("没有可推送的测速结果", "error");
    return;
  }

  const target = resultPushTargetProvider(options);
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
