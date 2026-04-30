<script setup lang="ts">
import {
  PhActivity,
  PhArrowClockwise,
  PhCopy,
  PhPause,
  PhPlay,
  PhPlayCircle,
  PhRocketLaunch,
  PhWarningCircle,
} from "@phosphor-icons/vue";
import type {
  ProbeResult,
  ProbeResultFilter,
  ProbeResultOrder,
  ProbeResultSortBy,
  TaskSnapshot,
  TaskTone,
} from "../lib/bridge";
import TaskProcessView from "../components/ui/TaskProcessView.vue";

interface ActivityEntry {
  detail: string;
  title: string;
  ts: string;
}

interface HistoryEntry {
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

defineProps<{
  activityFeed: ActivityEntry[];
  exportHistory: HistoryEntry[];
  hasActiveTask: boolean;
  lastHistoryEntry: HistoryEntry | null;
  loading: boolean;
  platform: "desktop" | "mobile";
  processTrace: ProcessEntry[];
  probeWarnings: string[];
  progressPercent: number;
  resultFilter: ProbeResultFilter;
  resultFilterOptions: Array<{ label: string; value: ProbeResultFilter }>;
  resultOrder: ProbeResultOrder;
  resultRows: ProbeResult[];
  resultSortBy: ProbeResultSortBy;
  resultSortOptions: Array<{ label: string; value: ProbeResultSortBy }>;
  resultsLoading: boolean;
  statusLabel: string;
  statusTone: TaskTone;
  summary: SummaryStats;
  task: TaskState;
  taskSnapshot: TaskSnapshot | null;
}>();

const emit = defineEmits<{
  (event: "clear-process"): void;
  (event: "copy-address", address: string): void;
  (event: "open-history-target", targetPath: string): void;
  (event: "pause"): void;
  (event: "refresh-results"): void;
  (event: "rerun-address", address: string): void;
  (event: "resume"): void;
  (event: "start"): void;
  (event: "update-filter", filter: ProbeResultFilter): void;
  (event: "update-order", order: ProbeResultOrder): void;
  (event: "update-sort", sortBy: ProbeResultSortBy): void;
}>();

function toneDotClass(tone: TaskTone) {
  if (tone === "completed" || tone === "partial" || tone === "no_results") {
    return "bg-emerald-500";
  }

  if (tone === "failed") {
    return "bg-rose-500";
  }

  if (tone === "cooling") {
    return "bg-amber-400";
  }

  if (tone === "running" || tone === "preparing") {
    return "bg-primary";
  }

  return "bg-slate-400";
}

function resultToneClass(stageStatus: string) {
  if (stageStatus.includes("failed")) {
    return "bg-rose-50 text-rose-700";
  }

  if (stageStatus.includes("passed")) {
    return "bg-emerald-50 text-emerald-700";
  }

  return "bg-slate-100 text-slate-600";
}

function taskStatusLabel(status: string | null | undefined) {
  const labels: Record<string, string> = {
    accepted: "已受理",
    completed: "已完成",
    cooling: "冷却中",
    failed: "失败",
    idle: "就绪",
    no_results: "无结果",
    pending: "待处理",
    preprocessed: "预处理",
    preparing: "准备中",
    running: "运行中",
    stage1: "第一阶段",
    stage2: "第二阶段",
    stage3: "第三阶段",
    waiting: "等待中",
  };

  return status ? labels[status] || status : "等待中";
}

function resultStageStatusLabel(stageStatus: string) {
  const status = stageStatus.trim();
  if (!status) {
    return "-";
  }

  if (status === "pending") {
    return "待处理";
  }

  if (status.endsWith("_passed")) {
    return `${taskStatusLabel(status.replace(/_passed$/, ""))}通过`;
  }

  if (status.endsWith("_failed")) {
    return `${taskStatusLabel(status.replace(/_failed$/, ""))}失败`;
  }

  return taskStatusLabel(status);
}

function exportStatusLabel(exportStatus: string) {
  const labels: Record<string, string> = {
    exported: "已导出",
    failed: "导出失败",
    pending: "待导出",
  };

  return labels[exportStatus] || exportStatus || "-";
}

function formatMetric(value: number | null | undefined, suffix = "ms") {
  return typeof value === "number" && Number.isFinite(value) ? `${value}${suffix}` : "-";
}

function formatSpeed(value: number | null | undefined) {
  return typeof value === "number" && Number.isFinite(value) ? `${value.toFixed(2)} Mbps` : "-";
}

function onFilterChange(event: Event) {
  emit("update-filter", (event.target as HTMLSelectElement).value as ProbeResultFilter);
}

function onSortChange(event: Event) {
  emit("update-sort", (event.target as HTMLSelectElement).value as ProbeResultSortBy);
}

function onOrderChange(event: Event) {
  emit("update-order", (event.target as HTMLSelectElement).value as ProbeResultOrder);
}
</script>

<template>
  <section v-if="platform === 'desktop'" class="space-y-6">
    <div class="grid gap-6 md:grid-cols-2 xl:grid-cols-4">
      <article class="ui-card p-5">
        <p class="text-sm font-medium text-slate-500">当前状态</p>
        <div class="mt-3 flex items-center">
          <span :class="toneDotClass(statusTone)" class="mr-2 h-3 w-3 rounded-full"></span>
          <strong class="text-2xl font-bold text-slate-800">{{ statusLabel }}</strong>
        </div>
      </article>

