package task

import (
	"sync"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/utils"
)

var (
	RetryMaxAttempts          int
	RetryBackoff              time.Duration
	CooldownConsecutiveFails  int
	CooldownDuration          time.Duration
	stageCooldownMu           sync.Mutex
	stageConsecutiveFailCount = map[string]int{}
)

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
	CheckProbePause(stage, ip)
	utils.DebugEvent("stage.detail", map[string]any{
		"ip":      ip,
		"message": "单 IP 探测失败，按重试策略等待后重试。",
		"reason":  "retry_backoff",
		"retry": map[string]any{
			"attempt":    attempt,
			"backoff_ms": RetryBackoff.Milliseconds(),
		},
		"stage": stage,
	})
	time.Sleep(RetryBackoff)
	CheckProbePause(stage, ip)
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
