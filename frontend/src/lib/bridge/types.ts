export type TaskTone = "idle" | "preparing" | "running" | "partial" | "cooling" | "warning" | "completed" | "no_results" | "failed";

export interface CommandResult<T = Record<string, unknown> | null> {
  code: string;
  data: T | null;
  message: string;
  ok: boolean;
  schema_version: string;
  task_id: string | null;
  warnings: string[];
}

export interface ProbeNumericTriple {
  stage1: number;
  stage2: number;
  stage3: number;
}

export interface ProbeStageLimits {
  stage1?: number;
  stage2?: number;
  stage3: number;
}

export interface ProbeTimeouts {
  stage1_ms: number;
  stage2_ms: number;
  stage3_ms: number;
}

export interface ProbeThresholds {
  max_http_latency_ms: number | null;
  max_tcp_latency_ms: number | null;
  min_download_mbps: number;
}

export type ProbeStrategy = "fast" | "full";
export type SourceColoFilterPhase = "precheck" | "stage2";
export type ColoFilterMode = "allow" | "deny";
export type TraceColoMode = "standard" | "trace_url";
export type DebugLogMode = "structured" | "freeform";
export type DebugLogVerbosity = "simple" | "detailed";
export type DownloadHTTPProtocol = "auto" | "h1" | "h2" | "h3";
export type DownloadSpeedMetric = "average" | "max";
export type CSVEncoding = "utf-8" | "utf-8-bom";
export type SourceKind = "inline" | "file" | "url";
export type SourceIPMode = "traverse" | "mcis";
export type ThemeMode = "light" | "dark" | "auto_system_time" | "auto_time";

export interface DesktopSourceConfig {
  colo_filter: string;
  colo_filter_mode: ColoFilterMode;
  content: string;
  enabled: boolean;
  id: string;
  ip_limit: number;
  ip_mode: SourceIPMode;
  kind: SourceKind;
  last_fetched_at: string;
  last_fetched_count: number;
  name: string;
  path: string;
  status_text: string;
  url: string;
}

export interface SourcePreviewSummary {
  action: string;
  invalid_count: number;
  mode: SourceIPMode;
  name: string;
  total_count: number;
}

export interface SourcePreviewPayload {
  port_summary?: Record<string, unknown> | null;
  preview_entries: string[];
  source_status: Partial<DesktopSourceConfig> | null;
  summary: SourcePreviewSummary | null;
}

export interface ColoDictionaryStatus {
  colo_ipv4_path: string;
  colo_ipv4_rows: number;
  colo_ipv6_path: string;
  colo_ipv6_rows: number;
  colo_path: string;
  colo_rows: number;
  geofeed_path: string;
  geofeed_rows: number;
  last_updated_at: string;
  matched_rows: number;
  missing_rows: number;
  source_url: string;
  updated: boolean;
  unmatched_rows: number;
}

export interface PathSelectionPayload {
  androidExportUri?: string;
  canceled?: boolean;
  content?: string;
  content_base64?: string;
  directory?: string;
  display_name?: string;
  file_name?: string;
  mode?: string;
  path?: string;
  storage_uri?: string;
  target_uri?: string;
  uri?: string;
}

export interface StorageHealth {
  checked_at: string;
  exists: boolean;
  free_bytes: number;
  is_dir: boolean;
  message: string;
  path: string;
  portable_mode: boolean;
  writable: boolean;
}

export interface StorageStatus {
  backend?: "private";
  bootstrap_path: string;
  current_dir: string;
  default_dir: string;
  display_name?: string;
  health?: StorageHealth;
  last_sync_at?: string;
  last_sync_error?: string;
  log_uri?: string;
  permission_ok?: boolean;
  portable_mode: boolean;
  runtime_dir?: string;
  setup_completed: boolean;
  setup_required: boolean;
  storage_uri?: string;
  writable: boolean;
}

