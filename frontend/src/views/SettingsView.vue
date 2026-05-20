<script setup lang="ts">
import { computed, ref } from "vue";
import { PhCloud, PhArrowSquareOut, PhArrowsClockwise, PhCaretDown, PhDatabase, PhDownload, PhEye, PhEyeSlash, PhFileArrowUp, PhFloppyDisk, PhFolderOpen, PhGauge, PhMoon, PhShieldCheck } from "@phosphor-icons/vue";

interface SettingsForm {
  apiToken: string;
  comment: string;
  uploadCloudflareTopN: number;
  uploadGitHubTopN: number;
  uploadSharedFilterColoAllow: string;
  uploadSharedFilterColoDeny: string;
  uploadSharedFilterEnabled: boolean;
  uploadSharedFilterIPVersion: "any" | "ipv4" | "ipv6";
  uploadSharedFilterMaxLossRate: number | null;
  uploadSharedFilterMaxTcpLatencyMs: number | null;
  uploadSharedFilterMaxTraceLatencyMs: number | null;
  uploadSharedFilterMinDownloadMbps: number;
  uploadSharedFilterStatus: "all" | "passed";
  exportFileName: string;
  exportFileNameTemplate: string;
  githubBranch: string;
  githubCSVHeaderTemplate: string;
  githubCSVRowTemplate: string;
  githubCommitMessageTemplate: string;
  githubExportEnabled: boolean;
  githubFormat: "csv" | "txt";
  githubLastExportAt: string;
  githubOwner: string;
  githubPathTemplate: string;
  githubRepo: string;
  githubToken: string;
  githubTXTRowTemplate: string;
  exportCSVEncoding: CSVEncoding;
  exportOverwrite: string;
  exportTargetDir: string;
  exportTargetUri: string;
  maxHttpLatencyMs: number | null;
  maxTcpLatencyMs: number | null;
  maxLossRate: number;
  minDownloadMbps: number;
  minDelayMs: number;
  probeDebug: boolean;
  probeDebugCaptureAddress: string;
  probeDebugCaptureEnabled: boolean;
  probeDebugLogFormat: string;
  probeDebugLogMode: "structured" | "freeform";
  probeDebugLogVerbosity: "simple" | "detailed";
  probeDisableDownload: boolean;
  probeConcurrencyStage1: number;
  probeConcurrencyStage2: number;
  probeConcurrencyStage3: number;
  probeCooldownFailures: number;
  probeCooldownMs: number;
  probeDownloadBufferKB: number;
  probeDownloadCount: number;
  probeDownloadGetConcurrency: number;
  probeDownloadHTTPProtocol: "auto" | "h1" | "h2" | "h3";
  probeDownloadSpeedMetric: DownloadSpeedMetric;
  probeDownloadSpeedSampleIntervalMs: number;
  probeDownloadTimeSeconds: number;
  probeDownloadWarmupSeconds: number;
  probeEventThrottleMs: number;
  probeHostHeader: string;
  probeHttping: boolean;
  probeHttpingCfColo: string;
  probeHttpingCfColoMode: ColoFilterMode;
  probeHttpingStatusCode: number;
  probePingTimes: number;
  probePrintNum: number;
  probePortPolicy: "source_override_global" | "fixed_global";
  probeRetryBackoffMs: number;
  probeRetryMaxAttempts: number;
  probeRequestHeaders: string;
  probeSNI: string;
  probeSourceColoFilterPhase: "precheck" | "stage2";
  probeStageLimitStage3: number;
  probeStrategy: "fast" | "full";
  probeTcpPort: number;
  probeTimeoutStage1Ms: number;
  probeTimeoutStage2Ms: number;
  probeTimeoutStage3Ms: number;
  probeTraceColoMode: "standard" | "trace_url";
  probeTraceURL: string;
  probeURL: string;
  probeUserAgent: string;
  proxied: boolean;
  recordName: string;
  schedulerAutoDnsPush: boolean;
  schedulerAutoGithubExport: boolean;
  schedulerDailyTimes: string;
  schedulerEnabled: boolean;
  schedulerIntervalMinutes: number;
  schedulerSkipIfActive: boolean;
  sourceAutoDetectName: boolean;
  themeDarkStart: string;
  themeLightStart: string;
  themeMode: "light" | "dark" | "auto_system_time" | "auto_time";
  utcOffsetMinutes: number;
  ttl: number;
  webdavEnabled: boolean;
  webdavLastBackupAt: string;
  webdavLastRestoreAt: string;
  webdavPassword: string;
  webdavRemotePath: string;
  webdavServerURL: string;
  webdavTimeoutSeconds: number;
  webdavUsername: string;
  zoneId: string;
}

type ColoFilterMode = "allow" | "deny";
type CSVEncoding = "utf-8" | "utf-8-bom";
type DownloadSpeedMetric = "average" | "max";

interface StorageStatus {
  backend?: "private" | "saf_mirror";
  current_dir: string;
  default_dir: string;
  display_name?: string;
  health?: {
    free_bytes: number;
    message: string;
    writable: boolean;
  };
  last_sync_at?: string;
  last_sync_error?: string;
  log_uri?: string;
  permission_ok?: boolean;
  portable_mode: boolean;
  runtime_dir?: string;
  setup_required: boolean;
  storage_uri?: string;
  writable: boolean;
}

interface AndroidBatteryStatus {
  brand: string;
  ignoring_optimizations: boolean;
  manufacturer: string;
  model: string;
  needs_guidance: boolean;
  settings_hint: string;
  supported: boolean;
}

const REQUEST_HEADERS_TEMPLATE = ["Accept: */*", "Accept-Language: zh-CN,zh;q=0.9,en;q=0.8", "Cache-Control: no-cache", "Pragma: no-cache", "DNT: 1", "Sec-Fetch-Dest: empty", "Sec-Fetch-Mode: cors", "Sec-Fetch-Site: none"].join("\n");

interface AppInfo {
  current_version: string;
  install_mode: string;
  platform: string;
  release_url: string;
}

interface UpdateState {
  assetName: string;
  checkedAt: string;
  downloadPath: string;
  installStarted: boolean;
  installMode: string;
  installing: boolean;
  latestVersion: string;
  message: string;
  nextAction: string;
  releaseUrl: string;
  status: "idle" | "checking" | "available" | "latest" | "installing" | "ready" | "failed";
  updateAvailable: boolean;
}

interface ProfileListItem {
  config_snapshot?: Record<string, unknown>;
  id: string;
  name: string;
  updated_at: string;
}

interface ProfileStore {
  active_profile_id: string;
  items: ProfileListItem[];
}

interface SchedulerStatus {
  cloudflare_upload_count?: number;
  enabled: boolean;
  github_upload_count?: number;
  next_run_at: string;
  last_run_at: string;
  last_task_id: string;
  last_probe_status: string;
  last_dns_status: string;
  last_github_status: string;
  last_message: string;
  upload_filtered_count?: number;
  upload_input_count?: number;
  workflow_stage?: string;
  config_source?: string;
  last_profile_action?: string;
  last_source_profile_action?: string;
}

interface ViewportPreset {
  description: string;
  height?: number;
  id: string;
  label: string;
  mode: "adaptive" | "fixed";
  shell: "mobile" | "desktop";
  width?: number;
}

interface ViewportSize {
  cssHeight: number;
  cssWidth: number;
  height: number;
  updatedAt: string;
  width: number;
}

interface TimestampFormatOptions {
  fallback?: string;
  includeDate?: boolean;
  includeOffset?: boolean;
  includeSeconds?: boolean;
}

type SettingsSectionKey = "updates" | "viewport" | "appearance" | "storage" | "backup" | "profiles" | "sources" | "cloudflare" | "probe" | "scheduler" | "export" | "protection" | "debug";

const props = defineProps<{
  appInfo: AppInfo;
  loading: boolean;
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  githubTesting: boolean;
  maskedTokenHint: string;
  platform: "desktop" | "mobile";
  profiles: ProfileStore;
  androidBatteryStatus?: AndroidBatteryStatus | null;
  saveBlockedByMaskedToken: boolean;
  settings: SettingsForm;
  showToken: boolean;
  schedulerStatus: SchedulerStatus | null;
  storage: StorageStatus | null;
  updateState: UpdateState;
  viewportAdaptiveActive: boolean;
  viewportPresets: ViewportPreset[];
  viewportRuntimeSupported: boolean;
  viewportSize: ViewportSize;
  viewportSwitching: boolean;
  utcOffsetLabel: string;
}>();

