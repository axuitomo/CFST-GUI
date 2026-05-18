<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { PhArrowsClockwise, PhCaretDown, PhCaretUp, PhDatabase, PhEye, PhFloppyDisk, PhFolderOpen, PhPlus, PhTrash } from "@phosphor-icons/vue";
import { sourceUrlCdnSwitch } from "../lib/sourceUrls";

interface SourceEntry {
  colo_filter: string;
  colo_filter_mode: "allow" | "deny";
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
  portSummary: Record<string, unknown> | null;
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

interface TimestampFormatOptions {
  fallback?: string;
  includeDate?: boolean;
  includeOffset?: boolean;
  includeSeconds?: boolean;
}

const props = defineProps<{
  accepted: number;
  coloDictionaryProcessing: boolean;
  coloDictionaryStatus: ColoDictionaryStatus | null;
  coloDictionaryUpdating: boolean;
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  invalid: number;
  platform: "desktop" | "mobile";
  preparedCount: number;
  previewStates: Record<string, PreviewState | undefined>;
  requestStates: Record<string, string | undefined>;
  sourceProfiles: SourceProfileStore;
  sources: SourceEntry[];
  taskStage: string;
}>();

const sourceProfileNameDraft = ref("");
const coloDictionaryExpanded = ref(props.platform === "desktop");
const sourceProfilesExpanded = ref(false);
const expandedSourceIds = ref(new Set<string>());
const visiblePreviewSourceIds = ref(new Set<string>());
let knownSourceIds = new Set(props.sources.map((source) => source.id));
const activeSourceProfile = computed(
  () => props.sourceProfiles.items.find((profile) => profile.id === props.sourceProfiles.active_profile_id) || null,
);

function isActiveSourceProfile(profile: SourceProfileItem) {
  return profile.id === props.sourceProfiles.active_profile_id;
}

function sourceProfileSavedAt(profile: SourceProfileItem) {
  const value = profile.updated_at || profile.created_at;
  return value.trim() ? props.formatTimestamp(value) : "未记录保存时间";
}

function sourceProfileSourceCount(profile: SourceProfileItem) {
  return `${profile.sources.length} 个输入源`;
}

function sourceProfileSourceNames(profile: SourceProfileItem) {
  if (profile.sources.length === 0) {
    return "无输入源";
  }

  return profile.sources
    .map((source, index) => source.name.trim() || `输入源 ${index + 1}`)
    .join("、");
}

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

function sourceColoModeLabel(mode: SourceEntry["colo_filter_mode"]) {
  return mode === "deny" ? "黑名单" : "白名单";
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

function sourceTargetSummary(source: SourceEntry) {
  if (source.kind === "inline") {
    const lines = source.content
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean);
    if (lines.length === 0) {
      return "未填写手动输入";
    }
    return `${lines.length} 行：${lines[0]}`;
  }

  if (source.kind === "file") {
    return source.path.trim() || "未选择文件";
  }

  return source.url.trim() || "未填写 URL";
}

function sourceColoSummary(source: SourceEntry) {
  if (!source.colo_filter.trim()) {
    return "COLO 不限制";
  }
  return `${sourceColoModeLabel(source.colo_filter_mode)} ${source.colo_filter.trim()}`;
}

function sourceCdnSwitch(source: SourceEntry) {
  if (source.kind !== "url") {
    return null;
  }
  return sourceUrlCdnSwitch(source.url);
}

function toggleSourceCdn(source: SourceEntry) {
  const next = sourceCdnSwitch(source);
  if (!next) {
    return;
  }
  source.url = next.nextUrl;
  emit("detect-source-name", source.id);
}

function sourcePreviewSummary(sourceId: string) {
  const preview = sourcePreviewState(sourceId);
  if (!preview) {
    return "";
  }
  const invalid = preview.invalidCount > 0 ? `，忽略 ${preview.invalidCount} 条` : "";
  const portSummary = sourcePortSummaryText(preview);
  return `预览结果：${preview.totalCount} 条${invalid}${portSummary ? `，${portSummary}` : ""}`;
}

function numberArray(value: unknown) {
  return Array.isArray(value)
    ? value.map((entry) => Number(entry)).filter((entry) => Number.isFinite(entry) && entry > 0)
    : [];
}

function sourcePortSummaryText(preview: PreviewState) {
  const summary = preview.portSummary || {};
  const policy = String(summary.port_policy || "").trim();
  const globalPort = Number(summary.global_tcp_port);
  const currentPort = Number(summary.current_test_port);
  const sourcePorts = numberArray(summary.source_port_values);
  const groupedPorts = numberArray(summary.grouped_ports);
  if (policy === "fixed_global") {
    if (sourcePorts.length > 0) {
      return `检测到源端口 ${sourcePorts.join(" / ")}，当前使用固定端口 ${Number.isFinite(globalPort) && globalPort > 0 ? globalPort : "-"}`;
    }
    return Number.isFinite(globalPort) && globalPort > 0 ? `当前使用固定端口 ${globalPort}` : "";
  }
  if (groupedPorts.length > 1) {
    const sourceText = sourcePorts.length > 0 ? `源端口 ${sourcePorts.join(" / ")}，` : "";
    return `${sourceText}按端口分组 ${groupedPorts.join(" / ")}`;
  }
  if (sourcePorts.length > 0) {
    return `源端口 ${sourcePorts.join(" / ")}，当前测试端口 ${Number.isFinite(currentPort) && currentPort > 0 ? currentPort : sourcePorts[0]}`;
  }
  if (Number.isFinite(globalPort) && globalPort > 0) {
    return `回退全局端口 ${globalPort}`;
  }
  return "";
}

function formatTimestampLabel(value: string, options?: TimestampFormatOptions) {
  return props.formatTimestamp(value, options);
}

function sourcePreviewState(sourceId: string) {
  return props.previewStates[sourceId];
}

function sourceRequestState(sourceId: string) {
  return props.requestStates[sourceId] || "";
}

function isSourcePreviewVisible(sourceId: string) {
  return visiblePreviewSourceIds.value.has(sourceId);
}

function setSourcePreviewVisible(sourceId: string, visible: boolean) {
  const next = new Set(visiblePreviewSourceIds.value);
  if (visible) {
    next.add(sourceId);
  } else {
    next.delete(sourceId);
  }
  visiblePreviewSourceIds.value = next;
}

function previewButtonLabel(sourceId: string) {
  if (sourceRequestState(sourceId)) {
    return "加载中";
  }

  if (!sourcePreviewState(sourceId)) {
    return "预览";
  }

  return isSourcePreviewVisible(sourceId) ? "隐藏" : "显示";
}

function toggleSourcePreview(sourceId: string) {
  if (sourceRequestState(sourceId)) {
    return;
  }

  if (sourcePreviewState(sourceId)) {
    setSourcePreviewVisible(sourceId, !isSourcePreviewVisible(sourceId));
    return;
  }

  setSourcePreviewVisible(sourceId, true);
  emit("preview-request", sourceId);
}

function requestSourceFetch(sourceId: string) {
  setSourcePreviewVisible(sourceId, true);
  emit("fetch-source", sourceId);
}

function dictionaryUpdatedAt() {
  const value = props.coloDictionaryStatus?.last_updated_at || "";
  return value.trim() ? props.formatTimestamp(value) : "尚未更新";
}

function isSourceExpanded(sourceId: string) {
  return expandedSourceIds.value.has(sourceId);
}

function setSourceExpanded(sourceId: string, expanded: boolean) {
  const next = new Set(expandedSourceIds.value);
  if (expanded) {
    next.add(sourceId);
  } else {
    next.delete(sourceId);
  }
  expandedSourceIds.value = next;
}

function toggleSourceExpanded(sourceId: string) {
  setSourceExpanded(sourceId, !isSourceExpanded(sourceId));
}

function removeSource(sourceId: string) {
  setSourceExpanded(sourceId, false);
  setSourcePreviewVisible(sourceId, false);
  emit("remove", sourceId);
}

watch(
  () => props.sources.map((source) => source.id),
  (sourceIds) => {
    const nextKnownSourceIds = new Set(sourceIds);
    const nextExpandedSourceIds = new Set(expandedSourceIds.value);
    const addedSourceIds = sourceIds.filter((sourceId) => !knownSourceIds.has(sourceId));

    if (addedSourceIds.length === 1 && sourceIds.length > knownSourceIds.size) {
      nextExpandedSourceIds.add(addedSourceIds[0]);
    }

    for (const sourceId of nextExpandedSourceIds) {
      if (!nextKnownSourceIds.has(sourceId)) {
        nextExpandedSourceIds.delete(sourceId);
      }
    }

    const nextVisiblePreviewSourceIds = new Set(visiblePreviewSourceIds.value);
    for (const sourceId of nextVisiblePreviewSourceIds) {
      if (!nextKnownSourceIds.has(sourceId)) {
        nextVisiblePreviewSourceIds.delete(sourceId);
      }
    }

    knownSourceIds = nextKnownSourceIds;
    expandedSourceIds.value = nextExpandedSourceIds;
    visiblePreviewSourceIds.value = nextVisiblePreviewSourceIds;
  },
);

const emit = defineEmits<{
  (event: "add"): void;
  (event: "delete-source-profile", profileId: string): void;
  (event: "detect-source-name", sourceId: string): void;
  (event: "fetch-source", sourceId: string): void;
  (event: "process-colo-dictionary"): void;
  (event: "preview", sourceId: string): void;
  (event: "preview-request", sourceId: string): void;
  (event: "refresh-colo-dictionary"): void;
  (event: "remove", sourceId: string): void;
  (event: "save"): void;
  (event: "save-source-profile", name: string, profileId?: string, sources?: SourceEntry[], setActive?: boolean): void;
  (event: "update-current-source-profile"): void;
  (event: "select-file", sourceId: string): void;
  (event: "switch-source-profile", profileId: string): void;
}>();

function renameSourceProfile(profile: SourceProfileItem) {
  const nextName = window.prompt("新的输入源档案名称", profile.name)?.trim();
  if (!nextName || nextName === profile.name) {
    return;
  }
  if (isActiveSourceProfile(profile)) {
    emit("save-source-profile", nextName, profile.id, undefined, true);
    return;
  }
  emit("save-source-profile", nextName, profile.id, profile.sources, false);
}

function duplicateSourceProfile(profile: SourceProfileItem) {
  emit("save-source-profile", `${profile.name} 副本`, "", profile.sources, false);
}

function createBlankSourceProfile() {
  emit("save-source-profile", sourceProfileNameDraft.value, "", [], true);
}

function updateActiveSourceProfile() {
  emit("update-current-source-profile");
}
</script>

<template>
  <section v-if="platform === 'desktop'" class="space-y-5">
    <div class="flex flex-wrap items-end justify-between gap-4">
      <div class="min-w-0">
        <h2 class="text-lg font-semibold text-slate-800">输入源管理</h2>
        <p class="mt-1 text-sm text-slate-500">输入源会跟随全局配置一起保存，每个来源都可以独立设置 IP 上限与 IP 模式。</p>
      </div>
      <div class="sources-header-actions">
        <button type="button" class="sources-header-button sources-header-button-primary" @click="$emit('add')">
          <PhPlus size="18" />
          新增输入源
        </button>
        <button
          type="button"
          class="sources-header-button sources-header-button-secondary"
          @click="$emit('save')"
        >
          <PhFloppyDisk size="18" />
          保存配置
        </button>
      </div>
    </div>

