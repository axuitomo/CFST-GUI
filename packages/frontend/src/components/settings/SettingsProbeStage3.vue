<script setup vapor lang="ts">
import { computed, ref } from "vue";
import { PhCaretDown } from "@phosphor-icons/vue";
import type { AppInfo } from "../../lib/bridge";
import type { SettingsForm } from "../../lib/config/settingsForm";

const props = defineProps<{
  appInfo: AppInfo;
  settings: SettingsForm;
}>();

const PROBE_SPEED_PRESETS = [5, 15, 50] as const;
const probeStage3TuningOpen = ref(false);
const probeMinSpeedMbps = computed(() => Math.round(props.settings.minDownloadMbps * 8));

const isAndroidApp = computed(() => props.appInfo.platform === "android");
const isLinuxArmHost = computed(() => {
  const platform = props.appInfo.platform.trim().toLowerCase();
  return platform.startsWith("linux/") && (platform.includes("arm64") || platform.endsWith("/arm") || platform.includes("/arm/"));
});

const downloadProtocolHint = computed(() => {
  if (isAndroidApp.value) {
    return "Android 上 Auto 会回退到 TCP（HTTP/1.1 或 HTTP/2），避免蜂窝网或禁 UDP 网络上的 H3 超时。仍可手动选择 H3。";
  }
  if (isLinuxArmHost.value) {
    return "当前 Linux ARM 主机上 Auto 会回退到 TCP，降低 H3/UDP 异常；Windows、macOS 和 Linux amd64 仍会先尝试 H3。";
  }
  return "Auto 默认先尝试 HTTP/3，失败后回退 TCP 上的 HTTP/1.1/2。Linux ARM 与 Android 上 Auto 会直接使用 TCP。";
});
</script>

