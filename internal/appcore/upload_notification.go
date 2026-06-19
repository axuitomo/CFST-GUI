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
)

type TelegramNotificationConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type UploadProviderReport struct {
	Status      string `json:"status"`
	UploadCount int    `json:"upload_count"`
}

type UploadNotification struct {
	CloudflareStatus      string `json:"cloudflare_status,omitempty"`
	CloudflareUploadCount int    `json:"cloudflare_upload_count,omitempty"`
	CreatedAt             string `json:"created_at"`
	GitHubStatus          string `json:"github_status,omitempty"`
	GitHubUploadCount     int    `json:"github_upload_count,omitempty"`
	Message               string `json:"message"`
	Source                string `json:"source"`
	Status                string `json:"status"`
	TaskID                string `json:"task_id,omitempty"`
}

type UploadNotificationInput struct {
	Cloudflare *UploadProviderReport
	CreatedAt  time.Time
	GitHub     *UploadProviderReport
	Message    string
	Source     string
	TaskID     string
}

type CommandResultUploadNotificationInput struct {
	CreatedAt time.Time
	Provider  string
	Result    CommandResult
	Source    string
	TaskID    string
}

func TelegramNotificationConfigFromSnapshot(snapshot map[string]any) TelegramNotificationConfig {
	notifications := mapValue(firstNonNil(snapshot["notifications"], snapshot["notification"]))
	telegram := mapValue(firstNonNil(notifications["telegram"], notifications["tg"], snapshot["telegram"]))
	return TelegramNotificationConfig{
		Enabled:  boolValue(firstNonNil(telegram["enabled"], telegram["telegram_enabled"], telegram["telegramEnabled"]), false),
		BotToken: strings.TrimSpace(stringValue(firstNonNil(telegram["bot_token"], telegram["botToken"], telegram["token"]), "")),
		ChatID:   strings.TrimSpace(stringValue(firstNonNil(telegram["chat_id"], telegram["chatId"], telegram["chat"]), "")),
	}
}

func BuildUploadNotification(input UploadNotificationInput) UploadNotification {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	notification := UploadNotification{
		CreatedAt: createdAt.Format(time.RFC3339),
		Message:   strings.TrimSpace(input.Message),
		Source:    strings.TrimSpace(input.Source),
		TaskID:    strings.TrimSpace(input.TaskID),
		Status:    UploadNotificationStatusSkipped,
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
		CreatedAt: input.CreatedAt,
		Message:   input.Result.Message,
		Source:    input.Source,
		TaskID:    input.TaskID,
	}
	switch input.Provider {
	case UploadNotificationProviderCloudflare:
		notificationInput.Cloudflare = &report
	case UploadNotificationProviderGitHub:
		notificationInput.GitHub = &report
	}
	return BuildUploadNotification(notificationInput)
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
	if message := strings.TrimSpace(notification.Message); message != "" {
		lines = append(lines, "消息："+truncateTelegramLine(message, 700))
	}
	return strings.Join(lines, "\n")
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
	return SendTelegramMessage(ctx, cfg, UploadNotificationText(notification), client, apiBaseURL)
}

func SendTelegramMessage(ctx context.Context, cfg TelegramNotificationConfig, text string, client *http.Client, apiBaseURL string) error {
	if !cfg.Enabled {
		return nil
	}
	cfg.BotToken = strings.TrimSpace(cfg.BotToken)
	cfg.ChatID = strings.TrimSpace(cfg.ChatID)
	if cfg.BotToken == "" || cfg.ChatID == "" || IsMaskedSecret(cfg.BotToken) {
		return errors.New("Telegram 通知配置不完整")
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
	body, err := json.Marshal(map[string]any{
		"chat_id":                  cfg.ChatID,
		"disable_web_page_preview": true,
		"text":                     text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/bot"+cfg.BotToken+"/sendMessage", bytes.NewReader(body))
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
