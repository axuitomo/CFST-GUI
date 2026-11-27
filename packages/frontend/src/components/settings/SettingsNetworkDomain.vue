<script setup vapor lang="ts">
import { computed, ref, watch } from "vue";
import { PhArrowsClockwise, PhCaretDown, PhDatabase, PhFileArrowUp, PhGlobeHemisphereWest, PhQuestion, PhShieldCheck } from "@phosphor-icons/vue";
import type { AppInfo, DNSSubdomainSnapshot } from "../../lib/bridge";
import type { SettingsForm } from "../../lib/config/settingsForm";
import { DOH_PRESET_SHORT_LABELS, DOH_PRESETS, formatSettingsTimestamp, isMaskedSecretValue, type SettingsSectionController, type SettingsTimestampFormatOptions } from "./types";
import SettingsDnsProvidersSection from "./SettingsDnsProvidersSection.vue";
import SettingsProbeSection from "./SettingsProbeSection.vue";

const props = defineProps<{
  appInfo: AppInfo;
  dnsSubdomainErrors: Record<string, string>;
  dnsSubdomains: Record<string, DNSSubdomainSnapshot[]>;
  dnsSubdomainsLoading: Record<string, boolean>;
  dohTestResult: { message: string; ok: boolean } | null;
  dohTesting: boolean;
  formatTimestamp: (value: string, options?: SettingsTimestampFormatOptions) => string;
  githubTesting: boolean;
  loading: boolean;
  sections: SettingsSectionController;
  settings: SettingsForm;
}>();

defineEmits<{
  (event: "load-dns-subdomains", providerId: string): void;
  (event: "test-doh"): void;
  (event: "test-github-export"): void;
}>();

function dohPresetForURL(url: string): string {
  return DOH_PRESETS.find((preset) => preset.url === url.trim())?.id ?? "custom";
}

const dohPresetId = ref(dohPresetForURL(props.settings.probeDoHURL));
watch(
  () => props.settings.probeDoHURL,
  (url) => {
    dohPresetId.value = dohPresetForURL(url);
  },
);

function selectDohPreset(event: Event) {
  const id = (event.currentTarget as HTMLSelectElement).value;
  dohPresetId.value = id;
  const preset = DOH_PRESETS.find((entry) => entry.id === id);
  if (preset) props.settings.probeDoHURL = preset.url;
}

const dohPresetLabel = computed(() => DOH_PRESET_SHORT_LABELS[dohPresetId.value] ?? "自定义 DoH");
const githubConfigComplete = computed(() => {
  const token = props.settings.githubToken.trim();
  return Boolean(props.settings.githubOwner.trim() && props.settings.githubRepo.trim() && props.settings.githubBranch.trim() && props.settings.githubPathTemplate.trim() && token && !isMaskedSecretValue(token));
});
const githubConfigLabel = computed(() => (githubConfigComplete.value ? "GitHub 配置完整" : "GitHub 配置未完整"));
</script>

