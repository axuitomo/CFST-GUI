<script setup lang="ts">
import { computed } from "vue";
import {
  PhCheckCircle,
  PhClockCounterClockwise,
  PhListChecks,
  PhSpinnerGap,
  PhWarningCircle,
  PhXCircle,
} from "@phosphor-icons/vue";

type ProcessTone = "success" | "error" | "running" | "info" | "warning";

interface ProcessEntry {
  detail: string;
  stage: string;
  title: string;
  tone: ProcessTone;
  ts: string;
}

const props = withDefaults(
  defineProps<{
    emptyText?: string;
    entries: ProcessEntry[];
    mobile?: boolean;
    title?: string;
  }>(),
  {
    emptyText: "等待任务启动...",
    mobile: false,
    title: "实时测试进程",
  }
);

defineEmits<{
  (event: "clear"): void;
}>();

const shellClass = computed(() => (props.mobile ? "rounded-2xl" : "rounded-2xl"));

function toneCardClass(tone: ProcessTone) {
  if (tone === "success") {
    return "border-emerald-200 bg-emerald-50/80";
  }
  if (tone === "error") {
    return "border-rose-200 bg-rose-50/80";
  }
  if (tone === "warning") {
    return "border-amber-200 bg-amber-50/80";
  }
  if (tone === "running") {
    return "border-primary/20 bg-primary/5";
  }
  return "border-slate-200 bg-slate-50/80";
}

function toneTextClass(tone: ProcessTone) {
  if (tone === "success") {
    return "text-emerald-700";
  }
  if (tone === "error") {
    return "text-rose-700";
  }
  if (tone === "warning") {
    return "text-amber-700";
  }
  if (tone === "running") {
    return "text-primary";
  }
  return "text-slate-700";
}

function stageLabel(stage: string) {
  const normalized = stage.trim().toLowerCase();
  if (!normalized) {
    return "通用";
  }

  const labels: Record<string, string> = {
    accepted: "提交",
    cooling: "冷却",
    completed: "完成",
    download: "下载测速",
    export: "导出",
    failed: "失败",
    latency: "延迟测速",
    preprocessed: "预处理",
    probe: "探测",
    warning: "提示",
  };

  return labels[normalized] || stage;
}

function formatTimestamp(ts: string) {
  if (!props.mobile) {
    return ts;
  }

  const [, time = ts] = ts.split("T");
  return time.split(".")[0] || ts;
}

function toneIcon(tone: ProcessTone) {
  if (tone === "success") {
    return PhCheckCircle;
  }
  if (tone === "error") {
    return PhXCircle;
  }
  if (tone === "warning") {
    return PhWarningCircle;
  }
  if (tone === "running") {
    return PhSpinnerGap;
  }
  return PhClockCounterClockwise;
}
</script>

<template>
  <div :class="shellClass" class="overflow-hidden border border-slate-200 bg-white shadow-panel">
    <div class="flex items-center justify-between border-b border-slate-200 bg-slate-50/80 px-4 py-3">
      <div class="flex items-center gap-2 text-sm font-semibold text-slate-700">
        <PhListChecks :size="mobile ? 16 : 18" />
        <span>{{ title }}</span>
      </div>

      <button
        type="button"
        class="rounded-lg bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-500 transition hover:text-slate-800"
        @click="$emit('clear')"
      >
        清空
      </button>
    </div>

    <div class="max-h-[26rem] overflow-y-auto p-3 lg:p-4">
      <div v-if="entries.length === 0" class="py-10 text-center text-sm italic text-slate-400">
        {{ emptyText }}
      </div>

      <div v-else class="space-y-3">
        <article
          v-for="(entry, index) in entries"
          :key="`${entry.ts}-${entry.stage}-${index}`"
          :class="toneCardClass(entry.tone)"
          class="rounded-2xl border px-4 py-3"
        >
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <component :is="toneIcon(entry.tone)" :class="toneTextClass(entry.tone)" :size="18" />
                <p :class="toneTextClass(entry.tone)" class="font-semibold">{{ entry.title }}</p>
                <span class="rounded-full bg-white/70 px-2 py-0.5 text-xs text-slate-500">
                  {{ stageLabel(entry.stage) }}
                </span>
              </div>
              <p class="mt-2 text-sm leading-6 text-slate-600">{{ entry.detail }}</p>
            </div>
            <p class="shrink-0 text-xs text-slate-400">{{ formatTimestamp(entry.ts) }}</p>
          </div>
        </article>
      </div>
    </div>
  </div>
</template>
