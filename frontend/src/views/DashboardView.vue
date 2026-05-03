<script setup lang="ts">
import {
  PhActivity,
  PhPause,
  PhPlay,
  PhPlayCircle,
  PhWarningCircle,
} from "@phosphor-icons/vue";
import type { TaskTone } from "../lib/bridge";
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
  statusLabel: string;
  statusTone: TaskTone;
  summary: SummaryStats;
  task: TaskState;
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

  if (tone === "cooling") {
    return "bg-amber-400";
  }

  if (tone === "running" || tone === "preparing") {
    return "bg-primary";
  }

  return "bg-slate-400";
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
          <p class="mt-1 text-sm text-slate-500">实时展示 IP池、TCP测延迟、追踪探测、文件测速、导出与失败节点。</p>
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
            继续任务
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
          继续
        </button>
      </div>
    </article>

    <TaskProcessView :entries="processTrace" mobile title="实时测试进程" @clear="$emit('clear-process')" />

    <article class="ui-card p-4 text-sm text-slate-500">
      <div class="flex items-start gap-2">
        <PhWarningCircle class="mt-0.5 text-amber-500" size="18" />
        <p>{{ probeWarnings[0] || "当前结果已移到“结果”页面，移动端也可以直接查看本次测速结果和导出位置。" }}</p>
      </div>
    </article>
  </section>
</template>
