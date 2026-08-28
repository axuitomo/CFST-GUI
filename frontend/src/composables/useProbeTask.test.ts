import { describe, expect, it } from "vitest";
import { useProbeTask } from "./useProbeTask";

describe("useProbeTask", () => {
  it("derives task actions from runtime state", () => {
    const state = useProbeTask();
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

  it("never exposes resume for a persisted-only snapshot", () => {
    const state = useProbeTask();
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
    const state = useProbeTask();
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
});
