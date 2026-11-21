// 配置快照构建（重构计划 §4.2）：buildConfigSnapshot / stableSnapshotValue /
// snapshotSignature / schedulerIntervalMinutesForSnapshot 从 App.vue 原样迁出。
// 纯函数化：settings、DNS 提供商快照、源 IP 模式与源载荷由调用方注入；
// App 保留同名薄包装，25 处调用点零改动。
import { isMaskedTokenValue, type DNSProviderSnapshot, type ProbeStrategy } from "./bridge";
import { DEFAULT_CLOUDFLARE_TTL, DEFAULT_DEBUG_LOG_FORMAT, DEFAULT_HTTPING_STATUS_CODE, DEFAULT_MAX_LOSS_RATE, DEFAULT_PROBE_USER_AGENT, DEFAULT_SOURCE_POOL_LIMIT, MAX_LOSS_RATE, MAX_SOURCE_POOL_LIMIT, MIN_PROBE_PING_TIMES, type SettingsForm } from "./settingsForm";
import { boundedCount, clampedNumber, minimumCount, nonNegativeCount, nonNegativeNumber, normalizeDownloadHTTPProtocol, optionalNumberForPayload, positiveCount, schedulerDailyTimesFromText } from "./valueCoerce";
import { normalizeUTCOffsetMinutes } from "./time";

