package host

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore"
)

// The host adapter only wakes the shared scheduler. Workflow state and execution
// remain owned by appcore.Service.
func (h *Host) ReloadSchedulerFromDisk() {
	loaded, err := h.Core.LoadConfig()
	if err != nil {
		h.StopScheduler()
		status, _ := h.Core.CurrentSchedulerStatus()
		status.Enabled = false
		status.NextRunAt = ""
		status.LastMessage = fmt.Sprintf("读取定时任务配置失败：%v", err)
		status.WorkflowStage = "load_config_failed"
		status.ConfigSource = "saved"
		status.LastSourceProfileAction = ""
		_ = h.Core.SaveSchedulerStatus(status)
		return
	}
	h.ReloadSchedulerFromSnapshot(loaded.Snapshot)
}

func (h *Host) ReloadSchedulerFromSnapshot(snapshot map[string]any) {
	h.StopScheduler()
	cfg := h.Core.SchedulerConfig(snapshot)
	status, err := h.Core.RefreshSchedulerStatus(snapshot)
	if err != nil || !cfg.Enabled || status.NextRunAt == "" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.schedulerMu.Lock()
	h.schedulerCancel = cancel
	h.schedulerMu.Unlock()
	go h.schedulerLoop(ctx, cfg)
}

func (h *Host) StopScheduler() {
	h.schedulerMu.Lock()
	cancel := h.schedulerCancel
	h.schedulerCancel = nil
	h.schedulerMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (h *Host) schedulerLoop(ctx context.Context, cfg appcore.SchedulerConfig) {
	for {
		status, err := h.Core.CurrentSchedulerStatus()
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
			_ = h.Core.SaveSchedulerStatus(status)
			return
		}
		status.Enabled = cfg.Enabled
		status.NextRunAt = next.Format(time.RFC3339)
		_ = h.Core.SaveSchedulerStatus(status)
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
			result := h.Core.RunScheduledProbe(ctx, appcore.SchedulerRunRequest{})
			if !result.OK && strings.TrimSpace(result.Message) != "" {
				status, _ := h.Core.CurrentSchedulerStatus()
				status.LastMessage = result.Message
				_ = h.Core.SaveSchedulerStatus(status)
			}
		}
	}
}

func parseSchedulerTimestamp(raw string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	return parsed
}

func (h *Host) CurrentSchedulerStatus() appcore.SchedulerStatus {
	status, err := h.Core.CurrentSchedulerStatus()
	if err != nil {
		return appcore.SchedulerStatus{LastMessage: err.Error()}
	}
	return status
}
