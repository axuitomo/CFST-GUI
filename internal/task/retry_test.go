package task

import (
	"context"
	"net/http"
	"testing"
	"testing/synctest"
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

func TestRetryWaitStopsWhenProbeContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	engine := NewEngine(Config{}, Hooks{ProbeContext: func() context.Context { return ctx }})
	cancel()

	startedAt := time.Now()
	engine.sleepBeforeRetryDelay("stage2_trace", "1.1.1.1", 1, time.Second, "retry_backoff", "retry")
	if elapsed := time.Since(startedAt); elapsed > 100*time.Millisecond {
		t.Fatalf("canceled retry wait took %v, want prompt return", elapsed)
	}
}

func TestCooldownWaitStopsWhenProbeContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	engine := NewEngine(Config{
		CooldownFailures: 1,
		CooldownDuration: time.Second,
	}, Hooks{ProbeContext: func() context.Context { return ctx }})
	cancel()

	startedAt := time.Now()
	engine.noteStageProbeOutcome("stage2_trace", "1.1.1.1", probeSystemicFailed)
	if elapsed := time.Since(startedAt); elapsed > 100*time.Millisecond {
		t.Fatalf("canceled cooldown wait took %v, want prompt return", elapsed)
	}
}

func TestRetryWaitEntersPausePromptlyAndFreezesDelay(t *testing.T) {
	if testing.Short() {
		t.Skip("实时暂停编排；退避/冷却时长契约由 synctest 虚拟时钟测试 TestRetryAndCooldownUseConfiguredDelays 覆盖")
	}
	pauseRequested := make(chan struct{})
	pauseEntered := make(chan struct{}, 1)
	resume := make(chan struct{})
	engine := NewEngine(Config{}, Hooks{
		ProbeContext: func() context.Context { return context.Background() },
		ProbePause: func(stage, ip string) {
			select {
			case <-pauseRequested:
				select {
				case pauseEntered <- struct{}{}:
				default:
				}
				<-resume
			default:
			}
		},
	})

	done := make(chan time.Duration, 1)
	startedAt := time.Now()
	go func() {
		engine.waitProbeDelay("stage2_trace", "1.1.1.1", 150*time.Millisecond)
		done <- time.Since(startedAt)
	}()
	time.Sleep(40 * time.Millisecond)
	close(pauseRequested)
	select {
	case <-pauseEntered:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("retry wait did not enter pause promptly")
	}
	time.Sleep(100 * time.Millisecond)
	close(resume)

	select {
	case elapsed := <-done:
		if elapsed < 230*time.Millisecond {
			t.Fatalf("retry wait elapsed = %v, want pause duration excluded from delay", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("retry wait did not finish after resume")
	}
}

// 退避和冷却的时长是行为契约，但按真实时间验证要等几百毫秒、还会被机器负载带偏。
// synctest 用虚拟时钟跑 time.Sleep，既精确又不占真实时间。
func TestRetryAndCooldownUseConfiguredDelays(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		config := DefaultConfig()
		config.RetryBackoff = 120 * time.Millisecond
		config.CooldownFailures = 3
		config.CooldownDuration = 250 * time.Millisecond
		cooldowns := 0
		engine := NewEngine(config, Hooks{DebugEvent: func(event string, _ map[string]any) {
			if event == "stage.cooldown" {
				cooldowns++
			}
		}})

		startedAt := time.Now()
		engine.sleepBeforeRetry("stage2_trace", "1.1.1.1", 1)
		if elapsed := time.Since(startedAt); elapsed != config.RetryBackoff {
			t.Fatalf("退避等待 = %v, want %v", elapsed, config.RetryBackoff)
		}

		startedAt = time.Now()
		engine.noteStageProbeOutcome("stage2_trace", "1.1.1.1", probeSystemicFailed)
		engine.noteStageProbeOutcome("stage2_trace", "1.1.1.1", probeSystemicFailed)
		if elapsed := time.Since(startedAt); elapsed != 0 {
			t.Fatalf("未达连续失败阈值就等待了 %v", elapsed)
		}
		engine.noteStageProbeOutcome("stage2_trace", "1.1.1.1", probeSystemicFailed)
		if elapsed := time.Since(startedAt); elapsed != config.CooldownDuration {
			t.Fatalf("冷却等待 = %v, want %v", elapsed, config.CooldownDuration)
		}
		if cooldowns != 1 {
			t.Fatalf("stage.cooldown 事件 = %d, want 1", cooldowns)
		}

		// 触发冷却后计数归零，接下来的两次失败不该再等。
		startedAt = time.Now()
		engine.noteStageProbeOutcome("stage2_trace", "1.1.1.1", probeSystemicFailed)
		engine.noteStageProbeOutcome("stage2_trace", "1.1.1.1", probeSystemicFailed)
		if elapsed := time.Since(startedAt); elapsed != 0 {
			t.Fatalf("冷却后重新计数期间等待了 %v", elapsed)
		}
	})
}

// 候选不通的慢失败不得触发冷却：实测一次 13863 颗候选里 11948 颗吃满超时，按旧逻辑
// 它们触发了 1051 次 2500ms 冷却，白吃掉阶段 1 约 11% 的墙钟时间，而同时在跑的另外
// 199 个 worker 一个都没慢下来——对远端压力没有任何缓解。
func TestCooldownIgnoresSlowCandidateFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		config := DefaultConfig()
		config.CooldownFailures = 1
		config.CooldownDuration = 250 * time.Millisecond
		cooldowns := 0
		engine := NewEngine(config, Hooks{DebugEvent: func(event string, _ map[string]any) {
			if event == "stage.cooldown" {
				cooldowns++
			}
		}})

		budget := 500 * time.Millisecond
		startedAt := time.Now()
		engine.noteStageProbeOutcome("stage1_tcp", "1.1.1.1", classifyProbeOutcome(false, budget, budget))
		if elapsed := time.Since(startedAt); elapsed != 0 {
			t.Fatalf("候选超时的慢失败触发了 %v 冷却", elapsed)
		}
		if cooldowns != 0 {
			t.Fatalf("stage.cooldown 事件 = %d, want 0", cooldowns)
		}

		// 阶段 3 的失败一律按候选侧处理（budget 传 0），同样不冷却。
		engine.noteStageProbeOutcome("stage3_get", "1.1.1.3", classifyProbeOutcome(false, 0, 0))
		if elapsed := time.Since(startedAt); elapsed != 0 {
			t.Fatalf("阶段 3 失败触发了 %v 冷却", elapsed)
		}

		// 毫秒级就返回的失败才是链路异常，这时冷却才有意义。
		engine.noteStageProbeOutcome("stage1_tcp", "1.1.1.2", classifyProbeOutcome(false, time.Millisecond, budget))
		if elapsed := time.Since(startedAt); elapsed != config.CooldownDuration {
			t.Fatalf("链路异常冷却 = %v, want %v", elapsed, config.CooldownDuration)
		}
	})
}
