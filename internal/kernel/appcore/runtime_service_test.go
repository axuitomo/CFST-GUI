package appcore

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestRuntimeCleanupLifecycleIsOwnedByService(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(service.StopRuntimeCleanup)

	if !service.StartRuntimeCleanup(ctx) {
		t.Fatal("first StartRuntimeCleanup returned false")
	}
	if service.StartRuntimeCleanup(ctx) {
		t.Fatal("second StartRuntimeCleanup returned true")
	}
}

func TestRuntimeStatusDoesNotStartCleanupLoop(t *testing.T) {
	t.Setenv("CFST_RUNTIME_DIAGNOSTICS", "1")
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	result := service.RuntimeStatus()
	if !result.OK || result.Code != "RUNTIME_STATUS_READY" {
		t.Fatalf("RuntimeStatus() = %#v", result)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(service.StopRuntimeCleanup)
	if !service.StartRuntimeCleanup(ctx) {
		t.Fatal("RuntimeStatus started the cleanup loop")
	}
}

func TestRuntimeCleanupUsesInjectedPlatformHooks(t *testing.T) {
	busy := true
	cleanupCount := 0
	service := NewService(ServiceOptions{
		RuntimeCleanupBusy: func() bool { return busy },
		RuntimeCleanupHook: func() { cleanupCount++ },
		Storage:            StorageLayout{Root: t.TempDir()},
	})
	status := service.ensureRuntimeCleaner().RunOnce("test")
	if cleanupCount != 1 {
		t.Fatalf("cleanup hook count = %d, want 1", cleanupCount)
	}
	if status.LastSkippedHeavyReason != "busy" {
		t.Fatalf("LastSkippedHeavyReason = %q, want busy", status.LastSkippedHeavyReason)
	}
}

func TestCleanupExpiredTerminalTaskFiles(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, time.June, 18, 12, 0, 0, 0, time.UTC)
	service := NewService(ServiceOptions{
		Clock:   func() time.Time { return now },
		Storage: StorageLayout{Root: root, ConfigFileName: "config.json"},
	})
	if _, err := service.SaveConfig(map[string]any{
		"maintenance": map[string]any{"completed_task_retention_days": 7},
	}); err != nil {
		t.Fatal(err)
	}

	writeSnapshot := func(snapshot TaskSnapshot) {
		t.Helper()
		if err := service.taskStore.WriteSnapshot(snapshot, TaskAttachment{}); err != nil {
			t.Fatal(err)
		}
	}
	old := TaskSnapshot{CompletedAt: now.Add(-8 * 24 * time.Hour).Format(time.RFC3339), Status: "completed", TaskID: "old", UpdatedAt: now.Add(-8 * 24 * time.Hour).Format(time.RFC3339)}
	recent := TaskSnapshot{CompletedAt: now.Add(-2 * 24 * time.Hour).Format(time.RFC3339), Status: "failed", TaskID: "recent", UpdatedAt: now.Add(-2 * 24 * time.Hour).Format(time.RFC3339)}
	writeSnapshot(old)
	writeSnapshot(recent)
	if err := service.taskStore.WriteResults(old.TaskID, []ProbeResultRow{{Address: "1.1.1.1"}}); err != nil {
		t.Fatal(err)
	}

	service.CleanupExpiredTerminalTaskFiles(now)
	if _, err := os.Stat(service.taskStore.SnapshotPath(old.TaskID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old snapshot err = %v, want not exist", err)
	}
	if _, err := os.Stat(service.taskStore.ResultsPath(old.TaskID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old results err = %v, want not exist", err)
	}
	if _, err := os.Stat(service.taskStore.SnapshotPath(recent.TaskID)); err != nil {
		t.Fatalf("recent snapshot should be preserved: %v", err)
	}

	if _, err := service.SaveConfig(map[string]any{
		"maintenance": map[string]any{"completed_task_retention_days": 0},
	}); err != nil {
		t.Fatal(err)
	}
	disabled := TaskSnapshot{CompletedAt: now.Add(-30 * 24 * time.Hour).Format(time.RFC3339), Status: "no_results", TaskID: "disabled", UpdatedAt: now.Add(-30 * 24 * time.Hour).Format(time.RFC3339)}
	writeSnapshot(disabled)
	service.CleanupExpiredTerminalTaskFiles(now)
	if _, err := os.Stat(service.taskStore.SnapshotPath(disabled.TaskID)); err != nil {
		t.Fatalf("disabled retention snapshot should be preserved: %v", err)
	}
}
