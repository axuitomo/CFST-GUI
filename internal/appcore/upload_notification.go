package appcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

const (
	UploadNotificationSourceManualPush        = "manual_push"
	UploadNotificationSourcePostProbePush     = "post_probe_push"
	UploadNotificationSourceScheduledProbe    = "scheduled_probe"
	UploadNotificationSourceScheduledPipeline = "scheduled_pipeline"

	UploadNotificationProviderCloudflare = "cloudflare"
	UploadNotificationProviderGitHub     = "github"

	UploadNotificationStatusCompleted   = "completed"
	UploadNotificationStatusFailed      = "failed"
	UploadNotificationStatusPartial     = "partial"
	UploadNotificationStatusSkipped     = "skipped"
	UploadNotificationStatusUnsupported = "unsupported"

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
	BotToken            string `json:"bot_token"`
	ChatID              string `json:"chat_id"`
	Enabled             bool   `json:"enabled"`
	IncludeTopN         bool   `json:"include_top_n"`
	PersonalChatID      string `json:"personal_chat_id"`
	RecipientMode       string `json:"recipient_mode"`
	TopN                int    `json:"top_n"`
	TopNRecipientMode   string `json:"top_n_recipient_mode"`
	UploadRecipientMode string `json:"upload_recipient_mode"`
}

type telegramNotificationTestReceipt struct {
	ChatID   string
	Purposes []string
}

type UploadProviderReport struct {
	Status      string `json:"status"`
	UploadCount int    `json:"upload_count"`
}

type UploadNotificationTopEntry struct {
	Colo            string  `json:"colo,omitempty"`
	DownloadSpeedMB float64 `json:"download_speed_mb,omitempty"`
	IP              string  `json:"ip"`
	Rank            int     `json:"rank"`
	TCPDelayMS      float64 `json:"tcp_delay_ms,omitempty"`
	TraceDelayMS    float64 `json:"trace_delay_ms,omitempty"`
}

type UploadNotification struct {
	CloudflareStatus      string                       `json:"cloudflare_status,omitempty"`
	CloudflareUploadCount int                          `json:"cloudflare_upload_count,omitempty"`
	CreatedAt             string                       `json:"created_at"`
	GitHubStatus          string                       `json:"github_status,omitempty"`
	GitHubUploadCount     int                          `json:"github_upload_count,omitempty"`
	Message               string                       `json:"message"`
	Source                string                       `json:"source"`
	Status                string                       `json:"status"`
	TaskID                string                       `json:"task_id,omitempty"`
	TopEntries            []UploadNotificationTopEntry `json:"top_entries,omitempty"`
}

type UploadNotificationInput struct {
	Cloudflare *UploadProviderReport
	CreatedAt  time.Time
	GitHub     *UploadProviderReport
	Message    string
	Source     string
	TaskID     string
	TopEntries []UploadNotificationTopEntry
}

type TaskFailureNotificationInput struct {
	CreatedAt time.Time
	Message   string
	Stage     string
	TaskID    string
}

type CommandResultUploadNotificationInput struct {
	CreatedAt  time.Time
	Provider   string
	Result     CommandResult
	Source     string
	TaskID     string
	TopEntries []UploadNotificationTopEntry
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
		BotToken:            strings.TrimSpace(stringValue(firstNonNil(telegram["bot_token"], telegram["botToken"], telegram["token"]), "")),
		ChatID:              strings.TrimSpace(stringValue(firstNonNil(telegram["chat_id"], telegram["chatId"], telegram["chat"], telegram["target_chat_id"], telegram["targetChatId"], telegram["channel_chat_id"], telegram["channelChatId"], telegram["group_chat_id"], telegram["groupChatId"]), "")),
		Enabled:             boolValue(firstNonNil(telegram["enabled"], telegram["telegram_enabled"], telegram["telegramEnabled"]), false),
		IncludeTopN:         boolValue(firstNonNil(telegram["include_top_n"], telegram["includeTopN"], telegram["top_n_enabled"], telegram["topNEnabled"]), false),
		PersonalChatID:      strings.TrimSpace(stringValue(firstNonNil(telegram["personal_chat_id"], telegram["personalChatId"], telegram["private_chat_id"], telegram["privateChatId"], telegram["user_chat_id"], telegram["userChatId"]), "")),
		RecipientMode:       uploadRecipientMode,
		TopN:                normalizeTelegramTopN(topN),
		TopNRecipientMode:   topNRecipientMode,
		UploadRecipientMode: uploadRecipientMode,
	}
}

