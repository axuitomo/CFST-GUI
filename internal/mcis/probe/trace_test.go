package probe

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

var testProbeIP = netip.MustParseAddr("198.51.100.10")

func newTLSTestProber(t *testing.T, handler http.Handler, rounds, skipFirst int) *Prober {
	t.Helper()
	t.Setenv("CFST_HTTP_PROTOCOL", "h1")

	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	return NewProber(Config{
		Timeout:            2 * time.Second,
		SNI:                "trace.example.com",
		HostHeader:         "trace.example.com",
		DialAddress:        strings.TrimPrefix(server.URL, "https://"),
		InsecureSkipVerify: true,
		Rounds:             rounds,
		SkipFirst:          skipFirst,
	})
}

func TestProbeHTTPTraceUsesGETAndParsesTrace(t *testing.T) {
	var method, host, path string
	prober := newTLSTestProber(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		host = r.Host
		path = r.URL.Path
		_, _ = io.WriteString(w, "fl=29f42\r\ncolo=SJC\r\ninvalid\r\n")
	}), 1, 0)

	result := prober.ProbeHTTPTrace(context.Background(), testProbeIP)
	if !result.OK || result.Status != http.StatusOK {
		t.Fatalf("ProbeHTTPTrace() OK/status = %v/%d, want true/%d: %s", result.OK, result.Status, http.StatusOK, result.Error)
	}
	if method != http.MethodGet {
		t.Fatalf("request method = %q, want GET", method)
	}
	if host != "trace.example.com" {
		t.Fatalf("request host = %q, want trace.example.com", host)
	}
	if path != "/cdn-cgi/trace" {
		t.Fatalf("request path = %q, want /cdn-cgi/trace", path)
	}
	if result.Trace["colo"] != "SJC" || result.Trace["fl"] != "29f42" {
		t.Fatalf("trace = %#v, want parsed colo and fl", result.Trace)
	}
}

func TestProbeHTTPTraceLimitsParsedBody(t *testing.T) {
	prober := newTLSTestProber(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "colo=SJC\n")
		_, _ = io.WriteString(w, strings.Repeat("x", int(maxTraceBodyBytes)))
		_, _ = io.WriteString(w, "\ncolo=HKG\n")
	}), 1, 0)

	result := prober.ProbeHTTPTrace(context.Background(), testProbeIP)
	if !result.OK {
		t.Fatalf("ProbeHTTPTrace() failed: %s", result.Error)
	}
	if result.Trace["colo"] != "SJC" {
		t.Fatalf("trace colo = %q, want SJC from bounded parse window", result.Trace["colo"])
	}
}

func TestProbeHTTPTraceRejectsNon2xx(t *testing.T) {
	prober := newTLSTestProber(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "colo=SJC\n")
	}), 1, 0)

	result := prober.ProbeHTTPTrace(context.Background(), testProbeIP)
	if result.OK || result.Status != http.StatusNotFound {
		t.Fatalf("ProbeHTTPTrace() OK/status = %v/%d, want false/%d", result.OK, result.Status, http.StatusNotFound)
	}
	if result.Error != "http_status_404" {
		t.Fatalf("error = %q, want http_status_404", result.Error)
	}
}

func TestProbeHTTPTraceHonorsCertificateVerification(t *testing.T) {
	t.Setenv("CFST_HTTP_PROTOCOL", "h1")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "colo=SJC\n")
	}))
	t.Cleanup(server.Close)

	prober := NewProber(Config{
		Timeout:            2 * time.Second,
		SNI:                "trace.example.com",
		HostHeader:         "trace.example.com",
		DialAddress:        strings.TrimPrefix(server.URL, "https://"),
		InsecureSkipVerify: false,
	})
	result := prober.ProbeHTTPTrace(context.Background(), testProbeIP)
	if result.OK {
		t.Fatal("ProbeHTTPTrace() succeeded with an untrusted certificate")
	}
	if !strings.Contains(strings.ToLower(result.Error), "certificate") {
		t.Fatalf("error = %q, want certificate verification failure", result.Error)
	}
}

func TestProbeHTTPTraceMultiUsesLastSuccessfulTrace(t *testing.T) {
	colos := []string{"LAX", "SJC", "HKG"}
	requestCount := 0
	prober := newTLSTestProber(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		colo := colos[requestCount]
		requestCount++
		_, _ = io.WriteString(w, "colo="+colo+"\n")
	}), len(colos), 1)

	result := prober.ProbeHTTPTraceMulti(context.Background(), testProbeIP)
	if !result.OK {
		t.Fatalf("ProbeHTTPTraceMulti() failed: %s", result.Error)
	}
	if requestCount != len(colos) {
		t.Fatalf("request count = %d, want %d", requestCount, len(colos))
	}
	if result.Trace["colo"] != "HKG" {
		t.Fatalf("trace colo = %q, want HKG from last successful round", result.Trace["colo"])
	}
}
