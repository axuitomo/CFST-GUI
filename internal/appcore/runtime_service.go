package appcore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

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
			Now:          s.now,
		})
	}
	return s.cleaner
}

func (s *Service) StartRuntimeCleanup(ctx context.Context) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.ensureRuntimeCleaner().Start(ctx)
}

func (s *Service) StopRuntimeCleanup() {
	s.runtimeCleanupMu.Lock()
	cleaner := s.cleaner
	s.runtimeCleanupMu.Unlock()
	cleaner.Stop()
}

func (s *Service) TriggerRuntimeCleanupAfterTask() {
	s.runtimeCleanupMu.Lock()
	cleaner := s.cleaner
	s.runtimeCleanupMu.Unlock()
	if cleaner != nil {
		cleaner.TriggerDelayed()
	}
}

func (s *Service) RuntimeStatus() CommandResult {
	if !runtimecleanup.DiagnosticsEnabled() {
		return NewCommandResult("RUNTIME_DIAGNOSTICS_DISABLED", map[string]any{"diagnostics_enabled": false}, "Runtime diagnostics are disabled.", true, nil, nil)
	}
	return NewCommandResult("RUNTIME_STATUS_READY", s.RuntimeStatusData(), "Runtime diagnostics read.", true, nil, nil)
}

func (s *Service) RuntimeStatusData() map[string]any {
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
	s.mu.RLock()
	hook := s.options.RuntimeCleanupHook
	s.mu.RUnlock()
	if hook != nil {
		hook()
	}
	if s.taskStore != nil {
		s.taskStore.TrimTerminalCache()
	}
	s.CleanupExpiredTerminalTaskFiles(s.now())
}

func (s *Service) runtimeCleanupBusy() bool {
	state := s.runtime.State()
	if strings.TrimSpace(state.CurrentTaskID) != "" || strings.TrimSpace(state.PausedTaskID) != "" {
		return true
	}
	s.mu.RLock()
	busy := s.options.RuntimeCleanupBusy
	s.mu.RUnlock()
	return busy != nil && busy()
}

func (s *Service) runtimeCleanupCounts() runtimecleanup.Counts {
	if s.taskStore == nil {
		return runtimecleanup.Counts{}
	}
	return runtimecleanup.Counts{TaskSnapshots: s.taskStore.CacheCount()}
}

func (s *Service) CleanupExpiredTerminalTaskFiles(now time.Time) {
	loaded, err := s.LoadConfig()
	snapshot := loaded.Snapshot
	if err != nil || snapshot == nil {
		s.mu.RLock()
		policy := s.options.ConfigPolicy
		s.mu.RUnlock()
		snapshot = sanitizeServiceConfig(nil, policy)
	}
	retentionDays := CompletedTaskRetentionDaysFromSnapshot(snapshot)
	if retentionDays <= 0 {
		return
	}
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)
	root := s.StorageLayout().TasksRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || strings.HasSuffix(entry.Name(), "-results.json") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var snapshot TaskSnapshot
		if err := json.Unmarshal(raw, &snapshot); err != nil || !isTerminalTaskStatus(snapshot.Status) || snapshot.RuntimeAttached || snapshot.ResumeCapable || snapshot.SessionState == "active_runtime" || snapshot.SessionState == "paused_runtime" {
			continue
		}
		terminalAt := terminalTaskSnapshotTime(snapshot)
		if terminalAt.IsZero() || !terminalAt.Before(cutoff) {
			continue
		}
		_ = os.Remove(path)
		_ = os.Remove(strings.TrimSuffix(path, ".json") + "-results.json")
	}
}

func terminalTaskSnapshotTime(snapshot TaskSnapshot) time.Time {
	for _, value := range []string{snapshot.CompletedAt, snapshot.UpdatedAt} {
		if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func CompletedTaskRetentionDaysFromSnapshot(snapshot map[string]any) int {
	maintenance := mapValue(snapshot["maintenance"])
	value := intValue(firstNonNil(maintenance["completed_task_retention_days"], maintenance["completedTaskRetentionDays"]), probecore.DefaultCompletedTaskRetentionDays)
	if value < 0 {
		return probecore.DefaultCompletedTaskRetentionDays
	}
	return value
}
