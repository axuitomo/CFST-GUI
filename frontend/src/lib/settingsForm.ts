// 设置域共享契约（重构计划 §4.1）：SettingsForm / DnsProviderForm / CloudflareRoutingRuleForm
// 与设置默认值的唯一来源。App.vue 与 SettingsView.vue 原本各持一份，现统一从这里导入。
// 字段类型与 bridge/types 逐字一致；DNSProviderKind 只 re-export，不新造副本。
import type { ColoFilterMode, CSVEncoding, DebugLogMode, DebugLogVerbosity, DNSProviderKind, DownloadSpeedMetric, ProbeStrategy, SourceColoFilterPhase, TelegramRecipientMode, TraceColoMode } from "./bridge";
import { DEFAULT_UTC_OFFSET_MINUTES } from "./time";

export type { DNSProviderKind } from "./bridge";

export type SchedulerTriggerMode = "interval" | "daily";

export interface CloudflareRoutingRuleForm {
  enabled: boolean;
  filterMode: "allow" | "deny";
  filterTokens: string;
  id: string;
  name: string;
  recordName: string;
  recordType: "A" | "AAAA" | "ALL";
  topN: number;
}

export interface DnsProviderForm {
  allowCNAME: boolean;
  apiKey: string;
  apiSecret: string;
  apiToken: string;
  enabled: boolean;
  id: string;
  label: string;
  maskedKey: string;
  maskedSecret: string;
  maskedToken: string;
  provider: DNSProviderKind;
  recordName: string;
  recordType: "A" | "AAAA" | "ALL";
  routingEnabled: boolean;
  routingRules: CloudflareRoutingRuleForm[];
  subdomain: string;
  subdomainId: number;
  topN: number;
  ttl: number;
  zoneId: string;
}

export interface SettingsForm {
  comment: string;
  dnsProviders: DnsProviderForm[];
  postProbePushCloudflareEnabled: boolean;
  postProbePushGitHubEnabled: boolean;
  telegramAPIBaseURL: string;
  telegramBotToken: string;
  telegramChatId: string;
  telegramIncludeTopN: boolean;
  telegramNotificationEnabled: boolean;
  telegramPersonalChatId: string;
  telegramTopNRecipientMode: TelegramRecipientMode;
  telegramTopN: number;
  telegramUseSystemProxy: boolean;
  telegramUploadRecipientMode: TelegramRecipientMode;
  webhookEnabled: boolean;
  webhookURLs: string;
  webhookUseSystemProxy: boolean;
  webhookDingTalkURL: string;
  webhookDingTalkSecret: string;
  webhookWeComURLs: string;
  webhookWeComCorpID: string;
  webhookWeComAgentID: string;
  webhookWeComSecret: string;
  webhookWeComMobiles: string;
  webhookHeadersJSON: string;
  emailEnabled: boolean;
  emailHost: string;
  emailPort: number;
  emailUsername: string;
  emailPassword: string;
  emailFrom: string;
  emailTo: string;
  emailUseTLS: boolean;
  uploadGitHubTopN: number;
  uploadSharedFilterColoAllow: string;
  uploadSharedFilterColoDeny: string;
  uploadSharedFilterEnabled: boolean;
  uploadSharedFilterIPVersion: "any" | "ipv4" | "ipv6";
  uploadSharedFilterMaxLossRate: number | null;
  uploadSharedFilterMaxTcpLatencyMs: number | null;
  uploadSharedFilterMaxTraceLatencyMs: number | null;
  uploadSharedFilterMinDownloadMbps: number;
  uploadSharedFilterStatus: "all" | "passed";
  exportFileName: string;
  exportFileNameTemplate: string;
  githubBranch: string;
  githubCSVHeaderTemplate: string;
  githubCSVRowTemplate: string;
  githubCommitMessageTemplate: string;
  githubExportEnabled: boolean;
  githubFormat: "csv" | "txt";
  githubLastExportAt: string;
  githubOwner: string;
  githubPathTemplate: string;
  githubRepo: string;
  githubToken: string;
  githubTXTRowTemplate: string;
  exportCSVEncoding: CSVEncoding;
  exportOverwrite: string;
  exportTargetDir: string;
  exportTargetUri: string;
  maxTcpLatencyMs: number | null;
  maxLossRate: number;
  minDownloadMbps: number;
  minDelayMs: number;
  probeDebug: boolean;
  probeDebugCaptureAddress: string;
  probeDebugCaptureEnabled: boolean;
  probeDebugLogFormat: string;
  probeDebugLogMode: DebugLogMode;
  probeDebugLogVerbosity: DebugLogVerbosity;
  probeDisableDownload: boolean;
  probeConcurrencyStage1: number;
  probeConcurrencyStage2: number;
  probeConcurrencyStage3: number;
  probeMCISBudget: number;
  probeMCISConcurrency: number;
  probeCooldownFailures: number;
  probeCooldownMs: number;
  probeDownloadBufferKB: number;
  probeDownloadSuccessLimit: number;
  probeDownloadGetConcurrency: number;
  probeDownloadHostHeader: string;
  probeDownloadHTTPProtocol: "auto" | "h1" | "h2" | "h3";
  probeDownloadSNI: string;
  probeDownloadSpeedMetric: DownloadSpeedMetric;
  probeDownloadSpeedSampleIntervalMs: number;
  probeDownloadTimeSeconds: number;
  probeDownloadWarmupSeconds: number;
  probeEventThrottleMs: number;
  probeHostHeader: string;
  probeTraceCfColo: string;
  probeTraceCfColoMode: ColoFilterMode;
  probePingTimes: number;
  probePrintNum: number;
  probePortPolicy: "source_override_global" | "fixed_global";
  probeRetryBackoffMs: number;
  probeRetryMaxAttempts: number;
  probeRequestHeaders: string;
  probeSNI: string;
  probeSourcePoolLimit: number;
  probeVerifyTLSCertificate: boolean;
  probeSourceColoFilterPhase: SourceColoFilterPhase;
  probeStageLimitStage2: number;
  probeStageLimitStage3: number;
  probeStrategy: ProbeStrategy;
  probeTcpPort: number;
  probeTimeoutStage1Ms: number;
  probeTimeoutStage2Ms: number;
  probeTraceColoMode: TraceColoMode;
  probeTraceURL: string;
  probeDoHURL: string;
  probeECH: boolean;
  probeECHConfig: string;
  probeECHDomain: string;
  probeURL: string;
  probeUserAgent: string;
  proxied: boolean;
  schedulerAutoDnsPush: boolean;
  schedulerAutoGithubExport: boolean;
  schedulerDailyTimes: string;
  schedulerEnabled: boolean;
  schedulerIntervalMinutes: number;
  schedulerIntervalMinutesDraft: number;
  schedulerSkipIfActive: boolean;
  schedulerTriggerMode: SchedulerTriggerMode;
  maintenanceCompletedTaskRetentionDays: number;
  sourceAutoDetectName: boolean;
  themeDarkStart: string;
  themeLightStart: string;
  themeMode: "light" | "dark" | "auto_system_time" | "auto_time";
  utcOffsetMinutes: number;
  webdavEnabled: boolean;
  webdavLastBackupAt: string;
  webdavLastRestoreAt: string;
  webdavPassword: string;
  webdavRemotePath: string;
  webdavServerURL: string;
  webdavTimeoutSeconds: number;
  webdavUsername: string;
}

