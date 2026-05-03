<script setup lang="ts">
import type { Component } from "vue";
import {
  PhCloud,
  PhDatabase,
  PhGear,
  PhGlobeHemisphereWest,
  PhSquaresFour,
  PhTable,
} from "@phosphor-icons/vue";

type ViewName = "dashboard" | "results" | "sources" | "settings" | "dns";

interface RouteItem {
  copy: string;
  id: ViewName;
  shortLabel: string;
  title: string;
}

const props = defineProps<{
  routeTitle: string;
  selectedView: ViewName;
  views: RouteItem[];
}>();

defineEmits<{
  (event: "change-view", view: ViewName): void;
}>();

const iconMap: Record<ViewName, Component> = {
  dashboard: PhSquaresFour,
  dns: PhGlobeHemisphereWest,
  results: PhTable,
  settings: PhGear,
  sources: PhDatabase,
};
</script>

<template>
  <div class="flex h-screen flex-col overflow-hidden bg-slate-50 text-slate-800 lg:hidden">
    <header class="fixed inset-x-0 top-0 z-40 flex h-14 items-center justify-between border-b border-slate-200 bg-white px-4 shadow-sm">
      <div class="flex items-center">
        <PhCloud class="mr-2 text-cf" size="24" weight="fill" />
        <span class="font-bold text-slate-800">{{ props.routeTitle }}</span>
      </div>
    </header>

    <main class="no-scrollbar flex-1 overflow-y-auto pt-14 pb-16">
      <div class="p-4">
        <slot />
      </div>
    </main>

    <nav class="pb-safe fixed inset-x-0 bottom-0 z-50 flex h-16 items-center justify-around border-t border-slate-200 bg-white">
      <button
        v-for="view in props.views"
        :key="view.id"
        :class="props.selectedView === view.id ? 'text-primary' : 'text-slate-400'"
        class="flex h-full w-full flex-col items-center justify-center gap-1 transition"
        type="button"
        @click="$emit('change-view', view.id)"
      >
        <component :is="iconMap[view.id]" size="24" :weight="props.selectedView === view.id ? 'fill' : 'regular'" />
        <span class="text-[11px] font-medium">{{ view.shortLabel }}</span>
      </button>
    </nav>
  </div>
</template>
