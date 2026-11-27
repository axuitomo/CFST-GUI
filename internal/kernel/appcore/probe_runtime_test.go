package appcore

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestProbeRuntimeIsolatesLifecycleAndCommitsTerminalOnce(t *testing.T) {
	runtime := NewProbeRuntime()
	ctx, ok, current := runtime.Start("task-1")
	if !ok || current != "task-1" || ctx.Err() != nil {
		t.Fatalf("Start = (%v, %q, %v)", ok, current, ctx.Err())
	}
	if _, ok, current = runtime.Start("task-2"); ok || current != "task-1" {
		t.Fatalf("second Start = (%v, %q), want active task-1", ok, current)
	}
	committed, active, cancelled := runtime.TryCommitCompletion("task-1")
	if !committed || !active || cancelled {
		t.Fatalf("first commit = (%v, %v, %v)", committed, active, cancelled)
	}
	committed, active, cancelled = runtime.TryCommitCompletion("task-1")
	if committed || active || cancelled {
		t.Fatalf("second commit = (%v, %v, %v), want rejected terminal", committed, active, cancelled)
	}
}

func TestProbeRuntimePauseResumeAndCancelInterrupts(t *testing.T) {
	runtime := NewProbeRuntime()
	ctx, ok, _ := runtime.Start("task-1")
	if !ok {
		t.Fatal("Start rejected")
	}
	var interrupted atomic.Int32
	unregister := runtime.RegisterInterrupt("task-1", "stage2_trace", func() { interrupted.Add(1) })
	defer unregister()

	if transition := runtime.Pause("task-1"); transition.Unavailable {
		t.Fatalf("Pause = %#v", transition)
	}
	if interrupted.Load() != 1 {
		t.Fatalf("pause interrupts = %d, want 1", interrupted.Load())
	}
	if transition := runtime.Resume("task-1"); transition.Unavailable {
		t.Fatalf("Resume = %#v", transition)
	}
	if transition := runtime.Cancel("task-1"); transition.Unavailable {
		t.Fatalf("Cancel = %#v", transition)
	}
	if interrupted.Load() != 2 {
		t.Fatalf("cancel interrupts = %d, want 2", interrupted.Load())
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("cancel did not close probe context")
	}
}

func TestProbeRuntimeWaitWhilePaused(t *testing.T) {
	runtime := NewProbeRuntime()
	_, ok, _ := runtime.Start("task-1")
	if !ok {
		t.Fatal("Start rejected")
	}
	runtime.Pause("task-1")
	done := make(chan struct{})
	var announced atomic.Int32
	go func() {
		runtime.WaitWhilePaused("task-1", func() { announced.Add(1) })
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	if announced.Load() != 1 {
		t.Fatalf("announcements = %d, want 1", announced.Load())
	}
	runtime.Resume("task-1")
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("waiter did not resume")
	}
}
