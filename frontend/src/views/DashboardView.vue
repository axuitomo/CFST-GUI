<script setup lang="ts">
import { PhActivity, PhPause, PhPlay, PhPlayCircle } from "@phosphor-icons/vue";
import type { TaskTone } from "../lib/bridge";
import type { TaskSnapshot } from "../lib/bridge";
import TaskProcessView from "../components/ui/TaskProcessView.vue";

interface ActivityEntry {
  detail: string;
  title: string;
  ts: string;
}

interface HistoryEntry {
  debugLogPath?: string;
  debugLogTarget?: string;
  detail: string;
  exported: number;
  failureSummary: string;
  targetPath: string;
  taskId: string;
  title: string;
  tone: TaskTone;
  updatedAt: string;
}

interface SummaryStats {
  accepted: number;
  exported: number;
  failed: number;
  filtered: number;
  invalid: number;
  passed: number;
  processed: number;
  total: number;
}

interface ProcessEntry {
  detail: string;
  stage: string;
  title: string;
  tone: "success" | "error" | "running" | "info" | "warning";
  ts: string;
}

interface TaskState {
  active: boolean;
  exportPath: string;
  stage: string;
  taskId: string;
}

interface ProbeConfigSummary {
  portPolicy: "source_override_global" | "fixed_global";
  tcpPort: number;
}

interface DownloadSpeedState {
  active: boolean;
  averageSpeedMbS: number | null;
  bytesRead: number;
  colo: string;
  currentSpeedMbS: number | null;
  elapsedMs: number;
  ip: string;
}

interface TimestampFormatOptions {
  fallback?: string;
  includeDate?: boolean;
  includeOffset?: boolean;
  includeSeconds?: boolean;
}

const props = defineProps<{
  activityFeed: ActivityEntry[];
  canPauseTask: boolean;
  canResumeTask: boolean;
  canStartTask: boolean;
  downloadSpeedState: DownloadSpeedState;
  exportHistory: HistoryEntry[];
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  hasActiveTask: boolean;
  loading: boolean;
  platform: "desktop" | "mobile";
  processTrace: ProcessEntry[];
  probeConfig: ProbeConfigSummary;
  progressPercent: number;
  statusLabel: string;
  statusTone: TaskTone;
  summary: SummaryStats;
  task: TaskState;
  taskSnapshot: TaskSnapshot | null;
}>();

defineEmits<{
  (event: "clear-process"): void;
  (event: "open-history-target", targetPath: string): void;
  (event: "pause"): void;
  (event: "resume"): void;
  (event: "start"): void;
}>();

function toneDotClass(tone: TaskTone) {
  if (tone === "completed" || tone === "partial" || tone === "no_results") {
    return "bg-emerald-500";
  }

  if (tone === "failed") {
    return "bg-rose-500";
  }

  if (tone === "cooling" || tone === "warning") {
    return "bg-amber-400";
  }

  if (tone === "running" || tone === "preparing") {
    return "bg-primary";
  }

  return "bg-slate-400";
}

function formatSpeed(value: number | null) {
  return typeof value === "number" && Number.isFinite(value) ? `${value.toFixed(2)} MB/s` : "-";
}

function formatTimestampLabel(value: string, options?: TimestampFormatOptions) {
  return props.formatTimestamp(value, options);
}

function taskContextNumber(key: string) {
  const value = props.taskSnapshot?.task_context?.[key];
  const numeric = Number(value);
  return Number.isFinite(numeric) && numeric > 0 ? numeric : null;
}

function taskContextString(key: string) {
  const value = props.taskSnapshot?.task_context?.[key];
  return typeof value === "string" && value.trim() ? value.trim() : "";
}

function taskContextPorts() {
  const value = props.taskSnapshot?.task_context?.source_port_values;
  if (!Array.isArray(value)) {
    return [];
  }
  return value.map((entry) => Number(entry)).filter((entry) => Number.isFinite(entry) && entry > 0);
}

