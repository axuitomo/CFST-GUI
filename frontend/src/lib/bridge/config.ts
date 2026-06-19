import { clampInteger, clampNumber, isObject, nonNegativeInteger, nonNegativeNumber, positiveInteger, toBoolean, toInteger, toObjectArray, toObjectRecord, toOptionalInteger, toStringArray, toStringValue, toUnknownArray } from "../bridgeValues";
import type {
  CloudflareRoutingRuleSnapshot,
  ColoFilterMode,
  ConfigSnapshot,
  CSVEncoding,
  DebugLogMode,
  DebugLogVerbosity,
  DesktopSourceConfig,
  DownloadHTTPProtocol,
  DownloadSpeedMetric,
  GitHubConfigSnapshot,
  ProbeStrategy,
  SourceColoFilterPhase,
  SourceIPMode,
  SourceKind,
  SourceProfileItem,
  SourceProfileStore,
  SourceProfileUpdatePayload,
  ThemeMode,
  TraceColoMode,
} from "./types";

export const MIN_PROBE_PING_TIMES = 2;
export const DEFAULT_MAX_LOSS_RATE = 0.15;
export const MAX_LOSS_RATE = 1;
export const DEFAULT_HTTPING_STATUS_CODE = 0;
export const DEFAULT_FILE_TEST_URL = "https://speedtest.xyz9923.dpdns.org/500m";
const DEFAULT_CLOUDFLARE_UPLOAD_TOP_N = 5;
const DEFAULT_GITHUB_UPLOAD_TOP_N = 20;
const DEFAULT_SOURCE_IP_LIMIT = 500;
const DEFAULT_CLOUDFLARE_TTL = 300;
const DEFAULT_UTC_OFFSET_MINUTES = 8 * 60;

function downloadSpeedSampleIntervalMs(probe: Record<string, unknown>) {
  const msValue = probe.download_speed_sample_interval_ms ?? probe.downloadSpeedSampleIntervalMs;
  if (msValue !== null && msValue !== undefined && msValue !== "") {
    return positiveInteger(msValue, 500);
  }

  const secondsValue = probe.download_speed_sample_interval_seconds ?? probe.downloadSpeedSampleIntervalSeconds;
  if (secondsValue !== null && secondsValue !== undefined && secondsValue !== "") {
    return positiveInteger(secondsValue, 1) * 1000;
  }

  return 500;
}

function normalizeDebugLogMode(value: unknown): DebugLogMode {
  return toStringValue(value).toLowerCase() === "freeform" ? "freeform" : "structured";
}

function normalizeDownloadSpeedMetric(value: unknown): DownloadSpeedMetric {
  const normalized = toStringValue(value).trim().toLowerCase();
  return normalized === "max" || normalized === "peak" || normalized === "highest" ? "max" : "average";
}

function normalizeDebugLogVerbosity(value: unknown): DebugLogVerbosity {
  return toStringValue(value).toLowerCase() === "simple" ? "simple" : "detailed";
}

function normalizeThemeMode(value: unknown): ThemeMode {
  const normalized = toStringValue(value).toLowerCase().trim();
  if (normalized === "light") {
    return "light";
  }
  if (normalized === "dark") {
    return "dark";
  }
  if (normalized === "auto_time") {
    return "auto_time";
  }
  return "auto_system_time";
}

function normalizeUTCOffsetMinutes(value: unknown) {
  return clampInteger(value, DEFAULT_UTC_OFFSET_MINUTES, -12 * 60, 14 * 60);
}

function normalizeDownloadHTTPProtocol(value: unknown): DownloadHTTPProtocol {
  const normalized = toStringValue(value).toLowerCase();
  if (normalized === "h1" || normalized === "h1.1" || normalized === "http/1.1") {
    return "h1";
  }
  if (normalized === "h2" || normalized === "http/2") {
    return "h2";
  }
  if (normalized === "h3" || normalized === "http/3") {
    return "h3";
  }
  return "auto";
}

function minimumInteger(value: unknown, fallback: number, min: number, max?: number) {
  return Math.max(min, positiveInteger(value, fallback, max));
}

function toOptionalPositiveInteger(value: unknown) {
  const parsed = toOptionalInteger(value);
  return parsed !== null && parsed > 0 ? parsed : null;
}

