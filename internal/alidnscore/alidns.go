// Package alidnscore implements the Alibaba Cloud DNS (Alidns) record upload
// used by the multi-platform DNS push pipeline. Requests are signed with the
// RPC style SignatureVersion 1.0 (HMAC-SHA1) declared by the API.
package alidnscore

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/dnspush"
	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
)

var APIBaseURL = "https://alidns.aliyuncs.com/"

const (
	RecordTypeA    = "A"
	RecordTypeAAAA = "AAAA"
	DefaultTTL     = 600

	apiVersion        = "2015-01-09"
	dateLayout        = "2006-01-02T15:04:05Z"
	maxZonePageSize   = 100
	maxRecordPageSize = 500
	signatureMethod   = "HMAC-SHA1"
	signatureVersion  = "1.0"
)

type Config struct {
	AccessKeyID     string
	AccessKeySecret string
	RecordName      string
	RecordType      string
	TTL             int
}

type Zone struct {
	Name string
}

type ClientOptions struct {
	AccessKeyID     string
	AccessKeySecret string
	BaseURL         string
	HTTPClient      *http.Client
	Now             func() time.Time
}

type Client struct {
	accessKeyID     string
	accessKeySecret string
	baseURL         string
	httpClient      *http.Client
	now             func() time.Time
}

type apiError struct {
	Code      string `json:"Code"`
	Message   string `json:"Message"`
	RequestID string `json:"RequestId"`
}

func (e *apiError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "阿里云 DNS API 返回失败"
	}
	if code := strings.TrimSpace(e.Code); code != "" {
		return message + " (" + code + ")"
	}
	return message
}

type domainsResponse struct {
	RequestID  string `json:"RequestId"`
	TotalCount int    `json:"TotalCount"`
	Domains    struct {
		Domain []struct {
			DomainName string `json:"DomainName"`
		} `json:"Domain"`
	} `json:"Domains"`
}

type domainRecordsResponse struct {
	RequestID     string `json:"RequestId"`
	TotalCount    int    `json:"TotalCount"`
	DomainRecords struct {
		Record []apiRecord `json:"Record"`
	} `json:"DomainRecords"`
}

type recordIDResponse struct {
	RequestID string `json:"RequestId"`
	RecordID  string `json:"RecordId"`
}

type apiRecord struct {
	RecordID   string `json:"RecordId"`
	RR         string `json:"RR"`
	Type       string `json:"Type"`
	Value      string `json:"Value"`
	TTL        int    `json:"TTL"`
	DomainName string `json:"DomainName"`
	Line       string `json:"Line"`
	Status     string `json:"Status"`
}

func NewClient(accessKeyID, accessKeySecret string) *Client {
	return NewClientWithOptions(ClientOptions{AccessKeyID: accessKeyID, AccessKeySecret: accessKeySecret})
}

func NewClientWithOptions(options ClientOptions) *Client {
	baseURL := strings.TrimSpace(options.BaseURL)
	if baseURL == "" {
		baseURL = APIBaseURL
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
		accessKeyID:     strings.TrimSpace(options.AccessKeyID),
		accessKeySecret: strings.TrimSpace(options.AccessKeySecret),
		baseURL:         strings.TrimRight(baseURL, "/"),
		httpClient:      httpClient,
		now:             now,
	}
}

// ListZones reads every domain in the account.
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	zones := make([]Zone, 0)
	for page := 1; page <= 100; page++ {
		var response domainsResponse
		err := c.do(ctx, "DescribeDomains", map[string]string{
			"PageNumber": strconv.Itoa(page),
			"PageSize":   strconv.Itoa(maxZonePageSize),
		}, &response)
		if err != nil {
			return nil, err
		}
		for _, item := range response.Domains.Domain {
			if name := normalizeDomain(item.DomainName); name != "" {
				zones = append(zones, Zone{Name: name})
			}
		}
		if len(response.Domains.Domain) == 0 || len(zones) >= response.TotalCount {
			break
		}
	}
	return zones, nil
}

// ResolveZone finds the cloud zone that owns fullName.
func (c *Client) ResolveZone(ctx context.Context, fullName string) (Zone, error) {
	fullName = normalizeDomain(fullName)
	if fullName == "" {
		return Zone{}, errors.New("缺少阿里云 DNS 域名")
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
		return Zone{}, fmt.Errorf("阿里云账号下找不到与 %s 匹配的域名，请先在云解析 DNS 中添加该域名", fullName)
	}
	return Zone{Name: name}, nil
}

func (c *Client) ListRecords(ctx context.Context, zone Zone) ([]dnspush.Record, error) {
	zone.Name = normalizeDomain(zone.Name)
	records := make([]dnspush.Record, 0)
	for page := 1; page <= 100; page++ {
		var response domainRecordsResponse
		err := c.do(ctx, "DescribeDomainRecords", map[string]string{
			"DomainName": zone.Name,
			"PageNumber": strconv.Itoa(page),
			"PageSize":   strconv.Itoa(maxRecordPageSize),
		}, &response)
		if err != nil {
			return nil, err
		}
		for _, item := range response.DomainRecords.Record {
			records = append(records, fromAPIRecord(zone, item))
		}
		if len(response.DomainRecords.Record) == 0 || len(records) >= response.TotalCount {
			break
		}
	}
	return records, nil
}