function taskGroupedPorts() {
  const value = props.taskSnapshot?.task_context?.grouped_ports;
  if (!Array.isArray(value)) {
    return [];
  }
  return value.map((entry) => Number(entry)).filter((entry) => Number.isFinite(entry) && entry > 0);
}

function taskCurrentPortLabel() {
  const policy = resolvedPortPolicy();
  const currentPort = taskContextNumber("current_test_port");
  if (currentPort) {
    return String(currentPort);
  }
  const groupedPorts = taskGroupedPorts();
  if (groupedPorts.length > 1) {
    return `按端口分组 ${groupedPorts.join(" / ")}`;
  }
  if (groupedPorts.length === 1) {
    return String(groupedPorts[0]);
  }
  if (policy === "source_override_global" && taskContextPorts().length > 0) {
    return `源端口 ${taskContextPorts().join(" / ")}`;
  }
  const globalPort = resolvedGlobalPort();
  return globalPort ? String(globalPort) : "-";
}

function portPolicyLabel(policy: string) {
  if (policy === "fixed_global") {
    return "固定测速端口";
  }
  return policy === "source_override_global" ? "输入源端口优先" : policy || "-";
}

function resolvedGlobalPort() {
  return taskContextNumber("global_tcp_port") || normalizedPositivePort(props.probeConfig.tcpPort);
}

function resolvedPortPolicy() {
  const policy = taskContextString("port_policy");
  return policy || props.probeConfig.portPolicy || "source_override_global";
}

function normalizedPositivePort(value: number | null | undefined) {
  const numeric = Number(value);
  return Number.isFinite(numeric) && numeric > 0 ? numeric : null;
}
</script>

