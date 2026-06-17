package appcore

import (
	"strings"
	"time"
)

type TaskLifecycleAudit struct {
	CompletedAt    string   `json:"completed_at,omitempty"`
	Entrypoint     string   `json:"entrypoint"`
	FailureStage   string   `json:"failure_stage,omitempty"`
	ResultCount    int      `json:"result_count"`
	RuntimeCleared bool     `json:"runtime_cleared"`
	SnapshotStatus string   `json:"snapshot_status,omitempty"`
	StartedAt      string   `json:"started_at"`
	Stages         []string `json:"stages,omitempty"`
	TaskID         string   `json:"task_id"`
	TerminalEvent  string   `json:"terminal_event,omitempty"`
	TerminalReason string   `json:"terminal_reason,omitempty"`
}

func NewTaskLifecycleAudit(taskID string, entrypoint string, startedAt time.Time) *TaskLifecycleAudit {
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	return &TaskLifecycleAudit{
		Entrypoint: strings.TrimSpace(entrypoint),
		StartedAt:  startedAt.Format(time.RFC3339Nano),
		TaskID:     strings.TrimSpace(taskID),
	}
}

func (audit *TaskLifecycleAudit) RecordStage(stage string) {
	if audit == nil {
		return
	}
	stage = strings.TrimSpace(stage)
	if stage == "" {
		return
	}
	if len(audit.Stages) > 0 && audit.Stages[len(audit.Stages)-1] == stage {
		return
	}
	audit.Stages = append(audit.Stages, stage)
}

func (audit *TaskLifecycleAudit) RecordStages(stages []string) {
	for _, stage := range stages {
		audit.RecordStage(stage)
	}
}

func (audit *TaskLifecycleAudit) Finish(event string, reason string, snapshotStatus string, resultCount int, failureStage string) {
	if audit == nil || strings.TrimSpace(audit.TerminalEvent) != "" {
		return
	}
	audit.TerminalEvent = strings.TrimSpace(event)
	audit.TerminalReason = strings.TrimSpace(reason)
	audit.SnapshotStatus = strings.TrimSpace(snapshotStatus)
	audit.ResultCount = resultCount
	audit.FailureStage = strings.TrimSpace(failureStage)
	audit.CompletedAt = time.Now().Format(time.RFC3339Nano)
}

func (audit *TaskLifecycleAudit) MarkRuntimeCleared() {
	if audit != nil {
		audit.RuntimeCleared = true
	}
}

func (audit *TaskLifecycleAudit) Fields() map[string]any {
	if audit == nil {
		return nil
	}
	return map[string]any{
		"completed_at":    audit.CompletedAt,
		"entrypoint":      audit.Entrypoint,
		"failure_stage":   audit.FailureStage,
		"result_count":    audit.ResultCount,
		"runtime_cleared": audit.RuntimeCleared,
		"snapshot_status": audit.SnapshotStatus,
		"started_at":      audit.StartedAt,
		"stages":          append([]string(nil), audit.Stages...),
		"task_id":         audit.TaskID,
		"terminal_event":  audit.TerminalEvent,
		"terminal_reason": audit.TerminalReason,
	}
}

func TerminalSnapshotStatus(event string, resultCount int) string {
	switch strings.TrimSpace(event) {
	case "probe.completed":
		if resultCount > 0 {
			return "completed"
		}
		return "no_results"
	case "probe.no_results":
		return "no_results"
	case "probe.failed":
		return "failed"
	case "scheduler.probe_skipped_active":
		return "skipped"
	default:
		return strings.TrimSpace(event)
	}
}

func TaskLifecycleAuditNeedsErrorLog(audit *TaskLifecycleAudit) bool {
	if audit == nil {
		return false
	}
	switch strings.TrimSpace(audit.TerminalEvent) {
	case "probe.completed":
		return false
	case "probe.no_results":
		return true
	case "":
		return false
	default:
		return true
	}
}
