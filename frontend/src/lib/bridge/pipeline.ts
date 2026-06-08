import { isObject, toBoolean, toInteger, toNumber, toStringValue } from "../bridgeValues";
import { normalizePipelineNodeAction, normalizePipelineNodeType } from "../pipelineRunResults";
import { DEFAULT_FILE_TEST_URL, DEFAULT_HTTPING_STATUS_CODE, DEFAULT_MAX_LOSS_RATE, MAX_LOSS_RATE, MIN_PROBE_PING_TIMES, normalizeConfigSnapshot } from "./config";
import type {
  PipelineDNSPushPolicy,
  PipelineEdge,
  PipelineNode,
  PipelineNodeCatalogField,
  PipelineNodeCatalogFieldOption,
  PipelineNodeCatalogFieldVisibleWhen,
  PipelineNodeCatalogItem,
  PipelineNodeCatalogOutcome,
  PipelineNodeFieldType,
  PipelineNodeType,
  PipelineProfile,
  PipelineProfileStore,
  PipelineTarget,
  PipelineTemplate,
  PipelineWorkspace,
  PipelineCanvasPosition,
  PipelineViewport,
  PipelineNodeUI,
  PipelineTemplateUI,
} from "./types";

function normalizePipelineDNSPushPolicy(value: unknown): PipelineDNSPushPolicy {
  const normalized = toStringValue(value).trim().toLowerCase();
  return normalized === "skip" || normalized === "manual" || normalized === "disabled" || normalized === "none" ? "skip" : "auto";
}

function normalizePipelineNodeConfig(nodeType: PipelineNodeType, action: string, config: unknown): Record<string, unknown> {
  const source = isObject(config) ? { ...config } : {};
  if (action === "select_sources") {
    if (!Array.isArray(source.source_ids)) {
      source.source_ids = [];
    }
    if (typeof source.source_profile_id !== "string") {
      source.source_profile_id = "";
    }
    if (source.source_selection !== "custom") {
      source.source_selection = "enabled";
    }
  }
  if (action === "filter_sources") {
    if (typeof source.source_ip_limit !== "number") {
      source.source_ip_limit = 500;
    }
    if (typeof source.source_ip_mode !== "string") {
      source.source_ip_mode = "traverse";
    }
    if (typeof source.source_colo_filter !== "string") {
      source.source_colo_filter = "";
    }
    if (typeof source.source_colo_filter_mode !== "string") {
      source.source_colo_filter_mode = "allow";
    }
  }
  if (action === "probe_tcp" || action === "probe_trace" || action === "probe_download") {
    for (const [key, value] of Object.entries(pipelineProbeFullModeDefaultConfig())) {
      if (!(key in source)) {
        source[key] = value;
      }
    }
  }
  if (action === "branch_has_results" && typeof source.source !== "string") {
    source.source = "probe_results";
  }
  if (action === "filter_results" && typeof source.source !== "string") {
    source.source = "probe_results";
  }
  if ((action === "deliver_dns" || action === "deliver_github") && typeof source.source !== "string") {
    source.source = "filtered_rows";
  }
  if (action === "recovery_mark") {
    if (typeof source.status !== "string") {
      source.status = "manual_review";
    }
    if (typeof source.message !== "string") {
      source.message = "需要人工复核。";
    }
  }
  if (action === "end") {
    if (typeof source.status !== "string") {
      source.status = nodeType === "end" ? "completed" : "manual_review";
    }
    if (typeof source.message !== "string") {
      source.message = source.status === "manual_review" ? "需要人工复核。" : "流程已结束。";
    }
  }
  if (action === "check_output") {
    if (typeof source.source !== "string") {
      source.source = "probe_results";
    }
    if (typeof source.require_csv !== "boolean") {
      source.require_csv = true;
    }
    if (typeof source.export_if_missing !== "boolean") {
      source.export_if_missing = true;
    }
    if (typeof source.status !== "string") {
      source.status = "passed";
    }
    if (typeof source.top_n !== "number") {
      source.top_n = 0;
    }
  }
  return source;
}

function normalizePipelineNodeCatalogFieldOption(input: unknown): PipelineNodeCatalogFieldOption {
  const source = isObject(input) ? input : {};
  return {
    label: toStringValue(source.label),
    value: toStringValue(source.value),
  };
}

function normalizePipelineNodeCatalogFieldVisibleWhen(input: unknown): PipelineNodeCatalogFieldVisibleWhen | undefined {
  const source = isObject(input) ? input : {};
  const field = toStringValue(source.field).trim();
  if (!field) {
    return undefined;
  }
  const result: PipelineNodeCatalogFieldVisibleWhen = {
    field,
  };
  if ("equals" in source) {
    result.equals = source.equals;
  }
  if ("not_equals" in source || "notEquals" in source) {
    result.not_equals = source.not_equals ?? source.notEquals;
  }
  return result;
}

