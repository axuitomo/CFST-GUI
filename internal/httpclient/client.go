package httpclient

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
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

// A blocked UDP path affects every origin, so one failed QUIC attempt parks H3
// for the whole process for a short while. Probes build a fresh client per IP,
// so a per-client, per-origin cache could never suppress the next attempt.
//
// Only the TTL lifts the park. A success cannot clear it, because any probe that
// was already in flight when the failure landed would otherwise reopen H3 for
// every new client while the UDP path is still broken.
const defaultH3UnhealthyTTL = 30 * time.Second

var h3UnhealthyUntil atomic.Int64

func h3Unhealthy(now time.Time) bool {
	until := h3UnhealthyUntil.Load()
	return until != 0 && now.UnixNano() < until
}

func markH3Unhealthy(now time.Time) {
	h3UnhealthyUntil.Store(now.Add(defaultH3UnhealthyTTL).UnixNano())
}

// tlsConfigCache holds one config per profile/ALPN pair. Each config owns its
// session cache because the resume cache is keyed by server name only, so a
// shared cache would let a session built with InsecureSkipVerify be resumed by a
// profile that verifies certificates.
var tlsConfigCache struct {
	mu      sync.Mutex
	configs map[string]*tls.Config
}

type Options struct {
	Protocol              Protocol
	Profile               httpcfg.Profile
	Timeout               time.Duration
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	DialContext           func(ctx context.Context, network, address string) (net.Conn, error)
	// Resolver resolves hostnames when the transport dials by name. Nil uses
	// the system resolver.
	Resolver func(ctx context.Context, host string) ([]net.IPAddr, error)
	// ECHConfig returns a serialized ECHConfigList for host. When set, https
	// requests try Encrypted Client Hello and fall back to a plain handshake
	// if the server rejects it or publishes no config. ECH requires TLS and
	// therefore only applies to TCP transports.
	ECHConfig    func(ctx context.Context, host string) ([]byte, error)
	DialAddress  string
	CapturedConn *net.Conn
	// DisableProxy turns off the system proxy. Only the IP pool stage (source
	// reading) may follow it, so every other caller leaves this set.
	DisableProxy  bool
	CheckRedirect func(req *http.Request, via []*http.Request) error

	// tlsConfigOverride replaces the shared profile config for one transport.
	// ECH uses it to attach a host-specific ECHConfigList.
	tlsConfigOverride *tls.Config
}

type closeIdleRoundTripper interface {
	CloseIdleConnections()
}

type closeRoundTripper interface {
	Close() error
}

type h3FailureCache struct {
	ts    sync.Mutex
	until map[string]time.Time
}

