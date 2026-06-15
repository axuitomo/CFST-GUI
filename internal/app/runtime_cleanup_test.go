package app

import (
	"context"
	"testing"
)

func TestAppStartupStartsRuntimeCleanup(t *testing.T) {
	app := NewApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.startup(ctx)
	t.Cleanup(app.stopScheduler)

	if app.cleaner == nil {
		t.Fatal("cleaner is nil after startup")
	}
	if app.cleaner.Start(ctx) {
		t.Fatal("cleaner Start returned true, want already-running cleaner after startup")
	}
}

func TestAppRuntimeStatusDoesNotStartCleanupLoop(t *testing.T) {
	t.Setenv("CFST_RUNTIME_DIAGNOSTICS", "1")
	app := NewApp()

	result := app.GetRuntimeStatus()
	if !result.OK {
		t.Fatalf("GetRuntimeStatus = %#v, want ok", result)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleaner := app.ensureRuntimeCleaner()
	if !cleaner.Start(ctx) {
		t.Fatal("cleaner Start returned false, want status read to leave cleanup loop stopped")
	}
	t.Cleanup(cleaner.Stop)
}

func TestAppRuntimeCleanupTrimsTerminalSnapshots(t *testing.T) {
	app := NewApp()
	app.taskSnapshots["active-task"] = taskSnapshot{Status: "running", TaskID: "active-task"}
	app.taskSnapshots["done-task"] = taskSnapshot{Status: "completed", TaskID: "done-task"}

	app.trimRuntimeTaskSnapshots()

	if _, ok := app.taskSnapshots["active-task"]; !ok {
		t.Fatal("active snapshot was removed")
	}
	if _, ok := app.taskSnapshots["done-task"]; ok {
		t.Fatal("terminal snapshot still exists")
	}
}

func TestAppRuntimeCleanupBusyIncludesSchedulerWorkflow(t *testing.T) {
	app := NewApp()
	app.schedulerStatus.WorkflowStage = "dns"

	if !app.runtimeCleanupBusy() {
		t.Fatal("runtimeCleanupBusy = false, want true during scheduler DNS stage")
	}
}