function normalizePipelineNodeCatalogField(input: unknown): PipelineNodeCatalogField {
  const source = isObject(input) ? input : {};
  const rawOptions = Array.isArray(source.options) ? source.options : [];
  const fieldType = toStringValue(source.field_type ?? source.fieldType)
    .trim()
    .toLowerCase();
  const numericRows = toInteger(source.rows, 0);
  const numericMin = toNumber(source.min, Number.NaN);
  const numericMax = toNumber(source.max, Number.NaN);
  const numericStep = toNumber(source.step, Number.NaN);
  return {
    default_value: source.default_value ?? source.defaultValue,
    description: toStringValue(source.description),
    field_type: fieldType === "textarea" || fieldType === "select" || fieldType === "checkbox" || fieldType === "number" || fieldType === "json" ? (fieldType as PipelineNodeFieldType) : "text",
    group: toStringValue(source.group),
    help_text: toStringValue(source.help_text ?? source.helpText),
    key: toStringValue(source.key),
    label: toStringValue(source.label),
    max: Number.isFinite(numericMax) ? numericMax : undefined,
    min: Number.isFinite(numericMin) ? numericMin : undefined,
    options: rawOptions.map((entry) => normalizePipelineNodeCatalogFieldOption(entry)).filter((entry) => entry.value),
    placeholder: toStringValue(source.placeholder),
    required: toBoolean(source.required, false),
    rows: numericRows > 0 ? numericRows : undefined,
    step: Number.isFinite(numericStep) ? numericStep : undefined,
    visible_when: normalizePipelineNodeCatalogFieldVisibleWhen(source.visible_when ?? source.visibleWhen),
  };
}

function normalizePipelineNodeCatalogOutcome(input: unknown): PipelineNodeCatalogOutcome {
  const source = isObject(input) ? input : {};
  return {
    description: toStringValue(source.description),
    label: toStringValue(source.label),
    value: toStringValue(source.value),
  };
}

export function normalizePipelineNodeCatalogItem(input: unknown): PipelineNodeCatalogItem {
  const source = isObject(input) ? input : {};
  const rawNodeType = normalizePipelineNodeType(source.node_type ?? source.nodeType);
  const action = normalizePipelineNodeAction(source.action, rawNodeType);
  const nodeType = action === "check_output" || action === "deliver_dns" || action === "deliver_github" ? "deliver" : rawNodeType;
  const rawFormSchema = source.form_schema ?? source.formSchema;
  return {
    action,
    default_config: normalizePipelineNodeConfig(nodeType, action, source.default_config ?? source.defaultConfig ?? {}),
    description: toStringValue(source.description),
    display_name: toStringValue(source.display_name ?? source.displayName),
    form_schema: Array.isArray(rawFormSchema) ? rawFormSchema.map((entry) => normalizePipelineNodeCatalogField(entry)) : [],
    node_type: nodeType,
    outcomes: Array.isArray(source.outcomes) ? source.outcomes.map((entry) => normalizePipelineNodeCatalogOutcome(entry)) : [],
  };
}

function normalizePipelineCanvasPosition(input: unknown): PipelineCanvasPosition | undefined {
  const source = isObject(input) ? input : {};
  const x = toNumber(source.x, Number.NaN);
  const y = toNumber(source.y, Number.NaN);
  if (!Number.isFinite(x) || !Number.isFinite(y)) {
    return undefined;
  }
  return { x, y };
}

function normalizePipelineViewport(input: unknown): PipelineViewport | undefined {
  const source = isObject(input) ? input : {};
  const x = toNumber(source.x, Number.NaN);
  const y = toNumber(source.y, Number.NaN);
  const zoom = toNumber(source.zoom, Number.NaN);
  if (!Number.isFinite(x) || !Number.isFinite(y) || !Number.isFinite(zoom)) {
    return undefined;
  }
  return { x, y, zoom };
}

function normalizePipelineNodeUI(input: unknown): PipelineNodeUI | undefined {
  const source = isObject(input) ? input : {};
  const position = normalizePipelineCanvasPosition(source.position);
  const width = toNumber(source.width, Number.NaN);
  const collapsed = "collapsed" in source ? toBoolean(source.collapsed, false) : undefined;
  if (!position && !Number.isFinite(width) && collapsed === undefined) {
    return undefined;
  }
  return {
    collapsed,
    position,
    width: Number.isFinite(width) ? width : undefined,
  };
}

function normalizePipelineTemplateUI(input: unknown): PipelineTemplateUI | undefined {
  const source = isObject(input) ? input : {};
  const viewport = normalizePipelineViewport(source.viewport);
  if (!viewport) {
    return undefined;
  }
  return {
    viewport,
  };
}

function normalizePipelineNode(input: unknown, index: number): PipelineNode {
  const source = isObject(input) ? input : {};
  const rawNodeType = normalizePipelineNodeType(source.node_type ?? source.nodeType);
  const action = normalizePipelineNodeAction(source.action, rawNodeType);
  const nodeType = action === "check_output" || action === "deliver_dns" || action === "deliver_github" ? "deliver" : rawNodeType;
  return {
    action,
    config: normalizePipelineNodeConfig(nodeType, action, source.config),
    id: toStringValue(source.id) || `pipeline-node-${index + 1}`,
    name: toStringValue(source.name) || `步骤 ${index + 1}`,
    node_type: nodeType,
    ui: normalizePipelineNodeUI(source.ui),
    updated_at: toStringValue(source.updated_at ?? source.updatedAt),
  };
}

