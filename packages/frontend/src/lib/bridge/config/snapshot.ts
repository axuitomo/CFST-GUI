// 配置快照的组装：把各域归一化结果拼成完整 ConfigSnapshot。
import { clampInteger, clampNumber, isObject, nonNegativeInteger, nonNegativeNumber, positiveInteger, toBoolean, toObjectRecord, toStringArray, toStringValue, toUnknownArray } from "../values";
import { resolveSourceGroupIPMode } from "../../sources/groups";
import type { ConfigSnapshot, GitHubConfigSnapshot, SourceProfileUpdatePayload } from "../types";
import { DEFAULT_CLOUDFLARE_UPLOAD_TOP_N, DEFAULT_DOH_URL, DEFAULT_ECH_DOMAIN, DEFAULT_GITHUB_UPLOAD_TOP_N, DEFAULT_MAX_LOSS_RATE, DEFAULT_SOURCE_POOL_LIMIT, DEFAULT_TELEGRAM_NOTIFICATION_TOP_N, MAX_LOSS_RATE, MAX_SOURCE_POOL_LIMIT, MAX_TELEGRAM_NOTIFICATION_TOP_N, MIN_PROBE_PING_TIMES } from "./defaults";
import { normalizeCloudflareRecordType, normalizeCloudflareRoutingRules, normalizeCloudflareTTL, normalizeDNSProviders } from "./dns";
import {
  downloadSpeedSampleIntervalMs,
  downloadTimeSeconds,
  minimumInteger,
  normalizeColoFilterMode,
  normalizeCSVEncoding,
  normalizeDebugLogMode,
  normalizeDebugLogVerbosity,
  normalizeDownloadHTTPProtocol,
  normalizeDownloadSpeedMetric,
  normalizeExportOverwrite,
  normalizeFileTestURL,
  normalizeGitHubFormat,
  normalizeHTTPStatusCode,
  normalizePortPolicy,
  normalizeSchedulerDailyTimes,
  normalizeSourceColoFilterPhase,
  normalizeStrategy,
  normalizeTelegramRecipientMode,
  normalizeThemeMode,
  normalizeTraceColoMode,
  normalizeUploadIPVersion,
  normalizeUploadStatus,
  normalizeUTCOffsetMinutes,
  toOptionalNonNegativeNumber,
  toOptionalPositiveInteger,
} from "./scalars";
import { normalizeSourceConfig, normalizeSourceProfileStore } from "./sources";

