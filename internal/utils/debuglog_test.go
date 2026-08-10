package utils

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type failingDebugLogWriter struct{}

func (failingDebugLogWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("stdout is unavailable")
}

func TestDebugLoggersKeepFilesAndTaskContextsIsolated(t *testing.T) {
	paths := []string{
		filepath.Join(t.TempDir(), "first.log"),
		filepath.Join(t.TempDir(), "second.log"),
	}
	loggers := []*DebugLogger{NewDebugLogger(), NewDebugLogger()}
	for index, logger := range loggers {
		logger.console = io.Discard
		if _, err := logger.Configure(true, paths[index]); err != nil {
			t.Fatal(err)
		}
		logger.SetContext([]string{"task-first", "task-second"}[index])
	}

	var wg sync.WaitGroup
	for index, logger := range loggers {
		index, logger := index, logger
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sequence := range 25 {
				logger.Event("probe.progress", map[string]any{"sequence": sequence})
			}
			if err := logger.Close(); err != nil {
				t.Errorf("close logger %d: %v", index, err)
			}
		}()
	}
	wg.Wait()

	for index, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		ownTask := []string{"task-first", "task-second"}[index]
		otherTask := []string{"task-second", "task-first"}[index]
		if strings.Count(string(raw), ownTask) != 25 || strings.Contains(string(raw), otherTask) {
			t.Fatalf("logger %d output mixed task contexts: %s", index, raw)
		}
	}
}

func TestDebugEventWritesJSONLAndRedactsSensitiveFields(t *testing.T) {
	logger := NewDebugLogger()
	logger.console = io.Discard
	logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
	if _, err := logger.Configure(true, logPath); err != nil {
		t.Fatalf("ConfigureDebugLog returned error: %v", err)
	}
	logger.SetContext("task-redaction")

	logger.Event("probe.start", map[string]any{
		"config": map[string]any{
			"api_token": "secret-token",
			"url":       "https://example.com/file?token=query-secret&ok=1",
		},
		"headers": map[string]string{
			"Authorization": "Bearer header-secret",
			"Host":          "example.com",
		},
		"message": "Authorization Bearer inline-secret-token",
	})
	if err := logger.Close(); err != nil {
		t.Fatalf("CloseDebugLog returned error: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("log line count = %d, want 1: %q", len(lines), string(raw))
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, lines[0])
	}
	if entry["event"] != "probe.start" {
		t.Fatalf("event = %v, want probe.start", entry["event"])
	}
	if entry["task_id"] != "task-redaction" {
		t.Fatalf("task_id = %v, want task-redaction", entry["task_id"])
	}
	if strings.Contains(lines[0], "secret-token") || strings.Contains(lines[0], "header-secret") || strings.Contains(lines[0], "inline-secret-token") || strings.Contains(lines[0], "query-secret") {
		t.Fatalf("log line leaked a sensitive value: %s", lines[0])
	}

	config, ok := entry["config"].(map[string]any)
	if !ok {
		t.Fatalf("config field has type %T, want object", entry["config"])
	}
	if config["api_token"] != redactedValue {
		t.Fatalf("api_token = %v, want %s", config["api_token"], redactedValue)
	}
	redactedURL, err := url.Parse(config["url"].(string))
	if err != nil {
		t.Fatalf("redacted URL did not parse: %v", err)
	}
	if redactedURL.Query().Get("token") != redactedValue {
		t.Fatalf("token query = %q, want %q", redactedURL.Query().Get("token"), redactedValue)
	}
}

