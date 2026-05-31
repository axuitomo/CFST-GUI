import { computed, ref, watch, type Ref } from "vue";
import { normalizeConfigSnapshot, type PipelineEdge, type PipelineNodeCatalogItem, type PipelineRunResult, type PipelineTemplate, type PipelineWorkspace } from "../lib/bridge";
import { buildPipelineOverlay, buildTemplateIssues, ensureTemplateLayout, getActiveTargets, stablePipelineWorkspaceSignature } from "../lib/pipelineStudio";

interface UsePipelineStudioOptions {
  activePipelineId: Ref<string>;
  nodeCatalog: Ref<PipelineNodeCatalogItem[]>;
  pipelineResults: Ref<PipelineRunResult[]>;
  pipelineWorkspace: Ref<PipelineWorkspace>;
}

export function usePipelineStudio(options: UsePipelineStudioOptions) {
  const selectedNodeIds = ref<string[]>([]);
  const selectedEdgeIds = ref<string[]>([]);
  const catalogSearch = ref("");
  const lastCopiedNodeIds = ref<string[]>([]);

  const activeTemplate = computed(() => {
    const workspace = options.pipelineWorkspace.value;
    return workspace.templates.find((template) => template.id === workspace.active_template_id) || workspace.templates[0] || null;
  });

  const activeTargets = computed(() => (activeTemplate.value ? getActiveTargets(activeTemplate.value.id, options.pipelineWorkspace.value.targets) : []));
  const overlayTargetId = computed(() => {
    const activeTargetId = options.pipelineWorkspace.value.active_target_id;
    if (activeTargets.value.some((target) => target.id === activeTargetId)) {
      return activeTargetId;
    }
    return activeTargets.value[0]?.id || "";
  });

  const issues = computed(() => (activeTemplate.value ? buildTemplateIssues(activeTemplate.value, options.nodeCatalog.value) : []));
  const issueNodeIds = computed(() => new Set(issues.value.map((issue) => issue.nodeId).filter(Boolean) as string[]));
  const issueEdgeIds = computed(() => new Set(issues.value.map((issue) => issue.edgeId).filter(Boolean) as string[]));

  const overlay = computed(() => {
    if (!activeTemplate.value) {
      return buildPipelineOverlay({ bound_config_snapshot: normalizeConfigSnapshot({}), created_at: "", description: "", enabled: true, entry_node_id: "", edges: [], id: "", name: "", nodes: [], updated_at: "", version: 1 }, [], "", "");
    }
    return buildPipelineOverlay(activeTemplate.value, options.pipelineResults.value, options.activePipelineId.value, overlayTargetId.value);
  });

  const workspaceSignature = computed(() => stablePipelineWorkspaceSignature(options.pipelineWorkspace.value));

  watch(
    activeTemplate,
    (template) => {
      if (!template) {
        return;
      }
      ensureTemplateLayout(template);
    },
    { immediate: true },
  );

  function clearSelection() {
    selectedNodeIds.value = [];
    selectedEdgeIds.value = [];
  }

  function setSelectedNodes(nodeIds: string[]) {
    selectedNodeIds.value = [...nodeIds];
  }

  function setSelectedEdges(edgeIds: string[]) {
    selectedEdgeIds.value = [...edgeIds];
  }

  function currentSelectedEdge(template: PipelineTemplate | null): PipelineEdge | null {
    if (!template || selectedEdgeIds.value.length !== 1) {
      return null;
    }
    return template.edges.find((edge) => edge.id === selectedEdgeIds.value[0]) || null;
  }

  return {
    activeTemplate,
    catalogSearch,
    clearSelection,
    currentSelectedEdge,
    issueEdgeIds,
    issueNodeIds,
    issues,
    lastCopiedNodeIds,
    overlay,
    selectedEdgeIds,
    selectedNodeIds,
    setSelectedEdges,
    setSelectedNodes,
    workspaceSignature,
  };
}
