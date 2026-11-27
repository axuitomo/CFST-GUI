import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { useProbeTask, type ProbeTaskDeps } from "./useProbeTask";

const { resumeProbe, startProbe, stopProbe } = vi.hoisted(() => ({
  resumeProbe: vi.fn(),
  startProbe: vi.fn(),
  stopProbe: vi.fn(),
}));

vi.mock("../lib/bridge", () => ({
  resumeProbe,
  startProbe,
  stopProbe,
}));

function makeDeps(overrides: Partial<ProbeTaskDeps> = {}) {
  return {
    loading: ref(false),
    probeWarnings: ref<string[]>([]),
    pushActivity: vi.fn(),
    showToast: vi.fn(),
    navigateTo: vi.fn(),
    setStatus: vi.fn(),
    appendLog: vi.fn(),
    pushProcessTrace: vi.fn(),
    pushWarningTrace: vi.fn(),
    clearProcessTrace: vi.fn(),
    buildConfigSnapshot: vi.fn(() => ({}) as never),
    reconcileTaskData: vi.fn(async () => {}),
    resetDownloadSpeedState: vi.fn(),
    captureDownloadSpeedTaskConfig: vi.fn(),
    clearCurrentTaskResultWorkspace: vi.fn(),
    clearResultRows: vi.fn(),
    preparedSources: vi.fn(() => []),
    sourcePayloads: vi.fn(() => []),
    applySourceStatuses: vi.fn(),
    ...overrides,
  } satisfies ProbeTaskDeps;
}

describe("useProbeTask", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("derives task actions from runtime state", () => {
    const state = useProbeTask(makeDeps());
    state.task.taskId = "task-a";
    state.task.active = true;
    state.task.stage = "stage1_tcp";
    state.taskSessionState.value = "active_runtime";

    expect(state.canCancelTask.value).toBe(true);
    expect(state.canPauseTask.value).toBe(true);
    expect(state.canStartTask.value).toBe(false);

    state.beginTaskAction("pause");
    expect(state.taskActionInFlight.value).toBe(true);
    expect(state.canCancelTask.value).toBe(false);
    state.finishTaskAction("pause");
    expect(state.taskActionInFlight.value).toBe(false);
  });

  it("keeps every task action disabled while the board only shows optimistic state", () => {
    const state = useProbeTask(makeDeps());
    state.task.taskId = "task-a";
    state.task.active = true;
    state.task.stage = "stage1_tcp";
    state.taskSessionState.value = "active_runtime";

    state.startupSyncing.value = true;
    expect(state.canCancelTask.value).toBe(false);
    expect(state.canPauseTask.value).toBe(false);
    expect(state.canResumeTask.value).toBe(false);
    expect(state.canStartTask.value).toBe(false);

    state.startupSyncing.value = false;
    expect(state.canCancelTask.value).toBe(true);
  });

  it("never exposes resume for a persisted-only snapshot", () => {
    const state = useProbeTask(makeDeps());
    state.task.taskId = "task-a";
    state.task.active = false;
    state.taskSnapshot.value = {
      resume_capable: true,
      runtime_attached: false,
      session_state: "persisted_only",
      status: "failed",
      task_id: "task-a",
      updated_at: "2026-08-06T00:00:00Z",
    };

    expect(state.hasDetachedTaskSnapshot.value).toBe(true);
    expect(state.canResumeTask.value).toBe(false);
    expect(state.canStartTask.value).toBe(true);
  });
  it("uses the MICS budget for progress while sampling", () => {
    const state = useProbeTask(makeDeps());
    state.task.stage = "stage0_mcis";
    state.mcisProgress.active = true;
    state.mcisProgress.completed = 64;
    state.mcisProgress.total = 256;

    expect(state.progressPercent.value).toBe(25);

    state.mcisProgress.active = false;
    state.summary.processed = 3;
    state.summary.total = 10;
    expect(state.progressPercent.value).toBe(30);
  });

  it("uses the download success quota for progress when the stage stops early", () => {
    const state = useProbeTask(makeDeps());
    state.task.stage = "stage3_get";
    state.summary.processed = 12;
    state.summary.passed = 2;
    state.summary.total = 2000;
    state.summary.downloadTarget = 5;

    expect(state.progressPercent.value).toBe(40);

    state.summary.downloadTarget = 0;
    expect(state.progressPercent.value).toBe(1);
  });

  it("blocks launch without usable sources and never touches the bridge", async () => {
    const deps = makeDeps();
    const state = useProbeTask(deps);
    await state.launchProbe();
    expect(startProbe).not.toHaveBeenCalled();
    expect(deps.showToast).toHaveBeenCalledWith("请先配置至少一个输入源", "error");
    expect(state.task.active).toBe(false);
    expect(state.taskActionInFlight.value).toBe(false);
  });

  it("blocks launch while another task runs and routes to the dashboard", async () => {
    const deps = makeDeps({ preparedSources: vi.fn(() => [{} as never]) });
    const state = useProbeTask(deps);
    state.task.taskId = "task-a";
    state.task.active = true;
    state.taskSessionState.value = "active_runtime";
    await state.launchProbe();
    expect(startProbe).not.toHaveBeenCalled();
    expect(deps.showToast).toHaveBeenCalledWith("已有任务运行中", "error");
    expect(deps.navigateTo).toHaveBeenCalledWith("dashboard");
  });

  it("reports a failed pause request and releases the action lock", async () => {
    stopProbe.mockResolvedValueOnce({ ok: false, message: "暂停被拒绝" });
    const deps = makeDeps();
    const state = useProbeTask(deps);
    state.task.taskId = "task-a";
    await state.pauseProbe();
    expect(stopProbe).toHaveBeenCalledWith({ mode: "pause", task_id: "task-a" });
    expect(deps.showToast).toHaveBeenCalledWith("暂停失败", "error");
    expect(state.taskActionInFlight.value).toBe(false);
    expect(deps.loading.value).toBe(false);
  });

  it("refuses to continue a persisted-only snapshot and routes to results", async () => {
    const deps = makeDeps();
    const state = useProbeTask(deps);
    state.task.taskId = "task-a";
    state.taskSnapshot.value = {
      resume_capable: true,
      runtime_attached: false,
      session_state: "persisted_only",
      status: "completed",
      task_id: "task-a",
      updated_at: "2026-08-06T00:00:00Z",
    };
    await state.continueProbe();
    expect(resumeProbe).not.toHaveBeenCalled();
    expect(deps.showToast).toHaveBeenCalledWith("历史快照不能直接继续", "info");
    expect(deps.navigateTo).toHaveBeenCalledWith("results");
    expect(state.taskActionInFlight.value).toBe(false);
  });
});
