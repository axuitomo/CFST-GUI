package mobileapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func NewService() *Service {
	service := &Service{
		pipelineResults:   map[string]appcore.PipelineRunResult{},
		taskEventMetadata: map[string]map[string]any{},
		taskSnapshots:     map[string]taskSnapshot{},
	}
	service.pauseCond = sync.NewCond(&service.stateMu)
	return service
}

func (s *Service) SetEventSink(sink EventSink) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.eventSink = sink
}

func (s *Service) Init(baseDir string) string {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = defaultBaseDir()
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return encodeCommand(commandResultFor("MOBILE_INIT_FAILED", nil, err.Error(), false, nil, nil))
	}
	s.stateMu.Lock()
	s.baseDir = baseDir
	s.stateMu.Unlock()
	return encodeCommand(commandResultFor("MOBILE_INIT_OK", map[string]any{
		"base_dir":    baseDir,
		"config_path": s.configPath(),
	}, "Android mobile API 已初始化。", true, nil, nil))
}

func (s *Service) basePath() string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if strings.TrimSpace(s.baseDir) != "" {
		return s.baseDir
	}
	return defaultBaseDir()
}

func defaultBaseDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		dir = "."
	}
	return filepath.Join(dir, "CFST-GUI", "mobile")
}

func (s *Service) configPath() string {
	return filepath.Join(s.basePath(), "mobile-config.json")
}

func (s *Service) schedulerStatusPath() string {
	return filepath.Join(s.basePath(), "scheduler-status.json")
}

func (s *Service) debugLogPath() string {
	return filepath.Join(s.logDirectoryPath(), "cfip-log.txt")
}

func (s *Service) errorLogPath() string {
	return filepath.Join(s.logDirectoryPath(), "error-log.txt")
}

func (s *Service) logDirectoryPath() string {
	return filepath.Join(s.basePath(), "logs")
}

func (s *Service) exportPath(outputFile string) string {
	outputFile = strings.TrimSpace(outputFile)
	if outputFile == "" {
		outputFile = "result.csv"
	}
	if filepath.IsAbs(outputFile) {
		return outputFile
	}
	return filepath.Join(s.basePath(), "exports", outputFile)
}

func (s *Service) tasksRootPath() string {
	return filepath.Join(s.basePath(), "tasks")
}

func (s *Service) taskSnapshotPath(taskID string) string {
	return filepath.Join(s.tasksRootPath(), strings.TrimSpace(taskID)+".json")
}

func (s *Service) taskResultsPath(taskID string) string {
	return filepath.Join(s.tasksRootPath(), strings.TrimSpace(taskID)+"-results.json")
}

func shouldCacheTaskSnapshotInMemory(status string) bool {
	switch strings.TrimSpace(status) {
	case "running", "preparing", "cooling", "partial":
		return true
	default:
		return false
	}
}

func (s *Service) writeTaskSnapshot(snapshot taskSnapshot) error {
	taskID := strings.TrimSpace(snapshot.TaskID)
	if taskID == "" {
		return nil
	}
	snapshot.TaskID = taskID
	snapshot.UpdatedAt = nowRFC3339()
	s.stateMu.Lock()
	currentTaskID := s.currentTaskID
	pauseRequested := s.pauseRequested
	pausedTaskID := s.pausedTaskID
	s.stateMu.Unlock()
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
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := appcore.WriteFileAtomic(s.taskSnapshotPath(taskID), raw, 0o600); err != nil {
		return err
	}
	s.stateMu.Lock()
	if shouldCacheTaskSnapshotInMemory(snapshot.Status) {
		s.taskSnapshots[taskID] = snapshot
	} else {
		delete(s.taskSnapshots, taskID)
	}
	s.stateMu.Unlock()
	return nil
}

func (s *Service) writeTaskResults(taskID string, rows []probeResultRow) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	raw, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return appcore.WriteFileAtomic(s.taskResultsPath(taskID), raw, 0o600)
}

func (s *Service) loadTaskSnapshot(taskID string) (taskSnapshot, bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return taskSnapshot{}, false, nil
	}
	var (
		snapshot taskSnapshot
		ok       bool
	)
	s.stateMu.Lock()
	snapshot, ok = s.taskSnapshots[taskID]
	s.stateMu.Unlock()
	if !ok {
		raw, err := os.ReadFile(s.taskSnapshotPath(taskID))
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
	s.stateMu.Lock()
	if shouldCacheTaskSnapshotInMemory(snapshot.Status) {
		s.taskSnapshots[taskID] = snapshot
	} else {
		delete(s.taskSnapshots, taskID)
	}
	if snapshot.Status == "running" || snapshot.Status == "preparing" || snapshot.Status == "cooling" || snapshot.Status == "partial" {
		snapshot.RuntimeAttached = s.currentTaskID == taskID
		if snapshot.RuntimeAttached {
			snapshot.ResumeCapable = s.pauseRequested && s.pausedTaskID == taskID
			if snapshot.ResumeCapable {
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
		if shouldCacheTaskSnapshotInMemory(snapshot.Status) {
			s.taskSnapshots[taskID] = snapshot
		} else {
			delete(s.taskSnapshots, taskID)
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
	s.stateMu.Unlock()
	if changed {
		_ = s.writeTaskSnapshot(snapshot)
	}
	return snapshot, true, nil
}

func (s *Service) loadTaskResults(taskID string) ([]probeResultRow, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(s.taskResultsPath(taskID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []probeResultRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func encodeJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return `{"code":"JSON_ENCODE_FAILED","data":null,"message":"` + escapeJSONString(err.Error()) + `","ok":false,"schema_version":"` + schemaVersion + `","task_id":null,"warnings":[]}`
	}
	return string(raw)
}

func encodeCommand(result commandResult) string {
	return encodeJSON(result)
}

func commandResultFor(code string, data any, message string, ok bool, taskID *string, warnings []string) commandResult {
	if warnings == nil {
		warnings = []string{}
	}
	return commandResult{
		Code:          code,
		Data:          data,
		Message:       message,
		OK:            ok,
		SchemaVersion: schemaVersion,
		TaskID:        taskID,
		Warnings:      dedupeStrings(warnings),
	}
}

func escapeJSONString(value string) string {
	raw, _ := json.Marshal(value)
	return strings.Trim(string(raw), `"`)
}

func decodeObject(raw string) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func decodeInto(raw string, target any) error {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func mapValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func intValue(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	case string:
		var parsed int
		if _, err := fmt.Sscanf(typed, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func floatValue(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err == nil {
			return parsed
		}
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(typed, "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func boolValue(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed != 0
		}
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		switch normalized {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

func stringValue(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
