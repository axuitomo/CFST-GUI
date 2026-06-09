#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const backendPath = path.join(root, "internal/appcore/pipeline.go");
const frontendPath = path.join(root, "frontend/src/lib/bridge/pipeline.ts");

const backendSource = fs.readFileSync(backendPath, "utf8");
const frontendSource = fs.readFileSync(frontendPath, "utf8");
const backendCatalogStart = backendSource.indexOf("func DefaultPipelineNodeCatalog()");
const backendCatalogEnd = backendSource.indexOf("func pipelineNodeCatalogByAction()", backendCatalogStart);
const backendCatalogSource =
  backendCatalogStart >= 0 && backendCatalogEnd > backendCatalogStart
    ? backendSource.slice(backendCatalogStart, backendCatalogEnd)
    : backendSource;

const backendActionValues = new Map([
  ["PipelineNodeActionSelectSources", "select_sources"],
  ["PipelineNodeActionFilterSources", "filter_sources"],
  ["PipelineNodeActionProbeTCP", "probe_tcp"],
  ["PipelineNodeActionProbeTrace", "probe_trace"],
  ["PipelineNodeActionProbeDownload", "probe_download"],
  ["PipelineNodeActionFilterResults", "filter_results"],
  ["PipelineNodeActionBranchHasResults", "branch_has_results"],
  ["PipelineNodeActionDeliverDNS", "deliver_dns"],
  ["PipelineNodeActionDeliverGitHub", "deliver_github"],
  ["PipelineNodeActionRecoveryMark", "recovery_mark"],
  ["PipelineNodeActionCheckOutput", "check_output"],
  ["PipelineNodeActionEnd", "end"],
]);

const backendNodeTypeValues = new Map([
  ["PipelineNodeTypeSource", "source"],
  ["PipelineNodeTypeProbe", "probe"],
  ["PipelineNodeTypeFilter", "filter"],
  ["PipelineNodeTypeBranch", "branch"],
  ["PipelineNodeTypeDeliver", "deliver"],
  ["PipelineNodeTypeRecovery", "recovery"],
  ["PipelineNodeTypeEnd", "end"],
]);

function findMatchingBrace(source, openIndex) {
  let depth = 0;
  let inString = false;
  let escaped = false;
  for (let index = openIndex; index < source.length; index += 1) {
    const char = source[index];
    if (inString) {
      if (escaped) {
        escaped = false;
      } else if (char === "\\") {
        escaped = true;
      } else if (char === "\"") {
        inString = false;
      }
      continue;
    }
    if (char === "\"") {
      inString = true;
      continue;
    }
    if (char === "{") {
      depth += 1;
    } else if (char === "}") {
      depth -= 1;
      if (depth === 0) {
        return index;
      }
    }
  }
  return -1;
}

function backendItems() {
  const items = [];
  const actionPattern = /Action:\s*(PipelineNodeAction[A-Za-z0-9]+)/g;
  for (const match of backendCatalogSource.matchAll(actionPattern)) {
    const blockStart = backendCatalogSource.lastIndexOf("{", match.index);
    const blockEnd = findMatchingBrace(backendCatalogSource, blockStart);
    if (blockStart < 0 || blockEnd < 0) {
      continue;
    }
    const block = backendCatalogSource.slice(blockStart, blockEnd + 1);
    const typeMatch = block.match(/NodeType:\s*(PipelineNodeType[A-Za-z0-9]+)/);
    const action = backendActionValues.get(match[1]);
    const nodeType = typeMatch ? backendNodeTypeValues.get(typeMatch[1]) : "";
    if (!action || !nodeType) {
      continue;
    }
    items.push({
      action,
      fieldKeys: [...block.matchAll(/Key:\s*"([^"]+)"/g)].map((entry) => entry[1]),
      nodeType,
    });
  }
  return items;
}

