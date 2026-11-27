package appcore

import (
	"context"
	"fmt"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/notify"
)

func (s *Service) processPostProbePush(ctx context.Context, payload contracts.ProbePayload, result contracts.ProbeRunResult, checkpoints ...func()) ProbePostProcessResult {
	if payload.DisablePostProbePush || !runPostProbeCheckpoint(ctx, checkpoints) {
		return ProbePostProcessResult{}
	}
	cfg := notify.PostProbePushConfigFromSnapshot(payload.Config)
	if len(result.Results) == 0 {
		return s.postProbePushNoRows(ctx, payload.Config, cfg, result, payload.TaskID, checkpoints...)
	}
	dnsReady := cfg.DNSEnabled && DNSProviderEnabledFromSnapshot(payload.Config)
	githubReady := cfg.GitHubEnabled && notify.GitHubProviderEnabledFromSnapshot(payload.Config)
	if !cfg.DNSEnabled && !cfg.GitHubEnabled {
		return ProbePostProcessResult{}
	}
	var dnsReports []contracts.UploadProviderResult
	var githubReport *contracts.UploadProviderReport
	if cfg.DNSEnabled && !dnsReady {
		dnsReports = skippedDNSProviderReports(payload.Config)
	}
	if cfg.GitHubEnabled && !githubReady {
		githubReport = &contracts.UploadProviderReport{Status: notify.UploadNotificationStatusSkipped}
	}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.progress", TaskID: payload.TaskID, Payload: map[string]any{
		"failed": result.Summary.Failed, "passed": result.Summary.Passed, "processed": result.Summary.Total,
		"stage": "post_probe_push", "total": result.Summary.Total,
	}})
	selection, err := notify.BuildUploadSelectionWithColoPaths(payload.Config, result.Results, result.Config.DownloadSpeedMetric, s.ColoPaths())
	if err != nil {
		return ProbePostProcessResult{Warnings: []string{fmt.Sprintf("post-probe upload selection failed: %v", err)}}
	}
	warnings := append([]string(nil), selection.Warnings...)
	if dnsReady {
		if !runPostProbeCheckpoint(ctx, checkpoints) {
			return ProbePostProcessResult{}
		}
		command := s.PushDNSProviders(ctx, map[string]any{"config": payload.Config, "results": result.Results})
		warnings = append(warnings, command.Warnings...)
		dnsReports = DNSProviderReportsFromCommandData(command.Data)
		if !command.OK {
			warnings = append(warnings, "post-probe DNS push failed: "+command.Message)
		}
	}
	if githubReady {
		if !runPostProbeCheckpoint(ctx, checkpoints) {
			return ProbePostProcessResult{}
		}
		if len(selection.GitHubRows) == 0 {
			warnings = append(warnings, "post-probe GitHub export skipped: no results after filtering")
			githubReport = &contracts.UploadProviderReport{Status: notify.UploadNotificationStatusSkipped}
		} else {
			command := s.exportResultsToGitHubContext(ctx, map[string]any{"config": payload.Config, "results": selection.GitHubRows, "task_id": payload.TaskID})
			warnings = append(warnings, command.Warnings...)
			report := notify.UploadProviderReportFromCommandResult(notify.UploadNotificationProviderGitHub, command)
			githubReport = &report
			if !command.OK {
				warnings = append(warnings, "post-probe GitHub export failed: "+command.Message)
			}
		}
	}
	if len(dnsReports) == 0 && githubReport == nil {
		return ProbePostProcessResult{Warnings: notify.DedupeStrings(warnings)}
	}
	if !runPostProbeCheckpoint(ctx, checkpoints) {
		return ProbePostProcessResult{}
	}
	notification := notify.BuildUploadNotification(notify.UploadNotificationInput{
		CreatedAt: s.now(), DurationMS: result.DurationMS, GitHub: githubReport,
		Funnel:    notify.UploadNotificationFunnelForProbe(result.Source, result.Summary, len(selection.FilteredRows)),
		Providers: dnsReports,
		Source:    notify.UploadNotificationSourcePostProbePush, TaskID: payload.TaskID,
		TopEntries: notify.BuildUploadNotificationTopEntriesForSnapshot(payload.Config, selection.FilteredRows, result.Config.DownloadSpeedMetric),
	})
	warnings = append(warnings, s.sendUploadNotification(ctx, payload.Config, notification)...)
	if !runPostProbeCheckpoint(ctx, checkpoints) {
		return ProbePostProcessResult{}
	}
	return ProbePostProcessResult{Notification: &notification, Warnings: notify.DedupeStrings(warnings)}
}

