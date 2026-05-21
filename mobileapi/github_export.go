package mobileapi

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/githubcore"
)

const (
	defaultMobileGitHubExportBranch                = githubcore.DefaultBranch
	defaultMobileGitHubExportCommitMessageTemplate = githubcore.DefaultCommitMessageTemplate
	defaultMobileGitHubExportOwner                 = githubcore.DefaultOwner
	defaultMobileGitHubExportPathTemplate          = githubcore.DefaultPathTemplate
	defaultMobileGitHubExportRepo                  = githubcore.DefaultRepo
)

var mobileGitHubAPIBaseURL = githubcore.APIBaseURL

type mobileGitHubExportConfig = githubcore.Config
type mobileGitHubExportResult = githubcore.ExportResult
type mobileGitHubContentsResponse = githubcore.ContentsResponse
type mobileGitHubPutContentsResponse = githubcore.PutContentsResponse
type mobileGitHubContentsPutRequest = githubcore.ContentsPutRequest
type mobileGitHubExportClient = githubcore.Client

func (s *Service) LoadSchedulerStatus() string {
	return encodeCommand(commandResultFor("SCHEDULER_UNSUPPORTED", map[string]any{
		"enabled":            false,
		"last_dns_status":    "",
		"last_github_status": "",
		"last_message":       "Android 端不支持后台定时任务。",
		"last_probe_status":  "unsupported",
		"last_run_at":        "",
		"last_task_id":       "",
		"next_run_at":        "",
	}, "Android 端不支持后台定时任务。", false, nil, nil))
}

