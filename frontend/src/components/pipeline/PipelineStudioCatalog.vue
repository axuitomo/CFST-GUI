<script setup lang="ts">
import { computed } from "vue";
import { PhMagnifyingGlass, PhPlus } from "@phosphor-icons/vue";
import type { PipelineNodeCatalogItem, PipelineNodeType } from "../../lib/bridge";
import { nodeTypeLabel } from "../../lib/pipelineStudio";

const props = defineProps<{
  items: PipelineNodeCatalogItem[];
  search: string;
}>();

const emit = defineEmits<{
  (event: "add", item: PipelineNodeCatalogItem): void;
  (event: "update:search", value: string): void;
}>();

const groupedItems = computed(() => {
  const keyword = props.search.trim().toLowerCase();
  const filtered = props.items.filter((item) => {
    if (!keyword) {
      return true;
    }
    return [item.display_name, item.action, item.description || "", nodeTypeLabel(item.node_type)]
      .join(" ")
      .toLowerCase()
      .includes(keyword);
  });
  const groupOrder: PipelineNodeType[] = ["source", "probe", "filter", "branch", "deliver", "recovery", "end"];
  return groupOrder
    .map((nodeType) => ({
      items: filtered.filter((item) => item.node_type === nodeType).sort((a, b) => (a.action === "check_output" ? -1 : 0) - (b.action === "check_output" ? -1 : 0)),
      nodeType,
      title: nodeTypeLabel(nodeType),
    }))
    .filter((group) => group.items.length > 0);
});
</script>

<template>
  <aside class="flex min-h-0 flex-col overflow-hidden rounded-[1.75rem] border border-slate-200 bg-white shadow-panel">
    <div class="border-b border-slate-200 px-4 py-4">
      <p class="text-sm font-semibold text-slate-800">节点库</p>
      <p class="mt-1 text-xs text-slate-500">搜索后点击即可添加到画布。</p>
      <label class="relative mt-3 block">
        <PhMagnifyingGlass class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size="16" />
        <input
          :value="search"
          class="ui-field !rounded-2xl !pl-10"
          placeholder="搜索节点、动作或类型"
          @input="emit('update:search', ($event.target as HTMLInputElement).value)"
        />
      </label>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto px-3 py-3">
      <div v-if="groupedItems.length === 0" class="rounded-2xl border border-dashed border-slate-200 px-4 py-8 text-center text-sm text-slate-400">
        没找到匹配的节点。
      </div>

      <div v-else class="space-y-4">
        <section v-for="group in groupedItems" :key="group.nodeType" class="space-y-2">
          <div class="flex items-center justify-between gap-2 px-1">
            <p class="text-xs font-semibold uppercase tracking-[0.14em] text-slate-400">{{ group.title }}</p>
            <span class="text-xs text-slate-400">{{ group.items.length }}</span>
          </div>

          <button
            v-for="item in group.items"
            :key="item.action"
            type="button"
            class="w-full rounded-2xl border border-slate-200 bg-slate-50/70 px-3 py-3 text-left transition hover:-translate-y-0.5 hover:border-slate-300 hover:bg-white"
            @click="emit('add', item)"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <p class="text-sm font-semibold text-slate-800">{{ item.display_name }}</p>
                <p class="mt-1 text-xs text-slate-500">{{ item.action }}</p>
              </div>
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-primary">
                <PhPlus size="16" />
              </span>
            </div>
            <p v-if="item.description" class="mt-3 text-xs leading-5 text-slate-500">{{ item.description }}</p>
          </button>
        </section>
      </div>
    </div>
  </aside>
</template>
