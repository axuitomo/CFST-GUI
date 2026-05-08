<script setup lang="ts">
import { computed, ref } from "vue";
import { PhArrowsClockwise, PhDatabase, PhEye, PhFloppyDisk, PhFolderOpen, PhPlus, PhTrash } from "@phosphor-icons/vue";

interface SourceEntry {
  colo_filter: string;
  content: string;
  enabled: boolean;
  id: string;
  ip_limit: number;
  ip_mode: "traverse" | "mcis";
  kind: "inline" | "file" | "url";
  last_fetched_at: string;
  last_fetched_count: number;
  name: string;
  path: string;
  status_text: string;
  url: string;
}

interface PreviewState {
  action: string;
  entries: string[];
  invalidCount: number;
  totalCount: number;
  updatedAt: string;
  warnings: string[];
}

interface ColoDictionaryStatus {
  colo_ipv4_path: string;
  colo_ipv4_rows: number;
  colo_ipv6_path: string;
  colo_ipv6_rows: number;
  colo_path: string;
  colo_rows: number;
  geofeed_path: string;
  geofeed_rows: number;
  last_updated_at: string;
  matched_rows: number;
  missing_rows: number;
  source_url: string;
  updated: boolean;
  unmatched_rows: number;
}

interface SourceProfileItem {
  created_at: string;
  id: string;
  name: string;
  sources: SourceEntry[];
  updated_at: string;
}

interface SourceProfileStore {
  active_profile_id: string;
  items: SourceProfileItem[];
  schema_version: string;
  updated_at: string;
}

const props = defineProps<{
  accepted: number;
  coloDictionaryProcessing: boolean;
  coloDictionaryStatus: ColoDictionaryStatus | null;
  coloDictionaryUpdating: boolean;
  invalid: number;
  platform: "desktop" | "mobile";
  preparedCount: number;
  previewStates: Record<string, PreviewState | undefined>;
  requestStates: Record<string, string | undefined>;
  sourceProfiles: SourceProfileStore;
  sources: SourceEntry[];
  taskStage: string;
}>();

const enabledCount = computed(() => props.sources.filter((source) => source.enabled).length);
const mcisCount = computed(() => props.sources.filter((source) => source.ip_mode === "mcis").length);
const sourceProfileNameDraft = ref("");
const coloDictionaryExpanded = ref(props.platform === "desktop");
const activeSourceProfile = computed(
  () => props.sourceProfiles.items.find((profile) => profile.id === props.sourceProfiles.active_profile_id) || null,
);

function sourceTypeLabel(kind: SourceEntry["kind"]) {
  if (kind === "inline") {
    return "手动输入";
  }
  if (kind === "file") {
    return "本地文件";
  }
  return "URL 列表";
}

function sourceFieldLabel(kind: SourceEntry["kind"]) {
  if (kind === "inline") {
    return "输入内容";
  }
  if (kind === "file") {
    return "文件路径";
  }
  return "URL";
}

function sourceModeCopy(mode: SourceEntry["ip_mode"]) {
  return mode === "mcis" ? "MICS抽样先探索候选，再交给当前 CFST 做最终测速" : "按顺序展开并整理来源中的候选 IP";
}

function sourceStatusText(source: SourceEntry) {
  if (!source.enabled) {
    return "已停用，启动任务时不会读取该输入源。";
  }

  if (source.status_text.trim()) {
    return source.status_text.trim();
  }

  if (source.kind === "url") {
    return "尚未抓取远程列表。";
  }

  if (source.kind === "file") {
    return "尚未读取本地文件。";
  }

  return "尚未整理手动输入。";
}

function sourcePreviewState(sourceId: string) {
  return props.previewStates[sourceId];
}

function sourceRequestState(sourceId: string) {
  return props.requestStates[sourceId] || "";
}

function dictionaryUpdatedAt() {
  return props.coloDictionaryStatus?.last_updated_at || "尚未更新";
}

