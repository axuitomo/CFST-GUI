package appcore

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type TaskProgressSnapshot struct {
	Failed    int    `json:"failed"`
	Passed    int    `json:"passed"`
	Processed int    `json:"processed"`
	Stage     string `json:"stage"`
	Total     int    `json:"total,omitempty"`
}

type ExportRecordSnapshot struct {
	FileName     string `json:"file_name"`
	Format       string `json:"format"`
	LastWriteAt  string `json:"last_write_at,omitempty"`
	SourcePath   string `json:"source_path,omitempty"`
	TargetDir    string `json:"target_dir"`
	TaskID       string `json:"task_id"`
	WrittenCount int    `json:"written_count"`
}

type TaskSnapshot struct {
	CompletedAt     string                `json:"completed_at,omitempty"`
	Archived        bool                  `json:"archived,omitempty"`
	ConfigDigest    string                `json:"config_digest,omitempty"`
	CurrentStage    string                `json:"current_stage,omitempty"`
	ExportRecord    *ExportRecordSnapshot `json:"export_record,omitempty"`
	FailureSummary  map[string]any        `json:"failure_summary,omitempty"`
	Progress        *TaskProgressSnapshot `json:"progress,omitempty"`
	ResumeCapable   bool                  `json:"resume_capable,omitempty"`
	RuntimeAttached bool                  `json:"runtime_attached,omitempty"`
	SessionState    string                `json:"session_state,omitempty"`
	StartedAt       string                `json:"started_at,omitempty"`
	Status          string                `json:"status"`
	TaskContext     map[string]any        `json:"task_context,omitempty"`
	TaskID          string                `json:"task_id"`
	UpdatedAt       string                `json:"updated_at"`

	resumeCapableSet   bool
	runtimeAttachedSet bool
}

func BuildAcceptedTaskSnapshot(taskID string, now time.Time) TaskSnapshot {
	timestamp := now.Format(time.RFC3339)
	return TaskSnapshot{
		CurrentStage:       "accepted",
		RuntimeAttached:    true,
		runtimeAttachedSet: true,
		SessionState:       "active_runtime",
		StartedAt:          timestamp,
		Status:             "preparing",
		TaskID:             strings.TrimSpace(taskID),
		UpdatedAt:          timestamp,
	}
}

func ShouldCacheTaskSnapshot(status string) bool {
	switch strings.TrimSpace(status) {
	case "running", "preparing", "cooling", "partial":
		return true
	default:
		return false
	}
}

func TaskSnapshotFromEvent(taskID, event string, payload map[string]any, now time.Time) TaskSnapshot {
	taskID = strings.TrimSpace(taskID)
	timestamp := now.Format(time.RFC3339)
	snapshot := TaskSnapshot{
		Status:    taskSnapshotStatusForEvent(event, payload),
		TaskID:    taskID,
		UpdatedAt: timestamp,
	}
	if startedAt := strings.TrimSpace(taskSnapshotString(payload["started_at"])); startedAt != "" {
		snapshot.StartedAt = startedAt
	}
	if completedAt := strings.TrimSpace(taskSnapshotString(payload["completed_at"])); completedAt != "" {
		snapshot.CompletedAt = completedAt
	}
	if stage := strings.TrimSpace(taskSnapshotString(taskSnapshotFirst(payload["stage"], payload["current_stage"]))); stage != "" {
		snapshot.CurrentStage = stage
	}
	if taskContext, ok := payload["task_context"].(map[string]any); ok && len(taskContext) > 0 {
		snapshot.TaskContext = taskContext
	}
	if failureSummary, ok := payload["failure_summary"].(map[string]any); ok && len(failureSummary) > 0 {
		snapshot.FailureSummary = failureSummary
	}
	snapshot.Progress = taskProgressSnapshotFromEvent(event, payload)
	snapshot.ExportRecord = exportRecordSnapshotFromEvent(taskID, event, payload, now)
	if event == "probe.cooling" {
		recoverable := taskSnapshotBool(payload["recoverable"], true)
		snapshot.ResumeCapable = recoverable
		snapshot.RuntimeAttached = recoverable
		snapshot.resumeCapableSet = true
		snapshot.runtimeAttachedSet = true
		if recoverable {
			snapshot.SessionState = "paused_runtime"
		} else {
			snapshot.SessionState = "idle"
		}
	}
	if event == "probe.resumed" {
		snapshot.ResumeCapable = false
		snapshot.RuntimeAttached = true
		snapshot.resumeCapableSet = true
		snapshot.runtimeAttachedSet = true
		snapshot.SessionState = "active_runtime"
	}
	if isTerminalTaskSnapshotStatus(snapshot.Status) {
		snapshot.CompletedAt = timestamp
		snapshot.ResumeCapable = false
		snapshot.RuntimeAttached = false
		snapshot.resumeCapableSet = true
		snapshot.runtimeAttachedSet = true
		snapshot.SessionState = "persisted_only"
	}
	return snapshot
}

