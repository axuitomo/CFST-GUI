<script setup vapor lang="ts">
import { computed, ref, watch } from "vue";
import {
  PhBroadcast,
  PhBuildings,
  PhChatCircleText,
  PhCloud,
  PhArrowSquareOut,
  PhArrowsClockwise,
  PhCaretDown,
  PhDatabase,
  PhDownload,
  PhEnvelopeSimple,
  PhEye,
  PhEyeSlash,
  PhFileArrowUp,
  PhFolderOpen,
  PhGauge,
  PhGlobeHemisphereWest,
  PhLink,
  PhMoon,
  PhQuestion,
  PhShieldCheck,
  PhTelegramLogo,
} from "@phosphor-icons/vue";
import type { ColoDictionaryStatus, TelegramRecipientMode } from "../lib/bridge";
interface CloudflareRoutingRuleForm {
  enabled: boolean;
  filterMode: "allow" | "deny";
  filterTokens: string;
  id: string;
  name: string;
  recordName: string;
  recordType: "A" | "AAAA" | "ALL";
  topN: number;
}

interface SettingsForm {
  apiToken: string;
  comment: string;
  cloudflareEnabled: boolean;
  postProbePushCloudflareEnabled: boolean;
  postProbePushGitHubEnabled: boolean;
  telegramBotToken: string;
  telegramChatId: string;
  telegramIncludeTopN: boolean;
  telegramNotificationEnabled: boolean;
  telegramPersonalChatId: string;
  telegramTopNRecipientMode: TelegramRecipientMode;
  telegramTopN: number;
  telegramUseSystemProxy: boolean;
  webhookEnabled: boolean;
  webhookURLs: string;
  webhookUseSystemProxy: boolean;
  webhookDingTalkURL: string;
  webhookDingTalkSecret: string;
  webhookWeComURLs: string;
  webhookWeComCorpID: string;
  webhookWeComAgentID: string;
  webhookWeComSecret: string;
  webhookWeComMobiles: string;
  webhookHeadersJSON: string;
  emailEnabled: boolean;
  emailHost: string;
  emailPort: number;
  emailUsername: string;
  emailPassword: string;
  emailFrom: string;
  emailTo: string;
  emailUseTLS: boolean;
  telegramUploadRecipientMode: TelegramRecipientMode;
  uploadCloudflareRoutingEnabled: boolean;
  uploadCloudflareRoutingRules: CloudflareRoutingRuleForm[];
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
  probeMCISBudget: number;
  probeMCISConcurrency: number;
  probeCooldownFailures: number;
  probeCooldownMs: number;
  probeDownloadBufferKB: number;
  probeDownloadSuccessLimit: number;
  probeIgnore403: boolean;
  probeIgnore1034: boolean;
  probeDownloadGetConcurrency: number;
  probeDownloadHostHeader: string;
  probeDownloadHTTPProtocol: "auto" | "h1" | "h2" | "h3";
  probeDownloadSNI: string;
  probeDownloadSpeedMetric: DownloadSpeedMetric;
  probeDownloadSpeedSampleIntervalMs: number;
  probeDownloadTimeSeconds: number;
  probeDownloadWarmupSeconds: number;
  probeEventThrottleMs: number;
  probeHostHeader: string;
  probeHttping: boolean;
  probeTraceCfColo: string;
  probeTraceCfColoMode: ColoFilterMode;
  probeTraceStatusCode: number;
  probePingTimes: number;
  probeSourcePoolLimit: number;
  probePrintNum: number;
  probePortPolicy: "source_override_global" | "fixed_global";
  probeRetryBackoffMs: number;
  probeRetryMaxAttempts: number;
  probeRequestHeaders: string;
  probeSNI: string;
  probeVerifyTLSCertificate: boolean;
  probeSourceColoFilterPhase: "precheck" | "stage2";
  probeStageLimitStage2: number;
  probeStageLimitStage3: number;
  probeStrategy: "fast" | "full";
  probeTcpPort: number;
  probeTimeoutStage1Ms: number;
  probeTimeoutStage2Ms: number;
  probeTraceColoMode: "standard" | "trace_url";
  probeTraceURL: string;
  probeDoHURL: string;
  probeECH: boolean;
  probeECHConfig: string;
  probeECHDomain: string;
  probeURL: string;
  probeUserAgent: string;
  proxied: boolean;
  recordName: string;
  schedulerAutoDnsPush: boolean;
  schedulerAutoGithubExport: boolean;
  schedulerDailyTimes: string;
  schedulerEnabled: boolean;
  schedulerIntervalMinutes: number;
  schedulerIntervalMinutesDraft: number;
  schedulerSkipIfActive: boolean;
  schedulerTriggerMode: SchedulerTriggerMode;
  maintenanceCompletedTaskRetentionDays: number;
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
type SchedulerTriggerMode = "interval" | "daily";

interface StorageStatus {
  backend?: "private";
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

interface AndroidKeepAliveStatus {
  enabled: boolean;
  message: string;
  notification_permission_granted: boolean;
  running: boolean;
  supported: boolean;
}

interface AndroidNotificationPermissionStatus {
  can_request: boolean;
  granted: boolean;
  message: string;
  open_settings_recommended: boolean;
  request_already_attempted: boolean;
  should_show_rationale: boolean;
  state: string;
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
  dockerImage: string;
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

type SettingsSectionKey = "updates" | "viewport" | "appearance" | "storage" | "colo" | "backup" | "sources" | "cloudflare" | "probe" | "scheduler" | "export" | "github" | "postPush" | "upload" | "protection" | "debug" | "telegram" | "tls" | "dns" | "webhook";

const props = defineProps<{
  appInfo: AppInfo;
  coloDictionaryProcessing: boolean;
  coloDictionaryStatus: ColoDictionaryStatus | null;
  coloDictionaryUpdating: boolean;
  loading: boolean;
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  githubTesting: boolean;
  telegramTesting: boolean;
  maskedTokenHint: string;
  platform: "desktop" | "mobile";
  dohTesting: boolean;
  dohTestResult: { message: string; ok: boolean } | null;
  androidBatteryStatus?: AndroidBatteryStatus | null;
  androidKeepAliveStatus?: AndroidKeepAliveStatus | null;
  androidNotificationStatus?: AndroidNotificationPermissionStatus | null;
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

defineEmits<{
  (event: "apply-viewport-preset", presetId: string): void;
  (event: "open-battery-settings", mode: "request" | "settings" | "details"): void;
  (event: "set-keep-alive-enabled", enabled: boolean): void;
  (event: "open-notification-settings"): void;
  (event: "request-notification-permission"): void;
  (event: "backup-config-webdav"): void;
  (event: "check-storage-health"): void;
  (event: "check-update"): void;
  (event: "export-config"): void;
  (event: "export-debug-log"): void;
  (event: "export-diagnostic-package"): void;
  (event: "import-config"): void;
  (event: "open-log-directory"): void;
  (event: "open-storage-dir"): void;
  (event: "open-release-page"): void;
  (event: "auto-save"): void;
  (event: "select-export-target"): void;
  (event: "scheduler-daily-times-blur"): void;
  (event: "restore-config-webdav"): void;
  (event: "test-webdav"): void;
  (event: "test-doh"): void;
  (event: "test-github-export"): void;
  (event: "test-telegram-notification"): void;
  (event: "toggle-token"): void;
  (event: "install-update"): void;
  (event: "process-colo-dictionary"): void;
  (event: "refresh-colo-dictionary"): void;
}>();

const collapsedCloudflareRoutingRuleIds = ref(new Set<string>());

function toggleCloudflareRoutingRule(id: string) {
  const collapsed = collapsedCloudflareRoutingRuleIds.value;
  if (collapsed.has(id)) {
    collapsed.delete(id);
  } else {
    collapsed.add(id);
  }
}

function createCloudflareRoutingRule(): CloudflareRoutingRuleForm {
  const index = props.settings.uploadCloudflareRoutingRules.length + 1;
  return {
    enabled: true,
    filterMode: "allow",
    filterTokens: "",
    id: `cf-route-${Date.now()}-${index}`,
    name: `分流规则 ${index}`,
    recordName: "",
    recordType: "A",
    topN: 5,
  };
}

function addCloudflareRoutingRule() {
  props.settings.uploadCloudflareRoutingRules.push(createCloudflareRoutingRule());
}

function removeCloudflareRoutingRule(index: number) {
  props.settings.uploadCloudflareRoutingRules.splice(index, 1);
}

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
    cancelled: "已终止",
    completed: "完成",
    failed: "失败",
    partial: "部分完成",
    running: "执行中",
    skipped: "跳过",
    unsupported: "未接入",
  };
  return value ? labels[value] || value : "未运行";
}

// 展开状态跨导航保存：设置页每次进入都会重新挂载，否则用户刚展开的分区会被重置。
const EXPANDED_SECTIONS_STORAGE_KEY = "cfst.settings.expandedSections.v1";
function loadExpandedSections(): Record<SettingsSectionKey, boolean> {
  const sections: Record<SettingsSectionKey, boolean> = {
    appearance: false,
    backup: false,
    cloudflare: false,
    debug: false,
    dns: false,
    export: false,
    github: false,
    postPush: false,
    probe: false,
    protection: false,
    scheduler: false,
    sources: false,
    colo: false,
    storage: false,
    telegram: false,
    tls: false,
    upload: false,
    updates: false,
    viewport: false,
    webhook: false,
  };
  try {
    const raw = window.localStorage.getItem(EXPANDED_SECTIONS_STORAGE_KEY);
    const parsed = raw ? (JSON.parse(raw) as Record<string, unknown>) : null;
    if (parsed) {
      for (const key of Object.keys(sections) as SettingsSectionKey[]) {
        if (typeof parsed[key] === "boolean") {
          sections[key] = parsed[key];
        }
      }
    }
  } catch {
    // 存储不可用时从全收起开始，不影响设置页使用。
  }
  return sections;
}
const expandedSections = ref<Record<SettingsSectionKey, boolean>>(loadExpandedSections());
watch(
  expandedSections,
  (value) => {
    try {
      window.localStorage.setItem(EXPANDED_SECTIONS_STORAGE_KEY, JSON.stringify(value));
    } catch {
      // Ignore storage failures so the settings page stays usable.
    }
  },
  { deep: true },
);
const pinnedHelpSection = ref<string | null>(null);
function toggleHelpSection(key: string) {
  pinnedHelpSection.value = pinnedHelpSection.value === key ? null : key;
}
const isDockerWebUI = computed(() => props.appInfo.install_mode === "docker_compose");
const isAndroidApp = computed(() => props.appInfo.platform === "android");
const isLinuxArmHost = computed(() => {
  const platform = props.appInfo.platform.trim().toLowerCase();
  return platform.startsWith("linux/") && (platform.includes("arm64") || platform.endsWith("/arm") || platform.includes("/arm/"));
});
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

// 探测策略的派生值与快捷操作：都直接读写 settings，不额外存一份状态。
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
const PROBE_CONCURRENCY_PRESETS = [128, 200, 384] as const;
const PROBE_SPEED_PRESETS = [5, 15, 50] as const;
const probeMinSpeedMbps = computed(() => Math.round(props.settings.minDownloadMbps * 8));
const probeStage3TuningOpen = ref(false);
const probeTraceURLMuted = computed(() => props.settings.probeTraceColoMode !== "trace_url");

// 地区码按标签容器维护，落盘仍是同一个逗号分隔字符串。
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

// MICS 的"自动"由后台按下述规则推导。界面直接显示推导值，不把 0 塞进禁用输入框，
// 以免和全局「0 = 不限制」的语义打架。
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
const DOH_PRESETS = [
  { id: "alidns", label: "阿里 DoH（dns.alidns.com）", url: "https://dns.alidns.com/dns-query" },
  { id: "dohpub", label: "DoH.pub（sm2.doh.pub）", url: "https://sm2.doh.pub/dns-query" },
  { id: "cloudflare", label: "Cloudflare（cloudflare-dns.com）", url: "https://cloudflare-dns.com/dns-query" },
] as const;
// 下拉选择单独存状态，而不是从地址反查：地址默认就是阿里预设，反查会让「自定义」
// 刚选中就弹回预设，条件渲染的输入框也就永远不出现。地址被外部改写（加载或导入配置）
// 时才让下拉跟随，用户自己选的自定义保持不动。
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
  // 选「自定义」时保留当前地址，让用户在原值上改，而不是从空白重打。
  if (preset) props.settings.probeDoHURL = preset.url;
}
// 折叠状态只用一个短标签，完整预设名在展开后的下拉里。
const DOH_PRESET_SHORT_LABELS: Record<string, string> = { alidns: "阿里 DoH", cloudflare: "Cloudflare", dohpub: "DoH.pub" };
const dohPresetLabel = computed(() => DOH_PRESET_SHORT_LABELS[dohPresetId.value] ?? "自定义 DoH");
const downloadProtocolHint = computed(() => {
  if (isAndroidApp.value) {
    return "Android 上 Auto 会回退到 TCP（HTTP/1.1 或 HTTP/2），避免蜂窝网或禁 UDP 网络上的 H3 超时。仍可手动选择 H3。";
  }
  if (isLinuxArmHost.value) {
    return "当前 Linux ARM 主机上 Auto 会回退到 TCP，降低 H3/UDP 异常；Windows、macOS 和 Linux amd64 仍会先尝试 H3。";
  }
  return "Auto 默认先尝试 HTTP/3，失败后回退 TCP 上的 HTTP/1.1/2。Linux ARM 与 Android 上 Auto 会直接使用 TCP。";
});
const isWebUIDesktopShell = computed(() => isDockerWebUI.value);
const schedulerAvailable = computed(() => {
  const runtimePlatform = props.appInfo.platform.trim();
  return runtimePlatform === "" || runtimePlatform === "android" || runtimePlatform.includes("/") || isDockerWebUI.value;
});
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
    return "存储状态异常";
  }
  if (props.storage.writable) {
    return props.storage.portable_mode ? "便携可写" : "可写";
  }
  return "不可写";
});
const storageDisplayPath = computed(() => props.storage?.display_name || props.storage?.current_dir || "尚未读取应用数据目录");
const coloDictionaryUpdatedAt = computed(() => {
  const value = props.coloDictionaryStatus?.last_updated_at || "";
  return value.trim() ? props.formatTimestamp(value) : "尚未更新";
});
const cloudflareConfigComplete = computed(() => Boolean((props.settings.apiToken.trim() || props.maskedTokenHint.trim()) && props.settings.zoneId.trim() && props.settings.recordName.trim()));
const githubConfigComplete = computed(() => {
  const token = props.settings.githubToken.trim();
  return Boolean(props.settings.githubOwner.trim() && props.settings.githubRepo.trim() && props.settings.githubBranch.trim() && props.settings.githubPathTemplate.trim() && token && !isMaskedSecretValue(token));
});
const cloudflareConfigLabel = computed(() => (cloudflareConfigComplete.value ? "Cloudflare 配置完整" : "Cloudflare 配置未完整"));
const githubConfigLabel = computed(() => (githubConfigComplete.value ? "GitHub 配置完整" : "GitHub 配置未完整"));
const telegramChannelStatusLabel = computed(() => (props.settings.telegramNotificationEnabled ? "已启用" : "未启用"));
function telegramRecipientModeText(mode: TelegramRecipientMode) {
  const labels: Record<TelegramRecipientMode, string> = {
    both: "个人+群组/频道",
    chat: "群组/频道",
    personal: "仅个人",
  };
  return labels[mode] || labels.chat;
}