export interface AndroidBatteryStatus {
  brand: string;
  ignoring_optimizations: boolean;
  manufacturer: string;
  model: string;
  needs_guidance: boolean;
  settings_hint: string;
  supported: boolean;
}

export interface AndroidNotificationPermissionStatus {
  can_request: boolean;
  granted: boolean;
  message: string;
  open_settings_recommended: boolean;
  request_already_attempted: boolean;
  should_show_rationale: boolean;
  state: string;
  supported: boolean;
}

export interface AndroidKeepAliveStatus {
  enabled: boolean;
  message: string;
  notification_permission_granted: boolean;
  running: boolean;
  supported: boolean;
}

export interface AndroidRuntimeStatus {
  battery?: AndroidBatteryStatus | null;
  foreground_service_running: boolean;
  has_task_snapshot: boolean;
  keep_alive?: AndroidKeepAliveStatus | null;
  resume_capable: boolean;
  runtime?: RuntimeDiagnostics | null;
  runtime_attached: boolean;
  session_state: string;
  task_id: string;
  task_snapshot?: TaskSnapshot | null;
}

export interface RuntimeDiagnostics {
  cleanup_count?: number;
  diagnostics_enabled: boolean;
  goroutines?: number;
  heap_alloc_bytes?: number;
  heap_inuse_bytes?: number;
  heap_sys_bytes?: number;
  heavy_cleanup_count?: number;
  last_cleanup_at?: string;
  last_cleanup_reason?: string;
  last_heavy_cleanup_at?: string;
  last_skipped_heavy_at?: string;
  last_skipped_heavy_reason?: string;
  memory_sys_bytes?: number;
  pipeline_results?: number;
  task_snapshots?: number;
}

export interface TraceDiagnosticSample {
  colo?: string;
  error?: string;
  ip?: string;
  reason: string;
  retry_after_ms?: number;
  status_code?: number;
  url?: string;
}

export interface TraceDiagnostics {
  reason_counts?: Record<string, number>;
  samples?: TraceDiagnosticSample[];
  status_counts?: Record<string, number>;
  trace_colo_mode?: string;
  trace_url?: string;
}

export interface AppInfo {
  current_version: string;
  install_mode: string;
  platform: string;
  release_url: string;
}

export interface UpdateInfo extends AppInfo {
  asset_name: string;
  download_url: string;
  docker_image: string;
  latest_version: string;
  release_name: string;
  sha256: string;
  update_available: boolean;
}

export interface UpdateInstallResult extends UpdateInfo {
  downloaded_path: string;
  install_started: boolean;
  next_action: string;
}

export type PipelineDNSPushPolicy = "auto" | "skip";
export type SchedulerRunMode = "probe" | "pipeline";
export type PipelineNodeFieldType = "text" | "textarea" | "select" | "checkbox" | "number" | "json";

export interface PipelineProfile {
  config_snapshot: ConfigSnapshot;
  created_at: string;
  dns_push_policy: PipelineDNSPushPolicy;
  domain: string;
  enabled: boolean;
  id: string;
  name: string;
  region: string;
  updated_at: string;
}

export interface PipelineProfileStore {
  active_profile_id: string;
  items: PipelineProfile[];
  schema_version: string;
  updated_at: string;
}

export type PipelineNodeType = "source" | "probe" | "filter" | "branch" | "deliver" | "recovery" | "end";

export interface PipelineNodeCatalogFieldOption {
  label: string;
  value: string;
}

export interface PipelineNodeCatalogFieldVisibleWhen {
  equals?: unknown;
  field: string;
  not_equals?: unknown;
}

export interface PipelineNodeCatalogField {
  default_value?: unknown;
  description?: string;
  field_type: PipelineNodeFieldType;
  group?: string;
  help_text?: string;
  key: string;
  label: string;
  max?: number;
  min?: number;
  options?: PipelineNodeCatalogFieldOption[];
  placeholder?: string;
  required?: boolean;
  rows?: number;
  step?: number;
  visible_when?: PipelineNodeCatalogFieldVisibleWhen;
}

