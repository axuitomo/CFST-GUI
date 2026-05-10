package main

import (
	"context"
	"testing"
	"time"
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
