<script setup lang="ts">
import PipelineStudioDesktop from "../components/pipeline/PipelineStudioDesktop.vue";
import PipelineStudioMobile from "../components/pipeline/PipelineStudioMobile.vue";
import type { PipelineNodeCatalogItem, PipelineRunResult, PipelineTemplate, PipelineWorkspace, SchedulerStatus } from "../lib/bridge";

interface TimestampFormatOptions {
  fallback?: string;
  includeDate?: boolean;
  includeOffset?: boolean;
  includeSeconds?: boolean;
}

interface WorkflowSchedulerState {
  autoDnsPush: boolean;
  dailyTimes: string;
  enabled: boolean;
  intervalMinutes: number;
  skipIfActive: boolean;
  templateId: string;
}

interface ProcessEntry {
  detail: string;
  stage: string;
  title: string;
  tone: "success" | "error" | "running" | "info" | "warning";
  ts: string;
}

const props = withDefaults(
  defineProps<{
  activePipelineId: string;
  canStartPipeline: boolean;
  formatTimestamp: (value: string, options?: TimestampFormatOptions) => string;
  fitRequestKey?: number;
  loading: boolean;
  nodeCatalog: PipelineNodeCatalogItem[];
  pipelineResults: PipelineRunResult[];
  pipelineWorkspace: PipelineWorkspace;
  platform: "desktop" | "mobile";
  processTrace: ProcessEntry[];
  schedulerState: WorkflowSchedulerState;
  schedulerStatus: SchedulerStatus | null;
  workspaceDirty: boolean;
}>(),
  {
    fitRequestKey: 0,
  },
);

const emit = defineEmits<{
  (event: "activate-template", templateId: string): void;
  (event: "create-template"): void;
  (event: "delete-template", templateId: string): void;
  (event: "open-dashboard"): void;
  (event: "apply-template-config", templateId: string): void;
  (event: "bind-template-config", templateId: string): void;
  (event: "clear-process"): void;
  (event: "save-scheduler", payload: WorkflowSchedulerState): void;
  (event: "save-template", template: PipelineTemplate): void;
  (event: "save-workspace"): void;
  (event: "start-pipeline", templateId: string): void;
}>();
</script>

<template>
  <PipelineStudioDesktop
    v-if="platform === 'desktop'"
    :active-pipeline-id="activePipelineId"
    :can-start-pipeline="canStartPipeline"
    :format-timestamp="formatTimestamp"
    :fit-request-key="props.fitRequestKey"
    :loading="loading"
    :node-catalog="nodeCatalog"
    :pipeline-results="pipelineResults"
    :pipeline-workspace="pipelineWorkspace"
    :process-trace="processTrace"
    :scheduler-state="schedulerState"
    :scheduler-status="schedulerStatus"
    :workspace-dirty="workspaceDirty"
    @activate-template="emit('activate-template', $event)"
    @apply-template-config="emit('apply-template-config', $event)"
    @bind-template-config="emit('bind-template-config', $event)"
    @clear-process="emit('clear-process')"
    @create-template="emit('create-template')"
    @delete-template="emit('delete-template', $event)"
    @open-dashboard="emit('open-dashboard')"
    @save-scheduler="emit('save-scheduler', $event)"
    @save-template="emit('save-template', $event)"
    @save-workspace="emit('save-workspace')"
    @start-pipeline="emit('start-pipeline', $event)"
  />

  <PipelineStudioMobile
    v-else
    :active-pipeline-id="activePipelineId"
    :can-start-pipeline="canStartPipeline"
    :format-timestamp="formatTimestamp"
    :loading="loading"
    :node-catalog="nodeCatalog"
    :pipeline-results="pipelineResults"
    :pipeline-workspace="pipelineWorkspace"
    :workspace-dirty="workspaceDirty"
    @activate-template="emit('activate-template', $event)"
    @create-template="emit('create-template')"
    @delete-template="emit('delete-template', $event)"
    @open-dashboard="emit('open-dashboard')"
    @save-template="emit('save-template', $event)"
    @save-workspace="emit('save-workspace')"
    @start-pipeline="emit('start-pipeline', $event)"
  />
</template>