export interface PipelineNodeCatalogOutcome {
  description?: string;
  label: string;
  value: string;
}

export interface PipelineNodeCatalogItem {
  action: string;
  default_config: Record<string, unknown>;
  description?: string;
  display_name: string;
  form_schema: PipelineNodeCatalogField[];
  node_type: PipelineNodeType;
  outcomes: PipelineNodeCatalogOutcome[];
}

export interface PipelineCanvasPosition {
  x: number;
  y: number;
}

export interface PipelineViewport {
  x: number;
  y: number;
  zoom: number;
}

export interface PipelineNodeUI {
  collapsed?: boolean;
  position?: PipelineCanvasPosition;
  width?: number;
}

export interface PipelineTemplateUI {
  viewport?: PipelineViewport;
}

export interface PipelineNode {
  action: string;
  config: Record<string, unknown>;
  id: string;
  name: string;
  node_type: PipelineNodeType;
  ui?: PipelineNodeUI;
  updated_at: string;
}

export interface PipelineEdge {
  id: string;
  label: string;
  outcome: string;
  source_node_id: string;
  target_node_id: string;
}

export interface PipelineTemplate {
  bound_config_snapshot: ConfigSnapshot;
  created_at: string;
  description: string;
  enabled: boolean;
  entry_node_id: string;
  edges: PipelineEdge[];
  id: string;
  name: string;
  nodes: PipelineNode[];
  ui?: PipelineTemplateUI;
  updated_at: string;
  version: number;
}

export interface PipelineTarget {
  config_snapshot: ConfigSnapshot;
  created_at: string;
  dns_push_policy: PipelineDNSPushPolicy;
  domain: string;
  enabled: boolean;
  id: string;
  name: string;
  region: string;
  tags: string[];
  template_id: string;
  updated_at: string;
}

export interface PipelineWorkspace {
  active_target_id: string;
  active_template_id: string;
  schema_version: string;
  targets: PipelineTarget[];
  templates: PipelineTemplate[];
  updated_at: string;
}

export interface SourceProfileItem {
  created_at: string;
  id: string;
  name: string;
  sources: DesktopSourceConfig[];
  updated_at: string;
}

export interface SourceProfileStore {
  active_profile_id: string;
  items: SourceProfileItem[];
  schema_version: string;
  updated_at: string;
}

export interface SourceProfileUpdatePayload {
  config_snapshot?: ConfigSnapshot;
  source_profiles: SourceProfileStore;
  sources: DesktopSourceConfig[];
}

export interface PipelineProfileRunResult {
  dns_result?: unknown;
  domain: string;
  message: string;
  node_results?: PipelineNodeRunResult[];
  profile_id: string;
  profile_name: string;
  probe_result?: ProbeRunResultPayload | null;
  region: string;
  status: string;
  task_id: string;
  target_id?: string;
  target_name?: string;
  warnings?: string[];
}

export interface PipelineNodeRunResult {
  action: string;
  branch_taken: string;
  completed_at: string;
  message: string;
  metrics: Record<string, unknown> | null;
  node_id: string;
  node_name: string;
  node_type: PipelineNodeType;
  outcome: string;
  output_summary: string;
  started_at: string;
  status: string;
}

export interface PipelineRunResult {
  completed_at: string;
  duration_ms: number;
  failed: number;
  pipeline_id: string;
  results: PipelineProfileRunResult[];
  skipped: number;
  started_at: string;
  status: string;
  succeeded: number;
  task_id: string;
  target_ids: string[];
  target_results: PipelineProfileRunResult[];
  template_id: string;
  total: number;
  warnings: string[];
}

export interface CloudflareRoutingRuleSnapshot {
  enabled: boolean;
  filter_mode: "allow" | "deny";
  filter_tokens: string;
  name: string;
  record_name: string;
  record_type: "A" | "AAAA" | "ALL";
  top_n: number;
}

