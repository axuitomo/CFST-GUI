package appcore

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

// ProbeControlRequest is the transport-independent request for probe lifecycle
// controls. An empty task id targets the currently active or paused task.
type ProbeControlRequest struct {
	TaskID string `json:"task_id"`
}

type TaskQueryRequest struct {
	TaskID string `json:"task_id"`
	Limit  int    `json:"limit"`
}

type TaskResultsRequest struct {
	TaskID   string `json:"task_id"`
	Path     string `json:"path"`
	SortBy   string `json:"sort_by"`
	Order    string `json:"order"`
	Filter   string `json:"filter"`
	IPFilter string `json:"ip_filter"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

type TaskResultsPage struct {
	Count       int              `json:"count"`
	Results     []ProbeResultRow `json:"results"`
	TotalCount  int              `json:"total_count"`
	SourceCount int              `json:"-"`
	Found       bool             `json:"-"`
	SourceKind  string           `json:"-"`
	SourcePath  string           `json:"-"`
}

// ProbeRunner is kept private to the lifecycle wrapper. Probe preparation,
// stage execution, export, and persistence are owned by Service.RunProbe.
type ProbeRunner func(context.Context) (any, error)

func (s *Service) runProbeWithRunner(taskID string, runner ProbeRunner) (result any, err error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, NewProbeTaskError("PROBE_TASK_ID_REQUIRED", "task id is required")
	}
	ctx, ok, current := s.runtime.Start(taskID)
	if !ok {
		return nil, NewProbeTaskError("PROBE_ALREADY_RUNNING", current)
	}
	s.resetProbeEventState(taskID)
	defer s.runtime.Clear(taskID)
	_ = s.writeTaskSnapshot(BuildAcceptedTaskSnapshot(taskID, s.now()))
	if runner == nil {
		return nil, NewProbeTaskError("PROBE_RUNNER_REQUIRED", "probe runner is required")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = NewProbeTaskError("PROBE_PANIC", fmt.Sprint(recovered))
		}
	}()
	return runner(ctx)
}

func (s *Service) startProbeWithRunner(taskID string, runner ProbeRunner) CommandResult {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return NewCommandResult("PROBE_TASK_ID_REQUIRED", nil, "task id is required", false, nil, nil)
	}
	ctx, ok, current := s.runtime.Start(taskID)
	if !ok {
		return NewCommandResult("PROBE_ALREADY_RUNNING", nil, "probe task already running", false, stringPtr(current), nil)
	}
	s.resetProbeEventState(taskID)
	_ = s.writeTaskSnapshot(BuildAcceptedTaskSnapshot(taskID, s.now()))
	go func() {
		defer s.runtime.Clear(taskID)
		defer func() {
			if recovered := recover(); recovered != nil {
				payload := map[string]any{"message": "probe runner panic", "recoverable": false}
				if value, ok := recovered.(error); ok {
					payload["error"] = value.Error()
				} else {
					payload["error"] = recovered
				}
				_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.failed", TaskID: taskID, Payload: payload})
			}
		}()
		if runner == nil {
			payload := map[string]any{"message": "probe runner is required", "recoverable": false}
			_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.failed", TaskID: taskID, Payload: payload})
			return
		}
		if _, err := runner(ctx); err != nil {
			payload := map[string]any{"message": err.Error(), "recoverable": false}
			_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.failed", TaskID: taskID, Payload: payload})
		}
	}()
	return NewCommandResult("PROBE_ACCEPTED", map[string]any{
		"accepted":        true,
		"export_path":     "",
		"source_statuses": []SourceStatus{},
		"task_id":         taskID,
	}, "probe task accepted", true, stringPtr(taskID), nil)
}

type ProbeTaskError struct {
	Code    string
	Message string
}

func NewProbeTaskError(code, message string) error {
	return &ProbeTaskError{Code: code, Message: message}
}

func (err *ProbeTaskError) Error() string {
	if err == nil {
		return ""
	}
	return err.Message
}

func (s *Service) taskAttachment() TaskAttachment {
	state := s.runtime.State()
	return TaskAttachment{CurrentTaskID: state.CurrentTaskID, PauseRequested: state.PauseRequested, PausedTaskID: state.PausedTaskID}
}

func (s *Service) taskStoreSnapshot(taskID string) (TaskSnapshot, bool, error) {
	s.mu.RLock()
	store := s.taskStore
	s.mu.RUnlock()
	if store == nil {
		return TaskSnapshot{}, false, nil
	}
	return store.LoadSnapshot(taskID, s.taskAttachment())
}

func (s *Service) writeTaskSnapshot(snapshot TaskSnapshot) error {
	s.mu.RLock()
	store := s.taskStore
	s.mu.RUnlock()
	if store == nil {
		return nil
	}
	return store.WriteSnapshot(snapshot, s.taskAttachment())
}

func (s *Service) recordTaskEvent(taskID, event string, payload map[string]any) error {
	current, _, err := s.taskStoreSnapshot(taskID)
	if err != nil {
		return err
	}
	next := MergeTaskSnapshots(current, TaskSnapshotFromEvent(taskID, event, payload, s.now()))
	return s.writeTaskSnapshot(next)
}

func (s *Service) RecordProbeEvent(taskID, event string, payload map[string]any) error {
	return s.recordTaskEvent(taskID, event, payload)
}

func (s *Service) LoadTaskSnapshot(taskID string) (TaskSnapshot, bool, error) {
	return s.taskStoreSnapshot(taskID)
}

func (s *Service) WriteTaskSnapshot(snapshot TaskSnapshot) error {
	return s.writeTaskSnapshot(snapshot)
}

func (s *Service) WriteTaskResults(taskID string, rows []ProbeResultRow) error {
	s.mu.RLock()
	store := s.taskStore
	s.mu.RUnlock()
	if store == nil {
		return nil
	}
	return store.WriteResults(taskID, rows)
}

func (s *Service) now() time.Time {
	s.mu.RLock()
	clock := s.options.Clock
	s.mu.RUnlock()
	if clock == nil {
		return time.Now()
	}
	return clock()
}

func (s *Service) PauseProbe(request ProbeControlRequest) CommandResult {
	transition := s.runtime.Pause(strings.TrimSpace(request.TaskID))
	taskID := transition.TaskID
	if transition.Unavailable {
		return NewCommandResult("PROBE_PAUSE_UNAVAILABLE", nil, "no pausable probe task is running", false, stringPtr(taskID), nil)
	}
	payload := map[string]any{"reason": "pause requested", "recoverable": true}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.cooling", TaskID: taskID, Payload: payload})
	return NewCommandResult("PROBE_PAUSE_REQUESTED", nil, "probe pause requested", true, stringPtr(taskID), nil)
}

func (s *Service) CancelProbe(request ProbeControlRequest) CommandResult {
	transition := s.runtime.Cancel(strings.TrimSpace(request.TaskID))
	taskID := transition.TaskID
	if transition.Unavailable {
		state := s.runtime.State()
		if state.CurrentTaskID == "" && taskID != "" && s.runtime.QueueCancel(taskID) {
			return NewCommandResult("PROBE_CANCEL_REQUESTED", nil, "probe cancel queued", true, stringPtr(taskID), nil)
		}
		return NewCommandResult("PROBE_CANCEL_UNAVAILABLE", nil, "no cancellable probe task is running", false, stringPtr(taskID), nil)
	}
	payload := map[string]any{"reason": "cancel requested", "recoverable": false}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.cooling", TaskID: taskID, Payload: payload})
	if transition.WasPaused {
		s.mu.RLock()
		timeout := s.options.CancelTimeout
		s.mu.RUnlock()
		if !s.runtime.WaitStopped(taskID, timeout) {
			return NewCommandResult("PROBE_CANCEL_PENDING", nil, "probe cancellation is pending", false, stringPtr(taskID), nil)
		}
	}
	return NewCommandResult("PROBE_CANCEL_REQUESTED", nil, "probe cancellation requested", true, stringPtr(taskID), nil)
}

func (s *Service) ResumeProbe(request ProbeControlRequest) CommandResult {
	transition := s.runtime.Resume(strings.TrimSpace(request.TaskID))
	taskID := transition.TaskID
	if transition.Unavailable {
		return NewCommandResult("PROBE_RESUME_UNAVAILABLE", nil, "no paused probe task is available", false, stringPtr(taskID), nil)
	}
	snapshot, ok, _ := s.taskStoreSnapshot(taskID)
	if !ok || strings.TrimSpace(snapshot.TaskID) == "" {
		snapshot = BuildAcceptedTaskSnapshot(taskID, s.now())
	}
	snapshot.Status = "running"
	snapshot.RuntimeAttached = true
	snapshot.ResumeCapable = false
	snapshot.SessionState = "active_runtime"
	if strings.TrimSpace(snapshot.CurrentStage) == "" || snapshot.CurrentStage == "cooling" {
		if snapshot.Progress != nil && strings.TrimSpace(snapshot.Progress.Stage) != "" {
			snapshot.CurrentStage = strings.TrimSpace(snapshot.Progress.Stage)
		} else {
			snapshot.CurrentStage = "stage1_tcp"
		}
	}
	_ = s.writeTaskSnapshot(snapshot)
	payload := map[string]any{"message": "probe resumed", "current_stage": snapshot.CurrentStage, "stage": snapshot.CurrentStage}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.resumed", TaskID: taskID, Payload: payload})
	return NewCommandResult("PROBE_RESUME_REQUESTED", nil, "probe resume requested", true, stringPtr(taskID), nil)
}

func (s *Service) GetTask(request TaskQueryRequest) CommandResult {
	taskID := strings.TrimSpace(request.TaskID)
	if taskID == "" {
		state := s.runtime.State()
		if state.CurrentTaskID != "" {
			taskID = state.CurrentTaskID
		} else {
			taskID = state.PausedTaskID
		}
	}
	snapshot, ok, err := s.taskStoreSnapshot(taskID)
	if err != nil {
		return NewCommandResult("TASK_SNAPSHOT_LOAD_FAILED", nil, err.Error(), false, stringPtr(taskID), nil)
	}
	if !ok {
		return NewCommandResult("TASK_NOT_FOUND", nil, "task not found", false, stringPtr(taskID), nil)
	}
	return NewCommandResult("TASK_SNAPSHOT", snapshot, "task snapshot loaded", true, stringPtr(taskID), nil)
}

func (s *Service) ListTasks(request TaskQueryRequest) CommandResult {
	limit := request.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	s.mu.RLock()
	store := s.taskStore
	s.mu.RUnlock()
	if store == nil {
		return NewCommandResult("TASK_SNAPSHOT_LIST", map[string]any{"count": 0, "items": []TaskSnapshot{}}, "task history loaded", true, nil, nil)
	}
	candidates, err := store.ListSnapshots(limit)
	if err != nil {
		return NewCommandResult("TASK_SNAPSHOT_LIST_FAILED", nil, err.Error(), false, nil, nil)
	}
	items := make([]TaskSnapshot, 0, len(candidates))
	for _, candidate := range candidates {
		snapshot, ok, err := store.LoadSnapshot(candidate.TaskID, s.taskAttachment())
		if err != nil {
			return NewCommandResult("TASK_SNAPSHOT_LIST_FAILED", nil, err.Error(), false, nil, nil)
		}
		if ok {
			items = append(items, snapshot)
		}
	}
	SortTaskSnapshotsLatestFirst(items)
	if len(items) > limit {
		items = items[:limit]
	}
	return NewCommandResult("TASK_SNAPSHOT_LIST", map[string]any{"count": len(items), "items": items}, "task history loaded", true, nil, nil)
}

func (s *Service) ListTaskResults(taskID string) ([]ProbeResultRow, error) {
	s.mu.RLock()
	store := s.taskStore
	s.mu.RUnlock()
	if store == nil {
		return nil, nil
	}
	return store.LoadResults(taskID)
}

func (s *Service) QueryTaskResults(request TaskResultsRequest) (TaskResultsPage, error) {
	s.mu.RLock()
	store := s.taskStore
	s.mu.RUnlock()
	var page TaskResultsPage
	if store != nil {
		persisted, err := store.QueryResults(request.TaskID, request)
		if err != nil {
			return TaskResultsPage{}, err
		}
		page = persisted
		if page.Found {
			page.SourceKind = "empty_persisted"
			if page.SourceCount > 0 {
				page.SourceKind = "persisted"
			}
			page.SourcePath = store.ResultsPath(request.TaskID)
		}
	}
	if page.Found && page.SourceCount > 0 {
		return page, nil
	}
	var firstErr error
	for _, path := range s.taskResultCandidatePaths(request) {
		csvPage, csvErr := QueryProbeResultRowsFromCSV(path, request)
		if csvErr != nil {
			if firstErr == nil {
				firstErr = csvErr
			}
			continue
		}
		if csvPage.SourceCount == 0 && page.Found {
			continue
		}
		return csvPage, nil
	}
	if page.Found {
		return page, nil
	}
	if firstErr != nil {
		return TaskResultsPage{}, firstErr
	}
	return page, nil
}

func (s *Service) taskResultCandidatePaths(request TaskResultsRequest) []string {
	paths := make([]string, 0, 4)
	appendPath := func(path string, resolveRelative bool) {
		path = strings.TrimSpace(path)
		if path == "" || strings.HasPrefix(path, "content://") {
			return
		}
		if resolveRelative && !filepath.IsAbs(path) {
			path = filepath.Join(s.StorageLayout().ExportsRoot(), path)
		}
		for _, existing := range paths {
			if filepath.Clean(existing) == filepath.Clean(path) {
				return
			}
		}
		paths = append(paths, path)
	}
	appendPath(request.Path, true)
	if taskID := strings.TrimSpace(request.TaskID); taskID != "" {
		snapshot, ok, err := s.LoadTaskSnapshot(taskID)
		if err == nil && ok && snapshot.ExportRecord != nil {
			appendPath(snapshot.ExportRecord.SourcePath, false)
			targetDir := strings.TrimSpace(snapshot.ExportRecord.TargetDir)
			fileName := strings.TrimSpace(snapshot.ExportRecord.FileName)
			if targetDir != "" && fileName != "" && !strings.HasPrefix(targetDir, "content://") {
				appendPath(filepath.Join(targetDir, fileName), false)
			}
		}
	}
	return paths
}

func (s *Service) InvokeTaskResults(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("RESULT_FILE_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	request := TaskResultsRequest{
		TaskID:   taskID,
		Path:     strings.TrimSpace(stringValue(firstNonNil(payload["path"], payload["source_path"], payload["sourcePath"], payload["target_path"], payload["targetPath"], payload["export_path"], payload["exportPath"]), "")),
		SortBy:   strings.TrimSpace(stringValue(firstNonNil(payload["sort_by"], payload["sortBy"]), "")),
		Order:    strings.TrimSpace(stringValue(payload["order"], "asc")),
		Filter:   strings.TrimSpace(stringValue(payload["filter"], "all")),
		IPFilter: strings.TrimSpace(stringValue(firstNonNil(payload["ip_filter"], payload["ipFilter"]), "all")),
		Limit:    intValue(firstNonNil(payload["limit"], payload["page_size"], payload["pageSize"]), 0),
		Offset:   intValue(payload["offset"], 0),
	}
	page, err := s.QueryTaskResults(request)
	if err != nil {
		return NewCommandResult("RESULT_FILE_UNAVAILABLE", nil, err.Error(), false, stringPtr(taskID), nil)
	}
	return NewCommandResult("RESULT_FILE_LISTED", map[string]any{
		"count":       page.Count,
		"results":     page.Results,
		"source_kind": page.SourceKind,
		"source_path": page.SourcePath,
		"total_count": page.TotalCount,
	}, "已从结果文件读取当前结果。", true, stringPtr(taskID), nil)
}

func (s *Service) PersistProbeResults(taskID string, rows []probecore.ProbeRow) error {
	s.mu.RLock()
	store := s.taskStore
	s.mu.RUnlock()
	if store == nil {
		return nil
	}
	return store.WriteResults(taskID, ProbeRowsToResultRows(rows))
}

func (s *Service) PersistCompletedProbe(taskID string, result ProbeRunResult) error {
	if err := s.PersistProbeResults(taskID, result.Results); err != nil {
		return err
	}
	payload := map[string]any{
		"completed_at":    s.now().Format(time.RFC3339),
		"current_stage":   "completed",
		"exported":        len(result.Results),
		"failure_summary": map[string]any{},
		"passed":          result.Summary.Passed,
		"result_count":    len(result.Results),
		"started_at":      result.StartedAt,
		"target_path":     result.OutputFile,
		"task_context":    structToMap(result.TaskContext),
	}
	snapshot := MergeTaskSnapshots(TaskSnapshot{}, TaskSnapshotFromEvent(taskID, "probe.completed", payload, s.now()))
	return s.writeTaskSnapshot(snapshot)
}

func ResultRowsToProbeRows(rows []ProbeResultRow) []probecore.ProbeRow {
	result := make([]probecore.ProbeRow, 0, len(rows))
	for _, row := range rows {
		item := probecore.ProbeRow{IP: strings.TrimSpace(row.Address)}
		if row.Colo != nil {
			item.Colo = strings.TrimSpace(*row.Colo)
		}
		if row.TCPLatencyMS != nil {
			item.DelayMS = *row.TCPLatencyMS
		}
		if row.DownloadMbps != nil {
			item.DownloadSpeedMB = *row.DownloadMbps
		}
		if row.MaxDownloadMbps != nil {
			item.MaxDownloadSpeedMB = *row.MaxDownloadMbps
		}
		if row.SourcePort != nil {
			item.SourcePort = *row.SourcePort
		}
		if row.TestPort != nil {
			item.TestPort = *row.TestPort
		}
		if row.TraceLatencyMS != nil {
			item.TraceDelayMS = *row.TraceLatencyMS
		}
		if item.IP == "" {
			continue
		}
		result = append(result, item)
	}
	return result
}

func ProbeRowsToResultRows(rows []probecore.ProbeRow) []ProbeResultRow {
	resultRows := make([]ProbeResultRow, 0, len(rows))
	for _, row := range rows {
		colo := strings.TrimSpace(row.Colo)
		if strings.EqualFold(colo, "N/A") {
			colo = ""
		}
		result := ProbeResultRow{
			Address:      strings.TrimSpace(row.IP),
			ExportStatus: "exported",
			StageStatus:  "completed",
		}
		if colo != "" {
			value := colo
			result.Colo = &value
		}
		if row.DelayMS > 0 {
			value := row.DelayMS
			result.TCPLatencyMS = &value
		}
		if row.TraceDelayMS > 0 {
			value := row.TraceDelayMS
			result.TraceLatencyMS = &value
		}
		if row.DownloadSpeedMB >= 0 {
			value := row.DownloadSpeedMB
			result.DownloadMbps = &value
		}
		if row.MaxDownloadSpeedMB >= 0 {
			value := row.MaxDownloadSpeedMB
			result.MaxDownloadMbps = &value
		}
		if row.SourcePort > 0 {
			value := row.SourcePort
			result.SourcePort = &value
		}
		if row.TestPort > 0 {
			value := row.TestPort
			result.TestPort = &value
		}
		resultRows = append(resultRows, result)
	}
	return resultRows
}

func structToMap(value any) map[string]any {
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

func PageTaskResults(rows []ProbeResultRow, request TaskResultsRequest) TaskResultsPage {
	filtered := FilterSortProbeResultRows(rows, request.SortBy, request.Order, request.Filter, request.IPFilter)
	totalCount := len(filtered)
	paged := PaginateProbeResultRows(filtered, request.Limit, request.Offset)
	if paged == nil {
		paged = []ProbeResultRow{}
	}
	return TaskResultsPage{
		Count:       len(paged),
		Results:     paged,
		TotalCount:  totalCount,
		SourceCount: len(rows),
	}
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func decodeProbeControlRequest(raw []byte) (ProbeControlRequest, error) {
	var request ProbeControlRequest
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return ProbeControlRequest{}, err
	}
	return request, nil
}
