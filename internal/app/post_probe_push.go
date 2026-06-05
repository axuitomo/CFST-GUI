package app

import (
	"fmt"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func (a *App) runDesktopPostProbePush(payload DesktopProbePayload, result ProbeRunResult) []string {
	if payload.DisablePostProbePush || strings.TrimSpace(payload.PipelineID) != "" || len(result.Results) == 0 {
		return nil
	}
	return a.runPostProbePushForSnapshot(payload.Config, result, taskIDOrFallback(payload.TaskID))
}

func (a *App) runPostProbePushForSnapshot(snapshot map[string]any, result ProbeRunResult, taskID string) []string {
	cfg := appcore.PostProbePushConfigFromSnapshot(snapshot)
	warnings := make([]string, 0)
	if cfg.CloudflareEnabled && appcore.CloudflareProviderEnabledFromSnapshot(snapshot) {
		dnsResult := a.PushCloudflareDNSRecords(map[string]any{
			"config":  snapshot,
			"results": result.Results,
		})
		warnings = append(warnings, dnsResult.Warnings...)
		if !dnsResult.OK {
			warnings = append(warnings, fmt.Sprintf("测速后 Cloudflare 自动推送失败：%s", dnsResult.Message))
		}
	}
	if cfg.GitHubEnabled && appcore.GitHubProviderEnabledFromSnapshot(snapshot) {
		githubResult := a.ExportResultsToGitHub(map[string]any{
			"config":  snapshot,
			"results": result.Results,
			"task_id": taskID,
		})
		warnings = append(warnings, githubResult.Warnings...)
		if !githubResult.OK {
			warnings = append(warnings, fmt.Sprintf("测速后 GitHub 自动推送失败：%s", githubResult.Message))
		}
	}
	return dedupeStrings(warnings)
}

func taskIDOrFallback(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "post-probe"
	}
	return taskID
}
