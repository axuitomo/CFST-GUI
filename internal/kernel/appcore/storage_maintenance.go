package appcore

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CleanupPolicy struct {
	TaskArchiveRetentionDays int `json:"task_archive_retention_days"`
	DiagnosticRetentionDays  int `json:"diagnostic_retention_days"`
}

type CleanupResult struct {
	ArchivedTasks      int      `json:"archived_tasks"`
	DeletedTasks       int      `json:"deleted_tasks"`
	DeletedDiagnostics int      `json:"deleted_diagnostics"`
	FreedBytes         int64    `json:"freed_bytes"`
	Errors             []string `json:"errors,omitempty"`
	CompletedAt        string   `json:"completed_at"`
}

type storageMaintenance struct {
	mu       sync.RWMutex
	last     CleanupResult
	health   map[string]any
	healthAt time.Time
}

func (s *Service) maintenance() *storageMaintenance {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.maintenanceState == nil {
		s.maintenanceState = &storageMaintenance{}
	}
	return s.maintenanceState
}

func (s *Service) runStorageCleanup(policy CleanupPolicy) CleanupResult {
	layout := s.StorageLayout()
	result := CleanupResult{CompletedAt: s.now().Format(time.RFC3339)}
	if policy.TaskArchiveRetentionDays > 0 {
		before := s.now().AddDate(0, 0, -policy.TaskArchiveRetentionDays)
		store := NewFileHotStore(layout.TasksRoot())
		deleted, err := store.DeleteArchivedBefore(before, 0)
		result.DeletedTasks += deleted
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
		for _, entry := range []string{layout.TaskDataPath(".keep")} {
			_ = entry
		}
	}
	if policy.DiagnosticRetentionDays > 0 {
		cutoff := s.now().AddDate(0, 0, -policy.DiagnosticRetentionDays)
		_ = filepath.WalkDir(layout.ColdDiagnosticsDir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			info, statErr := entry.Info()
			if statErr == nil && info.ModTime().Before(cutoff) {
				if removeErr := os.Remove(path); removeErr != nil {
					result.Errors = append(result.Errors, removeErr.Error())
				} else {
					result.DeletedDiagnostics++
					result.FreedBytes += info.Size()
				}
			}
			return nil
		})
	}
	s.maintenance().mu.Lock()
	s.maintenance().last = result
	s.maintenance().mu.Unlock()
	return result
}

func (s *Service) storageHealth(payload map[string]any) map[string]any {
	state := s.maintenance()
	state.mu.RLock()
	if !boolValue(payload["refresh"], false) && !state.healthAt.IsZero() && time.Since(state.healthAt) < 5*time.Minute {
		cached := state.health
		state.mu.RUnlock()
		return cached
	}
	state.mu.RUnlock()
	layout := s.StorageLayout()
	tiers := []map[string]any{}
	total := int64(0)
	for _, tier := range []struct{ name, path string }{{"hot", layout.Root}, {"warm", layout.WarmConfigDir}, {"cold", layout.ExportsRoot()}} {
		used := int64(0)
		if entries, err := os.ReadDir(tier.path); err == nil {
			for _, entry := range entries {
				if info, infoErr := entry.Info(); infoErr == nil && !entry.IsDir() {
					used += info.Size()
				}
			}
		}
		tiers = append(tiers, map[string]any{"tier": tier.name, "path": tier.path, "used_bytes": used})
		total += used
	}
	result := map[string]any{"root": layout.Root, "total_used_bytes": total, "estimated_reclaimable_bytes": int64(0), "cached": false, "cached_at": s.now().Format(time.RFC3339), "tiers": tiers}
	state.mu.Lock()
	state.health, state.healthAt = result, s.now()
	state.mu.Unlock()
	return result
}

func (s *Service) storageCleanupStatus() CleanupResult {
	state := s.maintenance()
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.last
}

func (s *Service) forceCleanLegacy() map[string]any {
	layout := s.StorageLayout()
	removed := []string{}
	for _, name := range []string{FileConfig, FileDesktopDraft, FileScheduler, FileSourceProfiles, FileDebugLog, FileErrorLog} {
		path := layout.LegacyPath(name)
		if path == layout.ConfigPath() || path == layout.DraftPath() || path == layout.SchedulerPath() || path == layout.SourceProfilesPath() {
			continue
		}
		if err := os.Remove(path); err == nil {
			removed = append(removed, name)
		}
	}
	return map[string]any{"removed": removed}
}