function normalizeHTTPStatusCode(value: unknown) {
  const parsed = toInteger(value, DEFAULT_HTTPING_STATUS_CODE);
  return parsed === 0 || (parsed >= 100 && parsed <= 599) ? parsed : DEFAULT_HTTPING_STATUS_CODE;
}

function normalizeExportOverwrite(value: unknown) {
  return toStringValue(value) === "append" ? "append" : "replace_on_start";
}

function normalizeCSVEncoding(value: unknown): CSVEncoding {
  const normalized = toStringValue(value)
    .trim()
    .toLowerCase()
    .replace(/[_\s]+/g, "-");
  if (normalized === "utf-8-bom" || normalized === "utf8-bom" || normalized === "utf-8-with-bom" || normalized === "utf8-with-bom" || normalized === "utf-8-sig" || normalized === "bom") {
    return "utf-8-bom";
  }
  return "utf-8";
}

function normalizeStrategy(value: unknown): ProbeStrategy {
  const normalized = toStringValue(value).toLowerCase();
  if (normalized === "fast" || normalized === "latency" || normalized === "http-colo") {
    return "fast";
  }
  if (normalized === "full" || normalized === "speed" || normalized === "exhaustive") {
    return "full";
  }
  return "fast";
}

export function normalizeSourceColoFilterPhase(value: unknown): SourceColoFilterPhase {
  const normalized = toStringValue(value).toLowerCase().trim();
  if (normalized === "stage2" || normalized === "stage-2" || normalized === "second_stage" || normalized === "second-stage") {
    return "stage2";
  }
  return "precheck";
}

export function normalizeTraceColoMode(value: unknown): TraceColoMode {
  const normalized = toStringValue(value).toLowerCase().trim();
  if (normalized === "trace_url" || normalized === "trace-url" || normalized === "traceurl") {
    return "trace_url";
  }
  return "standard";
}

export function normalizeColoFilterMode(value: unknown): ColoFilterMode {
  const normalized = toStringValue(value).toLowerCase().trim();
  if (normalized === "deny" || normalized === "blacklist" || normalized === "black-list" || normalized === "black_list" || normalized === "blocklist" || normalized === "block-list" || normalized === "block_list") {
    return "deny";
  }
  return "allow";
}

function normalizeCloudflareTTL(value: unknown) {
  const ttl = toInteger(value, DEFAULT_CLOUDFLARE_TTL);
  return [60, 300, 600].includes(ttl) ? ttl : DEFAULT_CLOUDFLARE_TTL;
}

function normalizeCloudflareRecordType(value: unknown): "A" | "AAAA" | "ALL" {
  const normalized = toStringValue(value).trim().toUpperCase();
  if (normalized === "ALL" || normalized === "AAAA") {
    return normalized;
  }
  return "A";
}

function normalizeSourceKind(value: unknown): SourceKind {
  const normalized = toStringValue(value).toLowerCase();
  if (normalized === "inline" || normalized === "file") {
    return normalized;
  }
  return "url";
}

function normalizeSourceIPMode(value: unknown): SourceIPMode {
  return toStringValue(value).toLowerCase() === "mcis" ? "mcis" : "traverse";
}

export function normalizeSourceConfig(input: unknown, index: number): DesktopSourceConfig {
  const source = toObjectRecord(input);

  return {
    colo_filter: toStringValue(source.colo_filter ?? source.coloFilter),
    colo_filter_mode: normalizeColoFilterMode(source.colo_filter_mode ?? source.coloFilterMode),
    content: toStringValue(source.content),
    enabled: toBoolean(source.enabled, true),
    id: toStringValue(source.id) || `source-${index + 1}`,
    ip_limit: Math.max(1, toInteger(source.ip_limit ?? source.ipLimit, DEFAULT_SOURCE_IP_LIMIT)),
    ip_mode: normalizeSourceIPMode(source.ip_mode ?? source.ipMode),
    kind: normalizeSourceKind(source.kind ?? source.type),
    last_fetched_at: toStringValue(source.last_fetched_at ?? source.lastFetchedAt),
    last_fetched_count: Math.max(0, toInteger(source.last_fetched_count ?? source.lastFetchedCount, 0)),
    name: toStringValue(source.name) || `输入源 ${index + 1}`,
    path: toStringValue(source.path),
    status_text: toStringValue(source.status_text ?? source.statusText),
    url: toStringValue(source.url),
  };
}