const emit = defineEmits<{
  (event: "apply-viewport-preset", presetId: string): void;
  (event: "open-battery-settings", mode: "request" | "settings" | "details"): void;
  (event: "backup-config-webdav"): void;
  (event: "check-storage-health"): void;
  (event: "check-update"): void;
  (event: "delete-profile", profileId: string): void;
  (event: "export-config"): void;
  (event: "import-config"): void;
  (event: "open-storage-dir"): void;
  (event: "open-release-page"): void;
  (event: "refresh"): void;
  (event: "save"): void;
  (event: "save-profile", name: string, profileId?: string, configSnapshot?: Record<string, unknown>, setActive?: boolean): void;
  (event: "update-current-profile"): void;
  (event: "select-export-target"): void;
  (event: "select-storage-dir"): void;
  (event: "restore-config-webdav"): void;
  (event: "switch-profile", profileId: string): void;
  (event: "test-webdav"): void;
  (event: "test-github-export"): void;
  (event: "toggle-token"): void;
  (event: "install-update"): void;
  (event: "use-default-storage-dir"): void;
}>();

function strategyLabel(strategy: SettingsForm["probeStrategy"]) {
  return strategy === "full" ? "完整模式" : "极速模式";
}

function overwriteLabel(value: string) {
  return value === "append" ? "追加写入" : "覆盖写出";
}

function coloModeLabel(mode: ColoFilterMode) {
  return mode === "deny" ? "黑名单" : "白名单";
}

function statusText(value: string) {
  const labels: Record<string, string> = {
    completed: "完成",
    failed: "失败",
    skipped: "跳过",
  };
  return value ? labels[value] || value : "未运行";
}

