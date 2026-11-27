package appcore

import (
	"context"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/notify"
)

func (s *Service) invokeTelegramTest(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("TELEGRAM_NOTIFICATION_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(config) == 0 {
		config = payload
	}
	cfg := notify.TelegramNotificationConfigFromSnapshot(config)
	cfg.Enabled = true
	s.mu.RLock()
	client, baseURL := s.options.HTTPClient, s.options.TelegramAPIBaseURL
	s.mu.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	chatIDs, err := notify.SendTelegramTestNotification(ctx, cfg, client, baseURL)
	if err != nil {
		return NewCommandResult("TELEGRAM_NOTIFICATION_TEST_FAILED", nil, err.Error(), false, nil, nil)
	}
	chatID := ""
	if len(chatIDs) > 0 {
		chatID = chatIDs[0]
	}
	return NewCommandResult("TELEGRAM_NOTIFICATION_TEST_OK", map[string]any{
		"chat_id": chatID, "chat_ids": chatIDs,
	}, "Telegram notification test sent.", true, nil, nil)
}

func (s *Service) sendUploadNotification(ctx context.Context, snapshot map[string]any, notification contracts.UploadNotification) []string {
	if strings.TrimSpace(notification.Status) == "" || ctx.Err() != nil {
		return nil
	}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{
		Event: UploadNotificationEvent, TaskID: notification.TaskID, Payload: notify.UploadNotificationEventPayload(notification),
	})
	s.mu.RLock()
	client, baseURL := s.options.HTTPClient, s.options.TelegramAPIBaseURL
	s.mu.RUnlock()
	warnings := make([]string, 0, 2)
	if err := notify.SendTelegramUploadNotification(ctx, snapshot, notification, client, baseURL); err != nil {
		warnings = append(warnings, "Telegram notification failed: "+err.Error())
	}
	if err := notify.SendWebhookUploadNotification(ctx, snapshot, notification, client); err != nil {
		warnings = append(warnings, "Webhook notification failed: "+err.Error())
	}
	if err := notify.SendEmailUploadNotification(ctx, snapshot, notification); err != nil {
		warnings = append(warnings, "Email notification failed: "+err.Error())
	}
	if ctx.Err() == nil && notify.UploadNotificationHasFailure(notification) {
		input := notify.TaskFailureNotificationInputFromUploadNotification(notification)
		if err := notify.SendTelegramTaskFailureNotification(ctx, snapshot, input, client, baseURL); err != nil {
			warnings = append(warnings, "Telegram task failure notification failed: "+err.Error())
		}
		if err := notify.SendWebhookTaskFailureNotification(ctx, snapshot, input, client); err != nil {
			warnings = append(warnings, "Webhook task failure notification failed: "+err.Error())
		}
		if err := notify.SendEmailTaskFailureNotification(ctx, snapshot, input); err != nil {
			warnings = append(warnings, "Email task failure notification failed: "+err.Error())
		}
	}
	return notify.DedupeStrings(warnings)
}

func (s *Service) recordTaskFailureNotification(ctx context.Context, taskID string, payload map[string]any) {
	loaded, err := s.LoadConfig()
	snapshot := loaded.Snapshot
	if err != nil || snapshot == nil {
		s.mu.RLock()
		policy := s.options.ConfigPolicy
		s.mu.RUnlock()
		snapshot = sanitizeServiceConfig(nil, policy)
	}
	input := notify.TaskFailureNotificationInput{
		CreatedAt: s.now(),
		Message:   taskFailureNotificationMessage(payload),
		Stage:     strings.TrimSpace(stringValue(firstNonNil(payload["failure_stage"], payload["stage"], payload["current_stage"]), "")),
		TaskID:    taskID,
	}
	s.mu.RLock()
	client, baseURL := s.options.HTTPClient, s.options.TelegramAPIBaseURL
	s.mu.RUnlock()
	if err := notify.SendTelegramTaskFailureNotification(ctx, snapshot, input, client, baseURL); err != nil {
		s.DebugLogger().Event("telegram.task_failure_notification_failed", map[string]any{"message": err.Error(), "task_id": taskID})
	}
	if err := notify.SendWebhookTaskFailureNotification(ctx, snapshot, input, client); err != nil {
		s.DebugLogger().Event("webhook.task_failure_notification_failed", map[string]any{"message": err.Error(), "task_id": taskID})
	}
	if err := notify.SendEmailTaskFailureNotification(ctx, snapshot, input); err != nil {
		s.DebugLogger().Event("email.task_failure_notification_failed", map[string]any{"message": err.Error(), "task_id": taskID})
	}
}

