package app

import (
	"context"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/runtimecleanup"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func (a *App) ensureRuntimeCleaner() *runtimecleanup.Cleaner {
	a.runtimeCleanupMu.Lock()
	defer a.runtimeCleanupMu.Unlock()
	if a.cleaner == nil {
		a.cleaner = runtimecleanup.New(runtimecleanup.Options{
			IsBusy:       a.runtimeCleanupBusy,
			LightCleanup: a.runLightRuntimeCleanup,
			Counts:       a.runtimeCleanupCounts,
		})
	}
	return a.cleaner
}

func (a *App) startRuntimeCleanup(ctx context.Context) {
	a.ensureRuntimeCleaner().Start(ctx)
}

func (a *App) triggerRuntimeCleanupAfterTask() {
	a.runtimeCleanupMu.Lock()
	cleaner := a.cleaner
	a.runtimeCleanupMu.Unlock()
	cleaner.TriggerDelayed()
}

func (a *App) GetRuntimeStatus() DesktopCommandResult {
	if !runtimecleanup.DiagnosticsEnabled() {
		return desktopCommandResult("RUNTIME_DIAGNOSTICS_DISABLED", map[string]any{
			"diagnostics_enabled": false,
		}, "运行时诊断未启用。", true, nil, nil)
	}
	return desktopCommandResult("RUNTIME_STATUS_READY", a.ensureRuntimeCleaner().Status(), "运行时诊断已读取。", true, nil, nil)
}

func (a *App) runLightRuntimeCleanup() {
	closeUpdateIdleConnections()
	httpclient.CleanupExpiredH3FailureCache()
	_ = utils.CleanupConfiguredRuntimeLogs(logDirectoryPath())
	a.trimRuntimeTaskSnapshots()
}

func (a *App) runtimeCleanupBusy() bool {
	if a.currentProbeRuntimeTaskID() != "" {
		return true
	}
	status := a.currentSchedulerStatus()
	return schedulerRunStageActive(status.RunStage)
}

func (a *App) runtimeCleanupCounts() runtimecleanup.Counts {
	a.taskStateMu.Lock()
	taskSnapshots := len(a.taskSnapshots)
	a.taskStateMu.Unlock()
	return runtimecleanup.Counts{
		TaskSnapshots: taskSnapshots,
	}
}

func (a *App) trimRuntimeTaskSnapshots() {
	a.taskStateMu.Lock()
	defer a.taskStateMu.Unlock()
	for taskID, snapshot := range a.taskSnapshots {
		if !shouldCacheTaskSnapshotInMemory(snapshot.Status) {
			delete(a.taskSnapshots, taskID)
		}
	}
}

func schedulerRunStageActive(stage string) bool {
	switch strings.TrimSpace(stage) {
	case "probe", "dns", "github":
		return true
	default:
		return false
	}
}

func isTerminalTaskSnapshotStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "no_results":
		return true
	default:
		return false
	}
}
