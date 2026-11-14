<script setup vapor lang="ts">
import { computed, ref, watch } from "vue";
import { PhCaretDown, PhCaretUp, PhDatabase, PhDotsThreeVertical, PhEye, PhFloppyDisk, PhFolderOpen, PhTrash } from "@phosphor-icons/vue";
import { sourceUrlCdnSwitch } from "../lib/sourceUrls";
import type { ColoFilterMode, SourceIPMode, SourceKind } from "../lib/bridge";
import type { SourceGroupIPMode } from "../lib/sourceGroups";
import { sourceInputIssue } from "../lib/sourceGroups";
import SourceAddMenu from "../components/ui/SourceAddMenu.vue";
import SourceGroupControls from "../components/ui/SourceGroupControls.vue";
import SourceFeedback from "../components/ui/SourceFeedback.vue";

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

interface SourceProfileItem {
  created_at: string;
  id: string;
  name: string;
  colo_filter: string;
  colo_filter_mode: ColoFilterMode | "";
  ip_mode: SourceGroupIPMode;
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
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  invalid: number;
  platform: "desktop" | "mobile";
  preparedCount: number;
  previewStates: Record<string, PreviewState | undefined>;
  requestStates: Record<string, string | undefined>;
  sourceProfiles: SourceProfileStore;
  sources: SourceEntry[];
  groupIpMode: SourceGroupIPMode;
  groupColoFilter: string;
  groupColoFilterMode: ColoFilterMode | "";
  previewErrors: Record<string, string | undefined>;
  taskStage: string;
}>();

const SOURCE_GROUP_EXPANDED_STORAGE_KEY = "cfst-source-group-expanded";