    <article class="ui-card overflow-hidden">
      <div
        class="flex flex-wrap items-center justify-between gap-3 bg-slate-50/70 px-5 py-3"
        :class="sourceProfilesExpanded ? 'border-b border-slate-200' : ''"
      >
        <div class="min-w-0">
          <h3 class="flex items-center text-base font-semibold text-slate-800">
            <PhFloppyDisk class="mr-2 text-primary" size="20" weight="fill" />
            输入源配置档案
          </h3>
          <p class="mt-1 text-xs text-slate-500">只保存和切换输入源列表，不影响测速、Cloudflare 和导出设置。</p>
        </div>
        <div class="flex flex-wrap items-center justify-end gap-2">
          <span class="ui-pill ui-pill-subtle">{{ activeSourceProfile?.name || "未选择档案" }}</span>
          <span class="ui-pill bg-slate-100 text-slate-600">{{ sourceProfiles.items.length }} 个档案</span>
          <button type="button" class="ui-button ui-button-ghost px-3" @click="sourceProfilesExpanded = !sourceProfilesExpanded">
            <component :is="sourceProfilesExpanded ? PhCaretUp : PhCaretDown" size="16" />
            {{ sourceProfilesExpanded ? "收起" : "展开" }}
          </button>
        </div>
      </div>
      <div v-if="sourceProfilesExpanded" class="grid gap-3 p-5 lg:grid-cols-[minmax(0,1fr)_auto]">
        <label class="min-w-0">
          <span class="ui-label">新建空白输入源档案</span>
          <input v-model="sourceProfileNameDraft" class="ui-field" placeholder="例如：VPS789 组合 / 自建源" type="text" />
        </label>
        <div class="flex flex-wrap items-end gap-2">
          <button type="button" class="ui-button ui-button-primary" @click="createBlankSourceProfile">
            <PhFloppyDisk size="18" weight="fill" />
            新建空白档案
          </button>
          <button
            type="button"
            class="ui-button ui-button-ghost"
            @click="updateActiveSourceProfile"
          >
            更新并保存当前档案
          </button>
        </div>
      </div>
      <div v-if="sourceProfilesExpanded && sourceProfiles.items.length > 0" class="grid gap-3 border-t border-slate-100 p-5 pt-4 lg:grid-cols-2">
        <div
          v-for="profile in sourceProfiles.items"
          :key="profile.id"
          class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2.5"
        >
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <p class="truncate text-sm font-medium text-slate-700">{{ profile.name }}</p>
              <span v-if="isActiveSourceProfile(profile)" class="ui-pill ui-pill-success">当前</span>
              <span class="ui-pill bg-slate-100 text-slate-600">{{ sourceProfileSourceCount(profile) }}</span>
            </div>
            <p class="mt-1 text-xs text-slate-400">保存时间：{{ sourceProfileSavedAt(profile) }}</p>
            <p class="mt-1 truncate text-xs text-slate-500">输入源：{{ sourceProfileSourceNames(profile) }}</p>
          </div>
          <div class="flex flex-wrap justify-end gap-1.5">
            <button type="button" class="ui-button ui-button-ghost px-2.5 py-1.5" :disabled="isActiveSourceProfile(profile)" @click="emit('switch-source-profile', profile.id)">切换</button>
            <button type="button" class="ui-button ui-button-ghost px-2.5 py-1.5" @click="renameSourceProfile(profile)">重命名</button>
            <button type="button" class="ui-button ui-button-ghost px-2.5 py-1.5" @click="duplicateSourceProfile(profile)">复制</button>
            <button type="button" class="ui-button ui-button-ghost px-2.5 py-1.5" @click="emit('delete-source-profile', profile.id)">删除</button>
          </div>
        </div>
      </div>
    </article>

