package cloudflarecore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	configvalue "github.com/axuitomo/CFST-GUI/internal/config"
	"github.com/axuitomo/CFST-GUI/internal/dnspush"
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

type PushSummary = dnspush.Summary

type PushIPGroups = dnspush.IPGroups

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

type OperationError = dnspush.OperationError

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
	config := configvalue.Map(payload["config"])
	if len(config) == 0 {
		config = configvalue.Map(payload["config_snapshot"])
	}
	if len(config) == 0 {
		config = payload
	}
	cloudflare := configvalue.Map(config["cloudflare"])
	if len(cloudflare) == 0 {
		cloudflare = config
	}

	cfg := Config{
		APIToken:   strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(cloudflare["api_token"], cloudflare["apiToken"]), "")),
		Comment:    strings.TrimSpace(configvalue.String(cloudflare["comment"], "")),
		Proxied:    configvalue.Bool(cloudflare["proxied"], false),
		RecordName: strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(cloudflare["record_name"], cloudflare["recordName"]), "")),
		RecordType: strings.ToUpper(strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(cloudflare["record_type"], cloudflare["recordType"]), RecordTypeA))),
		TTL:        DefaultTTL,
		ZoneID:     strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(cloudflare["zone_id"], cloudflare["zoneId"]), "")),
	}

	warnings := make([]string, 0)
	if cfg.RecordType != RecordTypeAAAA {
		cfg.RecordType = RecordTypeA
	}
	if rawTTL := cloudflare["ttl"]; rawTTL != nil {
		cfg.TTL = configvalue.Int(rawTTL, 0)
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
			DisableProxy: true,
			Profile:      httpcfg.Resolve("", "", "", "", false),
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

	reconciled, err := dnspush.Reconcile(ctx, pushBackend{client: client, cfg: cfg}, desiredRecords(cfg, ipGroups))
	result.Summary.Created = reconciled.Summary.Created
	result.Summary.Deleted = reconciled.Summary.Deleted
	result.Summary.Updated = reconciled.Summary.Updated
	result.Warnings = append(result.Warnings, reconciled.Warnings...)
	if err != nil {
		return result, err
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

type pushBackend struct {
	client *Client
	cfg    Config
}

func (b pushBackend) ListRecords(ctx context.Context) ([]dnspush.Record, error) {
	records, err := b.client.ListRecordsWithOptions(ctx, b.cfg, ListOptions{Name: b.cfg.RecordName})
	if err != nil {
		return nil, err
	}
	converted := make([]dnspush.Record, 0, len(records))
	for _, record := range records {
		converted = append(converted, toPushRecord(record))
	}
	return converted, nil
}

func (b pushBackend) CreateRecord(ctx context.Context, record dnspush.Record) (dnspush.Record, error) {
	created, err := b.client.CreateRecord(ctx, b.cfg, fromPushRecord(record))
	if err != nil {
		return record, err
	}
	return toPushRecord(created), nil
}

func (b pushBackend) UpdateRecord(ctx context.Context, record dnspush.Record) (dnspush.Record, error) {
	updated, err := b.client.UpdateRecord(ctx, b.cfg, record.ID, fromPushRecord(record))
	if err != nil {
		return record, err
	}
	return toPushRecord(updated), nil
}

func (b pushBackend) DeleteRecord(ctx context.Context, record dnspush.Record) error {
	return b.client.DeleteRecord(ctx, b.cfg, record.ID)
}

func (b pushBackend) EquivalentRecord(existing, desired dnspush.Record) bool {
	return sameManagedRecord(fromPushRecord(existing), fromPushRecord(desired))
}

func toPushRecord(record Record) dnspush.Record {
	return dnspush.Record{
		Attrs:   map[string]string{"comment": record.Comment, "proxied": boolAttr(record.Proxied)},
		Content: record.Content,
		ID:      record.ID,
		Name:    record.Name,
		TTL:     record.TTL,
		Type:    record.Type,
	}
}

func fromPushRecord(record dnspush.Record) Record {
	return Record{
		Comment: record.Attrs["comment"],
		Content: record.Content,
		ID:      record.ID,
		Name:    record.Name,
		Proxied: record.Attrs["proxied"] == "true",
		TTL:     record.TTL,
		Type:    record.Type,
	}
}

func boolAttr(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func desiredRecords(cfg Config, groups PushIPGroups) []dnspush.Record {
	records := make([]dnspush.Record, 0, len(groups.A)+len(groups.AAAA))
	for _, recordType := range []string{RecordTypeA, RecordTypeAAAA} {
		for _, content := range groups.ForType(recordType) {
			records = append(records, toPushRecord(desiredRecord(cfg, recordType, content)))
		}
	}
	return records
}

func desiredRecord(cfg Config, recordType, content string) Record {
	return Record{Comment: cfg.Comment, Content: content, Name: cfg.RecordName, Proxied: cfg.Proxied, TTL: cfg.TTL, Type: recordType}
}

func sameManagedRecord(existing, desired Record) bool {
	return existing.Comment == desired.Comment && existing.Content == desired.Content && existing.Name == desired.Name && existing.Proxied == desired.Proxied && existing.TTL == desired.TTL && strings.EqualFold(existing.Type, desired.Type)
}

func SummaryMap(summary PushSummary) map[string]any {
	return dnspush.SummaryMap(summary)
}

func NormalizePushIPs(raw string) (PushIPGroups, []string) {
	return dnspush.NormalizeIPs(raw)
}
func IsMaskedSecret(value string) bool {
	value = strings.TrimSpace(value)
	return strings.Contains(value, "...") || strings.Contains(value, "***") || strings.Trim(value, "*") == ""
}

func OperationFromError(err error) string {
	return dnspush.OperationFromError(err)
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
