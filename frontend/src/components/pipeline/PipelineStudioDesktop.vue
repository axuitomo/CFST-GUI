<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, toRef, useTemplateRef, watch } from "vue";
import { Background } from "@vue-flow/background";
import { Controls } from "@vue-flow/controls";
import { MarkerType, VueFlow, useVueFlow, type Connection, type Edge as FlowEdge, type EdgeChange, type Node as FlowNode, type NodeChange, type ViewportTransform } from "@vue-flow/core";
import { MiniMap } from "@vue-flow/minimap";
import { PhArrowsClockwise, PhClock, PhEye, PhFloppyDisk, PhGauge, PhPlay, PhPlus, PhRows, PhTrash, PhX } from "@phosphor-icons/vue";
import type { DesktopSourceConfig, PipelineEdge, PipelineNode, PipelineNodeCatalogItem, PipelineNodeCatalogOutcome, PipelineRunResult, PipelineTemplate, PipelineWorkspace, ProbeResult, SchedulerStatus, SourceProfileStore } from "../../lib/bridge";
import { usePipelineStudio } from "../../composables/usePipelineStudio";
import { actionLabel, availableSourceNodes, availableTargetNodes, branchOutcomes, createsCycle, ensureTemplateLayout, metricsSummary, nodeTypeLabel, statusLabel, statusTone, summarizeNodeConfig, syncEdgeOutcome } from "../../lib/pipelineStudio";
import PipelineStudioNode from "./PipelineStudioNode.vue";
import TaskProcessView from "../ui/TaskProcessView.vue";

interface TimestampFormatOptions {
  fallback?: string;
  includeDate?: boolean;
  includeOffset?: boolean;
  includeSeconds?: boolean;
}

type SchedulerTriggerMode = "interval" | "daily";

interface WorkflowSchedulerState {
  autoDnsPush: boolean;
  dailyTimes: string;
  enabled: boolean;
  intervalMinutes: number;
  skipIfActive: boolean;
  templateId: string;
  triggerMode: SchedulerTriggerMode;
}

