<script setup vapor lang="ts">
import { computed, ref } from "vue";
import { PhArrowsClockwise, PhCaretDown, PhCloud, PhEye, PhEyeSlash } from "@phosphor-icons/vue";
import type { DNSSubdomainSnapshot } from "../../lib/bridge";
import type { CloudflareRoutingRuleForm, DNSProviderKind, DnsProviderForm, SettingsForm } from "../../lib/config/settingsForm";
import { dnsProviderFormComplete } from "../../lib/dns/providerForm";
import { DNS_PROVIDER_KINDS, dnsProviderCredentialLabels, dnsProviderDefaultTTL, dnsProviderPlatformLabel, dnsProviderRecordNameHint, dnsProviderSpec } from "../../lib/dns/providerRegistry";
import { TTL_OPTIONS, type SettingsSectionController } from "./types";

const props = defineProps<{
  dnsSubdomainErrors: Record<string, string>;
  dnsSubdomains: Record<string, DNSSubdomainSnapshot[]>;
  dnsSubdomainsLoading: Record<string, boolean>;
  sections: SettingsSectionController;
  settings: SettingsForm;
}>();

defineEmits<{
  (event: "load-dns-subdomains", providerId: string): void;
}>();

const collapsedCloudflareRoutingRuleIds = ref(new Set<string>());
const expandedDnsProviderIds = ref(new Set<string>());
const visibleDnsProviderSecrets = ref(new Set<string>());

const enabledDnsProviderCount = computed(() => props.settings.dnsProviders.filter((entry) => entry.enabled).length);
const cloudflareConfigLabel = computed(() => {
  const total = props.settings.dnsProviders.length;
  if (total === 0) return "未添加平台";
  return `${enabledDnsProviderCount.value}/${total} 个平台启用`;
});

function toggleCloudflareRoutingRule(id: string) {
  const collapsed = collapsedCloudflareRoutingRuleIds.value;
  if (collapsed.has(id)) collapsed.delete(id);
  else collapsed.add(id);
}

function createCloudflareRoutingRule(provider: DnsProviderForm): CloudflareRoutingRuleForm {
  const index = provider.routingRules.length + 1;
  return {
    enabled: true,
    filterMode: "allow",
    filterTokens: "",
    id: `${provider.id}-route-${Date.now()}-${index}`,
    name: `分流规则 ${index}`,
    recordName: "",
    recordType: "A",
    topN: 5,
  };
}

function addCloudflareRoutingRule(provider: DnsProviderForm) {
  provider.routingRules.push(createCloudflareRoutingRule(provider));
}

function removeCloudflareRoutingRule(provider: DnsProviderForm, index: number) {
  provider.routingRules.splice(index, 1);
}

function createDnsProviderForm(provider: DNSProviderKind): DnsProviderForm {
  return {
    allowCNAME: false,
    apiKey: "",
    apiSecret: "",
    apiToken: "",
    enabled: true,
    id: `${provider}-${Date.now().toString(36)}`,
    label: dnsProviderPlatformLabel(provider),
    maskedKey: "",
    maskedSecret: "",
    maskedToken: "",
    provider,
    recordName: "",
    recordType: "A",
    routingEnabled: false,
    routingRules: [],
    subdomain: "",
    subdomainId: 0,
    topN: 5,
    ttl: dnsProviderDefaultTTL(provider),
    zoneId: "",
  };
}

function addDnsProvider(provider: DNSProviderKind) {
  const form = createDnsProviderForm(provider);
  props.settings.dnsProviders.push(form);
  expandedDnsProviderIds.value.add(form.id);
}

function removeDnsProvider(index: number) {
  const [removed] = props.settings.dnsProviders.splice(index, 1);
  if (removed) expandedDnsProviderIds.value.delete(removed.id);
}

function dnsProviderCardExpanded(id: string) {
  return expandedDnsProviderIds.value.has(id);
}

function toggleDnsProviderCard(id: string) {
  const expanded = expandedDnsProviderIds.value;
  if (expanded.has(id)) expanded.delete(id);
  else expanded.add(id);
}

function dnsProviderStatusText(entry: DnsProviderForm) {
  if (!entry.enabled) return "已停用";
  return dnsProviderFormComplete(entry) ? "已就绪" : "待补全";
}

function dnsProviderStatusClass(entry: DnsProviderForm) {
  if (!entry.enabled) return "bg-slate-100 text-slate-500";
  return dnsProviderFormComplete(entry) ? "bg-emerald-50 text-emerald-700" : "bg-amber-50 text-amber-700";
}

