import { beforeEach, describe, expect, it, vi } from "vitest";
import { reactive, ref } from "vue";
import { useTaskSync, type TaskSyncDeps } from "./useTaskSync";

const { getTaskSnapshot, listTaskResults } = vi.hoisted(() => ({
  getTaskSnapshot: vi.fn(),
  listTaskResults: vi.fn(),
}));

vi.mock("../lib/bridge", () => ({ getTaskSnapshot, listTaskResults }));

function makeDeps(): TaskSyncDeps {
  return {
    task: reactive({ taskId: "", active: false, exportPath: "", stage: "idle", completedAt: "" }),
    taskSnapshot: ref(null),
    taskSessionState: ref("idle"),
    summary: reactive({ processed: 0, total: 0, passed: 0, failed: 0, exported: 0 }),
    updateMCISProgress: vi.fn(),
    taskResultCSVPath: ref(""),
    resultsLoading: ref(false),
    resultWorkspaceTaskId: ref(""),
    resultRows: ref([]),
    resultSortBy: ref("download"),
    resultOrder: ref("desc"),
    resultIpFilter: ref("all"),
    resultsPageLimit: ref(200),
    clearCurrentTaskResultWorkspace: vi.fn(),
    applyCurrentTaskResultWorkspace: vi.fn(),
    navigateTo: vi.fn(),
    appendLog: vi.fn(),
  };
}

function snapshot(overrides: Record<string, unknown> = {}) {
  return {
    task_id: "task-1",
    session_state: "running",
    current_stage: "stage2_trace",
    completed_at: "",
    status: "running",
    runtime_attached: true,
    progress: { processed: 10, passed: 8, failed: 2, total: 100 },
    export_record: { written_count: 5, target_dir: "C:/out", file_name: "r.csv", source_path: "C:/out/r.csv" },
    ...overrides,
  };
}

describe("useTaskSync", () => {
  beforeEach(() => {
    getTaskSnapshot.mockReset();
    listTaskResults.mockReset();
    listTaskResults.mockResolvedValue({ ok: true, data: { results: [], total_count: 0 } });
  });

  it("writes snapshot fields back to task/summary/mcis/export state", () => {
    const deps = makeDeps();
    const sync = useTaskSync(deps);
    sync.applyTaskSnapshot(snapshot() as never);
    expect(deps.task).toMatchObject({ taskId: "task-1", active: true, stage: "stage2_trace", exportPath: "C:/out/r.csv" });
    expect(deps.taskSessionState.value).toBe("running");
    expect(deps.summary).toMatchObject({ processed: 10, passed: 8, failed: 2, total: 100, exported: 5 });
    expect(deps.taskResultCSVPath.value).toBe("C:/out/r.csv");
  });

  it("keeps paused_runtime active and marks finished sessions inactive", () => {
    const deps = makeDeps();
    const sync = useTaskSync(deps);
    sync.applyTaskSnapshot(snapshot({ status: "paused", session_state: "paused_runtime", runtime_attached: false }) as never);
    expect(deps.task.active).toBe(true);
    sync.applyTaskSnapshot(snapshot({ status: "completed", session_state: "completed" }) as never);
    expect(deps.task.active).toBe(false);
    sync.applyTaskSnapshot(snapshot({ status: "running", session_state: "idle", runtime_attached: true }) as never);
    expect(deps.task.active).toBe(false);
  });

  it("prefers snapshot source path, then cached path, then export path", () => {
    const deps = makeDeps();
    const sync = useTaskSync(deps);
    expect(sync.taskResultCSVFallbackPath(snapshot({ export_record: { source_path: "C:/snap.csv" } }) as never)).toBe("C:/snap.csv");
    deps.taskResultCSVPath.value = "C:/cache.csv";
    expect(sync.taskResultCSVFallbackPath(null)).toBe("C:/cache.csv");
    deps.taskResultCSVPath.value = "";
    deps.task.exportPath = "C:/export";
    expect(sync.taskResultCSVFallbackPath(null)).toBe("C:/export");
    expect(sync.buildResultFileFallbackPayload("C:/fallback", snapshot({ export_record: { source_path: "C:/snap.csv" } }) as never)).toEqual({
      path: "C:/snap.csv",
    });
  });

  it("refreshes snapshot and results workspace, logging both bridge calls", async () => {
    const deps = makeDeps();
    const sync = useTaskSync(deps);
    getTaskSnapshot.mockResolvedValue({ ok: true, data: snapshot() });
    listTaskResults.mockResolvedValue({ ok: true, data: { results: [{ ip: "1.1.1.1" } as never], total_count: 1 } });

    await sync.refreshTaskData("task-1");

    expect(getTaskSnapshot).toHaveBeenCalledWith("task-1");
    expect(listTaskResults).toHaveBeenCalledWith("task-1", "download", "desc", { path: "C:/out/r.csv" }, "all", { limit: 200, offset: 0 });
    expect(deps.applyCurrentTaskResultWorkspace).toHaveBeenCalledWith("task-1", [{ ip: "1.1.1.1" }], 1);
    expect(deps.resultsLoading.value).toBe(false);
    expect(deps.appendLog).toHaveBeenCalledWith("bridge.get_task_snapshot", expect.anything());
    expect(deps.appendLog).toHaveBeenCalledWith("bridge.list_task_results", expect.anything());
  });

  it("queues a concurrent refresh and replays it with the newest task id", async () => {
    const deps = makeDeps();
    const sync = useTaskSync(deps);
    let releaseFirst: (value: unknown) => void = () => {};
    getTaskSnapshot
      .mockImplementationOnce(() => new Promise((resolve) => (releaseFirst = resolve)))
      .mockResolvedValue({ ok: true, data: snapshot({ task_id: "task-2" }) });

    const first = sync.refreshTaskData("task-1");
    const second = sync.refreshTaskData("task-2");
    expect(await second).toBeUndefined();
    releaseFirst({ ok: false });
    await first;

    expect(getTaskSnapshot).toHaveBeenCalledTimes(2);
    expect(getTaskSnapshot).toHaveBeenLastCalledWith("task-2");
    expect(deps.resultsLoading.value).toBe(false);
  });

  it("clears a different result workspace before reconciling", async () => {
    const deps = makeDeps();
    const sync = useTaskSync(deps);
    deps.resultWorkspaceTaskId.value = "old-task";
    getTaskSnapshot.mockResolvedValue({ ok: true, data: snapshot({ task_id: "new-task" }) });
    listTaskResults.mockResolvedValue({ ok: true, data: { results: [], total_count: 0 } });

    await sync.reconcileTaskData("new-task", { switchToResultsOnData: false });

    expect(deps.clearCurrentTaskResultWorkspace).toHaveBeenCalled();
    expect(getTaskSnapshot).toHaveBeenCalledWith("new-task");
  });

  it("switches to results when reconciliation finds rows", async () => {
    const deps = makeDeps();
    const sync = useTaskSync(deps);
    deps.resultRows.value = [{ ip: "1.1.1.1" } as never];
    getTaskSnapshot.mockResolvedValue({ ok: true, data: snapshot({ task_id: "task-1" }) });

    await sync.reconcileTaskData("task-1", { switchToResultsOnData: true });
    expect(deps.navigateTo).toHaveBeenCalledWith("results");
  });

  it("ignores a blank task id", async () => {
    const deps = makeDeps();
    const sync = useTaskSync(deps);
    await sync.reconcileTaskData("   ");
    expect(getTaskSnapshot).not.toHaveBeenCalled();
  });
});
