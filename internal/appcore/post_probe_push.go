package appcore

import (
	"context"
	"fmt"
	"strings"
)

type PostProbePushConfig struct {
	CloudflareEnabled bool `json:"cloudflare_enabled"`
	GitHubEnabled     bool `json:"github_enabled"`
}

func PostProbePushConfigFromSnapshot(snapshot map[string]any) PostProbePushConfig {
	raw := mapValue(snapshot["post_probe_push"])
	return PostProbePushConfig{
		CloudflareEnabled: boolValue(firstNonNil(raw["cloudflare_enabled"], raw["cloudflareEnabled"]), false),
		GitHubEnabled:     boolValue(firstNonNil(raw["github_enabled"], raw["githubEnabled"]), false),
	}
}

func CloudflareProviderEnabledFromSnapshot(snapshot map[string]any) bool {
	cloudflare := mapValue(snapshot["cloudflare"])
	if boolValue(firstNonNil(cloudflare["enabled"], cloudflare["cloudflare_enabled"], cloudflare["cloudflareEnabled"]), false) {
		return true
	}
	return cloudflareRoutingProviderReady(snapshot)
}

func GitHubProviderEnabledFromSnapshot(snapshot map[string]any) bool {
	github := mapValue(snapshot["github"])
	if len(github) == 0 {
		export := mapValue(snapshot["export"])
		github = mapValue(export["github"])
	}
	return boolValue(firstNonNil(github["enabled"], github["github_enabled"], github["githubEnabled"]), false)
}

func cloudflareRoutingProviderReady(snapshot map[string]any) bool {
	cloudflare := mapValue(snapshot["cloudflare"])
	if len(cloudflare) == 0 {
		cloudflare = snapshot
	}
	apiToken := strings.TrimSpace(stringValue(firstNonNil(cloudflare["api_token"], cloudflare["apiToken"]), ""))
	zoneID := strings.TrimSpace(stringValue(firstNonNil(cloudflare["zone_id"], cloudflare["zoneId"]), ""))
	if apiToken == "" || IsMaskedSecret(apiToken) || zoneID == "" {
		return false
	}

	routing := CloudflareRoutingConfigFromSnapshot(snapshot)
	if !routing.Enabled {
		return false
	}
	for _, rule := range routing.Rules {
		if rule.Enabled && strings.TrimSpace(rule.RecordName) != "" {
			return true
		}
	}
	return false
}

