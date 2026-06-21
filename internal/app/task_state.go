package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

type taskProgressSnapshot struct {
	Failed    int    `json:"failed"`
	Passed    int    `json:"passed"`
	Processed int    `json:"processed"`
	Stage     string `json:"stage"`
	Total     int    `json:"total,omitempty"`
}

type exportRecordSnapshot struct {
	FileName     string `json:"file_name"`
	Format       string `json:"format"`
	LastWriteAt  string `json:"last_write_at,omitempty"`
	SourcePath   string `json:"source_path,omitempty"`
	TargetDir    string `json:"target_dir"`
	TaskID       string `json:"task_id"`
	WrittenCount int    `json:"written_count"`
}

type taskSnapshot struct {
	CompletedAt     string                `json:"completed_at,omitempty"`
	ConfigDigest    string                `json:"config_digest,omitempty"`
	CurrentStage    string                `json:"current_stage,omitempty"`
	ExportRecord    *exportRecordSnapshot `json:"export_record,omitempty"`
	FailureSummary  map[string]any        `json:"failure_summary,omitempty"`
	Progress        *taskProgressSnapshot `json:"progress,omitempty"`
	ResumeCapable   bool                  `json:"resume_capable,omitempty"`
	RuntimeAttached bool                  `json:"runtime_attached,omitempty"`
	SessionState    string                `json:"session_state,omitempty"`
	StartedAt       string                `json:"started_at,omitempty"`
	Status          string                `json:"status"`
	TaskContext     map[string]any        `json:"task_context,omitempty"`
	TaskID          string                `json:"task_id"`
	UpdatedAt       string                `json:"updated_at"`
}

const hashedTaskSnapshotStoragePrefix = "task-hash-"

func buildAcceptedTaskSnapshot(taskID string) taskSnapshot {
	now := time.Now().Format(time.RFC3339)
	return taskSnapshot{
		CurrentStage:    "accepted",
		RuntimeAttached: true,
		SessionState:    "active_runtime",
		StartedAt:       now,
		Status:          "preparing",
		TaskID:          strings.TrimSpace(taskID),
		UpdatedAt:       now,
	}
}

func shouldCacheTaskSnapshotInMemory(status string) bool {
	switch strings.TrimSpace(status) {
	case "running", "preparing", "cooling", "partial":
		return true
	default:
		return false
	}
}

func taskSnapshotFromEvent(taskID string, event string, payload map[string]any) taskSnapshot {
	taskID = strings.TrimSpace(taskID)
	now := time.Now().Format(time.RFC3339)
	snapshot := taskSnapshot{
		Status:    snapshotStatusForEvent(event, payload),
		TaskID:    taskID,
		UpdatedAt: now,
	}
	if startedAt := strings.TrimSpace(stringValue(payload["started_at"], "")); startedAt != "" {
		snapshot.StartedAt = startedAt
	}
	if completedAt := strings.TrimSpace(stringValue(payload["completed_at"], "")); completedAt != "" {
		snapshot.CompletedAt = completedAt
	}
	if stage := strings.TrimSpace(stringValue(firstNonNil(payload["stage"], payload["current_stage"]), "")); stage != "" {
		snapshot.CurrentStage = stage
	}
	if taskContext := mapValue(payload["task_context"]); len(taskContext) > 0 {
		snapshot.TaskContext = taskContext
	}
	if failureSummary := mapValue(payload["failure_summary"]); len(failureSummary) > 0 {
		snapshot.FailureSummary = failureSummary
	}
	if progress := progressSnapshotFromEvent(event, payload); progress != nil {
		snapshot.Progress = progress
	}
	if exportRecord := exportRecordFromEvent(taskID, event, payload); exportRecord != nil {
		snapshot.ExportRecord = exportRecord
	}
	if event == "probe.cooling" {
		recoverable := boolValue(payload["recoverable"], true)
		snapshot.ResumeCapable = recoverable
		snapshot.RuntimeAttached = recoverable
		if recoverable {
			snapshot.SessionState = "paused_runtime"
		} else {
			snapshot.SessionState = "idle"
		}
	}
	if event == "probe.resumed" {
		snapshot.ResumeCapable = false
		snapshot.RuntimeAttached = true
		snapshot.SessionState = "active_runtime"
	}
	if snapshot.Status == "completed" || snapshot.Status == "failed" || snapshot.Status == "no_results" {
		snapshot.CompletedAt = now
	}
	return snapshot
}

