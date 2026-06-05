package cloudflarecore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
)

var APIBaseURL = "https://api.cloudflare.com/client/v4"

const (
	RecordTypeA     = "A"
	RecordTypeAAAA  = "AAAA"
	RecordTypeCNAME = "CNAME"
	DefaultTTL      = 300

	OperationList   = "list"
	OperationCreate = "create"
	OperationUpdate = "update"
	OperationDelete = "delete"
)

type Config struct {
	APIToken   string
	Comment    string
	Proxied    bool
	RecordName string
	RecordType string
	TTL        int
	ZoneID     string
}

type Record struct {
	Comment    string `json:"comment"`
	Content    string `json:"content"`
	CreatedOn  string `json:"created_on,omitempty"`
	ID         string `json:"id"`
	ModifiedOn string `json:"modified_on,omitempty"`
	Name       string `json:"name"`
	Proxied    bool   `json:"proxied"`
	TTL        int    `json:"ttl"`
	Type       string `json:"type"`
}

type PushSummary struct {
	Created int `json:"created"`
	Deleted int `json:"deleted"`
	Ignored int `json:"ignored"`
	Updated int `json:"updated"`
}

type PushIPGroups struct {
	A    []string
	AAAA []string
}

type ListOptions struct {
	Name string
	Type string
}

