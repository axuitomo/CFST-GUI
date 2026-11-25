import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ProbeResult } from "./bridge";
import { buildResultDisplay, historyRangeStart, readResultHistory, recordResultHistoryBatch, RESULT_HISTORY_MAX_RECORDS, RESULT_HISTORY_RETENTION_DAYS, RESULT_HISTORY_STORAGE_KEY, UNKNOWN_COUNTRY_FILTER, type ResultHistoryRecord } from "./resultHistory";

const DAY_MS = 24 * 60 * 60 * 1000;
const NOW = Date.UTC(2026, 0, 20, 12, 0, 0);

let store: Record<string, string> = {};

function row(address: string, patch: Partial<ProbeResult> = {}): ProbeResult {
  return { address, export_status: "", stage_status: "completed", ...patch };
}

function seed(records: Array<Partial<ResultHistoryRecord> & { address: string }>) {
  store[RESULT_HISTORY_STORAGE_KEY] = JSON.stringify(
    records.map((record) => ({
      batch_id: "batch-old",
      colo: "HKG",
      download_mbps: null,
      max_download_mbps: null,
      source_port: null,
      stage_status: "completed",
      tcp_latency_ms: null,
      test_port: null,
      timestamp: NOW - DAY_MS,
      trace_latency_ms: null,
      ...record,
    })),
  );
}

