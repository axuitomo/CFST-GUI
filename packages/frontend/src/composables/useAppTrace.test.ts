import { beforeEach, describe, expect, it, vi } from "vitest";
import { reactive, ref } from "vue";
import { useAppTrace, type AppTraceDeps } from "./useAppTrace";

function makeDeps(overrides: Partial<AppTraceDeps> = {}): AppTraceDeps {
  const status = reactive({ title: "", detail: "", tone: "idle" as const });
  return {
    settings: { utcOffsetMinutes: 480 },
    configPath: ref("C:/config/cfst.json"),
    status: () => status,
    isAndroidApp: () => false,
    ...overrides,
  };
}

describe("useAppTrace", () => {
  beforeEach(() => {
    vi.useRealTimers();
  });

  it("bounds the debug log buffer to 160 entries", () => {
    const trace = useAppTrace(makeDeps());
    for (let index = 0; index < 170; index += 1) {
      trace.appendLog(`event-${index}`, { index });
    }
    expect(trace.logs).toHaveLength(160);
    expect(trace.logs[0].event).toBe("event-10");
    expect(trace.logs[trace.logs.length - 1].event).toBe("event-169");
  });

  it("maps bridge diagnostics into the command payload shape", () => {
    const trace = useAppTrace(makeDeps());
    const payload = trace.commandDiagnosticPayload({
      code: " storage_failed ",
      data: {
        configPath: "C:/cfg.json",
        storage: { storageUri: "file:///data", runtimeDir: "/run", lastSyncError: "boom", permissionOk: false },
      },
      message: " fail ",
      warnings: [" w1 ", ""],
    });
    expect(payload).toEqual({
      code: "storage_failed",
      message: "fail",
      warnings: [" w1 "],
      config_path: "C:/cfg.json",
      storage_uri: "file:///data",
      runtime_dir: "/run",
      last_sync_error: "boom",
      permission_ok: false,
    });
  });

  it("falls back to the config path ref and snake_case storage keys", () => {
    const trace = useAppTrace(makeDeps());
    const payload = trace.commandDiagnosticPayload({
      data: { storage: { storage_uri: "s", runtime_dir: "r", last_sync_error: "e" } },
    });
    expect(payload.config_path).toBe("C:/config/cfst.json");
    expect(payload.storage_uri).toBe("s");
    expect(payload.runtime_dir).toBe("r");
    expect(payload.last_sync_error).toBe("e");
    expect(payload.permission_ok).toBe(true);
  });

  it("writes status through the lazy accessor", () => {
    const status = reactive({ title: "", detail: "", tone: "idle" as const });
    const trace = useAppTrace(makeDeps({ status: () => status }));
    trace.setStatus({ title: "运行中", detail: "stage1", tone: "warning" });
    expect(status).toMatchObject({ title: "运行中", detail: "stage1", tone: "warning" });
  });

  it("dedupes adjacent traces with the same stage/title/tone and caps at 80", () => {
    const trace = useAppTrace(makeDeps());
    trace.pushProcessTrace({ detail: "a", stage: "stage1", title: "探测", tone: "running", ts: "t1" });
    trace.pushProcessTrace({ detail: "b", stage: "stage1", title: "探测", tone: "running", ts: "t2" });
    expect(trace.processTrace.value).toHaveLength(1);
    expect(trace.processTrace.value[0]).toMatchObject({ detail: "b", id: 0 });

    for (let index = 0; index < 85; index += 1) {
      trace.pushProcessTrace({ detail: `d-${index}`, stage: `stage-${index}`, title: "探测", tone: "info", ts: "t" });
    }
    expect(trace.processTrace.value.length).toBe(80);
  });

  it("skips empty warning batches and joins multiple warnings", () => {
    const trace = useAppTrace(makeDeps());
    trace.pushWarningTrace([]);
    expect(trace.processTrace.value).toHaveLength(0);
    trace.pushWarningTrace(["w1", "w2"], "2026-01-01T00:00:00Z");
    expect(trace.processTrace.value[0]).toMatchObject({
      detail: "w1；w2",
      stage: "warning",
      title: "任务提示",
      tone: "warning",
      ts: "2026-01-01T00:00:00Z",
    });
  });

  it("updates a running download-speed trace in place and removes only those keys", () => {
    const trace = useAppTrace(makeDeps());
    trace.updateDownloadSpeedTrace("1.1.1.1", "HKG", "t1");
    const [first] = trace.processTrace.value;
    trace.updateDownloadSpeedTrace("1.1.1.1", "NRT", "t2");
    expect(trace.processTrace.value).toHaveLength(1);
    expect(trace.processTrace.value[0]).toMatchObject({ detail: "1.1.1.1(NRT) 正在测速中", id: first.id, key: "download-speed:1.1.1.1" });

    trace.pushProcessTrace({ detail: "keep", stage: "stage2", title: "探测", tone: "info", ts: "t3" });
    trace.clearDownloadSpeedTraceEntries();
    expect(trace.processTrace.value.map((entry) => entry.key)).toEqual([undefined]);
  });

  it("summarizes duplicate and invalid counts", () => {
    const trace = useAppTrace(makeDeps());
    expect(trace.summarizeFailureSummary(null)).toBe("");
    expect(trace.summarizeFailureSummary({ duplicate_count: 3, invalid_count: 0 })).toBe("重复 3");
    expect(trace.summarizeFailureSummary({ duplicate_count: 3, invalid_count: 2 })).toBe("重复 3，非法 2");
  });

  it("formats timestamps with the configured UTC offset", () => {
    const trace = useAppTrace(makeDeps({ settings: { utcOffsetMinutes: 480 } }));
    expect(trace.utcOffsetLabel()).toBe("UTC+08:00");
    expect(trace.formatAppTimestamp("2026-01-01T00:00:00Z", { includeSeconds: false, includeOffset: true })).toBe("2026-01-01 08:00 UTC+08:00");
    expect(trace.formatAppTimestamp("not-a-time", { fallback: "-" })).toBe("not-a-time");
  });

  it("passes progress refreshes through unless running on Android", () => {
    const trace = useAppTrace(makeDeps({ isAndroidApp: () => false }));
    expect(trace.shouldRefreshProgressUI("stage1", 10, 100)).toBe(true);
  });

  it("throttles Android progress refreshes within the same stage window", () => {
    let now = 1000;
    vi.spyOn(performance, "now").mockImplementation(() => now);
    const trace = useAppTrace(makeDeps({ isAndroidApp: () => true }));
    expect(trace.shouldRefreshProgressUI("stage1", 10, 100)).toBe(true);
    now += 50;
    expect(trace.shouldRefreshProgressUI("stage1", 12, 100)).toBe(false);
    now += 500;
    expect(trace.shouldRefreshProgressUI("stage1", 50, 100)).toBe(true);
    expect(trace.shouldRefreshProgressUI("stage2", 50, 100)).toBe(true);
  });

  it("clears the whole process trace", () => {
    const trace = useAppTrace(makeDeps());
    trace.pushProcessTrace({ detail: "a", stage: "stage1", title: "探测", tone: "running", ts: "t1" });
    trace.clearProcessTrace();
    expect(trace.processTrace.value).toEqual([]);
  });
});