export interface GitHubConfigSnapshot {
  branch: string;
  csv_header_template?: string;
  csv_row_template?: string;
  commit_message_template: string;
  enabled: boolean;
  format?: "csv" | "txt" | string;
  last_export_at: string;
  owner: string;
  path_template: string;
  repo: string;
  token: string;
  top_n?: number;
  txt_row_template?: string;
}

export interface ConfigSnapshot {
  backup: {
    webdav: {
      enabled: boolean;
      last_backup_at: string;
      last_restore_at: string;
      password: string;
      remote_path: string;
      server_url: string;
      timeout_seconds: number;
      username: string;
    };
  };
  cloudflare: {
    api_token: string;
    comment: string;
    enabled: boolean;
    proxied: boolean;
    record_name: string;
    record_type?: "A" | "AAAA" | "ALL";
    routing_enabled: boolean;
    routing_rules: CloudflareRoutingRuleSnapshot[];
    top_n: number;
    ttl: number;
    zone_id: string;
  };
  github: GitHubConfigSnapshot;
  maintenance: {
    completed_task_retention_days: number;
  };
  notifications: {
    telegram: {
      bot_token: string;
      chat_id: string;
      enabled: boolean;
    };
  };
  post_probe_push: {
    cloudflare_enabled: boolean;
    github_enabled: boolean;
  };
  upload: {
    cloudflare: {
      routing_enabled: boolean;
      routing_rules: CloudflareRoutingRuleSnapshot[];
      top_n: number;
    };
    github: {
      top_n: number;
    };
    shared_filter: {
      colo_allow: string;
      colo_deny: string;
      enabled: boolean;
      ip_version: "any" | "ipv4" | "ipv6";
      max_loss_rate: number | null;
      max_tcp_latency_ms: number | null;
      max_trace_latency_ms: number | null;
      min_download_mbps: number;
      status: "all" | "passed";
    };
  };
  export: {
    csv_encoding: CSVEncoding;
    file_name?: string;
    file_name_template?: string;
    format?: string;
    github: GitHubConfigSnapshot;
    overwrite?: string;
    target_dir: string;
    target_uri?: string;
  };
  probe: {
    concurrency: ProbeNumericTriple;
    cooldown_policy: {
      consecutive_failures: number;
      cooldown_ms: number;
    };
    debug: boolean;
    debug_capture_address: string;
    debug_capture_enabled: boolean;
    debug_log_format: string;
    debug_log_mode: DebugLogMode;
    debug_log_verbosity: DebugLogVerbosity;
    disable_download: boolean;
    download_buffer_kb: number;
    download_count: number;
    download_get_concurrency: number;
    download_http_protocol: DownloadHTTPProtocol;
    download_speed_metric: DownloadSpeedMetric;
    download_speed_sample_interval_ms: number;
    download_speed_sample_interval_seconds: number;
    download_time_seconds: number;
    download_warmup_seconds: number;
    event_throttle_ms: number;
    host_header: string;
    httping: boolean;
    httping_cf_colo: string;
    httping_cf_colo_mode: ColoFilterMode;
    httping_status_code: number;
    max_loss_rate: number;
    min_delay_ms: number;
    ping_times: number;
    print_num: number;
    retry_policy: {
      backoff_ms: number;
      max_attempts: number;
    };
    request_headers: string;
    skip_first_latency_sample: boolean;
    source_colo_filter_phase: SourceColoFilterPhase;
    stage_limits: ProbeStageLimits;
    port_policy: string;
    strategy: ProbeStrategy;
    sni: string;
    tcp_port: number;
    test_all: boolean;
    thresholds: ProbeThresholds;
    timeouts: ProbeTimeouts;
    trace_colo_mode: TraceColoMode;
    trace_url: string;
    url: string;
    user_agent: string;
  };
  sources: DesktopSourceConfig[];
  scheduler: {
    auto_dns_push: boolean;
    auto_github_export: boolean;
    config_source: string;
    daily_times: string[];
    enabled: boolean;
    interval_minutes: number;
    pipeline_template_id: string;
    post_run_source_profile_action: string;
    run_mode: SchedulerRunMode;
    skip_if_active: boolean;
  };
  ui: {
    auto_detect_source_name: boolean;
    theme_dark_start: string;
    theme_light_start: string;
    theme_mode: ThemeMode;
    utc_offset_minutes: number;
  };
}