func BuildUploadNotification(input UploadNotificationInput) UploadNotification {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	notification := UploadNotification{
		CreatedAt:  createdAt.Format(time.RFC3339),
		Message:    strings.TrimSpace(input.Message),
		Source:     strings.TrimSpace(input.Source),
		TaskID:     strings.TrimSpace(input.TaskID),
		Status:     UploadNotificationStatusSkipped,
		TopEntries: cloneUploadNotificationTopEntries(input.TopEntries),
	}
	if input.Cloudflare != nil {
		notification.CloudflareStatus = normalizeUploadNotificationStatus(input.Cloudflare.Status)
		notification.CloudflareUploadCount = max(0, input.Cloudflare.UploadCount)
	}
	if input.GitHub != nil {
		notification.GitHubStatus = normalizeUploadNotificationStatus(input.GitHub.Status)
		notification.GitHubUploadCount = max(0, input.GitHub.UploadCount)
	}
	notification.Status = uploadNotificationOverallStatus(notification.CloudflareStatus, notification.GitHubStatus)
	if notification.Message == "" {
		notification.Message = uploadNotificationDefaultMessage(notification)
	}
	return notification
}

func BuildUploadNotificationFromCommandResult(input CommandResultUploadNotificationInput) UploadNotification {
	report := UploadProviderReportFromCommandResult(input.Provider, input.Result)
	notificationInput := UploadNotificationInput{
		CreatedAt:  input.CreatedAt,
		Message:    input.Result.Message,
		Source:     input.Source,
		TaskID:     input.TaskID,
		TopEntries: input.TopEntries,
	}
	switch input.Provider {
	case UploadNotificationProviderCloudflare:
		notificationInput.Cloudflare = &report
	case UploadNotificationProviderGitHub:
		notificationInput.GitHub = &report
	}
	return BuildUploadNotification(notificationInput)
}

func BuildPostProbeNoRowsUploadNotification(cfg PostProbePushConfig, taskID string) *UploadNotification {
	cloudflareReport, githubReport := skippedUploadReportsForEnabledPostProbePushProviders(cfg)
	if cloudflareReport == nil && githubReport == nil {
		return nil
	}
	notification := BuildUploadNotification(UploadNotificationInput{
		Cloudflare: cloudflareReport,
		CreatedAt:  time.Now(),
		GitHub:     githubReport,
		Message:    "没有可上传结果，测速后自动上传已跳过。",
		Source:     UploadNotificationSourcePostProbePush,
		TaskID:     taskID,
	})
	return &notification
}

func UploadNotificationHasFailure(notification UploadNotification) bool {
	switch normalizeUploadNotificationStatus(notification.Status) {
	case UploadNotificationStatusFailed, UploadNotificationStatusPartial:
		return true
	}
	for _, status := range []string{notification.CloudflareStatus, notification.GitHubStatus} {
		switch normalizeUploadNotificationStatus(status) {
		case UploadNotificationStatusFailed, UploadNotificationStatusPartial:
			return true
		}
	}
	return false
}

func TaskFailureNotificationInputFromUploadNotification(notification UploadNotification) TaskFailureNotificationInput {
	createdAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(notification.CreatedAt))
	return TaskFailureNotificationInput{
		CreatedAt: createdAt,
		Message:   uploadFailureTaskReason(notification),
		Stage:     "upload",
		TaskID:    notification.TaskID,
	}
}

