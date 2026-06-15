package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MainHeartbeatStateRunning  = "running"
	MainHeartbeatStateShutdown = "shutdown"
)

type MainHeartbeat struct {
	LastSeenAt string `json:"last_seen_at"`
	LogDir     string `json:"log_dir"`
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
	State      string `json:"state"`
}

func NewMainHeartbeat(pid int, startedAt time.Time, lastSeenAt time.Time, state string, logDir string) MainHeartbeat {
	state = strings.TrimSpace(state)
	if state == "" {
		state = MainHeartbeatStateRunning
	}
	return MainHeartbeat{
		LastSeenAt: lastSeenAt.Format(time.RFC3339Nano),
		LogDir:     strings.TrimSpace(logDir),
		PID:        pid,
		StartedAt:  startedAt.Format(time.RFC3339Nano),
		State:      state,
	}
}

func WriteMainHeartbeat(path string, heartbeat MainHeartbeat) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("heartbeat path is required")
	}
	raw, err := json.MarshalIndent(heartbeat, "", "  ")
	if err != nil {
		return err
	}
	return writeJSONFileAtomicSynced(path, raw, 0o600)
}

func ReadMainHeartbeat(path string) (MainHeartbeat, error) {
	raw, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return MainHeartbeat{}, err
	}
	var heartbeat MainHeartbeat
	if err := json.Unmarshal(raw, &heartbeat); err != nil {
		return MainHeartbeat{}, err
	}
	return heartbeat, nil
}

func MainHeartbeatLastSeen(heartbeat MainHeartbeat) (time.Time, error) {
	raw := strings.TrimSpace(heartbeat.LastSeenAt)
	if raw == "" {
		return time.Time{}, fmt.Errorf("heartbeat last_seen_at is empty")
	}
	return time.Parse(time.RFC3339Nano, raw)
}

func MainHeartbeatStale(heartbeat MainHeartbeat, now time.Time, staleAfter time.Duration) bool {
	if staleAfter <= 0 {
		return false
	}
	lastSeen, err := MainHeartbeatLastSeen(heartbeat)
	if err != nil {
		return true
	}
	return now.Sub(lastSeen) > staleAfter
}

func writeJSONFileAtomicSynced(path string, raw []byte, perm os.FileMode) (retErr error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	defer func() {
		if file != nil {
			if closeErr := file.Close(); retErr == nil && closeErr != nil {
				retErr = closeErr
			}
		}
	}()
	if err := file.Chmod(perm); err != nil {
		return err
	}
	if _, err := file.Write(raw); err != nil {
		return err
	}
	if err := syncRuntimeLogFile(file); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		file = nil
		return err
	}
	file = nil
	if err := commitSyncedJSONFile(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	bestEffortSyncParentDir(path)
	return nil
}

func bestEffortSyncParentDir(path string) {
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return
	}
	defer dir.Close()
	_ = syncRuntimeLogFile(dir)
}