export const DEFAULT_PROBE_USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0";
export const DEFAULT_FILE_TEST_URL = "https://speed.cloudflare.com/__down?bytes=99999999";
export const DEFAULT_DOH_URL = "https://dns.alidns.com/dns-query";
export const DEFAULT_ECH_DOMAIN = "cloudflare-ech.com";
export const DEFAULT_DEBUG_LOG_FORMAT = "{ts} [{level}] {event} task={task_id} stage={stage} {message}";
export const DEFAULT_SOURCE_POOL_LIMIT = 65536;
export const MAX_SOURCE_POOL_LIMIT = 1000000;
export const DEFAULT_CLOUDFLARE_TTL = 300;
export const DEFAULT_DNSHE_TTL = 600;
export const DEFAULT_ALIYUN_TTL = 600;
export const DEFAULT_DNSPOD_TTL = 600;
export const MIN_PROBE_PING_TIMES = 2;
export const DEFAULT_MAX_LOSS_RATE = 0.15;
export const MAX_LOSS_RATE = 1;
export const DEFAULT_HTTPING_STATUS_CODE = 200;

// 与后端 DeriveTraceURL 对齐：把文件测速URL的路径换成 /cdn-cgi/trace。
// 切换「追踪URL」时用它填入标准模式实际使用的端点，避免切过去只剩空地址。
export function deriveTraceURL(rawURL: string) {
  try {
    const url = new URL(rawURL.trim());
    url.pathname = "/cdn-cgi/trace";
    url.search = "";
    url.hash = "";
    return url.toString();
  } catch {
    return "";
  }
}

