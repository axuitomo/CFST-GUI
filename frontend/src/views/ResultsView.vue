<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef, watch } from "vue";
import { PhArrowClockwise, PhCaretDown, PhCheck, PhCloud, PhCopy, PhFileCsv, PhRocketLaunch, PhTable } from "@phosphor-icons/vue";
import type { ProbeResult, ProbeResultFilter, ProbeResultIPFilter, ProbeResultOrder, ProbeResultSortBy, TaskSnapshot } from "../lib/bridge";

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

interface TimestampFormatOptions {
  fallback?: string;
  includeDate?: boolean;
  includeOffset?: boolean;
  includeSeconds?: boolean;
}

type CloudflarePushRecordType = "ALL" | "A" | "AAAA";
type MobilePickerKey = "filter" | "ip" | "sort" | "order";
type MobilePickerValue = ProbeResultFilter | ProbeResultIPFilter | ProbeResultSortBy | ProbeResultOrder;

interface CloudflarePushSettings {
  recordName: string;
  recordType: CloudflarePushRecordType;
  topN: number;
}

const props = defineProps<{
  canRerunTask: boolean;
  cloudflarePushing: boolean;
  cloudflarePushSettings: CloudflarePushSettings;
  cloudflareRoutingActive: boolean;
  cloudflareRoutingRuleCount: number;
  csvExporting: boolean;
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  githubExporting: boolean;
  githubTopN: number;
  hasActiveTask: boolean;
  loading: boolean;
  platform: "desktop" | "mobile";
  resultFilter: ProbeResultFilter;
  resultFilterOptions: Array<{ label: string; value: ProbeResultFilter }>;
  resultIpFilter: ProbeResultIPFilter;
  resultIpFilterOptions: Array<{ label: string; value: ProbeResultIPFilter }>;
  resultOrder: ProbeResultOrder;
  resultRows: ProbeResult[];
  resultsTotalCount?: number;
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
  (event: "push-cloudflare"): void;
  (event: "refresh-results"): void;
  (event: "load-more-results"): void;
  (event: "rerun-address", address: string): void;
  (event: "update-cloudflare-push-settings", settings: Partial<CloudflarePushSettings>): void;
  (event: "update-github-top-n", value: number): void;
  (event: "update-filter", filter: ProbeResultFilter): void;
  (event: "update-ip-filter", filter: ProbeResultIPFilter): void;
  (event: "update-order", order: ProbeResultOrder): void;
  (event: "update-sort", sortBy: ProbeResultSortBy): void;
}>();

const cloudflarePanelOpen = ref(false);
const mobilePickerOpen = ref<MobilePickerKey | "">("");
const mobilePickerRoot = useTemplateRef<HTMLElement>("mobilePickerRoot");
const resultActionDisabled = computed(() => props.loading || props.csvExporting || props.githubExporting || props.cloudflarePushing || props.hasActiveTask || props.resultRows.length === 0);
const cloudflarePushDisabled = computed(() => resultActionDisabled.value || (!props.cloudflareRoutingActive && !props.cloudflarePushSettings.recordName.trim()));
const cloudflarePushPreviewCount = computed(() => {
  const topN = Math.max(0, Number(props.cloudflarePushSettings.topN) || 0);
  return topN > 0 ? Math.min(topN, props.resultRows.length) : props.resultRows.length;
});
const orderOptions: Array<{ label: string; value: ProbeResultOrder }> = [
  { label: "升序", value: "asc" },
  { label: "降序", value: "desc" },
];

const mobileFilterLabel = computed(() => optionLabel(props.resultFilterOptions, props.resultFilter));
const mobileIpFilterLabel = computed(() => optionLabel(props.resultIpFilterOptions, props.resultIpFilter));
const mobileSortLabel = computed(() => optionLabel(props.resultSortOptions, props.resultSortBy));
const mobileOrderLabel = computed(() => optionLabel(orderOptions, props.resultOrder));

function optionLabel<T extends string>(options: Array<{ label: string; value: T }>, value: T) {
  return options.find((option) => option.value === value)?.label || value;
}

