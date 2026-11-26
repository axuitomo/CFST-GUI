package appcore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

type telegramRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn telegramRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

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

func TestTelegramAPIBaseURLResolution(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		fallback string
		want     string
	}{
		{name: "default", want: TelegramAPIBaseURL},
		{name: "fallback", fallback: "https://fallback.example.com/", want: "https://fallback.example.com"},
		{name: "config wins", config: "https://config.example.com/", fallback: "https://fallback.example.com", want: "https://config.example.com"},
		{name: "scheme added", config: "tg.example.com/api", want: "https://tg.example.com/api"},
		{name: "blank config uses fallback", config: "  ", fallback: "https://fallback.example.com", want: "https://fallback.example.com"},
	}
	for _, testCase := range cases {
		if got := telegramAPIBaseURL(testCase.config, testCase.fallback); got != testCase.want {
			t.Fatalf("%s: telegramAPIBaseURL(%q, %q) = %q, want %q", testCase.name, testCase.config, testCase.fallback, got, testCase.want)
		}
	}
}

func TestTelegramNotificationUsesConfiguredAPIBaseURL(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	cfg := TelegramNotificationConfigFromSnapshot(map[string]any{
		"notifications": map[string]any{"telegram": map[string]any{
			"api_base_url": server.URL + "/", "bot_token": "token:secret", "chat_id": "group-chat", "enabled": true,
		}},
	})
	if err := SendTelegramMessage(context.Background(), cfg, "hello", server.Client(), ""); err != nil {
		t.Fatal(err)
	}
	if requestPath != "/bottoken:secret/sendMessage" {
		t.Fatalf("request path = %q, want configured API base URL path", requestPath)
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
	for _, want := range []string{"来源：定时任务自动上传", "状态：⚠️ 部分成功", "Cloudflare：✅ 成功，上传 3 条", "GitHub：❌ 失败，上传 0 条", "任务：scheduled-1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text = %q, missing %q", text, want)
		}
	}
	if strings.Contains(text, "测速") && strings.Contains(text, "MB/s") {
		t.Fatalf("text unexpectedly contains probe detail: %q", text)
	}
}

func TestBuildPostProbeNoRowsUploadNotification(t *testing.T) {
	if notification := BuildPostProbeNoRowsUploadNotification(PostProbePushConfig{}, nil, nil, 0, "task-1"); notification != nil {
		t.Fatalf("notification = %#v, want nil when no provider is enabled", notification)
	}
	notification := BuildPostProbeNoRowsUploadNotification(PostProbePushConfig{
		DNSEnabled:    true,
		GitHubEnabled: true,
	}, []UploadProviderResult{{ID: "cloudflare", Label: "Cloudflare", Provider: DNSProviderCloudflare, Status: UploadNotificationStatusSkipped}}, &UploadNotificationFunnel{Candidates: 8, Scanned: 10}, 5000, "task-1")
	if notification == nil {
		t.Fatal("notification = nil, want skipped upload notification")
	}
	if len(notification.Providers) != 1 || notification.Providers[0].Status != UploadNotificationStatusSkipped || notification.GitHubStatus != UploadNotificationStatusSkipped || notification.Status != UploadNotificationStatusSkipped {
		t.Fatalf("notification = %#v, want skipped provider statuses", notification)
	}
	if notification.Source != UploadNotificationSourcePostProbePush || notification.TaskID != "task-1" {
		t.Fatalf("notification source/task = %q/%q, want post probe task", notification.Source, notification.TaskID)
	}
}

func TestProcessPostProbePushSkipsWorkWhenContextIsCanceled(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := service.ProcessPostProbePush(ctx, ProbePayload{
		Config: map[string]any{"post_probe_push": map[string]any{"cloudflare_enabled": true}},
		TaskID: "cancelled-task",
	}, ProbeRunResult{Results: []probecore.ProbeRow{{IP: "1.1.1.1"}}})

	if result.Notification != nil || len(result.Warnings) != 0 {
		t.Fatalf("canceled post-push result = %#v, want empty", result)
	}
}

