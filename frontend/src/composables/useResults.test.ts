import { beforeEach, describe, expect, it, vi } from "vitest";
import { reactive, ref } from "vue";

const { listTaskResults, mergeResultHistoryBatch } = vi.hoisted(() => ({ listTaskResults: vi.fn(), mergeResultHistoryBatch: vi.fn() }));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, listTaskResults, mergeResultHistoryBatch };
});

import type { ProbeResult, TaskSnapshot } from "../lib/bridge";
import { useResults, type ResultsDeps } from "./useResults";

const row = (address: string) => ({ address, export_status: "", stage_status: "" }) as ProbeResult;

function makeDeps() {
  const task = reactive({ exportPath: "C:/out.csv", taskId: "task-1" });
  const taskSnapshot = ref<TaskSnapshot | null>({ export_record: { source_path: "C:/snap.csv" } } as TaskSnapshot);
  const deps: ResultsDeps = {
    task,
    taskSnapshot,
    buildResultFileFallbackPayload: (exportPath) => ({ path: exportPath }),
    refreshTaskData: vi.fn(async () => {}),
    appendLog: vi.fn(),
    refreshResultHistory: vi.fn(),
  };
  return { deps, task, taskSnapshot };
}

describe("useResults", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listTaskResults.mockResolvedValue({ ok: true, message: "", warnings: [], data: { results: [row("2.2.2.2")], total_count: 5 } });
    mergeResultHistoryBatch.mockResolvedValue({ ok: true, message: "", warnings: [], data: { changed: true, dropped: 0, merged: 2, records: 2, skipped: 0 } });
  });

  it("同列排序切换升降序并重置分页刷新", () => {
    const { deps } = makeDeps();
    const api = useResults(deps);
    api.resultRows.value = [row("1.1.1.1")];
    api.resultsTotalCount.value = 9;
    api.updateResultSort("download"); // 默认 download/desc → 切 asc
    expect(api.resultOrder.value).toBe("asc");
    expect(api.resultRows.value).toEqual([]);
    expect(api.resultsTotalCount.value).toBe(0);
    expect(deps.refreshTaskData).toHaveBeenCalledTimes(1);
  });

  it("换列排序：速率列降序、延迟列升序", () => {
    const { deps } = makeDeps();
    const api = useResults(deps);
    api.updateResultSort("tcp");
    expect(api.resultSortBy.value).toBe("tcp");
    expect(api.resultOrder.value).toBe("asc");
    api.updateResultSort("max_download");
    expect(api.resultOrder.value).toBe("desc");
  });

  it("IP 过滤与排序方向：重复值不触发刷新", () => {
    const { deps } = makeDeps();
    const api = useResults(deps);
    api.updateResultIpFilter("all"); // 与默认相同
    api.updateResultOrder("desc");
    expect(deps.refreshTaskData).not.toHaveBeenCalled();
    api.updateResultIpFilter("ipv4");
    expect(api.resultIpFilter.value).toBe("ipv4");
    expect(deps.refreshTaskData).toHaveBeenCalledTimes(1);
  });

  it("清空工作区：默认连同任务快照，preserveSnapshot 保留", () => {
    const { deps, taskSnapshot } = makeDeps();
    const api = useResults(deps);
    api.applyCurrentTaskResultWorkspace(" task-9 ", [row("1.1.1.1")], 3);
    expect(api.resultWorkspaceTaskId.value).toBe("task-9");
    expect(api.resultsTotalCount.value).toBe(3);
    api.taskResultCSVPath.value = "C:/x.csv";
    api.clearCurrentTaskResultWorkspace({ preserveSnapshot: true });
    expect(api.resultRows.value).toEqual([]);
    expect(api.taskResultCSVPath.value).toBe("");
    expect(taskSnapshot.value).not.toBeNull();
    api.clearCurrentTaskResultWorkspace();
    expect(taskSnapshot.value).toBeNull();
  });

  it("loadMoreResults：无任务或已取满时不请求", async () => {
    const { deps, task } = makeDeps();
    const api = useResults(deps);
    task.taskId = "  ";
    await api.loadMoreResults();
    task.taskId = "task-1";
    api.resultRows.value = [row("1.1.1.1")];
    api.resultsTotalCount.value = 1;
    await api.loadMoreResults();
    expect(listTaskResults).not.toHaveBeenCalled();
  });

  it("loadMoreResults：追加行、更新总数与工作区任务号", async () => {
    const { deps } = makeDeps();
    const api = useResults(deps);
    api.resultRows.value = [row("1.1.1.1")];
    api.resultsTotalCount.value = 5;
    await api.loadMoreResults();
    expect(listTaskResults).toHaveBeenCalledWith("task-1", "download", "desc", { path: "C:/out.csv" }, "all", { limit: 200, offset: 1 });
    expect(api.resultRows.value.map((r) => r.address)).toEqual(["1.1.1.1", "2.2.2.2"]);
    expect(api.resultsTotalCount.value).toBe(5);
    expect(api.resultWorkspaceTaskId.value).toBe("task-1");
    expect(api.resultsLoading.value).toBe(false);
    // 可见行返回副本，外部修改不影响内部状态。
    const visible = api.currentVisibleResultRows();
    visible.pop();
    expect(api.resultRows.value.length).toBe(2);
  });

  it("结果落地把整个批次写进后端历史,并用快照时间当测速时间", async () => {
    const { deps, taskSnapshot } = makeDeps();
    taskSnapshot.value = { task_id: "task-1", status: "completed", updated_at: "2026-01-20T12:00:00Z" } as TaskSnapshot;
    const api = useResults(deps);
    const rows = [row("1.1.1.1")];

    api.applyCurrentTaskResultWorkspace(" task-9 ", rows, 1);
    await vi.waitFor(() => expect(mergeResultHistoryBatch).toHaveBeenCalledTimes(1));

    expect(mergeResultHistoryBatch).toHaveBeenCalledWith("task-9", rows, Date.parse("2026-01-20T12:00:00Z"));
    expect(deps.refreshResultHistory).toHaveBeenCalledTimes(1);
  });

  it("历史写入失败只留日志,不打断结果页", async () => {
    const { deps } = makeDeps();
    mergeResultHistoryBatch.mockResolvedValue({ ok: false, code: "RESULT_HISTORY_WRITE_FAILED", message: "disk full", warnings: [] });
    const api = useResults(deps);

    api.applyCurrentTaskResultWorkspace("task-1", [row("1.1.1.1")], 1);
    await vi.waitFor(() => expect(deps.appendLog).toHaveBeenCalledWith("bridge.result_history.merge", expect.anything()));

    expect(deps.refreshResultHistory).not.toHaveBeenCalled();
    expect(api.resultRows.value.map((entry) => entry.address)).toEqual(["1.1.1.1"]);
  });
});