function normalizeSourceProfileItem(input: unknown, index: number): SourceProfileItem {
  const source = toObjectRecord(input);

  return {
    created_at: toStringValue(source.created_at ?? source.createdAt),
    id: toStringValue(source.id) || `source-profile-${index + 1}`,
    name: toStringValue(source.name) || `输入源档案 ${index + 1}`,
    sources: toUnknownArray(source.sources).map((entry, sourceIndex) => normalizeSourceConfig(entry, sourceIndex)),
    updated_at: toStringValue(source.updated_at ?? source.updatedAt),
  };
}

export function normalizeSourceProfileStore(input: unknown): SourceProfileStore {
  const source = toObjectRecord(input);

  return {
    active_profile_id: toStringValue(source.active_profile_id ?? source.activeProfileId),
    items: toUnknownArray(source.items).map((entry, index) => normalizeSourceProfileItem(entry, index)),
    schema_version: toStringValue(source.schema_version ?? source.schemaVersion),
    updated_at: toStringValue(source.updated_at ?? source.updatedAt),
  };
}

export function normalizeSourceProfileUpdatePayload(input: unknown): SourceProfileUpdatePayload {
  const source = toObjectRecord(input);
  return {
    config_snapshot: isObject(source.config_snapshot ?? source.configSnapshot) ? normalizeConfigSnapshot(source.config_snapshot ?? source.configSnapshot) : undefined,
    source_profiles: normalizeSourceProfileStore(source.source_profiles ?? source.sourceProfiles ?? source),
    sources: toUnknownArray(source.sources).map((entry, index) => normalizeSourceConfig(entry, index)),
  };
}

function normalizeCloudflareRoutingRules(input: unknown): CloudflareRoutingRuleSnapshot[] {
  return toObjectArray(input).map((item) => ({
    enabled: toBoolean(item.enabled, true),
    filter_mode: toStringValue(item.filter_mode ?? item.filterMode).toLowerCase() === "deny" ? "deny" : "allow",
    filter_tokens: toStringValue(item.filter_tokens ?? item.filterTokens),
    name: toStringValue(item.name),
    record_name: toStringValue(item.record_name ?? item.recordName),
    record_type: normalizeCloudflareRecordType(item.record_type ?? item.recordType),
    top_n: nonNegativeInteger(item.top_n ?? item.topN, DEFAULT_CLOUDFLARE_UPLOAD_TOP_N),
  }));
}

export function isMaskedTokenValue(value: string) {
  return value.includes("...") || value.includes("***") || /^\*+$/.test(value);
}

