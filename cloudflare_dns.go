package main

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
)

var cloudflareAPIBaseURL = "https://api.cloudflare.com/client/v4"

const (
	cloudflareRecordTypeA    = "A"
	cloudflareRecordTypeAAAA = "AAAA"
	defaultCloudflareTTL     = 300
)

type cloudflareDNSConfig struct {
	APIToken   string
	Comment    string
	Proxied    bool
	RecordName string
	RecordType string
	TTL        int
	ZoneID     string
}

type CloudflareDNSRecord struct {
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

type cloudflareDNSPushSummary struct {
	Created int `json:"created"`
	Deleted int `json:"deleted"`
	Ignored int `json:"ignored"`
	Updated int `json:"updated"`
}

type cloudflareListResponse struct {
	Success    bool                  `json:"success"`
	Errors     []cloudflareAPIError  `json:"errors"`
	Messages   []cloudflareAPIError  `json:"messages"`
	Result     []CloudflareDNSRecord `json:"result"`
	ResultInfo struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

type cloudflareRecordResponse struct {
	Success  bool                 `json:"success"`
	Errors   []cloudflareAPIError `json:"errors"`
	Messages []cloudflareAPIError `json:"messages"`
	Result   CloudflareDNSRecord  `json:"result"`
}

type cloudflareAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareDNSClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

type cloudflareDNSPushIPGroups struct {
	A    []string
	AAAA []string
}

func (a *App) ListCloudflareDNSRecords(payload map[string]any) DesktopCommandResult {
	cfg, warnings, err := cloudflareDNSConfigFromPayload(payload)
	if err != nil {
		return desktopCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := newCloudflareDNSClient(cfg.APIToken)
	records, err := client.listRecords(ctx, cfg)
	if err != nil {
		return desktopCommandResult("DNS_LIST_FAILED", nil, err.Error(), false, nil, warnings)
	}

	return desktopCommandResult("DNS_RECORDS_LISTED", map[string]any{
		"count":   len(records),
		"records": records,
	}, fmt.Sprintf("已读取 Cloudflare 中匹配 %s 的 A/AAAA 记录 %d 条。", cfg.RecordName, len(records)), true, nil, warnings)
}

func (a *App) PushCloudflareDNSRecords(payload map[string]any) DesktopCommandResult {
	cfg, warnings, err := cloudflareDNSConfigFromPayload(payload)
	if err != nil {
		return desktopCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, warnings)
	}

	ipsRaw := stringValue(firstNonNil(payload["ipsRaw"], payload["ips_raw"]), "")
	ipGroups, ignored := normalizeDNSPushIPs(ipsRaw)
	if !ipGroups.hasIPs() {
		return desktopCommandResult("DNS_INPUT_EMPTY", map[string]any{
			"ignored_entries": ignored,
			"records_after":   []CloudflareDNSRecord{},
			"summary":         cloudflareSummaryMap(cloudflareDNSPushSummary{Ignored: len(ignored)}),
		}, "没有可推送的有效 IP。", false, nil, warnings)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client := newCloudflareDNSClient(cfg.APIToken)
	summary := cloudflareDNSPushSummary{Ignored: len(ignored)}
	for _, recordType := range []string{cloudflareRecordTypeA, cloudflareRecordTypeAAAA} {
		ips := ipGroups.forType(recordType)
		if len(ips) == 0 {
			continue
		}
		existing, err := client.listRecordsByType(ctx, cfg, recordType)
		if err != nil {
			return desktopCommandResult("DNS_LIST_FAILED", nil, err.Error(), false, nil, warnings)
		}
		for index, ip := range ips {
			record := CloudflareDNSRecord{
				Comment: cfg.Comment,
				Content: ip,
				Name:    cfg.RecordName,
				Proxied: cfg.Proxied,
				TTL:     cfg.TTL,
				Type:    recordType,
			}
			if index < len(existing) {
				if _, err := client.updateRecord(ctx, cfg, existing[index].ID, record); err != nil {
					return desktopCommandResult("DNS_UPDATE_FAILED", nil, err.Error(), false, nil, warnings)
				}
				summary.Updated++
				continue
			}
			if _, err := client.createRecord(ctx, cfg, record); err != nil {
				return desktopCommandResult("DNS_CREATE_FAILED", nil, err.Error(), false, nil, warnings)
			}
			summary.Created++
		}
		if len(existing) > len(ips) {
			for _, extra := range existing[len(ips):] {
				if err := client.deleteRecord(ctx, cfg, extra.ID); err != nil {
					return desktopCommandResult("DNS_DELETE_FAILED", nil, err.Error(), false, nil, warnings)
				}
				summary.Deleted++
			}
		}
	}

	recordsAfter, err := client.listRecords(ctx, cfg)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("推送完成但刷新 DNS 记录失败：%v", err))
		recordsAfter = []CloudflareDNSRecord{}
	}

	return desktopCommandResult("DNS_PUSH_COMPLETED", map[string]any{
		"ignored_entries": ignored,
		"records_after":   recordsAfter,
		"summary":         cloudflareSummaryMap(summary),
	}, fmt.Sprintf("Cloudflare DNS 覆盖推送完成：创建 %d、更新 %d、删除 %d、忽略 %d。", summary.Created, summary.Updated, summary.Deleted, summary.Ignored), true, nil, dedupeStrings(warnings))
}

func cloudflareSummaryMap(summary cloudflareDNSPushSummary) map[string]any {
	return map[string]any{
		"created": summary.Created,
		"deleted": summary.Deleted,
		"ignored": summary.Ignored,
		"updated": summary.Updated,
	}
}

func cloudflareDNSConfigFromPayload(payload map[string]any) (cloudflareDNSConfig, []string, error) {
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

	cfg := cloudflareDNSConfig{
		APIToken:   strings.TrimSpace(stringValue(firstNonNil(cloudflare["api_token"], cloudflare["apiToken"]), "")),
		Comment:    strings.TrimSpace(stringValue(cloudflare["comment"], "")),
		Proxied:    boolValue(cloudflare["proxied"], false),
		RecordName: strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_name"], cloudflare["recordName"]), "")),
		RecordType: strings.ToUpper(strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_type"], cloudflare["recordType"]), cloudflareRecordTypeA))),
		TTL:        defaultCloudflareTTL,
		ZoneID:     strings.TrimSpace(stringValue(firstNonNil(cloudflare["zone_id"], cloudflare["zoneId"]), "")),
	}

	warnings := make([]string, 0)
	if cfg.RecordType != cloudflareRecordTypeAAAA {
		cfg.RecordType = cloudflareRecordTypeA
	}
	if rawTTL := cloudflare["ttl"]; rawTTL != nil {
		cfg.TTL = intValue(rawTTL, 0)
		if !isAllowedCloudflareTTL(cfg.TTL) {
			cfg.TTL = defaultCloudflareTTL
			warnings = append(warnings, "Cloudflare TTL 仅支持 60、300、600 秒，已改为 300 秒。")
		}
	}

	if cfg.APIToken == "" || isMaskedSecret(cfg.APIToken) {
		return cfg, warnings, errors.New("缺少完整 Cloudflare API Token")
	}
	if cfg.ZoneID == "" {
		return cfg, warnings, errors.New("缺少 Cloudflare Zone ID")
	}
	if cfg.RecordName == "" {
		return cfg, warnings, errors.New("缺少 Cloudflare DNS 记录名称")
	}
	return cfg, warnings, nil
}

func isAllowedCloudflareTTL(ttl int) bool {
	return ttl == 60 || ttl == 300 || ttl == 600
}

func newCloudflareDNSClient(token string) *cloudflareDNSClient {
	return &cloudflareDNSClient{
		baseURL: strings.TrimRight(cloudflareAPIBaseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token: token,
	}
}

func (c *cloudflareDNSClient) listRecords(ctx context.Context, cfg cloudflareDNSConfig) ([]CloudflareDNSRecord, error) {
	records := make([]CloudflareDNSRecord, 0)
	for _, recordType := range []string{cloudflareRecordTypeA, cloudflareRecordTypeAAAA} {
		items, err := c.listRecordsByType(ctx, cfg, recordType)
		if err != nil {
			return nil, err
		}
		records = append(records, items...)
	}
	return records, nil
}

func (c *cloudflareDNSClient) listRecordsByType(ctx context.Context, cfg cloudflareDNSConfig, recordType string) ([]CloudflareDNSRecord, error) {
	records := make([]CloudflareDNSRecord, 0)
	for page := 1; ; page++ {
		endpoint, err := c.endpoint("/zones/" + url.PathEscape(cfg.ZoneID) + "/dns_records")
		if err != nil {
			return nil, err
		}
		query := endpoint.Query()
		query.Set("name", cfg.RecordName)
		query.Set("type", recordType)
		query.Set("per_page", "100")
		query.Set("page", fmt.Sprint(page))
		endpoint.RawQuery = query.Encode()

		var response cloudflareListResponse
		if err := c.do(ctx, http.MethodGet, endpoint.String(), nil, &response); err != nil {
			return nil, err
		}
		records = append(records, response.Result...)
		if response.ResultInfo.TotalPages <= 0 || page >= response.ResultInfo.TotalPages {
			break
		}
	}
	return records, nil
}

func (c *cloudflareDNSClient) createRecord(ctx context.Context, cfg cloudflareDNSConfig, record CloudflareDNSRecord) (CloudflareDNSRecord, error) {
	var response cloudflareRecordResponse
	endpoint, err := c.endpoint("/zones/" + url.PathEscape(cfg.ZoneID) + "/dns_records")
	if err != nil {
		return CloudflareDNSRecord{}, err
	}
	if err := c.do(ctx, http.MethodPost, endpoint.String(), record, &response); err != nil {
		return CloudflareDNSRecord{}, err
	}
	return response.Result, nil
}

func (c *cloudflareDNSClient) updateRecord(ctx context.Context, cfg cloudflareDNSConfig, recordID string, record CloudflareDNSRecord) (CloudflareDNSRecord, error) {
	var response cloudflareRecordResponse
	endpoint, err := c.endpoint("/zones/" + url.PathEscape(cfg.ZoneID) + "/dns_records/" + url.PathEscape(recordID))
	if err != nil {
		return CloudflareDNSRecord{}, err
	}
	if err := c.do(ctx, http.MethodPatch, endpoint.String(), record, &response); err != nil {
		return CloudflareDNSRecord{}, err
	}
	return response.Result, nil
}

func (c *cloudflareDNSClient) deleteRecord(ctx context.Context, cfg cloudflareDNSConfig, recordID string) error {
	endpoint, err := c.endpoint("/zones/" + url.PathEscape(cfg.ZoneID) + "/dns_records/" + url.PathEscape(recordID))
	if err != nil {
		return err
	}
	var response cloudflareRecordResponse
	return c.do(ctx, http.MethodDelete, endpoint.String(), nil, &response)
}

func (c *cloudflareDNSClient) endpoint(rawPath string) (*url.URL, error) {
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	parsed.Path = path.Join(parsed.Path, rawPath)
	return parsed, nil
}

func (c *cloudflareDNSClient) do(ctx context.Context, method, endpoint string, body any, target any) error {
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
	if err := cloudflareResponseError(target); err != nil {
		return err
	}
	return nil
}

func cloudflareResponseError(response any) error {
	var success bool
	var apiErrors []cloudflareAPIError
	switch typed := response.(type) {
	case *cloudflareListResponse:
		success = typed.Success
		apiErrors = typed.Errors
	case *cloudflareRecordResponse:
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

func normalizeDNSPushIPs(raw string) (cloudflareDNSPushIPGroups, []string) {
	tokens := strings.FieldsFunc(strings.ReplaceAll(raw, "\r\n", "\n"), func(r rune) bool {
		return r == ',' || r == ';' || r == '\t' || r == ' ' || r == '\n'
	})
	seen := map[string]struct{}{}
	groups := cloudflareDNSPushIPGroups{
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

func (groups cloudflareDNSPushIPGroups) hasIPs() bool {
	return len(groups.A) > 0 || len(groups.AAAA) > 0
}

func (groups cloudflareDNSPushIPGroups) forType(recordType string) []string {
	if recordType == cloudflareRecordTypeAAAA {
		return groups.AAAA
	}
	return groups.A
}

func isMaskedSecret(value string) bool {
	value = strings.TrimSpace(value)
	return strings.Contains(value, "...") || strings.Contains(value, "***") || strings.Trim(value, "*") == ""
}
