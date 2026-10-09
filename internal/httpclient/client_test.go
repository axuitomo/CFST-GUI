package httpclient

import (
	"crypto/tls"
	"errors"
	"io"
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

func TestFallbackRoundTripperCachesH3Failure(t *testing.T) {
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
