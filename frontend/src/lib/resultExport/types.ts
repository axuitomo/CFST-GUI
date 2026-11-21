// 结果导出域类型契约（重构计划 §4.2 二次拆解）。
import type { Ref } from "vue";
import type { ProbeResult, TaskTone } from "../bridge";
import type { DnsProviderForm, SettingsForm } from "../settingsForm";
import type { HistoryEntry, ResultCloudflareRecordType, ToastTone, ViewName } from "../appTypes";
import type { buildConfigSnapshot as buildConfigSnapshotFrom } from "../configSnapshotBuild";

export interface ResultCloudflarePushSettings {
  providerId: string;
  recordName: string;
  recordType: ResultCloudflareRecordType;
  topN: number;
}

export interface ResultExportDeps {
  settings: SettingsForm;
  task: { exportPath: string; taskId: string };
  isAndroidApp: () => boolean;
  enabledDNSProviders: () => DnsProviderForm[];
  primaryCloudflareProvider: () => DnsProviderForm | undefined;
  dnsProviderTargetLabel: (entry: DnsProviderForm) => string;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  currentVisibleResultRows: () => ProbeResult[];
  pathLeafName: (rawPath: string) => string;
  probeWarnings: Ref<string[]>;
  pushWarningTrace: (warnings: string[], ts?: string) => void;
  navigateTo: (view: ViewName) => void;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  pushActivity: (title: string, detail: string) => void;
}

export type ExportHistoryEntry = HistoryEntry;