const telegramUploadRecipientModeLabel = computed(() => `上传目标 ${telegramRecipientModeText(props.settings.telegramUploadRecipientMode)}`);
const telegramTopNLabel = computed(() => (props.settings.telegramIncludeTopN ? `Top ${props.settings.telegramTopN || 5} ${telegramRecipientModeText(props.settings.telegramTopNRecipientMode)}` : "未推送 Top N 列表"));
const exportTargetDisplay = computed(() => {
  if (props.settings.exportTargetUri.trim()) {
    return `Android SAF 导出目录：${props.settings.exportTargetUri.trim()}`;
  }
  if (props.settings.exportTargetDir.trim()) {
    return `当前导出目录：${props.settings.exportTargetDir.trim()}`;
  }
  return isAndroidApp.value ? "尚未选择 Android SAF 导出目录" : "当前导出目录：应用默认导出目录";
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
const schedulerTriggerModeLabel = computed(() => (props.settings.schedulerTriggerMode === "daily" ? "固定时间" : "固定间隔"));
const schedulerSummaryLabel = computed(() => {
  if (!schedulerAvailable.value) {
    return "移动端隐藏";
  }
  if (!props.settings.schedulerEnabled) {
    return "单任务模式未启用";
  }
  return props.schedulerStatus?.next_run_at ? `单任务模式·${schedulerTriggerModeLabel.value}已计划` : `单任务模式·${schedulerTriggerModeLabel.value}待保存`;
});

const schedulerTriggerModeModel = computed({
  get: () => props.settings.schedulerTriggerMode,
  set: (mode: SchedulerTriggerMode) => {
    props.settings.schedulerTriggerMode = mode;
    if (mode === "interval") {
      props.settings.schedulerDailyTimes = "";
      if (!Number.isFinite(props.settings.schedulerIntervalMinutesDraft) || props.settings.schedulerIntervalMinutesDraft <= 0) {
        props.settings.schedulerIntervalMinutesDraft = props.settings.schedulerIntervalMinutes > 0 ? props.settings.schedulerIntervalMinutes : 60;
      }
      if (!Number.isFinite(props.settings.schedulerIntervalMinutes) || props.settings.schedulerIntervalMinutes <= 0) {
        props.settings.schedulerIntervalMinutes = props.settings.schedulerIntervalMinutesDraft;
      }
      return;
    }
    if (Number.isFinite(props.settings.schedulerIntervalMinutes) && props.settings.schedulerIntervalMinutes > 0) {
      props.settings.schedulerIntervalMinutesDraft = props.settings.schedulerIntervalMinutes;
    }
  },
});
const batteryStatusLabel = computed(() => {
  if (!props.androidBatteryStatus?.supported) {
    return "系统未提供";
  }
  return props.androidBatteryStatus.ignoring_optimizations ? "已放行" : "待放行";
});
const keepAliveStatusLabel = computed(() => {
  const status = props.androidKeepAliveStatus;
  if (!status?.supported) {
    return "不支持";
  }
  if (!status.enabled) {
    return "已关闭";
  }
  if (!status.notification_permission_granted) {
    return "待通知权限";
  }
  return status.running ? "运行中" : "启动中";
});
const notificationPermissionLabel = computed(() => {
  if (!props.androidNotificationStatus?.supported) {
    return "无需授权";
  }
  return props.androidNotificationStatus.granted ? "已允许" : "待允许";
});
const notificationPermissionCanRequest = computed(() => {
  return props.androidNotificationStatus?.can_request === true;
});
const notificationPermissionNeedsSettings = computed(() => {
  return props.androidNotificationStatus?.open_settings_recommended === true;
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
    cancelled: "已终止",
    completed: "完成",
    draft: "草稿配置",
    draft_preferred: "草稿优先",
    formal: "正式配置",
    post_run_source_profiles: "更新最近运行输入组",
    saved: "正式配置",
    skipped: "跳过",
    update_recent_run_source_profile: "更新最近运行输入组",
  };
  return value ? labels[value] || value : "-";
}

function formatTimestampText(value: string, fallback = "未记录时间") {
  return value.trim() ? props.formatTimestamp(value) : fallback;
}

function formatTimestampLabel(value: string, options?: TimestampFormatOptions) {
  return props.formatTimestamp(value, options);
}

function isMaskedSecretValue(value: string) {
  return value.includes("...") || value.includes("***") || /^\*+$/.test(value);
}
const ttlOptions = [
  { label: "1分钟", value: 60 },
  { label: "5分钟", value: 300 },
  { label: "10分钟", value: 600 },
];

function isSectionOpen(section: SettingsSectionKey) {
  return expandedSections.value[section];
}

// 折叠的分区不构建表单内容：进入设置页只挂摘要，首次展开才创建，之后保持挂载。
const mountedSections = ref(new Set<SettingsSectionKey>());
function isSectionMounted(section: SettingsSectionKey) {
  return mountedSections.value.has(section) || expandedSections.value[section];
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
  const element = event.currentTarget as HTMLDetailsElement;
  const open = element.open;
  if (open === expandedSections.value[section]) {
    // 挂载时浏览器会把已恢复的 open 属性再回放一次 toggle，那不是用户操作。
    return;
  }
  if (open) mountedSections.value.add(section);
  expandedSections.value[section] = open;
}
</script>

<template>
  <section class="settings-view-root" :class="platform === 'desktop' ? 'space-y-4' : 'space-y-3'" @click="$emit('auto-save')" @focusout="$emit('auto-save')">
    <section class="settings-domain">
      <div class="settings-domain-header">
        <div class="settings-domain-title-wrap" :class="{ pinned: pinnedHelpSection === 'general' }">
          <h3 class="settings-domain-title flex items-center">
            通用设置
            <button type="button" class="settings-help-btn" aria-label="查看说明" @click="toggleHelpSection('general')">
              <PhQuestion size="13" />
            </button>
          </h3>
          <p class="settings-domain-copy">检查在线更新，调整窗口尺寸、显示主题，管理 COLO 词典与时间显示。</p>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details :open="isSectionOpen('updates')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('updates', $event)">
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
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('updates') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('updates')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 lg:grid-cols-[1fr_auto] lg:items-center lg:p-4">
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

        <details :open="isSectionOpen('viewport')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('viewport', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
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
          <div v-if="isSectionMounted('viewport')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
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

        <details :open="isSectionOpen('appearance')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('appearance', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhMoon class="mr-2 shrink-0 text-slate-600" size="20" weight="fill" />
              外观与自动主题
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ themeSummaryLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('appearance') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('appearance')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
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
        <details :open="isSectionOpen('colo')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('colo', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhGlobeHemisphereWest class="mr-2 shrink-0 text-slate-600" size="20" />
              COLO 词典
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ coloDictionaryUpdating ? "拉取中" : coloDictionaryProcessing ? "处理中" : `综合 ${coloDictionaryStatus?.colo_rows || 0} 行` }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('colo') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('colo')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
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

    <section class="settings-domain">
      <div class="settings-domain-header">
        <div class="settings-domain-title-wrap" :class="{ pinned: pinnedHelpSection === 'storage' }">
          <h3 class="settings-domain-title flex items-center">
            数据与存储
            <button type="button" class="settings-help-btn" aria-label="查看说明" @click="toggleHelpSection('storage')">
              <PhQuestion size="13" />
            </button>
          </h3>
          <p class="settings-domain-copy">查看应用数据目录，管理配置导入导出和 WebDAV 手动备份。</p>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details :open="isSectionOpen('storage')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('storage', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhFolderOpen class="mr-2 shrink-0 text-slate-600" size="20" />
              应用数据目录
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ storageHealthLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('storage') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('storage')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
            <div>
              <span class="ui-label">固定目录</span>
              <p class="break-all rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 font-mono text-xs text-slate-600">
                {{ storageDisplayPath }}
              </p>
              <p v-if="storage?.runtime_dir && storage.runtime_dir !== storage.current_dir" class="mt-2 break-all text-xs text-slate-500">运行时目录：{{ storage.runtime_dir }}</p>
              <p v-if="storage?.last_sync_error" class="mt-2 text-xs text-amber-600">最近同步：{{ storage.last_sync_error }}</p>
              <p v-if="storage?.health?.message" class="mt-2 text-xs text-slate-500">{{ storage.health.message }}</p>
            </div>
            <div class="grid gap-2 sm:flex sm:flex-wrap sm:gap-3">
              <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('open-storage-dir')">打开目录</button>
              <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('check-storage-health')">健康检查</button>
            </div>
            <p class="text-xs text-slate-500">存储目录不再支持自定义；导出 CSV 和测速文件请在“结果导出”里设置目录。Android 的调试日志和诊断包会在导出时单独选择 SAF 目录。</p>
          </div>
        </details>

        <details :open="isSectionOpen('backup')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('backup', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhCloud class="mr-2 shrink-0 text-cf" size="20" weight="fill" />
              配置备份与同步
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">WebDAV {{ settings.webdavEnabled ? "已启用" : "未启用" }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('backup') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('backup')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
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
      </div>
    </section>

    <section class="settings-domain">
      <div class="settings-domain-header">
        <div class="settings-domain-title-wrap" :class="{ pinned: pinnedHelpSection === 'network' }">
          <h3 class="settings-domain-title flex items-center">
            网络与任务
            <button type="button" class="settings-help-btn" aria-label="查看说明" @click="toggleHelpSection('network')">
              <PhQuestion size="13" />
            </button>
          </h3>
          <p class="settings-domain-copy">设置 DNS 解析与 ECH、输入源行为、测速参数、Cloudflare DNS 推送与 GitHub 结果导出。</p>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details :open="isSectionOpen('dns')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('dns', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhGlobeHemisphereWest class="mr-2 shrink-0 text-sky-600" size="20" weight="fill" />
              DNS 与 ECH
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ dohPresetLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('dns') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('dns')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
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
        <details :open="isSectionOpen('tls')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('tls', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhShieldCheck class="mr-2 shrink-0 text-emerald-600" size="20" weight="fill" />
              TLS 证书校验
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ settings.probeVerifyTLSCertificate ? "严格校验" : "默认跳过" }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('tls') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('tls')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
            <label class="md:col-span-2 flex items-start gap-3">
              <input v-model="settings.probeVerifyTLSCertificate" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">严格校验 TLS 证书与各阶段实际使用的 SNI 域名匹配</span>
                <span class="text-xs text-slate-500">默认关闭，作用于追踪、HTTPing 和文件测速；远端来源抓取、更新包和 Cloudflare DNS 始终校验证书。</span>
              </span>
            </label>
          </div>
        </details>
        <details :open="isSectionOpen('sources')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('sources', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhDatabase class="mr-2 shrink-0 text-slate-600" size="20" weight="fill" />
              输入源行为
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ settings.sourceAutoDetectName ? "自动识别" : "手动命名" }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('sources') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('sources')" class="border-t border-slate-100 p-3 sm:p-4 lg:p-4">
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

        <details :open="isSectionOpen('probe')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('probe', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhGauge class="mr-2 shrink-0 text-primary" size="20" weight="fill" />
              探测策略
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ strategyLabel(settings.probeStrategy) }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('probe') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('probe')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
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
                <p v-if="isAndroidApp" class="mt-1 text-xs text-amber-600">Android 默认禁止明文 HTTP 测速地址，请使用 https://。</p>
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

                  <label>
                    <span class="ui-label">TCP 丢包率上限</span>
                    <span class="ui-input-unit">
                      <input v-model.number="probeLossPercent" max="100" min="0" step="1" type="number" class="ui-field" />
                      <span class="ui-input-unit-label">%</span>
                    </span>
                  </label>

                  <div class="md:col-span-2 xl:col-span-3">
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
                    <span class="ui-label">阶段1 TCP 超时</span>
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

              <details class="probe-stage border-b border-slate-200 last:border-b-0">
                <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
                  <span class="min-w-0">
                    <span class="block text-sm font-semibold text-slate-800">第二阶段 · 追踪与地区码</span>
                    <span class="mt-0.5 block truncate text-xs text-slate-500"
                      >并发 {{ settings.probeConcurrencyStage2 }} · 状态码 {{ settings.probeTraceStatusCode || "不限" }} · {{ probeRegionCodes.length ? (settings.probeTraceCfColoMode === "allow" ? "白名单 " : "黑名单 ") + probeRegionCodes.length + " 项" : "地区码不限" }}</span
                    >
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
                          <button
                            type="button"
                            class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs"
                            :class="settings.probeTraceColoMode === 'standard' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                            @click="settings.probeTraceColoMode = 'standard'"
                          >
                            标准
                          </button>
                          <button
                            type="button"
                            class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs"
                            :class="settings.probeTraceColoMode === 'trace_url' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                            @click="settings.probeTraceColoMode = 'trace_url'"
                          >
                            追踪URL
                          </button>
                        </div>
                        <p class="mt-2 text-xs text-slate-500">标准按 IP 逐个请求它的 /cdn-cgi/trace；追踪URL 只用下面的地址，适合自建探针。</p>
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
                            <p>只接受该状态码的追踪响应；选「不限制」等价于 0，关闭状态码筛选。</p>
                          </details></span
                        >
                        <div class="flex flex-wrap items-center gap-2">
                          <span class="ui-input-unit w-28">
                            <input v-model.number="settings.probeTraceStatusCode" max="599" min="0" :disabled="settings.probeTraceStatusCode === 0" type="number" class="ui-field" />
                            <span v-if="settings.probeTraceStatusCode === 0" class="ui-input-unit-label">不限制</span>
                          </span>
                          <button v-for="code in [200, 204]" :key="code" type="button" class="probe-chip" :class="settings.probeTraceStatusCode === code ? 'probe-chip-active' : ''" @click="settings.probeTraceStatusCode = code">
                            {{ code }}
                          </button>
                          <button type="button" class="probe-chip" :class="settings.probeTraceStatusCode === 0 ? 'probe-chip-active' : ''" @click="settings.probeTraceStatusCode = 0">不限制</button>
                        </div>
                      </div>
                      <label class="md:col-span-2 xl:col-span-3" :class="probeTraceURLMuted ? 'opacity-60' : ''">
                        <span class="ui-label">追踪 URL（可选）</span>
                        <input v-model="settings.probeTraceURL" placeholder="留空时从文件测速URL派生 /cdn-cgi/trace" type="url" class="ui-field font-mono" />
                        <p class="mt-2 text-xs text-slate-500">{{ probeTraceURLMuted ? "标准模式下仅作为备用端点；选「追踪URL」时作为唯一端点。" : "当前是唯一追踪端点；适用于自建探针的 /trace。" }}</p>
                      </label>
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
                          <button
                            type="button"
                            class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs"
                            :class="settings.probeDownloadSpeedMetric === 'average' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                            @click="settings.probeDownloadSpeedMetric = 'average'"
                          >
                            平均速率
                          </button>
                          <button
                            type="button"
                            class="rounded-full px-4 py-2 text-sm font-semibold transition lg:px-3 lg:py-1.5 lg:text-xs"
                            :class="settings.probeDownloadSpeedMetric === 'max' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                            @click="settings.probeDownloadSpeedMetric = 'max'"
                          >
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
                    </div>
                  </section>

                  <section class="space-y-3 border-t border-slate-100 pt-4">
                    <p class="text-xs font-semibold text-slate-600">异常过滤</p>
                    <div class="grid gap-3 md:grid-cols-2">
                      <article class="rounded-xl border border-slate-200 bg-slate-50/70 p-3">
                        <div class="flex items-center justify-between gap-3">
                          <span class="ui-label mb-0">屏蔽 403</span>
                          <input v-model="settings.probeIgnore403" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                        </div>
                        <p class="mt-1 text-xs text-slate-500">开启：直接淘汰，不重试、不计冷却 · 关闭：按普通失败重试并计入冷却</p>
                      </article>
                      <article class="rounded-xl border border-slate-200 bg-slate-50/70 p-3">
                        <div class="flex items-center justify-between gap-3">
                          <span class="ui-label mb-0"
                            >屏蔽 1034
                            <details class="probe-hint">
                              <summary aria-label="屏蔽 1034 说明">?</summary>
                              <p>Cloudflare 边缘 IP 受限（403 error code 1034）。1034 判定优先于 403；两者都不会进结果，日志里以 edge_ip_restricted 标记。</p>
                            </details></span
                          >
                          <input v-model="settings.probeIgnore1034" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                        </div>
                        <p class="mt-1 text-xs text-slate-500">开启：直接淘汰，不重试、不计冷却 · 关闭：按普通失败重试并计入冷却</p>
                      </article>
                    </div>
                  </section>

                  <section class="space-y-3 border-t border-slate-100 pt-4">
                    <button type="button" class="flex w-full items-center justify-between gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-3 py-2 text-left" @click="probeStage3TuningOpen = !probeStage3TuningOpen">
                      <span class="text-xs font-semibold text-slate-600">底层调优</span>
                      <span class="text-xs text-slate-500">{{ probeStage3TuningOpen ? "收起" : "展开 7 项" }}</span>
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
                          <span class="ui-input-unit w-24">
                            <input v-model.number="settings.probeDownloadWarmupSeconds" min="0" type="number" class="ui-field" />
                            <span class="ui-input-unit-label">秒</span>
                          </span>
                          <span class="shrink-0 text-xs text-slate-400">→</span>
                          <span class="text-xs text-slate-500">正式测速</span>
                          <span class="ui-input-unit w-24">
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
                </div>
                <p v-else class="border-t border-slate-100 px-4 py-3 text-xs text-slate-500 sm:px-5 lg:px-4">极速模式不执行文件测速；切到完整模式后可配置核心目标、异常过滤与底层调优。</p>
              </details>
            </div>
          </div>
        </details>

        <details :open="isSectionOpen('cloudflare')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('cloudflare', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhCloud class="mr-2 shrink-0 text-cf" size="20" weight="fill" />
              Cloudflare 配置
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ cloudflareConfigLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('cloudflare') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('cloudflare')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
            <label class="md:col-span-2">
              <span class="ui-label">API Token</span>
              <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:gap-3">
                <input v-model="settings.apiToken" :placeholder="maskedTokenHint || '重新输入完整 Token 以保存'" :type="showToken ? 'text' : 'password'" class="ui-field" />
                <button type="button" class="ui-button ui-button-ghost px-4" @click="$emit('toggle-token')">
                  <component :is="showToken ? PhEyeSlash : PhEye" :size="18" />
                  {{ showToken ? "隐藏" : "显示" }}
                </button>
              </div>
              <p class="mt-2 text-xs text-slate-500">Token、Zone ID 和记录名称完整时自动启用 Cloudflare 功能。</p>
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

            <div class="md:col-span-2 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-sm text-slate-500">Cloudflare DNS 上传固定使用灰色解析，不开启 orange cloud 代理。</div>

            <label class="md:col-span-2">
              <span class="ui-label">Cloudflare Top N</span>
              <input v-model.number="settings.uploadCloudflareTopN" min="0" type="number" class="ui-field" />
              <p class="mt-2 text-xs text-slate-500">0 表示不限数量；共享上传筛选仍会先执行。</p>
            </label>

            <div class="md:col-span-2 space-y-3 border-t border-slate-100 pt-4">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <label class="flex items-start gap-3">
                  <input v-model="settings.uploadCloudflareRoutingEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                  <span class="min-w-0">
                    <span class="block text-sm font-semibold text-slate-800">启用 Cloudflare 分流规则</span>
                    <span class="text-xs text-slate-500">测速结果先经过共享上传筛选，再按国家/COLO 规则分别覆盖推送到指定 DNS 记录名。</span>
                  </span>
                </label>
              </div>
              <p class="text-xs text-slate-500">国家/COLO 筛选词支持 JP,HKG,NRT,US,UK；国家码遵循 ISO 3166-1 alpha-2，UK 兼容为 GB，国家筛选依赖本地 Cloudflare COLO 字典派生。</p>

              <div v-if="settings.uploadCloudflareRoutingRules.length > 0" class="space-y-3">
                <div v-for="(rule, index) in settings.uploadCloudflareRoutingRules" :key="rule.id" class="rounded-xl border border-slate-200 bg-slate-50/70 p-4">
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <label class="flex items-center gap-2 text-sm font-semibold text-slate-700">
                      <input v-model="rule.enabled" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                      启用规则
                    </label>
                    <span v-if="collapsedCloudflareRoutingRuleIds.has(rule.id)" class="min-w-0 truncate text-sm text-slate-600">{{ rule.name || `规则 ${index + 1}` }}</span>
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
                      <button type="button" class="text-sm font-semibold text-rose-600 hover:text-rose-700" @click="removeCloudflareRoutingRule(index)">删除</button>
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
              <div v-else class="rounded-xl border border-dashed border-sky-200 bg-sky-50/70 px-4 py-3 text-sm text-slate-500">尚未添加分流规则。关闭分流或没有启用规则时，将继续使用单域名 Cloudflare Top N 推送。</div>
              <div class="flex justify-end">
                <button type="button" class="ui-button ui-button-secondary" @click="addCloudflareRoutingRule">新增规则</button>
              </div>
            </div>
          </div>
        </details>

        <details :open="isSectionOpen('github')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('github', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhFileArrowUp class="mr-2 shrink-0 text-slate-500" size="20" />
              GitHub 配置
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ githubConfigLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('github') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('github')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
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
              <p class="break-all text-xs text-slate-500">最近导出：{{ formatTimestampText(settings.githubLastExportAt, "尚未导出") }}。推荐使用 fine-grained PAT，仅授予目标仓库 Contents Read and write。</p>
              <button type="button" class="ui-button ui-button-secondary" :disabled="loading || githubTesting" @click="$emit('test-github-export')">
                <PhArrowsClockwise size="18" />
                {{ githubTesting ? "测试中" : "测试 GitHub" }}
              </button>
            </div>
          </div>
        </details>
      </div>
    </section>

    <section class="settings-domain">
      <div class="settings-domain-header">
        <div class="settings-domain-title-wrap" :class="{ pinned: pinnedHelpSection === 'automation' }">
          <h3 class="settings-domain-title flex items-center">
            自动化与导出
            <button type="button" class="settings-help-btn" aria-label="查看说明" @click="toggleHelpSection('automation')">
              <PhQuestion size="13" />
            </button>
          </h3>
          <p class="settings-domain-copy">配置定时任务、结果导出、测速后自动推送与共享上传筛选。</p>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details v-if="schedulerAvailable" :open="isSectionOpen('scheduler')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('scheduler', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhGauge class="mr-2 shrink-0 text-cf" size="20" />
              定时任务
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ schedulerSummaryLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('scheduler') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('scheduler')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
            <button type="button" class="md:col-span-2 flex items-center justify-between gap-4 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left" @click="settings.schedulerEnabled = !settings.schedulerEnabled">
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">启用定时任务</span>
                <span class="text-xs text-slate-500">
                  {{ isAndroidApp ? "Android 使用系统 WorkManager 调度单任务测速；触发时间可能受省电策略影响，建议放行电池优化。" : isWebUIDesktopShell ? "WebUI 服务进程常驻时生效；关闭浏览器后仍由 Docker 服务调度。" : "应用进程和托盘常驻时生效；窗口关闭后仍由桌面进程调度。" }}
                </span>
              </span>
              <span class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="settings.schedulerEnabled ? 'bg-primary' : 'bg-slate-300'">
                <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.schedulerEnabled ? 'translate-x-5' : 'translate-x-0'"></span>
              </span>
            </button>

            <div v-if="isAndroidApp" class="md:col-span-2 rounded-xl border border-sky-100 bg-sky-50/70 px-4 py-3 text-sm text-slate-600">Android 后台定时任务使用系统 WorkManager；测速完成后仍可继续 DNS 推送和 GitHub 导出，触发时间可能被厂商省电策略延后。</div>

            <label class="md:col-span-2">
              <span class="ui-label">触发方式</span>
              <select v-model="schedulerTriggerModeModel" class="ui-field">
                <option value="interval">固定间隔</option>
                <option value="daily">每日固定时间</option>
              </select>
              <p class="mt-2 text-xs text-slate-500">二选一生效，切换时会保留上次有效间隔值；保存时只提交当前触发规则。</p>
            </label>
            <label v-if="settings.schedulerTriggerMode === 'interval'" class="md:col-span-2">
              <span class="ui-label">间隔分钟</span>
              <input v-model.number="settings.schedulerIntervalMinutes" min="1" type="number" class="ui-field" @input="settings.schedulerIntervalMinutesDraft = settings.schedulerIntervalMinutes" />
              <p class="mt-2 text-xs text-slate-500">按固定分钟数循环触发；建议从 5 分钟以上开始，避免和手动测速争用任务状态。</p>
            </label>
            <label v-else class="md:col-span-2">
              <span class="ui-label">每日固定时间</span>
              <textarea v-model="settings.schedulerDailyTimes" class="ui-field min-h-24 font-mono" placeholder="09:00&#10;21:30" spellcheck="false" @blur="$emit('scheduler-daily-times-blur')"></textarea>
              <p class="mt-2 text-xs text-slate-500">支持多个 HH:mm 或 HH:mm:ss 时间点，每行、逗号或顿号分隔。</p>
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
                <p class="text-xs font-semibold text-slate-500">下次运行</p>
                <p class="mt-2 break-all font-mono text-xs text-slate-700">{{ schedulerStatus?.next_run_at ? formatTimestampLabel(schedulerStatus.next_run_at) : "保存后计算" }}</p>
              </div>
              <div>
                <p class="text-xs font-semibold text-slate-500">上次任务</p>
                <p class="mt-2 break-all font-mono text-xs text-slate-700">{{ schedulerStatus?.last_task_id || "-" }}</p>
              </div>
              <div>
                <p class="text-xs font-semibold text-slate-500">执行 / DNS / GitHub</p>
                <p class="mt-2 text-xs text-slate-700">
                  {{ statusText(schedulerStatus?.last_probe_status || "") }} / {{ statusText(schedulerStatus?.last_dns_status || "") }} /
                  {{ statusText(schedulerStatus?.last_github_status || "") }}
                </p>
              </div>
              <div>
                <p class="text-xs font-semibold text-slate-500">上传筛选</p>
                <p class="mt-2 text-xs text-slate-700">原始 {{ schedulerStatus?.upload_input_count ?? 0 }} / 筛后 {{ schedulerStatus?.upload_filtered_count ?? 0 }}</p>
              </div>
              <div>
                <p class="text-xs font-semibold text-slate-500">最近消息</p>
                <p class="mt-2 text-xs text-slate-700">{{ schedulerStatus?.last_message || "尚无定时任务记录" }}</p>
              </div>
              <div>
                <p class="text-xs font-semibold text-slate-500">运行阶段</p>
                <p class="mt-2 text-xs text-slate-700">{{ workflowLabel(schedulerStatus?.workflow_stage || "") }}</p>
              </div>
              <div>
                <p class="text-xs font-semibold text-slate-500">配置来源</p>
                <p class="mt-2 text-xs text-slate-700">{{ workflowLabel(schedulerStatus?.config_source || "draft_preferred") }}</p>
              </div>
              <div v-if="!isAndroidApp">
                <p class="text-xs font-semibold text-slate-500">输入组动作</p>
                <p class="mt-2 text-xs text-slate-700">{{ workflowLabel(schedulerStatus?.last_source_profile_action || "") }}</p>
              </div>
            </div>
            <label class="md:col-span-2">
              <span class="ui-label">任务保留天数</span>
              <input v-model.number="settings.maintenanceCompletedTaskRetentionDays" min="0" type="number" class="ui-field" />
              <p class="mt-2 text-xs text-slate-500">0 表示不自动清理已完成任务；终态任务默认保留 7 天。</p>
            </label>
          </div>
        </details>

        <details :open="isSectionOpen('export')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('export', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhDownload class="mr-2 shrink-0 text-slate-500" size="20" />
              导出设置
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ overwriteLabel(settings.exportOverwrite) }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('export') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('export')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
            <label class="md:col-span-2">
              <span class="ui-label">导出目录</span>
              <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:gap-3">
                <input v-model="settings.exportTargetDir" type="text" class="ui-field" />
                <button type="button" class="ui-button ui-button-ghost px-4" @click="$emit('select-export-target')">
                  <PhFolderOpen size="18" />
                  选择目录
                </button>
                <button
                  type="button"
                  class="ui-button ui-button-ghost px-4"
                  @click="
                    settings.exportTargetDir = '';
                    settings.exportTargetUri = '';
                  "
                >
                  清除
                </button>
              </div>
              <p class="mt-2 break-all text-xs text-slate-500">{{ exportTargetDisplay }}</p>
              <p v-if="isAndroidApp" class="mt-1 text-xs text-slate-500">调试日志和诊断包导出时会单独选择目录，不受此 CSV 导出目录权限影响。</p>
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
          </div>
        </details>

        <details :open="isSectionOpen('postPush')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('postPush', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhArrowSquareOut class="mr-2 shrink-0 text-emerald-600" size="20" weight="fill" />
              测速后自动推送列表
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">按 provider 勾选</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('postPush') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('postPush')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
            <div class="md:col-span-2 text-sm text-slate-500">手动单任务测速完成后按勾选推送；定时任务沿用原 scheduler 逻辑，避免重复。</div>
            <label class="flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <input v-model="settings.postProbePushCloudflareEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">测速完成后推送 Cloudflare</span>
                <span class="text-xs text-slate-500">需要 Cloudflare 配置完整；会复用共享上传策略、Cloudflare Top N 和分流规则。</span>
              </span>
            </label>
            <label class="flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <input v-model="settings.postProbePushGitHubEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">测速完成后推送 GitHub</span>
                <span class="text-xs text-slate-500">需要 GitHub 配置完整；会复用共享上传策略和 GitHub Top N。</span>
              </span>
            </label>
          </div>
        </details>

        <details :open="isSectionOpen('upload')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('upload', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhShieldCheck class="mr-2 shrink-0 text-emerald-600" size="20" weight="fill" />
              共享上传策略
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ settings.uploadSharedFilterEnabled ? "共享筛选已启用" : "共享筛选未启用" }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('upload') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('upload')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
            <p class="md:col-span-2 text-sm text-slate-500">统一控制 Cloudflare、GitHub 和测速后自动推送的结果筛选；各 provider 的 Top N 在对应配置区块里设置。</p>

            <label class="md:col-span-2 flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <input v-model="settings.uploadSharedFilterEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              <span class="min-w-0">
                <span class="block text-sm font-medium text-slate-700">启用共享上传筛选</span>
                <span class="text-xs text-slate-500">关闭后保留填写值，但上传时不会生效。</span>
              </span>
            </label>

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
              <input v-model="settings.uploadSharedFilterColoAllow" placeholder="JP,HKG,NRT,US,UK" type="text" class="ui-field font-mono" />
            </label>
            <label>
              <span class="ui-label">COLO 黑名单</span>
              <input v-model="settings.uploadSharedFilterColoDeny" placeholder="JP,HKG,NRT,US,UK" type="text" class="ui-field font-mono" />
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
        </details>
      </div>
    </section>

    <section class="settings-domain">
      <div class="settings-domain-header">
        <div class="settings-domain-title-wrap" :class="{ pinned: pinnedHelpSection === 'notify' }">
          <h3 class="settings-domain-title flex items-center">
            通知配置
            <button type="button" class="settings-help-btn" aria-label="查看说明" @click="toggleHelpSection('notify')">
              <PhQuestion size="13" />
            </button>
          </h3>
          <p class="settings-domain-copy">Telegram、Webhook 和邮件可报告手动推送、测速后自动上传及定时任务的上传结论；也可提示任务失败。</p>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details :open="isSectionOpen('telegram')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('telegram', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhTelegramLogo class="mr-2 shrink-0 text-primary" size="20" weight="fill" />
              Telegram
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ telegramChannelStatusLabel }}</span>
              <span class="ui-pill ui-pill-subtle">{{ telegramUploadRecipientModeLabel }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('telegram') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('telegram')" class="border-t border-slate-100">
            <div class="grid gap-4 p-3 sm:p-4 lg:grid-cols-2 lg:p-4">
              <div class="space-y-4">
                <label class="flex items-start gap-3">
                  <input v-model="settings.telegramNotificationEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                  <span class="min-w-0">
                    <span class="block text-sm font-medium text-slate-700">Telegram 上传通知</span>
                    <span class="text-xs text-slate-500">默认推送上传结论。</span>
                  </span>
                </label>
                <label class="block min-w-0">
                  <span class="ui-label">网络代理</span>
                  <select v-model="settings.telegramUseSystemProxy" class="ui-field">
                    <option :value="false">不跟随系统代理（默认）</option>
                    <option :value="true">跟随系统代理</option>
                  </select>
                </label>

                <div class="space-y-3 border-t border-slate-100 pt-4">
                  <h4 class="text-sm font-semibold text-slate-700">基础连接</h4>
                  <label class="block min-w-0">
                    <span class="ui-label">Bot Token</span>
                    <input v-model="settings.telegramBotToken" :type="showToken ? 'text' : 'password'" class="ui-field font-mono" autocomplete="off" />
                  </label>
                  <div class="grid gap-3 sm:grid-cols-2">
                    <label class="block min-w-0">
                      <span class="ui-label">群组/频道 Chat ID</span>
                      <input v-model="settings.telegramChatId" class="ui-field font-mono" autocomplete="off" />
                    </label>
                    <label class="block min-w-0">
                      <span class="ui-label">个人 Chat ID</span>
                      <input v-model="settings.telegramPersonalChatId" class="ui-field font-mono" autocomplete="off" />
                    </label>
                  </div>
                </div>
              </div>

              <div class="space-y-4">
                <div class="space-y-3">
                  <h4 class="text-sm font-semibold text-slate-700">上传结论</h4>
                  <label class="block min-w-0">
                    <span class="ui-label">上传目标模式</span>
                    <select v-model="settings.telegramUploadRecipientMode" class="ui-field">
                      <option value="chat">群组/频道</option>
                      <option value="personal">仅个人</option>
                      <option value="both">个人+群组/频道</option>
                    </select>
                  </label>
                </div>

                <div class="space-y-3 border-t border-slate-100 pt-4">
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <h4 class="text-sm font-semibold text-slate-700">Top N 列表</h4>
                    <span v-if="settings.telegramIncludeTopN" class="ui-pill ui-pill-subtle">{{ telegramTopNLabel }}</span>
                  </div>
                  <label class="flex items-start gap-3">
                    <input v-model="settings.telegramIncludeTopN" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                    <span class="min-w-0">
                      <span class="block text-sm font-medium text-slate-700">推送 Top N 列表</span>
                      <span class="text-xs text-slate-500">使用上传筛选后的结果。</span>
                    </span>
                  </label>
                  <div v-if="settings.telegramIncludeTopN" class="grid gap-3 sm:grid-cols-2">
                    <label class="block min-w-0">
                      <span class="ui-label">Top N 数量</span>
                      <input v-model.number="settings.telegramTopN" min="1" max="50" type="number" class="ui-field" />
                    </label>
                    <label class="block min-w-0">
                      <span class="ui-label">Top N 目标模式</span>
                      <select v-model="settings.telegramTopNRecipientMode" class="ui-field">
                        <option value="chat">群组/频道</option>
                        <option value="personal">仅个人</option>
                        <option value="both">个人+群组/频道</option>
                      </select>
                    </label>
                  </div>
                  <p v-else class="text-xs text-slate-500">未推送 Top N 列表</p>
                </div>
              </div>
            </div>

            <div class="flex flex-col gap-3 border-t border-slate-100 px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-5">
              <p class="text-xs text-slate-500">手动推送、测速后自动上传和定时任务自动上传共用此渠道。</p>
              <button type="button" class="ui-button ui-button-secondary w-full sm:w-auto" :disabled="loading || telegramTesting" @click="$emit('test-telegram-notification')">
                <PhArrowsClockwise size="18" />
                {{ telegramTesting ? "测试中" : "测试 Telegram" }}
              </button>
            </div>
          </div>
        </details>
        <details :open="isSectionOpen('webhook')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('webhook', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhBroadcast class="mr-2 shrink-0 text-primary" size="20" weight="fill" />
              其他通知 Webhook
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ settings.webhookEnabled ? "已启用" : "未启用" }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('webhook') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('webhook')" class="border-t border-slate-100">
            <div class="space-y-4 p-3 sm:p-4 lg:p-5">
              <div class="grid gap-4 rounded-xl border border-slate-200 bg-slate-50/60 p-4 sm:grid-cols-[minmax(0,1fr)_minmax(15rem,0.8fr)]">
                <label class="flex items-start gap-3">
                  <input v-model="settings.webhookEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                  <span class="min-w-0"><span class="block text-sm font-medium text-slate-700">启用其他通知 Webhook</span><span class="mt-1 block text-xs text-slate-500">可同时发送到通用地址、钉钉和企业微信。</span></span>
                </label>
                <label class="block min-w-0">
                  <span class="ui-label">网络代理</span>
                  <select v-model="settings.webhookUseSystemProxy" class="ui-field">
                    <option :value="false">不跟随系统代理（默认）</option>
                    <option :value="true">跟随系统代理</option>
                  </select>
                </label>
              </div>

              <div class="rounded-xl border border-slate-200 bg-white p-4">
                <div class="mb-3 flex items-center gap-2">
                  <PhLink class="text-primary" size="18" />
                  <h4 class="text-sm font-semibold text-slate-700">通用 Webhook</h4>
                </div>
                <div class="grid gap-3">
                  <label class="block"><span class="ui-label">Webhook 地址</span><span class="mb-1 block text-xs text-slate-500">每行一个地址，可配置多个接收端。</span><textarea v-model="settings.webhookURLs" rows="3" class="ui-field font-mono" placeholder="https://example.com/webhook"></textarea></label>
                  <label class="block"><span class="ui-label">自定义请求头（JSON）</span><textarea v-model="settings.webhookHeadersJSON" rows="3" class="ui-field font-mono" placeholder='{"Authorization":"Bearer ..."}'></textarea></label>
                </div>
              </div>

              <div class="grid gap-4 lg:grid-cols-2">
                <div class="rounded-xl border border-slate-200 bg-white p-4">
                  <div class="mb-3 flex items-center gap-2">
                    <PhBroadcast class="text-sky-600" size="18" />
                    <h4 class="text-sm font-semibold text-slate-700">钉钉机器人</h4>
                  </div>
                  <div class="space-y-3">
                    <label class="block"><span class="ui-label">机器人地址</span><input v-model="settings.webhookDingTalkURL" class="ui-field font-mono" autocomplete="off" placeholder="https://oapi.dingtalk.com/robot/send?..." /></label>
                    <label class="block"><span class="ui-label">加签密钥（可选）</span><input v-model="settings.webhookDingTalkSecret" type="password" class="ui-field font-mono" autocomplete="off" /></label>
                  </div>
                </div>
                <div class="rounded-xl border border-slate-200 bg-white p-4">
                  <div class="mb-3 flex items-center gap-2">
                    <PhChatCircleText class="text-emerald-600" size="18" />
                    <h4 class="text-sm font-semibold text-slate-700">企业微信机器人</h4>
                  </div>
                  <label class="block"><span class="ui-label">机器人地址</span><span class="mb-1 block text-xs text-slate-500">每行一个地址。</span><textarea v-model="settings.webhookWeComURLs" rows="3" class="ui-field font-mono" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..."></textarea></label>
                </div>
              </div>

              <div class="rounded-xl border border-slate-200 bg-white p-4">
                <div class="mb-3 flex items-center gap-2">
                  <PhBuildings class="text-emerald-600" size="18" />
                  <h4 class="text-sm font-semibold text-slate-700">企业微信应用消息</h4>
                  <span class="text-xs text-slate-500">手机号目标</span>
                </div>
                <div class="grid gap-3 sm:grid-cols-2">
                  <label class="block"><span class="ui-label">企业 ID</span><input v-model="settings.webhookWeComCorpID" class="ui-field" /></label>
                  <label class="block"><span class="ui-label">应用 AgentID</span><input v-model="settings.webhookWeComAgentID" class="ui-field" /></label>
                  <label class="block"><span class="ui-label">应用 Secret</span><input v-model="settings.webhookWeComSecret" type="password" class="ui-field" /></label>
                  <label class="block"><span class="ui-label">手机号</span><input v-model="settings.webhookWeComMobiles" class="ui-field" placeholder="每行一个手机号" /></label>
                </div>
              </div>

              <div class="rounded-xl border border-slate-200 bg-white p-4">
                <div class="mb-3 flex items-center gap-2">
                  <PhEnvelopeSimple class="text-amber-600" size="18" />
                  <h4 class="text-sm font-semibold text-slate-700">邮件通知</h4>
                </div>
                <div class="grid gap-3 sm:grid-cols-2">
                  <label class="flex items-center gap-2 sm:col-span-2"><input v-model="settings.emailEnabled" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />启用邮件通知</label>
                  <label class="block"><span class="ui-label">SMTP 主机</span><input v-model="settings.emailHost" class="ui-field" /></label>
                  <label class="block"><span class="ui-label">端口</span><input v-model.number="settings.emailPort" type="number" class="ui-field" /></label>
                  <label class="block"><span class="ui-label">用户名</span><input v-model="settings.emailUsername" class="ui-field" /></label>
                  <label class="block"><span class="ui-label">密码</span><input v-model="settings.emailPassword" type="password" class="ui-field" /></label>
                  <label class="block"><span class="ui-label">发件人</span><input v-model="settings.emailFrom" class="ui-field" /></label>
                  <label class="block"><span class="ui-label">收件人</span><input v-model="settings.emailTo" class="ui-field" placeholder="每行一个邮箱" /></label>
                </div>
              </div>
            </div>
          </div>
        </details>
      </div>
    </section>

    <section class="settings-domain">
      <div class="settings-domain-header">
        <div class="settings-domain-title-wrap" :class="{ pinned: pinnedHelpSection === 'security' }">
          <h3 class="settings-domain-title flex items-center">
            安全与诊断
            <button type="button" class="settings-help-btn" aria-label="查看说明" @click="toggleHelpSection('security')">
              <PhQuestion size="13" />
            </button>
          </h3>
          <p class="settings-domain-copy">管理异常冷却与重试、Android 通知和后台权限、请求身份及调试日志。</p>
        </div>
      </div>
      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <details :open="isSectionOpen('protection')" class="border-b border-slate-200 last:border-b-0" @toggle="syncSectionOpen('protection', $event)">
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhShieldCheck class="mr-2 shrink-0 text-emerald-600" size="20" weight="fill" />
              异常保护
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">事件节流 {{ settings.probeEventThrottleMs }}ms</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('protection') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('protection')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
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

            <div v-if="platform === 'mobile' && androidKeepAliveStatus" class="md:col-span-2 rounded-xl border border-emerald-200 bg-emerald-50/70 p-4 text-sm text-slate-700">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="font-semibold text-slate-800">
                    通知栏保活
                    <span class="ml-2 ui-pill ui-pill-subtle">{{ keepAliveStatusLabel }}</span>
                  </p>
                  <p class="mt-2 text-slate-600">{{ androidKeepAliveStatus.message }}</p>
                  <p v-if="androidKeepAliveStatus.supported" class="mt-2 text-xs text-slate-500">保活只提升后台存活概率，不会绕过系统强停、厂商省电策略或用户关闭通知。</p>
                  <p v-else class="mt-2 text-xs text-slate-500">后台触发由 WorkManager 管理，真实测速任务仍会按需启动前台服务。</p>
                </div>
                <button type="button" class="ui-button px-3 py-2 text-xs" :class="androidKeepAliveStatus.enabled ? 'ui-button-ghost' : 'ui-button-primary'" :disabled="loading || !androidKeepAliveStatus.supported" @click="$emit('set-keep-alive-enabled', !androidKeepAliveStatus.enabled)">
                  {{ androidKeepAliveStatus.enabled ? "关闭保活" : "开启保活" }}
                </button>
              </div>
            </div>

            <div v-if="platform === 'mobile' && androidNotificationStatus" class="md:col-span-2 rounded-xl border border-sky-200 bg-sky-50/70 p-4 text-sm text-slate-700">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="font-semibold text-slate-800">
                    通知权限
                    <span class="ml-2 ui-pill ui-pill-subtle">{{ notificationPermissionLabel }}</span>
                  </p>
                  <p class="mt-2 text-slate-600">{{ androidNotificationStatus.message }}</p>
                </div>
                <button v-if="notificationPermissionNeedsSettings" type="button" class="ui-button ui-button-primary px-3 py-2 text-xs" :disabled="loading || androidNotificationStatus.granted" @click="$emit('open-notification-settings')">去系统设置</button>
                <button v-else type="button" class="ui-button ui-button-primary px-3 py-2 text-xs" :disabled="loading || !androidNotificationStatus.supported || androidNotificationStatus.granted || !notificationPermissionCanRequest" @click="$emit('request-notification-permission')">允许通知</button>
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
          <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
            <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
              <PhShieldCheck class="mr-2 shrink-0 text-amber-600" size="20" weight="fill" />
              请求身份与调试
            </h3>
            <div class="flex shrink-0 items-center gap-3">
              <span class="ui-pill ui-pill-subtle">{{ settings.probeDebug ? "调试日志开启" : "调试日志关闭" }}</span>
              <PhCaretDown class="text-slate-400 transition" :class="isSectionOpen('debug') ? 'rotate-180' : ''" size="18" />
            </div>
          </summary>
          <div v-if="isSectionMounted('debug')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
            <label class="md:col-span-2">
              <span class="ui-label">User-Agent</span>
              <input v-model="settings.probeUserAgent" type="text" class="ui-field font-mono" />
            </label>
            <p class="md:col-span-2 text-xs text-slate-500">四个字段留空时，Host 与 SNI 自动跟随 URL，与只填 URL 完全一致；任一字段填了值即覆盖对应请求。</p>
            <label>
              <span class="ui-label">追踪 Host Header</span>
              <input v-model="settings.probeHostHeader" placeholder="留空时跟随追踪 URL" type="text" class="ui-field font-mono" />
            </label>
            <label>
              <span class="ui-label">追踪 TLS SNI</span>
              <input v-model="settings.probeSNI" placeholder="留空时跟随追踪 URL" type="text" class="ui-field font-mono" />
            </label>
            <label>
              <span class="ui-label">下载 Host Header</span>
              <input v-model="settings.probeDownloadHostHeader" placeholder="留空时自动选择" type="text" class="ui-field font-mono" />
            </label>
            <label>
              <span class="ui-label">下载 TLS SNI</span>
              <input v-model="settings.probeDownloadSNI" placeholder="留空时自动选择" type="text" class="ui-field font-mono" />
            </label>
            <p class="md:col-span-2 text-xs text-slate-500">下载与追踪 URL 的主机和端口相同时，空值继承追踪设置；不同时，空值跟随下载 URL。Host 与 SNI 是两个独立字段，要伪装就两个都填同样的值。</p>
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
                <span class="text-xs text-slate-400">开启后 Go 后端会把调试日志写入 `logs/cfip-log.txt`；错误日志始终写入 `logs/error-log.txt`。</span>
              </span>
              <span class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="settings.probeDebug ? 'bg-primary' : 'bg-slate-300'">
                <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.probeDebug ? 'translate-x-5' : 'translate-x-0'"></span>
              </span>
            </button>

            <div class="md:col-span-2 grid gap-2 sm:grid-cols-3">
              <button type="button" class="ui-button ui-button-primary" :disabled="loading" @click="$emit('export-diagnostic-package')">
                <PhDownload size="18" />
                导出诊断包
              </button>
              <button type="button" class="ui-button ui-button-secondary" :disabled="loading" @click="$emit('export-debug-log')">
                <PhDownload size="18" />
                导出调试日志
              </button>
              <button type="button" class="ui-button ui-button-secondary" :disabled="loading" @click="$emit('open-log-directory')">
                <PhFolderOpen size="18" />
                打开日志目录
              </button>
            </div>

            <div class="md:col-span-2 rounded-xl border border-slate-200 bg-slate-50/70 p-4 text-sm text-slate-500">
              <p>抓包监听地址只在调试模式下生效；留空时仍按正常目标 IP 和端口直连。</p>
              <p class="mt-1">TLS 证书校验已移到「网络与任务 → TLS 证书校验」。</p>
            </div>
          </div>
        </details>
      </div>
    </section>
  </section>
