package appcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func TestTelegramNotificationConfigFromSnapshot(t *testing.T) {
	cfg := TelegramNotificationConfigFromSnapshot(map[string]any{
		"notifications": map[string]any{
			"telegram": map[string]any{
				"enabled":               true,
				"bot_token":             "  token:secret  ",
				"chat_id":               "  12345 ",
				"include_top_n":         true,
				"personal_chat_id":      " 67890 ",
				"recipient_mode":        "both",
				"top_n":                 9,
				"top_n_recipient_mode":  "chat",
				"upload_recipient_mode": "personal",
			},
		},
	})
	if !cfg.Enabled || cfg.BotToken != "token:secret" || cfg.ChatID != "12345" || cfg.PersonalChatID != "67890" || cfg.RecipientMode != TelegramNotificationRecipientPersonal || cfg.UploadRecipientMode != TelegramNotificationRecipientPersonal || cfg.TopNRecipientMode != TelegramNotificationRecipientChat || !cfg.IncludeTopN || cfg.TopN != 9 {
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

func TestBuildPostProbeNoRowsUploadNotification(t *testing.T) {
	if notification := BuildPostProbeNoRowsUploadNotification(PostProbePushConfig{}, "task-1"); notification != nil {
		t.Fatalf("notification = %#v, want nil when no provider is enabled", notification)
	}
	notification := BuildPostProbeNoRowsUploadNotification(PostProbePushConfig{
		CloudflareEnabled: true,
		GitHubEnabled:     true,
	}, "task-1")
	if notification == nil {
		t.Fatal("notification = nil, want skipped upload notification")
	}
	if notification.CloudflareStatus != UploadNotificationStatusSkipped || notification.GitHubStatus != UploadNotificationStatusSkipped || notification.Status != UploadNotificationStatusSkipped {
		t.Fatalf("notification = %#v, want skipped provider statuses", notification)
	}
	if notification.Source != UploadNotificationSourcePostProbePush || notification.TaskID != "task-1" {
		t.Fatalf("notification source/task = %q/%q, want post probe task", notification.Source, notification.TaskID)
	}
}

func TestUploadNotificationTextIncludesTopEntriesWhenProvided(t *testing.T) {
	notification := BuildUploadNotification(UploadNotificationInput{
		Cloudflare: &UploadProviderReport{Status: UploadNotificationStatusCompleted, UploadCount: 1},
		Source:     UploadNotificationSourceManualPush,
		TopEntries: BuildUploadNotificationTopEntries([]probecore.ProbeRow{{
			Colo:               "HKG",
			DelayMS:            12.34,
			DownloadSpeedMB:    45.67,
			IP:                 "1.1.1.1",
			MaxDownloadSpeedMB: 56.78,
			TraceDelayMS:       23.45,
		}}, 1, "max"),
	})
	text := UploadNotificationText(notification)
	for _, want := range []string{"Top 1：", "1. 1.1.1.1 HKG", "TCP 12.34ms", "追踪 23.45ms", "下载 56.78 MB/s"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text = %q, missing %q", text, want)
		}
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

func TestSendTelegramMessagePostsToConfiguredRecipients(t *testing.T) {
	gotChatIDs := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		gotChatIDs = append(gotChatIDs, body["chat_id"].(string))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	err := SendTelegramMessage(context.Background(), TelegramNotificationConfig{
		BotToken:       "token:secret",
		ChatID:         "group-chat",
		Enabled:        true,
		PersonalChatID: "personal-chat",
		RecipientMode:  TelegramNotificationRecipientBoth,
	}, "hello", server.Client(), server.URL)
	if err != nil {
		t.Fatalf("SendTelegramMessage returned error: %v", err)
	}
	if strings.Join(gotChatIDs, ",") != "group-chat,personal-chat" {
		t.Fatalf("chat IDs = %#v, want group and personal recipients", gotChatIDs)
	}
}

func TestSendTelegramUploadNotificationRoutesUploadAndTopNSeparately(t *testing.T) {
	type request struct {
		chatID string
		text   string
	}
	requests := make([]request, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests = append(requests, request{
			chatID: body["chat_id"].(string),
			text:   body["text"].(string),
		})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	notification := BuildUploadNotification(UploadNotificationInput{
		Cloudflare: &UploadProviderReport{Status: UploadNotificationStatusCompleted, UploadCount: 1},
		Source:     UploadNotificationSourceManualPush,
		TopEntries: []UploadNotificationTopEntry{{IP: "1.1.1.1", Rank: 1}},
	})
	err := SendTelegramUploadNotification(context.Background(), map[string]any{
		"notifications": map[string]any{
			"telegram": map[string]any{
				"bot_token":             "token:secret",
				"chat_id":               "group-chat",
				"enabled":               true,
				"include_top_n":         true,
				"personal_chat_id":      "personal-chat",
				"top_n_recipient_mode":  "chat",
				"upload_recipient_mode": "personal",
			},
		},
	}, notification, server.Client(), server.URL)
	if err != nil {
		t.Fatalf("SendTelegramUploadNotification returned error: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %#v, want upload and Top N messages", requests)
	}
	if requests[0].chatID != "personal-chat" || strings.Contains(requests[0].text, "Top 1") {
		t.Fatalf("upload request = %#v, want upload-only personal message", requests[0])
	}
	if requests[1].chatID != "group-chat" || !strings.Contains(requests[1].text, "Top 1") {
		t.Fatalf("top request = %#v, want Top N group message", requests[1])
	}
}

func TestSendTelegramTestNotificationMergesPurposesForSameChat(t *testing.T) {
	type request struct {
		chatID string
		text   string
	}
	requests := make([]request, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests = append(requests, request{
			chatID: body["chat_id"].(string),
			text:   body["text"].(string),
		})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	chatIDs, err := SendTelegramTestNotification(context.Background(), TelegramNotificationConfig{
		BotToken:            "token:secret",
		ChatID:              "group-chat",
		Enabled:             true,
		IncludeTopN:         true,
		TopNRecipientMode:   TelegramNotificationRecipientChat,
		UploadRecipientMode: TelegramNotificationRecipientChat,
	}, server.Client(), server.URL)
	if err != nil {
		t.Fatalf("SendTelegramTestNotification returned error: %v", err)
	}
	if strings.Join(chatIDs, ",") != "group-chat" {
		t.Fatalf("chat IDs = %#v, want one merged target", chatIDs)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %#v, want one merged receipt", requests)
	}
	if requests[0].chatID != "group-chat" || !strings.Contains(requests[0].text, "用途：上传结论、Top N 列表") {
		t.Fatalf("request = %#v, want merged purpose label", requests[0])
	}
}

func TestSendTelegramTestNotificationKeepsOneReceiptPerChat(t *testing.T) {
	type request struct {
		chatID string
		text   string
	}
	requests := make([]request, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests = append(requests, request{
			chatID: body["chat_id"].(string),
			text:   body["text"].(string),
		})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	chatIDs, err := SendTelegramTestNotification(context.Background(), TelegramNotificationConfig{
		BotToken:            "token:secret",
		ChatID:              "group-chat",
		Enabled:             true,
		IncludeTopN:         true,
		PersonalChatID:      "personal-chat",
		TopNRecipientMode:   TelegramNotificationRecipientChat,
		UploadRecipientMode: TelegramNotificationRecipientPersonal,
	}, server.Client(), server.URL)
	if err != nil {
		t.Fatalf("SendTelegramTestNotification returned error: %v", err)
	}
	if strings.Join(chatIDs, ",") != "personal-chat,group-chat" {
		t.Fatalf("chat IDs = %#v, want personal then group", chatIDs)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %#v, want one receipt per chat", requests)
	}
	if requests[0].chatID != "personal-chat" || !strings.Contains(requests[0].text, "用途：上传结论") || strings.Contains(requests[0].text, "Top N 列表") {
		t.Fatalf("personal request = %#v, want upload-only receipt", requests[0])
	}
	if requests[1].chatID != "group-chat" || !strings.Contains(requests[1].text, "用途：Top N 列表") || strings.Contains(requests[1].text, "上传结论") {
		t.Fatalf("group request = %#v, want Top N-only receipt", requests[1])
	}
}

func TestUploadNotificationTopNTextTruncatesLongMessage(t *testing.T) {
	entries := make([]UploadNotificationTopEntry, 0, 50)
	for i := 0; i < 50; i++ {
		entries = append(entries, UploadNotificationTopEntry{
			Colo:            "HKG",
			DownloadSpeedMB: 123.45,
			IP:              strings.Repeat("1234", 10),
			Rank:            i + 1,
			TCPDelayMS:      12.34,
			TraceDelayMS:    23.45,
		})
	}
	text := UploadNotificationTopNText(BuildUploadNotification(UploadNotificationInput{
		Source:     UploadNotificationSourceManualPush,
		TopEntries: entries,
	}))
	if got := len([]rune(text)); got > MaxTelegramMessageLength {
		t.Fatalf("Top N text length = %d, want <= %d", got, MaxTelegramMessageLength)
	}
	if !strings.HasSuffix(text, "...") {
		t.Fatalf("Top N text = %q, want truncated suffix", text[len(text)-10:])
	}
}

func TestTelegramNotificationTestChatIDsDedupesUploadAndTopNRecipients(t *testing.T) {
	chatIDs := TelegramNotificationTestChatIDs(TelegramNotificationConfig{
		ChatID:              "group-chat",
		IncludeTopN:         true,
		PersonalChatID:      "personal-chat",
		TopNRecipientMode:   TelegramNotificationRecipientBoth,
		UploadRecipientMode: TelegramNotificationRecipientBoth,
	})
	if strings.Join(chatIDs, ",") != "group-chat,personal-chat" {
		t.Fatalf("chat IDs = %#v, want deduped upload and Top N recipients", chatIDs)
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

func TestContextWithTelegramNotificationTimeout(t *testing.T) {
	ctx, cancel := contextWithTelegramNotificationTimeout(context.Background())
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("deadline missing for background context")
	}

	shortCtx, shortCancel := context.WithTimeout(context.Background(), time.Second)
	defer shortCancel()
	got, gotCancel := contextWithTelegramNotificationTimeout(shortCtx)
	defer gotCancel()
	gotDeadline, gotOK := got.Deadline()
	wantDeadline, wantOK := shortCtx.Deadline()
	if !gotOK || !wantOK || !gotDeadline.Equal(wantDeadline) {
		t.Fatalf("deadline = %v/%v, want existing deadline %v/%v", gotDeadline, gotOK, wantDeadline, wantOK)
	}
}