    <article class="ui-card p-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <h3 class="text-base font-semibold text-slate-800">COLO 词典</h3>
          <p class="mt-1 text-sm text-slate-500">先拉取 Cloudflare GEOFEED 与辅助映射，再本地生成 COLO 文件供输入源预筛使用。</p>
        </div>
        <div class="flex shrink-0 flex-wrap gap-2">
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
      <div v-if="coloDictionaryExpanded" class="mt-3 grid gap-3 md:grid-cols-6">
        <div class="min-w-0">
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">GEOFEED</p>
          <p class="mt-1 text-base font-semibold text-slate-800">{{ coloDictionaryStatus?.geofeed_rows || 0 }}</p>
        </div>
        <div class="min-w-0">
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">综合</p>
          <p class="mt-1 text-base font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_rows || 0 }}</p>
        </div>
        <div class="min-w-0">
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">IPv4</p>
          <p class="mt-1 text-base font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_ipv4_rows || 0 }}</p>
        </div>
        <div class="min-w-0">
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">IPv6</p>
          <p class="mt-1 text-base font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_ipv6_rows || 0 }}</p>
        </div>
        <div class="min-w-0">
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">未覆盖</p>
          <p class="mt-1 text-base font-semibold text-slate-800">{{ coloDictionaryStatus?.unmatched_rows || 0 }}</p>
        </div>
        <div class="min-w-0">
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">更新时间</p>
          <p class="mt-1 break-all text-xs text-slate-700">{{ dictionaryUpdatedAt() }}</p>
        </div>
      </div>
      <div v-if="coloDictionaryExpanded" class="mt-3 space-y-1 text-xs text-slate-500">
        <p class="break-all">GEOFEED：{{ coloDictionaryStatus?.geofeed_path || "local-ip-ranges.csv" }}</p>
        <p class="break-all">综合：{{ coloDictionaryStatus?.colo_path || "cloudflare-colos.csv" }}</p>
        <p class="break-all">IPv4：{{ coloDictionaryStatus?.colo_ipv4_path || "cloudflare-colos-ipv4.csv" }}</p>
        <p class="break-all">IPv6：{{ coloDictionaryStatus?.colo_ipv6_path || "cloudflare-colos-ipv6.csv" }}</p>
        <p>更新词典只拉取原始文件；处理词典会读取本地文件生成综合、IPv4、IPv6 COLO。未覆盖表示 GEOFEED 行暂未派生出 COLO。</p>
      </div>
    </article>

