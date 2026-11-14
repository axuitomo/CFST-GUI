<script setup vapor lang="ts">
import { computed } from "vue";

// 自动保存以前是静默的：用户改完设置不知道有没有落盘，只能反复点一下确认。
const { state } = defineProps<{
  state: "error" | "idle" | "saved" | "saving";
}>();

const shown = computed(() => {
  if (state === "saving") return { dot: "bg-amber-400", text: "保存中…" };
  if (state === "error") return { dot: "bg-rose-500", text: "保存失败" };
  if (state === "saved") return { dot: "bg-emerald-500", text: "已保存" };
  return null;
});
</script>

<template>
  <span v-if="shown" class="inline-flex shrink-0 items-center gap-1.5 text-xs text-slate-500" role="status">
    <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="shown.dot"></span>
    {{ shown.text }}
  </span>
</template>