function dnsSubdomainOptions(providerId: string): DNSSubdomainSnapshot[] {
  return props.dnsSubdomains[providerId] ?? [];
}

function dnsSubdomainSelected(entry: DnsProviderForm): DNSSubdomainSnapshot | undefined {
  const value = entry.subdomain.trim().toLowerCase();
  if (!value) return undefined;
  return dnsSubdomainOptions(entry.id).find((item) => item.subdomain.toLowerCase() === value || item.full_domain.toLowerCase() === value);
}

function dnsComposedTarget(entry: DnsProviderForm) {
  if (dnsSubdomainSelected(entry)) return "";
  const value = entry.subdomain.trim();
  const base = entry.recordName.trim().replace(/\.$/, "");
  if (!value || value.includes(".") || !base) return "";
  if (base.toLowerCase().startsWith(`${value.toLowerCase()}.`)) return base;
  return `${value}.${base}`;
}

function dnsProviderTargetSummary(entry: DnsProviderForm) {
  const composed = dnsProviderSpec(entry.provider).supportsSubdomain ? dnsComposedTarget(entry) : "";
  if (composed) return composed;
  const recordName = entry.recordName.trim();
  if (recordName) return recordName;
  const subdomain = entry.subdomain.trim();
  if (dnsProviderSpec(entry.provider).supportsSubdomain && subdomain) return `子域名 ${subdomain}`;
  return "尚未填写记录名";
}

function dnsSubdomainPreview(entry: DnsProviderForm) {
  const selected = dnsSubdomainSelected(entry);
  if (selected) return selected.full_domain;
  const value = entry.subdomain.trim();
  if (value.includes(".")) return value;
  if (value) {
    if (!entry.recordName.trim()) return `${value}（需填写完整域名）`;
    return dnsComposedTarget(entry) || `${value}（保存后解析）`;
  }
  return entry.recordName.trim() || "保存后按账号下的子域名自动匹配";
}

function dnsProviderSecretsVisible(key: string) {
  return visibleDnsProviderSecrets.value.has(key);
}

function toggleDnsProviderSecrets(key: string) {
  const visible = visibleDnsProviderSecrets.value;
  if (visible.has(key)) visible.delete(key);
  else visible.add(key);
}

function dnsProviderTitle(entry: DnsProviderForm, index: number) {
  if (entry.label.trim()) return entry.label.trim();
  const platform = dnsProviderPlatformLabel(entry.provider);
  const target = entry.recordName.trim() || (dnsProviderSpec(entry.provider).supportsSubdomain && entry.subdomain.trim() ? `子域名 ${entry.subdomain.trim()}` : "");
  return target ? `${platform} · ${target}` : `${platform} 平台 ${index + 1}`;
}
</script>