    <div v-if="sources.length === 0" class="ui-card flex flex-col items-center border-dashed px-5 py-10 text-center">
      <PhDatabase class="mb-3 text-slate-300" size="44" />
      <p class="text-slate-500">暂无输入源，任务启动前至少需要配置一个来源。</p>
      <button type="button" class="ui-button ui-button-ghost mt-5" @click="$emit('add')">添加首个来源</button>
    </div>

    <article v-for="source in sources" :key="source.id" class="ui-card p-4">
      <template v-if="!isSourceExpanded(source.id)">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <h3 class="truncate text-base font-semibold text-slate-800">{{ source.name || "未命名输入源" }}</h3>
              <span class="ui-pill ui-pill-subtle">{{ sourceTypeLabel(source.kind) }}</span>
              <span class="ui-pill" :class="source.enabled ? 'ui-pill-success' : 'bg-slate-100 text-slate-500'">{{ source.enabled ? "已启用" : "已停用" }}</span>
            </div>
            <p class="mt-2 truncate font-mono text-xs text-slate-500">{{ sourceTargetSummary(source) }}</p>
            <div class="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">
              <span class="rounded-full bg-slate-100 px-2.5 py-1">{{ source.ip_mode === "mcis" ? "MICS抽样" : "遍历" }}</span>
              <span class="rounded-full bg-slate-100 px-2.5 py-1">上限 {{ source.ip_limit }}</span>
              <span class="max-w-full truncate rounded-full bg-slate-100 px-2.5 py-1">{{ sourceColoSummary(source) }}</span>
            </div>
          </div>

          <div class="flex flex-wrap items-center justify-end gap-2">
            <div class="flex items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1.5">
              <span class="text-sm font-medium text-slate-600">{{ source.enabled ? "启用" : "停用" }}</span>
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
            <button type="button" class="ui-button ui-button-ghost px-3" :disabled="Boolean(sourceRequestState(source.id))" @click="toggleSourcePreview(source.id)">
              <PhEye size="16" />
              {{ previewButtonLabel(source.id) }}
            </button>
            <button v-if="sourceCdnSwitch(source)" type="button" class="ui-button ui-button-ghost px-3" @click="toggleSourceCdn(source)">
              {{ sourceCdnSwitch(source)?.label }}
            </button>
            <button type="button" class="ui-button ui-button-secondary px-3" :disabled="Boolean(sourceRequestState(source.id))" @click="requestSourceFetch(source.id)">
              <PhArrowsClockwise size="16" />
              {{ sourceRequestState(source.id) === "fetch" ? "抓取中" : "抓取" }}
            </button>
            <button type="button" class="ui-button ui-button-ghost px-3" @click="toggleSourceExpanded(source.id)">
              <PhCaretDown size="16" />
              编辑
            </button>
            <button type="button" class="ui-button ui-button-ghost px-3" @click="removeSource(source.id)">
              <PhTrash size="18" />
            </button>
          </div>
        </div>

