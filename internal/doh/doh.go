// Package doh resolves hostnames over DNS-over-HTTPS (RFC 8484).
//
// The GUI resolves input-source domains and source-URL hosts itself; a hidden
// ISP resolver can poison or drop those answers. DoH lets the user pick a
// trusted HTTPS resolver instead, while keeping the system resolver as a
// fallback when the DoH endpoint itself is unreachable.
package doh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"golang.org/x/net/dns/dnsmessage"
)

// DefaultTimeout bounds one A/AAAA query. A lookup runs two queries in
// parallel, so this is also roughly the lookup budget.
const DefaultTimeout = 4 * time.Second

// maxResponseBytes caps a DoH response. DNS messages are far smaller; the cap
// only stops a broken or hostile endpoint from streaming into memory.
const maxResponseBytes = 64 << 10

// Resolver answers A/AAAA lookups over DoH and implements sourceparse.Resolver.
type Resolver struct {
	Endpoint string
	Timeout  time.Duration
	// Fallback runs when the DoH endpoint fails at the transport, HTTP or
	// parsing level. A valid DNS answer (including NXDOMAIN) never falls back.
	// Nil uses the system resolver.
	Fallback func(ctx context.Context, host string) ([]net.IPAddr, error)
}

// New validates the endpoint and returns a resolver. The endpoint must be an
// https URL; plain http is rejected so a misconfiguration cannot downgrade to
// cleartext DNS.
func New(endpoint string) (*Resolver, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("DoH 地址无效：%w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return nil, errors.New("DoH 地址必须是 https:// 开头的完整 URL")
	}
	return &Resolver{Endpoint: parsed.String()}, nil
}