</template>

<style scoped>
.settings-domain {
  display: flex;
  flex-direction: column;
  gap: 0.625rem;
}

.settings-domain-header {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.settings-domain-title {
  font-size: 1.125rem;
  font-weight: 700;
  color: rgb(15 23 42);
}

:global(html[data-theme="dark"] .settings-domain-title) {
  color: rgb(255 255 255);
}

/* ---- 暗夜模式适配：设置页硬编码 Tailwind 浅色区块 ---- */
:global(html[data-theme="dark"] .settings-view-root .bg-white) {
  background-color: var(--app-panel-bg);
}
:global(html[data-theme="dark"] .settings-view-root .rounded-full.bg-white) {
  background-color: var(--selected-bg-strong);
}
:global(html[data-theme="dark"] .settings-view-root .absolute.bg-white) {
  background-color: var(--toggle-thumb-bg);
}
:global(html[data-theme="dark"] .settings-view-root .bg-indigo-50) {
  background-color: var(--selected-bg);
}
:global(html[data-theme="dark"] .settings-view-root .bg-slate-50\/60),
:global(html[data-theme="dark"] .settings-view-root .bg-slate-50\/70),
:global(html[data-theme="dark"] .settings-view-root .bg-slate-50) {
  background-color: var(--app-muted-bg);
}
:global(html[data-theme="dark"] .settings-view-root .bg-slate-100) {
  background-color: var(--app-muted-strong-bg);
}
:global(html[data-theme="dark"] .settings-view-root .hover\:bg-slate-100\/70:hover),
:global(html[data-theme="dark"] .settings-view-root .hover\:bg-slate-100:hover) {
  background-color: var(--hover-bg);
}
:global(html[data-theme="dark"] .settings-view-root .disabled\:bg-slate-100:disabled) {
  background-color: var(--disabled-bg);
}
:global(html[data-theme="dark"] .settings-view-root .bg-slate-300) {
  background-color: var(--toggle-off-bg);
}
:global(html[data-theme="dark"] .settings-view-root .bg-amber-50\/70),
:global(html[data-theme="dark"] .settings-view-root .bg-amber-50) {
  background-color: var(--warning-bg);
}
:global(html[data-theme="dark"] .settings-view-root .bg-emerald-50\/70) {
  background-color: var(--success-bg);
}
:global(html[data-theme="dark"] .settings-view-root .bg-sky-50\/70) {
  background-color: var(--info-bg);
}
:global(html[data-theme="dark"] .settings-view-root .border-slate-100) {
  border-color: var(--border-subtle);
}
:global(html[data-theme="dark"] .settings-view-root .border-slate-200) {
  border-color: var(--border-default);
}
:global(html[data-theme="dark"] .settings-view-root .border-slate-300),
:global(html[data-theme="dark"] .settings-view-root .hover\:border-slate-300:hover) {
  border-color: var(--border-strong);
}
:global(html[data-theme="dark"] .settings-view-root .border-amber-200) {
  border-color: var(--warning-border);
}
:global(html[data-theme="dark"] .settings-view-root .border-emerald-200) {
  border-color: var(--success-border);
}
:global(html[data-theme="dark"] .settings-view-root .border-sky-200),
:global(html[data-theme="dark"] .settings-view-root .border-sky-100) {
  border-color: var(--info-border);
}
:global(html[data-theme="dark"] .settings-view-root .text-slate-900),
:global(html[data-theme="dark"] .settings-view-root .text-slate-800) {
  color: oklab(0.9851036 0 0);
}
:global(html[data-theme="dark"] .settings-view-root .text-slate-700) {
  color: oklab(0.9674343 0.0003738 -0.0012721);
}
:global(html[data-theme="dark"] .settings-view-root .text-slate-600) {
  color: oklab(0.8711079 0.0015288 -0.0052327);
}
:global(html[data-theme="dark"] .settings-view-root .text-slate-500) {
  color: oklab(0.7118148 0.0035804 -0.0124317);
}
:global(html[data-theme="dark"] .settings-view-root .text-slate-400) {
  color: oklab(0.6400006 0.0040668 -0.0142063);
}
:global(html[data-theme="dark"] .settings-view-root .hover\:text-slate-700:hover) {
  color: oklab(0.9674343 0.0003738 -0.0012721);
}
:global(html[data-theme="dark"] .settings-view-root .disabled\:text-slate-400:disabled) {
  color: var(--disabled-text);
}
:global(html[data-theme="dark"] .settings-view-root .text-amber-600) {
  color: oklab(0.8368606 0.015963 0.1636449);
}
:global(html[data-theme="dark"] .settings-view-root .text-amber-700) {
  color: oklab(0.7685904 0.0560997 0.1548081);
}
:global(html[data-theme="dark"] .settings-view-root .text-emerald-600) {
  color: oklab(0.7729444 -0.1469572 0.0443044);
}
:global(html[data-theme="dark"] .settings-view-root .text-rose-600) {
  color: oklab(0.6450321 0.2065974 0.0609589);
}
:global(html[data-theme="dark"] .settings-view-root .hover\:text-rose-700:hover) {
  color: oklab(0.7191861 0.164364 0.0392419);
}

.settings-domain-title-wrap {
  position: relative;
  display: inline-block;
}

.settings-help-btn {
  display: inline-flex;
  width: 1.05rem;
  height: 1.05rem;
  align-items: center;
  justify-content: center;
  margin-left: 0.35rem;
  border-radius: 9999px;
  color: var(--text-faint);
  transition:
    color 0.15s ease,
    background-color 0.15s ease;
}

.settings-help-btn:hover,
.settings-help-btn:focus-visible {
  color: var(--text-secondary);
  background: var(--hover-bg);
}

/* 阶段折叠块的箭头跟随原生 open 状态，避免为三个阶段各存一份展开状态。
   summary 的默认三角标记要关掉，否则会和这里画的箭头同时出现。 */
.probe-stage > summary {
  list-style: none;
}

.probe-stage > summary::-webkit-details-marker {
  display: none;
}
.probe-stage > summary .probe-stage-caret {
  transition: transform 0.15s ease;
}

.probe-stage[open] > summary .probe-stage-caret {
  transform: rotate(180deg);
}

/* 输入框内的单位 / 不限制后缀：绝对定位的静态标签，不参与布局过渡。 */
.ui-input-unit {
  position: relative;
  display: block;
}

.ui-input-unit > .ui-field {
  padding-right: 3.5rem;
}

.ui-input-unit-label {
  position: absolute;
  top: 50%;
  right: 0.75rem;
  transform: translateY(-50%);
  font-size: 0.75rem;
  line-height: 1;
  color: var(--text-faint);
  pointer-events: none;
}

/* 取值胶囊：点击填数，没有滑动指示器，低端 WebView 也不掉帧。 */
.probe-chip {
  border: 1px solid var(--border-default);
  border-radius: 9999px;
  background: var(--app-panel-bg);
  padding: 0.25rem 0.6rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-muted);
  cursor: pointer;
}

