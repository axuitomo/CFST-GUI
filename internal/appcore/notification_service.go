package appcore

import (
	"context"
	"strings"
	"time"
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
	cfg := TelegramNotificationConfigFromSnapshot(config)
	cfg.Enabled = true
	s.mu.RLock()
	client, baseURL := s.options.HTTPClient, s.options.TelegramAPIBaseURL
	s.mu.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	chatIDs, err := SendTelegramTestNotification(ctx, cfg, client, baseURL)
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

func (s *Service) sendUploadNotification(ctx context.Context, snapshot map[string]any, notification UploadNotification) []string {
	if strings.TrimSpace(notification.Status) == "" || ctx.Err() != nil {
		return nil
	}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{
		Event: "upload.notification", TaskID: notification.TaskID, Payload: UploadNotificationEventPayload(notification),
	})
	s.mu.RLock()
	client, baseURL := s.options.HTTPClient, s.options.TelegramAPIBaseURL
	s.mu.RUnlock()
	warnings := make([]string, 0, 2)
	if err := SendTelegramUploadNotification(ctx, snapshot, notification, client, baseURL); err != nil {
		warnings = append(warnings, "Telegram notification failed: "+err.Error())
	}
	if ctx.Err() == nil && UploadNotificationHasFailure(notification) {
		input := TaskFailureNotificationInputFromUploadNotification(notification)
		if err := SendTelegramTaskFailureNotification(ctx, snapshot, input, client, baseURL); err != nil {
			warnings = append(warnings, "Telegram task failure notification failed: "+err.Error())
		}
	}
	return dedupeStrings(warnings)
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
	input := TaskFailureNotificationInput{
		CreatedAt: s.now(),
		Message:   taskFailureNotificationMessage(payload),
		Stage:     strings.TrimSpace(stringValue(firstNonNil(payload["failure_stage"], payload["stage"], payload["current_stage"]), "")),
		TaskID:    taskID,
	}
	s.mu.RLock()
	client, baseURL := s.options.HTTPClient, s.options.TelegramAPIBaseURL
	s.mu.RUnlock()
	if err := SendTelegramTaskFailureNotification(ctx, snapshot, input, client, baseURL); err != nil {
		s.DebugLogger().Event("telegram.task_failure_notification_failed", map[string]any{
			"message": err.Error(),
			"task_id": taskID,
		})
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

func (s *Service) RecordSchedulerUploadNotification(ctx context.Context, snapshot map[string]any, status SchedulerStatus, source string, taskID string, includeCloudflare bool, includeGitHub bool, topEntries []UploadNotificationTopEntry) (SchedulerStatus, []string) {
	if !includeCloudflare && !includeGitHub {
		return status, nil
	}
	notification := BuildUploadNotification(UploadNotificationInput{
		Cloudflare: schedulerUploadProviderReport(includeCloudflare, status.LastProbeStatus, status.LastDNSStatus, status.CloudflareUploadCount),
		CreatedAt:  s.now(),
		GitHub:     schedulerUploadProviderReport(includeGitHub, status.LastProbeStatus, status.LastGitHubStatus, status.GitHubUploadCount),
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

func schedulerUploadProviderReport(include bool, probeStatus string, providerStatus string, uploadCount int) *UploadProviderReport {
	if !include {
		return nil
	}
	status := strings.TrimSpace(providerStatus)
	if status == "" {
		status = UploadNotificationStatusSkipped
	}
	if probeStatus == "failed" && status == UploadNotificationStatusSkipped {
		status = UploadNotificationStatusFailed
	}
	return &UploadProviderReport{Status: status, UploadCount: uploadCount}
}