// LookupIPAddr resolves host with parallel A and AAAA queries.
func (r *Resolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	parent := ctx
	host = trimHost(host)
	if host == "" {
		return nil, &net.DNSError{Err: "empty host", Name: host, IsNotFound: true}
	}
	if ip := net.ParseIP(host); ip != nil {
		return []net.IPAddr{{IP: ip}}, nil
	}
	if r == nil || r.Endpoint == "" {
		return r.fallback(ctx, host)
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var (
		wait      sync.WaitGroup
		mu        sync.Mutex
		addrs     []net.IPAddr
		answered  bool
		lastError error
	)
	for _, qtype := range []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA} {
		queryType := qtype
		wait.Add(1)
		go func() {
			defer wait.Done()
			ips, gotAnswer, err := r.query(ctx, host, queryType)
			mu.Lock()
			defer mu.Unlock()
			if gotAnswer {
				answered = true
				addrs = append(addrs, ips...)
			}
			if err != nil && lastError == nil {
				lastError = err
			}
		}()
	}
	wait.Wait()

	if !answered {
		// The endpoint itself failed: fall back instead of failing the source.
		// The DoH budget is already spent, so the fallback gets the parent
		// context rather than the expired child.
		addrs, fallbackErr := r.fallback(parent, host)
		if fallbackErr != nil && lastError != nil {
			return nil, errors.Join(lastError, fallbackErr)
		}
		return addrs, fallbackErr
	}
	sortIPv4First(addrs)
	if len(addrs) == 0 {
		if lastError != nil {
			return nil, lastError
		}
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return addrs, nil
}

func (r *Resolver) fallback(ctx context.Context, host string) ([]net.IPAddr, error) {
	if r != nil && r.Fallback != nil {
		return r.Fallback(ctx, host)
	}
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// ECHConfig returns the ECHConfigList published in host's HTTPS RR (RFC 9460)
// and RFC 9849, or nil when the record has no ech parameter. Callers treat a
// missing list as "no ECH for this host" rather than an error.
func (r *Resolver) ECHConfig(ctx context.Context, host string) ([]byte, error) {
	host = trimHost(host)
	if r == nil || r.Endpoint == "" {
		return nil, nil
	}
	if host == "" || net.ParseIP(host) != nil {
		return nil, nil
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body, err := r.exchange(ctx, host, dnsmessage.TypeHTTPS)
	if err != nil {
		return nil, err
	}
	return parseECHConfig(body, host)
}

// query returns the addresses of one record type, whether the endpoint
// answered at all, and an error for a negative or malformed answer.
func (r *Resolver) query(ctx context.Context, host string, qtype dnsmessage.Type) ([]net.IPAddr, bool, error) {
	payload, err := packQuery(host, qtype)
	if err != nil {
		return nil, true, err
	}
	body, err := r.exchangePayload(ctx, payload)
	if err != nil {
		return nil, false, err
	}
	return parseAnswers(body, host)
}

func (r *Resolver) exchange(ctx context.Context, host string, qtype dnsmessage.Type) ([]byte, error) {
	payload, err := packQuery(host, qtype)
	if err != nil {
		return nil, err
	}
	return r.exchangePayload(ctx, payload)
}

// exchangePayload posts one packed query and returns the raw response body.
func (r *Resolver) exchangePayload(ctx context.Context, payload []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/dns-message")
	request.Header.Set("Accept", "application/dns-message")
	response, err := sharedClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("DoH 返回状态 %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("DoH 响应超过 %d 字节上限", maxResponseBytes)
	}
	return body, nil
}

func packQuery(host string, qtype dnsmessage.Type) ([]byte, error) {
	name, err := dnsmessage.NewName(host + ".")
	if err != nil {
		return nil, fmt.Errorf("域名 %q 无效：%w", host, err)
	}
	message := dnsmessage.Message{
		Header: dnsmessage.Header{RecursionDesired: true},
		Questions: []dnsmessage.Question{{
			Name:  name,
			Type:  qtype,
			Class: dnsmessage.ClassINET,
		}},
	}
	return message.Pack()
}

func parseAnswers(body []byte, host string) ([]net.IPAddr, bool, error) {
	var parser dnsmessage.Parser
	header, err := parser.Start(body)
	if err != nil {
		return nil, false, fmt.Errorf("DoH 响应无效：%w", err)
	}
	if header.RCode != dnsmessage.RCodeSuccess {
		notFound := header.RCode == dnsmessage.RCodeNameError
		return nil, true, &net.DNSError{Err: header.RCode.String(), Name: host, IsNotFound: notFound}
	}
	// Answers can only be parsed after the question section.
	if err := parser.SkipAllQuestions(); err != nil {
		return nil, false, fmt.Errorf("DoH 响应无效：%w", err)
	}
	answers, err := parser.AllAnswers()
	if err != nil {
		return nil, false, fmt.Errorf("DoH 响应无效：%w", err)
	}
	addrs := make([]net.IPAddr, 0, len(answers))
	for _, answer := range answers {
		switch record := answer.Body.(type) {
		case *dnsmessage.AResource:
			addrs = append(addrs, net.IPAddr{IP: net.IP(append([]byte(nil), record.A[:]...))})
		case *dnsmessage.AAAAResource:
			addrs = append(addrs, net.IPAddr{IP: net.IP(append([]byte(nil), record.AAAA[:]...))})
		}
	}
	return addrs, true, nil
}

// parseECHConfig extracts the ech parameter from an HTTPS RR answer. A
// negative answer means the host publishes no ECH, not a failure.
func parseECHConfig(body []byte, host string) ([]byte, error) {
	var parser dnsmessage.Parser
	header, err := parser.Start(body)
	if err != nil {
		return nil, fmt.Errorf("DoH 响应无效：%w", err)
	}
	if header.RCode != dnsmessage.RCodeSuccess {
		return nil, nil
	}
	if err := parser.SkipAllQuestions(); err != nil {
		return nil, fmt.Errorf("DoH 响应无效：%w", err)
	}
	answers, err := parser.AllAnswers()
	if err != nil {
		return nil, fmt.Errorf("DoH 响应无效：%w", err)
	}
	for _, answer := range answers {
		record, ok := answer.Body.(*dnsmessage.HTTPSResource)
		if !ok {
			continue
		}
		// Alias-mode records point at another name; following them would need a
		// second query, and no public ECH deployment uses alias mode.
		for _, param := range record.Params {
			if param.Key == dnsmessage.SVCParamECH {
				return append([]byte(nil), param.Value...), nil
			}
		}
	}
	return nil, nil
}

func trimHost(host string) string {
	return strings.TrimSuffix(strings.TrimSpace(host), ".")
}

// sortIPv4First keeps dialing cheap on networks where a broken IPv6 path would
// otherwise eat the first connection attempt.
func sortIPv4First(addrs []net.IPAddr) {
	for index := 1; index < len(addrs); index++ {
		for current := index; current > 0 && addrs[current].IP.To4() != nil && addrs[current-1].IP.To4() == nil; current-- {
			addrs[current], addrs[current-1] = addrs[current-1], addrs[current]
		}
	}
}

var (
	clientOnce sync.Once
	client     *http.Client
)

// sharedClient reuses one keep-alive HTTP/2 client for every endpoint, so a
// task resolving several sources does not pay a TLS handshake per source.
// TCP is pinned for the same reason as source fetching: a stalled HTTP/3
// handshake must not consume the lookup budget.
func sharedClient() *http.Client {
	clientOnce.Do(func() {
		// Direct连接与来源读取保持一致：系统代理在需要 DoH 的网络里往往正是
		// 失败或劫持的那一跳，走代理会让 DoH 失去意义。
		client = httpclient.NewClient(httpclient.Options{
			Protocol:     httpclient.ProtocolTCP,
			Timeout:      DefaultTimeout,
			DisableProxy: true,
		})
	})
	return client
}
