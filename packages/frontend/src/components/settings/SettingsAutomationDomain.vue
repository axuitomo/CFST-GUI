<script setup vapor lang="ts">
import { computed } from "vue";
import { PhArrowSquareOut, PhCaretDown, PhDownload, PhFolderOpen, PhGauge, PhQuestion, PhShieldCheck } from "@phosphor-icons/vue";
import type { AppInfo, SchedulerStatus } from "../../lib/bridge";
import type { SchedulerTriggerMode, SettingsForm } from "../../lib/config/settingsForm";
import { statusText, workflowLabel, type SettingsSectionController, type SettingsTimestampFormatOptions } from "./types";

const props = defineProps<{
  appInfo: AppInfo;
  formatTimestamp: (value: string, options?: SettingsTimestampFormatOptions) => string;
  schedulerStatus: SchedulerStatus | null;
  sections: SettingsSectionController;
  settings: SettingsForm;
}>();

defineEmits<{
  (event: "scheduler-daily-times-blur"): void;
  (event: "select-export-target"): void;
}>();

const isDockerWebUI = computed(() => props.appInfo.install_mode === "docker_compose");
const isAndroidApp = computed(() => props.appInfo.platform === "android");
const isWebUIDesktopShell = computed(() => isDockerWebUI.value);

const schedulerAvailable = computed(() => {
  const runtimePlatform = props.appInfo.platform.trim();
  return runtimePlatform === "" || runtimePlatform === "android" || runtimePlatform.includes("/") || isDockerWebUI.value;
});

const schedulerTriggerModeLabel = computed(() => (props.settings.schedulerTriggerMode === "daily" ? "固定时间" : "固定间隔"));
const schedulerSummaryLabel = computed(() => {
  if (!schedulerAvailable.value) return "移动端隐藏";
  if (!props.settings.schedulerEnabled) return "单任务模式未启用";
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

const exportTargetDisplay = computed(() => {
  if (props.settings.exportTargetUri.trim()) return `Android SAF 导出目录：${props.settings.exportTargetUri.trim()}`;
  if (props.settings.exportTargetDir.trim()) return `当前导出目录：${props.settings.exportTargetDir.trim()}`;
  return isAndroidApp.value ? "尚未选择 Android SAF 导出目录" : "当前导出目录：应用默认导出目录";
});
</script>

<template>
  <section class="settings-domain">
    <div class="settings-domain-header">
      <div class="settings-domain-title-wrap" :class="{ pinned: sections.pinnedHelpSection === 'automation' }">
        <h3 class="settings-domain-title flex items-center">
          自动化与导出
          <button type="button" class="settings-help-btn" aria-label="查看说明" @click="sections.toggleHelp('automation')">
            <PhQuestion size="13" />
          </button>
        </h3>
        <p class="settings-domain-copy">配置定时任务、结果导出、测速后自动推送与共享上传筛选。</p>
      </div>
    </div>
    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <details v-if="schedulerAvailable" :open="sections.isOpen('scheduler')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('scheduler', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhGauge class="mr-2 shrink-0 text-cf" size="20" />
            定时任务
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ schedulerSummaryLabel }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('scheduler') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('scheduler')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
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
              <span class="block text-sm font-medium text-slate-700">测速成功后自动推送 DNS 记录</span>
              <span class="text-xs text-slate-500">写入「DNS 推送平台」中全部已启用平台。</span>
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
              <p class="mt-2 break-all font-mono text-xs text-slate-700">{{ schedulerStatus?.next_run_at ? formatTimestamp(schedulerStatus.next_run_at) : "保存后计算" }}</p>
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

      <details :open="sections.isOpen('export')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('export', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhDownload class="mr-2 shrink-0 text-slate-500" size="20" />
            导出设置
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ settings.exportOverwrite === "append" ? "追加写入" : "覆盖写出" }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('export') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('export')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
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

      <details :open="sections.isOpen('postPush')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('postPush', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhArrowSquareOut class="mr-2 shrink-0 text-emerald-600" size="20" weight="fill" />
            测速后自动推送列表
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">按 provider 勾选</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('postPush') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('postPush')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
          <div class="md:col-span-2 text-sm text-slate-500">手动单任务测速完成后按勾选推送；定时任务沿用原 scheduler 逻辑，避免重复。</div>
          <label class="flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
            <input v-model="settings.postProbePushCloudflareEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
            <span class="min-w-0">
              <span class="block text-sm font-medium text-slate-700">测速完成后推送 DNS 记录</span>
              <span class="text-xs text-slate-500">会复用共享上传策略和各平台的 Top N、记录类型与分流规则。</span>
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

      <details :open="sections.isOpen('upload')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('upload', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhShieldCheck class="mr-2 shrink-0 text-emerald-600" size="20" weight="fill" />
            共享上传策略
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ settings.uploadSharedFilterEnabled ? "共享筛选已启用" : "共享筛选未启用" }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('upload') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('upload')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
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
</template>
