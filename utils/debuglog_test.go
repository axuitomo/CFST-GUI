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
