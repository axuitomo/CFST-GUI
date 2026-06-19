package mobileapi

import (
	"context"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func (s *Service) attachManualUploadNotification(payload map[string]any, provider string, response string) string {
	if appcore.UploadNotificationTriggerFromPayload(payload) != appcore.UploadNotificationSourceManualPush {
		return response
	}
	command := decodeCommandResult(response)
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	notification := appcore.BuildUploadNotificationFromCommandResult(appcore.CommandResultUploadNotificationInput{
		CreatedAt: time.Now(),
		Provider:  provider,
		Result:    command,
		Source:    appcore.UploadNotificationSourceManualPush,
		TaskID:    appcore.CommandResultTaskID(payload, command),
	})
	warnings := s.recordUploadNotification(config, notification)
	command.Data = appcore.CommandResultDataWithUploadNotification(command.Data, notification)
	command.Warnings = dedupeStrings(append(command.Warnings, warnings...))
	return encodeCommand(command)
}

func (s *Service) recordUploadNotification(snapshot map[string]any, notification appcore.UploadNotification) []string {
	if strings.TrimSpace(notification.Status) == "" {
		return nil
	}
	s.emit(notification.TaskID, "upload.notification", appcore.UploadNotificationEventPayload(notification))
	if err := appcore.SendTelegramUploadNotification(context.Background(), snapshot, notification, nil, ""); err != nil {
		return []string{"Telegram 通知发送失败：" + err.Error()}
	}
	return nil
}

func (s *Service) TestTelegramNotification(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("TELEGRAM_NOTIFICATION_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(config) == 0 {
		config = payload
	}
	cfg := appcore.TelegramNotificationConfigFromSnapshot(config)
	cfg.Enabled = true
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := appcore.SendTelegramMessage(ctx, cfg, "CFST 上传通知测试\n状态：Telegram 通知渠道可用。", nil, ""); err != nil {
		return encodeCommand(commandResultFor("TELEGRAM_NOTIFICATION_TEST_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("TELEGRAM_NOTIFICATION_TEST_OK", map[string]any{
		"chat_id": cfg.ChatID,
	}, "Telegram 通知测试已发送。", true, nil, nil))
}

func (s *Service) recordSchedulerUploadNotification(snapshot map[string]any, source string, taskID string, includeCloudflare bool, includeGitHub bool) (mobileSchedulerStatus, []string) {
	status := s.currentSchedulerStatus()
	if !includeCloudflare && !includeGitHub {
		return status, nil
	}
	var cloudflareReport *appcore.UploadProviderReport
	var githubReport *appcore.UploadProviderReport
	if includeCloudflare {
		cloudflareReport = &appcore.UploadProviderReport{
			Status:      mobileFirstNonEmpty(status.LastDNSStatus, appcore.UploadNotificationStatusSkipped),
			UploadCount: status.CloudflareUploadCount,
		}
	}
	if includeGitHub {
		githubReport = &appcore.UploadProviderReport{
			Status:      mobileFirstNonEmpty(status.LastGitHubStatus, appcore.UploadNotificationStatusSkipped),
			UploadCount: status.GitHubUploadCount,
		}
	}
	notification := appcore.BuildUploadNotification(appcore.UploadNotificationInput{
		Cloudflare: cloudflareReport,
		CreatedAt:  time.Now(),
		GitHub:     githubReport,
		Source:     source,
		TaskID:     taskID,
	})
	warnings := s.recordUploadNotification(snapshot, notification)
	status.UploadNotification = &notification
	if len(warnings) > 0 {
		status.LastMessage = mobileUploadNotificationMessage(status.LastMessage, warnings)
	}
	_ = s.writeSchedulerStatus(status)
	return status, warnings
}

func mobileUploadNotificationMessage(message string, warnings []string) string {
	message = strings.TrimSpace(message)
	if len(warnings) == 0 {
		return message
	}
	if message == "" {
		return strings.Join(warnings, " ")
	}
	return message + " " + strings.Join(warnings, " ")
}