interface ProcessEntry {
  detail: string;
  stage: string;
  title: string;
  tone: "success" | "error" | "running" | "info" | "warning";
  ts: string;
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

interface CreateTemplatePayload {
  preset?: "default" | "upload_recovery";
}

interface StudioFlowNodeData {
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

interface CatalogMenuState {
  open: boolean;
  x: number;
  y: number;
}

interface NodeMenuState extends CatalogMenuState {
  nodeId: string;
}

interface PreviewRow {
  address: string;
  averageSpeed: number | null;
  key: string;
  maxSpeed: number | null;
  testPort: number | null;
}

interface PanePoint {
  x: number;
  y: number;
}

type FloatingPanel = "logs" | "preview" | "process" | null;

const props = defineProps<{
  activePipelineId: string;
  canStartPipeline: boolean;
  currentResultRows: ProbeResult[];
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  fitRequestKey: number;
  loading: boolean;
  nodeCatalog: PipelineNodeCatalogItem[];
  pipelineResults: PipelineRunResult[];
  pipelineWorkspace: PipelineWorkspace;
  processTrace: ProcessEntry[];
  schedulerState: WorkflowSchedulerState;
  schedulerStatus: SchedulerStatus | null;
  sourceProfiles: SourceProfileStore;
  workspaceDirty: boolean;
}>();

const emit = defineEmits<{
  (event: "activate-template", templateId: string): void;
  (event: "clear-process"): void;
  (event: "create-template", payload?: CreateTemplatePayload): void;
  (event: "delete-template", templateId: string): void;
  (event: "open-dashboard"): void;
  (event: "save-scheduler", payload: WorkflowSchedulerState): void;
  (event: "save-workspace"): void;
  (event: "start-pipeline", templateId: string): void;
}>();

const studio = usePipelineStudio({
  activePipelineId: toRef(props, "activePipelineId"),
  nodeCatalog: toRef(props, "nodeCatalog"),
  pipelineResults: toRef(props, "pipelineResults"),
  pipelineWorkspace: toRef(props, "pipelineWorkspace"),
});

const { fitView } = useVueFlow();
const activeTemplate = studio.activeTemplate;
const catalogSearch = studio.catalogSearch;
const issues = studio.issues;
const issueEdgeIds = studio.issueEdgeIds;
const overlay = studio.overlay;
const selectedEdgeIds = studio.selectedEdgeIds;
const selectedNodeIds = studio.selectedNodeIds;

const canvasPaneRef = useTemplateRef<HTMLElement>("canvasPaneRef");
const currentViewport = ref<ViewportTransform>({ x: 0, y: 0, zoom: 1 });
const floatingPanel = ref<FloatingPanel>(null);
const schedulerPopoverOpen = ref(false);
const catalogPlacement = ref<PanePoint | null>(null);

const schedulerDraft = reactive<WorkflowSchedulerState>({
  autoDnsPush: false,
  dailyTimes: "",
  enabled: false,
  intervalMinutes: 60,
  skipIfActive: true,
  templateId: "",
  triggerMode: "interval",
});

const catalogMenu = reactive<CatalogMenuState>({
  open: false,
  x: 24,
  y: 24,
});

const nodeMenu = reactive<NodeMenuState>({
  nodeId: "",
  open: false,
  x: 24,
  y: 24,
});

const toneClass = (status: string) => {
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
};

const toneDotClass = (status: string) => {
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
};

const selectedEdge = computed(() => {
  if (!activeTemplate.value || selectedEdgeIds.value.length !== 1 || selectedNodeIds.value.length > 0) {
    return null;
  }
  return activeTemplate.value.edges.find((edge) => edge.id === selectedEdgeIds.value[0]) || null;
});

const sourceOptions = computed(() => (activeTemplate.value ? availableSourceNodes(activeTemplate.value) : []));
const targetOptions = computed(() => (activeTemplate.value && selectedEdge.value ? availableTargetNodes(activeTemplate.value, selectedEdge.value) : []));
const selectedEdgeSourceNode = computed(() => {
  if (!activeTemplate.value || !selectedEdge.value) {
    return null;
  }
  return activeTemplate.value.nodes.find((node) => node.id === selectedEdge.value?.source_node_id) || null;
});
const selectedEdgeOutcomes = computed<PipelineNodeCatalogOutcome[]>(() => {
  if (!selectedEdgeSourceNode.value) {
    return [];
  }
  return branchOutcomes(selectedEdgeSourceNode.value, props.nodeCatalog);
});

const selectedCount = computed(() => selectedNodeIds.value.length + selectedEdgeIds.value.length);
const hasBoundConfig = computed(() => Object.keys(activeTemplate.value?.bound_config_snapshot || {}).length > 0);
const builtInTemplateIds = new Set(["pipeline-template-default", "pipeline-template-advanced-upload"]);
const isBuiltInActiveTemplate = computed(() => builtInTemplateIds.has(activeTemplate.value?.id || ""));
const canLaunchActiveTemplate = computed(() => Boolean(activeTemplate.value) && props.canStartPipeline && issues.value.length === 0 && hasBoundConfig.value);
const activeRunLabel = computed(() => {
  const activeRun = overlay.value.activeRun;
  if (!activeRun) {
    return "还没有运行记录";
  }
  return `${statusLabel(overlay.value.latestStatus || activeRun.status)} · ${props.formatTimestamp(activeRun.started_at, { fallback: "-" })}`;
});
const templateStatus = computed(() => {
  if (issues.value.some((issue) => issue.tone === "error")) {
    return "需要修正";
  }
  if (issues.value.length > 0) {
    return "有提醒";
  }
  if (!hasBoundConfig.value) {
    return "未绑定配置";
  }
  if (props.workspaceDirty) {
    return "有未保存改动";
  }
  return "已就绪";
});
const latestPreviewRows = computed<PreviewRow[]>(() => {
  const pipelineRows = probePreviewRows(overlay.value.latestTargetResult?.probe_result?.results);
  if (pipelineRows.length > 0) {
    return pipelineRows;
  }
  return props.currentResultRows.slice(0, 12).map((row, index) => ({
    address: row.address || "-",
    averageSpeed: row.download_mbps ?? null,
    key: `${row.address || "row"}-${row.test_port || "port"}-${index}`,
    maxSpeed: row.max_download_mbps ?? row.download_mbps ?? null,
    testPort: row.test_port ?? null,
  }));
});
const latestTargetLabel = computed(() => overlay.value.latestTargetResult?.target_name || overlay.value.latestTargetResult?.profile_name || "当前绑定配置");
const floatingPanelTitle = computed(() => {
  if (floatingPanel.value === "preview") {
    return "数据预览";
  }
  if (floatingPanel.value === "process") {
    return "当前测试进程";
  }
  return "运行日志";
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

function sourceChoicesForNode(node: PipelineNode) {
  const profileId = String(node.config?.source_profile_id || "");
  return sourceChoiceGroups.value.find((group) => group.id === profileId)?.sources || [];
}

const nodeTypes = {
  studio: PipelineStudioNode,
};

function canonicalNodeId(flowNodeId: string) {
  return flowNodeId;
}

function nodeByFlowId(template: PipelineTemplate, flowNodeId: string) {
  const id = canonicalNodeId(flowNodeId);
  return template.nodes.find((node) => node.id === id) || null;
}

function displayPositionForNode(node: PipelineNode) {
  const position = node.ui?.position || { x: 40, y: 40 };
  return {
    x: position.x,
    y: position.y,
  };
}

function canonicalPositionFromFlowPosition(template: PipelineTemplate, flowNodeId: string, position: { x: number; y: number }) {
  const node = nodeByFlowId(template, flowNodeId);
  if (!node) {
    return position;
  }
  return {
    x: position.x,
    y: position.y,
  };
}

const flowNodes = computed<FlowNode<StudioFlowNodeData>[]>(() => {
  const template = activeTemplate.value;
  if (!template) {
    return [];
  }
  return template.nodes.map((node) => {
    const catalogItem = props.nodeCatalog.find((item) => item.action === node.action) || null;
    const nodeResult = overlay.value.nodeMap.get(node.id) || null;
    const issueMessages = issues.value.filter((issue) => issue.nodeId === node.id).map((issue) => issue.message);
    const metricText = nodeResult ? metricsSummary(nodeResult) : "";
    return {
      id: node.id,
      label: node.name || node.id,
      position: displayPositionForNode(node),
      selected: selectedNodeIds.value.includes(node.id),
      style: {
        width: `${node.ui?.width || 320}px`,
      },
      type: "studio",
      data: {
        actionLabel: actionLabel(node.action, props.nodeCatalog),
        canonicalNodeId: node.id,
        catalogItem,
        configSummary: summarizeNodeConfig(node, catalogItem),
        isEntry: template.entry_node_id === node.id,
        issues: issueMessages,
        message: [nodeResult?.message || "", metricText].filter(Boolean).join(" · "),
        nodeRef: node,
        nodeType: node.node_type,
        nodeTypeLabel: nodeTypeLabel(node.node_type),
        sourceChoices: sourceChoicesForNode(node),
        sourceChoiceGroups: sourceChoiceGroups.value,
        status: nodeResult?.status || "",
      },
    };
  });
});

const flowEdges = computed<FlowEdge[]>(() => {
  const template = activeTemplate.value;
  if (!template) {
    return [];
  }
  const savedEdges = template.edges.map((edge) => {
    const source = template.nodes.find((node) => node.id === edge.source_node_id) || null;
    const outcomeLabel = source ? branchOutcomes(source, props.nodeCatalog).find((item) => item.value === edge.outcome)?.label || "" : "";
    const highlighted = overlay.value.edgeIds.has(edge.id);
    const hasIssue = issueEdgeIds.value.has(edge.id);
    const label = edge.label.trim() || outcomeLabel || "";
    return {
      id: edge.id,
      label,
      markerEnd: MarkerType.ArrowClosed,
      selected: selectedEdgeIds.value.includes(edge.id),
      source: edge.source_node_id,
      style: {
        stroke: hasIssue ? "#e11d48" : highlighted ? "#2563eb" : "#9ca3af",
        strokeWidth: highlighted ? 3 : hasIssue ? 2.5 : 2,
      },
      target: edge.target_node_id,
      animated: highlighted && overlay.value.latestStatus === "running",
      labelBgStyle: {
        fill: "#ffffff",
      },
      labelStyle: {
        fill: hasIssue ? "#be123c" : highlighted ? "#1d4ed8" : "#475569",
        fontSize: "12px",
        fontWeight: 600,
      },
    };
  });
  return savedEdges;
});

const catalogGroups = computed(() => {
  const search = catalogSearch.value.trim().toLowerCase();
  const matchItem = (item: PipelineNodeCatalogItem) => {
    if (!search) {
      return true;
    }
    const haystack = [item.display_name, item.description || "", item.action, item.node_type].join(" ").toLowerCase();
    return haystack.includes(search);
  };
  const groups = [
    {
      description: "勾选当前绑定配置里的输入源，组成后续测速输入。",
      items: props.nodeCatalog.filter((item) => item.node_type === "source" && matchItem(item)),
      key: "source",
      label: "输入源组",
    },
    {
      description: "执行 TCP 延迟测速、追踪测试和下载测速。",
      items: props.nodeCatalog.filter((item) => item.node_type === "probe" && matchItem(item)),
      key: "probe",
      label: "测速",
    },
    {
      description: "检查 CSV 写入、导出、推送或其他可继续的交付处理。",
      items: props.nodeCatalog.filter((item) => item.node_type === "deliver" && matchItem(item)).sort((a, b) => (a.action === "check_output" ? -1 : 0) - (b.action === "check_output" ? -1 : 0)),
      key: "deliver",
      label: "结果检查与输出",
    },
    {
      description: "对测速结果继续缩小范围。",
      items: props.nodeCatalog.filter((item) => item.node_type === "filter" && matchItem(item)),
      key: "filter",
      label: "筛选",
    },
    {
      description: "按条件分支走不同后续路径。",
      items: props.nodeCatalog.filter((item) => item.node_type === "branch" && matchItem(item)),
      key: "branch",
      label: "判断",
    },
    {
      description: "记录异常、人工复核或回退原因。",
      items: props.nodeCatalog.filter((item) => item.node_type === "recovery" && matchItem(item)),
      key: "recovery",
      label: "异常处理",
    },
    {
      description: "声明当前路径的最终状态，结束后不再接下一步。",
      items: props.nodeCatalog.filter((item) => item.node_type === "end" && matchItem(item)),
      key: "end",
      label: "结束",
    },
  ];
  return groups.filter((group) => group.items.length > 0);
});

watch(
  () => props.schedulerState,
  (value) => {
    schedulerDraft.autoDnsPush = value.autoDnsPush;
    schedulerDraft.dailyTimes = value.dailyTimes;
    schedulerDraft.enabled = value.enabled;
    schedulerDraft.intervalMinutes = value.intervalMinutes;
    schedulerDraft.skipIfActive = value.skipIfActive;
    schedulerDraft.templateId = value.templateId;
    schedulerDraft.triggerMode = value.triggerMode;
  },
  { deep: true, immediate: true },
);

watch(
  () => schedulerDraft.triggerMode,
  (mode) => {
    if (mode === "interval") {
      schedulerDraft.dailyTimes = "";
      if (!Number.isFinite(schedulerDraft.intervalMinutes) || schedulerDraft.intervalMinutes <= 0) {
        schedulerDraft.intervalMinutes = 60;
      }
      return;
    }
    schedulerDraft.intervalMinutes = 0;
  },
);

watch(
  () => activeTemplate.value?.id,
  () => {
    studio.clearSelection();
    closeCatalogMenu();
    closeNodeMenu();
    currentViewport.value = activeTemplate.value?.ui?.viewport || { x: 0, y: 0, zoom: 1 };
    schedulerDraft.templateId = activeTemplate.value?.id || props.schedulerState.templateId;
  },
  { immediate: true },
);

function clampOverlayPoint(point: PanePoint, width: number, height: number) {
  const host = canvasPaneRef.value;
  if (!host) {
    return point;
  }
  const rect = host.getBoundingClientRect();
  return {
    x: Math.max(12, Math.min(point.x, Math.max(12, rect.width - width - 12))),
    y: Math.max(12, Math.min(point.y, Math.max(12, rect.height - height - 12))),
  };
}

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
  return rows.slice(0, 12).map((entry, index) => {
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
  const lastNode = [...template.nodes].reverse().find((node) => node.ui?.position);
  if (!lastNode?.ui?.position) {
    return { x: 80, y: 80 };
  }
  return {
    x: lastNode.ui.position.x + 48,
    y: lastNode.ui.position.y + 48,
  };
}

function toFlowPosition(point: PanePoint) {
  const viewport = currentViewport.value;
  const zoom = viewport.zoom || 1;
  return {
    x: (point.x - viewport.x) / zoom,
    y: (point.y - viewport.y) / zoom,
  };
}

function openCatalogAt(clientX: number, clientY: number) {
  const host = canvasPaneRef.value;
  if (!host) {
    return;
  }
  const rect = host.getBoundingClientRect();
  const localPoint = {
    x: clientX - rect.left,
    y: clientY - rect.top,
  };
  const position = clampOverlayPoint(localPoint, 360, 440);
  catalogMenu.open = true;
  catalogMenu.x = position.x;
  catalogMenu.y = position.y;
  catalogPlacement.value = toFlowPosition(localPoint);
  nodeMenu.open = false;
}

function closeCatalogMenu() {
  catalogMenu.open = false;
  catalogPlacement.value = null;
}

function openNodeMenuAt(nodeId: string, clientX: number, clientY: number) {
  const host = canvasPaneRef.value;
  if (!host) {
    return;
  }
  const rect = host.getBoundingClientRect();
  const position = clampOverlayPoint(
    {
      x: clientX - rect.left,
      y: clientY - rect.top,
    },
    200,
    188,
  );
  nodeMenu.nodeId = nodeId;
  nodeMenu.open = true;
  nodeMenu.x = position.x;
  nodeMenu.y = position.y;
  closeCatalogMenu();
}

function closeNodeMenu() {
  nodeMenu.nodeId = "";
  nodeMenu.open = false;
}

function refreshSelection() {
  const template = activeTemplate.value;
  if (!template) {
    studio.clearSelection();
    return;
  }
  studio.setSelectedNodes(selectedNodeIds.value.filter((nodeId) => template.nodes.some((node) => node.id === nodeId)));
  studio.setSelectedEdges(selectedEdgeIds.value.filter((edgeId) => template.edges.some((edge) => edge.id === edgeId)));
}

function setNodeSelection(nodeId: string, selected: boolean) {
  nodeId = canonicalNodeId(nodeId);
  const next = new Set(selectedNodeIds.value);
  if (selected) {
    next.add(nodeId);
  } else {
    next.delete(nodeId);
  }
  studio.setSelectedNodes([...next]);
}

function setEdgeSelection(edgeId: string, selected: boolean) {
  const next = new Set(selectedEdgeIds.value);
  if (selected) {
    next.add(edgeId);
  } else {
    next.delete(edgeId);
  }
  studio.setSelectedEdges([...next]);
}

function pickBranchOutcome(template: PipelineTemplate, edge: PipelineEdge) {
  const source = template.nodes.find((node) => node.id === edge.source_node_id);
  if (!source || source.node_type !== "branch") {
    edge.outcome = "";
    return;
  }
  const outcomes = branchOutcomes(source, props.nodeCatalog);
  if (outcomes.length === 0) {
    edge.outcome = "";
    return;
  }
  const used = new Set(
    template.edges
      .filter((item) => item.id !== edge.id && item.source_node_id === edge.source_node_id)
      .map((item) => item.outcome)
      .filter(Boolean),
  );
  const match = outcomes.find((item) => !used.has(item.value)) || outcomes.find((item) => item.value === edge.outcome) || outcomes[0];
  edge.outcome = match.value;
  if (!edge.label.trim() || outcomes.some((item) => item.label === edge.label.trim())) {
    edge.label = match.label;
  }
}

function addNodeFromCatalog(item: PipelineNodeCatalogItem, position?: PanePoint | null) {
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
      position: position ? { x: position.x, y: position.y } : nextNodePosition(template),
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
  nodeId = canonicalNodeId(nodeId);
  template.nodes = template.nodes.filter((node) => node.id !== nodeId);
  template.edges = template.edges.filter((edge) => edge.source_node_id !== nodeId && edge.target_node_id !== nodeId);
  if (template.entry_node_id === nodeId) {
    template.entry_node_id = template.nodes[0]?.id || "";
  }
  refreshSelection();
  if (nodeMenu.nodeId === nodeId) {
    closeNodeMenu();
  }
}

function removeEdge(edgeId: string) {
  const template = activeTemplate.value;
  if (!template) {
    return;
  }
  template.edges = template.edges.filter((edge) => edge.id !== edgeId);
  refreshSelection();
}

function removeSelection() {
  const template = activeTemplate.value;
  if (!template) {
    return;
  }
  const nodeIds = new Set(selectedNodeIds.value.map(canonicalNodeId));
  const edgeIds = new Set(selectedEdgeIds.value);
  template.edges = template.edges.filter((edge) => !edgeIds.has(edge.id) && !nodeIds.has(edge.source_node_id) && !nodeIds.has(edge.target_node_id));
  template.nodes = template.nodes.filter((node) => !nodeIds.has(node.id));
  if (nodeIds.has(template.entry_node_id)) {
    template.entry_node_id = template.nodes[0]?.id || "";
  }
  studio.clearSelection();
  closeNodeMenu();
}

function copySelection() {
  const template = activeTemplate.value;
  if (!template || selectedNodeIds.value.length === 0) {
    return;
  }
  studio.lastCopiedNodeIds.value = [...selectedNodeIds.value];
}

function pasteSelection() {
  const template = activeTemplate.value;
  const copiedIds = studio.lastCopiedNodeIds.value;
  if (!template || copiedIds.length === 0) {
    return;
  }
  const sourceNodes = template.nodes.filter((node) => copiedIds.includes(node.id));
  if (sourceNodes.length === 0) {
    return;
  }
  const now = new Date().toISOString();
  const idMap = new Map<string, string>();
  const clones = sourceNodes.map((node) => {
    const nextId = makeLocalId("node");
    idMap.set(node.id, nextId);
    return {
      ...cloneValue(node),
      id: nextId,
      name: node.name ? `${node.name} 副本` : nextId,
      ui: {
        ...(cloneValue(node.ui || {}) || {}),
        collapsed: node.ui?.collapsed === true,
        position: {
          x: (node.ui?.position?.x || 80) + 48,
          y: (node.ui?.position?.y || 80) + 48,
        },
        width: node.ui?.width || 320,
      },
      updated_at: now,
    } satisfies PipelineNode;
  });
  const copiedEdges = template.edges
    .filter((edge) => copiedIds.includes(edge.source_node_id) && copiedIds.includes(edge.target_node_id))
    .map((edge) => ({
      ...cloneValue(edge),
      id: makeLocalId("edge"),
      source_node_id: idMap.get(edge.source_node_id) || edge.source_node_id,
      target_node_id: idMap.get(edge.target_node_id) || edge.target_node_id,
    }));
  template.nodes.push(...clones);
  template.edges.push(...copiedEdges);
  studio.setSelectedNodes(clones.map((node) => node.id));
  studio.setSelectedEdges([]);
}

function onNodesChange(changes: NodeChange[]) {
  const template = activeTemplate.value;
  if (!template) {
    return;
  }
  for (const change of changes) {
    if (change.type === "position") {
      const node = nodeByFlowId(template, change.id);
      if (node && change.position) {
        const position = canonicalPositionFromFlowPosition(template, change.id, change.position);
        node.ui = {
          ...(node.ui || {}),
          collapsed: node.ui?.collapsed === true,
          position: {
            x: position.x,
            y: position.y,
          },
          width: node.ui?.width || 320,
        };
      }
    } else if (change.type === "remove") {
      removeNode(change.id);
    } else if (change.type === "select") {
      setNodeSelection(change.id, change.selected);
    }
  }
}

function onEdgesChange(changes: EdgeChange[]) {
  for (const change of changes) {
    if (change.type === "remove") {
      removeEdge(change.id);
    } else if (change.type === "select") {
      setEdgeSelection(change.id, change.selected);
    }
  }
}

function onConnect(connection: Connection) {
  const template = activeTemplate.value;
  if (!template || !connection.source || !connection.target) {
    return;
  }
  const sourceNodeId = canonicalNodeId(connection.source);
  const targetNodeId = canonicalNodeId(connection.target);
  if (sourceNodeId === targetNodeId) {
    return;
  }
  if (createsCycle(template, sourceNodeId, targetNodeId)) {
    window.alert("节点之间不能形成循环，请换一个连接。");
    return;
  }
  const source = template.nodes.find((node) => node.id === sourceNodeId) || null;
  if (!source || source.node_type === "end") {
    return;
  }
  if (template.edges.some((edge) => edge.source_node_id === sourceNodeId && edge.target_node_id === targetNodeId)) {
    return;
  }
  if (source.node_type !== "branch") {
    template.edges = template.edges.filter((edge) => edge.source_node_id !== source.id);
  }
  const edge: PipelineEdge = {
    id: makeLocalId("edge"),
    label: "",
    outcome: "",
    source_node_id: sourceNodeId,
    target_node_id: targetNodeId,
  };
  syncEdgeOutcome(template, edge, props.nodeCatalog);
  pickBranchOutcome(template, edge);
  template.edges.push(edge);
  studio.setSelectedNodes([]);
  studio.setSelectedEdges([edge.id]);
}

function updateEdgeSource(sourceNodeId: string) {
  const template = activeTemplate.value;
  if (!template || !selectedEdge.value || !sourceNodeId || sourceNodeId === selectedEdge.value.target_node_id) {
    return;
  }
  const source = template.nodes.find((node) => node.id === sourceNodeId);
  if (!source || source.node_type === "end" || createsCycle(template, sourceNodeId, selectedEdge.value.target_node_id, selectedEdge.value.id)) {
    return;
  }
  selectedEdge.value.source_node_id = sourceNodeId;
  syncEdgeOutcome(template, selectedEdge.value, props.nodeCatalog);
  pickBranchOutcome(template, selectedEdge.value);
}

function updateEdgeTarget(targetNodeId: string) {
  const template = activeTemplate.value;
  if (!template || !selectedEdge.value || !targetNodeId || targetNodeId === selectedEdge.value.source_node_id) {
    return;
  }
  if (createsCycle(template, selectedEdge.value.source_node_id, targetNodeId, selectedEdge.value.id)) {
    return;
  }
  selectedEdge.value.target_node_id = targetNodeId;
}

function updateEdgeOutcome(outcome: string) {
  if (!selectedEdge.value) {
    return;
  }
  selectedEdge.value.outcome = outcome;
  const match = selectedEdgeOutcomes.value.find((item) => item.value === outcome);
  if (match) {
    selectedEdge.value.label = match.label;
  }
}

function applyLayout() {
  const template = activeTemplate.value;
  if (!template) {
    return;
  }
  for (const node of template.nodes) {
    node.ui = {
      ...(node.ui || {}),
      collapsed: node.ui?.collapsed === true,
      position: undefined,
      width: node.ui?.width || 320,
    };
  }
  ensureTemplateLayout(template);
}

function fitActiveTemplateToView() {
  if (!activeTemplate.value) {
    return;
  }
  applyLayout();
  void nextTick(() => {
    window.requestAnimationFrame(() => {
      void fitView({ duration: 250, padding: 0.18 });
    });
  });
}

function onViewportChange(viewport: ViewportTransform) {
  currentViewport.value = {
    x: viewport.x,
    y: viewport.y,
    zoom: viewport.zoom,
  };
}

function onViewportChangeEnd(viewport: ViewportTransform) {
  currentViewport.value = {
    x: viewport.x,
    y: viewport.y,
    zoom: viewport.zoom,
  };
  const template = activeTemplate.value;
  if (!template) {
    return;
  }
  template.ui = {
    ...(template.ui || {}),
    viewport: {
      x: viewport.x,
      y: viewport.y,
      zoom: viewport.zoom,
    },
  };
}

watch(
  () => props.fitRequestKey,
  (value, previous) => {
    if (value > 0 && value !== previous) {
      fitActiveTemplateToView();
    }
  },
);

function launchActiveTemplate() {
  if (activeTemplate.value) {
    emit("start-pipeline", activeTemplate.value.id);
  }
}

function setEntryNode(nodeId: string) {
  if (activeTemplate.value) {
    activeTemplate.value.entry_node_id = canonicalNodeId(nodeId);
  }
}

function toggleNodeCollapsed(nodeId: string) {
  const template = activeTemplate.value;
  nodeId = canonicalNodeId(nodeId);
  const node = template?.nodes.find((item) => item.id === nodeId);
  if (!node) {
    return;
  }
  node.ui = {
    ...(node.ui || {}),
    collapsed: !(node.ui?.collapsed === true),
    position: node.ui?.position,
    width: node.ui?.width || 320,
  };
}

function deleteActiveTemplate() {
  if (activeTemplate.value && !isBuiltInActiveTemplate.value) {
    emit("delete-template", activeTemplate.value.id);
  }
}

function toggleFloatingPanel(panel: Exclude<FloatingPanel, null>) {
  floatingPanel.value = floatingPanel.value === panel ? null : panel;
}

function openCatalogFromToolbar() {
  const host = canvasPaneRef.value;
  if (!host) {
    return;
  }
  const rect = host.getBoundingClientRect();
  openCatalogAt(rect.left + Math.min(180, rect.width * 0.25), rect.top + 84);
}

function addCatalogItem(item: PipelineNodeCatalogItem) {
  addNodeFromCatalog(item, catalogPlacement.value);
  closeCatalogMenu();
}

function saveSchedulerShortcut() {
  if (!activeTemplate.value) {
    return;
  }
  emit("save-scheduler", {
    autoDnsPush: schedulerDraft.autoDnsPush,
    dailyTimes: schedulerDraft.dailyTimes.trim(),
    enabled: schedulerDraft.enabled,
    intervalMinutes: schedulerDraft.triggerMode === "interval" ? Math.max(1, Math.round(Number(schedulerDraft.intervalMinutes) || 1)) : 0,
    skipIfActive: schedulerDraft.skipIfActive,
    templateId: activeTemplate.value.id,
    triggerMode: schedulerDraft.triggerMode,
  });
  schedulerPopoverOpen.value = false;
}

function handleNodeContextMenu(payload: { event: MouseEvent; nodeId: string }) {
  const nodeId = canonicalNodeId(payload.nodeId);
  studio.setSelectedNodes([nodeId]);
  studio.setSelectedEdges([]);
  openNodeMenuAt(nodeId, payload.event.clientX, payload.event.clientY);
}

function handleCanvasContextMenu(event: MouseEvent) {
  const target = event.target as HTMLElement | null;
  if (target?.closest("[data-workflow-popup='true']") || target?.closest(".vue-flow__node") || target?.closest(".vue-flow__controls") || target?.closest(".vue-flow__minimap")) {
    return;
  }
  event.preventDefault();
  studio.clearSelection();
  openCatalogAt(event.clientX, event.clientY);
}

function handlePaneClick() {
  studio.clearSelection();
  closeCatalogMenu();
  closeNodeMenu();
}

function onWindowPointerDown(event: PointerEvent) {
  const target = event.target as HTMLElement | null;
  if (target?.closest("[data-workflow-popup='true']")) {
    return;
  }
  closeCatalogMenu();
  closeNodeMenu();
  schedulerPopoverOpen.value = false;
}

function onKeydown(event: KeyboardEvent) {
  const target = event.target as HTMLElement | null;
  const tagName = target?.tagName || "";
  if (target?.isContentEditable || tagName === "INPUT" || tagName === "TEXTAREA" || tagName === "SELECT") {
    return;
  }
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "c") {
    if (selectedNodeIds.value.length > 0) {
      event.preventDefault();
      copySelection();
    }
    return;
  }
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "v") {
    if (studio.lastCopiedNodeIds.value.length > 0) {
      event.preventDefault();
      pasteSelection();
    }
    return;
  }
  if (event.key === "Escape") {
    closeCatalogMenu();
    closeNodeMenu();
    schedulerPopoverOpen.value = false;
    return;
  }
  if ((event.key === "Delete" || event.key === "Backspace") && selectedCount.value > 0) {
    event.preventDefault();
    removeSelection();
  }
}

onMounted(() => {
  window.addEventListener("keydown", onKeydown);
  window.addEventListener("pointerdown", onWindowPointerDown);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onKeydown);
  window.removeEventListener("pointerdown", onWindowPointerDown);
});
</script>

<template>
  <section class="workflow-workbench flex h-full min-h-[calc(100vh-3.5rem)] flex-col">
    <header class="workflow-toolbar flex shrink-0 flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
      <div class="flex min-w-0 flex-wrap items-center gap-2 sm:gap-3">
        <select :value="pipelineWorkspace.active_template_id" class="workflow-select min-w-[14rem] max-w-full sm:w-60" @change="emit('activate-template', selectValue($event))">
          <option v-for="template in pipelineWorkspace.templates" :key="template.id" :value="template.id">{{ template.name || template.id }}</option>
        </select>
        <span class="workflow-chip gap-1.5" :class="toneClass(overlay.latestStatus || '')">
          <span class="h-1.5 w-1.5 rounded-full" :class="toneDotClass(overlay.latestStatus || '')" />
          {{ activeRunLabel }}
        </span>
        <span class="workflow-chip">{{ templateStatus }}</span>
        <span v-if="workspaceDirty" class="workflow-chip workflow-chip-warning">未保存</span>
        <span v-if="issues.length > 0" class="workflow-chip workflow-chip-danger">{{ issues.length }} 条校验</span>
        <span class="workflow-chip">{{ hasBoundConfig ? "已绑定配置" : "待绑定配置" }}</span>
      </div>

