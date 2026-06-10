<script setup lang="ts">
import { computed, ref, toRef, watch } from "vue";
import { PhFloppyDisk, PhGauge, PhPlay, PhPlus, PhRows, PhTrash } from "@phosphor-icons/vue";
import type { DesktopSourceConfig, PipelineNode, PipelineNodeCatalogItem, PipelineRunResult, PipelineTemplate, PipelineWorkspace, ProbeResult, SourceProfileStore } from "../../lib/bridge";
import { usePipelineStudio } from "../../composables/usePipelineStudio";
import { actionLabel, availableSourceNodes, availableTargetNodes, branchOutcomes, createsCycle, metricsSummary, nodeTypeLabel, statusLabel, statusTone, summarizeNodeConfig, syncEdgeOutcome } from "../../lib/pipelineStudio";
import PipelineStudioCatalog from "./PipelineStudioCatalog.vue";
import PipelineStudioInspector from "./PipelineStudioInspector.vue";

interface TimestampFormatOptions {
  fallback?: string;
  includeDate?: boolean;
  includeOffset?: boolean;
  includeSeconds?: boolean;
}

interface ProcessEntry {
  detail: string;
  stage: string;
  title: string;
  tone: "success" | "error" | "running" | "info" | "warning";
  ts: string;
}

interface PreviewRow {
  address: string;
  averageSpeed: number | null;
  key: string;
  maxSpeed: number | null;
  testPort: number | null;
}

