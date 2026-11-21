import type { CloudflareRoutingRuleSnapshot, DNSProviderSnapshot } from "./dns";
import type { CSVEncoding, DebugLogMode, DebugLogVerbosity, DownloadHTTPProtocol, DownloadSpeedMetric, ProbeStrategy, SourceColoFilterPhase, SourceIPMode, TelegramRecipientMode, ThemeMode, TraceColoMode } from "./enums";
import type { ProbeNumericTriple, ProbeStageLimits, ProbeThresholds, ProbeTimeouts } from "./probe";
import type { SourceConfig } from "./sources";
import type { UploadNotification } from "./task";

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

export interface SourceProfileItem {
  colo_filter: string;
  colo_filter_mode: "allow" | "deny" | "";
  created_at: string;
  id: string;
  ip_mode: SourceIPMode | "";
  name: string;
  sources: SourceConfig[];
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
  sources: SourceConfig[];
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
  dns_providers: DNSProviderSnapshot[];
  github: GitHubConfigSnapshot;
  maintenance: {
    completed_task_retention_days: number;
  };
  notifications: {
    telegram: {
      bot_token: string;
      chat_id: string;
      enabled: boolean;
      include_top_n: boolean;
      personal_chat_id: string;
      recipient_mode: TelegramRecipientMode;
      top_n: number;
      top_n_recipient_mode: TelegramRecipientMode;
      upload_recipient_mode: TelegramRecipientMode;
      use_system_proxy: boolean;
    };
    webhook: {
      enabled: boolean;
      urls: string[];
      dingtalk_url: string;
      dingtalk_secret: string;
      wecom_urls: string[];
      wecom_corp_id: string;
      wecom_agent_id: string;
      wecom_secret: string;
      wecom_mobiles: string[];
      headers: Record<string, string>;
      use_system_proxy: boolean;
    };
    email: { enabled: boolean; host: string; port: number; username: string; password: string; from: string; to: string[]; use_tls: boolean };
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
    mcis: {
      budget: number;
      concurrency: number;
    };
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
    download_success_limit: number;
    download_get_concurrency: number;
    download_host_header: string;
    download_http_protocol: DownloadHTTPProtocol;
    download_sni: string;
    download_speed_metric: DownloadSpeedMetric;
    download_speed_sample_interval_ms: number;
    download_time_seconds: number;
    download_warmup_seconds: number;
    event_throttle_ms: number;
    host_header: string;
    trace_cf_colo: string;
    trace_cf_colo_mode: "allow" | "deny";
    trace_status_code: number;
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
    source_pool_limit: number;
    stage_limits: ProbeStageLimits;
    port_policy: string;
    strategy: ProbeStrategy;
    sni: string;
    verify_tls_certificate: boolean;
    tcp_port: number;
    test_all: boolean;
    thresholds: ProbeThresholds;
    timeouts: ProbeTimeouts;
    trace_colo_mode: TraceColoMode;
    trace_url: string;
    doh_url: string;
    ech: boolean;
    ech_config: string;
    ech_domain: string;
    url: string;
    user_agent: string;
  };
  source_ip_mode: SourceIPMode | "";
  sources: SourceConfig[];
  scheduler: {
    auto_dns_push: boolean;
    auto_github_export: boolean;
    config_source: string;
    daily_times: string[];
    enabled: boolean;
    interval_minutes: number;
    post_run_source_profile_action: string;
    skip_if_active: boolean;
  };
  ui: {
    auto_detect_source_name: boolean;
    settings_mode: "basic" | "advanced";
    theme_dark_start: string;
    theme_light_start: string;
    theme_mode: ThemeMode;
    utc_offset_minutes: number;
  };
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
  github_upload_count?: number;
  upload_notification?: UploadNotification | null;
  upload_filtered_count?: number;
  upload_input_count?: number;
  workflow_stage?: string;
}
