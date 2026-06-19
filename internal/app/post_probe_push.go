package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

type postProbePushResult struct {
	Notification *appcore.UploadNotification
	Warnings     []string
}

func (a *App) runDesktopPostProbePush(payload DesktopProbePayload, result ProbeRunResult) postProbePushResult {
	if payload.DisablePostProbePush || strings.TrimSpace(payload.PipelineID) != "" {
		return postProbePushResult{}
	}
	return a.runPostProbePushForSnapshot(payload.Config, result, taskIDOrFallback(payload.TaskID))
}

func (a *App) runPostProbePushForSnapshot(snapshot map[string]any, result ProbeRunResult, taskID string) postProbePushResult {
	cfg := appcore.PostProbePushConfigFromSnapshot(snapshot)
	warnings := make([]string, 0)
	var cloudflareReport *appcore.UploadProviderReport
	var githubReport *appcore.UploadProviderReport
	cloudflareReady := cfg.CloudflareEnabled && appcore.CloudflareProviderEnabledFromSnapshot(snapshot)
	githubReady := cfg.GitHubEnabled && appcore.GitHubProviderEnabledFromSnapshot(snapshot)
	if cfg.CloudflareEnabled && !cloudflareReady {
		cloudflareReport = &appcore.UploadProviderReport{Status: appcore.UploadNotificationStatusSkipped}
	}
	if cfg.GitHubEnabled && !githubReady {
		githubReport = &appcore.UploadProviderReport{Status: appcore.UploadNotificationStatusSkipped}
	}
	if len(result.Results) == 0 {
		if cfg.CloudflareEnabled {
			cloudflareReport = &appcore.UploadProviderReport{Status: appcore.UploadNotificationStatusSkipped}
		}
		if cfg.GitHubEnabled {
			githubReport = &appcore.UploadProviderReport{Status: appcore.UploadNotificationStatusSkipped}
		}
		if cloudflareReport == nil && githubReport == nil {
			return postProbePushResult{}
		}
		notification := appcore.BuildUploadNotification(appcore.UploadNotificationInput{
			Cloudflare: cloudflareReport,
			CreatedAt:  time.Now(),
			GitHub:     githubReport,
			Message:    "没有可上传结果，测速后自动上传已跳过。",
			Source:     appcore.UploadNotificationSourcePostProbePush,
			TaskID:     taskID,
		})
		warnings = append(warnings, a.sendUploadNotification(context.Background(), snapshot, notification)...)
		return postProbePushResult{Notification: &notification, Warnings: dedupeStrings(warnings)}
	}
	if cloudflareReady {
		dnsResult := a.PushCloudflareDNSRecords(map[string]any{
			"config":  snapshot,
			"results": result.Results,
		})
		warnings = append(warnings, dnsResult.Warnings...)
		report := appcore.UploadProviderReportFromCommandResult(appcore.UploadNotificationProviderCloudflare, dnsResult)
		cloudflareReport = &report
		if !dnsResult.OK {
			warnings = append(warnings, fmt.Sprintf("测速后 Cloudflare 自动推送失败：%s", dnsResult.Message))
		}
	}
	if githubReady {
		githubResult := a.ExportResultsToGitHub(map[string]any{
			"config":  snapshot,
			"results": result.Results,
			"task_id": taskID,
		})
		warnings = append(warnings, githubResult.Warnings...)
		report := appcore.UploadProviderReportFromCommandResult(appcore.UploadNotificationProviderGitHub, githubResult)
		githubReport = &report
		if !githubResult.OK {
			warnings = append(warnings, fmt.Sprintf("测速后 GitHub 自动推送失败：%s", githubResult.Message))
		}
	}
	if cloudflareReport == nil && githubReport == nil {
		return postProbePushResult{Warnings: dedupeStrings(warnings)}
	}
	notification := appcore.BuildUploadNotification(appcore.UploadNotificationInput{
		Cloudflare: cloudflareReport,
		CreatedAt:  time.Now(),
		GitHub:     githubReport,
		Source:     appcore.UploadNotificationSourcePostProbePush,
		TaskID:     taskID,
	})
	warnings = append(warnings, a.sendUploadNotification(context.Background(), snapshot, notification)...)
	return postProbePushResult{
		Notification: &notification,
		Warnings:     dedupeStrings(warnings),
	}
}

func taskIDOrFallback(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "post-probe"
	}
	return taskID
}
