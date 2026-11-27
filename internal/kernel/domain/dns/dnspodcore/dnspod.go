// Package dnspodcore implements the DNSPod (Tencent Cloud) record upload used by
// the multi-platform DNS push pipeline. Requests use the Tencent Cloud API 3.0
// signature (TC3-HMAC-SHA256) against dnspod.tencentcloudapi.com.
package dnspodcore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspush"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
)

var APIBaseURL = "https://dnspod.tencentcloudapi.com/"

const (
	RecordTypeA    = "A"
	RecordTypeAAAA = "AAAA"
	DefaultTTL     = 600
	DefaultLine    = "默认"

	apiVersion  = "2021-03-23"
	service     = "dnspod"
	algorithm   = "TC3-HMAC-SHA256"
	dateLayout  = "2006-01-02"
	contentTyp  = "application/json; charset=utf-8"
	maxPageSize = 100
)

type Config struct {
	AllowCNAME bool
	RecordName string
	RecordType string
	SecretID   string
	SecretKey  string
	TTL        int
}

type Zone struct {
	Name string
}

type ClientOptions struct {
	SecretID   string
	SecretKey  string
	BaseURL    string
	HTTPClient *http.Client
	Now        func() time.Time
}

type Client struct {
	secretID   string
	secretKey  string
	baseURL    string
	host       string
	httpClient *http.Client
	now        func() time.Time
}

type apiError struct {
	Code      string
	Message   string
	RequestID string
}

func (e *apiError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "DNSPod API 返回失败"
	}
	if code := strings.TrimSpace(e.Code); code != "" {
		return message + " (" + code + ")"
	}
	return message
}

type envelope struct {
	Response json.RawMessage `json:"Response"`
}

type responseFailure struct {
	Error *struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	} `json:"Error"`
	RequestID string `json:"RequestId"`
}

type domainListResponse struct {
	DomainCountInfo struct {
		AllTotal int `json:"AllTotal"`
	} `json:"DomainCountInfo"`
	DomainList []struct {
		Domain string `json:"Domain"`
	} `json:"DomainList"`
}

type recordListResponse struct {
	RecordCountInfo struct {
		TotalCount int `json:"TotalCount"`
	} `json:"RecordCountInfo"`
	RecordList []apiRecord `json:"RecordList"`
}

type recordIDResponse struct {
	RecordID int64 `json:"RecordId"`
}