      <article class="ui-card p-5">
        <p class="text-sm font-medium text-slate-500">已处理</p>
        <strong class="mt-3 block text-2xl font-bold text-slate-800">
          {{ summary.processed }} / {{ summary.total || summary.accepted || "-" }}
        </strong>
        <p class="mt-1 text-xs text-slate-400">已过滤 {{ summary.filtered }} / 无效 {{ summary.invalid }}</p>
      </article>

      <article class="ui-card p-5">
        <p class="text-sm font-medium text-slate-500">有效结果</p>
        <strong class="mt-3 block text-2xl font-bold text-emerald-600">{{ summary.passed || summary.exported }}</strong>
        <p class="mt-1 text-xs text-slate-400">已导出 {{ summary.exported }}</p>
      </article>

      <article class="ui-card p-5">
        <p class="text-sm font-medium text-slate-500">失败结果</p>
        <strong class="mt-3 block text-2xl font-bold text-rose-500">{{ summary.failed }}</strong>
        <p class="mt-1 text-xs text-slate-400">已接收 {{ summary.accepted }}</p>
      </article>
    </div>

    <article class="ui-card p-6">
      <div class="mb-4 flex items-center justify-between">
        <div>
          <h2 class="flex items-center text-lg font-semibold text-slate-800">
            <PhActivity class="mr-2 text-primary" size="20" />
            探测进度
          </h2>
          <p class="mt-1 text-sm text-slate-500">实时展示预处理、延迟测速、下载测速、导出与失败节点。</p>
        </div>

        <div class="flex items-center gap-3">
          <button type="button" class="ui-button ui-button-primary" :disabled="loading" @click="$emit('start')">
            <PhPlay size="18" weight="fill" />
            启动任务
          </button>
          <button type="button" class="ui-button ui-button-warning" :disabled="loading || !hasActiveTask" @click="$emit('pause')">
            <PhPause size="18" weight="fill" />
            暂停任务
          </button>
          <button type="button" class="ui-button ui-button-success" :disabled="loading || !task.taskId" @click="$emit('resume')">
            <PhPlayCircle size="18" weight="fill" />
            恢复任务
          </button>
        </div>
      </div>

      <div class="h-4 overflow-hidden rounded-full border border-slate-200 bg-slate-100">
        <div class="h-full rounded-full bg-primary transition-all duration-300" :style="{ width: `${progressPercent}%` }"></div>
      </div>
      <div class="mt-2 flex items-center justify-between text-xs text-slate-500">
        <span>任务 {{ task.taskId || "等待中" }}</span>
        <span>{{ progressPercent }}% 完成</span>
      </div>
    </article>

    <article class="ui-card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 bg-slate-50/70 px-6 py-4">
        <div>
          <h3 class="text-lg font-semibold text-slate-800">结果表</h3>
          <p class="mt-1 text-sm text-slate-500">这里会自动同步当前任务的事件进度与结果快照。</p>
        </div>

        <div class="flex flex-wrap items-center gap-3">
          <label class="text-sm text-slate-500">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">过滤</span>
            <select class="ui-field min-w-28" :value="resultFilter" @change="onFilterChange">
              <option v-for="option in resultFilterOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
          <label class="text-sm text-slate-500">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">排序</span>
            <select class="ui-field min-w-28" :value="resultSortBy" @change="onSortChange">
              <option v-for="option in resultSortOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
          <label class="text-sm text-slate-500">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">方向</span>
            <select class="ui-field min-w-24" :value="resultOrder" @change="onOrderChange">
              <option value="asc">升序</option>
              <option value="desc">降序</option>
            </select>
          </label>
          <button type="button" class="ui-button ui-button-ghost" :disabled="!task.taskId || resultsLoading" @click="$emit('refresh-results')">
            <PhArrowClockwise size="16" />
            {{ resultsLoading ? "刷新中" : "刷新表格" }}
          </button>
        </div>
      </div>

