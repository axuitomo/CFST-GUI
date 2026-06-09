<script setup lang="ts">
import { computed } from "vue";
import type { Component } from "vue";
import { PhDatabase, PhGear, PhGlobeHemisphereWest, PhSquaresFour, PhTable } from "@phosphor-icons/vue";

type AppMode = "single" | "workflow";
type ViewName = "dashboard" | "results" | "sources" | "settings" | "dns";

interface RouteItem {
  copy: string;
  id: ViewName;
  shortLabel: string;
  title: string;
}

const props = defineProps<{
  appMode: AppMode;
  hideWorkflow?: boolean;
  routeTitle: string;
  selectedView: ViewName;
  views: RouteItem[];
}>();

defineEmits<{
  (event: "change-app-mode", mode: AppMode): void;
  (event: "change-view", view: ViewName): void;
}>();

const iconMap: Record<ViewName, Component> = {
  dashboard: PhSquaresFour,
  dns: PhGlobeHemisphereWest,
  results: PhTable,
  settings: PhGear,
  sources: PhDatabase,
};

const effectiveAppMode = computed<AppMode>(() => (props.hideWorkflow ? "single" : props.appMode));
</script>

<template>
  <div class="theme-shell app-screen flex flex-col overflow-hidden lg:hidden" :class="effectiveAppMode === 'workflow' ? 'workflow-mode' : ''">
    <header class="theme-header fixed inset-x-0 top-0 z-40 flex h-24 flex-col justify-center gap-2 border-b px-4 shadow-sm">
      <div class="flex items-center justify-between gap-3">
        <div class="flex min-w-0 items-center">
          <img src="/favicon.png" alt="" class="mr-2 h-6 w-6 shrink-0 rounded-md" />
          <span class="truncate font-bold text-slate-800">{{ props.routeTitle }}</span>
        </div>
        <div v-if="!props.hideWorkflow" class="inline-flex shrink-0 rounded-lg border border-black/10 bg-white p-0.5 text-xs font-semibold text-slate-600">
          <button type="button" class="inline-flex h-8 items-center gap-1.5 rounded-md px-2.5 transition" :class="effectiveAppMode === 'single' ? 'bg-slate-900 text-white' : 'hover:bg-slate-100'" @click="$emit('change-app-mode', 'single')">
            <PhSquaresFour size="15" />
            单任务
          </button>
        </div>
      </div>
    </header>

    <main class="no-scrollbar min-h-0 flex-1 overflow-y-auto pt-24" :class="effectiveAppMode === 'single' ? 'touch-bottom-buffer' : 'bg-[rgb(247,247,247)]'">
      <div class="mx-auto w-full p-4 md:p-5" :class="effectiveAppMode === 'workflow' ? 'max-w-none' : 'max-w-[52rem]'">
        <slot />
      </div>
    </main>

    <nav v-if="effectiveAppMode === 'single'" class="theme-nav pb-safe fixed inset-x-0 bottom-0 z-50 flex min-h-16 items-center justify-around border-t">
      <button v-for="view in props.views" :key="view.id" :class="props.selectedView === view.id ? 'text-primary' : 'text-slate-400'" class="flex min-h-16 flex-1 flex-col items-center justify-center gap-1 py-2 transition" type="button" @click="$emit('change-view', view.id)">
        <component :is="iconMap[view.id]" size="24" :weight="props.selectedView === view.id ? 'fill' : 'regular'" />
        <span class="text-[11px] font-medium">{{ view.shortLabel }}</span>
      </button>
    </nav>
  </div>
</template>