export function normalizeSourceProfileUpdatePayload(input: unknown): SourceProfileUpdatePayload {
  const source = toObjectRecord(input);
  return {
    config_snapshot: isObject(source.config_snapshot ?? source.configSnapshot) ? normalizeConfigSnapshot(source.config_snapshot ?? source.configSnapshot) : undefined,
    source_profiles: normalizeSourceProfileStore(source.source_profiles ?? source.sourceProfiles ?? source),
    sources: toUnknownArray(source.sources).map((entry, index) => normalizeSourceConfig(entry, index)),
  };
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
  const sources = toUnknownArray(source.sources).map((entry, index) => normalizeSourceConfig(entry, index));
  const sourceIPMode = resolveSourceGroupIPMode(source.source_ip_mode ?? source.sourceIpMode, sources);
  const scheduler = toObjectRecord(source.scheduler);
  const schedulerDailyTimes = scheduler.daily_times ?? scheduler.dailyTimes;
  const ui = toObjectRecord(source.ui);
  const timeouts = toObjectRecord(probe.timeouts);
  const concurrency = toObjectRecord(probe.concurrency);
  const mcis = toObjectRecord(probe.mcis ?? probe.MCIS);
  const stageLimits = isObject(probe.stage_limits) ? probe.stage_limits : isObject(probe.stageLimits) ? probe.stageLimits : {};
  const stage2LimitSource = stageLimits.stage2 ?? probe.stage2_limit ?? probe.stage2Limit ?? probe.headTestCount;
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
  const legacyTelegramRecipientMode = normalizeTelegramRecipientMode(telegram.recipient_mode ?? telegram.recipientMode ?? telegram.target_mode ?? telegram.targetMode);
  const telegramUploadRecipientMode = normalizeTelegramRecipientMode(telegram.upload_recipient_mode ?? telegram.uploadRecipientMode ?? legacyTelegramRecipientMode);
  const telegramTopNRecipientMode = normalizeTelegramRecipientMode(telegram.top_n_recipient_mode ?? telegram.topNRecipientMode ?? telegram.top_recipient_mode ?? telegram.topRecipientMode ?? legacyTelegramRecipientMode);
  const normalizedGitHub: GitHubConfigSnapshot = {
    branch: toStringValue(github.branch ?? githubExport.branch) || "main",
    commit_message_template: toStringValue(github.commit_message_template ?? github.commitMessageTemplate ?? githubExport.commit_message_template ?? githubExport.commitMessageTemplate) || "CFST results {date} {time}",
    csv_header_template: toStringValue(github.csv_header_template ?? github.csvHeaderTemplate ?? githubExport.csv_header_template ?? githubExport.csvHeaderTemplate),
    csv_row_template: toStringValue(github.csv_row_template ?? github.csvRowTemplate ?? githubExport.csv_row_template ?? githubExport.csvRowTemplate),
    enabled: toBoolean(github.enabled ?? github.github_enabled ?? github.githubEnabled ?? githubExport.enabled, false),
    format: normalizeGitHubFormat(github.format ?? githubExport.format),
    last_export_at: toStringValue(github.last_export_at ?? github.lastExportAt ?? githubExport.last_export_at ?? githubExport.lastExportAt),
    owner: toStringValue(github.owner ?? githubExport.owner),
    path_template: toStringValue(github.path_template ?? github.pathTemplate ?? githubExport.path_template ?? githubExport.pathTemplate) || "cfst-results/{date}/{time}-{task_id}.csv",
    repo: toStringValue(github.repo ?? githubExport.repo),
    token: toStringValue(github.token ?? githubExport.token),
    top_n: normalizedGitHubTopN,
    txt_row_template: toStringValue(github.txt_row_template ?? github.txtRowTemplate ?? githubExport.txt_row_template ?? githubExport.txtRowTemplate) || "{ip}",
  };

  return {
    dns_providers: normalizeDNSProviders(source.dns_providers ?? source.dnsProviders, cloudflare),
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
        api_base_url: toStringValue(telegram.api_base_url ?? telegram.apiBaseUrl ?? telegram.api_host ?? telegram.apiHost),
        bot_token: toStringValue(telegram.bot_token ?? telegram.botToken ?? telegram.token),
        chat_id: toStringValue(telegram.chat_id ?? telegram.chatId ?? telegram.chat ?? telegram.target_chat_id ?? telegram.targetChatId ?? telegram.channel_chat_id ?? telegram.channelChatId ?? telegram.group_chat_id ?? telegram.groupChatId),
        enabled: toBoolean(telegram.enabled ?? telegram.telegram_enabled ?? telegram.telegramEnabled, false),
        include_top_n: toBoolean(telegram.include_top_n ?? telegram.includeTopN ?? telegram.top_n_enabled ?? telegram.topNEnabled, false),
        personal_chat_id: toStringValue(telegram.personal_chat_id ?? telegram.personalChatId ?? telegram.private_chat_id ?? telegram.privateChatId ?? telegram.user_chat_id ?? telegram.userChatId),
        recipient_mode: telegramUploadRecipientMode,
        top_n: positiveInteger(telegram.top_n ?? telegram.topN, DEFAULT_TELEGRAM_NOTIFICATION_TOP_N, MAX_TELEGRAM_NOTIFICATION_TOP_N),
        top_n_recipient_mode: telegramTopNRecipientMode,
        upload_recipient_mode: telegramUploadRecipientMode,
        use_system_proxy: toBoolean(telegram.use_system_proxy ?? telegram.useSystemProxy, false),
      },
      webhook: (() => {
        const webhook = toObjectRecord(notifications.webhook);
        const rawURLs = webhook.urls ?? webhook.url ?? [];
        const urls = Array.isArray(rawURLs)
          ? toStringArray(rawURLs, { trim: true }).filter(Boolean)
          : toStringValue(rawURLs)
              .split(/[\n\r,;]+/)
              .map((value) => value.trim())
              .filter(Boolean);
        const dingtalkURL = toStringValue(webhook.dingtalk_url ?? webhook.dingtalkUrl);
        const dingtalkSecret = toStringValue(webhook.dingtalk_secret ?? webhook.dingtalkSecret);
        const wecomURLs = Array.isArray(webhook.wecom_urls) ? toStringArray(webhook.wecom_urls, { trim: true }).filter(Boolean) : [];
        const headers = Object.fromEntries(
          Object.entries(toObjectRecord(webhook.headers))
            .map(([key, value]) => [key.trim(), toStringValue(value)])
            .filter(([key]) => Boolean(key)),
        );
        return {
          enabled: toBoolean(webhook.enabled, false),
          urls: [...new Set(urls)],
          dingtalk_url: dingtalkURL,
          dingtalk_secret: dingtalkSecret,
          wecom_urls: [...new Set(wecomURLs)],
          wecom_corp_id: toStringValue(webhook.wecom_corp_id),
          wecom_agent_id: toStringValue(webhook.wecom_agent_id),
          wecom_secret: toStringValue(webhook.wecom_secret),
          wecom_mobiles: Array.isArray(webhook.wecom_mobiles) ? toStringArray(webhook.wecom_mobiles, { trim: true }).filter(Boolean) : [],
          headers,
          use_system_proxy: toBoolean(webhook.use_system_proxy ?? webhook.useSystemProxy, false),
        };
      })(),
      email: (() => {
        const email = toObjectRecord(notifications.email);
        return {
          enabled: toBoolean(email.enabled, false),
          host: toStringValue(email.host),
          port: positiveInteger(email.port, 587),
          username: toStringValue(email.username),
          password: toStringValue(email.password),
          from: toStringValue(email.from),
          to: Array.isArray(email.to)
            ? toStringArray(email.to, { trim: true }).filter(Boolean)
            : toStringValue(email.to)
                .split(/[,;\n]+/)
                .map((value) => value.trim())
                .filter(Boolean),
          use_tls: toBoolean(email.use_tls ?? email.useTLS, true),
        };
      })(),
    },
    post_probe_push: {
      cloudflare_enabled: toBoolean(postProbePush.cloudflare_enabled ?? postProbePush.cloudflareEnabled, true),
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
        stage3: positiveInteger(concurrency.stage3 ?? concurrency.stage3Concurrency, 1, 16),
      },
      mcis: {
        budget: nonNegativeInteger(mcis.budget ?? mcis.budgetOverride, 0),
        concurrency: nonNegativeInteger(mcis.concurrency ?? mcis.concurrencyOverride, 0),
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
      download_success_limit: nonNegativeInteger(probe.download_success_limit ?? probe.downloadSuccessLimit, 0),
      download_get_concurrency: clampInteger(probe.download_get_concurrency ?? probe.downloadGetConcurrency, 4, 1, 32),
      download_host_header: toStringValue(probe.download_host_header ?? probe.downloadHostHeader),
      download_http_protocol: normalizeDownloadHTTPProtocol(probe.download_http_protocol ?? probe.downloadHTTPProtocol),
      download_sni: toStringValue(probe.download_sni ?? probe.downloadSNI ?? probe.downloadSni),
      download_speed_metric: normalizeDownloadSpeedMetric(probe.download_speed_metric ?? probe.downloadSpeedMetric),
      download_speed_sample_interval_ms: downloadSpeedSampleIntervalMs(probe),
      download_time_seconds: downloadTimeSeconds(probe, timeouts),
      download_warmup_seconds: nonNegativeInteger(probe.download_warmup_seconds ?? probe.downloadWarmupSeconds, 1),
      event_throttle_ms: positiveInteger(probe.event_throttle_ms ?? probe.eventThrottleMs, 100),
      host_header: toStringValue(probe.host_header ?? probe.hostHeader),
      trace_cf_colo: toStringValue(probe.trace_cf_colo ?? probe.httping_cf_colo ?? probe.httpingCfColo),
      trace_cf_colo_mode: normalizeColoFilterMode(probe.trace_cf_colo_mode ?? probe.httping_cf_colo_mode ?? probe.httpingCfColoMode),
      trace_status_code: normalizeHTTPStatusCode(probe.trace_status_code ?? probe.httping_status_code ?? probe.httpingStatusCode),
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
      source_pool_limit: clampInteger(probe.source_pool_limit ?? probe.sourcePoolLimit, DEFAULT_SOURCE_POOL_LIMIT, 1, MAX_SOURCE_POOL_LIMIT),
      stage_limits: {
        stage2: nonNegativeInteger(stage2LimitSource, 0),
        stage3: positiveInteger(stage3LimitSource, 10),
      },
      port_policy: normalizePortPolicy(probe.port_policy ?? probe.portPolicy),
      strategy,
      sni: toStringValue(probe.sni),
      verify_tls_certificate: toBoolean(probe.verify_tls_certificate ?? probe.verifyTLSCertificate, false),
      tcp_port: clampInteger(probe.tcp_port ?? probe.tcpPort, 443, 1, 65535),
      test_all: testAll,
      thresholds: {
        max_tcp_latency_ms: toOptionalPositiveInteger(thresholds.max_tcp_latency_ms ?? thresholds.maxTcpLatencyMs),
        min_download_mbps: nonNegativeNumber(thresholds.min_download_mbps ?? thresholds.minDownloadMbps, 0),
      },
      timeouts: {
        stage1_ms: positiveInteger(timeouts.stage1_ms ?? timeouts.stage1Ms, 1000),
        stage2_ms: positiveInteger(timeouts.stage2_ms ?? timeouts.stage2Ms, 1000),
      },
      trace_colo_mode: normalizeTraceColoMode(probe.trace_colo_mode ?? probe.traceColoMode),
      trace_url: toStringValue(probe.trace_url ?? probe.traceUrl),
      doh_url: toStringValue(probe.doh_url ?? probe.dohUrl) || DEFAULT_DOH_URL,
      ech: toBoolean(probe.ech ?? probe.echEnabled, true),
      ech_config: toStringValue(probe.ech_config ?? probe.echConfig),
      ech_domain: toStringValue(probe.ech_domain ?? probe.echDomain) || DEFAULT_ECH_DOMAIN,
      url: normalizeFileTestURL(probe.url),
      user_agent: toStringValue(probe.user_agent ?? probe.userAgent) || "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0",
    },
    source_ip_mode: sourceIPMode,
    sources: sources.map((entry) => (sourceIPMode ? { ...entry, ip_mode: sourceIPMode } : entry)),
    scheduler: {
      auto_dns_push: toBoolean(scheduler.auto_dns_push ?? scheduler.autoDnsPush, true),
      auto_github_export: toBoolean(scheduler.auto_github_export ?? scheduler.autoGithubExport, true),
      config_source: toStringValue(scheduler.config_source ?? scheduler.configSource) || "draft_preferred",
      daily_times: normalizeSchedulerDailyTimes(schedulerDailyTimes),
      enabled: toBoolean(scheduler.enabled, false),
      interval_minutes: nonNegativeInteger(scheduler.interval_minutes ?? scheduler.intervalMinutes, 0),
      post_run_source_profile_action: toStringValue(scheduler.post_run_source_profile_action ?? scheduler.postRunSourceProfileAction) || "update_recent_run_source_profile",
      skip_if_active: toBoolean(scheduler.skip_if_active ?? scheduler.skipIfActive, true),
    },
    ui: {
      auto_detect_source_name: toBoolean(ui.auto_detect_source_name ?? ui.autoDetectSourceName, true),
      settings_mode: ui.settings_mode === "advanced" || ui.settingsMode === "advanced" ? "advanced" : "basic",
      theme_dark_start: toStringValue(ui.theme_dark_start ?? ui.themeDarkStart) || "19:00",
      theme_light_start: toStringValue(ui.theme_light_start ?? ui.themeLightStart) || "07:00",
      theme_mode: normalizeThemeMode(ui.theme_mode ?? ui.themeMode),
      utc_offset_minutes: normalizeUTCOffsetMinutes(ui.utc_offset_minutes ?? ui.utcOffsetMinutes),
    },
  };
}
