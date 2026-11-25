// 结果页跨批次历史（近 30 天）与展示管道：时间范围 → IP 去重 → IP/国家筛选 → 排序。
// 存储沿用前端既有的 localStorage 偏好层（同 pushSettings.ts / coloCountry.ts），不新增后端契约与配置字段。
import type { ProbeResult, ProbeResultIPFilter, ProbeResultOrder, ProbeResultSortBy } from "./bridge";
import { coloCountryCode, countryName } from "./coloCountry";

export const RESULT_HISTORY_STORAGE_KEY = "cfst.result.history.v1";
export const RESULT_HISTORY_RETENTION_DAYS = 30;
// localStorage 按字符计约 5MB：实测单条约 240–300B，12000 条留出余量，超出时按时间保留最新记录。
export const RESULT_HISTORY_MAX_RECORDS = 12000;
// 国家筛选里"未知地区"的取值；真实国家码是两位字母，不会冲突。空串表示不筛选。
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

export interface ResultHistoryRecord {
  address: string;
  batch_id: string;
  colo: string;
  download_mbps: number | null;
  max_download_mbps: number | null;
  source_port: number | null;
  stage_status: string;
  tcp_latency_ms: number | null;
  test_port: number | null;
  timestamp: number;
  trace_latency_ms: number | null;
}

export interface ResultDisplayRow extends ProbeResult {
  /** 所选周期内同一去重键（IP 或 IP:Port）的测速次数。 */
  batch_count?: number;
  /** 该行的测速时间（毫秒时间戳）。 */
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

function storage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null;
  }
}

// 解析缓存：展示管道每次重算都会读一遍历史，raw 没变时直接复用已解析记录，省掉整份 JSON.parse。
let cachedRaw: string | null = null;
let cachedRecords: ResultHistoryRecord[] = [];
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

function pruneRecords(records: ResultHistoryRecord[], now: number): ResultHistoryRecord[] {
  const cutoff = now - RESULT_HISTORY_RETENTION_DAYS * DAY_MS;
  const kept = records.filter((record) => record.timestamp >= cutoff);
  if (kept.length <= RESULT_HISTORY_MAX_RECORDS) {
    return kept;
  }
  // 超上限时按时间保留最新的，而不是按存储位置：后写入的旧批次不该挤掉真正较新的记录。
  return kept.sort((left, right) => left.timestamp - right.timestamp).slice(kept.length - RESULT_HISTORY_MAX_RECORDS);
}

function persistRecords(store: Storage, records: ResultHistoryRecord[]): boolean {
  const payload = JSON.stringify(records);
  try {
    store.setItem(RESULT_HISTORY_STORAGE_KEY, payload);
  } catch {
    return false;
  }
  cachedRaw = payload;
  cachedRecords = records;
  return true;
}

export function saveResultHistory(records: ResultHistoryRecord[]) {
  const store = storage();
  if (!store) {
    return;
  }
  // 配额写满时先丢掉较旧的一半再试一次，避免新批次完全落不了盘；仍失败就只保留本次会话内的历史。
  if (!persistRecords(store, records)) {
    persistRecords(store, records.slice(Math.floor(records.length / 2)));
  }
}

/** 读取历史并按 30 天滚动过期过滤；过期记录的物理清理只在写盘时发生，读路径保持无副作用。 */
export function readResultHistory(now = Date.now()): ResultHistoryRecord[] {
  const store = storage();
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
    cachedRaw = null;
    cachedRecords = [];
    return [];
  }
  if (raw !== cachedRaw) {
    cachedRaw = raw;
    cachedRecords = parseStoredRecords(raw);
  }
  return pruneRecords(cachedRecords, now);
}

function parseStoredRecords(raw: string): ResultHistoryRecord[] {
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

function historyRecordFromRow(batchId: string, row: ProbeResult, timestamp: number): ResultHistoryRecord | null {
  const address = (row.address || "").trim();
  const stageStatus = (row.stage_status || "").trim();
  if (!address || stageStatus.includes("failed")) {
    return null;
  }
  return {
    address,
    batch_id: batchId,
    colo: (row.colo || "").trim().toUpperCase(),
    download_mbps: finiteOrNull(row.download_mbps),
    max_download_mbps: finiteOrNull(row.max_download_mbps),
    source_port: portOrNull(row.source_port),
    stage_status: stageStatus || "completed",
    tcp_latency_ms: finiteOrNull(row.tcp_latency_ms),
    test_port: portOrNull(row.test_port),
    timestamp,
    trace_latency_ms: finiteOrNull(row.trace_latency_ms),
  };
}

function historyKey(record: ResultHistoryRecord): string {
  return [record.batch_id, record.address, record.source_port ?? "", record.test_port ?? "", record.stage_status].join("\u0000");
}

function sameHistoryRecord(existing: ResultHistoryRecord | undefined, next: ResultHistoryRecord): boolean {
  return (
    existing !== undefined &&
    existing.address === next.address &&
    existing.batch_id === next.batch_id &&
    existing.colo === next.colo &&
    existing.download_mbps === next.download_mbps &&
    existing.max_download_mbps === next.max_download_mbps &&
    existing.source_port === next.source_port &&
    existing.stage_status === next.stage_status &&
    existing.tcp_latency_ms === next.tcp_latency_ms &&
    existing.test_port === next.test_port &&
    // 时间戳故意不参与比较：任务运行中的快照时间不断变化，比时间会让每次刷新都全量重写。
    existing.trace_latency_ms === next.trace_latency_ms
  );
}

/**
 * 写入一个测速批次的有效结果，按「批次 + IP + 端口 + 阶段」与已存记录合并：
 * 分页、排序、IP 或时间范围筛选只会带来同批次的子集，不能把已存的其它行覆盖掉。
 * 内容完全未变时跳过整份重写，避免每次刷新都全量落盘。
 */
export function recordResultHistoryBatch(batchId: string, rows: ProbeResult[], now = Date.now()) {
  const normalizedBatchId = (batchId || "").trim();
  if (!normalizedBatchId || rows.length === 0) {
    return;
  }
  const fresh = rows.map((row) => historyRecordFromRow(normalizedBatchId, row, now)).filter((record): record is ResultHistoryRecord => record !== null);
  if (fresh.length === 0) {
    return;
  }
  const retained = readResultHistory(now);
  const merged = new Map<string, ResultHistoryRecord>();
  for (const record of retained) {
    merged.set(historyKey(record), record);
  }
  let changed = false;
  for (const record of fresh) {
    const key = historyKey(record);
    if (sameHistoryRecord(merged.get(key), record)) {
      continue;
    }
    merged.set(key, record);
    changed = true;
  }
  if (!changed && merged.size === retained.length) {
    return;
  }
  saveResultHistory(pruneRecords([...merged.values()], now));
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

// 同一 IP 在同批次可能按输入源端口/测速端口重复出现；只有多端口场景才把端口并入去重键。
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
    // 缺失值永远排在最后，避免"最快"列表被空数据占据。
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
 * 生成结果页最终展示行：本次结果沿用后端已排序分页的数据；
 * 跨批次范围合并本地历史与当前批次，按 IP（多端口场景按 IP:Port）保留最新记录。
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
  const historyRows = readResultHistory(options.now)
    .filter((record) => record.timestamp >= start && (!batchId || record.batch_id !== batchId))
    .map(recordToRow);
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
