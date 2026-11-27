// 结果页跨批次历史(近 30 天)与展示管道:时间范围 → IP 去重 → IP/国家筛选 → 排序。
// 历史本身由后端持久化(appcore result_history:合并、过期、上限都在那边),
// 桌面/WebUI/Android 因此共用同一份;这里只保留展示逻辑与旧版 localStorage 的迁移读取。
import { importResultHistory } from "../bridge";
import type { CommandResult, ProbeResult, ProbeResultIPFilter, ProbeResultOrder, ProbeResultSortBy, ResultHistoryMergeResult, ResultHistoryRecord } from "../bridge";
import { coloCountryCode, countryName } from "./coloCountry";

export type { ResultHistoryRecord };

/** 旧版(≤1.0)历史的存储键:只在首启迁移时读取一次,新版本不再写入 localStorage。 */
export const RESULT_HISTORY_STORAGE_KEY = "cfst.result.history.v1";
// 后端按同一保留期清理(appcore resultHistoryRetentionDays),两端改动必须同步。
export const RESULT_HISTORY_RETENTION_DAYS = 30;
// 国家筛选里"未知地区"的取值;真实国家码是两位字母,不会冲突。空串表示不筛选。
export const UNKNOWN_COUNTRY_FILTER = "-";

const DAY_MS = 24 * 60 * 60 * 1000;

export type ResultTimeRange = "current" | "1d" | "7d" | "15d" | "30d" | "all";

export const RESULT_TIME_RANGE_OPTIONS: Array<{ label: string; value: ResultTimeRange }> = [
  { label: "本次结果", value: "current" },
  { label: "近 1 天", value: "1d" },
  { label: "近 7 天", value: "7d" },
  { label: "近 15 天", value: "15d" },
  { label: "近 30 天", value: "30d" },
  { label: "全部 30 天", value: "all" },
];

export interface ResultDisplayRow extends ProbeResult {
  /** 所选周期内同一去重键(IP 或 IP:Port)的测速次数。 */
  batch_count?: number;
  /** 该行的测速时间(毫秒时间戳)。 */
  measured_at?: number;
}

export interface ResultCountryOption {
  label: string;
  value: string;
}

export interface ResultDisplayOptions {
  countryFilter: string;
  currentBatchId: string;
  currentBatchTimestamp?: number;
  currentRows: ProbeResult[];
  /** 后端历史的本地镜像,由调用方在切换历史范围或写入新批次后刷新。 */
  historyRecords: ResultHistoryRecord[];
  ipFilter: ProbeResultIPFilter;
  mergeDuplicates: boolean;
  now: number;
  order: ProbeResultOrder;
  range: ResultTimeRange;
  sortBy: ProbeResultSortBy;
}

export interface ResultDisplay {
  countryOptions: ResultCountryOption[];
  rows: ResultDisplayRow[];
}

function legacyStorage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null;
  }
}

/**
 * 读旧版存在 localStorage 里的历史,仅供迁移使用。
 * 单条约 240–300B:旧版按 5MB 配额留余量,上限 12000 条。
 */
export function readLegacyResultHistory(): ResultHistoryRecord[] {
  const store = legacyStorage();
  if (!store) {
    return [];
  }
  let raw: string | null = null;
  try {
    raw = store.getItem(RESULT_HISTORY_STORAGE_KEY);
  } catch {
    return [];
  }
  if (!raw) {
    return [];
  }
  return parseLegacyResultHistory(raw);
}

/** 迁移成功后删掉旧键;删不掉只会让下次启动重复导入,后端按键去重不会产生重复记录。 */
export function clearLegacyResultHistory() {
  const store = legacyStorage();
  if (!store) {
    return;
  }
  try {
    store.removeItem(RESULT_HISTORY_STORAGE_KEY);
  } catch {
    // 隐私模式等拒绝删除的场景保持原样,下次启动重试。
  }
}