      <div class="flex flex-wrap gap-3 border-b border-slate-200 px-6 py-3 text-xs text-slate-500">
        <span>状态：{{ taskStatusLabel(taskSnapshot?.status) }}</span>
        <span>阶段：{{ taskStatusLabel(taskSnapshot?.current_stage || task.stage) }}</span>
        <span>结果：{{ resultRows.length }}</span>
        <span>更新：{{ taskSnapshot?.updated_at || "-" }}</span>
      </div>

      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead class="bg-slate-50 text-left text-slate-500">
            <tr>
              <th class="px-6 py-3 font-semibold">地址</th>
              <th class="px-6 py-3 font-semibold">阶段状态</th>
              <th class="px-6 py-3 font-semibold">TCP</th>
              <th class="px-6 py-3 font-semibold">HTTP / TLS</th>
              <th class="px-6 py-3 font-semibold">下载</th>
              <th class="px-6 py-3 font-semibold">导出</th>
              <th class="px-6 py-3 font-semibold">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="row in resultRows" :key="row.address" class="bg-white hover:bg-slate-50/80">
              <td class="px-6 py-4 font-mono text-sm text-slate-700">{{ row.address }}</td>
              <td class="px-6 py-4">
                <span :class="resultToneClass(row.stage_status)" class="ui-pill">
                  {{ resultStageStatusLabel(row.stage_status) }}
                </span>
                <span v-if="row.colo" class="ml-2 text-xs text-slate-400">{{ row.colo }}</span>
              </td>
              <td class="px-6 py-4 text-slate-600">{{ formatMetric(row.tcp_latency_ms) }}</td>
              <td class="px-6 py-4 text-slate-600">
                {{ formatMetric(row.http_latency_ms) }}
                <span class="text-xs text-slate-400">/ {{ formatMetric(row.tls_latency_ms) }}</span>
              </td>
              <td class="px-6 py-4 text-slate-600">{{ formatSpeed(row.download_mbps) }}</td>
              <td class="px-6 py-4 text-slate-600">{{ exportStatusLabel(row.export_status) }}</td>
              <td class="px-6 py-4">
                <div class="flex items-center gap-2">
                  <button type="button" class="ui-button ui-button-ghost px-3 py-2 text-xs" @click="$emit('copy-address', row.address)">
                    <PhCopy size="14" />
                    复制 IP
                  </button>
                  <button type="button" class="ui-button ui-button-secondary px-3 py-2 text-xs" :disabled="loading" @click="$emit('rerun-address', row.address)">
                    <PhRocketLaunch size="14" />
                    单条重测
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="resultRows.length === 0">
              <td colspan="7" class="px-6 py-10 text-center text-sm text-slate-400">
                当前还没有结果快照。启动任务后会自动填充。
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </article>

    <TaskProcessView :entries="processTrace" title="实时测试进程" @clear="$emit('clear-process')" />

    <div class="grid gap-6 xl:grid-cols-2">
      <article class="ui-card p-6">
        <div class="mb-4 flex items-center justify-between">
          <div>
            <h3 class="text-lg font-semibold text-slate-800">最近活动</h3>
            <p class="mt-1 text-sm text-slate-500">关键状态变化会在这里滚动保留。</p>
          </div>
          <span class="ui-pill ui-pill-subtle">{{ activityFeed.length }} 条</span>
        </div>

        <ul class="space-y-3">
          <li v-for="entry in activityFeed" :key="`${entry.ts}-${entry.title}`" class="rounded-2xl border border-slate-100 bg-slate-50/70 p-4">
            <p class="font-semibold text-slate-800">{{ entry.title }}</p>
            <p class="mt-1 text-sm text-slate-500">{{ entry.detail }}</p>
            <p class="mt-2 text-xs text-slate-400">{{ entry.ts }}</p>
          </li>
          <li v-if="activityFeed.length === 0" class="rounded-2xl border border-dashed border-slate-200 p-6 text-center text-sm text-slate-400">
            当前还没有活动记录。
          </li>
        </ul>
      </article>

      <article class="ui-card p-6">
        <div class="mb-4 flex items-center justify-between">
          <div>
            <h3 class="text-lg font-semibold text-slate-800">最近导出</h3>
            <p class="mt-1 text-sm text-slate-500">保留最新的导出路径、数量和失败摘要。</p>
          </div>
          <span class="ui-pill ui-pill-subtle">{{ exportHistory.length }} 条</span>
        </div>