type PushResult struct {
	HasInputIPs    bool
	IgnoredEntries []string
	RecordsAfter   []Record
	Summary        PushSummary
	Warnings       []string
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

type OperationError struct {
	Operation string
	Err       error
}

func (e *OperationError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type listResponse struct {
	Success    bool       `json:"success"`
	Errors     []apiError `json:"errors"`
	Messages   []apiError `json:"messages"`
	Result     []Record   `json:"result"`
	ResultInfo struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

type recordResponse struct {
	Success  bool       `json:"success"`
	Errors   []apiError `json:"errors"`
	Messages []apiError `json:"messages"`
	Result   Record     `json:"result"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func ParseConfigFromPayload(payload map[string]any) (Config, []string, error) {
	return parseConfigFromPayload(payload, true)
}

func ParseListConfigFromPayload(payload map[string]any) (Config, []string, error) {
	return parseConfigFromPayload(payload, false)
}

func parseConfigFromPayload(payload map[string]any, requireRecordName bool) (Config, []string, error) {
	config := mapValue(payload["config"])
	if len(config) == 0 {
		config = mapValue(payload["config_snapshot"])
	}
	if len(config) == 0 {
		config = payload
	}
	cloudflare := mapValue(config["cloudflare"])
	if len(cloudflare) == 0 {
		cloudflare = config
	}

	cfg := Config{
		APIToken:   strings.TrimSpace(stringValue(firstNonNil(cloudflare["api_token"], cloudflare["apiToken"]), "")),
		Comment:    strings.TrimSpace(stringValue(cloudflare["comment"], "")),
		Proxied:    boolValue(cloudflare["proxied"], false),
		RecordName: strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_name"], cloudflare["recordName"]), "")),
		RecordType: strings.ToUpper(strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_type"], cloudflare["recordType"]), RecordTypeA))),
		TTL:        DefaultTTL,
		ZoneID:     strings.TrimSpace(stringValue(firstNonNil(cloudflare["zone_id"], cloudflare["zoneId"]), "")),
	}

	warnings := make([]string, 0)
	if cfg.RecordType != RecordTypeAAAA {
		cfg.RecordType = RecordTypeA
	}
	if rawTTL := cloudflare["ttl"]; rawTTL != nil {
		cfg.TTL = intValue(rawTTL, 0)
		if !IsAllowedTTL(cfg.TTL) {
			cfg.TTL = DefaultTTL
			warnings = append(warnings, "Cloudflare TTL 仅支持 60、300、600 秒，已改为 300 秒。")
		}
	}

	if cfg.APIToken == "" || IsMaskedSecret(cfg.APIToken) {
		return cfg, warnings, errors.New("缺少完整 Cloudflare API Token")
	}
	if cfg.ZoneID == "" {
		return cfg, warnings, errors.New("缺少 Cloudflare Zone ID")
	}
	if requireRecordName && cfg.RecordName == "" {
		return cfg, warnings, errors.New("缺少 Cloudflare DNS 记录名称")
	}
	return cfg, warnings, nil
}

func IsAllowedTTL(ttl int) bool {
	return ttl == 60 || ttl == 300 || ttl == 600
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
		httpClient = httpclient.NewClient(httpclient.Options{
			Profile: httpcfg.Resolve("", "", "", "", true),
			Timeout: 30 * time.Second,
		})
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		token:      options.Token,
	}
}

func (c *Client) ListRecords(ctx context.Context, cfg Config) ([]Record, error) {
	records := make([]Record, 0)
	for _, recordType := range []string{RecordTypeA, RecordTypeAAAA} {
		items, err := c.ListRecordsByType(ctx, cfg, recordType)
		if err != nil {
			return nil, err
		}
		records = append(records, items...)
	}
	return records, nil
}

func (c *Client) ListRecordsWithOptions(ctx context.Context, cfg Config, options ListOptions) ([]Record, error) {
	recordType := normalizeListRecordType(options.Type)
	if recordType != "" {
		return c.ListRecordsByNameAndType(ctx, cfg, strings.TrimSpace(options.Name), recordType)
	}
	return c.listRecords(ctx, cfg, strings.TrimSpace(options.Name), "")
}

func (c *Client) ListRecordsByType(ctx context.Context, cfg Config, recordType string) ([]Record, error) {
	return c.ListRecordsByNameAndType(ctx, cfg, cfg.RecordName, recordType)
}

func (c *Client) ListRecordsByNameAndType(ctx context.Context, cfg Config, name string, recordType string) ([]Record, error) {
	return c.listRecords(ctx, cfg, strings.TrimSpace(name), normalizeListRecordType(recordType))
}

func (c *Client) listRecords(ctx context.Context, cfg Config, name string, recordType string) ([]Record, error) {
	records := make([]Record, 0)
	for pageNum := 1; ; pageNum++ {
		endpoint, err := c.endpoint("/zones/" + url.PathEscape(cfg.ZoneID) + "/dns_records")
		if err != nil {
			return nil, err
		}
		query := endpoint.Query()
		if name != "" {
			query.Set("name", name)
		}
		if recordType != "" {
			query.Set("type", recordType)
		}
		query.Set("per_page", "100")
		query.Set("page", fmt.Sprint(pageNum))
		endpoint.RawQuery = query.Encode()

		var response listResponse
		if err := c.do(ctx, http.MethodGet, endpoint.String(), nil, &response); err != nil {
			return nil, err
		}
		records = append(records, response.Result...)
		if response.ResultInfo.TotalPages <= 0 || pageNum >= response.ResultInfo.TotalPages {
			break
		}
	}
	return records, nil
}

func normalizeListRecordType(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case RecordTypeA:
		return RecordTypeA
	case RecordTypeAAAA:
		return RecordTypeAAAA
	default:
		return ""
	}
}

func (c *Client) CreateRecord(ctx context.Context, cfg Config, record Record) (Record, error) {
	var response recordResponse
	endpoint, err := c.endpoint("/zones/" + url.PathEscape(cfg.ZoneID) + "/dns_records")
	if err != nil {
		return Record{}, err
	}
	if err := c.do(ctx, http.MethodPost, endpoint.String(), record, &response); err != nil {
		return Record{}, err
	}
	return response.Result, nil
}

func (c *Client) UpdateRecord(ctx context.Context, cfg Config, recordID string, record Record) (Record, error) {
	var response recordResponse
	endpoint, err := c.endpoint("/zones/" + url.PathEscape(cfg.ZoneID) + "/dns_records/" + url.PathEscape(recordID))
	if err != nil {
		return Record{}, err
	}
	if err := c.do(ctx, http.MethodPatch, endpoint.String(), record, &response); err != nil {
		return Record{}, err
	}
	return response.Result, nil
}

func (c *Client) DeleteRecord(ctx context.Context, cfg Config, recordID string) error {
	endpoint, err := c.endpoint("/zones/" + url.PathEscape(cfg.ZoneID) + "/dns_records/" + url.PathEscape(recordID))
	if err != nil {
		return err
	}
	var response recordResponse
	return c.do(ctx, http.MethodDelete, endpoint.String(), nil, &response)
}

func PushRecords(ctx context.Context, client *Client, cfg Config, ipsRaw string) (PushResult, error) {
	if client == nil {
		client = NewClient(cfg.APIToken)
	}
	ipGroups, ignored := NormalizePushIPs(ipsRaw)
	result := PushResult{
		HasInputIPs:    ipGroups.HasIPs(),
		IgnoredEntries: ignored,
		RecordsAfter:   []Record{},
		Summary:        PushSummary{Ignored: len(ignored)},
		Warnings:       []string{},
	}
	if !result.HasInputIPs {
		return result, nil
	}

	cnameDeleted := false
	deleteConflictingCNAMEs := func() error {
		if cnameDeleted {
			return nil
		}
		cnameDeleted = true
		records, err := client.ListRecordsWithOptions(ctx, cfg, ListOptions{Name: cfg.RecordName})
		if err != nil {
			return &OperationError{Operation: OperationList, Err: err}
		}
		for _, record := range records {
			if strings.EqualFold(strings.TrimSpace(record.Type), RecordTypeCNAME) {
				if err := client.DeleteRecord(ctx, cfg, record.ID); err != nil {
					return &OperationError{Operation: OperationDelete, Err: err}
				}
				result.Summary.Deleted++
			}
		}
		return nil
	}

	for _, recordType := range []string{RecordTypeA, RecordTypeAAAA} {
		ips := ipGroups.ForType(recordType)
		if len(ips) == 0 {
			continue
		}
		existing, err := client.ListRecordsByType(ctx, cfg, recordType)
		if err != nil {
			return result, &OperationError{Operation: OperationList, Err: err}
		}
		for index, ip := range ips {
			record := Record{
				Comment: cfg.Comment,
				Content: ip,
				Name:    cfg.RecordName,
				Proxied: cfg.Proxied,
				TTL:     cfg.TTL,
				Type:    recordType,
			}
			if index < len(existing) {
				if _, err := client.UpdateRecord(ctx, cfg, existing[index].ID, record); err != nil {
					return result, &OperationError{Operation: OperationUpdate, Err: err}
				}
				result.Summary.Updated++
				continue
			}
			if err := deleteConflictingCNAMEs(); err != nil {
				return result, err
			}
			if _, err := client.CreateRecord(ctx, cfg, record); err != nil {
				return result, &OperationError{Operation: OperationCreate, Err: err}
			}
			result.Summary.Created++
		}
		if len(existing) > len(ips) {
			for _, extra := range existing[len(ips):] {
				if err := client.DeleteRecord(ctx, cfg, extra.ID); err != nil {
					return result, &OperationError{Operation: OperationDelete, Err: err}
				}
				result.Summary.Deleted++
			}
		}
	}

	recordsAfter, err := client.ListRecords(ctx, cfg)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("推送完成但刷新 DNS 记录失败：%v", err))
		result.RecordsAfter = []Record{}
	} else {
		result.RecordsAfter = recordsAfter
	}
	return result, nil
}

func SummaryMap(summary PushSummary) map[string]any {
	return map[string]any{
		"created": summary.Created,
		"deleted": summary.Deleted,
		"ignored": summary.Ignored,
		"updated": summary.Updated,
	}
}

func NormalizePushIPs(raw string) (PushIPGroups, []string) {
	tokens := strings.FieldsFunc(strings.ReplaceAll(raw, "\r\n", "\n"), func(r rune) bool {
		return r == ',' || r == ';' || r == '\t' || r == ' ' || r == '\n'
	})
	seen := map[string]struct{}{}
	groups := PushIPGroups{
		A:    make([]string, 0),
		AAAA: make([]string, 0),
	}
	ignored := make([]string, 0)

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		ip := net.ParseIP(token)
		if ip == nil {
			ignored = append(ignored, token)
			continue
		}
		isV4 := ip.To4() != nil
		normalized := ip.String()
		if _, exists := seen[normalized]; exists {
			ignored = append(ignored, token)
			continue
		}
		seen[normalized] = struct{}{}
		if isV4 {
			groups.A = append(groups.A, normalized)
		} else {
			groups.AAAA = append(groups.AAAA, normalized)
		}
	}
	return groups, ignored
}

func (groups PushIPGroups) HasIPs() bool {
	return len(groups.A) > 0 || len(groups.AAAA) > 0
}

func (groups PushIPGroups) ForType(recordType string) []string {
	if recordType == RecordTypeAAAA {
		return groups.AAAA
	}
	return groups.A
}

func IsMaskedSecret(value string) bool {
	value = strings.TrimSpace(value)
	return strings.Contains(value, "...") || strings.Contains(value, "***") || strings.Trim(value, "*") == ""
}

func OperationFromError(err error) string {
	var opErr *OperationError
	if errors.As(err, &opErr) {
		return opErr.Operation
	}
	return ""
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
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare API 返回状态 %s：%s", res.Status, strings.TrimSpace(string(raw)))
	}
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	return responseError(target)
}

func responseError(response any) error {
	var success bool
	var apiErrors []apiError
	switch typed := response.(type) {
	case *listResponse:
		success = typed.Success
		apiErrors = typed.Errors
	case *recordResponse:
		success = typed.Success
		apiErrors = typed.Errors
	default:
		return nil
	}
	if success {
		return nil
	}
	if len(apiErrors) == 0 {
		return errors.New("Cloudflare API 返回失败")
	}
	parts := make([]string, 0, len(apiErrors))
	for _, item := range apiErrors {
		if item.Message != "" {
			parts = append(parts, item.Message)
		}
	}
	if len(parts) == 0 {
		return errors.New("Cloudflare API 返回失败")
	}
	return errors.New(strings.Join(parts, "；"))
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
