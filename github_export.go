package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGitHubExportBranch                = "main"
	defaultGitHubExportCommitMessageTemplate = "CFST results {date} {time}"
	defaultGitHubExportPathTemplate          = "cfst-results/{date}/{time}-{task_id}.csv"
)

var githubAPIBaseURL = "https://api.github.com"

type githubExportConfig struct {
	Enabled               bool
	Owner                 string
	Repo                  string
	Branch                string
	PathTemplate          string
	Token                 string
	CommitMessageTemplate string
	LastExportAt          string
}

type GitHubExportResult struct {
	Branch      string `json:"branch"`
	CommitSHA   string `json:"commit_sha"`
	ContentSHA  string `json:"content_sha"`
	ExportedAt  string `json:"exported_at"`
	HTMLURL     string `json:"html_url"`
	Owner       string `json:"owner"`
	Path        string `json:"path"`
	Repo        string `json:"repo"`
	WrittenRows int    `json:"written_rows"`
}

type githubContentsResponse struct {
	SHA string `json:"sha"`
}

type githubPutContentsResponse struct {
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
	Content struct {
		HTMLURL string `json:"html_url"`
		Path    string `json:"path"`
		SHA     string `json:"sha"`
	} `json:"content"`
}

type githubContentsPutRequest struct {
	Branch  string `json:"branch,omitempty"`
	Content string `json:"content"`
	Message string `json:"message"`
	SHA     string `json:"sha,omitempty"`
}

type githubExportClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

