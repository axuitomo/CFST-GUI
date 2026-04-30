<script setup lang="ts">
import type { Component } from "vue";
import {
  PhCloud,
  PhDatabase,
  PhGear,
  PhGlobeHemisphereWest,
  PhPlugsConnected,
  PhSquaresFour,
} from "@phosphor-icons/vue";

type ViewName = "dashboard" | "sources" | "settings" | "dns";

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
  settings: PhGear,
  sources: PhDatabase,
};

function statusClass(tone: string) {
  if (tone === "completed" || tone === "partial" || tone === "no_results") {
    return "bg-emerald-50 text-emerald-700";
  }

  if (tone === "failed") {
    return "bg-rose-50 text-rose-700";
  }

  if (tone === "cooling") {
    return "bg-amber-50 text-amber-700";
  }

  if (tone === "running" || tone === "preparing") {
    return "bg-indigo-50 text-primary";
  }

  return "bg-slate-100 text-slate-600";
}
</script>

<template>
  <main class="hidden h-screen overflow-hidden bg-slate-50 text-slate-800 lg:flex">
    <aside class="sticky top-0 flex h-screen w-64 shrink-0 flex-col bg-slate-900 text-slate-300">
      <div class="flex h-16 items-center border-b border-slate-800 px-6">
        <PhCloud class="mr-3 text-cf" size="26" weight="fill" />
        <span class="text-lg font-bold tracking-wide text-white">CFIPTool</span>
      </div>

      <nav class="flex-1 space-y-1 overflow-y-auto px-3 py-6" aria-label="Desktop sections">
        <button
          v-for="view in props.views"
          :key="view.id"
          :class="
            props.selectedView === view.id
              ? 'bg-primary text-white'
              : 'text-slate-300 hover:bg-slate-800 hover:text-white'
          "
          class="flex w-full items-center rounded-xl px-3 py-3 text-left transition"
          type="button"
          @click="$emit('change-view', view.id)"
        >
          <component :is="iconMap[view.id]" class="mr-3 shrink-0" size="20" :weight="props.selectedView === view.id ? 'fill' : 'regular'" />
          <div>
            <p class="font-medium">{{ view.title }}</p>
            <p class="mt-0.5 text-xs text-slate-400">{{ view.copy }}</p>
          </div>
        </button>
      </nav>

      <div class="border-t border-slate-800 px-4 py-4 text-sm">
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
      <header class="sticky top-0 z-20 flex h-16 items-center justify-between border-b border-slate-200 bg-white/95 px-8 shadow-sm backdrop-blur">
        <h1 class="text-xl font-semibold text-slate-800">{{ props.routeTitle }}</h1>

        <div class="flex items-center gap-3">
          <span class="inline-flex items-center gap-1.5 rounded-full border border-slate-200 bg-slate-100 px-3 py-1 text-sm font-medium text-slate-600">
            <PhPlugsConnected size="16" />
            本地服务 127.0.0.1:3210
          </span>
        </div>
      </header>

      <div class="border-b border-slate-200 bg-white/80 px-8 py-4 backdrop-blur">
        <div class="flex items-center justify-between gap-6">
          <div class="min-w-0">
            <p class="truncate text-sm text-slate-500">{{ props.statusDetail }}</p>
            <p class="mt-1 truncate text-xs text-slate-400">
              {{ props.configPath || "配置路径将在读取后显示" }}
            </p>
          </div>

          <div class="flex shrink-0 items-center gap-3">
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

      <div class="flex-1 overflow-y-auto px-8 py-8">
        <slot />
      </div>
    </section>
  </main>
</template>
