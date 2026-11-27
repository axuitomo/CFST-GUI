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
