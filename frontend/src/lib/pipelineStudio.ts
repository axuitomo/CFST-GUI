import dagre from "@dagrejs/dagre";
import type {
  PipelineEdge,
  PipelineNode,
  PipelineNodeCatalogField,
  PipelineNodeCatalogItem,
  PipelineNodeRunResult,
  PipelineNodeType,
  PipelineProfileRunResult,
  PipelineRunResult,
  PipelineTarget,
  PipelineTemplate,
  PipelineViewport,
} from "./bridge";

const DEFAULT_NODE_WIDTH = 320;
const DEFAULT_NODE_HEIGHT = 140;
const COLLAPSED_NODE_HEIGHT = 92;

export interface PipelineTemplateIssue {
  edgeId?: string;
  id: string;
  message: string;
  nodeId?: string;
  tone: "error" | "warning";
}

export interface PipelineStudioOverlay {
  activeRun: PipelineRunResult | null;
  edgeIds: Set<string>;
  latestStatus: string;
  latestTargetResult: PipelineProfileRunResult | null;
  nodeMap: Map<string, PipelineNodeRunResult>;
  targetResults: PipelineProfileRunResult[];
}

function ensureNodeUI(node: PipelineNode) {
  node.ui = {
    ...(node.ui || {}),
    collapsed: node.ui?.collapsed === true,
    width: typeof node.ui?.width === "number" && Number.isFinite(node.ui.width) && node.ui.width > 200 ? node.ui.width : DEFAULT_NODE_WIDTH,
  };
  return node.ui;
}

function ensureTemplateUI(template: PipelineTemplate) {
  template.ui = {
    ...(template.ui || {}),
    viewport: normalizeViewport(template.ui?.viewport),
  };
  return template.ui;
}

function normalizeViewport(viewport?: PipelineViewport): PipelineViewport {
  return {
    x: typeof viewport?.x === "number" && Number.isFinite(viewport.x) ? viewport.x : 0,
    y: typeof viewport?.y === "number" && Number.isFinite(viewport.y) ? viewport.y : 0,
    zoom: typeof viewport?.zoom === "number" && Number.isFinite(viewport.zoom) && viewport.zoom > 0 ? viewport.zoom : 1,
  };
}

export function stablePipelineWorkspaceSignature(workspace: {
  active_target_id: string;
  active_template_id: string;
  schema_version: string;
  targets: unknown[];
  templates: unknown[];
}) {
  return JSON.stringify({
    active_target_id: workspace.active_target_id,
    active_template_id: workspace.active_template_id,
    schema_version: workspace.schema_version,
    targets: workspace.targets,
    templates: workspace.templates,
  });
}

export function nodeTypeLabel(value: PipelineNodeType) {
  const labels: Record<PipelineNodeType, string> = {
    branch: "判断",
    deliver: "投递",
    end: "结束",
    filter: "筛选",
    probe: "测速",
    recovery: "恢复",
    source: "输入源组",
  };
  return labels[value];
}

export function statusLabel(status: string) {
  const labels: Record<string, string> = {
    cancelled: "已停止",
    completed: "完成",
    dns_failed: "DNS 失败",
    failed: "失败",
    idle: "等待中",
    manual_review: "需要手动处理",
    partial: "部分完成",
    running: "运行中",
    skipped: "已跳过",
  };
  return labels[status] || status || "等待中";
}

export function statusTone(status: string) {
  if (status === "completed") {
    return "success";
  }
  if (status === "running") {
    return "running";
  }
  if (status === "manual_review" || status === "partial") {
    return "warning";
  }
  if (status === "failed" || status === "dns_failed" || status === "cancelled") {
    return "error";
  }
  return "idle";
}

export function actionLabel(action: string, catalog: PipelineNodeCatalogItem[]) {
  return catalog.find((item) => item.action === action)?.display_name || action || "未命名动作";
}

export function metricsSummary(nodeResult: PipelineNodeRunResult) {
  if (!nodeResult.metrics) {
    return "";
  }
  return Object.entries(nodeResult.metrics)
    .map(([key, value]) => `${key}: ${value}`)
    .join(" · ");
}