<template>
  <details class="probe-stage border-b border-slate-200 last:border-b-0">
    <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
      <span class="min-w-0">
        <span class="block text-sm font-semibold text-slate-800">第三阶段 · 文件测速与结果</span>
        <span v-if="settings.probeStrategy === 'fast'" class="mt-0.5 block text-xs text-slate-500">极速模式已跳过</span>
        <span v-else class="mt-0.5 block truncate text-xs text-slate-500">并发 {{ settings.probeConcurrencyStage3 }} · {{ settings.probeDownloadTimeSeconds }}s/IP · {{ settings.probeDownloadSpeedMetric === "max" ? "最高速率" : "平均速率" }} · 结果 {{ settings.probePrintNum || "不限" }}</span>
      </span>
      <PhCaretDown class="probe-stage-caret shrink-0 text-slate-400" size="16" />
    </summary>
    <div v-if="settings.probeStrategy === 'full'" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
      <section class="space-y-3">
        <p class="text-xs font-semibold text-slate-600">核心目标</p>
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <div>
            <span class="ui-label">最低下载速度</span>
            <div class="flex flex-wrap items-center gap-2">
              <span class="ui-input-unit w-28">
                <input v-model.number="settings.minDownloadMbps" min="0" step="0.1" type="number" class="ui-field" />
                <span class="ui-input-unit-label">MB/s</span>
              </span>
              <span class="text-xs text-slate-500">≈ {{ probeMinSpeedMbps }} Mbps</span>
              <button v-for="preset in PROBE_SPEED_PRESETS" :key="preset" type="button" class="probe-chip" :class="settings.minDownloadMbps === preset ? 'probe-chip-active' : ''" @click="settings.minDownloadMbps = preset">
                {{ preset }}
              </button>
            </div>
            <p class="mt-1 text-xs text-slate-400">低于该值的 IP 会提前结束测速，不再跑满整个测量窗口。</p>
          </div>
          <div>
            <span class="ui-label">达到成功 IP 数后停止</span>
            <span class="ui-input-unit">
              <input v-model.number="settings.probeDownloadSuccessLimit" min="0" type="number" class="ui-field" />
              <span v-if="settings.probeDownloadSuccessLimit === 0" class="ui-input-unit-label">不限制</span>
            </span>
          </div>
          <div>
            <span class="ui-label"
              >下载 HTTP 协议
              <details class="probe-hint">
                <summary aria-label="下载协议说明">?</summary>
                <p>{{ downloadProtocolHint }}</p>
              </details></span
            >
            <select v-model="settings.probeDownloadHTTPProtocol" class="ui-field">
              <option value="auto">Auto</option>
              <option value="h1">H1.1</option>
              <option value="h2">H2</option>
              <option value="h3">H3</option>
            </select>
          </div>
          <div>
            <span class="ui-label">下载速率依据</span>
            <div class="inline-flex max-w-full flex-wrap rounded-full border border-slate-200 bg-slate-100 p-1">
              <button type="button" class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs" :class="settings.probeDownloadSpeedMetric === 'average' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probeDownloadSpeedMetric = 'average'">
                平均速率
              </button>
              <button type="button" class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs" :class="settings.probeDownloadSpeedMetric === 'max' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probeDownloadSpeedMetric = 'max'">
                最高速率
              </button>
            </div>
          </div>
          <div>
            <span class="ui-label">结果展示数量</span>
            <span class="ui-input-unit">
              <input v-model.number="settings.probePrintNum" min="0" type="number" class="ui-field" />
              <span v-if="settings.probePrintNum === 0" class="ui-input-unit-label">不限制</span>
            </span>
          </div>
          <div>
            <span class="ui-label"
              >输入上限
              <details class="probe-hint">
                <summary aria-label="输入上限说明">?</summary>
                <p>按 TCP RTT 升序选取进入文件测速的 IP 数。</p>
              </details></span
            >
            <input v-model.number="settings.probeStageLimitStage3" min="1" type="number" class="ui-field" />
          </div>
        </div>
      </section>

      <section class="space-y-3 border-t border-slate-100 pt-4">
        <button type="button" class="flex w-full items-center justify-between gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-3 py-2 text-left" @click="probeStage3TuningOpen = !probeStage3TuningOpen">
          <span class="text-xs font-semibold text-slate-600">底层调优</span>
          <span class="text-xs text-slate-500">{{ probeStage3TuningOpen ? "收起" : "展开 6 项" }}</span>
        </button>
        <div v-show="probeStage3TuningOpen" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <div class="md:col-span-2">
            <span class="ui-label"
              >单 IP 测量窗口
              <details class="probe-hint">
                <summary aria-label="测量窗口说明">?</summary>
                <p>每个 IP 先跳过预热时间再开始计量；正式测速时间内遇到短文件结束或断流会自动续连，预热之后的窗口继续累加。</p>
              </details></span
            >
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-xs text-slate-500">预热</span>
              <span class="ui-input-unit min-w-0 flex-1">
                <input v-model.number="settings.probeDownloadWarmupSeconds" min="0" type="number" class="ui-field" />
                <span class="ui-input-unit-label">秒</span>
              </span>
              <span class="shrink-0 text-xs text-slate-400">→</span>
              <span class="text-xs text-slate-500">正式测速</span>
              <span class="ui-input-unit min-w-0 flex-1">
                <input v-model.number="settings.probeDownloadTimeSeconds" min="1" type="number" class="ui-field" />
                <span class="ui-input-unit-label">秒</span>
              </span>
            </div>
          </div>
          <div>
            <span class="ui-label"
              >并发测速 IP 数
              <details class="probe-hint">
                <summary aria-label="并发测速说明">?</summary>
                <p>默认 1 让每个 IP 独占带宽；调大能缩短大列表总耗时，但并发下载会互相抢带宽，测得的速度偏低。</p>
              </details></span
            >
            <input v-model.number="settings.probeConcurrencyStage3" min="1" max="16" type="number" class="ui-field" />
          </div>
          <label>
            <span class="ui-label">单 IP GET 分片并发</span>
            <input v-model.number="settings.probeDownloadGetConcurrency" min="1" max="32" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">下载缓冲</span>
            <span class="ui-input-unit">
              <input v-model.number="settings.probeDownloadBufferKB" min="64" max="4096" step="64" type="number" class="ui-field" />
              <span class="ui-input-unit-label">KiB</span>
            </span>
          </label>
          <label>
            <span class="ui-label">下载测速刷新间隔</span>
            <span class="ui-input-unit">
              <input v-model.number="settings.probeDownloadSpeedSampleIntervalMs" min="1" step="100" type="number" class="ui-field" />
              <span class="ui-input-unit-label">ms</span>
            </span>
          </label>
        </div>
      </section>
    </div>
    <p v-else class="border-t border-slate-100 px-4 py-3 text-xs text-slate-500 sm:px-5 lg:px-4">极速模式不执行文件测速；切到完整模式后可配置核心目标与底层调优。</p>
  </details>
</template>