        <div v-if="sourcePreviewState(source.id) && isSourcePreviewVisible(source.id)" class="mt-3 rounded-xl border border-slate-200 bg-slate-50/70 px-3 py-2 text-sm text-slate-600">
          <p>{{ sourcePreviewSummary(source.id) }}</p>
          <p class="mt-1 truncate font-mono text-xs text-slate-500">{{ (sourcePreviewState(source.id)?.entries || []).join(", ") }}</p>
        </div>
      </template>

      <template v-else>
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <label class="ui-label">名称</label>
          <input v-model="source.name" type="text" placeholder="例如：优选远程源 / 自建名单" class="ui-field" />
        </div>

        <div class="flex flex-wrap items-center justify-end gap-2 pt-5">
          <div class="flex items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1.5">
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

          <button type="button" class="ui-button ui-button-ghost px-3" @click="toggleSourceExpanded(source.id)">
            <PhCaretUp size="18" />
            收起
          </button>

          <button type="button" class="ui-button ui-button-ghost px-3" @click="removeSource(source.id)">
            <PhTrash size="18" />
          </button>
        </div>
      </div>

      <div class="mt-3 grid gap-3 md:grid-cols-4">
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
        <div>
          <div class="mb-2 flex items-center justify-between gap-2">
            <span class="ui-label mb-0">COLO 筛选</span>
            <div class="inline-flex rounded-full border border-slate-200 bg-slate-100 p-1">
              <button
                type="button"
                class="rounded-full px-3 py-1 text-xs font-semibold transition"
                :class="source.colo_filter_mode === 'allow' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                @click="source.colo_filter_mode = 'allow'"
              >
                白
              </button>
              <button
                type="button"
                class="rounded-full px-3 py-1 text-xs font-semibold transition"
                :class="source.colo_filter_mode === 'deny' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'"
                @click="source.colo_filter_mode = 'deny'"
              >
                黑
              </button>
            </div>
          </div>
          <input v-model="source.colo_filter" placeholder="HKG,NRT,LAX" type="text" class="ui-field font-mono" />
          <p class="mt-2 text-xs text-slate-500">{{ sourceColoModeLabel(source.colo_filter_mode) }}模式；空列表不限制。</p>
        </div>
      </div>

      <div class="mt-3">
        <label class="ui-label">{{ sourceFieldLabel(source.kind) }}</label>
        <textarea
          v-if="source.kind === 'inline'"
          v-model="source.content"
          rows="6"
          placeholder="# 支持注释和域名&#10;1.1.1.1 # inline note&#10;104.16.0.0/16&#10;example.com"
          class="ui-field min-h-32 font-mono"
        />
        <div v-else-if="source.kind === 'file'" class="flex flex-col gap-3 sm:flex-row">
          <input
            v-model="source.path"
            type="text"
            placeholder="/data/cfips/ip.txt"
            class="ui-field h-10 min-w-0 flex-1 font-mono"
          />
          <button type="button" class="ui-button ui-button-ghost px-4" @click="$emit('select-file', source.id)">
            <PhFolderOpen size="18" />
            选择文件
          </button>
        </div>
        <div v-else class="flex flex-col gap-2 sm:flex-row">
          <input
            v-model="source.url"
            type="text"
            placeholder="https://example.com/ips.txt 或 example.com/ips.txt"
            class="ui-field h-10 min-w-0 flex-1 font-mono"
            @blur="emit('detect-source-name', source.id)"
            @change="emit('detect-source-name', source.id)"
          />
          <button v-if="sourceCdnSwitch(source)" type="button" class="ui-button ui-button-ghost h-10 px-3" @click="toggleSourceCdn(source)">
            {{ sourceCdnSwitch(source)?.label }}
          </button>
        </div>
      </div>

      <div class="mt-3 flex flex-wrap gap-2">
        <button
          type="button"
          class="ui-button ui-button-ghost px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="toggleSourcePreview(source.id)"
        >
          <PhEye size="16" />
          {{ previewButtonLabel(source.id) }}
        </button>
        <button
          type="button"
          class="ui-button ui-button-secondary px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="requestSourceFetch(source.id)"
        >
          <PhArrowsClockwise size="16" />
          {{ sourceRequestState(source.id) === "fetch" ? "抓取中" : "抓取" }}
        </button>
      </div>

      <div class="mt-3 grid gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-3 py-3 md:grid-cols-[minmax(0,1fr)_220px]">
        <div class="overflow-safe">
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">状态</p>
          <p class="mt-2 text-sm text-slate-700">{{ sourceStatusText(source) }}</p>
        </div>
        <div class="overflow-safe">
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">模式说明</p>
          <p class="mt-2 text-sm text-slate-700">{{ sourceModeCopy(source.ip_mode) }}</p>
        </div>
      </div>

