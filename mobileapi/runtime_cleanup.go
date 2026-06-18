package mobileapi

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/runtimecleanup"
)

func (s *Service) ensureRuntimeCleaner() *runtimecleanup.Cleaner {
	s.runtimeCleanupMu.Lock()
	defer s.runtimeCleanupMu.Unlock()
	if s.cleaner == nil {
		s.cleaner = runtimecleanup.New(runtimecleanup.Options{
			IsBusy:       s.runtimeCleanupBusy,
			LightCleanup: s.runLightRuntimeCleanup,
			Counts:       s.runtimeCleanupCounts,
		})
	}
	return s.cleaner
}

func (s *Service) startRuntimeCleanup() {
	s.ensureRuntimeCleaner().Start(context.Background())
}

func (s *Service) triggerRuntimeCleanupAfterTask() {
	s.runtimeCleanupMu.Lock()
	cleaner := s.cleaner
	s.runtimeCleanupMu.Unlock()
	cleaner.TriggerDelayed()
}

func (s *Service) RuntimeStatus() string {
	if !runtimecleanup.DiagnosticsEnabled() {
		return encodeCommand(commandResultFor("RUNTIME_DIAGNOSTICS_DISABLED", map[string]any{
			"diagnostics_enabled": false,
		}, "运行时诊断未启用。", true, nil, nil))
	}
	return encodeCommand(commandResultFor("RUNTIME_STATUS_READY", s.runtimeStatusData(), "运行时诊断已读取。", true, nil, nil))
}

func (s *Service) runtimeStatusData() map[string]any {
	raw, err := json.Marshal(s.ensureRuntimeCleaner().Status())
	if err != nil {
		return map[string]any{"diagnostics_enabled": runtimecleanup.DiagnosticsEnabled()}
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return map[string]any{"diagnostics_enabled": runtimecleanup.DiagnosticsEnabled()}
	}
	return data
}

func (s *Service) runLightRuntimeCleanup() {
	httpclient.CleanupExpiredH3FailureCache()
	s.trimRuntimeTaskSnapshots()
	s.cleanupExpiredTerminalTaskFiles(time.Now())
	s.trimRuntimePipelineResults()
}

func (s *Service) runtimeCleanupBusy() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return strings.TrimSpace(s.currentTaskID) != "" || strings.TrimSpace(s.pausedTaskID) != "" || strings.TrimSpace(s.currentPipelineID) != ""
}

func (s *Service) runtimeCleanupCounts() runtimecleanup.Counts {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return runtimecleanup.Counts{
		PipelineResults: len(s.pipelineResults),
		TaskSnapshots:   len(s.taskSnapshots),
	}
}

func (s *Service) trimRuntimeTaskSnapshots() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	for taskID, snapshot := range s.taskSnapshots {
		if !shouldCacheTaskSnapshotInMemory(snapshot.Status) {
			delete(s.taskSnapshots, taskID)
		}
	}
}

func (s *Service) trimRuntimePipelineResults() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.pipelineResults == nil {
		s.pipelineResults = map[string]appcore.PipelineRunResult{}
	}
	if s.currentPipelineID != "" || len(s.pipelineResults) <= 1 {
		return
	}
	for pipelineID := range s.pipelineResults {
		delete(s.pipelineResults, pipelineID)
		if len(s.pipelineResults) <= 1 {
			break
		}
	}
}

func mobileTerminalTaskSnapshotStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "no_results":
		return true
	default:
		return false
	}
}

func (s *Service) cleanupExpiredTerminalTaskFiles(now time.Time) {
	retentionDays := completedTaskRetentionDaysFromSnapshot(s.loadConfigSnapshotForRetention())
	if retentionDays <= 0 {
		return
	}
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)
	entries, err := os.ReadDir(s.tasksRootPath())
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" || strings.HasSuffix(name, "-results.json") {
			continue
		}
		path := filepath.Join(s.tasksRootPath(), name)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var snapshot taskSnapshot
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			continue
		}
		if !mobileTerminalTaskSnapshotStatus(snapshot.Status) || snapshot.RuntimeAttached || snapshot.ResumeCapable || strings.TrimSpace(snapshot.SessionState) == "active_runtime" || strings.TrimSpace(snapshot.SessionState) == "paused_runtime" {
			continue
		}
		terminalAt := mobileTerminalTaskSnapshotTime(snapshot)
		if terminalAt.IsZero() || !terminalAt.Before(cutoff) {
			continue
		}
		_ = os.Remove(path)
		_ = os.Remove(strings.TrimSuffix(path, ".json") + "-results.json")
	}
}

func (s *Service) loadConfigSnapshotForRetention() map[string]any {
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		return defaultConfigSnapshot()
	}
	return snapshot
}

func completedTaskRetentionDaysFromSnapshot(snapshot map[string]any) int {
	maintenance := mapValue(snapshot["maintenance"])
	value := intValue(firstNonNil(maintenance["completed_task_retention_days"], maintenance["completedTaskRetentionDays"]), probecore.DefaultCompletedTaskRetentionDays)
	if value < 0 {
		return probecore.DefaultCompletedTaskRetentionDays
	}
	return value
}

func mobileTerminalTaskSnapshotTime(snapshot taskSnapshot) time.Time {
	if parsed := parseMobileTaskSnapshotTime(snapshot.CompletedAt); !parsed.IsZero() {
		return parsed
	}
	return parseMobileTaskSnapshotTime(snapshot.UpdatedAt)
}

func parseMobileTaskSnapshotTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed
}
