<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { Handle, Position, type NodeProps } from "@vue-flow/core";
import { PhCaretDown, PhCaretUp, PhCircleDashed, PhFlagBanner, PhWarningCircle, PhTrash } from "@phosphor-icons/vue";
import type { PipelineNode, PipelineNodeCatalogField, PipelineNodeCatalogItem } from "../../lib/bridge";
import { isFieldVisible, statusLabel, statusTone } from "../../lib/pipelineStudio";

interface StudioNodeData {
  actionLabel: string;
  catalogItem: PipelineNodeCatalogItem | null;
  canonicalNodeId: string;
  configSummary: string;
  isEntry: boolean;
  issues: string[];
  message: string;
  nodeRef: PipelineNode;
  nodeType: string;
  nodeTypeLabel: string;
  sourceChoices: SourceChoice[];
  sourceChoiceGroups: SourceChoiceGroup[];
  status: string;
}

interface SourceChoice {
  enabled: boolean;
  id: string;
  kind: string;
  name: string;
  path: string;
  url: string;
}

interface SourceChoiceGroup {
  id: string;
  label: string;
  sources: SourceChoice[];
}

const props = defineProps<NodeProps<StudioNodeData>>();

const emit = defineEmits<{
  (event: "delete-node", nodeId: string): void;
  (event: "node-contextmenu", payload: { event: MouseEvent; nodeId: string }): void;
  (event: "set-entry", nodeId: string): void;
  (event: "toggle-collapse", nodeId: string): void;
}>();

const jsonDrafts = reactive<Record<string, string>>({});

const nodeRef = computed(() => props.data?.nodeRef || null);
const catalogItem = computed(() => props.data?.catalogItem || null);
const isCollapsed = computed(() => nodeRef.value?.ui?.collapsed === true);
const isExpanded = computed(() => props.selected && !isCollapsed.value);
const canSetEntry = computed(() => true);
const sourceSearch = ref("");

const tone = computed(() => statusTone(props.data?.status || ""));

const toneClass = computed(() => {
  if (tone.value === "success") {
    return "border-emerald-200 bg-white shadow-[0_10px_24px_rgba(15,23,42,0.04)]";
  }
  if (tone.value === "error") {
    return "border-rose-200 bg-white shadow-[0_10px_24px_rgba(15,23,42,0.04)]";
  }
  if (tone.value === "warning") {
    return "border-amber-200 bg-white shadow-[0_10px_24px_rgba(15,23,42,0.04)]";
  }
  if (tone.value === "running") {
    return "border-blue-200 bg-white shadow-[0_10px_24px_rgba(15,23,42,0.04)]";
  }
  return "border-black/10 bg-white shadow-[0_10px_24px_rgba(15,23,42,0.04)]";
});

const statusBadgeClass = computed(() => {
  if (tone.value === "success") {
    return "border-emerald-200 bg-white text-emerald-700";
  }
  if (tone.value === "error") {
    return "border-rose-200 bg-white text-rose-700";
  }
  if (tone.value === "warning") {
    return "border-amber-200 bg-white text-amber-700";
  }
  if (tone.value === "running") {
    return "border-blue-200 bg-white text-blue-700";
  }
  return "border-black/10 bg-[rgb(251,251,251)] text-slate-500";
});

const statusDotClass = computed(() => {
  if (tone.value === "success") {
    return "bg-emerald-500";
  }
  if (tone.value === "error") {
    return "bg-rose-500";
  }
  if (tone.value === "warning") {
    return "bg-amber-500";
  }
  if (tone.value === "running") {
    return "bg-blue-500";
  }
  return "bg-slate-400";
});

const groupedFields = computed(() => {
  if (!nodeRef.value || !catalogItem.value) {
    return [];
  }
  const groups = new Map<string, PipelineNodeCatalogField[]>();
  for (const field of catalogItem.value.form_schema || []) {
    if (!isFieldVisible(field, nodeRef.value.config || {})) {
      continue;
    }
    if (catalogItem.value.action === "select_sources" && ["source_ids", "source_profile_id", "source_selection"].includes(field.key)) {
      continue;
    }
    const groupName = field.group || "常规";
    groups.set(groupName, [...(groups.get(groupName) || []), field]);
  }
  return [...groups.entries()].map(([name, fields]) => ({ fields, name }));
});

watch(
  () => [nodeRef.value?.id, catalogItem.value?.action],
  () => {
    sourceSearch.value = "";
    for (const key of Object.keys(jsonDrafts)) {
      delete jsonDrafts[key];
    }
    if (!nodeRef.value) {
      return;
    }
    for (const field of catalogItem.value?.form_schema || []) {
      if (field.field_type === "json") {
        jsonDrafts[field.key] = JSON.stringify(nodeRef.value.config?.[field.key] ?? {}, null, 2);
      }
    }
  },
  { immediate: true },
);