export function summarizeNodeConfig(node: PipelineNode, catalogItem?: PipelineNodeCatalogItem | null) {
  const fieldSchema = catalogItem?.form_schema || [];
  const flowSummaries: Record<string, string> = {
    branch_has_results: "输入:结果集 · 输出:有结果/无结果路径",
    check_output: "输入:测速/筛选结果 · 输出:CSV 检查",
    deliver_dns: "输入:结果集 · 输出:Cloudflare DNS",
    deliver_github: "输入:结果集 · 输出:GitHub CSV",
    filter_sources: "输入:输入源组 · 输出:筛选后输入源",
    filter_results: "输入:测速结果 · 输出:筛选结果",
    probe_download: "输入:追踪候选 · 输出:测速结果",
    probe_tcp: "输入:输入源 · 输出:TCP候选",
    probe_trace: "输入:TCP候选 · 输出:追踪候选",
    recovery_mark: "输入:回退原因 · 输出:人工复核状态",
    select_sources: "输入:绑定配置 · 输出:输入源组",
  };
  const priorityKeys = ["source", "top_n"];
  const prioritizedFields = [
    ...priorityKeys.flatMap((key) => fieldSchema.filter((field) => field.key === key)),
    ...fieldSchema.filter((field) => !priorityKeys.includes(field.key)),
  ];
  const summaryParts = prioritizedFields
    .slice(0, 3)
    .map((field) => {
      const value = node.config?.[field.key];
      if (value === undefined || value === null || value === "") {
        return "";
      }
      if (typeof value === "boolean") {
        return `${field.label}:${value ? "开" : "关"}`;
      }
      if (typeof value === "object") {
        return `${field.label}:已配置`;
      }
      return `${field.label}:${String(value)}`;
    })
    .filter(Boolean);
  if (summaryParts.length > 0) {
    return [summaryParts.join(" · "), flowSummaries[node.action]].filter(Boolean).join(" · ");
  }
  const fallbackEntries = Object.entries(node.config || {})
    .slice(0, 2)
    .map(([key, value]) => `${key}:${typeof value === "object" ? "已配置" : String(value)}`);
  return [fallbackEntries.join(" · "), flowSummaries[node.action]].filter(Boolean).join(" · ");
}

export function isFieldVisible(field: PipelineNodeCatalogField, config: Record<string, unknown>) {
  const condition = field.visible_when;
  if (!condition?.field) {
    return true;
  }
  const currentValue = config?.[condition.field];
  if (condition.not_equals !== undefined) {
    return currentValue !== condition.not_equals;
  }
  if (condition.equals !== undefined) {
    return currentValue === condition.equals;
  }
  return true;
}

export function getNodeById(template: PipelineTemplate, nodeId: string) {
  return template.nodes.find((node) => node.id === nodeId);
}

export function getActiveTargets(templateId: string, targets: PipelineTarget[]) {
  return targets.filter((target) => target.template_id === templateId);
}

export function createsCycle(template: PipelineTemplate, sourceId: string, targetId: string, currentEdgeId = "") {
  if (!sourceId || !targetId || sourceId === targetId) {
    return true;
  }
  const adjacency = new Map<string, string[]>();
  for (const node of template.nodes) {
    adjacency.set(node.id, []);
  }
  for (const edge of template.edges) {
    if (edge.id === currentEdgeId) {
      continue;
    }
    adjacency.set(edge.source_node_id, [...(adjacency.get(edge.source_node_id) || []), edge.target_node_id]);
  }
  const queue = [targetId];
  const visited = new Set<string>([targetId]);
  while (queue.length > 0) {
    const current = queue.shift() || "";
    if (current === sourceId) {
      return true;
    }
    for (const next of adjacency.get(current) || []) {
      if (visited.has(next)) {
        continue;
      }
      visited.add(next);
      queue.push(next);
    }
  }
  return false;
}

export function availableSourceNodes(template: PipelineTemplate) {
  return template.nodes.filter((node) => node.node_type !== "end");
}

