package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	TelegramAPIBaseURL = "https://api.telegram.org"

	TelegramNotificationRecipientChat     = "chat"
	TelegramNotificationRecipientPersonal = "personal"
	TelegramNotificationRecipientBoth     = "both"

	DefaultTelegramNotificationTopN = 5
	MaxTelegramNotificationTopN     = 50
	MaxTelegramMessageLength        = 4096
	TelegramNotificationSendTimeout = 20 * time.Second
)

type TelegramNotificationConfig struct {
	APIBaseURL          string `json:"api_base_url"`
	BotToken            string `json:"bot_token"`
	ChatID              string `json:"chat_id"`
	Enabled             bool   `json:"enabled"`
	IncludeTopN         bool   `json:"include_top_n"`
	PersonalChatID      string `json:"personal_chat_id"`
	RecipientMode       string `json:"recipient_mode"`
	TopN                int    `json:"top_n"`
	TopNRecipientMode   string `json:"top_n_recipient_mode"`
	UploadRecipientMode string `json:"upload_recipient_mode"`
	UseSystemProxy      bool   `json:"use_system_proxy"`
}

type telegramNotificationTestReceipt struct {
	ChatID   string
	Purposes []string
}

func TelegramNotificationConfigFromSnapshot(snapshot map[string]any) TelegramNotificationConfig {
	notifications := mapValue(firstNonNil(snapshot["notifications"], snapshot["notification"]))
	telegram := mapValue(firstNonNil(notifications["telegram"], notifications["tg"], snapshot["telegram"]))
	topN := DefaultTelegramNotificationTopN
	if value := firstNonNil(telegram["top_n"], telegram["topN"]); value != nil {
		topN = intValue(value, DefaultTelegramNotificationTopN)
	}
	legacyRecipientMode := normalizeTelegramRecipientMode(stringValue(firstNonNil(telegram["recipient_mode"], telegram["recipientMode"], telegram["target_mode"], telegram["targetMode"]), TelegramNotificationRecipientChat))
	uploadRecipientMode := normalizeTelegramRecipientMode(stringValue(firstNonNil(telegram["upload_recipient_mode"], telegram["uploadRecipientMode"], legacyRecipientMode), legacyRecipientMode))
	topNRecipientMode := normalizeTelegramRecipientMode(stringValue(firstNonNil(telegram["top_n_recipient_mode"], telegram["topNRecipientMode"], telegram["top_recipient_mode"], telegram["topRecipientMode"], legacyRecipientMode), legacyRecipientMode))
	return TelegramNotificationConfig{
		APIBaseURL:          strings.TrimSpace(stringValue(firstNonNil(telegram["api_base_url"], telegram["apiBaseUrl"], telegram["api_host"], telegram["apiHost"]), "")),
		BotToken:            strings.TrimSpace(stringValue(firstNonNil(telegram["bot_token"], telegram["botToken"], telegram["token"]), "")),
		ChatID:              strings.TrimSpace(stringValue(firstNonNil(telegram["chat_id"], telegram["chatId"], telegram["chat"], telegram["target_chat_id"], telegram["targetChatId"], telegram["channel_chat_id"], telegram["channelChatId"], telegram["group_chat_id"], telegram["groupChatId"]), "")),
		Enabled:             boolValue(firstNonNil(telegram["enabled"], telegram["telegram_enabled"], telegram["telegramEnabled"]), false),
		IncludeTopN:         boolValue(firstNonNil(telegram["include_top_n"], telegram["includeTopN"], telegram["top_n_enabled"], telegram["topNEnabled"]), false),
		PersonalChatID:      strings.TrimSpace(stringValue(firstNonNil(telegram["personal_chat_id"], telegram["personalChatId"], telegram["private_chat_id"], telegram["privateChatId"], telegram["user_chat_id"], telegram["userChatId"]), "")),
		RecipientMode:       uploadRecipientMode,
		TopN:                normalizeTelegramTopN(topN),
		TopNRecipientMode:   topNRecipientMode,
		UploadRecipientMode: uploadRecipientMode,
		UseSystemProxy:      boolValue(firstNonNil(telegram["use_system_proxy"], telegram["useSystemProxy"]), false),
	}
}

