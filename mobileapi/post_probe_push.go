package mobileapi

import (
	"fmt"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func (s *Service) runMobilePostProbePush(payload desktopProbePayload, result probeRunResult) []string {
	if payload.DisablePostProbePush || strings.TrimSpace(payload.PipelineID) != "" || len(result.Results) == 0 {
		return nil
	}
	cfg := appcore.PostProbePushConfigFromSnapshot(payload.Config)
	if !cfg.CloudflareEnabled && !cfg.GitHubEnabled {
		return nil
	}
	selection, err := appcore.BuildUploadSelectionWithColoPaths(payload.Config, result.Results, result.Config.DownloadSpeedMetric, s.coloDictionaryPaths())
	warnings := make([]string, 0)
	if err != nil {
		return []string{fmt.Sprintf("测速后自动推送筛选失败：%v", err)}
	}
	warnings = append(warnings, selection.Warnings...)
	if cfg.CloudflareEnabled && appcore.CloudflareProviderEnabledFromSnapshot(payload.Config) {
		dnsCommand := decodeCommandResult(s.PushCloudflareDNSRecords(encodeJSON(map[string]any{
			"config":  payload.Config,
			"results": result.Results,
		})))
		warnings = append(warnings, dnsCommand.Warnings...)
		if !dnsCommand.OK {
			warnings = append(warnings, fmt.Sprintf("测速后 Cloudflare 自动推送失败：%s", dnsCommand.Message))
		}
	}
	if cfg.GitHubEnabled && appcore.GitHubProviderEnabledFromSnapshot(payload.Config) {
		if len(selection.GitHubRows) == 0 {
			warnings = append(warnings, "测速后 GitHub 自动推送跳过：筛选后没有可导出结果。")
		} else {
			githubCommand := decodeCommandResult(s.ExportResultsToGitHub(encodeJSON(map[string]any{
				"config":  payload.Config,
				"results": selection.GitHubRows,
				"task_id": payload.TaskID,
			})))
			warnings = append(warnings, githubCommand.Warnings...)
			if !githubCommand.OK {
				warnings = append(warnings, fmt.Sprintf("测速后 GitHub 自动推送失败：%s", githubCommand.Message))
			}
		}
	}
	return dedupeStrings(warnings)
}
