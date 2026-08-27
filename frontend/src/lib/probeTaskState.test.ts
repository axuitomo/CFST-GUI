import { describe, expect, it } from "vitest";
import { deriveTaskStateFromProbeEvent } from "./bridge";
import type { ProbeEventEnvelope } from "./bridge/types";

function event(name: string, payload: Record<string, unknown>): ProbeEventEnvelope {
  return {
    event: name,
    payload,
    schema_version: "test",
    seq: 1,
    task_id: "task-a",
    ts: "2026-08-06T00:00:00Z",
  };
}

describe("deriveTaskStateFromProbeEvent", () => {
  it("keeps partial exports nonterminal", () => {
    expect(deriveTaskStateFromProbeEvent(event("probe.partial_export", { written: 3 }))).toMatchObject({
      title: "结果已落盘",
      tone: "running",
    });
  });

  it("distinguishes an empty preprocessed pool", () => {
    expect(deriveTaskStateFromProbeEvent(event("probe.preprocessed", { accepted: 0, filtered: 2, invalid: 1, total: 3 }))).toMatchObject({
      title: "IP池没有可用结果",
      tone: "no_results",
    });
  });

  it("summarizes trace-stage failures", () => {
    const state = deriveTaskStateFromProbeEvent(
      event("probe.failed", {
        failure_stage: "stage2_trace",
        trace_diagnostics: { reason_counts: { status_mismatch: 2 } },
      }),
    );
    expect(state.tone).toBe("failed");
    expect(state.detail).toContain("状态码不匹配 2 次");
  });
});