export function normalizeConfigSnapshot(input: unknown): ConfigSnapshot {
  const source = toObjectRecord(input);
  const cloudflare = toObjectRecord(source.cloudflare);
  const github = toObjectRecord(source.github);
  const postProbePush = isObject(source.post_probe_push) ? source.post_probe_push : isObject(source.postProbePush) ? source.postProbePush : {};
  const upload = toObjectRecord(source.upload);
  const uploadCloudflare = toObjectRecord(upload.cloudflare);
  const uploadGitHub = toObjectRecord(upload.github);
  const uploadSharedFilter = isObject(upload.shared_filter) ? upload.shared_filter : isObject(upload.sharedFilter) ? upload.sharedFilter : {};
  const exportConfig = toObjectRecord(source.export);
  const githubExport = toObjectRecord(exportConfig.github);
  const backup = toObjectRecord(source.backup);
  const webdav = toObjectRecord(backup.webdav);
  const maintenance = toObjectRecord(source.maintenance);
  const notifications = toObjectRecord(source.notifications);
  const telegram = toObjectRecord(notifications.telegram ?? notifications.tg ?? source.telegram);
  const probe = toObjectRecord(source.probe);
  const sources = toUnknownArray(source.sources);
  const scheduler = toObjectRecord(source.scheduler);
  const schedulerDailyTimes = scheduler.daily_times ?? scheduler.dailyTimes;
  const ui = toObjectRecord(source.ui);
  const timeouts = toObjectRecord(probe.timeouts);
  const concurrency = toObjectRecord(probe.concurrency);
  const stageLimits = isObject(probe.stage_limits) ? probe.stage_limits : isObject(probe.stageLimits) ? probe.stageLimits : {};
  const stage3LimitSource = stageLimits.stage3 ?? probe.stage3_limit ?? probe.stage3Limit ?? probe.download_count ?? probe.downloadCount;
  const cooldownPolicy = isObject(probe.cooldown_policy) ? probe.cooldown_policy : isObject(probe.cooldownPolicy) ? probe.cooldownPolicy : {};
  const retryPolicy = isObject(probe.retry_policy) ? probe.retry_policy : isObject(probe.retryPolicy) ? probe.retryPolicy : {};
  const thresholds = toObjectRecord(probe.thresholds);
  const strategy = normalizeStrategy(probe.strategy);
  const testAll = toBoolean(probe.test_all ?? probe.testAll, false);
  const normalizedCloudflareRoutingRules = normalizeCloudflareRoutingRules(cloudflare.routing_rules ?? cloudflare.routingRules ?? uploadCloudflare.routing_rules ?? uploadCloudflare.routingRules);
  const normalizedCloudflareTopN = nonNegativeInteger(cloudflare.top_n ?? cloudflare.topN ?? uploadCloudflare.top_n ?? uploadCloudflare.topN, DEFAULT_CLOUDFLARE_UPLOAD_TOP_N);
  const normalizedCloudflareRoutingEnabled = toBoolean(cloudflare.routing_enabled ?? cloudflare.routingEnabled ?? uploadCloudflare.routing_enabled ?? uploadCloudflare.routingEnabled, false);
  const normalizedGitHubTopN = nonNegativeInteger(github.top_n ?? github.topN ?? uploadGitHub.top_n ?? uploadGitHub.topN, DEFAULT_GITHUB_UPLOAD_TOP_N);
  const normalizedGitHub: GitHubConfigSnapshot = {
    branch: toStringValue(github.branch ?? githubExport.branch) || "main",
    commit_message_template: toStringValue(github.commit_message_template ?? github.commitMessageTemplate ?? githubExport.commit_message_template ?? githubExport.commitMessageTemplate) || "CFST results {date} {time}",
    csv_header_template: toStringValue(github.csv_header_template ?? github.csvHeaderTemplate ?? githubExport.csv_header_template ?? githubExport.csvHeaderTemplate),
    csv_row_template: toStringValue(github.csv_row_template ?? github.csvRowTemplate ?? githubExport.csv_row_template ?? githubExport.csvRowTemplate),
    enabled: toBoolean(github.enabled ?? github.github_enabled ?? github.githubEnabled ?? githubExport.enabled, false),
    format: normalizeGitHubFormat(github.format ?? githubExport.format),
    last_export_at: toStringValue(github.last_export_at ?? github.lastExportAt ?? githubExport.last_export_at ?? githubExport.lastExportAt),
    owner: toStringValue(github.owner ?? githubExport.owner) || "axuitomo",
    path_template: toStringValue(github.path_template ?? github.pathTemplate ?? githubExport.path_template ?? githubExport.pathTemplate) || "cfst-results/{date}/{time}-{task_id}.csv",
    repo: toStringValue(github.repo ?? githubExport.repo) || "CFST-GUI",
    token: toStringValue(github.token ?? githubExport.token),
    top_n: normalizedGitHubTopN,
    txt_row_template: toStringValue(github.txt_row_template ?? github.txtRowTemplate ?? githubExport.txt_row_template ?? githubExport.txtRowTemplate) || "{ip}",
  };

  return {
    backup: {
      webdav: {
        enabled: toBoolean(webdav.enabled, false),
        last_backup_at: toStringValue(webdav.last_backup_at ?? webdav.lastBackupAt),
        last_restore_at: toStringValue(webdav.last_restore_at ?? webdav.lastRestoreAt),
        password: toStringValue(webdav.password),
        remote_path: toStringValue(webdav.remote_path ?? webdav.remotePath) || "cfst-gui-config.zip",
        server_url: toStringValue(webdav.server_url ?? webdav.serverUrl ?? webdav.url),
        timeout_seconds: positiveInteger(webdav.timeout_seconds ?? webdav.timeoutSeconds, 30),
        username: toStringValue(webdav.username),
      },
    },
    cloudflare: {
      api_token: toStringValue(cloudflare.api_token),
      comment: toStringValue(cloudflare.comment),
      enabled: toBoolean(cloudflare.enabled ?? cloudflare.cloudflare_enabled ?? cloudflare.cloudflareEnabled, false),
      proxied: Boolean(cloudflare.proxied),
      record_name: toStringValue(cloudflare.record_name),
      record_type: normalizeCloudflareRecordType(cloudflare.record_type),
      routing_enabled: normalizedCloudflareRoutingEnabled,
      routing_rules: normalizedCloudflareRoutingRules,
      top_n: normalizedCloudflareTopN,
      ttl: normalizeCloudflareTTL(cloudflare.ttl),
      zone_id: toStringValue(cloudflare.zone_id),
    },
    github: normalizedGitHub,
    maintenance: {
      completed_task_retention_days: nonNegativeInteger(maintenance.completed_task_retention_days ?? maintenance.completedTaskRetentionDays, 7),
    },
    notifications: {
      telegram: {
        bot_token: toStringValue(telegram.bot_token ?? telegram.botToken ?? telegram.token),
        chat_id: toStringValue(telegram.chat_id ?? telegram.chatId ?? telegram.chat),
        enabled: toBoolean(telegram.enabled ?? telegram.telegram_enabled ?? telegram.telegramEnabled, false),
      },
    },
    post_probe_push: {
      cloudflare_enabled: toBoolean(postProbePush.cloudflare_enabled ?? postProbePush.cloudflareEnabled, false),
      github_enabled: toBoolean(postProbePush.github_enabled ?? postProbePush.githubEnabled, false),
    },
    upload: {
      cloudflare: {
        routing_enabled: normalizedCloudflareRoutingEnabled,
        routing_rules: normalizedCloudflareRoutingRules,
        top_n: normalizedCloudflareTopN,
      },
      github: {
        top_n: normalizedGitHubTopN,
      },
      shared_filter: {
        colo_allow: toStringValue(uploadSharedFilter.colo_allow ?? uploadSharedFilter.coloAllow),
        colo_deny: toStringValue(uploadSharedFilter.colo_deny ?? uploadSharedFilter.coloDeny),
        enabled: toBoolean(uploadSharedFilter.enabled, false),
        ip_version: normalizeUploadIPVersion(uploadSharedFilter.ip_version ?? uploadSharedFilter.ipVersion),
        max_loss_rate: toOptionalNonNegativeNumber(uploadSharedFilter.max_loss_rate ?? uploadSharedFilter.maxLossRate),
        max_tcp_latency_ms: toOptionalPositiveInteger(uploadSharedFilter.max_tcp_latency_ms ?? uploadSharedFilter.maxTcpLatencyMs),
        max_trace_latency_ms: toOptionalPositiveInteger(uploadSharedFilter.max_trace_latency_ms ?? uploadSharedFilter.maxTraceLatencyMs),
        min_download_mbps: nonNegativeNumber(uploadSharedFilter.min_download_mbps ?? uploadSharedFilter.minDownloadMbps, 0),
        status: normalizeUploadStatus(uploadSharedFilter.status),
      },
    },
    export: {
      csv_encoding: normalizeCSVEncoding(exportConfig.csv_encoding ?? exportConfig.csvEncoding),
      file_name: toStringValue(exportConfig.file_name),
      file_name_template: toStringValue(exportConfig.file_name_template ?? exportConfig.fileNameTemplate),
      format: toStringValue(exportConfig.format),
      github: {
        ...normalizedGitHub,
      },
      overwrite: normalizeExportOverwrite(exportConfig.overwrite),
      target_dir: toStringValue(exportConfig.target_dir),
      target_uri: toStringValue(exportConfig.target_uri ?? exportConfig.targetUri),
    },
    probe: {
      concurrency: {
        stage1: positiveInteger(concurrency.stage1, 200, 1000),
        stage2: clampInteger(concurrency.stage2, 6, 1, 30),
        stage3: 1,
      },
      cooldown_policy: {
        consecutive_failures: nonNegativeInteger(cooldownPolicy.consecutive_failures ?? cooldownPolicy.consecutiveFailures, 3),
        cooldown_ms: nonNegativeInteger(cooldownPolicy.cooldown_ms ?? cooldownPolicy.cooldownMs, 250),
      },
      debug: toBoolean(probe.debug, false),
      debug_capture_address: toStringValue(probe.debug_capture_address ?? probe.debugCaptureAddress),
      debug_capture_enabled: toBoolean(probe.debug_capture_enabled ?? probe.debugCaptureEnabled, Boolean(toStringValue(probe.debug_capture_address ?? probe.debugCaptureAddress).trim())),
      debug_log_format: toStringValue(probe.debug_log_format ?? probe.debugLogFormat),
      debug_log_mode: normalizeDebugLogMode(probe.debug_log_mode ?? probe.debugLogMode),
      debug_log_verbosity: normalizeDebugLogVerbosity(probe.debug_log_verbosity ?? probe.debugLogVerbosity),
      disable_download: strategy === "fast",
      download_buffer_kb: clampInteger(probe.download_buffer_kb ?? probe.downloadBufferKB, 256, 64, 4096),
      download_count: positiveInteger(probe.download_count ?? probe.downloadCount ?? stageLimits.stage3, 10),
      download_get_concurrency: clampInteger(probe.download_get_concurrency ?? probe.downloadGetConcurrency, 4, 1, 32),
      download_http_protocol: normalizeDownloadHTTPProtocol(probe.download_http_protocol ?? probe.downloadHTTPProtocol),
      download_speed_metric: normalizeDownloadSpeedMetric(probe.download_speed_metric ?? probe.downloadSpeedMetric),
      download_speed_sample_interval_ms: downloadSpeedSampleIntervalMs(probe),
      download_speed_sample_interval_seconds: positiveInteger(probe.download_speed_sample_interval_seconds ?? probe.downloadSpeedSampleIntervalSeconds, 0),
      download_time_seconds: positiveInteger(probe.download_time_seconds ?? probe.downloadTimeSeconds, 4),
      download_warmup_seconds: nonNegativeInteger(probe.download_warmup_seconds ?? probe.downloadWarmupSeconds, 1),
      event_throttle_ms: positiveInteger(probe.event_throttle_ms ?? probe.eventThrottleMs, 100),
      host_header: toStringValue(probe.host_header ?? probe.hostHeader),
      httping: false,
      httping_cf_colo: toStringValue(probe.httping_cf_colo ?? probe.httpingCfColo),
      httping_cf_colo_mode: normalizeColoFilterMode(probe.httping_cf_colo_mode ?? probe.httpingCfColoMode),
      httping_status_code: normalizeHTTPStatusCode(probe.httping_status_code ?? probe.httpingStatusCode),
      max_loss_rate: clampNumber(probe.max_loss_rate ?? probe.maxLossRate, DEFAULT_MAX_LOSS_RATE, 0, MAX_LOSS_RATE),
      min_delay_ms: nonNegativeInteger(probe.min_delay_ms ?? probe.minDelayMs, 0),
      ping_times: minimumInteger(probe.ping_times ?? probe.pingTimes, 4, MIN_PROBE_PING_TIMES),
      print_num: nonNegativeInteger(probe.print_num ?? probe.printNum, 0),
      retry_policy: {
        backoff_ms: nonNegativeInteger(retryPolicy.backoff_ms ?? retryPolicy.backoffMs, 0),
        max_attempts: nonNegativeInteger(retryPolicy.max_attempts ?? retryPolicy.maxAttempts, 0),
      },
      request_headers: toStringValue(probe.request_headers ?? probe.requestHeaders),
      skip_first_latency_sample: toBoolean(probe.skip_first_latency_sample ?? probe.skipFirstLatencySample, true),
      source_colo_filter_phase: normalizeSourceColoFilterPhase(probe.source_colo_filter_phase ?? probe.sourceColoFilterPhase),
      stage_limits: {
        stage3: positiveInteger(stage3LimitSource, 10),
      },
      port_policy: normalizePortPolicy(probe.port_policy ?? probe.portPolicy),
      strategy,
      sni: toStringValue(probe.sni),
      tcp_port: clampInteger(probe.tcp_port ?? probe.tcpPort, 443, 1, 65535),
      test_all: testAll,
      thresholds: {
        max_http_latency_ms: null,
        max_tcp_latency_ms: toOptionalPositiveInteger(thresholds.max_tcp_latency_ms ?? thresholds.maxTcpLatencyMs),
        min_download_mbps: nonNegativeNumber(thresholds.min_download_mbps ?? thresholds.minDownloadMbps, 0),
      },
      timeouts: {
        stage1_ms: positiveInteger(timeouts.stage1_ms ?? timeouts.stage1Ms, 1000),
        stage2_ms: positiveInteger(timeouts.stage2_ms ?? timeouts.stage2Ms, 1000),
        stage3_ms: positiveInteger(timeouts.stage3_ms ?? timeouts.stage3Ms, 10000),
      },
      trace_colo_mode: normalizeTraceColoMode(probe.trace_colo_mode ?? probe.traceColoMode),
      trace_url: toStringValue(probe.trace_url ?? probe.traceUrl),
      url: toStringValue(probe.url) || DEFAULT_FILE_TEST_URL,
      user_agent: toStringValue(probe.user_agent ?? probe.userAgent) || "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0",
    },
    sources: sources.map((entry, index) => normalizeSourceConfig(entry, index)),
    scheduler: {
      auto_dns_push: toBoolean(scheduler.auto_dns_push ?? scheduler.autoDnsPush, true),
      auto_github_export: toBoolean(scheduler.auto_github_export ?? scheduler.autoGithubExport, true),
      config_source: toStringValue(scheduler.config_source ?? scheduler.configSource) || "draft_preferred",
      daily_times: Array.isArray(schedulerDailyTimes)
        ? toStringArray(schedulerDailyTimes, { trim: true })
        : toStringValue(schedulerDailyTimes)
            .split(/[,\s;]+/)
            .map((entry) => entry.trim())
            .filter(Boolean),
      enabled: toBoolean(scheduler.enabled, false),
      interval_minutes: nonNegativeInteger(scheduler.interval_minutes ?? scheduler.intervalMinutes, 0),
      pipeline_template_id: toStringValue(scheduler.pipeline_template_id ?? scheduler.pipelineTemplateId),
      post_run_source_profile_action: toStringValue(scheduler.post_run_source_profile_action ?? scheduler.postRunSourceProfileAction) || "update_recent_run_source_profile",
      run_mode: toStringValue(scheduler.run_mode ?? scheduler.runMode) === "pipeline" ? "pipeline" : "probe",
      skip_if_active: toBoolean(scheduler.skip_if_active ?? scheduler.skipIfActive, true),
    },
    ui: {
      auto_detect_source_name: toBoolean(ui.auto_detect_source_name ?? ui.autoDetectSourceName, true),
      theme_dark_start: toStringValue(ui.theme_dark_start ?? ui.themeDarkStart) || "19:00",
      theme_light_start: toStringValue(ui.theme_light_start ?? ui.themeLightStart) || "07:00",
      theme_mode: normalizeThemeMode(ui.theme_mode ?? ui.themeMode),
      utc_offset_minutes: normalizeUTCOffsetMinutes(ui.utc_offset_minutes ?? ui.utcOffsetMinutes),
    },
  };
}

function normalizeUploadStatus(value: unknown): "all" | "passed" {
  return toStringValue(value).trim().toLowerCase() === "all" ? "all" : "passed";
}

function normalizeGitHubFormat(value: unknown): "csv" | "txt" {
  return toStringValue(value).trim().toLowerCase() === "txt" ? "txt" : "csv";
}

function normalizePortPolicy(value: unknown): "source_override_global" | "fixed_global" {
  return toStringValue(value).trim().toLowerCase() === "fixed_global" ? "fixed_global" : "source_override_global";
}

function normalizeUploadIPVersion(value: unknown): "any" | "ipv4" | "ipv6" {
  const normalized = toStringValue(value).trim().toLowerCase();
  if (normalized === "ipv4" || normalized === "ipv6") {
    return normalized;
  }
  return "any";
}

function toOptionalNonNegativeNumber(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  const parsed = Number.parseFloat(String(value));
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }
  return parsed;
}
