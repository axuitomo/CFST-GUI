<script setup vapor lang="ts">
import { computed } from "vue";
import { PhArrowSquareOut, PhArrowsClockwise, PhCaretDown, PhDownload, PhGauge, PhGlobeHemisphereWest, PhMoon, PhQuestion } from "@phosphor-icons/vue";
import type { AppInfo, ColoDictionaryStatus } from "../../lib/bridge";
import type { SettingsForm } from "../../lib/config/settingsForm";
import type { ViewportPreset, ViewportSize } from "../../lib/app/types";
import type { SettingsSectionController, SettingsTimestampFormatOptions, SettingsUpdateState } from "./types";

const props = defineProps<{
  appInfo: AppInfo;
  coloDictionaryProcessing: boolean;
  coloDictionaryStatus: ColoDictionaryStatus | null;
  coloDictionaryUpdating: boolean;
  formatTimestamp: (value: string, options?: SettingsTimestampFormatOptions) => string;
  loading: boolean;
  platform: "desktop" | "mobile";
  sections: SettingsSectionController;
  settings: SettingsForm;
  updateState: SettingsUpdateState;
  utcOffsetLabel: string;
  viewportAdaptiveActive: boolean;
  viewportPresets: ViewportPreset[];
  viewportRuntimeSupported: boolean;
  viewportSize: ViewportSize;
  viewportSwitching: boolean;
}>();

defineEmits<{
  (event: "apply-viewport-preset", presetId: string): void;
  (event: "check-update"): void;
  (event: "install-update"): void;
  (event: "open-release-page"): void;
  (event: "process-colo-dictionary"): void;
  (event: "refresh-colo-dictionary"): void;
}>();

const updateRequiresManualInstall = computed(() => props.updateState.installMode === "docker_compose" || props.updateState.nextAction === "manual");
const updateRequiresWebUIDeployGuide = computed(() => props.updateState.installMode === "docker_compose");

const updateStatusLabel = computed(() => {
  const labels: Record<SettingsUpdateState["status"], string> = {
    available: "发现新版",
    checking: "检查中",
    failed: "检查失败",
    idle: "未检查",
    installing: "下载中",
    latest: "已是最新",
    ready: updateRequiresManualInstall.value ? "已下载待部署" : "已触发安装",
  };
  return labels[props.updateState.status] || "未检查";
});

const viewportSummaryLabel = computed(() => {
  if (!props.viewportRuntimeSupported) {
    const label = props.platform === "mobile" ? "移动端自适应" : "浏览器自适应";
    return props.viewportSize.cssWidth && props.viewportSize.cssHeight ? `${label} ${props.viewportSize.cssWidth}x${props.viewportSize.cssHeight}` : label;
  }
  if (props.viewportAdaptiveActive) {
    return props.viewportSize.cssWidth && props.viewportSize.cssHeight ? `自适应 ${props.viewportSize.cssWidth}x${props.viewportSize.cssHeight}` : "自适应";
  }
  return props.viewportSize.cssWidth && props.viewportSize.cssHeight ? `${props.viewportSize.cssWidth}x${props.viewportSize.cssHeight}` : "未读取";
});

const themeSummaryLabel = computed(() => {
  if (props.settings.themeMode === "light") return "浅色";
  if (props.settings.themeMode === "dark") return "深色";
  if (props.settings.themeMode === "auto_time") return `时间自动 · ${props.utcOffsetLabel}`;
  return "系统自动";
});

const coloDictionaryUpdatedAt = computed(() => {
  const value = props.coloDictionaryStatus?.last_updated_at || "";
  return value.trim() ? props.formatTimestamp(value) : "尚未更新";
});

function isViewportPresetActive(preset: ViewportPreset) {
  if (preset.mode === "adaptive") return props.viewportAdaptiveActive;
  return !props.viewportAdaptiveActive && props.viewportSize.cssWidth === preset.width && props.viewportSize.cssHeight === preset.height;
}

function viewportPresetShellLabel(preset: ViewportPreset) {
  if (preset.mode === "adaptive") return "自适应";
  return preset.shell === "mobile" ? "移动壳" : "桌面壳";
}

function viewportPresetDescription(preset: ViewportPreset) {
  if (preset.mode === "adaptive" && !props.viewportRuntimeSupported) {
    return "跟随当前浏览器或 WebView viewport，作为 WebUI 默认尺寸模式。";
  }
  return preset.description;
}

function isViewportPresetDisabled(preset: ViewportPreset) {
  if (props.viewportSwitching) return true;
  return !props.viewportRuntimeSupported && preset.mode !== "adaptive";
}
</script>

