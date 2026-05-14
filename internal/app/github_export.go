package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/githubcore"
)

const (
	defaultGitHubExportBranch                = githubcore.DefaultBranch
	defaultGitHubExportCommitMessageTemplate = githubcore.DefaultCommitMessageTemplate
	defaultGitHubExportPathTemplate          = githubcore.DefaultPathTemplate
)

var githubAPIBaseURL = githubcore.APIBaseURL

type githubExportConfig = githubcore.Config
type GitHubExportResult = githubcore.ExportResult
type githubContentsResponse = githubcore.ContentsResponse
type githubPutContentsResponse = githubcore.PutContentsResponse
type githubContentsPutRequest = githubcore.ContentsPutRequest
type githubExportClient = githubcore.Client

func (a *App) TestGitHubExport(payload map[string]any) DesktopCommandResult {
	cfg, warnings, err := githubExportConfigFromPayload(payload)
	if err != nil {
		return desktopCommandResult("GITHUB_EXPORT_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := newGitHubExportClient(cfg.Token).CheckExportAccess(ctx, cfg); err != nil {
		return desktopCommandResult("GITHUB_EXPORT_TEST_FAILED", nil, err.Error(), false, nil, warnings)
	}
	return desktopCommandResult("GITHUB_EXPORT_TEST_OK", map[string]any{
		"branch": cfg.Branch,
		"owner":  cfg.Owner,
		"repo":   cfg.Repo,
	}, "GitHub 仓库、分支与 Contents 读取权限已验证。", true, nil, warnings)
}

func (a *App) ExportResultsCSV(payload map[string]any) DesktopCommandResult {
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	csvEncoding := csvEncodingFromPayload(payload)
	body, rowCount, err := githubExportCSVFromPayload(payload, csvEncoding)
	if err != nil {
		return desktopCommandResult("RESULTS_CSV_EXPORT_INPUT_INVALID", nil, err.Error(), false, &taskID, nil)
	}

	targetURI := strings.TrimSpace(stringValue(firstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"]), ""))
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	fileName := exportCSVTargetFileName(payload, firstNonEmpty(targetPath, targetURI), "result.csv")
	message := fmt.Sprintf("已导出 %d 条测速结果 CSV。", rowCount)

	if targetURI != "" {
		return desktopCommandResult("RESULTS_CSV_EXPORT_OK", map[string]any{
			"content_base64": base64.StdEncoding.EncodeToString(body),
			"file_name":      fileName,
			"target_uri":     targetURI,
			"written_count":  rowCount,
		}, message, true, &taskID, nil)
	}
	if targetPath == "" {
		return desktopCommandResult("RESULTS_CSV_EXPORT_INVALID", nil, "缺少导出目标路径。", false, &taskID, nil)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return desktopCommandResult("RESULTS_CSV_EXPORT_WRITE_FAILED", nil, err.Error(), false, &taskID, nil)
	}
	if err := os.WriteFile(targetPath, body, 0o644); err != nil {
		return desktopCommandResult("RESULTS_CSV_EXPORT_WRITE_FAILED", nil, err.Error(), false, &taskID, nil)
	}
	return desktopCommandResult("RESULTS_CSV_EXPORT_OK", map[string]any{
		"file_name":     exportCSVTargetFileName(payload, targetPath, fileName),
		"path":          targetPath,
		"written_count": rowCount,
	}, message, true, &taskID, nil)
}

func (a *App) ExportResultsToGitHub(payload map[string]any) DesktopCommandResult {
	cfg, warnings, err := githubExportConfigFromPayload(payload)
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	if err != nil {
		return desktopCommandResult("GITHUB_EXPORT_CONFIG_INVALID", nil, err.Error(), false, &taskID, warnings)
	}
	body, rowCount, err := githubExportCSVFromPayload(payload, cfg.CSVEncoding)
	if err != nil {
		return desktopCommandResult("GITHUB_EXPORT_INPUT_INVALID", nil, err.Error(), false, &taskID, warnings)
	}
	if taskID == "" {
		taskID = fmt.Sprintf("manual-%s", time.Now().Format("20060102-150405"))
	}
	result, err := exportCSVToGitHub(context.Background(), cfg, taskID, body, rowCount, time.Now())
	if err != nil {
		return desktopCommandResult("GITHUB_EXPORT_FAILED", nil, err.Error(), false, &taskID, warnings)
	}
	return desktopCommandResult("GITHUB_EXPORT_OK", result, fmt.Sprintf("已导出 %d 条测速结果到 GitHub。", rowCount), true, &taskID, warnings)
}

func exportProbeRowsToGitHub(ctx context.Context, config map[string]any, taskID string, rows []ProbeRow, now time.Time) (GitHubExportResult, error) {
	cfg, _, err := githubExportConfigFromSnapshot(config)
	if err != nil {
		return GitHubExportResult{}, err
	}
	body, err := encodeProbeRowsCSVWithEncoding(rows, cfg.CSVEncoding)
	if err != nil {
		return GitHubExportResult{}, err
	}
	return exportCSVToGitHub(ctx, cfg, taskID, body, len(rows), now)
}

func exportCSVToGitHub(ctx context.Context, cfg githubExportConfig, taskID string, body []byte, rowCount int, now time.Time) (GitHubExportResult, error) {
	return githubcore.ExportCSV(ctx, newGitHubExportClient(cfg.Token), cfg, taskID, body, rowCount, now)
}

func githubExportConfigFromPayload(payload map[string]any) (githubExportConfig, []string, error) {
	return githubcore.ParseConfigFromPayload(payload, githubExportConfigDefaults())
}

func githubExportConfigFromSnapshot(config map[string]any) (githubExportConfig, []string, error) {
	return githubcore.ParseConfigFromSnapshot(config, githubExportConfigDefaults())
}

func githubExportCSVFromPayload(payload map[string]any, csvEncoding string) ([]byte, int, error) {
	if rawRows := firstNonNil(payload["results"], payload["rows"]); rawRows != nil {
		rows := probeRowsFromAny(rawRows)
		if len(rows) == 0 {
			return nil, 0, errors.New("没有可导出的有效测速结果行")
		}
		body, err := encodeProbeRowsCSVWithEncoding(rows, csvEncoding)
		return body, len(rows), err
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	cfg, _ := desktopConfigToProbeConfig(config)
	sourcePath := resolveDesktopResultFilePath(payload, cfg)
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, 0, fmt.Errorf("读取结果 CSV 失败：%w", err)
	}
	rowCount := countCSVDataRows(raw)
	if rowCount == 0 {
		return nil, 0, errors.New("结果 CSV 没有可导出的数据行")
	}
	return raw, rowCount, nil
}

func csvEncodingFromPayload(payload map[string]any) string {
	return githubcore.CSVEncodingFromPayload(payload)
}

func exportCSVTargetFileName(payload map[string]any, targetValue string, fallback string) string {
	return githubcore.ExportCSVTargetFileName(payload, targetValue, fallback)
}

func probeRowsFromAny(value any) []ProbeRow {
	return githubcore.ProbeRowsFromAny(value)
}

func compactProbeRows(rows []ProbeRow) []ProbeRow {
	return githubcore.CompactProbeRows(rows)
}

func encodeProbeRowsCSV(rows []ProbeRow) ([]byte, error) {
	return githubcore.EncodeProbeRowsCSV(rows)
}

func encodeProbeRowsCSVWithEncoding(rows []ProbeRow, csvEncoding string) ([]byte, error) {
	return githubcore.EncodeProbeRowsCSVWithEncoding(rows, csvEncoding)
}

func countCSVDataRows(raw []byte) int {
	return githubcore.CountCSVDataRows(raw)
}

func newGitHubExportClient(token string) *githubExportClient {
	return githubcore.NewClientWithOptions(githubcore.ClientOptions{
		BaseURL: githubAPIBaseURL,
		Token:   token,
	})
}

func renderGitHubExportTemplate(template string, taskID string, now time.Time) string {
	return githubcore.RenderTemplate(template, taskID, now)
}

func escapeGitHubContentPath(targetPath string) string {
	return githubcore.EscapeContentPath(targetPath)
}

func githubExportConfigDefaults() githubcore.ConfigDefaults {
	return githubcore.ConfigDefaults{
		Owner: defaultGitHubExportOwner(),
		Repo:  defaultGitHubExportRepo(),
	}
}

func defaultGitHubExportOwner() string {
	owner, _ := defaultGitHubOwnerRepoFromOrigin()
	if owner != "" {
		return owner
	}
	return githubcore.DefaultOwner
}

func defaultGitHubExportRepo() string {
	_, repo := defaultGitHubOwnerRepoFromOrigin()
	if repo != "" {
		return repo
	}
	return githubcore.DefaultRepo
}

func defaultGitHubOwnerRepoFromOrigin() (string, string) {
	raw, err := os.ReadFile(filepath.Join(".git", "config"))
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "url =") {
			continue
		}
		owner, repo := parseGitHubOwnerRepo(strings.TrimSpace(strings.TrimPrefix(line, "url =")))
		if owner != "" && repo != "" {
			return owner, repo
		}
	}
	return "", ""
}

func parseGitHubOwnerRepo(raw string) (string, string) {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, ".git"))
	if strings.HasPrefix(raw, "git@github.com:") {
		raw = strings.TrimPrefix(raw, "git@github.com:")
	} else if parsed, err := url.Parse(raw); err == nil && strings.EqualFold(parsed.Host, "github.com") {
		raw = strings.TrimPrefix(parsed.Path, "/")
	}
	parts := strings.Split(raw, "/")
	if len(parts) < 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[len(parts)-2]), strings.TrimSpace(parts[len(parts)-1])
}
