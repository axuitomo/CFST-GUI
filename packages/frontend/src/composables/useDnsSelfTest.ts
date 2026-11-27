// DNS 自检与 DoH 测试域（重构计划 §4.2）：从 App.vue 原样迁出。
import { computed, ref, type Ref } from "vue";
import { runDnsSelfTest, testDoH, type DnsSelfTestProviderOption, type DnsSelfTestResult, type TaskTone } from "../lib/bridge";
import { dnsProviderPlatformLabel } from "../lib/dns/providerForm";
import type { DnsProviderForm, SettingsForm } from "../lib/config/settingsForm";
import { asBoolean, asCount, asRecord, asString } from "../lib/app/valueCoerce";
import type { ToastTone } from "../lib/app/types";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../lib/config/snapshotBuild";

export interface DnsSelfTestDeps {
  settings: SettingsForm;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  enabledDNSProviders: () => DnsProviderForm[];
  targetLabel: (entry: DnsProviderForm) => string;
  resolvedRecordName: (entry: DnsProviderForm) => string;
  zoneName: (entry: DnsProviderForm) => string;
  probeWarnings: Ref<string[]>;
  pushWarningTrace: (warnings: string[]) => void;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  pushActivity: (title: string, detail: string) => void;
}

export function useDnsSelfTest(deps: DnsSelfTestDeps) {
  const { settings, buildConfigSnapshot, enabledDNSProviders, probeWarnings, pushWarningTrace, appendLog, showToast, setStatus, pushActivity } = deps;
  const dnsSelfTesting = ref(false);
  const dnsSelfTestResult = ref<DnsSelfTestResult | null>(null);
  const dohTesting = ref(false);
  const dohTestResult = ref<{ message: string; ok: boolean } | null>(null);

  function normalizeDnsSelfTestStatus(value: unknown): "passed" | "failed" | "skipped" {
    const status = asString(value).trim();
    return status === "passed" || status === "failed" ? status : "skipped";
  }

  function normalizeDnsSelfTestResult(data: Record<string, unknown>): DnsSelfTestResult {
    const stages = Array.isArray(data.stages)
      ? data.stages.map((item) => {
          const stage = asRecord(item);
          return {
            details: asRecord(stage.details),
            duration_ms: asCount(stage.duration_ms),
            id: asString(stage.id),
            message: asString(stage.message),
            status: normalizeDnsSelfTestStatus(stage.status),
            title: asString(stage.title),
          };
        })
      : [];
    return {
      alt_ttl: asCount(data.alt_ttl),
      base_ttl: asCount(data.base_ttl),
      cleanup_ok: asBoolean(data.cleanup_ok, true),
      failed: asCount(data.failed),
      passed: asCount(data.passed),
      provider: asString(data.provider),
      provider_id: asString(data.provider_id),
      record_name: asString(data.record_name),
      skipped: asCount(data.skipped),
      stages,
      test_name: asString(data.test_name),
      warnings: Array.isArray(data.warnings) ? data.warnings.map((warning) => asString(warning)).filter(Boolean) : [],
      wrote_records: asBoolean(data.wrote_records, true),
    };
  }

  async function runDNSSelfTest(payload: { providerId: string; testName: string }) {
    if (dnsSelfTesting.value) {
      return;
    }
    const provider = enabledDNSProviders().find((entry) => entry.id === payload.providerId);
    if (!provider) {
      showToast("请先在设置页添加并启用 DNS 推送平台", "error");
      return;
    }
    const platformLabel = dnsProviderPlatformLabel(provider.provider);

    dnsSelfTesting.value = true;
    dnsSelfTestResult.value = null;
    try {
      const result = await runDnsSelfTest({
        config: buildConfigSnapshot(),
        provider_id: payload.providerId,
        test_name: payload.testName,
      });
      appendLog("bridge.dns_selftest", result);
      const normalized = normalizeDnsSelfTestResult(asRecord(result.data));
      dnsSelfTestResult.value = normalized;
      probeWarnings.value = result.warnings || [];
      pushWarningTrace(probeWarnings.value);

      if (!result.ok) {
        const detail = result.message || `${platformLabel} 自检未通过。`;
        setStatus({
          detail,
          title: `${platformLabel} 自检失败`,
          tone: "failed",
        });
        pushActivity(`${platformLabel} 自检失败`, detail);
        showToast(`${platformLabel} 自检失败`, "error");
        return;
      }

      const detail = result.message || `${platformLabel} 自检通过。`;
      setStatus({
        detail,
        title: normalized.skipped > 0 ? `${platformLabel} 自检部分通过` : `${platformLabel} 自检通过`,
        tone: "completed",
      });
      pushActivity(normalized.skipped > 0 ? `${platformLabel} 自检部分通过` : `${platformLabel} 自检通过`, detail);
      showToast(normalized.skipped > 0 ? `${platformLabel} 自检部分通过` : `${platformLabel} 自检通过`, "success");
    } catch (error) {
      const message = error instanceof Error ? error.message : "DNS 自检失败";
      setStatus({ detail: message, title: `${platformLabel} 自检失败`, tone: "failed" });
      pushActivity(`${platformLabel} 自检失败`, message);
      showToast(`${platformLabel} 自检失败`, "error");
    } finally {
      dnsSelfTesting.value = false;
    }
  }

  function probeURLHostName() {
    const raw = settings.probeURL.trim();
    if (!raw) {
      return "";
    }
    try {
      return new URL(raw).hostname;
    } catch {
      return "";
    }
  }

  async function testDohConnection() {
    if (dohTesting.value) {
      return;
    }
    dohTesting.value = true;
    try {
      const result = await testDoH({
        doh_url: settings.probeDoHURL.trim(),
        ech: settings.probeECH,
        ech_config: settings.probeECHConfig.trim(),
        ech_domain: settings.probeECHDomain.trim(),
        host: probeURLHostName(),
      });
      appendLog("bridge.doh_test", result);
      const warnings = (result.warnings || []).join(" ");
      dohTestResult.value = {
        message: [result.message || (result.ok ? "DoH 可用。" : "DoH 测试失败。"), warnings].filter(Boolean).join(" "),
        ok: result.ok,
      };
      if (!result.ok) {
        setStatus({ detail: result.message || "DoH 端点没有返回有效答案。", title: "DoH 测试失败", tone: "failed" });
        pushActivity("DoH 测试失败", result.message || "DoH 端点没有返回有效答案。");
        showToast("DoH 测试失败", "error");
        return;
      }
      pushActivity("DoH 测试通过", result.message || "DoH 端点可用。");
      showToast("DoH 测试通过", "info");
    } catch (error) {
      const message = error instanceof Error ? error.message : "DoH 测试失败";
      dohTestResult.value = { message, ok: false };
      showToast(message, "error");
    } finally {
      dohTesting.value = false;
    }
  }

  const dnsSelfTestProviders = computed<DnsSelfTestProviderOption[]>(() =>
    enabledDNSProviders().map((entry) => ({
      id: entry.id,
      label: entry.label.trim() || `${dnsProviderPlatformLabel(entry.provider)}${deps.targetLabel(entry) ? ` · ${deps.targetLabel(entry)}` : ""}`,
      provider: entry.provider,
      recordName: deps.resolvedRecordName(entry),
      zoneName: deps.zoneName(entry),
    })),
  );

  return {
    dnsSelfTesting,
    dnsSelfTestResult,
    dohTesting,
    dohTestResult,
    dnsSelfTestProviders,
    runDNSSelfTest,
    testDohConnection,
  };
}
