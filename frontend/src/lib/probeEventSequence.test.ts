import { describe, expect, it } from "vitest";
import { classifyProbeEventSequence } from "./probeEventSequence";
import type { ProbeEventEnvelope } from "./bridge/types";

function event(taskId: string, seq: number): ProbeEventEnvelope {
  return {
    event: "probe.progress",
    payload: {},
    schema_version: "test",
    seq,
    task_id: taskId,
    ts: "2026-08-06T00:00:00Z",
  };
}

describe("classifyProbeEventSequence", () => {
  it("rejects foreign and duplicate events", () => {
    expect(classifyProbeEventSequence("task-a", 4, event("task-b", 5))).toBe("foreign");
    expect(classifyProbeEventSequence("task-a", 4, event("task-a", 4))).toBe("duplicate");
  });

  it("detects a replay gap and accepts the next event", () => {
    expect(classifyProbeEventSequence("task-a", 4, event("task-a", 6))).toBe("gap");
    expect(classifyProbeEventSequence("task-a", 4, event("task-a", 5))).toBe("accept");
  });

  it("accepts legacy events without sequence numbers", () => {
    expect(classifyProbeEventSequence("task-a", 4, event("task-a", 0))).toBe("accept");
  });
});
