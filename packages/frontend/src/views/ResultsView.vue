<script setup vapor lang="ts">
import { computed, nextTick, onMounted, ref, useTemplateRef, watch } from "vue";
import { PhArrowsClockwise, PhCloud, PhCopy, PhFileCsv, PhGlobeHemisphereWest, PhRocketLaunch, PhTable } from "@phosphor-icons/vue";
import type { ProbeResultIPFilter, ProbeResultOrder, ProbeResultSortBy, TaskSnapshot } from "../lib/bridge";
import { formatColoCountry, readColoCountryFormat, saveColoCountryFormat, type ColoCountryFormat } from "../lib/results/coloCountry";
import ResultToolbar from "../components/ResultToolbar.vue";
import { type ResultCountryOption, type ResultDisplayRow, type ResultTimeRange } from "../lib/results/history";
import { computeTableWindow } from "../lib/results/tableViewport";
import { taskActionLabels, taskSessionLabel, taskStageLabel, taskStatusLabel } from "../lib/app/labels";

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

interface CloudflarePushSettings {
  providerId: string;
  recordName: string;
  recordType: CloudflarePushRecordType;
  topN: number;
}

interface DnsPushTarget {
  id: string;
  label: string;
  recordTarget?: boolean;
}

const props = defineProps<{
  canRerunTask: boolean;
  cloudflarePushing: boolean;
  cloudflarePushSettings: CloudflarePushSettings;
  cloudflareRoutingActive: boolean;
  cloudflareRoutingRuleCount: number;
  csvExporting: boolean;
  dnsPushTargets: DnsPushTarget[];
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  githubExporting: boolean;
  githubTopN: number;
  hasActiveTask: boolean;
  loading: boolean;
  mergeDuplicateIps: boolean;
  platform: "desktop" | "mobile";
  resultCountryFilter: string;
  resultCountryFilterOptions: ResultCountryOption[];
  resultIpFilter: ProbeResultIPFilter;
  resultIpFilterOptions: Array<{ label: string; value: ProbeResultIPFilter }>;
  resultOrder: ProbeResultOrder;
  resultRows: ResultDisplayRow[];
  resultTimeRange: ResultTimeRange;
  resultsTotalCount?: number;
  resultSortBy: ProbeResultSortBy;
  resultSortOptions: Array<{ label: string; value: ProbeResultSortBy }>;
  resultsLoading: boolean;
  summary: SummaryStats;
  task: TaskState;
  taskSnapshot: TaskSnapshot | null;
}>();

// 空状态里引用的按钮名必须和屏幕上那串字一致：桌面是"启动任务"，移动端是"启动"。
const startActionLabel = computed(() => (props.platform === "mobile" ? taskActionLabels.start.verb : taskActionLabels.start.button));

const emit = defineEmits<{
  (event: "copy-address", address: string): void;
  (event: "export-current-results-csv"): void;
  (event: "export-github"): void;
  (event: "go-sources"): void;
  (event: "push-cloudflare"): void;
  (event: "refresh-results"): void;
  (event: "load-more-results"): void;
  (event: "rerun-address", address: string): void;
  (event: "update-cloudflare-push-settings", settings: Partial<CloudflarePushSettings>): void;
  (event: "update-github-top-n", value: number): void;
  (event: "update-ip-filter", filter: ProbeResultIPFilter): void;
  (event: "update-order", order: ProbeResultOrder): void;
  (event: "update-sort", sortBy: ProbeResultSortBy): void;
  (event: "update-country-filter", value: string): void;
  (event: "update-merge-duplicates", value: boolean): void;
  (event: "update-time-range", range: ResultTimeRange): void;
}>();

const cloudflarePanelOpen = ref(false);
const resultActionDisabled = computed(() => props.loading || props.csvExporting || props.githubExporting || props.cloudflarePushing || props.hasActiveTask || props.resultRows.length === 0);
const selectedPushTarget = computed(() => props.dnsPushTargets.find((item) => item.id === props.cloudflarePushSettings.providerId) ?? props.dnsPushTargets[0]);
const cloudflarePushDisabled = computed(() => resultActionDisabled.value || (!props.cloudflareRoutingActive && !props.cloudflarePushSettings.recordName.trim() && !selectedPushTarget.value?.recordTarget));
const selectedPushTargetLabel = computed(() => selectedPushTarget.value?.label || "DNS 平台");
const cloudflarePushPreviewCount = computed(() => {
  const topN = Math.max(0, Number(props.cloudflarePushSettings.topN) || 0);
  return topN > 0 ? Math.min(topN, props.resultRows.length) : props.resultRows.length;
});
const countryFormatOptions: Array<{ label: string; value: ColoCountryFormat }> = [
  { label: "国家名称", value: "name" },
  { label: "地区代码", value: "code" },
];
// 国家列的显示口径是纯展示偏好，跟着浏览器配置走，不进后端配置。
const countryFormat = ref<ColoCountryFormat>(readColoCountryFormat());
watch(countryFormat, (value) => saveColoCountryFormat(value));

