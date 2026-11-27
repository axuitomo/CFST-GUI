// Package dnshecore implements the DNSHE free domain API (V2.0) DNS record
// upload used by the multi-platform DNS push pipeline.
package dnshecore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspush"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
)

var APIBaseURL = "https://api005.dnshe.com/index.php"

const (
	RecordTypeA    = "A"
	RecordTypeAAAA = "AAAA"
	DefaultTTL     = 600

	moduleName = "domain_hub"
)

// errSubdomainNotFound marks a lookup that matched no subdomain entry, so
// callers can fall back to reading a bare label as a host label.
var errSubdomainNotFound = errors.New("DNSHE 账号下找不到子域名")

type Config struct {
	APIKey      string
	APISecret   string
	AllowCNAME  bool
	RecordName  string
	RecordType  string
	Subdomain   string
	SubdomainID int
	TTL         int
}

type Subdomain struct {
	ID         int
	FullDomain string
	RootDomain string
	Subdomain  string
	Status     string
}

type Record struct {
	Content          string
	ID               string
	Name             string
	Proxied          bool
	ProviderRecordID string
	Status           string
	TTL              int
	Type             string
}

type PushResult struct {
	HasInputIPs    bool
	IgnoredEntries []string
	RecordsAfter   []Record
	Summary        dnspush.Summary
	Warnings       []string
}

type ClientOptions struct {
	APIKey     string
	APISecret  string
	BaseURL    string
	HTTPClient *http.Client
}

type Client struct {
	apiKey     string
	apiSecret  string
	baseURL    string
	httpClient *http.Client

	requestMu          sync.Mutex
	lastRequest        time.Time
	minRequestInterval time.Duration
	retryBaseDelay     time.Duration
	maxRetries         int
}

type apiError struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

func (e *apiError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "DNSHE API 返回失败"
	}
	if code := strings.TrimSpace(e.ErrorCode); code != "" {
		return message + " (" + code + ")"
	}
	return message
}

type subdomainsResponse struct {
	Success    bool        `json:"success"`
	ErrorCode  string      `json:"error_code"`
	Message    string      `json:"message"`
	Subdomains []subdomain `json:"subdomains"`
	Pagination struct {
		HasMore  bool `json:"has_more"`
		NextPage int  `json:"next_page"`
	} `json:"pagination"`
}

type subdomain struct {
	ID         int    `json:"id"`
	Subdomain  string `json:"subdomain"`
	RootDomain string `json:"rootdomain"`
	FullDomain string `json:"full_domain"`
	Status     string `json:"status"`
}

type subdomainResponse struct {
	Success   bool      `json:"success"`
	ErrorCode string    `json:"error_code"`
	Message   string    `json:"message"`
	Subdomain subdomain `json:"subdomain"`
}

type recordsResponse struct {
	Success   bool        `json:"success"`
	ErrorCode string      `json:"error_code"`
	Message   string      `json:"message"`
	Records   []apiRecord `json:"records"`
}

type recordResponse struct {
	Success   bool      `json:"success"`
	ErrorCode string    `json:"error_code"`
	Message   string    `json:"message"`
	ID        int       `json:"id"`
	RecordID  string    `json:"record_id"`
	Record    apiRecord `json:"record"`
}

