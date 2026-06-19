package app

import (
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func TestAttachManualUploadNotificationAddsCommandData(t *testing.T) {
	app := NewApp()
	taskID := "manual-task"
	result := desktopCommandResult("DNS_PUSH_COMPLETED", map[string]any{
		"upload_count": 2,
	}, "Cloudflare 推送完成。", true, &taskID, nil)

	got := app.attachManualUploadNotification(map[string]any{
		"config":               map[string]any{},
		"notification_trigger": appcore.UploadNotificationSourceManualPush,
	}, appcore.UploadNotificationProviderCloudflare, result)

	data := mapValue(got.Data)
	notification, ok := data["upload_notification"].(appcore.UploadNotification)
	if !ok {
		t.Fatalf("upload_notification = %#v, want appcore.UploadNotification", data["upload_notification"])
	}
	if notification.Source != appcore.UploadNotificationSourceManualPush {
		t.Fatalf("upload_notification source = %q", notification.Source)
	}
	if notification.CloudflareStatus != appcore.UploadNotificationStatusCompleted {
		t.Fatalf("cloudflare_status = %q", notification.CloudflareStatus)
	}
	if notification.CloudflareUploadCount != 2 {
		t.Fatalf("cloudflare_upload_count = %d", notification.CloudflareUploadCount)
	}
}

func TestPostProbeUploadNotificationSkipsUnavailableProvider(t *testing.T) {
	app := NewApp()
	result := app.runPostProbePushForSnapshot(map[string]any{
		"post_probe_push": map[string]any{
			"cloudflare_enabled": true,
		},
	}, ProbeRunResult{
		Results: []probecore.ProbeRow{{IP: "1.1.1.1"}},
	}, "probe-task")

	if result.Notification == nil {
		t.Fatal("Notification = nil, want skipped upload notification")
	}
	if result.Notification.CloudflareStatus != appcore.UploadNotificationStatusSkipped {
		t.Fatalf("cloudflare_status = %q, want skipped", result.Notification.CloudflareStatus)
	}
	if result.Notification.Status != appcore.UploadNotificationStatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Notification.Status)
	}
}

func TestSchedulerUploadNotificationUsesUploadMessage(t *testing.T) {
	app := NewApp()
	app.setSchedulerStatus(func(status *SchedulerStatus) {
		status.LastDNSStatus = appcore.UploadNotificationStatusCompleted
		status.CloudflareUploadCount = 2
		status.LastMessage = "定时测速完成，结果 2 条。"
	})

	app.recordSchedulerUploadNotification(map[string]any{}, appcore.UploadNotificationSourceScheduledProbe, "scheduled-task", true, false)

	status := app.currentSchedulerStatus()
	if status.UploadNotification == nil {
		t.Fatal("UploadNotification = nil")
	}
	if strings.Contains(status.UploadNotification.Message, "测速") {
		t.Fatalf("message = %q, want upload-only message", status.UploadNotification.Message)
	}
	if status.UploadNotification.Message != "上传流程已完成。" {
		t.Fatalf("message = %q, want default upload message", status.UploadNotification.Message)
	}
}