      <div
        v-if="sourcePreviewState(source.id) && isSourcePreviewVisible(source.id)"
        class="mt-3 rounded-xl border border-slate-200 bg-white px-3 py-3"
      >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="min-w-0">
            <p class="text-xs uppercase tracking-[0.14em] text-slate-500">
              预览结果
            </p>
            <p class="mt-1 text-sm text-slate-700">
              共 {{ sourcePreviewState(source.id)?.totalCount || 0 }} 条候选
              <span v-if="(sourcePreviewState(source.id)?.invalidCount || 0) > 0">
                ，忽略 {{ sourcePreviewState(source.id)?.invalidCount || 0 }} 条非法输入
              </span>
            </p>
          </div>
          <p class="break-all text-xs text-slate-500">{{ sourcePreviewState(source.id)?.updatedAt ? formatTimestampLabel(sourcePreviewState(source.id)?.updatedAt || "") : "" }}</p>
        </div>

        <div class="mt-3 flex flex-wrap gap-2">
          <code
            v-for="entry in sourcePreviewState(source.id)?.entries || []"
            :key="entry"
            class="break-all rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-xs text-slate-700"
          >
            {{ entry }}
          </code>
        </div>

        <div
          v-if="(sourcePreviewState(source.id)?.warnings || []).length > 0"
          class="mt-3 space-y-1 text-xs text-amber-600"
        >
          <p v-for="warning in sourcePreviewState(source.id)?.warnings || []" :key="warning" class="break-all">{{ warning }}</p>
        </div>
      </div>
      </template>
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
        <span class="text-[15px] font-bold tracking-[0.08em]">保存输入源</span>
      </button>
    </div>
  </section>

  <section v-else class="space-y-4">
    <article class="ui-card overflow-hidden">
      <div
        class="flex items-center justify-between gap-3 bg-slate-50 px-4 py-3"
        :class="sourceProfilesExpanded ? 'border-b border-slate-100' : ''"
      >
        <div class="min-w-0 flex items-center">
          <PhFloppyDisk class="mr-2 text-primary" size="18" weight="fill" />
          <h3 class="text-sm font-semibold text-slate-800">输入源配置档案</h3>
        </div>
        <div class="flex min-w-0 items-center gap-2">
          <span class="max-w-[8rem] truncate text-xs text-slate-500">{{ activeSourceProfile?.name || "未选择" }}</span>
          <span class="shrink-0 text-xs text-slate-400">{{ sourceProfiles.items.length }} 个</span>
          <button type="button" class="ui-button ui-button-ghost h-9 px-3" @click="sourceProfilesExpanded = !sourceProfilesExpanded">
            <component :is="sourceProfilesExpanded ? PhCaretUp : PhCaretDown" size="14" />
            {{ sourceProfilesExpanded ? "收起" : "展开" }}
          </button>
        </div>
      </div>
      <div v-if="sourceProfilesExpanded" class="space-y-3 p-4">
        <div class="flex gap-2">
          <input v-model="sourceProfileNameDraft" class="ui-field h-11 min-w-0 flex-1" placeholder="档案名称" type="text" />
          <button type="button" class="ui-button ui-button-primary h-11 px-3" @click="createBlankSourceProfile">
            新建空白
          </button>
        </div>
        <button
          type="button"
          class="ui-button ui-button-ghost h-11 w-full"
          @click="updateActiveSourceProfile"
        >
          更新并保存当前档案
        </button>
        <div v-if="sourceProfiles.items.length > 0" class="space-y-2">
          <div v-for="profile in sourceProfiles.items" :key="profile.id" class="ui-card-subtle px-3 py-3">
            <div class="flex items-center justify-between gap-2">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <p class="truncate text-sm font-medium text-slate-700">{{ profile.name }}</p>
                  <span v-if="isActiveSourceProfile(profile)" class="ui-pill ui-pill-success">当前</span>
                </div>
                <p class="mt-1 truncate text-xs text-slate-400">保存时间：{{ sourceProfileSavedAt(profile) }}</p>
                <p class="mt-1 truncate text-xs text-slate-500">{{ sourceProfileSourceCount(profile) }} · {{ sourceProfileSourceNames(profile) }}</p>
              </div>
              <button type="button" class="ui-button ui-button-ghost h-9 px-3" :disabled="isActiveSourceProfile(profile)" @click="emit('switch-source-profile', profile.id)">
                切换
              </button>
            </div>
            <div class="mt-3 grid grid-cols-3 gap-2">
              <button type="button" class="ui-button ui-button-ghost h-9 px-1.5 text-xs" @click="renameSourceProfile(profile)">重命名</button>
              <button type="button" class="ui-button ui-button-ghost h-9 px-1.5 text-xs" @click="duplicateSourceProfile(profile)">复制</button>
              <button type="button" class="ui-button ui-button-ghost h-9 px-1.5 text-xs" @click="emit('delete-source-profile', profile.id)">删除</button>
            </div>
          </div>
        </div>
      </div>
    </article>

