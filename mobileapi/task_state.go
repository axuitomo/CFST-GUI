package mobileapi

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func taskSnapshotFromEvent(taskID string, event string, payload map[string]any) taskSnapshot {
	taskID = strings.TrimSpace(taskID)
	now := nowRFC3339()
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
	progress := progressSnapshotFromEvent(event, payload)
	if progress != nil {
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
		return "partial"
	case "probe.cooling":
		return "cooling"
	case "probe.failed":
		return "failed"
	case "probe.export_completed", "probe.export_failed":
		return "completed"
	case "upload.notification":
		return ""
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
	if targetPath == "" && event != "probe.completed" && event != "probe.partial_export" && event != "probe.export_completed" {
		return nil
	}
	written := intValue(payload["written"], 0)
	if event == "probe.completed" {
		written = intValue(firstNonNil(payload["exported"], payload["result_count"], payload["passed"]), written)
	}
	if written <= 0 && targetPath == "" {
		return nil
	}
	return &exportRecordSnapshot{
		FileName:     filepath.Base(targetPath),
		Format:       "csv",
		LastWriteAt:  nowRFC3339(),
		SourcePath:   sourcePath,
		TargetDir:    strings.TrimSuffix(targetPath, "/"+filepath.Base(targetPath)),
		TaskID:       taskID,
		WrittenCount: written,
	}
}

func buildCompletedTaskSnapshot(taskID string, result probeRunResult) taskSnapshot {
	rows := make([]probeResultRow, 0, len(result.Results))
	for _, row := range result.Results {
		rows = append(rows, probeRowToResultRow(row))
	}
	payload := map[string]any{
		"completed_at":    nowRFC3339(),
		"current_stage":   "completed",
		"exported":        len(rows),
		"failure_summary": map[string]any{},
		"passed":          result.Summary.Passed,
		"result_count":    len(rows),
		"started_at":      result.StartedAt,
		"target_path":     result.OutputFile,
		"task_context":    taskContextToMap(result.TaskContext),
	}
	snapshot := taskSnapshotFromEvent(taskID, "probe.completed", payload)
	snapshot.ResumeCapable = false
	snapshot.RuntimeAttached = false
	snapshot.SessionState = "persisted_only"
	return snapshot
}

func taskContextToMap(value probeTaskContext) map[string]any {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	return result
}

func probeRowToResultRow(row probecore.ProbeRow) probeResultRow {
	colo := strings.TrimSpace(row.Colo)
	if colo == "" || strings.EqualFold(colo, "N/A") {
		colo = ""
	}
	stageStatus := "completed"
	exportStatus := "exported"
	delay := row.DelayMS
	trace := row.TraceDelayMS
	download := row.DownloadSpeedMB
	maxDownload := row.MaxDownloadSpeedMB
	sourcePort := row.SourcePort
	testPort := row.TestPort
	result := probeResultRow{
		Address:      strings.TrimSpace(row.IP),
		ExportStatus: exportStatus,
		StageStatus:  stageStatus,
	}
	if colo != "" {
		result.Colo = &colo
	}
	if delay > 0 {
		delayCopy := delay
		result.TCPLatencyMS = &delayCopy
	}
	if trace > 0 {
		traceCopy := trace
		result.TraceLatencyMS = &traceCopy
	}
	if download >= 0 {
		downloadCopy := download
		result.DownloadMbps = &downloadCopy
	}
	if maxDownload >= 0 {
		maxDownloadCopy := maxDownload
		result.MaxDownloadMbps = &maxDownloadCopy
	}
	if sourcePort > 0 {
		sourcePortCopy := sourcePort
		result.SourcePort = &sourcePortCopy
	}
	if testPort > 0 {
		testPortCopy := testPort
		result.TestPort = &testPortCopy
	}
	return result
}
