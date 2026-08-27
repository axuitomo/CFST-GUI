package appcore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteConfigSnapshotV2BacksUpLegacyOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-config.json")
	legacy := []byte(`{"schema_version":"cfst-gui-wails-v1","config_snapshot":{"probe":{"tcp_port":443}}}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfigSnapshotV2(path, map[string]any{"probe": map[string]any{"tcp_port": 8443}}, func(value map[string]any) map[string]any { return value }); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".v1.bak")
	if err != nil || string(backup) != string(legacy) {
		t.Fatalf("backup = %q, %v", backup, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["schema_version"] != ConfigSchemaVersion {
		t.Fatalf("schema_version = %#v", envelope["schema_version"])
	}
}

func TestServiceConfigRepositoryReadsLegacyAndWritesV2(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "desktop-config.json")
	if err := os.WriteFile(path, []byte(`{"probe":{"tcp_port":2053}} trailing`), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(ServiceOptions{
		Storage: StorageLayout{Root: root, ConfigFileName: "desktop-config.json"},
		ConfigPolicy: ConfigPolicy{
			DefaultSnapshot:  func() map[string]any { return map[string]any{"probe": map[string]any{"tcp_port": 443}} },
			SanitizeSnapshot: func(snapshot map[string]any) map[string]any { return snapshot },
		},
	})
	loaded, err := service.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Existed || !loaded.CompatInfo.IgnoredTrailingContent {
		t.Fatalf("LoadConfig = %#v", loaded)
	}
	if _, err := service.SaveConfig(loaded.Snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".v1.bak"); err != nil {
		t.Fatalf("legacy backup: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if _, err := UnmarshalJSONCompat(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["schema_version"] != ConfigSchemaVersion {
		t.Fatalf("schema_version = %v", envelope["schema_version"])
	}
}

func TestNewCommandResultUsesSharedSchemaAndDedupesWarnings(t *testing.T) {
	result := NewCommandResult("READY", nil, "ready", true, nil, []string{"a", "a", "b"})
	if result.SchemaVersion != CommandSchemaVersion {
		t.Fatalf("schema = %q, want %q", result.SchemaVersion, CommandSchemaVersion)
	}
	if len(result.Warnings) != 2 || result.Warnings[0] != "a" || result.Warnings[1] != "b" {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
}

func TestServiceEmitFillsSharedEventEnvelope(t *testing.T) {
	now := time.Date(2026, time.August, 8, 10, 30, 0, 0, time.UTC)
	var received ProbeEvent
	service := NewService(ServiceOptions{
		Clock: func() time.Time { return now },
		EventSink: EventSinkFunc(func(_ context.Context, event ProbeEvent) {
			received = event
		}),
	})
	service.Emit(context.Background(), ProbeEvent{Event: "probe.progress", TaskID: "task-1"})
	if received.SchemaVersion != EventSchemaVersion || received.TS != now.Format(time.RFC3339) {
		t.Fatalf("event = %#v", received)
	}
}

func TestServiceSequencesEventsPerTaskAndLocksTerminalLifecycle(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	events := make([]ProbeEvent, 0, 4)
	service.SetEventSink(EventSinkFunc(func(_ context.Context, event ProbeEvent) {
		events = append(events, event)
	}))
	for _, event := range []ProbeEvent{
		{Event: "probe.progress", TaskID: "task-a"},
		{Event: "probe.progress", TaskID: "task-b"},
		{Event: "probe.cancelled", TaskID: "task-a"},
		{Event: "probe.failed", TaskID: "task-a"},
	} {
		if err := service.PublishProbeEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(events) != 3 {
		t.Fatalf("events = %#v, want duplicate terminal suppressed", events)
	}
	if events[0].Seq != 1 || events[1].Seq != 1 || events[2].Seq != 2 {
		t.Fatalf("per-task sequences = %#v", events)
	}
	snapshot, ok, err := service.LoadTaskSnapshot("task-a")
	if err != nil || !ok || snapshot.Status != "cancelled" {
		t.Fatalf("terminal snapshot = %#v, %v, %v", snapshot, ok, err)
	}
}

func TestStorageLayoutKeepsPlatformConfigName(t *testing.T) {
	layout := StorageLayout{
		Root:               filepath.Join("root", "app"),
		ConfigFileName:     "mobile-config.json",
		DraftFileName:      "draft.json",
		SchedulerFileName:  "scheduler-status.json",
		SourceProfilesFile: "source-profiles.json",
	}
	if got, want := layout.ConfigPath(), filepath.Join("root", "app", "mobile-config.json"); got != want {
		t.Fatalf("ConfigPath() = %q, want %q", got, want)
	}
	if got, want := layout.TasksRoot(), filepath.Join("root", "app", "tasks"); got != want {
		t.Fatalf("TasksRoot() = %q, want %q", got, want)
	}
}

func TestServicesOwnIndependentDebugLoggers(t *testing.T) {
	first := NewService(ServiceOptions{})
	second := NewService(ServiceOptions{})
	if first.DebugLogger() == nil || second.DebugLogger() == nil || first.DebugLogger() == second.DebugLogger() {
		t.Fatalf("service debug loggers are not isolated: %p %p", first.DebugLogger(), second.DebugLogger())
	}
}

func TestServiceDraftCommandsUseSharedSchemaAndInjectedClock(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, time.August, 8, 10, 30, 0, 0, time.UTC)
	service := NewService(ServiceOptions{
		Clock: func() time.Time { return now },
		Storage: StorageLayout{
			Root:           root,
			ConfigFileName: "config.json",
			DraftFileName:  "draft.json",
		},
		ConfigPolicy: ConfigPolicy{
			DefaultSnapshot:  func() map[string]any { return map[string]any{"probe": map[string]any{"tcp_port": 443}} },
			SanitizeSnapshot: func(snapshot map[string]any) map[string]any { return snapshot },
		},
	})

	saved := service.Invoke("draft.save", `{"config_snapshot":{"probe":{"tcp_port":8443}}}`)
	if !saved.OK || saved.Code != "DRAFT_SAVE_OK" {
		t.Fatalf("draft.save = %#v", saved)
	}
	status, ok := saved.Data.(map[string]any)
	if !ok || status["saved_at"] != now.Format(time.RFC3339) {
		t.Fatalf("draft status = %#v", saved.Data)
	}
	if saved.SchemaVersion != CommandSchemaVersion {
		t.Fatalf("schema = %q", saved.SchemaVersion)
	}

	loaded := service.Invoke("draft.load", `{}`)
	if !loaded.OK || loaded.Code != "DRAFT_READY" {
		t.Fatalf("draft.load = %#v", loaded)
	}

	discarded := service.Invoke("draft.discard", `{}`)
	if !discarded.OK || discarded.Code != "DRAFT_DISCARDED" {
		t.Fatalf("draft.discard = %#v", discarded)
	}
	status = discarded.Data.(map[string]any)
	if status["exists"] != false {
		t.Fatalf("discarded status = %#v", status)
	}
}
