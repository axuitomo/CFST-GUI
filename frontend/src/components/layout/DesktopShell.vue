<script setup lang="ts">
import type { Component } from "vue";
import {
  PhCloud,
  PhDatabase,
  PhGear,
  PhGitBranch,
  PhGlobeHemisphereWest,
  PhMinus,
  PhSquaresFour,
  PhSquare,
  PhTable,
  PhX,
} from "@phosphor-icons/vue";
import { Quit, WindowMinimise, WindowToggleMaximise } from "../../../wailsjs/runtime/runtime";

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

function contentClass(view: ViewName) {
  return view === "results" || view === "dns" ? "app-content-wide" : "app-content";
}

function minimiseWindow() {
  WindowMinimise();
}

function toggleMaximiseWindow() {
  WindowToggleMaximise();
}

function closeWindow() {
  Quit();
}
</script>

<template>
  <main class="theme-shell app-screen hidden overflow-hidden lg:flex" :class="props.appMode === 'workflow' ? 'workflow-mode' : ''">
    <aside v-if="props.appMode === 'single'" class="theme-sidebar app-screen sticky top-0 flex w-56 shrink-0 flex-col">
      <div class="flex h-14 items-center border-b border-slate-800 px-5">
        <PhCloud class="mr-2.5 text-cf" size="24" weight="fill" />
        <span class="text-base font-bold tracking-wide text-white">CFST-GUI</span>
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
      <header class="theme-header desktop-drag-region sticky top-0 z-20 flex h-14 items-center justify-between border-b px-6 shadow-sm">
        <div class="flex min-w-0 items-center gap-4">
          <div v-if="props.appMode === 'workflow'" class="flex shrink-0 items-center">
            <PhCloud class="mr-2 text-cf" size="23" weight="fill" />
            <span class="text-sm font-bold text-slate-900">CFST-GUI</span>
          </div>
          <h1 class="min-w-0 truncate text-lg font-semibold text-slate-800">{{ props.routeTitle }}</h1>
          <div class="desktop-no-drag inline-flex rounded-lg border border-black/10 bg-white p-0.5 text-xs font-semibold text-slate-600">
            <button
              type="button"
              class="inline-flex h-8 items-center gap-1.5 rounded-md px-3 transition"
              :class="props.appMode === 'single' ? 'bg-slate-900 text-white' : 'hover:bg-slate-100'"
              @click="$emit('change-app-mode', 'single')"
            >
              <PhSquaresFour size="15" />
              单任务
            </button>
            <button
              type="button"
              class="inline-flex h-8 items-center gap-1.5 rounded-md px-3 transition"
              :class="props.appMode === 'workflow' ? 'bg-slate-900 text-white' : 'hover:bg-slate-100'"
              @click="$emit('change-app-mode', 'workflow')"
            >
              <PhGitBranch size="15" />
              工作流
            </button>
          </div>
        </div>

        <div class="desktop-no-drag flex items-center gap-1.5">
          <button
            type="button"
            class="desktop-window-control"
            aria-label="最小化"
            title="最小化"
            @click="minimiseWindow"
          >
            <PhMinus size="22" weight="bold" />
          </button>
          <button
            type="button"
            class="desktop-window-control"
            aria-label="切换窗口大小"
            title="切换窗口大小"
            @click="toggleMaximiseWindow"
          >
            <PhSquare size="19" weight="bold" />
          </button>
          <button
            type="button"
            class="desktop-window-control"
            aria-label="关闭"
            title="关闭"
            @click="closeWindow"
          >
            <PhX size="22" weight="bold" />
          </button>
        </div>
      </header>

      <div
        class="min-h-0 flex-1"
        :class="props.appMode === 'workflow' ? 'overflow-hidden bg-[rgb(247,247,247)]' : 'overflow-y-auto px-6 py-5 2xl:px-8 2xl:py-6'"
      >
        <div :class="props.appMode === 'workflow' ? 'h-full' : contentClass(props.selectedView)">
          <slot />
        </div>
      </div>
    </section>
  </main>
</template>

<style scoped>
.desktop-window-control {
  display: inline-flex;
  height: 2.5rem;
  width: 2.5rem;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 0.875rem;
  background: transparent;
  color: var(--button-ghost-text);
  transition:
    background-color 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease;
}

.desktop-window-control:hover {
  background: var(--button-ghost-hover);
}

.desktop-window-control:active {
  transform: scale(0.96);
}

.desktop-window-control:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}
</style>