<template>
  <section class="settings-domain">
    <div class="settings-domain-header">
      <div class="settings-domain-title-wrap" :class="{ pinned: sections.pinnedHelpSection === 'network' }">
        <h3 class="settings-domain-title flex items-center">
          网络与任务
          <button type="button" class="settings-help-btn" aria-label="查看说明" @click="sections.toggleHelp('network')">
            <PhQuestion size="13" />
          </button>
        </h3>
        <p class="settings-domain-copy">设置 DNS 解析与 ECH、输入源行为、测速参数、DNS 推送平台与 GitHub 结果导出。</p>
      </div>
    </div>
    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <details :open="sections.isOpen('dns')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('dns', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhGlobeHemisphereWest class="mr-2 shrink-0 text-sky-600" size="20" weight="fill" />
            DNS 与 ECH
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ dohPresetLabel }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('dns') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('dns')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
          <div class="grid gap-4 md:grid-cols-2">
            <div>
              <span class="ui-label">DoH 地址</span>
              <select :value="dohPresetId" class="ui-field mt-2" @change="selectDohPreset($event)">
                <option v-for="preset in DOH_PRESETS" :key="preset.id" :value="preset.id">{{ preset.label }}</option>
                <option value="custom">自定义…</option>
              </select>
              <input v-if="dohPresetId === 'custom'" v-model="settings.probeDoHURL" placeholder="https://example.com/dns-query" type="url" class="ui-field mt-2 font-mono" />
              <p class="mt-2 text-xs text-slate-500">解析输入源域名和来源 URL 主机名用；留空或不可用时自动回退系统 DNS。修改会随配置自动保存。</p>
            </div>
            <div class="space-y-3">
              <label class="flex items-start gap-2 text-sm text-slate-700">
                <input v-model="settings.probeECH" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                <span class="min-w-0">对来源 URL 请求启用 ECH（加密 Client Hello）</span>
              </label>
              <label class="block">
                <span class="ui-label">ECH 域名</span>
                <input v-model="settings.probeECHDomain" placeholder="cloudflare-ech.com" type="text" class="ui-field font-mono" />
                <span class="mt-2 block text-xs text-slate-500">自动获取 ECH 配置时查询该域名的 HTTPS RR；来源主机自己发布 ech 记录时优先用主机记录。默认 cloudflare-ech.com，也是 ECH 外层 SNI。</span>
              </label>
              <label class="block">
                <span class="ui-label">ECH 配置（可选）</span>
                <input v-model="settings.probeECHConfig" placeholder="留空则按 ECH 域名自动获取" type="text" class="ui-field font-mono" />
                <span class="mt-2 block text-xs text-slate-500">base64 ECHConfigList；手填后覆盖 DoH 自动获取，两者都没有时不启用 ECH。仅作用于来源 URL 请求。</span>
              </label>
            </div>
          </div>
          <div class="flex flex-wrap items-start gap-3 border-t border-slate-100 pt-3">
            <button type="button" class="ui-button ui-button-cf" :disabled="dohTesting" @click="$emit('test-doh')">
              <PhArrowsClockwise size="16" />
              {{ dohTesting ? "测试中" : "测试 DoH" }}
            </button>
            <p v-if="dohTestResult" class="min-w-0 flex-1 text-xs" :class="dohTestResult.ok ? 'text-emerald-600' : 'text-rose-600'">{{ dohTestResult.message }}</p>
            <p v-else class="min-w-0 flex-1 text-xs text-slate-500">测试会解析文件测速 URL 的主机，并按手填配置、来源主机、ECH 域名的顺序取 ECH 配置；端点故障不回退系统 DNS，因此能测出坏端点。</p>
          </div>
        </div>
      </details>

      <details :open="sections.isOpen('tls')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('tls', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhShieldCheck class="mr-2 shrink-0 text-emerald-600" size="20" weight="fill" />
            TLS 证书校验
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ settings.probeVerifyTLSCertificate ? "严格校验" : "默认跳过" }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('tls') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('tls')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
          <label class="md:col-span-2 flex items-start gap-3">
            <input v-model="settings.probeVerifyTLSCertificate" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
            <span class="min-w-0">
              <span class="block text-sm font-medium text-slate-700">严格校验 TLS 证书与各阶段实际使用的 SNI 域名匹配</span>
              <span class="text-xs text-slate-500">默认关闭，作用于追踪、HTTPing 和文件测速；远端来源抓取、更新包和 Cloudflare DNS 始终校验证书。</span>
            </span>
          </label>
        </div>
      </details>

      <details :open="sections.isOpen('sources')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('sources', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhDatabase class="mr-2 shrink-0 text-slate-600" size="20" weight="fill" />
            输入源行为
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ settings.sourceAutoDetectName ? "自动识别" : "手动命名" }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('sources') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('sources')" class="border-t border-slate-100 p-3 sm:p-4 lg:p-4">
          <button type="button" class="flex w-full items-center justify-between gap-4 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left" @click="settings.sourceAutoDetectName = !settings.sourceAutoDetectName">
            <span class="min-w-0">
              <span class="block text-sm font-medium text-slate-700">自动识别输入源名称</span>
              <span class="text-xs text-slate-400">URL 来源会优先匹配内置来源表；手动填写过的名称不会被覆盖。</span>
            </span>
            <span class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="settings.sourceAutoDetectName ? 'bg-primary' : 'bg-slate-300'">
              <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.sourceAutoDetectName ? 'translate-x-5' : 'translate-x-0'"></span>
            </span>
          </button>
        </div>
      </details>

      <SettingsProbeSection :app-info="appInfo" :sections="sections" :settings="settings" />

      <SettingsDnsProvidersSection :dns-subdomain-errors="dnsSubdomainErrors" :dns-subdomains="dnsSubdomains" :dns-subdomains-loading="dnsSubdomainsLoading" :sections="sections" :settings="settings" @load-dns-subdomains="$emit('load-dns-subdomains', $event)" />

      <details :open="sections.isOpen('github')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('github', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhFileArrowUp class="mr-2 shrink-0 text-slate-500" size="20" />
            GitHub 配置
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ githubConfigLabel }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('github') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('github')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
          <div class="md:col-span-2 text-sm text-slate-500">只写入测速结果文件（支持 CSV / TXT），不提交配置包，避免泄露 Cloudflare Token 或 WebDAV 凭据。必填项完整时自动启用 GitHub 结果导出。</div>
          <label>
            <span class="ui-label">Owner</span>
            <input v-model="settings.githubOwner" placeholder="axuitomo" type="text" class="ui-field font-mono" />
          </label>
          <label>
            <span class="ui-label">Repo</span>
            <input v-model="settings.githubRepo" placeholder="CFST-GUI" type="text" class="ui-field font-mono" />
          </label>
          <label>
            <span class="ui-label">Branch</span>
            <input v-model="settings.githubBranch" placeholder="main" type="text" class="ui-field font-mono" />
          </label>
          <label>
            <span class="ui-label">PAT Token</span>
            <input v-model="settings.githubToken" type="password" class="ui-field font-mono" autocomplete="off" />
          </label>
          <label>
            <span class="ui-label">GitHub Top N</span>
            <input v-model.number="settings.uploadGitHubTopN" min="0" type="number" class="ui-field" />
            <p class="mt-2 text-xs text-slate-500">0 表示不限数量；共享上传筛选仍会先执行。</p>
          </label>
          <label>
            <span class="ui-label">上传格式</span>
            <select v-model="settings.githubFormat" class="ui-field">
              <option value="csv">CSV</option>
              <option value="txt">TXT</option>
            </select>
            <p class="mt-2 text-xs text-slate-500">不会自动改写路径扩展名，若选择 TXT 请自行把路径模板改成 `.txt`。</p>
          </label>
          <label class="md:col-span-2">
            <span class="ui-label">路径模板</span>
            <input v-model="settings.githubPathTemplate" placeholder="cfst-results/{date}/{time}-{task_id}.csv" type="text" class="ui-field font-mono" />
            <p class="mt-2 text-xs text-slate-500">支持 {date}、{time}、{task_id}、{timestamp}；重复路径会先读取 sha 再覆盖。</p>
          </label>
          <label class="md:col-span-2">
            <span class="ui-label">提交信息模板</span>
            <input v-model="settings.githubCommitMessageTemplate" placeholder="CFST results {date} {time}" type="text" class="ui-field font-mono" />
          </label>
          <label class="md:col-span-2">
            <span class="ui-label">CSV 表头模板</span>
            <input v-model="settings.githubCSVHeaderTemplate" placeholder="IP,COLO,TCP,DOWNLOAD" type="text" class="ui-field font-mono" />
            <p class="mt-2 text-xs text-slate-500">留空时沿用默认 CSV 表头；仅在 GitHub 上传生效。</p>
          </label>
          <label class="md:col-span-2">
            <span class="ui-label">CSV 行模板</span>
            <textarea v-model="settings.githubCSVRowTemplate" rows="3" class="ui-field font-mono" placeholder="{ip},{colo},{tcp_latency_ms},{download_mbps},{source_port},{test_port}"></textarea>
            <p class="mt-2 text-xs text-slate-500">占位符支持 {index}、{ip}、{colo}、{sended}、{received}、{loss_rate}、{tcp_latency_ms}、{trace_latency_ms}、{download_mbps}、{max_download_mbps}、{source_port}、{test_port}。</p>
          </label>
          <label class="md:col-span-2">
            <span class="ui-label">TXT 行模板</span>
            <textarea v-model="settings.githubTXTRowTemplate" rows="3" class="ui-field font-mono" placeholder="{ip}"></textarea>
            <p class="mt-2 text-xs text-slate-500">TXT 每行渲染一条结果；空值会输出为空字符串。</p>
          </label>

          <div class="md:col-span-2 grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
            <p class="break-all text-xs text-slate-500">最近导出：{{ formatSettingsTimestamp(formatTimestamp, settings.githubLastExportAt, "尚未导出") }}。推荐使用 fine-grained PAT，仅授予目标仓库 Contents Read and write。</p>
            <button type="button" class="ui-button ui-button-secondary" :disabled="loading || githubTesting" @click="$emit('test-github-export')">
              <PhArrowsClockwise size="18" />
              {{ githubTesting ? "测试中" : "测试 GitHub" }}
            </button>
          </div>
        </div>
      </details>
    </div>
  </section>
</template>
