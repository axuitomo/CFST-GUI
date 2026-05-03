<script setup lang="ts">
import { computed, ref } from "vue";
import {
  PhCloud,
  PhArrowSquareOut,
  PhArrowsClockwise,
  PhDownload,
  PhEye,
  PhEyeSlash,
  PhFileArrowUp,
  PhFloppyDisk,
  PhFolderOpen,
  PhGauge,
  PhShieldCheck,
} from "@phosphor-icons/vue";

interface SettingsForm {
  apiToken: string;
  comment: string;
  exportFileName: string;
  exportFileNameTemplate: string;
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
  probeDisableDownload: boolean;
  probeConcurrencyStage1: number;
  probeConcurrencyStage2: number;
  probeConcurrencyStage3: number;
  probeCooldownFailures: number;
  probeCooldownMs: number;
  probeDownloadCount: number;
  probeDownloadSpeedSampleIntervalSeconds: number;
  probeDownloadTimeSeconds: number;
  probeEventThrottleMs: number;
  probeHostHeader: string;
  probeHttping: boolean;
  probeHttpingCfColo: string;
  probeHttpingStatusCode: number;
  probePingTimes: number;
  probePrintNum: number;
  probeRetryBackoffMs: number;
  probeRetryMaxAttempts: number;
  probeSNI: string;
  probeStageLimitStage1: number;
  probeStageLimitStage2: number;
  probeStageLimitStage3: number;
  probeStrategy: "fast" | "full";
  probeTcpPort: number;
  probeTimeoutStage1Ms: number;
  probeTimeoutStage2Ms: number;
  probeTimeoutStage3Ms: number;
  probeTraceURL: string;
  probeURL: string;
  probeUserAgent: string;
  proxied: boolean;
  recordName: string;
  ttl: number;
  zoneId: string;
}

interface StorageStatus {
  current_dir: string;
  default_dir: string;
  display_name?: string;
  health?: {
    free_bytes: number;
    message: string;
    writable: boolean;
  };
  portable_mode: boolean;
  setup_required: boolean;
  storage_uri?: string;
  writable: boolean;
}

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
  installMode: string;
  installing: boolean;
  latestVersion: string;
  message: string;
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

const props = defineProps<{
  appInfo: AppInfo;
  loading: boolean;
  maskedTokenHint: string;
  platform: "desktop" | "mobile";
  profiles: ProfileStore;
  saveBlockedByMaskedToken: boolean;
  settings: SettingsForm;
  showToken: boolean;
  storage: StorageStatus | null;
  updateState: UpdateState;
}>();

