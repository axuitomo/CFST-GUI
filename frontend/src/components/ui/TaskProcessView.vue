<script setup lang="ts">
import { computed } from "vue";
import { PhCheckCircle, PhClockCounterClockwise, PhListChecks, PhSpinnerGap, PhWarningCircle, PhXCircle } from "@phosphor-icons/vue";

type ProcessTone = "success" | "error" | "running" | "info" | "warning";

interface ProcessEntry {
  detail: string;
  stage: string;
  title: string;
  tone: ProcessTone;
  ts: string;
}

interface TimestampFormatOptions {
  fallback?: string;
  includeDate?: boolean;
  includeOffset?: boolean;
  includeSeconds?: boolean;
}

const {
  emptyText = "等待任务启动...",
  entries,
  formatTimestamp,
  mobile = false,
  title = "实时测试进程",
} = defineProps<{
  emptyText?: string;
  entries: ProcessEntry[];
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  mobile?: boolean;
  title?: string;
}>();

defineEmits<{
  (event: "clear"): void;
}>();

const shellClass = computed(() => (mobile ? "rounded-2xl" : "rounded-xl"));

function toneCardClass(tone: ProcessTone) {
  if (tone === "success") {
    return "ui-tone-card-success";
  }
  if (tone === "error") {
    return "ui-tone-card-danger";
  }
  if (tone === "warning") {
    return "ui-tone-card-warning";
  }
  if (tone === "running") {
    return "ui-tone-card-info";
  }
  return "ui-tone-card-neutral";
}

function toneTextClass(tone: ProcessTone) {
  if (tone === "success") {
    return "ui-tone-text-success";
  }
  if (tone === "error") {
    return "ui-tone-text-danger";
  }
  if (tone === "warning") {
    return "ui-tone-text-warning";
  }
  if (tone === "running") {
    return "ui-tone-text-info";
  }
  return "ui-tone-text-neutral";
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
    export: "导出",
    failed: "失败",
    preprocessed: "预处理",
    probe: "探测",
    stage0_pool: "IP池",
    stage1_tcp: "TCP测延迟",
    stage2_head: "追踪探测",
    stage2_trace: "追踪探测",
    stage3_get: "文件测速",
    warning: "提示",
  };

  return labels[normalized] || stage;
}

function formatTimestampLabel(ts: string) {
  return formatTimestamp(ts, {
    includeDate: !mobile,
    includeSeconds: true,
  });
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
  <div :class="shellClass" class="ui-card overflow-hidden shadow-panel">
    <div class="task-process-header flex items-center justify-between gap-3 px-4 py-3">
      <div class="min-w-0 flex items-center gap-2 text-sm font-semibold">
        <PhListChecks :size="mobile ? 16 : 18" />
        <span class="truncate">{{ title }}</span>
      </div>

      <button type="button" class="task-process-clear ui-pill ui-pill-neutral rounded-lg px-2.5 py-1 text-xs font-medium transition" @click="$emit('clear')">清空</button>
    </div>

    <div class="max-h-[26rem] overflow-y-auto p-3 lg:max-h-[22rem]">
      <div v-if="entries.length === 0" class="task-process-empty py-10 text-center text-sm italic">
        {{ emptyText }}
      </div>

      <div v-else class="space-y-3 lg:space-y-2.5">
        <article v-for="(entry, index) in entries" :key="`${entry.ts}-${entry.stage}-${index}`" :class="toneCardClass(entry.tone)" class="task-process-entry rounded-2xl border px-4 py-3 lg:rounded-xl lg:px-3 lg:py-2.5">
          <div class="flex items-start justify-between gap-4">
            <div class="overflow-safe">
              <div class="flex flex-wrap items-center gap-2">
                <component :is="toneIcon(entry.tone)" :class="toneTextClass(entry.tone)" :size="18" />
                <p :class="toneTextClass(entry.tone)" class="min-w-0 font-semibold">{{ entry.title }}</p>
                <span class="task-process-stage rounded-full px-2 py-0.5 text-xs">
                  {{ stageLabel(entry.stage) }}
                </span>
              </div>
              <p class="task-process-detail mt-2 text-sm leading-6">{{ entry.detail }}</p>
            </div>
            <p class="task-process-ts shrink-0 break-all text-right text-xs">{{ formatTimestampLabel(entry.ts) }}</p>
          </div>
        </article>
      </div>
    </div>
  </div>
</template>
