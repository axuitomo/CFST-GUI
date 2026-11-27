<script setup vapor lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { PhArrowDown, PhArrowUp, PhArrowsClockwise, PhCaretDown, PhCopy, PhFileCsv } from "@phosphor-icons/vue";
import type { ProbeResultIPFilter, ProbeResultOrder, ProbeResultSortBy } from "../lib/bridge";
import { buildCopyQueue, normalizeCopyCount, readResultCopySettings, RESULT_COPY_CUSTOM_MAX, RESULT_COPY_TOP_OPTIONS, saveResultCopySettings, type ResultCopySettings, type ResultCopyTopMode } from "../lib/results/copy";
import { RESULT_TIME_RANGE_OPTIONS, type ResultCountryOption, type ResultTimeRange } from "../lib/results/history";

// 结果页工具栏：窄屏三行（数据源 / IP+国家 / 排序+复制导出），桌面（lg+）两行（数据源+筛选 / 排序+复制导出）。控件不带上行标签，
// 语义交给控件自身文案、分段按钮，以及 aria-label / title。
// 表单控件继承根节点字号：styles.css 的 `font: inherit` 重置是无层的，会压过所有 text-* 工具类。
const props = defineProps<{
  csvExporting: boolean;
  exportDisabled: boolean;
  mergeDuplicateIps: boolean;
  resultCountryFilter: string;
  resultCountryFilterOptions: ResultCountryOption[];
  resultIpFilter: ProbeResultIPFilter;
  resultIpFilterOptions: Array<{ label: string; value: ProbeResultIPFilter }>;
  resultOrder: ProbeResultOrder;
  resultRows: Array<{ address: string }>;
  resultSortBy: ProbeResultSortBy;
  resultSortOptions: Array<{ label: string; value: ProbeResultSortBy }>;
  resultTimeRange: ResultTimeRange;
  resultsLoading: boolean;
}>();

const emit = defineEmits<{
  (event: "export-csv"): void;
  (event: "refresh"): void;
  (event: "update-country-filter", value: string): void;
  (event: "update-ip-filter", filter: ProbeResultIPFilter): void;
  (event: "update-merge-duplicates", value: boolean): void;
  (event: "update-order", order: ProbeResultOrder): void;
  (event: "update-sort", sortBy: ProbeResultSortBy): void;
  (event: "update-time-range", range: ResultTimeRange): void;
}>();

// 胶囊里只放最少的字形，完整含义由 optionLabel 生成的 title 承担。
const ipFilterSegments: Array<{ label: string; value: ProbeResultIPFilter }> = [
  { label: "全部", value: "all" },
  { label: "v4", value: "ipv4" },
  { label: "v6", value: "ipv6" },
];

const countryFilterOptions = computed(() => [{ label: "全部国家", value: "" }, ...props.resultCountryFilterOptions]);
const sortByModel = computed({
  get: () => props.resultSortBy,
  set: (value) => emit("update-sort", value as ProbeResultSortBy),
});
const orderLabel = computed(() => (props.resultOrder === "asc" ? "升序（点击切到降序）" : "降序（点击切到升序）"));
// 本次结果只含当前任务数据，没有跨批次重复，去重开关此时无意义。
const mergeDuplicatesDisabled = computed(() => props.resultTimeRange === "current");
const mergeDuplicatesHint = computed(() => (mergeDuplicatesDisabled.value ? "本次结果不做跨批次去重；切到历史范围后可合并同一 IP" : "跨批次范围下按 IP 保留最新一次测速记录；关闭后展示全部原始记录"));

// Top-N 与自定义数量都是纯展示偏好，和 coloCountry 一样存 localStorage，不进后端配置。
const storedCopySettings = readResultCopySettings();
const copyTopMode = ref<ResultCopyTopMode>(storedCopySettings.mode);
const copyCustomCount = ref(storedCopySettings.customCount);
const copySettings = computed<ResultCopySettings>(() => ({ customCount: normalizeCopyCount(copyCustomCount.value), mode: copyTopMode.value }));
watch(copyTopMode, (mode) => saveResultCopySettings({ customCount: normalizeCopyCount(copyCustomCount.value), mode }));
watch(copyCustomCount, (count) => saveResultCopySettings({ customCount: normalizeCopyCount(count), mode: copyTopMode.value }));
const copyQueue = computed(() => buildCopyQueue(props.resultRows, copySettings.value));
const copyDisabled = computed(() => copyQueue.value.length === 0);
const copiedCount = ref(0);
const copyFailed = ref(false);
const copyButtonLabel = computed(() => {
  if (copyFailed.value) {
    return "复制失败";
  }
  return copiedCount.value > 0 ? `已复制 ${copiedCount.value} 条` : `复制 IP (${copyQueue.value.length})`;
});
let copyResetTimer: ReturnType<typeof setTimeout> | undefined;