function toggleMobilePicker(key: MobilePickerKey) {
  mobilePickerOpen.value = mobilePickerOpen.value === key ? "" : key;
}

function closeMobilePicker() {
  mobilePickerOpen.value = "";
}

function mobilePickerMenuId(key: MobilePickerKey) {
  return `mobile-result-picker-${key}`;
}

function updateMobilePicker(key: MobilePickerKey, value: MobilePickerValue) {
  closeMobilePicker();
  if (key === "filter") {
    emit("update-filter", value as ProbeResultFilter);
    return;
  }
  if (key === "ip") {
    emit("update-ip-filter", value as ProbeResultIPFilter);
    return;
  }
  if (key === "sort") {
    emit("update-sort", value as ProbeResultSortBy);
    return;
  }
  emit("update-order", value as ProbeResultOrder);
}

function handleMobilePickerPointerDown(event: PointerEvent) {
  if (!mobilePickerOpen.value) {
    return;
  }
  const target = event.target;
  if (target instanceof Node && mobilePickerRoot.value?.contains(target)) {
    return;
  }
  closeMobilePicker();
}

function handleMobilePickerKeydown(event: KeyboardEvent) {
  if (event.key === "Escape") {
    closeMobilePicker();
  }
}

function inputValue(event: Event) {
  return (event.currentTarget as HTMLInputElement).value;
}

function selectValue(event: Event) {
  return (event.currentTarget as HTMLSelectElement).value;
}

function numberInputValue(event: Event) {
  const value = Number.parseInt(inputValue(event) || "0", 10);
  return Number.isFinite(value) ? value : 0;
}

function updateCloudflareRecordName(event: Event) {
  emit("update-cloudflare-push-settings", { recordName: inputValue(event) });
}

function updateCloudflareRecordType(event: Event) {
  emit("update-cloudflare-push-settings", { recordType: selectValue(event) as CloudflarePushRecordType });
}

function updateCloudflareTopN(event: Event) {
  emit("update-cloudflare-push-settings", { topN: numberInputValue(event) });
}

function updateGitHubTopN(event: Event) {
  emit("update-github-top-n", numberInputValue(event));
}

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
    recovery_required: "需要恢复",
    persisted_only: "仅恢复快照",
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

function formatTimestampLabel(value: string, options?: TimestampFormatOptions) {
  return props.formatTimestamp(value, options);
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
  emit("update-filter", selectValue(event) as ProbeResultFilter);
}

function onIpFilterChange(event: Event) {
  emit("update-ip-filter", selectValue(event) as ProbeResultIPFilter);
}

function onSortChange(event: Event) {
  emit("update-sort", selectValue(event) as ProbeResultSortBy);
}

function onOrderChange(event: Event) {
  emit("update-order", selectValue(event) as ProbeResultOrder);
}

const MOBILE_ROW_HEIGHT = 188;
const MOBILE_OVERSCAN = 6;
const mobileScrollContainer = useTemplateRef<HTMLDivElement>("mobileScrollContainer");
const mobileScrollTop = ref(0);

const visibleMobileRows = computed(() => {
  if (props.platform !== "mobile") {
    return props.resultRows.map((row, index) => ({ row, index }));
  }
  const start = Math.max(0, Math.floor(mobileScrollTop.value / MOBILE_ROW_HEIGHT) - MOBILE_OVERSCAN);
  const windowSize = 12 + MOBILE_OVERSCAN * 2;
  return props.resultRows.slice(start, start + windowSize).map((row, index) => ({ row, index: start + index }));
});

const mobileWindowOffset = computed(() => {
  if (props.platform !== "mobile") {
    return 0;
  }
  return Math.max(0, Math.floor(mobileScrollTop.value / MOBILE_ROW_HEIGHT) - MOBILE_OVERSCAN) * MOBILE_ROW_HEIGHT;
});

const mobileTotalHeight = computed(() => props.resultRows.length * MOBILE_ROW_HEIGHT);
const showRecoveringState = computed(() => props.platform === "mobile" && props.resultRows.length === 0 && props.resultsLoading && ["completed", "cooling", "no_results"].includes(props.taskSnapshot?.status || props.task.stage));

