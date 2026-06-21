package mobileapi

import (
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func TestMobileAttachManualUploadNotificationAddsTopEntries(t *testing.T) {
	service := NewService()
	taskID := "manual-task"
	response := encodeCommand(commandResultFor("DNS_PUSH_COMPLETED", map[string]any{
		"upload_count": 1,
	}, "Cloudflare 推送完成。", true, &taskID, nil))

	got := decodeCommandForTest(t, service.attachManualUploadNotification(map[string]any{
		"config": map[string]any{
			"notifications": map[string]any{
				"telegram": map[string]any{
					"include_top_n": true,
					"top_n":         1,
				},
			},
		},
		"notification_trigger": appcore.UploadNotificationSourceManualPush,
		"results": []probeRow{
			{IP: "1.1.1.1", DelayMS: 100, DownloadSpeedMB: 1},
			{IP: "1.1.1.2", DelayMS: 10, DownloadSpeedMB: 100},
		},
	}, appcore.UploadNotificationProviderCloudflare, response))

	data := mapValue(got["data"])
	notification := mapValue(data["upload_notification"])
	entries, ok := notification["top_entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("top_entries = %#v, want one entry", notification["top_entries"])
	}
	entry := mapValue(entries[0])
	if got := stringValue(entry["ip"], ""); got != "1.1.1.2" {
		t.Fatalf("top entry IP = %q, want fastest row", got)
	}
}

func TestMobilePostProbeUploadNotificationSkipsUnavailableProvider(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	result := service.runMobilePostProbePush(desktopProbePayload{
		Config: map[string]any{
			"post_probe_push": map[string]any{
				"cloudflare_enabled": true,
			},
		},
		TaskID: "probe-task",
	}, probeRunResult{
		Results: []probeRow{{IP: "1.1.1.1"}},
	})

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

func TestMobileTestTelegramNotificationFailsWhenUploadRecipientMissing(t *testing.T) {
	service := NewService()
	result := decodeCommandForTest(t, service.TestTelegramNotification(encodeJSON(map[string]any{
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
	})))
	if boolValue(result["ok"], true) {
		t.Fatalf("result = %#v, want failure when upload recipient target is missing", result)
	}
	if !strings.Contains(stringValue(result["message"], ""), "Telegram 通知目标配置不完整") {
		t.Fatalf("message = %q, want missing target error", stringValue(result["message"], ""))
	}
}
