package utils

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingDebugLogWriter struct{}

func (failingDebugLogWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("stdout is unavailable")
}

func TestDebugEventWritesJSONLAndRedactsSensitiveFields(t *testing.T) {
	oldDebug := Debug
	t.Cleanup(func() {
		Debug = oldDebug
		_ = CloseDebugLog()
	})

	Debug = true
	logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
	if _, err := ConfigureDebugLog(true, logPath); err != nil {
		t.Fatalf("ConfigureDebugLog returned error: %v", err)
	}
	SetDebugLogContext("task-redaction")

	DebugEvent("probe.start", map[string]any{
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
	if err := CloseDebugLog(); err != nil {
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
	if entry["schema_version"] != LogSchemaVersion || entry["channel"] != LogChannelDebug {
		t.Fatalf("debug schema/channel = %#v", entry)
	}
	if strings.Contains(lines[0], "secret-token") || strings.Contains(lines[0], "header-secret") || strings.Contains(lines[0], "inline-secret-token") || strings.Contains(lines[0], "query-secret") {
		t.Fatalf("log line leaked a sensitive value: %s", lines[0])
	}

	data, ok := entry["data"].(map[string]any)
	if !ok {
		t.Fatalf("data field has type %T, want object", entry["data"])
	}
	config, ok := data["config"].(map[string]any)
	if !ok {
		t.Fatalf("config field has type %T, want object", data["config"])
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
	oldDebug := Debug
	oldConsoleOutput := debugLogConsoleOutput
	t.Cleanup(func() {
		Debug = oldDebug
		debugLogConsoleOutput = oldConsoleOutput
		_ = CloseDebugLog()
	})

	Debug = true
	debugLogConsoleOutput = failingDebugLogWriter{}
	logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
	if _, err := ConfigureDebugLog(true, logPath); err != nil {
		t.Fatalf("ConfigureDebugLog returned error: %v", err)
	}

	DebugEvent("probe.start", map[string]any{
		"message": "file write should not depend on stdout",
	})
	if err := CloseDebugLog(); err != nil {
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

func TestDebugEventIgnoresFreeformModeAndWritesStructuredJSONL(t *testing.T) {
	oldDebug := Debug
	t.Cleanup(func() {
		Debug = oldDebug
		_ = CloseDebugLog()
	})

	Debug = true
	logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
	if _, err := ConfigureDebugLog(true, logPath, DebugLogModeFreeform, "{ts} {event} task={task_id} stage={stage} missing={missing} config={config} {message}"); err != nil {
		t.Fatalf("ConfigureDebugLog returned error: %v", err)
	}
	SetDebugLogContext("task-freeform")

	DebugEvent("probe.start", map[string]any{
		"config": map[string]any{
			"api_token": "secret-token",
			"url":       "https://example.com/file?token=query-secret&ok=1",
		},
		"message": "Authorization Bearer inline-secret-token",
		"stage":   "stage0_pool",
	})
	if err := CloseDebugLog(); err != nil {
		t.Fatalf("CloseDebugLog returned error: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	line := strings.TrimSpace(string(raw))
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("freeform mode line is not structured JSON: %v\n%s", err, line)
	}
	if entry["schema_version"] != LogSchemaVersion || entry["channel"] != LogChannelDebug || entry["event"] != "probe.start" {
		t.Fatalf("structured entry = %#v", entry)
	}
	if entry["task_id"] != "task-freeform" || entry["stage"] != "stage0_pool" {
		t.Fatalf("task/stage = %#v", entry)
	}
	data, ok := entry["data"].(map[string]any)
	if !ok {
		t.Fatalf("data field has type %T, want object", entry["data"])
	}
	config, ok := data["config"].(map[string]any)
	if !ok || config["api_token"] != redactedValue {
		t.Fatalf("config data = %#v", data["config"])
	}
	if strings.Contains(line, "secret-token") || strings.Contains(line, "inline-secret-token") || strings.Contains(line, "query-secret") {
		t.Fatalf("structured line leaked a sensitive value: %s", line)
	}
}

func TestDebugEventSimpleVerbosityFiltersDetailedEvents(t *testing.T) {
	oldDebug := Debug
	t.Cleanup(func() {
		Debug = oldDebug
		_ = CloseDebugLog()
	})

	Debug = true
	logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
	if _, err := ConfigureDebugLog(true, logPath, DebugLogModeStructured, "", DebugLogVerbositySimple); err != nil {
		t.Fatalf("ConfigureDebugLog returned error: %v", err)
	}

	DebugEvent("probe.start", map[string]any{"message": "start"})
	DebugEvent("stage.start", map[string]any{"stage": "stage1_tcp"})
	DebugEvent("stage.detail", map[string]any{"stage": "stage1_tcp", "message": "detail"})
	DebugEvent("stage.complete", map[string]any{"stage": "stage1_tcp"})
	DebugEvent("probe.export", map[string]any{"message": "export"})
	DebugEvent("probe.complete", map[string]any{"message": "complete"})
	DebugEvent("probe.failed", map[string]any{"message": "failed"})
	if err := CloseDebugLog(); err != nil {
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
	if first["schema_version"] != LogSchemaVersion || first["channel"] != LogChannelError {
		t.Fatalf("error schema/channel = %#v", first)
	}
	data, ok := first["data"].(map[string]any)
	if !ok {
		t.Fatalf("data field has type %T, want object", first["data"])
	}
	if data["api_token"] != redactedValue {
		t.Fatalf("api_token = %v, want %s", data["api_token"], redactedValue)
	}
	if data["debug_log_path"] != filepath.Join("logs", "cfip-log.txt") {
		t.Fatalf("debug_log_path = %v", data["debug_log_path"])
	}
	if _, ok := first["debug_log_path"]; ok {
		t.Fatalf("debug_log_path leaked to top-level entry: %#v", first)
	}
}