onBeforeUnmount(() => {
  if (copyResetTimer) {
    clearTimeout(copyResetTimer);
  }
});

function normalizeCopyCountInput() {
  copyCustomCount.value = normalizeCopyCount(copyCustomCount.value);
}

async function copyAllAddresses() {
  const addresses = copyQueue.value;
  if (addresses.length === 0) {
    return;
  }
  try {
    await navigator.clipboard.writeText(addresses.join("\n"));
    copyFailed.value = false;
    copiedCount.value = addresses.length;
  } catch {
    copyFailed.value = true;
    copiedCount.value = 0;
  }
  if (copyResetTimer) {
    clearTimeout(copyResetTimer);
  }
  copyResetTimer = setTimeout(() => {
    copiedCount.value = 0;
    copyFailed.value = false;
  }, 2000);
}

function optionLabel<T extends string>(options: Array<{ label: string; value: T }>, value: T) {
  return options.find((option) => option.value === value)?.label || value;
}

function selectValue(event: Event) {
  return (event.currentTarget as HTMLSelectElement).value;
}

function onTimeRangeChange(event: Event) {
  emit("update-time-range", selectValue(event) as ResultTimeRange);
}

function onCountryFilterChange(event: Event) {
  emit("update-country-filter", selectValue(event));
}

function onMergeDuplicatesChange(event: Event) {
  emit("update-merge-duplicates", (event.currentTarget as HTMLInputElement).checked);
}

function toggleOrder() {
  emit("update-order", props.resultOrder === "asc" ? "desc" : "asc");
}
</script>

