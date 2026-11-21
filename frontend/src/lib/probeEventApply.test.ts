import { describe, expect, it, vi } from "vitest";
import { reactive, ref } from "vue";
import type { ProbeEventEnvelope, ProbeResult, TaskSnapshot, TaskTone } from "./bridge";
import type { DownloadSpeedRow } from "./downloadSpeedRows";
import { stageTitle } from "./labels";
import { applyProbeEventFrom, type ProbeEventApplyDeps } from "./probeEventApply";

function makeHarness(overrides: Partial<ProbeEventApplyDeps> = {}) {
  const task = reactive({ active: false, completedAt: "", exportPath: "", lastEvent: "", lastSeq: 0, stage: "idle", taskId: "task-1" });
  const taskSessionState = ref("idle");
  const activeTaskSessionState = ref("idle");
  const taskSnapshot = ref<TaskSnapshot | null>(null);
  const status = reactive({ detail: "就绪", title: "就绪", tone: "idle" as TaskTone });
  const summary = reactive({ accepted: 0, exported: 0, failed: 0, filtered: 0, invalid: 0, passed: 0, processed: 0, total: 0 });
  const mcisProgress = reactive({ active: false, candidateCount: 0, completed: 0, failed: 0, lastColo: "", lastIP: "", sourceName: "", succeeded: 0, total: 0 });
  const downloadSpeedState = reactive({ active: false, averageSpeedMbS: null as number | null, bytesRead: 0, colo: "", currentSpeedMbS: null as number | null, elapsedMs: 0, ip: "" });
  const downloadSpeedRows = reactive<DownloadSpeedRow[]>([]);
  const resultRows = ref<ProbeResult[]>([]);
  const taskResultCSVPath = ref("");
  const probeWarnings = ref<string[]>([]);
  const deps: ProbeEventApplyDeps = {
    task,
    taskSessionState,
    activeTaskSessionState,
    taskSnapshot,
    status,
    summary,
    mcisProgress,
    finishTaskAction: vi.fn(),
    downloadSpeedState,
    downloadSpeedRows,
    activeDownloadConcurrency: ref(1),
    activeDownloadSampleIntervalMs: ref(200),
    resetDownloadSpeedState: vi.fn(),
    ensureDownloadSpeedRowPruneTimer: vi.fn(),
    stopDownloadSpeedRowPruneTimer: vi.fn(),
    clearDownloadSpeedTraceEntries: vi.fn(),
    updateDownloadSpeedTrace: vi.fn(),
    applySourceStatuses: vi.fn(),
    resultRows,
    taskResultCSVPath,
    probeWarnings,
    updateHistory: vi.fn(),
    shouldRefreshProgressUI: () => true,
    updateMCISProgress: vi.fn(),
    pushProcessTrace: vi.fn(),
    pushWarningTrace: vi.fn(),
    summarizeFailureSummary: () => "",
    edgeRejectedItems: (event) => (Array.isArray(event.payload.items) ? event.payload.items : [event.payload]) as Array<Record<string, unknown>>,
    notifyUploadResult: vi.fn(),
    eventDebugLogDisplayPath: () => "",
    eventDebugLogOpenTarget: () => "",
    eventTraceFailureSummary: () => "",
    reconcileTaskData: vi.fn(async () => {}),
    isAndroidApp: () => false,
    appendLog: vi.fn(),
    showToast: vi.fn(),
    setStatus: vi.fn(),
    pushActivity: vi.fn(),
    ...overrides,
  };
  return { deps, task, taskSessionState, activeTaskSessionState, status, summary, mcisProgress, downloadSpeedState, downloadSpeedRows, resultRows, taskResultCSVPath, probeWarnings };
}

const mk = (event: string, payload: Record<string, unknown> = {}, seq = 1): ProbeEventEnvelope => ({ event, payload, schema_version: "1", seq, task_id: "task-1", ts: "2026-11-21T00:00:00Z" });