function normalizePipelineEdge(input: unknown, index: number): PipelineEdge {
  const source = isObject(input) ? input : {};
  return {
    id: toStringValue(source.id) || `pipeline-edge-${index + 1}`,
    label: toStringValue(source.label),
    outcome: toStringValue(source.outcome),
    source_node_id: toStringValue(source.source_node_id ?? source.sourceNodeId),
    target_node_id: toStringValue(source.target_node_id ?? source.targetNodeId),
  };
}

export function normalizePipelineTemplate(input: unknown, index = 0): PipelineTemplate {
  const source = isObject(input) ? input : {};
  const nodes = Array.isArray(source.nodes) ? source.nodes : [];
  const edges = Array.isArray(source.edges) ? source.edges : [];
  return {
    bound_config_snapshot: normalizeConfigSnapshot(source.bound_config_snapshot ?? source.boundConfigSnapshot ?? {}),
    created_at: toStringValue(source.created_at ?? source.createdAt),
    description: toStringValue(source.description),
    enabled: toBoolean(source.enabled, true),
    entry_node_id: toStringValue(source.entry_node_id ?? source.entryNodeId),
    edges: edges.map((entry, edgeIndex) => normalizePipelineEdge(entry, edgeIndex)),
    id: toStringValue(source.id) || `pipeline-template-${index + 1}`,
    name: toStringValue(source.name) || `流程 ${index + 1}`,
    nodes: nodes.map((entry, nodeIndex) => normalizePipelineNode(entry, nodeIndex)),
    ui: normalizePipelineTemplateUI(source.ui),
    updated_at: toStringValue(source.updated_at ?? source.updatedAt),
    version: toInteger(source.version, 1),
  };
}

export function normalizePipelineTarget(input: unknown, index = 0): PipelineTarget {
  const source = isObject(input) ? input : {};
  const snapshot = normalizeConfigSnapshot(source.config_snapshot ?? source.configSnapshot ?? {});
  return {
    config_snapshot: snapshot,
    created_at: toStringValue(source.created_at ?? source.createdAt),
    dns_push_policy: normalizePipelineDNSPushPolicy(source.dns_push_policy ?? source.dnsPushPolicy),
    domain: toStringValue(source.domain ?? snapshot.cloudflare.record_name),
    enabled: toBoolean(source.enabled, true),
    id: toStringValue(source.id) || `pipeline-target-${index + 1}`,
    name: toStringValue(source.name) || `目标 ${index + 1}`,
    region: toStringValue(source.region) || "未分组",
    tags: Array.isArray(source.tags) ? source.tags.map((entry) => toStringValue(entry)).filter(Boolean) : [],
    template_id: toStringValue(source.template_id ?? source.templateId) || "pipeline-template-default",
    updated_at: toStringValue(source.updated_at ?? source.updatedAt),
  };
}

export function normalizePipelineWorkspace(input: unknown): PipelineWorkspace {
  const source = isObject(input) ? input : {};
  const templates = Array.isArray(source.templates) ? source.templates : [];
  const rawTargets = Array.isArray(source.targets) ? source.targets : [];
  const activeTemplateId = toStringValue(source.active_template_id ?? source.activeTemplateId) || "pipeline-template-default";
  const activeTargetId = toStringValue(source.active_target_id ?? source.activeTargetId);
  const normalizedTemplates = templates.length > 0 ? templates.map((entry, index) => normalizePipelineTemplate(entry, index)) : [normalizePipelineTemplate({}, 0)];
  const legacyTargets = rawTargets.map((entry, index) => normalizePipelineTarget(entry, index));
  const normalizedTargets = normalizedTemplates.map((template, index) => {
    const preferred = legacyTargets.find((target) => target.id === activeTargetId && target.template_id === template.id) || legacyTargets.find((target) => target.template_id === template.id && target.enabled) || legacyTargets.find((target) => target.template_id === template.id) || null;
    if (Object.keys(template.bound_config_snapshot || {}).length === 0 && preferred?.config_snapshot) {
      template.bound_config_snapshot = normalizeConfigSnapshot(preferred.config_snapshot);
    }
    const snapshot = normalizeConfigSnapshot(template.bound_config_snapshot || {});
    const templateTargetId = preferred?.id || `${template.id || `pipeline-template-${index + 1}`}-target`;
    const domain = toStringValue(preferred?.domain ?? snapshot.cloudflare.record_name);
    return {
      config_snapshot: snapshot,
      created_at: preferred?.created_at || template.created_at,
      dns_push_policy: preferred?.dns_push_policy || "auto",
      domain,
      enabled: preferred?.enabled ?? true,
      id: templateTargetId,
      name: preferred?.name || template.name || `工作流 ${index + 1}`,
      region: preferred?.region || "当前配置",
      tags: preferred?.tags || [],
      template_id: template.id,
      updated_at: preferred?.updated_at || template.updated_at,
    } satisfies PipelineTarget;
  });
  const normalizedActiveTargetId = normalizedTargets.find((target) => target.template_id === activeTemplateId)?.id || normalizedTargets[0]?.id || "";
  return {
    active_target_id: normalizedActiveTargetId,
    active_template_id: activeTemplateId,
    schema_version: toStringValue(source.schema_version ?? source.schemaVersion),
    targets: normalizedTargets,
    templates: normalizedTemplates,
    updated_at: toStringValue(source.updated_at ?? source.updatedAt),
  };
}

