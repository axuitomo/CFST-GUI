package mobileapi

import (
	"context"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func (s *Service) attachManualUploadNotification(payload map[string]any, provider string, response string) string {
	if appcore.UploadNotificationTriggerFromPayload(payload) != appcore.UploadNotificationSourceManualPush {
		return response
	}
	command := decodeCommandResult(response)
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	notification := appcore.BuildUploadNotificationFromCommandResult(appcore.CommandResultUploadNotificationInput{
		CreatedAt:  time.Now(),
		Provider:   provider,
		Result:     command,
		Source:     appcore.UploadNotificationSourceManualPush,
		TaskID:     appcore.CommandResultTaskID(payload, command),
		TopEntries: s.manualUploadNotificationTopEntries(payload),
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
	warnings := []string(nil)
	if err := appcore.SendTelegramUploadNotification(context.Background(), snapshot, notification, nil, ""); err != nil {
		warnings = append(warnings, "Telegram 通知发送失败："+err.Error())
	}
	if appcore.UploadNotificationHasFailure(notification) {
		input := appcore.TaskFailureNotificationInputFromUploadNotification(notification)
		if err := appcore.SendTelegramTaskFailureNotification(context.Background(), snapshot, input, nil, ""); err != nil {
			warnings = append(warnings, "Telegram 任务失败通知发送失败："+err.Error())
		}
	}
	return warnings
}

func (s *Service) recordTaskFailureNotification(taskID string, payload map[string]any) {
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		snapshot = defaultConfigSnapshot()
	}
	input := appcore.TaskFailureNotificationInput{
		CreatedAt: time.Now(),
		Message:   taskFailureNotificationMessage(payload),
		Stage:     strings.TrimSpace(stringValue(firstNonNil(payload["failure_stage"], payload["stage"], payload["current_stage"]), "")),
		TaskID:    taskID,
	}
	if err := appcore.SendTelegramTaskFailureNotification(context.Background(), snapshot, input, nil, ""); err != nil {
		_ = utils.AppendErrorLog(s.errorLogPath(), "telegram.task_failure_notification_failed", map[string]any{
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
	if recoveryStatus := strings.TrimSpace(stringValue(failureSummary["recovery_status"], "")); recoveryStatus != "" {
		return recoveryStatus
	}
	return ""
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
	chatIDs, err := appcore.SendTelegramTestNotification(ctx, cfg, nil, "")
	if err != nil {
		return encodeCommand(commandResultFor("TELEGRAM_NOTIFICATION_TEST_FAILED", nil, err.Error(), false, nil, nil))
	}
	chatID := ""
	if len(chatIDs) > 0 {
		chatID = chatIDs[0]
	}
	return encodeCommand(commandResultFor("TELEGRAM_NOTIFICATION_TEST_OK", map[string]any{
		"chat_id":  chatID,
		"chat_ids": chatIDs,
	}, "Telegram 通知测试已发送。", true, nil, nil))
}

func (s *Service) recordSchedulerUploadNotification(snapshot map[string]any, source string, taskID string, includeCloudflare bool, includeGitHub bool, topEntries ...[]appcore.UploadNotificationTopEntry) (mobileSchedulerStatus, []string) {
	status := s.currentSchedulerStatus()
	if !includeCloudflare && !includeGitHub {
		return status, nil
	}
	notificationTopEntries := []appcore.UploadNotificationTopEntry(nil)
	if len(topEntries) > 0 {
		notificationTopEntries = topEntries[0]
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
		TopEntries: notificationTopEntries,
	})
	warnings := s.recordUploadNotification(snapshot, notification)
	status.UploadNotification = &notification
	if len(warnings) > 0 {
		status.LastMessage = mobileUploadNotificationMessage(status.LastMessage, warnings)
	}
	_ = s.writeSchedulerStatus(status)
	return status, warnings
}

func (s *Service) manualUploadNotificationTopEntries(payload map[string]any) []appcore.UploadNotificationTopEntry {
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	rawRows := firstNonNil(payload["results"], payload["rows"])
	if rawRows == nil {
		return nil
	}
	rows := mobileProbeRowsFromAny(rawRows)
	if len(rows) == 0 {
		return nil
	}
	probeCfg, _ := configToProbeConfig(config)
	selection, err := appcore.BuildUploadSelectionWithColoPaths(config, rows, probeCfg.DownloadSpeedMetric, s.coloDictionaryPaths())
	if err != nil {
		return nil
	}
	return appcore.BuildUploadNotificationTopEntriesForSnapshot(config, selection.FilteredRows, probeCfg.DownloadSpeedMetric)
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