func SendTelegramUploadNotification(ctx context.Context, snapshot map[string]any, notification UploadNotification, client *http.Client, apiBaseURL string) error {
	cfg := TelegramNotificationConfigFromSnapshot(snapshot)
	if !cfg.Enabled {
		return nil
	}
	ctx, cancel := contextWithTelegramNotificationTimeout(ctx)
	defer cancel()
	client = notificationHTTPClient(client, cfg.UseSystemProxy)
	failures := make([]string, 0, 2)
	if err := SendTelegramMessageToChatIDs(ctx, cfg, TelegramNotificationChatIDs(cfg), UploadNotificationSummaryText(notification), client, apiBaseURL); err != nil {
		failures = append(failures, "上传结论："+err.Error())
	}
	if cfg.IncludeTopN && len(notification.TopEntries) > 0 {
		if err := SendTelegramMessageToChatIDs(ctx, cfg, TelegramNotificationTopNChatIDs(cfg), UploadNotificationTopNText(notification), client, apiBaseURL); err != nil {
			failures = append(failures, "Top N："+err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "；"))
	}
	return nil
}

func SendTelegramTaskFailureNotification(ctx context.Context, snapshot map[string]any, input TaskFailureNotificationInput, client *http.Client, apiBaseURL string) error {
	cfg := TelegramNotificationConfigFromSnapshot(snapshot)
	if !cfg.Enabled {
		return nil
	}
	return SendTelegramMessageToChatIDs(ctx, cfg, TelegramNotificationChatIDs(cfg), TaskFailureNotificationText(input), client, apiBaseURL)
}

func SendTelegramMessage(ctx context.Context, cfg TelegramNotificationConfig, text string, client *http.Client, apiBaseURL string) error {
	return SendTelegramMessageToChatIDs(ctx, cfg, TelegramNotificationChatIDs(cfg), text, client, apiBaseURL)
}

func SendTelegramTestNotification(ctx context.Context, cfg TelegramNotificationConfig, client *http.Client, apiBaseURL string) ([]string, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	ctx, cancel := contextWithTelegramNotificationTimeout(ctx)
	defer cancel()
	client = notificationHTTPClient(client, cfg.UseSystemProxy)
	cfg.BotToken = strings.TrimSpace(cfg.BotToken)
	if cfg.BotToken == "" || IsMaskedSecret(cfg.BotToken) {
		return nil, errors.New("Telegram 通知配置不完整")
	}
	receipts, err := telegramNotificationTestReceipts(cfg)
	if err != nil {
		return nil, err
	}
	apiBaseURL = telegramAPIBaseURL(cfg.APIBaseURL, apiBaseURL)
	chatIDs := make([]string, 0, len(receipts))
	failures := make([]string, 0, len(receipts))
	for index, receipt := range receipts {
		chatIDs = append(chatIDs, receipt.ChatID)
		if err := sendTelegramMessageToChat(ctx, cfg.BotToken, receipt.ChatID, telegramNotificationTestReceiptText(receipt), client, apiBaseURL); err != nil {
			failures = append(failures, fmt.Sprintf("Telegram target %d: %v", index+1, err))
		}
	}
	if len(failures) > 0 {
		return chatIDs, errors.New(strings.Join(failures, "；"))
	}
	return chatIDs, nil
}

func SendTelegramMessageToChatIDs(ctx context.Context, cfg TelegramNotificationConfig, chatIDs []string, text string, client *http.Client, apiBaseURL string) error {
	if !cfg.Enabled {
		return nil
	}
	ctx, cancel := contextWithTelegramNotificationTimeout(ctx)
	defer cancel()
	client = notificationHTTPClient(client, cfg.UseSystemProxy)
	cfg.BotToken = strings.TrimSpace(cfg.BotToken)
	if cfg.BotToken == "" || IsMaskedSecret(cfg.BotToken) {
		return errors.New("Telegram 通知配置不完整")
	}
	chatIDs = dedupeTelegramChatIDs(chatIDs)
	if len(chatIDs) == 0 {
		return errors.New("Telegram 通知目标配置不完整")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("Telegram 通知内容为空")
	}
	apiBaseURL = telegramAPIBaseURL(cfg.APIBaseURL, apiBaseURL)
	failures := make([]string, 0)
	for index, chatID := range chatIDs {
		if err := sendTelegramMessageToChat(ctx, cfg.BotToken, chatID, text, client, apiBaseURL); err != nil {
			failures = append(failures, fmt.Sprintf("Telegram target %d: %v", index+1, err))
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "；"))
	}
	return nil
}

func ContextWithTelegramNotificationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, TelegramNotificationSendTimeout)
}

// TelegramAPIBaseURLForConfig 决定实际请求根地址：配置里的自定义网关优先，其次服务级覆盖，
// 最后回落到官方地址。缺少协议头时按 https 补齐，方便只填域名。
func TelegramAPIBaseURLForConfig(configValue string, fallback string) string {
	for _, candidate := range []string{configValue, fallback} {
		base := strings.TrimSpace(candidate)
		if base == "" {
			continue
		}
		if !strings.Contains(base, "://") {
			base = "https://" + base
		}
		return strings.TrimRight(base, "/")
	}
	return TelegramAPIBaseURL
}

func TelegramNotificationChatIDs(cfg TelegramNotificationConfig) []string {
	mode := cfg.UploadRecipientMode
	if strings.TrimSpace(mode) == "" {
		mode = cfg.RecipientMode
	}
	return telegramNotificationChatIDsForMode(cfg, mode)
}

func TelegramNotificationTopNChatIDs(cfg TelegramNotificationConfig) []string {
	mode := cfg.TopNRecipientMode
	if strings.TrimSpace(mode) == "" {
		mode = cfg.RecipientMode
	}
	return telegramNotificationChatIDsForMode(cfg, mode)
}

func TelegramNotificationActiveChatIDs(cfg TelegramNotificationConfig) []string {
	chatIDs := TelegramNotificationChatIDs(cfg)
	if cfg.IncludeTopN {
		chatIDs = append(chatIDs, TelegramNotificationTopNChatIDs(cfg)...)
	}
	return dedupeTelegramChatIDs(chatIDs)
}

func TelegramNotificationTestChatIDs(cfg TelegramNotificationConfig) []string {
	chatIDs := append([]string{}, TelegramNotificationChatIDs(cfg)...)
	if cfg.IncludeTopN {
		chatIDs = append(chatIDs, TelegramNotificationTopNChatIDs(cfg)...)
	}
	return dedupeTelegramChatIDs(chatIDs)
}

func telegramNotificationTestReceipts(cfg TelegramNotificationConfig) ([]telegramNotificationTestReceipt, error) {
	uploadChatIDs := TelegramNotificationChatIDs(cfg)
	if len(uploadChatIDs) == 0 {
		return nil, errors.New("Telegram 通知目标配置不完整")
	}
	topNChatIDs := []string(nil)
	if cfg.IncludeTopN {
		topNChatIDs = TelegramNotificationTopNChatIDs(cfg)
		if len(topNChatIDs) == 0 {
			return nil, errors.New("Telegram 通知目标配置不完整")
		}
	}
	receiptCapacity := len(uploadChatIDs)
	if len(topNChatIDs) <= math.MaxInt-receiptCapacity {
		receiptCapacity += len(topNChatIDs)
	}
	receipts := make([]telegramNotificationTestReceipt, 0, receiptCapacity)
	indexByChatID := make(map[string]int, receiptCapacity)
	appendPurpose := func(chatIDs []string, purpose string) {
		for _, chatID := range chatIDs {
			if index, exists := indexByChatID[chatID]; exists {
				receipts[index].Purposes = append(receipts[index].Purposes, purpose)
				continue
			}
			indexByChatID[chatID] = len(receipts)
			receipts = append(receipts, telegramNotificationTestReceipt{
				ChatID:   chatID,
				Purposes: []string{purpose},
			})
		}
	}
	appendPurpose(uploadChatIDs, "upload")
	appendPurpose(topNChatIDs, "topn")
	return receipts, nil
}