export function pipelineWorkspaceFromProfileStore(input: unknown): PipelineWorkspace {
  const source = isObject(input) ? input : {};
  const profiles = Array.isArray(source.items) ? source.items : [];
  const activeProfileId = toStringValue(source.active_profile_id ?? source.activeProfileId);
  const template = normalizePipelineTemplate({}, 0);
  const targets = profiles.map((entry, index) => normalizePipelineTarget(entry, index));
  return {
    active_target_id: targets.find((target) => target.id === activeProfileId)?.id || targets[0]?.id || "",
    active_template_id: template.id,
    schema_version: toStringValue(source.schema_version ?? source.schemaVersion),
    targets: targets.map((target) => ({ ...target, template_id: template.id })),
    templates: [template],
    updated_at: toStringValue(source.updated_at ?? source.updatedAt),
  };
}

function pipelineProbeFullModeDefaultConfig(): Record<string, unknown> {
  return {
    concurrency_stage1: 200,
    concurrency_stage2: 30,
    concurrency_stage3: 1,
    disable_download: false,
    download_buffer_kb: 256,
    download_count: 10,
    download_get_concurrency: 4,
    download_http_protocol: "auto",
    download_speed_metric: "average",
    download_speed_sample_interval_ms: 500,
    download_time_seconds: 10,
    download_warmup_seconds: 5,
    httping_cf_colo: "",
    httping_cf_colo_mode: "allow",
    httping_status_code: 0,
    max_loss_rate: DEFAULT_MAX_LOSS_RATE,
    max_tcp_latency_ms: null,
    max_trace_latency_ms: null,
    min_delay_ms: 0,
    min_download_mbps: 0,
    ping_times: 4,
    port_policy: "source_override_global",
    print_num: 0,
    source_colo_filter_phase: "precheck",
    stage3_limit: 10,
    strategy: "full",
    tcp_port: 443,
    timeout_stage1_ms: 1000,
    timeout_stage2_ms: 1000,
    timeout_stage3_ms: 10000,
    trace_colo_mode: "standard",
    trace_url: "",
    url: DEFAULT_FILE_TEST_URL,
  };
}

