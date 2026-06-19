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
	cloudflareReady := cfg.CloudflareEnabled && appcore.CloudflareProviderEnabledFromSnapshot(payload.Config)
	githubReady := cfg.GitHubEnabled && appcore.GitHubProviderEnabledFromSnapshot(payload.Config)
	if !cloudflareReady && !githubReady && len(result.Results) > 0 {
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
	if len(result.Results) == 0 {
		if cfg.CloudflareEnabled {
			cloudflareReport = &appcore.UploadProviderReport{Status: appcore.UploadNotificationStatusSkipped}
		}
		if cfg.GitHubEnabled {
			githubReport = &appcore.UploadProviderReport{Status: appcore.UploadNotificationStatusSkipped}
		}
		if cloudflareReport == nil && githubReport == nil {
			return mobilePostProbePushResult{}
		}
		notification := appcore.BuildUploadNotification(appcore.UploadNotificationInput{
			Cloudflare: cloudflareReport,
			CreatedAt:  time.Now(),
			GitHub:     githubReport,
			Message:    "没有可上传结果，测速后自动上传已跳过。",
			Source:     appcore.UploadNotificationSourcePostProbePush,
			TaskID:     payload.TaskID,
		})
		return mobilePostProbePushResult{
			Notification: &notification,
			Warnings:     s.recordUploadNotification(payload.Config, notification),
		}
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
		if len(selection.GitHubRows) == 0 {
			warnings = append(warnings, "测速后 GitHub 自动推送跳过：筛选后没有可导出结果。")
			githubReport = &appcore.UploadProviderReport{Status: appcore.UploadNotificationStatusSkipped}
		} else {
			githubCommand := decodeCommandResult(s.ExportResultsToGitHub(encodeJSON(map[string]any{
				"config":  payload.Config,
				"results": selection.GitHubRows,
				"task_id": payload.TaskID,
			})))
			warnings = append(warnings, githubCommand.Warnings...)
			report := appcore.UploadProviderReportFromCommandResult(appcore.UploadNotificationProviderGitHub, githubCommand)
			githubReport = &report
			if !githubCommand.OK {
				warnings = append(warnings, fmt.Sprintf("测速后 GitHub 自动推送失败：%s", githubCommand.Message))
			}
		}
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
	})
	warnings = append(warnings, s.recordUploadNotification(payload.Config, notification)...)
	return mobilePostProbePushResult{
		Notification: &notification,
		Warnings:     dedupeStrings(warnings),
	}
}