      <div class="flex min-w-0 flex-wrap items-center justify-end gap-2">
        <button type="button" class="workflow-button" :disabled="loading" @click="emit('create-template')">
          <PhPlus size="16" />
          新建工作流
        </button>
        <button type="button" class="workflow-button" :disabled="loading" @click="emit('create-template', { preset: 'upload_recovery' })">
          <PhPlus size="16" />
          高级上传模板
        </button>
        <button type="button" class="workflow-button" :disabled="loading || !activeTemplate" @click="openCatalogFromToolbar">
          <PhPlus size="16" />
          添加节点
        </button>
        <button type="button" class="workflow-button workflow-button-danger" :disabled="loading || !activeTemplate || isBuiltInActiveTemplate" @click="deleteActiveTemplate">
          <PhTrash size="16" />
          删除
        </button>
        <button type="button" class="workflow-button" :disabled="loading" @click="emit('save-workspace')">
          <PhFloppyDisk size="16" />
          保存
        </button>
        <div class="relative" data-workflow-popup="true">
          <button type="button" class="workflow-button" :disabled="loading || !activeTemplate" @click="schedulerPopoverOpen = !schedulerPopoverOpen">
            <PhClock size="16" />
            定时任务
          </button>
          <div v-if="schedulerPopoverOpen" class="workflow-popover absolute right-0 top-[calc(100%+0.5rem)] z-40 w-[22rem] rounded-[1.5rem] border p-4 shadow-[0_18px_48px_rgba(15,23,42,0.16)]">
            <div class="flex items-start justify-between gap-3">
              <div>
                <p class="text-sm font-semibold text-slate-900">工作流定时任务</p>
                <p class="mt-1 text-xs leading-5 text-slate-500">当前只作用于这个工作流绑定配置，不再额外选目标。</p>
              </div>
              <button type="button" class="inline-flex h-8 w-8 items-center justify-center rounded-full border border-black/10 text-slate-500 transition hover:border-black/20 hover:text-slate-900" @click="schedulerPopoverOpen = false">
                <PhX size="16" />
              </button>
            </div>