export interface UploadNotification {
  cloudflare_status?: string;
  cloudflare_upload_count?: number;
  created_at: string;
  github_status?: string;
  github_upload_count?: number;
  message: string;
  source: string;
  status: string;
  task_id?: string;
}

export interface ProbeEventEnvelope {
  event: string;
  payload: Record<string, unknown>;
  schema_version: string;
  seq: number;
  task_id: string;
  ts: string;
}

export interface DnsRecordSnapshot {
  comment: string;
  content: string;
  created_on?: string;
  id: string;
  modified_on?: string;
  name: string;
  proxied: boolean;
  ttl: number;
  type: string;
}

export interface DerivedTaskState {
  detail: string;
  title: string;
  tone: TaskTone;
}

export interface TaskProgress {
  failed: number;
  passed: number;
  processed: number;
  stage: string;
  total?: number | null;
}

export interface ExportRecord {
  file_name: string;
  format: string;
  last_write_at?: string | null;
  source_path?: string | null;
  target_dir: string;
  task_id: string;
  written_count: number;
}

export interface TaskSnapshot {
  completed_at?: string | null;
  config_digest?: string | null;
  current_stage?: string | null;
  export_record?: ExportRecord | null;
  failure_summary?: Record<string, unknown> | null;
  progress?: TaskProgress | null;
  resume_capable?: boolean | null;
  runtime_attached?: boolean | null;
  session_state?: string | null;
  started_at?: string | null;
  status: string;
  task_context?: Record<string, unknown> | null;
  task_id: string;
  updated_at: string;
}

export interface TaskResultPage {
  count: number;
  results: ProbeResult[];
  source_kind?: string | null;
  source_path?: string | null;
  total_count?: number | null;
}

export interface ProbeResult {
  address: string;
  colo?: string | null;
  download_mbps?: number | null;
  export_status: string;
  last_error_code?: string | null;
  max_download_mbps?: number | null;
  source_port?: number | null;
  stage_status: string;
  tcp_latency_ms?: number | null;
  test_port?: number | null;
  trace_latency_ms?: number | null;
}

export interface SchedulerStatus {
  config_source?: string;
  cloudflare_upload_count?: number;
  enabled: boolean;
  last_dns_status: string;
  last_github_status: string;
  last_message: string;
  last_probe_status: string;
  last_run_at: string;
  last_source_profile_action?: string;
  last_task_id: string;
  next_run_at: string;
  run_mode?: SchedulerRunMode;
  github_upload_count?: number;
  upload_notification?: UploadNotification | null;
  upload_filtered_count?: number;
  upload_input_count?: number;
  workflow_stage?: string;
}

export interface ProbeRunResultPayload extends Record<string, unknown> {
  outputFile?: unknown;
  results?: unknown;
  source?: unknown;
  sourceStatuses?: unknown;
  startedAt?: unknown;
  summary?: unknown;
  task_context?: unknown;
  taskContext?: unknown;
  upload_notification?: UploadNotification | null;
  warnings?: unknown;
}

export type ProbeResultFilter = "all" | "exported" | "pending" | "failed";
export type ProbeResultIPFilter = "all" | "ipv4" | "ipv6";
export type ProbeResultOrder = "asc" | "desc";
export type ProbeResultSortBy = "address" | "stage" | "tcp" | "trace" | "download" | "max_download" | "export_status";