func TestDebugEventWritesFileWhenConsoleWriterFails(t *testing.T) {
	logger := NewDebugLogger()
	logger.console = failingDebugLogWriter{}
	logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
	if _, err := logger.Configure(true, logPath); err != nil {
		t.Fatalf("ConfigureDebugLog returned error: %v", err)
	}

	logger.Event("probe.start", map[string]any{
		"message": "file write should not depend on stdout",
	})
	if err := logger.Close(); err != nil {
		t.Fatalf("CloseDebugLog returned error: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(raw), `"event":"probe.start"`) {
		t.Fatalf("log file was not written after console failure: %q", string(raw))
	}
}

func TestDebugEventWritesFreeformAndRedactsSensitiveFields(t *testing.T) {
	logger := NewDebugLogger()
	logger.console = io.Discard
	logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
	if _, err := logger.Configure(true, logPath, DebugLogModeFreeform, "{ts} {event} task={task_id} stage={stage} missing={missing} config={config} {message}"); err != nil {
		t.Fatalf("ConfigureDebugLog returned error: %v", err)
	}
	logger.SetContext("task-freeform")

	logger.Event("probe.start", map[string]any{
		"config": map[string]any{
			"api_token": "secret-token",
			"url":       "https://example.com/file?token=query-secret&ok=1",
		},
		"message": "Authorization Bearer inline-secret-token",
		"stage":   "stage0_pool",
	})
	if err := logger.Close(); err != nil {
		t.Fatalf("CloseDebugLog returned error: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	line := strings.TrimSpace(string(raw))
	if !strings.Contains(line, "probe.start task=task-freeform stage=stage0_pool missing= ") {
		t.Fatalf("freeform line = %q, want event, task_id, stage and empty missing field", line)
	}
	if !strings.Contains(line, "api_token") || !strings.Contains(line, redactedValue) || !strings.Contains(line, "%3Credacted%3E") {
		t.Fatalf("freeform line did not include redacted config JSON: %s", line)
	}
	if strings.Contains(line, "secret-token") || strings.Contains(line, "inline-secret-token") || strings.Contains(line, "query-secret") {
		t.Fatalf("freeform line leaked a sensitive value: %s", line)
	}
}

func TestDebugEventSimpleVerbosityFiltersDetailedEvents(t *testing.T) {
	logger := NewDebugLogger()
	logger.console = io.Discard
	logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
	if _, err := logger.Configure(true, logPath, DebugLogModeStructured, "", DebugLogVerbositySimple); err != nil {
		t.Fatalf("ConfigureDebugLog returned error: %v", err)
	}

	logger.Event("probe.start", map[string]any{"message": "start"})
	logger.Event("stage.start", map[string]any{"stage": "stage1_tcp"})
	logger.Event("stage.detail", map[string]any{"stage": "stage1_tcp", "message": "detail"})
	logger.Event("stage.complete", map[string]any{"stage": "stage1_tcp"})
	logger.Event("probe.export", map[string]any{"message": "export"})
	logger.Event("probe.complete", map[string]any{"message": "complete"})
	logger.Event("probe.failed", map[string]any{"message": "failed"})
	if err := logger.Close(); err != nil {
		t.Fatalf("CloseDebugLog returned error: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	text := string(raw)
	for _, event := range []string{"probe.start", "stage.complete", "probe.export", "probe.complete", "probe.failed"} {
		if !strings.Contains(text, `"event":"`+event+`"`) {
			t.Fatalf("simple log missing %s: %s", event, text)
		}
	}
	for _, event := range []string{"stage.start", "stage.detail"} {
		if strings.Contains(text, `"event":"`+event+`"`) {
			t.Fatalf("simple log included %s: %s", event, text)
		}
	}
}

func TestAppendErrorLogCreatesJSONLAndRedactsSensitiveFields(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "logs", "error-log.txt")

	if err := AppendErrorLog(logPath, "probe.failed", map[string]any{
		"api_token": "secret-token",
		"headers": map[string]string{
			"Authorization": "Bearer header-secret",
			"Host":          "example.com",
		},
		"message":        "failed with Bearer inline-secret-token",
		"stage":          "stage1_tcp",
		"task_id":        "task-error-log",
		"debug_log_path": filepath.Join("logs", "cfip-log.txt"),
	}); err != nil {
		t.Fatalf("AppendErrorLog returned error: %v", err)
	}
	if err := AppendErrorLog(logPath, "desktop.snapshot.persist_failed", map[string]any{
		"message":      "snapshot failed",
		"source_event": "probe.completed",
		"task_id":      "task-error-log",
	}); err != nil {
		t.Fatalf("second AppendErrorLog returned error: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2: %q", len(lines), string(raw))
	}
	if strings.Contains(string(raw), "secret-token") || strings.Contains(string(raw), "header-secret") || strings.Contains(string(raw), "inline-secret-token") {
		t.Fatalf("error log leaked a sensitive value: %s", string(raw))
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first line is not JSON: %v", err)
	}
	if first["event"] != "probe.failed" || first["level"] != "error" {
		t.Fatalf("first entry = %#v, want probe.failed error", first)
	}
	if first["api_token"] != redactedValue {
		t.Fatalf("api_token = %v, want %s", first["api_token"], redactedValue)
	}
	if first["debug_log_path"] != filepath.Join("logs", "cfip-log.txt") {
		t.Fatalf("debug_log_path = %v", first["debug_log_path"])
	}
}
