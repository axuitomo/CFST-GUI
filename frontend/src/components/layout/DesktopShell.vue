<script setup lang="ts">
import type { Component } from "vue";
import {
  PhCloud,
  PhDatabase,
  PhGear,
  PhGlobeHemisphereWest,
  PhPlugsConnected,
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
  configPath: string;
  loading: boolean;
  routeTitle: string;
  selectedView: ViewName;
  statusDetail: string;
  statusLabel: string;
  statusTone: string;
  views: RouteItem[];
}>();

defineEmits<{
  (event: "change-view", view: ViewName): void;
  (event: "persist-config"): void;
  (event: "refresh-config"): void;
}>();

const iconMap: Record<ViewName, Component> = {
  dashboard: PhSquaresFour,
  dns: PhGlobeHemisphereWest,
  results: PhTable,
  settings: PhGear,
  sources: PhDatabase,
};

function contentClass(view: ViewName) {
  return view === "results" || view === "dns" ? "app-content-wide" : "app-content";
}

function statusClass(tone: string) {
  if (tone === "completed" || tone === "partial" || tone === "no_results") {
    return "ui-pill-success";
  }

  if (tone === "failed") {
    return "ui-pill-danger";
  }

  if (tone === "cooling" || tone === "warning") {
    return "ui-pill-warning";
  }

  if (tone === "running" || tone === "preparing") {
    return "ui-pill-info";
  }

  return "ui-pill-neutral";
}
</script>

<template>
  <main class="theme-shell app-screen hidden overflow-hidden lg:flex">
    <aside class="theme-sidebar app-screen sticky top-0 flex w-56 shrink-0 flex-col">
      <div class="flex h-14 items-center border-b border-slate-800 px-5">
        <PhCloud class="mr-2.5 text-cf" size="24" weight="fill" />
        <span class="text-base font-bold tracking-wide text-white">CFIPTool</span>
      </div>

      <nav class="flex-1 space-y-1 overflow-y-auto px-2.5 py-4" aria-label="Desktop sections">
        <button
          v-for="view in props.views"
          :key="view.id"
          :class="
            props.selectedView === view.id
              ? 'bg-primary text-white'
              : 'text-slate-300 hover:bg-slate-800 hover:text-white'
          "
          class="flex w-full items-center rounded-lg px-3 py-2.5 text-left transition"
          type="button"
          @click="$emit('change-view', view.id)"
        >
          <component :is="iconMap[view.id]" class="mr-2.5 shrink-0" size="19" :weight="props.selectedView === view.id ? 'fill' : 'regular'" />
          <div class="min-w-0">
            <p class="truncate text-sm font-medium">{{ view.title }}</p>
            <p class="mt-0.5 text-xs text-slate-400">{{ view.copy }}</p>
          </div>
        </button>
      </nav>

      <div class="border-t border-slate-800 px-3 py-3 text-xs">
        <div class="flex items-center justify-between">
          <div class="flex items-center">
            <span class="mr-2 h-2 w-2 animate-pulse rounded-full bg-emerald-500"></span>
            <span>服务已连接</span>
          </div>
          <span class="text-xs text-slate-500">桌面版</span>
        </div>
      </div>
    </aside>

    <section class="flex min-w-0 flex-1 flex-col overflow-hidden">
      <header class="theme-header sticky top-0 z-20 flex h-14 items-center justify-between border-b px-6 shadow-sm backdrop-blur">
        <h1 class="text-lg font-semibold text-slate-800">{{ props.routeTitle }}</h1>

        <div class="flex items-center gap-2">
          <span class="ui-pill ui-pill-neutral inline-flex items-center gap-1.5 whitespace-nowrap">
            <PhPlugsConnected size="16" />
            本地服务 127.0.0.1:3210
          </span>
        </div>
      </header>

      <div class="theme-subheader border-b px-6 py-3 backdrop-blur">
        <div class="flex items-center justify-between gap-4">
          <div class="min-w-0">
            <p class="truncate text-sm text-slate-500">{{ props.statusDetail }}</p>
            <p class="mt-1 truncate text-xs text-slate-400">
              {{ props.configPath || "配置路径将在读取后显示" }}
            </p>
          </div>

          <div class="flex shrink-0 items-center gap-2">
            <span :class="statusClass(props.statusTone)" class="ui-pill">
              {{ props.statusLabel }}
            </span>
            <button
              type="button"
              class="ui-button ui-button-ghost"
              :disabled="props.loading"
              @click="$emit('refresh-config')"
            >
              读取配置
            </button>
            <button
              type="button"
              class="ui-button ui-button-primary"
              :disabled="props.loading"
              @click="$emit('persist-config')"
            >
              保存配置
            </button>
          </div>
        </div>
      </div>

      <div class="min-h-0 flex-1 overflow-y-auto px-6 py-5 2xl:px-8 2xl:py-6">
        <div :class="contentClass(props.selectedView)">
          <slot />
        </div>
      </div>
    </section>
  </main>
</template>
