package mobileapi

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestRunScheduledProbeFailsWhenUploadSelectionFails(t *testing.T) {
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	oldDownload := mobileDownloadProbeRunner
	t.Cleanup(func() {
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
		mobileDownloadProbeRunner = oldDownload
	})

	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return utils.PingDelaySet{{
			PingData: &utils.PingData{
				IP:       parseMobileTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		}}, nil
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet { return input }
	mobileDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	probe := mapValue(snapshot["probe"])
	probe["disable_download"] = true
	probe["print_num"] = 0
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["auto_dns_push"] = false
	scheduler["auto_github_export"] = false
	scheduler["enabled"] = true
	scheduler["interval_minutes"] = 15
	sharedFilter := mapValue(mapValue(snapshot["upload"])["shared_filter"])
	sharedFilter["colo_allow"] = "JP"
	sharedFilter["enabled"] = true
	sources := []map[string]any{{
		"content":  "1.1.1.1",
		"enabled":  true,
		"ip_limit": 10,
		"ip_mode":  "traverse",
		"kind":     "inline",
		"name":     "valid-source",
	}}
	snapshot["sources"] = sources
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if boolValue(result["ok"], true) {
		t.Fatalf("RunScheduledProbe unexpectedly succeeded: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "SCHEDULER_RUN_FAILED" {
		t.Fatalf("code = %q, want SCHEDULER_RUN_FAILED", got)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["run_stage"], ""); got != "upload_selection_failed" {
		t.Fatalf("run_stage = %q, want upload_selection_failed", got)
	}
	if got := stringValue(data["last_probe_status"], ""); got != "failed" {
		t.Fatalf("last_probe_status = %q, want failed", got)
	}
	if got := stringValue(data["next_run_at"], ""); got == "" {
		t.Fatal("next_run_at is empty, want scheduler to rearm after failure")
	}
	if message := stringValue(result["message"], ""); !strings.Contains(message, "COLO 文件不存在") {
		t.Fatalf("message = %q, want missing COLO dictionary error", message)
	}
}

func TestRunScheduledProbeSkipActiveRearmsFutureRun(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["enabled"] = true
	scheduler["interval_minutes"] = 15
	scheduler["skip_if_active"] = false
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}
	if err := service.writeSchedulerStatus(mobileSchedulerStatus{
		Enabled:   true,
		NextRunAt: time.Now().Add(-time.Minute).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("writeSchedulerStatus: %v", err)
	}
	service.stateMu.Lock()
	service.currentTaskID = "manual-task"
	service.stateMu.Unlock()

	start := time.Now()
	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunScheduledProbe failed: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "SCHEDULER_RUN_SKIPPED" {
		t.Fatalf("code = %q, want SCHEDULER_RUN_SKIPPED", got)
	}
	data := mapValue(result["data"])
	next := parseMobileSchedulerTime(stringValue(data["next_run_at"], ""))
	if next.IsZero() || !next.After(start) {
		t.Fatalf("next_run_at = %q, want future rearmed time after skip", data["next_run_at"])
	}
	if got := stringValue(data["last_probe_status"], ""); got != "skipped" {
		t.Fatalf("last_probe_status = %q, want skipped", got)
	}
	entries := readMobileLogEntries(t, service.errorLogPath())
	if len(entries) != 2 {
		t.Fatalf("error log entries = %d, want scheduler skip and lifecycle audit: %#v", len(entries), entries)
	}
	skipEntry := findMobileLogEntryByEvent(t, entries, "scheduler.probe_skipped_active")
	if got := stringValue(skipEntry["event"], ""); got != "scheduler.probe_skipped_active" {
		t.Fatalf("error event = %q, want scheduler.probe_skipped_active", got)
	}
	if got := stringValue(skipEntry["task_id"], ""); got != stringValue(data["last_task_id"], "") {
		t.Fatalf("error task_id = %q, want %q", got, data["last_task_id"])
	}
	auditEntry := findMobileLogEntryByEvent(t, entries, "task.lifecycle.audit")
	auditData := mapValue(auditEntry["data"])
	if got := stringValue(auditData["terminal_event"], ""); got != "scheduler.probe_skipped_active" {
		t.Fatalf("audit terminal_event = %q, want scheduler.probe_skipped_active: %#v", got, auditEntry)
	}
	if got := stringValue(auditData["snapshot_status"], ""); got != "skipped" {
		t.Fatalf("audit snapshot_status = %q, want skipped: %#v", got, auditEntry)
	}
}

func TestLogMobileStageRejectWritesErrorLogFields(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	service.logMobileStageReject("task-1", task.StageRejectEvent{
		Error:   "trace failed",
		IP:      "1.1.1.1",
		Message: "追踪请求失败，淘汰该 IP。",
		Reason:  "trace_error",
		Stage:   "stage2_trace",
	})

	entries := readMobileLogEntries(t, service.errorLogPath())
	if len(entries) != 1 {
		t.Fatalf("error log entries = %d, want 1: %#v", len(entries), entries)
	}
	entry := entries[0]
	if got := stringValue(entry["event"], ""); got != "stage.reject" {
		t.Fatalf("event = %q, want stage.reject", got)
	}
	for key, want := range map[string]string{
		"error":   "trace failed",
		"ip":      "1.1.1.1",
		"message": "追踪请求失败，淘汰该 IP。",
		"reason":  "trace_error",
		"stage":   "stage2_trace",
		"task_id": "task-1",
	} {
		if got := mobileLogEntryString(entry, key); got != want {
			t.Fatalf("%s = %q, want %q in %#v", key, got, want, entry)
		}
	}
}

func TestRunScheduledProbeLoadConfigFailureClearsStaleNextRun(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	if err := service.writeSchedulerStatus(mobileSchedulerStatus{
		Enabled:   true,
		NextRunAt: time.Now().Add(-time.Minute).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("writeSchedulerStatus: %v", err)
	}
	if err := os.WriteFile(service.configPath(), []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if boolValue(result["ok"], true) {
		t.Fatalf("RunScheduledProbe unexpectedly succeeded: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "SCHEDULER_RUN_FAILED" {
		t.Fatalf("code = %q, want SCHEDULER_RUN_FAILED", got)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["next_run_at"], ""); got != "" {
		t.Fatalf("next_run_at = %q, want cleared after config load failure", got)
	}
	if got := stringValue(data["run_stage"], ""); got != "load_config_failed" {
		t.Fatalf("run_stage = %q, want load_config_failed", got)
	}
}

func TestRunScheduledProbeDisabledClearsStaleNextRun(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["enabled"] = false
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}
	if err := service.writeSchedulerStatus(mobileSchedulerStatus{
		Enabled:   true,
		NextRunAt: time.Now().Add(-time.Minute).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("writeSchedulerStatus: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunScheduledProbe failed: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "SCHEDULER_RUN_SKIPPED" {
		t.Fatalf("code = %q, want SCHEDULER_RUN_SKIPPED", got)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["next_run_at"], ""); got != "" {
		t.Fatalf("next_run_at = %q, want cleared when scheduler is disabled", got)
	}
	if boolValue(data["enabled"], true) {
		t.Fatal("enabled = true, want disabled scheduler status")
	}
}

func readMobileLogEntries(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line is not JSON: %v\n%s", err, line)
		}
		entries = append(entries, entry)
	}
	return entries
}

func mobileLogEntryString(entry map[string]any, key string) string {
	if value := stringValue(entry[key], ""); value != "" {
		return value
	}
	return stringValue(mapValue(entry["data"])[key], "")
}