.probe-chip:hover {
  color: var(--text-secondary);
}

.probe-chip-active {
  background: var(--app-surface, #fff);
  color: var(--text-primary);
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.08);
}

/* 地区码标签容器：静态 DOM，回车添加、点 × 删除，落盘仍是逗号串。 */
.probe-tags {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
  min-height: 2.5rem;
  padding: 0.375rem 0.5rem;
  border: 1px solid var(--input-border);
  border-radius: 0.75rem;
  background: var(--input-bg);
}

.probe-tag {
  display: inline-flex;
  align-items: center;
  gap: 0.15rem;
  border: 1px solid var(--border-default);
  border-radius: 9999px;
  background: var(--hover-bg);
  padding: 0.1rem 0.45rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-secondary);
}

.probe-tag button {
  font-size: 0.85rem;
  line-height: 1;
}

.probe-tag-input {
  flex: 1;
  min-width: 10rem;
  border: 0;
  background: transparent;
  font-size: 0.8125rem;
  color: var(--text-primary);
  outline: none;
}

/* 字段级 ? 提示：原生 details，展开直接占位，没有高度动画。 */
.probe-hint {
  display: inline-block;
  margin-left: 0.35rem;
  vertical-align: middle;
}

.probe-hint[open] {
  display: block;
  margin-top: 0.25rem;
  margin-left: 0;
}