        <ul class="space-y-3">
          <li v-for="entry in exportHistory" :key="entry.taskId" class="rounded-2xl border border-slate-100 bg-slate-50/70 p-4">
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0">
                <p class="font-semibold text-slate-800">{{ entry.title }}</p>
                <p class="mt-1 text-sm text-slate-500">{{ entry.detail }}</p>
                <p class="mt-2 truncate text-xs text-slate-400">任务 {{ entry.taskId }} · {{ entry.updatedAt }}</p>
                <p v-if="entry.failureSummary" class="mt-1 text-xs text-amber-600">异常摘要：{{ entry.failureSummary }}</p>
              </div>
              <button
                type="button"
                class="ui-button ui-button-ghost shrink-0 px-3 py-2 text-xs"
                :disabled="!entry.targetPath"
                @click="$emit('open-history-target', entry.targetPath)"
              >
                打开路径
              </button>
            </div>
          </li>
          <li v-if="exportHistory.length === 0" class="rounded-2xl border border-dashed border-slate-200 p-6 text-center text-sm text-slate-400">
            当前还没有导出记录。
          </li>
        </ul>
      </article>
    </div>

    <div class="rounded-2xl border border-slate-200 bg-white/80 px-5 py-4 text-sm text-slate-500 shadow-sm">
      <span v-if="lastHistoryEntry">最近一次导出：{{ lastHistoryEntry.title }}，路径 {{ lastHistoryEntry.targetPath || "尚未生成" }}。</span>
      <span v-else>提示：{{ probeWarnings[0] || "当前没有额外提示。" }}</span>
    </div>
  </section>

  <section v-else class="space-y-4">
    <article class="ui-card flex items-center justify-between p-4">
      <div>
        <p class="text-xs font-medium text-slate-500">当前任务状态</p>
        <div class="mt-1 flex items-center">
          <span :class="toneDotClass(statusTone)" class="mr-2 h-3 w-3 rounded-full"></span>
          <strong class="text-xl font-bold text-slate-800">{{ statusLabel }}</strong>
        </div>
      </div>
      <div class="text-right">
        <p class="text-xs font-medium text-slate-500">处理进度</p>
        <p class="text-xl font-bold text-slate-800">{{ progressPercent }}%</p>
      </div>
    </article>

    <div class="grid grid-cols-2 gap-3">
      <article class="ui-card p-4">
        <p class="text-xs font-medium text-slate-500">总计</p>
        <strong class="mt-2 block text-2xl font-bold text-slate-800">{{ summary.accepted || "-" }}</strong>
      </article>
      <article class="ui-card p-4">
        <p class="text-xs font-medium text-slate-500">已处理</p>
        <strong class="mt-2 block text-2xl font-bold text-primary">{{ summary.processed }}</strong>
      </article>
      <article class="ui-card p-4">
        <p class="text-xs font-medium text-slate-500">有效结果</p>
        <strong class="mt-2 block text-2xl font-bold text-emerald-500">{{ summary.passed }}</strong>
      </article>
      <article class="ui-card p-4">
        <p class="text-xs font-medium text-slate-500">失败结果</p>
        <strong class="mt-2 block text-2xl font-bold text-rose-500">{{ summary.failed }}</strong>
      </article>
    </div>

    <article class="ui-card p-4">
      <div class="mb-4 h-3 overflow-hidden rounded-full bg-slate-100">
        <div class="h-full rounded-full bg-primary transition-all duration-300" :style="{ width: `${progressPercent}%` }"></div>
      </div>
      <div class="flex gap-3">
        <button type="button" class="ui-button ui-button-primary h-12 flex-1" :disabled="loading" @click="$emit('start')">
          <PhPlay size="18" weight="fill" />
          开始探测
        </button>
        <button type="button" class="ui-button ui-button-warning h-12 flex-1" :disabled="loading || !hasActiveTask" @click="$emit('pause')">
          <PhPause size="18" weight="fill" />
          暂停
        </button>
        <button type="button" class="ui-button ui-button-success h-12 flex-1" :disabled="loading || !task.taskId" @click="$emit('resume')">
          <PhPlayCircle size="18" weight="fill" />
          恢复
        </button>
      </div>
    </article>

    <TaskProcessView :entries="processTrace" mobile title="实时测试进程" @clear="$emit('clear-process')" />

    <article class="ui-card p-4 text-sm text-slate-500">
      <div class="flex items-start gap-2">
        <PhWarningCircle class="mt-0.5 text-amber-500" size="18" />
        <p>{{ probeWarnings[0] || "移动端保留核心任务态、日志与控制操作，完整结果表可在桌面宽度查看。" }}</p>
      </div>
    </article>
  </section>
</template>