            <div class="mt-4 space-y-3">
              <label class="workflow-surface-soft flex items-center justify-between gap-3 rounded-2xl border px-3 py-3 text-sm">
                <span>启用定时任务</span>
                <input v-model="schedulerDraft.enabled" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              </label>

              <label class="block">
                <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">触发方式</span>
                <select v-model="schedulerDraft.triggerMode" class="ui-field !rounded-2xl">
                  <option value="interval">固定间隔</option>
                  <option value="daily">每日固定时间</option>
                </select>
              </label>

              <label v-if="schedulerDraft.triggerMode === 'interval'" class="block">
                <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">间隔分钟</span>
                <input v-model.number="schedulerDraft.intervalMinutes" type="number" min="1" class="ui-field !rounded-2xl" />
              </label>

              <label v-else class="block">
                <span class="ui-label !mb-2 !text-[11px] !tracking-[0.12em]">每日时间</span>
                <input v-model="schedulerDraft.dailyTimes" class="ui-field !rounded-2xl" placeholder="例如 09:00,21:30" />
              </label>

              <label class="workflow-surface-soft flex items-center justify-between gap-3 rounded-2xl border px-3 py-3 text-sm">
                <span>自动 DNS 推送</span>
                <input v-model="schedulerDraft.autoDnsPush" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              </label>

