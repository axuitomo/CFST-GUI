<script setup vapor lang="ts">
import type { ColoFilterMode, SourceIPMode } from "../../lib/bridge";
import type { SourceGroupIPMode } from "../../lib/sourceGroups";

defineProps<{ mode: SourceGroupIPMode; canCopy: boolean; coloFilter: string; coloFilterMode: ColoFilterMode | "" }>();
const emit = defineEmits<{
  (event: "mode-change", mode: SourceIPMode): void;
  (event: "colo-filter-change", filter: string): void;
  (event: "colo-filter-mode-change", mode: ColoFilterMode): void;
  (event: "copy"): void;
}>();
function changeMode(event: Event) {
  const mode = (event.target as HTMLSelectElement).value;
  if (mode === "traverse" || mode === "mcis") emit("mode-change", mode);
}
function changeColoMode(mode: ColoFilterMode) {
  emit("colo-filter-mode-change", mode);
}
function changeColoFilter(event: Event) {
  emit("colo-filter-change", (event.target as HTMLInputElement).value);
}
</script>

<template>
  <div class="grid gap-3 border-t border-slate-100 px-4 py-3 sm:px-5 md:grid-cols-2">
    <label class="min-w-0">
      <span class="ui-label">输入组 IP 模式</span>
      <select :value="mode" class="ui-field h-11" @change="changeMode">
        <option v-if="!mode" value="" disabled>旧组含不同模式，请选择统一模式</option>
        <option value="traverse">逐个检查</option>
        <option value="mcis">随机抽查</option>
      </select>
    </label>
    <label class="min-w-0">
      <span class="ui-label">节点筛选（可选）</span>
      <input :value="coloFilter" placeholder="JP,HKG,NRT,US,UK" type="text" class="ui-field h-11 font-mono" @input="changeColoFilter" />
    </label>
    <div class="flex min-w-0 items-center justify-between gap-3">
      <div class="flex shrink-0 gap-1 rounded-lg border border-slate-200 bg-slate-100 p-1" role="group" aria-label="节点筛选模式">
        <button type="button" class="ui-button h-9 px-3" :class="coloFilterMode === 'allow' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'" @click="changeColoMode('allow')">白名单</button>
        <button type="button" class="ui-button h-9 px-3" :class="coloFilterMode === 'deny' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'" @click="changeColoMode('deny')">黑名单</button>
      </div>
      <button type="button" class="ui-button ui-button-ghost h-11 shrink-0 whitespace-nowrap" :disabled="!canCopy" @click.stop="emit('copy')">复制当前组并编辑</button>
    </div>
    <div class="space-y-1 text-xs md:col-span-2">
      <p v-if="!coloFilterMode" class="text-amber-700" role="status">旧组含有来源级筛选；选择模式后会统一为输入组设置。</p>
      <p v-else class="text-slate-500">空列表不限制；国家码遵循 ISO 3166-1 alpha-2，UK 兼容为 GB。</p>
      <p v-if="!mode" class="text-amber-700" role="status">旧配置中的来源模式不同，暂按原设置运行。选择后将统一应用到本组所有来源。</p>
      <p v-else class="text-slate-500">{{ mode === "mcis" ? "本组所有来源先随机抽查候选，再进行最终测速。" : "本组所有来源按顺序展开并整理候选 IP。" }} 新增来源自动沿用此模式。</p>
    </div>
  </div>
</template>
