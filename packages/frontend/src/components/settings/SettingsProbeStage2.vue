<script setup vapor lang="ts">
import { computed, ref } from "vue";
import { PhCaretDown } from "@phosphor-icons/vue";
import type { TraceColoMode } from "../../lib/bridge";
import { deriveTraceURL } from "../../lib/config/settingsForm";
import type { SettingsForm } from "../../lib/config/settingsForm";
import { coloModeLabel } from "./types";

const props = defineProps<{
  settings: SettingsForm;
}>();

const PROBE_REGION_PRESETS = [
  { label: "亚太优选", codes: ["JP", "HKG", "SG", "TW", "KR"] },
  { label: "美西", codes: ["LAX", "SJC", "SEA"] },
] as const;

const probeRegionDraft = ref("");
const probeRegionCodes = computed(() =>
  props.settings.probeTraceCfColo
    .split(/[\s,]+/)
    .filter(Boolean)
    .map((code) => code.toUpperCase()),
);

function switchProbeTraceColoMode(mode: TraceColoMode) {
  props.settings.probeTraceColoMode = mode;
  if (mode === "trace_url" && !props.settings.probeTraceURL.trim()) {
    props.settings.probeTraceURL = deriveTraceURL(props.settings.probeURL);
  }
}

// 1034「边缘 IP 受限」是 zone 级响应：Cloudflare 官方 zone（speed.cloudflare.com 等）不会
// 返回它，用官方端点当追踪地址时阶段 2 永远筛不出 1034，必须换成自己的站点。
const probeTraceHost = computed(() => {
  const raw = props.settings.probeTraceURL.trim() || props.settings.probeURL.trim();
  try {
    return new URL(raw).hostname.toLowerCase();
  } catch {
    return "";
  }
});
const probeTraceHostCannotSee1034 = computed(() => probeTraceHost.value === "cloudflare.com" || probeTraceHost.value.endsWith(".cloudflare.com"));

function setProbeRegionCodes(codes: string[]) {
  props.settings.probeTraceCfColo = [...new Set(codes.map((code) => code.trim().toUpperCase()).filter(Boolean))].join(",");
}

function commitProbeRegionDraft(raw = probeRegionDraft.value) {
  const parts = raw.split(/[\s,]+/).filter(Boolean);
  if (!parts.length) return;
  setProbeRegionCodes([...probeRegionCodes.value, ...parts]);
  probeRegionDraft.value = "";
}

function onProbeRegionInput(event: Event) {
  const value = (event.target as HTMLInputElement).value;
  probeRegionDraft.value = value;
  if (/[\s,]/.test(value)) commitProbeRegionDraft(value);
}

function removeProbeRegionCode(code: string) {
  setProbeRegionCodes(probeRegionCodes.value.filter((item) => item !== code));
}

function applyProbeRegionPreset(codes: readonly string[]) {
  setProbeRegionCodes([...probeRegionCodes.value, ...codes]);
}
</script>

