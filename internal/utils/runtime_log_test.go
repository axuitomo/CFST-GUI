package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func preserveRuntimeLogForTest(t *testing.T) {
	t.Helper()
	runtimeLogMu.Lock()
	oldConfig := runtimeLog
	oldNow := runtimeLogNow
	oldSync := syncRuntimeLogFile
	runtimeLogMu.Unlock()
	t.Cleanup(func() {
		runtimeLogMu.Lock()
		runtimeLog = oldConfig
		runtimeLogNow = oldNow
		syncRuntimeLogFile = oldSync
		runtimeLogMu.Unlock()
	})
}

func TestRuntimeLogWritesDailyJSONLFiltersLevelAndCleansRetention(t *testing.T) {
	preserveRuntimeLogForTest(t)

	now := time.Date(2026, 6, 14, 15, 30, 0, 0, time.UTC)
	runtimeLogNow = func() time.Time { return now }
	dir := t.TempDir()
	oldDebugLogPath := filepath.Join(dir, "cfip-log.txt")
	if err := os.WriteFile(oldDebugLogPath, []byte("old debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldDebugLogPath, now.AddDate(0, 0, -8), now.AddDate(0, 0, -8)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-2026-06-07.jsonl"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-2026-06-08.jsonl"), []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ConfigureRuntimeLog(true, dir, "WARN", 7); err != nil {
		t.Fatalf("ConfigureRuntimeLog returned error: %v", err)
	}
	if err := AppendRuntimeLog(LogLevelInfo, "probe.info", map[string]any{"message": "filtered"}); err != nil {
		t.Fatalf("AppendRuntimeLog info returned error: %v", err)
	}
	if err := AppendRuntimeLog(LogLevelError, "probe.failed", map[string]any{
		"api_token": "secret-token",
		"message":   "failed with Bearer inline-secret-token",
	}); err != nil {
		t.Fatalf("AppendRuntimeLog error returned error: %v", err)
	}
	if err := AppendRuntimeLog(LogLevelWarn, "probe.warn", map[string]any{"message": "warning"}); err != nil {
		t.Fatalf("AppendRuntimeLog warn returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "app-2026-06-07.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("old runtime log still exists, err=%v", err)
	}
	if _, err := os.Stat(oldDebugLogPath); !os.IsNotExist(err) {
		t.Fatalf("old cfip-log.txt still exists, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app-2026-06-08.jsonl")); err != nil {
		t.Fatalf("retained runtime log missing: %v", err)
	}
	raw, err := os.ReadFile(RuntimeLogFilePath(dir, now))
	if err != nil {
		t.Fatalf("read runtime log: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "probe.info") {
		t.Fatalf("runtime log included filtered info event: %s", text)
	}
	if strings.Contains(text, "secret-token") || strings.Contains(text, "inline-secret-token") {
		t.Fatalf("runtime log leaked sensitive values: %s", text)
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 2 {
		t.Fatalf("runtime log line count = %d, want 2: %s", len(lines), text)
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first runtime log line is not JSON: %v", err)
	}
	if first["event"] != "probe.failed" || first["level"] != LogLevelError {
		t.Fatalf("first runtime entry = %#v, want probe.failed error", first)
	}
	if first["schema_version"] != LogSchemaVersion || first["channel"] != LogChannelRuntime {
		t.Fatalf("first runtime schema/channel = %#v", first)
	}
	data, ok := first["data"].(map[string]any)
	if !ok {
		t.Fatalf("data field has type %T, want object", first["data"])
	}
	if data["api_token"] != redactedValue {
		t.Fatalf("api_token = %v, want %s", data["api_token"], redactedValue)
	}
	if _, ok := first["api_token"]; ok {
		t.Fatalf("api_token leaked to top-level entry: %#v", first)
	}
}

func TestConfiguredRuntimeLogCleanupRemovesOldCfipLogWhenLoggingDisabled(t *testing.T) {
	preserveRuntimeLogForTest(t)

	now := time.Date(2026, 6, 14, 11, 0, 0, 0, time.UTC)
	runtimeLogNow = func() time.Time { return now }
	dir := t.TempDir()
	debugLogPath := filepath.Join(dir, "cfip-log.txt")
	if err := os.WriteFile(debugLogPath, []byte("old debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(debugLogPath, now.AddDate(0, 0, -3), now.AddDate(0, 0, -3)); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureRuntimeLog(false, dir, "error", 1); err != nil {
		t.Fatalf("ConfigureRuntimeLog returned error: %v", err)
	}
	if _, err := os.Stat(debugLogPath); !os.IsNotExist(err) {
		t.Fatalf("cfip-log.txt still exists after disabled cleanup, err=%v", err)
	}
}

func TestRuntimeLogSplitDurabilitySyncsWarningsAndErrorsOnly(t *testing.T) {
	preserveRuntimeLogForTest(t)

	now := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	runtimeLogNow = func() time.Time { return now }
	dir := t.TempDir()
	syncs := make([]string, 0)
	syncRuntimeLogFile = func(file *os.File) error {
		syncs = append(syncs, filepath.Base(file.Name()))
		return nil
	}

	if err := ConfigureRuntimeLog(true, dir, "debug", 7, RuntimeLogDurabilitySplit); err != nil {
		t.Fatalf("ConfigureRuntimeLog returned error: %v", err)
	}
	for _, level := range []string{LogLevelError, LogLevelWarn, LogLevelInfo, LogLevelDebug} {
		if err := AppendRuntimeLog(level, "probe."+level, nil); err != nil {
			t.Fatalf("AppendRuntimeLog(%s) returned error: %v", level, err)
		}
	}

	if len(syncs) != 2 {
		t.Fatalf("sync count = %d, want 2 for error/warn: %#v", len(syncs), syncs)
	}
	for _, name := range syncs {
		if name != "app-2026-06-14.jsonl" {
			t.Fatalf("synced file = %q, want app daily log", name)
		}
	}
}

func TestMonitorLogWritesDailyJSONLAndCleansRetention(t *testing.T) {
	preserveRuntimeLogForTest(t)

	now := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	runtimeLogNow = func() time.Time { return now }
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "monitor-2026-06-07.jsonl"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureRuntimeLog(true, dir, "error", 7); err != nil {
		t.Fatalf("ConfigureRuntimeLog returned error: %v", err)
	}
	if err := AppendMonitorLog(dir, "main.hung", map[string]any{"pid": 123}); err != nil {
		t.Fatalf("AppendMonitorLog returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "monitor-2026-06-07.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("old monitor log still exists, err=%v", err)
	}
	raw, err := os.ReadFile(MonitorLogFilePath(dir, now))
	if err != nil {
		t.Fatalf("read monitor log: %v", err)
	}
	if !strings.Contains(string(raw), `"event":"main.hung"`) {
		t.Fatalf("monitor log missing event: %s", string(raw))
	}
	var entry map[string]any
	if err := json.Unmarshal(raw[:len(raw)-1], &entry); err != nil {
		t.Fatalf("monitor line is not JSON: %v", err)
	}
	if entry["schema_version"] != LogSchemaVersion || entry["channel"] != LogChannelMonitor {
		t.Fatalf("monitor schema/channel = %#v", entry)
	}
	data, ok := entry["data"].(map[string]any)
	if !ok || data["pid"] != float64(123) {
		t.Fatalf("monitor data = %#v, want pid in data", entry["data"])
	}
}

func TestMainHeartbeatWriteReadAndStale(t *testing.T) {
	preserveRuntimeLogForTest(t)

	path := filepath.Join(t.TempDir(), "logs", "main-heartbeat.json")
	startedAt := time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)
	lastSeenAt := startedAt.Add(2 * time.Second)
	heartbeat := NewMainHeartbeat(42, startedAt, lastSeenAt, MainHeartbeatStateRunning, "/tmp/logs")
	if err := WriteMainHeartbeat(path, heartbeat); err != nil {
		t.Fatalf("WriteMainHeartbeat returned error: %v", err)
	}
	got, err := ReadMainHeartbeat(path)
	if err != nil {
		t.Fatalf("ReadMainHeartbeat returned error: %v", err)
	}
	if got.PID != 42 || got.State != MainHeartbeatStateRunning || got.LogDir != "/tmp/logs" {
		t.Fatalf("heartbeat = %#v", got)
	}
	if MainHeartbeatStale(got, lastSeenAt.Add(9*time.Second), 10*time.Second) {
		t.Fatal("heartbeat marked stale before stale window")
	}
	if !MainHeartbeatStale(got, lastSeenAt.Add(11*time.Second), 10*time.Second) {
		t.Fatal("heartbeat not marked stale after stale window")
	}
}

func TestAppendErrorLogMirrorsRuntimeLogAndKeepsLegacyFile(t *testing.T) {
	preserveRuntimeLogForTest(t)

	now := time.Date(2026, 6, 14, 18, 0, 0, 0, time.UTC)
	runtimeLogNow = func() time.Time { return now }
	logDir := filepath.Join(t.TempDir(), "logs")
	legacyPath := filepath.Join(logDir, "error-log.txt")

	if err := AppendErrorLog(legacyPath, "desktop.probe_event_emit_failed", map[string]any{
		"message": "emit failed",
		"task_id": "task-1",
	}); err != nil {
		t.Fatalf("AppendErrorLog returned error: %v", err)
	}

	for _, path := range []string{legacyPath, RuntimeLogFilePath(logDir, now)} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(raw), `"event":"desktop.probe_event_emit_failed"`) {
			t.Fatalf("%s missing mirrored event: %s", path, string(raw))
		}
		if !strings.Contains(string(raw), `"schema_version":"cfst-log-v1"`) {
			t.Fatalf("%s missing schema version: %s", path, string(raw))
		}
	}
}

func TestNormalizeLogLevelStrictlyAllowsOnlyCanonicalValues(t *testing.T) {
	for _, level := range []string{LogLevelError, LogLevelWarn, LogLevelInfo, LogLevelDebug} {
		if got := NormalizeLogLevel(level); got != level {
			t.Fatalf("NormalizeLogLevel(%q) = %q, want same", level, got)
		}
	}
	for _, level := range []string{"warning", "trace", ""} {
		if got := NormalizeLogLevel(level); got != LogLevelError {
			t.Fatalf("NormalizeLogLevel(%q) = %q, want error", level, got)
		}
	}
}