export function availableTargetNodes(template: PipelineTemplate, edge: PipelineEdge) {
  return template.nodes.filter((node) => node.id !== edge.source_node_id && !createsCycle(template, edge.source_node_id, node.id, edge.id));
}

export function branchOutcomes(node: PipelineNode, catalog: PipelineNodeCatalogItem[]) {
  return catalog.find((item) => item.action === node.action)?.outcomes || [];
}

export function syncEdgeOutcome(template: PipelineTemplate, edge: PipelineEdge, catalog: PipelineNodeCatalogItem[]) {
  const source = getNodeById(template, edge.source_node_id);
  if (!source) {
    edge.outcome = "";
    return;
  }
  if (source.node_type !== "branch") {
    edge.outcome = "";
    return;
  }
  const outcomes = branchOutcomes(source, catalog);
  if (outcomes.length === 0) {
    return;
  }
  if (!outcomes.some((item) => item.value === edge.outcome)) {
    edge.outcome = outcomes[0].value;
  }
}

export function ensureTemplateLayout(template: PipelineTemplate) {
  ensureTemplateUI(template);
  const needsLayout = template.nodes.some((node) => !node.ui?.position);
  if (!needsLayout) {
    return;
  }
  const graph = new dagre.graphlib.Graph();
  graph.setGraph({
    marginx: 40,
    marginy: 40,
    nodesep: 36,
    rankdir: "LR",
    ranksep: 72,
  });
  graph.setDefaultEdgeLabel(() => ({}));

  for (const node of template.nodes) {
    const ui = ensureNodeUI(node);
    graph.setNode(node.id, {
      height: ui.collapsed ? COLLAPSED_NODE_HEIGHT : DEFAULT_NODE_HEIGHT,
      width: ui.width || DEFAULT_NODE_WIDTH,
    });
  }

  for (const edge of template.edges) {
    if (edge.source_node_id && edge.target_node_id) {
      graph.setEdge(edge.source_node_id, edge.target_node_id);
    }
  }

  dagre.layout(graph);

  for (const node of template.nodes) {
    const layoutNode = graph.node(node.id);
    const ui = ensureNodeUI(node);
    const width = ui.width || DEFAULT_NODE_WIDTH;
    const height = ui.collapsed ? COLLAPSED_NODE_HEIGHT : DEFAULT_NODE_HEIGHT;
    if (layoutNode) {
      ui.position = {
        x: layoutNode.x - width / 2,
        y: layoutNode.y - height / 2,
      };
    } else {
      ui.position = {
        x: 40,
        y: 40,
      };
    }
  }
}

