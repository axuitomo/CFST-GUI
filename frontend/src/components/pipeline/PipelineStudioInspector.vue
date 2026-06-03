<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { PhArrowsClockwise, PhGitBranch, PhSelectionBackground, PhTrash } from "@phosphor-icons/vue";
import type { PipelineEdge, PipelineNode, PipelineNodeCatalogField, PipelineNodeCatalogItem, PipelineTemplate } from "../../lib/bridge";
import type { PipelineTemplateIssue } from "../../lib/pipelineStudio";
import { branchOutcomes, isFieldVisible, nodeTypeLabel } from "../../lib/pipelineStudio";

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

const props = defineProps<{
  activeTemplate: PipelineTemplate | null;
  issues: PipelineTemplateIssue[];
  nodeCatalog: PipelineNodeCatalogItem[];
  selectedEdge: PipelineEdge | null;
  selectedEdgeIds: string[];
  selectedNode: PipelineNode | null;
  selectedNodeIds: string[];
  sourceChoiceGroups: SourceChoiceGroup[];
  sourceOptions: PipelineNode[];
  targetOptions: PipelineNode[];
}>();

const emit = defineEmits<{
  (event: "apply-layout"): void;
  (event: "edge-source-change", value: string): void;
  (event: "edge-target-change", value: string): void;
  (event: "remove-edge", edgeId: string): void;
  (event: "remove-node", nodeId: string): void;
  (event: "remove-selection"): void;
  (event: "set-entry-node", nodeId: string): void;
}>();

const jsonDrafts = reactive<Record<string, string>>({});
const sourceSearch = ref("");

const selectedCatalogItem = computed(() => props.nodeCatalog.find((item) => item.action === props.selectedNode?.action) || null);
const edgeSourceNode = computed(() => {
  if (!props.activeTemplate || !props.selectedEdge) {
    return null;
  }
  return props.activeTemplate.nodes.find((node) => node.id === props.selectedEdge?.source_node_id) || null;
});
const branchOptions = computed(() => {
  if (props.selectedNode) {
    return branchOutcomes(props.selectedNode, props.nodeCatalog);
  }
  if (edgeSourceNode.value) {
    return branchOutcomes(edgeSourceNode.value, props.nodeCatalog);
  }
  return [];
});

const groupedFields = computed(() => {
  if (!props.selectedNode || !selectedCatalogItem.value) {
    return [];
  }
  const groups = new Map<string, PipelineNodeCatalogField[]>();
  for (const field of selectedCatalogItem.value.form_schema || []) {
    if (!isFieldVisible(field, props.selectedNode.config || {})) {
      continue;
    }
    if (selectedCatalogItem.value.action === "select_sources" && ["source_ids", "source_profile_id", "source_selection"].includes(field.key)) {
      continue;
    }
    const groupName = field.group || "常规";
    groups.set(groupName, [...(groups.get(groupName) || []), field]);
  }
  return [...groups.entries()].map(([name, fields]) => ({ fields, name }));
});

watch(
  () => props.selectedNode?.id,
  () => {
    sourceSearch.value = "";
    for (const key of Object.keys(jsonDrafts)) {
      delete jsonDrafts[key];
    }
    if (!props.selectedNode) {
      return;
    }
    for (const field of selectedCatalogItem.value?.form_schema || []) {
      if (field.field_type === "json") {
        jsonDrafts[field.key] = JSON.stringify(props.selectedNode.config?.[field.key] ?? {}, null, 2);
      }
    }
  },
  { immediate: true },
);

function stringValue(field: PipelineNodeCatalogField) {
  if (!props.selectedNode) {
    return "";
  }
  const value = props.selectedNode.config?.[field.key];
  if (value === undefined || value === null || value === "") {
    return typeof field.default_value === "string" ? field.default_value : "";
  }
  if (typeof value === "object") {
    return JSON.stringify(value);
  }
  return String(value);
}

function booleanValue(field: PipelineNodeCatalogField) {
  if (!props.selectedNode) {
    return Boolean(field.default_value);
  }
  const value = props.selectedNode.config?.[field.key];
  return typeof value === "boolean" ? value : Boolean(field.default_value);
}

function numberValue(field: PipelineNodeCatalogField) {
  if (!props.selectedNode) {
    return typeof field.default_value === "number" ? field.default_value : 0;
  }
  const value = Number(props.selectedNode.config?.[field.key]);
  if (Number.isFinite(value)) {
    return value;
  }
  return typeof field.default_value === "number" ? field.default_value : 0;
}

function setConfigValue(key: string, value: unknown) {
  if (!props.selectedNode) {
    return;
  }
  props.selectedNode.config = {
    ...(props.selectedNode.config || {}),
    [key]: value,
  };
}

