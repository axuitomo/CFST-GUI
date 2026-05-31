package app

import (
	"testing"
	"time"
)

func TestResumeProbeUpdatesSnapshotToActiveRuntime(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	emitter := newDesktopProbeEmitter(app, "resume-task", 0)
	if ok, _ := app.setCurrentProbeTask("resume-task", emitter); !ok {
		t.Fatal("setCurrentProbeTask returned false")
	}
	t.Cleanup(func() {
		app.clearCurrentProbeTask("resume-task")
	})

	app.probeControlMu.Lock()
	app.pauseRequested = true
	app.pausedTaskID = "resume-task"
	app.probeControlMu.Unlock()

	err := app.writeTaskSnapshot(taskSnapshot{
		CurrentStage: "stage1_tcp",
		Progress: &taskProgressSnapshot{
			Stage: "stage1_tcp",
		},
		RuntimeAttached: true,
		SessionState:    "paused_runtime",
		StartedAt:       time.Now().Format(time.RFC3339),
		Status:          "cooling",
		TaskID:          "resume-task",
		UpdatedAt:       time.Now().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("writeTaskSnapshot: %v", err)
	}

	result := app.ResumeProbe(map[string]any{"task_id": "resume-task"})
	if !result.OK {
		t.Fatalf("ResumeProbe = %#v, want ok", result)
	}

	snapshot, ok, err := app.loadTaskSnapshot("resume-task")
	if err != nil {
		t.Fatalf("loadTaskSnapshot: %v", err)
	}
	if !ok {
		t.Fatal("loadTaskSnapshot = not found, want snapshot")
	}
	if snapshot.SessionState != "active_runtime" {
		t.Fatalf("session_state = %q, want active_runtime", snapshot.SessionState)
	}
	if snapshot.ResumeCapable {
		t.Fatal("resume_capable = true, want false after resume")
	}
	if !snapshot.RuntimeAttached {
		t.Fatal("runtime_attached = false, want true after resume")
	}
	if snapshot.Status != "running" {
		t.Fatalf("status = %q, want running", snapshot.Status)
	}
	if snapshot.CurrentStage != "stage1_tcp" {
		t.Fatalf("current_stage = %q, want stage1_tcp", snapshot.CurrentStage)
	}

	app.probeControlMu.Lock()
	defer app.probeControlMu.Unlock()
	if app.pauseRequested || app.pausedTaskID != "" {
		t.Fatalf("pause state = (%v,%q), want cleared", app.pauseRequested, app.pausedTaskID)
	}
}
