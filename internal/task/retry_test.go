package task

import (
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryAfterDelayParsesSecondsAndHTTPDate(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	if got := retryAfterDelay("5", now); got != 5*time.Second {
		t.Fatalf("seconds Retry-After = %v, want 5s", got)
	}
	if got := retryAfterDelay(now.Add(10*time.Second).Format(http.TimeFormat), now); got != 10*time.Second {
		t.Fatalf("date Retry-After = %v, want 10s", got)
	}
	if got := retryAfterDelay(now.Add(45*time.Second).Format(http.TimeFormat), now); got != maxRetryAfterWait {
		t.Fatalf("capped Retry-After = %v, want %v", got, maxRetryAfterWait)
	}
	if got := retryAfterDelay(now.Add(-time.Second).Format(http.TimeFormat), now); got != 0 {
		t.Fatalf("expired Retry-After = %v, want 0", got)
	}
	if got := retryAfterDelay("invalid", now); got != 0 {
		t.Fatalf("invalid Retry-After = %v, want 0", got)
	}
}

func TestSleepBeforeRetryReturnsWhenCanceledDuringBackoff(t *testing.T) {
	oldRetryBackoff := RetryBackoff
	oldCancelHook := ProbeCancelHook
	t.Cleanup(func() {
		RetryBackoff = oldRetryBackoff
		ProbeCancelHook = oldCancelHook
	})

	const stage = "stage-retry-cancel"
	const ip = "1.1.1.1"
	RetryBackoff = time.Second
	var canceled atomic.Bool
	ProbeCancelHook = func(cancelStage, cancelIP string) bool {
		return cancelStage == stage && cancelIP == ip && canceled.Load()
	}

	done := make(chan bool, 1)
	go func() {
		done <- sleepBeforeRetry(stage, ip, 1)
	}()

	time.Sleep(30 * time.Millisecond)
	canceled.Store(true)

	select {
	case wasCanceled := <-done:
		if !wasCanceled {
			t.Fatal("sleepBeforeRetry returned false, want cancellation")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("sleepBeforeRetry waited for the full backoff after cancellation")
	}
}

func TestSleepBeforeRetryChecksPauseDuringBackoff(t *testing.T) {
	oldRetryBackoff := RetryBackoff
	oldPauseHook := ProbePauseHook
	t.Cleanup(func() {
		RetryBackoff = oldRetryBackoff
		ProbePauseHook = oldPauseHook
	})

	const stage = "stage-retry-pause"
	const ip = "1.1.1.1"
	RetryBackoff = 200 * time.Millisecond
	pauseChecked := make(chan struct{})
	var closed atomic.Bool
	ProbePauseHook = func(pauseStage, pauseIP string) {
		if pauseStage == stage && pauseIP == ip && closed.CompareAndSwap(false, true) {
			close(pauseChecked)
		}
	}

	done := make(chan bool, 1)
	go func() {
		done <- sleepBeforeRetry(stage, ip, 1)
	}()

	select {
	case <-pauseChecked:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("sleepBeforeRetry did not check pause during backoff")
	}

	select {
	case wasCanceled := <-done:
		if wasCanceled {
			t.Fatal("sleepBeforeRetry returned cancellation without cancel hook")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("sleepBeforeRetry did not finish after backoff")
	}
}

func TestStageCooldownReturnsWhenCanceledDuringCooldown(t *testing.T) {
	oldCooldownFails := CooldownConsecutiveFails
	oldCooldownDuration := CooldownDuration
	oldCancelHook := ProbeCancelHook
	stageCooldownMu.Lock()
	oldCounts := stageConsecutiveFailCount
	stageConsecutiveFailCount = map[string]int{}
	stageCooldownMu.Unlock()
	t.Cleanup(func() {
		CooldownConsecutiveFails = oldCooldownFails
		CooldownDuration = oldCooldownDuration
		ProbeCancelHook = oldCancelHook
		stageCooldownMu.Lock()
		stageConsecutiveFailCount = oldCounts
		stageCooldownMu.Unlock()
	})

	const stage = "stage-cooldown-cancel"
	const ip = "1.1.1.1"
	CooldownConsecutiveFails = 1
	CooldownDuration = time.Second
	var canceled atomic.Bool
	ProbeCancelHook = func(cancelStage, cancelIP string) bool {
		return cancelStage == stage && cancelIP == ip && canceled.Load()
	}

	done := make(chan bool, 1)
	go func() {
		done <- noteStageProbeOutcome(stage, ip, false)
	}()

	time.Sleep(30 * time.Millisecond)
	canceled.Store(true)

	select {
	case wasCanceled := <-done:
		if !wasCanceled {
			t.Fatal("noteStageProbeOutcome returned false, want cancellation")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("stage cooldown waited for the full delay after cancellation")
	}
}
