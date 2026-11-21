// bridge 类型门面：实现按域拆到 `types/*`，这里保持原有的 `./types` 导入路径与导出集合不变。
export type { AppInfo, CommandResult, PathSelectionPayload, RuntimeDiagnostics, StorageHealth, StorageStatus, TraceDiagnosticSample, TraceDiagnostics, UpdateInfo, UpdateInstallResult } from "./types/core";
export type { AndroidBatteryStatus, AndroidKeepAliveStatus, AndroidNotificationPermissionStatus, AndroidRuntimeStatus } from "./types/android";
export type { ColoDictionaryStatus, SourceConfig, SourcePreviewPayload, SourcePreviewSummary } from "./types/sources";
export type { ProbeNumericTriple, ProbeStageLimits, ProbeThresholds, ProbeTimeouts } from "./types/probe";
export type { DerivedTaskState, ExportRecord, MCISProgress, ProbeEventEnvelope, ProbeResult, ProbeRunResultPayload, TaskProgress, TaskResultPage, TaskSnapshot, TaskSnapshotList, UploadNotification, UploadNotificationTopEntry } from "./types/task";
export type { CloudflareRoutingRuleSnapshot, DNSProviderSnapshot, DNSSubdomainListResult, DNSSubdomainSnapshot, DnsReadProviderOption, DnsRecordSnapshot, DnsSelfTestProviderOption, DnsSelfTestResult, DnsSelfTestStage } from "./types/dns";
export type { ConfigSnapshot, GitHubConfigSnapshot, SchedulerStatus, SourceProfileItem, SourceProfileStore, SourceProfileUpdatePayload } from "./types/config";
export type { CSVEncoding, ColoFilterMode, DNSProviderKind, DebugLogMode, DebugLogVerbosity, DownloadHTTPProtocol, DownloadSpeedMetric, NotificationProxyMode, ProbeResultIPFilter, ProbeResultOrder, ProbeResultSortBy, ProbeStrategy, SourceColoFilterPhase, SourceIPMode, SourceKind, TaskTone, TelegramRecipientMode, ThemeMode, TraceColoMode } from "./types/enums";
