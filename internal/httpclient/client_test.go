package httpclient

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func resetH3Health(t *testing.T) {
	t.Helper()
	h3UnhealthyUntil.Store(0)
	t.Cleanup(func() { h3UnhealthyUntil.Store(0) })
}

func TestFallbackRoundTripperCachesH3Failure(t *testing.T) {
	resetH3Health(t)
	var h3Calls atomic.Int32
	var tcpCalls atomic.Int32
	transport := &fallbackRoundTripper{
		h3: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			h3Calls.Add(1)
			return nil, errors.New("udp blocked")
		}),
		tcp: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			tcpCalls.Add(1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	for i := 0; i < 2; i++ {
		req, err := http.NewRequest(http.MethodGet, "https://example.test/data", nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
	}

	if h3Calls.Load() != 1 {
		t.Fatalf("h3 calls = %d, want cached after first failure", h3Calls.Load())
	}
	if tcpCalls.Load() != 2 {
		t.Fatalf("tcp calls = %d, want fallback for both requests", tcpCalls.Load())
	}
}

func TestFallbackRoundTripperSkipsH3ForUnsafeRequest(t *testing.T) {
	var h3Calls atomic.Int32
	var tcpCalls atomic.Int32
	transport := &fallbackRoundTripper{
		h3: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			h3Calls.Add(1)
			return nil, errors.New("should not be called")
		}),
		tcp: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			tcpCalls.Add(1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.example.test/write", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if h3Calls.Load() != 0 || tcpCalls.Load() != 1 {
		t.Fatalf("calls = h3 %d tcp %d, want only tcp", h3Calls.Load(), tcpCalls.Load())
	}
}

func TestNewH2TransportNegotiatesHTTP2Only(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Proto)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	client := &http.Client{Transport: NewRoundTripper(Options{
		Protocol: ProtocolH2,
		Profile:  httpcfg.Profile{InsecureSkipVerify: true},
	})}
	res, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("h2 request failed: %v", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(body); got != "HTTP/2.0" {
		t.Fatalf("server saw protocol %q, want HTTP/2.0", got)
	}
}

func TestNewH2TransportRejectsHTTP1OnlyServer(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	client := &http.Client{Transport: NewRoundTripper(Options{
		Protocol: ProtocolH2,
		Profile:  httpcfg.Profile{InsecureSkipVerify: true},
	})}
	if _, err := client.Get(srv.URL); err == nil {
		t.Fatal("HTTP/2-only transport unexpectedly succeeded against an HTTP/1.1-only server")
	} else if !strings.Contains(err.Error(), "no application protocol") {
		t.Fatalf("error = %v, want the server's ALPN rejection", err)
	}
}

// Go's TLS server answers a non-overlapping ALPN set with a no_application_protocol
// alert, so the case above is rejected during the handshake and never reaches the
// client-side guard. A server that leaves NextProtos empty completes the handshake
// with no protocol negotiated, which is the path that guard exists for.
func TestNewH2TransportRejectsServerWithoutALPN(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.TLS = &tls.Config{NextProtos: []string{}}
	srv.StartTLS()
	defer srv.Close()

	client := &http.Client{Transport: NewRoundTripper(Options{
		Protocol: ProtocolH2,
		Profile:  httpcfg.Profile{InsecureSkipVerify: true},
	})}
	if _, err := client.Get(srv.URL); err == nil || !strings.Contains(err.Error(), "h2 requires an HTTP/2 server") {
		t.Fatalf("error = %v, want the client-side ALPN guard", err)
	}
}

func TestH2TransportRejectsCleartextHTTP(t *testing.T) {
	client := &http.Client{Transport: NewRoundTripper(Options{Protocol: ProtocolH2})}
	if _, err := client.Get("http://example.test/"); err == nil || !strings.Contains(err.Error(), "h2 requires https") {
		t.Fatalf("cleartext error = %v, want scheme guard failure", err)
	}
}

// A peer that accepts the connection and then stays silent never completes the TLS
// handshake, so only the handshake deadline can end the request. Without it the
// request runs until Options.Timeout.
func TestH2TransportAppliesHandshakeTimeout(t *testing.T) {
	silent, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer silent.Close()
	go func() {
		for {
			conn, err := silent.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
		}
	}()

	client := &http.Client{Transport: NewRoundTripper(Options{
		Protocol:            ProtocolH2,
		Profile:             httpcfg.Profile{InsecureSkipVerify: true},
		TLSHandshakeTimeout: 150 * time.Millisecond,
		Timeout:             10 * time.Second,
	})}
	start := time.Now()
	if _, err := client.Get("https://" + silent.Addr().String() + "/"); err == nil {
		t.Fatal("handshake against a silent peer unexpectedly succeeded")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("handshake timeout not applied: request took %v", elapsed)
	}
}