.probe-hint > summary {
  display: inline-flex;
  width: 1rem;
  height: 1rem;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-default);
  border-radius: 9999px;
  font-size: 0.65rem;
  font-weight: 600;
  color: var(--text-faint);
  cursor: pointer;
  list-style: none;
}

.probe-hint > summary::-webkit-details-marker {
  display: none;
}

.probe-hint > p {
  margin-top: 0.35rem;
  font-size: 0.75rem;
  font-weight: 400;
  line-height: 1.5;
  color: var(--text-muted);
}

/* 移动端（Android 移动壳与窄窗口，Tailwind sm 断点以下）恢复分区卡片的可读尺寸：
   v2.0.0 统一收紧间距后移动端只剩基础类生效，标题 14px、摘要行上下内边距 10px，卡片明显偏小。
   桌面端走 lg: 断点，不受这里影响。 */
@media (max-width: 639px) {
  .settings-summary {
    padding-top: 0.75rem;
    padding-bottom: 0.75rem;
  }

  .settings-summary h3 {
    font-size: 1rem;
    line-height: 1.4;
  }

  /* 摘要行变大后正文同步回到 16px 内边距，避免卡片头重脚轻。 */
  .settings-summary + div {
    padding: 1rem;
  }

  .settings-help-btn {
    width: 1.35rem;
    height: 1.35rem;
  }
}

