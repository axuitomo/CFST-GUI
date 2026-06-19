package appcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTelegramNotificationConfigFromSnapshot(t *testing.T) {
	cfg := TelegramNotificationConfigFromSnapshot(map[string]any{
		"notifications": map[string]any{
			"telegram": map[string]any{
				"enabled":   true,
				"bot_token": "  token:secret  ",
				"chat_id":   "  12345 ",
			},
		},
	})
	if !cfg.Enabled || cfg.BotToken != "token:secret" || cfg.ChatID != "12345" {
		t.Fatalf("config = %#v, want trimmed enabled Telegram config", cfg)
	}
}

func TestBuildUploadNotificationOverallStatus(t *testing.T) {
	createdAt := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	notification := BuildUploadNotification(UploadNotificationInput{
		Cloudflare: &UploadProviderReport{Status: UploadNotificationStatusCompleted, UploadCount: 3},
		CreatedAt:  createdAt,
		GitHub:     &UploadProviderReport{Status: UploadNotificationStatusFailed},
		Source:     UploadNotificationSourceScheduledProbe,
		TaskID:     "scheduled-1",
	})
	if notification.Status != UploadNotificationStatusPartial {
		t.Fatalf("Status = %q, want partial", notification.Status)
	}
	if notification.CloudflareUploadCount != 3 || notification.GitHubUploadCount != 0 {
		t.Fatalf("upload counts = (%d,%d)", notification.CloudflareUploadCount, notification.GitHubUploadCount)
	}
	text := UploadNotificationText(notification)
	for _, want := range []string{"来源：定时任务自动上传", "状态：部分完成", "Cloudflare：完成，上传 3 条", "GitHub：失败，上传 0 条", "任务：scheduled-1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text = %q, missing %q", text, want)
		}
	}
	if strings.Contains(text, "测速") && strings.Contains(text, "MB/s") {
		t.Fatalf("text unexpectedly contains probe detail: %q", text)
	}
}

func TestSendTelegramMessagePostsJSON(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	err := SendTelegramMessage(context.Background(), TelegramNotificationConfig{
		Enabled:  true,
		BotToken: "token:secret",
		ChatID:   "chat-1",
	}, "hello", server.Client(), server.URL)
	if err != nil {
		t.Fatalf("SendTelegramMessage returned error: %v", err)
	}
	if gotPath != "/bottoken:secret/sendMessage" {
		t.Fatalf("path = %q, want Telegram sendMessage path", gotPath)
	}
	if gotBody["chat_id"] != "chat-1" || gotBody["text"] != "hello" {
		t.Fatalf("body = %#v, want chat_id and text", gotBody)
	}
}

func TestSendTelegramUploadNotificationDisabledDoesNotPost(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := SendTelegramUploadNotification(context.Background(), map[string]any{
		"notifications": map[string]any{
			"telegram": map[string]any{"enabled": false},
		},
	}, BuildUploadNotification(UploadNotificationInput{Source: UploadNotificationSourceManualPush}), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("SendTelegramUploadNotification returned error: %v", err)
	}
	if called {
		t.Fatal("Telegram server was called even though notifications are disabled")
	}
}