func (s *Service) TestGitHubExport(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("GITHUB_EXPORT_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	cfg, warnings, err := mobileGitHubExportConfigFromPayload(payload)
	if err != nil {
		return encodeCommand(commandResultFor("GITHUB_EXPORT_CONFIG_INVALID", nil, err.Error(), false, nil, warnings))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := newMobileGitHubExportClient(cfg.Token).CheckExportAccess(ctx, cfg); err != nil {
		return encodeCommand(commandResultFor("GITHUB_EXPORT_TEST_FAILED", nil, err.Error(), false, nil, warnings))
	}
	return encodeCommand(commandResultFor("GITHUB_EXPORT_TEST_OK", map[string]any{
		"branch": cfg.Branch,
		"owner":  cfg.Owner,
		"repo":   cfg.Repo,
	}, "GitHub 仓库、分支与 Contents 读取权限已验证。", true, nil, warnings))
}

func (s *Service) ExportResultsCSV(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("RESULTS_CSV_EXPORT_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	body, rowCount, err := s.mobileGitHubExportBodyFromPayload(payload, mobileGitHubExportConfig{
		Format:      "csv",
		CSVEncoding: mobileCSVEncodingFromPayload(payload),
	})
	if err != nil {
		return encodeCommand(commandResultFor("RESULTS_CSV_EXPORT_INPUT_INVALID", nil, err.Error(), false, &taskID, nil))
	}

	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	exportCfg := mapValue(config["export"])
	targetURI := strings.TrimSpace(stringValue(firstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"], exportCfg["target_uri"], exportCfg["targetUri"]), ""))
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	targetValue := targetPath
	if targetValue == "" {
		targetValue = targetURI
	}
	fileName := mobileExportCSVTargetFileName(payload, targetValue, "result.csv")
	if targetURI == "" && targetPath == "" {
		targetDir := strings.TrimSpace(stringValue(firstNonNil(payload["target_dir"], payload["targetDir"], exportCfg["target_dir"], exportCfg["targetDir"]), ""))
		if targetDir != "" {
			targetPath = filepath.Join(targetDir, filepath.Base(fileName))
		}
	}
	message := fmt.Sprintf("已导出 %d 条测速结果 CSV。", rowCount)
	if targetURI != "" {
		return encodeCommand(commandResultFor("RESULTS_CSV_EXPORT_OK", map[string]any{
			"content_base64": base64.StdEncoding.EncodeToString(body),
			"file_name":      fileName,
			"target_uri":     targetURI,
			"written_count":  rowCount,
		}, message, true, &taskID, nil))
	}
	if targetPath == "" {
		return encodeCommand(commandResultFor("RESULTS_CSV_EXPORT_INVALID", nil, "缺少导出目标路径。", false, &taskID, nil))
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return encodeCommand(commandResultFor("RESULTS_CSV_EXPORT_WRITE_FAILED", nil, err.Error(), false, &taskID, nil))
	}
	if err := os.WriteFile(targetPath, body, 0o644); err != nil {
		return encodeCommand(commandResultFor("RESULTS_CSV_EXPORT_WRITE_FAILED", nil, err.Error(), false, &taskID, nil))
	}
	return encodeCommand(commandResultFor("RESULTS_CSV_EXPORT_OK", map[string]any{
		"file_name":     mobileExportCSVTargetFileName(payload, targetPath, fileName),
		"path":          targetPath,
		"written_count": rowCount,
	}, message, true, &taskID, nil))
}

func (s *Service) ExportResultsToGitHub(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("GITHUB_EXPORT_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	cfg, warnings, err := mobileGitHubExportConfigFromPayload(payload)
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	if err != nil {
		return encodeCommand(commandResultFor("GITHUB_EXPORT_CONFIG_INVALID", nil, err.Error(), false, &taskID, warnings))
	}
	body, rowCount, err := s.mobileGitHubExportBodyFromPayload(payload, cfg)
	if err != nil {
		return encodeCommand(commandResultFor("GITHUB_EXPORT_INPUT_INVALID", nil, err.Error(), false, &taskID, warnings))
	}
	if taskID == "" {
		taskID = fmt.Sprintf("manual-%s", time.Now().Format("20060102-150405"))
	}
	result, err := mobileExportCSVToGitHub(context.Background(), cfg, taskID, body, rowCount, time.Now())
	if err != nil {
		return encodeCommand(commandResultFor("GITHUB_EXPORT_FAILED", nil, err.Error(), false, &taskID, warnings))
	}
	return encodeCommand(commandResultFor("GITHUB_EXPORT_OK", result, fmt.Sprintf("已导出 %d 条测速结果到 GitHub。", rowCount), true, &taskID, warnings))
}

func mobileGitHubExportConfigFromPayload(payload map[string]any) (mobileGitHubExportConfig, []string, error) {
	return appcore.GitHubExportConfigFromPayload(payload, mobileGitHubExportConfigDefaults())
}

func mobileGitHubExportConfigFromSnapshot(config map[string]any) (mobileGitHubExportConfig, []string, error) {
	return appcore.GitHubExportConfigFromSnapshot(config, mobileGitHubExportConfigDefaults())
}

func (s *Service) mobileGitHubExportBodyFromPayload(payload map[string]any, cfg mobileGitHubExportConfig) ([]byte, int, error) {
	if rawRows := firstNonNil(payload["results"], payload["rows"]); rawRows != nil {
		rows := mobileProbeRowsFromAny(rawRows)
		if len(rows) == 0 {
			return nil, 0, errors.New("没有可导出的有效测速结果行")
		}
		return encodeMobileProbeRowsForGitHub(rows, cfg)
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	probeCfg, _ := configToProbeConfig(config)
	sourcePath := s.resolveResultFilePath(payload, probeCfg)
	rows, err := readMobileProbeRowsForGitHubFromCSV(sourcePath)
	if err != nil {
		return nil, 0, err
	}
	return encodeMobileProbeRowsForGitHub(rows, cfg)
}

func mobileCSVEncodingFromPayload(payload map[string]any) string {
	return appcore.GitHubCSVEncodingFromPayload(payload)
}

func mobileExportCSVTargetFileName(payload map[string]any, targetValue string, fallback string) string {
	return appcore.GitHubExportCSVTargetFileName(payload, targetValue, fallback)
}

func mobileProbeRowsFromAny(value any) []probeRow {
	return appcore.ProbeRowsFromAny(value)
}

func compactMobileProbeRows(rows []probeRow) []probeRow {
	return appcore.CompactProbeRows(rows)
}

func encodeMobileProbeRowsCSV(rows []probeRow) ([]byte, error) {
	return appcore.EncodeProbeRowsCSV(rows)
}

func encodeMobileProbeRowsCSVWithEncoding(rows []probeRow, csvEncoding string) ([]byte, error) {
	return appcore.EncodeProbeRowsCSVWithEncoding(rows, csvEncoding)
}

func encodeMobileProbeRowsForGitHub(rows []probeRow, cfg mobileGitHubExportConfig) ([]byte, int, error) {
	return appcore.EncodeProbeRowsForGitHub(rows, cfg)
}

func countMobileCSVDataRows(raw []byte) int {
	return appcore.CountCSVDataRows(raw)
}

func readMobileProbeRowsForGitHubFromCSV(path string) ([]probeRow, error) {
	return appcore.ReadProbeRowsForGitHubFromCSV(path)
}

func valueOrDefaultMobileString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func valueOrDefaultMobileFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func valueOrDefaultMobileInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func mobileExportCSVToGitHub(ctx context.Context, cfg mobileGitHubExportConfig, taskID string, body []byte, rowCount int, now time.Time) (mobileGitHubExportResult, error) {
	return appcore.ExportCSVToGitHub(ctx, cfg, taskID, body, rowCount, now, newMobileGitHubExportClient(cfg.Token))
}

func newMobileGitHubExportClient(token string) *mobileGitHubExportClient {
	return appcore.NewGitHubExportClient(token, mobileGitHubAPIBaseURL)
}

func renderMobileGitHubExportTemplate(template string, taskID string, now time.Time) string {
	return appcore.RenderGitHubTemplate(template, taskID, now)
}

func escapeMobileGitHubContentPath(targetPath string) string {
	return appcore.EscapeGitHubContentPath(targetPath)
}

func mobileGitHubExportConfigDefaults() githubcore.ConfigDefaults {
	return githubcore.ConfigDefaults{
		Owner: defaultMobileGitHubExportOwner,
		Repo:  defaultMobileGitHubExportRepo,
	}
}