export function buildTemplateIssues(template: PipelineTemplate, catalog: PipelineNodeCatalogItem[]): PipelineTemplateIssue[] {
  const issues: PipelineTemplateIssue[] = [];
  if (template.nodes.length === 0) {
    return [{ id: "template-empty", message: "流程里至少要有 1 个步骤。", tone: "error" }];
  }
  const catalogIndex = new Map(catalog.map((item) => [item.action, item]));
  const nodeById = new Map<string, PipelineNode>();
  const outgoing = new Map<string, PipelineEdge[]>();
  const incoming = new Map<string, number>();
  let endCount = 0;

  for (const node of template.nodes) {
    const nodeId = node.id.trim();
    if (!nodeId) {
      issues.push({ id: `node-empty-${node.name}`, message: "有步骤还没填 ID，请补全后再保存。", tone: "error" });
      continue;
    }
    if (nodeById.has(nodeId)) {
      issues.push({ id: `node-duplicate-${nodeId}`, message: `步骤 ${nodeId} 重复了，请修改后再试。`, nodeId, tone: "error" });
      continue;
    }
    nodeById.set(nodeId, node);
    outgoing.set(nodeId, []);
    incoming.set(nodeId, 0);
    if (node.node_type === "end") {
      endCount += 1;
    }
    const catalogItem = catalogIndex.get(node.action);
    if (!catalogItem) {
      issues.push({ id: `node-action-${nodeId}`, message: `步骤 ${node.name || nodeId} 使用了未识别的动作 ${node.action}。`, nodeId, tone: "error" });
    } else if (catalogItem.node_type !== node.node_type) {
      issues.push({
        id: `node-type-${nodeId}`,
        message: `步骤 ${node.name || nodeId} 的动作 ${node.action} 和步骤类型 ${node.node_type} 对不上。`,
        nodeId,
        tone: "error",
      });
    }
  }

  if (!template.entry_node_id.trim()) {
    issues.push({ id: "entry-missing", message: "请先选一个起始步骤。", tone: "error" });
  } else if (!nodeById.has(template.entry_node_id)) {
    issues.push({ id: "entry-not-found", message: `起始步骤 ${template.entry_node_id} 不存在，请重新选择。`, tone: "error" });
  }
  if (endCount === 0) {
    issues.push({ id: "end-missing", message: "流程里至少要有 1 个结束步骤。", tone: "error" });
  }

  for (const edge of template.edges) {
    const source = nodeById.get(edge.source_node_id);
    const target = nodeById.get(edge.target_node_id);
    if (!source || !target) {
      issues.push({
        edgeId: edge.id,
        id: `edge-missing-${edge.id}`,
        message: `下一步设置 ${edge.id} 的来源或去向不存在，请检查。`,
        tone: "error",
      });
      continue;
    }
    if (source.node_type === "end") {
      issues.push({
        edgeId: edge.id,
        id: `edge-source-end-${edge.id}`,
        message: `结束步骤 ${source.name || source.id} 后面不能再接下一步。`,
        nodeId: source.id,
        tone: "error",
      });
    }
    outgoing.set(source.id, [...(outgoing.get(source.id) || []), edge]);
    incoming.set(target.id, (incoming.get(target.id) || 0) + 1);
  }

  for (const node of template.nodes) {
    const edges = outgoing.get(node.id) || [];
    if (node.node_type === "branch") {
      const seen = new Set<string>();
      if (edges.length === 0) {
        issues.push({ id: `branch-empty-${node.id}`, message: `判断步骤 ${node.name || node.id} 还没有设置下一步。`, nodeId: node.id, tone: "error" });
      }
      for (const edge of edges) {
        if (!edge.outcome.trim()) {
          issues.push({
            edgeId: edge.id,
            id: `branch-outcome-empty-${edge.id}`,
            message: `判断步骤 ${node.name || node.id} 的下一步 ${edge.id} 还没选条件。`,
            nodeId: node.id,
            tone: "error",
          });
          continue;
        }
        if (seen.has(edge.outcome)) {
          issues.push({
            edgeId: edge.id,
            id: `branch-outcome-duplicate-${edge.id}`,
            message: `判断步骤 ${node.name || node.id} 的条件 ${edge.outcome} 重复了。`,
            nodeId: node.id,
            tone: "error",
          });
        }
        seen.add(edge.outcome);
        const outcomes = branchOutcomes(node, catalog);
        if (outcomes.length > 0 && !outcomes.some((item) => item.value === edge.outcome)) {
          issues.push({
            edgeId: edge.id,
            id: `branch-outcome-invalid-${edge.id}`,
            message: `判断步骤 ${node.name || node.id} 使用了不支持的条件 ${edge.outcome}。`,
            nodeId: node.id,
            tone: "error",
          });
        }
      }
      const outcomes = branchOutcomes(node, catalog);
      for (const outcome of outcomes) {
        if (!seen.has(outcome.value)) {
          issues.push({
            id: `branch-outcome-missing-${node.id}-${outcome.value}`,
            message: `判断步骤 ${node.name || node.id} 还没有配置“${outcome.label || outcome.value}”路径，空结果或异常路径可能无法明确收口。`,
            nodeId: node.id,
            tone: "warning",
          });
        }
      }
      continue;
    }
    if (node.node_type !== "end" && edges.length === 0) {
      issues.push({ id: `node-edge-missing-${node.id}`, message: `步骤 ${node.name || node.id} 还没有设置下一步。`, nodeId: node.id, tone: "error" });
    }
    if (node.node_type !== "end" && edges.length > 1) {
      issues.push({ id: `node-edge-too-many-${node.id}`, message: `步骤 ${node.name || node.id} 只能指向 1 个下一步。`, nodeId: node.id, tone: "error" });
    }
    for (const edge of edges) {
      if (edge.outcome.trim()) {
        issues.push({
          edgeId: edge.id,
          id: `edge-outcome-unexpected-${edge.id}`,
          message: `步骤 ${node.name || node.id} 不是判断步骤，不需要给下一步 ${edge.id} 设置条件。`,
          nodeId: node.id,
          tone: "error",
        });
      }
    }
  }

  if (template.entry_node_id && nodeById.has(template.entry_node_id)) {
    const reachable = new Set<string>([template.entry_node_id]);
    const queue = [template.entry_node_id];
    while (queue.length > 0) {
      const current = queue.shift() || "";
      for (const edge of outgoing.get(current) || []) {
        if (reachable.has(edge.target_node_id)) {
          continue;
        }
        reachable.add(edge.target_node_id);
        queue.push(edge.target_node_id);
      }
    }
    for (const node of template.nodes) {
      if (!reachable.has(node.id)) {
        issues.push({
          id: `node-unreachable-${node.id}`,
          message: `步骤 ${node.name || node.id} 从起始步骤走不到，请检查下一步走向。`,
          nodeId: node.id,
          tone: "warning",
        });
      }
    }
    if (!template.nodes.some((node) => node.node_type === "end" && reachable.has(node.id))) {
      issues.push({ id: "end-unreachable", message: "从起始步骤出发走不到任何结束步骤。", tone: "error" });
    }
  }

  const pending = [...incoming.entries()].filter(([, degree]) => degree === 0).map(([nodeId]) => nodeId);
  const degrees = new Map(incoming);
  let visited = 0;
  while (pending.length > 0) {
    const current = pending.shift() || "";
    visited += 1;
    for (const edge of outgoing.get(current) || []) {
      const nextDegree = (degrees.get(edge.target_node_id) || 0) - 1;
      degrees.set(edge.target_node_id, nextDegree);
      if (nextDegree === 0) {
        pending.push(edge.target_node_id);
      }
    }
  }
  if (visited !== template.nodes.length) {
    issues.push({ id: "template-cycle", message: "步骤之间出现了循环，请检查下一步走向。", tone: "error" });
  }

  return issues;
}