func taskFailureNotificationMessage(payload map[string]any) string {
	message := strings.TrimSpace(stringValue(firstNonNil(payload["message"], payload["error"], payload["reason"]), ""))
	if message != "" {
		return message
	}
	failureSummary := mapValue(payload["failure_summary"])
	return strings.TrimSpace(stringValue(failureSummary["recovery_status"], ""))
}

func (s *Service) RecordSchedulerUploadNotification(ctx context.Context, snapshot map[string]any, status SchedulerStatus, source string, taskID string, includeCloudflare bool, includeGitHub bool, topEntries []contracts.UploadNotificationTopEntry, funnel *contracts.UploadNotificationFunnel, durationMS int64) (SchedulerStatus, []string) {
	if !includeCloudflare && !includeGitHub {
		return status, nil
	}
	notification := notify.BuildUploadNotification(notify.UploadNotificationInput{
		CreatedAt:  s.now(),
		DurationMS: durationMS,
		Funnel:     funnel,
		GitHub:     schedulerUploadProviderReport(includeGitHub, status.LastProbeStatus, status.LastGitHubStatus, status.GitHubUploadCount),
		Providers:  schedulerDNSProviderReports(includeCloudflare, status),
		Source:     source,
		TaskID:     taskID,
		TopEntries: topEntries,
	})
	warnings := s.sendUploadNotification(ctx, snapshot, notification)
	status.UploadNotification = &notification
	if len(warnings) > 0 {
		status.LastMessage = strings.TrimSpace(strings.Join(append([]string{status.LastMessage}, warnings...), " "))
	}
	return status, warnings
}

func schedulerDNSProviderReports(include bool, status SchedulerStatus) []contracts.UploadProviderResult {
	if !include {
		return nil
	}
	if len(status.DNSProviders) == 0 {
		provider := contracts.UploadProviderResult{ID: "cloudflare", Label: "Cloudflare", Provider: DNSProviderCloudflare, Status: status.LastDNSStatus, UploadCount: status.CloudflareUploadCount}
		if status.LastProbeStatus == "failed" && notify.NormalizeUploadNotificationStatus(provider.Status) == notify.UploadNotificationStatusSkipped {
			provider.Status = notify.UploadNotificationStatusFailed
		}
		return []contracts.UploadProviderResult{provider}
	}
	reports := make([]contracts.UploadProviderResult, 0, len(status.DNSProviders))
	for _, provider := range status.DNSProviders {
		if strings.TrimSpace(provider.Status) == "" {
			provider.Status = status.LastDNSStatus
		}
		if status.LastProbeStatus == "failed" && notify.NormalizeUploadNotificationStatus(provider.Status) == notify.UploadNotificationStatusSkipped {
			provider.Status = notify.UploadNotificationStatusFailed
		}
		reports = append(reports, provider)
	}
	return reports
}

func schedulerUploadProviderReport(include bool, probeStatus string, providerStatus string, uploadCount int) *contracts.UploadProviderReport {
	if !include {
		return nil
	}
	status := strings.TrimSpace(providerStatus)
	if status == "" {
		status = notify.UploadNotificationStatusSkipped
	}
	if probeStatus == "failed" && status == notify.UploadNotificationStatusSkipped {
		status = notify.UploadNotificationStatusFailed
	}
	return &contracts.UploadProviderReport{Status: status, UploadCount: uploadCount}
}