function pipelineProbeFullModeFormSchema(primaryStage: string): PipelineNodeCatalogField[] {
  const tcpFields: PipelineNodeCatalogField[] = [
    { default_value: 443, field_type: "number", group: "第一阶段 TCP", key: "tcp_port", label: "全局测速端口", max: 65535, min: 1, step: 1 },
    {
      default_value: "source_override_global",
      field_type: "select",
      group: "第一阶段 TCP",
      help_text: "输入源声明端口时优先使用，否则回退到固定端口。",
      key: "port_policy",
      label: "端口策略",
      options: [
        { label: "输入源端口优先", value: "source_override_global" },
        { label: "固定全局端口", value: "fixed_global" },
      ],
    },
    { default_value: 200, field_type: "number", group: "第一阶段 TCP", key: "concurrency_stage1", label: "TCP 并发线程", max: 1000, min: 1, step: 1 },
    { default_value: 4, field_type: "number", group: "第一阶段 TCP", key: "ping_times", label: "TCP 发包次数", min: MIN_PROBE_PING_TIMES, step: 1 },
    { field_type: "number", group: "第一阶段 TCP", key: "max_tcp_latency_ms", label: "TCP 延迟上限(ms)", min: 1, step: 1 },
    { default_value: 0, field_type: "number", group: "第一阶段 TCP", key: "min_delay_ms", label: "TCP 延迟下限(ms)", min: 0, step: 1 },
    { default_value: DEFAULT_MAX_LOSS_RATE, field_type: "number", group: "第一阶段 TCP", key: "max_loss_rate", label: "TCP 丢包率上限", max: MAX_LOSS_RATE, min: 0, step: 0.01 },
    { default_value: 1000, field_type: "number", group: "第一阶段 TCP", key: "timeout_stage1_ms", label: "阶段 1 TCP 超时(ms)", min: 1, step: 1 },
  ];
  const traceFields: PipelineNodeCatalogField[] = [
    { default_value: "", field_type: "text", group: "第二阶段 追踪/COLO", help_text: "留空时从文件测速 URL 派生 /cdn-cgi/trace。", key: "trace_url", label: "追踪 URL", placeholder: "https://speed.cloudflare.com/cdn-cgi/trace" },
    {
      default_value: "standard",
      field_type: "select",
      group: "第二阶段 追踪/COLO",
      key: "trace_colo_mode",
      label: "第二阶段 COLO 获取模式",
      options: [
        { label: "标准", value: "standard" },
        { label: "追踪 URL", value: "trace_url" },
      ],
    },
    {
      default_value: "precheck",
      field_type: "select",
      group: "第二阶段 追踪/COLO",
      help_text: "国家/COLO 筛选词复用 Cloudflare COLO 字典派生链路。",
      key: "source_colo_filter_phase",
      label: "输入源 COLO 筛选阶段",
      options: [
        { label: "cloudflare-colos", value: "precheck" },
        { label: "第二阶段起效", value: "stage2" },
      ],
    },
    { default_value: 30, field_type: "number", group: "第二阶段 追踪/COLO", key: "concurrency_stage2", label: "追踪并发线程", max: 30, min: 1, step: 1 },
    { default_value: 1000, field_type: "number", group: "第二阶段 追踪/COLO", key: "timeout_stage2_ms", label: "追踪超时(ms)", min: 1, step: 1 },
    { default_value: DEFAULT_HTTPING_STATUS_CODE, field_type: "number", group: "第二阶段 追踪/COLO", help_text: "0 表示不限制；100-599 表示启用状态码筛选。", key: "httping_status_code", label: "追踪有效状态码", max: 599, min: 0, step: 1 },
    { field_type: "number", group: "第二阶段 追踪/COLO", key: "max_trace_latency_ms", label: "追踪延迟上限(ms)", min: 1, step: 1 },
    { default_value: "", field_type: "text", group: "第二阶段 追踪/COLO", help_text: "空列表不限制；可填写 JP,HKG,NRT,US,UK；国家码遵循 ISO 3166-1 alpha-2，UK 兼容为 GB。", key: "httping_cf_colo", label: "最终国家/COLO 筛选词", placeholder: "JP,HKG,NRT,US,UK" },
    {
      default_value: "allow",
      field_type: "select",
      group: "第二阶段 追踪/COLO",
      key: "httping_cf_colo_mode",
      label: "最终筛选方式",
      options: [
        { label: "白名单", value: "allow" },
        { label: "黑名单", value: "deny" },
      ],
    },
  ];
  const downloadFields: PipelineNodeCatalogField[] = [
    { default_value: DEFAULT_FILE_TEST_URL, field_type: "text", group: "第三阶段 下载", help_text: "文件测速阶段只访问该文件 URL；不要填写 /cdn-cgi/trace。", key: "url", label: "文件测速 URL" },
    { default_value: 10, field_type: "number", group: "第三阶段 下载", help_text: "限制完整模式进入文件测速的候选数。", key: "stage3_limit", label: "测速上限", min: 1, step: 1 },
    { default_value: 10, field_type: "number", group: "第三阶段 下载", key: "download_count", label: "下载测速数量", min: 1, step: 1 },
    { default_value: 0, field_type: "number", group: "第三阶段 下载", help_text: "0 不限制；正数按速度指标输出前 N 条。", key: "print_num", label: "结果显示数量", min: 0, step: 1 },
    { default_value: 1, field_type: "number", group: "第三阶段 下载", help_text: "文件测速阶段保持串行时维持 1。", key: "concurrency_stage3", label: "下载阶段并发", min: 1, step: 1 },
    { default_value: 4, field_type: "number", group: "第三阶段 下载", key: "download_get_concurrency", label: "单 IP GET 分片并发", max: 32, min: 1, step: 1 },
    { default_value: 10, field_type: "number", group: "第三阶段 下载", key: "download_time_seconds", label: "单 IP 下载测速时间(秒)", min: 1, step: 1 },
    { default_value: 10000, field_type: "number", group: "第三阶段 下载", key: "timeout_stage3_ms", label: "阶段 3 下载超时(ms)", min: 1, step: 1 },
    { default_value: 5, field_type: "number", group: "第三阶段 下载", key: "download_warmup_seconds", label: "下载预热时间(秒)", min: 0, step: 1 },
    { default_value: 500, field_type: "number", group: "第三阶段 下载", key: "download_speed_sample_interval_ms", label: "下载测速采样间隔(ms)", min: 1, step: 100 },
    { default_value: 256, field_type: "number", group: "第三阶段 下载", key: "download_buffer_kb", label: "下载缓冲(KiB)", max: 4096, min: 64, step: 64 },
    {
      default_value: "auto",
      field_type: "select",
      group: "第三阶段 下载",
      key: "download_http_protocol",
      label: "下载 HTTP 协议",
      options: [
        { label: "Auto", value: "auto" },
        { label: "H1.1", value: "h1" },
        { label: "H2", value: "h2" },
        { label: "H3", value: "h3" },
      ],
    },
    {
      default_value: "average",
      field_type: "select",
      group: "第三阶段 下载",
      key: "download_speed_metric",
      label: "下载速率依据",
      options: [
        { label: "平均速率", value: "average" },
        { label: "最高速率", value: "max" },
      ],
    },
    { default_value: 0, field_type: "number", group: "第三阶段 下载", key: "min_download_mbps", label: "最低下载速度(MB/s)", min: 0, step: 0.1 },
  ];
  if (primaryStage === "probe_trace") {
    return traceFields;
  }
  if (primaryStage === "probe_download") {
    return downloadFields;
  }
  return tcpFields;
}