.settings-domain-copy {
  display: none;
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 40;
  width: max(20rem, 26rem);
  max-width: min(92vw, 32rem);
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--border-default);
  border-radius: 0.5rem;
  background: var(--app-elevated-bg);
  box-shadow: var(--shadow-panel);
  font-size: 0.75rem;
  line-height: 1.55;
  color: var(--text-secondary);
}

/* 说明面板的显隐：触屏上只认 .pinned，悬停/键盘聚焦的自动揭示仅限精确指针环境。
   Android WebView 会把 :hover 粘在最后点击的元素上、把 :focus-within 粘在被点的按钮上，
   两者叠加后「再点一次问号」收不起面板；:focus-visible 而不是 :focus-within，保证鼠标
   点击不会把面板钉住，键盘 Tab 仍能看到说明。 */
@media (hover: hover) and (pointer: fine) {
  .settings-domain-title-wrap:hover .settings-domain-copy {
    display: block;
  }

  /* 键盘可达性单独一条规则：不支持 :has() 的旧内核只会丢弃这一条，不会连带废掉悬停揭示。 */
  .settings-domain-title-wrap:has(.settings-help-btn:focus-visible) .settings-domain-copy {
    display: block;
  }
}

.settings-domain-title-wrap.pinned .settings-domain-copy {
  display: block;
}