func skippedUploadReportsForEnabledPostProbePushProviders(cfg PostProbePushConfig) (*UploadProviderReport, *UploadProviderReport) {
	var cloudflareReport *UploadProviderReport
	var githubReport *UploadProviderReport
	if cfg.CloudflareEnabled {
		cloudflareReport = &UploadProviderReport{Status: UploadNotificationStatusSkipped}
	}
	if cfg.GitHubEnabled {
		githubReport = &UploadProviderReport{Status: UploadNotificationStatusSkipped}
	}
	return cloudflareReport, githubReport
}

func UploadProviderReportFromCommandResult(provider string, result CommandResult) UploadProviderReport {
	status := UploadNotificationStatusCompleted
	if !result.OK {
		status = UploadNotificationStatusFailed
		if strings.Contains(result.Code, "INPUT_EMPTY") {
			status = UploadNotificationStatusSkipped
		}
	}
	if strings.Contains(result.Code, "PARTIAL") {
		status = UploadNotificationStatusPartial
	}
	return UploadProviderReport{
		Status:      status,
		UploadCount: uploadNotificationCountFromCommandData(provider, result.Data),
	}
}

func CommandResultDataWithUploadNotification(data any, notification UploadNotification) any {
	mapped := commandResultDataMap(data)
	if len(mapped) == 0 {
		mapped = map[string]any{}
	}
	mapped["upload_notification"] = notification
	return mapped
}

func CommandResultTaskID(payload map[string]any, result CommandResult) string {
	if result.TaskID != nil {
		return strings.TrimSpace(*result.TaskID)
	}
	return strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
}

func UploadNotificationTriggerFromPayload(payload map[string]any) string {
	return strings.TrimSpace(stringValue(firstNonNil(payload["notification_trigger"], payload["notificationTrigger"]), ""))
}

func UploadNotificationEventPayload(notification UploadNotification) map[string]any {
	raw, err := json.Marshal(notification)
	if err != nil {
		return uploadNotificationFallbackPayload(notification)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return uploadNotificationFallbackPayload(notification)
	}
	return payload
}

func UploadNotificationText(notification UploadNotification) string {
	lines := uploadNotificationSummaryLines(notification)
	if len(notification.TopEntries) > 0 {
		lines = append(lines, uploadNotificationTopNLines(notification)...)
	}
	if message := strings.TrimSpace(notification.Message); message != "" {
		lines = append(lines, "消息："+truncateTelegramLine(message, 700))
	}
	return strings.Join(lines, "\n")
}

func UploadNotificationSummaryText(notification UploadNotification) string {
	lines := uploadNotificationSummaryLines(notification)
	if message := strings.TrimSpace(notification.Message); message != "" {
		lines = append(lines, "消息："+truncateTelegramLine(message, 700))
	}
	return strings.Join(lines, "\n")
}

func UploadNotificationTopNText(notification UploadNotification) string {
	lines := []string{
		"CFST Top N 上传列表",
		"来源：" + UploadNotificationSourceLabel(notification.Source),
	}
	if taskID := strings.TrimSpace(notification.TaskID); taskID != "" {
		lines = append(lines, "任务："+taskID)
	}
	if createdAt := strings.TrimSpace(notification.CreatedAt); createdAt != "" {
		lines = append(lines, "时间："+createdAt)
	}
	lines = append(lines, uploadNotificationTopNLines(notification)...)
	return truncateTelegramMessage(strings.Join(lines, "\n"), MaxTelegramMessageLength)
}

func uploadFailureTaskReason(notification UploadNotification) string {
	parts := []string{"上传失败：" + UploadNotificationSourceLabel(notification.Source)}
	if line := uploadFailureProviderLine("Cloudflare", notification.CloudflareStatus, notification.CloudflareUploadCount); line != "" {
		parts = append(parts, line)
	}
	if line := uploadFailureProviderLine("GitHub", notification.GitHubStatus, notification.GitHubUploadCount); line != "" {
		parts = append(parts, line)
	}
	if len(parts) == 1 {
		parts = append(parts, "状态："+UploadNotificationStatusLabel(notification.Status))
	}
	if message := strings.TrimSpace(notification.Message); message != "" {
		parts = append(parts, truncateTelegramLine(message, 160))
	}
	return strings.Join(parts, "；")
}