              <label class="workflow-surface-soft flex items-center justify-between gap-3 rounded-2xl border px-3 py-3 text-sm">
                <span>跳过活跃任务</span>
                <input v-model="schedulerDraft.skipIfActive" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
              </label>
            </div>

            <div class="workflow-surface-soft mt-4 rounded-2xl border px-3 py-3 text-xs">
              <p>最近执行：{{ formatTimestamp(schedulerStatus?.last_run_at || "", { fallback: "-" }) }}</p>
              <p class="mt-1">下次执行：{{ formatTimestamp(schedulerStatus?.next_run_at || "", { fallback: "-" }) }}</p>
              <p class="mt-1 truncate">状态：{{ schedulerStatus?.last_message || "尚未运行" }}</p>
            </div>

            <div class="mt-4 flex items-center justify-end gap-2">
              <button type="button" class="workflow-button workflow-button-compact" @click="schedulerPopoverOpen = false">取消</button>
              <button type="button" class="workflow-button workflow-button-primary workflow-button-compact" @click="saveSchedulerShortcut">保存定时</button>
            </div>
          </div>
        </div>
        <button type="button" class="workflow-button" @click="toggleFloatingPanel('process')">
          <PhGauge size="16" />
          任务看板
        </button>
        <button type="button" class="workflow-button workflow-button-primary" :disabled="!canLaunchActiveTemplate" @click="launchActiveTemplate">
          <PhPlay size="16" weight="fill" />
          运行
        </button>
      </div>
    </header>