function frontendItems() {
  const items = [];
  const pattern = /normalizePipelineNodeCatalogItem\(\{/g;
  for (const match of frontendSource.matchAll(pattern)) {
    const openIndex = frontendSource.indexOf("{", match.index);
    const closeIndex = findMatchingBrace(frontendSource, openIndex);
    if (closeIndex < 0) {
      continue;
    }
    const block = frontendSource.slice(openIndex, closeIndex + 1);
    const action = block.match(/action:\s*"([^"]+)"/)?.[1] || "";
    const nodeType = block.match(/node_type:\s*"([^"]+)"/)?.[1] || "";
    if (!action || !nodeType) {
      continue;
    }
    items.push({
      action,
      fieldKeys: [...block.matchAll(/key:\s*"([^"]+)"/g)].map((entry) => entry[1]),
      nodeType,
    });
  }
  return items;
}

function uniqueByAction(items) {
  const result = new Map();
  for (const item of items) {
    result.set(item.action, item);
  }
  return result;
}

function sorted(values) {
  return [...new Set(values)].sort();
}

const backend = uniqueByAction(backendItems());
const frontend = uniqueByAction(frontendItems());
const errors = [];
const requiredProbeFieldKeys = [
  "concurrency_stage1",
  "concurrency_stage2",
  "concurrency_stage3",
  "download_buffer_kb",
  "download_count",
  "download_get_concurrency",
  "download_http_protocol",
  "download_speed_metric",
  "download_speed_sample_interval_ms",
  "download_time_seconds",
  "download_warmup_seconds",
  "httping_cf_colo",
  "httping_cf_colo_mode",
  "httping_status_code",
  "max_loss_rate",
  "max_tcp_latency_ms",
  "max_trace_latency_ms",
  "min_delay_ms",
  "min_download_mbps",
  "ping_times",
  "port_policy",
  "print_num",
  "source_colo_filter_phase",
  "stage3_limit",
  "tcp_port",
  "timeout_stage1_ms",
  "timeout_stage2_ms",
  "timeout_stage3_ms",
  "trace_colo_mode",
  "trace_url",
  "url",
];

for (const [action, backendItem] of backend) {
  const frontendItem = frontend.get(action);
  if (!frontendItem) {
    errors.push(`frontend fallback is missing action ${action}`);
    continue;
  }
  if (frontendItem.nodeType !== backendItem.nodeType) {
    errors.push(`${action}: frontend node_type=${frontendItem.nodeType}, backend node_type=${backendItem.nodeType}`);
  }
  const backendKeys = sorted(backendItem.fieldKeys).join(",");
  const frontendKeys = sorted(frontendItem.fieldKeys).join(",");
  if (frontendKeys !== backendKeys) {
    errors.push(`${action}: frontend fields=[${frontendKeys}], backend fields=[${backendKeys}]`);
  }
}

for (const action of frontend.keys()) {
  if (!backend.has(action)) {
    errors.push(`frontend fallback has unknown action ${action}`);
  }
}

for (const action of ["probe_tcp", "probe_trace", "probe_download"]) {
  const backendItem = backend.get(action);
  const frontendItem = frontend.get(action);
  if (!backendItem || !frontendItem) {
    continue;
  }
  for (const key of requiredProbeFieldKeys) {
    const backendHasKey = backendItem.fieldKeys.includes(key) || new RegExp(`Key:\\s*"${key}"`).test(backendSource);
    const frontendHasKey = frontendItem.fieldKeys.includes(key) || new RegExp(`key:\\s*"${key}"`).test(frontendSource);
    if (!backendHasKey) {
      errors.push(`${action}: backend full-mode helper is missing ${key}`);
    }
    if (!frontendHasKey) {
      errors.push(`${action}: frontend full-mode helper is missing ${key}`);
    }
  }
}

if (errors.length > 0) {
  console.error("Pipeline catalog consistency check failed:");
  for (const error of errors) {
    console.error(`- ${error}`);
  }
  process.exit(1);
}

console.log(`Pipeline catalog consistency OK (${backend.size} actions).`);
