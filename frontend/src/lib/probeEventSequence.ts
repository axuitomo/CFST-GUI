import type { ProbeEventEnvelope } from "./bridge/types";

export type ProbeEventSequenceDecision = "accept" | "duplicate" | "foreign" | "gap";

export function classifyProbeEventSequence(currentTaskId: string, lastSeq: number, event: ProbeEventEnvelope): ProbeEventSequenceDecision {
  const incomingTaskId = event.task_id.trim();
  const normalizedCurrentTaskId = currentTaskId.trim();
  if (incomingTaskId && normalizedCurrentTaskId && incomingTaskId !== normalizedCurrentTaskId) {
    return "foreign";
  }
  if (event.seq <= 0 || lastSeq <= 0) {
    return "accept";
  }
  if (event.seq <= lastSeq) {
    return "duplicate";
  }
  if (event.seq > lastSeq + 1) {
    return "gap";
  }
  return "accept";
}