function stringValue(field: PipelineNodeCatalogField) {
  if (!nodeRef.value) {
    return "";
  }
  const value = nodeRef.value.config?.[field.key];
  if (value === undefined || value === null || value === "") {
    return typeof field.default_value === "string" ? field.default_value : "";
  }
  if (typeof value === "object") {
    return JSON.stringify(value);
  }
  return String(value);
}

function booleanValue(field: PipelineNodeCatalogField) {
  if (!nodeRef.value) {
    return Boolean(field.default_value);
  }
  const value = nodeRef.value.config?.[field.key];
  return typeof value === "boolean" ? value : Boolean(field.default_value);
}

function numberValue(field: PipelineNodeCatalogField) {
  if (!nodeRef.value) {
    return typeof field.default_value === "number" ? field.default_value : 0;
  }
  const value = Number(nodeRef.value.config?.[field.key]);
  if (Number.isFinite(value)) {
    return value;
  }
  return typeof field.default_value === "number" ? field.default_value : 0;
}

function setConfigValue(key: string, value: unknown) {
  if (!nodeRef.value) {
    return;
  }
  nodeRef.value.config = {
    ...(nodeRef.value.config || {}),
    [key]: value,
  };
}

function inputValue(event: Event) {
  return (event.currentTarget as HTMLInputElement).value;
}

function textareaValue(event: Event) {
  return (event.currentTarget as HTMLTextAreaElement).value;
}

function selectValue(event: Event) {
  return (event.currentTarget as HTMLSelectElement).value;
}

function checkedValue(event: Event) {
  return (event.currentTarget as HTMLInputElement).checked;
}

function numberInputValue(event: Event) {
  return Number(inputValue(event));
}

const selectedSourceIds = computed(() => {
  if (sourceSelectionMode.value !== "custom") {
    return new Set((props.data?.sourceChoices || []).filter((source) => source.enabled).map((source) => source.id));
  }
  const value = nodeRef.value?.config?.source_ids;
  if (!Array.isArray(value)) {
    return new Set<string>();
  }
  return new Set(value.map((entry) => String(entry)).filter(Boolean));
});

const sourceChoiceGroups = computed(() => props.data?.sourceChoiceGroups || []);

const sourceProfileId = computed(() => String(nodeRef.value?.config?.source_profile_id || ""));

const sourceSelectionMode = computed(() => String(nodeRef.value?.config?.source_selection || "enabled"));

const enabledSelectedCount = computed(() => selectedSourceIds.value.size);

const filteredSourceChoices = computed(() => {
  const search = sourceSearch.value.trim().toLowerCase();
  const sources = props.data?.sourceChoices || [];
  if (!search) {
    return sources;
  }
  return sources.filter((source) => [source.name, source.id, source.url, source.path, source.kind].join(" ").toLowerCase().includes(search));
});

function changeSourceProfile(profileId: string) {
  setConfigValue("source_profile_id", profileId);
  setConfigValue("source_selection", "enabled");
  setConfigValue("source_ids", []);
  sourceSearch.value = "";
}

function useEnabledSources() {
  setConfigValue("source_selection", "enabled");
  setConfigValue("source_ids", []);
}

function toggleSourceId(sourceId: string, checked: boolean) {
  const next = new Set(selectedSourceIds.value);
  if (checked) {
    next.add(sourceId);
  } else {
    next.delete(sourceId);
  }
  setConfigValue("source_selection", "custom");
  setConfigValue("source_ids", [...next]);
}

function updateJsonField(field: PipelineNodeCatalogField, raw: string) {
  jsonDrafts[field.key] = raw;
}

function commitJsonField(field: PipelineNodeCatalogField) {
  if (!nodeRef.value) {
    return;
  }
  try {
    const parsed = jsonDrafts[field.key].trim() ? JSON.parse(jsonDrafts[field.key]) : {};
    setConfigValue(field.key, parsed);
  } catch {
    // Keep draft for user correction.
  }
}

function openContextMenu(event: MouseEvent) {
  emit("node-contextmenu", {
    event,
    nodeId: props.id,
  });
}
</script>

