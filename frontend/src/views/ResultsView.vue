<script setup lang="ts">
import {
  PhArrowClockwise,
  PhCopy,
  PhFileCsv,
  PhRocketLaunch,
  PhTable,
} from "@phosphor-icons/vue";
import type {
  ProbeResult,
  ProbeResultFilter,
  ProbeResultIPFilter,
  ProbeResultOrder,
  ProbeResultSortBy,
  TaskSnapshot,
} from "../lib/bridge";

interface SummaryStats {
  exported: number;
  failed: number;
  passed: number;
  processed: number;
  total: number;
}

interface TaskState {
  exportPath: string;
  stage: string;
  taskId: string;
}

defineProps<{
  csvExporting: boolean;
  githubExporting: boolean;
  hasActiveTask: boolean;
  loading: boolean;
  platform: "desktop" | "mobile";
  resultFilter: ProbeResultFilter;
  resultFilterOptions: Array<{ label: string; value: ProbeResultFilter }>;
  resultIpFilter: ProbeResultIPFilter;
  resultIpFilterOptions: Array<{ label: string; value: ProbeResultIPFilter }>;
  resultOrder: ProbeResultOrder;
  resultRows: ProbeResult[];
  resultSortBy: ProbeResultSortBy;
  resultSortOptions: Array<{ label: string; value: ProbeResultSortBy }>;
  resultsLoading: boolean;
  summary: SummaryStats;
  task: TaskState;
  taskSnapshot: TaskSnapshot | null;
}>();

const emit = defineEmits<{
  (event: "copy-address", address: string): void;
  (event: "export-current-results-csv"): void;
  (event: "export-github"): void;
  (event: "refresh-results"): void;
  (event: "rerun-address", address: string): void;
  (event: "update-filter", filter: ProbeResultFilter): void;
  (event: "update-ip-filter", filter: ProbeResultIPFilter): void;
  (event: "update-order", order: ProbeResultOrder): void;
  (event: "update-sort", sortBy: ProbeResultSortBy): void;
}>();

