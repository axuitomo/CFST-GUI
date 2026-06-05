package githubcore

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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const (
	DefaultBranch                = "main"
	DefaultCommitMessageTemplate = "CFST results {date} {time}"
	DefaultOwner                 = "axuitomo"
	DefaultPathTemplate          = "cfst-results/{date}/{time}-{task_id}.csv"
	DefaultRepo                  = "CFST-GUI"
)

var APIBaseURL = "https://api.github.com"

var ErrContentNotFound = errors.New("github content not found")

type Config struct {
	Enabled               bool
	Owner                 string
	Repo                  string
	Branch                string
	PathTemplate          string
	Token                 string
	CommitMessageTemplate string
	LastExportAt          string
	CSVEncoding           string
	Format                string
	CSVHeaderTemplate     string
	CSVRowTemplate        string
	TXTRowTemplate        string
}

type ConfigDefaults struct {
	Owner string
	Repo  string
}

type ExportResult struct {
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

type ContentsResponse struct {
	SHA string `json:"sha"`
}

type PutContentsResponse struct {
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
	Content struct {
		HTMLURL string `json:"html_url"`
		Path    string `json:"path"`
		SHA     string `json:"sha"`
	} `json:"content"`
}

type ContentsPutRequest struct {
	Branch  string `json:"branch,omitempty"`
	Content string `json:"content"`
	Message string `json:"message"`
	SHA     string `json:"sha,omitempty"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

type ClientOptions struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

func ParseConfigFromPayload(payload map[string]any, defaults ConfigDefaults) (Config, []string, error) {
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(config) == 0 {
		config = payload
	}
	return ParseConfigFromSnapshot(config, defaults)
}

func ParseConfigFromSnapshot(config map[string]any, defaults ConfigDefaults) (Config, []string, error) {
	defaults = normalizeConfigDefaults(defaults)
	exportCfg := mapValue(config["export"])
	legacyGithubCfg := mapValue(exportCfg["github"])
	githubCfg := mapValue(config["github"])
	if len(githubCfg) == 0 {
		githubCfg = legacyGithubCfg
	}
	cfg := Config{
		Enabled:               boolValue(githubCfg["enabled"], false),
		Owner:                 strings.TrimSpace(stringValue(githubCfg["owner"], defaults.Owner)),
		Repo:                  strings.TrimSpace(stringValue(githubCfg["repo"], defaults.Repo)),
		Branch:                strings.TrimSpace(stringValue(githubCfg["branch"], DefaultBranch)),
		PathTemplate:          strings.TrimSpace(stringValue(firstNonNil(githubCfg["path_template"], githubCfg["pathTemplate"]), DefaultPathTemplate)),
		Token:                 strings.TrimSpace(stringValue(githubCfg["token"], "")),
		CommitMessageTemplate: strings.TrimSpace(stringValue(firstNonNil(githubCfg["commit_message_template"], githubCfg["commitMessageTemplate"]), DefaultCommitMessageTemplate)),
		LastExportAt:          strings.TrimSpace(stringValue(firstNonNil(githubCfg["last_export_at"], githubCfg["lastExportAt"]), "")),
		CSVEncoding:           utils.NormalizeCSVEncoding(stringValue(firstNonNil(exportCfg["csv_encoding"], exportCfg["csvEncoding"]), utils.CSVEncodingUTF8)),
		Format:                NormalizeGitHubExportFormat(stringValue(firstNonNil(githubCfg["format"], githubCfg["github_format"], exportCfg["github_format"]), "csv")),
		CSVHeaderTemplate:     stringValue(firstNonNil(githubCfg["csv_header_template"], githubCfg["csvHeaderTemplate"]), ""),
		CSVRowTemplate:        stringValue(firstNonNil(githubCfg["csv_row_template"], githubCfg["csvRowTemplate"]), ""),
		TXTRowTemplate:        stringValue(firstNonNil(githubCfg["txt_row_template"], githubCfg["txtRowTemplate"]), "{ip}"),
	}
	if cfg.Branch == "" {
		cfg.Branch = DefaultBranch
	}
	if cfg.PathTemplate == "" {
		cfg.PathTemplate = DefaultPathTemplate
	}
	if cfg.CommitMessageTemplate == "" {
		cfg.CommitMessageTemplate = DefaultCommitMessageTemplate
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

func CSVEncodingFromPayload(payload map[string]any) string {
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(config) == 0 {
		config = payload
	}
	exportCfg := mapValue(config["export"])
	return utils.NormalizeCSVEncoding(stringValue(firstNonNil(exportCfg["csv_encoding"], exportCfg["csvEncoding"]), utils.CSVEncodingUTF8))
}

func ProbeRowsFromAny(value any) []probecore.ProbeRow {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var rows []probecore.ProbeRow
	if err := json.Unmarshal(raw, &rows); err == nil {
		rows = CompactProbeRows(rows)
		if len(rows) > 0 {
			return rows
		}
	}
	var generic []map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil
	}
	result := make([]probecore.ProbeRow, 0, len(generic))
	for _, row := range generic {
		ip := strings.TrimSpace(stringValue(firstNonNil(row["ip"], row["address"]), ""))
		if ip == "" {
			continue
		}
		result = append(result, probecore.ProbeRow{
			Colo:               strings.TrimSpace(stringValue(row["colo"], "N/A")),
			DelayMS:            floatValue(firstNonNil(row["delayMs"], row["delay_ms"], row["tcp_latency_ms"], row["tcpLatencyMs"]), 0),
			DownloadSpeedMB:    floatValue(firstNonNil(row["downloadSpeedMb"], row["download_speed_mb"], row["download_mbps"], row["downloadMbps"]), 0),
			IP:                 ip,
			LossRate:           floatValue(firstNonNil(row["lossRate"], row["loss_rate"]), 0),
			MaxDownloadSpeedMB: floatValue(firstNonNil(row["maxDownloadSpeedMb"], row["max_download_speed_mb"], row["max_download_mbps"], row["maxDownloadMbps"]), 0),
			Received:           intValue(firstNonNil(row["received"], row["Received"]), 0),
			Sended:             intValue(firstNonNil(row["sended"], row["sent"], row["Sended"]), 0),
			SourcePort:         intValue(firstNonNil(row["source_port"], row["sourcePort"]), 0),
			TestPort:           intValue(firstNonNil(row["test_port"], row["testPort"]), 0),
			TraceDelayMS:       floatValue(firstNonNil(row["traceDelayMs"], row["trace_delay_ms"], row["trace_latency_ms"], row["traceLatencyMs"]), 0),
		})
	}
	return result
}

func CompactProbeRows(rows []probecore.ProbeRow) []probecore.ProbeRow {
	result := rows[:0]
	for _, row := range rows {
		if strings.TrimSpace(row.IP) != "" {
			result = append(result, row)
		}
	}
	return result
}

func EncodeProbeRowsCSV(rows []probecore.ProbeRow) ([]byte, error) {
	return EncodeProbeRowsCSVWithEncoding(rows, utils.CSVEncodingUTF8)
}

func EncodeProbeRowsCSVWithEncoding(rows []probecore.ProbeRow, csvEncoding string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	if bom := utils.CSVEncodingBOM(csvEncoding); len(bom) > 0 {
		buffer.Write(bom)
	}
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

func NormalizeGitHubExportFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "txt", "text":
		return "txt"
	default:
		return "csv"
	}
}

func EncodeProbeRowsForGitHub(rows []probecore.ProbeRow, cfg Config) ([]byte, int, error) {
	rows = CompactProbeRows(rows)
	if len(rows) == 0 {
		return nil, 0, errors.New("没有可导出的有效测速结果行")
	}
	format := NormalizeGitHubExportFormat(cfg.Format)
	if format == "txt" {
		return encodeProbeRowsTXTWithTemplate(rows, cfg.TXTRowTemplate), len(rows), nil
	}
	if strings.TrimSpace(cfg.CSVHeaderTemplate) == "" && strings.TrimSpace(cfg.CSVRowTemplate) == "" {
		body, err := EncodeProbeRowsCSVWithEncoding(rows, cfg.CSVEncoding)
		return body, len(rows), err
	}
	return encodeProbeRowsCSVWithTemplate(rows, cfg.CSVHeaderTemplate, cfg.CSVRowTemplate), len(rows), nil
}

func encodeProbeRowsCSVWithTemplate(rows []probecore.ProbeRow, headerTemplate, rowTemplate string) []byte {
	lines := make([]string, 0, len(rows)+1)
	if header := strings.TrimSpace(headerTemplate); header != "" {
		lines = append(lines, header)
	}
	rowTemplate = strings.TrimSpace(rowTemplate)
	if rowTemplate == "" {
		rowTemplate = "{ip},{sended},{received},{loss_rate},{tcp_latency_ms},{download_mbps},{max_download_mbps},{colo},{trace_latency_ms},{source_port},{test_port}"
	}
	for index, row := range rows {
		lines = append(lines, renderProbeRowTemplate(rowTemplate, row, index+1))
	}
	return []byte(strings.Join(lines, "\n"))
}

func encodeProbeRowsTXTWithTemplate(rows []probecore.ProbeRow, rowTemplate string) []byte {
	rowTemplate = strings.TrimSpace(rowTemplate)
	if rowTemplate == "" {
		rowTemplate = "{ip}"
	}
	lines := make([]string, 0, len(rows))
	for index, row := range rows {
		lines = append(lines, renderProbeRowTemplate(rowTemplate, row, index+1))
	}
	return []byte(strings.Join(lines, "\n"))
}

func renderProbeRowTemplate(template string, row probecore.ProbeRow, index int) string {
	replacements := map[string]string{
		"{index}":             strconv.Itoa(index),
		"{ip}":                row.IP,
		"{colo}":              rowColo(row),
		"{sended}":            strconv.Itoa(row.Sended),
		"{received}":          strconv.Itoa(row.Received),
		"{loss_rate}":         formatMetric(row.LossRate),
		"{tcp_latency_ms}":    formatMetric(row.DelayMS),
		"{trace_latency_ms}":  formatMetric(row.TraceDelayMS),
		"{download_mbps}":     formatMetric(row.DownloadSpeedMB),
		"{max_download_mbps}": formatMetric(probeRowMaxDownloadSpeedMB(row)),
		"{source_port}":       formatOptionalInt(row.SourcePort),
		"{test_port}":         formatOptionalInt(row.TestPort),
	}
	for key, value := range replacements {
		template = strings.ReplaceAll(template, key, value)
	}
	return template
}

func CountCSVDataRows(raw []byte) int {
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

func ExportCSVTargetFileName(payload map[string]any, targetValue string, fallback string) string {
	if fileName := SanitizeTemplateFileName(stringValue(firstNonNil(payload["file_name"], payload["fileName"]), "")); fileName != "" {
		return fileName
	}
	targetValue = strings.TrimSpace(targetValue)
	if strings.HasPrefix(targetValue, "browser-download:") {
		if fileName := SanitizeTemplateFileName(strings.TrimPrefix(targetValue, "browser-download:")); fileName != "" {
			return fileName
		}
	}
	if targetValue != "" {
		if fileName := SanitizeTemplateFileName(filepath.Base(targetValue)); fileName != "" && fileName != "." {
			return fileName
		}
		if parsed, err := url.Parse(targetValue); err == nil {
			if fileName := SanitizeTemplateFileName(path.Base(parsed.Path)); fileName != "" && fileName != "." {
				return fileName
			}
		}
	}
	return fallback
}

func SanitizeTemplateFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return strings.TrimSpace(replacer.Replace(value))
}

func NewClient(token string) *Client {
	return NewClientWithOptions(ClientOptions{Token: token})
}

func NewClientWithOptions(options ClientOptions) *Client {
	baseURL := strings.TrimRight(options.BaseURL, "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(APIBaseURL, "/")
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		token:      options.Token,
	}
}

func (c *Client) CheckRepository(ctx context.Context, cfg Config) error {
	endpoint, err := c.endpoint("/repos/" + url.PathEscape(cfg.Owner) + "/" + url.PathEscape(cfg.Repo))
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodGet, endpoint.String(), nil, nil)
}

func (c *Client) CheckExportAccess(ctx context.Context, cfg Config) error {
	if err := c.CheckRepository(ctx, cfg); err != nil {
		return err
	}
	if err := c.CheckBranch(ctx, cfg); err != nil {
		return err
	}
	return c.CheckContentsRead(ctx, cfg, RenderTemplate(cfg.PathTemplate, "test", time.Now()))
}

func (c *Client) CheckBranch(ctx context.Context, cfg Config) error {
	endpoint, err := c.endpoint("/repos/" + url.PathEscape(cfg.Owner) + "/" + url.PathEscape(cfg.Repo) + "/branches/" + url.PathEscape(cfg.Branch))
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodGet, endpoint.String(), nil, nil)
}

func (c *Client) CheckContentsRead(ctx context.Context, cfg Config, targetPath string) error {
	contentPath := path.Dir(strings.Trim(targetPath, "/"))
	if contentPath == "." {
		contentPath = ""
	}
	endpoint, err := c.endpoint("/repos/" + url.PathEscape(cfg.Owner) + "/" + url.PathEscape(cfg.Repo) + "/contents/" + EscapeContentPath(contentPath))
	if err != nil {
		return err
	}
	query := endpoint.Query()
	if cfg.Branch != "" {
		query.Set("ref", cfg.Branch)
	}
	endpoint.RawQuery = query.Encode()
	err = c.do(ctx, http.MethodGet, endpoint.String(), nil, nil)
	if errors.Is(err, ErrContentNotFound) {
		return nil
	}
	return err
}

func (c *Client) GetContentSHA(ctx context.Context, cfg Config, targetPath string) (string, error) {
	endpoint, err := c.endpoint("/repos/" + url.PathEscape(cfg.Owner) + "/" + url.PathEscape(cfg.Repo) + "/contents/" + EscapeContentPath(targetPath))
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	if cfg.Branch != "" {
		query.Set("ref", cfg.Branch)
	}
	endpoint.RawQuery = query.Encode()
	var response ContentsResponse
	err = c.do(ctx, http.MethodGet, endpoint.String(), nil, &response)
	if err != nil {
		if errors.Is(err, ErrContentNotFound) {
			return "", nil
		}
		return "", err
	}
	return response.SHA, nil
}

func (c *Client) PutContent(ctx context.Context, cfg Config, targetPath string, message string, body []byte, sha string) (PutContentsResponse, error) {
	endpoint, err := c.endpoint("/repos/" + url.PathEscape(cfg.Owner) + "/" + url.PathEscape(cfg.Repo) + "/contents/" + EscapeContentPath(targetPath))
	if err != nil {
		return PutContentsResponse{}, err
	}
	request := ContentsPutRequest{
		Branch:  cfg.Branch,
		Content: base64.StdEncoding.EncodeToString(body),
		Message: message,
		SHA:     sha,
	}
	var response PutContentsResponse
	if err := c.do(ctx, http.MethodPut, endpoint.String(), request, &response); err != nil {
		return PutContentsResponse{}, err
	}
	return response, nil
}

func ExportCSV(ctx context.Context, client *Client, cfg Config, taskID string, body []byte, rowCount int, now time.Time) (ExportResult, error) {
	if rowCount <= 0 || len(body) == 0 {
		return ExportResult{}, errors.New("没有可导出的测速结果")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	if client == nil {
		client = NewClient(cfg.Token)
	}
	targetPath := RenderTemplate(cfg.PathTemplate, taskID, now)
	message := RenderTemplate(cfg.CommitMessageTemplate, taskID, now)
	if message == "" {
		message = RenderTemplate(DefaultCommitMessageTemplate, taskID, now)
	}
	sha, err := client.GetContentSHA(ctx, cfg, targetPath)
	if err != nil {
		return ExportResult{}, err
	}
	response, err := client.PutContent(ctx, cfg, targetPath, message, body, sha)
	if err != nil {
		return ExportResult{}, err
	}
	return ExportResult{
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

func RenderTemplate(template string, taskID string, now time.Time) string {
	if template == "" {
		template = DefaultPathTemplate
	}
	values := map[string]string{
		"{date}":      now.Format("2006-01-02"),
		"{time}":      now.Format("150405"),
		"{task_id}":   sanitizePathPart(taskID),
		"{taskId}":    sanitizePathPart(taskID),
		"{timestamp}": now.Format("20060102-150405"),
	}
	result := template
	for key, value := range values {
		result = strings.ReplaceAll(result, key, value)
	}
	result = strings.ReplaceAll(result, "\\", "/")
	result = path.Clean(strings.TrimLeft(result, "/"))
	if result == "." || strings.HasPrefix(result, "../") || result == ".." {
		return DefaultPathTemplate
	}
	return result
}

func EscapeContentPath(targetPath string) string {
	parts := strings.Split(strings.Trim(targetPath, "/"), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func (c *Client) endpoint(rawPath string) (*url.URL, error) {
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	parsed.Path = path.Join(parsed.Path, rawPath)
	return parsed, nil
}

func (c *Client) do(ctx context.Context, method, endpoint string, body any, target any) error {
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
		return ErrContentNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("GitHub API 返回状态 %s：%s", res.Status, strings.TrimSpace(string(raw)))
	}
	if target == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func normalizeConfigDefaults(defaults ConfigDefaults) ConfigDefaults {
	defaults.Owner = strings.TrimSpace(defaults.Owner)
	defaults.Repo = strings.TrimSpace(defaults.Repo)
	if defaults.Owner == "" {
		defaults.Owner = DefaultOwner
	}
	if defaults.Repo == "" {
		defaults.Repo = DefaultRepo
	}
	return defaults
}

func probeRowMaxDownloadSpeedMB(row probecore.ProbeRow) float64 {
	if row.MaxDownloadSpeedMB > 0 {
		return row.MaxDownloadSpeedMB
	}
	return row.DownloadSpeedMB
}

func rowColo(row probecore.ProbeRow) string {
	colo := strings.TrimSpace(row.Colo)
	if colo == "" {
		return ""
	}
	return colo
}

func formatMetric(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func formatOptionalInt(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func sanitizePathPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "manual"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-")
	return replacer.Replace(value)
}

func isMaskedSecret(value string) bool {
	value = strings.TrimSpace(value)
	return strings.Contains(value, "...") || strings.Contains(value, "***") || strings.Trim(value, "*") == ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func mapValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if value == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]any{}
	}
	if result == nil {
		return map[string]any{}
	}
	return result
}

func stringValue(value any, fallback string) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case nil:
		return fallback
	default:
		return fmt.Sprint(value)
	}
}

func boolValue(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	}
	return fallback
}

func intValue(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func floatValue(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