<template>
  <details :open="sections.isOpen('cloudflare')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('cloudflare', $event)">
    <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
      <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
        <PhCloud class="mr-2 shrink-0 text-cf" size="20" weight="fill" />
        DNS 推送平台
      </h3>
      <div class="flex shrink-0 items-center gap-3">
        <span class="ui-pill ui-pill-subtle">{{ cloudflareConfigLabel }}</span>
        <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('cloudflare') ? 'rotate-180' : ''" size="18" />
      </div>
    </summary>
    <div v-if="sections.isMounted('cloudflare')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
      <p class="text-sm text-slate-500">每个平台独立保存凭据、记录名称、记录类型、TTL 和 Top N。测速后自动推送、定时任务和手动推送都会写入全部已启用平台；凭据与目标完整时自动启用。</p>
      <div class="flex flex-wrap gap-2">
        <button v-for="kind in DNS_PROVIDER_KINDS" :key="kind" type="button" class="ui-button ui-button-secondary" @click="addDnsProvider(kind)">添加 {{ dnsProviderPlatformLabel(kind) }} 平台</button>
      </div>
      <div v-if="settings.dnsProviders.length === 0" class="rounded-xl border border-dashed border-sky-200 bg-sky-50/70 px-4 py-3 text-sm text-slate-500">还没有 DNS 平台。添加平台并填完整凭据后才会执行自动 DNS 推送。</div>

      <div v-for="(entry, providerIndex) in settings.dnsProviders" :key="entry.id" class="rounded-xl border border-slate-200 bg-slate-50/70">
        <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
          <button type="button" class="flex min-w-0 flex-1 basis-[13rem] items-center gap-2 text-left" :aria-expanded="dnsProviderCardExpanded(entry.id)" :aria-label="`${dnsProviderCardExpanded(entry.id) ? '折叠' : '展开'} ${dnsProviderTitle(entry, providerIndex)}`" @click="toggleDnsProviderCard(entry.id)">
            <span class="min-w-0 flex-1">
              <span class="flex min-w-0 items-center gap-2">
                <span class="min-w-0 truncate text-sm font-semibold text-slate-700">{{ dnsProviderTitle(entry, providerIndex) }}</span>
                <span class="ui-pill shrink-0" :class="dnsProviderStatusClass(entry)">{{ dnsProviderStatusText(entry) }}</span>
              </span>
              <span class="block truncate text-xs text-slate-500">{{ dnsProviderPlatformLabel(entry.provider) }} · {{ dnsProviderTargetSummary(entry) }}</span>
            </span>
            <PhCaretDown class="shrink-0 text-slate-400 transition" :class="dnsProviderCardExpanded(entry.id) ? 'rotate-180' : ''" size="16" />
          </button>
          <div class="flex shrink-0 items-center gap-4">
            <label class="flex items-center gap-2 text-sm text-slate-600">
              <input v-model="entry.enabled" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              启用
            </label>
            <button type="button" class="text-sm font-semibold text-rose-600 hover:text-rose-700" @click="removeDnsProvider(providerIndex)">删除</button>
          </div>
        </div>

        <div v-show="dnsProviderCardExpanded(entry.id)" class="space-y-3 border-t border-slate-200 p-4">
          <div class="grid gap-3 md:grid-cols-2">
            <label>
              <span class="ui-label">平台备注名</span>
              <input v-model="entry.label" type="text" class="ui-field" placeholder="主域名 / 备用域名" />
            </label>
            <label>
              <span class="ui-label">{{ dnsProviderSpec(entry.provider).tokenOnly ? "记录名称" : "完整域名" }}</span>
              <input v-model="entry.recordName" :placeholder="dnsProviderSpec(entry.provider).tokenOnly ? 'edge.example.com' : 'api.example.com'" type="text" class="ui-field font-mono" />
              <p v-if="!dnsProviderSpec(entry.provider).tokenOnly" class="mt-2 text-xs text-slate-500">{{ dnsProviderRecordNameHint(entry.provider) }}</p>
            </label>

            <template v-if="!dnsProviderSpec(entry.provider).tokenOnly">
              <label>
                <span class="ui-label">{{ dnsProviderCredentialLabels(entry.provider).idLabel }}</span>
                <input v-model="entry.apiKey" :placeholder="entry.maskedKey || dnsProviderCredentialLabels(entry.provider).idPlaceholder" :type="dnsProviderSecretsVisible(entry.id) ? 'text' : 'password'" class="ui-field font-mono" autocomplete="off" />
              </label>
              <label>
                <span class="ui-label">{{ dnsProviderCredentialLabels(entry.provider).secretLabel }}</span>
                <input v-model="entry.apiSecret" :placeholder="entry.maskedSecret || dnsProviderCredentialLabels(entry.provider).secretHint" :type="dnsProviderSecretsVisible(entry.id) ? 'text' : 'password'" class="ui-field font-mono" autocomplete="off" />
              </label>
              <label v-if="dnsProviderSpec(entry.provider).supportsSubdomain" class="md:col-span-2">
                <span class="ui-label">子域名（可选）</span>
                <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:gap-3">
                  <input v-model="entry.subdomain" :list="`dnshe-subdomains-${entry.id}`" class="ui-field font-mono" placeholder="test" type="text" />
                  <datalist :id="`dnshe-subdomains-${entry.id}`">
                    <option v-for="item in dnsSubdomainOptions(entry.id)" :key="item.id" :label="`${item.full_domain} · ${item.status}`" :value="item.subdomain" />
                  </datalist>
                  <button type="button" class="ui-button ui-button-secondary px-4" :disabled="props.dnsSubdomainsLoading[entry.id]" @click="$emit('load-dns-subdomains', entry.id)">
                    <PhArrowsClockwise :class="{ 'animate-spin': props.dnsSubdomainsLoading[entry.id] }" size="16" />
                    {{ props.dnsSubdomainsLoading[entry.id] ? "读取中" : "读取子域名" }}
                  </button>
                </div>
                <p class="mt-2 text-xs text-slate-500">填子域名标签（如 <span class="font-mono">test</span>）或完整域名；账号下没有同名子域时，标签会解析为 <span class="font-mono">test.&lt;上方完整域名&gt;</span>，留空时按上方完整域名匹配。</p>
                <div class="mt-2 flex flex-wrap items-center gap-2 text-xs">
                  <span class="text-slate-500">当前目标：</span>
                  <span class="ui-pill bg-sky-50 font-mono text-sky-700">{{ dnsSubdomainPreview(entry) }}</span>
                  <span v-if="dnsSubdomainSelected(entry)" class="ui-pill bg-emerald-50 text-emerald-700">{{ dnsSubdomainSelected(entry)?.status || "已选择" }}</span>
                </div>
                <p v-if="dnsSubdomainOptions(entry.id).length > 0" class="mt-2 text-xs text-slate-500">已读取 {{ dnsSubdomainOptions(entry.id).length }} 个子域名，可在输入框中输入标签或从下拉提示中选择。</p>
                <p v-if="props.dnsSubdomainErrors[entry.id]" class="mt-2 text-xs text-rose-600">{{ props.dnsSubdomainErrors[entry.id] }}</p>
              </label>
              <div class="flex items-end">
                <button type="button" class="ui-button ui-button-ghost px-4" @click="toggleDnsProviderSecrets(entry.id)">
                  <component :is="dnsProviderSecretsVisible(entry.id) ? PhEyeSlash : PhEye" :size="18" />
                  {{ dnsProviderSecretsVisible(entry.id) ? "隐藏凭据" : "显示凭据" }}
                </button>
              </div>
            </template>

            <template v-else>
              <label class="md:col-span-2">
                <span class="ui-label">API Token</span>
                <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:gap-3">
                  <input v-model="entry.apiToken" :placeholder="entry.maskedToken || '重新输入完整 Token 以保存'" :type="dnsProviderSecretsVisible(entry.id) ? 'text' : 'password'" class="ui-field font-mono" autocomplete="off" />
                  <button type="button" class="ui-button ui-button-ghost px-4" @click="toggleDnsProviderSecrets(entry.id)">
                    <component :is="dnsProviderSecretsVisible(entry.id) ? PhEyeSlash : PhEye" :size="18" />
                    {{ dnsProviderSecretsVisible(entry.id) ? "隐藏 Token" : "显示 Token" }}
                  </button>
                </div>
              </label>
              <label>
                <span class="ui-label">Zone ID</span>
                <input v-model="entry.zoneId" type="text" class="ui-field font-mono" />
              </label>
              <div class="flex items-center rounded-xl border border-slate-200 bg-white/70 px-4 py-3 text-sm text-slate-500">IPv4 写入 A，IPv6 写入 AAAA；上传固定使用灰色解析。</div>
            </template>

            <label>
              <span class="ui-label">记录类型</span>
              <select v-model="entry.recordType" class="ui-field">
                <option v-if="dnsProviderSpec(entry.provider).supportsRecordTypeAll" value="ALL">ALL / A + AAAA</option>
                <option value="A">A / IPv4</option>
                <option value="AAAA">AAAA / IPv6</option>
              </select>
            </label>
            <label v-if="dnsProviderSpec(entry.provider).tokenOnly">
              <span class="ui-label">TTL</span>
              <select v-model.number="entry.ttl" class="ui-field">
                <option v-for="option in TTL_OPTIONS" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </label>
            <label v-else>
              <span class="ui-label">TTL（秒）</span>
              <input v-model.number="entry.ttl" min="60" type="number" class="ui-field" />
            </label>
            <label>
              <span class="ui-label">Top N</span>
              <input v-model.number="entry.topN" min="0" type="number" class="ui-field" />
              <p class="mt-2 text-xs text-slate-500">0 表示不限数量；共享上传筛选仍会先执行。</p>
            </label>
            <label class="flex items-start gap-3 rounded-xl border border-amber-200 bg-amber-50/60 px-4 py-3 md:col-span-2">
              <input v-model="entry.allowCNAME" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              <span class="min-w-0">
                <span class="block text-sm font-semibold text-slate-800">允许修改 CNAME</span>
                <span class="text-xs text-slate-500">默认关闭。关闭时，目标记录名上存在 CNAME 会拒绝本次推送，不会删除别名；开启后按覆盖语义先删除 CNAME 再写入 A/AAAA。</span>
              </span>
            </label>
          </div>

          <div class="space-y-3 border-t border-slate-200 pt-3">
            <label class="flex items-start gap-3">
              <input v-model="entry.routingEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              <span class="min-w-0">
                <span class="block text-sm font-semibold text-slate-800">启用分流设置</span>
                <span class="text-xs text-slate-500">测速结果先经过共享上传筛选，再按国家/COLO 规则分别覆盖推送到指定 DNS 记录名。所有 DNS 平台都支持。</span>
              </span>
            </label>
            <p class="text-xs text-slate-500">国家/COLO 筛选词支持 JP,HKG,NRT,US,UK；国家码遵循 ISO 3166-1 alpha-2，UK 兼容为 GB。</p>
            <div v-if="entry.routingRules.length > 0" class="space-y-3">
              <div v-for="(rule, ruleIndex) in entry.routingRules" :key="rule.id" class="rounded-xl border border-slate-200 bg-white/70 p-4">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <label class="flex items-center gap-2 text-sm font-semibold text-slate-700">
                    <input v-model="rule.enabled" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                    启用规则
                  </label>
                  <span v-if="collapsedCloudflareRoutingRuleIds.has(rule.id)" class="min-w-0 truncate text-sm text-slate-600">{{ rule.name || `规则 ${ruleIndex + 1}` }}</span>
                  <div class="flex items-center gap-3">
                    <button
                      type="button"
                      class="flex items-center gap-1 text-sm font-medium text-slate-600 hover:text-slate-800"
                      :aria-expanded="!collapsedCloudflareRoutingRuleIds.has(rule.id)"
                      :aria-label="collapsedCloudflareRoutingRuleIds.has(rule.id) ? '展开规则' : '折叠规则'"
                      @click="toggleCloudflareRoutingRule(rule.id)"
                    >
                      <PhCaretDown class="transition" :class="collapsedCloudflareRoutingRuleIds.has(rule.id) ? '' : 'rotate-180'" size="16" />
                      {{ collapsedCloudflareRoutingRuleIds.has(rule.id) ? "展开" : "折叠" }}
                    </button>
                    <button type="button" class="text-sm font-semibold text-rose-600 hover:text-rose-700" @click="removeCloudflareRoutingRule(entry, ruleIndex)">删除</button>
                  </div>
                </div>
                <div v-if="!collapsedCloudflareRoutingRuleIds.has(rule.id)" class="mt-3 grid gap-3 md:grid-cols-2">
                  <label>
                    <span class="ui-label">规则名称</span>
                    <input v-model="rule.name" type="text" class="ui-field" placeholder="日本优选" />
                  </label>
                  <label>
                    <span class="ui-label">目标 DNS 记录名</span>
                    <input v-model="rule.recordName" type="text" class="ui-field font-mono" placeholder="jp.example.com" />
                  </label>
                  <label>
                    <span class="ui-label">记录类型</span>
                    <select v-model="rule.recordType" class="ui-field w-full max-w-[14rem]">
                      <option value="ALL">ALL / A + AAAA</option>
                      <option value="A">A / IPv4</option>
                      <option value="AAAA">AAAA / IPv6</option>
                    </select>
                  </label>
                  <label>
                    <span class="ui-label">白名单/黑名单</span>
                    <select v-model="rule.filterMode" class="ui-field w-full max-w-[14rem]">
                      <option value="allow">白名单：只上传匹配项</option>
                      <option value="deny">黑名单：排除匹配项</option>
                    </select>
                  </label>
                  <label>
                    <span class="ui-label">国家/COLO 筛选词</span>
                    <input v-model="rule.filterTokens" type="text" class="ui-field font-mono" placeholder="JP,HKG,NRT,US,UK" />
                  </label>
                  <label>
                    <span class="ui-label">规则 Top N</span>
                    <input v-model.number="rule.topN" min="0" type="number" class="ui-field" />
                    <p class="mt-2 text-xs text-slate-500">0 表示该规则不限数量。</p>
                  </label>
                </div>
              </div>
            </div>
            <div v-else class="rounded-xl border border-dashed border-sky-200 bg-sky-50/70 px-4 py-3 text-sm text-slate-500">尚未添加分流规则。关闭分流或没有启用规则时，将继续使用单记录名 Top N 推送。</div>
            <div class="flex justify-end">
              <button type="button" class="ui-button ui-button-secondary" @click="addCloudflareRoutingRule(entry)">新增规则</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </details>
</template>