func MergeTaskSnapshots(base, next TaskSnapshot) TaskSnapshot {
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
	if !next.resumeCapableSet {
		next.ResumeCapable = base.ResumeCapable
	}
	if !next.runtimeAttachedSet {
		next.RuntimeAttached = base.RuntimeAttached
	}
	return next
}

func taskSnapshotStatusForEvent(event string, payload map[string]any) string {
	switch event {
	case "probe.preprocessed":
		if taskSnapshotInt(payload["accepted"], 0) > 0 {
			return "preparing"
		}
		return "no_results"
	case "probe.progress", "probe.resumed", "probe.speed", "probe.partial_export":
		return "running"
	case "probe.cooling":
		return "cooling"
	case "probe.failed":
		return "failed"
	case "probe.cancelled":
		return "cancelled"
	case "probe.export_completed", "probe.export_failed":
		return "completed"
	case "upload.notification":
		return ""
	case "probe.completed":
		if taskSnapshotInt(taskSnapshotFirst(payload["result_count"], payload["passed"], payload["exported"]), 0) > 0 {
			return "completed"
		}
		return "no_results"
	default:
		return "running"
	}
}

func taskProgressSnapshotFromEvent(event string, payload map[string]any) *TaskProgressSnapshot {
	switch event {
	case "probe.preprocessed":
		return &TaskProgressSnapshot{
			Failed: taskSnapshotInt(payload["invalid"], 0), Passed: taskSnapshotInt(payload["accepted"], 0),
			Stage: "stage0_pool", Total: taskSnapshotInt(payload["total"], 0),
		}
	case "probe.progress":
		return &TaskProgressSnapshot{
			Failed: taskSnapshotInt(payload["failed"], 0), Passed: taskSnapshotInt(payload["passed"], 0),
			Processed: taskSnapshotInt(payload["processed"], 0), Stage: strings.TrimSpace(taskSnapshotString(payload["stage"])),
			Total: taskSnapshotInt(payload["total"], 0),
		}
	default:
		return nil
	}
}

func exportRecordSnapshotFromEvent(taskID, event string, payload map[string]any, now time.Time) *ExportRecordSnapshot {
	targetPath := strings.TrimSpace(taskSnapshotString(payload["target_path"]))
	sourcePath := strings.TrimSpace(taskSnapshotString(taskSnapshotFirst(payload["source_path"], payload["sourcePath"])))
	if targetPath == "" && event != "probe.completed" && event != "probe.partial_export" && event != "probe.export_completed" {
		return nil
	}
	written := taskSnapshotInt(payload["written"], 0)
	if event == "probe.completed" {
		written = taskSnapshotInt(taskSnapshotFirst(payload["exported"], payload["result_count"], payload["passed"]), written)
	}
	if written <= 0 && targetPath == "" {
		return nil
	}
	fileName, targetDir := splitTaskExportPath(targetPath)
	return &ExportRecordSnapshot{
		FileName: fileName, Format: "csv", LastWriteAt: now.Format(time.RFC3339), SourcePath: sourcePath,
		TargetDir: targetDir, TaskID: taskID, WrittenCount: written,
	}
}

func splitTaskExportPath(targetPath string) (string, string) {
	normalized := strings.ReplaceAll(targetPath, "\\", "/")
	fileName := filepath.Base(normalized)
	if fileName == "." || fileName == "/" {
		fileName = ""
	}
	targetDir := strings.TrimSuffix(normalized, "/"+fileName)
	if strings.Contains(targetPath, "\\") {
		targetDir = strings.ReplaceAll(targetDir, "/", "\\")
	}
	return fileName, targetDir
}

func isTerminalTaskSnapshotStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "cancelled", "completed", "failed", "no_results":
		return true
	default:
		return false
	}
}

func taskSnapshotFirst(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func taskSnapshotString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return strings.TrimSpace(strings.Trim(string(mustTaskSnapshotJSON(value)), `"`))
}

func mustTaskSnapshotJSON(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func taskSnapshotInt(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := strconv.Atoi(typed.String())
		if err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func taskSnapshotBool(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
}
