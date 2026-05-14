package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/utils"
)

func TestNextSchedulerRun(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 5, 9, 10, 30, 0, 0, location)

	tests := []struct {
		name    string
		cfg     SchedulerConfig
		lastRun time.Time
		want    time.Time
	}{
		{
			name: "disabled",
			cfg:  SchedulerConfig{Enabled: false, IntervalMinutes: 30},
		},
		{
			name: "no rules",
			cfg:  SchedulerConfig{Enabled: true},
		},
		{
			name: "interval without last run",
			cfg:  SchedulerConfig{Enabled: true, IntervalMinutes: 30},
			want: now.Add(30 * time.Minute),
		},
		{
			name:    "interval advances from last run",
			cfg:     SchedulerConfig{Enabled: true, IntervalMinutes: 30},
			lastRun: now.Add(-75 * time.Minute),
			want:    time.Date(2026, 5, 9, 10, 45, 0, 0, location),
		},
		{
			name: "daily future today",
			cfg:  SchedulerConfig{Enabled: true, DailyTimes: []string{"11:15"}},
			want: time.Date(2026, 5, 9, 11, 15, 0, 0, location),
		},
		{
			name: "daily rolls to next day",
			cfg:  SchedulerConfig{Enabled: true, DailyTimes: []string{"09:00"}},
			want: time.Date(2026, 5, 10, 9, 0, 0, 0, location),
		},
		{
			name: "earliest interval or daily",
			cfg:  SchedulerConfig{Enabled: true, IntervalMinutes: 120, DailyTimes: []string{"10:45"}},
			want: time.Date(2026, 5, 9, 10, 45, 0, 0, location),
		},
		{
			name: "invalid daily time ignored",
			cfg:  SchedulerConfig{Enabled: true, DailyTimes: []string{"bad", "25:00", "10:31:05"}},
			want: time.Date(2026, 5, 9, 10, 31, 5, 0, location),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nextSchedulerRun(now, tc.lastRun, tc.cfg)
			if !got.Equal(tc.want) {
				t.Fatalf("nextSchedulerRun() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRunScheduledProbeSkipsWhenActive(t *testing.T) {
	app := NewApp()
	if ok, _ := app.setCurrentProbeTask("manual-task", nil); !ok {
		t.Fatal("setCurrentProbeTask returned false")
	}
	defer app.clearCurrentProbeTask("manual-task")

	app.runScheduledProbe(context.Background(), SchedulerConfig{
		Enabled:      true,
		SkipIfActive: true,
	})

	status := app.currentSchedulerStatus()
	if status.LastProbeStatus != "skipped" {
		t.Fatalf("LastProbeStatus = %q, want skipped", status.LastProbeStatus)
	}
	if status.LastTaskID == "" || status.LastRunAt == "" {
		t.Fatalf("scheduler status missing task/run metadata: %#v", status)
	}
	if status.LastDNSStatus != "" || status.LastGitHubStatus != "" {
		t.Fatalf("downstream statuses = (%q,%q), want empty", status.LastDNSStatus, status.LastGitHubStatus)
	}
}

func TestSchedulerSnapshotForRunPrefersNewerDraft(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	savedSnapshot := defaultDesktopConfigSnapshot()
	mapValue(savedSnapshot["cloudflare"])["record_name"] = "saved.example.com"
	if result := app.SaveDesktopConfig(map[string]any{"config_snapshot": savedSnapshot}); !result.OK {
		t.Fatalf("SaveDesktopConfig failed: %#v", result)
	}

	time.Sleep(1100 * time.Millisecond)
	draftSnapshot := defaultDesktopConfigSnapshot()
	mapValue(draftSnapshot["cloudflare"])["record_name"] = "draft.example.com"
	if result := app.SaveDesktopDraft(map[string]any{"config_snapshot": draftSnapshot}); !result.OK {
		t.Fatalf("SaveDesktopDraft failed: %#v", result)
	}

	got, source, err := schedulerSnapshotForRun(SchedulerConfig{ConfigSource: defaultSchedulerConfigSource})
	if err != nil {
		t.Fatalf("schedulerSnapshotForRun returned error: %v", err)
	}
	if source != "draft" {
		t.Fatalf("source = %q, want draft", source)
	}
	if gotName := stringValue(mapValue(got["cloudflare"])["record_name"], ""); gotName != "draft.example.com" {
		t.Fatalf("record_name = %q, want draft snapshot", gotName)
	}
}

func TestSchedulerSnapshotForRunFallsBackToSavedConfig(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	savedSnapshot := defaultDesktopConfigSnapshot()
	mapValue(savedSnapshot["cloudflare"])["record_name"] = "saved-only.example.com"
	if result := app.SaveDesktopConfig(map[string]any{"config_snapshot": savedSnapshot}); !result.OK {
		t.Fatalf("SaveDesktopConfig failed: %#v", result)
	}

	got, source, err := schedulerSnapshotForRun(SchedulerConfig{ConfigSource: defaultSchedulerConfigSource})
	if err != nil {
		t.Fatalf("schedulerSnapshotForRun returned error: %v", err)
	}
	if source != "saved" {
		t.Fatalf("source = %q, want saved", source)
	}
	if gotName := stringValue(mapValue(got["cloudflare"])["record_name"], ""); gotName != "saved-only.example.com" {
		t.Fatalf("record_name = %q, want saved snapshot", gotName)
	}
}

func TestRunScheduledProbePassesConfigSourceToTaskContext(t *testing.T) {
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
	})
	desktopTCPProbeRunner = func() utils.PingDelaySet {
		return utils.PingDelaySet{{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
		}}
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	for _, tc := range []struct {
		name       string
		withDraft  bool
		wantSource string
	}{
		{name: "saved fallback", wantSource: "saved"},
		{name: "newer draft", withDraft: true, wantSource: "draft"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateStorageForTest(t)
			app := NewApp()
			savedSnapshot := defaultDesktopConfigSnapshot()
			mapValue(savedSnapshot["probe"])["disable_download"] = true
			savedSnapshot["sources"] = []any{
				map[string]any{
					"content": "1.1.1.1",
					"enabled": true,
					"id":      "scheduled-source",
					"kind":    "inline",
					"name":    "Scheduled Source",
				},
			}
			if result := app.SaveDesktopConfig(map[string]any{"config_snapshot": savedSnapshot}); !result.OK {
				t.Fatalf("SaveDesktopConfig failed: %#v", result)
			}
			if tc.withDraft {
				time.Sleep(1100 * time.Millisecond)
				draftSnapshot := defaultDesktopConfigSnapshot()
				mapValue(draftSnapshot["probe"])["disable_download"] = true
				draftSnapshot["sources"] = savedSnapshot["sources"]
				mapValue(draftSnapshot["cloudflare"])["record_name"] = "draft.example.com"
				if result := app.SaveDesktopDraft(map[string]any{"config_snapshot": draftSnapshot}); !result.OK {
					t.Fatalf("SaveDesktopDraft failed: %#v", result)
				}
			}

			app.runScheduledProbe(context.Background(), SchedulerConfig{
				AutoDNSPush:      false,
				AutoGitHubExport: false,
				ConfigSource:     defaultSchedulerConfigSource,
			})
			status := app.currentSchedulerStatus()
			if status.LastProbeStatus != "completed" {
				t.Fatalf("LastProbeStatus = %q, want completed; message=%s", status.LastProbeStatus, status.LastMessage)
			}
			snapshot, _, err := schedulerSnapshotForRun(SchedulerConfig{ConfigSource: defaultSchedulerConfigSource})
			if err != nil {
				t.Fatalf("schedulerSnapshotForRun returned error: %v", err)
			}
			probeResult, err := app.RunDesktopProbe(DesktopProbePayload{
				Config:       snapshot,
				ConfigSource: status.ConfigSource,
				Sources:      desktopSourcesFromAny(snapshot["sources"]),
				TaskID:       "direct-" + tc.name,
			})
			if err != nil {
				t.Fatalf("RunDesktopProbe returned error: %v", err)
			}
			if got := probeResult.TaskContext.ConfigSource; got != tc.wantSource {
				t.Fatalf("task_context.config_source = %q, want %q; result=%#v", got, tc.wantSource, probeResult.TaskContext)
			}
			if strings.TrimSpace(status.ConfigSource) != tc.wantSource {
				t.Fatalf("scheduler status config source = %q, want %q", status.ConfigSource, tc.wantSource)
			}
		})
	}
}

func TestSchedulerRecentRunProfilesUseFixedIDs(t *testing.T) {
	isolateStorageForTest(t)
	snapshot := defaultDesktopConfigSnapshot()
	mapValue(snapshot["cloudflare"])["record_name"] = "recent.example.com"
	sources := []DesktopSource{sourceProfileTestSource("recent-source", "Recent Source")}

	if action := updateRecentRunProfile(snapshot); action != "created" {
		t.Fatalf("profile action = %q, want created", action)
	}
	if action := updateRecentRunSourceProfile(sources); action != "created" {
		t.Fatalf("source profile action = %q, want created", action)
	}

	profiles, err := loadProfileStore()
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	if len(profiles.Items) != 1 || profiles.Items[0].ID != recentRunProfileID {
		t.Fatalf("profiles = %#v, want fixed recent-run profile", profiles.Items)
	}
	if got := stringValue(mapValue(profiles.Items[0].ConfigSnapshot["cloudflare"])["record_name"], ""); got != "recent.example.com" {
		t.Fatalf("recent profile record_name = %q, want snapshot saved", got)
	}

	sourceProfiles, err := loadSourceProfileStore()
	if err != nil {
		t.Fatalf("load source profiles: %v", err)
	}
	if len(sourceProfiles.Items) != 1 || sourceProfiles.Items[0].ID != recentRunSourceProfileID {
		t.Fatalf("source profiles = %#v, want fixed recent-run source profile", sourceProfiles.Items)
	}
	if len(sourceProfiles.Items[0].Sources) != 1 || sourceProfiles.Items[0].Sources[0].Name != "Recent Source" {
		t.Fatalf("recent source profile sources = %#v, want saved sources", sourceProfiles.Items[0].Sources)
	}

	mapValue(snapshot["cloudflare"])["record_name"] = "recent-updated.example.com"
	if action := updateRecentRunProfile(snapshot); action != "updated" {
		t.Fatalf("profile action = %q, want updated", action)
	}
	if action := updateRecentRunSourceProfile([]DesktopSource{sourceProfileTestSource("recent-source-2", "Recent Source 2")}); action != "updated" {
		t.Fatalf("source profile action = %q, want updated", action)
	}
}

func TestGitHubExportEnabledFromSnapshot(t *testing.T) {
	if githubExportEnabledFromSnapshot(map[string]any{}) {
		t.Fatal("empty snapshot should not enable GitHub export")
	}
	if !githubExportEnabledFromSnapshot(map[string]any{
		"export": map[string]any{
			"github": map[string]any{
				"enabled": true,
			},
		},
	}) {
		t.Fatal("export.github.enabled=true should enable GitHub export")
	}
	if !githubExportEnabledFromSnapshot(map[string]any{
		"github": map[string]any{
			"enabled": "true",
		},
	}) {
		t.Fatal("legacy github.enabled=true should enable GitHub export")
	}
}