func (a *App) TestGitHubExport(payload map[string]any) DesktopCommandResult {
	cfg, warnings, err := githubExportConfigFromPayload(payload)
	if err != nil {
		return desktopCommandResult("GITHUB_EXPORT_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := newGitHubExportClient(cfg.Token)
	if err := client.checkExportAccess(ctx, cfg); err != nil {
		return desktopCommandResult("GITHUB_EXPORT_TEST_FAILED", nil, err.Error(), false, nil, warnings)
	}
	return desktopCommandResult("GITHUB_EXPORT_TEST_OK", map[string]any{
		"branch": cfg.Branch,
		"owner":  cfg.Owner,
		"repo":   cfg.Repo,
	}, "GitHub 仓库、分支与 Contents 读取权限已验证。", true, nil, warnings)
}

func (a *App) ExportResultsToGitHub(payload map[string]any) DesktopCommandResult {
	cfg, warnings, err := githubExportConfigFromPayload(payload)
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	if err != nil {
		return desktopCommandResult("GITHUB_EXPORT_CONFIG_INVALID", nil, err.Error(), false, &taskID, warnings)
	}
	body, rowCount, err := githubExportCSVFromPayload(payload)
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
	body, err := encodeProbeRowsCSV(rows)
	if err != nil {
		return GitHubExportResult{}, err
	}
	return exportCSVToGitHub(ctx, cfg, taskID, body, len(rows), now)
}

func exportCSVToGitHub(ctx context.Context, cfg githubExportConfig, taskID string, body []byte, rowCount int, now time.Time) (GitHubExportResult, error) {
	if rowCount <= 0 || len(body) == 0 {
		return GitHubExportResult{}, errors.New("没有可导出的测速结果")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	targetPath := renderGitHubExportTemplate(cfg.PathTemplate, taskID, now)
	message := renderGitHubExportTemplate(cfg.CommitMessageTemplate, taskID, now)
	if message == "" {
		message = renderGitHubExportTemplate(defaultGitHubExportCommitMessageTemplate, taskID, now)
	}
	client := newGitHubExportClient(cfg.Token)
	sha, err := client.getContentSHA(ctx, cfg, targetPath)
	if err != nil {
		return GitHubExportResult{}, err
	}
	response, err := client.putContent(ctx, cfg, targetPath, message, body, sha)
	if err != nil {
		return GitHubExportResult{}, err
	}
	return GitHubExportResult{
		Branch:      cfg.Branch,
		CommitSHA:   response.Commit.SHA,
		ContentSHA:  response.Content.SHA,
		ExportedAt:  now.Format(time.RFC3339),
		HTMLURL:     response.Content.HTMLURL,
		Owner:       cfg.Owner,
		Path:        response.Content.Path,
		Repo:        cfg.Repo,
		WrittenRows: rowCount,
	}, nil
}

func githubExportConfigFromPayload(payload map[string]any) (githubExportConfig, []string, error) {
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(config) == 0 {
		config = payload
	}
	return githubExportConfigFromSnapshot(config)
}

func githubExportConfigFromSnapshot(config map[string]any) (githubExportConfig, []string, error) {
	exportCfg := mapValue(config["export"])
	githubCfg := mapValue(exportCfg["github"])
	if len(githubCfg) == 0 {
		githubCfg = mapValue(config["github"])
	}
	cfg := githubExportConfig{
		Enabled:               boolValue(githubCfg["enabled"], false),
		Owner:                 strings.TrimSpace(stringValue(githubCfg["owner"], defaultGitHubExportOwner())),
		Repo:                  strings.TrimSpace(stringValue(githubCfg["repo"], defaultGitHubExportRepo())),
		Branch:                strings.TrimSpace(stringValue(githubCfg["branch"], defaultGitHubExportBranch)),
		PathTemplate:          strings.TrimSpace(stringValue(firstNonNil(githubCfg["path_template"], githubCfg["pathTemplate"]), defaultGitHubExportPathTemplate)),
		Token:                 strings.TrimSpace(stringValue(githubCfg["token"], "")),
		CommitMessageTemplate: strings.TrimSpace(stringValue(firstNonNil(githubCfg["commit_message_template"], githubCfg["commitMessageTemplate"]), defaultGitHubExportCommitMessageTemplate)),
		LastExportAt:          strings.TrimSpace(stringValue(firstNonNil(githubCfg["last_export_at"], githubCfg["lastExportAt"]), "")),
	}
	if cfg.Branch == "" {
		cfg.Branch = defaultGitHubExportBranch
	}
	if cfg.PathTemplate == "" {
		cfg.PathTemplate = defaultGitHubExportPathTemplate
	}
	if cfg.CommitMessageTemplate == "" {
		cfg.CommitMessageTemplate = defaultGitHubExportCommitMessageTemplate
	}
	warnings := make([]string, 0)
	if cfg.Owner == "" {
		return cfg, warnings, errors.New("缺少 GitHub owner")
	}
	if cfg.Repo == "" {
		return cfg, warnings, errors.New("缺少 GitHub repo")
	}
	if cfg.Token == "" || isMaskedSecret(cfg.Token) {
		return cfg, warnings, errors.New("缺少完整 GitHub PAT")
	}
	return cfg, warnings, nil
}

func githubExportCSVFromPayload(payload map[string]any) ([]byte, int, error) {
	if rawRows := firstNonNil(payload["results"], payload["rows"]); rawRows != nil {
		rows := probeRowsFromAny(rawRows)
		if len(rows) == 0 {
			return nil, 0, errors.New("没有可导出的有效测速结果行")
		}
		body, err := encodeProbeRowsCSV(rows)
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

func probeRowsFromAny(value any) []ProbeRow {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var rows []ProbeRow
	if err := json.Unmarshal(raw, &rows); err == nil {
		rows = compactProbeRows(rows)
		if len(rows) > 0 {
			return rows
		}
	}
	var generic []map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil
	}
	result := make([]ProbeRow, 0, len(generic))
	for _, row := range generic {
		ip := strings.TrimSpace(stringValue(firstNonNil(row["ip"], row["address"]), ""))
		if ip == "" {
			continue
		}
		result = append(result, ProbeRow{
			Colo:               strings.TrimSpace(stringValue(row["colo"], "N/A")),
			DelayMS:            floatValue(firstNonNil(row["delayMs"], row["delay_ms"], row["tcp_latency_ms"], row["tcpLatencyMs"]), 0),
			DownloadSpeedMB:    floatValue(firstNonNil(row["downloadSpeedMb"], row["download_speed_mb"], row["download_mbps"], row["downloadMbps"]), 0),
			IP:                 ip,
			LossRate:           floatValue(firstNonNil(row["lossRate"], row["loss_rate"]), 0),
			MaxDownloadSpeedMB: floatValue(firstNonNil(row["maxDownloadSpeedMb"], row["max_download_speed_mb"], row["max_download_mbps"], row["maxDownloadMbps"]), 0),
			Received:           intValue(firstNonNil(row["received"], row["Received"]), 0),
			Sended:             intValue(firstNonNil(row["sended"], row["sent"], row["Sended"]), 0),
			TraceDelayMS:       floatValue(firstNonNil(row["traceDelayMs"], row["trace_delay_ms"], row["trace_latency_ms"], row["traceLatencyMs"]), 0),
		})
	}
	return result
}

func compactProbeRows(rows []ProbeRow) []ProbeRow {
	result := rows[:0]
	for _, row := range rows {
		if strings.TrimSpace(row.IP) != "" {
			result = append(result, row)
		}
	}
	return result
}

func encodeProbeRowsCSV(rows []ProbeRow) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write([]string{"IP 地址", "已发送", "已接收", "丢包率", "TCP延迟(ms)", "平均速率(MB/s)", "最高速率(MB/s)", "地区码", "追踪延迟(ms)"}); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if strings.TrimSpace(row.IP) == "" {
			continue
		}
		colo := strings.TrimSpace(row.Colo)
		if colo == "" {
			colo = "N/A"
		}
		record := []string{
			row.IP,
			strconv.Itoa(row.Sended),
			strconv.Itoa(row.Received),
			strconv.FormatFloat(row.LossRate, 'f', 2, 64),
			strconv.FormatFloat(row.DelayMS, 'f', 2, 64),
			strconv.FormatFloat(row.DownloadSpeedMB, 'f', 2, 64),
			strconv.FormatFloat(probeRowMaxDownloadSpeedMB(row), 'f', 2, 64),
			colo,
			strconv.FormatFloat(row.TraceDelayMS, 'f', 2, 64),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

func probeRowMaxDownloadSpeedMB(row ProbeRow) float64 {
	if row.MaxDownloadSpeedMB > 0 {
		return row.MaxDownloadSpeedMB
	}
	return row.DownloadSpeedMB
}

func countCSVDataRows(raw []byte) int {
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) <= 1 {
		return 0
	}
	count := 0
	for _, record := range records[1:] {
		if len(record) > 0 && strings.TrimSpace(record[0]) != "" {
			count++
		}
	}
	return count
}

func newGitHubExportClient(token string) *githubExportClient {
	return &githubExportClient{
		baseURL: strings.TrimRight(githubAPIBaseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token: token,
	}
}

func (c *githubExportClient) checkRepository(ctx context.Context, cfg githubExportConfig) error {
	endpoint, err := c.endpoint("/repos/" + url.PathEscape(cfg.Owner) + "/" + url.PathEscape(cfg.Repo))
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodGet, endpoint.String(), nil, nil)
}

func (c *githubExportClient) checkExportAccess(ctx context.Context, cfg githubExportConfig) error {
	if err := c.checkRepository(ctx, cfg); err != nil {
		return err
	}
	if err := c.checkBranch(ctx, cfg); err != nil {
		return err
	}
	return c.checkContentsRead(ctx, cfg, renderGitHubExportTemplate(cfg.PathTemplate, "test", time.Now()))
}

func (c *githubExportClient) checkBranch(ctx context.Context, cfg githubExportConfig) error {
	endpoint, err := c.endpoint("/repos/" + url.PathEscape(cfg.Owner) + "/" + url.PathEscape(cfg.Repo) + "/branches/" + url.PathEscape(cfg.Branch))
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodGet, endpoint.String(), nil, nil)
}

func (c *githubExportClient) checkContentsRead(ctx context.Context, cfg githubExportConfig, targetPath string) error {
	contentPath := path.Dir(strings.Trim(targetPath, "/"))
	if contentPath == "." {
		contentPath = ""
	}
	endpoint, err := c.endpoint("/repos/" + url.PathEscape(cfg.Owner) + "/" + url.PathEscape(cfg.Repo) + "/contents/" + escapeGitHubContentPath(contentPath))
	if err != nil {
		return err
	}
	query := endpoint.Query()
	if cfg.Branch != "" {
		query.Set("ref", cfg.Branch)
	}
	endpoint.RawQuery = query.Encode()
	err = c.do(ctx, http.MethodGet, endpoint.String(), nil, nil)
	if errors.Is(err, errGitHubContentNotFound) {
		return nil
	}
	return err
}

func (c *githubExportClient) getContentSHA(ctx context.Context, cfg githubExportConfig, targetPath string) (string, error) {
	endpoint, err := c.endpoint("/repos/" + url.PathEscape(cfg.Owner) + "/" + url.PathEscape(cfg.Repo) + "/contents/" + escapeGitHubContentPath(targetPath))
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	if cfg.Branch != "" {
		query.Set("ref", cfg.Branch)
	}
	endpoint.RawQuery = query.Encode()
	var response githubContentsResponse
	err = c.do(ctx, http.MethodGet, endpoint.String(), nil, &response)
	if err != nil {
		if errors.Is(err, errGitHubContentNotFound) {
			return "", nil
		}
		return "", err
	}
	return response.SHA, nil
}

func (c *githubExportClient) putContent(ctx context.Context, cfg githubExportConfig, targetPath string, message string, body []byte, sha string) (githubPutContentsResponse, error) {
	endpoint, err := c.endpoint("/repos/" + url.PathEscape(cfg.Owner) + "/" + url.PathEscape(cfg.Repo) + "/contents/" + escapeGitHubContentPath(targetPath))
	if err != nil {
		return githubPutContentsResponse{}, err
	}
	request := githubContentsPutRequest{
		Branch:  cfg.Branch,
		Content: base64.StdEncoding.EncodeToString(body),
		Message: message,
		SHA:     sha,
	}
	var response githubPutContentsResponse
	if err := c.do(ctx, http.MethodPut, endpoint.String(), request, &response); err != nil {
		return githubPutContentsResponse{}, err
	}
	return response, nil
}

var errGitHubContentNotFound = errors.New("github content not found")

func (c *githubExportClient) endpoint(rawPath string) (*url.URL, error) {
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	parsed.Path = path.Join(parsed.Path, rawPath)
	return parsed, nil
}

func (c *githubExportClient) do(ctx context.Context, method, endpoint string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusNotFound && method == http.MethodGet && strings.Contains(endpoint, "/contents") {
		return errGitHubContentNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("GitHub API 返回状态 %s：%s", res.Status, strings.TrimSpace(string(raw)))
	}
	if target == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func renderGitHubExportTemplate(template string, taskID string, now time.Time) string {
	if template == "" {
		template = defaultGitHubExportPathTemplate
	}
	values := map[string]string{
		"{date}":      now.Format("2006-01-02"),
		"{time}":      now.Format("150405"),
		"{task_id}":   sanitizeGitHubPathPart(taskID),
		"{taskId}":    sanitizeGitHubPathPart(taskID),
		"{timestamp}": now.Format("20060102-150405"),
	}
	result := template
	for key, value := range values {
		result = strings.ReplaceAll(result, key, value)
	}
	result = strings.ReplaceAll(result, "\\", "/")
	result = path.Clean(strings.TrimLeft(result, "/"))
	if result == "." || strings.HasPrefix(result, "../") || result == ".." {
		return defaultGitHubExportPathTemplate
	}
	return result
}

func sanitizeGitHubPathPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "manual"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-")
	return replacer.Replace(value)
}

func escapeGitHubContentPath(targetPath string) string {
	parts := strings.Split(strings.Trim(targetPath, "/"), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func defaultGitHubExportOwner() string {
	owner, _ := defaultGitHubOwnerRepoFromOrigin()
	if owner != "" {
		return owner
	}
	return "axuitomo"
}

func defaultGitHubExportRepo() string {
	_, repo := defaultGitHubOwnerRepoFromOrigin()
	if repo != "" {
		return repo
	}
	return "CFST-GUI"
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