function resultToneClass(stageStatus: string) {
  if (stageStatus.includes("failed")) {
    return "bg-rose-50 text-rose-700";
  }

  if (stageStatus.includes("passed") || stageStatus === "completed") {
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
    stage0_pool: "IP池",
    stage1_tcp: "TCP测延迟",
    stage2_head: "追踪探测",
    stage2_trace: "追踪探测",
    stage3_get: "文件测速",
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

  if (status === "completed") {
    return "已完成";
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
  return typeof value === "number" && Number.isFinite(value) ? `${value.toFixed(2)}${suffix}` : "-";
}

function formatSpeed(value: number | null | undefined) {
  return typeof value === "number" && Number.isFinite(value) ? `${value.toFixed(2)} MB/s` : "-";
}

function formatPort(value: number | null | undefined) {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? String(value) : "-";
}

function taskContextNumber(snapshot: TaskSnapshot | null, key: string) {
  const value = snapshot?.task_context?.[key];
  const numeric = Number(value);
  return Number.isFinite(numeric) && numeric > 0 ? numeric : null;
}

function taskContextPorts(snapshot: TaskSnapshot | null) {
  const value = snapshot?.task_context?.source_port_values;
  if (!Array.isArray(value)) {
    return [];
  }
  return value.map((entry) => Number(entry)).filter((entry) => Number.isFinite(entry) && entry > 0);
}

function taskGroupedPorts(snapshot: TaskSnapshot | null) {
  const value = snapshot?.task_context?.grouped_ports;
  if (!Array.isArray(value)) {
    return [];
  }
  return value.map((entry) => Number(entry)).filter((entry) => Number.isFinite(entry) && entry > 0);
}

function taskCurrentPortLabel(snapshot: TaskSnapshot | null) {
  const policy = String(snapshot?.task_context?.port_policy || "").trim();
  const currentPort = taskContextNumber(snapshot, "current_test_port");
  if (currentPort) {
    return String(currentPort);
  }
  const groupedPorts = taskGroupedPorts(snapshot);
  if (groupedPorts.length > 1) {
    return `按端口分组 ${groupedPorts.join(" / ")}`;
  }
  if (groupedPorts.length === 1) {
    return String(groupedPorts[0]);
  }
  if (policy === "source_override_global" && taskContextPorts(snapshot).length > 0) {
    return `源端口 ${taskContextPorts(snapshot).join(" / ")}`;
  }
  const globalPort = taskContextNumber(snapshot, "global_tcp_port");
  return globalPort ? String(globalPort) : "-";
}

function onFilterChange(event: Event) {
  emit("update-filter", (event.target as HTMLSelectElement).value as ProbeResultFilter);
}

function onIpFilterChange(event: Event) {
  emit("update-ip-filter", (event.target as HTMLSelectElement).value as ProbeResultIPFilter);
}

function onSortChange(event: Event) {
  emit("update-sort", (event.target as HTMLSelectElement).value as ProbeResultSortBy);
}

function onOrderChange(event: Event) {
  emit("update-order", (event.target as HTMLSelectElement).value as ProbeResultOrder);
}
</script>

<template>
  <section v-if="platform === 'desktop'" class="space-y-5">
    <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
      <article class="ui-card p-4">
        <p class="text-sm font-medium text-slate-500">当前结果</p>
        <strong class="mt-2 block text-xl font-bold text-slate-800">{{ resultRows.length }}</strong>
        <p class="mt-1 text-xs text-slate-400">只保留本次测速快照</p>
      </article>
      <article class="ui-card p-4">
        <p class="text-sm font-medium text-slate-500">有效结果</p>
        <strong class="mt-2 block text-xl font-bold text-emerald-600">{{ summary.passed || summary.exported }}</strong>
        <p class="mt-1 text-xs text-slate-400">已导出 {{ summary.exported }}</p>
      </article>
      <article class="ui-card p-4">
        <p class="text-sm font-medium text-slate-500">处理进度</p>
        <strong class="mt-2 block text-xl font-bold text-slate-800">{{ summary.processed }} / {{ summary.total || "-" }}</strong>
        <p class="mt-1 text-xs text-slate-400">失败 {{ summary.failed }}</p>
      </article>
      <article class="ui-card min-w-0 p-4">
        <p class="text-sm font-medium text-slate-500">导出位置</p>
        <p class="mt-2 truncate font-mono text-xs text-slate-700">{{ task.exportPath || "尚未导出" }}</p>
        <p class="mt-1 text-xs text-slate-400">Android 会显示系统文件 URI</p>
      </article>
    </div>

    <article class="ui-card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-slate-50/70 px-5 py-3">
        <div class="min-w-0">
          <h3 class="flex items-center text-base font-semibold text-slate-800">
            <PhTable class="mr-2 text-primary" size="20" />
            当前测速结果
          </h3>
          <p class="mt-1 text-sm text-slate-500">新任务或单条重测会清空这里，仅展示当前任务结果。</p>
        </div>

        <div class="flex min-w-0 flex-wrap items-center justify-end gap-2">
          <label class="min-w-0 text-sm text-slate-500">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">状态</span>
            <select class="ui-field min-w-28" :value="resultFilter" @change="onFilterChange">
              <option v-for="option in resultFilterOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
          <label class="min-w-0 text-sm text-slate-500">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">IP 版本</span>
            <select class="ui-field min-w-28" :value="resultIpFilter" @change="onIpFilterChange">
              <option v-for="option in resultIpFilterOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
          <label class="min-w-0 text-sm text-slate-500">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">排序</span>
            <select class="ui-field min-w-28" :value="resultSortBy" @change="onSortChange">
              <option v-for="option in resultSortOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
          <label class="min-w-0 text-sm text-slate-500">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">方向</span>
            <select class="ui-field min-w-24" :value="resultOrder" @change="onOrderChange">
              <option value="asc">升序</option>
              <option value="desc">降序</option>
            </select>
          </label>
          <button type="button" class="ui-button ui-button-ghost whitespace-nowrap" :disabled="!task.taskId || resultsLoading" @click="$emit('refresh-results')">
            <PhArrowClockwise size="16" />
            {{ resultsLoading ? "刷新中" : "刷新" }}
          </button>
          <button
            type="button"
            class="ui-button ui-button-secondary whitespace-nowrap"
            :disabled="loading || csvExporting || githubExporting || hasActiveTask || resultRows.length === 0"
            @click="$emit('export-current-results-csv')"
          >
            <PhFileCsv size="16" />
            {{ csvExporting ? "导出中" : "CSV" }}
          </button>
          <button
            type="button"
            class="ui-button ui-button-ghost whitespace-nowrap"
            :disabled="loading || csvExporting || githubExporting || hasActiveTask || resultRows.length === 0"
            @click="$emit('export-github')"
          >
            <PhFileCsv size="16" />
            {{ githubExporting ? "导出中" : "GitHub" }}
          </button>
        </div>
      </div>

      <div class="flex flex-wrap gap-2 border-b border-slate-200 px-5 py-2.5 text-xs text-slate-500">
        <span class="overflow-safe">任务：{{ task.taskId || "等待中" }}</span>
        <span class="overflow-safe">状态：{{ taskStatusLabel(taskSnapshot?.status) }}</span>
        <span class="overflow-safe">阶段：{{ taskStatusLabel(taskSnapshot?.current_stage || task.stage) }}</span>
        <span class="overflow-safe">全局端口：{{ taskContextNumber(taskSnapshot, "global_tcp_port") || "-" }}</span>
        <span class="overflow-safe">源端口：{{ taskContextPorts(taskSnapshot).join(" / ") || "未指定" }}</span>
        <span class="overflow-safe">实际测速端口：{{ taskCurrentPortLabel(taskSnapshot) }}</span>
        <span class="overflow-safe">更新：{{ taskSnapshot?.updated_at || "-" }}</span>
      </div>

      <div class="table-scroll">
        <table class="min-w-[64rem] text-sm">
          <thead class="bg-slate-50 text-left text-slate-500">
            <tr>
              <th class="px-4 py-2.5 font-semibold">IP 地址</th>
              <th class="px-4 py-2.5 font-semibold">输入源端口</th>
              <th class="px-4 py-2.5 font-semibold">实际测速端口</th>
              <th class="px-4 py-2.5 font-semibold">阶段状态</th>
              <th class="px-4 py-2.5 font-semibold">TCP</th>
              <th class="px-4 py-2.5 font-semibold">追踪</th>
              <th class="px-4 py-2.5 font-semibold">平均速率</th>
              <th class="px-4 py-2.5 font-semibold">最高速率</th>
              <th class="px-4 py-2.5 font-semibold">导出</th>
              <th class="px-4 py-2.5 font-semibold">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="row in resultRows" :key="row.address" class="bg-white hover:bg-slate-50/80">
              <td class="max-w-[11rem] truncate px-4 py-3 font-mono text-xs text-slate-700">{{ row.address }}</td>
              <td class="whitespace-nowrap px-4 py-3 font-mono text-xs text-slate-600">{{ formatPort(row.source_port) }}</td>
              <td class="whitespace-nowrap px-4 py-3 font-mono text-xs text-slate-600">{{ formatPort(row.test_port) }}</td>
              <td class="whitespace-nowrap px-4 py-3">
                <span :class="resultToneClass(row.stage_status)" class="ui-pill">
                  {{ resultStageStatusLabel(row.stage_status) }}
                </span>
                <span v-if="row.colo" class="ml-2 text-xs text-slate-400">{{ row.colo }}</span>
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ formatMetric(row.tcp_latency_ms) }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ formatMetric(row.trace_latency_ms) }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ formatSpeed(row.download_mbps) }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ formatSpeed(row.max_download_mbps) }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ exportStatusLabel(row.export_status) }}</td>
              <td class="px-4 py-3">
                <div class="flex items-center gap-1.5 whitespace-nowrap">
                  <button type="button" class="ui-button ui-button-ghost px-2.5 py-1.5 text-xs" @click="$emit('copy-address', row.address)">
                    <PhCopy size="14" />
                    复制
                  </button>
                  <button type="button" class="ui-button ui-button-secondary px-2.5 py-1.5 text-xs" :disabled="loading || hasActiveTask" @click="$emit('rerun-address', row.address)">
                    <PhRocketLaunch size="14" />
                    重测
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="resultRows.length === 0">
              <td colspan="10" class="px-4 py-8 text-center text-sm text-slate-400">
                当前还没有结果快照。启动任务后会自动填充。
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </article>
  </section>

  <section v-else class="space-y-4">
    <article class="ui-card p-4">
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <div class="flex items-center">
            <PhFileCsv class="mr-2 text-primary" size="18" />
            <h3 class="text-sm font-semibold text-slate-800">当前测速结果</h3>
          </div>
          <p class="mt-2 truncate font-mono text-xs text-slate-500">{{ task.exportPath || "尚未导出" }}</p>
        </div>
        <div class="flex shrink-0 flex-col gap-2 sm:flex-row">
          <button type="button" class="ui-button ui-button-ghost px-3 py-2 text-xs" :disabled="!task.taskId || resultsLoading" @click="$emit('refresh-results')">
            <PhArrowClockwise size="14" />
            {{ resultsLoading ? "刷新中" : "刷新" }}
          </button>
          <button
            type="button"
            class="ui-button ui-button-secondary px-3 py-2 text-xs"
            :disabled="loading || csvExporting || githubExporting || hasActiveTask || resultRows.length === 0"
            @click="$emit('export-current-results-csv')"
          >
            <PhFileCsv size="14" />
            {{ csvExporting ? "导出中" : "CSV" }}
          </button>
          <button
            type="button"
            class="ui-button ui-button-ghost px-3 py-2 text-xs"
            :disabled="loading || csvExporting || githubExporting || hasActiveTask || resultRows.length === 0"
            @click="$emit('export-github')"
          >
            <PhFileCsv size="14" />
            {{ githubExporting ? "导出中" : "GitHub" }}
          </button>
        </div>
      </div>

      <div class="mt-4 grid grid-cols-3 gap-3 text-center">
        <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
          <p class="text-xs text-slate-500">结果</p>
          <strong class="mt-1 block text-xl text-slate-800">{{ resultRows.length }}</strong>
        </div>
        <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
          <p class="text-xs text-slate-500">通过</p>
          <strong class="mt-1 block text-xl text-emerald-600">{{ summary.passed }}</strong>
        </div>
        <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
          <p class="text-xs text-slate-500">失败</p>
          <strong class="mt-1 block text-xl text-rose-500">{{ summary.failed }}</strong>
        </div>
      </div>
    </article>

    <article class="ui-card p-4">
      <div class="grid gap-3">
        <div class="grid grid-cols-2 gap-3">
          <label>
            <span class="ui-label">状态</span>
            <select class="ui-field" :value="resultFilter" @change="onFilterChange">
              <option v-for="option in resultFilterOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
          <label>
            <span class="ui-label">IP 版本</span>
            <select class="ui-field" :value="resultIpFilter" @change="onIpFilterChange">
              <option v-for="option in resultIpFilterOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <label>
            <span class="ui-label">排序</span>
            <select class="ui-field" :value="resultSortBy" @change="onSortChange">
              <option v-for="option in resultSortOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
          <label>
            <span class="ui-label">方向</span>
            <select class="ui-field" :value="resultOrder" @change="onOrderChange">
              <option value="asc">升序</option>
              <option value="desc">降序</option>
            </select>
          </label>
        </div>
      </div>
    </article>

    <div v-if="resultRows.length === 0" class="ui-card p-8 text-center text-sm text-slate-400">
      当前还没有结果快照。启动任务后会自动填充。
    </div>

    <div v-else class="space-y-3">
      <article v-for="row in resultRows" :key="row.address" class="ui-card p-4">
        <div class="flex items-start justify-between gap-3">
          <div class="overflow-safe">
            <p class="truncate font-mono text-base font-semibold text-slate-800">{{ row.address }}</p>
            <div class="mt-2 flex flex-wrap items-center gap-2">
              <span :class="resultToneClass(row.stage_status)" class="ui-pill">
                {{ resultStageStatusLabel(row.stage_status) }}
              </span>
              <span v-if="row.colo" class="ui-pill ui-pill-subtle">{{ row.colo }}</span>
              <span class="ui-pill ui-pill-subtle">源端口 {{ formatPort(row.source_port) }}</span>
              <span class="ui-pill ui-pill-subtle">测速端口 {{ formatPort(row.test_port) }}</span>
              <span class="ui-pill ui-pill-subtle">{{ exportStatusLabel(row.export_status) }}</span>
            </div>
          </div>
          <button type="button" class="ui-button ui-button-ghost shrink-0 px-3 py-2 text-xs" @click="$emit('copy-address', row.address)">
            <PhCopy size="14" />
            复制
          </button>
        </div>

        <div class="mt-4 grid grid-cols-2 gap-2 text-xs text-slate-500 sm:grid-cols-4">
          <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
            <p>TCP</p>
            <strong class="mt-1 block text-sm text-slate-800">{{ formatMetric(row.tcp_latency_ms) }}</strong>
          </div>
          <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
            <p>追踪</p>
            <strong class="mt-1 block text-sm text-slate-800">{{ formatMetric(row.trace_latency_ms) }}</strong>
          </div>
          <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
            <p>平均速率</p>
            <strong class="mt-1 block text-sm text-slate-800">{{ formatSpeed(row.download_mbps) }}</strong>
          </div>
          <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
            <p>最高速率</p>
            <strong class="mt-1 block text-sm text-slate-800">{{ formatSpeed(row.max_download_mbps) }}</strong>
          </div>
        </div>

        <button type="button" class="ui-button ui-button-secondary mt-4 h-11 w-full" :disabled="loading || hasActiveTask" @click="$emit('rerun-address', row.address)">
          <PhRocketLaunch size="16" />
          单条重测
        </button>
      </article>
    </div>
  </section>
</template>