const emit = defineEmits<{
  (event: "check-storage-health"): void;
  (event: "check-update"): void;
  (event: "delete-profile", profileId: string): void;
  (event: "export-config"): void;
  (event: "import-config"): void;
  (event: "open-storage-dir"): void;
  (event: "open-release-page"): void;
  (event: "save"): void;
  (event: "save-profile", name: string, profileId?: string, configSnapshot?: Record<string, unknown>, setActive?: boolean): void;
  (event: "select-export-target"): void;
  (event: "select-storage-dir"): void;
  (event: "switch-profile", profileId: string): void;
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

const saveButtonText = computed(() => (props.saveBlockedByMaskedToken ? "需要完整 Token" : "保存配置"));
const profileNameDraft = ref("");
const activeProfile = computed(() => props.profiles.items.find((profile) => profile.id === props.profiles.active_profile_id) || null);
const updateStatusLabel = computed(() => {
  const labels: Record<UpdateState["status"], string> = {
    available: "发现新版",
    checking: "检查中",
    failed: "检查失败",
    idle: "未检查",
    installing: "下载中",
    latest: "已是最新",
    ready: "已触发安装",
  };
  return labels[props.updateState.status] || "未检查";
});
const storageHealthLabel = computed(() => {
  if (!props.storage) {
    return "未读取";
  }
  if (props.storage.writable) {
    return props.storage.portable_mode ? "便携可写" : "可写";
  }
  return "不可写";
});
const storageDisplayPath = computed(() => props.storage?.display_name || props.storage?.current_dir || props.storage?.storage_uri || "尚未读取储存目录");
const strategyDescription = computed(() =>
  props.settings.probeStrategy === "full"
    ? "按 IP池、TCP、追踪、文件测速四阶段执行，所有追踪通过 IP 都会串行进入文件测速。"
    : "按 IP池、TCP、追踪三阶段执行，跳过文件测速。"
);
const ttlOptions = [
  { label: "1分钟", value: 60 },
  { label: "5分钟", value: 300 },
  { label: "10分钟", value: 600 },
];

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
  <section v-if="platform === 'desktop'" class="space-y-6">
    <article class="ui-card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 bg-slate-50/70 px-6 py-4">
        <div>
          <h3 class="flex items-center text-lg font-semibold text-slate-800">
            <PhArrowsClockwise class="mr-2 text-primary" size="20" weight="bold" />
            在线更新
          </h3>
          <p class="mt-1 text-xs text-slate-500">当前版本 {{ appInfo.current_version || "1.0" }} · {{ appInfo.platform || "desktop" }}</p>
        </div>
        <span class="ui-pill ui-pill-subtle">{{ updateStatusLabel }}</span>
      </div>
      <div class="grid gap-4 p-6 lg:grid-cols-[1fr_auto] lg:items-center">
        <div class="min-w-0">
          <p class="text-sm font-medium text-slate-700">
            {{ updateState.message }}
          </p>
          <p v-if="updateState.latestVersion" class="mt-2 text-xs text-slate-500">
            最新版本 {{ updateState.latestVersion }}{{ updateState.assetName ? ` · ${updateState.assetName}` : "" }}
          </p>
          <p v-if="updateState.downloadPath" class="mt-2 break-all font-mono text-xs text-slate-500">{{ updateState.downloadPath }}</p>
        </div>
        <div class="flex flex-wrap gap-3 lg:justify-end">
          <button type="button" class="ui-button ui-button-ghost" :disabled="loading || updateState.status === 'checking'" @click="$emit('check-update')">
            <PhArrowsClockwise size="18" />
            检查更新
          </button>
          <button type="button" class="ui-button ui-button-primary" :disabled="loading || !updateState.updateAvailable || updateState.installing" @click="$emit('install-update')">
            <PhDownload size="18" />
            下载并安装
          </button>
          <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('open-release-page')">
            <PhArrowSquareOut size="18" />
            发行页
          </button>
        </div>
      </div>
    </article>

    <div class="grid gap-6 xl:grid-cols-2">
      <article class="ui-card overflow-hidden">
        <div class="flex items-center justify-between border-b border-slate-200 bg-slate-50/70 px-6 py-4">
          <h3 class="flex items-center text-lg font-semibold text-slate-800">
            <PhFolderOpen class="mr-2 text-slate-600" size="20" />
            储存目录
          </h3>
          <span class="ui-pill ui-pill-subtle">{{ storageHealthLabel }}</span>
        </div>
        <div class="space-y-4 p-6">
          <div>
            <span class="ui-label">当前目录</span>
            <p class="break-all rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 font-mono text-xs text-slate-600">
              {{ storageDisplayPath }}
            </p>
            <p v-if="storage?.storage_uri" class="mt-2 break-all text-xs text-slate-500">Android SAF：{{ storage.storage_uri }}</p>
            <p v-if="storage?.health?.message" class="mt-2 text-xs text-slate-500">{{ storage.health.message }}</p>
          </div>
          <div class="flex flex-wrap gap-3">
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('select-storage-dir')">
              <PhFolderOpen size="18" />
              选择目录
            </button>
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('open-storage-dir')">
              打开目录
            </button>
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('check-storage-health')">
              健康检查
            </button>
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('use-default-storage-dir')">
              重置默认
            </button>
          </div>
          <p class="text-xs text-slate-500">更换目录时会复制现有配置、词典、日志和结果文件；旧目录不会自动删除。</p>
        </div>
      </article>

      <article class="ui-card overflow-hidden">
        <div class="flex items-center justify-between border-b border-slate-200 bg-slate-50/70 px-6 py-4">
          <h3 class="flex items-center text-lg font-semibold text-slate-800">
            <PhFloppyDisk class="mr-2 text-primary" size="20" weight="fill" />
            配置档案
          </h3>
          <span class="ui-pill ui-pill-subtle">{{ activeProfile?.name || "未选择档案" }}</span>
        </div>
        <div class="space-y-4 p-6">
          <label>
            <span class="ui-label">保存为档案</span>
            <div class="flex gap-3">
              <input v-model="profileNameDraft" class="ui-field flex-1" placeholder="例如：家庭宽带 / 服务器 DNS" type="text" />
              <button type="button" class="ui-button ui-button-primary" :disabled="loading" @click="$emit('save-profile', profileNameDraft)">
                保存档案
              </button>
            </div>
          </label>
          <div v-if="profiles.items.length > 0" class="space-y-2">
            <div
              v-for="profile in profiles.items"
              :key="profile.id"
              class="flex items-center justify-between gap-3 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3"
            >
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-slate-700">{{ profile.name }}</p>
                <p class="text-xs text-slate-400">{{ profile.updated_at || "未记录更新时间" }}</p>
              </div>
              <div class="flex shrink-0 gap-2">
                <button type="button" class="ui-button ui-button-ghost px-3 py-2" :disabled="loading || profile.id === profiles.active_profile_id" @click="$emit('switch-profile', profile.id)">
                  切换
                </button>
                <button type="button" class="ui-button ui-button-ghost px-3 py-2" :disabled="loading" @click="renameProfile(profile)">
                  重命名
                </button>
                <button type="button" class="ui-button ui-button-ghost px-3 py-2" :disabled="loading" @click="duplicateProfile(profile)">
                  复制
                </button>
                <button type="button" class="ui-button ui-button-ghost px-3 py-2" :disabled="loading" @click="$emit('delete-profile', profile.id)">
                  删除
                </button>
              </div>
            </div>
          </div>
          <p v-else class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-sm text-slate-500">
            还没有配置档案；保存当前配置后可在不同网络环境之间快速切换。
          </p>
        </div>
      </article>
    </div>

    <div class="grid gap-6 xl:grid-cols-2">
      <article class="ui-card overflow-hidden">
        <div class="flex items-center justify-between border-b border-slate-200 bg-slate-50/70 px-6 py-4">
          <h3 class="flex items-center text-lg font-semibold text-slate-800">
            <PhCloud class="mr-2 text-cf" size="20" weight="fill" />
            Cloudflare 配置
          </h3>
          <span class="ui-pill ui-pill-subtle">自动识别 A / AAAA</span>
        </div>

        <div class="grid gap-4 p-6 md:grid-cols-2">
          <label class="md:col-span-2">
            <span class="ui-label">API Token</span>
            <div class="flex gap-3">
              <input
                v-model="settings.apiToken"
                :placeholder="maskedTokenHint || '重新输入完整 Token 以保存'"
                :type="showToken ? 'text' : 'password'"
                class="ui-field flex-1"
              />
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
          <div class="flex items-center rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-sm text-slate-500">
            DNS 记录类型会按 IP 自动识别：IPv4 写入 A，IPv6 写入 AAAA。
          </div>
          <label class="md:col-span-2">
            <span class="ui-label">备注</span>
            <input v-model="settings.comment" type="text" class="ui-field" />
          </label>

          <div class="md:col-span-2 flex items-center justify-between rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-3">
            <div>
              <p class="text-sm font-medium text-slate-700">开启代理</p>
              <p class="text-xs text-slate-400">对应 Cloudflare 的 orange cloud 开关。</p>
            </div>
            <button
              type="button"
              class="relative inline-flex h-6 w-11 items-center rounded-full transition"
              :class="settings.proxied ? 'bg-primary' : 'bg-slate-300'"
              @click="settings.proxied = !settings.proxied"
            >
              <span
                class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition"
                :class="settings.proxied ? 'translate-x-5' : 'translate-x-0'"
              ></span>
            </button>
          </div>
        </div>
      </article>

      <article class="ui-card overflow-hidden">
        <div class="flex items-center justify-between border-b border-slate-200 bg-slate-50/70 px-6 py-4">
          <h3 class="flex items-center text-lg font-semibold text-slate-800">
            <PhGauge class="mr-2 text-primary" size="20" weight="fill" />
            探测策略
          </h3>
          <span class="ui-pill ui-pill-subtle">{{ strategyLabel(settings.probeStrategy) }}</span>
        </div>

        <div class="grid gap-4 p-6 md:grid-cols-2">
          <div class="md:col-span-2">
            <span class="ui-label">策略预设</span>
            <div class="grid gap-3 md:grid-cols-2">
              <button
                type="button"
                class="rounded-2xl border px-4 py-4 text-left transition"
                :class="
                  settings.probeStrategy === 'fast'
                    ? 'border-primary bg-indigo-50 text-slate-800 shadow-sm'
                    : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300'
                "
                @click="settings.probeStrategy = 'fast'"
              >
                <p class="text-sm font-semibold">极速模式</p>
                <p class="mt-1 text-xs text-slate-500">执行 IP池、TCP测延迟、追踪探测，跳过文件测速。</p>
              </button>
              <button
                type="button"
                class="rounded-2xl border px-4 py-4 text-left transition"
                :class="
                  settings.probeStrategy === 'full'
                    ? 'border-primary bg-indigo-50 text-slate-800 shadow-sm'
                    : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300'
                "
                @click="settings.probeStrategy = 'full'"
              >
                <p class="text-sm font-semibold">完整模式</p>
                <p class="mt-1 text-xs text-slate-500">在追踪通过后追加文件测速，文件测速串行执行。</p>
              </button>
            </div>
            <p class="mt-3 text-sm text-slate-500">{{ strategyDescription }}</p>
          </div>

          <label>
            <span class="ui-label">TCP并发线程</span>
            <input v-model.number="settings.probeConcurrencyStage1" min="1" max="1000" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">追踪并发线程</span>
            <input v-model.number="settings.probeConcurrencyStage2" min="1" max="20" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">TCP 发包次数</span>
            <input v-model.number="settings.probePingTimes" min="2" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">单 IP 下载测速时间（秒）</span>
            <input v-model.number="settings.probeDownloadTimeSeconds" :disabled="settings.probeStrategy === 'fast'" min="10" type="number" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
          </label>
          <label>
            <span class="ui-label">下载速度采样间隔（秒）</span>
            <input v-model.number="settings.probeDownloadSpeedSampleIntervalSeconds" :disabled="settings.probeStrategy === 'fast'" min="1" type="number" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
          </label>
          <label>
            <span class="ui-label">测速端口</span>
            <input v-model.number="settings.probeTcpPort" min="1" max="65535" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">结果显示数量</span>
            <input v-model.number="settings.probePrintNum" min="0" type="number" class="ui-field" />
            <p class="mt-2 text-xs text-slate-500">0 表示不限制；大于 0 会同时限制 UI 结果和 CSV 导出。</p>
          </label>
          <label class="md:col-span-2">
            <span class="ui-label">文件测速URL</span>
            <input v-model="settings.probeURL" type="url" class="ui-field font-mono" />
            <p class="mt-2 text-xs text-slate-500">文件测速阶段只访问该文件 URL；不要填写 /cdn-cgi/trace。</p>
          </label>
          <label class="md:col-span-2">
            <span class="ui-label">追踪 URL（可选）</span>
            <input v-model="settings.probeTraceURL" placeholder="留空时从文件测速URL派生 /cdn-cgi/trace" type="url" class="ui-field font-mono" />
          </label>
          <label>
            <span class="ui-label">TCP 延迟上限（ms）</span>
            <input v-model.number="settings.maxTcpLatencyMs" min="1" placeholder="留空" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">TCP 延迟下限（ms）</span>
            <input v-model.number="settings.minDelayMs" min="0" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">TCP 丢包率上限（最大 15%）</span>
            <input v-model.number="settings.maxLossRate" max="0.15" min="0" step="0.01" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">最低下载速度 (MB/s)</span>
            <input v-model.number="settings.minDownloadMbps" :disabled="settings.probeStrategy === 'fast'" min="0" step="0.1" type="number" class="ui-field disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
          </label>
          <label>
            <span class="ui-label">追踪有效状态码</span>
            <input v-model.number="settings.probeHttpingStatusCode" max="599" min="0" type="number" class="ui-field" />
            <p class="mt-2 text-xs text-slate-500">0 表示默认接受 200 / 301 / 302。</p>
          </label>
          <label>
            <span class="ui-label">最终地区码筛选</span>
            <input v-model="settings.probeHttpingCfColo" placeholder="HKG,NRT,LAX" type="text" class="ui-field font-mono" />
          </label>

          <label>
            <span class="ui-label">阶段1候选上限</span>
            <input v-model.number="settings.probeStageLimitStage1" min="1" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">追踪候选上限</span>
            <input v-model.number="settings.probeStageLimitStage2" min="1" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">阶段1 TCP 超时 (ms)</span>
            <input v-model.number="settings.probeTimeoutStage1Ms" min="1" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">追踪超时 (ms)</span>
            <input v-model.number="settings.probeTimeoutStage2Ms" min="1" type="number" class="ui-field" />
          </label>

          <div class="md:col-span-2 rounded-2xl border border-slate-200 bg-slate-50/70 p-4 text-sm text-slate-500">
            追踪并发线程后端上限为 20；文件测速固定串行执行。极速模式会跳过文件测速时间和最低速度。
          </div>

          <div class="md:col-span-2 rounded-2xl border border-slate-200 bg-slate-50/70 p-4 text-sm text-slate-500">
            <p>TCP 延迟默认 4 次发包并跳过首包，只用后续成功样本计算平均值。</p>
            <p class="mt-1">追踪阶段负责地区码识别，并在结果表展示追踪延迟；CSV 仍保持旧列格式。</p>
          </div>
        </div>
      </article>

      <article class="ui-card overflow-hidden">
        <div class="flex items-center justify-between border-b border-slate-200 bg-slate-50/70 px-6 py-4">
          <h3 class="flex items-center text-lg font-semibold text-slate-800">
            <PhDownload class="mr-2 text-slate-500" size="20" />
            导出设置
          </h3>
          <span class="ui-pill ui-pill-subtle">{{ overwriteLabel(settings.exportOverwrite) }}</span>
        </div>

        <div class="grid gap-4 p-6 md:grid-cols-2">
          <label class="md:col-span-2">
            <span class="ui-label">导出目录</span>
            <div class="flex gap-3">
              <input v-model="settings.exportTargetDir" type="text" class="ui-field flex-1" />
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
          <button type="button" class="ui-button ui-button-ghost md:col-span-2" :disabled="loading" @click="$emit('export-config')">
            <PhFileArrowUp size="18" />
            导出完整配置
          </button>
          <p class="md:col-span-2 text-xs text-amber-600">完整配置导出会包含 Cloudflare API Token，请只保存到可信位置。</p>
        </div>
      </article>

      <article class="ui-card overflow-hidden">
        <div class="flex items-center justify-between border-b border-slate-200 bg-slate-50/70 px-6 py-4">
          <h3 class="flex items-center text-lg font-semibold text-slate-800">
            <PhShieldCheck class="mr-2 text-emerald-600" size="20" weight="fill" />
            异常保护
          </h3>
          <span class="ui-pill ui-pill-subtle">事件节流 {{ settings.probeEventThrottleMs }}ms</span>
        </div>

        <div class="grid gap-4 p-6 md:grid-cols-2">
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

          <div class="md:col-span-2 rounded-2xl border border-slate-200 bg-slate-50/70 p-4 text-sm text-slate-500">
            <p v-if="maskedTokenHint">当前已加载脱敏 Token：{{ maskedTokenHint }}</p>
            <p>冷却与重试策略已接入 TCP、追踪、文件测速阶段；0 表示关闭对应保护。</p>
            <p class="mt-1">保存后 Wails 桌面端会把这些参数映射到当前 Go CFST 后端。</p>
            <p class="mt-1">若没有重新输入完整 Token，保存动作会被阻止，避免占位值覆盖本地配置。</p>
          </div>
        </div>
      </article>

      <article class="ui-card overflow-hidden">
        <div class="flex items-center justify-between border-b border-slate-200 bg-slate-50/70 px-6 py-4">
          <h3 class="flex items-center text-lg font-semibold text-slate-800">
            <PhShieldCheck class="mr-2 text-amber-600" size="20" weight="fill" />
            请求调试
          </h3>
          <span class="ui-pill ui-pill-subtle">{{ settings.probeDebug ? "调试开启" : "调试关闭" }}</span>
        </div>

        <div class="grid gap-4 p-6 md:grid-cols-2">
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
          <label class="md:col-span-2">
            <span class="ui-label">抓包监听地址</span>
            <input
              v-model="settings.probeDebugCaptureAddress"
              placeholder="127.0.0.1:8080 或仅填写端口 8080"
              type="text"
              :disabled="!settings.probeDebug"
              class="ui-field font-mono disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400"
            />
          </label>

          <button
            type="button"
            class="md:col-span-2 flex items-center justify-between rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left"
            @click="settings.probeDebug = !settings.probeDebug"
          >
            <span>
              <span class="block text-sm font-medium text-slate-700">启用调试日志</span>
              <span class="text-xs text-slate-400">开启后 Go 后端会把结构化 JSONL 调试日志写入 `cfip-log.txt`。</span>
            </span>
            <span class="relative inline-flex h-6 w-11 items-center rounded-full transition" :class="settings.probeDebug ? 'bg-primary' : 'bg-slate-300'">
              <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.probeDebug ? 'translate-x-5' : 'translate-x-0'"></span>
            </span>
          </button>

          <div class="md:col-span-2 rounded-2xl border border-slate-200 bg-slate-50/70 p-4 text-sm text-slate-500">
            <p>后端默认忽略 TLS 证书校验，便于本地抓包、自签证书和自定义监听调试。</p>
            <p class="mt-1">抓包监听地址只在调试模式下生效；留空时仍按正常目标 IP 和端口直连。</p>
          </div>
        </div>
      </article>
    </div>

    <div class="flex justify-end">
      <button
        type="button"
        class="ui-button ui-button-ghost mr-3 min-w-36"
        :disabled="loading"
        @click="$emit('import-config')"
      >
        <PhFileArrowUp size="18" />
        导入配置
      </button>
      <button
        type="button"
        class="ui-button ui-button-primary min-w-40"
        :disabled="loading || saveBlockedByMaskedToken"
        @click="$emit('save')"
      >
        <PhFloppyDisk size="18" weight="fill" />
        {{ saveButtonText }}
      </button>
    </div>
  </section>

  <section v-else class="space-y-4">
    <article class="ui-card overflow-hidden">
      <div class="flex items-center justify-between border-b border-slate-100 bg-slate-50 px-4 py-3">
        <div class="flex items-center">
          <PhArrowsClockwise class="mr-2 text-primary" size="18" weight="bold" />
          <h3 class="text-sm font-semibold text-slate-800">在线更新</h3>
        </div>
        <span class="text-xs text-slate-500">{{ updateStatusLabel }}</span>
      </div>
      <div class="space-y-3 p-4">
        <p class="text-sm font-medium text-slate-700">当前版本 {{ appInfo.current_version || "1.0" }}</p>
        <p class="text-xs text-slate-500">{{ updateState.message }}</p>
        <p v-if="updateState.latestVersion" class="text-xs text-slate-500">最新版本 {{ updateState.latestVersion }}</p>
        <div class="grid grid-cols-3 gap-2">
          <button type="button" class="ui-button ui-button-ghost h-11 px-2" :disabled="loading || updateState.status === 'checking'" @click="$emit('check-update')">检查</button>
          <button type="button" class="ui-button ui-button-primary h-11 px-2" :disabled="loading || !updateState.updateAvailable || updateState.installing" @click="$emit('install-update')">安装</button>
          <button type="button" class="ui-button ui-button-ghost h-11 px-2" :disabled="loading" @click="$emit('open-release-page')">发行页</button>
        </div>
      </div>
    </article>

    <article class="ui-card overflow-hidden">
      <div class="flex items-center justify-between border-b border-slate-100 bg-slate-50 px-4 py-3">
        <div class="flex items-center">
          <PhFolderOpen class="mr-2 text-slate-600" size="18" />
          <h3 class="text-sm font-semibold text-slate-800">储存目录</h3>
        </div>
        <span class="text-xs text-slate-500">{{ storageHealthLabel }}</span>
      </div>
      <div class="space-y-3 p-4">
        <p class="break-all rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 font-mono text-xs text-slate-600">{{ storageDisplayPath }}</p>
        <p v-if="storage?.storage_uri" class="break-all text-xs text-slate-500">SAF：{{ storage.storage_uri }}</p>
        <div class="grid grid-cols-2 gap-2">
          <button type="button" class="ui-button ui-button-ghost h-11" :disabled="loading" @click="$emit('select-storage-dir')">选择目录</button>
          <button type="button" class="ui-button ui-button-ghost h-11" :disabled="loading" @click="$emit('check-storage-health')">健康检查</button>
          <button type="button" class="ui-button ui-button-ghost h-11" :disabled="loading" @click="$emit('use-default-storage-dir')">重置默认</button>
          <button type="button" class="ui-button ui-button-ghost h-11" :disabled="loading" @click="$emit('export-config')">导出配置</button>
        </div>
      </div>
    </article>

    <article class="ui-card overflow-hidden">
      <div class="flex items-center border-b border-slate-100 bg-slate-50 px-4 py-3">
        <PhFloppyDisk class="mr-2 text-primary" size="18" weight="fill" />
        <h3 class="text-sm font-semibold text-slate-800">配置档案</h3>
      </div>
      <div class="space-y-3 p-4">
        <div class="flex gap-2">
          <input v-model="profileNameDraft" class="ui-field h-11 flex-1" placeholder="档案名称" type="text" />
          <button type="button" class="ui-button ui-button-primary h-11 px-3" :disabled="loading" @click="$emit('save-profile', profileNameDraft)">保存</button>
        </div>
        <div v-for="profile in profiles.items" :key="profile.id" class="flex items-center justify-between gap-2 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <div class="min-w-0">
            <p class="truncate text-sm font-medium text-slate-700">{{ profile.name }}</p>
            <p class="text-xs text-slate-400">{{ profile.id === profiles.active_profile_id ? "当前档案" : profile.updated_at }}</p>
          </div>
          <div class="flex shrink-0 gap-2">
            <button type="button" class="ui-button ui-button-ghost h-9 px-3" :disabled="loading || profile.id === profiles.active_profile_id" @click="$emit('switch-profile', profile.id)">切换</button>
            <button type="button" class="ui-button ui-button-ghost h-9 px-3" :disabled="loading" @click="renameProfile(profile)">重命名</button>
            <button type="button" class="ui-button ui-button-ghost h-9 px-3" :disabled="loading" @click="duplicateProfile(profile)">复制</button>
            <button type="button" class="ui-button ui-button-ghost h-9 px-3" :disabled="loading" @click="$emit('delete-profile', profile.id)">删除</button>
          </div>
        </div>
      </div>
    </article>

    <article class="ui-card overflow-hidden">
      <div class="flex items-center border-b border-slate-100 bg-slate-50 px-4 py-3">
        <PhCloud class="mr-2 text-cf" size="18" weight="fill" />
        <h3 class="text-sm font-semibold text-slate-800">Cloudflare 配置</h3>
      </div>
      <div class="space-y-4 p-4">
        <div>
          <label class="block text-xs text-slate-500">API Token</label>
          <div class="relative">
            <input
              v-model="settings.apiToken"
              :placeholder="maskedTokenHint || '重新输入完整 Token 以保存'"
              :type="showToken ? 'text' : 'password'"
              class="ui-field h-11 pr-10"
            />
            <button type="button" class="absolute inset-y-0 right-0 flex items-center pr-3 text-slate-400" @click="$emit('toggle-token')">
              <component :is="showToken ? PhEyeSlash : PhEye" :size="20" />
            </button>
          </div>
        </div>
        <div>
          <label class="block text-xs text-slate-500">Zone ID</label>
          <input v-model="settings.zoneId" type="text" class="ui-field h-11 font-mono" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">记录名称</label>
          <input v-model="settings.recordName" type="text" class="ui-field h-11 font-mono" />
        </div>
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-sm text-slate-500">
          自动识别 A / AAAA：IPv4 写入 A，IPv6 写入 AAAA。
        </div>
        <div>
          <label class="block text-xs text-slate-500">TTL</label>
          <select v-model.number="settings.ttl" class="ui-field h-11">
            <option v-for="option in ttlOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
        </div>
        <div class="flex items-center justify-between rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <label class="text-sm font-medium text-slate-700">开启代理</label>
          <button
            type="button"
            class="relative inline-flex h-6 w-11 items-center rounded-full transition"
            :class="settings.proxied ? 'bg-primary' : 'bg-slate-300'"
            @click="settings.proxied = !settings.proxied"
          >
            <span
              class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition"
              :class="settings.proxied ? 'translate-x-5' : 'translate-x-0'"
            ></span>
          </button>
        </div>
        <div>
          <label class="block text-xs text-slate-500">备注</label>
          <input v-model="settings.comment" type="text" class="ui-field h-11" />
        </div>
      </div>
    </article>

    <article class="ui-card overflow-hidden">
      <div class="flex items-center border-b border-slate-100 bg-slate-50 px-4 py-3">
        <PhGauge class="mr-2 text-primary" size="18" weight="fill" />
        <h3 class="text-sm font-semibold text-slate-800">探测策略</h3>
      </div>
      <div class="space-y-4 p-4">
        <div>
          <label class="block text-xs text-slate-500">策略预设</label>
          <div class="mt-2 grid gap-3">
            <button
              type="button"
              class="rounded-xl border px-4 py-3 text-left transition"
              :class="
                settings.probeStrategy === 'fast'
                  ? 'border-primary bg-indigo-50 text-slate-800'
                  : 'border-slate-200 bg-white text-slate-600'
              "
              @click="settings.probeStrategy = 'fast'"
              >
                <p class="text-sm font-semibold">极速模式</p>
                <p class="mt-1 text-xs text-slate-500">执行 IP池、TCP、追踪，跳过文件测速。</p>
              </button>
            <button
              type="button"
              class="rounded-xl border px-4 py-3 text-left transition"
              :class="
                settings.probeStrategy === 'full'
                  ? 'border-primary bg-indigo-50 text-slate-800'
                  : 'border-slate-200 bg-white text-slate-600'
              "
              @click="settings.probeStrategy = 'full'"
              >
                <p class="text-sm font-semibold">完整模式</p>
                <p class="mt-1 text-xs text-slate-500">追踪通过后追加文件测速，文件测速串行执行。</p>
              </button>
          </div>
          <p class="mt-3 text-xs text-slate-500">{{ strategyDescription }}</p>
        </div>

        <div>
          <label class="block text-xs text-slate-500">导出目录</label>
          <div class="mt-1 flex gap-2">
            <input v-model="settings.exportTargetDir" type="text" class="ui-field h-11 flex-1" />
            <button type="button" class="ui-button ui-button-ghost h-11 px-3" @click="$emit('select-export-target')">
              <PhFolderOpen size="18" />
              选择文件
            </button>
          </div>
          <p v-if="settings.exportTargetUri" class="mt-2 break-all text-xs text-slate-500">导出 URI：{{ settings.exportTargetUri }}</p>
        </div>
        <div>
          <label class="block text-xs text-slate-500">文件名</label>
          <input v-model="settings.exportFileName" type="text" class="ui-field h-11" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">文件名模板</label>
          <input v-model="settings.exportFileNameTemplate" placeholder="result-{date}-{profile}.csv" type="text" class="ui-field h-11 font-mono" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">覆盖策略</label>
          <select v-model="settings.exportOverwrite" class="ui-field h-11">
            <option value="replace_on_start">启动时覆盖</option>
            <option value="append">追加写入</option>
          </select>
          <p class="mt-2 text-xs text-slate-500">追加写入会复用已有 CSV 表头。</p>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-xs text-slate-500">TCP并发线程</label>
            <input v-model.number="settings.probeConcurrencyStage1" min="1" max="1000" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">TCP 发包次数</label>
            <input v-model.number="settings.probePingTimes" min="2" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">追踪并发线程</label>
            <input v-model.number="settings.probeConcurrencyStage2" min="1" max="20" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">单 IP 下载时间（秒）</label>
            <input v-model.number="settings.probeDownloadTimeSeconds" :disabled="settings.probeStrategy === 'fast'" min="10" type="number" class="ui-field h-11 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">速度采样间隔（秒）</label>
            <input v-model.number="settings.probeDownloadSpeedSampleIntervalSeconds" :disabled="settings.probeStrategy === 'fast'" min="1" type="number" class="ui-field h-11 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">测速端口</label>
            <input v-model.number="settings.probeTcpPort" min="1" max="65535" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">结果显示数量</label>
            <input v-model.number="settings.probePrintNum" min="0" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">TCP 延迟上限</label>
            <input v-model.number="settings.maxTcpLatencyMs" min="1" placeholder="留空" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">TCP 延迟下限</label>
            <input v-model.number="settings.minDelayMs" min="0" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">最低速度 (MB/s)</label>
            <input v-model.number="settings.minDownloadMbps" :disabled="settings.probeStrategy === 'fast'" min="0" step="0.1" type="number" class="ui-field h-11 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">TCP 丢包率上限（最大 15%）</label>
            <input v-model.number="settings.maxLossRate" max="0.15" min="0" step="0.01" type="number" class="ui-field h-11" />
          </div>
        </div>

        <div>
          <label class="block text-xs text-slate-500">文件测速URL</label>
          <input v-model="settings.probeURL" type="url" class="ui-field h-11 font-mono" />
          <p class="mt-2 text-xs text-slate-500">文件测速阶段只访问该文件 URL；不要填写 /cdn-cgi/trace。</p>
        </div>
        <div>
          <label class="block text-xs text-slate-500">追踪 URL（可选）</label>
          <input v-model="settings.probeTraceURL" placeholder="留空时自动派生 /cdn-cgi/trace" type="url" class="ui-field h-11 font-mono" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">追踪有效状态码</label>
          <input v-model.number="settings.probeHttpingStatusCode" max="599" min="0" type="number" class="ui-field h-11" />
          <p class="mt-2 text-xs text-slate-500">0 表示默认接受 200 / 301 / 302。</p>
        </div>
        <div>
          <label class="block text-xs text-slate-500">最终地区码筛选</label>
          <input v-model="settings.probeHttpingCfColo" placeholder="HKG,NRT,LAX" type="text" class="ui-field h-11 font-mono" />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-xs text-slate-500">阶段1候选上限</label>
            <input v-model.number="settings.probeStageLimitStage1" min="1" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">追踪候选上限</label>
            <input v-model.number="settings.probeStageLimitStage2" min="1" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">TCP 超时 (ms)</label>
            <input v-model.number="settings.probeTimeoutStage1Ms" min="1" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">追踪超时 (ms)</label>
            <input v-model.number="settings.probeTimeoutStage2Ms" min="1" type="number" class="ui-field h-11" />
          </div>
        </div>

        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-xs text-slate-500">
          追踪并发线程上限为 20；文件测速固定串行执行。极速模式会跳过文件测速相关配置。
        </div>
      </div>
    </article>

    <article class="ui-card overflow-hidden">
      <div class="flex items-center border-b border-slate-100 bg-slate-50 px-4 py-3">
        <PhShieldCheck class="mr-2 text-emerald-600" size="18" weight="fill" />
        <h3 class="text-sm font-semibold text-slate-800">异常保护</h3>
      </div>
      <div class="grid grid-cols-2 gap-3 p-4">
        <div>
          <label class="block text-xs text-slate-500">触发冷却的连续失败次数</label>
          <input v-model.number="settings.probeCooldownFailures" min="0" type="number" class="ui-field h-11" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">冷却时长 (ms)</label>
          <input v-model.number="settings.probeCooldownMs" min="0" type="number" class="ui-field h-11" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">重试次数</label>
          <input v-model.number="settings.probeRetryMaxAttempts" min="0" type="number" class="ui-field h-11" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">重试退避 (ms)</label>
          <input v-model.number="settings.probeRetryBackoffMs" min="0" type="number" class="ui-field h-11" />
        </div>
        <div class="col-span-2">
          <label class="block text-xs text-slate-500">事件节流 (ms)</label>
          <input v-model.number="settings.probeEventThrottleMs" min="1" type="number" class="ui-field h-11" />
        </div>
        <p class="col-span-2 text-xs text-slate-500">冷却与重试策略已接入 TCP、追踪、文件测速阶段；0 表示关闭对应保护。</p>
      </div>
    </article>

    <article class="ui-card overflow-hidden">
      <div class="flex items-center border-b border-slate-100 bg-slate-50 px-4 py-3">
        <PhShieldCheck class="mr-2 text-amber-600" size="18" weight="fill" />
        <h3 class="text-sm font-semibold text-slate-800">请求调试</h3>
      </div>
      <div class="space-y-4 p-4">
        <div>
          <label class="block text-xs text-slate-500">User-Agent</label>
          <input v-model="settings.probeUserAgent" type="text" class="ui-field h-11 font-mono" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">Host Header</label>
          <input v-model="settings.probeHostHeader" placeholder="留空时跟随测速 URL" type="text" class="ui-field h-11 font-mono" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">TLS SNI</label>
          <input v-model="settings.probeSNI" placeholder="留空时跟随测速 URL" type="text" class="ui-field h-11 font-mono" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">抓包监听地址</label>
          <input
            v-model="settings.probeDebugCaptureAddress"
            placeholder="127.0.0.1:8080 或仅填写端口 8080"
            type="text"
            :disabled="!settings.probeDebug"
            class="ui-field h-11 font-mono disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400"
          />
        </div>
        <button
          type="button"
          class="flex w-full items-center justify-between rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-left"
          @click="settings.probeDebug = !settings.probeDebug"
        >
          <span>
            <span class="block text-sm font-medium text-slate-700">启用调试日志</span>
            <span class="text-xs text-slate-400">开启后写入 JSONL 格式的 `cfip-log.txt`，抓包监听地址仅在调试时生效。</span>
          </span>
          <span class="relative inline-flex h-6 w-11 items-center rounded-full transition" :class="settings.probeDebug ? 'bg-primary' : 'bg-slate-300'">
            <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.probeDebug ? 'translate-x-5' : 'translate-x-0'"></span>
          </span>
        </button>
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-xs text-slate-500">
          后端默认忽略 TLS 证书校验，适合接本地抓包工具或自定义监听服务。
        </div>
      </div>
    </article>

    <div class="grid grid-cols-2 gap-3">
      <button
        type="button"
        class="ui-button ui-button-ghost h-12"
        :disabled="loading"
        @click="$emit('import-config')"
      >
        <PhFileArrowUp size="18" />
        导入配置
      </button>
      <button
        type="button"
        class="ui-button ui-button-primary h-12"
        :disabled="loading || saveBlockedByMaskedToken"
        @click="$emit('save')"
      >
        <PhFloppyDisk size="18" weight="fill" />
        {{ saveButtonText }}
      </button>
    </div>
  </section>
</template>