func mergeTaskSnapshot(base taskSnapshot, next taskSnapshot) taskSnapshot {
	if strings.TrimSpace(next.TaskID) == "" {
		next.TaskID = base.TaskID
	}
	if strings.TrimSpace(next.StartedAt) == "" {
		next.StartedAt = base.StartedAt
	}
	if strings.TrimSpace(next.CompletedAt) == "" {
		next.CompletedAt = base.CompletedAt
	}
	if strings.TrimSpace(next.CurrentStage) == "" {
		next.CurrentStage = base.CurrentStage
	}
	if strings.TrimSpace(next.Status) == "" {
		next.Status = base.Status
	}
	if next.Progress == nil {
		next.Progress = base.Progress
	}
	if next.ExportRecord == nil {
		next.ExportRecord = base.ExportRecord
	}
	if len(next.TaskContext) == 0 {
		next.TaskContext = base.TaskContext
	}
	if len(next.FailureSummary) == 0 {
		next.FailureSummary = base.FailureSummary
	}
	if strings.TrimSpace(next.ConfigDigest) == "" {
		next.ConfigDigest = base.ConfigDigest
	}
	if strings.TrimSpace(next.SessionState) == "" {
		next.SessionState = base.SessionState
	}
	if !next.ResumeCapable {
		next.ResumeCapable = base.ResumeCapable
	}
	if !next.RuntimeAttached {
		next.RuntimeAttached = base.RuntimeAttached
	}
	return next
}

func snapshotStatusForEvent(event string, payload map[string]any) string {
	switch event {
	case "probe.preprocessed":
		if intValue(payload["accepted"], 0) > 0 {
			return "preparing"
		}
		return "no_results"
	case "probe.progress", "probe.resumed", "probe.speed":
		return "running"
	case "probe.partial_export":
		return "running"
	case "probe.cooling":
		return "cooling"
	case "probe.failed":
		return "failed"
	case "probe.completed":
		if intValue(firstNonNil(payload["result_count"], payload["passed"], payload["exported"]), 0) > 0 {
			return "completed"
		}
		return "no_results"
	default:
		return "running"
	}
}

func progressSnapshotFromEvent(event string, payload map[string]any) *taskProgressSnapshot {
	switch event {
	case "probe.preprocessed":
		total := intValue(payload["total"], 0)
		return &taskProgressSnapshot{
			Failed:    intValue(payload["invalid"], 0),
			Passed:    intValue(payload["accepted"], 0),
			Processed: 0,
			Stage:     "stage0_pool",
			Total:     total,
		}
	case "probe.progress":
		return &taskProgressSnapshot{
			Failed:    intValue(payload["failed"], 0),
			Passed:    intValue(payload["passed"], 0),
			Processed: intValue(payload["processed"], 0),
			Stage:     strings.TrimSpace(stringValue(payload["stage"], "")),
			Total:     intValue(payload["total"], 0),
		}
	default:
		return nil
	}
}

func exportRecordFromEvent(taskID string, event string, payload map[string]any) *exportRecordSnapshot {
	targetPath := strings.TrimSpace(stringValue(payload["target_path"], ""))
	sourcePath := strings.TrimSpace(stringValue(firstNonNil(payload["source_path"], payload["sourcePath"]), ""))
	if targetPath == "" && event != "probe.completed" && event != "probe.partial_export" {
		return nil
	}
	written := intValue(payload["written"], 0)
	if event == "probe.completed" {
		written = intValue(firstNonNil(payload["exported"], payload["result_count"], payload["passed"]), written)
	}
	if written <= 0 && targetPath == "" {
		return nil
	}
	base := filepath.Base(targetPath)
	targetDir := strings.TrimSuffix(targetPath, "/"+base)
	if strings.Contains(targetPath, "\\") {
		targetDir = strings.TrimSuffix(targetPath, "\\"+base)
	}
	return &exportRecordSnapshot{
		FileName:     base,
		Format:       "csv",
		LastWriteAt:  time.Now().Format(time.RFC3339),
		SourcePath:   sourcePath,
		TargetDir:    targetDir,
		TaskID:       taskID,
		WrittenCount: written,
	}
}