function loadSourceGroupExpanded() {
  try {
    return window.localStorage.getItem(SOURCE_GROUP_EXPANDED_STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

const sourceGroupExpanded = ref(loadSourceGroupExpanded());
watch(sourceGroupExpanded, (expanded) => {
  try {
    window.localStorage.setItem(SOURCE_GROUP_EXPANDED_STORAGE_KEY, String(expanded));
  } catch {
    // Ignore storage failures so source editing remains available.
  }
});
const sourceGroupSummary = computed(() => {
  const mode = props.groupIpMode === "mcis" ? "随机抽查" : props.groupIpMode ? "逐个检查" : "模式待统一";
  const filter = props.groupColoFilter.trim();
  const colo = filter ? `${props.groupColoFilterMode === "deny" ? "黑名单" : "白名单"} ${filter}` : "不限节点";
  return `${mode} · ${colo} · ${props.sources.length} 个源`;
});
const sourceProfileNameDraft = ref("");
const sourceProfilesExpanded = ref(false);
const expandedSourceIds = ref(new Set<string>());
const visiblePreviewSourceIds = ref(new Set<string>());
let knownSourceIds = new Set(props.sources.map((source) => source.id));
const activeSourceProfile = computed(() => props.sourceProfiles.items.find((profile) => profile.id === props.sourceProfiles.active_profile_id) || null);

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

  return profile.sources.map((source, index) => source.name.trim() || `输入源 ${index + 1}`).join("、");
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

function sourceTargetSummary(source: SourceEntry) {
  if (source.kind === "inline") {
    const lines = source.content
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean);
    if (lines.length === 0) return "未填写手动输入";
    return `${lines.length} 行：${lines[0]}`;
  }
  if (source.kind === "file") return source.path.trim() || "未选择文件";
  return source.url.trim() || "未填写 URL";
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
  return Array.isArray(value) ? value.map((entry) => Number(entry)).filter((entry) => Number.isFinite(entry) && entry > 0) : [];
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
    return "检查并预览";
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

function closeSourceMenu(event: Event) {
  if (event.currentTarget instanceof HTMLElement) {
    event.currentTarget.closest("details")?.removeAttribute("open");
  }
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
  (event: "add", kind: SourceKind): void;
  (event: "group-mode-change", mode: SourceIPMode): void;
  (event: "copy-current-source-profile"): void;
  (event: "delete-source-profile", profileId: string): void;
  (event: "detect-source-name", sourceId: string): void;
  (event: "preview", sourceId: string): void;
  (event: "preview-request", sourceId: string): void;
  (event: "remove", sourceId: string): void;
  (event: "group-colo-filter-change", filter: string): void;
  (event: "group-colo-filter-mode-change", mode: ColoFilterMode): void;
  (event: "save-source-profile", name: string, profileId?: string, sources?: SourceEntry[], setActive?: boolean, ipMode?: SourceGroupIPMode, coloFilter?: string, coloFilterMode?: ColoFilterMode | ""): void;
  (event: "auto-save"): void;
  (event: "update-current-source-profile"): void;
  (event: "select-file", sourceId: string): void;
  (event: "switch-source-profile", profileId: string): void;
}>();

function renameSourceProfile(profile: SourceProfileItem) {
  const nextName = window.prompt("新的输入组名称", profile.name)?.trim();
  if (!nextName || nextName === profile.name) {
    return;
  }
  if (isActiveSourceProfile(profile)) {
    emit("save-source-profile", nextName, profile.id, undefined, true);
    return;
  }
  emit("save-source-profile", nextName, profile.id, profile.sources, false, profile.ip_mode, profile.colo_filter, profile.colo_filter_mode);
}

function duplicateSourceProfile(profile: SourceProfileItem) {
  if (isActiveSourceProfile(profile)) {
    emit("copy-current-source-profile");
    return;
  }
  emit("save-source-profile", `${profile.name} 副本`, "", profile.sources, true, profile.ip_mode, profile.colo_filter, profile.colo_filter_mode);
}

function createBlankSourceProfile() {
  emit("save-source-profile", sourceProfileNameDraft.value, "", [], true, "traverse", "", "allow");
}

function updateActiveSourceProfile() {
  emit("update-current-source-profile");
}
</script>

<template>
  <section v-if="platform === 'desktop'" class="space-y-5" @click="$emit('auto-save')" @focusout="$emit('auto-save')">
    <article class="ui-card overflow-hidden">
      <div class="flex items-center justify-between gap-3 px-4 py-2 sm:px-5" :class="sourceProfilesExpanded || sourceGroupExpanded ? 'border-b border-slate-200' : ''">
        <div class="min-w-0">
          <div class="flex min-w-0 items-center gap-2">
            <span class="text-sm font-semibold text-slate-800">输入组</span>
            <span class="truncate text-sm text-slate-600">{{ activeSourceProfile?.name || "未选择输入组" }}</span>
            <span class="ui-pill ui-pill-subtle shrink-0">{{ sourceProfiles.items.length }} 组</span>
          </div>
          <p v-if="!sourceGroupExpanded" class="mt-1 truncate text-xs text-slate-500">{{ sourceGroupSummary }}</p>
        </div>
        <div class="flex shrink-0 items-center gap-1">
          <SourceAddMenu :compact="true" @add="emit('add', $event)" />
          <button type="button" class="ui-button ui-button-ghost h-9 w-9 p-0" :aria-label="sourceProfilesExpanded ? '收起输入组管理' : '展开输入组管理'" :title="sourceProfilesExpanded ? '收起输入组管理' : '展开输入组管理'" @click.stop="sourceProfilesExpanded = !sourceProfilesExpanded">
            <PhFolderOpen size="16" />
          </button>
          <button type="button" class="ui-button ui-button-ghost h-9 w-9 p-0" :aria-expanded="sourceGroupExpanded" :aria-label="sourceGroupExpanded ? '折叠输入组设置' : '展开输入组设置'" :title="sourceGroupExpanded ? '折叠输入组设置' : '展开输入组设置'" @click.stop="sourceGroupExpanded = !sourceGroupExpanded">
            <PhCaretUp v-if="sourceGroupExpanded" size="15" /><PhCaretDown v-else size="15" />
          </button>
        </div>
      </div>
      <Transition name="source-group">
        <div v-if="sourceGroupExpanded" class="source-group-shell">
          <SourceGroupControls
            :mode="groupIpMode"
            :can-copy="sources.length > 0"
            :colo-filter="groupColoFilter"
            :colo-filter-mode="groupColoFilterMode"
            @mode-change="emit('group-mode-change', $event)"
            @colo-filter-change="emit('group-colo-filter-change', $event)"
            @colo-filter-mode-change="emit('group-colo-filter-mode-change', $event)"
            @copy="emit('copy-current-source-profile')"
          />
        </div>
      </Transition>
      <Transition name="ui-reveal">
        <div v-if="sourceProfilesExpanded" class="grid gap-3 p-5 lg:grid-cols-[minmax(0,1fr)_auto]">
          <label class="min-w-0">
            <span class="ui-label">新建空白输入组</span>
            <input v-model="sourceProfileNameDraft" class="ui-field" placeholder="例如：VPS789 组合 / 自建源" type="text" />
          </label>
          <div class="flex flex-wrap items-end gap-2">
            <button type="button" class="ui-button ui-button-primary" @click.stop="createBlankSourceProfile">
              <PhFloppyDisk size="18" weight="fill" />
              新建空白输入组
            </button>
            <button type="button" class="ui-button ui-button-ghost" @click.stop="updateActiveSourceProfile">更新并保存当前输入组</button>
          </div>
        </div>
      </Transition>
      <Transition name="ui-reveal">
        <div v-if="sourceProfilesExpanded && sourceProfiles.items.length > 0" class="grid gap-3 border-t border-slate-100 p-5 pt-4 lg:grid-cols-2">
          <div v-for="profile in sourceProfiles.items" :key="profile.id" class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2.5">
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
              <button type="button" class="ui-button ui-button-ghost px-2.5 py-1.5" :disabled="isActiveSourceProfile(profile)" @click.stop="emit('switch-source-profile', profile.id)">切换</button>
              <button type="button" class="ui-button ui-button-ghost px-2.5 py-1.5" @click.stop="renameSourceProfile(profile)">重命名</button>
              <button type="button" class="ui-button ui-button-ghost px-2.5 py-1.5" @click.stop="duplicateSourceProfile(profile)">复制</button>
              <button type="button" class="ui-button ui-button-ghost px-2.5 py-1.5" @click.stop="emit('delete-source-profile', profile.id)">删除</button>
            </div>
          </div>
        </div>
      </Transition>
    </article>

    <div v-if="sources.length === 0" class="ui-card flex flex-col items-center gap-3 border-dashed px-5 py-8 text-center">
      <PhDatabase class="text-slate-300" size="40" />
      <p class="text-sm text-slate-500">添加链接、文件或手动名单后即可开始测速。IP 模式在输入组中统一设置。</p>
      <SourceAddMenu @add="emit('add', $event)" />
    </div>

    <article v-for="source in sources" :key="source.id" class="ui-card p-4">
      <Transition name="ui-reveal" mode="out-in">
        <div v-if="!isSourceExpanded(source.id)" key="summary" class="space-y-2">
          <div class="flex items-center gap-2">
            <div class="flex min-w-0 flex-1 items-center gap-2">
              <h3 class="max-w-[12rem] truncate text-sm font-semibold text-slate-800">{{ source.name || "未命名输入源" }}</h3>
              <span class="ui-pill ui-pill-subtle shrink-0">{{ sourceTypeLabel(source.kind) }}</span>
              <span v-if="source.kind !== 'url'" class="min-w-0 flex-1 truncate font-mono text-xs text-slate-500">{{ sourceTargetSummary(source) }}</span>
            </div>
            <button type="button" class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="source.enabled ? 'bg-primary' : 'bg-slate-300'" role="switch" :aria-checked="source.enabled" :aria-label="source.enabled ? '停用输入源' : '启用输入源'" @click="source.enabled = !source.enabled">
              <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="source.enabled ? 'translate-x-5' : 'translate-x-0'"></span>
            </button>
            <details class="relative shrink-0">
              <summary class="ui-button ui-button-ghost flex h-9 w-9 list-none items-center justify-center p-0 [&::-webkit-details-marker]:hidden" :aria-label="`${source.name || '输入源'} 更多操作`" title="更多操作"><PhDotsThreeVertical size="20" /></summary>
              <div class="absolute right-0 top-full z-20 mt-1 min-w-40 rounded-lg border border-slate-200 bg-white p-1 shadow-lg">
                <button
                  type="button"
                  class="w-full rounded-md px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-100 disabled:text-slate-400"
                  :disabled="Boolean(sourceRequestState(source.id)) || Boolean(sourceInputIssue(source))"
                  @click="
                    toggleSourcePreview(source.id);
                    closeSourceMenu($event);
                  "
                >
                  {{ previewButtonLabel(source.id) }}
                </button>
                <button
                  v-if="sourceCdnSwitch(source)"
                  type="button"
                  class="w-full rounded-md px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-100"
                  @click="
                    toggleSourceCdn(source);
                    closeSourceMenu($event);
                  "
                >
                  {{ sourceCdnSwitch(source)?.label }}
                </button>
                <button
                  type="button"
                  class="w-full rounded-md px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-100"
                  @click="
                    toggleSourceExpanded(source.id);
                    closeSourceMenu($event);
                  "
                >
                  编辑
                </button>
                <button
                  type="button"
                  class="w-full rounded-md px-3 py-2 text-left text-sm text-rose-600 hover:bg-rose-50"
                  @click="
                    removeSource(source.id);
                    closeSourceMenu($event);
                  "
                >
                  删除
                </button>
              </div>
            </details>
          </div>
          <div v-if="sourcePreviewState(source.id) && isSourcePreviewVisible(source.id)" class="min-w-0 rounded-lg border border-slate-200 bg-slate-50/70 px-3 py-2 text-sm text-slate-600">
            <p>{{ sourcePreviewSummary(source.id) }}</p>
            <p class="mt-1 truncate font-mono text-xs text-slate-500">{{ (sourcePreviewState(source.id)?.entries || []).join(", ") }}</p>
          </div>
        </div>
        <div v-else key="editor">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <label class="ui-label">名称（可选）</label>
              <input v-model="source.name" type="text" placeholder="例如：优选远程源 / 自建名单" class="ui-field" />
            </div>

            <div class="flex flex-wrap items-center justify-end gap-2 pt-5">
              <div class="flex items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1.5">
                <span class="text-sm font-medium text-slate-600">{{ source.enabled ? "已启用" : "已停用" }}</span>
                <button type="button" role="switch" :aria-checked="source.enabled" :aria-label="source.enabled ? '停用输入源' : '启用输入源'" class="relative inline-flex h-6 w-11 items-center rounded-full transition" :class="source.enabled ? 'bg-primary' : 'bg-slate-300'" @click="source.enabled = !source.enabled">
                  <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="source.enabled ? 'translate-x-5' : 'translate-x-0'"></span>
                </button>
              </div>

              <button type="button" class="ui-button ui-button-ghost px-3" @click="toggleSourceExpanded(source.id)">
                <PhCaretUp size="18" />
                收起
              </button>

              <button type="button" aria-label="删除输入源" class="ui-button ui-button-ghost px-3" @click="removeSource(source.id)">
                <PhTrash size="18" />
              </button>
            </div>
          </div>

          <p class="mt-3 text-xs text-slate-500">{{ sourceTypeLabel(source.kind) }}</p>

          <div class="mt-4">
            <label class="ui-label">{{ sourceFieldLabel(source.kind) }}</label>
            <textarea v-if="source.kind === 'inline'" v-model="source.content" rows="6" placeholder="# 支持注释和域名&#10;1.1.1.1 # inline note&#10;104.16.0.0/16&#10;example.com" class="ui-field min-h-32 font-mono" />
            <div v-else-if="source.kind === 'file'" class="flex flex-col gap-3 sm:flex-row">
              <input v-model="source.path" type="text" placeholder="/data/cfips/ip.txt" class="ui-field h-10 min-w-0 flex-1 font-mono" />
              <button type="button" class="ui-button ui-button-ghost px-4" @click="$emit('select-file', source.id)">
                <PhFolderOpen size="18" />
                选择文件
              </button>
            </div>
            <div v-else class="flex flex-col gap-2 sm:flex-row">
              <input v-model="source.url" type="text" placeholder="https://example.com/ips.txt 或 example.com/ips.txt" class="ui-field h-10 min-w-0 flex-1 font-mono" @blur="emit('detect-source-name', source.id)" @change="emit('detect-source-name', source.id)" />
              <button v-if="sourceCdnSwitch(source)" type="button" class="ui-button ui-button-ghost h-10 px-3" @click="toggleSourceCdn(source)">
                {{ sourceCdnSwitch(source)?.label }}
              </button>
            </div>
          </div>

          <details class="mt-3 rounded-xl border border-slate-200 bg-slate-50/70 px-3 py-3">
            <summary class="cursor-pointer text-sm text-slate-600">更多设置（类型）</summary>
            <label class="mt-3 block">
              <span class="ui-label">来源类型</span>
              <select v-model="source.kind" class="ui-field h-11">
                <option value="inline">手动输入</option>
                <option value="url">URL 列表</option>
                <option value="file">本地文件</option>
              </select>
            </label>
          </details>

          <div class="mt-3 flex flex-wrap gap-2">
            <button type="button" class="ui-button ui-button-ghost px-3" :disabled="Boolean(sourceRequestState(source.id)) || Boolean(sourceInputIssue(source))" @click="toggleSourcePreview(source.id)">
              <PhEye size="16" />
              {{ previewButtonLabel(source.id) }}
            </button>
          </div>

          <div v-if="sourcePreviewState(source.id) && isSourcePreviewVisible(source.id)" class="mt-3 rounded-xl border border-slate-200 bg-white px-3 py-3">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div class="min-w-0">
                <p class="text-xs uppercase tracking-[0.14em] text-slate-500">预览结果</p>
                <p class="mt-1 text-sm text-slate-700">
                  共 {{ sourcePreviewState(source.id)?.totalCount || 0 }} 条候选
                  <span v-if="(sourcePreviewState(source.id)?.invalidCount || 0) > 0"> ，忽略 {{ sourcePreviewState(source.id)?.invalidCount || 0 }} 条非法输入 </span>
                </p>
              </div>
              <p class="break-all text-xs text-slate-500">{{ sourcePreviewState(source.id)?.updatedAt ? formatTimestampLabel(sourcePreviewState(source.id)?.updatedAt || "") : "" }}</p>
            </div>

            <div class="mt-3 flex flex-wrap gap-2">
              <code v-for="entry in sourcePreviewState(source.id)?.entries || []" :key="entry" class="break-all rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-xs text-slate-700">
                {{ entry }}
              </code>
            </div>

            <div v-if="(sourcePreviewState(source.id)?.warnings || []).length > 0" class="mt-3 space-y-1 text-xs text-amber-600">
              <p v-for="warning in sourcePreviewState(source.id)?.warnings || []" :key="warning" class="break-all">{{ warning }}</p>
            </div>
          </div>
        </div>
      </Transition>
      <SourceFeedback :source="source" :busy="Boolean(sourceRequestState(source.id))" :error="previewErrors[source.id]" :preview="sourcePreviewState(source.id)" />
    </article>
    <div v-if="sources.length > 0" class="flex justify-end border-t border-slate-200 pt-3">
      <SourceAddMenu @add="emit('add', $event)" />
    </div>
  </section>

  <section v-else class="space-y-4" @click="$emit('auto-save')" @focusout="$emit('auto-save')">
    <article class="ui-card overflow-hidden">
      <div class="flex items-center justify-between gap-3 px-3 py-2" :class="sourceProfilesExpanded || sourceGroupExpanded ? 'border-b border-slate-100' : ''">
        <div class="min-w-0">
          <div class="flex min-w-0 items-center gap-1.5">
            <h3 class="shrink-0 text-sm font-semibold text-slate-800">输入组</h3>
            <span class="truncate text-sm text-slate-600">{{ activeSourceProfile?.name || "未选择" }}</span>
            <span class="ui-pill ui-pill-subtle shrink-0">{{ sourceProfiles.items.length }} 组</span>
          </div>
          <p v-if="!sourceGroupExpanded" class="mt-1 truncate text-xs text-slate-500">{{ sourceGroupSummary }}</p>
        </div>
        <div class="flex shrink-0 items-center gap-1">
          <SourceAddMenu :compact="true" @add="emit('add', $event)" />
          <button type="button" class="ui-button ui-button-ghost h-9 w-9 p-0" :aria-label="sourceProfilesExpanded ? '收起输入组管理' : '展开输入组管理'" :title="sourceProfilesExpanded ? '收起输入组管理' : '展开输入组管理'" @click.stop="sourceProfilesExpanded = !sourceProfilesExpanded">
            <PhFolderOpen size="16" />
          </button>
          <button type="button" class="ui-button ui-button-ghost h-9 w-9 p-0" :aria-expanded="sourceGroupExpanded" :aria-label="sourceGroupExpanded ? '折叠输入组设置' : '展开输入组设置'" :title="sourceGroupExpanded ? '折叠输入组设置' : '展开输入组设置'" @click.stop="sourceGroupExpanded = !sourceGroupExpanded">
            <PhCaretUp v-if="sourceGroupExpanded" size="14" /><PhCaretDown v-else size="14" />
          </button>
        </div>
      </div>
      <Transition name="source-group">
        <div v-if="sourceGroupExpanded" class="source-group-shell">
          <SourceGroupControls
            :mode="groupIpMode"
            :can-copy="sources.length > 0"
            :colo-filter="groupColoFilter"
            :colo-filter-mode="groupColoFilterMode"
            @mode-change="emit('group-mode-change', $event)"
            @colo-filter-change="emit('group-colo-filter-change', $event)"
            @colo-filter-mode-change="emit('group-colo-filter-mode-change', $event)"
            @copy="emit('copy-current-source-profile')"
          />
        </div>
      </Transition>
      <Transition name="ui-reveal">
        <div v-if="sourceProfilesExpanded" class="space-y-3 p-4">
          <div class="flex gap-2">
            <input v-model="sourceProfileNameDraft" class="ui-field h-11 min-w-0 flex-1" placeholder="输入组名称" type="text" />
            <button type="button" class="ui-button ui-button-primary h-11 px-3" @click.stop="createBlankSourceProfile">新建空白</button>
          </div>
          <button type="button" class="ui-button ui-button-ghost h-11 w-full" @click.stop="updateActiveSourceProfile">更新并保存当前输入组</button>
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
                <button type="button" class="ui-button ui-button-ghost h-9 px-3" :disabled="isActiveSourceProfile(profile)" @click.stop="emit('switch-source-profile', profile.id)">切换</button>
              </div>
              <div class="mt-3 grid grid-cols-3 gap-2">
                <button type="button" class="ui-button ui-button-ghost h-9 px-1.5 text-xs" @click.stop="renameSourceProfile(profile)">重命名</button>
                <button type="button" class="ui-button ui-button-ghost h-9 px-1.5 text-xs" @click.stop="duplicateSourceProfile(profile)">复制</button>
                <button type="button" class="ui-button ui-button-ghost h-9 px-1.5 text-xs" @click.stop="emit('delete-source-profile', profile.id)">删除</button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </article>

    <div v-if="sources.length === 0" class="ui-card border-2 border-dashed p-6 text-center">
      <p class="text-sm text-slate-500">暂无输入源。添加链接、文件或手动名单后即可开始测速。</p>
      <SourceAddMenu class="mt-3" @add="emit('add', $event)" />
    </div>

    <article v-for="source in sources" :key="source.id" class="ui-card p-4">
      <Transition name="ui-reveal" mode="out-in">
        <div v-if="!isSourceExpanded(source.id)" key="summary" class="space-y-2">
          <div class="flex items-center gap-2">
            <div class="flex min-w-0 flex-1 items-center gap-2">
              <h3 class="max-w-[7rem] truncate text-sm font-semibold text-slate-800">{{ source.name || "未命名输入源" }}</h3>
              <span class="ui-pill ui-pill-subtle shrink-0">{{ sourceTypeLabel(source.kind) }}</span>
              <span v-if="source.kind !== 'url'" class="min-w-0 flex-1 truncate font-mono text-xs text-slate-500">{{ sourceTargetSummary(source) }}</span>
            </div>
            <button type="button" class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="source.enabled ? 'bg-primary' : 'bg-slate-300'" role="switch" :aria-checked="source.enabled" :aria-label="source.enabled ? '停用输入源' : '启用输入源'" @click="source.enabled = !source.enabled">
              <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="source.enabled ? 'translate-x-5' : 'translate-x-0'"></span>
            </button>
            <details class="relative shrink-0">
              <summary class="ui-button ui-button-ghost flex h-9 w-9 list-none items-center justify-center p-0 [&::-webkit-details-marker]:hidden" :aria-label="`${source.name || '输入源'} 更多操作`" title="更多操作"><PhDotsThreeVertical size="20" /></summary>
              <div class="absolute right-0 top-full z-20 mt-1 min-w-40 rounded-lg border border-slate-200 bg-white p-1 shadow-lg">
                <button
                  type="button"
                  class="w-full rounded-md px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-100 disabled:text-slate-400"
                  :disabled="Boolean(sourceRequestState(source.id)) || Boolean(sourceInputIssue(source))"
                  @click="
                    toggleSourcePreview(source.id);
                    closeSourceMenu($event);
                  "
                >
                  {{ previewButtonLabel(source.id) }}
                </button>
                <button
                  v-if="sourceCdnSwitch(source)"
                  type="button"
                  class="w-full rounded-md px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-100"
                  @click="
                    toggleSourceCdn(source);
                    closeSourceMenu($event);
                  "
                >
                  {{ sourceCdnSwitch(source)?.label }}
                </button>
                <button
                  type="button"
                  class="w-full rounded-md px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-100"
                  @click="
                    toggleSourceExpanded(source.id);
                    closeSourceMenu($event);
                  "
                >
                  编辑
                </button>
                <button
                  type="button"
                  class="w-full rounded-md px-3 py-2 text-left text-sm text-rose-600 hover:bg-rose-50"
                  @click="
                    removeSource(source.id);
                    closeSourceMenu($event);
                  "
                >
                  删除
                </button>
              </div>
            </details>
          </div>
          <div v-if="sourcePreviewState(source.id) && isSourcePreviewVisible(source.id)" class="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">
            <p>{{ sourcePreviewSummary(source.id) }}</p>
            <p class="mt-1 truncate font-mono text-xs text-slate-500">{{ (sourcePreviewState(source.id)?.entries || []).join(", ") }}</p>
          </div>
        </div>
        <div v-else key="editor">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <label class="block text-xs text-slate-500">名称（可选）</label>
              <input v-model="source.name" type="text" placeholder="输入源名称" class="ui-field h-11" />
            </div>
            <button type="button" aria-label="收起" class="ui-button ui-button-ghost px-3" @click="toggleSourceExpanded(source.id)">
              <PhCaretUp size="18" />
            </button>
            <button type="button" aria-label="删除输入源" class="ui-button ui-button-ghost px-3" @click="removeSource(source.id)">
              <PhTrash size="18" />
            </button>
          </div>

          <div class="mt-4 flex items-center justify-between gap-3 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
            <div class="min-w-0">
              <p class="text-xs text-slate-500">启用状态</p>
              <p class="mt-1 text-sm font-medium text-slate-700">{{ source.enabled ? "已启用" : "已停用" }}</p>
            </div>
            <button type="button" role="switch" :aria-checked="source.enabled" :aria-label="source.enabled ? '停用输入源' : '启用输入源'" class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="source.enabled ? 'bg-primary' : 'bg-slate-300'" @click="source.enabled = !source.enabled">
              <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="source.enabled ? 'translate-x-5' : 'translate-x-0'"></span>
            </button>
          </div>

          <p class="mt-3 text-xs text-slate-500">{{ sourceTypeLabel(source.kind) }}</p>

          <div class="mt-4">
            <label class="block text-xs text-slate-500">{{ sourceFieldLabel(source.kind) }}</label>
            <textarea v-if="source.kind === 'inline'" v-model="source.content" rows="5" placeholder="# 支持注释和域名&#10;1.1.1.1 # inline note&#10;example.com" class="ui-field mt-1 min-h-28 font-mono" />
            <div v-else-if="source.kind === 'file'" class="mt-1 flex gap-2">
              <input v-model="source.path" type="text" placeholder="/data/cfips/ip.txt" class="ui-field h-11 min-w-0 flex-1 font-mono" />
              <button type="button" class="ui-button ui-button-ghost h-11 px-3" @click="$emit('select-file', source.id)">
                <PhFolderOpen size="18" />
                选择
              </button>
            </div>
            <div v-else class="mt-1 flex flex-col gap-2 sm:flex-row">
              <input v-model="source.url" type="text" placeholder="https://example.com/ips.txt 或 example.com/ips.txt" class="ui-field h-11 min-w-0 flex-1 font-mono" @blur="emit('detect-source-name', source.id)" @change="emit('detect-source-name', source.id)" />
              <button v-if="sourceCdnSwitch(source)" type="button" class="ui-button ui-button-ghost h-11 px-3" @click="toggleSourceCdn(source)">
                {{ sourceCdnSwitch(source)?.label }}
              </button>
            </div>
          </div>

          <details class="mt-4 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
            <summary class="cursor-pointer text-sm text-slate-600">更多设置（类型）</summary>
            <label class="mt-3 block">
              <span class="ui-label">来源类型</span>
              <select v-model="source.kind" class="ui-field h-11">
                <option value="inline">手动输入</option>
                <option value="url">URL 列表</option>
                <option value="file">本地文件</option>
              </select>
            </label>
          </details>

          <div class="mt-4 grid grid-cols-1 gap-3">
            <button type="button" class="ui-button ui-button-ghost px-3" :disabled="Boolean(sourceRequestState(source.id)) || Boolean(sourceInputIssue(source))" @click="toggleSourcePreview(source.id)">
              <PhEye size="16" />
              {{ previewButtonLabel(source.id) }}
            </button>
          </div>

          <div v-if="sourcePreviewState(source.id) && isSourcePreviewVisible(source.id)" class="mt-4 rounded-xl border border-slate-200 bg-white px-3 py-3">
            <p class="text-xs text-slate-500">预览结果 · 共 {{ sourcePreviewState(source.id)?.totalCount || 0 }} 条</p>
            <div class="mt-3 flex flex-wrap gap-2">
              <code v-for="entry in sourcePreviewState(source.id)?.entries || []" :key="entry" class="break-all rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-xs text-slate-700">
                {{ entry }}
              </code>
            </div>
            <div v-if="(sourcePreviewState(source.id)?.warnings || []).length > 0" class="mt-3 space-y-1 text-xs text-amber-600">
              <p v-for="warning in sourcePreviewState(source.id)?.warnings || []" :key="warning" class="break-all">{{ warning }}</p>
            </div>
          </div>

          <div class="mt-3 flex items-center justify-between gap-3 text-xs text-slate-500">
            <span class="min-w-0 truncate">{{ sourceTypeLabel(source.kind) }}</span>
            <span class="shrink-0">{{ source.enabled ? "任务启动时参与读取" : "任务启动时跳过" }}</span>
          </div>
        </div>
      </Transition>
      <SourceFeedback :source="source" :busy="Boolean(sourceRequestState(source.id))" :error="previewErrors[source.id]" :preview="sourcePreviewState(source.id)" />
    </article>
    <div v-if="sources.length > 0" class="flex justify-end border-t border-slate-200 pt-3">
      <SourceAddMenu @add="emit('add', $event)" />
    </div>
  </section>
</template>