func uploadFailureProviderLine(provider string, status string, uploadCount int) string {
	switch normalizeUploadNotificationStatus(status) {
	case UploadNotificationStatusFailed, UploadNotificationStatusPartial:
		return fmt.Sprintf("%s：%s，上传 %d 条", provider, UploadNotificationStatusLabel(status), max(0, uploadCount))
	default:
		return ""
	}
}

func TaskFailureNotificationText(input TaskFailureNotificationInput) string {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	reason := strings.TrimSpace(input.Message)
	if reason == "" {
		reason = "任务异常结束，未提供详细原因。"
	}
	lines := []string{
		"CFST 任务失败",
		"状态：失败",
	}
	if taskID := strings.TrimSpace(input.TaskID); taskID != "" {
		lines = append(lines, "任务："+taskID)
	}
	if stage := strings.TrimSpace(input.Stage); stage != "" {
		lines = append(lines, "阶段："+stage)
	}
	lines = append(lines,
		"原因："+truncateTelegramLine(reason, 300),
		"时间："+createdAt.Format(time.RFC3339),
	)
	return truncateTelegramMessage(strings.Join(lines, "\n"), MaxTelegramMessageLength)
}

func UploadNotificationStatusLabel(status string) string {
	switch normalizeUploadNotificationStatus(status) {
	case UploadNotificationStatusCompleted:
		return "完成"
	case UploadNotificationStatusFailed:
		return "失败"
	case UploadNotificationStatusPartial:
		return "部分完成"
	case UploadNotificationStatusUnsupported:
		return "暂不支持"
	default:
		return "跳过"
	}
}

func UploadNotificationSourceLabel(source string) string {
	switch strings.TrimSpace(source) {
	case UploadNotificationSourceManualPush:
		return "手动推送"
	case UploadNotificationSourcePostProbePush:
		return "手动测速后自动上传"
	case UploadNotificationSourceScheduledProbe:
		return "定时任务自动上传"
	case UploadNotificationSourceScheduledPipeline:
		return "定时工作流自动上传"
	default:
		if strings.TrimSpace(source) == "" {
			return "上传任务"
		}
		return strings.TrimSpace(source)
	}
}

func SendTelegramUploadNotification(ctx context.Context, snapshot map[string]any, notification UploadNotification, client *http.Client, apiBaseURL string) error {
	cfg := TelegramNotificationConfigFromSnapshot(snapshot)
	if !cfg.Enabled {
		return nil
	}
	ctx, cancel := contextWithTelegramNotificationTimeout(ctx)
	defer cancel()
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
	cfg.BotToken = strings.TrimSpace(cfg.BotToken)
	if cfg.BotToken == "" || IsMaskedSecret(cfg.BotToken) {
		return nil, errors.New("Telegram 通知配置不完整")
	}
	receipts, err := telegramNotificationTestReceipts(cfg)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	apiBaseURL = strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	if apiBaseURL == "" {
		apiBaseURL = TelegramAPIBaseURL
	}
	chatIDs := make([]string, 0, len(receipts))
	failures := make([]string, 0, len(receipts))
	for _, receipt := range receipts {
		chatIDs = append(chatIDs, receipt.ChatID)
		if err := sendTelegramMessageToChat(ctx, cfg.BotToken, receipt.ChatID, telegramNotificationTestReceiptText(receipt), client, apiBaseURL); err != nil {
			failures = append(failures, fmt.Sprintf("%s：%v", receipt.ChatID, err))
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
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	apiBaseURL = strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	if apiBaseURL == "" {
		apiBaseURL = TelegramAPIBaseURL
	}
	failures := make([]string, 0)
	for _, chatID := range chatIDs {
		if err := sendTelegramMessageToChat(ctx, cfg.BotToken, chatID, text, client, apiBaseURL); err != nil {
			failures = append(failures, fmt.Sprintf("%s：%v", chatID, err))
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "；"))
	}
	return nil
}

func contextWithTelegramNotificationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, TelegramNotificationSendTimeout)
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
	receipts := make([]telegramNotificationTestReceipt, 0, len(uploadChatIDs)+len(topNChatIDs))
	indexByChatID := make(map[string]int, len(uploadChatIDs)+len(topNChatIDs))
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