<template>
  <section class="settings-domain">
    <div class="settings-domain-header">
      <div class="settings-domain-title-wrap" :class="{ pinned: sections.pinnedHelpSection === 'general' }">
        <h3 class="settings-domain-title flex items-center">
          通用设置
          <button type="button" class="settings-help-btn" aria-label="查看说明" @click="sections.toggleHelp('general')">
            <PhQuestion size="13" />
          </button>
        </h3>
        <p class="settings-domain-copy">检查在线更新，调整窗口尺寸、显示主题，管理 COLO 词典与时间显示。</p>
      </div>
    </div>
    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <details :open="sections.isOpen('updates')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('updates', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <div class="min-w-0">
            <h3 class="flex items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhArrowsClockwise class="mr-2 shrink-0 text-primary" size="20" weight="bold" />
              在线更新
            </h3>
            <p class="mt-1 hidden truncate text-xs text-slate-500 sm:block">当前版本 {{ appInfo.current_version || "1.0" }} · {{ appInfo.platform || "desktop" }}</p>
          </div>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ updateStatusLabel }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('updates') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('updates')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 lg:grid-cols-[1fr_auto] lg:items-center lg:p-4">
          <div class="min-w-0">
            <p class="text-sm font-medium text-slate-700">{{ updateState.message }}</p>
            <p v-if="updateState.latestVersion" class="mt-2 text-xs text-slate-500">最新版本 {{ updateState.latestVersion }}{{ updateState.assetName ? ` · ${updateState.assetName}` : "" }}</p>
            <p v-if="updateRequiresWebUIDeployGuide && updateState.dockerImage" class="mt-2 break-all font-mono text-xs text-slate-500">{{ updateState.dockerImage }}</p>
            <p v-if="updateState.downloadPath" class="mt-2 break-all font-mono text-xs text-slate-500">{{ updateState.downloadPath }}</p>
            <p v-if="updateRequiresWebUIDeployGuide" class="mt-2 text-xs text-slate-500">Linux WebUI 发行包仅提供下载；请解压后按 Docker Compose 或 `run-local.sh` 的方式手动部署。</p>
            <p v-else-if="updateRequiresManualInstall" class="mt-2 text-xs text-slate-500">当前平台未触发自动覆盖安装；请在下载完成后按系统提示手动安装或替换现有文件。</p>
          </div>
          <div class="grid gap-2 sm:flex sm:flex-wrap sm:justify-end sm:gap-3">
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading || updateState.status === 'checking'" @click="$emit('check-update')">
              <PhArrowsClockwise size="18" />
              检查更新
            </button>
            <button type="button" class="ui-button ui-button-primary" :disabled="loading || !updateState.updateAvailable || updateState.installing" @click="$emit('install-update')">
              <PhDownload size="18" />
              {{ updateRequiresManualInstall ? "下载更新包" : "下载并安装" }}
            </button>
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('open-release-page')">
              <PhArrowSquareOut size="18" />
              发行页
            </button>
          </div>
        </div>
      </details>

      <details :open="sections.isOpen('viewport')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('viewport', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <div class="min-w-0">
            <h3 class="flex items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhGauge class="mr-2 shrink-0 text-primary" size="20" weight="fill" />
              UI尺寸设置
            </h3>
          </div>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ viewportSummaryLabel }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('viewport') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('viewport')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
          <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            <button
              v-for="preset in viewportPresets"
              :key="preset.id"
              type="button"
              class="rounded-xl border px-4 py-3 text-left transition disabled:cursor-not-allowed disabled:opacity-60 lg:px-3 lg:py-2.5"
              :class="isViewportPresetActive(preset) ? 'border-primary bg-indigo-50 text-slate-800 shadow-sm' : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300'"
              :disabled="isViewportPresetDisabled(preset)"
              @click="$emit('apply-viewport-preset', preset.id)"
            >
              <span class="flex items-center justify-between gap-3">
                <span class="font-mono text-sm font-semibold">{{ preset.label }}</span>
                <span class="ui-pill ui-pill-subtle">{{ viewportPresetShellLabel(preset) }}</span>
              </span>
              <span class="mt-2 block text-xs text-slate-500">{{ viewportPresetDescription(preset) }}</span>
            </button>
          </div>

          <div class="grid gap-3 text-sm text-slate-600 sm:grid-cols-2">
            <div class="rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <p class="text-xs font-semibold text-slate-500">窗口尺寸</p>
              <p class="mt-2 font-mono text-base font-semibold text-slate-800">{{ viewportSize.width || "-" }} x {{ viewportSize.height || "-" }}</p>
            </div>
            <div class="rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <p class="text-xs font-semibold text-slate-500">页面视口</p>
              <p class="mt-2 font-mono text-base font-semibold text-slate-800">{{ viewportSize.cssWidth || "-" }} x {{ viewportSize.cssHeight || "-" }}</p>
            </div>
          </div>

          <p class="text-xs text-slate-500">
            {{ viewportRuntimeSupported ? "自适应会最大化窗口；固定尺寸会调整真实桌面窗口并居中，若显示器尺寸不足，以系统实际钳制后的回显为准。" : "浏览器 / WebUI 中会随窗口自适应；固定验收尺寸仅在桌面应用内生效。" }}
          </p>
        </div>
      </details>

      <details :open="sections.isOpen('appearance')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('appearance', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhMoon class="mr-2 shrink-0 text-slate-600" size="20" weight="fill" />
            外观与自动主题
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ themeSummaryLabel }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('appearance') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('appearance')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
          <label class="md:col-span-2">
            <span class="ui-label">主题模式</span>
            <select v-model="settings.themeMode" class="ui-field">
              <option value="auto_system_time">跟随系统，失败时按时间</option>
              <option value="auto_time">按本地时间自动切换</option>
              <option value="light">固定浅色</option>
              <option value="dark">固定深色</option>
            </select>
            <p class="mt-2 text-xs text-slate-500">“跟随系统”优先监听系统深浅色，失败时才按时间兜底；“按本地时间”会始终核对设备当前时间并切换主题。</p>
          </label>
          <label>
            <span class="ui-label">浅色开始时间</span>
            <input v-model="settings.themeLightStart" type="time" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">深色开始时间</span>
            <input v-model="settings.themeDarkStart" type="time" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">UTC 偏移（分钟）</span>
            <input v-model.number="settings.utcOffsetMinutes" min="-720" max="840" step="15" type="number" class="ui-field" />
            <p class="mt-2 text-xs text-slate-500">默认 UTC+8 为 480。所有时间显示与“按时间自动切换主题”都会使用这里的时区偏移。</p>
          </label>
          <div class="rounded-xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-600">
            <p class="text-xs font-semibold text-slate-500">当前时区</p>
            <p class="mt-2 font-mono text-base font-semibold text-slate-800">{{ utcOffsetLabel }}</p>
          </div>
          <div class="md:col-span-2 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-xs text-slate-500">当前保存到配置的字段为 theme_mode、theme_light_start、theme_dark_start 和 utc_offset_minutes，会随草稿一起保存。</div>
        </div>
      </details>

      <details :open="sections.isOpen('colo')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('colo', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhGlobeHemisphereWest class="mr-2 shrink-0 text-slate-600" size="20" />
            COLO 词典
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ coloDictionaryUpdating ? "拉取中" : coloDictionaryProcessing ? "处理中" : `综合 ${coloDictionaryStatus?.colo_rows || 0} 行` }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('colo') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('colo')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
          <div class="grid gap-2 sm:flex sm:flex-wrap sm:gap-3">
            <button type="button" class="ui-button ui-button-secondary" :disabled="coloDictionaryUpdating" @click="$emit('refresh-colo-dictionary')">
              <PhArrowsClockwise size="18" />
              {{ coloDictionaryUpdating ? "拉取中" : "更新词典" }}
            </button>
            <button type="button" class="ui-button ui-button-primary" :disabled="coloDictionaryProcessing" @click="$emit('process-colo-dictionary')">
              <PhArrowsClockwise size="18" />
              {{ coloDictionaryProcessing ? "处理中" : "处理词典" }}
            </button>
          </div>
          <p class="text-xs text-slate-500">可选功能：更新 Cloudflare 节点名称，用于输入源 COLO 预筛、国家/COLO 筛选和 MICS 抽样。普通测速不需要设置这里。</p>
          <div class="grid gap-3 rounded-xl border border-slate-200 bg-white p-4 text-sm text-slate-600 sm:grid-cols-3 xl:grid-cols-6">
            <div>
              <p class="text-xs font-semibold text-slate-500">GEOFEED</p>
              <p class="mt-2 text-base font-semibold text-slate-800">{{ coloDictionaryStatus?.geofeed_rows || 0 }}</p>
            </div>
            <div>
              <p class="text-xs font-semibold text-slate-500">综合</p>
              <p class="mt-2 text-base font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_rows || 0 }}</p>
            </div>
            <div>
              <p class="text-xs font-semibold text-slate-500">IPv4</p>
              <p class="mt-2 text-base font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_ipv4_rows || 0 }}</p>
            </div>
            <div>
              <p class="text-xs font-semibold text-slate-500">IPv6</p>
              <p class="mt-2 text-base font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_ipv6_rows || 0 }}</p>
            </div>
            <div>
              <p class="text-xs font-semibold text-slate-500">未覆盖</p>
              <p class="mt-2 text-base font-semibold text-slate-800">{{ coloDictionaryStatus?.unmatched_rows || 0 }}</p>
            </div>
            <div>
              <p class="text-xs font-semibold text-slate-500">更新时间</p>
              <p class="mt-2 break-all text-xs text-slate-700">{{ coloDictionaryUpdatedAt }}</p>
            </div>
          </div>
          <div class="space-y-1 text-xs text-slate-500">
            <p class="break-all">GEOFEED：{{ coloDictionaryStatus?.geofeed_path || "local-ip-ranges.csv" }}</p>
            <p class="break-all">综合：{{ coloDictionaryStatus?.colo_path || "cloudflare-colos.csv" }}</p>
            <p class="break-all">IPv4：{{ coloDictionaryStatus?.colo_ipv4_path || "cloudflare-colos-ipv4.csv" }}</p>
            <p class="break-all">IPv6：{{ coloDictionaryStatus?.colo_ipv6_path || "cloudflare-colos-ipv6.csv" }}</p>
            <p>更新词典只拉取原始文件；处理词典会读取本地文件生成综合、IPv4、IPv6 COLO。未覆盖仅表示 GEOFEED 行暂未派生出 COLO，不代表测速失败。</p>
          </div>
        </div>
      </details>
    </div>
  </section>
</template>