type apiRecord struct {
	ID        int    `json:"id"`
	RecordID  string `json:"record_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	TTL       int    `json:"ttl"`
	Proxied   bool   `json:"proxied"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

func NewClient(apiKey, apiSecret string) *Client {
	return NewClientWithOptions(ClientOptions{APIKey: apiKey, APISecret: apiSecret})
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
	return &Client{
		apiKey:         strings.TrimSpace(options.APIKey),
		apiSecret:      strings.TrimSpace(options.APISecret),
		baseURL:        strings.TrimRight(baseURL, "/"),
		httpClient:     httpClient,
		retryBaseDelay: time.Second,
		maxRetries:     3,
	}
}

// SetRequestInterval spaces consecutive API calls. It is used by the DNS
// self-test, whose full flow is longer than DNSHE's default 30 requests/min.
func (c *Client) SetRequestInterval(interval time.Duration) {
	c.requestMu.Lock()
	c.minRequestInterval = interval
	c.requestMu.Unlock()
}

func (c *Client) waitRequestInterval(ctx context.Context) error {
	c.requestMu.Lock()
	defer c.requestMu.Unlock()
	if c.minRequestInterval > 0 && !c.lastRequest.IsZero() {
		if wait := c.minRequestInterval - time.Since(c.lastRequest); wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	c.lastRequest = time.Now()
	return nil
}

// ListSubdomains reads every subdomain available to the API key.
func (c *Client) ListSubdomains(ctx context.Context) ([]Subdomain, error) {
	result := make([]Subdomain, 0)
	for page := 1; page <= 100; page++ {
		query := url.Values{}
		query.Set("m", moduleName)
		query.Set("endpoint", "subdomains")
		query.Set("action", "list")
		query.Set("page", strconv.Itoa(page))
		query.Set("per_page", "500")
		var response subdomainsResponse
		if err := c.do(ctx, http.MethodGet, query, nil, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Subdomains {
			result = append(result, Subdomain{
				ID:         item.ID,
				FullDomain: strings.TrimSpace(item.FullDomain),
				RootDomain: strings.TrimSpace(item.RootDomain),
				Status:     strings.TrimSpace(item.Status),
				Subdomain:  strings.TrimSpace(item.Subdomain),
			})
		}
		if !response.Pagination.HasMore {
			break
		}
	}
	return result, nil
}

// GetSubdomain reads one subdomain by its module ID.
func (c *Client) GetSubdomain(ctx context.Context, id int) (Subdomain, error) {
	if id <= 0 {
		return Subdomain{}, errors.New("缺少 DNSHE 子域 ID")
	}
	query := url.Values{}
	query.Set("m", moduleName)
	query.Set("endpoint", "subdomains")
	query.Set("action", "get")
	query.Set("subdomain_id", strconv.Itoa(id))
	var response subdomainResponse
	if err := c.do(ctx, http.MethodGet, query, nil, &response); err != nil {
		return Subdomain{}, err
	}
	item := response.Subdomain
	if item.ID == 0 {
		return Subdomain{}, fmt.Errorf("DNSHE 未返回子域 %d 的信息", id)
	}
	return Subdomain{
		ID:         item.ID,
		FullDomain: strings.TrimSpace(item.FullDomain),
		RootDomain: strings.TrimSpace(item.RootDomain),
		Status:     strings.TrimSpace(item.Status),
		Subdomain:  strings.TrimSpace(item.Subdomain),
	}, nil
}

// ResolveSubdomain finds the subdomain that owns fullDomain, preferring the
// longest matching suffix so nested records still resolve to their zone.
func (c *Client) ResolveSubdomain(ctx context.Context, fullDomain string) (Subdomain, error) {
	fullDomain = normalizeDomain(fullDomain)
	if fullDomain == "" {
		return Subdomain{}, errors.New("缺少 DNSHE 域名")
	}
	items, err := c.ListSubdomains(ctx)
	if err != nil {
		return Subdomain{}, err
	}
	best := Subdomain{}
	for _, item := range items {
		item.FullDomain = normalizeDomain(item.FullDomain)
		if item.FullDomain == "" {
			continue
		}
		if item.FullDomain != fullDomain && !strings.HasSuffix(fullDomain, "."+item.FullDomain) {
			continue
		}
		if len(item.FullDomain) > len(best.FullDomain) {
			best = item
		}
	}
	if best.ID == 0 {
		return Subdomain{}, fmt.Errorf("DNSHE 账号下找不到与 %s 匹配的子域名，请先在自由域名管理中注册", fullDomain)
	}
	return best, nil
}

// ResolveSubdomainByName finds a subdomain by its label (for example "test")
// or its full domain (for example "test.example.com"). A bare label that
// exists under several roots is reported as ambiguous.
func (c *Client) ResolveSubdomainByName(ctx context.Context, value string) (Subdomain, error) {
	normalized := normalizeDomain(value)
	if normalized == "" {
		return Subdomain{}, errors.New("缺少 DNSHE 子域名")
	}
	items, err := c.ListSubdomains(ctx)
	if err != nil {
		return Subdomain{}, err
	}
	matches := make([]Subdomain, 0, 2)
	for _, item := range items {
		fullDomain := normalizeDomain(item.FullDomain)
		if fullDomain == "" {
			fullDomain = normalizeDomain(item.Subdomain + "." + item.RootDomain)
		}
		if fullDomain == "" || item.ID == 0 {
			continue
		}
		item.FullDomain = fullDomain
		if fullDomain == normalized || strings.EqualFold(strings.TrimSpace(item.Subdomain), normalized) {
			matches = append(matches, item)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return Subdomain{}, fmt.Errorf("%w %s", errSubdomainNotFound, value)
	default:
		fullDomains := make([]string, 0, len(matches))
		for _, item := range matches {
			fullDomains = append(fullDomains, item.FullDomain)
		}
		slices.Sort(fullDomains)
		return Subdomain{}, fmt.Errorf("DNSHE 子域名 %s 对应多个完整域名，请改为填写完整域名：%s", value, strings.Join(fullDomains, ", "))
	}
}

// ResolveSubdomainTarget resolves the zone that owns recordName and returns the
// record name to write. A bare label that is not a registered subdomain is read
// as a host label under recordName: "test" with "axuitomo.l.cd" targets
// "test.axuitomo.l.cd". A full domain or an exact subdomain label keeps the
// zone-selector meaning so existing configurations stay valid.
func (c *Client) ResolveSubdomainTarget(ctx context.Context, subdomainValue, recordName string) (Subdomain, string, error) {
	value := normalizeDomain(subdomainValue)
	base := normalizeDomain(recordName)
	if value == "" {
		return Subdomain{}, "", errors.New("缺少 DNSHE 子域名")
	}
	subdomain, err := c.ResolveSubdomainByName(ctx, value)
	if err == nil {
		return subdomain, base, nil
	}
	// Only a bare label may act as a host label. A full domain miss stays an
	// error so a typo can never silently retarget another zone.
	if !errors.Is(err, errSubdomainNotFound) || strings.Contains(value, ".") || base == "" {
		return Subdomain{}, "", err
	}
	target := base
	if !strings.HasPrefix(base, value+".") {
		target = value + "." + base
	}
	subdomain, resolveErr := c.ResolveSubdomain(ctx, target)
	if resolveErr != nil {
		return Subdomain{}, "", fmt.Errorf("%w；按完整域名 %s 解析时同样失败：%v", err, target, resolveErr)
	}
	return subdomain, target, nil
}

func (c *Client) ListRecords(ctx context.Context, subdomainID int) ([]Record, error) {
	query := url.Values{}
	query.Set("m", moduleName)
	query.Set("endpoint", "dns_records")
	query.Set("action", "list")
	query.Set("subdomain_id", strconv.Itoa(subdomainID))
	var response recordsResponse
	if err := c.do(ctx, http.MethodGet, query, nil, &response); err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(response.Records))
	for _, item := range response.Records {
		records = append(records, fromAPIRecord(item))
	}
	return records, nil
}

func (c *Client) CreateRecord(ctx context.Context, subdomainID int, record Record) (Record, error) {
	body := map[string]any{
		"subdomain_id": subdomainID,
		"content":      record.Content,
		"name":         record.Name,
		"ttl":          record.TTL,
		"type":         record.Type,
	}
	query := url.Values{}
	query.Set("m", moduleName)
	query.Set("endpoint", "dns_records")
	query.Set("action", "create")
	var response recordResponse
	if err := c.do(ctx, http.MethodPost, query, body, &response); err != nil {
		return Record{}, err
	}
	created := fromAPIRecord(response.Record)
	if created.ID == "" {
		if response.ID > 0 {
			created.ID = strconv.Itoa(response.ID)
		}
		created.ProviderRecordID = strings.TrimSpace(response.RecordID)
	}
	return created, nil
}

func (c *Client) UpdateRecord(ctx context.Context, record Record) (Record, error) {
	if strings.TrimSpace(record.ID) == "" && strings.TrimSpace(record.ProviderRecordID) == "" {
		return Record{}, errors.New("缺少 DNSHE 记录 ID")
	}
	body := map[string]any{
		"name":    record.Name,
		"ttl":     record.TTL,
		"type":    record.Type,
		"content": record.Content,
	}
	if record.ID != "" {
		if id, err := strconv.Atoi(record.ID); err == nil {
			body["id"] = id
		} else {
			body["record_id"] = record.ID
		}
	} else {
		body["record_id"] = record.ProviderRecordID
	}
	query := url.Values{}
	query.Set("m", moduleName)
	query.Set("endpoint", "dns_records")
	query.Set("action", "update")
	var response recordResponse
	if err := c.do(ctx, http.MethodPost, query, body, &response); err != nil {
		return Record{}, err
	}
	updated := fromAPIRecord(response.Record)
	if updated.ID == "" {
		updated = record
	}
	return updated, nil
}

func (c *Client) DeleteRecord(ctx context.Context, record Record) error {
	body := map[string]any{}
	if record.ID != "" {
		if id, err := strconv.Atoi(record.ID); err == nil {
			body["id"] = id
		} else {
			body["record_id"] = record.ID
		}
	} else if record.ProviderRecordID != "" {
		body["record_id"] = record.ProviderRecordID
	} else {
		return errors.New("缺少 DNSHE 记录 ID")
	}
	query := url.Values{}
	query.Set("m", moduleName)
	query.Set("endpoint", "dns_records")
	query.Set("action", "delete")
	var response recordResponse
	return c.do(ctx, http.MethodPost, query, body, &response)
}

// Push synchronizes the managed record name with ipsRaw through the shared
// provider-agnostic reconcile loop. It manages A and AAAA from the given pool;
// cfg.RecordType is not consulted, so callers must filter the address family.
func Push(ctx context.Context, client *Client, cfg Config, ipsRaw string) (PushResult, error) {
	if client == nil {
		client = NewClient(cfg.APIKey, cfg.APISecret)
	}
	ipGroups, ignored := dnspush.NormalizeIPs(ipsRaw)
	result := newPushResult(ignored)
	result.HasInputIPs = ipGroups.HasIPs()
	if !result.HasInputIPs {
		return result, nil
	}
	subdomain, recordName, err := resolveTarget(ctx, client, cfg)
	if err != nil {
		return result, &dnspush.OperationError{Operation: dnspush.OperationList, Err: err}
	}
	cfg.RecordName = recordName
	return pushResolvedWithGroups(ctx, client, cfg, subdomain, ipGroups, result)
}

// PushResolved is Push with an already resolved subdomain. The self-test uses
// it to avoid repeating GetSubdomain/ListSubdomains on every stage.
func PushResolved(ctx context.Context, client *Client, cfg Config, subdomain Subdomain, ipsRaw string) (PushResult, error) {
	if client == nil {
		client = NewClient(cfg.APIKey, cfg.APISecret)
	}
	ipGroups, ignored := dnspush.NormalizeIPs(ipsRaw)
	result := newPushResult(ignored)
	result.HasInputIPs = ipGroups.HasIPs()
	if !result.HasInputIPs {
		return result, nil
	}
	subdomain.FullDomain = normalizeDomain(subdomain.FullDomain)
	if subdomain.FullDomain == "" {
		return result, fmt.Errorf("DNSHE 子域 %d 缺少完整域名", subdomain.ID)
	}
	recordName := normalizeDomain(cfg.RecordName)
	if recordName != "" && recordName != subdomain.FullDomain && !strings.HasSuffix(recordName, "."+subdomain.FullDomain) {
		return result, fmt.Errorf("DNSHE 记录名 %s 不属于子域名 %s，已拒绝写入以避免覆盖其它子域", recordName, subdomain.FullDomain)
	}
	if recordName == "" {
		cfg.RecordName = subdomain.FullDomain
	}
	return pushResolvedWithGroups(ctx, client, cfg, subdomain, ipGroups, result)
}

func newPushResult(ignored []string) PushResult {
	return PushResult{
		HasInputIPs:    false,
		IgnoredEntries: ignored,
		RecordsAfter:   []Record{},
		Summary:        dnspush.Summary{Ignored: len(ignored)},
		Warnings:       []string{},
	}
}

func pushResolvedWithGroups(ctx context.Context, client *Client, cfg Config, subdomain Subdomain, ipGroups dnspush.IPGroups, result PushResult) (PushResult, error) {
	result.HasInputIPs = ipGroups.HasIPs()
	backend := &pushBackend{client: client, cfg: cfg, subdomain: subdomain}
	desired := make([]dnspush.Record, 0, len(ipGroups.A)+len(ipGroups.AAAA))
	for _, recordType := range []string{RecordTypeA, RecordTypeAAAA} {
		for _, content := range ipGroups.ForType(recordType) {
			desired = append(desired, dnspush.Record{
				Content: content,
				Name:    cfg.RecordName,
				TTL:     cfg.TTL,
				Type:    recordType,
			})
		}
	}
	reconciled, err := dnspush.ReconcileWithOptions(ctx, backend, desired, dnspush.ReconcileOptions{AllowCNAME: cfg.AllowCNAME})
	result.Summary.Created = reconciled.Summary.Created
	result.Summary.Deleted = reconciled.Summary.Deleted
	result.Summary.Updated = reconciled.Summary.Updated
	result.Warnings = append(result.Warnings, reconciled.Warnings...)
	if err != nil {
		return result, err
	}
	recordsAfter, listErr := client.ListRecords(ctx, subdomain.ID)
	if listErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("推送完成但刷新 DNS 记录失败：%v", listErr))
	} else {
		result.RecordsAfter = recordsAfter
	}
	return result, nil
}

func resolveTarget(ctx context.Context, client *Client, cfg Config) (Subdomain, string, error) {
	var (
		subdomain  Subdomain
		recordName string
		err        error
	)
	switch {
	case strings.TrimSpace(cfg.Subdomain) != "":
		subdomain, recordName, err = client.ResolveSubdomainTarget(ctx, cfg.Subdomain, cfg.RecordName)
	case cfg.SubdomainID > 0:
		subdomain, err = client.GetSubdomain(ctx, cfg.SubdomainID)
		recordName = cfg.RecordName
	default:
		subdomain, err = client.ResolveSubdomain(ctx, cfg.RecordName)
		recordName = cfg.RecordName
	}
	if err != nil {
		return Subdomain{}, "", err
	}
	subdomain.FullDomain = normalizeDomain(subdomain.FullDomain)
	if subdomain.FullDomain == "" {
		return Subdomain{}, "", fmt.Errorf("DNSHE 子域 %d 缺少完整域名", subdomain.ID)
	}
	recordName = normalizeDomain(recordName)
	if recordName == "" {
		recordName = subdomain.FullDomain
	}
	if recordName != subdomain.FullDomain && !strings.HasSuffix(recordName, "."+subdomain.FullDomain) {
		return Subdomain{}, "", fmt.Errorf("DNSHE 记录名 %s 不属于子域名 %s，已拒绝写入以避免覆盖其它子域", recordName, subdomain.FullDomain)
	}
	return subdomain, recordName, nil
}

type pushBackend struct {
	client    *Client
	cfg       Config
	subdomain Subdomain
}

func (b *pushBackend) ListRecords(ctx context.Context) ([]dnspush.Record, error) {
	records, err := b.client.ListRecords(ctx, b.subdomain.ID)
	if err != nil {
		return nil, err
	}
	name := normalizeDomain(b.cfg.RecordName)
	converted := make([]dnspush.Record, 0, len(records))
	for _, record := range records {
		full := recordFullName(b.subdomain.FullDomain, record.Name)
		if full != name {
			continue
		}
		record.Name = full
		converted = append(converted, toPushRecord(record))
	}
	return converted, nil
}

func (b *pushBackend) CreateRecord(ctx context.Context, record dnspush.Record) (dnspush.Record, error) {
	created, err := b.client.CreateRecord(ctx, b.subdomain.ID, fromPushRecord(b.subdomain, record))
	if err != nil {
		return record, err
	}
	created.Name = record.Name
	return toPushRecord(created), nil
}

func (b *pushBackend) UpdateRecord(ctx context.Context, record dnspush.Record) (dnspush.Record, error) {
	updated, err := b.client.UpdateRecord(ctx, fromPushRecord(b.subdomain, record))
	if err != nil {
		return record, err
	}
	updated.Name = record.Name
	return toPushRecord(updated), nil
}

func (b *pushBackend) DeleteRecord(ctx context.Context, record dnspush.Record) error {
	return b.client.DeleteRecord(ctx, fromPushRecord(b.subdomain, record))
}

func (b *pushBackend) EquivalentRecord(existing, desired dnspush.Record) bool {
	return strings.EqualFold(strings.TrimSpace(existing.Type), strings.TrimSpace(desired.Type)) &&
		normalizeDomain(existing.Name) == normalizeDomain(desired.Name) &&
		strings.TrimSpace(existing.Content) == strings.TrimSpace(desired.Content) &&
		existing.TTL == desired.TTL
}

func toPushRecord(record Record) dnspush.Record {
	return dnspush.Record{
		Attrs:   map[string]string{"provider_record_id": record.ProviderRecordID},
		Content: record.Content,
		ID:      record.ID,
		Name:    record.Name,
		TTL:     record.TTL,
		Type:    record.Type,
	}
}

func fromPushRecord(subdomain Subdomain, record dnspush.Record) Record {
	return Record{
		Content:          record.Content,
		ID:               record.ID,
		Name:             relativeRecordName(subdomain.FullDomain, record.Name),
		ProviderRecordID: record.Attrs["provider_record_id"],
		TTL:              record.TTL,
		Type:             record.Type,
	}
}

// relativeRecordName converts a full record name into the value DNSHE expects:
// the subdomain itself is addressed as "@", child records by their prefix.
func relativeRecordName(subdomainFullDomain, recordName string) string {
	subdomainFullDomain = normalizeDomain(subdomainFullDomain)
	recordName = normalizeDomain(recordName)
	if subdomainFullDomain == "" || recordName == "" || recordName == subdomainFullDomain {
		return "@"
	}
	if strings.HasSuffix(recordName, "."+subdomainFullDomain) {
		prefix := strings.TrimSuffix(recordName, "."+subdomainFullDomain)
		if prefix != "" {
			return prefix
		}
	}
	return "@"
}

// recordFullName expands the record name DNSHE returns ("@", a bare label or a
// full domain) into the absolute record name.
func recordFullName(subdomainFullDomain, name string) string {
	subdomainFullDomain = normalizeDomain(subdomainFullDomain)
	normalized := normalizeDomain(name)
	switch {
	case normalized == "" || normalized == "@":
		return subdomainFullDomain
	case !strings.Contains(normalized, "."):
		return normalized + "." + subdomainFullDomain
	default:
		return normalized
	}
}
func normalizeDomain(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

func fromAPIRecord(item apiRecord) Record {
	return Record{
		Content:          strings.TrimSpace(item.Content),
		ID:               recordID(item),
		Name:             strings.TrimSpace(item.Name),
		Proxied:          item.Proxied,
		ProviderRecordID: strings.TrimSpace(item.RecordID),
		Status:           strings.TrimSpace(item.Status),
		TTL:              item.TTL,
		Type:             strings.ToUpper(strings.TrimSpace(item.Type)),
	}
}

func recordID(item apiRecord) string {
	if item.ID > 0 {
		return strconv.Itoa(item.ID)
	}
	return strings.TrimSpace(item.RecordID)
}

func (c *Client) do(ctx context.Context, method string, query url.Values, body any, target any) error {
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	endpoint.RawQuery = query.Encode()

	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	for attempt := 0; ; attempt++ {
		if err := c.waitRequestInterval(ctx); err != nil {
			return err
		}
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
		if err != nil {
			return err
		}
		req.Header.Set("X-API-Key", c.apiKey)
		req.Header.Set("X-API-Secret", c.apiSecret)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		if res.StatusCode == http.StatusTooManyRequests {
			if attempt >= c.maxRetries {
				return fmt.Errorf("DNSHE API 触发限流（429），已重试 %d 次：%s", attempt, apiFailureMessage(raw))
			}
			delay := c.retryDelay(res, attempt)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("DNSHE API 返回状态 %s：%s", res.Status, apiFailureMessage(raw))
		}
		if len(raw) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return err
		}
		return responseError(target)
	}
}

func (c *Client) retryDelay(res *http.Response, attempt int) time.Duration {
	if res != nil {
		if seconds, err := strconv.Atoi(strings.TrimSpace(res.Header.Get("Retry-After"))); err == nil && seconds > 0 {
			if seconds > 30 {
				seconds = 30
			}
			return time.Duration(seconds) * time.Second
		}
	}
	base := c.retryBaseDelay
	if base <= 0 {
		base = time.Second
	}
	delay := base << attempt
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return delay
}

func responseError(response any) error {
	var success bool
	var failure apiError
	switch typed := response.(type) {
	case *subdomainsResponse:
		success, failure = typed.Success, apiError{ErrorCode: typed.ErrorCode, Message: typed.Message}
	case *subdomainResponse:
		success, failure = typed.Success, apiError{ErrorCode: typed.ErrorCode, Message: typed.Message}
	case *recordsResponse:
		success, failure = typed.Success, apiError{ErrorCode: typed.ErrorCode, Message: typed.Message}
	case *recordResponse:
		success, failure = typed.Success, apiError{ErrorCode: typed.ErrorCode, Message: typed.Message}
	default:
		return nil
	}
	if success {
		return nil
	}
	return &failure
}

func apiFailureMessage(raw []byte) string {
	var failure apiError
	if err := json.Unmarshal(raw, &failure); err == nil && failure.Error() != "" {
		return failure.Error()
	}
	return strings.TrimSpace(string(raw))
}

// ErrorCode reports the DNSHE error code carried by err, if any.
func ErrorCode(err error) string {
	var failure *apiError
	if errors.As(err, &failure) {
		return strings.TrimSpace(failure.ErrorCode)
	}
	return ""
}