function resolveActiveRun(templateId: string, results: PipelineRunResult[], activePipelineId: string) {
  if (activePipelineId) {
    const matched = results.find((entry) => entry.pipeline_id === activePipelineId || entry.task_id === activePipelineId);
    if (matched) {
      return matched;
    }
  }
  return results.find((entry) => entry.template_id === templateId) || null;
}

function buildTraversedEdges(template: PipelineTemplate, nodeResults: PipelineNodeRunResult[]) {
  const traversed = new Set<string>();
  for (let index = 0; index < nodeResults.length - 1; index += 1) {
    const current = nodeResults[index];
    const next = nodeResults[index + 1];
    const matchedEdge = template.edges.find((edge) => edge.source_node_id === current.node_id && edge.target_node_id === next.node_id);
    if (matchedEdge) {
      traversed.add(matchedEdge.id);
    }
  }
  return traversed;
}

export function buildPipelineOverlay(template: PipelineTemplate, results: PipelineRunResult[], activePipelineId: string, preferredTargetId = ""): PipelineStudioOverlay {
  const activeRun = resolveActiveRun(template.id, results, activePipelineId);
  const targetResults = activeRun?.target_results || activeRun?.results || [];
  const latestTargetResult =
    targetResults.find((entry) => entry.target_id === preferredTargetId || entry.profile_id === preferredTargetId) ||
    targetResults.find((entry) => entry.status === "running") ||
    targetResults[0] ||
    null;
  const nodeResults = latestTargetResult?.node_results || [];
  return {
    activeRun,
    edgeIds: buildTraversedEdges(template, nodeResults),
    latestStatus: latestTargetResult?.status || activeRun?.status || "",
    latestTargetResult,
    nodeMap: new Map(nodeResults.map((entry) => [entry.node_id, entry])),
    targetResults,
  };
}
