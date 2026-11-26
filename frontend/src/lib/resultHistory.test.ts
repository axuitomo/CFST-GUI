import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { importResultHistory } = vi.hoisted(() => ({ importResultHistory: vi.fn() }));

vi.mock("./bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./bridge")>();
  return { ...actual, importResultHistory };
});

import type { ProbeResult } from "./bridge";
import { buildResultDisplay, clearLegacyResultHistory, historyRangeStart, migrateLegacyResultHistory, readLegacyResultHistory, RESULT_HISTORY_RETENTION_DAYS, RESULT_HISTORY_STORAGE_KEY, UNKNOWN_COUNTRY_FILTER, type ResultDisplayOptions, type ResultHistoryRecord } from "./resultHistory";

const DAY_MS = 24 * 60 * 60 * 1000;
const HOUR = 60 * 60 * 1000;
const NOW = Date.UTC(2026, 0, 20, 12, 0, 0);

let store: Record<string, string> = {};

function row(address: string, patch: Partial<ProbeResult> = {}): ProbeResult {
  return { address, export_status: "", stage_status: "completed", ...patch };
}

function record(address: string, patch: Partial<ResultHistoryRecord> = {}): ResultHistoryRecord {
  return {
    address,
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
    ...patch,
  };
}

// 展示管道的默认参数:各用例只覆盖自己关心的字段,历史行来自后端镜像。
function displayOptions(patch: Partial<ResultDisplayOptions> = {}): ResultDisplayOptions {
  return {
    countryFilter: "",
    currentBatchId: "batch-now",
    currentRows: [],
    historyRecords: [],
    ipFilter: "all",
    mergeDuplicates: true,
    now: NOW,
    order: "asc",
    range: "7d",
    sortBy: "tcp",
    ...patch,
  };
}

function seedLegacy(records: Array<Partial<ResultHistoryRecord> & { address: string }>) {
  store[RESULT_HISTORY_STORAGE_KEY] = JSON.stringify(records.map((entry) => record(entry.address, entry)));
}