/**
 * 一次性迁移:把旧版 localStorage 历史交给后端并删掉旧键。
 * 返回导入命令的结果;没有旧数据时返回 null。失败时保留旧键,下次启动重试,
 * 后端按合并键与时间戳去重,重复导入不会产生重复记录。
 */
export async function migrateLegacyResultHistory(): Promise<CommandResult<ResultHistoryMergeResult> | null> {
  const records = readLegacyResultHistory();
  if (records.length === 0) {
    return null;
  }
  const result = await importResultHistory(records);
  if (result.ok) {
    clearLegacyResultHistory();
  }
  return result;
}

function finiteOrNull(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function portOrNull(value: unknown): number | null {
  const port = finiteOrNull(value);
  return port !== null && port > 0 ? Math.trunc(port) : null;
}

function normalizeRecord(value: unknown): ResultHistoryRecord | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  const record = value as Record<string, unknown>;
  const address = typeof record.address === "string" ? record.address.trim() : "";
  const timestamp = finiteOrNull(record.timestamp);
  if (!address || timestamp === null) {
    return null;
  }
  return {
    address,
    batch_id: typeof record.batch_id === "string" ? record.batch_id : "",
    colo: typeof record.colo === "string" ? record.colo.trim().toUpperCase() : "",
    download_mbps: finiteOrNull(record.download_mbps),
    max_download_mbps: finiteOrNull(record.max_download_mbps),
    source_port: portOrNull(record.source_port),
    stage_status: typeof record.stage_status === "string" && record.stage_status.trim() ? record.stage_status : "completed",
    tcp_latency_ms: finiteOrNull(record.tcp_latency_ms),
    test_port: portOrNull(record.test_port),
    timestamp,
    trace_latency_ms: finiteOrNull(record.trace_latency_ms),
  };
}

function parseLegacyResultHistory(raw: string): ResultHistoryRecord[] {
  let parsed: unknown = null;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return [];
  }
  if (!Array.isArray(parsed)) {
    return [];
  }
  return parsed.map(normalizeRecord).filter((record): record is ResultHistoryRecord => record !== null);
}

export function historyRangeStart(range: ResultTimeRange, now: number): number {
  if (range === "1d") {
    return now - DAY_MS;
  }
  if (range === "7d") {
    return now - 7 * DAY_MS;
  }
  if (range === "15d") {
    return now - 15 * DAY_MS;
  }
  return now - RESULT_HISTORY_RETENTION_DAYS * DAY_MS;
}

function recordToRow(record: ResultHistoryRecord): ResultDisplayRow {
  return {
    address: record.address,
    colo: record.colo || null,
    download_mbps: record.download_mbps,
    export_status: "",
    max_download_mbps: record.max_download_mbps,
    note: null,
    source_port: record.source_port,
    stage_status: record.stage_status,
    tcp_latency_ms: record.tcp_latency_ms,
    test_port: record.test_port,
    trace_latency_ms: record.trace_latency_ms,
    measured_at: record.timestamp,
  };
}

export function rowCountryCode(row: ProbeResult): string {
  return coloCountryCode(row.colo) || UNKNOWN_COUNTRY_FILTER;
}

function matchesIpFilter(address: string, filter: ProbeResultIPFilter): boolean {
  if (filter === "ipv4") {
    return !address.includes(":");
  }
  if (filter === "ipv6") {
    return address.includes(":");
  }
  return true;
}

// 同一 IP 在同批次可能按输入源端口/测速端口重复出现;只有多端口场景才把端口并入去重键。
function dedupeKey(row: ResultDisplayRow, withPort: boolean): string {
  if (!withPort) {
    return row.address;
  }
  const port = portOrNull(row.test_port) ?? portOrNull(row.source_port);
  return port === null ? row.address : `${row.address}:${port}`;
}

function dedupeLatest(rows: ResultDisplayRow[], keyOf: (row: ResultDisplayRow) => string): ResultDisplayRow[] {
  const latest = new Map<string, ResultDisplayRow>();
  for (const row of rows) {
    const key = keyOf(row);
    const existing = latest.get(key);
    if (!existing || (row.measured_at ?? 0) >= (existing.measured_at ?? 0)) {
      latest.set(key, row);
    }
  }
  return [...latest.values()];
}

