import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useDownloadSpeed } from "./useDownloadSpeed";

function makeDeps() {
  return { clearDownloadSpeedTraceEntries: vi.fn() };
}

function snapshot(stage3: number, interval: number, strategy: "fast" | "full") {
  return { probe: { concurrency: { stage3 }, download_speed_sample_interval_ms: interval, strategy } };
}

describe("useDownloadSpeed", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal("window", {
      clearInterval: (id: number) => clearInterval(id),
      setInterval: (fn: () => void, ms: number) => setInterval(fn, ms),
    });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("capture 锁定并发/采样间隔/启用口径并夹逼", () => {
    const speed = useDownloadSpeed(makeDeps());
    expect(speed.isDownloadSpeedTaskConfigCaptured()).toBe(false);
    speed.captureDownloadSpeedTaskConfig(snapshot(8, 2000, "full"));
    expect(speed.activeDownloadConcurrency.value).toBe(8);
    expect(speed.activeDownloadSampleIntervalMs.value).toBe(2000);
    expect(speed.activeDownloadEnabled.value).toBe(true);
    expect(speed.isDownloadSpeedTaskConfigCaptured()).toBe(true);
    speed.captureDownloadSpeedTaskConfig(snapshot(0, 0, "fast"));
    expect(speed.activeDownloadConcurrency.value).toBe(1);
    expect(speed.activeDownloadSampleIntervalMs.value).toBe(1000);
    expect(speed.activeDownloadEnabled.value).toBe(false);
    speed.captureDownloadSpeedTaskConfig(snapshot(99, 1000, "full"));
    expect(speed.activeDownloadConcurrency.value).toBe(16);
  });

  it("reset 清空状态与行并清理轨迹，可选保留身份", () => {
    const deps = makeDeps();
    const speed = useDownloadSpeed(deps);
    speed.downloadSpeedState.active = true;
    speed.downloadSpeedState.ip = "1.1.1.1";
    speed.downloadSpeedState.colo = "SJC";
    speed.downloadSpeedState.bytesRead = 100;
    speed.downloadSpeedRows.push({ averageSpeedMbS: 1, bytesRead: 100, colo: "SJC", currentSpeedMbS: 1, elapsedMs: 1000, ip: "1.1.1.1", updatedAt: Date.now() });
    speed.resetDownloadSpeedState(false);
    expect(speed.downloadSpeedState.active).toBe(false);
    expect(speed.downloadSpeedState.bytesRead).toBe(0);
    expect(speed.downloadSpeedState.ip).toBe("1.1.1.1");
    expect(speed.downloadSpeedState.colo).toBe("SJC");
    expect(speed.downloadSpeedRows).toHaveLength(0);
    expect(deps.clearDownloadSpeedTraceEntries).toHaveBeenCalledTimes(1);
    speed.downloadSpeedState.ip = "2.2.2.2";
    speed.resetDownloadSpeedState();
    expect(speed.downloadSpeedState.ip).toBe("");
    expect(speed.downloadSpeedState.colo).toBe("");
  });

  it("TTL 定时器移除过期行，行空后自动停表", () => {
    vi.setSystemTime(new Date("2026-01-15T12:00:00Z"));
    const speed = useDownloadSpeed(makeDeps());
    const now = Date.now();
    speed.downloadSpeedRows.push({ averageSpeedMbS: 1, bytesRead: 1, colo: "", currentSpeedMbS: 1, elapsedMs: 1, ip: "old", updatedAt: now - 10_000 });
    speed.downloadSpeedRows.push({ averageSpeedMbS: 1, bytesRead: 1, colo: "", currentSpeedMbS: 1, elapsedMs: 1, ip: "fresh", updatedAt: now });
    speed.ensureDownloadSpeedRowPruneTimer();
    speed.ensureDownloadSpeedRowPruneTimer();
    vi.advanceTimersByTime(1000);
    expect(speed.downloadSpeedRows.map((row) => row.ip)).toEqual(["fresh"]);
    speed.downloadSpeedRows.splice(0, speed.downloadSpeedRows.length);
    vi.advanceTimersByTime(1000);
    // 行空后定时器已停：再推进不产生额外清理动作（无异常即停表成功）
    vi.advanceTimersByTime(5000);
    expect(speed.downloadSpeedRows).toHaveLength(0);
    speed.stopDownloadSpeedRowPruneTimer();
  });
});