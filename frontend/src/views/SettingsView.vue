<script setup lang="ts">
import { computed } from "vue";
import {
  PhCloud,
  PhDownload,
  PhEye,
  PhEyeSlash,
  PhFloppyDisk,
  PhGauge,
  PhShieldCheck,
} from "@phosphor-icons/vue";

interface SettingsForm {
  apiToken: string;
  comment: string;
  exportFileName: string;
  exportOverwrite: string;
  exportTargetDir: string;
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
  probeURL: string;
  probeUserAgent: string;
  proxied: boolean;
  recordName: string;
  recordType: "A" | "AAAA";
  ttl: number;
  zoneId: string;
}

const props = defineProps<{
  loading: boolean;
  maskedTokenHint: string;
  platform: "desktop" | "mobile";
  saveBlockedByMaskedToken: boolean;
  settings: SettingsForm;
  showToken: boolean;
}>();

defineEmits<{
  (event: "save"): void;
  (event: "toggle-token"): void;
}>();

function strategyLabel(strategy: SettingsForm["probeStrategy"]) {
  return strategy === "full" ? "完整模式" : "极速模式";
}

function overwriteLabel(value: string) {
  return value === "append" ? "追加写入" : "启动时覆盖";
}

const saveButtonText = computed(() => (props.saveBlockedByMaskedToken ? "需要完整 Token" : "保存配置"));
const strategyDescription = computed(() =>
  props.settings.probeStrategy === "full"
    ? "在低延迟筛选后继续执行真实大文件下载测速，更适合高带宽节点和流媒体代理场景。"
    : "仅执行 TCP/HTTP 响应测速，默认跳过下载环节，适合日常快速更新节点。"
);
</script>