func BuildUploadNotificationTopEntries(rows []probecore.ProbeRow, limit int, metric string) []UploadNotificationTopEntry {
	limit = normalizeTelegramTopN(limit)
	if limit <= 0 || len(rows) == 0 {
		return nil
	}
	cloned := make([]probecore.ProbeRow, len(rows))
	copy(cloned, rows)
	topRows := probecore.SelectTopProbeRowsByMetric(cloned, limit, metric)
	entries := make([]UploadNotificationTopEntry, 0, len(topRows))
	for _, row := range topRows {
		ip := strings.TrimSpace(row.IP)
		if ip == "" {
			continue
		}
		entries = append(entries, UploadNotificationTopEntry{
			Colo:            strings.TrimSpace(row.Colo),
			DownloadSpeedMB: uploadNotificationSpeedForMetric(row, metric),
			IP:              ip,
			Rank:            len(entries) + 1,
			TCPDelayMS:      row.DelayMS,
			TraceDelayMS:    row.TraceDelayMS,
		})
	}
	return entries
}

func BuildUploadNotificationTopEntriesForSnapshot(snapshot map[string]any, rows []probecore.ProbeRow, metric string) []UploadNotificationTopEntry {
	cfg := TelegramNotificationConfigFromSnapshot(snapshot)
	if !cfg.IncludeTopN {
		return nil
	}
	return BuildUploadNotificationTopEntries(rows, cfg.TopN, metric)
}

func sendTelegramMessageToChat(ctx context.Context, botToken string, chatID string, text string, client *http.Client, apiBaseURL string) error {
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
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
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
	return fmt.Errorf("Telegram 通知发送失败：HTTP %d：%s", res.StatusCode, truncateTelegramLine(message, 300))
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

func normalizeUploadNotificationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case UploadNotificationStatusCompleted, "ok", "success", "succeeded":
		return UploadNotificationStatusCompleted
	case UploadNotificationStatusFailed, "error":
		return UploadNotificationStatusFailed
	case UploadNotificationStatusPartial:
		return UploadNotificationStatusPartial
	case UploadNotificationStatusUnsupported:
		return UploadNotificationStatusUnsupported
	default:
		return UploadNotificationStatusSkipped
	}
}

func uploadNotificationOverallStatus(statuses ...string) string {
	success := 0
	failed := 0
	for _, status := range statuses {
		switch normalizeUploadNotificationStatus(status) {
		case UploadNotificationStatusCompleted:
			success++
		case UploadNotificationStatusPartial:
			success++
			failed++
		case UploadNotificationStatusFailed:
			failed++
		}
	}
	switch {
	case success > 0 && failed > 0:
		return UploadNotificationStatusPartial
	case failed > 0:
		return UploadNotificationStatusFailed
	case success > 0:
		return UploadNotificationStatusCompleted
	default:
		return UploadNotificationStatusSkipped
	}
}

func uploadNotificationDefaultMessage(notification UploadNotification) string {
	switch notification.Status {
	case UploadNotificationStatusCompleted:
		return "上传流程已完成。"
	case UploadNotificationStatusPartial:
		return "上传流程部分完成。"
	case UploadNotificationStatusFailed:
		return "上传流程失败。"
	default:
		return "没有执行上传或没有可上传结果。"
	}
}

func truncateTelegramLine(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}

func truncateTelegramMessage(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	if limit <= 3 {
		return string(runes[:limit])
	}
	return strings.TrimSpace(string(runes[:limit-3])) + "..."
}

