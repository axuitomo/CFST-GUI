package httpclient

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/http2"
)

type Protocol string

const (
	ProtocolAuto Protocol = "auto"
	ProtocolTCP  Protocol = "tcp"
	ProtocolH1   Protocol = "h1"
	ProtocolH2   Protocol = "h2"
	ProtocolH3   Protocol = "h3"
)

const defaultH3FailureTTL = 2 * time.Minute

type Options struct {
	Protocol              Protocol
	Profile               httpcfg.Profile
	Timeout               time.Duration
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	DialContext           func(ctx context.Context, network, address string) (net.Conn, error)
	DialAddress           string
	CapturedConn          *net.Conn
	DisableProxy          bool
	CheckRedirect         func(req *http.Request, via []*http.Request) error
}

type fallbackRoundTripper struct {
	h3  http.RoundTripper
	tcp http.RoundTripper
}

type closeIdleRoundTripper interface {
	CloseIdleConnections()
}

type closeRoundTripper interface {
	Close() error
}

var h3FailureCache = struct {
	sync.Mutex
	until map[string]time.Time
}{
	until: map[string]time.Time{},
}

func NormalizeProtocol(value string, fallback Protocol) Protocol {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return fallback
	case "auto":
		return ProtocolAuto
	case "tcp":
		return ProtocolTCP
	case "h1", "h1.1", "http1", "http1.1", "http/1.1":
		return ProtocolH1
	case "h2", "http2", "http/2":
		return ProtocolH2
	case "h3", "http3", "http/3":
		return ProtocolH3
	default:
		return fallback
	}
}

func DefaultProtocolFromEnv() Protocol {
	return NormalizeProtocol(os.Getenv("CFST_HTTP_PROTOCOL"), ProtocolAuto)
}

func NewClient(opts Options) *http.Client {
	client := &http.Client{
		Transport: NewRoundTripper(opts),
		Timeout:   opts.Timeout,
	}
	if opts.CheckRedirect != nil {
		client.CheckRedirect = opts.CheckRedirect
	}
	return client
}

func NewRoundTripper(opts Options) http.RoundTripper {
	protocol := opts.Protocol
	if protocol == "" {
		protocol = DefaultProtocolFromEnv()
	}
	switch protocol {
	case ProtocolH1:
		return newTCPTransport(opts, false)
	case ProtocolH2:
		return newH2Transport(opts)
	case ProtocolH3:
		return newH3OnlyTransport(opts)
	case ProtocolTCP:
		return newTCPTransport(opts, true)
	default:
		return &fallbackRoundTripper{
			h3:  newH3OnlyTransport(opts),
			tcp: newTCPTransport(opts, true),
		}
	}
}

func ApplyNoCache(req *http.Request) {
	req.Header.Set("Cache-Control", "no-store")
	req.Header.Set("Pragma", "no-cache")
}