type apiRecord struct {
	RecordID int64  `json:"RecordId"`
	Name     string `json:"Name"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	TTL      int    `json:"TTL"`
	Line     string `json:"Line"`
	Status   string `json:"Status"`
}

func NewClient(secretID, secretKey string) *Client {
	return NewClientWithOptions(ClientOptions{SecretID: secretID, SecretKey: secretKey})
}

func NewClientWithOptions(options ClientOptions) *Client {
	baseURL := strings.TrimSpace(options.BaseURL)
	if baseURL == "" {
		baseURL = APIBaseURL
	}
	parsed, err := url.Parse(baseURL)
	host := ""
	if err == nil {
		host = parsed.Host
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
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Client{
		secretID:   strings.TrimSpace(options.SecretID),
		secretKey:  strings.TrimSpace(options.SecretKey),
		baseURL:    strings.TrimRight(baseURL, "/") + "/",
		host:       host,
		httpClient: httpClient,
		now:        now,
	}
}

// ListZones reads every domain in the account.
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	zones := make([]Zone, 0)
	for offset := 0; offset < 10000; offset += maxPageSize {
		var response domainListResponse
		err := c.call(ctx, "DescribeDomainList", map[string]any{
			"Offset": offset,
			"Limit":  maxPageSize,
		}, &response)
		if err != nil {
			return nil, err
		}
		for _, item := range response.DomainList {
			if name := normalizeDomain(item.Domain); name != "" {
				zones = append(zones, Zone{Name: name})
			}
		}
		if len(response.DomainList) == 0 || len(zones) >= response.DomainCountInfo.AllTotal {
			break
		}
	}
	return zones, nil
}

// ResolveZone finds the DNSPod domain that owns fullName.
func (c *Client) ResolveZone(ctx context.Context, fullName string) (Zone, error) {
	fullName = normalizeDomain(fullName)
	if fullName == "" {
		return Zone{}, errors.New("缺少 DNSPod 域名")
	}
	zones, err := c.ListZones(ctx)
	if err != nil {
		return Zone{}, err
	}
	names := make([]string, 0, len(zones))
	for _, zone := range zones {
		names = append(names, zone.Name)
	}
	name, ok := dnspush.MatchZone(names, fullName)
	if !ok {
		return Zone{}, fmt.Errorf("DNSPod 账号下找不到与 %s 匹配的域名，请先在 DNSPod 控制台添加该域名", fullName)
	}
	return Zone{Name: name}, nil
}

func (c *Client) ListRecords(ctx context.Context, zone Zone) ([]dnspush.Record, error) {
	zone.Name = normalizeDomain(zone.Name)
	records := make([]dnspush.Record, 0)
	for offset := 0; offset < 10000; offset += maxPageSize {
		var response recordListResponse
		err := c.call(ctx, "DescribeRecordList", map[string]any{
			"Domain": zone.Name,
			"Limit":  maxPageSize,
			"Offset": offset,
		}, &response)
		if err != nil {
			return nil, err
		}
		for _, item := range response.RecordList {
			records = append(records, fromAPIRecord(zone, item))
		}
		if len(response.RecordList) == 0 || len(records) >= response.RecordCountInfo.TotalCount {
			break
		}
	}
	return records, nil
}

func (c *Client) CreateRecord(ctx context.Context, zone Zone, record dnspush.Record) (dnspush.Record, error) {
	payload := map[string]any{
		"Domain":     zone.Name,
		"RecordLine": DefaultLine,
		"RecordType": strings.ToUpper(strings.TrimSpace(record.Type)),
		"SubDomain":  relativeName(zone.Name, record.Name),
		"TTL":        normalizedTTL(record.TTL),
		"Value":      strings.TrimSpace(record.Content),
	}
	var response recordIDResponse
	if err := c.call(ctx, "CreateRecord", payload, &response); err != nil {
		return dnspush.Record{}, err
	}
	created := record
	created.ID = strconv.FormatInt(response.RecordID, 10)
	created.TTL = normalizedTTL(record.TTL)
	return created, nil
}

func (c *Client) UpdateRecord(ctx context.Context, zone Zone, record dnspush.Record) (dnspush.Record, error) {
	recordID, err := recordIDNumber(record.ID)
	if err != nil {
		return dnspush.Record{}, err
	}
	payload := map[string]any{
		"Domain":     zone.Name,
		"RecordId":   recordID,
		"RecordLine": DefaultLine,
		"RecordType": strings.ToUpper(strings.TrimSpace(record.Type)),
		"SubDomain":  relativeName(zone.Name, record.Name),
		"TTL":        normalizedTTL(record.TTL),
		"Value":      strings.TrimSpace(record.Content),
	}
	var response recordIDResponse
	if err := c.call(ctx, "ModifyRecord", payload, &response); err != nil {
		return dnspush.Record{}, err
	}
	updated := record
	updated.TTL = normalizedTTL(record.TTL)
	return updated, nil
}

func (c *Client) DeleteRecord(ctx context.Context, zone Zone, record dnspush.Record) error {
	recordID, err := recordIDNumber(record.ID)
	if err != nil {
		return err
	}
	return c.call(ctx, "DeleteRecord", map[string]any{
		"Domain":   zone.Name,
		"RecordId": recordID,
	}, nil)
}

// Push synchronizes the managed record name with ipsRaw through the shared
// provider-agnostic reconcile loop. It manages A and AAAA from the given pool;
// cfg.RecordType is not consulted, so callers must filter the address family.
func Push(ctx context.Context, client *Client, cfg Config, ipsRaw string) (dnspush.PushResult, error) {
	if client == nil {
		client = NewClient(cfg.SecretID, cfg.SecretKey)
	}
	desired, ipGroups, ignored := dnspush.DesiredRecords(cfg.RecordName, normalizedTTL(cfg.TTL), ipsRaw)
	result := dnspush.PushResult{
		HasInputIPs:    ipGroups.HasIPs(),
		IgnoredEntries: ignored,
		RecordsAfter:   []dnspush.Record{},
		Summary:        dnspush.Summary{Ignored: len(ignored)},
		Warnings:       []string{},
	}
	if !result.HasInputIPs {
		return result, nil
	}

	zone, err := client.ResolveZone(ctx, cfg.RecordName)
	if err != nil {
		return result, &dnspush.OperationError{Operation: dnspush.OperationList, Err: err}
	}
	backend := &pushBackend{client: client, cfg: cfg, zone: zone}
	reconciled, err := dnspush.ReconcileWithOptions(ctx, backend, desired, dnspush.ReconcileOptions{AllowCNAME: cfg.AllowCNAME})
	result.Summary.Created = reconciled.Summary.Created
	result.Summary.Deleted = reconciled.Summary.Deleted
	result.Summary.Updated = reconciled.Summary.Updated
	result.Warnings = append(result.Warnings, reconciled.Warnings...)
	if err != nil {
		return result, err
	}
	recordsAfter, listErr := client.ListRecords(ctx, zone)
	if listErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("推送完成但刷新 DNS 记录失败：%v", listErr))
	} else {
		result.RecordsAfter = recordsAfter
	}
	return result, nil
}

type pushBackend struct {
	client *Client
	cfg    Config
	zone   Zone
}

func (b *pushBackend) ListRecords(ctx context.Context) ([]dnspush.Record, error) {
	records, err := b.client.ListRecords(ctx, b.zone)
	if err != nil {
		return nil, err
	}
	name := normalizeDomain(b.cfg.RecordName)
	converted := make([]dnspush.Record, 0, len(records))
	for _, record := range records {
		if normalizeDomain(record.Name) != name {
			continue
		}
		converted = append(converted, record)
	}
	return converted, nil
}

func (b *pushBackend) CreateRecord(ctx context.Context, record dnspush.Record) (dnspush.Record, error) {
	created, err := b.client.CreateRecord(ctx, b.zone, record)
	if err != nil {
		return record, err
	}
	created.Name = record.Name
	return created, nil
}

func (b *pushBackend) UpdateRecord(ctx context.Context, record dnspush.Record) (dnspush.Record, error) {
	updated, err := b.client.UpdateRecord(ctx, b.zone, record)
	if err != nil {
		return record, err
	}
	updated.Name = record.Name
	return updated, nil
}

func (b *pushBackend) DeleteRecord(ctx context.Context, record dnspush.Record) error {
	return b.client.DeleteRecord(ctx, b.zone, record)
}

func (b *pushBackend) EquivalentRecord(existing, desired dnspush.Record) bool {
	return strings.EqualFold(strings.TrimSpace(existing.Type), strings.TrimSpace(desired.Type)) &&
		normalizeDomain(existing.Name) == normalizeDomain(desired.Name) &&
		strings.TrimSpace(existing.Content) == strings.TrimSpace(desired.Content) &&
		existing.TTL == desired.TTL
}

func fromAPIRecord(zone Zone, item apiRecord) dnspush.Record {
	return dnspush.Record{
		Attrs:   map[string]string{"line": strings.TrimSpace(item.Line), "status": strings.TrimSpace(item.Status)},
		Content: strings.TrimSpace(item.Value),
		ID:      strconv.FormatInt(item.RecordID, 10),
		Name:    dnspush.AbsoluteName(zone.Name, item.Name),
		TTL:     item.TTL,
		Type:    strings.ToUpper(strings.TrimSpace(item.Type)),
	}
}

func recordIDNumber(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("缺少 DNSPod 记录 ID")
	}
	return id, nil
}

func relativeName(zone, name string) string {
	return dnspush.RelativeName(zone, name)
}

func normalizedTTL(ttl int) int {
	if ttl <= 0 {
		return DefaultTTL
	}
	return ttl
}

func normalizeDomain(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

// Authorization builds the TC3-HMAC-SHA256 Authorization header for one request.
// It is exported so tests can pin the canonical request and signature shape.
func Authorization(secretID, secretKey, host, payload string, timestamp int64) string {
	date := time.Unix(timestamp, 0).UTC().Format(dateLayout)
	canonicalRequest := CanonicalRequest(host, payload)
	stringToSign := strings.Join([]string{
		algorithm,
		strconv.FormatInt(timestamp, 10),
		date + "/" + service + "/tc3_request",
		sha256Hex(canonicalRequest),
	}, "\n")
	secretDate := hmacSHA256([]byte("TC3"+secretKey), date)
	secretService := hmacSHA256(secretDate, service)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	credential := strings.Join([]string{secretID, date, service, "tc3_request"}, "/")
	return algorithm + " Credential=" + credential + ", SignedHeaders=content-type;host, Signature=" + signature
}

// CanonicalRequest returns the canonical request string for one payload, which
// documents the exact shape signed by Authorization.
func CanonicalRequest(host, payload string) string {
	return strings.Join([]string{
		http.MethodPost,
		"/",
		"",
		"content-type:" + contentTyp,
		"host:" + host,
		"",
		"content-type;host",
		sha256Hex(payload),
	}, "\n")
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

func (c *Client) call(ctx context.Context, action string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	timestamp := c.now().UTC().Unix()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", Authorization(c.secretID, c.secretKey, c.host, string(body), timestamp))
	request.Header.Set("Content-Type", contentTyp)
	request.Header.Set("Host", c.host)
	request.Header.Set("X-TC-Action", action)
	request.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	request.Header.Set("X-TC-Version", apiVersion)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("DNSPod API 返回状态 %s：%w", response.Status, parseFailure(raw))
	}
	if len(raw) == 0 {
		return nil
	}
	parsed := envelope{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	failure := responseFailure{}
	if err := json.Unmarshal(parsed.Response, &failure); err != nil {
		return err
	}
	if failure.Error != nil && strings.TrimSpace(failure.Error.Code) != "" {
		return &apiError{Code: failure.Error.Code, Message: failure.Error.Message, RequestID: failure.RequestID}
	}
	if target == nil {
		return nil
	}
	return json.Unmarshal(parsed.Response, target)
}

func parseFailure(raw []byte) error {
	parsed := envelope{}
	failure := responseFailure{}
	if err := json.Unmarshal(raw, &parsed); err == nil {
		if err := json.Unmarshal(parsed.Response, &failure); err == nil && failure.Error != nil && strings.TrimSpace(failure.Error.Code) != "" {
			return &apiError{Code: failure.Error.Code, Message: failure.Error.Message, RequestID: failure.RequestID}
		}
	}
	return errors.New(strings.TrimSpace(string(raw)))
}

// ErrorCode reports the DNSPod error code carried by err, if any.
func ErrorCode(err error) string {
	var failure *apiError
	if errors.As(err, &failure) {
		return strings.TrimSpace(failure.Code)
	}
	return ""
}
