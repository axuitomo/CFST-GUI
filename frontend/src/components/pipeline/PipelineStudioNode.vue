<script setup lang="ts">
import { computed, reactive, watch } from "vue";
import { Handle, Position, type NodeProps } from "@vue-flow/core";
import { PhCaretDown, PhCaretUp, PhCircleDashed, PhFlagBanner, PhWarningCircle, PhTrash } from "@phosphor-icons/vue";
import type { PipelineNode, PipelineNodeCatalogField, PipelineNodeCatalogItem } from "../../lib/bridge";
import { isFieldVisible, statusLabel, statusTone } from "../../lib/pipelineStudio";

interface StudioNodeData {
  actionLabel: string;
  catalogItem: PipelineNodeCatalogItem | null;
  configSummary: string;
  isEntry: boolean;
  issues: string[];
  message: string;
  nodeRef: PipelineNode;
  nodeType: string;
  nodeTypeLabel: string;
  sourceChoices: SourceChoice[];
  status: string;
}

interface SourceChoice {
  enabled: boolean;
  id: string;
  name: string;
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
    if (catalogItem.value.action === "select_sources" && field.key === "source_ids" && (props.data?.sourceChoices || []).length > 0) {
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

const selectedSourceIds = computed(() => {
  const value = nodeRef.value?.config?.source_ids;
  if (!Array.isArray(value)) {
    return new Set<string>();
  }
  return new Set(value.map((entry) => String(entry)).filter(Boolean));
});

const probeStages = [
  { description: "筛掉不可连通或延迟超阈值的候选。", title: "TCP延迟测速" },
  { description: "检查追踪连通性、状态码与延迟。", title: "追踪测试" },
  { description: "按当前下载策略测平均/峰值速度。", title: "下载测速" },
];

function toggleSourceId(sourceId: string, checked: boolean) {
  const next = new Set(selectedSourceIds.value);
  if (checked) {
    next.add(sourceId);
  } else {
    next.delete(sourceId);
  }
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
  <div
    class="group relative min-w-[18rem] rounded-3xl border p-4 transition-all duration-200"
    :class="[
      toneClass,
      selected ? 'border-primary/60 ring-1 ring-primary/30' : 'hover:border-black/20',
      dragging ? 'opacity-90' : '',
    ]"
    @contextmenu.prevent.stop="openContextMenu"
  >
    <Handle id="target" type="target" :position="Position.Left" class="!h-3 !w-3 !border-2 !border-white !bg-slate-500" :connectable="connectable" />
    <Handle v-if="data?.nodeType !== 'end'" id="source" type="source" :position="Position.Right" class="!h-3 !w-3 !border-2 !border-white !bg-primary" :connectable="connectable" />

    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          <span class="rounded-full border border-black/10 bg-[rgb(251,251,251)] px-2 py-0.5 text-[11px] font-semibold text-slate-500">{{ data?.nodeTypeLabel || "节点" }}</span>
          <span v-if="data?.isEntry" class="rounded-full border border-blue-200 bg-white px-2 py-0.5 text-[11px] font-semibold text-blue-700">起点</span>
          <span v-if="data?.issues?.length" class="rounded-full border border-rose-200 bg-white px-2 py-0.5 text-[11px] font-semibold text-rose-700">有问题</span>
        </div>

        <input
          v-if="selected && nodeRef"
          v-model="nodeRef.name"
          class="ui-field nodrag nopan mt-2 !rounded-2xl !px-3 !py-2 text-base font-semibold"
          @mousedown.stop
        />
        <p v-else class="mt-2 truncate text-base font-semibold text-slate-900">{{ label || id }}</p>
        <p class="mt-1 text-xs text-slate-500">{{ data?.actionLabel || "-" }}</p>
      </div>

      <span class="inline-flex items-center gap-1.5 rounded-full border px-2 py-1 text-[11px] font-semibold" :class="statusBadgeClass">
        <PhCircleDashed v-if="!data?.status" size="12" class="text-slate-400" />
        <span v-else class="h-1.5 w-1.5 rounded-full" :class="statusDotClass" />
        {{ statusLabel(data?.status || "idle") }}
      </span>
    </div>

    <div v-if="selected" class="mt-3 flex flex-wrap items-center gap-2">
      <button
        type="button"
        class="nodrag nopan inline-flex items-center gap-1 rounded-full border px-3 py-1.5 text-xs font-semibold transition"
        :class="data?.isEntry ? 'border-blue-200 bg-white text-blue-700' : 'border-black/10 bg-slate-50 text-slate-600 hover:border-black/20'"
        @mousedown.stop
        @click.stop="emit('set-entry', id)"
      >
        <PhFlagBanner size="14" />
        {{ data?.isEntry ? "当前起点" : "设为起点" }}
      </button>

      <button
        type="button"
        class="nodrag nopan inline-flex items-center gap-1 rounded-full border border-black/10 bg-slate-50 px-3 py-1.5 text-xs font-semibold text-slate-600 transition hover:border-black/20"
        @mousedown.stop
        @click.stop="emit('toggle-collapse', id)"
      >
        <PhCaretUp v-if="!isCollapsed" size="14" />
        <PhCaretDown v-else size="14" />
        {{ isCollapsed ? "展开配置" : "折叠节点" }}
      </button>

      <button
        type="button"
        class="nodrag nopan inline-flex items-center gap-1 rounded-full border border-rose-200 bg-rose-50 px-3 py-1.5 text-xs font-semibold text-rose-700 transition hover:bg-rose-100"
        @mousedown.stop
        @click.stop="emit('delete-node', id)"
      >
        <PhTrash size="14" />
        删除
      </button>
    </div>

    <div v-if="nodeRef?.action === 'run_probe'" class="mt-3 grid gap-2">
      <div v-for="(stage, index) in probeStages" :key="stage.title" class="rounded-2xl border border-slate-200 bg-slate-50/70 px-3 py-2">
        <div class="flex items-center justify-between gap-2">
          <p class="truncate text-xs font-semibold text-slate-700">{{ stage.title }}</p>
          <span class="shrink-0 rounded-full border border-black/10 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-500">阶段 {{ index + 1 }}</span>
        </div>
        <p v-if="isExpanded" class="mt-1 text-[11px] leading-5 text-slate-500">{{ stage.description }}</p>
      </div>
    </div>

    <p v-if="!isExpanded && data?.configSummary" class="mt-3 line-clamp-2 text-xs leading-5 text-slate-600">{{ data.configSummary }}</p>
    <p v-else-if="!isExpanded" class="mt-3 text-xs text-slate-400">选中节点后可直接在卡片里编辑配置。</p>

    <div v-if="!isExpanded && data?.message" class="mt-3 rounded-2xl border border-black/10 bg-[rgb(251,251,251)] px-3 py-2 text-xs leading-5 text-slate-600">
      {{ data.message }}
    </div>

    <div v-if="data?.issues?.length" class="mt-3 rounded-2xl border border-rose-200 bg-white px-3 py-2 text-xs text-slate-700">
      <div class="flex items-center gap-1 font-semibold text-rose-700">
        <PhWarningCircle size="14" />
        需要处理
      </div>
      <p class="mt-1 line-clamp-2">{{ data.issues[0] }}</p>
    </div>

    <div v-if="isExpanded" class="nodrag nopan mt-4 space-y-4 border-t border-black/10 pt-4" @mousedown.stop>
      <section v-if="nodeRef?.action === 'select_sources' && data?.sourceChoices?.length" class="rounded-2xl border border-slate-200 bg-slate-50/70 px-3 py-3">
        <p class="text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-400">输入组</p>
        <p class="mt-2 text-xs leading-5 text-slate-500">不勾选时使用全部启用输入源。</p>
        <div class="mt-3 space-y-2">
          <label v-for="source in data.sourceChoices" :key="source.id" class="flex items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700">
            <span class="min-w-0">
              <span class="block truncate font-medium">{{ source.name }}</span>
              <span class="block truncate text-xs text-slate-400">{{ source.id }} · {{ source.enabled ? "启用" : "停用" }}</span>
            </span>
            <input
              :checked="selectedSourceIds.has(source.id)"
              type="checkbox"
              class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary"
              @change="toggleSourceId(source.id, ($event.target as HTMLInputElement).checked)"
            />
          </label>
        </div>
      </section>

      <section v-for="group in groupedFields" :key="group.name" class="rounded-2xl border border-slate-200 bg-slate-50/70 px-3 py-3">
        <p class="text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-400">{{ group.name }}</p>
        <div class="mt-3 space-y-3">
          <div v-for="field in group.fields" :key="field.key">
            <label v-if="field.field_type === 'textarea'" class="block">
              <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
              <textarea
                :value="stringValue(field)"
                class="ui-field min-h-24 nodrag nopan !rounded-2xl"
                :placeholder="field.placeholder || ''"
                :rows="field.rows || 4"
                @input="setConfigValue(field.key, ($event.target as HTMLTextAreaElement).value)"
              ></textarea>
            </label>

            <label v-else-if="field.field_type === 'json'" class="block">
              <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
              <textarea
                :value="jsonDrafts[field.key]"
                class="min-h-28 w-full rounded-2xl border border-slate-200 bg-slate-950 px-3 py-3 font-mono text-xs text-slate-100 outline-none focus:border-primary"
                spellcheck="false"
                :rows="field.rows || 6"
                @input="updateJsonField(field, ($event.target as HTMLTextAreaElement).value)"
                @blur="commitJsonField(field)"
              ></textarea>
            </label>

            <label v-else-if="field.field_type === 'select'" class="block">
              <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
              <select :value="stringValue(field)" class="ui-field nodrag nopan !rounded-2xl" @change="setConfigValue(field.key, ($event.target as HTMLSelectElement).value)">
                <option v-for="option in field.options || []" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </label>

            <label v-else-if="field.field_type === 'checkbox'" class="inline-flex items-center gap-3 rounded-2xl border border-slate-200 bg-white px-3 py-3 text-sm text-slate-700">
              <input :checked="booleanValue(field)" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" @change="setConfigValue(field.key, ($event.target as HTMLInputElement).checked)" />
              <span>{{ field.label }}</span>
            </label>

            <label v-else-if="field.field_type === 'number'" class="block">
              <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
              <input
                :value="numberValue(field)"
                type="number"
                class="ui-field nodrag nopan !rounded-2xl"
                :max="field.max"
                :min="field.min"
                :placeholder="field.placeholder || ''"
                :step="field.step || 1"
                @input="setConfigValue(field.key, Number(($event.target as HTMLInputElement).value))"
              />
            </label>

            <label v-else class="block">
              <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
              <input
                :value="stringValue(field)"
                class="ui-field nodrag nopan !rounded-2xl"
                :placeholder="field.placeholder || ''"
                @input="setConfigValue(field.key, ($event.target as HTMLInputElement).value)"
              />
            </label>

            <p v-if="field.help_text" class="mt-2 text-xs leading-5 text-slate-500">{{ field.help_text }}</p>
          </div>
        </div>
      </section>

      <div v-if="groupedFields.length === 0" class="rounded-2xl border border-dashed border-slate-300 bg-slate-50/80 px-3 py-3 text-xs leading-5 text-slate-500">
        这个节点没有额外表单字段，当前主要依赖上游数据和连线关系。
      </div>

      <div v-if="data?.message" class="rounded-2xl border border-black/10 bg-[rgb(251,251,251)] px-3 py-3 text-xs leading-5 text-slate-600">
        {{ data.message }}
      </div>
    </div>
  </div>
</template>