function onMobileScroll(event: Event) {
  mobileScrollTop.value = (event.currentTarget as HTMLDivElement).scrollTop || 0;
  closeMobilePicker();
}

function mobileRowKey(row: ProbeResult, index: number) {
  return [row.address, row.source_port ?? "", row.test_port ?? "", row.stage_status || "", index].join("|");
}

function mobileRowsSignature() {
  const first = props.resultRows[0];
  const last = props.resultRows[props.resultRows.length - 1];
  return [props.task.taskId, props.resultFilter, props.resultIpFilter, props.resultSortBy, props.resultOrder, props.resultRows.length, first ? mobileRowKey(first, 0) : "", last ? mobileRowKey(last, props.resultRows.length - 1) : ""].join("::");
}

watch(mobileRowsSignature, async () => {
  if (props.platform !== "mobile") {
    return;
  }
  mobileScrollTop.value = 0;
  await nextTick();
  if (mobileScrollContainer.value) {
    mobileScrollContainer.value.scrollTop = 0;
  }
});

watch(
  () => [props.platform, props.resultRows.length, mobileTotalHeight.value] as const,
  async () => {
    if (props.platform !== "mobile") {
      return;
    }
    await nextTick();
    const container = mobileScrollContainer.value;
    if (!container) {
      return;
    }
    const maxScrollTop = Math.max(0, mobileTotalHeight.value - container.clientHeight);
    if (mobileScrollTop.value > maxScrollTop) {
      mobileScrollTop.value = maxScrollTop;
      container.scrollTop = maxScrollTop;
    }
  },
);