type fallbackRoundTripper struct {
	h3    http.RoundTripper
	tcp   http.RoundTripper
	cache h3FailureCache
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

// Close releases a client built by NewClient. Unlike CloseIdleConnections it
// also closes transports that own sockets, such as the QUIC transport used when
// the HTTP protocol is auto or h3.
func Close(client *http.Client) {
	if client == nil {
		return
	}
	if closer, ok := client.Transport.(closeRoundTripper); ok {
		_ = closer.Close()
	}
	client.CloseIdleConnections()
}

func NewRoundTripper(opts Options) http.RoundTripper {
	protocol := opts.Protocol
	if protocol == "" {
		protocol = DefaultProtocolFromEnv()
	}
	var transport http.RoundTripper
	switch protocol {
	case ProtocolH1:
		transport = newTCPTransport(opts, false)
	case ProtocolH2:
		transport = newH2Transport(opts)
	case ProtocolH3:
		transport = newH3OnlyTransport(opts)
	case ProtocolTCP:
		transport = newTCPTransport(opts, true)
	default:
		if h3Unhealthy(time.Now()) {
			// H3 is known unreachable: skip the QUIC transport entirely instead of
			// paying a doomed handshake per probe.
			transport = newTCPTransport(opts, true)
		} else {
			transport = &fallbackRoundTripper{
				h3:    newH3OnlyTransport(opts),
				tcp:   newTCPTransport(opts, true),
				cache: h3FailureCache{until: make(map[string]time.Time)},
			}
		}
	}
	// ECH only wraps explicit TCP protocols. Auto and h3 keep their own
	// transports instead of being silently downgraded.
	if opts.ECHConfig != nil && protocol != ProtocolAuto && protocol != ProtocolH3 {
		transport = newECHRoundTripper(opts, transport)
	}
	return transport
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
	now := time.Now()
	if !canTryH3(req) || t.isH3OriginCached(req) || h3Unhealthy(now) {
		return t.tcp.RoundTrip(req)
	}
	res, err := t.h3.RoundTrip(req)
	if err == nil {
		// Deliberately does not lift the process-wide park: see defaultH3UnhealthyTTL.
		return res, nil
	}
	now = time.Now()
	t.rememberH3Failure(req, now)
	markH3Unhealthy(now)
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

// echRoundTripper attaches a host's ECHConfigList to its TLS handshakes. One
// transport per host is needed because an ECH config only applies to the name
// it was published for.
type echRoundTripper struct {
	opts     Options
	base     http.RoundTripper
	mu       sync.Mutex
	byHost   map[string]*http.Transport
	disabled map[string]bool
}

func newECHRoundTripper(opts Options, base http.RoundTripper) http.RoundTripper {
	return &echRoundTripper{
		opts:     opts,
		base:     base,
		byHost:   make(map[string]*http.Transport),
		disabled: make(map[string]bool),
	}
}

func (t *echRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	if req.URL.Scheme != "https" || host == "" {
		return t.base.RoundTrip(req)
	}
	t.mu.Lock()
	disabled := t.disabled[host]
	transport := t.byHost[host]
	t.mu.Unlock()
	if disabled {
		return t.base.RoundTrip(req)
	}
	if transport == nil {
		configList, err := t.opts.ECHConfig(req.Context(), host)
		if err != nil || len(configList) == 0 {
			return t.base.RoundTrip(req)
		}
		transport = t.transportForHost(host, configList)
	}
	res, err := transport.RoundTrip(req)
	if err != nil && echRetryable(err) {
		// The server rejected or ignored this config, or it rotated. Retry in
		// the clear once and stop offering ECH for this host.
		t.mu.Lock()
		t.disabled[host] = true
		if old := t.byHost[host]; old != nil {
			old.CloseIdleConnections()
			delete(t.byHost, host)
		}
		t.mu.Unlock()
		return t.base.RoundTrip(cloneRequestForRetry(req))
	}
	return res, err
}

// echRetryable reports whether an ECH attempt failed in a way a plain handshake
// might not. Servers that ignore or refuse the extension answer with a
// rejection, an alert, or the public-name certificate depending on their TLS
// stack, so all three count. Network failures are excluded: ECH is not the
// cause and an unreachable host must not be dialed twice.
func echRetryable(err error) bool {
	return asTLSFailure[*tls.ECHRejectionError](err) ||
		asTLSFailure[*tls.CertificateVerificationError](err) ||
		asTLSFailure[tls.AlertError](err) ||
		asTLSFailure[*tls.AlertError](err) ||
		asTLSFailure[tls.RecordHeaderError](err) ||
		asTLSFailure[*tls.RecordHeaderError](err)
}

func asTLSFailure[T error](err error) bool {
	var target T
	return errors.As(err, &target)
}

func (t *echRoundTripper) transportForHost(host string, configList []byte) *http.Transport {
	t.mu.Lock()
	defer t.mu.Unlock()
	if transport := t.byHost[host]; transport != nil {
		return transport
	}
	opts := t.opts
	opts.ECHConfig = nil
	tlsConfig := tlsConfigForProfile(opts.Profile, nil).Clone()
	// ECH configs rotate, so their sessions must not be shared with the plain
	// transport or with another host.
	tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(64)
	tlsConfig.EncryptedClientHelloConfigList = append([]byte(nil), configList...)
	// A rejected ECH attempt only reveals SNI, which the plain retry would
	// reveal anyway. Accept the rejection so the retry is deterministic even
	// for hosts whose public-name certificate is not in the root store.
	tlsConfig.EncryptedClientHelloRejectionVerify = func(tls.ConnectionState) error { return nil }
	opts.tlsConfigOverride = tlsConfig
	transport := newTCPTransport(opts, true)
	t.byHost[host] = transport
	return transport
}

func (t *echRoundTripper) CloseIdleConnections() {
	t.mu.Lock()
	transports := make([]*http.Transport, 0, len(t.byHost))
	for _, transport := range t.byHost {
		transports = append(transports, transport)
	}
	t.mu.Unlock()
	for _, transport := range transports {
		transport.CloseIdleConnections()
	}
	if closer, ok := t.base.(closeIdleRoundTripper); ok {
		closer.CloseIdleConnections()
	}
}

func (t *echRoundTripper) Close() error {
	t.CloseIdleConnections()
	t.mu.Lock()
	t.byHost = make(map[string]*http.Transport)
	t.mu.Unlock()
	if closer, ok := t.base.(closeRoundTripper); ok {
		return closer.Close()
	}
	return nil
}

func newTCPTransport(opts Options, forceHTTP2 bool) *http.Transport {
	// The TLS config is shared, so the ALPN set is selected here instead of
	// being mutated afterwards.
	nextProtos := []string{"http/1.1"}
	if forceHTTP2 {
		nextProtos = nil
	}
	tlsConfig := opts.tlsConfigOverride
	if tlsConfig == nil {
		tlsConfig = tlsConfigForProfile(opts.Profile, nextProtos)
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
		transport.Proxy = SystemProxy
	}
	return transport
}

func newH2Transport(opts Options) *schemeGuardTransport {
	tlsConfig := tlsConfigForProfile(opts.Profile, []string{"h2"})
	dialContext := dialContextForOptions(opts)
	tlsHandshakeTimeout := durationOrDefault(opts.TLSHandshakeTimeout, 10*time.Second)
	return &schemeGuardTransport{
		requiredScheme: "https",
		protocol:       ProtocolH2,
		next: &http.Transport{
			// http.Transport would otherwise broaden ALPN to h2,http/1.1 and silently
			// downgrade an explicit h2 request, so the handshake rejects non-h2 peers.
			DialTLSContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				conn, err := dialContext(ctx, network, address)
				if err != nil {
					return nil, err
				}
				handshakeCtx, cancel := context.WithTimeout(ctx, tlsHandshakeTimeout)
				defer cancel()
				tlsConn := tls.Client(conn, tlsConfig)
				if err := tlsConn.HandshakeContext(handshakeCtx); err != nil {
					_ = conn.Close()
					return nil, err
				}
				if tlsConn.ConnectionState().NegotiatedProtocol != "h2" {
					_ = conn.Close()
					return nil, errors.New("h2 requires an HTTP/2 server")
				}
				return tlsConn, nil
			},
			// Proxy is intentionally unset: the previous http2.Transport ignored
			// proxies too, so DisableProxy stays a no-op on this path.
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          1024,
			MaxIdleConnsPerHost:   256,
			IdleConnTimeout:       30 * time.Second,
			ResponseHeaderTimeout: opts.ResponseHeaderTimeout,
			ExpectContinueTimeout: 1 * time.Second,
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
	if opts.Resolver != nil {
		dialContext = resolverDialContext(dialContext, opts.Resolver)
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

// resolverDialContext resolves the host with lookup, then dials the resolved
// addresses in order. IPv4 is tried before IPv6 so a blackholed IPv6 path does
// not consume the whole dial budget before the working address is reached.
func resolverDialContext(next func(ctx context.Context, network, address string) (net.Conn, error), lookup func(ctx context.Context, host string) ([]net.IPAddr, error)) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || net.ParseIP(host) != nil {
			return next(ctx, network, address)
		}
		addrs, err := lookup(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(addrs) == 0 {
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
		var lastErr error
		for _, addr := range addrs {
			conn, err := next(ctx, network, net.JoinHostPort(addr.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

// tlsConfigForProfile returns a shared config per profile/ALPN pair. Sharing it
// keeps a warm session cache for the whole process instead of a fresh, empty cache
// per probe client. The session cache is per config, so a lenient profile cannot
// hand a resumed session to a verifying one.
func tlsConfigForProfile(profile httpcfg.Profile, nextProtos []string) *tls.Config {
	key := strconv.FormatBool(profile.InsecureSkipVerify) + "|" + profile.SNI + "|" + strings.Join(nextProtos, ",")
	tlsConfigCache.mu.Lock()
	defer tlsConfigCache.mu.Unlock()
	if cfg, ok := tlsConfigCache.configs[key]; ok {
		return cfg
	}
	cfg := &tls.Config{
		InsecureSkipVerify: profile.InsecureSkipVerify,
		ClientSessionCache: tls.NewLRUClientSessionCache(512),
	}
	if profile.HasCustomSNI() {
		cfg.ServerName = profile.SNI
	}
	if len(nextProtos) > 0 {
		cfg.NextProtos = nextProtos
	}
	if tlsConfigCache.configs == nil {
		tlsConfigCache.configs = make(map[string]*tls.Config)
	}
	tlsConfigCache.configs[key] = cfg
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

func (t *fallbackRoundTripper) isH3OriginCached(req *http.Request) bool {
	origin := h3Origin(req)
	if origin == "" {
		return false
	}
	now := time.Now()
	t.cache.ts.Lock()
	defer t.cache.ts.Unlock()
	until, ok := t.cache.until[origin]
	if !ok {
		return false
	}
	if now.After(until) {
		delete(t.cache.until, origin)
		return false
	}
	return true
}

func (t *fallbackRoundTripper) rememberH3Failure(req *http.Request, now time.Time) {
	origin := h3Origin(req)
	if origin == "" {
		return
	}
	t.cache.ts.Lock()
	if t.cache.until == nil {
		t.cache.until = make(map[string]time.Time)
	}
	t.cache.until[origin] = now.Add(defaultH3FailureTTL)
	t.cache.ts.Unlock()
}

func (t *fallbackRoundTripper) cleanupExpiredH3FailureCache(now time.Time) int {
	t.cache.ts.Lock()
	defer t.cache.ts.Unlock()
	removed := 0
	for origin, until := range t.cache.until {
		if now.After(until) {
			delete(t.cache.until, origin)
			removed++
		}
	}
	return removed
}