describe("resultHistory", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    importResultHistory.mockResolvedValue({ ok: true, code: "HISTORY_IMPORTED", message: "", warnings: [], data: { changed: true, dropped: 0, merged: 1, records: 1, skipped: 0 } });
    store = {};
    vi.stubGlobal("window", {
      localStorage: {
        getItem: (key: string) => store[key] ?? null,
        removeItem: (key: string) => {
          delete store[key];
        },
        setItem: (key: string, value: string) => {
          store[key] = value;
        },
      },
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("读取旧版 localStorage 历史供一次性迁移,并做同样的归一化", () => {
    seedLegacy([
      { address: " 1.1.1.1 ", colo: " hkg ", tcp_latency_ms: 30, test_port: 0 },
      { address: "2.2.2.2", stage_status: "" },
      { address: "   ", timestamp: NOW - DAY_MS },
      { address: "3.3.3.3", download_mbps: Number.NaN },
    ]);

    const records = readLegacyResultHistory();
    expect(records.map((entry) => entry.address)).toEqual(["1.1.1.1", "2.2.2.2", "3.3.3.3"]);
    expect(records[0].colo).toBe("HKG");
    expect(records[0].test_port).toBeNull();
    expect(records[1].stage_status).toBe("completed");
    expect(records[2].download_mbps).toBeNull();
  });

  it("旧键缺失或内容损坏时返回空历史", () => {
    expect(readLegacyResultHistory()).toEqual([]);

    store[RESULT_HISTORY_STORAGE_KEY] = "not json";
    expect(readLegacyResultHistory()).toEqual([]);

    store[RESULT_HISTORY_STORAGE_KEY] = JSON.stringify({ records: [] });
    expect(readLegacyResultHistory()).toEqual([]);
  });

  it("迁移完成后删除旧键", () => {
    seedLegacy([{ address: "1.1.1.1" }]);
    clearLegacyResultHistory();
    expect(store[RESULT_HISTORY_STORAGE_KEY]).toBeUndefined();
    expect(readLegacyResultHistory()).toEqual([]);
  });

  it("迁移把旧历史交给后端并在成功后删键", async () => {
    seedLegacy([{ address: "1.1.1.1" }]);

    const result = await migrateLegacyResultHistory();

    expect(importResultHistory).toHaveBeenCalledWith([expect.objectContaining({ address: "1.1.1.1", batch_id: "batch-old" })]);
    expect(result?.ok).toBe(true);
    expect(store[RESULT_HISTORY_STORAGE_KEY]).toBeUndefined();
  });

  it("导入失败时保留旧键,留待下次启动重试", async () => {
    importResultHistory.mockResolvedValue({ ok: false, code: "RESULT_HISTORY_WRITE_FAILED", message: "disk full", warnings: [] });
    seedLegacy([{ address: "1.1.1.1" }]);

    const result = await migrateLegacyResultHistory();

    expect(result?.ok).toBe(false);
    expect(store[RESULT_HISTORY_STORAGE_KEY]).toBeDefined();
  });

  it("没有旧历史时不调用导入命令", async () => {
    expect(await migrateLegacyResultHistory()).toBeNull();
    expect(importResultHistory).not.toHaveBeenCalled();
  });

  it("跨批次合并后按最新时间保留同一 IP 并统计出现次数", () => {
    const display = buildResultDisplay(
      displayOptions({
        currentRows: [row("1.1.1.1", { tcp_latency_ms: 55 })],
        historyRecords: [record("1.1.1.1", { tcp_latency_ms: 30, timestamp: NOW - 2 * DAY_MS }), record("1.1.1.1", { tcp_latency_ms: 80, timestamp: NOW - HOUR })],
      }),
    );

    expect(display.rows).toHaveLength(1);
    expect(display.rows[0].tcp_latency_ms).toBe(55);
    expect(display.rows[0].batch_count).toBe(3);
  });

  it("关闭合并时保留原始明细,并仍按选中条件排序", () => {
    const display = buildResultDisplay(
      displayOptions({
        currentRows: [row("1.1.1.1", { tcp_latency_ms: 55 })],
        historyRecords: [record("1.1.1.1", { tcp_latency_ms: 30, timestamp: NOW - 2 * DAY_MS })],
        mergeDuplicates: false,
        order: "desc",
      }),
    );

    expect(display.rows.map((entry) => entry.tcp_latency_ms)).toEqual([55, 30]);
  });

  it("多端口场景按 IP:Port 去重,单端口场景按 IP 去重", () => {
    const base = displayOptions();
    const samePort = [record("1.1.1.1", { test_port: 443, timestamp: NOW - 2 * DAY_MS }), record("1.1.1.1", { test_port: 443, timestamp: NOW - 3 * DAY_MS })];
    expect(buildResultDisplay({ ...base, historyRecords: samePort }).rows).toHaveLength(1);

    const mixedPorts = [record("1.1.1.1", { test_port: 443, timestamp: NOW - 2 * DAY_MS }), record("1.1.1.1", { test_port: 8443, timestamp: NOW - 3 * DAY_MS })];
    expect(buildResultDisplay({ ...base, historyRecords: mixedPorts }).rows).toHaveLength(2);
    // 当前批次的同端口记录替换历史记录,不会多出一条。
    expect(buildResultDisplay({ ...base, currentRows: [row("1.1.1.1", { test_port: 8443 })], historyRecords: mixedPorts }).rows).toHaveLength(2);
  });

  it("国家筛选与选项计数基于去重后的数据集", () => {
    const display = buildResultDisplay(
      displayOptions({
        countryFilter: "HK",
        historyRecords: [record("1.1.1.1", { colo: "HKG" }), record("2.2.2.2", { colo: "NRT" }), record("3.3.3.3", { colo: "" })],
        range: "30d",
      }),
    );

    expect(display.rows.map((entry) => entry.address)).toEqual(["1.1.1.1"]);
    expect(Object.fromEntries(display.countryOptions.map((option) => [option.value, option.label]))).toEqual({
      [UNKNOWN_COUNTRY_FILTER]: "未知地区 (1)",
      HK: "中国香港 (1)",
      JP: "日本 (1)",
    });
  });

  it("当前批次的历史行不参与跨批次合并", () => {
    const display = buildResultDisplay(
      displayOptions({
        currentBatchId: "batch-now",
        currentRows: [row("1.1.1.1", { tcp_latency_ms: 55 })],
        historyRecords: [record("1.1.1.1", { batch_id: "batch-now", tcp_latency_ms: 10 })],
      }),
    );

    expect(display.rows).toHaveLength(1);
    expect(display.rows[0].tcp_latency_ms).toBe(55);
  });

  it("时间范围之外的历史行不进入展示", () => {
    const display = buildResultDisplay(
      displayOptions({
        historyRecords: [record("9.9.9.9", { timestamp: NOW - (RESULT_HISTORY_RETENTION_DAYS + 1) * DAY_MS })],
        range: "30d",
      }),
    );

    expect(display.rows).toEqual([]);
  });

  it("本次结果不做跨批次去重,但跟随国家筛选", () => {
    const display = buildResultDisplay(
      displayOptions({
        countryFilter: "HK",
        currentBatchId: "batch-1",
        currentRows: [row("1.1.1.1", { colo: "HKG" }), row("1.1.1.1", { colo: "NRT" })],
        order: "desc",
        range: "current",
        sortBy: "download",
      }),
    );

    expect(display.rows).toHaveLength(1);
    expect(display.rows[0].colo).toBe("HKG");
  });

  it("时间范围起点按天回溯,全部与 30 天一致", () => {
    expect(historyRangeStart("1d", NOW)).toBe(NOW - DAY_MS);
    expect(historyRangeStart("7d", NOW)).toBe(NOW - 7 * DAY_MS);
    expect(historyRangeStart("15d", NOW)).toBe(NOW - 15 * DAY_MS);
    expect(historyRangeStart("all", NOW)).toBe(NOW - RESULT_HISTORY_RETENTION_DAYS * DAY_MS);
  });
});