const saveButtonText = computed(() => (props.saveBlockedByMaskedToken ? "需要完整 Token" : "保存配置"));
const profileNameDraft = ref("");
const expandedSections = ref<Record<SettingsSectionKey, boolean>>({
  appearance: false,
  backup: false,
  cloudflare: false,
  debug: false,
  export: false,
  probe: false,
  profiles: false,
  protection: false,
  scheduler: false,
  sources: false,
  storage: true,
  updates: false,
  viewport: false,
});
const activeProfile = computed(() => props.profiles.items.find((profile) => profile.id === props.profiles.active_profile_id) || null);
const isWebUIDesktopShell = computed(() => props.platform === "desktop" && props.appInfo.install_mode === "docker_compose");
const updateRequiresManualInstall = computed(() => props.updateState.installMode === "docker_compose" || props.updateState.nextAction === "manual");
const updateRequiresWebUIDeployGuide = computed(() => props.updateState.installMode === "docker_compose");
const updateStatusLabel = computed(() => {
  const labels: Record<UpdateState["status"], string> = {
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
const storageHealthLabel = computed(() => {
  if (!props.storage) {
    return "未读取";
  }
  if (props.storage.permission_ok === false) {
    return "权限失效";
  }
  if ((props.storage.last_sync_error || "").trim()) {
    return "同步异常";
  }
  if (props.storage.writable) {
    return props.storage.portable_mode ? "便携可写" : "可写";
  }
  return "不可写";
});
const storageDisplayPath = computed(() => props.storage?.display_name || props.storage?.current_dir || props.storage?.storage_uri || "尚未读取储存目录");
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
const schedulerSummaryLabel = computed(() => {
  if (props.platform !== "desktop") {
    return "移动端隐藏";
  }
  if (!props.settings.schedulerEnabled) {
    return "未启用";
  }
  return props.schedulerStatus?.next_run_at ? "已计划" : "等待保存";
});
const batteryStatusLabel = computed(() => {
  if (!props.androidBatteryStatus?.supported) {
    return "系统未提供";
  }
  return props.androidBatteryStatus.ignoring_optimizations ? "已放行" : "待放行";
});
const themeSummaryLabel = computed(() => {
  if (props.settings.themeMode === "light") {
    return "浅色";
  }
  if (props.settings.themeMode === "dark") {
    return "深色";
  }
  if (props.settings.themeMode === "auto_time") {
    return `时间自动 · ${props.utcOffsetLabel}`;
  }
  return "系统自动";
});
const strategyDescription = computed(() => (props.settings.probeStrategy === "full" ? "按 IP池、TCP、追踪、文件测速四阶段执行，所有追踪通过 IP 都会串行进入文件测速。" : "按 IP池、TCP、追踪三阶段执行，跳过文件测速。"));

function workflowLabel(value: string) {
  const labels: Record<string, string> = {
    draft: "草稿配置",
    draft_preferred: "草稿优先",
    formal: "正式配置",
    saved: "正式配置",
    update_recent_run_profile: "更新最近运行配置档案",
    update_recent_run_source_profile: "更新最近运行输入源档案",
  };
  return value ? labels[value] || value : "-";
}

function formatTimestampText(value: string, fallback = "未记录时间") {
  return value.trim() ? props.formatTimestamp(value) : fallback;
}

function formatTimestampLabel(value: string, options?: TimestampFormatOptions) {
  return props.formatTimestamp(value, options);
}
const ttlOptions = [
  { label: "1分钟", value: 60 },
  { label: "5分钟", value: 300 },
  { label: "10分钟", value: 600 },
];

function isSectionOpen(section: SettingsSectionKey) {
  return expandedSections.value[section];
}

function isViewportPresetActive(preset: ViewportPreset) {
  if (preset.mode === "adaptive") {
    return props.viewportAdaptiveActive;
  }
  return !props.viewportAdaptiveActive && props.viewportSize.cssWidth === preset.width && props.viewportSize.cssHeight === preset.height;
}

function viewportPresetShellLabel(preset: ViewportPreset) {
  if (preset.mode === "adaptive") {
    return "自适应";
  }
  return preset.shell === "mobile" ? "移动壳" : "桌面壳";
}

function viewportPresetDescription(preset: ViewportPreset) {
  if (preset.mode === "adaptive" && !props.viewportRuntimeSupported) {
    return "跟随当前浏览器或 WebView viewport，作为 WebUI 默认尺寸模式。";
  }
  return preset.description;
}

function isViewportPresetDisabled(preset: ViewportPreset) {
  if (props.viewportSwitching) {
    return true;
  }
  return !props.viewportRuntimeSupported && preset.mode !== "adaptive";
}

function syncSectionOpen(section: SettingsSectionKey, event: Event) {
  expandedSections.value[section] = (event.currentTarget as HTMLDetailsElement).open;
}

function renameProfile(profile: ProfileListItem) {
  const nextName = window.prompt("新的档案名称", profile.name)?.trim();
  if (!nextName || nextName === profile.name) {
    return;
  }
  emit("save-profile", nextName, profile.id, profile.config_snapshot, profile.id === props.profiles.active_profile_id);
}

function duplicateProfile(profile: ProfileListItem) {
  emit("save-profile", `${profile.name} 副本`, "", profile.config_snapshot, true);
}
</script>

<template>
  <section :class="platform === 'desktop' ? 'space-y-5' : 'space-y-4'">
    <section class="settings-domain">
      <div class="settings-domain-header">
        <div>
          <h3 class="settings-domain-title">通用设置</h3>
          <p class="settings-domain-copy">系统基础配置、界面尺寸和显示主题，影响启动体验与双端可读性。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <span class="ui-pill ui-pill-subtle">{{ updateStatusLabel }}</span>
          <span class="ui-pill ui-pill-subtle">{{ viewportSummaryLabel }}</span>
          <span class="ui-pill ui-pill-subtle">{{ themeSummaryLabel }}</span>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details :open="isSectionOpen('updates')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('updates', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <div class="min-w-0">
              <h3 class="flex items-center text-sm font-semibold text-slate-800 sm:text-lg">
                <PhArrowsClockwise class="mr-2 shrink-0 text-primary" size="20" weight="bold" />
                在线更新
              </h3>
              <p class="mt-1 hidden truncate text-xs text-slate-500 sm:block">当前版本 {{ appInfo.current_version || "1.0" }} · {{ appInfo.platform || "desktop" }}</p>
            </div>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ updateStatusLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('updates') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="grid gap-4 border-t border-slate-100 p-4 sm:p-6 lg:grid-cols-[1fr_auto] lg:items-center lg:p-5">
            <div class="min-w-0">
              <p class="text-sm font-medium text-slate-700">{{ updateState.message }}</p>
              <p v-if="updateState.latestVersion" class="mt-2 text-xs text-slate-500">最新版本 {{ updateState.latestVersion }}{{ updateState.assetName ? ` · ${updateState.assetName}` : "" }}</p>
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

        <details :open="isSectionOpen('viewport')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('viewport', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <div class="min-w-0">
              <h3 class="flex items-center text-sm font-semibold text-slate-800 sm:text-lg">
                <PhGauge class="mr-2 shrink-0 text-primary" size="20" weight="fill" />
                UI尺寸设置
              </h3>
            </div>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ viewportSummaryLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('viewport') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="space-y-4 border-t border-slate-100 p-4 sm:p-6 lg:p-5">
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
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">Wails 窗口</p>
                <p class="mt-2 font-mono text-base font-semibold text-slate-800">{{ viewportSize.width || "-" }} x {{ viewportSize.height || "-" }}</p>
              </div>
              <div class="rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">CSS viewport</p>
                <p class="mt-2 font-mono text-base font-semibold text-slate-800">{{ viewportSize.cssWidth || "-" }} x {{ viewportSize.cssHeight || "-" }}</p>
              </div>
            </div>

            <p class="text-xs text-slate-500">
              {{ viewportRuntimeSupported ? "自适应会最大化窗口；固定尺寸会调整真实桌面窗口并居中，若显示器尺寸不足，以系统实际钳制后的回显为准。" : "Linux WebUI/浏览器会随浏览器窗口自适应；固定验收尺寸仅 Wails 桌面支持。" }}
            </p>
          </div>
        </details>

        <details :open="isSectionOpen('appearance')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('appearance', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhMoon class="mr-2 shrink-0 text-slate-600" size="20" weight="fill" />
              外观与自动主题
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ themeSummaryLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('appearance') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="grid gap-4 border-t border-slate-100 p-4 sm:p-6 md:grid-cols-2 lg:p-5">
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
              <p class="text-xs uppercase tracking-[0.14em] text-slate-500">当前时区</p>
              <p class="mt-2 font-mono text-base font-semibold text-slate-800">{{ utcOffsetLabel }}</p>
            </div>
            <div class="md:col-span-2 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-xs text-slate-500">当前保存到配置的字段为 theme_mode、theme_light_start、theme_dark_start 和 utc_offset_minutes，会随配置档案和草稿一起保存。</div>
          </div>
        </details>
      </div>
    </section>

    <section class="settings-domain">
      <div class="settings-domain-header">
        <div>
          <h3 class="settings-domain-title">数据与存储</h3>
          <p class="settings-domain-copy">储存目录、配置包、同步备份和配置档案都放在这里，先解决“放在哪里”和“怎么恢复”。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <span class="ui-pill ui-pill-subtle">{{ storageHealthLabel }}</span>
          <span class="ui-pill ui-pill-subtle">WebDAV {{ settings.webdavEnabled ? "已启用" : "未启用" }}</span>
          <span class="ui-pill ui-pill-subtle">{{ activeProfile?.name || "未选择档案" }}</span>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details :open="isSectionOpen('storage')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('storage', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhFolderOpen class="mr-2 shrink-0 text-slate-600" size="20" />
              储存目录
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ storageHealthLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('storage') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="space-y-4 border-t border-slate-100 p-4 sm:p-6 lg:p-5">
            <div>
              <span class="ui-label">当前目录</span>
              <p class="break-all rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 font-mono text-xs text-slate-600">
                {{ storageDisplayPath }}
              </p>
              <p v-if="storage?.storage_uri" class="mt-2 break-all text-xs text-slate-500">Android SAF：{{ storage.storage_uri }}</p>
              <p v-if="storage?.runtime_dir" class="mt-2 break-all text-xs text-slate-500">运行时镜像目录：{{ storage.runtime_dir }}</p>
              <p v-if="storage?.last_sync_error" class="mt-2 text-xs text-amber-600">最近同步：{{ storage.last_sync_error }}</p>
              <p v-if="storage?.health?.message" class="mt-2 text-xs text-slate-500">{{ storage.health.message }}</p>
            </div>
            <div class="grid gap-2 sm:flex sm:flex-wrap sm:gap-3">
              <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('select-storage-dir')">
                <PhFolderOpen size="18" />
                选择目录
              </button>
              <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('open-storage-dir')">打开目录</button>
              <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('check-storage-health')">健康检查</button>
              <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('use-default-storage-dir')">重置默认</button>
            </div>
            <p class="text-xs text-slate-500">更换目录时会复制现有配置、词典、日志和结果文件；旧目录不会自动删除。</p>
          </div>
        </details>

        <details :open="isSectionOpen('backup')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('backup', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhCloud class="mr-2 shrink-0 text-cf" size="20" weight="fill" />
              配置备份与同步
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">统一 ZIP</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('backup') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="grid gap-4 border-t border-slate-100 p-4 sm:p-6 md:grid-cols-2 lg:p-5">
            <div class="md:col-span-2 grid gap-2 sm:grid-cols-2">
              <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('export-config')">
                <PhFileArrowUp size="18" />
                导出配置包
              </button>
              <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('import-config')">
                <PhFileArrowUp size="18" />
                加载配置包
              </button>
            </div>

            <label class="md:col-span-2">
              <span class="ui-label">WebDAV 地址</span>
              <input v-model="settings.webdavServerURL" placeholder="https://example.com/dav/backups/" type="url" class="ui-field font-mono" />
            </label>
            <label>
              <span class="ui-label">用户名</span>
              <input v-model="settings.webdavUsername" type="text" class="ui-field" autocomplete="username" />
            </label>
            <label>
              <span class="ui-label">密码 / Token</span>
              <input v-model="settings.webdavPassword" type="password" class="ui-field" autocomplete="current-password" />
            </label>
            <label>
              <span class="ui-label">远端文件</span>
              <input v-model="settings.webdavRemotePath" placeholder="cfst-gui-config.zip" type="text" class="ui-field font-mono" />
            </label>
            <label>
              <span class="ui-label">超时（秒）</span>
              <input v-model.number="settings.webdavTimeoutSeconds" min="1" type="number" class="ui-field" />
            </label>

            <button type="button" class="md:col-span-2 flex items-center justify-between gap-4 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left" @click="settings.webdavEnabled = !settings.webdavEnabled">
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">启用 WebDAV 备份配置</span>
                <span class="text-xs text-slate-500">只影响手动测试、备份和还原，不会自动同步。</span>
              </span>
              <span class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="settings.webdavEnabled ? 'bg-primary' : 'bg-slate-300'">
                <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.webdavEnabled ? 'translate-x-5' : 'translate-x-0'"></span>
              </span>
            </button>

            <div class="md:col-span-2 grid gap-2 sm:grid-cols-3">
              <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('test-webdav')">测试 WebDAV</button>
              <button type="button" class="ui-button ui-button-primary" :disabled="loading || !settings.webdavEnabled" @click="$emit('backup-config-webdav')">备份到 WebDAV</button>
              <button type="button" class="ui-button ui-button-secondary" :disabled="loading || !settings.webdavEnabled" @click="$emit('restore-config-webdav')">从 WebDAV 还原</button>
            </div>

            <div class="md:col-span-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-700">
              配置包完整包含 Cloudflare Token 和 WebDAV 凭据。最近 WebDAV 备份：{{ formatTimestampText(settings.webdavLastBackupAt, "无") }}；最近还原：{{ formatTimestampText(settings.webdavLastRestoreAt, "无") }}。
            </div>
          </div>
        </details>

        <details :open="isSectionOpen('profiles')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('profiles', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhFloppyDisk class="mr-2 shrink-0 text-primary" size="20" weight="fill" />
              配置档案
            </h3>
            <div class="flex min-w-0 shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle max-w-36 truncate sm:max-w-none">{{ activeProfile?.name || "未选择档案" }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('profiles') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="space-y-4 border-t border-slate-100 p-4 sm:p-6 lg:p-5">
            <label>
              <span class="ui-label">保存为档案</span>
              <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:gap-3">
                <input v-model="profileNameDraft" class="ui-field" placeholder="例如：家庭宽带 / 服务器 DNS" type="text" />
                <button type="button" class="ui-button ui-button-primary" :disabled="loading" @click="$emit('save-profile', profileNameDraft)">保存档案</button>
              </div>
            </label>
            <button type="button" class="ui-button ui-button-secondary" :disabled="loading" @click="$emit('update-current-profile')">更新并保存当前档案</button>
            <div v-if="profiles.items.length > 0" class="space-y-2">
              <div v-for="profile in profiles.items" :key="profile.id" class="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-slate-700">{{ profile.name }}</p>
                  <p class="text-xs text-slate-400">{{ profile.id === profiles.active_profile_id ? "当前档案" : formatTimestampText(profile.updated_at, "未记录更新时间") }}</p>
                </div>
                <div class="grid grid-cols-2 gap-2 sm:flex sm:shrink-0">
                  <button type="button" class="ui-button ui-button-ghost px-3 py-2" :disabled="loading || profile.id === profiles.active_profile_id" @click="$emit('switch-profile', profile.id)">切换</button>
                  <button type="button" class="ui-button ui-button-ghost px-3 py-2" :disabled="loading" @click="renameProfile(profile)">重命名</button>
                  <button type="button" class="ui-button ui-button-ghost px-3 py-2" :disabled="loading" @click="duplicateProfile(profile)">复制</button>
                  <button type="button" class="ui-button ui-button-ghost px-3 py-2" :disabled="loading" @click="$emit('delete-profile', profile.id)">删除</button>
                </div>
              </div>
            </div>
            <p v-else class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-sm text-slate-500">还没有配置档案；保存当前配置后可在不同网络环境之间快速切换。</p>
          </div>
        </details>
      </div>
    </section>

    <section class="settings-domain">
      <div class="settings-domain-header">
        <div>
          <h3 class="settings-domain-title">网络与任务</h3>
          <p class="settings-domain-copy">输入源命名、Cloudflare 上传目标和探测参数集中放在一个内容域里，方便按任务流从上到下阅读。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <span class="ui-pill ui-pill-subtle">{{ settings.sourceAutoDetectName ? "来源自动识别" : "来源手动命名" }}</span>
          <span class="ui-pill ui-pill-subtle">TTL {{ settings.ttl }}</span>
          <span class="ui-pill ui-pill-subtle">{{ strategyLabel(settings.probeStrategy) }}</span>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details :open="isSectionOpen('sources')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('sources', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhDatabase class="mr-2 shrink-0 text-slate-600" size="20" weight="fill" />
              输入源行为
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ settings.sourceAutoDetectName ? "自动识别" : "手动命名" }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('sources') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="border-t border-slate-100 p-4 sm:p-6 lg:p-5">
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

        <details :open="isSectionOpen('cloudflare')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('cloudflare', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhCloud class="mr-2 shrink-0 text-cf" size="20" weight="fill" />
              Cloudflare 配置
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">自动识别 A / AAAA</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('cloudflare') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="grid gap-4 border-t border-slate-100 p-4 sm:p-6 md:grid-cols-2 lg:p-5">
            <label class="md:col-span-2">
              <span class="ui-label">API Token</span>
              <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:gap-3">
                <input v-model="settings.apiToken" :placeholder="maskedTokenHint || '重新输入完整 Token 以保存'" :type="showToken ? 'text' : 'password'" class="ui-field" />
                <button type="button" class="ui-button ui-button-ghost px-4" @click="$emit('toggle-token')">
                  <component :is="showToken ? PhEyeSlash : PhEye" :size="18" />
                  {{ showToken ? "隐藏" : "显示" }}
                </button>
              </div>
            </label>

            <label>
              <span class="ui-label">Zone ID</span>
              <input v-model="settings.zoneId" type="text" class="ui-field font-mono" />
            </label>
            <label>
              <span class="ui-label">记录名称</span>
              <input v-model="settings.recordName" type="text" class="ui-field font-mono" />
            </label>
            <label>
              <span class="ui-label">TTL</span>
              <select v-model.number="settings.ttl" class="ui-field">
                <option v-for="option in ttlOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </label>
            <div class="flex items-center rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-sm text-slate-500">DNS 记录类型会按 IP 自动识别：IPv4 写入 A，IPv6 写入 AAAA。</div>
            <label class="md:col-span-2">
              <span class="ui-label">备注</span>
              <input v-model="settings.comment" type="text" class="ui-field" />
            </label>

            <div class="md:col-span-2 flex items-center justify-between gap-4 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <div class="min-w-0">
                <p class="text-sm font-medium text-slate-700">开启代理</p>
                <p class="text-xs text-slate-400">对应 Cloudflare 的 orange cloud 开关。</p>
              </div>
              <button type="button" class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="settings.proxied ? 'bg-primary' : 'bg-slate-300'" @click="settings.proxied = !settings.proxied">
                <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.proxied ? 'translate-x-5' : 'translate-x-0'"></span>
              </button>
            </div>
          </div>
        </details>

        <details :open="isSectionOpen('probe')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('probe', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhGauge class="mr-2 shrink-0 text-primary" size="20" weight="fill" />
              探测策略
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ strategyLabel(settings.probeStrategy) }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('probe') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="space-y-4 border-t border-slate-100 p-4 sm:p-6 lg:p-5">
            <section class="space-y-3">
              <span class="ui-label">策略预设</span>
              <div class="grid gap-3 md:grid-cols-2">
                <button type="button" class="rounded-xl border px-4 py-4 text-left transition lg:px-3 lg:py-3" :class="settings.probeStrategy === 'fast' ? 'border-primary bg-indigo-50 text-slate-800 shadow-sm' : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300'" @click="settings.probeStrategy = 'fast'">
                  <p class="text-sm font-semibold">极速模式</p>
                  <p class="mt-1 text-xs text-slate-500">执行 IP池、TCP测延迟、追踪探测，跳过文件测速。</p>
                </button>
                <button type="button" class="rounded-xl border px-4 py-4 text-left transition lg:px-3 lg:py-3" :class="settings.probeStrategy === 'full' ? 'border-primary bg-indigo-50 text-slate-800 shadow-sm' : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300'" @click="settings.probeStrategy = 'full'">
                  <p class="text-sm font-semibold">完整模式</p>
                  <p class="mt-1 text-xs text-slate-500">在追踪通过后追加文件测速，文件测速串行执行。</p>
                </button>
              </div>
              <p class="text-sm text-slate-500">{{ strategyDescription }}</p>
            </section>

            <section class="space-y-3 border-t border-slate-100 pt-5">
              <div>
                <p class="text-sm font-semibold text-slate-800">基础与目标设置</p>
                <p class="mt-1 text-xs text-slate-500">每次测速最常调整的目标地址、端口和结果范围。</p>
              </div>
              <div class="grid gap-4 md:grid-cols-2">
                <label class="md:col-span-2">
                  <span class="ui-label">文件测速 URL</span>
                  <input v-model="settings.probeURL" type="url" class="ui-field font-mono" />
                  <p class="mt-2 text-xs text-slate-500">文件测速阶段只访问该文件 URL；不要填写 /cdn-cgi/trace。</p>
                </label>
                <label class="md:col-span-2">
                  <span class="ui-label">追踪 URL（可选）</span>
                  <input v-model="settings.probeTraceURL" placeholder="留空时从文件测速URL派生 /cdn-cgi/trace" type="url" class="ui-field font-mono" />
                </label>
                <div>
                  <span class="ui-label">输入源 COLO 筛选阶段</span>
                  <div class="mt-2 inline-flex max-w-full flex-wrap rounded-full border border-slate-200 bg-slate-100 p-1">
                    <button
                      type="button"
                      class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs"
                      :class="settings.probeSourceColoFilterPhase === 'precheck' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                      @click="settings.probeSourceColoFilterPhase = 'precheck'"
                    >
                      cloudflare-colos
                    </button>
                    <button
                      type="button"
                      class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs"
                      :class="settings.probeSourceColoFilterPhase === 'stage2' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                      @click="settings.probeSourceColoFilterPhase = 'stage2'"
                    >
                      第二阶段起效
                    </button>
                  </div>
                </div>
                <div>
                  <span class="ui-label">第二阶段 COLO 获取模式</span>
                  <div class="mt-2 inline-flex max-w-full flex-wrap rounded-full border border-slate-200 bg-slate-100 p-1">
                    <button type="button" class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs" :class="settings.probeTraceColoMode === 'standard' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probeTraceColoMode = 'standard'">
                      标准
                    </button>
                    <button type="button" class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs" :class="settings.probeTraceColoMode === 'trace_url' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probeTraceColoMode = 'trace_url'">
                      追踪URL
                    </button>
                  </div>
                </div>
                <label>
                  <span class="ui-label">测速端口</span>
                  <input v-model.number="settings.probeTcpPort" min="1" max="65535" type="number" class="ui-field" />
                </label>
                <div>
                  <span class="ui-label">端口策略</span>
                  <div class="mt-2 inline-flex max-w-full flex-wrap rounded-full border border-slate-200 bg-slate-100 p-1">
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
                  <p class="mt-2 text-xs text-slate-500">输入源声明端口时可优先使用；未声明端口仍会回退到上面的固定测速端口。</p>
                </div>
                <div class="grid gap-4 md:col-span-2 md:grid-cols-2">
                  <label>
                    <span class="ui-label">测速上限</span>
                    <input v-model.number="settings.probeStageLimitStage3" min="1" type="number" class="ui-field" />
                    <p class="mt-2 text-xs text-slate-500">限制完整模式进入文件测速的候选数；极速模式不执行文件测速。</p>
                  </label>
                  <label>
                    <span class="ui-label">结果显示数量</span>
                    <input v-model.number="settings.probePrintNum" min="0" type="number" class="ui-field" />
                    <p class="mt-2 text-xs text-slate-500">0 不限制；正数按 30% 延迟 + 70% 速率评分筛选最终结果。</p>
                  </label>
                </div>
                <div class="md:col-span-2">
                  <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                    <span class="ui-label mb-0">最终地区码筛选</span>
                    <div class="inline-flex max-w-full flex-wrap rounded-full border border-slate-200 bg-slate-100 p-1">
                      <button type="button" class="rounded-full px-3 py-1.5 text-xs font-semibold transition" :class="settings.probeHttpingCfColoMode === 'allow' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probeHttpingCfColoMode = 'allow'">白名单</button>
                      <button type="button" class="rounded-full px-3 py-1.5 text-xs font-semibold transition" :class="settings.probeHttpingCfColoMode === 'deny' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'" @click="settings.probeHttpingCfColoMode = 'deny'">黑名单</button>
                    </div>
                  </div>
                  <input v-model="settings.probeHttpingCfColo" placeholder="HKG,NRT,LAX" type="text" class="ui-field font-mono" />
                  <p class="mt-2 text-xs text-slate-500">当前为{{ coloModeLabel(settings.probeHttpingCfColoMode) }}模式；空列表不限制。白名单会拒绝未知 COLO，黑名单会放行未知 COLO。</p>
                </div>
              </div>
            </section>

            <section class="space-y-3 border-t border-slate-100 pt-5">
              <div>
                <p class="text-sm font-semibold text-slate-800">筛选与阈值条件</p>
                <p class="mt-1 text-xs text-slate-500">决定 IP 是否进入后续阶段或最终结果的合格标准。</p>
              </div>
              <div class="grid gap-4 md:grid-cols-2">
                <label>
                  <span class="ui-label">TCP 延迟上限（ms）</span>
                  <input v-model.number="settings.maxTcpLatencyMs" min="1" placeholder="留空" type="number" class="ui-field" />
                </label>
                <label>
                  <span class="ui-label">TCP 延迟下限（ms）</span>
                  <input v-model.number="settings.minDelayMs" min="0" type="number" class="ui-field" />
                </label>
                <label>
                  <span class="ui-label">TCP 丢包率上限（最大 100%）</span>
                  <input v-model.number="settings.maxLossRate" max="1" min="0" step="0.01" type="number" class="ui-field" />
                </label>
                <label>
                  <span class="ui-label">最低下载速度 (MB/s)</span>
                  <input v-model.number="settings.minDownloadMbps" :disabled="settings.probeStrategy === 'fast'" min="0" step="0.1" type="number" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
                </label>
                <div>
                  <span class="ui-label">下载速率依据</span>
                  <div class="mt-2 inline-flex max-w-full flex-wrap rounded-full border border-slate-200 bg-slate-100 p-1" :class="settings.probeStrategy === 'fast' ? 'opacity-60' : ''">
                    <button
                      type="button"
                      class="rounded-full px-4 py-2 text-sm font-semibold transition disabled:cursor-not-allowed lg:px-3 lg:py-1.5 lg:text-xs"
                      :class="settings.probeDownloadSpeedMetric === 'average' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                      :disabled="settings.probeStrategy === 'fast'"
                      @click="settings.probeDownloadSpeedMetric = 'average'"
                    >
                      平均速率
                    </button>
                    <button
                      type="button"
                      class="rounded-full px-4 py-2 text-sm font-semibold transition disabled:cursor-not-allowed lg:px-3 lg:py-1.5 lg:text-xs"
                      :class="settings.probeDownloadSpeedMetric === 'max' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                      :disabled="settings.probeStrategy === 'fast'"
                      @click="settings.probeDownloadSpeedMetric = 'max'"
                    >
                      最高速率
                    </button>
                  </div>
                  <p class="mt-2 text-xs text-slate-500">仅影响最低下载速度阈值和结果显示数量评分。</p>
                </div>
                <label>
                  <span class="ui-label">追踪有效状态码</span>
                  <input v-model.number="settings.probeHttpingStatusCode" max="599" min="0" type="number" class="ui-field" />
                  <p class="mt-2 text-xs text-slate-500">默认 0 不限制；设置 100-599 才启用状态码筛选。</p>
                </label>
              </div>
            </section>

            <section class="space-y-3 border-t border-slate-100 pt-5">
              <div>
                <p class="text-sm font-semibold text-slate-800">并发与性能控制</p>
                <p class="mt-1 text-xs text-slate-500">控制本机线程数量、请求压力和 TCP 发包次数。</p>
              </div>
              <div class="grid gap-4 md:grid-cols-2">
                <label>
                  <span class="ui-label">TCP 并发线程</span>
                  <input v-model.number="settings.probeConcurrencyStage1" min="1" max="1000" type="number" class="ui-field" />
                </label>
                <label>
                  <span class="ui-label">追踪并发线程</span>
                  <input v-model.number="settings.probeConcurrencyStage2" min="1" max="30" type="number" class="ui-field" />
                </label>
                <label>
                  <span class="ui-label">单 IP GET 分片并发</span>
                  <input v-model.number="settings.probeDownloadGetConcurrency" :disabled="settings.probeStrategy === 'fast'" min="1" max="32" type="number" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
                </label>
                <label>
                  <span class="ui-label">TCP 发包次数</span>
                  <input v-model.number="settings.probePingTimes" min="2" type="number" class="ui-field" />
                </label>
              </div>
            </section>

            <details class="rounded-xl border border-slate-200 bg-slate-50/70">
              <summary class="flex cursor-pointer items-center justify-between gap-3 px-4 py-3 text-sm font-semibold text-slate-800 lg:px-3 lg:py-2.5">
                <span class="min-w-0">高级网络与超时控制</span>
                <span class="shrink-0 text-xs font-medium text-slate-500">高级设置</span>
              </summary>
              <div class="grid gap-4 border-t border-slate-200 p-4 md:grid-cols-2 lg:p-3">
                <label>
                  <span class="ui-label">单 IP 下载测速时间（秒）</span>
                  <input v-model.number="settings.probeDownloadTimeSeconds" :disabled="settings.probeStrategy === 'fast'" min="1" type="number" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
                  <p class="mt-2 text-xs text-slate-500">默认 10 秒；不设置最大值。</p>
                </label>
                <label>
                  <span class="ui-label">下载预热时间（秒）</span>
                  <input v-model.number="settings.probeDownloadWarmupSeconds" :disabled="settings.probeStrategy === 'fast'" min="0" type="number" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
                  <p class="mt-2 text-xs text-slate-500">默认 5 秒；0 表示不排除预热窗口。</p>
                </label>
                <label>
                  <span class="ui-label">下载测速采样间隔（毫秒）</span>
                  <input v-model.number="settings.probeDownloadSpeedSampleIntervalMs" :disabled="settings.probeStrategy === 'fast'" min="1" step="100" type="number" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
                </label>
                <label>
                  <span class="ui-label">下载缓冲（KiB）</span>
                  <input v-model.number="settings.probeDownloadBufferKB" :disabled="settings.probeStrategy === 'fast'" min="64" max="4096" step="64" type="number" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
                </label>
                <label>
                  <span class="ui-label">下载 HTTP 协议</span>
                  <select v-model="settings.probeDownloadHTTPProtocol" :disabled="settings.probeStrategy === 'fast'" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400">
                    <option value="auto">Auto</option>
                    <option value="h1">H1.1</option>
                    <option value="h2">H2</option>
                    <option value="h3">H3</option>
                  </select>
                </label>
                <label>
                  <span class="ui-label">阶段1 TCP 超时 (ms)</span>
                  <input v-model.number="settings.probeTimeoutStage1Ms" min="1" type="number" class="ui-field" />
                </label>
                <label>
                  <span class="ui-label">追踪超时 (ms)</span>
                  <input v-model.number="settings.probeTimeoutStage2Ms" min="1" type="number" class="ui-field" />
                </label>
              </div>
            </details>

            <div class="rounded-xl border border-slate-200 bg-slate-50/70 p-4 text-sm text-slate-500 lg:p-3">
              <p>追踪并发线程后端上限为 30；文件测速固定串行执行。极速模式会跳过文件测速时间和最低速度。</p>
              <p class="mt-1">TCP 延迟默认 4 次发包并跳过首包，只用后续成功样本计算平均值。</p>
              <p class="mt-1">追踪阶段负责地区码识别，并在结果表展示追踪延迟；CSV 仍保持旧列格式。</p>
            </div>
          </div>
        </details>
      </div>
    </section>

    <section class="settings-domain">
      <div class="settings-domain-header">
        <div>
          <h3 class="settings-domain-title">自动化与导出</h3>
          <p class="settings-domain-copy">定时执行、导出写盘、GitHub 上传和共享筛选策略放在同一区块，降低跨区切换成本。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <span v-if="platform === 'desktop'" class="ui-pill ui-pill-subtle">{{ schedulerSummaryLabel }}</span>
          <span class="ui-pill ui-pill-subtle">{{ overwriteLabel(settings.exportOverwrite) }}</span>
          <span class="ui-pill ui-pill-subtle">{{ settings.githubExportEnabled ? "GitHub 导出已启用" : "GitHub 导出未启用" }}</span>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details v-if="platform === 'desktop'" :open="isSectionOpen('scheduler')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('scheduler', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhGauge class="mr-2 shrink-0 text-cf" size="20" />
              定时任务
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ schedulerSummaryLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('scheduler') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="grid gap-4 border-t border-slate-100 p-4 sm:p-6 md:grid-cols-2 lg:p-5">
            <button type="button" class="md:col-span-2 flex items-center justify-between gap-4 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left" @click="settings.schedulerEnabled = !settings.schedulerEnabled">
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">启用桌面后台定时测速</span>
                <span class="text-xs text-slate-500">
                  {{ isWebUIDesktopShell ? "WebUI 服务进程常驻时生效；关闭浏览器后仍由服务端进程调度。" : "应用进程和托盘常驻时生效；窗口关闭后仍由桌面进程调度。" }}
                </span>
              </span>
              <span class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="settings.schedulerEnabled ? 'bg-primary' : 'bg-slate-300'">
                <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.schedulerEnabled ? 'translate-x-5' : 'translate-x-0'"></span>
              </span>
            </button>

            <label>
              <span class="ui-label">固定间隔（分钟）</span>
              <input v-model.number="settings.schedulerIntervalMinutes" min="0" type="number" class="ui-field" />
              <p class="mt-2 text-xs text-slate-500">0 表示不按固定间隔触发。</p>
            </label>
            <label>
              <span class="ui-label">每日固定时间</span>
              <textarea v-model="settings.schedulerDailyTimes" class="ui-field min-h-24 font-mono" placeholder="09:00&#10;21:30" spellcheck="false"></textarea>
              <p class="mt-2 text-xs text-slate-500">支持 HH:mm 或 HH:mm:ss，每行或逗号分隔。</p>
            </label>

            <label class="flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <input v-model="settings.schedulerAutoDnsPush" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">测速成功后自动推送 Cloudflare DNS</span>
                <span class="text-xs text-slate-500">需要 Cloudflare Token、Zone ID 和记录名完整。</span>
              </span>
            </label>
            <label class="flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <input v-model="settings.schedulerAutoGithubExport" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">DNS 推送后自动导出 GitHub</span>
                <span class="text-xs text-slate-500">失败只记录状态，不回滚测速或 DNS 推送结果。</span>
              </span>
            </label>
            <label class="md:col-span-2 flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <input v-model="settings.schedulerSkipIfActive" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">已有任务运行或暂停时跳过本次定时任务</span>
                <span class="text-xs text-slate-500">避免定时任务与手动测速争用同一套任务状态。</span>
              </span>
            </label>

            <div class="md:col-span-2 grid gap-3 rounded-xl border border-slate-200 bg-white p-4 text-sm text-slate-600 md:grid-cols-4">
              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">下次运行</p>
                <p class="mt-2 break-all font-mono text-xs text-slate-700">{{ schedulerStatus?.next_run_at ? formatTimestampLabel(schedulerStatus.next_run_at) : "保存后计算" }}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">上次任务</p>
                <p class="mt-2 break-all font-mono text-xs text-slate-700">{{ schedulerStatus?.last_task_id || "-" }}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">测速 / DNS / GitHub</p>
                <p class="mt-2 text-xs text-slate-700">
                  {{ statusText(schedulerStatus?.last_probe_status || "") }} / {{ statusText(schedulerStatus?.last_dns_status || "") }} /
                  {{ statusText(schedulerStatus?.last_github_status || "") }}
                </p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">上传筛选</p>
                <p class="mt-2 text-xs text-slate-700">原始 {{ schedulerStatus?.upload_input_count ?? 0 }} / 筛后 {{ schedulerStatus?.upload_filtered_count ?? 0 }}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">最近消息</p>
                <p class="mt-2 text-xs text-slate-700">{{ schedulerStatus?.last_message || "尚无定时任务记录" }}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">工作流阶段</p>
                <p class="mt-2 text-xs text-slate-700">{{ workflowLabel(schedulerStatus?.workflow_stage || "") }}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">配置来源</p>
                <p class="mt-2 text-xs text-slate-700">{{ workflowLabel(schedulerStatus?.config_source || "draft_preferred") }}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">配置档案动作</p>
                <p class="mt-2 text-xs text-slate-700">{{ workflowLabel(schedulerStatus?.last_profile_action || "") }}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">输入源档案动作</p>
                <p class="mt-2 text-xs text-slate-700">{{ workflowLabel(schedulerStatus?.last_source_profile_action || "") }}</p>
              </div>
            </div>
          </div>
        </details>

        <details :open="isSectionOpen('export')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('export', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhDownload class="mr-2 shrink-0 text-slate-500" size="20" />
              导出设置
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ overwriteLabel(settings.exportOverwrite) }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('export') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="grid gap-4 border-t border-slate-100 p-4 sm:p-6 md:grid-cols-2 lg:p-5">
            <label class="md:col-span-2">
              <span class="ui-label">导出目录</span>
              <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:gap-3">
                <input v-model="settings.exportTargetDir" type="text" class="ui-field" />
                <button type="button" class="ui-button ui-button-ghost px-4" @click="$emit('select-export-target')">
                  <PhFolderOpen size="18" />
                  选择目录
                </button>
              </div>
              <p v-if="settings.exportTargetUri" class="mt-2 break-all text-xs text-slate-500">Android 导出 URI：{{ settings.exportTargetUri }}</p>
            </label>
            <label>
              <span class="ui-label">文件名</span>
              <input v-model="settings.exportFileName" type="text" class="ui-field" />
            </label>
            <label>
              <span class="ui-label">文件名模板</span>
              <input v-model="settings.exportFileNameTemplate" placeholder="result-{date}-{profile}.csv" type="text" class="ui-field font-mono" />
              <p class="mt-2 text-xs text-slate-500">支持 {date}、{time}、{task_id}、{profile}；填写后优先于固定文件名。</p>
            </label>
            <label class="md:col-span-2">
              <span class="ui-label">覆盖策略</span>
              <select v-model="settings.exportOverwrite" class="ui-field">
                <option value="replace_on_start">启动时覆盖</option>
                <option value="append">追加写入</option>
              </select>
              <p class="mt-2 text-xs text-slate-500">追加写入会复用已有 CSV 表头，空文件会自动补表头。</p>
            </label>
            <label class="md:col-span-2">
              <span class="ui-label">CSV 编码</span>
              <select v-model="settings.exportCSVEncoding" class="ui-field">
                <option value="utf-8">UTF-8</option>
                <option value="utf-8-bom">UTF-8 with BOM</option>
              </select>
              <p class="mt-2 text-xs text-slate-500">BOM 只会写入新文件或空文件，追加到已有 CSV 时不会重复写入。</p>
            </label>
            <div class="md:col-span-2 rounded-xl border border-slate-200 bg-slate-50/70 p-4 lg:p-3">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="min-w-0">
                  <p class="text-sm font-semibold text-slate-800">GitHub 结果导出</p>
                  <p class="mt-1 text-xs text-slate-500">只写入测速结果文件（支持 CSV / TXT），不提交配置包，避免泄露 Cloudflare Token 或 WebDAV 凭据。</p>
                </div>
                <button type="button" class="flex items-center gap-3 rounded-full border border-slate-200 bg-white px-3 py-2" @click="settings.githubExportEnabled = !settings.githubExportEnabled">
                  <span class="text-sm font-medium text-slate-600">{{ settings.githubExportEnabled ? "已启用" : "未启用" }}</span>
                  <span class="relative inline-flex h-6 w-11 items-center rounded-full transition" :class="settings.githubExportEnabled ? 'bg-primary' : 'bg-slate-300'">
                    <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.githubExportEnabled ? 'translate-x-5' : 'translate-x-0'"></span>
                  </span>
                </button>
              </div>

              <div class="mt-4 grid gap-4 md:grid-cols-2">
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
              </div>

              <div class="mt-4 grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
                <p class="break-all text-xs text-slate-500">最近导出：{{ formatTimestampText(settings.githubLastExportAt, "尚未导出") }}。推荐使用 fine-grained PAT，仅授予目标仓库 Contents Read and write。</p>
                <button type="button" class="ui-button ui-button-secondary" :disabled="loading || githubTesting" @click="$emit('test-github-export')">
                  <PhArrowsClockwise size="18" />
                  {{ githubTesting ? "测试中" : "测试 GitHub" }}
                </button>
              </div>
            </div>

            <div class="md:col-span-2 rounded-xl border border-slate-200 bg-white p-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="min-w-0">
                  <p class="text-sm font-semibold text-slate-800">上传策略</p>
                  <p class="mt-1 text-xs text-slate-500">统一控制定时任务、手动 GitHub 导出和“从当前结果推送 DNS”的筛选与 Top N。</p>
                </div>
                <span class="ui-pill ui-pill-subtle">{{ settings.uploadSharedFilterEnabled ? "共享筛选已启用" : "共享筛选未启用" }}</span>
              </div>

              <div class="mt-4 grid gap-4 md:grid-cols-2">
                <label>
                  <span class="ui-label">Cloudflare Top N</span>
                  <input v-model.number="settings.uploadCloudflareTopN" min="0" type="number" class="ui-field" />
                  <p class="mt-2 text-xs text-slate-500">0 表示不限数量。</p>
                </label>
                <label>
                  <span class="ui-label">GitHub Top N</span>
                  <input v-model.number="settings.uploadGitHubTopN" min="0" type="number" class="ui-field" />
                  <p class="mt-2 text-xs text-slate-500">0 表示不限数量。</p>
                </label>
              </div>

              <label class="mt-4 flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
                <input v-model="settings.uploadSharedFilterEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                <span class="min-w-0">
                  <span class="block text-sm font-medium text-slate-700">启用共享上传筛选</span>
                  <span class="text-xs text-slate-500">关闭后保留填写值，但上传时不会生效。</span>
                </span>
              </label>

              <div class="mt-4 grid gap-4 md:grid-cols-2">
                <label>
                  <span class="ui-label">状态</span>
                  <select v-model="settings.uploadSharedFilterStatus" class="ui-field">
                    <option value="passed">仅通过结果</option>
                    <option value="all">全部结果</option>
                  </select>
                </label>
                <label>
                  <span class="ui-label">IP 版本</span>
                  <select v-model="settings.uploadSharedFilterIPVersion" class="ui-field">
                    <option value="any">全部</option>
                    <option value="ipv4">仅 IPv4</option>
                    <option value="ipv6">仅 IPv6</option>
                  </select>
                </label>
                <label>
                  <span class="ui-label">COLO 白名单</span>
                  <input v-model="settings.uploadSharedFilterColoAllow" placeholder="HKG,NRT,LAX" type="text" class="ui-field font-mono" />
                </label>
                <label>
                  <span class="ui-label">COLO 黑名单</span>
                  <input v-model="settings.uploadSharedFilterColoDeny" placeholder="HKG,NRT,LAX" type="text" class="ui-field font-mono" />
                </label>
                <label>
                  <span class="ui-label">最大 TCP 延迟 (ms)</span>
                  <input v-model.number="settings.uploadSharedFilterMaxTcpLatencyMs" min="0" type="number" class="ui-field" />
                </label>
                <label>
                  <span class="ui-label">最大追踪延迟 (ms)</span>
                  <input v-model.number="settings.uploadSharedFilterMaxTraceLatencyMs" min="0" type="number" class="ui-field" />
                </label>
                <label>
                  <span class="ui-label">最低下载速度 (MB/s)</span>
                  <input v-model.number="settings.uploadSharedFilterMinDownloadMbps" min="0" type="number" class="ui-field" />
                </label>
                <label>
                  <span class="ui-label">最大丢包率</span>
                  <input v-model.number="settings.uploadSharedFilterMaxLossRate" min="0" max="1" step="0.01" type="number" class="ui-field" />
                </label>
              </div>
            </div>
          </div>
        </details>
      </div>
    </section>

    <section class="settings-domain">
      <div class="settings-domain-header">
        <div>
          <h3 class="settings-domain-title">安全与诊断</h3>
          <p class="settings-domain-copy">长任务保护、厂商电池白名单提示、重试冷却和调试日志放在最后，便于按风险级别收尾检查。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <span class="ui-pill ui-pill-subtle">节流 {{ settings.probeEventThrottleMs }}ms</span>
          <span class="ui-pill ui-pill-subtle">{{ platform === "mobile" ? batteryStatusLabel : "桌面常驻" }}</span>
          <span class="ui-pill ui-pill-subtle">{{ settings.probeDebug ? "调试已开启" : "调试已关闭" }}</span>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details :open="isSectionOpen('protection')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('protection', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhShieldCheck class="mr-2 shrink-0 text-emerald-600" size="20" weight="fill" />
              异常保护
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">事件节流 {{ settings.probeEventThrottleMs }}ms</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('protection') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="grid gap-4 border-t border-slate-100 p-4 sm:p-6 md:grid-cols-2 lg:p-5">
            <div v-if="platform === 'mobile' && androidBatteryStatus" class="md:col-span-2 rounded-xl border border-amber-200 bg-amber-50/70 p-4 text-sm text-slate-700">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="font-semibold text-slate-800">
                    厂商省电策略
                    <span class="ml-2 ui-pill ui-pill-subtle">{{ batteryStatusLabel }}</span>
                  </p>
                  <p class="mt-2 text-slate-600">
                    {{ androidBatteryStatus.manufacturer || "Android" }} {{ androidBatteryStatus.model || "" }}
                    <span v-if="androidBatteryStatus.needs_guidance">仍可能回收后台任务，请把 CFST 加入电池优化豁免、自启动和后台白名单。</span>
                    <span v-else>当前已处于系统电池优化豁免状态。</span>
                  </p>
                  <p class="mt-2 text-xs text-slate-500">{{ androidBatteryStatus.settings_hint }}</p>
                </div>
                <div class="flex shrink-0 flex-wrap gap-2">
                  <button type="button" class="ui-button ui-button-primary px-3 py-2 text-xs" :disabled="loading || !androidBatteryStatus.supported" @click="$emit('open-battery-settings', 'request')">申请豁免</button>
                  <button type="button" class="ui-button ui-button-ghost px-3 py-2 text-xs" :disabled="loading" @click="$emit('open-battery-settings', 'settings')">系统电池设置</button>
                  <button type="button" class="ui-button ui-button-ghost px-3 py-2 text-xs" :disabled="loading" @click="$emit('open-battery-settings', 'details')">应用详情</button>
                </div>
              </div>
            </div>

            <label>
              <span class="ui-label">连续失败冷却阈值</span>
              <input v-model.number="settings.probeCooldownFailures" min="0" type="number" class="ui-field" />
            </label>
            <label>
              <span class="ui-label">冷却时长 (ms)</span>
              <input v-model.number="settings.probeCooldownMs" min="0" type="number" class="ui-field" />
            </label>
            <label>
              <span class="ui-label">重试最大次数</span>
              <input v-model.number="settings.probeRetryMaxAttempts" min="0" type="number" class="ui-field" />
            </label>
            <label>
              <span class="ui-label">重试退避 (ms)</span>
              <input v-model.number="settings.probeRetryBackoffMs" min="0" type="number" class="ui-field" />
            </label>
            <label class="md:col-span-2">
              <span class="ui-label">事件节流 (ms)</span>
              <input v-model.number="settings.probeEventThrottleMs" min="1" type="number" class="ui-field" />
            </label>

            <div class="md:col-span-2 rounded-xl border border-slate-200 bg-slate-50/70 p-4 text-sm text-slate-500">
              <p v-if="maskedTokenHint" class="break-all">当前已加载脱敏 Token：{{ maskedTokenHint }}</p>
              <p>冷却与重试策略已接入 TCP、追踪、文件测速阶段；0 表示关闭对应保护。</p>
              <p v-if="platform === 'mobile'" class="mt-1">移动端长任务建议同时关闭厂商省电限制、自启动拦截和后台冻结，否则前台服务也可能被系统延迟回收。</p>
              <p class="mt-1">保存后 Wails 桌面端会把这些参数映射到当前 Go CFST 后端。</p>
              <p class="mt-1">若没有重新输入完整 Token，保存动作会被阻止，避免占位值覆盖本地配置。</p>
            </div>
          </div>
        </details>

        <details :open="isSectionOpen('debug')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('debug', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 transition hover:bg-slate-100/70 sm:px-6 sm:py-4 lg:px-5 lg:py-3">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhShieldCheck class="mr-2 shrink-0 text-amber-600" size="20" weight="fill" />
              请求调试
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ settings.probeDebug ? "调试开启" : "调试关闭" }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('debug') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div class="grid gap-4 border-t border-slate-100 p-4 sm:p-6 md:grid-cols-2 lg:p-5">
            <label class="md:col-span-2">
              <span class="ui-label">User-Agent</span>
              <input v-model="settings.probeUserAgent" type="text" class="ui-field font-mono" />
            </label>
            <label>
              <span class="ui-label">Host Header</span>
              <input v-model="settings.probeHostHeader" placeholder="留空时跟随测速 URL" type="text" class="ui-field font-mono" />
            </label>
            <label>
              <span class="ui-label">TLS SNI</span>
              <input v-model="settings.probeSNI" placeholder="留空时跟随测速 URL" type="text" class="ui-field font-mono" />
            </label>
            <div class="md:col-span-2">
              <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                <span class="ui-label mb-0">通用请求 Headers</span>
                <button type="button" class="ui-button ui-button-ghost px-3 py-1.5 text-xs" @click="settings.probeRequestHeaders = REQUEST_HEADERS_TEMPLATE">填入通用模板</button>
              </div>
              <textarea v-model="settings.probeRequestHeaders" class="ui-field min-h-40 font-mono lg:min-h-32" placeholder="每行一个 Header，例如 Accept: */*" spellcheck="false"></textarea>
              <p class="mt-2 text-xs text-slate-500">仅作用于追踪探测和文件测速；Host、User-Agent、Range、Content-Length、Connection、Transfer-Encoding、Accept-Encoding 会被保留逻辑忽略。</p>
            </div>
            <div class="md:col-span-2">
              <label class="mb-2 flex items-center gap-2 text-sm text-slate-700">
                <input v-model="settings.probeDebugCaptureEnabled" :disabled="!settings.probeDebug" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary disabled:cursor-not-allowed disabled:opacity-50" />
                <span>启用抓包监听地址</span>
              </label>
              <input v-model="settings.probeDebugCaptureAddress" placeholder="127.0.0.1:8080 或仅填写端口 8080" type="text" :disabled="!settings.probeDebug || !settings.probeDebugCaptureEnabled" class="ui-field font-mono disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
            </div>
            <label>
              <span class="ui-label">日志模式</span>
              <select v-model="settings.probeDebugLogMode" :disabled="!settings.probeDebug" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400">
                <option value="structured">结构化 JSONL</option>
                <option value="freeform">自由格式文本</option>
              </select>
            </label>
            <label>
              <span class="ui-label">记录粒度</span>
              <select v-model="settings.probeDebugLogVerbosity" :disabled="!settings.probeDebug" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400">
                <option value="simple">简约记录</option>
                <option value="detailed">详细记录</option>
              </select>
              <span class="mt-1 block text-xs text-slate-400">简约记录保留任务启动、阶段完成、导出和最终状态；详细记录包含阶段启动和中间细节。</span>
            </label>
            <label v-if="settings.probeDebugLogMode === 'freeform'" class="md:col-span-2">
              <span class="ui-label">自由格式模板</span>
              <input v-model="settings.probeDebugLogFormat" placeholder="{ts} [{level}] {event} task={task_id} stage={stage} {message}" type="text" :disabled="!settings.probeDebug" class="ui-field font-mono disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
              <span class="mt-1 block text-xs text-slate-400">支持 {field} 占位符；未知字段输出为空。</span>
            </label>

            <button type="button" class="md:col-span-2 flex items-center justify-between gap-4 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left" @click="settings.probeDebug = !settings.probeDebug">
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">启用调试日志</span>
                <span class="text-xs text-slate-400">开启后 Go 后端会把调试日志写入 `cfip-log.txt`，默认使用结构化 JSONL。</span>
              </span>
              <span class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="settings.probeDebug ? 'bg-primary' : 'bg-slate-300'">
                <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.probeDebug ? 'translate-x-5' : 'translate-x-0'"></span>
              </span>
            </button>

            <div class="md:col-span-2 rounded-xl border border-slate-200 bg-slate-50/70 p-4 text-sm text-slate-500">
              <p>后端默认忽略 TLS 证书校验，便于本地抓包、自签证书和自定义监听调试。</p>
              <p class="mt-1">抓包监听地址只在调试模式下生效；留空时仍按正常目标 IP 和端口直连。</p>
            </div>
          </div>
        </details>
      </div>
    </section>

    <div class="settings-page-actions">
      <button type="button" class="settings-action-button settings-action-primary" :disabled="loading" @click="$emit('refresh')">
        <PhArrowsClockwise size="18" />
        读取配置
      </button>
      <button type="button" class="settings-action-button settings-action-secondary" :disabled="loading || saveBlockedByMaskedToken" @click="$emit('save')">
        <PhFloppyDisk size="18" />
        {{ saveButtonText }}
      </button>
    </div>
  </section>
