package appcore

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MigrationState records progress for the layered-storage migration.
type MigrationState struct {
	LayeredMigrationCompleted bool              `json:"layered_migration_completed"`
	MigrationSteps            map[string]bool   `json:"migration_steps"`
	MigrationErrors           map[string]string `json:"migration_errors,omitempty"`
	MigrationAttemptedAt      string            `json:"migration_attempted_at,omitempty"`
	LegacyCleanupScheduledAt  string            `json:"legacy_cleanup_scheduled_at,omitempty"`
}

var storageMigrationMu sync.Mutex

// RunStorageMigration performs the startup-safe part of migration. Logs are
// moved synchronously; other legacy files are copied and can be retried.
func RunStorageMigration(layout StorageLayout, now time.Time) (MigrationState, error) {
	storageMigrationMu.Lock()
	defer storageMigrationMu.Unlock()

	if now.IsZero() {
		now = time.Now()
	}
	state, err := loadMigrationState(layout)
	if err != nil {
		return state, err
	}
	if state.MigrationSteps == nil {
		state.MigrationSteps = map[string]bool{}
	}
	if state.MigrationErrors == nil {
		state.MigrationErrors = map[string]string{}
	}
	state.MigrationAttemptedAt = now.UTC().Format(time.RFC3339)
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		return state, err
	}
	if err := os.MkdirAll(layout.LogsRoot(), 0o755); err != nil {
		return state, err
	}

	for _, item := range []struct {
		step, name, target string
	}{
		{"logs_migrated", FileDebugLog, layout.DebugLogPath()},
		{"error_logs_migrated", FileErrorLog, layout.ErrorLogPath()},
	} {
		if state.MigrationSteps[item.step] {
			continue
		}
		if err := migrateFile(layout.LegacyPath(item.name), item.target, true); err != nil {
			state.MigrationErrors[item.step] = err.Error()
			continue
		}
		state.MigrationSteps[item.step] = true
		delete(state.MigrationErrors, item.step)
	}
	if err := persistMigrationState(layout, state); err != nil {
		return state, err
	}

	// The remaining scan is intentionally asynchronous. It never removes the
	// source so a failed copy remains retryable during the rollback window.
	go func() {
		_, _ = runBackgroundStorageMigration(layout, now)
	}()
	return state, nil
}

func runBackgroundStorageMigration(layout StorageLayout, now time.Time) (MigrationState, error) {
	storageMigrationMu.Lock()
	defer storageMigrationMu.Unlock()
	state, err := loadMigrationState(layout)
	if err != nil {
		return state, err
	}
	if state.MigrationSteps == nil {
		state.MigrationSteps = map[string]bool{}
	}
	if state.MigrationErrors == nil {
		state.MigrationErrors = map[string]string{}
	}
	for _, item := range []struct {
		step, name, target string
	}{
		{"config_migrated", FileConfig, layout.ConfigPath()},
		{"draft_migrated", FileDesktopDraft, layout.DraftPath()},
		{"scheduler_migrated", FileScheduler, layout.SchedulerPath()},
		{"source_profiles_migrated", FileSourceProfiles, layout.SourceProfilesPath()},
	} {
		if state.MigrationSteps[item.step] || item.target == layout.LegacyPath(item.name) {
			state.MigrationSteps[item.step] = true
			continue
		}
		if err := migrateFile(layout.LegacyPath(item.name), item.target, false); err != nil {
			state.MigrationErrors[item.step] = err.Error()
			continue
		}
		state.MigrationSteps[item.step] = true
		delete(state.MigrationErrors, item.step)
	}
	if err := migrateDirectoryContents(layout.TasksRoot(), filepath.Join(layout.Root, DirTaskData)); err != nil {
		state.MigrationErrors["tasks_migrated"] = err.Error()
	} else {
		state.MigrationSteps["tasks_migrated"] = true
	}
	if err := migrateDirectoryContents(layout.LegacyPath(DirExports), layout.ExportsRoot()); err != nil {
		state.MigrationErrors["exports_migrated"] = err.Error()
	} else {
		state.MigrationSteps["exports_migrated"] = true
	}
	if err := migrateDirectoryContents(layout.LegacyPath(DirBackups), layout.BackupsRoot()); err != nil {
		state.MigrationErrors["backups_migrated"] = err.Error()
	} else {
		state.MigrationSteps["backups_migrated"] = true
	}

	if len(state.MigrationErrors) == 0 {
		state.LayeredMigrationCompleted = true
		state.LegacyCleanupScheduledAt = now.Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)
	}
	return state, persistMigrationState(layout, state)
}

func loadMigrationState(layout StorageLayout) (MigrationState, error) {
	path := layout.LegacyPath(FileBootstrap)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return MigrationState{}, nil
	}
	if err != nil {
		return MigrationState{}, err
	}
	var state MigrationState
	if err := json.Unmarshal(raw, &state); err != nil {
		return MigrationState{}, err
	}
	return state, nil
}

func persistMigrationState(layout StorageLayout, state MigrationState) error {
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := layout.LegacyPath(FileBootstrap)
	var fields map[string]json.RawMessage
	if existing, readErr := os.ReadFile(path); readErr == nil {
		_ = json.Unmarshal(existing, &fields)
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	var updates map[string]json.RawMessage
	if err := json.Unmarshal(raw, &updates); err != nil {
		return err
	}
	for key, value := range updates {
		fields[key] = value
	}
	merged, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, merged, 0o600)
}

func migrateFile(source, target string, move bool) error {
	if source == target {
		return nil
	}
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if move {
		if err := os.Rename(source, target); err == nil {
			return nil
		}
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return closeErr
	}
	if move {
		return os.Remove(source)
	}
	return nil
}

func migrateDirectoryContents(source, target string) error {
	entries, err := os.ReadDir(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := migrateFile(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name()), false); err != nil {
			return err
		}
	}
	return nil
}