const sourceProfileId = computed(() => String(props.selectedNode?.config?.source_profile_id || ""));

const sourceSelectionMode = computed(() => String(props.selectedNode?.config?.source_selection || "enabled"));

const selectedSourceChoices = computed(() => props.sourceChoiceGroups.find((group) => group.id === sourceProfileId.value)?.sources || []);

const selectedSourceIds = computed(() => {
  if (sourceSelectionMode.value !== "custom") {
    return new Set(selectedSourceChoices.value.filter((source) => source.enabled).map((source) => source.id));
  }
  const value = props.selectedNode?.config?.source_ids;
  if (!Array.isArray(value)) {
    return new Set<string>();
  }
  return new Set(value.map((entry) => String(entry)).filter(Boolean));
});

const filteredSourceChoices = computed(() => {
  const search = sourceSearch.value.trim().toLowerCase();
  if (!search) {
    return selectedSourceChoices.value;
  }
  return selectedSourceChoices.value.filter((source) => [source.name, source.id, source.url, source.path, source.kind].join(" ").toLowerCase().includes(search));
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
  if (!props.selectedNode) {
    return;
  }
  try {
    const parsed = jsonDrafts[field.key].trim() ? JSON.parse(jsonDrafts[field.key]) : {};
    setConfigValue(field.key, parsed);
  } catch {
    // Keep draft for user correction.
  }
}
</script>

<template>
  <aside class="flex min-h-0 flex-col overflow-hidden rounded-[1.75rem] border border-slate-200 bg-white shadow-panel">
    <div class="border-b border-slate-200 px-4 py-4">
      <div class="flex items-start justify-between gap-3">
        <div>
          <p class="text-sm font-semibold text-slate-800">节点配置</p>
          <p class="mt-1 text-xs text-slate-500">
            {{ selectedNode ? "调整节点参数和起点状态。" : selectedEdge ? "设置这条连接。" : selectedNodeIds.length + selectedEdgeIds.length > 1 ? "你当前选中了多个元素。" : "查看工作流设置、布局和校验结果。" }}
          </p>
        </div>
        <button v-if="selectedNodeIds.length + selectedEdgeIds.length > 1" type="button" class="ui-button ui-button-danger !rounded-2xl !px-3" @click="emit('remove-selection')">
          <PhTrash size="16" />
        </button>
      </div>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 py-4">
      <div v-if="selectedNodeIds.length + selectedEdgeIds.length > 1" class="space-y-4">
        <div class="rounded-2xl border border-slate-200 bg-slate-50/80 px-4 py-4">
          <div class="flex items-center gap-2 text-sm font-semibold text-slate-800">
            <PhSelectionBackground size="18" class="text-primary" />
            已选 {{ selectedNodeIds.length + selectedEdgeIds.length }} 项
          </div>
          <p class="mt-2 text-sm text-slate-500">多选状态下支持统一删除。复杂参数仍需逐个节点设置。</p>
        </div>
      </div>

      <div v-else-if="selectedNode" class="space-y-4">
        <div class="rounded-2xl border border-slate-200 bg-slate-50/80 px-4 py-4">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="text-xs uppercase tracking-[0.14em] text-slate-400">{{ selectedNode.id }}</p>
              <input v-model="selectedNode.name" class="ui-field mt-2 !rounded-2xl font-semibold" />
              <p class="mt-2 text-xs text-slate-500">{{ selectedCatalogItem?.display_name || selectedNode.action }} · {{ nodeTypeLabel(selectedNode.node_type) }}</p>
            </div>
            <button type="button" class="ui-button ui-button-danger !rounded-2xl !px-3" @click="emit('remove-node', selectedNode.id)">
              <PhTrash size="16" />
            </button>
          </div>

          <div class="mt-4 flex flex-wrap gap-2">
            <button
              type="button"
              class="rounded-full px-3 py-1.5 text-xs font-semibold transition"
              :class="activeTemplate?.entry_node_id === selectedNode.id ? 'border border-blue-200 bg-white text-blue-700' : 'border border-black/10 bg-slate-100 text-slate-600 hover:border-black/20'"
              @click="emit('set-entry-node', selectedNode.id)"
            >
              {{ activeTemplate?.entry_node_id === selectedNode.id ? "当前起点" : "设为起点" }}
            </button>
            <button
              type="button"
              class="rounded-full px-3 py-1.5 text-xs font-semibold transition"
              :class="selectedNode.ui?.collapsed ? 'border border-slate-300 bg-white text-slate-900' : 'border border-black/10 bg-slate-100 text-slate-600 hover:border-black/20'"
              @click="selectedNode.ui = { ...(selectedNode.ui || {}), collapsed: !(selectedNode.ui?.collapsed === true) }"
            >
              {{ selectedNode.ui?.collapsed ? "已折叠" : "折叠节点" }}
            </button>
          </div>
        </div>

        <section v-if="selectedNode.action === 'select_sources'" class="rounded-2xl border border-slate-200 bg-white px-4 py-4">
          <p class="text-xs font-semibold uppercase tracking-[0.14em] text-slate-400">输入组</p>
          <label class="mt-3 block">
            <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">输入组档案</span>
            <select :value="sourceProfileId" class="ui-field !rounded-2xl" @change="changeSourceProfile(($event.target as HTMLSelectElement).value)">
              <option v-for="group in sourceChoiceGroups" :key="group.id || 'bound-config'" :value="group.id">{{ group.label }}</option>
            </select>
          </label>
          <div class="mt-3 flex items-center justify-between gap-2 text-xs text-slate-500">
            <span>{{ sourceSelectionMode === "custom" ? `自定义 ${selectedSourceIds.size} 个输入源` : "使用全部启用输入源" }}</span>
            <button v-if="sourceSelectionMode === 'custom'" type="button" class="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 font-semibold text-slate-600" @click="useEnabledSources">恢复全部启用</button>
          </div>
          <input v-model="sourceSearch" class="ui-field mt-3 !rounded-2xl" placeholder="筛选输入源名称、ID、URL 或路径..." />
          <div class="mt-3 space-y-2">
            <p v-if="filteredSourceChoices.length === 0" class="rounded-2xl border border-dashed border-slate-200 px-3 py-3 text-xs text-slate-500">没有匹配的输入源。</p>
            <label v-for="source in filteredSourceChoices" :key="source.id" class="flex items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-slate-50/70 px-3 py-2 text-sm text-slate-700">
              <span class="min-w-0">
                <span class="block truncate font-medium">{{ source.name }}</span>
                <span class="block truncate text-xs text-slate-400">{{ source.id }} · {{ source.enabled ? "启用" : "停用" }}{{ source.url || source.path ? ` · ${source.url || source.path}` : "" }}</span>
              </span>
              <input :checked="selectedSourceIds.has(source.id)" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" @change="toggleSourceId(source.id, ($event.target as HTMLInputElement).checked)" />
            </label>
          </div>
        </section>

        <section v-for="group in groupedFields" :key="group.name" class="rounded-2xl border border-slate-200 bg-white px-4 py-4">
          <p class="text-xs font-semibold uppercase tracking-[0.14em] text-slate-400">{{ group.name }}</p>
          <div class="mt-3 space-y-4">
            <div v-for="field in group.fields" :key="field.key">
              <label v-if="field.field_type === 'textarea'" class="block">
                <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
                <textarea :value="stringValue(field)" class="ui-field min-h-28 !rounded-2xl" :placeholder="field.placeholder || ''" :rows="field.rows || 4" @input="setConfigValue(field.key, ($event.target as HTMLTextAreaElement).value)"></textarea>
              </label>

              <label v-else-if="field.field_type === 'json'" class="block">
                <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
                <textarea
                  :value="jsonDrafts[field.key]"
                  class="mt-1 min-h-32 w-full rounded-2xl border border-slate-200 bg-slate-950 px-3 py-3 font-mono text-xs text-slate-100 outline-none focus:border-primary"
                  spellcheck="false"
                  :rows="field.rows || 6"
                  @input="updateJsonField(field, ($event.target as HTMLTextAreaElement).value)"
                  @blur="commitJsonField(field)"
                ></textarea>
              </label>

              <label v-else-if="field.field_type === 'select'" class="block">
                <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
                <select :value="stringValue(field)" class="ui-field !rounded-2xl" @change="setConfigValue(field.key, ($event.target as HTMLSelectElement).value)">
                  <option v-for="option in field.options || []" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
              </label>

              <label v-else-if="field.field_type === 'checkbox'" class="inline-flex items-center gap-3 rounded-2xl border border-slate-200 bg-slate-50/80 px-3 py-3 text-sm text-slate-700">
                <input :checked="booleanValue(field)" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" @change="setConfigValue(field.key, ($event.target as HTMLInputElement).checked)" />
                <span>{{ field.label }}</span>
              </label>

              <label v-else-if="field.field_type === 'number'" class="block">
                <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
                <input :value="numberValue(field)" type="number" class="ui-field !rounded-2xl" :min="field.min" :max="field.max" :step="field.step || 1" @input="setConfigValue(field.key, Number(($event.target as HTMLInputElement).value))" />
              </label>

              <label v-else class="block">
                <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">{{ field.label }}</span>
                <input :value="stringValue(field)" class="ui-field !rounded-2xl" :placeholder="field.placeholder || ''" @input="setConfigValue(field.key, ($event.target as HTMLInputElement).value)" />
              </label>

              <p v-if="field.help_text || field.description" class="mt-2 text-xs leading-5 text-slate-500">{{ field.help_text || field.description }}</p>
            </div>
          </div>
        </section>
      </div>

      <div v-else-if="selectedEdge" class="space-y-4">
        <div class="rounded-2xl border border-slate-200 bg-slate-50/80 px-4 py-4">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="text-xs uppercase tracking-[0.14em] text-slate-400">{{ selectedEdge.id }}</p>
              <p class="mt-2 text-sm font-semibold text-slate-800">连接</p>
            </div>
            <button type="button" class="ui-button ui-button-danger !rounded-2xl !px-3" @click="emit('remove-edge', selectedEdge.id)">
              <PhTrash size="16" />
            </button>
          </div>
        </div>

        <label class="block">
          <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">从哪个节点来</span>
          <select :value="selectedEdge.source_node_id" class="ui-field !rounded-2xl" @change="emit('edge-source-change', ($event.target as HTMLSelectElement).value)">
            <option v-for="node in sourceOptions" :key="node.id" :value="node.id">{{ node.name || node.id }}</option>
          </select>
        </label>

        <label class="block">
          <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">连接到哪个节点</span>
          <select :value="selectedEdge.target_node_id" class="ui-field !rounded-2xl" @change="emit('edge-target-change', ($event.target as HTMLSelectElement).value)">
            <option v-for="node in targetOptions" :key="node.id" :value="node.id">{{ node.name || node.id }}</option>
          </select>
        </label>

        <label class="block">
          <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">条件</span>
          <select v-if="branchOptions.length > 0" v-model="selectedEdge.outcome" class="ui-field !rounded-2xl">
            <option v-for="outcome in branchOptions" :key="outcome.value" :value="outcome.value">{{ outcome.label }} · {{ outcome.value }}</option>
          </select>
          <input v-else value="这条连线不需要条件" class="ui-field !rounded-2xl bg-slate-100 text-slate-500" readonly />
        </label>

        <label class="block">
          <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">说明</span>
          <input v-model="selectedEdge.label" class="ui-field !rounded-2xl" placeholder="例如：有结果 / 无结果" />
        </label>
      </div>

      <div v-else class="space-y-4">
        <div class="rounded-2xl border border-slate-200 bg-slate-50/80 px-4 py-4">
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-sm font-semibold text-slate-800">工作流设置</p>
              <p class="mt-1 text-xs text-slate-500">默认显示工作流基础信息、绑定配置状态和校验结果。</p>
            </div>
            <button type="button" class="ui-button ui-button-secondary !rounded-2xl !px-3" @click="emit('apply-layout')">
              <PhArrowsClockwise size="16" />
              自动排布
            </button>
          </div>
        </div>

        <label v-if="activeTemplate" class="block">
          <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">工作流名</span>
          <input v-model="activeTemplate.name" class="ui-field !rounded-2xl" />
        </label>

        <label v-if="activeTemplate" class="block">
          <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">说明</span>
          <textarea v-model="activeTemplate.description" class="ui-field min-h-28 !rounded-2xl" rows="4"></textarea>
        </label>

        <label v-if="activeTemplate" class="inline-flex items-center gap-3 rounded-2xl border border-slate-200 bg-slate-50/80 px-3 py-3 text-sm text-slate-700">
          <input v-model="activeTemplate.enabled" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
          <span>工作流启用</span>
        </label>

        <div class="rounded-2xl border border-slate-200 bg-white px-4 py-4">
          <div class="flex items-center gap-2 text-sm font-semibold text-slate-800">
            <PhGitBranch size="16" class="text-primary" />
            校验结果
          </div>
          <div v-if="issues.length === 0" class="mt-3 rounded-2xl border border-emerald-200 bg-white px-3 py-3 text-sm text-emerald-700">当前工作流结构检查通过。</div>
          <ul v-else class="mt-3 space-y-2 text-sm text-slate-600">
            <li v-for="issue in issues.slice(0, 8)" :key="issue.id" class="rounded-2xl border bg-white px-3 py-3" :class="issue.tone === 'error' ? 'border-rose-200 text-rose-700' : 'border-amber-200 text-amber-700'">
              {{ issue.message }}
            </li>
          </ul>
        </div>
      </div>
    </div>
  </aside>
</template>