func telegramNotificationTestReceiptText(receipt telegramNotificationTestReceipt) string {
	labels := make([]string, 0, len(receipt.Purposes))
	for _, purpose := range receipt.Purposes {
		labels = append(labels, telegramNotificationTestPurposeLabel(purpose))
	}
	return strings.Join([]string{
		"CFST Telegram 通知测试",
		"状态：Telegram 通知渠道可用。",
		"用途：" + strings.Join(labels, "、"),
	}, "\n")
}

func telegramNotificationTestPurposeLabel(purpose string) string {
	switch purpose {
	case "topn":
		return "Top N 列表"
	default:
		return "上传结论"
	}
}

func telegramNotificationChatIDsForMode(cfg TelegramNotificationConfig, mode string) []string {
	switch normalizeTelegramRecipientMode(mode) {
	case TelegramNotificationRecipientPersonal:
		return dedupeTelegramChatIDs([]string{cfg.PersonalChatID})
	case TelegramNotificationRecipientBoth:
		return dedupeTelegramChatIDs([]string{cfg.ChatID, cfg.PersonalChatID})
	default:
		return dedupeTelegramChatIDs([]string{cfg.ChatID})
	}
}

func dedupeTelegramChatIDs(chatIDs []string) []string {
	if len(chatIDs) == 0 {
		return nil
	}
	result := make([]string, 0, len(chatIDs))
	seen := make(map[string]struct{}, len(chatIDs))
	for _, chatID := range chatIDs {
		chatID = strings.TrimSpace(chatID)
		if chatID == "" {
			continue
		}
		if _, exists := seen[chatID]; exists {
			continue
		}
		seen[chatID] = struct{}{}
		result = append(result, chatID)
	}
	return result
}

func sendTelegramMessageToChat(ctx context.Context, botToken string, chatID string, text string, client *http.Client, apiBaseURL string) error {
	for _, chunk := range splitTelegramMessage(text, MaxTelegramMessageLength) {
		if err := sendTelegramMessageChunk(ctx, botToken, chatID, chunk, client, apiBaseURL); err != nil {
			return err
		}
	}
	return nil
}

func sendTelegramMessageChunk(ctx context.Context, botToken string, chatID string, text string, client *http.Client, apiBaseURL string) error {
	body, err := json.Marshal(map[string]any{
		"chat_id":                  chatID,
		"disable_web_page_preview": true,
		"text":                     text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/bot"+botToken+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return redactTelegramRequestError(err, botToken)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return redactTelegramRequestError(err, botToken)
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	message := strings.TrimSpace(string(raw))
	if message == "" {
		message = res.Status
	}
	return redactTelegramRequestError(fmt.Errorf("Telegram 通知发送失败：HTTP %d：%s", res.StatusCode, truncateTelegramLine(message, 300)), botToken)
}

func redactTelegramRequestError(err error, botToken string) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if token := strings.TrimSpace(botToken); token != "" {
		message = strings.ReplaceAll(message, token, "<redacted>")
	}
	return errors.New(message)
}

func normalizeTelegramRecipientMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case TelegramNotificationRecipientPersonal, "private", "direct", "user", "me":
		return TelegramNotificationRecipientPersonal
	case TelegramNotificationRecipientBoth, "all", "chat_personal", "chat_and_personal", "personal_and_chat":
		return TelegramNotificationRecipientBoth
	default:
		return TelegramNotificationRecipientChat
	}
}

func normalizeTelegramTopN(value int) int {
	if value < 0 {
		return 0
	}
	if value > MaxTelegramNotificationTopN {
		return MaxTelegramNotificationTopN
	}
	return value
}

func splitTelegramMessage(value string, limit int) []string {
	if value == "" {
		return nil
	}
	if limit <= 0 {
		return []string{value}
	}
	chunks := make([]string, 0)
	runes := []rune(value)
	start := 0
	units := 0
	for index, r := range runes {
		runeUnits := utf16.RuneLen(r)
		if units > 0 && units+runeUnits > limit {
			chunks = append(chunks, string(runes[start:index]))
			start = index
			units = 0
		}
		units += runeUnits
	}
	if start < len(runes) {
		chunks = append(chunks, string(runes[start:]))
	}
	return chunks
}