func TestProcessPostProbePushReportsUnavailableProvider(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	result := service.ProcessPostProbePush(context.Background(), ProbePayload{
		Config: map[string]any{"post_probe_push": map[string]any{"cloudflare_enabled": true}},
		TaskID: "probe-task",
	}, ProbeRunResult{Results: []probecore.ProbeRow{{IP: "1.1.1.1"}}})

	if result.Notification == nil || len(result.Notification.Providers) != 1 || result.Notification.Providers[0].Status != UploadNotificationStatusSkipped || result.Notification.Status != UploadNotificationStatusSkipped {
		t.Fatalf("notification = %#v, want skipped provider status", result.Notification)
	}
}

func TestProcessPostProbePushIncludesTopEntries(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	result := service.ProcessPostProbePush(context.Background(), ProbePayload{
		Config: map[string]any{
			"notifications":   map[string]any{"telegram": map[string]any{"include_top_n": true, "top_n": 1}},
			"post_probe_push": map[string]any{"cloudflare_enabled": true},
		},
		TaskID: "probe-task",
	}, ProbeRunResult{Results: []probecore.ProbeRow{
		{IP: "1.1.1.1", DelayMS: 100, DownloadSpeedMB: 1},
		{IP: "1.1.1.2", DelayMS: 10, DownloadSpeedMB: 100},
	}})

	if result.Notification == nil || len(result.Notification.TopEntries) != 1 || result.Notification.TopEntries[0].IP != "1.1.1.2" {
		t.Fatalf("TopEntries = %#v, want fastest row", result.Notification)
	}
}

func TestProcessPostProbePushReturnsSelectionFailureWarning(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	result := service.ProcessPostProbePush(context.Background(), ProbePayload{
		Config: map[string]any{
			"cloudflare":      map[string]any{"enabled": true, "api_token": "test-token", "record_name": "edge.example.com", "record_type": "A", "zone_id": "zone-123"},
			"post_probe_push": map[string]any{"cloudflare_enabled": true},
			"upload":          map[string]any{"shared_filter": map[string]any{"enabled": true, "colo_allow": "JP"}},
		},
		TaskID: "probe-task",
	}, ProbeRunResult{Results: []probecore.ProbeRow{{IP: "1.1.1.1", Colo: "LAX", DownloadSpeedMB: 10}}})

	if result.Notification != nil || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "post-probe upload selection failed") {
		t.Fatalf("result = %#v, want selection failure warning", result)
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

func TestSendTelegramMessageSplitsLongTextByUTF16Length(t *testing.T) {
	messages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		text, _ := body["text"].(string)
		if got := telegramUTF16Length(text); got > MaxTelegramMessageLength {
			t.Errorf("message UTF-16 length = %d, want <= %d", got, MaxTelegramMessageLength)
		}
		messages = append(messages, text)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	input := strings.Repeat("测速结果😀\n", 900) + "完成"
	err := SendTelegramMessage(context.Background(), TelegramNotificationConfig{
		Enabled: true, BotToken: "token:secret", ChatID: "chat-1",
	}, input, server.Client(), server.URL)
	if err != nil {
		t.Fatalf("SendTelegramMessage returned error: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("message count = %d, want split delivery", len(messages))
	}
	if got := strings.Join(messages, ""); got != input {
		t.Fatalf("joined messages differ from input: got %d runes, want %d", len([]rune(got)), len([]rune(input)))
	}
}

func telegramUTF16Length(value string) int {
	total := 0
	for _, r := range value {
		if r > 0xffff {
			total += 2
		} else {
			total++
		}
	}
	return total
}

func TestSendTelegramMessageRedactsBotTokenFromTransportError(t *testing.T) {
	botToken := "123456789:" + "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh-"
	client := &http.Client{Transport: telegramRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed")
	})}

	err := SendTelegramMessage(context.Background(), TelegramNotificationConfig{
		Enabled: true, BotToken: botToken, ChatID: "chat-1",
	}, "hello", client, TelegramAPIBaseURL)
	if err == nil {
		t.Fatal("SendTelegramMessage returned nil error")
	}
	if strings.Contains(err.Error(), botToken) {
		t.Fatalf("transport error leaked Telegram bot token: %s", err)
	}
	if !strings.Contains(err.Error(), "<redacted>") {
		t.Fatalf("transport error = %q, want redaction marker", err)
	}
}