func taskSnapshotsRootPath() string {
	return filepath.Join(storageRoot(), "tasks")
}

func taskSnapshotPath(taskID string) string {
	return filepath.Join(taskSnapshotsRootPath(), taskSnapshotStorageID(taskID)+".json")
}

func taskResultsPath(taskID string) string {
	return filepath.Join(taskSnapshotsRootPath(), taskSnapshotStorageID(taskID)+"-results.json")
}

func taskSnapshotStorageID(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if isSafeTaskSnapshotStorageID(taskID) {
		return taskID
	}
	sum := sha256.Sum256([]byte(taskID))
	return hashedTaskSnapshotStoragePrefix + hex.EncodeToString(sum[:])
}

func isSafeTaskSnapshotStorageID(taskID string) bool {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || taskID == "." || taskID == ".." || strings.HasPrefix(taskID, ".") || strings.HasPrefix(taskID, hashedTaskSnapshotStoragePrefix) {
		return false
	}
	for index := 0; index < len(taskID); index++ {
		char := taskID[index]
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '-' || char == '_' || char == '.':
		default:
			return false
		}
	}
	return true
}

func (a *App) writeTaskSnapshot(snapshot taskSnapshot) error {
	taskID := strings.TrimSpace(snapshot.TaskID)
	if taskID == "" {
		return nil
	}
	snapshot.TaskID = taskID
	snapshot.UpdatedAt = time.Now().Format(time.RFC3339)

	a.probeControlMu.Lock()
	currentTaskID := a.currentTaskID
	pauseRequested := a.pauseRequested
	pausedTaskID := a.pausedTaskID
	a.probeControlMu.Unlock()

	switch snapshot.Status {
	case "completed", "failed", "no_results":
		snapshot.RuntimeAttached = false
		snapshot.ResumeCapable = false
		snapshot.SessionState = "persisted_only"
	default:
		snapshot.RuntimeAttached = currentTaskID == taskID
		snapshot.ResumeCapable = pauseRequested && pausedTaskID == taskID
		if snapshot.ResumeCapable {
			snapshot.SessionState = "paused_runtime"
		} else if snapshot.RuntimeAttached {
			snapshot.SessionState = "active_runtime"
		} else if strings.TrimSpace(snapshot.SessionState) == "" {
			snapshot.SessionState = "persisted_only"
		}
	}

	if err := os.MkdirAll(taskSnapshotsRootPath(), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := appcore.WriteFileAtomic(taskSnapshotPath(taskID), raw, 0o600); err != nil {
		return err
	}

	a.taskStateMu.Lock()
	if a.taskSnapshots == nil {
		a.taskSnapshots = map[string]taskSnapshot{}
	}
	if shouldCacheTaskSnapshotInMemory(snapshot.Status) {
		a.taskSnapshots[taskID] = snapshot
	} else {
		delete(a.taskSnapshots, taskID)
	}
	a.taskStateMu.Unlock()
	if isTerminalTaskSnapshotStatus(snapshot.Status) {
		a.triggerRuntimeCleanupAfterTask()
	}
	return nil
}

func (a *App) writeTaskResults(taskID string, rows []ProbeResultRow) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	raw, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return appcore.WriteFileAtomic(taskResultsPath(taskID), raw, 0o600)
}

