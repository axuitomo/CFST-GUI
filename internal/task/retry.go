package task

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxRetryAfterWait   = 30 * time.Second
	minRateLimitBackoff = time.Second
	probeWaitQuantum    = 25 * time.Millisecond
)

func (e *Engine) retryAttemptLimit() int {
	if e.config.RetryMaxAttempts <= 0 {
		return 1
	}
	return e.config.RetryMaxAttempts + 1
}

func (e *Engine) sleepBeforeRetry(stage, ip string, attempt int) {
	if e.config.RetryBackoff <= 0 {
		return
	}
	e.sleepBeforeRetryDelay(stage, ip, attempt, e.config.RetryBackoff, "retry_backoff", "单 IP 探测失败，按重试策略等待后重试。")
}

func (e *Engine) sleepBeforeRateLimitRetry(stage, ip string, attempt int, retryAfter time.Duration) {
	e.sleepBeforeRetryDelay(stage, ip, attempt, e.rateLimitRetryDelay(retryAfter), "rate_limited", "服务端返回 429，按限流退避等待后重试。")
}

func (e *Engine) sleepBeforeRetryDelay(stage, ip string, attempt int, delay time.Duration, reason, message string) {
	if delay <= 0 {
		return
	}
	e.checkPause(stage, ip)
	e.debugEvent("stage.detail", map[string]any{
		"ip":      ip,
		"message": message,
		"reason":  reason,
		"retry": map[string]any{
			"attempt":    attempt,
			"backoff_ms": delay.Milliseconds(),
		},
		"stage": stage,
	})
	e.waitProbeDelay(stage, ip, delay)
}

func (e *Engine) rateLimitRetryDelay(retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return capRetryAfterDelay(retryAfter)
	}
	if e.config.RetryBackoff > minRateLimitBackoff {
		return e.config.RetryBackoff
	}
	return minRateLimitBackoff
}

func retryAfterDelay(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		return capRetryAfterDelay(time.Duration(seconds) * time.Second)
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		return capRetryAfterDelay(retryAt.Sub(now))
	}
	return 0
}

func capRetryAfterDelay(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	if delay > maxRetryAfterWait {
		return maxRetryAfterWait
	}
	return delay
}

func (e *Engine) noteStageProbeOutcome(stage, ip string, ok bool) {
	if e.config.CooldownFailures <= 0 || e.config.CooldownDuration <= 0 {
		return
	}

	e.cooldownMu.Lock()
	if ok {
		e.cooldownFails[stage] = 0
		e.cooldownMu.Unlock()
		return
	}

	nextCount := e.cooldownFails[stage] + 1
	if nextCount < e.config.CooldownFailures {
		e.cooldownFails[stage] = nextCount
		e.cooldownMu.Unlock()
		return
	}
	e.cooldownFails[stage] = 0
	e.cooldownMu.Unlock()

	e.debugEvent("stage.cooldown", map[string]any{
		"cooldown": map[string]any{
			"consecutive_failures": e.config.CooldownFailures,
			"duration_ms":          e.config.CooldownDuration.Milliseconds(),
		},
		"ip":      ip,
		"message": "连续失败达到阈值，当前探测阶段短暂冷却。",
		"reason":  "consecutive_failures",
		"stage":   stage,
	})
	e.waitProbeDelay(stage, ip, e.config.CooldownDuration)
}

func (e *Engine) waitProbeDelay(stage, ip string, delay time.Duration) {
	remaining := delay
	for remaining > 0 {
		e.checkPause(stage, ip)
		ctx := e.context()
		if ctx.Err() != nil {
			return
		}
		step := min(remaining, probeWaitQuantum)
		timer := time.NewTimer(step)
		select {
		case <-timer.C:
			remaining -= step
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
	e.checkPause(stage, ip)
}