func TestApplyNoCache(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.test/file", nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyNoCache(req)
	if req.Header.Get("Cache-Control") != "no-store" || req.Header.Get("Pragma") != "no-cache" {
		t.Fatalf("headers = %#v, want no-store/no-cache", req.Header)
	}
}

func okResponse(req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("ok")),
		Header:     make(http.Header),
		Request:    req,
	}
}

func TestH3FailureIsProcessWide(t *testing.T) {
	resetH3Health(t)

	var h3Calls atomic.Int32
	newTransport := func() *fallbackRoundTripper {
		return &fallbackRoundTripper{
			h3: roundTripFunc(func(*http.Request) (*http.Response, error) {
				h3Calls.Add(1)
				return nil, errors.New("udp blocked")
			}),
			tcp: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return okResponse(req), nil
			}),
		}
	}

	first, err := http.NewRequest(http.MethodGet, "https://203.0.113.7/data", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := newTransport().RoundTrip(first)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	// A later probe builds its own client and talks to a different IP; it must
	// not repeat the doomed QUIC attempt.
	second, err := http.NewRequest(http.MethodGet, "https://198.51.100.9/data", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = newTransport().RoundTrip(second)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	if h3Calls.Load() != 1 {
		t.Fatalf("h3 calls = %d, want 1 because the failure is process wide", h3Calls.Load())
	}
	if _, ok := NewRoundTripper(Options{Protocol: ProtocolAuto}).(*fallbackRoundTripper); ok {
		t.Fatal("auto protocol still builds an H3 transport while H3 is known unhealthy")
	}
}

func TestTLSConfigIsSharedButKeepsALPNVariantsSeparate(t *testing.T) {
	profile := httpcfg.Resolve("agent", "host.test", "sni.test", "", false)
	first := tlsConfigForProfile(profile, []string{"http/1.1"})
	second := tlsConfigForProfile(profile, []string{"http/1.1"})
	if first != second {
		t.Fatal("expected one shared tls config per profile and ALPN set")
	}
	if first.ClientSessionCache == nil {
		t.Fatal("expected a client session cache on the shared config")
	}
	if h3 := tlsConfigForProfile(profile, []string{"h3"}); h3 == first {
		t.Fatal("ALPN variants must not share one tls config")
	}
}

func TestTLSConfigDoesNotShareASessionCacheAcrossVerificationModes(t *testing.T) {
	strict := httpcfg.Resolve("agent", "host.test", "sni.test", "", false)
	lenient := httpcfg.Resolve("agent", "host.test", "sni.test", "", true)
	strictConfig := tlsConfigForProfile(strict, []string{"http/1.1"})
	lenientConfig := tlsConfigForProfile(lenient, []string{"http/1.1"})
	if strictConfig.ClientSessionCache == lenientConfig.ClientSessionCache {
		t.Fatal("a session built without verification must not be resumable by a verifying profile")
	}
}

func TestH3SuccessDoesNotLiftTheProcessWidePark(t *testing.T) {
	resetH3Health(t)
	okTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})

	failing := &fallbackRoundTripper{
		h3:    roundTripFunc(func(req *http.Request) (*http.Response, error) { return nil, errors.New("udp blocked") }),
		tcp:   okTransport,
		cache: h3FailureCache{until: make(map[string]time.Time)},
	}
	request := func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "https://example.test/data", nil)
		if err != nil {
			t.Fatal(err)
		}
		return req
	}
	res, err := failing.RoundTrip(request())
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if !h3Unhealthy(time.Now()) {
		t.Fatal("expected H3 to be parked after a failure")
	}

	// A probe that was already in flight when the failure landed can still
	// succeed; it must not reopen H3 for every new client.
	inFlight := &fallbackRoundTripper{h3: okTransport, tcp: okTransport}
	res, err = inFlight.RoundTrip(request())
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if !h3Unhealthy(time.Now()) {
		t.Fatal("an in-flight H3 success lifted the park and would restore the doomed handshake per probe")
	}
}

func TestResolverDialContextDialsResolvedAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "doh-dialed")
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	var lookedUp atomic.Value
	resolver := func(_ context.Context, host string) ([]net.IPAddr, error) {
		lookedUp.Store(host)
		// 127.0.0.2 has no listener, so the first dial fails and the second wins.
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.2")}, {IP: net.ParseIP("127.0.0.1")}}, nil
	}
	client := NewClient(Options{Protocol: ProtocolTCP, Resolver: resolver, DisableProxy: true})
	response, err := client.Get("http://source.test:" + port + "/")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "doh-dialed" {
		t.Fatalf("body = %q", body)
	}
	if got := lookedUp.Load(); got != "source.test" {
		t.Fatalf("lookup host = %v, want source.test", got)
	}

	// An IP literal must not trigger a lookup.
	lookedUp = atomic.Value{}
	if _, err := client.Get(server.URL); err != nil {
		t.Fatalf("Get(IP literal) error = %v", err)
	}
	if lookedUp.Load() != nil {
		t.Fatalf("lookup called for an IP literal: %v", lookedUp.Load())
	}
}

func TestECHRoundTripperNegotiatesECH(t *testing.T) {
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config := testECHConfig(key.PublicKey().Bytes(), "public.example")
	configList := testECHConfigList(config)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.TLS == nil || !request.TLS.ECHAccepted {
			t.Error("server received a request without ECH")
		}
		_, _ = io.WriteString(writer, "ech-ok")
	}))
	server.TLS = &tls.Config{
		MinVersion:               tls.VersionTLS13,
		Certificates:             []tls.Certificate{testSelfSignedCert(t, "secret.example", "public.example")},
		EncryptedClientHelloKeys: []tls.EncryptedClientHelloKey{{Config: config, PrivateKey: key.Bytes()}},
	}
	server.StartTLS()
	defer server.Close()

	var lookups atomic.Int32
	client := NewClient(Options{
		Protocol: ProtocolTCP,
		Profile:  httpcfg.Resolve("test", "", "", "", true),
		Resolver: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		},
		ECHConfig: func(_ context.Context, host string) ([]byte, error) {
			lookups.Add(1)
			if host != "secret.example" {
				return nil, nil
			}
			return configList, nil
		},
		DisableProxy: true,
	})
	response, err := client.Get("https://secret.example" + strings.TrimPrefix(server.URL, "https://127.0.0.1") + "/")
	if err != nil {
		t.Fatalf("GET with ECH error = %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "ech-ok" {
		t.Fatalf("body = %q", body)
	}
	if got := lookups.Load(); got != 1 {
		t.Fatalf("ECHConfig lookups = %d, want 1", got)
	}
}

func TestECHRoundTripperFallsBackWhenRejected(t *testing.T) {
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	configList := testECHConfigList(testECHConfig(key.PublicKey().Bytes(), "public.example"))

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "plain-ok")
	}))
	defer server.Close()

	var lookups atomic.Int32
	client := NewClient(Options{
		Protocol: ProtocolTCP,
		Profile:  httpcfg.Resolve("test", "", "", "", true),
		Resolver: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		},
		ECHConfig: func(context.Context, string) ([]byte, error) {
			lookups.Add(1)
			return configList, nil
		},
		DisableProxy: true,
	})
	target := "https://secret.example" + strings.TrimPrefix(server.URL, "https://127.0.0.1") + "/"
	for attempt := 0; attempt < 2; attempt++ {
		response, err := client.Get(target)
		if err != nil {
			t.Fatalf("attempt %d error = %v", attempt, err)
		}
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK || string(body) != "plain-ok" {
			t.Fatalf("attempt %d = %d %q", attempt, response.StatusCode, body)
		}
	}
	if got := lookups.Load(); got != 1 {
		t.Fatalf("ECHConfig lookups = %d, want 1 after the rejection is cached", got)
	}
}

func testECHConfig(publicKey []byte, publicName string) []byte {
	body := []byte{7}               // config_id
	body = append(body, 0x00, 0x20) // DHKEM(X25519, HKDF-SHA256)
	body = append(body, byte(len(publicKey)>>8), byte(len(publicKey)))
	body = append(body, publicKey...)
	body = append(body, 0x00, 0x04, 0x00, 0x01, 0x00, 0x01) // HKDF-SHA256 + AES-128-GCM
	body = append(body, 32)                                 // maximum_name_length
	body = append(body, byte(len(publicName)))
	body = append(body, publicName...)
	body = append(body, 0x00, 0x00) // extensions
	config := []byte{0xfe, 0x0d, byte(len(body) >> 8), byte(len(body))}
	return append(config, body...)
}

func testECHConfigList(config []byte) []byte {
	return append([]byte{byte(len(config) >> 8), byte(len(config))}, config...)
}

func testSelfSignedCert(t *testing.T, names ...string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: names[0]},
		DNSNames:              names,
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
