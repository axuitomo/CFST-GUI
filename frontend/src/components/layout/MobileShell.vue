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
  <div class="theme-shell app-screen flex flex-col overflow-hidden lg:hidden">
    <header class="theme-header fixed inset-x-0 top-0 z-40 flex h-14 items-center justify-between border-b px-4 shadow-sm">
      <div class="flex items-center">
        <PhCloud class="mr-2 text-cf" size="24" weight="fill" />
        <span class="font-bold text-slate-800">{{ props.routeTitle }}</span>
      </div>
    </header>

    <main class="no-scrollbar touch-bottom-buffer min-h-0 flex-1 overflow-y-auto pt-14">
      <div class="mx-auto w-full max-w-[52rem] p-4 md:p-5">
        <slot />
      </div>
    </main>

    <nav class="theme-nav pb-safe fixed inset-x-0 bottom-0 z-50 flex min-h-16 items-center justify-around border-t">
      <button
        v-for="view in props.views"
        :key="view.id"
        :class="props.selectedView === view.id ? 'text-primary' : 'text-slate-400'"
        class="flex min-h-16 flex-1 flex-col items-center justify-center gap-1 py-2 transition"
        type="button"
        @click="$emit('change-view', view.id)"
      >
        <component :is="iconMap[view.id]" size="24" :weight="props.selectedView === view.id ? 'fill' : 'regular'" />
        <span class="text-[11px] font-medium">{{ view.shortLabel }}</span>
      </button>
    </nav>
  </div>
</template>
