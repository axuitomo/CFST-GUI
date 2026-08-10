package appcore

import (
	"context"
	"strings"
	"sync"
	"time"
)

type ProbeRuntimeState struct {
	CurrentTaskID     string
	PausedTaskID      string
	PauseRequested    bool
	CancelRequested   bool
	TerminalCommitted bool
}

type ProbeControlTransition struct {
	TaskID      string
	WasPaused   bool
	Unavailable bool
	Terminal    bool
}

type ProbeRuntime struct {
	mu   sync.Mutex
	cond *sync.Cond

	currentTaskID       string
	pausedTaskID        string
	pauseRequested      bool
	cancelRequested     bool
	pendingCancelTaskID string
	terminalCommitted   bool
	ctx                 context.Context
	cancel              context.CancelFunc

	interrupts   map[int64]func()
	interruptSeq int64

	progressThrottle  time.Duration
	lastProgressStage string
	lastProgressAt    time.Time
}

func NewProbeRuntime() *ProbeRuntime {
	runtime := &ProbeRuntime{}
	runtime.cond = sync.NewCond(&runtime.mu)
	return runtime
}

func (r *ProbeRuntime) Start(taskID string) (context.Context, bool, string) {
	taskID = strings.TrimSpace(taskID)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.currentTaskID != "" {
		return r.contextLocked(), false, r.currentTaskID
	}
	r.currentTaskID = taskID
	r.pausedTaskID = ""
	r.pauseRequested = false
	r.cancelRequested = false
	r.terminalCommitted = false
	r.interrupts = nil
	r.ctx, r.cancel = context.WithCancel(context.Background())
	if r.pendingCancelTaskID == taskID {
		r.cancelRequested = true
		r.pendingCancelTaskID = ""
		r.cancel()
	}
	r.cond.Broadcast()
	return r.ctx, true, taskID
}

func (r *ProbeRuntime) QueueCancel(taskID string) bool {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.currentTaskID != "" {
		return false
	}
	r.pendingCancelTaskID = taskID
	return true
}

func (r *ProbeRuntime) Clear(taskID string) {
	taskID = strings.TrimSpace(taskID)
	r.mu.Lock()
	var cancel context.CancelFunc
	if r.currentTaskID == taskID {
		cancel = r.cancel
		r.currentTaskID = ""
		r.pausedTaskID = ""
		r.pauseRequested = false
		r.cancelRequested = false
		r.terminalCommitted = false
		r.interrupts = nil
		r.ctx = nil
		r.cancel = nil
		r.cond.Broadcast()
	}
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (r *ProbeRuntime) State() ProbeRuntimeState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return ProbeRuntimeState{
		CurrentTaskID:     r.currentTaskID,
		PausedTaskID:      r.pausedTaskID,
		PauseRequested:    r.pauseRequested,
		CancelRequested:   r.cancelRequested,
		TerminalCommitted: r.terminalCommitted,
	}
}

func (r *ProbeRuntime) Context(taskID string) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.currentTaskID == strings.TrimSpace(taskID) {
		return r.contextLocked()
	}
	return context.Background()
}

func (r *ProbeRuntime) Pause(taskID string) ProbeControlTransition {
	r.mu.Lock()
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		taskID = r.currentTaskID
	}
	transition := ProbeControlTransition{TaskID: taskID}
	if taskID == "" || taskID != r.currentTaskID {
		transition.Unavailable = true
		r.mu.Unlock()
		return transition
	}
	if r.terminalCommitted {
		transition.Unavailable = true
		transition.Terminal = true
		r.mu.Unlock()
		return transition
	}
	r.pauseRequested = true
	r.pausedTaskID = taskID
	interrupts := r.interruptsLocked()
	r.cond.Broadcast()
	r.mu.Unlock()
	runInterrupts(interrupts)
	return transition
}

func (r *ProbeRuntime) Resume(taskID string) ProbeControlTransition {
	r.mu.Lock()
	defer r.mu.Unlock()
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		taskID = r.pausedTaskID
	}
	transition := ProbeControlTransition{TaskID: taskID}
	if taskID == "" || taskID != r.pausedTaskID || !r.pauseRequested {
		transition.Unavailable = true
		return transition
	}
	r.pauseRequested = false
	r.pausedTaskID = ""
	r.cond.Broadcast()
	return transition
}

