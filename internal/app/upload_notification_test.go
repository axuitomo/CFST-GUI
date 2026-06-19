package app

import (
	"net/http"
	"net/http/httptest"
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

func TestAttachManualUploadNotificationAddsTopEntries(t *testing.T) {
	app := NewApp()
	taskID := "manual-task"
	result := desktopCommandResult("DNS_PUSH_COMPLETED", map[string]any{
		"upload_count": 1,
	}, "Cloudflare 推送完成。", true, &taskID, nil)

	got := app.attachManualUploadNotification(map[string]any{
		"config": map[string]any{
			"notifications": map[string]any{
				"telegram": map[string]any{
					"include_top_n": true,
					"top_n":         1,
				},
			},
		},
		"notification_trigger": appcore.UploadNotificationSourceManualPush,
		"results": []probecore.ProbeRow{
			{IP: "1.1.1.1", DelayMS: 100, DownloadSpeedMB: 1},
			{IP: "1.1.1.2", DelayMS: 10, DownloadSpeedMB: 100},
		},
	}, appcore.UploadNotificationProviderCloudflare, result)

	notification := mapValue(got.Data)["upload_notification"].(appcore.UploadNotification)
	if len(notification.TopEntries) != 1 {
		t.Fatalf("TopEntries = %#v, want one entry", notification.TopEntries)
	}
	if notification.TopEntries[0].IP != "1.1.1.2" {
		t.Fatalf("top entry IP = %q, want fastest row", notification.TopEntries[0].IP)
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

func TestPostProbeUploadNotificationIncludesTopEntries(t *testing.T) {
	app := NewApp()
	result := app.runPostProbePushForSnapshot(map[string]any{
		"notifications": map[string]any{
			"telegram": map[string]any{
				"include_top_n": true,
				"top_n":         1,
			},
		},
		"post_probe_push": map[string]any{
			"cloudflare_enabled": true,
		},
	}, ProbeRunResult{
		Results: []probecore.ProbeRow{
			{IP: "1.1.1.1", DelayMS: 100, DownloadSpeedMB: 1},
			{IP: "1.1.1.2", DelayMS: 10, DownloadSpeedMB: 100},
		},
	}, "probe-task")

	if result.Notification == nil {
		t.Fatal("Notification = nil")
	}
	if len(result.Notification.TopEntries) != 1 || result.Notification.TopEntries[0].IP != "1.1.1.2" {
		t.Fatalf("TopEntries = %#v, want fastest row", result.Notification.TopEntries)
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

func TestTestTelegramNotificationFailsWhenUploadRecipientMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	previousBaseURL := appcore.TelegramAPIBaseURL
	_ = previousBaseURL
	app := NewApp()
	result := app.TestTelegramNotification(map[string]any{
		"config": map[string]any{
			"notifications": map[string]any{
				"telegram": map[string]any{
					"bot_token":             "token:secret",
					"chat_id":               "group-chat",
					"enabled":               false,
					"include_top_n":         true,
					"top_n_recipient_mode":  "chat",
					"upload_recipient_mode": "personal",
				},
			},
		},
	})
	if result.OK {
		t.Fatalf("result = %#v, want failure when upload recipient target is missing", result)
	}
	if !strings.Contains(result.Message, "Telegram 通知目标配置不完整") {
		t.Fatalf("message = %q, want missing target error", result.Message)
	}
}
