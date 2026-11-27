import type { ColoFilterMode, TelegramRecipientMode } from "../../lib/bridge";

export interface SettingsUpdateState {
  assetName: string;
  checkedAt: string;
  dockerImage: string;
  downloadPath: string;
  installStarted: boolean;
  installMode: string;
  installing: boolean;
  latestVersion: string;
  message: string;
  nextAction: string;
  releaseUrl: string;
  status: "idle" | "checking" | "available" | "latest" | "installing" | "ready" | "failed";
  updateAvailable: boolean;
}

export interface SettingsTimestampFormatOptions {
  fallback?: string;
  includeDate?: boolean;
  includeOffset?: boolean;
  includeSeconds?: boolean;
}

export type SettingsSectionKey = "updates" | "viewport" | "appearance" | "storage" | "colo" | "backup" | "sources" | "cloudflare" | "probe" | "scheduler" | "export" | "github" | "postPush" | "upload" | "protection" | "debug" | "telegram" | "tls" | "dns" | "webhook";

export interface SettingsSectionController {
  isOpen: (section: SettingsSectionKey) => boolean;
  isMounted: (section: SettingsSectionKey) => boolean;
  syncOpen: (section: SettingsSectionKey, event: Event) => void;
  pinnedHelpSection: string | null;
  toggleHelp: (key: string) => void;
}

export const REQUEST_HEADERS_TEMPLATE = ["Accept: */*", "Accept-Language: zh-CN,zh;q=0.9,en;q=0.8", "Cache-Control: no-cache", "Pragma: no-cache", "DNT: 1", "Sec-Fetch-Dest: empty", "Sec-Fetch-Mode: cors", "Sec-Fetch-Site: none"].join("\n");

export const DOH_PRESETS = [
  { id: "alidns", label: "阿里 DoH（dns.alidns.com）", url: "https://dns.alidns.com/dns-query" },
  { id: "dohpub", label: "DoH.pub（sm2.doh.pub）", url: "https://sm2.doh.pub/dns-query" },
  { id: "cloudflare", label: "Cloudflare（cloudflare-dns.com）", url: "https://cloudflare-dns.com/dns-query" },
] as const;

export const DOH_PRESET_SHORT_LABELS: Record<string, string> = { alidns: "阿里 DoH", cloudflare: "Cloudflare", dohpub: "DoH.pub" };

export const TTL_OPTIONS = [
  { label: "1分钟", value: 60 },
  { label: "5分钟", value: 300 },
  { label: "10分钟", value: 600 },
];

export function coloModeLabel(mode: ColoFilterMode) {
  return mode === "deny" ? "黑名单" : "白名单";
}

export function statusText(value: string) {
  const labels: Record<string, string> = {
    aborted: "熔断/终止",
    cancelled: "已终止",
    completed: "完成",
    failed: "失败",
    partial: "部分成功",
    running: "执行中",
    skipped: "跳过",
    unsupported: "未接入",
  };
  return value ? labels[value] || value : "未运行";
}

export function workflowLabel(value: string) {
  const labels: Record<string, string> = {
    aborted: "已终止",
    cancelled: "已终止",
    completed: "完成",
    draft: "草稿配置",
    draft_preferred: "草稿优先",
    formal: "正式配置",
    post_run_source_profiles: "更新最近运行输入组",
    saved: "正式配置",
    skipped: "跳过",
    update_recent_run_source_profile: "更新最近运行输入组",
  };
  return value ? labels[value] || value : "-";
}

export function telegramRecipientModeText(mode: TelegramRecipientMode) {
  const labels: Record<TelegramRecipientMode, string> = {
    both: "个人+群组/频道",
    chat: "群组/频道",
    personal: "仅个人",
  };
  return labels[mode] || labels.chat;
}

export function isMaskedSecretValue(value: string) {
  return value.includes("...") || value.includes("***") || /^\*+$/.test(value);
}

export function formatSettingsTimestamp(formatTimestamp: (value: string, options?: SettingsTimestampFormatOptions) => string, value: string, fallback = "未记录时间") {
  return value.trim() ? formatTimestamp(value) : fallback;
}