export function defaultPipelineNodeCatalog(): PipelineNodeCatalogItem[] {
  const probeDefaultConfig = pipelineProbeFullModeDefaultConfig();
  return [
    normalizePipelineNodeCatalogItem({
      action: "select_sources",
      default_config: {
        source_ids: [],
        source_profile_id: "",
        source_selection: "enabled",
      },
      description: "从当前绑定配置或指定输入组档案中勾选输入源，作为后续测速的输入组。",
      display_name: "输入源组",
      form_schema: [
        {
          default_value: "",
          field_type: "select",
          group: "输入组",
          help_text: "留空使用当前工作流绑定配置；前端会把已有输入组档案作为选项注入。",
          key: "source_profile_id",
          label: "输入组档案",
          options: [{ label: "当前绑定配置", value: "" }],
        },
        {
          default_value: "enabled",
          field_type: "select",
          group: "输入组",
          help_text: "全部启用表示使用所选输入组中 enabled=true 的输入源；自定义勾选只使用 source_ids。",
          key: "source_selection",
          label: "选择方式",
          options: [
            { label: "全部启用输入源", value: "enabled" },
            { label: "自定义勾选", value: "custom" },
          ],
        },
      ],
      node_type: "source",
      outcomes: [],
    }),
    normalizePipelineNodeCatalogItem({
      action: "filter_sources",
      default_config: {
        source_colo_filter: "",
        source_colo_filter_mode: "allow",
        source_ip_limit: 500,
        source_ip_mode: "traverse",
      },
      description: "对上游输入源组批量覆盖 IP 上限、抽样模式和国家/COLO 筛选词，再输出新的输入源组。",
      display_name: "输入源筛选",
      form_schema: [
        {
          default_value: 500,
          field_type: "number",
          group: "输入源筛选",
          help_text: "批量覆盖每个输入源的候选 IP 上限；实际语义沿用输入源处理链路。",
          key: "source_ip_limit",
          label: "总测试 IP 上限",
          min: 1,
          step: 1,
        },
        {
          default_value: "traverse",
          field_type: "select",
          group: "输入源筛选",
          help_text: "遍历直接读取候选；MCIS 抽样复用现有输入源 MCIS 处理。",
          key: "source_ip_mode",
          label: "IP 获取模式",
          options: [
            { label: "遍历", value: "traverse" },
            { label: "MCIS 抽样", value: "mcis" },
          ],
        },
        {
          default_value: "",
          field_type: "textarea",
          group: "国家/COLO 筛选",
          help_text: "复用现有 COLO 词典筛选链路；国家码遵循 ISO 3166-1 alpha-2，UK 兼容为 GB，国家筛选需依赖 Cloudflare COLO 字典派生。",
          key: "source_colo_filter",
          label: "国家/COLO 筛选词",
          placeholder: "JP,HKG,NRT,US,UK",
          rows: 3,
        },
        {
          default_value: "allow",
          field_type: "select",
          group: "国家/COLO 筛选",
          key: "source_colo_filter_mode",
          label: "筛选方式",
          options: [
            { label: "白名单", value: "allow" },
            { label: "黑名单", value: "deny" },
          ],
        },
      ],
      node_type: "source",
      outcomes: [],
    }),
    normalizePipelineNodeCatalogItem({
      action: "probe_tcp",
      default_config: { ...probeDefaultConfig },
      description: "第一阶段：执行 TCP 延迟测速，输出可继续追踪的候选节点。",
      display_name: "TCP 延迟测速",
      form_schema: pipelineProbeFullModeFormSchema("probe_tcp"),
      node_type: "probe",
      outcomes: [],
    }),
    normalizePipelineNodeCatalogItem({
      action: "probe_trace",
      default_config: { ...probeDefaultConfig },
      description: "第二阶段：复用现有追踪/COLO 检查链路，输出可下载测速的候选节点。",
      display_name: "追踪测试",
      form_schema: pipelineProbeFullModeFormSchema("probe_trace"),
      node_type: "probe",
      outcomes: [],
    }),
    normalizePipelineNodeCatalogItem({
      action: "probe_download",
      default_config: { ...probeDefaultConfig },
      description: "第三阶段：执行下载测速，按速度指标排序并产出最终 probe_results。",
      display_name: "下载测速",
      form_schema: pipelineProbeFullModeFormSchema("probe_download"),
      node_type: "probe",
      outcomes: [],
    }),
    normalizePipelineNodeCatalogItem({
      action: "filter_results",
      default_config: { source: "probe_results", status: "passed" },
      description: "按共享上传规则筛选结果。",
      display_name: "结果筛选",
      form_schema: [
        {
          default_value: "probe_results",
          field_type: "select",
          group: "数据来源",
          help_text: "通常保持默认。只有想继续处理上一步筛选后的结果时，才改成“已筛选结果”。",
          key: "source",
          label: "筛选输入",
          options: [
            { label: "测速结果", value: "probe_results" },
            { label: "已筛选结果", value: "filtered_rows" },
          ],
        },
        {
          default_value: "passed",
          field_type: "select",
          group: "筛选条件",
          key: "status",
          label: "结果状态",
          options: [
            { label: "仅成功结果", value: "passed" },
            { label: "全部结果", value: "all" },
          ],
        },
        {
          default_value: "any",
          field_type: "select",
          group: "筛选条件",
          key: "ip_version",
          label: "IP 版本",
          options: [
            { label: "全部", value: "any" },
            { label: "仅 IPv4", value: "ipv4" },
            { label: "仅 IPv6", value: "ipv6" },
          ],
        },
        {
          field_type: "number",
          group: "筛选条件",
          key: "max_loss_rate",
          label: "最大丢包率",
          max: 1,
          min: 0,
          step: 0.01,
        },
        {
          field_type: "number",
          group: "筛选条件",
          key: "max_tcp_latency_ms",
          label: "最大 TCP 延迟(ms)",
          min: 1,
          step: 1,
        },
        {
          field_type: "number",
          group: "筛选条件",
          key: "max_trace_latency_ms",
          label: "最大追踪延迟(ms)",
          min: 1,
          step: 1,
        },
        {
          default_value: 0,
          field_type: "number",
          group: "筛选条件",
          key: "min_download_mbps",
          label: "最小下载速度(MB/s)",
          min: 0,
          step: 0.1,
        },
        {
          default_value: "",
          field_type: "textarea",
          group: "筛选条件",
          key: "colo_allow",
          label: "仅允许的国家/COLO",
          placeholder: "JP,HKG,NRT,US,UK",
          rows: 3,
        },
        {
          default_value: "",
          field_type: "textarea",
          group: "筛选条件",
          key: "colo_deny",
          label: "排除的国家/COLO",
          placeholder: "JP,HKG,NRT,US,UK",
          rows: 3,
        },
        {
          default_value: 0,
          field_type: "number",
          group: "筛选条件",
          help_text: "大于 0 时，只保留排序后的前 N 条结果继续向下游传递，并影响所有后续投递节点。",
          key: "top_n",
          label: "保留前 N 条",
          min: 0,
          step: 1,
        },
      ],
      node_type: "filter",
      outcomes: [],
    }),
    normalizePipelineNodeCatalogItem({
      action: "branch_has_results",
      default_config: { source: "filtered_rows" },
      description: "检查结果是否为空，并按 outcome 选择下一条边。",
      display_name: "结果检查",
      form_schema: [
        {
          default_value: "filtered_rows",
          field_type: "select",
          group: "数据来源",
          help_text: "一般选“筛选结果”。如果你想直接拿测速结果做判断，再切到“测速结果”。",
          key: "source",
          label: "检查输入",
          options: [
            { label: "筛选结果", value: "filtered_rows" },
            { label: "测速结果", value: "probe_results" },
          ],
        },
      ],
      node_type: "branch",
      outcomes: [
        { description: "存在可继续处理的结果。", label: "有结果", value: "true" },
        { description: "当前没有可继续处理的结果。", label: "无结果", value: "false" },
      ],
    }),
    normalizePipelineNodeCatalogItem({
      action: "deliver_dns",
      default_config: { source: "filtered_rows" },
      description: "把结果推送到 Cloudflare DNS。",
      display_name: "DNS 推送",
      form_schema: [
        {
          default_value: "filtered_rows",
          field_type: "select",
          group: "数据来源",
          key: "source",
          label: "推送输入",
          options: [
            { label: "筛选结果", value: "filtered_rows" },
            { label: "测速结果", value: "probe_results" },
          ],
        },
        {
          default_value: 0,
          field_type: "number",
          group: "推送行为",
          help_text: "只限制本 DNS 推送节点；留 0 时沿用上传配置或上游筛选结果。",
          key: "top_n",
          label: "推送前 N 条",
          min: 0,
          step: 1,
        },
        {
          field_type: "text",
          group: "DNS 记录",
          help_text: "留空时继承工作流绑定配置里的记录名。",
          key: "record_name",
          label: "记录名",
          placeholder: "sub.example.com",
        },
        {
          default_value: "A",
          field_type: "select",
          group: "DNS 记录",
          key: "record_type",
          label: "记录类型",
          options: [
            { label: "ALL (A + AAAA)", value: "ALL" },
            { label: "A (IPv4)", value: "A" },
            { label: "AAAA (IPv6)", value: "AAAA" },
          ],
        },
        {
          default_value: 300,
          field_type: "number",
          group: "DNS 记录",
          key: "ttl",
          label: "TTL",
          min: 1,
          step: 1,
        },
        {
          field_type: "text",
          group: "DNS 记录",
          key: "comment",
          label: "注释",
          placeholder: "可选，留空则沿用绑定配置。",
        },
      ],
      node_type: "deliver",
      outcomes: [],
    }),
    normalizePipelineNodeCatalogItem({
      action: "deliver_github",
      default_config: { source: "filtered_rows" },
      description: "把结果导出到 GitHub。",
      display_name: "GitHub 导出",
      form_schema: [
        {
          default_value: "filtered_rows",
          field_type: "select",
          group: "数据来源",
          key: "source",
          label: "导出输入",
          options: [
            { label: "筛选结果", value: "filtered_rows" },
            { label: "测速结果", value: "probe_results" },
          ],
        },
        {
          default_value: 0,
          field_type: "number",
          group: "导出行为",
          help_text: "只限制本 GitHub 导出节点；留 0 时沿用上传配置或上游筛选结果。",
          key: "top_n",
          label: "导出前 N 条",
          min: 0,
          step: 1,
        },
      ],
      node_type: "deliver",
      outcomes: [],
    }),
    normalizePipelineNodeCatalogItem({
      action: "recovery_mark",
      default_config: { message: "需要人工复核。", status: "manual_review" },
      description: "记录恢复/回退原因。",
      display_name: "人工复核标记",
      form_schema: [
        {
          default_value: "manual_review",
          field_type: "select",
          group: "结果状态",
          help_text: "这里决定这一步对外显示成什么状态。",
          key: "status",
          label: "标记状态",
          options: [
            { label: "人工复核", value: "manual_review" },
            { label: "已跳过", value: "skipped" },
            { label: "失败", value: "failed" },
          ],
        },
        {
          default_value: "需要人工复核。",
          field_type: "textarea",
          group: "说明",
          key: "message",
          label: "说明",
          placeholder: "说明为什么需要人工复核。",
          rows: 4,
        },
      ],
      node_type: "recovery",
      outcomes: [],
    }),
    normalizePipelineNodeCatalogItem({
      action: "end",
      default_config: { message: "流程已结束。", status: "completed" },
      description: "声明当前路径的最终状态。",
      display_name: "结束",
      form_schema: [
        {
          default_value: "completed",
          field_type: "select",
          group: "结果状态",
          help_text: "这里决定流程最后在运行记录里显示成完成、失败还是需要手动处理。",
          key: "status",
          label: "最终状态",
          options: [
            { label: "完成", value: "completed" },
            { label: "人工复核", value: "manual_review" },
            { label: "已跳过", value: "skipped" },
            { label: "失败", value: "failed" },
            { label: "部分完成", value: "partial" },
          ],
        },
        {
          default_value: "流程已结束。",
          field_type: "textarea",
          group: "说明",
          key: "message",
          label: "结束说明",
          placeholder: "展示给运行结果区的说明。",
          rows: 4,
        },
      ],
      node_type: "end",
      outcomes: [],
    }),
    normalizePipelineNodeCatalogItem({
      action: "check_output",
      default_config: { export_if_missing: true, require_csv: true, source: "probe_results", status: "passed", top_n: 0 },
      description: "检查测速结果与 CSV 写入状态，必要时补写结果。",
      display_name: "结果检查与输出",
      form_schema: [
        {
          default_value: "probe_results",
          field_type: "select",
          group: "结果检查",
          key: "source",
          label: "检查输入",
          options: [
            { label: "测速结果", value: "probe_results" },
            { label: "已筛选结果", value: "filtered_rows" },
          ],
        },
        {
          default_value: "passed",
          field_type: "select",
          group: "结果检查",
          key: "status",
          label: "结果状态",
          options: [
            { label: "仅成功结果", value: "passed" },
            { label: "全部结果", value: "all" },
          ],
        },
        {
          default_value: 0,
          field_type: "number",
          group: "结果检查",
          help_text: "大于 0 时仅检查排序后的前 N 条，并影响补写 CSV 的输入。",
          key: "top_n",
          label: "检查前 N 条",
          min: 0,
          step: 1,
        },
        {
          default_value: true,
          field_type: "checkbox",
          group: "CSV 输出",
          key: "require_csv",
          label: "要求 CSV 写入",
        },
        {
          default_value: true,
          field_type: "checkbox",
          group: "CSV 输出",
          help_text: "CSV 缺失且存在结果时，按工作流绑定配置里的导出路径补写。",
          key: "export_if_missing",
          label: "缺失时补写 CSV",
        },
      ],
      node_type: "deliver",
      outcomes: [],
    }),
  ];
}

