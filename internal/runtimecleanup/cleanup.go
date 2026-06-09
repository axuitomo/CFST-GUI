package runtimecleanup

import (
	"context"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const DiagnosticsEnv = "CFST_RUNTIME_DIAGNOSTICS"
const DiagnosticsRemoteEnv = "CFST_RUNTIME_DIAGNOSTICS_REMOTE"
const defaultCleanupInterval = 8 * time.Hour

type Counts struct {
	PipelineResults int `json:"pipeline_results"`
	TaskSnapshots   int `json:"task_snapshots"`
}

type Status struct {
	CleanupCount           int    `json:"cleanup_count"`
	DiagnosticsEnabled     bool   `json:"diagnostics_enabled"`
	Goroutines             int    `json:"goroutines"`
	HeapAllocBytes         uint64 `json:"heap_alloc_bytes"`
	HeapInuseBytes         uint64 `json:"heap_inuse_bytes"`
	HeapSysBytes           uint64 `json:"heap_sys_bytes"`
	LastCleanupAt          string `json:"last_cleanup_at,omitempty"`
	LastCleanupReason      string `json:"last_cleanup_reason,omitempty"`
	LastHeavyCleanupAt     string `json:"last_heavy_cleanup_at,omitempty"`
	LastSkippedHeavyAt     string `json:"last_skipped_heavy_at,omitempty"`
	LastSkippedHeavyReason string `json:"last_skipped_heavy_reason,omitempty"`
	MemorySysBytes         uint64 `json:"memory_sys_bytes"`
	PipelineResults        int    `json:"pipeline_results"`
	HeavyCleanupCount      int    `json:"heavy_cleanup_count"`
	TaskSnapshots          int    `json:"task_snapshots"`
}

type Options struct {
	Interval     time.Duration
	Delay        time.Duration
	HeavyEvery   time.Duration
	IsBusy       func() bool
	LightCleanup func()
	HeavyCleanup func()
	Counts       func() Counts
	Now          func() time.Time
}

type Cleaner struct {
	opts Options

	mu               sync.Mutex
	cancel           context.CancelFunc
	runID            uint64
	delayedScheduled bool
	lastHeavy        time.Time
	status           Status
}

func New(opts Options) *Cleaner {
	if opts.Interval <= 0 {
		opts.Interval = defaultCleanupInterval
	}
	if opts.Delay <= 0 {
		opts.Delay = 30 * time.Second
	}
	if opts.HeavyEvery <= 0 {
		opts.HeavyEvery = defaultCleanupInterval
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.HeavyCleanup == nil {
		opts.HeavyCleanup = ReleaseGoMemory
	}
	return &Cleaner{opts: opts}
}

func (c *Cleaner) Start(ctx context.Context) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	if c.cancel != nil {
		c.mu.Unlock()
		return false
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.runID++
	runID := c.runID
	c.mu.Unlock()
	go c.loop(runCtx, runID)
	return true
}

func (c *Cleaner) Stop() {
	if c == nil {
		return
	}
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.runID++
	c.delayedScheduled = false
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *Cleaner) TriggerDelayed() {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.cancel == nil || c.delayedScheduled {
		c.mu.Unlock()
		return
	}
	c.delayedScheduled = true
	delay := c.opts.Delay
	c.mu.Unlock()
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			c.mu.Lock()
			if c.cancel == nil {
				c.delayedScheduled = false
				c.mu.Unlock()
				return
			}
			c.delayedScheduled = false
			c.mu.Unlock()
			c.RunLight("task_terminal")
		}
	}()
}

func (c *Cleaner) RunLight(reason string) Status {
	if c == nil {
		return Status{}
	}
	now := c.opts.Now()
	if c.opts.LightCleanup != nil {
		c.opts.LightCleanup()
	}
	return c.recordLightCleanup(now, reason)
}

func (c *Cleaner) RunOnce(reason string) Status {
	if c == nil {
		return Status{}
	}
	now := c.opts.Now()
	c.runLightCleanup()
	status := c.recordLightCleanup(now, reason)
	if c.opts.IsBusy != nil && c.opts.IsBusy() {
		return c.recordHeavySkipped(now, "busy")
	}
	if !c.shouldRunHeavy(now) {
		return status
	}
	if c.opts.HeavyCleanup != nil {
		c.opts.HeavyCleanup()
	}
	return c.recordHeavyCleanup(now)
}

func (c *Cleaner) Status() Status {
	if c == nil {
		return Status{DiagnosticsEnabled: DiagnosticsEnabled()}
	}
	c.mu.Lock()
	status := c.status
	c.mu.Unlock()
	return c.populateRuntimeStatus(status)
}

func DiagnosticsEnabled() bool {
	return envEnabled(DiagnosticsEnv)
}

func DiagnosticsRemoteEnabled() bool {
	return envEnabled(DiagnosticsRemoteEnv)
}

func envEnabled(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes") || strings.EqualFold(value, "on")
}

func ReleaseGoMemory() {
	runtime.GC()
	debug.FreeOSMemory()
}

func (c *Cleaner) loop(ctx context.Context, runID uint64) {
	ticker := time.NewTicker(c.opts.Interval)
	defer func() {
		ticker.Stop()
		c.mu.Lock()
		if c.runID == runID {
			c.cancel = nil
			c.delayedScheduled = false
		}
		c.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.RunOnce("periodic")
		}
	}
}

func (c *Cleaner) runLightCleanup() {
	if c.opts.LightCleanup != nil {
		c.opts.LightCleanup()
	}
}

func (c *Cleaner) recordLightCleanup(now time.Time, reason string) Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.CleanupCount++
	c.status.LastCleanupAt = now.Format(time.RFC3339)
	c.status.LastCleanupReason = strings.TrimSpace(reason)
	c.status = c.populateRuntimeStatus(c.status)
	return c.status
}

func (c *Cleaner) shouldRunHeavy(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastHeavy.IsZero() || now.Sub(c.lastHeavy) >= c.opts.HeavyEvery
}

func (c *Cleaner) recordHeavyCleanup(now time.Time) Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastHeavy = now
	c.status.HeavyCleanupCount++
	c.status.LastHeavyCleanupAt = now.Format(time.RFC3339)
	c.status = c.populateRuntimeStatus(c.status)
	return c.status
}

func (c *Cleaner) recordHeavySkipped(now time.Time, reason string) Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.LastSkippedHeavyAt = now.Format(time.RFC3339)
	c.status.LastSkippedHeavyReason = reason
	c.status = c.populateRuntimeStatus(c.status)
	return c.status
}

func (c *Cleaner) populateRuntimeStatus(status Status) Status {
	status.DiagnosticsEnabled = DiagnosticsEnabled()
	if c != nil && c.opts.Counts != nil {
		counts := c.opts.Counts()
		status.TaskSnapshots = counts.TaskSnapshots
		status.PipelineResults = counts.PipelineResults
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	status.Goroutines = runtime.NumGoroutine()
	status.HeapAllocBytes = stats.HeapAlloc
	status.HeapInuseBytes = stats.HeapInuse
	status.HeapSysBytes = stats.HeapSys
	status.MemorySysBytes = stats.Sys
	return status
}