func (a *App) loadTaskSnapshot(taskID string) (taskSnapshot, bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return taskSnapshot{}, false, nil
	}

	a.taskStateMu.Lock()
	snapshot, ok := a.taskSnapshots[taskID]
	a.taskStateMu.Unlock()
	if !ok {
		raw, err := os.ReadFile(taskSnapshotPath(taskID))
		if err != nil {
			if os.IsNotExist(err) {
				return taskSnapshot{}, false, nil
			}
			return taskSnapshot{}, false, err
		}
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			return taskSnapshot{}, false, err
		}
	}

	changed := false
	a.probeControlMu.Lock()
	runtimeAttached := a.currentTaskID == taskID
	resumeCapable := runtimeAttached && a.pauseRequested && a.pausedTaskID == taskID
	a.probeControlMu.Unlock()

	if snapshot.Status == "running" || snapshot.Status == "preparing" || snapshot.Status == "cooling" || snapshot.Status == "partial" {
		if runtimeAttached {
			snapshot.RuntimeAttached = true
			snapshot.ResumeCapable = resumeCapable
			if resumeCapable {
				snapshot.SessionState = "paused_runtime"
			} else {
				snapshot.SessionState = "active_runtime"
			}
		} else {
			snapshot.ResumeCapable = false
			snapshot.RuntimeAttached = false
			snapshot.SessionState = "persisted_only"
			snapshot.Status = "failed"
			if strings.TrimSpace(snapshot.CurrentStage) == "" {
				snapshot.CurrentStage = "recovery_required"
			}
			if snapshot.FailureSummary == nil {
				snapshot.FailureSummary = map[string]any{}
			}
			if _, exists := snapshot.FailureSummary["recovery_status"]; !exists {
				snapshot.FailureSummary["recovery_status"] = "runtime_detached"
			}
		}
		changed = true
	} else if snapshot.Status == "completed" || snapshot.Status == "failed" || snapshot.Status == "no_results" {
		if snapshot.RuntimeAttached || snapshot.ResumeCapable || strings.TrimSpace(snapshot.SessionState) != "persisted_only" {
			snapshot.RuntimeAttached = false
			snapshot.ResumeCapable = false
			snapshot.SessionState = "persisted_only"
			changed = true
		}
	}

	a.taskStateMu.Lock()
	if a.taskSnapshots == nil {
		a.taskSnapshots = map[string]taskSnapshot{}
	}
	if shouldCacheTaskSnapshotInMemory(snapshot.Status) {
		a.taskSnapshots[taskID] = snapshot
	} else {
		delete(a.taskSnapshots, taskID)
	}
	a.taskStateMu.Unlock()

	if changed {
		_ = a.writeTaskSnapshot(snapshot)
	}
	return snapshot, true, nil
}

func (a *App) loadTaskResults(taskID string) ([]ProbeResultRow, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(taskResultsPath(taskID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []ProbeResultRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (a *App) recordTaskSnapshotEvent(taskID, event string, payload map[string]any) {
	current, _, err := a.loadTaskSnapshot(taskID)
	if err != nil {
		_ = utils.AppendErrorLog(errorLogFilePath(), "desktop.snapshot.persist_failed", map[string]any{
			"message":      err.Error(),
			"source_event": event,
			"task_id":      taskID,
		})
		return
	}
	next := mergeTaskSnapshot(current, taskSnapshotFromEvent(taskID, event, payload))
	if err := a.writeTaskSnapshot(next); err != nil {
		_ = utils.AppendErrorLog(errorLogFilePath(), "desktop.snapshot.persist_failed", map[string]any{
			"message":      err.Error(),
			"source_event": event,
			"task_id":      taskID,
		})
		return
	}
	if event == "probe.failed" {
		a.recordTaskFailureNotification(taskID, payload)
	}
}

func (a *App) LoadTaskSnapshot(payload map[string]any) DesktopCommandResult {
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	if taskID == "" {
		a.probeControlMu.Lock()
		if strings.TrimSpace(a.currentTaskID) != "" {
			taskID = strings.TrimSpace(a.currentTaskID)
		} else if strings.TrimSpace(a.pausedTaskID) != "" {
			taskID = strings.TrimSpace(a.pausedTaskID)
		}
		a.probeControlMu.Unlock()
	}

	snapshot, ok, err := a.loadTaskSnapshot(taskID)
	if err != nil {
		return desktopCommandResult("TASK_SNAPSHOT_LOAD_FAILED", nil, err.Error(), false, &taskID, nil)
	}
	if !ok {
		return desktopCommandResult("TASK_NOT_FOUND", nil, "任务不存在。", false, &taskID, nil)
	}

	raw, err := json.Marshal(snapshot)
	if err != nil {
		return desktopCommandResult("TASK_SNAPSHOT_ENCODE_FAILED", nil, err.Error(), false, &taskID, nil)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return desktopCommandResult("TASK_SNAPSHOT_ENCODE_FAILED", nil, err.Error(), false, &taskID, nil)
	}
	return desktopCommandResult("TASK_SNAPSHOT", data, "任务快照已读取。", true, &taskID, nil)
}
