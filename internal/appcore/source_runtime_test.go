package appcore

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/doh"
	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"golang.org/x/net/dns/dnsmessage"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type resolverFunc func(ctx context.Context, host string) ([]net.IPAddr, error)

func (f resolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

func TestNormalizeSourceURLInput(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "bare host", raw: "bestcf.pages.dev/xinyitang3/ipv4.txt", want: "https://bestcf.pages.dev/xinyitang3/ipv4.txt"},
		{name: "protocol relative", raw: "//bestcf.pages.dev/xinyitang3/ipv4.txt", want: "https://bestcf.pages.dev/xinyitang3/ipv4.txt"},
		{name: "https", raw: "https://example.com/ips.txt", want: "https://example.com/ips.txt"},
		{name: "http", raw: "http://example.com/ips.txt", want: "http://example.com/ips.txt"},
		{name: "empty", raw: " ", wantErr: "缺少远程 URL"},
		{name: "unsupported scheme", raw: "ftp://example.com/ips.txt", wantErr: "仅支持 http/https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeSourceURLInput(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeSourceURLInput() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeSourceURLInput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFetchSourceURLAppliesUserAgent(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	cfg.UserAgent = "test-agent"

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("User-Agent"); got != "test-agent" {
			t.Fatalf("User-Agent = %q, want test-agent", got)
		}
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("1.1.1.1\n")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	raw, statusCode, err := FetchSourceURL("https://example.com/ips.txt", cfg, client)
	if err != nil {
		t.Fatalf("FetchSourceURL() error = %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("statusCode = %d, want 200", statusCode)
	}
	if raw != "1.1.1.1\n" {
		t.Fatalf("raw = %q, want response body", raw)
	}
}

func TestFetchSourceURLRejectsOversizedBody(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("1", int(MaxSourceHTTPBodyBytes)+2))),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	_, _, err := FetchSourceURL("https://example.com/ips.txt", cfg, client)
	if err == nil || !strings.Contains(err.Error(), "上限") {
		t.Fatalf("err = %v, want oversized body error", err)
	}
}