<template>
  <details class="probe-stage border-b border-slate-200 last:border-b-0">
    <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
      <span class="min-w-0">
        <span class="block text-sm font-semibold text-slate-800">第二阶段 · 追踪与地区码</span>
        <span class="mt-0.5 block truncate text-xs text-slate-500">并发 {{ settings.probeConcurrencyStage2 }} · 状态码 200（固定） · {{ probeRegionCodes.length ? (settings.probeTraceCfColoMode === "allow" ? "白名单 " : "黑名单 ") + probeRegionCodes.length + " 项" : "地区码不限" }}</span>
      </span>
      <PhCaretDown class="probe-stage-caret shrink-0 text-slate-400" size="16" />
    </summary>
    <div class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
      <section class="space-y-3">
        <p class="text-xs font-semibold text-slate-600">阶段准入与 COLO 策略</p>
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <div>
            <span class="ui-label">COLO 获取模式</span>
            <div class="inline-flex max-w-full flex-wrap rounded-full border border-slate-200 bg-slate-100 p-1">
              <button type="button" class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs" :class="settings.probeTraceColoMode === 'standard' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="switchProbeTraceColoMode('standard')">标准</button>
              <button type="button" class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs" :class="settings.probeTraceColoMode === 'trace_url' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="switchProbeTraceColoMode('trace_url')">
                追踪URL
              </button>
            </div>
            <p class="mt-2 text-xs text-slate-500">标准按 IP 逐个直连 /cdn-cgi/trace，追踪 URL 决定 Host/SNI 并作为备用端点；追踪URL 只请求下面的地址，适合自建探针。</p>
          </div>

          <div>
            <span class="ui-label">输入源 COLO 筛选时机</span>
            <div class="inline-flex max-w-full flex-wrap rounded-full border border-slate-200 bg-slate-100 p-1">
              <button
                type="button"
                class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs"
                :class="settings.probeSourceColoFilterPhase === 'precheck' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                @click="settings.probeSourceColoFilterPhase = 'precheck'"
              >
                预检时筛
              </button>
              <button
                type="button"
                class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs"
                :class="settings.probeSourceColoFilterPhase === 'stage2' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                @click="settings.probeSourceColoFilterPhase = 'stage2'"
              >
                追踪阶段再筛
              </button>
            </div>
            <p class="mt-2 text-xs text-slate-500">预检按本地 cloudflare-colos 词典提前淘汰非目标地区，省掉无用探测；追踪阶段再筛则先探测、按实际 COLO 淘汰。</p>
          </div>

          <div>
            <span class="ui-label"
              >输入上限
              <details class="probe-hint">
                <summary aria-label="输入上限说明">?</summary>
                <p>按 TCP RTT 升序选取进入追踪的 IP 数；0 不限制。</p>
              </details></span
            >
            <span class="ui-input-unit">
              <input v-model.number="settings.probeStageLimitStage2" min="0" type="number" class="ui-field" />
              <span v-if="settings.probeStageLimitStage2 === 0" class="ui-input-unit-label">不限制</span>
            </span>
          </div>
        </div>
      </section>

      <section class="space-y-3 border-t border-slate-100 pt-4">
        <p class="text-xs font-semibold text-slate-600">追踪请求配置</p>
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <label>
            <span class="ui-label"
              >追踪并发线程
              <details class="probe-hint">
                <summary aria-label="追踪并发线程说明">?</summary>
                <p>后端上限 30；调大能缩短大列表的追踪耗时，但更容易触发边缘限流。</p>
              </details></span
            >
            <input v-model.number="settings.probeConcurrencyStage2" min="1" max="30" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">追踪超时</span>
            <span class="ui-input-unit">
              <input v-model.number="settings.probeTimeoutStage2Ms" min="1" type="number" class="ui-field" />
              <span class="ui-input-unit-label">ms</span>
            </span>
          </label>
          <div>
            <span class="ui-label"
              >追踪有效状态码
              <details class="probe-hint">
                <summary aria-label="有效状态码说明">?</summary>
                <p>固定 200；只有正常响应才算一次有效追踪，拦截页与限流页一律淘汰。</p>
              </details></span
            >
            <p class="ui-field bg-slate-50 text-slate-600">200（固定）</p>
          </div>
          <label class="md:col-span-2 xl:col-span-3">
            <span class="ui-label">追踪 URL（可选）</span>
            <input v-model="settings.probeTraceURL" placeholder="留空时从文件测速URL派生 /cdn-cgi/trace" type="url" class="ui-field font-mono" />
            <p class="mt-2 text-xs text-slate-500">标准模式逐个 IP 直连 /cdn-cgi/trace，用这个地址决定 Host/SNI；选「追踪URL」时只请求该地址，适用于自建探针的 /trace。</p>
          </label>
          <div v-if="probeTraceHostCannotSee1034" class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 md:col-span-2 xl:col-span-3">
            追踪域名是 Cloudflare 官方 zone（<span class="font-mono">{{ probeTraceHost }}</span>），它不会返回 1034「边缘 IP 受限」，因此筛不出 1034 IP。要按自己的站点拦截 1034，请把追踪 URL 填成 <span class="font-mono">https://你的域名/cdn-cgi/trace</span>。
          </div>
        </div>
      </section>

      <section class="space-y-3 border-t border-slate-100 pt-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <p class="text-xs font-semibold text-slate-600">
            最终地区码过滤
            <details class="probe-hint">
              <summary aria-label="地区码过滤说明">?</summary>
              <p>地区码来自追踪响应；白名单只留列表内地区，黑名单排除列表内地区，空列表不限制。结果表展示追踪延迟，CSV 末尾的「备注」列标记被保留的 403 / 1034。</p>
            </details>
          </p>
          <div class="flex flex-wrap items-center gap-2">
            <span class="inline-flex rounded-full border border-slate-200 bg-slate-100 p-1">
              <button type="button" class="rounded-full px-3 py-1.5 text-xs font-semibold transition" :class="settings.probeTraceCfColoMode === 'allow' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probeTraceCfColoMode = 'allow'">白名单</button>
              <button type="button" class="rounded-full px-3 py-1.5 text-xs font-semibold transition" :class="settings.probeTraceCfColoMode === 'deny' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probeTraceCfColoMode = 'deny'">黑名单</button>
            </span>
            <button v-for="preset in PROBE_REGION_PRESETS" :key="preset.label" type="button" class="probe-chip" @click="applyProbeRegionPreset(preset.codes)">+ {{ preset.label }}</button>
          </div>
        </div>
        <div class="probe-tags">
          <span v-for="code in probeRegionCodes" :key="code" class="probe-tag">
            {{ code }}
            <button type="button" :aria-label="`移除 ${code}`" @click="removeProbeRegionCode(code)">×</button>
          </span>
          <input :value="probeRegionDraft" placeholder="输入地区码后回车，或用逗号分隔批量粘贴" class="probe-tag-input" @input="onProbeRegionInput" @keydown.enter.prevent="commitProbeRegionDraft()" @blur="commitProbeRegionDraft()" />
        </div>
        <p class="text-xs text-slate-500">当前为{{ coloModeLabel(settings.probeTraceCfColoMode) }}模式；国家码遵循 ISO 3166-1 alpha-2，UK 兼容为 GB。</p>
      </section>
    </div>
  </details>
</template>