<template>
  <div class="flex flex-col gap-2 text-xs lg:text-sm">
    <div class="flex flex-col gap-2 lg:flex-row lg:flex-wrap lg:items-center lg:justify-center">
      <div class="flex min-w-0 items-center gap-2">
        <button
          type="button"
          class="ui-button ui-button-secondary h-9 w-9 shrink-0 justify-center gap-1.5 overflow-hidden px-0 py-0 whitespace-nowrap lg:h-8 lg:w-[5.5rem]"
          :disabled="resultsLoading"
          :title="resultsLoading ? '刷新中' : '刷新结果列表'"
          :aria-label="resultsLoading ? '刷新中' : '刷新结果列表'"
          @click="emit('refresh')"
        >
          <PhArrowsClockwise :class="{ 'animate-spin': resultsLoading }" size="15" />
          <span class="hidden lg:inline">{{ resultsLoading ? "刷新中" : "刷新" }}</span>
        </button>
        <select class="ui-field h-9 w-full min-w-0 flex-1 rounded-lg px-2 py-0 lg:h-8 lg:w-32 lg:flex-none" aria-label="时间范围" title="时间范围" :value="resultTimeRange" @change="onTimeRangeChange">
          <option v-for="option in RESULT_TIME_RANGE_OPTIONS" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <label class="inline-flex h-9 shrink-0 items-center gap-1 rounded-lg border border-[var(--input-border)] bg-[var(--input-bg)] px-1.5 font-semibold text-[var(--text-secondary)] select-none lg:h-8" :class="mergeDuplicatesDisabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'" :title="mergeDuplicatesHint">
          <input type="checkbox" class="h-3.5 w-3.5 accent-primary" :checked="mergeDuplicateIps" :disabled="mergeDuplicatesDisabled" @change="onMergeDuplicatesChange" />
          去重
        </label>
      </div>

      <div class="flex min-w-0 items-center gap-2 lg:border-l lg:border-[var(--border-default)] lg:pl-2.5">
        <div class="inline-flex h-9 min-w-0 flex-1 items-center gap-0.5 rounded-lg border border-[var(--input-border)] bg-[var(--input-bg)] p-0.5 lg:h-8 lg:flex-none" role="group" aria-label="IP 版本">
          <button
            v-for="segment in ipFilterSegments"
            :key="segment.value"
            type="button"
            class="min-w-0 flex-1 rounded-md px-1 py-0.5 leading-4 font-semibold whitespace-nowrap transition-colors lg:flex-none"
            :class="segment.value === resultIpFilter ? 'bg-primary text-white' : 'text-[var(--text-secondary)] hover:bg-[var(--selected-bg)]'"
            :aria-pressed="segment.value === resultIpFilter"
            :title="optionLabel(resultIpFilterOptions, segment.value)"
            @click="emit('update-ip-filter', segment.value)"
          >
            {{ segment.label }}
          </button>
        </div>
        <select class="ui-field h-9 w-full min-w-0 flex-1 rounded-lg px-2 py-0 lg:h-8 lg:w-40 lg:flex-none" aria-label="国家/地区" title="国家/地区" :value="resultCountryFilter" @change="onCountryFilterChange">
          <option v-for="option in countryFilterOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
      </div>
    </div>

    <div class="flex min-w-0 flex-wrap items-center justify-center gap-1 lg:gap-2">
      <!-- 排序与方向合成一个控件：箭头就地切换升降序，右侧保留下拉箭头。 -->
      <div class="relative flex h-9 min-w-[5.25rem] flex-1 items-center lg:h-8 lg:w-32 lg:flex-none">
        <select v-model="sortByModel" class="ui-field h-9 w-full appearance-none rounded-lg py-0 pr-7 pl-1.5 lg:h-8" aria-label="排序" title="排序">
          <option v-for="option in resultSortOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <button type="button" class="absolute top-1/2 right-4.5 -translate-y-1/2 rounded px-0.5 text-[var(--text-secondary)] transition-colors hover:text-primary" :aria-label="orderLabel" :title="orderLabel" @click="toggleOrder">
          <PhArrowUp v-if="resultOrder === 'asc'" size="12" weight="bold" />
          <PhArrowDown v-else size="12" weight="bold" />
        </button>
        <PhCaretDown class="pointer-events-none absolute top-1/2 right-1 -translate-y-1/2 text-[var(--text-secondary)]" size="12" />
      </div>
      <div class="flex min-w-0 items-stretch">
        <select v-model="copyTopMode" class="ui-field h-9 w-16 shrink-0 rounded-l-lg rounded-r-none py-0 pr-1 pl-2 lg:h-8 lg:w-20" aria-label="复制数量" title="复制数量">
          <option v-for="option in RESULT_COPY_TOP_OPTIONS" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <input
          v-if="copyTopMode === 'custom'"
          v-model.number="copyCustomCount"
          type="number"
          min="1"
          :max="RESULT_COPY_CUSTOM_MAX"
          class="ui-field h-9 w-11 min-w-0 shrink-0 rounded-none border-x-0 [appearance:textfield] px-0.5 py-0 text-center lg:h-8"
          aria-label="自定义复制条数"
          title="自定义复制条数"
          @change="normalizeCopyCountInput"
        />
        <button type="button" class="ui-button ui-button-primary h-9 w-[5.75rem] shrink-0 justify-center gap-1 overflow-hidden rounded-l-none rounded-r-lg px-1 py-0 whitespace-nowrap tabular-nums lg:h-8 lg:w-36 lg:gap-1.5 lg:px-2.5" :disabled="copyDisabled" @click="copyAllAddresses">
          <PhCopy class="hidden lg:inline" size="14" />
          {{ copyButtonLabel }}
        </button>
      </div>
      <button type="button" class="ui-button ui-button-secondary h-9 min-w-12 shrink-0 justify-center gap-1 overflow-hidden px-1.5 py-0 whitespace-nowrap lg:h-8 lg:min-w-24 lg:gap-1.5 lg:px-2.5" :disabled="exportDisabled" @click="emit('export-csv')">
        <PhFileCsv size="14" />
        {{ csvExporting ? "导出中" : "导出" }}<span class="hidden lg:inline"> CSV</span>
      </button>
    </div>
  </div>
</template>