    <article class="ui-card p-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <h3 class="text-sm font-semibold text-slate-800">COLO 词典</h3>
          <p class="mt-1 text-xs text-slate-500">用于输入源 COLO 预筛。</p>
        </div>
        <div class="flex flex-wrap justify-end gap-2">
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
        <div class="ui-card-subtle px-3 py-3">
          <p class="text-xs text-slate-500">GEOFEED</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.geofeed_rows || 0 }}</p>
        </div>
        <div class="ui-card-subtle px-3 py-3">
          <p class="text-xs text-slate-500">综合</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_rows || 0 }}</p>
        </div>
        <div class="ui-card-subtle px-3 py-3">
          <p class="text-xs text-slate-500">IPv4</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_ipv4_rows || 0 }}</p>
        </div>
        <div class="ui-card-subtle px-3 py-3">
          <p class="text-xs text-slate-500">IPv6</p>
          <p class="mt-2 text-lg font-semibold text-slate-800">{{ coloDictionaryStatus?.colo_ipv6_rows || 0 }}</p>
        </div>
        <div class="ui-card-subtle px-3 py-3">
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
      <template v-if="!isSourceExpanded(source.id)">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <h3 class="truncate text-sm font-semibold text-slate-800">{{ source.name || "未命名输入源" }}</h3>
              <span class="ui-pill ui-pill-subtle">{{ sourceTypeLabel(source.kind) }}</span>
              <span class="ui-pill" :class="source.enabled ? 'ui-pill-success' : 'bg-slate-100 text-slate-500'">{{ source.enabled ? "已启用" : "已停用" }}</span>
            </div>
            <p class="mt-2 truncate font-mono text-xs text-slate-500">{{ sourceTargetSummary(source) }}</p>
            <div class="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">
              <span class="rounded-full bg-slate-100 px-2.5 py-1">{{ source.ip_mode === "mcis" ? "MICS抽样" : "遍历" }}</span>
              <span class="rounded-full bg-slate-100 px-2.5 py-1">上限 {{ source.ip_limit }}</span>
              <span class="max-w-full truncate rounded-full bg-slate-100 px-2.5 py-1">{{ sourceColoSummary(source) }}</span>
            </div>
          </div>
          <button
            type="button"
            class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition"
            :class="source.enabled ? 'bg-primary' : 'bg-slate-300'"
            @click="source.enabled = !source.enabled"
          >
            <span
              class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition"
              :class="source.enabled ? 'translate-x-5' : 'translate-x-0'"
            ></span>
          </button>
        </div>

        <div v-if="sourcePreviewState(source.id) && isSourcePreviewVisible(source.id)" class="mt-3 rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">
          <p>{{ sourcePreviewSummary(source.id) }}</p>
          <p class="mt-1 truncate font-mono text-xs text-slate-500">{{ (sourcePreviewState(source.id)?.entries || []).join(", ") }}</p>
        </div>

        <button v-if="sourceCdnSwitch(source)" type="button" class="ui-button ui-button-ghost mt-3 h-10 w-full px-2 text-xs" @click="toggleSourceCdn(source)">
          {{ sourceCdnSwitch(source)?.label }}
        </button>

        <div class="mt-3 grid grid-cols-4 gap-2">
          <button type="button" class="ui-button ui-button-ghost h-10 px-2 text-xs" :disabled="Boolean(sourceRequestState(source.id))" @click="toggleSourcePreview(source.id)">
            {{ previewButtonLabel(source.id) }}
          </button>
          <button type="button" class="ui-button ui-button-secondary h-10 px-2 text-xs" :disabled="Boolean(sourceRequestState(source.id))" @click="requestSourceFetch(source.id)">
            抓取
          </button>
          <button type="button" class="ui-button ui-button-ghost h-10 px-2 text-xs" @click="toggleSourceExpanded(source.id)">
            编辑
          </button>
          <button type="button" class="ui-button ui-button-ghost h-10 px-2 text-xs" @click="removeSource(source.id)">
            删除
          </button>
        </div>
      </template>

      <template v-else>
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <label class="block text-xs text-slate-500">名称</label>
          <input v-model="source.name" type="text" placeholder="输入源名称" class="ui-field h-11" />
        </div>
        <button type="button" class="ui-button ui-button-ghost px-3" @click="toggleSourceExpanded(source.id)">
          <PhCaretUp size="18" />
        </button>
        <button type="button" class="ui-button ui-button-ghost px-3" @click="removeSource(source.id)">
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
          <div class="mb-1 flex items-center justify-between gap-2">
            <label class="block text-xs text-slate-500">COLO 筛选</label>
            <div class="inline-flex rounded-full border border-slate-200 bg-slate-100 p-0.5">
              <button
                type="button"
                class="rounded-full px-2 py-1 text-[11px] font-semibold transition"
                :class="source.colo_filter_mode === 'allow' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'"
                @click="source.colo_filter_mode = 'allow'"
              >
                白
              </button>
              <button
                type="button"
                class="rounded-full px-2 py-1 text-[11px] font-semibold transition"
                :class="source.colo_filter_mode === 'deny' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'"
                @click="source.colo_filter_mode = 'deny'"
              >
                黑
              </button>
            </div>
          </div>
          <input v-model="source.colo_filter" placeholder="HKG,NRT" type="text" class="ui-field h-11 font-mono" />
          <p class="mt-1 text-[11px] text-slate-500">{{ sourceColoModeLabel(source.colo_filter_mode) }}模式</p>
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
            class="ui-field h-11 min-w-0 flex-1 font-mono"
          />
          <button type="button" class="ui-button ui-button-ghost h-11 px-3" @click="$emit('select-file', source.id)">
            <PhFolderOpen size="18" />
            选择
          </button>
        </div>
        <div v-else class="mt-1 flex flex-col gap-2 sm:flex-row">
          <input
            v-model="source.url"
            type="text"
            placeholder="https://example.com/ips.txt 或 example.com/ips.txt"
            class="ui-field h-11 min-w-0 flex-1 font-mono"
            @blur="emit('detect-source-name', source.id)"
            @change="emit('detect-source-name', source.id)"
          />
          <button v-if="sourceCdnSwitch(source)" type="button" class="ui-button ui-button-ghost h-11 px-3" @click="toggleSourceCdn(source)">
            {{ sourceCdnSwitch(source)?.label }}
          </button>
        </div>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-3">
        <button
          type="button"
          class="ui-button ui-button-ghost px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="toggleSourcePreview(source.id)"
        >
          <PhEye size="16" />
          {{ previewButtonLabel(source.id) }}
        </button>
        <button
          type="button"
          class="ui-button ui-button-secondary px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="requestSourceFetch(source.id)"
        >
          <PhArrowsClockwise size="16" />
          {{ sourceRequestState(source.id) === "fetch" ? "抓取中" : "抓取" }}
        </button>
      </div>

      <div class="overflow-safe mt-4 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-sm text-slate-600">
        <p>{{ sourceStatusText(source) }}</p>
        <p class="mt-1 text-xs text-slate-500">模式说明：{{ sourceModeCopy(source.ip_mode) }}</p>
      </div>

      <div
        v-if="sourcePreviewState(source.id) && isSourcePreviewVisible(source.id)"
        class="mt-4 rounded-xl border border-slate-200 bg-white px-3 py-3"
      >
        <p class="text-xs text-slate-500">
          预览结果 · 共 {{ sourcePreviewState(source.id)?.totalCount || 0 }} 条
        </p>
        <div class="mt-3 flex flex-wrap gap-2">
          <code
            v-for="entry in sourcePreviewState(source.id)?.entries || []"
            :key="entry"
            class="break-all rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-xs text-slate-700"
          >
            {{ entry }}
          </code>
        </div>
        <div
          v-if="(sourcePreviewState(source.id)?.warnings || []).length > 0"
          class="mt-3 space-y-1 text-xs text-amber-600"
        >
          <p v-for="warning in sourcePreviewState(source.id)?.warnings || []" :key="warning" class="break-all">{{ warning }}</p>
        </div>
      </div>

      <div class="mt-3 flex items-center justify-between gap-3 text-xs text-slate-500">
        <span class="min-w-0 truncate">{{ sourceTypeLabel(source.kind) }}</span>
        <span class="shrink-0">{{ source.enabled ? "任务启动时参与读取" : "任务启动时跳过" }}</span>
      </div>
      </template>
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
        <span class="text-[15px] font-bold tracking-[0.08em]">保存输入源</span>
      </button>
    </div>
  </section>