// ProcessPostProbePush runs the shared post-probe provider and notification workflow.
// Platform adapters use it only when they need to bridge a platform-specific result.
func (s *Service) ProcessPostProbePush(ctx context.Context, payload contracts.ProbePayload, result contracts.ProbeRunResult, checkpoints ...func()) ProbePostProcessResult {
	return s.processPostProbePush(ctx, payload, result, checkpoints...)
}

func (s *Service) postProbePushNoRows(ctx context.Context, snapshot map[string]any, cfg notify.PostProbePushConfig, result contracts.ProbeRunResult, taskID string, checkpoints ...func()) ProbePostProcessResult {
	if !runPostProbeCheckpoint(ctx, checkpoints) {
		return ProbePostProcessResult{}
	}
	notification := notify.BuildPostProbeNoRowsUploadNotification(cfg, skippedDNSProviderReports(snapshot), notify.UploadNotificationFunnelForProbe(result.Source, result.Summary, 0), result.DurationMS, taskID)
	if notification == nil {
		return ProbePostProcessResult{}
	}
	warnings := s.sendUploadNotification(ctx, snapshot, *notification)
	if !runPostProbeCheckpoint(ctx, checkpoints) {
		return ProbePostProcessResult{}
	}
	return ProbePostProcessResult{Notification: notification, Warnings: notify.DedupeStrings(warnings)}
}

func skippedDNSProviderReports(snapshot map[string]any) []contracts.UploadProviderResult {
	return dnsProviderReportsWithStatus(snapshot, notify.UploadNotificationStatusSkipped)
}

func abortedDNSProviderReports(snapshot map[string]any) []contracts.UploadProviderResult {
	return dnsProviderReportsWithStatus(snapshot, notify.UploadNotificationStatusAborted)
}

func dnsProviderReportsWithStatus(snapshot map[string]any, status string) []contracts.UploadProviderResult {
	providers := DNSProvidersFromSnapshot(snapshot)
	reports := make([]contracts.UploadProviderResult, 0, len(providers))
	for _, provider := range providers {
		reports = append(reports, contracts.UploadProviderResult{
			ID:       provider.ID,
			Label:    dnsProviderDisplayName(provider),
			Provider: provider.Provider,
			Status:   status,
		})
	}
	return reports
}

// buildPostProbeAbortedNotification 在测速后自动上传被终止时补一条熔断/终止通知：
// 让用户分清「我按了终止」和「没有可推送结果」。
func (s *Service) buildPostProbeAbortedNotification(snapshot map[string]any, result contracts.ProbeRunResult) *contracts.UploadNotification {
	cfg := notify.PostProbePushConfigFromSnapshot(snapshot)
	if !cfg.DNSEnabled && !cfg.GitHubEnabled {
		return nil
	}
	var providers []contracts.UploadProviderResult
	if cfg.DNSEnabled {
		providers = abortedDNSProviderReports(snapshot)
	}
	var githubReport *contracts.UploadProviderReport
	if cfg.GitHubEnabled {
		githubReport = &contracts.UploadProviderReport{Status: notify.UploadNotificationStatusAborted}
	}
	notification := notify.BuildUploadNotification(notify.UploadNotificationInput{
		CreatedAt:  s.now(),
		DurationMS: result.DurationMS,
		Funnel:     notify.UploadNotificationFunnelForProbe(result.Source, result.Summary, 0),
		GitHub:     githubReport,
		Message:    "上传链路被终止，本次推送没有跑完。",
		Providers:  providers,
		Source:     notify.UploadNotificationSourcePostProbePush,
		TaskID:     result.TaskID,
	})
	return &notification
}

func runPostProbeCheckpoint(ctx context.Context, checkpoints []func()) bool {
	if ctx == nil || ctx.Err() != nil {
		return false
	}
	for _, checkpoint := range checkpoints {
		if checkpoint != nil {
			checkpoint()
		}
	}
	return ctx.Err() == nil
}
