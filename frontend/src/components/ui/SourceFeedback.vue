<script setup vapor lang="ts">
import { computed } from "vue";
import type { SourceConfig } from "../../lib/bridge";
import { sourceInputIssue } from "../../lib/sourceGroups";

const props = defineProps<{
  source: SourceConfig;
  busy: boolean;
  error?: string;
  preview?: { totalCount: number; invalidCount: number };
}>();
const issue = computed(() => sourceInputIssue(props.source));
</script>

<template>
  <div v-if="issue || busy || error || preview" class="mt-3 text-sm" role="status" aria-live="polite">
    <p v-if="issue" class="text-amber-700">{{ issue }}</p>
    <p v-else-if="busy" class="text-slate-500">正在读取并检查来源…</p>
    <p v-else-if="error" class="break-words text-red-600">{{ error }} 请检查来源内容或地址后重试。</p>
    <p v-else-if="preview" :class="preview.totalCount === 0 || preview.invalidCount > 0 ? 'text-amber-700' : 'text-emerald-700'">
      {{ preview.totalCount > 0 ? `已读取 ${preview.totalCount} 条候选。` : "未找到可用候选，请检查来源内容或筛选条件。" }}
      <span v-if="preview.invalidCount > 0">忽略 {{ preview.invalidCount }} 条非法输入，请检查原始名单。</span>
    </p>
  </div>
</template>