func TestSendTelegramMessageRedactsChatIDFromFailure(t *testing.T) {
	const chatID = "-1001234567890"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "recipient rejected", http.StatusBadRequest)
	}))
	defer server.Close()

	err := SendTelegramMessage(context.Background(), TelegramNotificationConfig{
		Enabled: true, BotToken: "token:secret", ChatID: chatID,
	}, "hello", server.Client(), server.URL)
	if err == nil {
		t.Fatal("SendTelegramMessage returned nil error")
	}
	if strings.Contains(err.Error(), chatID) {
		t.Fatalf("Telegram failure leaked chat ID: %s", err)
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

func TestTaskFailureNotificationTextIncludesBriefReason(t *testing.T) {
	text := TaskFailureNotificationText(TaskFailureNotificationInput{
		CreatedAt: time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
		Message:   strings.Repeat("下载测速失败", 80),
		Stage:     "stage3_download",
		TaskID:    "failed-task",
	})
	for _, want := range []string{"CFST 任务失败", "状态：❌ 失败", "任务：failed-task", "阶段：stage3_download", "原因："} {
		if !strings.Contains(text, want) {
			t.Fatalf("text = %q, missing %q", text, want)
		}
	}
	if !strings.Contains(text, "...") {
		t.Fatalf("text = %q, want truncated reason", text)
	}
}

func TestUploadNotificationFailureBecomesTaskFailureInput(t *testing.T) {
	notification := BuildUploadNotification(UploadNotificationInput{
		Cloudflare: &UploadProviderReport{Status: UploadNotificationStatusCompleted, UploadCount: 3},
		CreatedAt:  time.Date(2026, 6, 20, 11, 0, 0, 0, time.UTC),
		GitHub:     &UploadProviderReport{Status: UploadNotificationStatusFailed},
		Message:    "GitHub token 无效",
		Source:     UploadNotificationSourcePostProbePush,
		TaskID:     "upload-failed-task",
	})
	if notification.Status != UploadNotificationStatusPartial {
		t.Fatalf("status = %q, want partial", notification.Status)
	}
	if !UploadNotificationHasFailure(notification) {
		t.Fatal("UploadNotificationHasFailure = false, want true for partial upload")
	}
	input := TaskFailureNotificationInputFromUploadNotification(notification)
	if input.Stage != "upload" || input.TaskID != "upload-failed-task" {
		t.Fatalf("input = %#v, want upload stage and task ID", input)
	}
	for _, want := range []string{"上传失败：手动测速后自动上传", "GitHub：❌ 失败，上传 0 条", "GitHub token 无效"} {
		if !strings.Contains(input.Message, want) {
			t.Fatalf("message = %q, missing %q", input.Message, want)
		}
	}
}

func TestSendTelegramTaskFailureNotificationPostsReason(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	err := SendTelegramTaskFailureNotification(context.Background(), map[string]any{
		"notifications": map[string]any{
			"telegram": map[string]any{
				"bot_token":             "token:secret",
				"chat_id":               "group-chat",
				"enabled":               true,
				"upload_recipient_mode": "chat",
			},
		},
	}, TaskFailureNotificationInput{
		Message: "输入源为空",
		Stage:   "stage0_pool",
		TaskID:  "failed-task",
	}, server.Client(), server.URL)
	if err != nil {
		t.Fatalf("SendTelegramTaskFailureNotification returned error: %v", err)
	}
	if gotBody["chat_id"] != "group-chat" {
		t.Fatalf("chat_id = %#v, want group-chat", gotBody["chat_id"])
	}
	text := gotBody["text"].(string)
	for _, want := range []string{"CFST 任务失败", "任务：failed-task", "阶段：stage0_pool", "原因：输入源为空"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text = %q, missing %q", text, want)
		}
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

func TestServiceAttachesManualUploadNotificationWithTopEntries(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	taskID := "manual-task"
	result := NewCommandResult("DNS_PUSH_COMPLETED", map[string]any{"upload_count": 1}, "completed", true, &taskID, nil)
	result = service.attachManualUploadNotification(map[string]any{
		"config":               map[string]any{"notifications": map[string]any{"telegram": map[string]any{"include_top_n": true, "top_n": 1}}},
		"notification_trigger": UploadNotificationSourceManualPush,
		"results": []probecore.ProbeRow{
			{IP: "1.1.1.1", DelayMS: 100, DownloadSpeedMB: 1},
			{IP: "1.1.1.2", DelayMS: 10, DownloadSpeedMB: 100},
		},
	}, UploadNotificationProviderCloudflare, result)

	notification, ok := mapValue(result.Data)["upload_notification"].(UploadNotification)
	if !ok || notification.Source != UploadNotificationSourceManualPush || notification.CloudflareUploadCount != 1 {
		t.Fatalf("upload notification = %#v", mapValue(result.Data)["upload_notification"])
	}
	if len(notification.TopEntries) != 1 || notification.TopEntries[0].IP != "1.1.1.2" {
		t.Fatalf("TopEntries = %#v, want fastest row", notification.TopEntries)
	}
}

func TestServiceRecordsSchedulerUploadNotification(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	status, warnings := service.RecordSchedulerUploadNotification(context.Background(), map[string]any{}, SchedulerStatus{
		CloudflareUploadCount: 2,
		LastDNSStatus:         UploadNotificationStatusCompleted,
		LastMessage:           "probe completed",
		LastProbeStatus:       "completed",
	}, UploadNotificationSourceScheduledProbe, "scheduled-task", true, false, nil, nil, 0)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if status.UploadNotification == nil || status.UploadNotification.Message == "" || len(status.UploadNotification.Providers) != 1 || status.UploadNotification.Providers[0].UploadCount != 2 {
		t.Fatalf("UploadNotification = %#v", status.UploadNotification)
	}
}

func TestServiceTelegramTestRejectsMissingConfiguredRecipient(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	payload, err := json.Marshal(map[string]any{
		"config": map[string]any{"notifications": map[string]any{"telegram": map[string]any{
			"bot_token": "token:secret", "chat_id": "group-chat", "enabled": false,
			"include_top_n": true, "top_n_recipient_mode": "chat", "upload_recipient_mode": "personal",
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, handled := service.TryInvoke("telegram.test", string(payload))
	if !handled || result.OK || !strings.Contains(result.Message, "Telegram") {
		t.Fatalf("result = %#v, handled=%v", result, handled)
	}
}

func TestProbeFailureNotificationIsSentByService(t *testing.T) {
	requests := 0
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	service := NewService(ServiceOptions{
		HTTPClient:         server.Client(),
		Storage:            StorageLayout{Root: t.TempDir(), ConfigFileName: "config.json"},
		TelegramAPIBaseURL: server.URL,
	})
	if _, err := service.SaveConfig(map[string]any{
		"notifications": map[string]any{"telegram": map[string]any{
			"bot_token": "token:secret", "chat_id": "group-chat", "enabled": true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.PublishProbeEvent(context.Background(), ProbeEvent{
		Event: "probe.failed", TaskID: "failed-task", Payload: map[string]any{"message": "source failed", "stage": "source"},
	}); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || body["chat_id"] != "group-chat" || !strings.Contains(body["text"].(string), "source failed") {
		t.Fatalf("requests=%d body=%#v", requests, body)
	}
}

func TestNotificationHTTPClientHonorsSystemProxyToggle(t *testing.T) {
	proxyOf := func(client *http.Client) func(*http.Request) (*url.URL, error) {
		transport, ok := client.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("Transport = %T, want *http.Transport", client.Transport)
		}
		return transport.Proxy
	}
	// 勾选“跟随系统代理”时用共享的系统代理客户端。
	if client := notificationHTTPClient(nil, true); proxyOf(client) == nil {
		t.Fatal("use system proxy client has no proxy resolver")
	}
	// 关闭时一律直连，不能退回默认客户端去读 HTTP_PROXY 等环境变量。
	for _, client := range []*http.Client{nil, http.DefaultClient} {
		if proxyOf(notificationHTTPClient(client, false)) != nil {
			t.Fatalf("direct client from %#v follows a proxy", client)
		}
	}
	custom := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, nil
	})}
	if got := notificationHTTPClient(custom, true); got != custom {
		t.Fatalf("custom client was replaced: %#v", got)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestUploadNotificationStatusLabelsUseIcons(t *testing.T) {
	cases := map[string]string{
		UploadNotificationStatusCompleted:   "✅ 成功",
		UploadNotificationStatusFailed:      "❌ 失败",
		UploadNotificationStatusPartial:     "⚠️ 部分成功",
		UploadNotificationStatusAborted:     "🚫 熔断/终止",
		UploadNotificationStatusSkipped:     "⏭️ 跳过",
		UploadNotificationStatusUnsupported: "❓ 暂不支持",
	}
	for status, want := range cases {
		if got := UploadNotificationStatusLabel(status); got != want {
			t.Fatalf("UploadNotificationStatusLabel(%q) = %q, want %q", status, got, want)
		}
	}
	// 旧配置/旧代码写进来的取消状态按终止处理，不再显示成「跳过」。
	if got := normalizeUploadNotificationStatus("cancelled"); got != UploadNotificationStatusAborted {
		t.Fatalf("normalize(cancelled) = %q, want aborted", got)
	}
}

func TestUploadNotificationAbortedOverallStatus(t *testing.T) {
	notification := BuildUploadNotification(UploadNotificationInput{
		CreatedAt:  time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC),
		DurationMS: 225_000,
		Funnel:     &UploadNotificationFunnel{Candidates: 900, Passed: 42, Qualified: 5, Scanned: 1200},
		Providers:  []UploadProviderResult{{Label: "Cloudflare", Provider: DNSProviderCloudflare, Status: UploadNotificationStatusAborted}},
		Source:     UploadNotificationSourceScheduledProbe,
	})
	if notification.Status != UploadNotificationStatusAborted {
		t.Fatalf("Status = %q, want aborted", notification.Status)
	}
	if UploadNotificationHasFailure(notification) {
		t.Fatal("aborted notification must not be reported as a failure")
	}
	text := UploadNotificationText(notification)
	for _, want := range []string{
		"状态：🚫 熔断/终止",
		"Cloudflare：🚫 熔断/终止，上传 0 条",
		"漏斗：扫描 1200 → 候选 900 → 通过 42 → 达标 5",
		"耗时：3 分 45 秒",
		"任务被终止或熔断，上传流程没有跑完。",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text = %q, missing %q", text, want)
		}
	}
	// 成功与终止混在一起只算部分成功，不能整体显示成功。
	mixed := BuildUploadNotification(UploadNotificationInput{
		Cloudflare: &UploadProviderReport{Status: UploadNotificationStatusCompleted, UploadCount: 3},
		GitHub:     &UploadProviderReport{Status: UploadNotificationStatusAborted},
	})
	if mixed.Status != UploadNotificationStatusPartial {
		t.Fatalf("mixed Status = %q, want partial", mixed.Status)
	}
}

func TestNotificationTimeAndDurationLabels(t *testing.T) {
	label := notificationTimeLabel("2026-11-20T13:37:45Z")
	if strings.Contains(label, "2026-11-20T13:37:45") || strings.HasSuffix(label, "Z") {
		t.Fatalf("label = %q, want local wall clock instead of RFC3339", label)
	}
	if _, err := time.Parse("2006-01-02 15:04:05 MST", label); err != nil {
		t.Fatalf("label = %q, not a parseable local timestamp: %v", label, err)
	}
	if got := notificationTimeLabel("not-a-time"); got != "not-a-time" {
		t.Fatalf("unparsable time = %q, want original value", got)
	}
	for durationMS, want := range map[int64]string{0: "0 秒", 45_000: "45 秒", 225_000: "3 分 45 秒", 5_400_000: "1 时 30 分"} {
		if got := notificationDurationLabel(durationMS); got != want {
			t.Fatalf("notificationDurationLabel(%d) = %q, want %q", durationMS, got, want)
		}
	}
}