describe("applyProbeEventFrom 特征测试", () => {
  it("外来任务事件：只记 ignored 日志，不改状态", () => {
    const h = makeHarness();
    applyProbeEventFrom({ ...mk("probe.progress"), task_id: "other" }, h.deps);
    expect(h.deps.appendLog).toHaveBeenCalledWith("probe.progress.ignored", expect.anything());
    expect(h.deps.pushActivity).not.toHaveBeenCalled();
    expect(h.task.lastSeq).toBe(0);
  });

  it("重复序号：只记 duplicate 日志", () => {
    const h = makeHarness();
    h.task.lastSeq = 3;
    applyProbeEventFrom(mk("probe.progress", {}, 2), h.deps);
    expect(h.deps.appendLog).toHaveBeenCalledWith("probe.progress.duplicate", expect.anything());
    expect(h.deps.pushActivity).not.toHaveBeenCalled();
  });

  it("序号缺口：记日志并触发对账，事件继续应用", () => {
    const h = makeHarness();
    h.task.lastSeq = 1;
    applyProbeEventFrom(mk("probe.preprocessed", { accepted: 10, filtered: 2, invalid: 1, total: 13 }, 5), h.deps);
    expect(h.deps.appendLog).toHaveBeenCalledWith("probe.preprocessed.sequence_gap", expect.anything());
    expect(h.deps.reconcileTaskData).toHaveBeenCalledWith("task-1");
    expect(h.summary.accepted).toBe(10);
  });

  it("probe.preprocessed：写入 IP 池统计并推送阶段 trace", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.preprocessed", { accepted: 10, filtered: 2, invalid: 1, total: 13, source_statuses: [] }), h.deps);
    expect(h.summary).toMatchObject({ accepted: 10, filtered: 2, invalid: 1, total: 13, processed: 0, passed: 0, failed: 0 });
    expect(h.task.stage).toBe("stage0_pool");
    expect(h.taskSessionState.value).toBe("active_runtime");
    expect(h.deps.applySourceStatuses).toHaveBeenCalledWith([]);
    expect(h.deps.finishTaskAction).toHaveBeenCalledWith("start");
    expect(h.deps.finishTaskAction).toHaveBeenCalledWith("rerun");
    expect(h.deps.pushProcessTrace).toHaveBeenCalledWith(expect.objectContaining({ title: "阶段0 IP池完成", stage: "stage0_pool" }));
    expect(h.task.lastSeq).toBe(1);
    expect(h.task.lastEvent).toBe("probe.preprocessed");
  });

  it("probe.progress：按 payload 更新统计与阶段", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.progress", { stage: "stage1_tcp", processed: 5, passed: 4, failed: 1, total: 10 }), h.deps);
    expect(h.summary).toMatchObject({ processed: 5, passed: 4, failed: 1, total: 10 });
    expect(h.task.stage).toBe("stage1_tcp");
    expect(h.deps.pushProcessTrace).toHaveBeenCalledWith(expect.objectContaining({ title: `${stageTitle("stage1_tcp")}进行中` }));
  });

  it("probe.speed 初始空采样：提前返回，不推进面板与动态", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.speed", { ip: "1.1.1.1", elapsed_ms: 0, bytes_read: 0, current_ready: false, average_ready: false, current_speed_mb_s: 0, average_speed_mb_s: 0 }), h.deps);
    expect(h.downloadSpeedRows.length).toBe(0);
    expect(h.downloadSpeedState.active).toBe(false);
    expect(h.deps.pushActivity).not.toHaveBeenCalled();
    expect(h.task.lastSeq).toBe(1); // 序号仍推进
  });

  it("probe.speed 正常采样：面板、实时行、trace 与动态全更新", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.speed", { ip: "1.1.1.1", colo: "SJC", current_speed_mb_s: 10, average_speed_mb_s: 8, bytes_read: 1000, elapsed_ms: 500, current_ready: true, average_ready: true }), h.deps);
    expect(h.downloadSpeedState).toMatchObject({ active: true, ip: "1.1.1.1", colo: "SJC", currentSpeedMbS: 10, averageSpeedMbS: 8, bytesRead: 1000, elapsedMs: 500 });
    expect(h.downloadSpeedRows.length).toBe(1);
    expect(h.deps.updateDownloadSpeedTrace).toHaveBeenCalledWith("1.1.1.1", "SJC", "2026-11-21T00:00:00Z");
    expect(h.deps.ensureDownloadSpeedRowPruneTimer).toHaveBeenCalled();
    expect(h.deps.pushActivity).toHaveBeenCalled();
  });

  it("probe.speed 单并发换 IP：重置面板", () => {
    const h = makeHarness();
    h.downloadSpeedState.ip = "9.9.9.9";
    applyProbeEventFrom(mk("probe.speed", { ip: "1.1.1.1", current_speed_mb_s: 5, average_speed_mb_s: 5, bytes_read: 10, elapsed_ms: 100 }), h.deps);
    expect(h.deps.resetDownloadSpeedState).toHaveBeenCalled();
  });

  it("probe.completed 有结果：统计取最大值、历史 completed、对账并跳转", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.completed", { exported: 3, passed: 3, result_count: 3, target_path: "C:/r.csv", source_path: "C:/s.csv", warnings: ["w"] }), h.deps);
    expect(h.summary.exported).toBe(3);
    expect(h.summary.passed).toBe(3);
    expect(h.taskResultCSVPath.value).toBe("C:/s.csv");
    expect(h.task.exportPath).toBe("C:/r.csv");
    expect(h.probeWarnings.value).toEqual(["w"]);
    expect(h.deps.pushWarningTrace).toHaveBeenCalledWith(["w"], "2026-11-21T00:00:00Z");
    expect(h.deps.updateHistory).toHaveBeenCalledWith(expect.objectContaining({ tone: "completed", targetPath: "C:/r.csv" }));
    expect(h.deps.reconcileTaskData).toHaveBeenCalledWith("task-1", { switchToResultsOnData: true });
    expect(h.deps.showToast).toHaveBeenCalledWith("探测任务已完成", "success");
    expect(h.taskSessionState.value).toBe("idle");
    expect(h.task.active).toBe(false);
  });

  it("probe.completed 无结果：历史 no_results，提示 info", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.completed"), h.deps);
    expect(h.deps.updateHistory).toHaveBeenCalledWith(expect.objectContaining({ tone: "no_results" }));
    expect(h.deps.showToast).toHaveBeenCalledWith("任务结束但没有可用结果", "info");
    expect(h.deps.reconcileTaskData).toHaveBeenCalledWith("task-1", { switchToResultsOnData: false });
  });

  it("probe.failed：persisted_only 会话态保留", () => {
    const h = makeHarness();
    h.activeTaskSessionState.value = "persisted_only";
    applyProbeEventFrom(mk("probe.failed", { message: "上游超时" }), h.deps);
    expect(h.taskSessionState.value).toBe("persisted_only");
    expect(h.deps.updateHistory).toHaveBeenCalledWith(expect.objectContaining({ tone: "failed" }));
    expect(h.deps.showToast).toHaveBeenCalledWith(expect.any(String), "error");
    expect(h.task.active).toBe(false);
  });

  it("probe.cancelled：会话转 persisted_only，历史 cancelled", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.cancelled"), h.deps);
    expect(h.taskSessionState.value).toBe("persisted_only");
    expect(h.deps.updateHistory).toHaveBeenCalledWith(expect.objectContaining({ tone: "cancelled", title: "任务已终止" }));
    expect(h.deps.showToast).toHaveBeenCalledWith("探测任务已终止", "info");
  });

  it("probe.cooling 可恢复：暂停动作收尾、清测速表", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.cooling", { recoverable: true, reason: "限速" }), h.deps);
    expect(h.deps.finishTaskAction).toHaveBeenCalledWith("pause");
    expect(h.taskSessionState.value).toBe("paused_runtime");
    expect(h.task.stage).toBe("cooling");
    expect(h.downloadSpeedState.active).toBe(false);
    expect(h.deps.stopDownloadSpeedRowPruneTimer).toHaveBeenCalled();
    expect(h.deps.clearDownloadSpeedTraceEntries).toHaveBeenCalled();
    expect(h.deps.pushProcessTrace).toHaveBeenCalledWith(expect.objectContaining({ title: "任务进入冷却", detail: "限速" }));
  });

  it("probe.partial_export：累计导出数并记 partial 历史", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.partial_export", { written: 2, source_path: "C:/s.csv", target_path: "C:/r.csv" }), h.deps);
    expect(h.summary.exported).toBe(2);
    expect(h.taskResultCSVPath.value).toBe("C:/s.csv");
    expect(h.task.exportPath).toBe("C:/r.csv");
    expect(h.taskSessionState.value).toBe("active_runtime");
    expect(h.deps.updateHistory).toHaveBeenCalledWith(expect.objectContaining({ tone: "partial" }));
  });

  it("probe.export_completed：系统导出完成提示与历史", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.export_completed", { written: 4 }), h.deps);
    expect(h.summary.exported).toBe(4);
    expect(h.taskSessionState.value).toBe("idle");
    expect(h.deps.updateHistory).toHaveBeenCalledWith(expect.objectContaining({ tone: "completed" }));
    expect(h.deps.showToast).toHaveBeenCalledWith("Android 系统导出完成", "success");
  });

  it("probe.edge_rejected：1034 与 403 分别落 trace", () => {
    const h = makeHarness();
    applyProbeEventFrom(mk("probe.edge_rejected", { items: [{ ip: "1.2.3.4", note: "1034", message: "", stage: "stage3_get" }] }), h.deps);
    expect(h.deps.pushProcessTrace).toHaveBeenCalledWith(expect.objectContaining({ title: "边缘 IP 受限 1034", tone: "warning" }));
  });

  it("进度节流：不刷新状态与 trace，但序号与动态照常", () => {
    const h = makeHarness({ shouldRefreshProgressUI: () => false });
    applyProbeEventFrom(mk("probe.progress", { stage: "stage1_tcp", processed: 5, passed: 4, failed: 1, total: 10 }), h.deps);
    expect(h.deps.setStatus).not.toHaveBeenCalled();
    expect(h.deps.pushProcessTrace).not.toHaveBeenCalled();
    expect(h.summary.processed).toBe(0);
    expect(h.task.lastSeq).toBe(1);
    expect(h.deps.pushActivity).toHaveBeenCalled();
  });
});