const countryFormatLabel = computed(() => optionLabel(countryFormatOptions, countryFormat.value));
const countryFormatHint = computed(() => `切换国家/地区显示格式（当前：${countryFormatLabel.value}）`);
function cycleCountryFormat() {
  countryFormat.value = countryFormat.value === "name" ? "code" : "name";
}

function formatMeasuredAt(timestamp?: number) {
  return typeof timestamp === "number" && Number.isFinite(timestamp) ? formatTimestampLabel(new Date(timestamp).toISOString(), { includeSeconds: false }) : "-";
}

function resultRowNote(row: ResultDisplayRow) {
  const parts = [formatMeasuredAt(row.measured_at)];
  if ((row.batch_count ?? 1) > 1) {
    parts.push(`所选周期测速 ${row.batch_count} 次`);
  }
  return parts.join(" · ");
}

function optionLabel<T extends string>(options: Array<{ label: string; value: T }>, value: T) {
  return options.find((option) => option.value === value)?.label || value;
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

function updateCloudflarePushProvider(event: Event) {
  emit("update-cloudflare-push-settings", { providerId: inputValue(event) });
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

function resultToneClass(stageStatus: string) {
  if (stageStatus.includes("failed")) {
    return "bg-rose-50 text-rose-700";
  }

  if (stageStatus.includes("passed") || stageStatus === "completed") {
    return "bg-emerald-50 text-emerald-700";
  }

  return "bg-slate-100 text-slate-600";
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
    return `${taskStageLabel(status.replace(/_passed$/, ""))}通过`;
  }

  if (status.endsWith("_failed")) {
    return `${taskStageLabel(status.replace(/_failed$/, ""))}失败`;
  }

  return taskStageLabel(status);
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

// 桌面表格窗口化：历史范围一次可能上万行，全量进 DOM 会锁死 WebView；只渲染滚动窗口内的行，用上下占位行撑起滚动高度。
const DESKTOP_ROW_HEIGHT_FALLBACK = 57;
const DESKTOP_WINDOW_ROWS = 40;
const DESKTOP_OVERSCAN = 8;
const desktopScrollContainer = useTemplateRef<HTMLDivElement>("desktopScrollContainer");
const desktopScrollTop = ref(0);
const desktopHeaderHeight = ref(0);
const desktopRowHeight = ref(DESKTOP_ROW_HEIGHT_FALLBACK);
const desktopWindow = computed(() =>
  computeTableWindow({
    headerHeight: desktopHeaderHeight.value,
    overscan: DESKTOP_OVERSCAN,
    rowHeight: desktopRowHeight.value,
    scrollTop: desktopScrollTop.value,
    total: props.resultRows.length,
    windowRows: DESKTOP_WINDOW_ROWS,
  }),
);
const visibleDesktopRows = computed(() => props.resultRows.slice(desktopWindow.value.start, desktopWindow.value.start + DESKTOP_WINDOW_ROWS));

function onDesktopScroll(event: Event) {
  desktopScrollTop.value = (event.currentTarget as HTMLDivElement).scrollTop || 0;
}

// 列内容都是 nowrap，行高一致，量一次真实行高即可；量不到时退回固定值。
function measureDesktopTable() {
  const container = desktopScrollContainer.value;
  if (!container) {
    return;
  }
  const headerHeight = container.querySelector<HTMLTableSectionElement>("thead")?.getBoundingClientRect().height ?? 0;
  if (headerHeight > 0) {
    desktopHeaderHeight.value = headerHeight;
  }
  const rowHeight = container.querySelector<HTMLTableRowElement>("tbody tr.desktop-data-row")?.getBoundingClientRect().height ?? 0;
  if (rowHeight > 0) {
    desktopRowHeight.value = rowHeight;
  }
}

onMounted(() => {
  void nextTick().then(measureDesktopTable);
});

watch(
  () => [props.platform, props.resultRows.length] as const,
  async () => {
    if (props.platform !== "desktop") {
      return;
    }
    await nextTick();
    measureDesktopTable();
  },
);

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
}

// 结果行的 key 必须是复合键：同一 IP 会按输入源端口/实际测速端口出现在不同分组里，
// 只用 address 会在桌面表格里撞 key（Vue 复用错行、切换排序后行内容错位）。
function resultRowKey(row: ResultDisplayRow, index: number) {
  return [row.address, row.source_port ?? "", row.test_port ?? "", row.stage_status || "", index].join("|");
}

function mobileRowsSignature() {
  const first = props.resultRows[0];
  const last = props.resultRows[props.resultRows.length - 1];
  return [props.task.taskId, props.resultIpFilter, props.resultSortBy, props.resultOrder, props.resultTimeRange, props.mergeDuplicateIps, props.resultCountryFilter, props.resultRows.length, first ? resultRowKey(first, 0) : "", last ? resultRowKey(last, props.resultRows.length - 1) : ""].join("::");
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
</script>

<template>
  <section v-if="platform === 'desktop'" class="space-y-5">
    <div class="grid grid-cols-3 gap-4">
      <article class="ui-card p-4">
        <p class="text-sm font-medium text-slate-500">当前结果</p>
        <strong class="mt-2 block text-xl font-bold text-slate-800">{{ resultRows.length }}</strong>
      </article>
      <article class="ui-card p-4">
        <p class="text-sm font-medium text-slate-500">有效结果</p>
        <strong class="mt-2 block text-xl font-bold text-emerald-600">{{ summary.passed || summary.exported }}</strong>
      </article>
      <article class="ui-card p-4">
        <p class="text-sm font-medium text-slate-500">失败结果</p>
        <strong class="mt-2 block text-xl font-bold text-rose-500">{{ summary.failed }}</strong>
      </article>
    </div>

    <article class="ui-card overflow-hidden">
      <div class="border-b border-slate-200 bg-slate-50/70 px-5 py-3">
        <h3 class="flex items-center text-base font-semibold text-slate-800">
          <PhTable class="mr-2 text-primary" size="20" />
          当前测速结果
        </h3>
        <p class="mt-1 text-sm text-slate-500">
          {{ resultTimeRange === "current" ? "新任务或单条重测会清空这里，仅展示当前任务结果。" : "当前展示本地保存的历史测速记录，最长保留 30 天。" }}
        </p>
      </div>

      <div class="border-b border-slate-200 px-5 py-2.5">
        <ResultToolbar
          :csv-exporting="csvExporting"
          :export-disabled="resultActionDisabled"
          :merge-duplicate-ips="mergeDuplicateIps"
          :result-country-filter="resultCountryFilter"
          :result-country-filter-options="resultCountryFilterOptions"
          :result-ip-filter="resultIpFilter"
          :result-ip-filter-options="resultIpFilterOptions"
          :result-order="resultOrder"
          :result-rows="resultRows"
          :result-sort-by="resultSortBy"
          :result-sort-options="resultSortOptions"
          :result-time-range="resultTimeRange"
          :results-loading="resultsLoading"
          @export-csv="$emit('export-current-results-csv')"
          @refresh="$emit('refresh-results')"
          @update-country-filter="$emit('update-country-filter', $event)"
          @update-ip-filter="$emit('update-ip-filter', $event)"
          @update-merge-duplicates="$emit('update-merge-duplicates', $event)"
          @update-order="$emit('update-order', $event)"
          @update-sort="$emit('update-sort', $event)"
          @update-time-range="$emit('update-time-range', $event)"
        />
      </div>

      <div v-if="cloudflarePanelOpen" class="cf-push-panel grid gap-4 border-b px-5 py-4 md:grid-cols-[minmax(0,1fr)_9rem_8rem_auto] md:items-end">
        <div v-if="cloudflareRoutingActive" class="cf-push-routing md:col-span-3 rounded-lg border px-4 py-3 text-sm">
          <span class="cf-push-title block font-semibold">目标记录与分流推送</span>
          <span class="cf-push-copy mt-1 block text-xs">会先推送设置页主记录，再按 {{ cloudflareRoutingRuleCount }} 条启用分流规则继续推送；共享上传筛选、平台 Top N 和规则 Top N 控制最终数量。</span>
        </div>
        <label v-if="dnsPushTargets.length > 1" class="md:col-span-4">
          <span class="ui-label">推送平台</span>
          <select :value="cloudflarePushSettings.providerId" class="ui-field" @change="updateCloudflarePushProvider">
            <option v-for="target in dnsPushTargets" :key="target.id" :value="target.id">{{ target.label }}</option>
          </select>
        </label>
        <template v-if="!cloudflareRoutingActive">
          <label>
            <span class="ui-label">记录名称</span>
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
          <span class="ui-label">推送上限</span>
          <input :value="cloudflarePushSettings.topN" min="0" type="number" class="ui-field" @input="updateCloudflareTopN" />
        </label>
        <button type="button" class="ui-button ui-button-primary whitespace-nowrap" :disabled="cloudflarePushDisabled" @click="$emit('push-cloudflare')">
          <PhCloud size="16" weight="fill" />
          {{ cloudflarePushing ? "推送中" : cloudflareRoutingActive ? "推送目标" : `推送 ${selectedPushTargetLabel}` }}
        </button>
        <p class="cf-push-copy md:col-span-4 text-xs">
          <template v-if="cloudflareRoutingActive">凭据、TTL 和目标记录名沿用设置页；主目标推送后，当前可见列表会继续交给分流规则筛选。</template>
          <template v-else>凭据和 TTL 沿用设置页；本面板只覆盖本次推送目标和数量。0 表示当前可见列表不限数量；本次预计推送 {{ cloudflarePushPreviewCount }} 条。</template>
        </p>
      </div>

      <div class="border-b border-slate-200 px-5 py-2.5 text-xs text-slate-500">
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1">
          <span class="overflow-safe">状态：{{ taskStatusLabel(taskSnapshot?.status) }}</span>
          <span class="overflow-safe">阶段：{{ taskStageLabel(taskSnapshot?.current_stage || task.stage) }}</span>
          <span class="overflow-safe">更新：{{ taskSnapshot?.updated_at ? formatTimestampLabel(taskSnapshot.updated_at) : "-" }}</span>
        </div>
        <details class="mt-1.5">
          <summary class="cursor-pointer font-medium text-slate-600">任务详情</summary>
          <div class="mt-1.5 flex flex-wrap gap-x-4 gap-y-1">
            <span class="overflow-safe">任务：{{ task.taskId || "等待中" }}</span>
            <span class="overflow-safe">会话：{{ taskSessionLabel(taskSnapshot?.session_state || "persisted_only") }}</span>
            <span class="overflow-safe">全局端口：{{ taskContextNumber(taskSnapshot, "global_tcp_port") || "-" }}</span>
            <span class="overflow-safe">输入源端口：{{ taskContextPorts(taskSnapshot).join(" / ") || "未指定" }}</span>
            <span class="overflow-safe">实际测速端口：{{ taskCurrentPortLabel(taskSnapshot) }}</span>
            <span class="overflow-safe">原始状态：{{ taskSnapshot?.status || "-" }}</span>
          </div>
        </details>
      </div>

      <div ref="desktopScrollContainer" class="table-scroll desktop-data-table-scroll max-h-[70vh] overflow-y-auto" @scroll="onDesktopScroll">
        <table class="desktop-data-table desktop-results-table text-sm">
          <thead class="sticky top-0 z-10 text-left text-slate-500">
            <tr>
              <th class="whitespace-nowrap px-4 py-2.5 font-semibold">IP 地址</th>
              <th class="whitespace-nowrap px-4 py-2.5 font-semibold">输入源端口</th>
              <th class="whitespace-nowrap px-4 py-2.5 font-semibold">实际测速端口</th>
              <th class="whitespace-nowrap px-4 py-2.5 font-semibold">阶段状态</th>
              <!-- 列头里嵌了显示格式切换按钮:aria-label 让列头的可访问名只保留「国家/地区」,
                   否则按钮文案会被并进列头名称,读屏时整行表头都读成「国家/地区 切换国家/地区显示格式」。 -->
              <th class="whitespace-nowrap px-4 py-2.5 font-semibold" aria-label="国家/地区">
                <span class="inline-flex items-center gap-1">
                  国家/地区
                  <button type="button" class="rounded text-slate-400 transition-colors hover:text-primary focus-visible:text-primary focus-visible:outline-none" :title="countryFormatHint" :aria-label="countryFormatHint" @click="cycleCountryFormat">
                    <PhGlobeHemisphereWest size="15" />
                  </button>
                </span>
              </th>
              <th class="whitespace-nowrap px-4 py-2.5 font-semibold">TCP</th>
              <th class="whitespace-nowrap px-4 py-2.5 font-semibold">追踪</th>
              <th class="whitespace-nowrap px-4 py-2.5 font-semibold">平均速率</th>
              <th class="whitespace-nowrap px-4 py-2.5 font-semibold">最高速率</th>
              <th class="whitespace-nowrap px-4 py-2.5 font-semibold">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-if="desktopWindow.topSpacer > 0" aria-hidden="true">
              <td colspan="11" :style="{ borderWidth: 0, height: `${desktopWindow.topSpacer}px`, padding: 0 }"></td>
            </tr>
            <tr v-for="(row, index) in visibleDesktopRows" :key="resultRowKey(row, index)" class="desktop-data-row">
              <td class="max-w-[11rem] px-4 py-3 font-mono text-xs text-slate-700">
                <span class="block truncate">{{ row.address }}</span>
                <span class="mt-0.5 block truncate font-sans text-[11px] text-slate-400" :title="resultRowNote(row)">{{ resultRowNote(row) }}</span>
              </td>
              <td class="whitespace-nowrap px-4 py-3 font-mono text-xs text-slate-600">{{ formatPort(row.source_port) }}</td>
              <td class="whitespace-nowrap px-4 py-3 font-mono text-xs text-slate-600">{{ formatPort(row.test_port) }}</td>
              <td class="whitespace-nowrap px-4 py-3">
                <span :class="resultToneClass(row.stage_status)" class="ui-pill">
                  {{ resultStageStatusLabel(row.stage_status) }}
                </span>
                <span v-if="row.colo" class="ml-2 text-xs text-slate-400">{{ row.colo }}</span>
                <span v-if="row.note" class="ui-pill ui-pill-subtle ml-2">{{ row.note }}</span>
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ formatColoCountry(row.colo, countryFormat) }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ formatMetric(row.tcp_latency_ms) }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ formatMetric(row.trace_latency_ms) }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ formatSpeed(row.download_mbps) }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ formatSpeed(row.max_download_mbps) }}</td>
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
            <tr v-if="desktopWindow.bottomSpacer > 0" aria-hidden="true">
              <td colspan="11" :style="{ borderWidth: 0, height: `${desktopWindow.bottomSpacer}px`, padding: 0 }"></td>
            </tr>
            <tr v-if="resultRows.length === 0">
              <td colspan="11" class="px-4 py-8 text-center text-sm text-slate-400">
                <p>还没有测速结果。</p>
                <p class="mt-1">先添加输入源，再回到任务看板点击“{{ startActionLabel }}”。</p>
                <button type="button" class="ui-button ui-button-primary mt-3 px-4 py-2" @click="emit('go-sources')">去添加输入源</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="(resultsTotalCount || 0) > resultRows.length" class="border-t border-slate-200 px-5 py-2.5">
        <button type="button" class="ui-button ui-button-ghost w-full px-3 py-2 text-sm" :disabled="resultsLoading" @click="$emit('load-more-results')">
          <PhArrowsClockwise :size="14" />
          {{ resultsLoading ? "加载中" : `继续加载 (${resultRows.length}/${resultsTotalCount})` }}
        </button>
      </div>
    </article>
  </section>

  <section v-else class="space-y-4">
    <article class="ui-card p-4">
      <div class="flex min-w-0 items-center">
        <PhFileCsv class="mr-2 text-primary" size="18" />
        <h3 class="text-sm font-semibold text-slate-800">当前测速结果</h3>
        <button type="button" class="ml-1 rounded text-slate-400 transition-colors hover:text-primary" :title="countryFormatHint" :aria-label="countryFormatHint" @click="cycleCountryFormat">
          <PhGlobeHemisphereWest size="16" />
        </button>
      </div>

      <div v-if="cloudflarePanelOpen" class="cf-push-panel mt-4 grid gap-3 rounded-xl border p-3">
        <div v-if="cloudflareRoutingActive" class="cf-push-routing rounded-lg border px-3 py-2 text-sm">
          <span class="cf-push-title block font-semibold">目标记录与分流推送</span>
          <span class="cf-push-copy mt-1 block text-xs">先推送设置页主记录，再按 {{ cloudflareRoutingRuleCount }} 条启用分流规则继续推送。</span>
        </div>
        <label v-if="dnsPushTargets.length > 1">
          <span class="ui-label">推送平台</span>
          <select :value="cloudflarePushSettings.providerId" class="ui-field" @change="updateCloudflarePushProvider">
            <option v-for="target in dnsPushTargets" :key="target.id" :value="target.id">{{ target.label }}</option>
          </select>
        </label>
        <template v-if="!cloudflareRoutingActive">
          <label>
            <span class="ui-label">记录名称</span>
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
          <span class="ui-label">推送上限</span>
          <input :value="cloudflarePushSettings.topN" min="0" type="number" class="ui-field" @input="updateCloudflareTopN" />
        </label>
        <button type="button" class="ui-button ui-button-primary w-full px-3 py-2 text-xs" :disabled="cloudflarePushDisabled" @click="$emit('push-cloudflare')">
          <PhCloud size="14" weight="fill" />
          {{ cloudflarePushing ? "推送中" : cloudflareRoutingActive ? "推送目标" : `推送 ${selectedPushTargetLabel}` }}
        </button>
        <p class="cf-push-copy text-xs">
          <template v-if="cloudflareRoutingActive">凭据、TTL 和目标记录名沿用设置页；主目标推送后继续筛选分流规则。</template>
          <template v-else>凭据和 TTL 沿用设置页；仅覆盖本次目标和数量。0 表示不限数量；预计推送 {{ cloudflarePushPreviewCount }} 条。</template>
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

    <article class="ui-card p-2">
      <ResultToolbar
        :csv-exporting="csvExporting"
        :export-disabled="resultActionDisabled"
        :merge-duplicate-ips="mergeDuplicateIps"
        :result-country-filter="resultCountryFilter"
        :result-country-filter-options="resultCountryFilterOptions"
        :result-ip-filter="resultIpFilter"
        :result-ip-filter-options="resultIpFilterOptions"
        :result-order="resultOrder"
        :result-rows="resultRows"
        :result-sort-by="resultSortBy"
        :result-sort-options="resultSortOptions"
        :result-time-range="resultTimeRange"
        :results-loading="resultsLoading"
        @export-csv="$emit('export-current-results-csv')"
        @refresh="$emit('refresh-results')"
        @update-country-filter="$emit('update-country-filter', $event)"
        @update-ip-filter="$emit('update-ip-filter', $event)"
        @update-merge-duplicates="$emit('update-merge-duplicates', $event)"
        @update-order="$emit('update-order', $event)"
        @update-sort="$emit('update-sort', $event)"
        @update-time-range="$emit('update-time-range', $event)"
      />
    </article>

    <div v-if="resultRows.length === 0" class="ui-card p-8 text-center text-sm text-slate-400">
      <template v-if="showRecoveringState">正在恢复测速结果，请稍候…</template>
      <template v-else>
        <p>还没有测速结果。</p>
        <p class="mt-1">先添加输入源，再回到任务看板点击“{{ startActionLabel }}”。</p>
        <button type="button" class="ui-button ui-button-primary mt-3" @click="emit('go-sources')">去添加输入源</button>
      </template>
    </div>

    <div v-else class="space-y-3">
      <div ref="mobileScrollContainer" class="max-h-[68vh] overflow-y-auto" @scroll="onMobileScroll">
        <div :style="{ height: `${mobileTotalHeight}px`, position: 'relative' }">
          <div :style="{ transform: `translateY(${mobileWindowOffset}px)` }" class="space-y-3">
            <article v-for="item in visibleMobileRows" :key="resultRowKey(item.row, item.index)" class="ui-card p-4">
              <div class="flex items-start justify-between gap-3">
                <div class="overflow-safe">
                  <p class="truncate font-mono text-base font-semibold text-slate-800">{{ item.row.address }}</p>
                  <div class="mt-2 flex flex-wrap items-center gap-2">
                    <span :class="resultToneClass(item.row.stage_status)" class="ui-pill">
                      {{ resultStageStatusLabel(item.row.stage_status) }}
                    </span>
                    <span v-if="item.row.colo" class="ui-pill ui-pill-subtle">{{ item.row.colo }} · {{ formatColoCountry(item.row.colo, countryFormat) }}</span>
                    <span v-if="item.row.note" class="ui-pill ui-pill-subtle">{{ item.row.note }}</span>
                    <span class="ui-pill ui-pill-subtle">源端口 {{ formatPort(item.row.source_port) }}</span>
                    <span class="ui-pill ui-pill-subtle">测速端口 {{ formatPort(item.row.test_port) }}</span>
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
        <PhArrowsClockwise size="14" />
        {{ resultsLoading ? "加载中" : `继续加载 (${resultRows.length}/${resultsTotalCount})` }}
      </button>
    </div>
  </section>
</template>