</template>

<style scoped>
.settings-domain {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.settings-domain-header {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.settings-domain-title {
  font-size: 1.125rem;
  font-weight: 700;
  color: rgb(15 23 42);
}

.settings-domain-copy {
  margin-top: 0.375rem;
  max-width: 56rem;
  font-size: 0.875rem;
  line-height: 1.6;
  color: rgb(100 116 139);
}

.settings-page-actions {
  display: grid;
  gap: 0.875rem;
}

.settings-action-button {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  border-radius: 999px;
  padding: 0.95rem 1.5rem;
  font-size: 0.95rem;
  font-weight: 700;
  letter-spacing: 0;
  transition: all 0.2s ease;
}

.settings-action-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.settings-action-primary {
  border: 1px solid transparent;
  background: #111827;
  color: #ffffff;
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.16);
}

.settings-action-primary:hover:not(:disabled) {
  background: #0f172a;
}

.settings-action-secondary {
  border: 1px solid rgb(226 232 240);
  background: rgb(255 255 255);
  color: #111827;
  box-shadow: 0 10px 22px rgba(15, 23, 42, 0.08);
}

.settings-action-secondary:hover:not(:disabled) {
  background: rgb(248 250 252);
}

.settings-summary {
  list-style: none;
}

.settings-summary > * {
  min-width: 0;
}

.settings-summary .ui-pill {
  max-width: min(44vw, 12rem);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (min-width: 640px) {
  .settings-summary .ui-pill {
    max-width: none;
  }

  .settings-page-actions {
    grid-template-columns: repeat(2, minmax(0, max-content));
    justify-content: end;
  }

  .settings-action-button {
    min-width: 10rem;
    padding-inline: 1.75rem;
  }
}

.settings-summary::-webkit-details-marker {
  display: none;
}

:global(html[data-theme="dark"]) .settings-action-primary {
  background: #e5edf8;
  color: #0f172a;
  box-shadow: 0 14px 34px rgba(2, 6, 23, 0.34);
}

:global(html[data-theme="dark"]) .settings-action-primary:hover:not(:disabled) {
  background: #f8fafc;
}

:global(html[data-theme="dark"]) .settings-action-secondary {
  border-color: rgba(148, 163, 184, 0.22);
  background: #142033;
  color: #e5edf8;
  box-shadow: 0 18px 34px rgba(2, 6, 23, 0.3);
}

:global(html[data-theme="dark"]) .settings-action-secondary:hover:not(:disabled) {
  background: #1a2940;
}

@media (min-width: 1024px) {
  .settings-domain-header {
    align-items: flex-end;
    flex-direction: row;
    justify-content: space-between;
  }
}
</style>