interface CreateTemplatePayload {
  preset?: "default" | "upload_recovery";
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

const props = defineProps<{
  activePipelineId: string;
  canStartPipeline: boolean;
  currentResultRows: ProbeResult[];
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  loading: boolean;
  nodeCatalog: PipelineNodeCatalogItem[];
  pipelineResults: PipelineRunResult[];
  pipelineWorkspace: PipelineWorkspace;
  processTrace: ProcessEntry[];
  sourceProfiles: SourceProfileStore;
  workspaceDirty: boolean;
}>();

const emit = defineEmits<{
  (event: "activate-template", templateId: string): void;
  (event: "create-template", payload?: CreateTemplatePayload): void;
  (event: "delete-template", templateId: string): void;
  (event: "open-dashboard"): void;
  (event: "save-workspace"): void;
  (event: "start-pipeline", templateId: string): void;
}>();

const studio = usePipelineStudio({
  activePipelineId: toRef(props, "activePipelineId"),
  nodeCatalog: toRef(props, "nodeCatalog"),
  pipelineResults: toRef(props, "pipelineResults"),
  pipelineWorkspace: toRef(props, "pipelineWorkspace"),
});

const activeTemplate = studio.activeTemplate;
const catalogSearch = studio.catalogSearch;
const issues = studio.issues;
const overlay = studio.overlay;
const selectedEdgeIds = studio.selectedEdgeIds;
const selectedNodeIds = studio.selectedNodeIds;
const showRunLogs = ref(false);

const selectedNode = computed(() => {
  if (!activeTemplate.value || selectedNodeIds.value.length !== 1 || selectedEdgeIds.value.length > 0) {
    return null;
  }
  return activeTemplate.value.nodes.find((node) => node.id === selectedNodeIds.value[0]) || null;
});

const selectedEdge = computed(() => {
  if (!activeTemplate.value || selectedEdgeIds.value.length !== 1 || selectedNodeIds.value.length > 0) {
    return null;
  }
  return activeTemplate.value.edges.find((edge) => edge.id === selectedEdgeIds.value[0]) || null;
});

const sourceOptions = computed(() => (activeTemplate.value ? availableSourceNodes(activeTemplate.value) : []));
const targetOptions = computed(() => (activeTemplate.value && selectedEdge.value ? availableTargetNodes(activeTemplate.value, selectedEdge.value) : []));
const hasBoundConfig = computed(() => Object.keys(activeTemplate.value?.bound_config_snapshot || {}).length > 0);
const builtInTemplateIds = new Set(["pipeline-template-default", "pipeline-template-advanced-upload"]);
const isBuiltInActiveTemplate = computed(() => builtInTemplateIds.has(activeTemplate.value?.id || ""));
const canLaunchActiveTemplate = computed(() => Boolean(activeTemplate.value) && props.canStartPipeline && hasBoundConfig.value && issues.value.length === 0);
const latestPreviewRows = computed<PreviewRow[]>(() => {
  const pipelineRows = probePreviewRows(overlay.value.latestTargetResult?.probe_result?.results);
  if (pipelineRows.length > 0) {
    return pipelineRows;
  }
  return props.currentResultRows.slice(0, 6).map((row, index) => ({
    address: row.address || "-",
    averageSpeed: row.download_mbps ?? null,
    key: `${row.address || "row"}-${row.test_port || "port"}-${index}`,
    maxSpeed: row.max_download_mbps ?? row.download_mbps ?? null,
    testPort: row.test_port ?? null,
  }));
});

function sourceChoicesFromSources(rawSources: unknown): SourceChoice[] {
  if (!Array.isArray(rawSources)) {
    return [];
  }
  return rawSources
    .map((source, index) => {
      const item = (source || {}) as Partial<DesktopSourceConfig>;
      return {
        enabled: item.enabled !== false,
        id: String(item.id || `source-${index + 1}`),
        kind: String(item.kind || ""),
        name: String(item.name || `输入源 ${index + 1}`),
        path: String(item.path || ""),
        url: String(item.url || ""),
      };
    })
    .filter((source) => source.id.trim());
}

const sourceChoiceGroups = computed<SourceChoiceGroup[]>(() => {
  const groups: SourceChoiceGroup[] = [
    {
      id: "",
      label: "当前绑定配置",
      sources: sourceChoicesFromSources(activeTemplate.value?.bound_config_snapshot?.sources || []),
    },
  ];
  for (const profile of props.sourceProfiles.items || []) {
    groups.push({
      id: profile.id,
      label: profile.name || profile.id,
      sources: sourceChoicesFromSources(profile.sources as DesktopSourceConfig[]),
    });
  }
  return groups;
});

watch(
  () => activeTemplate.value?.id,
  () => studio.clearSelection(),
);

function makeLocalId(prefix: string) {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function cloneValue<T>(value: T): T {
  if (value === null || value === undefined) {
    return value;
  }
  return JSON.parse(JSON.stringify(value)) as T;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function optionalNumber(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  const parsed = Number.parseFloat(String(value));
  return Number.isFinite(parsed) ? parsed : null;
}

function optionalInteger(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  const parsed = Number.parseInt(String(value), 10);
  return Number.isFinite(parsed) ? parsed : null;
}

function selectValue(event: Event) {
  return (event.currentTarget as HTMLSelectElement).value;
}

function probePreviewRows(rows: unknown): PreviewRow[] {
  if (!Array.isArray(rows)) {
    return [];
  }
  return rows.slice(0, 6).map((entry, index) => {
    const row = isRecord(entry) ? entry : {};
    const averageSpeed = optionalNumber(row.download_mbps ?? row.downloadSpeedMb);
    const maxSpeed = optionalNumber(row.max_download_mbps ?? row.max_download_speed_mb ?? row.maxDownloadSpeedMb ?? row.maxDownloadMbps) ?? averageSpeed;
    const testPort = optionalInteger(row.test_port ?? row.testPort);
    const address = String(row.address ?? row.ip ?? "").trim();
    return {
      address: address || "-",
      averageSpeed: averageSpeed !== null && averageSpeed >= 0 ? averageSpeed : null,
      key: `${address || "row"}-${testPort || "port"}-${index}`,
      maxSpeed: maxSpeed !== null && maxSpeed >= 0 ? maxSpeed : null,
      testPort: testPort !== null && testPort > 0 ? testPort : null,
    };
  });
}

function formatPort(value: number | null) {
  return value && value > 0 ? String(value) : "-";
}

function formatSpeed(value: number | null) {
  return value !== null && Number.isFinite(value) ? `${value.toFixed(2)} MB/s` : "-";
}

function nextNodePosition(template: PipelineTemplate) {
  const index = template.nodes.length;
  return {
    x: 40 + (index % 2) * 20,
    y: 40 + index * 36,
  };
}

function addNodeFromCatalog(item: PipelineNodeCatalogItem) {
  const template = activeTemplate.value;
  if (!template) {
    return;
  }
  const now = new Date().toISOString();
  const node: PipelineNode = {
    action: item.action,
    config: cloneValue(item.default_config || {}),
    id: makeLocalId("node"),
    name: item.display_name || `节点 ${template.nodes.length + 1}`,
    node_type: item.node_type,
    ui: {
      collapsed: false,
      position: nextNodePosition(template),
      width: 320,
    },
    updated_at: now,
  };
  template.nodes.push(node);
  if (!template.entry_node_id) {
    template.entry_node_id = node.id;
  }
  studio.setSelectedNodes([node.id]);
  studio.setSelectedEdges([]);
}

function removeNode(nodeId: string) {
  const template = activeTemplate.value;
  if (!template) {
    return;
  }
  template.nodes = template.nodes.filter((node) => node.id !== nodeId);
  template.edges = template.edges.filter((edge) => edge.source_node_id !== nodeId && edge.target_node_id !== nodeId);
  if (template.entry_node_id === nodeId) {
    template.entry_node_id = template.nodes[0]?.id || "";
  }
  studio.clearSelection();
}

function removeEdge(edgeId: string) {
  const template = activeTemplate.value;
  if (!template) {
    return;
  }
  template.edges = template.edges.filter((edge) => edge.id !== edgeId);
  studio.clearSelection();
}

function removeSelection() {
  const template = activeTemplate.value;
  if (!template) {
    return;
  }
  const nodeIds = new Set(selectedNodeIds.value);
  const edgeIds = new Set(selectedEdgeIds.value);
  template.edges = template.edges.filter((edge) => !edgeIds.has(edge.id) && !nodeIds.has(edge.source_node_id) && !nodeIds.has(edge.target_node_id));
  template.nodes = template.nodes.filter((node) => !nodeIds.has(node.id));
  if (nodeIds.has(template.entry_node_id)) {
    template.entry_node_id = template.nodes[0]?.id || "";
  }
  studio.clearSelection();
}

function updateEdgeSource(sourceNodeId: string) {
  const template = activeTemplate.value;
  if (!template || !selectedEdge.value || !sourceNodeId || sourceNodeId === selectedEdge.value.target_node_id) {
    return;
  }
  const edge = selectedEdge.value;
  const source = template.nodes.find((node) => node.id === sourceNodeId);
  if (!source || source.node_type === "end" || createsCycle(template, sourceNodeId, edge.target_node_id, edge.id)) {
    return;
  }
  edge.source_node_id = sourceNodeId;
  syncEdgeOutcome(template, edge, props.nodeCatalog);
  if (source.node_type === "branch") {
    const outcome = branchOutcomes(source, props.nodeCatalog).find((item) => item.value === edge.outcome) || branchOutcomes(source, props.nodeCatalog)[0];
    if (outcome) {
      edge.outcome = outcome.value;
      if (!edge.label.trim()) {
        edge.label = outcome.label;
      }
    }
  }
}

function updateEdgeTarget(targetNodeId: string) {
  const template = activeTemplate.value;
  if (!template || !selectedEdge.value || !targetNodeId || targetNodeId === selectedEdge.value.source_node_id) {
    return;
  }
  const edge = selectedEdge.value;
  if (createsCycle(template, edge.source_node_id, targetNodeId, edge.id)) {
    return;
  }
  edge.target_node_id = targetNodeId;
}

function launchActiveTemplate() {
  if (!activeTemplate.value) {
    return;
  }
  emit("start-pipeline", activeTemplate.value.id);
}

function toneClass(status: string) {
  const tone = statusTone(status);
  if (tone === "success") {
    return "border-emerald-200 bg-[rgb(255,255,255)] text-emerald-700";
  }
  if (tone === "error") {
    return "border-rose-200 bg-[rgb(255,255,255)] text-rose-700";
  }
  if (tone === "warning") {
    return "border-amber-200 bg-[rgb(255,255,255)] text-amber-700";
  }
  if (tone === "running") {
    return "border-blue-200 bg-[rgb(255,255,255)] text-blue-700";
  }
  return "border-black/10 bg-[rgb(251,251,251)] text-slate-600";
}

function toneDotClass(status: string) {
  const tone = statusTone(status);
  if (tone === "success") {
    return "bg-emerald-500";
  }
  if (tone === "error") {
    return "bg-rose-500";
  }
  if (tone === "warning") {
    return "bg-amber-500";
  }
  if (tone === "running") {
    return "bg-blue-500";
  }
  return "bg-slate-400";
}

function nodeRunText(nodeId: string) {
  const result = overlay.value.nodeMap.get(nodeId);
  return [result?.message || "", result ? metricsSummary(result) : ""].filter(Boolean).join(" · ");
}

function setEntryNode(nodeId: string) {
  if (activeTemplate.value) {
    activeTemplate.value.entry_node_id = nodeId;
  }
}

function deleteActiveTemplate() {
  if (activeTemplate.value && !isBuiltInActiveTemplate.value) {
    emit("delete-template", activeTemplate.value.id);
  }
}

function noop() {
  return;
}
</script>

<template>
  <section class="workflow-mobile workflow-mode space-y-4">
    <article class="rounded-lg border border-black/10 bg-[rgb(255,255,255)] px-5 py-4">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p class="text-sm font-semibold text-slate-800">工作流模式</p>
          <p class="mt-1 text-xs text-slate-500">移动端保留简化编辑：看节点、改参数、查看绑定配置状态、发起运行。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="ui-button ui-button-secondary !rounded-2xl" :disabled="loading" @click="emit('create-template')">
            <PhPlus size="16" />
            新建工作流
          </button>
          <button type="button" class="ui-button ui-button-secondary !rounded-2xl" :disabled="loading" @click="emit('create-template', { preset: 'upload_recovery' })">
            <PhPlus size="16" />
            高级上传模板
          </button>
          <button type="button" class="ui-button ui-button-secondary !rounded-2xl" :disabled="loading" @click="emit('save-workspace')">
            <PhFloppyDisk size="16" />
            保存
          </button>
          <button type="button" class="ui-button ui-button-ghost !rounded-2xl" @click="emit('open-dashboard')">
            <PhGauge size="16" />
            任务看板
          </button>
          <button type="button" class="ui-button ui-button-secondary !rounded-2xl" @click="showRunLogs = !showRunLogs">
            <PhRows size="16" />
            运行日志
          </button>
          <button type="button" class="ui-button ui-button-primary !rounded-2xl" :disabled="!canLaunchActiveTemplate" @click="launchActiveTemplate">
            <PhPlay size="16" weight="fill" />
            运行
          </button>
        </div>
      </div>

      <div class="mt-4 grid gap-3">
        <select :value="pipelineWorkspace.active_template_id" class="ui-field !rounded-2xl" @change="emit('activate-template', selectValue($event))">
          <option v-for="template in pipelineWorkspace.templates" :key="template.id" :value="template.id">{{ template.name || template.id }}</option>
        </select>

        <div class="flex flex-wrap gap-2 text-xs">
          <span class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 font-semibold" :class="toneClass(overlay.latestStatus || '')">
            <span class="h-1.5 w-1.5 rounded-full" :class="toneDotClass(overlay.latestStatus || '')" />
            {{ overlay.activeRun ? `${statusLabel(overlay.latestStatus || overlay.activeRun.status)} · ${formatTimestamp(overlay.activeRun.started_at, { fallback: "-" })}` : "还没有运行记录" }}
          </span>
          <span class="rounded-full border px-3 py-1 font-semibold" :class="hasBoundConfig ? 'border-emerald-200 bg-[rgb(255,255,255)] text-emerald-700' : 'border-slate-200 bg-[rgb(255,255,255)] text-slate-500'">
            {{ hasBoundConfig ? "已绑定配置" : "未绑定配置" }}
          </span>
          <span v-if="workspaceDirty" class="rounded-full border border-amber-200 bg-[rgb(255,255,255)] px-3 py-1 font-semibold text-amber-700">有未保存改动</span>
          <span v-if="issues.length > 0" class="rounded-full border border-rose-200 bg-[rgb(255,255,255)] px-3 py-1 font-semibold text-rose-700">{{ issues.length }} 条校验提醒</span>
        </div>

        <div class="flex flex-wrap gap-2">
          <button type="button" class="ui-button ui-button-danger !rounded-2xl" :disabled="loading || !activeTemplate || isBuiltInActiveTemplate" @click="deleteActiveTemplate">
            <PhTrash size="16" />
            删除工作流
          </button>
        </div>
      </div>
    </article>

    <article v-if="showRunLogs" class="rounded-lg border border-black/10 bg-[rgb(255,255,255)] px-4 py-4">
      <div class="flex items-center justify-between gap-3">
        <div>
          <p class="text-sm font-semibold text-slate-800">运行日志</p>
          <p class="mt-1 text-xs text-slate-500">{{ overlay.activeRun ? formatTimestamp(overlay.activeRun.started_at, { fallback: "-" }) : "尚无运行记录" }}</p>
        </div>
        <span class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-semibold" :class="toneClass(overlay.latestStatus || '')">
          <span class="h-1.5 w-1.5 rounded-full" :class="toneDotClass(overlay.latestStatus || '')" />
          {{ statusLabel(overlay.latestStatus || "idle") }}
        </span>
      </div>
      <div v-if="processTrace.length === 0" class="mt-4 rounded-2xl border border-slate-200 bg-slate-50/80 px-4 py-6 text-center text-sm text-slate-400">暂无运行日志。</div>
      <div v-else class="mt-4 space-y-2">
        <div v-for="(entry, index) in processTrace" :key="`${entry.ts}-${entry.stage}-${index}`" class="rounded-2xl border border-slate-200 bg-slate-50/80 px-3 py-3">
          <div class="flex items-start justify-between gap-3">
            <p class="min-w-0 truncate text-sm font-semibold text-slate-800">{{ entry.title }}</p>
            <span class="inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2 py-1 text-[11px] font-semibold" :class="toneClass(entry.tone === 'running' ? 'running' : entry.tone === 'success' ? 'completed' : entry.tone === 'error' ? 'failed' : '')">
              <span class="h-1.5 w-1.5 rounded-full" :class="toneDotClass(entry.tone === 'running' ? 'running' : entry.tone === 'success' ? 'completed' : entry.tone === 'error' ? 'failed' : '')" />
              {{ formatTimestamp(entry.ts, { fallback: "-", includeSeconds: true }) }}
            </span>
          </div>
          <p class="mt-2 text-xs leading-5 text-slate-500">{{ entry.detail || "-" }}</p>
        </div>
      </div>
    </article>

    <article class="rounded-lg border border-black/10 bg-[rgb(255,255,255)] px-4 py-4">
      <div class="flex items-center justify-between gap-3">
        <div>
          <p class="text-sm font-semibold text-slate-800">数据预览</p>
          <p class="mt-1 text-xs text-slate-500">当前结果简略版</p>
        </div>
      </div>
      <div v-if="latestPreviewRows.length === 0" class="mt-4 rounded-2xl border border-slate-200 bg-slate-50/80 px-4 py-6 text-center text-sm text-slate-400">暂无运行数据。</div>
      <div v-else class="mt-4 grid gap-2">
        <div v-for="row in latestPreviewRows" :key="row.key" class="rounded-2xl border border-slate-200 bg-slate-50/80 px-3 py-3 text-xs">
          <p class="truncate font-mono text-sm font-semibold text-slate-800" :title="row.address">{{ row.address }}</p>
          <div class="mt-3 grid grid-cols-3 gap-2">
            <div>
              <p class="text-slate-400">测速端口</p>
              <p class="mt-1 font-mono font-semibold text-slate-800">{{ formatPort(row.testPort) }}</p>
            </div>
            <div>
              <p class="text-slate-400">平均速率</p>
              <p class="mt-1 font-semibold text-slate-800">{{ formatSpeed(row.averageSpeed) }}</p>
            </div>
            <div>
              <p class="text-slate-400">最高速率</p>
              <p class="mt-1 font-semibold text-slate-800">{{ formatSpeed(row.maxSpeed) }}</p>
            </div>
          </div>
        </div>
      </div>
    </article>

    <article class="rounded-lg border border-black/10 bg-[rgb(255,255,255)] px-4 py-4">
      <div class="flex items-center justify-between gap-3">
        <div>
          <p class="text-sm font-semibold text-slate-800">节点列表</p>
          <p class="mt-1 text-xs text-slate-500">点一下节点或连接，就能在下面改参数。</p>
        </div>
        <button type="button" class="ui-button ui-button-secondary !rounded-2xl !px-3" @click="studio.clearSelection">清空选择</button>
      </div>

      <div v-if="activeTemplate" class="mt-4 space-y-3">
        <button
          v-for="node in activeTemplate.nodes"
          :key="node.id"
          type="button"
          class="w-full rounded-2xl border px-4 py-3 text-left transition"
          :class="selectedNodeIds.includes(node.id) ? 'border-primary bg-primary/5' : 'border-slate-200 bg-slate-50/80'"
          @click="
            studio.setSelectedNodes([node.id]);
            studio.setSelectedEdges([]);
          "
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="text-sm font-semibold text-slate-800">{{ node.name || node.id }}</p>
              <p class="mt-1 text-xs text-slate-500">{{ actionLabel(node.action, nodeCatalog) }} · {{ nodeTypeLabel(node.node_type) }}</p>
              <p class="mt-2 text-xs text-slate-500">{{ summarizeNodeConfig(node, nodeCatalog.find((item) => item.action === node.action) || null) || "这一步还没有额外参数。" }}</p>
            </div>
            <span class="inline-flex items-center gap-1.5 rounded-full border px-2 py-1 text-[11px] font-semibold" :class="toneClass(overlay.nodeMap.get(node.id)?.status || '')">
              <span class="h-1.5 w-1.5 rounded-full" :class="toneDotClass(overlay.nodeMap.get(node.id)?.status || '')" />
              {{ statusLabel(overlay.nodeMap.get(node.id)?.status || "idle") }}
            </span>
          </div>
          <p v-if="nodeRunText(node.id)" class="mt-2 text-xs text-slate-500">{{ nodeRunText(node.id) }}</p>
        </button>
      </div>
    </article>

    <article v-if="activeTemplate" class="rounded-lg border border-black/10 bg-[rgb(255,255,255)] px-4 py-4">
      <p class="text-sm font-semibold text-slate-800">连接</p>
      <div class="mt-4 space-y-3">
        <button
          v-for="edge in activeTemplate.edges"
          :key="edge.id"
          type="button"
          class="w-full rounded-2xl border px-4 py-3 text-left transition"
          :class="selectedEdgeIds.includes(edge.id) ? 'border-primary bg-primary/5' : 'border-slate-200 bg-slate-50/80'"
          @click="
            studio.setSelectedEdges([edge.id]);
            studio.setSelectedNodes([]);
          "
        >
          <p class="text-sm font-semibold text-slate-800">{{ edge.label || edge.outcome || edge.id }}</p>
          <p class="mt-1 text-xs text-slate-500">{{ edge.source_node_id }} → {{ edge.target_node_id }}</p>
        </button>
      </div>
    </article>

    <PipelineStudioInspector
      :active-template="activeTemplate"
      :issues="issues"
      :node-catalog="nodeCatalog"
      :selected-edge="selectedEdge"
      :selected-edge-ids="selectedEdgeIds"
      :selected-node="selectedNode"
      :selected-node-ids="selectedNodeIds"
      :source-choice-groups="sourceChoiceGroups"
      :source-options="sourceOptions"
      :target-options="targetOptions"
      @apply-layout="noop"
      @edge-source-change="updateEdgeSource"
      @edge-target-change="updateEdgeTarget"
      @remove-edge="removeEdge"
      @remove-node="removeNode"
      @remove-selection="removeSelection"
      @set-entry-node="setEntryNode"
    />

    <PipelineStudioCatalog v-model:search="catalogSearch" :items="nodeCatalog" @add="addNodeFromCatalog" />
  </section>
</template>
