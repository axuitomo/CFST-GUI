<script setup vapor lang="ts">
import { computed } from "vue";
import { PhCaretDown, PhGauge } from "@phosphor-icons/vue";
import type { AppInfo } from "../../lib/bridge";
import type { SettingsForm } from "../../lib/config/settingsForm";
import type { SettingsSectionController } from "./types";
import SettingsProbeStage2 from "./SettingsProbeStage2.vue";
import SettingsProbeStage3 from "./SettingsProbeStage3.vue";

const props = defineProps<{
  appInfo: AppInfo;
  sections: SettingsSectionController;
  settings: SettingsForm;
}>();

const PROBE_CONCURRENCY_PRESETS = [128, 200, 384] as const;

const isAndroidApp = computed(() => props.appInfo.platform === "android");
const strategyDescription = computed(() => (props.settings.probeStrategy === "full" ? "按 IP池、TCP、追踪、文件测速四阶段执行，所有追踪通过 IP 都会串行进入文件测速。" : "按 IP池、TCP、追踪三阶段执行，跳过文件测速。"));
const probeConcurrencyHint = computed(() => {
  const platform = props.appInfo.platform.trim().toLowerCase();
  if (platform === "android") {
    return "Android 推荐 32-128；Wi-Fi 可从 64 开始，蜂窝网络或省电模式建议 16-64。并发过高会增加 NAT、TIME_WAIT 和连接超时。";
  }
  if (platform.startsWith("windows/")) {
    return "Windows 推荐 128-384；杀毒软件、VPN 或家庭路由器负载较高时建议从 128 开始。";
  }
  if (platform.startsWith("darwin/")) {
    return "macOS 推荐 96-256；电池模式或 Wi-Fi 环境建议从 128 开始。";
  }
  if (platform.startsWith("linux/")) {
    return "Linux 推荐 128-768；桌面或软路由建议从 200 开始，服务器需同时观察 fd、conntrack 和 SoftIRQ。";
  }
  return "跨平台建议：Windows 128-384，Linux 128-768，macOS 96-256，Android 32-128。优先从较低值开始。";
});

// 丢包率在配置里是 0~1 小数，界面按整数百分比展示，转换只发生在这一层。
const probeLossPercent = computed({
  get: () => Math.round(props.settings.maxLossRate * 100),
  set: (percent: number) => {
    const value = Number.isFinite(percent) ? Math.min(100, Math.max(0, Math.round(percent))) : 0;
    props.settings.maxLossRate = value / 100;
  },
});

const probeLatencyRangeInvalid = computed(() => {
  const { minDelayMs, maxTcpLatencyMs } = props.settings;
  return maxTcpLatencyMs != null && maxTcpLatencyMs > 0 && minDelayMs >= maxTcpLatencyMs;
});

const probeStage1TimeoutShort = computed(() => {
  const { probeTimeoutStage1Ms, maxTcpLatencyMs } = props.settings;
  return maxTcpLatencyMs != null && maxTcpLatencyMs > 0 && probeTimeoutStage1Ms < maxTcpLatencyMs;
});

function syncProbeStage1Timeout() {
  const ceiling = props.settings.maxTcpLatencyMs ?? 0;
  if (ceiling > 0) props.settings.probeTimeoutStage1Ms = ceiling + 100;
}

const probeMcisAutoBudget = computed(() => props.settings.probeSourcePoolLimit * 3);
const probeMcisAutoConcurrency = computed(() => props.settings.probeConcurrencyStage1);
const formatProbeNumber = (value: number) => value.toLocaleString("zh-CN");

function setProbeMcisBudgetAuto() {
  props.settings.probeMCISBudget = 0;
}
function setProbeMcisBudgetManual() {
  if (props.settings.probeMCISBudget <= 0) props.settings.probeMCISBudget = probeMcisAutoBudget.value;
}
function setProbeMcisConcurrencyAuto() {
  props.settings.probeMCISConcurrency = 0;
}
function setProbeMcisConcurrencyManual() {
  if (props.settings.probeMCISConcurrency <= 0) props.settings.probeMCISConcurrency = probeMcisAutoConcurrency.value;
}