.settings-summary {
  list-style: none;
}

.settings-summary {
  transition:
    background-color 0.8s cubic-bezier(0.22, 1, 0.36, 1),
    color 0.8s cubic-bezier(0.22, 1, 0.36, 1);
}

.settings-summary > :last-child > :last-child {
  transition: rotate 0.8s cubic-bezier(0.22, 1, 0.36, 1);
}

@media (prefers-reduced-motion: reduce) {
  .settings-summary,
  .settings-summary > :last-child > :last-child {
    transition-duration: 0.01ms;
  }
}

.settings-summary > * {
  min-width: 0;
}

.settings-summary .ui-pill {
  max-width: min(44vw, 12rem);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 0;
  border: none;
  background: transparent;
  font-size: 0.72rem;
  font-weight: 500;
  color: var(--text-muted);
  opacity: 0.8;
}

@media (min-width: 640px) {
  .settings-summary .ui-pill {
    max-width: none;
  }
}

.settings-summary::-webkit-details-marker {
  display: none;
}
/* 分区展开/收起不做过渡：details 的高度瞬时到位，内容不淡入。
   淡入（opacity）会和同一帧的布局变化叠成一次闪烁，比硬跳更刺眼。 */

@media (min-width: 1024px) {
  .settings-domain-header {
    align-items: flex-end;
    flex-direction: row;
    justify-content: space-between;
  }
}
</style>