func TestLoadSourceContentFallsBackToLaterAttempt(t *testing.T) {
	var hosts []string
	cfg := probecore.DefaultProbeConfig()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Host)
		if req.URL.Host == "raw.githubusercontent.com" {
			return &http.Response{
				Status:     "500 Internal Server Error",
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("raw failed")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("1.1.1.1\n")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	result, err := LoadSourceContent(Source{
		Kind: "url",
		Name: "fallback",
		URL:  "https://raw.githubusercontent.com/HandsomeMJZ/cfip/main/best_ips.txt",
	}, cfg, client, SourceContentLoadOptions{
		BuildAttempts: func(primaryURL string, source Source) []RemoteSourceAttempt {
			return []RemoteSourceAttempt{
				{URL: primaryURL},
				{URL: "https://cdn.jsdelivr.net/gh/HandsomeMJZ/cfip@main/best_ips.txt"},
			}
		},
		ShouldRetry: func(statusCode int, err error) bool {
			return err != nil && (statusCode == http.StatusTooManyRequests || statusCode >= 500 || statusCode == 0)
		},
		OnFallbackSuccess: func(primaryURL string, used RemoteSourceAttempt, source Source) []string {
			return []string{"fallback-used"}
		},
	})
	if err != nil {
		t.Fatalf("LoadSourceContent() error = %v", err)
	}
	if !reflect.DeepEqual(hosts, []string{"raw.githubusercontent.com", "cdn.jsdelivr.net"}) {
		t.Fatalf("hosts = %#v, want raw then cdn", hosts)
	}
	if result.Raw != "1.1.1.1\n" {
		t.Fatalf("Raw = %q, want fallback body", result.Raw)
	}
	if !reflect.DeepEqual(result.Warnings, []string{"fallback-used"}) {
		t.Fatalf("Warnings = %#v, want fallback warning", result.Warnings)
	}
}

func TestLoadSourceContentStopsWhenRetryRejected(t *testing.T) {
	var calls int
	cfg := probecore.DefaultProbeConfig()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			Status:     "404 Not Found",
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("missing")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	_, err := LoadSourceContent(Source{
		Kind: "url",
		URL:  "https://example.com/missing.txt",
	}, cfg, client, SourceContentLoadOptions{
		BuildAttempts: func(primaryURL string, source Source) []RemoteSourceAttempt {
			return []RemoteSourceAttempt{{URL: primaryURL}, {URL: "https://fallback.example.com/missing.txt"}}
		},
		ShouldRetry: func(statusCode int, err error) bool {
			return false
		},
	})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want 404", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestLoadSourceContentRetriesTransientServerErrorOnce(t *testing.T) {
	var calls int
	cfg := probecore.DefaultProbeConfig()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		status := http.StatusServiceUnavailable
		body := "busy"
		if calls > 1 {
			status, body = http.StatusOK, "1.1.1.1\n"
		}
		return &http.Response{
			Status:     http.StatusText(status),
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	result, err := LoadSourceContent(Source{Kind: "url", URL: "https://example.com/list.txt"}, cfg, client, SourceContentLoadOptions{})
	if err != nil {
		t.Fatalf("LoadSourceContent() error = %v", err)
	}
	if result.Raw != "1.1.1.1\n" || calls != 2 {
		t.Fatalf("Raw = %q, calls = %d, want retried body after 2 calls", result.Raw, calls)
	}
}

func TestLoadSourceContentDoesNotRetryClientError(t *testing.T) {
	var calls int
	cfg := probecore.DefaultProbeConfig()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			Status:     "404 Not Found",
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("missing")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	if _, err := LoadSourceContent(Source{Kind: "url", URL: "https://example.com/missing.txt"}, cfg, client, SourceContentLoadOptions{}); err == nil {
		t.Fatal("LoadSourceContent() error = nil, want 404")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on 4xx)", calls)
	}
}
func TestNewSourceHTTPClientRespectsOptions(t *testing.T) {
	// 来源抓取固定走 TCP：即使环境变量要求 auto/h3，也不能创建 QUIC 传输。
	t.Setenv("CFST_HTTP_PROTOCOL", "h3")
	cfg := probecore.DefaultProbeConfig()
	cfg.ECH = false
	client := NewSourceHTTPClient(cfg, SourceHTTPClientOptions{Timeout: 12 * time.Second})
	if client.Timeout != 12*time.Second {
		t.Fatalf("Timeout = %s, want 12s", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T, want *http.Transport", client.Transport)
	}
	// IP 池阶段是唯一允许跟随系统代理的阶段。
	if transport.Proxy == nil {
		t.Fatal("Proxy is nil, want the system proxy for the source stage")
	}
}

func TestBuildSourceEntriesWithConfigUsesSharedRunner(t *testing.T) {
	var gotTokens []string
	var gotLimit int
	cfg := probecore.DefaultProbeConfig()

	result, err := BuildSourceEntriesWithConfig(SourceEntryBuildOptions{
		Raw:            "1.1.1.1\n1.0.0.1",
		Source:         Source{Name: "shared", IPLimit: 3, IPMode: "mcis"},
		Config:         cfg,
		DefaultIPLimit: 9,
		Resolver: resolverFunc(func(ctx context.Context, host string) ([]net.IPAddr, error) {
			return nil, errors.New("unexpected resolver call")
		}),
		MCISRunner: func(tokens []string, source Source, cfg probecore.ProbeConfig, limit int) ([]probecore.MCISCandidate, []string, error) {
			gotTokens = append([]string(nil), tokens...)
			gotLimit = limit
			return []probecore.MCISCandidate{{IP: "1.1.1.1", TotalMS: 10}}, []string{"runner-warning"}, nil
		},
	})
	if err != nil {
		t.Fatalf("BuildSourceEntriesWithConfig() error = %v", err)
	}
	if !reflect.DeepEqual(gotTokens, []string{"1.1.1.1", "1.0.0.1"}) {
		t.Fatalf("tokens = %#v, want parsed tokens", gotTokens)
	}
	if gotLimit != 3 {
		t.Fatalf("limit = %d, want source ip_limit 3", gotLimit)
	}
	if !reflect.DeepEqual(result.Entries, []string{"1.1.1.1"}) {
		t.Fatalf("entries = %#v, want runner result", result.Entries)
	}
	if !reflect.DeepEqual(result.Warnings, []string{"runner-warning"}) {
		t.Fatalf("warnings = %#v, want runner warning", result.Warnings)
	}
}

func TestBuildMCISEngineConfigMapsFinalColoFilter(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	cfg.HttpingCFColo = "hkg,nrt LAX hkg zzz"

	mcisCfg := BuildMCISEngineConfig(cfg, 500)

	want := []string{"HKG", "NRT", "LAX", "ZZZ"}
	if !reflect.DeepEqual(mcisCfg.ColoAllow, want) {
		t.Fatalf("ColoAllow = %#v, want %#v", mcisCfg.ColoAllow, want)
	}
}

func TestBuildMCISProbeConfigOnlySetsDebugDialAddressWhenConfigured(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	cfg.Debug = true
	cfg.DebugCaptureAddress = ""

	probeCfg, _ := BuildMCISProbeConfig(cfg)
	if probeCfg.DialAddress != "" {
		t.Fatalf("DialAddress = %q, want direct connection when debug capture address is empty", probeCfg.DialAddress)
	}

	cfg.DebugCaptureAddress = "9000"
	cfg.DebugCaptureEnabled = true
	probeCfg, _ = BuildMCISProbeConfig(cfg)
	if probeCfg.DialAddress != "127.0.0.1:9000" {
		t.Fatalf("DialAddress = %q, want normalized debug capture address", probeCfg.DialAddress)
	}

	cfg.DebugCaptureEnabled = false
	probeCfg, _ = BuildMCISProbeConfig(cfg)
	if probeCfg.DialAddress != "" {
		t.Fatalf("DialAddress = %q, want direct connection when debug capture is disabled", probeCfg.DialAddress)
	}
}

func TestFetchSourceURLUsesDefaultUserAgentWhenBlank(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	cfg.UserAgent = ""
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("User-Agent"); got != httpcfg.DefaultUserAgent {
			t.Fatalf("User-Agent = %q, want default", got)
		}
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	raw, _, err := FetchSourceURL("https://example.com/test.txt", cfg, client)
	if err != nil {
		t.Fatalf("FetchSourceURL() error = %v", err)
	}
	if raw != "ok" {
		t.Fatalf("raw = %q, want ok", raw)
	}
}

func TestLoadSourceContentReadsFile(t *testing.T) {
	file := t.TempDir() + "/ips.txt"
	if err := os.WriteFile(file, []byte("1.1.1.1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := LoadSourceContent(Source{Kind: "file", Path: file}, probecore.DefaultProbeConfig(), nil, SourceContentLoadOptions{})
	if err != nil {
		t.Fatalf("LoadSourceContent() error = %v", err)
	}
	if result.Raw != "1.1.1.1\n" {
		t.Fatalf("Raw = %q, want file body", result.Raw)
	}
}

func TestLoadSourceContentRejectsOversizedFile(t *testing.T) {
	file := t.TempDir() + "/ips.txt"
	if err := os.WriteFile(file, []byte("1.1.1.1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err := loadLocalSourceFile(context.Background(), file, 1)
	if err == nil || !strings.Contains(err.Error(), "上限") {
		t.Fatalf("err = %v, want oversized file error", err)
	}
}

func TestProcessProbeSourceUnlimitedIPLimitKeepsCandidates(t *testing.T) {
	service := NewService(ServiceOptions{ProbeConfigOptions: probecore.ConfigSnapshotOptions{DefaultSourceIPLimit: 500}})

	result, err := service.processProbeSource(context.Background(), probecore.DefaultProbeConfig(), Source{
		Enabled: true,
		IPLimit: -1,
		IPMode:  "traverse",
		Kind:    "inline",
		Content: "1.1.1.1\n2.2.2.2",
	}, nil, service.now(), "")
	if err != nil {
		t.Fatalf("processProbeSource() error = %v", err)
	}
	if got, want := strings.Join(result.Entries, ","), "1.1.1.1,2.2.2.2"; got != want {
		t.Fatalf("entries = %q, want %q", got, want)
	}
	if strings.Contains(strings.Join(result.Warnings, "\n"), "达到 IP 上限 0") {
		t.Fatalf("warnings = %#v, want no zero-limit truncation", result.Warnings)
	}
}

type staticResolver struct {
	addrs []net.IPAddr
	err   error
}

func (r staticResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return r.addrs, r.err
}

func TestSourceResolverUsesDoHWhenConfigured(t *testing.T) {
	fallback := &staticResolver{addrs: []net.IPAddr{{IP: net.ParseIP("198.51.100.1")}}}
	service := NewService(ServiceOptions{SourceResolver: fallback})

	if _, ok := service.sourceResolver(probecore.ProbeConfig{DoHURL: "https://dns.alidns.com/dns-query"}).(*doh.Resolver); !ok {
		t.Fatal("a valid DoH endpoint must replace the injected resolver")
	}
	for _, cfg := range []probecore.ProbeConfig{{}, {DoHURL: "http://dns.example/dns-query"}} {
		if got := service.sourceResolver(cfg); got != fallback {
			t.Fatalf("sourceResolver(%q) = %T, want the fallback resolver", cfg.DoHURL, got)
		}
	}
}

func TestECHConfigProviderFallsBackToECHDomain(t *testing.T) {
	perHost := []byte{0xfe, 0x0d, 0x00, 0x02, 0xaa, 0xbb}
	shared := []byte{0xfe, 0x0d, 0x00, 0x02, 0xcc, 0xdd}
	queries := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		var parser dnsmessage.Parser
		if _, err := parser.Start(body); err != nil {
			t.Errorf("parse query: %v", err)
		}
		questions, err := parser.AllQuestions()
		if err != nil || len(questions) != 1 {
			t.Errorf("questions = %v, %v", questions, err)
		}
		question := questions[0]
		queries[question.Name.String()]++
		answer := dnsmessage.Message{Header: dnsmessage.Header{Response: true}, Questions: questions}
		list, ok := map[string][]byte{
			"edge.example.":       perHost,
			"cloudflare-ech.com.": shared,
		}[question.Name.String()]
		if ok && question.Type == dnsmessage.TypeHTTPS {
			answer.Answers = []dnsmessage.Resource{{
				Header: dnsmessage.ResourceHeader{Name: question.Name, Type: dnsmessage.TypeHTTPS, Class: dnsmessage.ClassINET, TTL: 60},
				Body: &dnsmessage.HTTPSResource{SVCBResource: dnsmessage.SVCBResource{
					Priority: 1,
					Target:   question.Name,
					Params:   []dnsmessage.SVCParam{{Key: dnsmessage.SVCParamECH, Value: list}},
				}},
			}}
		}
		packed, packErr := answer.Pack()
		if packErr != nil {
			t.Errorf("pack answer: %v", packErr)
		}
		writer.Header().Set("Content-Type", "application/dns-message")
		_, _ = writer.Write(packed)
	}))
	defer server.Close()

	provider := echConfigProvider(probecore.DefaultProbeConfig(), &doh.Resolver{Endpoint: server.URL})
	if provider == nil {
		t.Fatal("a DoH resolver must enable ECH")
	}
	if got, err := provider(context.Background(), "edge.example"); err != nil || !bytes.Equal(got, perHost) {
		t.Fatalf("provider(edge.example) = %x, %v, want the host record", got, err)
	}
	for _, host := range []string{"plain.example", "other.example"} {
		if got, err := provider(context.Background(), host); err != nil || !bytes.Equal(got, shared) {
			t.Fatalf("provider(%s) = %x, %v, want the ECH domain record", host, got, err)
		}
	}
	if got := queries["cloudflare-ech.com."]; got != 1 {
		t.Fatalf("ECH domain queries = %d, want the shared config cached after one lookup", got)
	}
}

func TestECHConfigProviderPrefersManualList(t *testing.T) {
	enabled := probecore.DefaultProbeConfig()
	if echConfigProvider(enabled, nil) != nil {
		t.Fatal("no DoH and no manual list must leave ECH off")
	}

	disabled := enabled
	disabled.ECH = false
	disabled.ECHConfigList = base64.StdEncoding.EncodeToString([]byte{0xfe, 0x0d})
	if echConfigProvider(disabled, nil) != nil {
		t.Fatal("ECH = false must disable ECH even with a manual list")
	}

	manual := enabled
	manual.ECHConfigList = base64.StdEncoding.EncodeToString([]byte{0xfe, 0x0d, 0x00, 0x04})
	provider := echConfigProvider(manual, nil)
	if provider == nil {
		t.Fatal("a manual ECHConfigList must enable ECH without DoH")
	}
	for _, host := range []string{"a.example", "b.example"} {
		got, err := provider(context.Background(), host)
		if err != nil {
			t.Fatalf("provider(%q) error = %v", host, err)
		}
		if want := []byte{0xfe, 0x0d, 0x00, 0x04}; !bytes.Equal(got, want) {
			t.Fatalf("provider(%q) = %x, want %x", host, got, want)
		}
	}

	if echConfigProvider(enabled, &doh.Resolver{Endpoint: "https://dns.example/dns-query"}) == nil {
		t.Fatal("a DoH resolver must start the automatic HTTPS RR lookup")
	}

	invalid := enabled
	invalid.ECHConfigList = "not base64"
	if echConfigProvider(invalid, nil) != nil {
		t.Fatal("an undecodable manual list must leave ECH off")
	}
}
