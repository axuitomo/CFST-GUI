package mobileapi

import (
	"context"
	"encoding/json"
	"testing"
)

func TestServiceInitStartsRuntimeCleanupOnce(t *testing.T) {
	service := NewService()
	response := service.Init(t.TempDir())
	if !commandOK(response) {
		t.Fatalf("Init response = %s, want ok", response)
	}
	first := service.cleaner
	if first == nil {
		t.Fatal("cleaner is nil after Init")
	}
	t.Cleanup(first.Stop)

	response = service.Init(t.TempDir())
	if !commandOK(response) {
		t.Fatalf("second Init response = %s, want ok", response)
	}
	if service.cleaner != first {
		t.Fatal("Init replaced cleaner, want idempotent reuse")
	}
}

func TestServiceRuntimeStatusDoesNotStartCleanupLoop(t *testing.T) {
	t.Setenv("CFST_RUNTIME_DIAGNOSTICS", "1")
	service := NewService()

	result := decodeCommandForTest(t, service.RuntimeStatus())
	if !boolValue(result["ok"], false) {
		t.Fatalf("RuntimeStatus = %#v, want ok", result)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleaner := service.ensureRuntimeCleaner()
	if !cleaner.Start(ctx) {
		t.Fatal("cleaner Start returned false, want status read to leave cleanup loop stopped")
	}
	t.Cleanup(cleaner.Stop)
}

func commandOK(raw string) bool {
	var result commandResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return false
	}
	return result.OK
}

func TestServiceRuntimeCleanupTrimsTerminalSnapshots(t *testing.T) {
	service := NewService()
	service.taskSnapshots["active-task"] = taskSnapshot{Status: "running", TaskID: "active-task"}
	service.taskSnapshots["done-task"] = taskSnapshot{Status: "completed", TaskID: "done-task"}

	service.trimRuntimeTaskSnapshots()

	if _, ok := service.taskSnapshots["active-task"]; !ok {
		t.Fatal("active snapshot was removed")
	}
	if _, ok := service.taskSnapshots["done-task"]; ok {
		t.Fatal("terminal snapshot still exists")
	}
}
