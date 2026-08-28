package appcore

import (
	"testing"
	"time"
)

func TestTaskSnapshotResumeEventClearsResumeCapable(t *testing.T) {
	base := TaskSnapshot{ResumeCapable: true, RuntimeAttached: true, SessionState: "paused_runtime"}
	event := TaskSnapshotFromEvent("task-1", "probe.resumed", map[string]any{}, time.Unix(1, 0))
	merged := MergeTaskSnapshots(base, event)
	if merged.ResumeCapable || !merged.RuntimeAttached || merged.SessionState != "active_runtime" {
		t.Fatalf("merged resume state = %#v", merged)
	}
}

func TestTaskSnapshotTerminalEventClearsRuntimeFlags(t *testing.T) {
	base := TaskSnapshot{ResumeCapable: true, RuntimeAttached: true, SessionState: "paused_runtime"}
	event := TaskSnapshotFromEvent("task-1", "probe.cancelled", map[string]any{}, time.Unix(1, 0))
	merged := MergeTaskSnapshots(base, event)
	if merged.ResumeCapable || merged.RuntimeAttached || merged.SessionState != "persisted_only" {
		t.Fatalf("merged terminal state = %#v", merged)
	}
}

func TestTaskSnapshotPersistsMCISProgressSeparately(t *testing.T) {
	payload := map[string]any{
		"candidate_count": 17, "completed": 64, "concurrency": 32, "elapsed_ms": 1500,
		"failed": 40, "last_colo": "HKG", "last_ip": "198.51.100.10", "last_ok": true,
		"source_id": "source-1", "source_name": "香港源", "stage": "stage0_mcis", "succeeded": 24, "total": 256,
	}
	snapshot := TaskSnapshotFromEvent("task-1", "probe.mcis.progress", payload, time.Unix(1, 0))
	if snapshot.CurrentStage != "stage0_mcis" || snapshot.Progress != nil || snapshot.MCISProgress == nil {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	progress := snapshot.MCISProgress
	if progress.Completed != 64 || progress.Total != 256 || progress.CandidateCount != 17 || progress.Succeeded != 24 || progress.Failed != 40 {
		t.Fatalf("MICS progress = %#v", progress)
	}
	merged := MergeTaskSnapshots(snapshot, TaskSnapshotFromEvent("task-1", "probe.progress", map[string]any{
		"processed": 1, "total": 10, "stage": "stage1_tcp",
	}, time.Unix(2, 0)))
	if merged.MCISProgress == nil || merged.MCISProgress.Completed != 64 || merged.Progress == nil || merged.Progress.Processed != 1 {
		t.Fatalf("merged snapshot = %#v", merged)
	}
}
