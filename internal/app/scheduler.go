package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

// The desktop adapter only wakes the shared scheduler. Workflow state and execution
// remain owned by appcore.Service.
func (a *App) reloadSchedulerFromDisk() {
	loaded, err := a.core.LoadConfig()
	if err != nil {
		a.stopScheduler()
		status, _ := a.core.CurrentSchedulerStatus()
		status.Enabled = false
		status.NextRunAt = ""
		status.LastMessage = fmt.Sprintf("读取定时任务配置失败：%v", err)
		status.WorkflowStage = "load_config_failed"
		status.ConfigSource = "saved"
		status.LastSourceProfileAction = ""
		_ = a.core.SaveSchedulerStatus(status)
		return
	}
	a.reloadSchedulerFromSnapshot(loaded.Snapshot)
}

func (a *App) reloadSchedulerFromSnapshot(snapshot map[string]any) {
	a.stopScheduler()
	cfg := a.core.SchedulerConfig(snapshot)
	status, err := a.core.RefreshSchedulerStatus(snapshot)
	if err != nil || !cfg.Enabled || status.NextRunAt == "" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.schedulerMu.Lock()
	a.schedulerCancel = cancel
	a.schedulerMu.Unlock()
	go a.schedulerLoop(ctx, cfg)
}

func (a *App) stopScheduler() {
	a.schedulerMu.Lock()
	cancel := a.schedulerCancel
	a.schedulerCancel = nil
	a.schedulerMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) schedulerLoop(ctx context.Context, cfg appcore.SchedulerConfig) {
	for {
		status, err := a.core.CurrentSchedulerStatus()
		if err != nil {
			return
		}
		next := appcore.NextSchedulerRun(time.Now(), parseSchedulerTimestamp(status.LastRunAt), appcore.SchedulerTimingConfig{
			Enabled:         cfg.Enabled,
			IntervalMinutes: cfg.IntervalMinutes,
			DailyTimes:      cfg.DailyTimes,
		})
		if next.IsZero() {
			status.NextRunAt = ""
			status.LastMessage = "定时任务没有下一次运行时间。"
			_ = a.core.SaveSchedulerStatus(status)
			return
		}
		status.Enabled = cfg.Enabled
		status.NextRunAt = next.Format(time.RFC3339)
		_ = a.core.SaveSchedulerStatus(status)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
			result := a.core.RunScheduledProbe(ctx, appcore.SchedulerRunRequest{})
			if !result.OK && strings.TrimSpace(result.Message) != "" {
				status, _ := a.core.CurrentSchedulerStatus()
				status.LastMessage = result.Message
				_ = a.core.SaveSchedulerStatus(status)
			}
		}
	}
}

func parseSchedulerTimestamp(raw string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	return parsed
}

func (a *App) currentSchedulerStatus() appcore.SchedulerStatus {
	status, err := a.core.CurrentSchedulerStatus()
	if err != nil {
		return appcore.SchedulerStatus{LastMessage: err.Error()}
	}
	return status
}
