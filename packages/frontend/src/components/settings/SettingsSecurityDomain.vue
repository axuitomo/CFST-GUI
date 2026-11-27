<script setup vapor lang="ts">
import { computed } from "vue";
import { PhCaretDown, PhDownload, PhFolderOpen, PhQuestion, PhShieldCheck } from "@phosphor-icons/vue";
import type { AndroidBatteryStatus, AndroidKeepAliveStatus, AndroidNotificationPermissionStatus } from "../../lib/bridge";
import type { SettingsForm } from "../../lib/config/settingsForm";
import { REQUEST_HEADERS_TEMPLATE, type SettingsSectionController } from "./types";

const props = defineProps<{
  androidBatteryStatus?: AndroidBatteryStatus | null;
  androidKeepAliveStatus?: AndroidKeepAliveStatus | null;
  androidNotificationStatus?: AndroidNotificationPermissionStatus | null;
  loading: boolean;
  maskedTokenHint: string;
  platform: "desktop" | "mobile";
  sections: SettingsSectionController;
  settings: SettingsForm;
}>();

defineEmits<{
  (event: "export-debug-log"): void;
  (event: "export-diagnostic-package"): void;
  (event: "open-battery-settings", mode: "request" | "settings" | "details"): void;
  (event: "open-log-directory"): void;
  (event: "open-notification-settings"): void;
  (event: "request-notification-permission"): void;
  (event: "set-keep-alive-enabled", enabled: boolean): void;
}>();

const batteryStatusLabel = computed(() => {
  if (!props.androidBatteryStatus?.supported) return "系统未提供";
  return props.androidBatteryStatus.ignoring_optimizations ? "已放行" : "待放行";
});

const keepAliveStatusLabel = computed(() => {
  const status = props.androidKeepAliveStatus;
  if (!status?.supported) return "不支持";
  if (!status.enabled) return "已关闭";
  if (!status.notification_permission_granted) return "待通知权限";
  return status.running ? "运行中" : "启动中";
});

const notificationPermissionLabel = computed(() => {
  if (!props.androidNotificationStatus?.supported) return "无需授权";
  return props.androidNotificationStatus.granted ? "已允许" : "待允许";
});

const notificationPermissionCanRequest = computed(() => props.androidNotificationStatus?.can_request === true);
const notificationPermissionNeedsSettings = computed(() => props.androidNotificationStatus?.open_settings_recommended === true);
</script>

<template>
  <section class="settings-domain">
    <div class="settings-domain-header">
      <div class="settings-domain-title-wrap" :class="{ pinned: sections.pinnedHelpSection === 'security' }">
        <h3 class="settings-domain-title flex items-center">
          安全与诊断
          <button type="button" class="settings-help-btn" aria-label="查看说明" @click="sections.toggleHelp('security')">
            <PhQuestion size="13" />
          </button>
        </h3>
        <p class="settings-domain-copy">管理异常冷却与重试、Android 通知和后台权限、请求身份及调试日志。</p>
      </div>
    </div>
    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <details :open="sections.isOpen('protection')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('protection', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhShieldCheck class="mr-2 shrink-0 text-emerald-600" size="20" weight="fill" />
            异常保护
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">事件节流 {{ settings.probeEventThrottleMs }}ms</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('protection') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('protection')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
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
            <p>冷却只对链路侧异常生效（毫秒级就返回的失败，如路由不可达、端口耗尽）；候选本身不通的超时不计入，文件测速阶段的失败也不参与冷却。重试对 TCP、追踪、文件测速三个阶段都生效；0 表示关闭对应保护。</p>
            <p v-if="platform === 'mobile'" class="mt-1">移动端长任务建议同时关闭厂商省电限制、自启动拦截和后台冻结，否则前台服务也可能被系统延迟回收。</p>
            <p class="mt-1">保存后 Wails 桌面端会把这些参数映射到当前 Go CFST 后端。</p>
            <p class="mt-1">若没有重新输入完整 Token，保存动作会被阻止，避免占位值覆盖本地配置。</p>
          </div>
        </div>
      </details>

      <details :open="sections.isOpen('debug')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('debug', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhShieldCheck class="mr-2 shrink-0 text-amber-600" size="20" weight="fill" />
            请求身份与调试
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ settings.probeDebug ? "调试日志开启" : "调试日志关闭" }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('debug') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('debug')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
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
</template>