<template>
  <section v-if="platform === 'desktop'" class="space-y-6">
    <div class="grid gap-6 xl:grid-cols-2">
      <article class="ui-card overflow-hidden">
        <div class="flex items-center justify-between border-b border-slate-200 bg-slate-50/70 px-6 py-4">
          <h3 class="flex items-center text-lg font-semibold text-slate-800">
            <PhCloud class="mr-2 text-cf" size="20" weight="fill" />
            Cloudflare 配置
          </h3>
          <span class="ui-pill ui-pill-subtle">{{ settings.recordType }} 记录</span>
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
            <span class="ui-label">记录类型</span>
            <select v-model="settings.recordType" class="ui-field">
              <option value="A">A</option>
              <option value="AAAA">AAAA</option>
            </select>
          </label>
          <label>
            <span class="ui-label">TTL</span>
            <input v-model.number="settings.ttl" min="1" type="number" class="ui-field" />
          </label>
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
                <p class="mt-1 text-xs text-slate-500">仅执行 TCP/HTTP 响应测速，跳过下载环节。</p>
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
                <p class="mt-1 text-xs text-slate-500">低延迟筛选后追加真实下载测速，适合带宽优先场景。</p>
              </button>
            </div>
            <p class="mt-3 text-sm text-slate-500">{{ strategyDescription }}</p>
          </div>

          <label>
            <span class="ui-label">延迟测速线程 (-n)</span>
            <input v-model.number="settings.probeConcurrencyStage1" min="1" max="1000" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">延迟测速次数 (-t)</span>
            <input v-model.number="settings.probePingTimes" min="1" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">下载测速数量 (-dn)</span>
            <input v-model.number="settings.probeDownloadCount" min="1" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">下载测速时间 (-dt 秒)</span>
            <input v-model.number="settings.probeDownloadTimeSeconds" min="1" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">测速端口 (-tp)</span>
            <input v-model.number="settings.probeTcpPort" min="1" max="65535" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">显示结果数量 (-p)</span>
            <input v-model.number="settings.probePrintNum" min="0" type="number" class="ui-field" />
          </label>
          <label class="md:col-span-2">
            <span class="ui-label">测速地址 (-url)</span>
            <input v-model="settings.probeURL" type="url" class="ui-field font-mono" />
          </label>
          <label>
            <span class="ui-label">平均延迟上限 (-tl ms)</span>
            <input v-model.number="settings.maxTcpLatencyMs" min="1" placeholder="留空" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">平均延迟下限 (-tll ms)</span>
            <input v-model.number="settings.minDelayMs" min="0" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">丢包率上限 (-tlr)</span>
            <input v-model.number="settings.maxLossRate" max="1" min="0" step="0.01" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">最低下载速度 (MB/s)</span>
            <input v-model.number="settings.minDownloadMbps" min="0" step="0.1" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">HTTPing 状态码</span>
            <input v-model.number="settings.probeHttpingStatusCode" min="0" type="number" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">地区码筛选 (-cfcolo)</span>
            <input v-model="settings.probeHttpingCfColo" placeholder="HKG,NRT,LAX" type="text" class="ui-field font-mono" />
          </label>

          <button
            type="button"
            class="md:col-span-2 flex items-center justify-between rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left"
            @click="settings.probeHttping = !settings.probeHttping"
          >
            <span>
              <span class="block text-sm font-medium text-slate-700">启用 HTTPing</span>
              <span class="text-xs text-slate-400">启用后会保留 HTTP 响应测速与地区码筛选能力。</span>
            </span>
            <span class="relative inline-flex h-6 w-11 items-center rounded-full transition" :class="settings.probeHttping ? 'bg-primary' : 'bg-slate-300'">
              <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.probeHttping ? 'translate-x-5' : 'translate-x-0'"></span>
            </span>
          </button>

          <div class="md:col-span-2 rounded-2xl border border-slate-200 bg-slate-50/70 p-4 text-sm text-slate-500">
            <p>所有延迟统计默认跳过首包，避免冷连接带来的首包偏移。</p>
            <p class="mt-1">`-p` 是结果显示上限，不是保证值；若不满足 `-tl`、`-tll`、`-sl` 等条件，实际结果数量可能更少。</p>
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
          <label>
            <span class="ui-label">导出目录</span>
            <input v-model="settings.exportTargetDir" type="text" class="ui-field" />
          </label>
          <label>
            <span class="ui-label">文件名</span>
            <input v-model="settings.exportFileName" type="text" class="ui-field" />
          </label>
          <label class="md:col-span-2">
            <span class="ui-label">覆盖策略</span>
            <select v-model="settings.exportOverwrite" class="ui-field">
              <option value="replace_on_start">启动时覆盖</option>
              <option value="append">追加写入</option>
            </select>
          </label>
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
            <input v-model.number="settings.probeEventThrottleMs" min="0" type="number" class="ui-field" />
          </label>

          <div class="md:col-span-2 rounded-2xl border border-slate-200 bg-slate-50/70 p-4 text-sm text-slate-500">
            <p v-if="maskedTokenHint">当前已加载脱敏 Token：{{ maskedTokenHint }}</p>
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
              class="ui-field font-mono"
            />
          </label>

          <button
            type="button"
            class="md:col-span-2 flex items-center justify-between rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left"
            @click="settings.probeDebug = !settings.probeDebug"
          >
            <span>
              <span class="block text-sm font-medium text-slate-700">启用调试日志</span>
              <span class="text-xs text-slate-400">开启后 Go 后端会把详细请求日志写入 `cfip-log.txt`。</span>
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
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-xs text-slate-500">记录名称</label>
            <input v-model="settings.recordName" type="text" class="ui-field h-11 font-mono" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">记录类型</label>
            <select v-model="settings.recordType" class="ui-field h-11">
              <option value="A">A</option>
              <option value="AAAA">AAAA</option>
            </select>
          </div>
        </div>
        <div>
          <label class="block text-xs text-slate-500">TTL</label>
          <input v-model.number="settings.ttl" min="1" type="number" class="ui-field h-11" />
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
              <p class="mt-1 text-xs text-slate-500">仅执行 TCP/HTTP 响应测速，跳过下载。</p>
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
              <p class="mt-1 text-xs text-slate-500">低延迟筛选后追加真实下载测速。</p>
            </button>
          </div>
          <p class="mt-3 text-xs text-slate-500">{{ strategyDescription }}</p>
        </div>

        <div>
          <label class="block text-xs text-slate-500">导出目录</label>
          <input v-model="settings.exportTargetDir" type="text" class="ui-field h-11" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">文件名</label>
          <input v-model="settings.exportFileName" type="text" class="ui-field h-11" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">覆盖策略</label>
          <select v-model="settings.exportOverwrite" class="ui-field h-11">
            <option value="replace_on_start">启动时覆盖</option>
            <option value="append">追加写入</option>
          </select>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-xs text-slate-500">线程 (-n)</label>
            <input v-model.number="settings.probeConcurrencyStage1" min="1" max="1000" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">次数 (-t)</label>
            <input v-model.number="settings.probePingTimes" min="1" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">下载数量 (-dn)</label>
            <input v-model.number="settings.probeDownloadCount" min="1" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">下载时间 (-dt 秒)</label>
            <input v-model.number="settings.probeDownloadTimeSeconds" min="1" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">测速端口 (-tp)</label>
            <input v-model.number="settings.probeTcpPort" min="1" max="65535" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">结果上限 (-p)</label>
            <input v-model.number="settings.probePrintNum" min="0" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">延迟上限 (-tl)</label>
            <input v-model.number="settings.maxTcpLatencyMs" min="1" placeholder="留空" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">延迟下限 (-tll)</label>
            <input v-model.number="settings.minDelayMs" min="0" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">最低速度 (MB/s)</label>
            <input v-model.number="settings.minDownloadMbps" min="0" step="0.1" type="number" class="ui-field h-11" />
          </div>
          <div>
            <label class="block text-xs text-slate-500">丢包率上限 (-tlr)</label>
            <input v-model.number="settings.maxLossRate" max="1" min="0" step="0.01" type="number" class="ui-field h-11" />
          </div>
        </div>

        <div>
          <label class="block text-xs text-slate-500">测速地址 (-url)</label>
          <input v-model="settings.probeURL" type="url" class="ui-field h-11 font-mono" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">HTTPing 状态码</label>
          <input v-model.number="settings.probeHttpingStatusCode" min="0" type="number" class="ui-field h-11" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">地区码筛选 (-cfcolo)</label>
          <input v-model="settings.probeHttpingCfColo" placeholder="HKG,NRT,LAX" type="text" class="ui-field h-11 font-mono" />
        </div>

        <button
          type="button"
          class="flex w-full items-center justify-between rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-left"
          @click="settings.probeHttping = !settings.probeHttping"
        >
          <span>
            <span class="block text-sm font-medium text-slate-700">启用 HTTPing</span>
            <span class="text-xs text-slate-400">所有延迟统计默认跳过首包。</span>
          </span>
          <span class="relative inline-flex h-6 w-11 items-center rounded-full transition" :class="settings.probeHttping ? 'bg-primary' : 'bg-slate-300'">
            <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.probeHttping ? 'translate-x-5' : 'translate-x-0'"></span>
          </span>
        </button>

        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-xs text-slate-500">
          `-p` 只是显示上限；若过滤后候选不足，实际结果数量可以更少。
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
          <input v-model.number="settings.probeEventThrottleMs" min="0" type="number" class="ui-field h-11" />
        </div>
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
            class="ui-field h-11 font-mono"
          />
        </div>
        <button
          type="button"
          class="flex w-full items-center justify-between rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-left"
          @click="settings.probeDebug = !settings.probeDebug"
        >
          <span>
            <span class="block text-sm font-medium text-slate-700">启用调试日志</span>
            <span class="text-xs text-slate-400">开启后写入 `cfip-log.txt`，抓包监听地址仅在调试时生效。</span>
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

    <button
      type="button"
      class="ui-button ui-button-primary h-12 w-full"
      :disabled="loading || saveBlockedByMaskedToken"
      @click="$emit('save')"
    >
      <PhFloppyDisk size="18" weight="fill" />
      {{ saveButtonText }}
    </button>
  </section>
</template>