func (c *Client) CreateRecord(ctx context.Context, zone Zone, record dnspush.Record) (dnspush.Record, error) {
	var response recordIDResponse
	err := c.do(ctx, "AddDomainRecord", map[string]string{
		"DomainName": zone.Name,
		"RR":         relativeName(zone.Name, record.Name),
		"TTL":        strconv.Itoa(normalizedTTL(record.TTL)),
		"Type":       strings.ToUpper(strings.TrimSpace(record.Type)),
		"Value":      strings.TrimSpace(record.Content),
	}, &response)
	if err != nil {
		return dnspush.Record{}, err
	}
	created := record
	created.ID = strings.TrimSpace(response.RecordID)
	created.TTL = normalizedTTL(record.TTL)
	return created, nil
}

func (c *Client) UpdateRecord(ctx context.Context, zone Zone, record dnspush.Record) (dnspush.Record, error) {
	if strings.TrimSpace(record.ID) == "" {
		return dnspush.Record{}, errors.New("缺少阿里云 DNS 记录 ID")
	}
	var response recordIDResponse
	err := c.do(ctx, "UpdateDomainRecord", map[string]string{
		"RecordId": strings.TrimSpace(record.ID),
		"RR":       relativeName(zone.Name, record.Name),
		"TTL":      strconv.Itoa(normalizedTTL(record.TTL)),
		"Type":     strings.ToUpper(strings.TrimSpace(record.Type)),
		"Value":    strings.TrimSpace(record.Content),
	}, &response)
	if err != nil {
		return dnspush.Record{}, err
	}
	updated := record
	updated.TTL = normalizedTTL(record.TTL)
	return updated, nil
}

func (c *Client) DeleteRecord(ctx context.Context, _ Zone, record dnspush.Record) error {
	if strings.TrimSpace(record.ID) == "" {
		return errors.New("缺少阿里云 DNS 记录 ID")
	}
	var response recordIDResponse
	return c.do(ctx, "DeleteDomainRecord", map[string]string{"RecordId": strings.TrimSpace(record.ID)}, &response)
}

// Push synchronizes the managed record name with ipsRaw through the shared
// provider-agnostic reconcile loop.
func Push(ctx context.Context, client *Client, cfg Config, ipsRaw string) (dnspush.PushResult, error) {
	if client == nil {
		client = NewClient(cfg.AccessKeyID, cfg.AccessKeySecret)
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
	reconciled, err := dnspush.Reconcile(ctx, backend, desired)
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
		ID:      strings.TrimSpace(item.RecordID),
		Name:    dnspush.AbsoluteName(zone.Name, item.RR),
		TTL:     item.TTL,
		Type:    strings.ToUpper(strings.TrimSpace(item.Type)),
	}
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

// Sign returns the base64 Signature for the given query parameters, using the
// Alidns RPC SignatureVersion 1.0 rules (RFC3986 encoding, sorted canonical
// query, HMAC-SHA1 over "GET&%2F&<encoded canonical query>").
func (c *Client) Sign(params map[string]string) string {
	return signParams(params, c.accessKeySecret)
}

func signParams(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, percentEncode(key)+"="+percentEncode(params[key]))
	}
	stringToSign := "GET&" + percentEncode("/") + "&" + percentEncode(strings.Join(pairs, "&"))
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func percentEncode(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for index := 0; index < len(value); index++ {
		ch := value[index]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
			builder.WriteByte(ch)
		case ch == '-' || ch == '_' || ch == '.' || ch == '~':
			builder.WriteByte(ch)
		default:
			fmt.Fprintf(&builder, "%%%02X", ch)
		}
	}
	return builder.String()
}

func (c *Client) do(ctx context.Context, action string, params map[string]string, target any) error {
	query := make(map[string]string, len(params)+7)
	for key, value := range params {
		query[key] = value
	}
	query["AccessKeyId"] = c.accessKeyID
	query["Action"] = action
	query["Format"] = "JSON"
	query["SignatureMethod"] = signatureMethod
	now := c.now().UTC()
	query["SignatureNonce"] = strconv.FormatInt(now.UnixNano(), 10)
	query["SignatureVersion"] = signatureVersion
	query["Timestamp"] = now.Format(dateLayout)
	query["Version"] = apiVersion
	query["Signature"] = c.Sign(query)

	endpoint, err := url.Parse(c.baseURL + "/")
	if err != nil {
		return err
	}
	values := endpoint.Query()
	for key, value := range query {
		values.Set(key, value)
	}
	endpoint.RawQuery = values.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")

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
		return fmt.Errorf("阿里云 DNS API 返回状态 %s：%w", response.Status, parseFailure(raw))
	}
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	return nil
}

func parseFailure(raw []byte) error {
	failure := &apiError{}
	if err := json.Unmarshal(raw, failure); err == nil && failure.Error() != "" {
		return failure
	}
	return errors.New(strings.TrimSpace(string(raw)))
}

// ErrorCode reports the Alidns error code carried by err, if any.
func ErrorCode(err error) string {
	var failure *apiError
	if errors.As(err, &failure) {
		return strings.TrimSpace(failure.Code)
	}
	return ""
}
