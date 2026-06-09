package runtimecleanup

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestCleanerStartIsIdempotent(t *testing.T) {
	cleaner := New(Options{Interval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !cleaner.Start(ctx) {
		t.Fatal("first Start returned false, want true")
	}
	if cleaner.Start(ctx) {
		t.Fatal("second Start returned true, want false")
	}
	cleaner.Stop()
}

func TestCleanerDefaultsUseEightHourCleanupPeriod(t *testing.T) {
	cleaner := New(Options{})
	if cleaner.opts.Interval != 8*time.Hour {
		t.Fatalf("Interval = %s, want 8h", cleaner.opts.Interval)
	}
	if cleaner.opts.HeavyEvery != 8*time.Hour {
		t.Fatalf("HeavyEvery = %s, want 8h", cleaner.opts.HeavyEvery)
	}
}

func TestCleanerSkipsHeavyCleanupWhenBusy(t *testing.T) {
	var heavy int32
	cleaner := New(Options{
		IsBusy:       func() bool { return true },
		HeavyCleanup: func() { atomic.AddInt32(&heavy, 1) },
	})
	status := cleaner.RunOnce("test")
	if got := atomic.LoadInt32(&heavy); got != 0 {
		t.Fatalf("heavy cleanup count = %d, want 0", got)
	}
	if status.LastSkippedHeavyReason != "busy" {
		t.Fatalf("LastSkippedHeavyReason = %q, want busy", status.LastSkippedHeavyReason)
	}
}

func TestCleanerRunsHeavyCleanupWhenIdle(t *testing.T) {
	var heavy int32
	cleaner := New(Options{
		HeavyCleanup: func() { atomic.AddInt32(&heavy, 1) },
	})
	status := cleaner.RunOnce("test")
	if got := atomic.LoadInt32(&heavy); got != 1 {
		t.Fatalf("heavy cleanup count = %d, want 1", got)
	}
	if status.HeavyCleanupCount != 1 {
		t.Fatalf("HeavyCleanupCount = %d, want 1", status.HeavyCleanupCount)
	}
}

func TestCleanerRunLightDoesNotRunHeavyCleanup(t *testing.T) {
	var light int32
	var heavy int32
	cleaner := New(Options{
		LightCleanup: func() { atomic.AddInt32(&light, 1) },
		HeavyCleanup: func() { atomic.AddInt32(&heavy, 1) },
	})

	status := cleaner.RunLight("task_terminal")
	if got := atomic.LoadInt32(&light); got != 1 {
		t.Fatalf("light cleanup count = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&heavy); got != 0 {
		t.Fatalf("heavy cleanup count = %d, want 0", got)
	}
	if status.LastCleanupReason != "task_terminal" {
		t.Fatalf("LastCleanupReason = %q, want task_terminal", status.LastCleanupReason)
	}
}

func TestCleanerDelayedCleanupIsLightOnly(t *testing.T) {
	var light int32
	var heavy int32
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	cleaner := New(Options{
		Delay:        time.Millisecond,
		Interval:     time.Hour,
		HeavyEvery:   time.Hour,
		LightCleanup: func() { atomic.AddInt32(&light, 1) },
		HeavyCleanup: func() { atomic.AddInt32(&heavy, 1) },
		Now:          func() time.Time { return now },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleaner.Start(ctx)
	defer cleaner.Stop()
	cleaner.TriggerDelayed()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&light) >= 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if got := atomic.LoadInt32(&light); got < 1 {
		t.Fatalf("light cleanup count = %d, want delayed cleanup", got)
	}
	if got := atomic.LoadInt32(&heavy); got != 0 {
		t.Fatalf("heavy cleanup count = %d, want delayed cleanup to stay light-only", got)
	}
}