func DirectDialContext(ip *net.IPAddr, port int, profile httpcfg.Profile) func(ctx context.Context, network, address string) (net.Conn, error) {
	dialAddress := profile.DialAddress(ip, port)
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if strings.TrimSpace(dialAddress) != "" {
			address = dialAddress
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
}

func (t *fallbackRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if !canTryH3(req) || isH3OriginCached(req) {
		return t.tcp.RoundTrip(req)
	}
	res, err := t.h3.RoundTrip(req)
	if err == nil {
		return res, nil
	}
	rememberH3Failure(req)
	return t.tcp.RoundTrip(cloneRequestForRetry(req))
}

func (t *fallbackRoundTripper) CloseIdleConnections() {
	if closer, ok := t.h3.(closeIdleRoundTripper); ok {
		closer.CloseIdleConnections()
	}
	if closer, ok := t.tcp.(closeIdleRoundTripper); ok {
		closer.CloseIdleConnections()
	}
}

func (t *fallbackRoundTripper) Close() error {
	var errs []error
	if closer, ok := t.h3.(closeRoundTripper); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := t.tcp.(closeRoundTripper); ok {
		errs = append(errs, closer.Close())
	}
	return errors.Join(errs...)
}

func canTryH3(req *http.Request) bool {
	if req == nil || req.URL == nil || !strings.EqualFold(req.URL.Scheme, "https") {
		return false
	}
	if req.Body != nil && req.GetBody == nil {
		return false
	}
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func cloneRequestForRetry(req *http.Request) *http.Request {
	cloned := req.Clone(req.Context())
	cloned.Host = req.Host
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err == nil {
			cloned.Body = body
		}
	}
	return cloned
}

func newTCPTransport(opts Options, forceHTTP2 bool) *http.Transport {
	tlsConfig := tlsConfigForProfile(opts.Profile, nil)
	if !forceHTTP2 {
		tlsConfig.NextProtos = []string{"http/1.1"}
	}
	transport := &http.Transport{
		DialContext:           dialContextForOptions(opts),
		ForceAttemptHTTP2:     forceHTTP2,
		MaxIdleConns:          1024,
		MaxIdleConnsPerHost:   256,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   durationOrDefault(opts.TLSHandshakeTimeout, 10*time.Second),
		ResponseHeaderTimeout: opts.ResponseHeaderTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsConfig,
	}
	if !opts.DisableProxy {
		transport.Proxy = http.ProxyFromEnvironment
	}
	return transport
}

func newH2Transport(opts Options) *schemeGuardTransport {
	tlsConfig := tlsConfigForProfile(opts.Profile, []string{"h2"})
	dialContext := dialContextForOptions(opts)
	return &schemeGuardTransport{
		requiredScheme: "https",
		protocol:       ProtocolH2,
		next: &http2.Transport{
			TLSClientConfig: tlsConfig,
			DialTLSContext: func(ctx context.Context, network, address string, cfg *tls.Config) (net.Conn, error) {
				conn, err := dialContext(ctx, network, address)
				if err != nil {
					return nil, err
				}
				tlsConn := tls.Client(conn, cfg)
				if err := tlsConn.HandshakeContext(ctx); err != nil {
					_ = conn.Close()
					return nil, err
				}
				return tlsConn, nil
			},
			IdleConnTimeout: 30 * time.Second,
		},
	}
}

func newH3OnlyTransport(opts Options) *schemeGuardTransport {
	dialAddress := strings.TrimSpace(opts.DialAddress)
	return &schemeGuardTransport{
		requiredScheme: "https",
		protocol:       ProtocolH3,
		next: &http3.Transport{
			TLSClientConfig: tlsConfigForProfile(opts.Profile, []string{"h3"}),
			QUICConfig: &quic.Config{
				HandshakeIdleTimeout: durationOrDefault(opts.TLSHandshakeTimeout, 10*time.Second),
				MaxIdleTimeout:       30 * time.Second,
			},
			Dial: func(ctx context.Context, address string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
				if dialAddress == "" {
					return quic.DialAddrEarly(ctx, address, tlsCfg, cfg)
				}
				udpAddr, err := net.ResolveUDPAddr("udp", dialAddress)
				if err != nil {
					return nil, err
				}
				conn, err := net.ListenUDP("udp", nil)
				if err != nil {
					return nil, err
				}
				quicConn, err := quic.DialEarly(ctx, conn, udpAddr, tlsCfg, cfg)
				if err != nil {
					_ = conn.Close()
					return nil, err
				}
				return quicConn, nil
			},
		},
	}
}

type schemeGuardTransport struct {
	requiredScheme string
	protocol       Protocol
	next           http.RoundTripper
}

func (t *schemeGuardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.requiredScheme != "" && (req == nil || req.URL == nil || !strings.EqualFold(req.URL.Scheme, t.requiredScheme)) {
		return nil, errors.New(string(t.protocol) + " requires " + t.requiredScheme)
	}
	return t.next.RoundTrip(req)
}

func (t *schemeGuardTransport) CloseIdleConnections() {
	if closer, ok := t.next.(closeIdleRoundTripper); ok {
		closer.CloseIdleConnections()
	}
}

func (t *schemeGuardTransport) Close() error {
	if closer, ok := t.next.(closeRoundTripper); ok {
		return closer.Close()
	}
	return nil
}

func dialContextForOptions(opts Options) func(ctx context.Context, network, address string) (net.Conn, error) {
	dialContext := opts.DialContext
	if dialContext == nil {
		dialer := &net.Dialer{
			Timeout:   durationOrDefault(opts.TLSHandshakeTimeout, 10*time.Second),
			KeepAlive: 30 * time.Second,
		}
		dialContext = dialer.DialContext
	}
	if opts.CapturedConn == nil {
		return dialContext
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dialContext(ctx, network, address)
		if err == nil && conn != nil {
			*opts.CapturedConn = conn
		}
		return conn, err
	}
}

func tlsConfigForProfile(profile httpcfg.Profile, nextProtos []string) *tls.Config {
	cfg := &tls.Config{
		InsecureSkipVerify: profile.InsecureSkipVerify,
	}
	if profile.HasCustomSNI() {
		cfg.ServerName = profile.SNI
	}
	if len(nextProtos) > 0 {
		cfg.NextProtos = nextProtos
	}
	return cfg
}

func durationOrDefault(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func h3Origin(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	return strings.ToLower(req.URL.Scheme + "://" + req.URL.Host)
}

func isH3OriginCached(req *http.Request) bool {
	origin := h3Origin(req)
	if origin == "" {
		return false
	}
	now := time.Now()
	h3FailureCache.Lock()
	defer h3FailureCache.Unlock()
	until, ok := h3FailureCache.until[origin]
	if !ok {
		return false
	}
	if now.After(until) {
		delete(h3FailureCache.until, origin)
		return false
	}
	return true
}

func rememberH3Failure(req *http.Request) {
	origin := h3Origin(req)
	if origin == "" {
		return
	}
	h3FailureCache.Lock()
	h3FailureCache.until[origin] = time.Now().Add(defaultH3FailureTTL)
	h3FailureCache.Unlock()
}

func CleanupExpiredH3FailureCache() int {
	now := time.Now()
	h3FailureCache.Lock()
	defer h3FailureCache.Unlock()
	removed := 0
	for origin, until := range h3FailureCache.until {
		if now.After(until) {
			delete(h3FailureCache.until, origin)
			removed++
		}
	}
	return removed
}

func ResetH3FailureCacheForTest() {
	h3FailureCache.Lock()
	h3FailureCache.until = map[string]time.Time{}
	h3FailureCache.Unlock()
}
