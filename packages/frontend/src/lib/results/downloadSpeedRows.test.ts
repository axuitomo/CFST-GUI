import { describe, expect, it } from "vitest";
import { DOWNLOAD_SPEED_ROW_LIMIT, clearDownloadSpeedRows, downloadSpeedRowTtlMs, pruneDownloadSpeedRows, upsertDownloadSpeedRow, type DownloadSpeedRow } from "./downloadSpeedRows";

function makeRow(ip: string, updatedAt = 0): DownloadSpeedRow {
  return {
    averageSpeedMbS: 10,
    bytesRead: 1024,
    colo: "LAX",
    currentSpeedMbS: 12,
    elapsedMs: 1000,
    ip,
    updatedAt,
  };
}

describe("downloadSpeedRows", () => {
  it("同一颗 IP 原位更新，不会新增重复行", () => {
    const rows = [makeRow("1.1.1.1", 1000)];
    upsertDownloadSpeedRow(rows, { ...makeRow("1.1.1.1", 2000), currentSpeedMbS: 20 }, { now: 2000, ttlMs: 6000 });
    expect(rows).toHaveLength(1);
    expect(rows[0].currentSpeedMbS).toBe(20);
  });

  it("同一次 tick 写入四颗 IP 时四行都保留", () => {
    const rows: DownloadSpeedRow[] = [];
    for (const [index, ip] of ["1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4"].entries()) {
      upsertDownloadSpeedRow(rows, makeRow(ip, 1000 + index), { now: 1000, ttlMs: 6000 });
    }
    expect(rows.map((row) => row.ip)).toEqual(["1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4"]);
  });

  it("超过上限时丢弃最早写入的行", () => {
    const rows: DownloadSpeedRow[] = [];
    for (let index = 0; index < DOWNLOAD_SPEED_ROW_LIMIT + 1; index += 1) {
      upsertDownloadSpeedRow(rows, makeRow(`10.0.0.${index}`, 1000), { now: 1000, ttlMs: 6000 });
    }
    expect(rows).toHaveLength(DOWNLOAD_SPEED_ROW_LIMIT);
    expect(rows[0].ip).toBe("10.0.0.1");
  });

  it("只清理超过 TTL 的行，仍在新窗口内的行保留", () => {
    const rows = [makeRow("1.1.1.1", 1000), makeRow("2.2.2.2", 7000)];
    pruneDownloadSpeedRows(rows, 8000, 6000);
    expect(rows.map((row) => row.ip)).toEqual(["2.2.2.2"]);
  });

  it("TTL 至少 6 秒，并按采样间隔三倍放宽", () => {
    expect(downloadSpeedRowTtlMs(1000)).toBe(6000);
    expect(downloadSpeedRowTtlMs(3000)).toBe(9000);
    expect(downloadSpeedRowTtlMs(0)).toBe(6000);
  });

  it("清空后可以重新收集下一轮 IP", () => {
    const rows = [makeRow("1.1.1.1")];
    clearDownloadSpeedRows(rows);
    expect(rows).toHaveLength(0);
    upsertDownloadSpeedRow(rows, makeRow("2.2.2.2", 1000), { now: 1000, ttlMs: 6000 });
    expect(rows.map((row) => row.ip)).toEqual(["2.2.2.2"]);
  });
});
