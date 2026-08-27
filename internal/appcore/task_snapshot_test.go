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
