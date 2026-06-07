import type { PipelineNodeRunResult, PipelineNodeType, PipelineProfileRunResult, PipelineRunResult, ProbeRunResultPayload } from "./bridge/types";
import { isObject, toInteger, toStringValue } from "./bridgeValues";

export function normalizePipelineNodeType(value: unknown): PipelineNodeType {
  const normalized = toStringValue(value).trim().toLowerCase();
  if (normalized === "source" || normalized === "filter" || normalized === "branch" || normalized === "deliver" || normalized === "recovery" || normalized === "end") {
    return normalized;
  }
  return "probe";
}

export function normalizePipelineNodeAction(value: unknown, nodeType: PipelineNodeType): string {
  const normalized = toStringValue(value).trim().toLowerCase();
  if (normalized === "source_group" || normalized === "select_source") {
    return "select_sources";
  }
  if (normalized === "run_probe") {
    return "probe_tcp";
  }
  if (normalized === "filter_candidates") {
    return "filter_results";
  }
  if (normalized === "has_results") {
    return "branch_has_results";
  }
  if (normalized === "dns_push") {
    return "deliver_dns";
  }
  if (normalized === "github_export") {
    return "deliver_github";
  }
  if (normalized === "mark_manual_review") {
    return "recovery_mark";
  }
  if (normalized === "completed" || normalized === "manual_review") {
    return "end";
  }
  if (
    normalized === "select_sources" ||
    normalized === "filter_sources" ||
    normalized === "probe_tcp" ||
    normalized === "probe_trace" ||
    normalized === "probe_download" ||
    normalized === "filter_results" ||
    normalized === "branch_has_results" ||
    normalized === "deliver_dns" ||
    normalized === "deliver_github" ||
    normalized === "recovery_mark" ||
    normalized === "check_output" ||
    normalized === "end"
  ) {
    return normalized;
  }
  if (normalized) {
    return normalized;
  }
  return defaultPipelineNodeAction(nodeType);
}

function defaultPipelineNodeAction(nodeType: PipelineNodeType): string {
  switch (nodeType) {
    case "source":
      return "select_sources";
    case "filter":
      return "filter_results";
    case "branch":
      return "branch_has_results";
    case "deliver":
      return "deliver_dns";
    case "recovery":
      return "recovery_mark";
    case "end":
      return "end";
    default:
      return "probe_tcp";
  }
}

function normalizePipelineProfileRunResult(input: unknown): PipelineProfileRunResult {
  const source = isObject(input) ? input : {};
  const rawNodeResults = source.node_results ?? source.nodeResults;
  const rawWarnings = source.warnings;
  return {
    dns_result: source.dns_result ?? source.dnsResult,
    domain: toStringValue(source.domain),
    message: toStringValue(source.message),
    node_results: Array.isArray(rawNodeResults) ? rawNodeResults.map((entry: unknown) => normalizePipelineNodeRunResult(entry)) : [],
    profile_id: toStringValue(source.profile_id ?? source.profileId),
    profile_name: toStringValue(source.profile_name ?? source.profileName),
    probe_result: isObject(source.probe_result ?? source.probeResult) ? ((source.probe_result ?? source.probeResult) as ProbeRunResultPayload) : null,
    region: toStringValue(source.region),
    status: toStringValue(source.status),
    task_id: toStringValue(source.task_id ?? source.taskId),
    target_id: toStringValue(source.target_id ?? source.targetId ?? source.profile_id ?? source.profileId),
    target_name: toStringValue(source.target_name ?? source.targetName ?? source.profile_name ?? source.profileName),
    warnings: Array.isArray(rawWarnings) ? rawWarnings.map((entry: unknown) => toStringValue(entry)).filter(Boolean) : [],
  };
}

function normalizePipelineNodeRunResult(input: unknown): PipelineNodeRunResult {
  const source = isObject(input) ? input : {};
  const nodeType = normalizePipelineNodeType(source.node_type ?? source.nodeType);
  const outcome = toStringValue(source.outcome ?? source.branch_taken ?? source.branchTaken);
  return {
    action: normalizePipelineNodeAction(source.action, nodeType),
    branch_taken: outcome,
    completed_at: toStringValue(source.completed_at ?? source.completedAt),
    message: toStringValue(source.message),
    metrics: isObject(source.metrics) ? source.metrics : null,
    node_id: toStringValue(source.node_id ?? source.nodeId),
    node_name: toStringValue(source.node_name ?? source.nodeName),
    node_type: nodeType,
    outcome,
    output_summary: toStringValue(source.output_summary ?? source.outputSummary),
    started_at: toStringValue(source.started_at ?? source.startedAt),
    status: toStringValue(source.status),
  };
}

export function normalizePipelineRunResult(input: unknown): PipelineRunResult {
  const source = isObject(input) ? input : {};
  const rawTargetResults = source.target_results ?? source.targetResults;
  const rawResults = source.results;
  const rawTargetIDs = source.target_ids ?? source.targetIds;
  const rawWarnings = source.warnings;
  const results: unknown[] = Array.isArray(rawTargetResults) ? rawTargetResults : Array.isArray(rawResults) ? rawResults : [];
  return {
    completed_at: toStringValue(source.completed_at ?? source.completedAt),
    duration_ms: toInteger(source.duration_ms ?? source.durationMS ?? source.durationMs, 0),
    failed: toInteger(source.failed, 0),
    pipeline_id: toStringValue(source.pipeline_id ?? source.pipelineId),
    results: results.map((entry: unknown) => normalizePipelineProfileRunResult(entry)),
    skipped: toInteger(source.skipped, 0),
    started_at: toStringValue(source.started_at ?? source.startedAt),
    status: toStringValue(source.status),
    succeeded: toInteger(source.succeeded, 0),
    task_id: toStringValue(source.task_id ?? source.taskId),
    target_ids: Array.isArray(rawTargetIDs) ? rawTargetIDs.map((entry: unknown) => toStringValue(entry)).filter(Boolean) : [],
    target_results: results.map((entry: unknown) => normalizePipelineProfileRunResult(entry)),
    template_id: toStringValue(source.template_id ?? source.templateId),
    total: toInteger(source.total, results.length),
    warnings: Array.isArray(rawWarnings) ? rawWarnings.map((entry: unknown) => toStringValue(entry)).filter(Boolean) : [],
  };
}

export function normalizePipelineRunResults(input: unknown): PipelineRunResult[] {
  return Array.isArray(input) ? input.map((entry) => normalizePipelineRunResult(entry)) : [];
}
