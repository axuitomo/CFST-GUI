package task

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/utils"
)

const (
	maxRetryAfterWait   = 30 * time.Second
	minRateLimitBackoff = time.Second
)

var (
	RetryMaxAttempts          int
	RetryBackoff              time.Duration
	CooldownConsecutiveFails  int
	CooldownDuration          time.Duration
	stageCooldownMu           sync.Mutex
	stageConsecutiveFailCount = map[string]int{}
)

// ResetStageCooldownCounters clears per-stage failure counters before a new probe task starts.
func ResetStageCooldownCounters() {
	stageCooldownMu.Lock()
	defer stageCooldownMu.Unlock()
	stageConsecutiveFailCount = map[string]int{}
}

func retryAttemptLimit() int {
	if RetryMaxAttempts <= 0 {
		return 1
	}
	return RetryMaxAttempts + 1
}

func sleepBeforeRetry(stage, ip string, attempt int) {
	if RetryBackoff <= 0 {
		return
	}
	sleepBeforeRetryDelay(stage, ip, attempt, RetryBackoff, "retry_backoff", "单 IP 探测失败，按重试策略等待后重试。")
}

func sleepBeforeRateLimitRetry(stage, ip string, attempt int, retryAfter time.Duration) {
	sleepBeforeRetryDelay(stage, ip, attempt, rateLimitRetryDelay(retryAfter), "rate_limited", "服务端返回 429，按限流退避等待后重试。")
}

func sleepBeforeRetryDelay(stage, ip string, attempt int, delay time.Duration, reason, message string) {
	if delay <= 0 {
		return
	}
	CheckProbePause(stage, ip)
	utils.DebugEvent("stage.detail", map[string]any{
		"ip":      ip,
		"message": message,
		"reason":  reason,
		"retry": map[string]any{
			"attempt":    attempt,
			"backoff_ms": delay.Milliseconds(),
		},
		"stage": stage,
	})
	time.Sleep(delay)
	CheckProbePause(stage, ip)
}

func rateLimitRetryDelay(retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return capRetryAfterDelay(retryAfter)
	}
	if RetryBackoff > minRateLimitBackoff {
		return RetryBackoff
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

func noteStageProbeOutcome(stage, ip string, ok bool) {
	if CooldownConsecutiveFails <= 0 || CooldownDuration <= 0 {
		return
	}

	stageCooldownMu.Lock()
	if ok {
		stageConsecutiveFailCount[stage] = 0
		stageCooldownMu.Unlock()
		return
	}

	nextCount := stageConsecutiveFailCount[stage] + 1
	if nextCount < CooldownConsecutiveFails {
		stageConsecutiveFailCount[stage] = nextCount
		stageCooldownMu.Unlock()
		return
	}
	stageConsecutiveFailCount[stage] = 0
	stageCooldownMu.Unlock()

	utils.DebugEvent("stage.cooldown", map[string]any{
		"cooldown": map[string]any{
			"consecutive_failures": CooldownConsecutiveFails,
			"duration_ms":          CooldownDuration.Milliseconds(),
		},
		"ip":      ip,
		"message": "连续失败达到阈值，当前探测阶段短暂冷却。",
		"reason":  "consecutive_failures",
		"stage":   stage,
	})
	CheckProbePause(stage, ip)
	time.Sleep(CooldownDuration)
	CheckProbePause(stage, ip)
}