func uploadNotificationSummaryLines(notification UploadNotification) []string {
	lines := []string{
		"CFST 上传通知",
		"来源：" + UploadNotificationSourceLabel(notification.Source),
		"状态：" + UploadNotificationStatusLabel(notification.Status),
	}
	if strings.TrimSpace(notification.CloudflareStatus) != "" {
		lines = append(lines, fmt.Sprintf("Cloudflare：%s，上传 %d 条", UploadNotificationStatusLabel(notification.CloudflareStatus), notification.CloudflareUploadCount))
	}
	if strings.TrimSpace(notification.GitHubStatus) != "" {
		lines = append(lines, fmt.Sprintf("GitHub：%s，上传 %d 条", UploadNotificationStatusLabel(notification.GitHubStatus), notification.GitHubUploadCount))
	}
	if taskID := strings.TrimSpace(notification.TaskID); taskID != "" {
		lines = append(lines, "任务："+taskID)
	}
	if createdAt := strings.TrimSpace(notification.CreatedAt); createdAt != "" {
		lines = append(lines, "时间："+createdAt)
	}
	return lines
}

func uploadNotificationTopNLines(notification UploadNotification) []string {
	if len(notification.TopEntries) == 0 {
		return nil
	}
	lines := []string{fmt.Sprintf("Top %d：", len(notification.TopEntries))}
	for index, entry := range notification.TopEntries {
		if line := uploadNotificationTopEntryText(index, entry); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func uploadNotificationTopEntryText(index int, entry UploadNotificationTopEntry) string {
	ip := strings.TrimSpace(entry.IP)
	if ip == "" {
		return ""
	}
	rank := entry.Rank
	if rank <= 0 {
		rank = index + 1
	}
	line := fmt.Sprintf("%d. %s", rank, ip)
	if colo := strings.TrimSpace(entry.Colo); colo != "" && !strings.EqualFold(colo, "N/A") {
		line += " " + strings.ToUpper(colo)
	}
	metrics := make([]string, 0, 3)
	if entry.TCPDelayMS > 0 {
		metrics = append(metrics, fmt.Sprintf("TCP %.2fms", entry.TCPDelayMS))
	}
	if entry.TraceDelayMS > 0 {
		metrics = append(metrics, fmt.Sprintf("追踪 %.2fms", entry.TraceDelayMS))
	}
	if entry.DownloadSpeedMB > 0 {
		metrics = append(metrics, fmt.Sprintf("下载 %.2f MB/s", entry.DownloadSpeedMB))
	}
	if len(metrics) > 0 {
		line += "（" + strings.Join(metrics, "，") + "）"
	}
	return truncateTelegramLine(line, 700)
}

func uploadNotificationSpeedForMetric(row probecore.ProbeRow, metric string) float64 {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "max", "peak", "highest":
		return row.MaxDownloadSpeedMB
	default:
		return row.DownloadSpeedMB
	}
}

func cloneUploadNotificationTopEntries(entries []UploadNotificationTopEntry) []UploadNotificationTopEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]UploadNotificationTopEntry, len(entries))
	copy(cloned, entries)
	return cloned
}

func uploadNotificationCountFromCommandData(provider string, data any) int {
	mapped := commandResultDataMap(data)
	switch provider {
	case UploadNotificationProviderGitHub:
		return intValue(firstNonNil(mapped["written_rows"], mapped["writtenRows"]), 0)
	default:
		return intValue(firstNonNil(mapped["upload_count"], mapped["uploadCount"], mapped["cloudflare_count"], mapped["cloudflareCount"]), 0)
	}
}

func commandResultDataMap(data any) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	if mapped, ok := data.(map[string]any); ok {
		return cloneAnyMap(mapped)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return map[string]any{"result": data}
	}
	var mapped map[string]any
	if err := json.Unmarshal(raw, &mapped); err != nil || mapped == nil {
		return map[string]any{"result": data}
	}
	return mapped
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func uploadNotificationFallbackPayload(notification UploadNotification) map[string]any {
	return map[string]any{
		"message": notification.Message,
		"source":  notification.Source,
		"status":  notification.Status,
		"task_id": notification.TaskID,
	}
}