    <div class="min-h-0 flex-1 p-3">
      <article class="workflow-panel flex h-full min-h-0 min-w-0 flex-col overflow-hidden">
        <div class="workflow-panel-head flex h-11 shrink-0 flex-wrap items-center justify-between gap-2 border-b px-3">
          <div class="flex flex-wrap items-center gap-2 text-xs font-semibold">
            <span>画布优先</span>
            <span>右键空白处添加节点</span>
            <span>节点内直接展开配置</span>
            <span>Shift 框选</span>
            <span>Ctrl/Cmd 复制粘贴</span>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <span v-if="selectedCount > 0" class="workflow-chip">{{ selectedCount }} 项已选</span>
            <button type="button" class="workflow-button workflow-button-compact" :disabled="!activeTemplate" @click="applyLayout">
              <PhArrowsClockwise size="14" />
              自动排布
            </button>
            <button v-if="selectedCount > 1" type="button" class="workflow-button workflow-button-danger workflow-button-compact" @click="removeSelection">
              <PhTrash size="14" />
              删除选中
            </button>
          </div>
        </div>

        <div ref="canvasPaneRef" class="pipeline-studio-flow workflow-canvas relative min-h-0 flex-1" @contextmenu="handleCanvasContextMenu">
          <VueFlow
            v-if="activeTemplate"
            :key="activeTemplate.id"
            :apply-default="false"
            :default-edge-options="{ markerEnd: MarkerType.ArrowClosed }"
            :default-viewport="activeTemplate.ui?.viewport || { x: 0, y: 0, zoom: 1 }"
            :delete-key-code="null"
            :edges="flowEdges"
            :elements-selectable="true"
            :max-zoom="1.8"
            :min-zoom="0.2"
            :multi-selection-key-code="'Shift'"
            :node-types="nodeTypes"
            :nodes="flowNodes"
            :pan-on-drag="true"
            :selection-key-code="'Shift'"
            :select-nodes-on-drag="false"
            :snap-grid="[24, 24]"
            :snap-to-grid="true"
            class="h-full w-full"
            @connect="onConnect"
            @edges-change="onEdgesChange"
            @nodes-change="onNodesChange"
            @pane-click="handlePaneClick"
            @viewport-change="onViewportChange"
            @viewport-change-end="onViewportChangeEnd"
          >
            <Background :gap="24" pattern-color="rgba(17, 24, 39, 0.12)" />
            <MiniMap pannable zoomable />
            <Controls position="bottom-right" />