export function parseWebhookHeaders(value: string) {
  try {
    const parsed = JSON.parse(value);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
}

export interface ConfigSnapshotInput<S extends { status_text: string } = { status_text: string }> {
  settings: SettingsForm;
  dnsProviderSnapshots: DNSProviderSnapshot[];
  primaryCloudflareSnapshot: DNSProviderSnapshot | null;
  sourceIPMode: string;
  sources: S[];
}

export function schedulerIntervalMinutesForSnapshot(settings: SettingsForm, enabled: boolean) {
  const value = Number.isFinite(settings.schedulerIntervalMinutes) && settings.schedulerIntervalMinutes > 0 ? settings.schedulerIntervalMinutes : settings.schedulerIntervalMinutesDraft;
  return enabled ? positiveCount(value, 60) : nonNegativeCount(value, 0);
}

export function buildConfigSnapshot<S extends { status_text: string }>(input: ConfigSnapshotInput<S>) {
  const normalizedStrategy: ProbeStrategy = input.settings.probeStrategy === "full" ? "full" : "fast";
  const normalizedGitHubToken = input.settings.githubToken.trim();
  const githubProviderEnabled = Boolean(input.settings.githubOwner.trim() && input.settings.githubRepo.trim() && input.settings.githubBranch.trim() && input.settings.githubPathTemplate.trim() && normalizedGitHubToken && !isMaskedTokenValue(normalizedGitHubToken));
  const primaryCloudflareSnapshot = input.primaryCloudflareSnapshot;
  const cloudflareRoutingRules = primaryCloudflareSnapshot?.routing_rules ?? [];
  const cloudflareProviderEnabled = Boolean(primaryCloudflareSnapshot?.enabled);
  const githubConfig = {
    branch: input.settings.githubBranch.trim() || "main",
    csv_header_template: input.settings.githubCSVHeaderTemplate,
    csv_row_template: input.settings.githubCSVRowTemplate,
    commit_message_template: input.settings.githubCommitMessageTemplate.trim() || "CFST results {date} {time}",
    enabled: githubProviderEnabled,
    format: input.settings.githubFormat,
    last_export_at: input.settings.githubLastExportAt.trim(),
    owner: input.settings.githubOwner.trim(),
    path_template: input.settings.githubPathTemplate.trim() || "cfst-results/{date}/{time}-{task_id}.csv",
    repo: input.settings.githubRepo.trim(),
    token: normalizedGitHubToken,
    top_n: nonNegativeCount(input.settings.uploadGitHubTopN, 20),
    txt_row_template: input.settings.githubTXTRowTemplate || "{ip}",
  };

  return {
    dns_providers: input.dnsProviderSnapshots,
    cloudflare: {
      ...(primaryCloudflareSnapshot?.api_token ? { api_token: primaryCloudflareSnapshot.api_token } : {}),
      allow_cname: primaryCloudflareSnapshot?.allow_cname ?? false,
      comment: input.settings.comment.trim(),
      enabled: cloudflareProviderEnabled,
      proxied: false,
      record_name: primaryCloudflareSnapshot?.record_name ?? "",
      routing_enabled: Boolean(primaryCloudflareSnapshot?.routing_enabled),
      routing_rules: cloudflareRoutingRules,
      top_n: primaryCloudflareSnapshot?.top_n ?? 5,
      ttl: primaryCloudflareSnapshot?.ttl ?? DEFAULT_CLOUDFLARE_TTL,
      zone_id: primaryCloudflareSnapshot?.zone_id ?? "",
    },
    github: githubConfig,
    post_probe_push: {
      cloudflare_enabled: input.settings.postProbePushCloudflareEnabled,
      github_enabled: input.settings.postProbePushGitHubEnabled,
    },
    upload: {
      cloudflare: {
        routing_enabled: Boolean(primaryCloudflareSnapshot?.routing_enabled),
        routing_rules: cloudflareRoutingRules,
        top_n: primaryCloudflareSnapshot?.top_n ?? 5,
      },
      github: {
        top_n: nonNegativeCount(input.settings.uploadGitHubTopN, 20),
      },
      shared_filter: {
        colo_allow: input.settings.uploadSharedFilterColoAllow.trim(),
        colo_deny: input.settings.uploadSharedFilterColoDeny.trim(),
        enabled: input.settings.uploadSharedFilterEnabled,
        ip_version: input.settings.uploadSharedFilterIPVersion,
        max_loss_rate: optionalNumberForPayload(input.settings.uploadSharedFilterMaxLossRate),
        max_tcp_latency_ms: optionalNumberForPayload(input.settings.uploadSharedFilterMaxTcpLatencyMs),
        max_trace_latency_ms: optionalNumberForPayload(input.settings.uploadSharedFilterMaxTraceLatencyMs),
        min_download_mbps: nonNegativeNumber(input.settings.uploadSharedFilterMinDownloadMbps, 0),
        status: input.settings.uploadSharedFilterStatus,
      },
    },
    backup: {
      webdav: {
        enabled: input.settings.webdavEnabled,
        last_backup_at: input.settings.webdavLastBackupAt.trim(),
        last_restore_at: input.settings.webdavLastRestoreAt.trim(),
        password: input.settings.webdavPassword,
        remote_path: input.settings.webdavRemotePath.trim() || "cfst-gui-config.zip",
        server_url: input.settings.webdavServerURL.trim(),
        timeout_seconds: positiveCount(input.settings.webdavTimeoutSeconds, 30),
        username: input.settings.webdavUsername.trim(),
      },
    },
    export: {
      csv_encoding: input.settings.exportCSVEncoding === "utf-8-bom" ? "utf-8-bom" : "utf-8",
      ...(input.settings.exportFileName.trim() ? { file_name: input.settings.exportFileName.trim() } : {}),
      ...(input.settings.exportFileNameTemplate.trim() ? { file_name_template: input.settings.exportFileNameTemplate.trim() } : {}),
      github: githubConfig,
      ...(input.settings.exportOverwrite.trim() ? { overwrite: input.settings.exportOverwrite.trim() } : {}),
      ...(input.settings.exportTargetDir.trim() ? { target_dir: input.settings.exportTargetDir.trim() } : {}),
      ...(input.settings.exportTargetUri.trim() ? { target_uri: input.settings.exportTargetUri.trim() } : {}),
    },
    probe: {
      concurrency: {
        stage1: positiveCount(input.settings.probeConcurrencyStage1, 200, 1000),
        stage2: Math.max(1, Math.min(30, input.settings.probeConcurrencyStage2)),
        stage3: positiveCount(input.settings.probeConcurrencyStage3, 1, 16),
      },
      mcis: {
        budget: nonNegativeCount(input.settings.probeMCISBudget, 0),
        concurrency: nonNegativeCount(input.settings.probeMCISConcurrency, 0),
      },
      cooldown_policy: {
        consecutive_failures: nonNegativeCount(input.settings.probeCooldownFailures, 3),
        cooldown_ms: nonNegativeCount(input.settings.probeCooldownMs, 250),
      },
      debug: input.settings.probeDebug,
      debug_capture_address: input.settings.probeDebugCaptureAddress.trim(),
      debug_capture_enabled: input.settings.probeDebugCaptureEnabled,
      debug_log_format: input.settings.probeDebugLogMode === "freeform" ? input.settings.probeDebugLogFormat.trim() || DEFAULT_DEBUG_LOG_FORMAT : "",
      debug_log_mode: input.settings.probeDebugLogMode === "freeform" ? "freeform" : "structured",
      debug_log_verbosity: input.settings.probeDebugLogVerbosity === "simple" ? "simple" : "detailed",
      disable_download: normalizedStrategy === "fast",
      download_buffer_kb: boundedCount(input.settings.probeDownloadBufferKB, 256, 64, 4096),
      download_success_limit: nonNegativeCount(input.settings.probeDownloadSuccessLimit, 0),
      download_get_concurrency: boundedCount(input.settings.probeDownloadGetConcurrency, 4, 1, 32),
      download_host_header: input.settings.probeDownloadHostHeader.trim(),
      download_http_protocol: normalizeDownloadHTTPProtocol(input.settings.probeDownloadHTTPProtocol),
      download_sni: input.settings.probeDownloadSNI.trim(),
      download_speed_metric: input.settings.probeDownloadSpeedMetric === "max" ? "max" : "average",
      download_speed_sample_interval_ms: positiveCount(input.settings.probeDownloadSpeedSampleIntervalMs, 1000),
      download_time_seconds: positiveCount(input.settings.probeDownloadTimeSeconds, 4),
      download_warmup_seconds: nonNegativeCount(input.settings.probeDownloadWarmupSeconds, 1),
      event_throttle_ms: positiveCount(input.settings.probeEventThrottleMs, 100),
      host_header: input.settings.probeHostHeader.trim(),
      trace_cf_colo: input.settings.probeTraceCfColo.trim(),
      trace_cf_colo_mode: input.settings.probeTraceCfColoMode === "deny" ? "deny" : "allow",
      trace_status_code: input.settings.probeTraceStatusCode === 0 || (input.settings.probeTraceStatusCode >= 100 && input.settings.probeTraceStatusCode <= 599) ? input.settings.probeTraceStatusCode : DEFAULT_HTTPING_STATUS_CODE,
      max_loss_rate: clampedNumber(input.settings.maxLossRate, DEFAULT_MAX_LOSS_RATE, 0, MAX_LOSS_RATE),
      min_delay_ms: nonNegativeCount(input.settings.minDelayMs, 0),
      ping_times: minimumCount(input.settings.probePingTimes, 4, MIN_PROBE_PING_TIMES),
      print_num: nonNegativeCount(input.settings.probePrintNum, 0),
      retry_policy: {
        backoff_ms: nonNegativeCount(input.settings.probeRetryBackoffMs, 0),
        max_attempts: nonNegativeCount(input.settings.probeRetryMaxAttempts, 0),
      },
      request_headers: input.settings.probeRequestHeaders.trim(),
      skip_first_latency_sample: true,
      source_colo_filter_phase: input.settings.probeSourceColoFilterPhase,
      source_pool_limit: boundedCount(input.settings.probeSourcePoolLimit, DEFAULT_SOURCE_POOL_LIMIT, 1, MAX_SOURCE_POOL_LIMIT),
      stage_limits: {
        stage2: nonNegativeCount(input.settings.probeStageLimitStage2, 0),
        stage3: positiveCount(input.settings.probeStageLimitStage3, 10),
      },
      port_policy: input.settings.probePortPolicy,
      strategy: normalizedStrategy,
      sni: input.settings.probeSNI.trim(),
      verify_tls_certificate: input.settings.probeVerifyTLSCertificate,
      tcp_port: positiveCount(input.settings.probeTcpPort, 443, 65535),
      test_all: false,
      thresholds: {
        max_tcp_latency_ms: optionalNumberForPayload(input.settings.maxTcpLatencyMs),
        min_download_mbps: nonNegativeNumber(input.settings.minDownloadMbps, 0),
      },
      timeouts: {
        stage1_ms: positiveCount(input.settings.probeTimeoutStage1Ms, 1000),
        stage2_ms: positiveCount(input.settings.probeTimeoutStage2Ms, 1000),
      },
      trace_colo_mode: input.settings.probeTraceColoMode,
      trace_url: input.settings.probeTraceURL.trim(),
      doh_url: input.settings.probeDoHURL.trim(),
      ech: input.settings.probeECH,
      ech_config: input.settings.probeECHConfig.trim(),
      ech_domain: input.settings.probeECHDomain.trim(),
      url: input.settings.probeURL.trim(),
      user_agent: input.settings.probeUserAgent.trim() || DEFAULT_PROBE_USER_AGENT,
    },
    ui: {
      auto_detect_source_name: input.settings.sourceAutoDetectName,
      // 普通/高级设置已合并；固定写 advanced，旧配置的 basic 值读入后忽略。
      settings_mode: "advanced",
      theme_dark_start: input.settings.themeDarkStart.trim() || "19:00",
      theme_light_start: input.settings.themeLightStart.trim() || "07:00",
      theme_mode: input.settings.themeMode,
      utc_offset_minutes: normalizeUTCOffsetMinutes(input.settings.utcOffsetMinutes),
    },
    maintenance: {
      completed_task_retention_days: nonNegativeCount(input.settings.maintenanceCompletedTaskRetentionDays, 7),
    },
    notifications: {
      telegram: {
        bot_token: input.settings.telegramBotToken.trim(),
        chat_id: input.settings.telegramChatId.trim(),
        enabled: input.settings.telegramNotificationEnabled,
        include_top_n: input.settings.telegramIncludeTopN,
        personal_chat_id: input.settings.telegramPersonalChatId.trim(),
        recipient_mode: input.settings.telegramUploadRecipientMode,
        top_n: positiveCount(input.settings.telegramTopN, 5, 50),
        top_n_recipient_mode: input.settings.telegramTopNRecipientMode,
        upload_recipient_mode: input.settings.telegramUploadRecipientMode,
        use_system_proxy: input.settings.telegramUseSystemProxy,
      },
      webhook: {
        enabled: input.settings.webhookEnabled,
        urls: input.settings.webhookURLs
          .split(/[\n\r,;]+/)
          .map((value) => value.trim())
          .filter(Boolean),
        dingtalk_url: input.settings.webhookDingTalkURL.trim(),
        dingtalk_secret: input.settings.webhookDingTalkSecret.trim(),
        wecom_urls: input.settings.webhookWeComURLs
          .split(/[\n\r,;]+/)
          .map((value) => value.trim())
          .filter(Boolean),
        use_system_proxy: input.settings.webhookUseSystemProxy,
        headers: parseWebhookHeaders(input.settings.webhookHeadersJSON),
        wecom_corp_id: input.settings.webhookWeComCorpID.trim(),
        wecom_agent_id: input.settings.webhookWeComAgentID.trim(),
        wecom_secret: input.settings.webhookWeComSecret.trim(),
        wecom_mobiles: input.settings.webhookWeComMobiles
          .split(/[\n\r,;]+/)
          .map((value) => value.trim())
          .filter(Boolean),
      },
      email: {
        enabled: input.settings.emailEnabled,
        host: input.settings.emailHost.trim(),
        port: positiveCount(input.settings.emailPort, 587, 65535),
        username: input.settings.emailUsername.trim(),
        password: input.settings.emailPassword,
        from: input.settings.emailFrom.trim(),
        to: input.settings.emailTo
          .split(/[\n\r,;]+/)
          .map((value) => value.trim())
          .filter(Boolean),
        use_tls: input.settings.emailUseTLS,
      },
    },
    scheduler: {
      auto_dns_push: input.settings.schedulerAutoDnsPush,
      auto_github_export: input.settings.schedulerAutoGithubExport,
      config_source: "draft_preferred",
      daily_times: input.settings.schedulerTriggerMode === "daily" ? schedulerDailyTimesFromText(input.settings.schedulerDailyTimes) : [],
      enabled: input.settings.schedulerEnabled,
      interval_minutes: input.settings.schedulerTriggerMode === "interval" ? schedulerIntervalMinutesForSnapshot(input.settings, input.settings.schedulerEnabled) : 0,
      post_run_source_profile_action: "update_recent_run_source_profile",
      skip_if_active: input.settings.schedulerSkipIfActive,
    },
    source_ip_mode: input.sourceIPMode,
    sources: input.sources.map((source) => ({
      ...source,
      status_text: source.status_text.trim(),
    })),
  };
}

export function stableSnapshotValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((entry) => stableSnapshotValue(entry));
  }
  if (value && typeof value === "object") {
    return Object.keys(value as Record<string, unknown>)
      .sort()
      .reduce<Record<string, unknown>>((acc, key) => {
        acc[key] = stableSnapshotValue((value as Record<string, unknown>)[key]);
        return acc;
      }, {});
  }
  return value;
}

export function snapshotSignature(snapshot: unknown) {
  return JSON.stringify(stableSnapshotValue(snapshot));
}