</template>

<style scoped>
.sources-header-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.875rem;
}

.sources-header-button {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  border-radius: 999px;
  padding: 0.95rem 1.75rem;
  font-size: 0.95rem;
  font-weight: 700;
  letter-spacing: 0;
  transition: all 0.2s ease;
}

.sources-header-button-primary {
  border: 1px solid transparent;
  background: #111827;
  color: #ffffff;
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.16);
}

.sources-header-button-primary:hover {
  background: #0f172a;
}

.sources-header-button-secondary {
  border: 1px solid rgb(226 232 240);
  background: rgb(255 255 255);
  color: #111827;
  box-shadow: 0 10px 22px rgba(15, 23, 42, 0.08);
}

.sources-header-button-secondary:hover {
  background: rgb(248 250 252);
}

:global(html[data-theme="dark"]) .sources-header-button-primary {
  background: #e5edf8;
  color: #0f172a;
  box-shadow: 0 16px 34px rgba(2, 6, 23, 0.34);
}

:global(html[data-theme="dark"]) .sources-header-button-primary:hover {
  background: #f8fafc;
}

:global(html[data-theme="dark"]) .sources-header-button-secondary {
  border-color: rgba(148, 163, 184, 0.22);
  background: #142033;
  color: #e5edf8;
  box-shadow: 0 18px 34px rgba(2, 6, 23, 0.3);
}

:global(html[data-theme="dark"]) .sources-header-button-secondary:hover {
  background: #1a2940;
}

@media (max-width: 639px) {
  .sources-header-actions {
    width: 100%;
  }

  .sources-header-button {
    flex: 1 1 100%;
  }
}
</style>
