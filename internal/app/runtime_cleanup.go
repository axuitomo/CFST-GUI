package app

import (
	"context"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/runtimecleanup"
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
	a.trimRuntimeTaskSnapshots()
	a.trimRuntimePipelineResults()
}

func (a *App) runtimeCleanupBusy() bool {
	if a.currentProbeRuntimeTaskID() != "" || a.hasActivePipelineTask() {
		return true
	}
	status := a.currentSchedulerStatus()
	return schedulerWorkflowStageActive(status.WorkflowStage)
}

func (a *App) runtimeCleanupCounts() runtimecleanup.Counts {
	a.taskStateMu.Lock()
	taskSnapshots := len(a.taskSnapshots)
	a.taskStateMu.Unlock()
	a.pipelineMu.Lock()
	pipelineResults := len(a.pipelineResults)
	a.pipelineMu.Unlock()
	return runtimecleanup.Counts{
		PipelineResults: pipelineResults,
		TaskSnapshots:   taskSnapshots,
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

func (a *App) trimRuntimePipelineResults() {
	a.pipelineMu.Lock()
	defer a.pipelineMu.Unlock()
	if a.pipelineResults == nil {
		a.pipelineResults = map[string]PipelineRunResult{}
	}
	if a.currentPipelineID != "" || len(a.pipelineResults) <= 1 {
		return
	}
	for pipelineID := range a.pipelineResults {
		delete(a.pipelineResults, pipelineID)
		if len(a.pipelineResults) <= 1 {
			break
		}
	}
}

func schedulerWorkflowStageActive(stage string) bool {
	switch strings.TrimSpace(stage) {
	case "probe", "dns", "github", "pipeline":
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