func (s *Service) processPostProbePush(ctx context.Context, payload ProbePayload, result ProbeRunResult, checkpoints ...func()) ProbePostProcessResult {
	if payload.DisablePostProbePush || !runPostProbeCheckpoint(ctx, checkpoints) {
		return ProbePostProcessResult{}
	}
	cfg := PostProbePushConfigFromSnapshot(payload.Config)
	if len(result.Results) == 0 {
		return s.postProbePushNoRows(ctx, payload.Config, cfg, payload.TaskID, checkpoints...)
	}
	cloudflareReady := cfg.CloudflareEnabled && CloudflareProviderEnabledFromSnapshot(payload.Config)
	githubReady := cfg.GitHubEnabled && GitHubProviderEnabledFromSnapshot(payload.Config)
	if !cfg.CloudflareEnabled && !cfg.GitHubEnabled {
		return ProbePostProcessResult{}
	}
	var cloudflareReport, githubReport *UploadProviderReport
	if cfg.CloudflareEnabled && !cloudflareReady {
		cloudflareReport = &UploadProviderReport{Status: UploadNotificationStatusSkipped}
	}
	if cfg.GitHubEnabled && !githubReady {
		githubReport = &UploadProviderReport{Status: UploadNotificationStatusSkipped}
	}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.progress", TaskID: payload.TaskID, Payload: map[string]any{
		"failed": result.Summary.Failed, "passed": result.Summary.Passed, "processed": result.Summary.Total,
		"stage": "post_probe_push", "total": result.Summary.Total,
	}})
	selection, err := BuildUploadSelectionWithColoPaths(payload.Config, result.Results, result.Config.DownloadSpeedMetric, s.ColoPaths())
	if err != nil {
		return ProbePostProcessResult{Warnings: []string{fmt.Sprintf("post-probe upload selection failed: %v", err)}}
	}
	warnings := append([]string(nil), selection.Warnings...)
	if cloudflareReady {
		if !runPostProbeCheckpoint(ctx, checkpoints) {
			return ProbePostProcessResult{}
		}
		command := s.pushCloudflareDNSRecordsContext(ctx, map[string]any{"config": payload.Config, "results": result.Results})
		warnings = append(warnings, command.Warnings...)
		report := UploadProviderReportFromCommandResult(UploadNotificationProviderCloudflare, command)
		cloudflareReport = &report
		if !command.OK {
			warnings = append(warnings, "post-probe Cloudflare push failed: "+command.Message)
		}
	}
	if githubReady {
		if !runPostProbeCheckpoint(ctx, checkpoints) {
			return ProbePostProcessResult{}
		}
		if len(selection.GitHubRows) == 0 {
			warnings = append(warnings, "post-probe GitHub export skipped: no results after filtering")
			githubReport = &UploadProviderReport{Status: UploadNotificationStatusSkipped}
		} else {
			command := s.exportResultsToGitHubContext(ctx, map[string]any{"config": payload.Config, "results": selection.GitHubRows, "task_id": payload.TaskID})
			warnings = append(warnings, command.Warnings...)
			report := UploadProviderReportFromCommandResult(UploadNotificationProviderGitHub, command)
			githubReport = &report
			if !command.OK {
				warnings = append(warnings, "post-probe GitHub export failed: "+command.Message)
			}
		}
	}
	if cloudflareReport == nil && githubReport == nil {
		return ProbePostProcessResult{Warnings: dedupeStrings(warnings)}
	}
	if !runPostProbeCheckpoint(ctx, checkpoints) {
		return ProbePostProcessResult{}
	}
	notification := BuildUploadNotification(UploadNotificationInput{
		Cloudflare: cloudflareReport, CreatedAt: s.now(), GitHub: githubReport,
		Source: UploadNotificationSourcePostProbePush, TaskID: payload.TaskID,
		TopEntries: BuildUploadNotificationTopEntriesForSnapshot(payload.Config, selection.FilteredRows, result.Config.DownloadSpeedMetric),
	})
	warnings = append(warnings, s.sendUploadNotification(ctx, payload.Config, notification)...)
	if !runPostProbeCheckpoint(ctx, checkpoints) {
		return ProbePostProcessResult{}
	}
	return ProbePostProcessResult{Notification: &notification, Warnings: dedupeStrings(warnings)}
}

// ProcessPostProbePush runs the shared post-probe provider and notification workflow.
// Platform adapters use it only when they need to bridge a platform-specific result.
func (s *Service) ProcessPostProbePush(ctx context.Context, payload ProbePayload, result ProbeRunResult, checkpoints ...func()) ProbePostProcessResult {
	return s.processPostProbePush(ctx, payload, result, checkpoints...)
}

func (s *Service) postProbePushNoRows(ctx context.Context, snapshot map[string]any, cfg PostProbePushConfig, taskID string, checkpoints ...func()) ProbePostProcessResult {
	if !runPostProbeCheckpoint(ctx, checkpoints) {
		return ProbePostProcessResult{}
	}
	notification := BuildPostProbeNoRowsUploadNotification(cfg, taskID)
	if notification == nil {
		return ProbePostProcessResult{}
	}
	warnings := s.sendUploadNotification(ctx, snapshot, *notification)
	if !runPostProbeCheckpoint(ctx, checkpoints) {
		return ProbePostProcessResult{}
	}
	return ProbePostProcessResult{Notification: notification, Warnings: dedupeStrings(warnings)}
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