onMounted(() => {
  document.addEventListener("pointerdown", handleMobilePickerPointerDown, true);
  document.addEventListener("keydown", handleMobilePickerKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", handleMobilePickerPointerDown, true);
  document.removeEventListener("keydown", handleMobilePickerKeydown);
});
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
          <button type="button" class="ui-button ui-button-secondary whitespace-nowrap" :disabled="resultActionDisabled" @click="$emit('export-current-results-csv')">
            <PhFileCsv size="16" />
            {{ csvExporting ? "导出中" : "CSV" }}
          </button>
          <button type="button" class="ui-button ui-button-ghost whitespace-nowrap" :disabled="resultActionDisabled" @click="$emit('export-github')">
            <PhFileCsv size="16" />
            {{ githubExporting ? "导出中" : "GitHub" }}
          </button>
          <label class="min-w-24 text-sm text-slate-500">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">GitHub 上限</span>
            <input :value="githubTopN" min="0" type="number" class="ui-field" @input="updateGitHubTopN" />
          </label>
          <button type="button" class="ui-button ui-button-ghost whitespace-nowrap" :disabled="resultActionDisabled" @click="cloudflarePanelOpen = !cloudflarePanelOpen">
            <PhCloud size="16" weight="fill" />
            {{ cloudflarePushing ? "推送中" : "Cloudflare" }}
          </button>
        </div>
      </div>

      <div v-if="cloudflarePanelOpen" class="cf-push-panel grid gap-4 border-b px-5 py-4 md:grid-cols-[minmax(0,1fr)_9rem_8rem_auto] md:items-end">
        <div v-if="cloudflareRoutingActive" class="cf-push-routing md:col-span-3 rounded-lg border px-4 py-3 text-sm">
          <span class="cf-push-title block font-semibold">CF 目标与分流推送</span>
          <span class="cf-push-copy mt-1 block text-xs">会先推送设置页主记录，再按 {{ cloudflareRoutingRuleCount }} 条启用分流规则继续推送；共享上传筛选、Cloudflare Top N 和规则 Top N 控制最终数量。</span>
        </div>
        <template v-else>
          <label>
            <span class="ui-label">Cloudflare 记录名称</span>
            <input :value="cloudflarePushSettings.recordName" type="text" class="ui-field font-mono" placeholder="edge.example.com" @input="updateCloudflareRecordName" />
          </label>
          <label>
            <span class="ui-label">记录类型</span>
            <select :value="cloudflarePushSettings.recordType" class="ui-field" @change="updateCloudflareRecordType">
              <option value="ALL">ALL / A + AAAA</option>
              <option value="A">A / IPv4</option>
              <option value="AAAA">AAAA / IPv6</option>
            </select>
          </label>
        </template>
        <label>
          <span class="ui-label">Cloudflare 上限</span>
          <input :value="cloudflarePushSettings.topN" min="0" type="number" class="ui-field" @input="updateCloudflareTopN" />
        </label>
        <button type="button" class="ui-button ui-button-primary whitespace-nowrap" :disabled="cloudflarePushDisabled" @click="$emit('push-cloudflare')">
          <PhCloud size="16" weight="fill" />
          {{ cloudflarePushing ? "推送中" : cloudflareRoutingActive ? "推送目标" : "推送" }}
        </button>
        <p class="cf-push-copy md:col-span-4 text-xs">
          <template v-if="cloudflareRoutingActive">Token、Zone ID、TTL 和目标记录名沿用设置页；主目标推送后，当前可见列表会继续交给分流规则筛选。</template>
          <template v-else>Token、Zone ID、TTL 沿用设置页；本面板只覆盖本次推送目标和数量。0 表示当前可见列表不限数量；本次预计推送 {{ cloudflarePushPreviewCount }} 条。</template>
        </p>
      </div>

      <div class="flex flex-wrap gap-2 border-b border-slate-200 px-5 py-2.5 text-xs text-slate-500">
        <span class="overflow-safe">任务：{{ task.taskId || "等待中" }}</span>
        <span class="overflow-safe">状态：{{ taskStatusLabel(taskSnapshot?.status) }}</span>
        <span class="overflow-safe">阶段：{{ taskStatusLabel(taskSnapshot?.current_stage || task.stage) }}</span>
        <span class="overflow-safe">会话：{{ taskStatusLabel(taskSnapshot?.session_state || "persisted_only") }}</span>
        <span class="overflow-safe">全局端口：{{ taskContextNumber(taskSnapshot, "global_tcp_port") || "-" }}</span>
        <span class="overflow-safe">源端口：{{ taskContextPorts(taskSnapshot).join(" / ") || "未指定" }}</span>
        <span class="overflow-safe">实际测速端口：{{ taskCurrentPortLabel(taskSnapshot) }}</span>
        <span class="overflow-safe">更新：{{ taskSnapshot?.updated_at ? formatTimestampLabel(taskSnapshot.updated_at) : "-" }}</span>
      </div>

      <div class="table-scroll desktop-data-table-scroll">
        <table class="desktop-data-table desktop-results-table text-sm">
          <thead class="text-left text-slate-500">
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
            <tr v-for="row in resultRows" :key="row.address" class="desktop-data-row">
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
                  <button type="button" class="ui-button ui-button-secondary px-2.5 py-1.5 text-xs" :disabled="loading || !canRerunTask" @click="$emit('rerun-address', row.address)">
                    <PhRocketLaunch size="14" />
                    重测
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="resultRows.length === 0">
              <td colspan="10" class="px-4 py-8 text-center text-sm text-slate-400">当前还没有结果快照。启动任务后会自动填充。</td>
            </tr>
          </tbody>
        </table>
      </div>
    </article>
  </section>

  <section v-else class="space-y-4">
    <article class="ui-card p-4">
      <div class="space-y-4">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0 flex-1">
            <div class="flex items-center">
              <PhFileCsv class="mr-2 text-primary" size="18" />
              <h3 class="text-sm font-semibold text-slate-800">当前测速结果</h3>
            </div>
            <p class="mt-2 break-all font-mono text-xs text-slate-500">{{ task.exportPath || "尚未导出" }}</p>
          </div>
          <button type="button" class="ui-button ui-button-ghost shrink-0 px-3 py-2 text-xs" :disabled="!task.taskId || resultsLoading" @click="$emit('refresh-results')">
            <PhArrowClockwise size="14" />
            {{ resultsLoading ? "刷新中" : "刷新" }}
          </button>
        </div>

        <div class="grid grid-cols-2 gap-2">
          <button type="button" class="ui-button ui-button-secondary w-full px-3 py-2 text-xs" :disabled="resultActionDisabled" @click="$emit('export-current-results-csv')">
            <PhFileCsv size="14" />
            {{ csvExporting ? "导出中" : "CSV" }}
          </button>
          <button type="button" class="ui-button ui-button-ghost w-full px-3 py-2 text-xs" :disabled="resultActionDisabled" @click="$emit('export-github')">
            <PhFileCsv size="14" />
            {{ githubExporting ? "导出中" : "GitHub" }}
          </button>
          <label class="min-w-0 text-xs text-slate-500">
            <span class="ui-label">GitHub 上限</span>
            <input :value="githubTopN" min="0" type="number" class="ui-field w-full px-2 py-2 text-xs" @input="updateGitHubTopN" />
          </label>
          <button type="button" class="ui-button ui-button-ghost w-full px-3 py-2 text-xs" :disabled="resultActionDisabled" @click="cloudflarePanelOpen = !cloudflarePanelOpen">
            <PhCloud size="14" weight="fill" />
            {{ cloudflarePushing ? "推送中" : "Cloudflare" }}
          </button>
        </div>
      </div>

      <div v-if="cloudflarePanelOpen" class="cf-push-panel mt-4 grid gap-3 rounded-xl border p-3">
        <div v-if="cloudflareRoutingActive" class="cf-push-routing rounded-lg border px-3 py-2 text-sm">
          <span class="cf-push-title block font-semibold">CF 目标与分流推送</span>
          <span class="cf-push-copy mt-1 block text-xs">先推送设置页主记录，再按 {{ cloudflareRoutingRuleCount }} 条启用分流规则继续推送。</span>
        </div>
        <template v-else>
          <label>
            <span class="ui-label">Cloudflare 记录名称</span>
            <input :value="cloudflarePushSettings.recordName" type="text" class="ui-field font-mono" placeholder="edge.example.com" @input="updateCloudflareRecordName" />
          </label>
          <div class="grid grid-cols-2 gap-3">
            <label>
              <span class="ui-label">记录类型</span>
              <select :value="cloudflarePushSettings.recordType" class="ui-field" @change="updateCloudflareRecordType">
                <option value="ALL">ALL</option>
                <option value="A">A</option>
                <option value="AAAA">AAAA</option>
              </select>
            </label>
          </div>
        </template>
        <label>
          <span class="ui-label">Cloudflare 上限</span>
          <input :value="cloudflarePushSettings.topN" min="0" type="number" class="ui-field" @input="updateCloudflareTopN" />
        </label>
        <button type="button" class="ui-button ui-button-primary w-full px-3 py-2 text-xs" :disabled="cloudflarePushDisabled" @click="$emit('push-cloudflare')">
          <PhCloud size="14" weight="fill" />
          {{ cloudflarePushing ? "推送中" : cloudflareRoutingActive ? "推送目标" : "推送 Cloudflare" }}
        </button>
        <p class="cf-push-copy text-xs">
          <template v-if="cloudflareRoutingActive">Token、Zone ID、TTL 和目标记录名沿用设置页；主目标推送后继续筛选分流规则。</template>
          <template v-else>Token、Zone ID、TTL 沿用设置页；仅覆盖本次目标和数量。0 表示不限数量；预计推送 {{ cloudflarePushPreviewCount }} 条。</template>
        </p>
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

    <article ref="mobilePickerRoot" class="ui-card p-4">
      <div class="grid gap-3">
        <div class="grid grid-cols-2 gap-3">
          <div class="relative">
            <span class="ui-label">状态</span>
            <button type="button" class="mobile-picker-button" aria-haspopup="listbox" :aria-controls="mobilePickerMenuId('filter')" :aria-expanded="mobilePickerOpen === 'filter'" @click="toggleMobilePicker('filter')">
              <span class="truncate">{{ mobileFilterLabel }}</span>
              <PhCaretDown class="shrink-0 text-slate-400" size="16" />
            </button>
            <div v-if="mobilePickerOpen === 'filter'" :id="mobilePickerMenuId('filter')" class="mobile-picker-menu" role="listbox">
              <button v-for="option in resultFilterOptions" :key="option.value" type="button" class="mobile-picker-option" role="option" :aria-selected="option.value === resultFilter" @click="updateMobilePicker('filter', option.value)">
                <span>{{ option.label }}</span>
                <PhCheck v-if="option.value === resultFilter" size="17" class="text-primary" />
              </button>
            </div>
          </div>
          <div class="relative">
            <span class="ui-label">IP 版本</span>
            <button type="button" class="mobile-picker-button" aria-haspopup="listbox" :aria-controls="mobilePickerMenuId('ip')" :aria-expanded="mobilePickerOpen === 'ip'" @click="toggleMobilePicker('ip')">
              <span class="truncate">{{ mobileIpFilterLabel }}</span>
              <PhCaretDown class="shrink-0 text-slate-400" size="16" />
            </button>
            <div v-if="mobilePickerOpen === 'ip'" :id="mobilePickerMenuId('ip')" class="mobile-picker-menu" role="listbox">
              <button v-for="option in resultIpFilterOptions" :key="option.value" type="button" class="mobile-picker-option" role="option" :aria-selected="option.value === resultIpFilter" @click="updateMobilePicker('ip', option.value)">
                <span>{{ option.label }}</span>
                <PhCheck v-if="option.value === resultIpFilter" size="17" class="text-primary" />
              </button>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div class="relative">
            <span class="ui-label">排序</span>
            <button type="button" class="mobile-picker-button" aria-haspopup="listbox" :aria-controls="mobilePickerMenuId('sort')" :aria-expanded="mobilePickerOpen === 'sort'" @click="toggleMobilePicker('sort')">
              <span class="truncate">{{ mobileSortLabel }}</span>
              <PhCaretDown class="shrink-0 text-slate-400" size="16" />
            </button>
            <div v-if="mobilePickerOpen === 'sort'" :id="mobilePickerMenuId('sort')" class="mobile-picker-menu" role="listbox">
              <button v-for="option in resultSortOptions" :key="option.value" type="button" class="mobile-picker-option" role="option" :aria-selected="option.value === resultSortBy" @click="updateMobilePicker('sort', option.value)">
                <span>{{ option.label }}</span>
                <PhCheck v-if="option.value === resultSortBy" size="17" class="text-primary" />
              </button>
            </div>
          </div>
          <div class="relative">
            <span class="ui-label">方向</span>
            <button type="button" class="mobile-picker-button" aria-haspopup="listbox" :aria-controls="mobilePickerMenuId('order')" :aria-expanded="mobilePickerOpen === 'order'" @click="toggleMobilePicker('order')">
              <span class="truncate">{{ mobileOrderLabel }}</span>
              <PhCaretDown class="shrink-0 text-slate-400" size="16" />
            </button>
            <div v-if="mobilePickerOpen === 'order'" :id="mobilePickerMenuId('order')" class="mobile-picker-menu" role="listbox">
              <button v-for="option in orderOptions" :key="option.value" type="button" class="mobile-picker-option" role="option" :aria-selected="option.value === resultOrder" @click="updateMobilePicker('order', option.value)">
                <span>{{ option.label }}</span>
                <PhCheck v-if="option.value === resultOrder" size="17" class="text-primary" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </article>

    <div v-if="resultRows.length === 0" class="ui-card p-8 text-center text-sm text-slate-400">
      {{ showRecoveringState ? "正在恢复测速结果，请稍候…" : "当前还没有结果快照。启动任务后会自动填充。" }}
    </div>

    <div v-else class="space-y-3">
      <div class="rounded-xl border border-slate-200 bg-slate-50/30 px-3 py-2 text-xs text-slate-500">当前使用窗口化列表渲染，优先降低 WebView 在大结果集下的内存压力。</div>

      <div ref="mobileScrollContainer" class="max-h-[68vh] overflow-y-auto" @scroll="onMobileScroll">
        <div :style="{ height: `${mobileTotalHeight}px`, position: 'relative' }">
          <div :style="{ transform: `translateY(${mobileWindowOffset}px)` }" class="space-y-3">
            <article v-for="item in visibleMobileRows" :key="mobileRowKey(item.row, item.index)" class="ui-card p-4">
              <div class="flex items-start justify-between gap-3">
                <div class="overflow-safe">
                  <p class="truncate font-mono text-base font-semibold text-slate-800">{{ item.row.address }}</p>
                  <div class="mt-2 flex flex-wrap items-center gap-2">
                    <span :class="resultToneClass(item.row.stage_status)" class="ui-pill">
                      {{ resultStageStatusLabel(item.row.stage_status) }}
                    </span>
                    <span v-if="item.row.colo" class="ui-pill ui-pill-subtle">{{ item.row.colo }}</span>
                    <span class="ui-pill ui-pill-subtle">源端口 {{ formatPort(item.row.source_port) }}</span>
                    <span class="ui-pill ui-pill-subtle">测速端口 {{ formatPort(item.row.test_port) }}</span>
                    <span class="ui-pill ui-pill-subtle">{{ exportStatusLabel(item.row.export_status) }}</span>
                  </div>
                </div>
                <button type="button" class="ui-button ui-button-ghost shrink-0 px-3 py-2 text-xs" @click="$emit('copy-address', item.row.address)">
                  <PhCopy size="14" />
                  复制
                </button>
              </div>

              <div class="mt-4 grid grid-cols-2 gap-2 text-xs text-slate-500 sm:grid-cols-4">
                <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
                  <p>TCP</p>
                  <strong class="mt-1 block text-sm text-slate-800">{{ formatMetric(item.row.tcp_latency_ms) }}</strong>
                </div>
                <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
                  <p>追踪</p>
                  <strong class="mt-1 block text-sm text-slate-800">{{ formatMetric(item.row.trace_latency_ms) }}</strong>
                </div>
                <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
                  <p>平均速率</p>
                  <strong class="mt-1 block text-sm text-slate-800">{{ formatSpeed(item.row.download_mbps) }}</strong>
                </div>
                <div class="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-3">
                  <p>最高速率</p>
                  <strong class="mt-1 block text-sm text-slate-800">{{ formatSpeed(item.row.max_download_mbps) }}</strong>
                </div>
              </div>

              <button type="button" class="ui-button ui-button-secondary mt-4 h-11 w-full" :disabled="loading || !canRerunTask" @click="$emit('rerun-address', item.row.address)">
                <PhRocketLaunch size="16" />
                单条重测
              </button>
            </article>
          </div>
        </div>
      </div>

      <button v-if="(resultsTotalCount || 0) > resultRows.length" type="button" class="ui-button ui-button-ghost w-full px-3 py-2 text-sm" :disabled="resultsLoading" @click="$emit('load-more-results')">
        <PhArrowClockwise size="14" />
        {{ resultsLoading ? "加载中" : `继续加载 (${resultRows.length}/${resultsTotalCount})` }}
      </button>
    </div>
  </section>
</template>

<style scoped>
.mobile-picker-button {
  display: flex;
  min-width: 0;
  height: 2.75rem;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  border: 1px solid var(--input-border);
  border-radius: 0.75rem;
  background: var(--input-bg);
  color: var(--text-primary);
  padding: 0 0.75rem;
  font-size: 0.875rem;
  transition:
    border-color 0.16s ease,
    box-shadow 0.16s ease;
}

.mobile-picker-button:focus-visible {
  border-color: var(--primary);
  outline: none;
  box-shadow: 0 0 0 2px var(--focus-ring);
}

.mobile-picker-menu {
  position: absolute;
  left: 0;
  right: 0;
  top: calc(100% + 0.35rem);
  z-index: 60;
  overflow: hidden;
  border: 1px solid var(--border-default);
  border-radius: 0.75rem;
  background: var(--app-elevated-bg);
  box-shadow: var(--shadow-panel);
}

.mobile-picker-option {
  display: flex;
  min-height: 2.75rem;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0 0.75rem;
  text-align: left;
  font-size: 0.875rem;
  color: var(--text-primary);
  background: transparent;
}

.mobile-picker-option + .mobile-picker-option {
  border-top: 1px solid var(--border-subtle);
}

.mobile-picker-option:active {
  background: var(--selected-bg);
}
</style>
