package appcore

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/githubcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func (s *Service) invokeGitHubTest(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("GITHUB_EXPORT_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	cfg, warnings, err := GitHubExportConfigFromPayload(payload, s.githubDefaults())
	if err != nil {
		return NewCommandResult("GITHUB_EXPORT_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.newGitHubClient(cfg.Token).CheckExportAccess(ctx, cfg); err != nil {
		return NewCommandResult("GITHUB_EXPORT_TEST_FAILED", nil, err.Error(), false, nil, warnings)
	}
	return NewCommandResult("GITHUB_EXPORT_TEST_OK", map[string]any{
		"branch": cfg.Branch, "owner": cfg.Owner, "repo": cfg.Repo,
	}, "GitHub repository, branch, and Contents access verified.", true, nil, warnings)
}

func (s *Service) invokeResultsCSVExport(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("RESULTS_CSV_EXPORT_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	body, rowCount, err := s.githubExportBodyFromPayload(payload, githubcore.Config{Format: "csv", CSVEncoding: GitHubCSVEncodingFromPayload(payload)})
	if err != nil {
		return NewCommandResult("RESULTS_CSV_EXPORT_INPUT_INVALID", nil, err.Error(), false, stringPtr(taskID), nil)
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	exportCfg := mapValue(config["export"])
	targetURI := strings.TrimSpace(stringValue(firstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"], exportCfg["target_uri"], exportCfg["targetUri"]), ""))
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	targetValue := targetPath
	if targetValue == "" {
		targetValue = targetURI
	}
	fileName := GitHubExportCSVTargetFileName(payload, targetValue, "result.csv")
	if targetURI == "" && targetPath == "" {
		targetDir := strings.TrimSpace(stringValue(firstNonNil(payload["target_dir"], payload["targetDir"], exportCfg["target_dir"], exportCfg["targetDir"]), ""))
		if targetDir == "" {
			targetDir = s.StorageLayout().ExportsRoot()
		}
		targetPath = filepath.Join(targetDir, filepath.Base(fileName))
	}
	message := fmt.Sprintf("Exported %d probe results to CSV.", rowCount)
	if targetURI != "" {
		return NewCommandResult("RESULTS_CSV_EXPORT_OK", map[string]any{
			"content_base64": base64.StdEncoding.EncodeToString(body), "file_name": fileName,
			"target_uri": targetURI, "written_count": rowCount,
		}, message, true, stringPtr(taskID), nil)
	}
	if targetPath == "" {
		return NewCommandResult("RESULTS_CSV_EXPORT_INVALID", nil, "missing export target path", false, stringPtr(taskID), nil)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return NewCommandResult("RESULTS_CSV_EXPORT_WRITE_FAILED", nil, err.Error(), false, stringPtr(taskID), nil)
	}
	if err := WriteFileAtomic(targetPath, body, 0o644); err != nil {
		return NewCommandResult("RESULTS_CSV_EXPORT_WRITE_FAILED", nil, err.Error(), false, stringPtr(taskID), nil)
	}
	return NewCommandResult("RESULTS_CSV_EXPORT_OK", map[string]any{
		"file_name": GitHubExportCSVTargetFileName(payload, targetPath, fileName), "path": targetPath, "written_count": rowCount,
	}, message, true, stringPtr(taskID), nil)
}

func (s *Service) invokeGitHubExport(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("GITHUB_EXPORT_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	result := s.exportResultsToGitHubContext(context.Background(), payload)
	return s.attachManualUploadNotification(payload, UploadNotificationProviderGitHub, result)
}

func (s *Service) exportResultsToGitHubContext(ctx context.Context, payload map[string]any) CommandResult {
	cfg, warnings, err := GitHubExportConfigFromPayload(payload, s.githubDefaults())
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	if err != nil {
		return NewCommandResult("GITHUB_EXPORT_CONFIG_INVALID", nil, err.Error(), false, stringPtr(taskID), warnings)
	}
	if rawRows := firstNonNil(payload["results"], payload["rows"]); rawRows != nil {
		rows := ProbeRowsFromAny(rawRows)
		if len(rows) == 0 {
			return NewCommandResult("GITHUB_EXPORT_INPUT_INVALID", nil, "no valid probe result rows to export", false, stringPtr(taskID), warnings)
		}
		config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
		probeCfg, _ := s.probeConfigFromSnapshot(config)
		selection, selectErr := BuildUploadSelectionWithColoPaths(config, rows, probeCfg.DownloadSpeedMetric, s.ColoPaths())
		if selectErr != nil {
			return NewCommandResult("GITHUB_EXPORT_INPUT_INVALID", nil, selectErr.Error(), false, stringPtr(taskID), warnings)
		}
		warnings = append(warnings, selection.Warnings...)
		if len(selection.GitHubRows) == 0 {
			return NewCommandResult("GITHUB_EXPORT_INPUT_INVALID", nil, "共享上传筛选后没有可导出的 GitHub 结果。", false, stringPtr(taskID), warnings)
		}
		payload = cloneAnyMap(payload)
		payload["results"] = selection.GitHubRows
	}
	body, rowCount, err := s.githubExportBodyFromPayload(payload, cfg)
	if err != nil {
		return NewCommandResult("GITHUB_EXPORT_INPUT_INVALID", nil, err.Error(), false, stringPtr(taskID), warnings)
	}
	if taskID == "" {
		taskID = fmt.Sprintf("manual-%s", s.now().Format("20060102-150405"))
	}
	result, err := ExportCSVToGitHub(ctx, cfg, taskID, body, rowCount, s.now(), s.newGitHubClient(cfg.Token))
	if err != nil {
		return NewCommandResult("GITHUB_EXPORT_FAILED", nil, err.Error(), false, stringPtr(taskID), warnings)
	}
	return NewCommandResult("GITHUB_EXPORT_OK", result, fmt.Sprintf("Exported %d probe results to GitHub.", rowCount), true, stringPtr(taskID), warnings)
}

func (s *Service) ExportProbeRowsToGitHub(ctx context.Context, snapshot map[string]any, taskID string, rows []probecore.ProbeRow) (githubcore.ExportResult, error) {
	cfg, _, err := GitHubExportConfigFromSnapshot(snapshot, s.githubDefaults())
	if err != nil {
		return githubcore.ExportResult{}, err
	}
	body, rowCount, err := EncodeProbeRowsForGitHub(rows, cfg)
	if err != nil {
		return githubcore.ExportResult{}, err
	}
	return ExportCSVToGitHub(ctx, cfg, taskID, body, rowCount, s.now(), s.newGitHubClient(cfg.Token))
}

func (s *Service) githubExportBodyFromPayload(payload map[string]any, cfg githubcore.Config) ([]byte, int, error) {
	if rawRows := firstNonNil(payload["results"], payload["rows"]); rawRows != nil {
		rows := ProbeRowsFromAny(rawRows)
		if len(rows) == 0 {
			return nil, 0, errors.New("no valid probe result rows to export")
		}
		return EncodeProbeRowsForGitHub(rows, cfg)
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	probeCfg, _ := s.probeConfigFromSnapshot(config)
	sourcePath := s.resolveResultFilePath(payload, probeCfg)
	rows, err := ReadProbeRowsForGitHubFromCSV(sourcePath)
	if err != nil {
		return nil, 0, err
	}
	return EncodeProbeRowsForGitHub(rows, cfg)
}

func (s *Service) resolveResultFilePath(payload map[string]any, cfg probecore.ProbeConfig) string {
	for _, key := range []string{"path", "source_path", "sourcePath", "target_path", "targetPath", "export_path", "exportPath"} {
		if path := strings.TrimSpace(stringValue(payload[key], "")); path != "" {
			return path
		}
	}
	if cfg.WriteOutput && strings.TrimSpace(cfg.OutputFile) != "" {
		return strings.TrimSpace(cfg.OutputFile)
	}
	path := filepath.Join(s.StorageLayout().Root, "result.csv")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return "result.csv"
}

func (s *Service) attachManualUploadNotification(payload map[string]any, provider string, result CommandResult) CommandResult {
	if UploadNotificationTriggerFromPayload(payload) != UploadNotificationSourceManualPush {
		return result
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	notification := BuildUploadNotificationFromCommandResult(CommandResultUploadNotificationInput{
		CreatedAt: s.now(), Provider: provider, Result: result, Source: UploadNotificationSourceManualPush,
		TaskID: CommandResultTaskID(payload, result), TopEntries: s.manualUploadNotificationTopEntries(payload),
	})
	warnings := s.sendUploadNotification(context.Background(), config, notification)
	result.Data = CommandResultDataWithUploadNotification(result.Data, notification)
	result.Warnings = dedupeStrings(append(result.Warnings, warnings...))
	return result
}

func (s *Service) manualUploadNotificationTopEntries(payload map[string]any) []UploadNotificationTopEntry {
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	rows := ProbeRowsFromAny(firstNonNil(payload["results"], payload["rows"]))
	if len(rows) == 0 {
		return nil
	}
	probeCfg, _ := s.probeConfigFromSnapshot(config)
	selection, err := BuildUploadSelectionWithColoPaths(config, rows, probeCfg.DownloadSpeedMetric, s.ColoPaths())
	if err != nil {
		return nil
	}
	return BuildUploadNotificationTopEntriesForSnapshot(config, selection.FilteredRows, probeCfg.DownloadSpeedMetric)
}

func (s *Service) probeConfigFromSnapshot(snapshot map[string]any) (probecore.ProbeConfig, []string) {
	s.mu.RLock()
	options := s.options.ProbeConfigOptions
	s.mu.RUnlock()
	return probecore.ConfigSnapshotToProbeConfig(snapshot, options)
}

func (s *Service) githubDefaults() githubcore.ConfigDefaults {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.options.GitHubDefaults
}

func (s *Service) newGitHubClient(token string) *githubcore.Client {
	s.mu.RLock()
	baseURL, client := s.options.GitHubAPIBaseURL, s.options.HTTPClient
	s.mu.RUnlock()
	return githubcore.NewClientWithOptions(githubcore.ClientOptions{BaseURL: baseURL, HTTPClient: client, Token: token})
}
