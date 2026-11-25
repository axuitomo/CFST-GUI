// 结果页「一键复制 IP」的 Top-N 设置（复用 localStorage 偏好层）与复制队列生成。
export const RESULT_COPY_SETTINGS_KEY = "cfst.result.copySettings.v1";
export const RESULT_COPY_CUSTOM_MAX = 9999;

export type ResultCopyTopMode = "all" | "5" | "10" | "15" | "custom";

export const RESULT_COPY_TOP_OPTIONS: Array<{ label: string; value: ResultCopyTopMode }> = [
  { label: "不限", value: "all" },
  { label: "Top 5", value: "5" },
  { label: "Top 10", value: "10" },
  { label: "Top 15", value: "15" },
  { label: "自定义", value: "custom" },
];

export interface ResultCopySettings {
  customCount: number;
  mode: ResultCopyTopMode;
}

const DEFAULT_SETTINGS: ResultCopySettings = { customCount: 10, mode: "all" };

function normalizeMode(value: unknown): ResultCopyTopMode {
  return RESULT_COPY_TOP_OPTIONS.some((option) => option.value === value) ? (value as ResultCopyTopMode) : DEFAULT_SETTINGS.mode;
}

export function normalizeCopyCount(value: unknown): number {
  const parsed = Number.parseInt(String(value ?? ""), 10);
  if (!Number.isFinite(parsed) || parsed < 1) {
    return DEFAULT_SETTINGS.customCount;
  }
  return Math.min(parsed, RESULT_COPY_CUSTOM_MAX);
}

export function readResultCopySettings(): ResultCopySettings {
  try {
    const raw = window.localStorage.getItem(RESULT_COPY_SETTINGS_KEY);
    if (!raw) {
      return { ...DEFAULT_SETTINGS };
    }
    const parsed = JSON.parse(raw) as Partial<ResultCopySettings>;
    return { customCount: normalizeCopyCount(parsed.customCount), mode: normalizeMode(parsed.mode) };
  } catch {
    return { ...DEFAULT_SETTINGS };
  }
}

export function saveResultCopySettings(settings: ResultCopySettings) {
  try {
    window.localStorage.setItem(RESULT_COPY_SETTINGS_KEY, JSON.stringify({ customCount: normalizeCopyCount(settings.customCount), mode: normalizeMode(settings.mode) }));
  } catch {
    // 隐私模式或存储被禁用时只保留本次会话的选择。
  }
}

/** 0 表示不限制条数。 */
export function resolveCopyLimit(settings: ResultCopySettings): number {
  if (settings.mode === "all") {
    return 0;
  }
  if (settings.mode === "custom") {
    return normalizeCopyCount(settings.customCount);
  }
  return Number(settings.mode);
}

/** 按当前展示顺序截取，同一 IP 只输出一次，一行一个。 */
export function buildCopyQueue(rows: Array<{ address: string }>, settings: ResultCopySettings): string[] {
  const limit = resolveCopyLimit(settings);
  const queue: string[] = [];
  const seen = new Set<string>();
  for (const row of rows) {
    const address = (row.address || "").trim();
    if (!address || seen.has(address)) {
      continue;
    }
    seen.add(address);
    queue.push(address);
    if (limit > 0 && queue.length >= limit) {
      break;
    }
  }
  return queue;
}