            <template #node-studio="nodeProps">
              <PipelineStudioNode v-bind="nodeProps" @delete-node="removeNode" @node-contextmenu="handleNodeContextMenu" @set-entry="setEntryNode" @toggle-collapse="toggleNodeCollapsed" />
            </template>
          </VueFlow>

          <div v-else class="workflow-empty flex h-full items-center justify-center px-8 text-center text-sm">先新建一个工作流，再在画布里右键添加节点。</div>

          <div v-if="selectedEdge && activeTemplate" class="workflow-popover absolute left-1/2 top-4 z-20 flex w-[min(56rem,calc(100%-2rem))] -translate-x-1/2 flex-wrap items-center gap-2 rounded-[1.4rem] border px-3 py-3 shadow-[0_18px_48px_rgba(15,23,42,0.12)]" data-workflow-popup="true">
            <span class="text-xs font-semibold uppercase tracking-[0.14em] text-slate-400">边设置</span>
            <label class="min-w-[11rem] flex-1">
              <span class="sr-only">来源节点</span>
              <select :value="selectedEdge.source_node_id" class="ui-field !rounded-2xl" @change="updateEdgeSource(selectValue($event))">
                <option v-for="node in sourceOptions" :key="node.id" :value="node.id">{{ node.name || node.id }}</option>
              </select>
            </label>
            <label class="min-w-[11rem] flex-1">
              <span class="sr-only">目标节点</span>
              <select :value="selectedEdge.target_node_id" class="ui-field !rounded-2xl" @change="updateEdgeTarget(selectValue($event))">
                <option v-for="node in targetOptions" :key="node.id" :value="node.id">{{ node.name || node.id }}</option>
              </select>
            </label>
            <label class="min-w-[9rem] flex-1">
              <span class="sr-only">分支结果</span>
              <select :value="selectedEdge.outcome" class="ui-field !rounded-2xl" :disabled="selectedEdgeOutcomes.length === 0" @change="updateEdgeOutcome(selectValue($event))">
                <option value="">{{ selectedEdgeOutcomes.length === 0 ? "默认单路输出" : "选择分支结果" }}</option>
                <option v-for="option in selectedEdgeOutcomes" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </label>
            <button type="button" class="workflow-button workflow-button-danger workflow-button-compact" @click="removeEdge(selectedEdge.id)">
              <PhTrash size="14" />
              删除连线
            </button>
          </div>

          <div v-if="catalogMenu.open" class="workflow-popover absolute z-30 w-[22rem] max-w-[calc(100%-1.5rem)] rounded-[1.6rem] border p-4 shadow-[0_24px_64px_rgba(15,23,42,0.18)]" :style="{ left: `${catalogMenu.x}px`, top: `${catalogMenu.y}px` }" data-workflow-popup="true">
            <div class="flex items-start justify-between gap-3">
              <div>
                <p class="text-sm font-semibold">添加节点</p>
                <p class="mt-1 text-xs leading-5">先按模糊类别选，再决定具体节点。</p>
              </div>
              <button type="button" class="workflow-icon-button inline-flex h-8 w-8 items-center justify-center rounded-full border transition" @click="closeCatalogMenu">
                <PhX size="16" />
              </button>
            </div>

            <input v-model="catalogSearch" class="ui-field mt-4 !rounded-2xl" placeholder="搜索节点名、说明或动作..." />

            <div v-if="!activeTemplate" class="workflow-node-empty mt-4 rounded-2xl border border-dashed px-4 py-4 text-sm">先创建工作流模板，才能把节点加入画布。</div>

