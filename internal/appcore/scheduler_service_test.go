package appcore

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestServiceSchedulerRunSkipsActiveTaskAndClearsProgress(t *testing.T) {
	service := newSchedulerServiceForTest(t, func() (utils.PingDelaySet, error) { return nil, nil })
	snapshot := schedulerSnapshotForTest(true)
	if _, err := service.SaveConfig(snapshot); err != nil {
		t.Fatal(err)
	}
	if err := service.SaveSchedulerStatus(SchedulerStatus{
		UploadInputCount:        2,
		UploadFilteredCount:     2,
		CloudflareUploadCount:   2,
		GitHubUploadCount:       1,
		LastSourceProfileAction: "updated",
		UploadNotification:      &UploadNotification{Status: UploadNotificationStatusCompleted},
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := service.runtime.Start("manual-task"); !ok {
		t.Fatal("failed to start active task")
	}
	defer service.runtime.Clear("manual-task")

	result := service.RunScheduledProbe(context.Background(), SchedulerRunRequest{TaskID: "scheduled-task"})
	if !result.OK || result.Code != "SCHEDULER_RUN_SKIPPED" {
		t.Fatalf("scheduler result = %#v", result)
	}
	status := result.Data.(SchedulerStatus)
	if status.LastProbeStatus != "skipped" || status.WorkflowStage != "skipped" {
		t.Fatalf("status = %#v", status)
	}
	if status.UploadInputCount != 0 || status.UploadFilteredCount != 0 || status.CloudflareUploadCount != 0 || status.GitHubUploadCount != 0 || status.UploadNotification != nil || status.LastSourceProfileAction != "" {
		t.Fatalf("stale progress was not cleared: %#v", status)
	}
}

func TestServiceSchedulerRunCompletesCoreWorkflowAndNotification(t *testing.T) {
	service := newSchedulerServiceForTest(t, func() (utils.PingDelaySet, error) { return utils.PingDelaySet{}, nil })
	var events []ProbeEvent
	service.SetEventSink(EventSinkFunc(func(_ context.Context, event ProbeEvent) { events = append(events, event) }))
	if _, err := service.SaveConfig(schedulerSnapshotForTest(true)); err != nil {
		t.Fatal(err)
	}
	result := service.RunScheduledProbe(context.Background(), SchedulerRunRequest{TaskID: "scheduled-empty"})
	if !result.OK || result.Code != "SCHEDULER_RUN_COMPLETED" {
		t.Fatalf("scheduler result = %#v", result)
	}
	status := result.Data.(SchedulerStatus)
	if status.LastProbeStatus != "completed" || status.WorkflowStage != "completed" || status.LastSourceProfileAction != "created" {
		t.Fatalf("status = %#v", status)
	}
	if status.UploadNotification == nil || status.UploadNotification.Status != UploadNotificationStatusSkipped {
		t.Fatalf("notification = %#v", status.UploadNotification)
	}
	store, err := LoadSourceProfileStore(service.StorageLayout().SourceProfilesPath(), DefaultSourceProfilesSchemaVersion)
	if err != nil || len(store.Items) != 1 || store.ActiveProfileID != RecentRunSourceProfileID {
		t.Fatalf("source profiles = %#v, %v", store, err)
	}
	foundNotification := false
	for _, event := range events {
		if event.Event == "upload.notification" {
			foundNotification = true
		}
	}
	if !foundNotification {
		t.Fatalf("events = %#v, want upload.notification", events)
	}
}

func TestServiceSchedulerCommandsUsePersistedCoreStatus(t *testing.T) {
	service := newSchedulerServiceForTest(t, func() (utils.PingDelaySet, error) { return nil, nil })
	snapshot := schedulerSnapshotForTest(true)
	if _, err := service.SaveConfig(snapshot); err != nil {
		t.Fatal(err)
	}
	refresh := service.Invoke("scheduler.refresh", `{"config_snapshot":{"scheduler":{"enabled":true,"interval_minutes":15}}}`)
	if !refresh.OK || refresh.Code != "SCHEDULER_REFRESH_OK" {
		t.Fatalf("refresh = %#v", refresh)
	}
	status := refresh.Data.(SchedulerStatus)
	if status.NextRunAt == "" || !status.Enabled {
		t.Fatalf("refreshed status = %#v", status)
	}
	loaded := service.Invoke("scheduler.status", `{}`)
	if !loaded.OK || loaded.Code != "SCHEDULER_STATUS_READY" {
		t.Fatalf("status = %#v", loaded)
	}
	if loaded.Data.(SchedulerStatus).NextRunAt != status.NextRunAt {
		t.Fatalf("persisted status = %#v, refreshed = %#v", loaded.Data, status)
	}
}

func newSchedulerServiceForTest(t *testing.T, runTCP func() (utils.PingDelaySet, error)) *Service {
	t.Helper()
	root := t.TempDir()
	service := NewService(ServiceOptions{
		AppVersion: "test",
		Clock:      func() time.Time { return time.Date(2026, time.August, 9, 14, 0, 0, 0, time.UTC) },
		Storage:    StorageLayout{Root: root, ConfigFileName: "config.json", DraftFileName: "draft.json", SchedulerFileName: "scheduler.json", SourceProfilesFile: "profiles.json"},
		ConfigPolicy: ConfigPolicy{
			DefaultSnapshot:  func() map[string]any { return schedulerSnapshotForTest(false) },
			SanitizeSnapshot: func(snapshot map[string]any) map[string]any { return snapshot },
		},
		SchedulerDefaults:  SchedulerConfig{AutoDNSPush: true, AutoGitHubExport: true, SkipIfActive: true, ConfigSource: "saved", PostRunSourceProfileAction: SchedulerSourceProfileActionUpdate},
		ProbeConfigOptions: probecore.ConfigSnapshotOptions{IncludeSchedulerMetadata: true},
		ProbeHooks: ProbeExecutionHooks{
			RunTCP:   func(*task.Engine) (utils.PingDelaySet, error) { return runTCP() },
			RunTrace: func(_ *task.Engine, input utils.PingDelaySet) utils.PingDelaySet { return input },
			RunDownload: func(_ *task.Engine, input utils.PingDelaySet) utils.DownloadSpeedSet {
				return utils.DownloadSpeedSet(input)
			},
		},
	})
	return service
}

func schedulerSnapshotForTest(enabled bool) map[string]any {
	snapshot := probecore.DefaultConfigSnapshot(probecore.ConfigSnapshotOptions{IncludeSchedulerMetadata: true, SchedulerConfigSource: "saved", SchedulerSourceProfileAction: SchedulerSourceProfileActionUpdate})
	probe := mapValue(snapshot["probe"])
	probe["disable_trace"] = true
	probe["disable_download"] = true
	probe["print_num"] = 0
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["enabled"] = enabled
	scheduler["interval_minutes"] = 15
	scheduler["auto_dns_push"] = true
	scheduler["auto_github_export"] = true
	snapshot["sources"] = []map[string]any{{"content": "1.1.1.1", "enabled": true, "ip_limit": 10, "ip_mode": "traverse", "kind": "inline", "name": "valid-source"}}
	return snapshot
}

func schedulerPing(ip string, speed float64) utils.PingDelaySet {
	return utils.PingDelaySet{{PingData: &utils.PingData{IP: &net.IPAddr{IP: net.ParseIP(ip)}, Sended: 3, Received: 3, Delay: 10 * time.Millisecond}, DownloadSpeed: speed}}
}