func (r *ProbeRuntime) Cancel(taskID string) ProbeControlTransition {
	r.mu.Lock()
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		taskID = r.currentTaskID
	}
	transition := ProbeControlTransition{TaskID: taskID}
	if taskID == "" || taskID != r.currentTaskID {
		transition.Unavailable = true
		r.mu.Unlock()
		return transition
	}
	if r.terminalCommitted {
		transition.Unavailable = true
		transition.Terminal = true
		r.mu.Unlock()
		return transition
	}
	transition.WasPaused = r.pauseRequested && r.pausedTaskID == taskID
	r.cancelRequested = true
	r.pauseRequested = false
	r.pausedTaskID = ""
	cancel := r.cancel
	interrupts := r.interruptsLocked()
	r.cond.Broadcast()
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	runInterrupts(interrupts)
	return transition
}

func (r *ProbeRuntime) IsCancelRequested(taskID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentTaskID == strings.TrimSpace(taskID) && r.cancelRequested
}

func (r *ProbeRuntime) TryCommitCompletion(taskID string) (committed, active, cancelled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.currentTaskID != strings.TrimSpace(taskID) {
		return false, false, true
	}
	if r.cancelRequested {
		return false, true, true
	}
	if r.pauseRequested && r.pausedTaskID == taskID {
		return false, true, false
	}
	if r.terminalCommitted {
		return false, false, false
	}
	r.terminalCommitted = true
	return true, true, false
}

func (r *ProbeRuntime) RegisterInterrupt(taskID, stage string, interrupt func()) func() {
	if interrupt == nil {
		return func() {}
	}
	r.mu.Lock()
	if r.currentTaskID != strings.TrimSpace(taskID) {
		r.mu.Unlock()
		return func() {}
	}
	r.interruptSeq++
	seq := r.interruptSeq
	if r.interrupts == nil {
		r.interrupts = make(map[int64]func())
	}
	r.interrupts[seq] = interrupt
	shouldInterrupt := r.pauseRequested || r.cancelRequested
	r.mu.Unlock()
	if shouldInterrupt {
		go interrupt()
	}
	return func() {
		r.mu.Lock()
		if r.currentTaskID == strings.TrimSpace(taskID) && r.interrupts != nil {
			delete(r.interrupts, seq)
			if len(r.interrupts) == 0 {
				r.interrupts = nil
			}
		}
		r.mu.Unlock()
	}
}

func (r *ProbeRuntime) WaitWhilePaused(taskID string, onPaused func()) {
	r.mu.Lock()
	announced := false
	for r.currentTaskID == strings.TrimSpace(taskID) && r.pauseRequested && r.pausedTaskID == taskID {
		if !announced && onPaused != nil {
			r.mu.Unlock()
			onPaused()
			r.mu.Lock()
			announced = true
			continue
		}
		r.cond.Wait()
	}
	r.mu.Unlock()
}

func (r *ProbeRuntime) WaitStopped(taskID string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if r.State().CurrentTaskID != strings.TrimSpace(taskID) {
			return true
		}
		if timeout > 0 && !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (r *ProbeRuntime) ConfigureProgressThrottle(throttle time.Duration) {
	if throttle <= 0 {
		throttle = 100 * time.Millisecond
	}
	r.mu.Lock()
	r.progressThrottle = throttle
	r.lastProgressStage = ""
	r.lastProgressAt = time.Time{}
	r.mu.Unlock()
}

func (r *ProbeRuntime) ShouldEmitProgress(stage string, processed, total int, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	throttle := r.progressThrottle
	if throttle <= 0 {
		throttle = 100 * time.Millisecond
	}
	shouldEmit := processed <= 1 || total <= 0 || processed >= total || stage != r.lastProgressStage || now.Sub(r.lastProgressAt) >= throttle
	if shouldEmit {
		r.lastProgressStage = stage
		r.lastProgressAt = now
	}
	return shouldEmit
}

func (r *ProbeRuntime) contextLocked() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

func (r *ProbeRuntime) interruptsLocked() []func() {
	interrupts := make([]func(), 0, len(r.interrupts))
	for _, interrupt := range r.interrupts {
		if interrupt != nil {
			interrupts = append(interrupts, interrupt)
		}
	}
	return interrupts
}

func runInterrupts(interrupts []func()) {
	for _, interrupt := range interrupts {
		interrupt()
	}
}