            <div v-else class="mt-4 max-h-[22rem] space-y-3 overflow-y-auto pr-1">
              <div v-if="catalogGroups.length === 0" class="workflow-node-empty rounded-2xl border border-dashed px-4 py-4 text-sm">没有匹配的节点，换个关键词试试。</div>
              <section v-for="group in catalogGroups" :key="group.key" class="workflow-node-group rounded-[1.35rem] border px-3 py-3">
                <div class="mb-3">
                  <p class="text-sm font-semibold">{{ group.label }}</p>
                  <p class="mt-1 text-xs leading-5">{{ group.description }}</p>
                </div>
                <div class="space-y-2">
                  <button v-for="item in group.items" :key="item.action" type="button" class="workflow-catalog-item block w-full rounded-2xl border border-transparent px-3 py-3 text-left transition" @click="addCatalogItem(item)">
                    <div class="flex items-start justify-between gap-3">
                      <div>
                        <p class="text-sm font-semibold">{{ item.display_name }}</p>
                        <p class="mt-1 text-xs leading-5">{{ item.description || item.action }}</p>
                      </div>
                      <span class="workflow-catalog-chip rounded-full border px-2 py-0.5 text-[11px] font-semibold">{{ nodeTypeLabel(item.node_type) }}</span>
                    </div>
                  </button>
                </div>
              </section>
            </div>
          </div>

          <div v-if="nodeMenu.open" class="workflow-popover absolute z-30 w-48 rounded-[1.35rem] border p-2 shadow-[0_20px_48px_rgba(15,23,42,0.18)]" :style="{ left: `${nodeMenu.x}px`, top: `${nodeMenu.y}px` }" data-workflow-popup="true">
            <button
              type="button"
              class="workflow-menu-item flex w-full items-center justify-between rounded-xl px-3 py-2 text-sm transition"
              @click="
                setEntryNode(nodeMenu.nodeId);
                closeNodeMenu();
              "
            >
              设为起点
            </button>
            <button
              type="button"
              class="workflow-menu-item mt-1 flex w-full items-center justify-between rounded-xl px-3 py-2 text-sm transition"
              @click="
                toggleNodeCollapsed(nodeMenu.nodeId);
                closeNodeMenu();
              "
            >
              {{ activeTemplate?.nodes.find((node) => node.id === nodeMenu.nodeId)?.ui?.collapsed ? "展开节点" : "折叠节点" }}
            </button>
            <button type="button" class="workflow-menu-item workflow-menu-item-danger mt-1 flex w-full items-center justify-between rounded-xl px-3 py-2 text-sm transition" @click="removeNode(nodeMenu.nodeId)">删除节点</button>
          </div>

          <div class="pointer-events-none absolute bottom-4 left-4 z-20 flex max-w-[calc(100%-2rem)] items-end gap-3">
            <div v-if="floatingPanel" class="workflow-popover pointer-events-auto w-[min(28rem,calc(100vw-8rem))] rounded-[1.6rem] border shadow-[0_20px_54px_rgba(15,23,42,0.16)]" data-workflow-popup="true">
              <div class="workflow-popover-head flex items-center justify-between gap-3 border-b px-4 py-3">
                <div>
                  <p class="text-sm font-semibold">{{ floatingPanelTitle }}</p>
                  <p class="mt-1 text-xs">{{ latestTargetLabel }}</p>
                </div>
                <button type="button" class="workflow-icon-button inline-flex h-8 w-8 items-center justify-center rounded-full border transition" @click="floatingPanel = null">
                  <PhX size="16" />
                </button>
              </div>

              <div v-if="floatingPanel === 'process'" class="px-3 py-3">
                <TaskProcessView :entries="processTrace" :format-timestamp="formatTimestamp" empty-text="当前没有测试进程。" title="当前测试进程" @clear="emit('clear-process')" />
                <button type="button" class="workflow-button workflow-button-compact mt-3 w-full justify-center" @click="emit('open-dashboard')">打开完整任务看板</button>
              </div>

              <div v-else-if="floatingPanel === 'logs'" class="max-h-[24rem] overflow-y-auto px-4 py-4">
                <div v-if="processTrace.length === 0" class="workflow-empty flex min-h-[12rem] items-center justify-center text-sm">暂无运行日志。</div>
                <div v-else class="space-y-2">
                  <div v-for="(entry, index) in processTrace" :key="`${entry.ts}-${entry.stage}-${index}`" class="workflow-log-row rounded-2xl border px-3 py-3">
                    <div class="flex items-center justify-between gap-3">
                      <p class="truncate text-sm font-semibold">{{ entry.title }}</p>
                      <span class="workflow-chip gap-1.5 !px-2 !py-0.5" :class="toneClass(entry.tone === 'running' ? 'running' : entry.tone === 'success' ? 'completed' : entry.tone === 'error' ? 'failed' : '')">
                        <span class="h-1.5 w-1.5 rounded-full" :class="toneDotClass(entry.tone === 'running' ? 'running' : entry.tone === 'success' ? 'completed' : entry.tone === 'error' ? 'failed' : '')" />
                        {{ formatTimestamp(entry.ts, { fallback: "-", includeSeconds: true }) }}
                      </span>
                    </div>
                    <p class="mt-2 text-xs leading-5">{{ entry.detail || "-" }}</p>
                  </div>
                </div>
              </div>

              <div v-else class="max-h-[24rem] overflow-auto">
                <table class="min-w-full text-left text-xs">
                  <thead class="workflow-table-head sticky top-0">
                    <tr>
                      <th class="px-4 py-3 font-semibold">IP地址</th>
                      <th class="px-4 py-3 font-semibold">测速端口</th>
                      <th class="px-4 py-3 font-semibold">平均速率</th>
                      <th class="px-4 py-3 font-semibold">最高速率</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-if="latestPreviewRows.length === 0">
                      <td colspan="4" class="workflow-empty px-4 py-10 text-center text-sm">暂无运行数据。</td>
                    </tr>
                    <tr v-for="row in latestPreviewRows" :key="row.key" class="workflow-table-row border-t">
                      <td class="max-w-[12rem] truncate px-4 py-3 font-mono text-[11px]" :title="row.address">{{ row.address }}</td>
                      <td class="px-4 py-3 font-mono">{{ formatPort(row.testPort) }}</td>
                      <td class="px-4 py-3">{{ formatSpeed(row.averageSpeed) }}</td>
                      <td class="px-4 py-3">{{ formatSpeed(row.maxSpeed) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <div class="pointer-events-auto flex flex-col gap-2" data-workflow-popup="true">
              <button type="button" class="workflow-float-button inline-flex items-center gap-2 rounded-full border px-3 py-2 text-sm font-semibold shadow-[0_12px_28px_rgba(15,23,42,0.1)] transition" @click="toggleFloatingPanel('logs')">
                <PhRows size="15" />
                运行日志
              </button>
              <button type="button" class="workflow-float-button inline-flex items-center gap-2 rounded-full border px-3 py-2 text-sm font-semibold shadow-[0_12px_28px_rgba(15,23,42,0.1)] transition" @click="toggleFloatingPanel('preview')">
                <PhEye size="15" />
                数据预览
              </button>
            </div>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>
