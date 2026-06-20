<script setup lang="ts">
import type { Component } from "vue";
import { computed, ref, watch } from "vue";
import { PhCaretLeft, PhCaretRight, PhDatabase, PhGear, PhGitBranch, PhGlobeHemisphereWest, PhMinus, PhSquaresFour, PhSquare, PhTable, PhX } from "@phosphor-icons/vue";
import { Quit, WindowMinimise, WindowToggleMaximise } from "../../../wailsjs/runtime/runtime";

type AppMode = "single" | "workflow";
type ViewName = "dashboard" | "results" | "sources" | "settings" | "dns";
const SIDEBAR_COLLAPSED_STORAGE_KEY = "cfst.desktop.sidebarCollapsed.v1";

interface RouteItem {
  copy: string;
  id: ViewName;
  shortLabel: string;
  title: string;
}

const { appMode, currentVersion, routeTitle, selectedView, views } = defineProps<{
  appMode: AppMode;
  currentVersion: string;
  routeTitle: string;
  selectedView: ViewName;
  views: RouteItem[];
}>();

const emit = defineEmits<{
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

const sidebarCollapsed = ref(loadSidebarCollapsed());
const appVersionLabel = computed(() => formatAppVersion(currentVersion));

watch(sidebarCollapsed, (collapsed) => {
  try {
    window.localStorage.setItem(SIDEBAR_COLLAPSED_STORAGE_KEY, collapsed ? "true" : "false");
  } catch {
    // Ignore storage failures so the sidebar remains usable in restricted runtimes.
  }
});

function contentClass(view: ViewName) {
  return view === "results" || view === "dns" ? "app-content-wide" : "app-content";
}

function loadSidebarCollapsed() {
  try {
    return window.localStorage.getItem(SIDEBAR_COLLAPSED_STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

function toggleSidebarCollapsed() {
  sidebarCollapsed.value = !sidebarCollapsed.value;
}

function formatAppVersion(version: string) {
  const value = version.trim();
  if (!value) {
    return "V1.0";
  }
  return value.toLowerCase().startsWith("v") ? `V${value.slice(1)}` : `V${value}`;
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
  <main class="theme-shell app-screen hidden overflow-hidden lg:flex" :class="appMode === 'workflow' ? 'workflow-mode' : ''">
    <aside v-if="appMode === 'single'" class="theme-sidebar app-screen sticky top-0 flex shrink-0 flex-col transition-[width] duration-200 ease-out" :class="sidebarCollapsed ? 'w-20' : 'w-56'">
      <div class="relative flex h-14 items-center border-b border-slate-800" :class="sidebarCollapsed ? 'justify-center px-3' : 'justify-between px-5'">
        <div class="flex min-w-0 items-center" :class="sidebarCollapsed ? 'justify-center' : ''">
          <img src="/favicon.png" alt="" class="h-6 w-6 shrink-0 rounded-md" :class="sidebarCollapsed ? '' : 'mr-2.5'" />
          <span v-if="!sidebarCollapsed" class="truncate text-base font-bold tracking-wide text-white">CFST-GUI</span>
        </div>
        <button type="button" class="desktop-sidebar-toggle desktop-no-drag" :class="sidebarCollapsed ? 'absolute right-1' : ''" :aria-label="sidebarCollapsed ? '展开侧边栏' : '折叠侧边栏'" :title="sidebarCollapsed ? '展开侧边栏' : '折叠侧边栏'" @click="toggleSidebarCollapsed">
          <PhCaretRight v-if="sidebarCollapsed" size="17" weight="bold" />
          <PhCaretLeft v-else size="17" weight="bold" />
        </button>
      </div>

      <nav class="flex-1 space-y-1 overflow-y-auto px-2.5 py-4" aria-label="Desktop sections">
        <button
          v-for="view in views"
          :key="view.id"
          :class="['flex w-full items-center rounded-lg text-left transition', selectedView === view.id ? 'bg-primary text-white' : 'text-slate-300 hover:bg-slate-800 hover:text-white', sidebarCollapsed ? 'h-14 justify-center px-0' : 'px-3 py-2.5']"
          type="button"
          :aria-label="view.title"
          :title="sidebarCollapsed ? view.title : undefined"
          @click="emit('change-view', view.id)"
        >
          <component :is="iconMap[view.id]" class="shrink-0" :class="sidebarCollapsed ? '' : 'mr-2.5'" size="19" :weight="selectedView === view.id ? 'fill' : 'regular'" />
          <div v-if="!sidebarCollapsed" class="min-w-0">
            <p class="truncate text-sm font-medium">{{ view.title }}</p>
            <p class="mt-0.5 text-xs text-slate-400">{{ view.copy }}</p>
          </div>
        </button>
      </nav>

      <div class="border-t border-slate-800 px-3 py-3 text-xs">
        <div v-if="sidebarCollapsed" class="flex items-center justify-center">
          <span class="truncate text-[11px] font-semibold text-slate-500" :title="`桌面版 ${appVersionLabel}`">{{ appVersionLabel }}</span>
        </div>
        <div v-else class="flex items-center justify-between">
          <div class="flex items-center" :class="sidebarCollapsed ? 'justify-center' : ''">
            <span class="mr-2 h-2 w-2 animate-pulse rounded-full bg-emerald-500"></span>
            <span>服务已连接</span>
          </div>
          <span class="shrink-0 text-xs text-slate-500">桌面版 {{ appVersionLabel }}</span>
        </div>
      </div>
    </aside>

    <section class="flex min-w-0 flex-1 flex-col overflow-hidden">
      <header class="theme-header desktop-drag-region sticky top-0 z-20 flex h-14 items-center justify-between border-b px-6 shadow-sm">
        <div class="flex min-w-0 items-center gap-4">
          <div v-if="appMode === 'workflow'" class="flex shrink-0 items-center">
            <img src="/favicon.png" alt="" class="mr-2 h-[23px] w-[23px] rounded-md" />
            <span class="text-sm font-bold text-slate-900">CFST-GUI</span>
          </div>
          <h1 class="min-w-0 truncate text-lg font-semibold text-slate-800">{{ routeTitle }}</h1>
          <div class="desktop-no-drag inline-flex rounded-lg border border-black/10 bg-white p-0.5 text-xs font-semibold text-slate-600">
            <button type="button" class="inline-flex h-8 items-center gap-1.5 rounded-md px-3 transition" :class="appMode === 'single' ? 'bg-slate-900 text-white' : 'hover:bg-slate-100'" @click="emit('change-app-mode', 'single')">
              <PhSquaresFour size="15" />
              单任务
            </button>
            <button type="button" class="inline-flex h-8 items-center gap-1.5 rounded-md px-3 transition" :class="appMode === 'workflow' ? 'bg-slate-900 text-white' : 'hover:bg-slate-100'" @click="emit('change-app-mode', 'workflow')">
              <PhGitBranch size="15" />
              工作流
            </button>
          </div>
        </div>

        <div class="desktop-no-drag flex items-center gap-1.5">
          <button type="button" class="desktop-window-control" aria-label="最小化" title="最小化" @click="minimiseWindow">
            <PhMinus size="22" weight="bold" />
          </button>
          <button type="button" class="desktop-window-control" aria-label="切换窗口大小" title="切换窗口大小" @click="toggleMaximiseWindow">
            <PhSquare size="19" weight="bold" />
          </button>
          <button type="button" class="desktop-window-control" aria-label="关闭" title="关闭" @click="closeWindow">
            <PhX size="22" weight="bold" />
          </button>
        </div>
      </header>

      <div class="min-h-0 flex-1" :class="appMode === 'workflow' ? 'overflow-hidden bg-[rgb(247,247,247)]' : 'overflow-y-auto px-6 py-5 2xl:px-8 2xl:py-6'">
        <div :class="appMode === 'workflow' ? 'h-full' : contentClass(selectedView)">
          <slot />
        </div>
      </div>
    </section>
  </main>
</template>

<style scoped>
.desktop-sidebar-toggle {
  display: inline-flex;
  height: 1.25rem;
  width: 1.25rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 0.375rem;
  background: transparent;
  color: var(--sidebar-text-muted);
  transition:
    background-color 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease;
}

.desktop-sidebar-toggle:hover {
  background: var(--sidebar-hover-bg);
  color: var(--text-inverse);
}

.desktop-sidebar-toggle:active {
  transform: scale(0.96);
}

.desktop-sidebar-toggle:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

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