// 设置表单默认值：每次调用返回全新对象，供 reactive() 初始化使用。
export function createDefaultSettingsForm(): SettingsForm {
  return {
    comment: "",
    dnsProviders: [],
    postProbePushCloudflareEnabled: true,
    postProbePushGitHubEnabled: false,
    telegramAPIBaseURL: "",
    telegramBotToken: "",
    telegramChatId: "",
    telegramIncludeTopN: false,
    telegramNotificationEnabled: false,
    telegramPersonalChatId: "",
    telegramTopNRecipientMode: "chat",
    telegramTopN: 5,
    telegramUseSystemProxy: false,
    telegramUploadRecipientMode: "chat",
    webhookEnabled: false,
    webhookURLs: "",
    webhookUseSystemProxy: false,
    webhookDingTalkURL: "",
    webhookDingTalkSecret: "",
    webhookWeComURLs: "",
    webhookWeComCorpID: "",
    webhookWeComAgentID: "",
    webhookWeComSecret: "",
    webhookWeComMobiles: "",
    webhookHeadersJSON: "{}",
    emailEnabled: false,
    emailHost: "",
    emailPort: 587,
    emailUsername: "",
    emailPassword: "",
    emailFrom: "",
    emailTo: "",
    emailUseTLS: true,
    uploadGitHubTopN: 20,
    uploadSharedFilterColoAllow: "",
    uploadSharedFilterColoDeny: "",
    uploadSharedFilterEnabled: false,
    uploadSharedFilterIPVersion: "any",
    uploadSharedFilterMaxLossRate: null,
    uploadSharedFilterMaxTcpLatencyMs: null,
    uploadSharedFilterMaxTraceLatencyMs: null,
    uploadSharedFilterMinDownloadMbps: 0,
    uploadSharedFilterStatus: "passed",
    exportFileName: "",
    exportFileNameTemplate: "",
    githubBranch: "main",
    githubCSVHeaderTemplate: "",
    githubCSVRowTemplate: "",
    githubCommitMessageTemplate: "CFST results {date} {time}",
    githubExportEnabled: false,
    githubFormat: "csv",
    githubLastExportAt: "",
    githubOwner: "axuitomo",
    githubPathTemplate: "cfst-results/{date}/{time}-{task_id}.csv",
    githubRepo: "CFST-GUI",
    githubToken: "",
    githubTXTRowTemplate: "{ip}",
    exportCSVEncoding: "utf-8",
    exportOverwrite: "replace_on_start",
    exportTargetDir: "",
    exportTargetUri: "",
    maxTcpLatencyMs: null,
    maxLossRate: DEFAULT_MAX_LOSS_RATE,
    minDownloadMbps: 0,
    minDelayMs: 0,
    probeDebug: false,
    probeDebugCaptureAddress: "",
    probeDebugCaptureEnabled: false,
    probeDebugLogFormat: "",
    probeDebugLogMode: "structured",
    probeDebugLogVerbosity: "detailed",
    probeDisableDownload: true,
    probeConcurrencyStage1: 200,
    probeConcurrencyStage2: 6,
    probeConcurrencyStage3: 1,
    probeCooldownFailures: 3,
    probeCooldownMs: 250,
    probeDownloadBufferKB: 256,
    probeDownloadSuccessLimit: 0,
    probeDownloadGetConcurrency: 4,
    probeDownloadHostHeader: "",
    probeDownloadHTTPProtocol: "auto",
    probeDownloadSNI: "",
    probeDownloadSpeedMetric: "average",
    probeDownloadSpeedSampleIntervalMs: 1000,
    probeDownloadTimeSeconds: 4,
    probeDownloadWarmupSeconds: 1,
    probeEventThrottleMs: 100,
    probeHostHeader: "",
    probeTraceCfColo: "",
    probeTraceCfColoMode: "allow",
    probePingTimes: 4,
    probePrintNum: 0,
    probePortPolicy: "fixed_global",
    probeRetryBackoffMs: 0,
    probeRetryMaxAttempts: 0,
    probeRequestHeaders: "",
    probeSNI: "",
    probeSourcePoolLimit: DEFAULT_SOURCE_POOL_LIMIT,
    probeVerifyTLSCertificate: false,
    probeSourceColoFilterPhase: "precheck",
    probeStageLimitStage2: 0,
    probeStageLimitStage3: 10,
    probeMCISBudget: 0,
    probeMCISConcurrency: 0,
    probeStrategy: "fast",
    probeTcpPort: 443,
    probeTimeoutStage1Ms: 1000,
    probeTimeoutStage2Ms: 1000,
    probeTraceColoMode: "standard",
    probeTraceURL: "",
    probeDoHURL: DEFAULT_DOH_URL,
    probeECH: true,
    probeECHConfig: "",
    probeECHDomain: DEFAULT_ECH_DOMAIN,
    probeURL: DEFAULT_FILE_TEST_URL,
    probeUserAgent: DEFAULT_PROBE_USER_AGENT,
    proxied: false,
    schedulerAutoDnsPush: true,
    schedulerAutoGithubExport: true,
    schedulerDailyTimes: "",
    schedulerEnabled: false,
    schedulerIntervalMinutes: 0,
    schedulerIntervalMinutesDraft: 60,
    schedulerSkipIfActive: true,
    schedulerTriggerMode: "interval",
    maintenanceCompletedTaskRetentionDays: 7,
    sourceAutoDetectName: true,
    themeDarkStart: "19:00",
    themeLightStart: "07:00",
    themeMode: "auto_system_time",
    utcOffsetMinutes: DEFAULT_UTC_OFFSET_MINUTES,
    webdavEnabled: false,
    webdavLastBackupAt: "",
    webdavLastRestoreAt: "",
    webdavPassword: "",
    webdavRemotePath: "cfst-gui-config.zip",
    webdavServerURL: "",
    webdavTimeoutSeconds: 30,
    webdavUsername: "",
  };
}
