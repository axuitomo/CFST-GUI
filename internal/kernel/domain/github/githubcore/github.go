package githubcore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	configvalue "github.com/axuitomo/CFST-GUI/internal/kernel/domain/config"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/utils"
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
	config := configvalue.Map(configvalue.FirstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(config) == 0 {
		config = payload
	}
	return ParseConfigFromSnapshot(config, defaults)
}

func ParseConfigFromSnapshot(config map[string]any, defaults ConfigDefaults) (Config, []string, error) {
	defaults = normalizeConfigDefaults(defaults)
	exportCfg := configvalue.Map(config["export"])
	legacyGithubCfg := configvalue.Map(exportCfg["github"])
	githubCfg := configvalue.Map(config["github"])
	if len(githubCfg) == 0 {
		githubCfg = legacyGithubCfg
	}
	cfg := Config{
		Enabled:               configvalue.Bool(githubCfg["enabled"], false),
		Owner:                 strings.TrimSpace(configvalue.String(githubCfg["owner"], defaults.Owner)),
		Repo:                  strings.TrimSpace(configvalue.String(githubCfg["repo"], defaults.Repo)),
		Branch:                strings.TrimSpace(configvalue.String(githubCfg["branch"], DefaultBranch)),
		PathTemplate:          strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(githubCfg["path_template"], githubCfg["pathTemplate"]), DefaultPathTemplate)),
		Token:                 strings.TrimSpace(configvalue.String(githubCfg["token"], "")),
		CommitMessageTemplate: strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(githubCfg["commit_message_template"], githubCfg["commitMessageTemplate"]), DefaultCommitMessageTemplate)),
		LastExportAt:          strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(githubCfg["last_export_at"], githubCfg["lastExportAt"]), "")),
		CSVEncoding:           utils.NormalizeCSVEncoding(configvalue.String(configvalue.FirstNonNil(exportCfg["csv_encoding"], exportCfg["csvEncoding"]), utils.CSVEncodingUTF8)),
		Format:                NormalizeGitHubExportFormat(configvalue.String(configvalue.FirstNonNil(githubCfg["format"], githubCfg["github_format"], exportCfg["github_format"]), "csv")),
		CSVHeaderTemplate:     configvalue.String(configvalue.FirstNonNil(githubCfg["csv_header_template"], githubCfg["csvHeaderTemplate"]), ""),
		CSVRowTemplate:        configvalue.String(configvalue.FirstNonNil(githubCfg["csv_row_template"], githubCfg["csvRowTemplate"]), ""),
		TXTRowTemplate:        configvalue.String(configvalue.FirstNonNil(githubCfg["txt_row_template"], githubCfg["txtRowTemplate"]), "{ip}"),
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
	config := configvalue.Map(configvalue.FirstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(config) == 0 {
		config = payload
	}
	exportCfg := configvalue.Map(config["export"])
	return utils.NormalizeCSVEncoding(configvalue.String(configvalue.FirstNonNil(exportCfg["csv_encoding"], exportCfg["csvEncoding"]), utils.CSVEncodingUTF8))
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
		// 仓库上传/检查不是测速阶段：固定直连，不跟随系统代理或环境代理变量。
		httpClient = httpclient.NewClient(httpclient.Options{
			DisableProxy: true,
			Protocol:     httpclient.ProtocolTCP,
			Timeout:      30 * time.Second,
		})
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

func isMaskedSecret(value string) bool {
	value = strings.TrimSpace(value)
	return strings.Contains(value, "...") || strings.Contains(value, "***") || strings.Trim(value, "*") == ""
}
