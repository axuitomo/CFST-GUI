<script setup vapor lang="ts">
import type { SourceKind } from "../../lib/bridge";

const props = defineProps<{ compact?: boolean }>();
const emit = defineEmits<{ (event: "add", kind: SourceKind): void }>();

function addSelectedType(event: Event) {
  const select = event.target as HTMLSelectElement;
  const kind = select.value;
  if (kind === "inline" || kind === "url" || kind === "file") {
    emit("add", kind);
  }
  select.value = "";
}
</script>

<template>
  <select :class="props.compact ? 'ui-field h-9 w-9 appearance-none rounded-lg border-slate-200 bg-white px-0 py-0 text-center text-lg leading-none text-slate-700' : 'ui-field h-11 w-auto max-w-full'" :title="props.compact ? '添加输入源' : undefined" aria-label="添加输入源" value="" @change="addSelectedType">
    <option disabled value="">{{ props.compact ? "＋" : "＋ 添加输入源…" }}</option>
    <option value="inline">手动输入 IP / CIDR / 域名</option>
    <option value="url">URL 列表</option>
    <option value="file">本地文件</option>
  </select>
</template>