function normalizePipelineProfile(input: unknown, index: number): PipelineProfile {
  const source = isObject(input) ? input : {};
  const target = normalizePipelineTarget(source, index);

  return {
    config_snapshot: target.config_snapshot,
    created_at: target.created_at,
    dns_push_policy: target.dns_push_policy,
    domain: target.domain,
    enabled: target.enabled,
    id: target.id,
    name: target.name,
    region: target.region,
    updated_at: target.updated_at,
  };
}

export function normalizePipelineProfileStore(input: unknown): PipelineProfileStore {
  const source = isObject(input) ? input : {};
  if (Array.isArray(source.targets) || Array.isArray(source.templates)) {
    return pipelineProfileStoreFromWorkspace(normalizePipelineWorkspace(input));
  }
  const items = Array.isArray(source.items) ? source.items : [];

  return {
    active_profile_id: toStringValue(source.active_profile_id ?? source.activeProfileId),
    items: items.map((entry, index) => normalizePipelineProfile(entry, index)),
    schema_version: toStringValue(source.schema_version ?? source.schemaVersion),
    updated_at: toStringValue(source.updated_at ?? source.updatedAt),
  };
}

export function pipelineProfileStoreFromWorkspace(workspace: PipelineWorkspace): PipelineProfileStore {
  return {
    active_profile_id: workspace.active_target_id,
    items: workspace.targets.map((entry, index) => normalizePipelineProfile(entry, index)),
    schema_version: workspace.schema_version,
    updated_at: workspace.updated_at,
  };
}