const SORT_ACCESSORS: Record<ProbeResultSortBy, (row: ProbeResult) => number | null> = {
  tcp: (row) => finiteOrNull(row.tcp_latency_ms),
  trace: (row) => finiteOrNull(row.trace_latency_ms),
  download: (row) => finiteOrNull(row.download_mbps),
  max_download: (row) => finiteOrNull(row.max_download_mbps),
};

function sortRows(rows: ResultDisplayRow[], sortBy: ProbeResultSortBy, order: ProbeResultOrder): ResultDisplayRow[] {
  const accessor = SORT_ACCESSORS[sortBy] ?? SORT_ACCESSORS.download;
  const direction = order === "asc" ? 1 : -1;
  return [...rows].sort((left, right) => {
    const leftValue = accessor(left);
    const rightValue = accessor(right);
    if (leftValue === null && rightValue === null) {
      return (right.measured_at ?? 0) - (left.measured_at ?? 0);
    }
    // 缺失值永远排在最后,避免"最快"列表被空数据占据。
    if (leftValue === null) {
      return 1;
    }
    if (rightValue === null) {
      return -1;
    }
    if (leftValue === rightValue) {
      return (right.measured_at ?? 0) - (left.measured_at ?? 0);
    }
    return leftValue < rightValue ? -direction : direction;
  });
}

function buildCountryOptions(rows: ResultDisplayRow[]): ResultCountryOption[] {
  const counts = new Map<string, number>();
  for (const row of rows) {
    const code = rowCountryCode(row);
    counts.set(code, (counts.get(code) ?? 0) + 1);
  }
  return [...counts.entries()]
    .sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))
    .map(([code, count]) => ({
      label: `${code === UNKNOWN_COUNTRY_FILTER ? "未知地区" : countryName(code)} (${count})`,
      value: code,
    }));
}

/**
 * 生成结果页最终展示行:本次结果沿用后端已排序分页的数据;
 * 跨批次范围合并后端历史镜像与当前批次,按 IP(多端口场景按 IP:Port)保留最新记录。
 */
export function buildResultDisplay(options: ResultDisplayOptions): ResultDisplay {
  const currentRows = options.currentRows.map((row) => ({ ...row, measured_at: finiteOrNull(options.currentBatchTimestamp) ?? options.now }));
  if (options.range === "current") {
    return {
      countryOptions: buildCountryOptions(currentRows),
      rows: currentRows.filter((row) => !options.countryFilter || rowCountryCode(row) === options.countryFilter),
    };
  }

  const start = historyRangeStart(options.range, options.now);
  const batchId = options.currentBatchId.trim();
  const historyRows = options.historyRecords.filter((record) => record.timestamp >= start && (!batchId || record.batch_id !== batchId)).map(recordToRow);
  const merged = [...historyRows, ...currentRows];
  const scoped = merged.filter((row) => matchesIpFilter(row.address, options.ipFilter));
  const ports = new Set(scoped.map((row) => portOrNull(row.test_port)).filter((port) => port !== null));
  const withPort = ports.size > 1;
  const keyOf = (row: ResultDisplayRow) => dedupeKey(row, withPort);
  const occurrences = new Map<string, number>();
  for (const row of scoped) {
    occurrences.set(keyOf(row), (occurrences.get(keyOf(row)) ?? 0) + 1);
  }
  const selected = options.mergeDuplicates ? dedupeLatest(scoped, keyOf) : scoped;
  const rows = sortRows(selected, options.sortBy, options.order).map((row) => ({ ...row, batch_count: occurrences.get(keyOf(row)) ?? 1 }));
  return {
    countryOptions: buildCountryOptions(selected),
    rows: rows.filter((row) => !options.countryFilter || rowCountryCode(row) === options.countryFilter),
  };
}