const emit = defineEmits<{
  (event: "add"): void;
  (event: "delete-source-profile", profileId: string): void;
  (event: "detect-source-name", sourceId: string): void;
  (event: "fetch-source", sourceId: string): void;
  (event: "process-colo-dictionary"): void;
  (event: "preview", sourceId: string): void;
  (event: "refresh-colo-dictionary"): void;
  (event: "remove", sourceId: string): void;
  (event: "save"): void;
  (event: "save-source-profile", name: string, profileId?: string, sources?: SourceEntry[], setActive?: boolean): void;
  (event: "select-file", sourceId: string): void;
  (event: "switch-source-profile", profileId: string): void;
}>();

function renameSourceProfile(profile: SourceProfileItem) {
  const nextName = window.prompt("新的输入源档案名称", profile.name)?.trim();
  if (!nextName || nextName === profile.name) {
    return;
  }
  emit("save-source-profile", nextName, profile.id, profile.sources, profile.id === props.sourceProfiles.active_profile_id);
}

function duplicateSourceProfile(profile: SourceProfileItem) {
  emit("save-source-profile", `${profile.name} 副本`, "", profile.sources, false);
}
</script>

<template>
  <section v-if="platform === 'desktop'" class="space-y-6">
    <div class="flex items-end justify-between gap-4">
      <div>
        <h2 class="text-lg font-semibold text-slate-800">输入源管理</h2>
        <p class="mt-1 text-sm text-slate-500">输入源会跟随全局配置一起保存，每个来源都可以独立设置 IP 上限与 IP 模式。</p>
      </div>
      <button type="button" class="ui-button ui-button-secondary" @click="$emit('add')">
        <PhPlus size="18" />
        新增输入源
      </button>
    </div>

    <article class="ui-card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-slate-50/70 px-6 py-4">
        <div>
          <h3 class="flex items-center text-lg font-semibold text-slate-800">
            <PhFloppyDisk class="mr-2 text-primary" size="20" weight="fill" />
            输入源配置档案
          </h3>
          <p class="mt-1 text-xs text-slate-500">只保存和切换输入源列表，不影响测速、Cloudflare 和导出设置。</p>
        </div>
        <span class="ui-pill ui-pill-subtle">{{ activeSourceProfile?.name || "未选择档案" }}</span>
      </div>
      <div class="grid gap-4 p-6 lg:grid-cols-[minmax(0,1fr)_auto]">
        <label class="min-w-0">
          <span class="ui-label">保存当前输入源为档案</span>
          <input v-model="sourceProfileNameDraft" class="ui-field" placeholder="例如：VPS789 组合 / 自建源" type="text" />
        </label>
        <div class="flex flex-wrap items-end gap-3">
          <button type="button" class="ui-button ui-button-primary" @click="emit('save-source-profile', sourceProfileNameDraft)">
            <PhFloppyDisk size="18" weight="fill" />
            保存档案
          </button>
          <button
            type="button"
            class="ui-button ui-button-ghost"
            :disabled="!activeSourceProfile"
            @click="activeSourceProfile && emit('save-source-profile', activeSourceProfile.name, activeSourceProfile.id)"
          >
            更新当前档案
          </button>
        </div>
      </div>
      <div v-if="sourceProfiles.items.length > 0" class="grid gap-3 border-t border-slate-100 p-6 pt-4 lg:grid-cols-2">
        <div
          v-for="profile in sourceProfiles.items"
          :key="profile.id"
          class="flex items-center justify-between gap-3 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3"
        >
          <div class="min-w-0">
            <p class="truncate text-sm font-medium text-slate-700">{{ profile.name }}</p>
            <p class="text-xs text-slate-400">{{ profile.id === sourceProfiles.active_profile_id ? "当前输入源档案" : profile.updated_at || "未记录更新时间" }}</p>
          </div>
          <div class="flex shrink-0 gap-2">
            <button type="button" class="ui-button ui-button-ghost px-3 py-2" :disabled="profile.id === sourceProfiles.active_profile_id" @click="emit('switch-source-profile', profile.id)">切换</button>
            <button type="button" class="ui-button ui-button-ghost px-3 py-2" @click="renameSourceProfile(profile)">重命名</button>
            <button type="button" class="ui-button ui-button-ghost px-3 py-2" @click="duplicateSourceProfile(profile)">复制</button>
            <button type="button" class="ui-button ui-button-ghost px-3 py-2" @click="emit('delete-source-profile', profile.id)">删除</button>
          </div>
        </div>
      </div>
    </article>

    <article class="ui-card p-6">
      <div class="grid gap-4 md:grid-cols-4">
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">全部来源</p>
          <p class="mt-2 text-2xl font-semibold text-slate-800">{{ sources.length }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">已启用</p>
          <p class="mt-2 text-2xl font-semibold text-slate-800">{{ enabledCount }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">待执行来源</p>
          <p class="mt-2 text-2xl font-semibold text-slate-800">{{ preparedCount }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">MICS抽样来源</p>
          <p class="mt-2 text-2xl font-semibold text-slate-800">{{ mcisCount }}</p>
        </div>
      </div>
      <div class="mt-4 flex flex-wrap gap-3 text-sm text-slate-500">
        <span>当前阶段：{{ taskStage || "idle" }}</span>
        <span>任务已接受：{{ accepted }}</span>
        <span>非法条目：{{ invalid }}</span>
      </div>
    </article>

    <article class="ui-card p-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h3 class="text-base font-semibold text-slate-800">COLO 词典</h3>
          <p class="mt-1 text-sm text-slate-500">先拉取 Cloudflare GEOFEED 与辅助映射，再本地生成 COLO 文件供输入源预筛使用。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="ui-button ui-button-ghost" @click="coloDictionaryExpanded = !coloDictionaryExpanded">
            {{ coloDictionaryExpanded ? "收起" : "展开" }}
          </button>
          <button
            type="button"
            class="ui-button ui-button-secondary"
            :disabled="coloDictionaryUpdating"
            @click="$emit('refresh-colo-dictionary')"
          >
            <PhArrowsClockwise size="18" />
            {{ coloDictionaryUpdating ? "拉取中" : "更新词典" }}
          </button>
          <button
            type="button"
            class="ui-button ui-button-primary"
            :disabled="coloDictionaryProcessing"
            @click="$emit('process-colo-dictionary')"
          >
            <PhArrowsClockwise size="18" />
            {{ coloDictionaryProcessing ? "处理中" : "处理词典" }}
          </button>
        </div>
      </div>
      <div v-if="coloDictionaryExpanded" class="mt-4 grid gap-4 md:grid-cols-6">
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">GEOFEED</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.geofeed_rows || 0 }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">综合</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_rows || 0 }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">IPv4</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_ipv4_rows || 0 }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">IPv6</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_ipv6_rows || 0 }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">未覆盖</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.unmatched_rows || 0 }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">更新时间</p>
          <p class="mt-2 break-all text-sm text-slate-700">{{ dictionaryUpdatedAt() }}</p>
        </div>
      </div>
      <div v-if="coloDictionaryExpanded" class="mt-4 space-y-1 text-xs text-slate-500">
        <p class="break-all">GEOFEED：{{ coloDictionaryStatus?.geofeed_path || "local-ip-ranges.csv" }}</p>
        <p class="break-all">综合：{{ coloDictionaryStatus?.colo_path || "cloudflare-colos.csv" }}</p>
        <p class="break-all">IPv4：{{ coloDictionaryStatus?.colo_ipv4_path || "cloudflare-colos-ipv4.csv" }}</p>
        <p class="break-all">IPv6：{{ coloDictionaryStatus?.colo_ipv6_path || "cloudflare-colos-ipv6.csv" }}</p>
        <p>更新词典只拉取原始文件；处理词典会读取本地文件生成综合、IPv4、IPv6 COLO。未覆盖表示 GEOFEED 行暂未派生出 COLO。</p>
      </div>
    </article>

    <div v-if="sources.length === 0" class="ui-card flex flex-col items-center border-dashed px-6 py-12 text-center">
      <PhDatabase class="mb-3 text-slate-300" size="44" />
      <p class="text-slate-500">暂无输入源，任务启动前至少需要配置一个来源。</p>
      <button type="button" class="ui-button ui-button-ghost mt-5" @click="$emit('add')">添加首个来源</button>
    </div>

    <article v-for="source in sources" :key="source.id" class="ui-card p-5">
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0 flex-1">
          <label class="ui-label">名称</label>
          <input v-model="source.name" type="text" placeholder="例如：优选远程源 / 自建名单" class="ui-field" />
        </div>

        <div class="flex shrink-0 items-center gap-3 pt-6">
          <div class="flex items-center gap-3 rounded-full border border-slate-200 bg-slate-50 px-3 py-2">
            <span class="text-sm font-medium text-slate-600">{{ source.enabled ? "已启用" : "已停用" }}</span>
            <button
              type="button"
              class="relative inline-flex h-6 w-11 items-center rounded-full transition"
              :class="source.enabled ? 'bg-primary' : 'bg-slate-300'"
              @click="source.enabled = !source.enabled"
            >
              <span
                class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition"
                :class="source.enabled ? 'translate-x-5' : 'translate-x-0'"
              ></span>
            </button>
          </div>

          <button type="button" class="ui-button ui-button-ghost px-3" @click="$emit('remove', source.id)">
            <PhTrash size="18" />
          </button>
        </div>
      </div>

      <div class="mt-4 grid gap-4 md:grid-cols-4">
        <label>
          <span class="ui-label">类型</span>
          <select v-model="source.kind" class="ui-field">
            <option value="url">URL 列表</option>
            <option value="file">本地文件</option>
            <option value="inline">手动输入</option>
          </select>
        </label>
        <label>
          <span class="ui-label">IP 模式</span>
          <select v-model="source.ip_mode" class="ui-field">
            <option value="traverse">遍历</option>
            <option value="mcis">MICS抽样</option>
          </select>
        </label>
        <label>
          <span class="ui-label">IP 上限</span>
          <input v-model.number="source.ip_limit" min="1" type="number" class="ui-field" />
        </label>
        <label>
          <span class="ui-label">COLO 筛选</span>
          <input v-model="source.colo_filter" placeholder="HKG,NRT,LAX" type="text" class="ui-field font-mono" />
        </label>
      </div>

      <div class="mt-4">
        <label class="ui-label">{{ sourceFieldLabel(source.kind) }}</label>
        <textarea
          v-if="source.kind === 'inline'"
          v-model="source.content"
          rows="6"
          placeholder="# 支持注释和域名&#10;1.1.1.1 # inline note&#10;104.16.0.0/16&#10;example.com"
          class="ui-field min-h-32 font-mono"
        />
        <div v-else-if="source.kind === 'file'" class="flex gap-3">
          <input
            v-model="source.path"
            type="text"
            placeholder="/data/cfips/ip.txt"
            class="ui-field h-11 flex-1 font-mono"
          />
          <button type="button" class="ui-button ui-button-ghost px-4" @click="$emit('select-file', source.id)">
            <PhFolderOpen size="18" />
            选择文件
          </button>
        </div>
        <input
          v-else
          v-model="source.url"
          type="url"
          placeholder="https://example.com/ips.txt"
          class="ui-field h-11 font-mono"
          @blur="emit('detect-source-name', source.id)"
          @change="emit('detect-source-name', source.id)"
        />
      </div>

      <div class="mt-4 flex flex-wrap gap-3">
        <button
          type="button"
          class="ui-button ui-button-ghost px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="$emit('preview', source.id)"
        >
          <PhEye size="16" />
          {{ sourceRequestState(source.id) === "preview" ? "预览中" : "预览" }}
        </button>
        <button
          type="button"
          class="ui-button ui-button-secondary px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="$emit('fetch-source', source.id)"
        >
          <PhArrowsClockwise size="16" />
          {{ sourceRequestState(source.id) === "fetch" ? "抓取中" : "抓取" }}
        </button>
      </div>

      <div class="mt-4 grid gap-3 rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-4 md:grid-cols-[minmax(0,1fr)_240px]">
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">状态</p>
          <p class="mt-2 text-sm text-slate-700">{{ sourceStatusText(source) }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">模式说明</p>
          <p class="mt-2 text-sm text-slate-700">{{ sourceModeCopy(source.ip_mode) }}</p>
        </div>
      </div>

      <div
        v-if="sourcePreviewState(source.id)"
        class="mt-4 rounded-2xl border border-slate-200 bg-white px-4 py-4"
      >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.14em] text-slate-500">
              {{ sourcePreviewState(source.id)?.action || "预览" }}结果
            </p>
            <p class="mt-1 text-sm text-slate-700">
              共 {{ sourcePreviewState(source.id)?.totalCount || 0 }} 条候选
              <span v-if="(sourcePreviewState(source.id)?.invalidCount || 0) > 0">
                ，忽略 {{ sourcePreviewState(source.id)?.invalidCount || 0 }} 条非法输入
              </span>
            </p>
          </div>
          <p class="text-xs text-slate-500">{{ sourcePreviewState(source.id)?.updatedAt || "" }}</p>
        </div>

        <div class="mt-3 flex flex-wrap gap-2">
          <code
            v-for="entry in sourcePreviewState(source.id)?.entries || []"
            :key="entry"
            class="rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-xs text-slate-700"
          >
            {{ entry }}
          </code>
        </div>

        <div
          v-if="(sourcePreviewState(source.id)?.warnings || []).length > 0"
          class="mt-3 space-y-1 text-xs text-amber-600"
        >
          <p v-for="warning in sourcePreviewState(source.id)?.warnings || []" :key="warning">{{ warning }}</p>
        </div>
      </div>
    </article>

    <div class="space-y-3">
      <button
        type="button"
        class="flex w-full items-center justify-center gap-2 rounded-full bg-[#2e333e] py-3 text-white shadow-sm transition-all duration-200 hover:bg-[#3a404e] active:scale-[0.99]"
        @click="emit('add')"
      >
        <PhPlus class="h-5 w-5" weight="bold" />
        <span class="text-[15px] font-bold tracking-[0.08em]">新增输入源</span>
      </button>
      <button
        type="button"
        class="flex w-full items-center justify-center gap-2 rounded-full border border-slate-200 bg-white py-3 text-[#111827] shadow-sm transition-all duration-200 hover:bg-slate-50 active:scale-[0.99]"
        @click="emit('save')"
      >
        <PhFloppyDisk class="h-5 w-5" weight="bold" />
        <span class="text-[15px] font-bold tracking-[0.08em]">保存</span>
      </button>
    </div>
  </section>

  <section v-else class="space-y-4">
    <article class="ui-card p-4">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-base font-semibold text-slate-800">输入源管理</h2>
          <p class="mt-1 text-xs text-slate-500">输入源会跟随全局配置保存。</p>
        </div>
        <button type="button" class="ui-button ui-button-secondary px-3" @click="$emit('add')">
          <PhPlus size="18" />
          新增
        </button>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-3 text-sm">
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <p class="text-xs text-slate-500">全部来源</p>
          <p class="mt-2 text-xl font-semibold text-slate-800">{{ sources.length }}</p>
        </div>
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <p class="text-xs text-slate-500">待执行来源</p>
          <p class="mt-2 text-xl font-semibold text-slate-800">{{ preparedCount }}</p>
        </div>
      </div>
    </article>

    <article class="ui-card overflow-hidden">
      <div class="flex items-center justify-between border-b border-slate-100 bg-slate-50 px-4 py-3">
        <div class="flex items-center">
          <PhFloppyDisk class="mr-2 text-primary" size="18" weight="fill" />
          <h3 class="text-sm font-semibold text-slate-800">输入源配置档案</h3>
        </div>
        <span class="max-w-[46%] truncate text-xs text-slate-500">{{ activeSourceProfile?.name || "未选择" }}</span>
      </div>
      <div class="space-y-3 p-4">
        <div class="flex gap-2">
          <input v-model="sourceProfileNameDraft" class="ui-field h-11 flex-1" placeholder="档案名称" type="text" />
          <button type="button" class="ui-button ui-button-primary h-11 px-3" @click="emit('save-source-profile', sourceProfileNameDraft)">
            保存
          </button>
        </div>
        <button
          type="button"
          class="ui-button ui-button-ghost h-11 w-full"
          :disabled="!activeSourceProfile"
          @click="activeSourceProfile && emit('save-source-profile', activeSourceProfile.name, activeSourceProfile.id)"
        >
          更新当前档案
        </button>
        <div v-if="sourceProfiles.items.length > 0" class="space-y-2">
          <div v-for="profile in sourceProfiles.items" :key="profile.id" class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
            <div class="flex items-center justify-between gap-2">
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-slate-700">{{ profile.name }}</p>
                <p class="text-xs text-slate-400">{{ profile.id === sourceProfiles.active_profile_id ? "当前输入源档案" : profile.updated_at || "未记录更新时间" }}</p>
              </div>
              <button type="button" class="ui-button ui-button-ghost h-9 px-3" :disabled="profile.id === sourceProfiles.active_profile_id" @click="emit('switch-source-profile', profile.id)">
                切换
              </button>
            </div>
            <div class="mt-3 grid grid-cols-3 gap-2">
              <button type="button" class="ui-button ui-button-ghost h-9 px-2" @click="renameSourceProfile(profile)">重命名</button>
              <button type="button" class="ui-button ui-button-ghost h-9 px-2" @click="duplicateSourceProfile(profile)">复制</button>
              <button type="button" class="ui-button ui-button-ghost h-9 px-2" @click="emit('delete-source-profile', profile.id)">删除</button>
            </div>
          </div>
        </div>
      </div>
    </article>

    <article class="ui-card p-4">
      <div class="flex items-start justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold text-slate-800">COLO 词典</h3>
          <p class="mt-1 text-xs text-slate-500">用于输入源 COLO 预筛。</p>
        </div>
        <div class="flex gap-2">
          <button type="button" class="ui-button ui-button-ghost px-3" @click="coloDictionaryExpanded = !coloDictionaryExpanded">
            {{ coloDictionaryExpanded ? "收起" : "展开" }}
          </button>
          <button
            type="button"
            class="ui-button ui-button-secondary px-3"
            :disabled="coloDictionaryUpdating"
            @click="$emit('refresh-colo-dictionary')"
          >
            <PhArrowsClockwise size="18" />
            {{ coloDictionaryUpdating ? "拉取中" : "更新" }}
          </button>
          <button
            type="button"
            class="ui-button ui-button-primary px-3"
            :disabled="coloDictionaryProcessing"
            @click="$emit('process-colo-dictionary')"
          >
            <PhArrowsClockwise size="18" />
            {{ coloDictionaryProcessing ? "处理中" : "处理" }}
          </button>
        </div>
      </div>
      <div v-if="coloDictionaryExpanded" class="mt-4 grid grid-cols-2 gap-3 text-sm">
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <p class="text-xs text-slate-500">GEOFEED</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.geofeed_rows || 0 }}</p>
        </div>
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <p class="text-xs text-slate-500">综合</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_rows || 0 }}</p>
        </div>
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <p class="text-xs text-slate-500">IPv4</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_ipv4_rows || 0 }}</p>
        </div>
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <p class="text-xs text-slate-500">IPv6</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_ipv6_rows || 0 }}</p>
        </div>
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <p class="text-xs text-slate-500">未覆盖</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.unmatched_rows || 0 }}</p>
        </div>
      </div>
      <div v-if="coloDictionaryExpanded" class="mt-3 space-y-1 text-xs text-slate-500">
        <p class="break-all">综合：{{ coloDictionaryStatus?.colo_path || "cloudflare-colos.csv" }}</p>
        <p class="break-all">IPv4：{{ coloDictionaryStatus?.colo_ipv4_path || "cloudflare-colos-ipv4.csv" }}</p>
        <p class="break-all">IPv6：{{ coloDictionaryStatus?.colo_ipv6_path || "cloudflare-colos-ipv6.csv" }}</p>
      </div>
      <p v-if="coloDictionaryExpanded" class="mt-3 text-xs text-slate-500">未覆盖仅表示未能从 GEOFEED 派生 COLO，不代表测速失败。</p>
    </article>

    <div v-if="sources.length === 0" class="ui-card border-2 border-dashed p-8 text-center">
      <p class="text-slate-500">暂无输入源</p>
      <button type="button" class="ui-button ui-button-ghost mt-5" @click="$emit('add')">添加首个来源</button>
    </div>

    <article v-for="source in sources" :key="source.id" class="ui-card p-4">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <label class="block text-xs text-slate-500">名称</label>
          <input v-model="source.name" type="text" placeholder="输入源名称" class="ui-field h-11" />
        </div>
        <button type="button" class="ui-button ui-button-ghost px-3" @click="$emit('remove', source.id)">
          <PhTrash size="18" />
        </button>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-3">
        <div>
          <label class="block text-xs text-slate-500">类型</label>
          <select v-model="source.kind" class="ui-field h-11">
            <option value="url">URL 列表</option>
            <option value="file">本地文件</option>
            <option value="inline">手动输入</option>
          </select>
        </div>
        <div>
          <label class="block text-xs text-slate-500">IP 模式</label>
          <select v-model="source.ip_mode" class="ui-field h-11">
            <option value="traverse">遍历</option>
            <option value="mcis">MICS抽样</option>
          </select>
        </div>
        <div>
          <label class="block text-xs text-slate-500">IP 上限</label>
          <input v-model.number="source.ip_limit" min="1" type="number" class="ui-field h-11" />
        </div>
        <div>
          <label class="block text-xs text-slate-500">COLO 筛选</label>
          <input v-model="source.colo_filter" placeholder="HKG,NRT" type="text" class="ui-field h-11 font-mono" />
        </div>
        <div class="flex items-end justify-between rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <div>
            <p class="text-xs text-slate-500">启用状态</p>
            <p class="mt-1 text-sm font-medium text-slate-700">{{ source.enabled ? "已启用" : "已停用" }}</p>
          </div>
          <button
            type="button"
            class="relative inline-flex h-6 w-11 items-center rounded-full transition"
            :class="source.enabled ? 'bg-primary' : 'bg-slate-300'"
            @click="source.enabled = !source.enabled"
          >
            <span
              class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition"
              :class="source.enabled ? 'translate-x-5' : 'translate-x-0'"
            ></span>
          </button>
        </div>
      </div>

      <div class="mt-4">
        <label class="block text-xs text-slate-500">{{ sourceFieldLabel(source.kind) }}</label>
        <textarea
          v-if="source.kind === 'inline'"
          v-model="source.content"
          rows="5"
          placeholder="# 支持注释和域名&#10;1.1.1.1 # inline note&#10;example.com"
          class="ui-field mt-1 min-h-28 font-mono"
        />
        <div v-else-if="source.kind === 'file'" class="mt-1 flex gap-2">
          <input
            v-model="source.path"
            type="text"
            placeholder="/data/cfips/ip.txt"
            class="ui-field h-11 flex-1 font-mono"
          />
          <button type="button" class="ui-button ui-button-ghost h-11 px-3" @click="$emit('select-file', source.id)">
            <PhFolderOpen size="18" />
            选择
          </button>
        </div>
        <input
          v-else
          v-model="source.url"
          type="url"
          placeholder="https://example.com/ips.txt"
          class="ui-field mt-1 h-11 font-mono"
          @blur="emit('detect-source-name', source.id)"
          @change="emit('detect-source-name', source.id)"
        />
      </div>

      <div class="mt-4 grid grid-cols-2 gap-3">
        <button
          type="button"
          class="ui-button ui-button-ghost px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="$emit('preview', source.id)"
        >
          <PhEye size="16" />
          {{ sourceRequestState(source.id) === "preview" ? "预览中" : "预览" }}
        </button>
        <button
          type="button"
          class="ui-button ui-button-secondary px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="$emit('fetch-source', source.id)"
        >
          <PhArrowsClockwise size="16" />
          {{ sourceRequestState(source.id) === "fetch" ? "抓取中" : "抓取" }}
        </button>
      </div>

      <div class="mt-4 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-sm text-slate-600">
        <p>{{ sourceStatusText(source) }}</p>
        <p class="mt-1 text-xs text-slate-500">模式说明：{{ sourceModeCopy(source.ip_mode) }}</p>
      </div>

      <div
        v-if="sourcePreviewState(source.id)"
        class="mt-4 rounded-xl border border-slate-200 bg-white px-3 py-3"
      >
        <p class="text-xs text-slate-500">
          {{ sourcePreviewState(source.id)?.action || "预览" }}结果 · 共 {{ sourcePreviewState(source.id)?.totalCount || 0 }} 条
        </p>
        <div class="mt-3 flex flex-wrap gap-2">
          <code
            v-for="entry in sourcePreviewState(source.id)?.entries || []"
            :key="entry"
            class="rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-xs text-slate-700"
          >
            {{ entry }}
          </code>
        </div>
        <div
          v-if="(sourcePreviewState(source.id)?.warnings || []).length > 0"
          class="mt-3 space-y-1 text-xs text-amber-600"
        >
          <p v-for="warning in sourcePreviewState(source.id)?.warnings || []" :key="warning">{{ warning }}</p>
        </div>
      </div>

      <div class="mt-3 flex items-center justify-between text-xs text-slate-500">
        <span>{{ sourceTypeLabel(source.kind) }}</span>
        <span>{{ source.enabled ? "任务启动时参与读取" : "任务启动时跳过" }}</span>
      </div>
    </article>

    <div class="space-y-3">
      <button
        type="button"
        class="flex w-full items-center justify-center gap-2 rounded-full bg-[#2e333e] py-3 text-white shadow-sm transition-all duration-200 hover:bg-[#3a404e] active:scale-[0.99]"
        @click="emit('add')"
      >
        <PhPlus class="h-5 w-5" weight="bold" />
        <span class="text-[15px] font-bold tracking-[0.08em]">新增输入源</span>
      </button>
      <button
        type="button"
        class="flex w-full items-center justify-center gap-2 rounded-full border border-slate-200 bg-white py-3 text-[#111827] shadow-sm transition-all duration-200 hover:bg-slate-50 active:scale-[0.99]"
        @click="emit('save')"
      >
        <PhFloppyDisk class="h-5 w-5" weight="bold" />
        <span class="text-[15px] font-bold tracking-[0.08em]">保存</span>
      </button>
    </div>
  </section>
</template>
