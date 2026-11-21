// DNS 记录读取域（重构计划 §4.2）：读取状态/子域名/记录列表/目标标签解析。
// 从 App.vue 原样迁出；跨域回调经 deps 注入。
import { computed, ref, watch } from "vue";
import { listDnsRecords, listDnsSubdomains, normalizeDnsRecords, type DnsRecordSnapshot, type DNSSubdomainSnapshot, type DnsReadProviderOption, type TaskTone } from "../lib/bridge";
import { dnsProviderFormComplete, dnsProviderPlatformLabel } from "../lib/dnsProviderForm";
import type { DnsProviderForm, SettingsForm } from "../lib/settingsForm";
import { asCount, asRecord, asString } from "../lib/valueCoerce";
import type { ToastTone, ViewName } from "../lib/appTypes";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../lib/configSnapshotBuild";

export interface DnsRecordsDeps {
  settings: SettingsForm;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  pushActivity: (title: string, detail: string) => void;
  navigateTo: (view: ViewName) => void;
  selectedView: () => string;
}

export function useDnsRecords(deps: DnsRecordsDeps) {
  const { settings, buildConfigSnapshot, appendLog, showToast, setStatus, pushActivity, navigateTo } = deps;
  const dnsReadName = ref("");
  const dnsReadProviderId = ref("");
  const dnsReadScope = ref<"zone" | "configured" | "custom">("zone");
  const dnsRecordType = ref<"all" | "A" | "AAAA">("all");
  const dnsRecords = ref<DnsRecordSnapshot[]>([]);
  const dnsSubdomainErrors = ref<Record<string, string>>({});
  const dnsSubdomainsByProvider = ref<Record<string, DNSSubdomainSnapshot[]>>({});
  const dnsSubdomainsLoading = ref<Record<string, boolean>>({});
  const isLoadingDns = ref(false);

  function dnsProviderResolvedRecordName(entry: DnsProviderForm) {
    const recordName = entry.recordName.trim().replace(/\.$/, "");
    const subdomain = entry.subdomain.trim();
    if (entry.provider !== "dnshe" || subdomain === "" || subdomain.includes(".")) {
      return recordName || subdomain;
    }
    const known = dnsSubdomainsByProvider.value[entry.id] ?? [];
    if (known.some((item) => item.subdomain.toLowerCase() === subdomain.toLowerCase() || item.full_domain.toLowerCase() === subdomain.toLowerCase())) {
      return recordName || subdomain;
    }
    if (!recordName) {
      return subdomain;
    }
    if (recordName.toLowerCase().startsWith(`${subdomain.toLowerCase()}.`)) {
      return recordName;
    }
    return `${subdomain}.${recordName}`;
  }

  function dnsProviderZoneName(entry: DnsProviderForm) {
    const target = dnsProviderResolvedRecordName(entry);
    if (entry.provider !== "dnshe") {
      return target;
    }
    const subdomain = entry.subdomain.trim().replace(/\.$/, "");
    if (subdomain.includes(".")) {
      return subdomain.toLowerCase();
    }
    const known = dnsSubdomainsByProvider.value[entry.id] ?? [];
    let best = "";
    for (const item of known) {
      const full = (item.full_domain || `${item.subdomain}.${item.rootdomain}`).trim().replace(/\.$/, "").toLowerCase();
      if (!full) {
        continue;
      }
      if (full === target.toLowerCase() || target.toLowerCase().endsWith(`.${full}`)) {
        if (full.length > best.length) {
          best = full;
        }
      }
    }
    return best || target;
  }

  function dnsProviderTargetLabel(entry: DnsProviderForm) {
    const recordName = entry.recordName.trim();
    if (recordName) {
      return dnsProviderResolvedRecordName(entry);
    }
    if (entry.provider === "dnshe" && entry.subdomain.trim()) {
      return `子域名 ${entry.subdomain.trim()}`;
    }
    return "";
  }

  async function loadDnsProviderSubdomains(providerId: string, silent = false) {
    const provider = settings.dnsProviders.find((entry) => entry.id === providerId);
    if (!provider || provider.provider !== "dnshe") {
      return;
    }
    dnsSubdomainsLoading.value = { ...dnsSubdomainsLoading.value, [providerId]: true };
    dnsSubdomainErrors.value = { ...dnsSubdomainErrors.value, [providerId]: "" };
    try {
      const result = await listDnsSubdomains({
        config: buildConfigSnapshot(),
        provider_id: providerId,
      });
      appendLog("bridge.list_dns_subdomains", result);
      if (!result.ok) {
        dnsSubdomainErrors.value = { ...dnsSubdomainErrors.value, [providerId]: result.message || "读取 DNSHE 子域名失败。" };
        if (!silent) {
          showToast(result.message || "读取 DNSHE 子域名失败", "error");
        }
        return;
      }
      const data = asRecord(result.data);
      const items = Array.isArray(data.subdomains)
        ? data.subdomains.map((item) => {
            const record = asRecord(item);
            return {
              full_domain: asString(record.full_domain),
              id: asCount(record.id, 0),
              rootdomain: asString(record.rootdomain),
              status: asString(record.status),
              subdomain: asString(record.subdomain),
            };
          })
        : [];
      dnsSubdomainsByProvider.value = { ...dnsSubdomainsByProvider.value, [providerId]: items };
      if (!silent) {
        showToast(items.length > 0 ? `已读取 ${items.length} 个 DNSHE 子域名` : "该账号下没有可用子域名", items.length > 0 ? "success" : "info");
      }
    } finally {
      dnsSubdomainsLoading.value = { ...dnsSubdomainsLoading.value, [providerId]: false };
    }
  }

  async function fetchDnsRecords() {
    if (dnsReadScope.value === "custom" && !dnsReadName.value.trim()) {
      void navigateTo("dns");
      showToast("请输入要读取的子域名或记录名", "error");
      return;
    }

    isLoadingDns.value = true;

    try {
      const payload: Record<string, unknown> = {
        config: buildConfigSnapshot(),
        scope: dnsReadScope.value,
      };
      if (dnsReadProviderId.value.trim()) {
        payload.provider_id = dnsReadProviderId.value.trim();
      }
      if (dnsReadScope.value === "custom") {
        payload.name = dnsReadName.value.trim();
      }
      if (dnsRecordType.value !== "all") {
        payload.record_type = dnsRecordType.value;
      }

      const result = await listDnsRecords(payload);
      const data = asRecord(result.data);
      appendLog("bridge.list_dns_records", result);
      if (!result.ok) {
        setStatus({
          detail: result.message || "读取 DNS 记录失败。",
          title: "DNS 读取失败",
          tone: "failed",
        });
        pushActivity("读取 DNS 失败", result.message || "未能读取所选平台的 DNS 记录。");
        showToast("读取 DNS 失败", "error");
        return;
      }

      dnsRecords.value = normalizeDnsRecords(data.records);
      pushActivity("DNS 已同步", result.message || `共加载 ${dnsRecords.value.length} 条记录。`);
      showToast(dnsRecords.value.length > 0 ? `已同步 ${dnsRecords.value.length} 条记录` : "当前没有匹配记录", "info");
    } finally {
      isLoadingDns.value = false;
    }
  }

  const dnsReadProviders = computed<DnsReadProviderOption[]>(() =>
    settings.dnsProviders.map((entry) => ({
      enabled: entry.enabled,
      id: entry.id,
      label: entry.label.trim() || `${dnsProviderPlatformLabel(entry.provider)}${dnsProviderTargetLabel(entry) ? ` · ${dnsProviderTargetLabel(entry)}` : ""}`,
      provider: entry.provider,
      recordName: dnsProviderResolvedRecordName(entry),
      status: entry.enabled ? (dnsProviderFormComplete(entry) ? "已就绪" : "待补全") : "已停用",
    })),
  );

  watch(
    dnsReadProviders,
    (providers) => {
      if (providers.some((provider) => provider.id === dnsReadProviderId.value)) {
        return;
      }
      const ready = providers.find((provider) => provider.status === "已就绪");
      dnsReadProviderId.value = ready?.id || providers[0]?.id || "";
    },
    { immediate: true },
  );

  watch(
    () =>
      deps.selectedView() === "dns"
        ? settings.dnsProviders
            .filter((entry) => entry.enabled && entry.provider === "dnshe")
            .map((entry) => entry.id)
            .join(",")
        : "",
    (ids) => {
      if (!ids) {
        return;
      }
      for (const id of ids.split(",")) {
        if (!dnsSubdomainsByProvider.value[id] && !dnsSubdomainsLoading.value[id]) {
          void loadDnsProviderSubdomains(id, true);
        }
      }
    },
    { immediate: true },
  );

  watch(dnsReadProviderId, () => {
    dnsRecords.value = [];
  });

  return {
    dnsReadName,
    dnsReadProviderId,
    dnsReadScope,
    dnsRecordType,
    dnsRecords,
    dnsSubdomainErrors,
    dnsSubdomainsByProvider,
    dnsSubdomainsLoading,
    isLoadingDns,
    dnsReadProviders,
    dnsProviderTargetLabel,
    dnsProviderZoneName,
    dnsProviderResolvedRecordName,
    fetchDnsRecords,
    loadDnsProviderSubdomains,
  };
}
