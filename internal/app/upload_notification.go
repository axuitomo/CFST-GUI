package app

import (
	"context"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func (a *App) attachManualUploadNotification(payload map[string]any, provider string, result DesktopCommandResult) DesktopCommandResult {
	if appcore.UploadNotificationTriggerFromPayload(payload) != appcore.UploadNotificationSourceManualPush {
		return result
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	notification := appcore.BuildUploadNotificationFromCommandResult(appcore.CommandResultUploadNotificationInput{
		CreatedAt: time.Now(),
		Provider:  provider,
		Result:    result,
		Source:    appcore.UploadNotificationSourceManualPush,
		TaskID:    appcore.CommandResultTaskID(payload, result),
	})
	warnings := a.sendUploadNotification(context.Background(), config, notification)
	result.Data = appcore.CommandResultDataWithUploadNotification(result.Data, notification)
	result.Warnings = dedupeStrings(append(result.Warnings, warnings...))
	return result
}

func (a *App) sendUploadNotification(ctx context.Context, snapshot map[string]any, notification appcore.UploadNotification) []string {
	if strings.TrimSpace(notification.Status) == "" {
		return nil
	}
	if err := appcore.SendTelegramUploadNotification(ctx, snapshot, notification, nil, ""); err != nil {
		return []string{"Telegram 通知发送失败：" + err.Error()}
	}
	return nil
}

func (a *App) recordSchedulerUploadNotification(snapshot map[string]any, source string, taskID string, includeCloudflare bool, includeGitHub bool) {
	if !includeCloudflare && !includeGitHub {
		return
	}
	status := a.currentSchedulerStatus()
	var cloudflareReport *appcore.UploadProviderReport
	var githubReport *appcore.UploadProviderReport
	if includeCloudflare {
		cloudflareReport = &appcore.UploadProviderReport{
			Status:      firstNonEmptyString(status.LastDNSStatus, appcore.UploadNotificationStatusSkipped),
			UploadCount: status.CloudflareUploadCount,
		}
	}
	if includeGitHub {
		githubReport = &appcore.UploadProviderReport{
			Status:      firstNonEmptyString(status.LastGitHubStatus, appcore.UploadNotificationStatusSkipped),
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
	warnings := a.sendUploadNotification(context.Background(), snapshot, notification)
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		status.UploadNotification = &notification
		if len(warnings) > 0 {
			status.LastMessage = schedulerStatusMessage(status.LastMessage, warnings)
		}
	})
}

func (a *App) TestTelegramNotification(payload map[string]any) DesktopCommandResult {
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(config) == 0 {
		config = payload
	}
	cfg := appcore.TelegramNotificationConfigFromSnapshot(config)
	cfg.Enabled = true
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	text := "CFST 上传通知测试\n状态：Telegram 通知渠道可用。"
	if err := appcore.SendTelegramMessage(ctx, cfg, text, nil, ""); err != nil {
		return desktopCommandResult("TELEGRAM_NOTIFICATION_TEST_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("TELEGRAM_NOTIFICATION_TEST_OK", map[string]any{
		"chat_id": cfg.ChatID,
	}, "Telegram 通知测试已发送。", true, nil, nil)
}