<template>
  <section v-if="platform === 'desktop'" class="dashboard-workbench space-y-5">
    <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
      <article class="ui-card dashboard-metric p-4">
        <p class="text-sm font-medium text-slate-500">当前状态</p>
        <div class="mt-2 flex items-center">
          <span :class="toneDotClass(statusTone)" class="mr-2 h-3 w-3 rounded-full"></span>
          <strong class="text-xl font-bold text-slate-800">{{ statusLabel }}</strong>
        </div>
      </article>

      <article class="ui-card dashboard-metric p-4">
        <p class="text-sm font-medium text-slate-500">已处理</p>
        <strong class="mt-2 block text-xl font-bold text-slate-800"> {{ summary.processed }} / {{ summary.total || summary.accepted || "-" }} </strong>
        <p class="mt-1 text-xs text-slate-400">已过滤 {{ summary.filtered }} / 无效 {{ summary.invalid }}</p>
      </article>

      <article class="ui-card dashboard-metric p-4">
        <p class="text-sm font-medium text-slate-500">有效结果</p>
        <strong class="mt-2 block text-xl font-bold text-emerald-600">{{ summary.passed || summary.exported }}</strong>
        <p class="mt-1 text-xs text-slate-400">已导出 {{ summary.exported }}</p>
      </article>

      <article class="ui-card dashboard-metric p-4">
        <p class="text-sm font-medium text-slate-500">失败结果</p>
        <strong class="mt-2 block text-xl font-bold text-rose-500">{{ summary.failed }}</strong>
        <p class="mt-1 text-xs text-slate-400">已接收 {{ summary.accepted }}</p>
      </article>
    </div>

    <article class="ui-card dashboard-progress p-5">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-4">
        <div class="min-w-0">
          <h2 class="flex items-center text-base font-semibold text-slate-800">
            <PhActivity class="mr-2 text-primary" size="20" />
            探测进度
          </h2>
          <p class="mt-1 text-sm text-slate-500">实时展示 IP池、TCP测延迟、追踪探测、文件测速、导出与失败节点。</p>
        </div>

        <div class="flex flex-wrap items-center justify-end gap-2">
          <button type="button" class="ui-button ui-button-primary" :disabled="loading || !canStartTask" @click="$emit('start')">
            <PhPlay size="18" weight="fill" />
            启动任务
          </button>
          <button type="button" class="ui-button ui-button-warning" :disabled="!canPauseTask" @click="$emit('pause')">
            <PhPause size="18" weight="fill" />
            暂停任务
          </button>
          <button type="button" class="ui-button ui-button-success" :disabled="loading || !canResumeTask" @click="$emit('resume')">
            <PhPlayCircle size="18" weight="fill" />
            继续任务
          </button>
        </div>
      </div>

      <div class="dashboard-progress-track h-3 overflow-hidden rounded-full border">
        <div class="h-full rounded-full bg-primary transition-all duration-300" :style="{ width: `${progressPercent}%` }"></div>
      </div>
      <div class="mt-2 flex items-center justify-between gap-3 text-xs text-slate-500">
        <span class="overflow-safe">任务 {{ task.taskId || "等待中" }}</span>
        <span>{{ progressPercent }}% 完成</span>
      </div>
    </article>

    <article class="ui-card dashboard-port-card p-4">
      <div class="grid grid-cols-2 gap-3 xl:grid-cols-4">
        <div class="min-w-0">
          <p class="text-sm font-medium text-slate-500">全局测速端口</p>
          <strong class="mt-1 block truncate text-base font-semibold text-slate-800">{{ resolvedGlobalPort() || "-" }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-sm font-medium text-slate-500">输入源端口</p>
          <strong class="mt-1 block truncate text-base font-semibold text-slate-800">{{ taskContextPorts().join(" / ") || "未指定" }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-sm font-medium text-slate-500">当前测试端口</p>
          <strong class="mt-1 block text-base font-semibold text-primary break-words">{{ taskCurrentPortLabel() }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-sm font-medium text-slate-500">端口策略</p>
          <strong class="mt-1 block truncate text-base font-semibold text-slate-800">{{ portPolicyLabel(resolvedPortPolicy()) }}</strong>
        </div>
      </div>
    </article>

    <article class="ui-card dashboard-speed-card p-4">
      <div class="grid grid-cols-2 gap-3 xl:grid-cols-4">
        <div class="min-w-0">
          <p class="text-sm font-medium text-slate-500">IP</p>
          <strong class="mt-1 block truncate text-base font-semibold text-slate-800">{{ downloadSpeedState.active || downloadSpeedState.ip ? downloadSpeedState.ip : "-" }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-sm font-medium text-slate-500">colo</p>
          <strong class="mt-1 block truncate text-base font-semibold text-slate-800">{{ downloadSpeedState.active || downloadSpeedState.colo ? downloadSpeedState.colo || "-" : "-" }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-sm font-medium text-slate-500">实时速率</p>
          <strong class="mt-1 block truncate text-base font-semibold text-primary">{{ formatSpeed(downloadSpeedState.currentSpeedMbS) }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-sm font-medium text-slate-500">平均速率</p>
          <strong class="mt-1 block truncate text-base font-semibold text-emerald-600">{{ formatSpeed(downloadSpeedState.averageSpeedMbS) }}</strong>
        </div>
      </div>
    </article>

    <TaskProcessView :entries="processTrace" :format-timestamp="formatTimestamp" title="实时测试进程" @clear="$emit('clear-process')" />

    <div class="grid gap-5 xl:grid-cols-2">
      <article class="ui-card dashboard-log-card p-5">
        <div class="mb-3 flex items-center justify-between gap-3">
          <div class="min-w-0">
            <h3 class="text-base font-semibold text-slate-800">最近活动</h3>
            <p class="mt-1 text-sm text-slate-500">关键状态变化会在这里滚动保留。</p>
          </div>
          <span class="ui-pill ui-pill-subtle">{{ activityFeed.length }} 条</span>
        </div>

        <ul class="space-y-2.5">
          <li v-for="entry in activityFeed" :key="`${entry.ts}-${entry.title}`" class="dashboard-activity-item rounded-xl border p-3">
            <p class="overflow-safe font-semibold text-slate-800">{{ entry.title }}</p>
            <p class="overflow-safe mt-1 text-sm text-slate-500">{{ entry.detail }}</p>
            <p class="overflow-safe mt-2 text-xs text-slate-400">{{ formatTimestampLabel(entry.ts) }}</p>
          </li>
          <li v-if="activityFeed.length === 0" class="ui-card border-dashed p-5 text-center text-sm text-slate-400">当前还没有活动记录。</li>
        </ul>
      </article>

      <article class="ui-card dashboard-export-card p-5">
        <div class="mb-3 flex items-center justify-between gap-3">
          <div class="min-w-0">
            <h3 class="text-base font-semibold text-slate-800">最近导出</h3>
            <p class="mt-1 text-sm text-slate-500">保留最新的导出路径、数量和失败摘要。</p>
          </div>
          <span class="ui-pill ui-pill-subtle">{{ exportHistory.length }} 条</span>
        </div>

        <ul class="space-y-2.5">
          <li v-for="entry in exportHistory" :key="entry.taskId" class="dashboard-export-item rounded-xl border p-3">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <p class="overflow-safe font-semibold text-slate-800">{{ entry.title }}</p>
                <p class="overflow-safe mt-1 text-sm text-slate-500">{{ entry.detail }}</p>
                <p class="mt-2 truncate text-xs text-slate-400">任务 {{ entry.taskId }} · {{ formatTimestampLabel(entry.updatedAt) }}</p>
                <p v-if="entry.debugLogPath" class="mt-1 truncate text-xs text-slate-400">LOG {{ entry.debugLogPath }}</p>
                <p v-if="entry.failureSummary" class="overflow-safe mt-1 text-xs text-amber-600">异常摘要：{{ entry.failureSummary }}</p>
              </div>
              <div class="grid shrink-0 gap-2">
                <button type="button" class="ui-button ui-button-ghost px-3 py-2 text-xs" :disabled="!entry.targetPath" @click="$emit('open-history-target', entry.targetPath)">打开路径</button>
                <button v-if="entry.debugLogPath" type="button" class="ui-button ui-button-ghost px-3 py-2 text-xs" @click="$emit('open-history-target', entry.debugLogTarget || entry.debugLogPath || '')">打开日志</button>
              </div>
            </div>
          </li>
          <li v-if="exportHistory.length === 0" class="ui-card border-dashed p-5 text-center text-sm text-slate-400">当前还没有导出记录。</li>
        </ul>
      </article>
    </div>
  </section>

  <section v-else class="dashboard-workbench space-y-4">
    <article class="ui-card dashboard-mobile-hero flex items-center justify-between gap-3 p-4">
      <div class="min-w-0">
        <p class="text-xs font-medium text-slate-500">当前任务状态</p>
        <div class="mt-1 flex items-center">
          <span :class="toneDotClass(statusTone)" class="mr-2 h-3 w-3 rounded-full"></span>
          <strong class="truncate text-xl font-bold text-slate-800">{{ statusLabel }}</strong>
        </div>
      </div>
      <div class="shrink-0 text-right">
        <p class="text-xs font-medium text-slate-500">处理进度</p>
        <p class="text-xl font-bold text-slate-800">{{ progressPercent }}%</p>
      </div>
    </article>

    <div class="grid grid-cols-2 gap-3">
      <article class="ui-card dashboard-metric p-4">
        <p class="text-xs font-medium text-slate-500">总计</p>
        <strong class="mt-2 block text-2xl font-bold text-slate-800">{{ summary.accepted || "-" }}</strong>
      </article>
      <article class="ui-card dashboard-metric p-4">
        <p class="text-xs font-medium text-slate-500">已处理</p>
        <strong class="mt-2 block text-2xl font-bold text-primary">{{ summary.processed }}</strong>
      </article>
      <article class="ui-card dashboard-metric p-4">
        <p class="text-xs font-medium text-slate-500">有效结果</p>
        <strong class="mt-2 block text-2xl font-bold text-emerald-500">{{ summary.passed }}</strong>
      </article>
      <article class="ui-card dashboard-metric p-4">
        <p class="text-xs font-medium text-slate-500">失败结果</p>
        <strong class="mt-2 block text-2xl font-bold text-rose-500">{{ summary.failed }}</strong>
      </article>
    </div>

    <article class="ui-card dashboard-progress p-4">
      <div class="dashboard-progress-track mb-4 h-3 overflow-hidden rounded-full">
        <div class="h-full rounded-full bg-primary transition-all duration-300" :style="{ width: `${progressPercent}%` }"></div>
      </div>
      <div class="grid grid-cols-3 gap-2">
        <button type="button" class="ui-button ui-button-primary h-12 px-2" :disabled="loading || !canStartTask" @click="$emit('start')">
          <PhPlay size="18" weight="fill" />
          开始探测
        </button>
        <button type="button" class="ui-button ui-button-warning h-12 px-2" :disabled="!canPauseTask" @click="$emit('pause')">
          <PhPause size="18" weight="fill" />
          暂停
        </button>
        <button type="button" class="ui-button ui-button-success h-12 px-2" :disabled="loading || !canResumeTask" @click="$emit('resume')">
          <PhPlayCircle size="18" weight="fill" />
          继续
        </button>
      </div>
    </article>

    <article class="ui-card dashboard-port-card p-4">
      <div class="grid grid-cols-2 gap-3">
        <div class="min-w-0">
          <p class="text-xs font-medium text-slate-500">全局端口</p>
          <strong class="mt-1 block truncate text-base font-semibold text-slate-800">{{ resolvedGlobalPort() || "-" }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-xs font-medium text-slate-500">实际端口</p>
          <strong class="mt-1 block text-base font-semibold text-primary break-words">{{ taskCurrentPortLabel() }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-xs font-medium text-slate-500">源端口</p>
          <strong class="mt-1 block truncate text-base font-semibold text-slate-800">{{ taskContextPorts().join(" / ") || "未指定" }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-xs font-medium text-slate-500">策略</p>
          <strong class="mt-1 block truncate text-base font-semibold text-slate-800">{{ portPolicyLabel(resolvedPortPolicy()) }}</strong>
        </div>
      </div>
    </article>

    <article class="ui-card dashboard-speed-card p-4">
      <div class="grid grid-cols-2 gap-3">
        <div class="min-w-0">
          <p class="text-xs font-medium text-slate-500">IP</p>
          <strong class="mt-1 block truncate text-base font-semibold text-slate-800">{{ downloadSpeedState.active || downloadSpeedState.ip ? downloadSpeedState.ip : "-" }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-xs font-medium text-slate-500">colo</p>
          <strong class="mt-1 block truncate text-base font-semibold text-slate-800">{{ downloadSpeedState.active || downloadSpeedState.colo ? downloadSpeedState.colo || "-" : "-" }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-xs font-medium text-slate-500">实时速率</p>
          <strong class="mt-1 block truncate text-base font-semibold text-primary">{{ formatSpeed(downloadSpeedState.currentSpeedMbS) }}</strong>
        </div>
        <div class="min-w-0">
          <p class="text-xs font-medium text-slate-500">平均速率</p>
          <strong class="mt-1 block truncate text-base font-semibold text-emerald-600">{{ formatSpeed(downloadSpeedState.averageSpeedMbS) }}</strong>
        </div>
      </div>
    </article>

    <TaskProcessView :entries="processTrace" :format-timestamp="formatTimestamp" mobile title="实时测试进程" @clear="$emit('clear-process')" />
  </section>
</template>
