package mobileapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

type mobilePostProbePushResult struct {
	Notification *appcore.UploadNotification
	Warnings     []string
}

func (s *Service) runMobilePostProbePush(payload desktopProbePayload, result probeRunResult) mobilePostProbePushResult {
	if payload.DisablePostProbePush || strings.TrimSpace(payload.PipelineID) != "" {
		return mobilePostProbePushResult{}
	}
	cfg := appcore.PostProbePushConfigFromSnapshot(payload.Config)
	if len(result.Results) == 0 {
		return s.mobilePostProbePushNoRowsResult(payload.Config, cfg, payload.TaskID)
	}
	cloudflareReady := cfg.CloudflareEnabled && appcore.CloudflareProviderEnabledFromSnapshot(payload.Config)
	githubReady := cfg.GitHubEnabled && appcore.GitHubProviderEnabledFromSnapshot(payload.Config)
	if !cloudflareReady && !githubReady {
		return mobilePostProbePushResult{}
	}
	var cloudflareReport *appcore.UploadProviderReport
	var githubReport *appcore.UploadProviderReport
	if cfg.CloudflareEnabled && !cloudflareReady {
		cloudflareReport = &appcore.UploadProviderReport{Status: appcore.UploadNotificationStatusSkipped}
	}
	if cfg.GitHubEnabled && !githubReady {
		githubReport = &appcore.UploadProviderReport{Status: appcore.UploadNotificationStatusSkipped}
	}
	s.emit(payload.TaskID, "probe.progress", map[string]any{
		"failed":    result.Summary.Failed,
		"passed":    result.Summary.Passed,
		"processed": result.Summary.Total,
		"stage":     "post_probe_push",
		"total":     result.Summary.Total,
	})
	selection, err := appcore.BuildUploadSelectionWithColoPaths(payload.Config, result.Results, result.Config.DownloadSpeedMetric, s.coloDictionaryPaths())
	warnings := make([]string, 0)
	if err != nil {
		return mobilePostProbePushResult{Warnings: []string{fmt.Sprintf("测速后自动推送筛选失败：%v", err)}}
	}
	warnings = append(warnings, selection.Warnings...)
	if cloudflareReady {
		dnsCommand := decodeCommandResult(s.PushCloudflareDNSRecords(encodeJSON(map[string]any{
			"config":  payload.Config,
			"results": result.Results,
		})))
		warnings = append(warnings, dnsCommand.Warnings...)
		report := appcore.UploadProviderReportFromCommandResult(appcore.UploadNotificationProviderCloudflare, dnsCommand)
		cloudflareReport = &report
		if !dnsCommand.OK {
			warnings = append(warnings, fmt.Sprintf("测速后 Cloudflare 自动推送失败：%s", dnsCommand.Message))
		}
	}
	if githubReady {
		githubReport, warnings = s.runMobilePostProbeGitHubPush(payload, selection, warnings)
	}
	if cloudflareReport == nil && githubReport == nil {
		return mobilePostProbePushResult{Warnings: dedupeStrings(warnings)}
	}
	notification := appcore.BuildUploadNotification(appcore.UploadNotificationInput{
		Cloudflare: cloudflareReport,
		CreatedAt:  time.Now(),
		GitHub:     githubReport,
		Source:     appcore.UploadNotificationSourcePostProbePush,
		TaskID:     payload.TaskID,
		TopEntries: appcore.BuildUploadNotificationTopEntriesForSnapshot(payload.Config, selection.FilteredRows, result.Config.DownloadSpeedMetric),
	})
	warnings = append(warnings, s.recordUploadNotification(payload.Config, notification)...)
	return mobilePostProbePushResult{
		Notification: &notification,
		Warnings:     dedupeStrings(warnings),
	}
}

func (s *Service) mobilePostProbePushNoRowsResult(snapshot map[string]any, cfg appcore.PostProbePushConfig, taskID string) mobilePostProbePushResult {
	notification := appcore.BuildPostProbeNoRowsUploadNotification(cfg, taskID)
	if notification == nil {
		return mobilePostProbePushResult{}
	}
	return mobilePostProbePushResult{
		Notification: notification,
		Warnings:     s.recordUploadNotification(snapshot, *notification),
	}
}

func (s *Service) runMobilePostProbeGitHubPush(payload desktopProbePayload, selection appcore.UploadSelectionResult, warnings []string) (*appcore.UploadProviderReport, []string) {
	if len(selection.GitHubRows) == 0 {
		warnings = append(warnings, "测速后 GitHub 自动推送跳过：筛选后没有可导出结果。")
		return &appcore.UploadProviderReport{Status: appcore.UploadNotificationStatusSkipped}, warnings
	}
	githubCommand := decodeCommandResult(s.ExportResultsToGitHub(encodeJSON(map[string]any{
		"config":  payload.Config,
		"results": selection.GitHubRows,
		"task_id": payload.TaskID,
	})))
	warnings = append(warnings, githubCommand.Warnings...)
	report := appcore.UploadProviderReportFromCommandResult(appcore.UploadNotificationProviderGitHub, githubCommand)
	if !githubCommand.OK {
		warnings = append(warnings, fmt.Sprintf("测速后 GitHub 自动推送失败：%s", githubCommand.Message))
	}
	return &report, warnings
}