const probeMcisSummary = computed(() => {
  const budget = props.settings.probeMCISBudget > 0 ? `手动(${props.settings.probeMCISBudget})` : "自动";
  const concurrency = props.settings.probeMCISConcurrency > 0 ? `手动(${props.settings.probeMCISConcurrency})` : "自动";
  return `预算 ${budget} · 并发 ${concurrency}`;
});
</script>

<template>
  <details :open="sections.isOpen('probe')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('probe', $event)">
    <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
      <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
        <PhGauge class="mr-2 shrink-0 text-primary" size="20" weight="fill" />
        探测策略
      </h3>
      <div class="flex shrink-0 items-center gap-3">
        <span class="ui-pill ui-pill-subtle">{{ settings.probeStrategy === "full" ? "完整模式" : "极速模式" }}</span>
        <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('probe') ? 'rotate-180' : ''" size="18" />
      </div>
    </summary>
    <div v-if="sections.isMounted('probe')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
      <section class="space-y-3">
        <span class="ui-label">策略预设</span>
        <div class="inline-flex max-w-full flex-wrap rounded-full border border-slate-200 bg-slate-100 p-1">
          <button type="button" class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs" :class="settings.probeStrategy === 'fast' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probeStrategy = 'fast'">极速模式</button>
          <button type="button" class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs" :class="settings.probeStrategy === 'full' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probeStrategy = 'full'">完整模式</button>
        </div>
        <p class="text-sm text-slate-500">{{ strategyDescription }}</p>
      </section>

      <section class="space-y-3">
        <label>
          <span class="ui-label">文件测速 URL</span>
          <input v-model="settings.probeURL" type="url" class="ui-field font-mono" />
          <p class="mt-2 text-xs text-slate-500">完整模式的文件测速地址；追踪 URL 留空时也从这里派生 /cdn-cgi/trace，因此不要填写 /cdn-cgi/trace。</p>
          <p v-if="isAndroidApp" class="mt-1 text-xs text-amber-600">Android 上 http:// 不会被拦截，但明文易被劫持，建议使用 https://。</p>
        </label>
      </section>

      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white">
        <details class="probe-stage border-b border-slate-200 last:border-b-0">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <span class="min-w-0">
              <span class="block text-sm font-semibold text-slate-800">MICS 抽样 · 仅 MICS 来源</span>
              <span class="mt-0.5 block truncate text-xs text-slate-500">{{ probeMcisSummary }}</span>
            </span>
            <PhCaretDown class="probe-stage-caret shrink-0 text-slate-400" size="16" />
          </summary>
          <div class="space-y-3 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
            <p class="text-xs text-slate-500">输入源的 IP 模式仍在「输入源」页按来源选择。复用抽样得到的延迟与 COLO、跳过重复 TCP 测速；所有 MICS 来源共享下面的资源上限。</p>
            <div class="grid gap-3 md:grid-cols-2">
              <article class="rounded-xl border border-slate-200 bg-slate-50/70 p-4 lg:p-3">
                <div class="mb-3 flex items-center justify-between gap-3">
                  <span class="ui-label mb-0">抽样预算</span>
                  <div class="inline-flex rounded-full border border-slate-200 bg-slate-100 p-1">
                    <button type="button" class="rounded-full px-3 py-1.5 text-xs font-semibold transition" :class="settings.probeMCISBudget === 0 ? 'bg-white text-emerald-700 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="setProbeMcisBudgetAuto">自动</button>
                    <button type="button" class="rounded-full px-3 py-1.5 text-xs font-semibold transition" :class="settings.probeMCISBudget > 0 ? 'bg-white text-amber-700 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="setProbeMcisBudgetManual">手动</button>
                  </div>
                </div>
                <p v-show="settings.probeMCISBudget === 0" class="ui-field bg-slate-50 text-slate-600">自动计算：{{ formatProbeNumber(probeMcisAutoBudget) }}</p>
                <input v-show="settings.probeMCISBudget > 0" v-model.number="settings.probeMCISBudget" min="1" type="number" class="ui-field" />
                <p class="mt-2 text-xs text-slate-500">自动：第一阶段候选池上限 ×3（当前 {{ formatProbeNumber(settings.probeSourcePoolLimit) }}）；手动值不设固定上限，候选空间不足时自动收敛。</p>
              </article>
              <article class="rounded-xl border border-slate-200 bg-slate-50/70 p-4 lg:p-3">
                <div class="mb-3 flex items-center justify-between gap-3">
                  <span class="ui-label mb-0">抽样并发线程</span>
                  <div class="inline-flex rounded-full border border-slate-200 bg-slate-100 p-1">
                    <button type="button" class="rounded-full px-3 py-1.5 text-xs font-semibold transition" :class="settings.probeMCISConcurrency === 0 ? 'bg-white text-emerald-700 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="setProbeMcisConcurrencyAuto">自动</button>
                    <button type="button" class="rounded-full px-3 py-1.5 text-xs font-semibold transition" :class="settings.probeMCISConcurrency > 0 ? 'bg-white text-amber-700 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="setProbeMcisConcurrencyManual">手动</button>
                  </div>
                </div>
                <p v-show="settings.probeMCISConcurrency === 0" class="ui-field bg-slate-50 text-slate-600">自动同步：{{ probeMcisAutoConcurrency }} 线程</p>
                <input v-show="settings.probeMCISConcurrency > 0" v-model.number="settings.probeMCISConcurrency" min="1" type="number" class="ui-field" />
                <p class="mt-2 text-xs text-slate-500">自动：与第一阶段的 TCP 并发线程数同步；手动设置不受影响。</p>
              </article>
            </div>
          </div>
        </details>

        <details class="probe-stage border-b border-slate-200 last:border-b-0">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <span class="min-w-0">
              <span class="block text-sm font-semibold text-slate-800">第一阶段 · TCP 筛选</span>
              <span class="mt-0.5 block truncate text-xs text-slate-500">端口 {{ settings.probeTcpPort }} · 并发 {{ settings.probeConcurrencyStage1 }} · 延迟 {{ settings.minDelayMs }}–{{ settings.maxTcpLatencyMs || "∞" }} ms · 丢包 ≤ {{ probeLossPercent }}%</span>
            </span>
            <PhCaretDown class="probe-stage-caret shrink-0 text-slate-400" size="16" />
          </summary>
          <div class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 xl:grid-cols-3 lg:p-4">
            <label>
              <span class="ui-label">{{ settings.probePortPolicy === "source_override_global" ? "回退测速端口" : "测速端口" }}</span>
              <input v-model.number="settings.probeTcpPort" min="1" max="65535" type="number" class="ui-field" />
            </label>

            <div>
              <span class="ui-label">端口策略</span>
              <div class="inline-flex max-w-full flex-wrap rounded-full border border-slate-200 bg-slate-100 p-1">
                <button type="button" class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs" :class="settings.probePortPolicy === 'fixed_global' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probePortPolicy = 'fixed_global'">
                  固定测速端口
                </button>
                <button
                  type="button"
                  class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs"
                  :class="settings.probePortPolicy === 'source_override_global' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                  @click="settings.probePortPolicy = 'source_override_global'"
                >
                  输入源端口优先
                </button>
              </div>
              <p class="mt-2 text-xs text-slate-500">输入源声明端口时优先使用，否则回退到固定端口。</p>
            </div>

            <div>
              <span class="ui-label"
                >TCP 发包次数
                <details class="probe-hint">
                  <summary aria-label="发包次数说明">?</summary>
                  <p>默认 4 次并跳过首包，只用后续成功样本计算平均延迟和丢包率；次数越多越稳，单 IP 耗时越长。</p>
                </details></span
              >
              <input v-model.number="settings.probePingTimes" min="2" type="number" class="ui-field" />
            </div>

            <div class="md:col-span-2">
              <span class="ui-label"
                >TCP 延迟区间
                <details class="probe-hint">
                  <summary aria-label="延迟区间说明">?</summary>
                  <p>下限剔除过近的可疑节点，上限淘汰高延迟；上限留空表示不限制。平均值只统计跳过后首包后的成功样本。</p>
                </details></span
              >
              <div class="flex flex-wrap items-center gap-2">
                <span class="ui-input-unit min-w-0 flex-1">
                  <input v-model.number="settings.minDelayMs" min="0" type="number" class="ui-field" :class="probeLatencyRangeInvalid ? 'border-rose-400' : ''" />
                  <span class="ui-input-unit-label">ms</span>
                </span>
                <span class="shrink-0 text-xs text-slate-400">—</span>
                <span class="ui-input-unit min-w-0 flex-1">
                  <input v-model.number="settings.maxTcpLatencyMs" min="1" placeholder="不限" type="number" class="ui-field" :class="probeLatencyRangeInvalid ? 'border-rose-400' : ''" />
                  <span class="ui-input-unit-label">ms</span>
                </span>
              </div>
              <p v-if="probeLatencyRangeInvalid" class="mt-1 text-xs text-rose-600">下限必须小于上限，否则没有 IP 能通过。</p>
            </div>

            <label>
              <span class="ui-label">TCP 丢包率上限</span>
              <span class="ui-input-unit">
                <input v-model.number="probeLossPercent" max="100" min="0" step="1" type="number" class="ui-field" />
                <span class="ui-input-unit-label">%</span>
              </span>
            </label>

            <label>
              <span class="ui-label">第一阶段 TCP 超时</span>
              <span class="ui-input-unit">
                <input v-model.number="settings.probeTimeoutStage1Ms" min="1" type="number" class="ui-field" />
                <span class="ui-input-unit-label">ms</span>
              </span>
            </label>

            <div>
              <span class="ui-label"
                >候选池上限
                <details class="probe-hint">
                  <summary aria-label="候选池上限说明">?</summary>
                  <p>单个输入源最多展开的候选 IP 数，超出会截断并在任务提示里说明。</p>
                </details></span
              >
              <input v-model.number="settings.probeSourcePoolLimit" min="1" max="1000000" type="number" class="ui-field" />
            </div>

            <div>
              <span class="ui-label"
                >TCP 并发线程
                <details class="probe-hint">
                  <summary aria-label="TCP 并发线程说明">?</summary>
                  <p>{{ probeConcurrencyHint }}</p>
                </details></span
              >
              <div class="flex flex-wrap items-center gap-2">
                <input v-model.number="settings.probeConcurrencyStage1" min="1" max="1000" type="number" class="ui-field w-24" />
                <button v-for="preset in PROBE_CONCURRENCY_PRESETS" :key="preset" type="button" class="probe-chip" :class="settings.probeConcurrencyStage1 === preset ? 'probe-chip-active' : ''" @click="settings.probeConcurrencyStage1 = preset">
                  {{ preset }}
                </button>
              </div>
            </div>

            <div v-if="probeStage1TimeoutShort" class="flex flex-wrap items-center gap-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 md:col-span-2 xl:col-span-3">
              <span class="min-w-0">超时（{{ settings.probeTimeoutStage1Ms }} ms）小于延迟上限（{{ settings.maxTcpLatencyMs }} ms）：超过上限的 IP 要等满超时才被淘汰。</span>
              <button type="button" class="ui-button ui-button-ghost px-3 py-1 text-xs" @click="syncProbeStage1Timeout">同步为上限 + 100 ms</button>
            </div>
          </div>
        </details>

        <SettingsProbeStage2 :settings="settings" />
        <SettingsProbeStage3 :app-info="appInfo" :settings="settings" />
      </div>
    </div>
  </details>
</template>
