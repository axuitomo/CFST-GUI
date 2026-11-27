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

// probeOutcome 把一次阶段探测结果分成三类。冷却只针对「本机/链路侧异常」：候选本身
// 不通的慢失败是 CFST 的常态（实测一次 13863 颗候选里 11948 颗吃满超时），把它计入
// 连续失败只会让某个 worker 白等一个冷却窗口，对远端压力没有任何缓解。
type probeOutcome int

const (
	probeSucceeded probeOutcome = iota
	probeCandidateFailed
	probeSystemicFailed
)

// systemicFailureRatio 是「瞬时失败」的判定线：耗时不到该阶段超时预算的 1/10。
// 路由不可达、端口耗尽这类本机异常在毫秒级返回，候选不通则要等满超时。
const systemicFailureRatio = 10

// classifyProbeOutcome 归纳一次探测结果。budget<=0 表示该阶段的失败一律按候选侧处理
// （阶段 3 的失败全部是速度不够或边缘拒绝，见 download.go 的调用点）。
func classifyProbeOutcome(ok bool, elapsed, budget time.Duration) probeOutcome {
	if ok {
		return probeSucceeded
	}
	if budget > 0 && elapsed < budget/systemicFailureRatio {
		return probeSystemicFailed
	}
	return probeCandidateFailed
}

func (e *Engine) noteStageProbeOutcome(stage, ip string, outcome probeOutcome) {
	if e.config.CooldownFailures <= 0 || e.config.CooldownDuration <= 0 {
		return
	}

	e.cooldownMu.Lock()
	if outcome == probeSucceeded {
		e.cooldownFails[stage] = 0
		e.cooldownMu.Unlock()
		return
	}
	if outcome != probeSystemicFailed {
		// 候选侧失败不推进计数：它既不代表链路有问题，也不该冲掉已经累计的异常计数。
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
