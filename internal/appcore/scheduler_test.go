package appcore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNextSchedulerRunUsesEarliestValidRule(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 5, 9, 10, 30, 0, 0, location)
	got := NextSchedulerRun(now, time.Time{}, SchedulerTimingConfig{Enabled: true, IntervalMinutes: 120, DailyTimes: []string{"10：45"}})
	want := time.Date(2026, 5, 9, 10, 45, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("NextSchedulerRun() = %v, want %v", got, want)
	}
}

func TestSchedulerConfigFromSnapshotUsesSharedSupersetAndLegacyFieldNames(t *testing.T) {
	config := SchedulerConfigFromSnapshot(map[string]any{
		"scheduler": map[string]any{
			"enabled":                    true,
			"intervalMinutes":            30,
			"daily_times":                "08:00，20:30",
			"autoDnsPush":                false,
			"configSource":               "draft_preferred",
			"postRunSourceProfileAction": "update_recent_run_source_profile",
		},
	}, SchedulerConfig{AutoDNSPush: true, AutoGitHubExport: true, SkipIfActive: true, ConfigSource: "saved"})
	if !config.Enabled || config.IntervalMinutes != 30 || len(config.DailyTimes) != 2 {
		t.Fatalf("timing config = %#v", config)
	}
	if config.AutoDNSPush || !config.AutoGitHubExport || !config.SkipIfActive {
		t.Fatalf("workflow defaults = %#v", config)
	}
	if config.ConfigSource != "draft_preferred" || config.PostRunSourceProfileAction != "update_recent_run_source_profile" {
		t.Fatalf("capability superset = %#v", config)
	}
}

func TestApplySchedulerSourceProfileActionCreatesUpdatesAndSkips(t *testing.T) {
	now := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)
	store, status, changed := ApplySchedulerSourceProfileAction(SourceProfileStore{}, []Source{{ID: "source-1"}}, SchedulerSourceProfileActionUpdate, now)
	if status != "created" || !changed || len(store.Items) != 1 || store.Items[0].ID != RecentRunSourceProfileID {
		t.Fatalf("create = %#v, %q, %v", store, status, changed)
	}
	store, status, changed = ApplySchedulerSourceProfileAction(store, []Source{{ID: "source-2"}}, SchedulerSourceProfileActionUpdate, now.Add(time.Minute))
	if status != "updated" || !changed || len(store.Items) != 1 || store.Items[0].Sources[0].ID != "source-2" {
		t.Fatalf("update = %#v, %q, %v", store, status, changed)
	}
	unchanged, status, changed := ApplySchedulerSourceProfileAction(store, nil, "none", now)
	if status != "skipped" || changed || unchanged.Items[0].Sources[0].ID != "source-2" {
		t.Fatalf("skip = %#v, %q, %v", unchanged, status, changed)
	}
}

func TestServiceSelectSchedulerConfigSupportsDraftPreferredAndPayload(t *testing.T) {
	root := t.TempDir()
	layout := StorageLayout{Root: root, ConfigFileName: "config.json", DraftFileName: "draft.json"}
	service := NewService(ServiceOptions{Storage: layout})
	if err := os.MkdirAll(filepath.Dir(layout.ConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(layout.ConfigPath(), []byte(`{"saved_at":"2026-08-08T10:00:00Z","config_snapshot":{"source":"saved"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(layout.DraftPath(), []byte(`{"saved_at":"2026-08-08T10:00:01Z","config_snapshot":{"source":"draft"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	selection, err := service.SelectSchedulerConfig("draft_preferred", nil)
	if err != nil || selection.Source != "draft" || selection.Snapshot["source"] != "draft" {
		t.Fatalf("draft selection = %#v, %v", selection, err)
	}
	selection, err = service.SelectSchedulerConfig("payload", map[string]any{"source": "payload"})
	if err != nil || selection.Source != "payload" || selection.Snapshot["source"] != "payload" {
		t.Fatalf("payload selection = %#v, %v", selection, err)
	}
}

func TestServiceSchedulerStatusRepositoryReadsLegacyAndWritesSuperset(t *testing.T) {
	layout := StorageLayout{Root: t.TempDir(), SchedulerFileName: "scheduler-status.json"}
	service := NewService(ServiceOptions{Storage: layout})
	legacy := []byte(`{"enabled":true,"last_task_id":"legacy-task","config_source":"saved"}`)
	if err := WriteFileAtomic(layout.SchedulerPath(), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	status, existed, err := service.LoadSchedulerStatus()
	if err != nil || !existed || status.LastTaskID != "legacy-task" || status.LastSourceProfileAction != "" {
		t.Fatalf("legacy status = %#v, %v, %v", status, existed, err)
	}
	status.LastSourceProfileAction = "updated"
	if err := service.SaveSchedulerStatus(status); err != nil {
		t.Fatal(err)
	}
	reloaded, existed, err := service.LoadSchedulerStatus()
	if err != nil || !existed || reloaded.LastSourceProfileAction != "updated" {
		t.Fatalf("reloaded status = %#v, %v, %v", reloaded, existed, err)
	}
}