describe("resultHistory", () => {
  beforeEach(() => {
    store = {};
    vi.stubGlobal("window", {
      localStorage: {
        getItem: (key: string) => store[key] ?? null,
        setItem: (key: string, value: string) => {
          store[key] = value;
        },
      },
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("写入批次时跳过失败行，并按 batchId 幂等覆盖", () => {
    recordResultHistoryBatch("batch-1", [row("1.1.1.1", { tcp_latency_ms: 30 }), row("2.2.2.2", { stage_status: "tcp_failed" })], NOW);
    recordResultHistoryBatch("batch-1", [row("1.1.1.1", { tcp_latency_ms: 42 })], NOW);

    const records = readResultHistory(NOW);
    expect(records).toHaveLength(1);
    expect(records[0].address).toBe("1.1.1.1");
    expect(records[0].tcp_latency_ms).toBe(42);
  });

  it("配额写满时缩减一半后重试，新批次仍能落盘", () => {
    const writes: string[] = [];
    let attempts = 0;
    vi.stubGlobal("window", {
      localStorage: {
        getItem: () => null,
        setItem: (_key: string, value: string) => {
          attempts += 1;
          if (attempts === 1) {
            throw new Error("quota exceeded");
          }
          writes.push(value);
        },
      },
    });

    recordResultHistoryBatch("batch-1", [row("1.1.1.1"), row("2.2.2.2")], NOW);

    expect(writes).toHaveLength(1);
    expect(JSON.parse(writes[0]).map((record: { address: string }) => record.address)).toEqual(["2.2.2.2"]);
  });

  it("读取只过滤过期记录，物理清理留到写盘时", () => {
    seed([
      { address: "1.1.1.1", timestamp: NOW - (RESULT_HISTORY_RETENTION_DAYS + 1) * DAY_MS },
      { address: "2.2.2.2", timestamp: NOW - DAY_MS },
    ]);

    expect(readResultHistory(NOW).map((record) => record.address)).toEqual(["2.2.2.2"]);
    expect(JSON.parse(store[RESULT_HISTORY_STORAGE_KEY])).toHaveLength(2);

    recordResultHistoryBatch("batch-new", [row("3.3.3.3")], NOW);
    expect(JSON.parse(store[RESULT_HISTORY_STORAGE_KEY]).map((record: { address: string }) => record.address)).toEqual(["2.2.2.2", "3.3.3.3"]);
  });

  it("同批次按行合并：分页或筛选带来的子集不覆盖已存行", () => {
    recordResultHistoryBatch("batch-1", [row("1.1.1.1", { test_port: 443 }), row("2.2.2.2", { test_port: 443 })], NOW);
    recordResultHistoryBatch("batch-1", [row("2.2.2.2", { test_port: 443, tcp_latency_ms: 12 })], NOW);

    const records = readResultHistory(NOW);
    expect(records.map((record) => record.address)).toEqual(["1.1.1.1", "2.2.2.2"]);
    expect(records[1].tcp_latency_ms).toBe(12);
  });

  it("内容未变的重复写入不落盘", () => {
    const writes: string[] = [];
    vi.stubGlobal("window", {
      localStorage: {
        getItem: () => writes[writes.length - 1] ?? null,
        setItem: (_key: string, value: string) => {
          writes.push(value);
        },
      },
    });

    recordResultHistoryBatch("batch-1", [row("1.1.1.1")], NOW);
    recordResultHistoryBatch("batch-1", [row("1.1.1.1")], NOW + 1000);

    expect(writes).toHaveLength(1);
  });

  it("超过条数上限时按时间保留最新的记录", () => {
    const total = RESULT_HISTORY_MAX_RECORDS + 1;
    seed(Array.from({ length: total }, (_, index) => ({ address: `10.0.${Math.floor(index / 250)}.${index % 250}`, timestamp: NOW - index })));

    const records = readResultHistory(NOW);
    expect(records).toHaveLength(RESULT_HISTORY_MAX_RECORDS);
    expect(records.some((record) => record.address === "10.0.48.0")).toBe(false);
    expect(records.some((record) => record.address === "10.0.0.0")).toBe(true);
  });

  it("跨批次合并后按最新时间保留同一 IP 并统计出现次数", () => {
    seed([
      { address: "1.1.1.1", tcp_latency_ms: 30, timestamp: NOW - 2 * DAY_MS },
      { address: "1.1.1.1", tcp_latency_ms: 80, timestamp: NOW - HOUR },
    ]);

    const display = buildResultDisplay({
      countryFilter: "",
      currentBatchId: "batch-now",
      currentRows: [row("1.1.1.1", { tcp_latency_ms: 55 })],
      ipFilter: "all",
      mergeDuplicates: true,
      now: NOW,
      order: "asc",
      range: "7d",
      sortBy: "tcp",
    });

    expect(display.rows).toHaveLength(1);
    expect(display.rows[0].tcp_latency_ms).toBe(55);
    expect(display.rows[0].batch_count).toBe(3);
  });

  it("关闭合并时保留原始明细，并仍按选中条件排序", () => {
    seed([{ address: "1.1.1.1", tcp_latency_ms: 30, timestamp: NOW - 2 * DAY_MS }]);

    const display = buildResultDisplay({
      countryFilter: "",
      currentBatchId: "batch-now",
      currentRows: [row("1.1.1.1", { tcp_latency_ms: 55 })],
      ipFilter: "all",
      mergeDuplicates: false,
      now: NOW,
      order: "desc",
      range: "7d",
      sortBy: "tcp",
    });

    expect(display.rows.map((entry) => entry.tcp_latency_ms)).toEqual([55, 30]);
  });

  it("多端口场景按 IP:Port 去重，单端口场景按 IP 去重", () => {
    const base = {
      countryFilter: "",
      currentBatchId: "",
      ipFilter: "all" as const,
      mergeDuplicates: true,
      now: NOW,
      order: "desc" as const,
      range: "7d" as const,
      sortBy: "tcp" as const,
    };
    seed([
      { address: "1.1.1.1", test_port: 443, timestamp: NOW - 2 * DAY_MS },
      { address: "1.1.1.1", test_port: 443, timestamp: NOW - 3 * DAY_MS },
    ]);
    expect(buildResultDisplay({ ...base, currentRows: [] }).rows).toHaveLength(1);

    seed([
      { address: "1.1.1.1", test_port: 443, timestamp: NOW - 2 * DAY_MS },
      { address: "1.1.1.1", test_port: 8443, timestamp: NOW - 3 * DAY_MS },
    ]);
    expect(buildResultDisplay({ ...base, currentRows: [] }).rows).toHaveLength(2);
    // 当前批次的同端口记录替换历史记录，不会多出一条。
    expect(buildResultDisplay({ ...base, currentRows: [row("1.1.1.1", { test_port: 8443 })] }).rows).toHaveLength(2);
  });

  it("国家筛选与选项计数基于去重后的数据集", () => {
    seed([
      { address: "1.1.1.1", colo: "HKG", timestamp: NOW - DAY_MS },
      { address: "2.2.2.2", colo: "NRT", timestamp: NOW - DAY_MS },
      { address: "3.3.3.3", colo: "ZZZ", timestamp: NOW - DAY_MS },
    ]);

    const display = buildResultDisplay({
      countryFilter: "JP",
      currentBatchId: "",
      currentRows: [],
      ipFilter: "all",
      mergeDuplicates: true,
      now: NOW,
      order: "asc",
      range: "30d",
      sortBy: "tcp",
    });

    expect(display.rows.map((entry) => entry.address)).toEqual(["2.2.2.2"]);
    expect(Object.fromEntries(display.countryOptions.map((option) => [option.value, option.label]))).toEqual({
      [UNKNOWN_COUNTRY_FILTER]: "未知地区 (1)",
      HK: "中国香港 (1)",
      JP: "日本 (1)",
    });
  });

  it("本次结果不做跨批次去重，但跟随国家筛选", () => {
    const display = buildResultDisplay({
      countryFilter: "HK",
      currentBatchId: "batch-1",
      currentRows: [row("1.1.1.1", { colo: "HKG" }), row("1.1.1.1", { colo: "NRT" })],
      ipFilter: "all",
      mergeDuplicates: true,
      now: NOW,
      order: "desc",
      range: "current",
      sortBy: "download",
    });

    expect(display.rows).toHaveLength(1);
    expect(display.rows[0].colo).toBe("HKG");
  });

  it("时间范围起点按天回溯，全部与 30 天一致", () => {
    expect(historyRangeStart("1d", NOW)).toBe(NOW - DAY_MS);
    expect(historyRangeStart("7d", NOW)).toBe(NOW - 7 * DAY_MS);
    expect(historyRangeStart("15d", NOW)).toBe(NOW - 15 * DAY_MS);
    expect(historyRangeStart("all", NOW)).toBe(NOW - RESULT_HISTORY_RETENTION_DAYS * DAY_MS);
  });
});

const HOUR = 60 * 60 * 1000;