<template>
  <div class="workflow-node group relative rounded-3xl border p-4 transition-all duration-200" :class="['min-w-[18rem]', toneClass, selected ? 'border-primary/60 ring-1 ring-primary/30' : 'hover:border-black/20', dragging ? 'opacity-90' : '']" @contextmenu.prevent.stop="openContextMenu">
    <Handle id="target" type="target" :position="Position.Left" class="!h-3 !w-3 !border-2 !border-white !bg-slate-500" :connectable="connectable" />
    <Handle v-if="data?.nodeType !== 'end'" id="source" type="source" :position="Position.Right" class="!h-3 !w-3 !border-2 !border-white !bg-primary" :connectable="connectable" />

    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <div class="workflow-node-badges flex flex-wrap items-center gap-2">
          <span class="workflow-node-badge rounded-full border px-2 py-0.5 text-[11px] font-semibold">{{ data?.nodeTypeLabel || "节点" }}</span>
          <span v-if="data?.isEntry" class="workflow-node-badge workflow-node-badge-entry rounded-full border px-2 py-0.5 text-[11px] font-semibold">起点</span>
          <span v-if="data?.issues?.length" class="workflow-node-badge workflow-node-badge-danger rounded-full border px-2 py-0.5 text-[11px] font-semibold">有问题</span>
        </div>

        <input v-if="selected && nodeRef" v-model="nodeRef.name" class="ui-field nodrag nopan mt-2 !rounded-2xl !px-3 !py-2 text-base font-semibold" @mousedown.stop />
        <p v-else class="mt-2 truncate text-base font-semibold">{{ label || id }}</p>
        <p class="mt-1 text-xs">{{ data?.actionLabel || "-" }}</p>
      </div>

      <span class="inline-flex items-center gap-1.5 rounded-full border px-2 py-1 text-[11px] font-semibold" :class="statusBadgeClass">
        <PhCircleDashed v-if="!data?.status" size="12" class="text-slate-400" />
        <span v-else class="h-1.5 w-1.5 rounded-full" :class="statusDotClass" />
        {{ statusLabel(data?.status || "idle") }}
      </span>
    </div>

    <div v-if="selected" class="mt-3 flex flex-wrap items-center gap-2">
      <button v-if="canSetEntry" type="button" class="workflow-node-action nodrag nopan inline-flex items-center gap-1 rounded-full border px-3 py-1.5 text-xs font-semibold transition" :class="data?.isEntry ? 'workflow-node-action-entry' : ''" @mousedown.stop @click.stop="emit('set-entry', id)">
        <PhFlagBanner size="14" />
        {{ data?.isEntry ? "当前起点" : "设为起点" }}
      </button>

      <button type="button" class="workflow-node-action nodrag nopan inline-flex items-center gap-1 rounded-full border px-3 py-1.5 text-xs font-semibold transition" @mousedown.stop @click.stop="emit('toggle-collapse', id)">
        <PhCaretUp v-if="!isCollapsed" size="14" />
        <PhCaretDown v-else size="14" />
        {{ isCollapsed ? "展开配置" : "折叠节点" }}
      </button>

      <button type="button" class="workflow-node-action workflow-node-action-danger nodrag nopan inline-flex items-center gap-1 rounded-full border px-3 py-1.5 text-xs font-semibold transition" @mousedown.stop @click.stop="emit('delete-node', id)">
        <PhTrash size="14" />
        删除
      </button>
    </div>

    <p v-if="!isExpanded && data?.configSummary" class="workflow-node-summary mt-3 line-clamp-2 text-xs leading-5">{{ data.configSummary }}</p>
    <p v-else-if="!isExpanded" class="workflow-node-muted mt-3 text-xs">选中节点后可直接在卡片里编辑配置。</p>

    <div v-if="!isExpanded && data?.message" class="workflow-node-message mt-3 rounded-2xl border px-3 py-2 text-xs leading-5">
      {{ data.message }}
    </div>

    <div v-if="data?.issues?.length" class="workflow-node-issue mt-3 rounded-2xl border px-3 py-2 text-xs">
      <div class="flex items-center gap-1 font-semibold">
        <PhWarningCircle size="14" />
        需要处理
      </div>
      <p class="mt-1 line-clamp-2">{{ data.issues[0] }}</p>
    </div>

    <div v-if="isExpanded" class="workflow-node-expanded nodrag nopan mt-4 space-y-4 border-t pt-4" @mousedown.stop>
      <section v-if="nodeRef?.action === 'select_sources'" class="workflow-node-group rounded-2xl border px-3 py-3">
        <p class="text-[11px] font-semibold uppercase tracking-[0.12em]">输入组</p>
        <label class="mt-3 block">
          <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">输入组档案</span>
          <select :value="sourceProfileId" class="ui-field nodrag nopan !rounded-2xl" @change="changeSourceProfile(selectValue($event))">
            <option v-for="group in sourceChoiceGroups" :key="group.id || 'bound-config'" :value="group.id">{{ group.label }}</option>
          </select>
        </label>
        <div class="mt-3 flex items-center justify-between gap-2 text-xs leading-5">
          <span>{{ sourceSelectionMode === "custom" ? `自定义 ${enabledSelectedCount} 个输入源` : "使用全部启用输入源" }}</span>
          <button v-if="sourceSelectionMode === 'custom'" type="button" class="workflow-node-action nodrag nopan rounded-full border px-2.5 py-1 font-semibold" @click.stop="useEnabledSources">恢复全部启用</button>
        </div>
        <input v-model="sourceSearch" class="ui-field nodrag nopan mt-3 !rounded-2xl" placeholder="筛选输入源名称、ID、URL 或路径..." @mousedown.stop />
        <div class="mt-3 space-y-2">
          <p v-if="filteredSourceChoices.length === 0" class="workflow-node-empty rounded-2xl border border-dashed px-3 py-3 text-xs">没有匹配的输入源。</p>
          <label v-for="source in filteredSourceChoices" :key="source.id" class="workflow-node-source flex items-center justify-between gap-3 rounded-2xl border px-3 py-2 text-sm">
            <span class="min-w-0">
              <span class="block truncate font-medium">{{ source.name }}</span>
              <span class="workflow-node-source-meta block truncate text-xs">{{ source.id }} · {{ source.enabled ? "启用" : "停用" }}{{ source.url || source.path ? ` · ${source.url || source.path}` : "" }}</span>
            </span>
            <input :checked="selectedSourceIds.has(source.id)" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" @change="toggleSourceId(source.id, checkedValue($event))" />
          </label>
        </div>
      </section>

      <section v-for="group in groupedFields" :key="group.name" class="workflow-node-group rounded-2xl border px-3 py-3">
        <p class="text-[11px] font-semibold uppercase tracking-[0.12em]">{{ group.name }}</p>
        <div class="mt-3 space-y-3">
          <div v-for="field in group.fields" :key="field.key">
            <label v-if="field.field_type === 'textarea'" class="block">
              <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
              <textarea :value="stringValue(field)" class="ui-field min-h-24 nodrag nopan !rounded-2xl" :placeholder="field.placeholder || ''" :rows="field.rows || 4" @input="setConfigValue(field.key, textareaValue($event))"></textarea>
            </label>

            <label v-else-if="field.field_type === 'json'" class="block">
              <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
              <textarea :value="jsonDrafts[field.key]" class="workflow-node-json min-h-28 w-full rounded-2xl border px-3 py-3 font-mono text-xs outline-none focus:border-primary" spellcheck="false" :rows="field.rows || 6" @input="updateJsonField(field, textareaValue($event))" @blur="commitJsonField(field)"></textarea>
            </label>

            <label v-else-if="field.field_type === 'select'" class="block">
              <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
              <select :value="stringValue(field)" class="ui-field nodrag nopan !rounded-2xl" @change="setConfigValue(field.key, selectValue($event))">
                <option v-for="option in field.options || []" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </label>

            <label v-else-if="field.field_type === 'checkbox'" class="workflow-node-check inline-flex items-center gap-3 rounded-2xl border px-3 py-3 text-sm">
              <input :checked="booleanValue(field)" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" @change="setConfigValue(field.key, checkedValue($event))" />
              <span>{{ field.label }}</span>
            </label>

            <label v-else-if="field.field_type === 'number'" class="block">
              <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
              <input :value="numberValue(field)" type="number" class="ui-field nodrag nopan !rounded-2xl" :max="field.max" :min="field.min" :placeholder="field.placeholder || ''" :step="field.step || 1" @input="setConfigValue(field.key, numberInputValue($event))" />
            </label>

            <label v-else class="block">
              <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
              <input :value="stringValue(field)" class="ui-field nodrag nopan !rounded-2xl" :placeholder="field.placeholder || ''" @input="setConfigValue(field.key, inputValue($event))" />
            </label>

            <p v-if="field.help_text" class="mt-2 text-xs leading-5 text-slate-500">{{ field.help_text }}</p>
          </div>
        </div>
      </section>

      <div v-if="groupedFields.length === 0" class="workflow-node-empty rounded-2xl border border-dashed px-3 py-3 text-xs leading-5">这个节点没有额外表单字段，当前主要依赖上游数据和连线关系。</div>

      <div v-if="data?.message" class="workflow-node-message rounded-2xl border px-3 py-3 text-xs leading-5">
        {{ data.message }}
      </div>
    </div>
  </div>
</template>
