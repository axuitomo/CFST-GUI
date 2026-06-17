package appcore

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
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

func TestLoadSourceContentMemoryCacheDedupesURLAndKeepsSourceWarning(t *testing.T) {
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
	opts := SourceContentLoadOptions{
		ContentCache: NewMemorySourceContentCache(),
		BuildAttempts: func(primaryURL string, source Source) []RemoteSourceAttempt {
			return []RemoteSourceAttempt{
				{URL: primaryURL},
				{URL: "https://cdn.jsdelivr.net/gh/HandsomeMJZ/cfip@main/best_ips.txt"},
			}
		},
		ShouldRetry: func(statusCode int, err error) bool {
			return err != nil && statusCode >= 500
		},
		OnFallbackSuccess: func(primaryURL string, used RemoteSourceAttempt, source Source) []string {
			return []string{"fallback for " + SourceName(source)}
		},
	}
	first, err := LoadSourceContent(Source{
		Kind: "url",
		Name: "first",
		URL:  "https://raw.githubusercontent.com/HandsomeMJZ/cfip/main/best_ips.txt",
	}, cfg, client, opts)
	if err != nil {
		t.Fatalf("LoadSourceContent first error = %v", err)
	}
	second, err := LoadSourceContent(Source{
		Kind: "url",
		Name: "second",
		URL:  "https://raw.githubusercontent.com/HandsomeMJZ/cfip/main/best_ips.txt",
	}, cfg, client, opts)
	if err != nil {
		t.Fatalf("LoadSourceContent second error = %v", err)
	}
	if !reflect.DeepEqual(hosts, []string{"raw.githubusercontent.com", "cdn.jsdelivr.net"}) {
		t.Fatalf("hosts = %#v, want one raw request and one fallback request", hosts)
	}
	if first.Diagnostics.CacheHit {
		t.Fatal("first Diagnostics.CacheHit = true, want false")
	}
	if !second.Diagnostics.CacheHit || second.Diagnostics.CacheKind != "url" {
		t.Fatalf("second Diagnostics = %#v, want url cache hit", second.Diagnostics)
	}
	if !reflect.DeepEqual(first.Warnings, []string{"fallback for first"}) {
		t.Fatalf("first warnings = %#v, want first source fallback warning", first.Warnings)
	}
	if !reflect.DeepEqual(second.Warnings, []string{"fallback for second"}) {
		t.Fatalf("second warnings = %#v, want second source fallback warning", second.Warnings)
	}
}

func TestLoadSourceContentMemoryCacheDedupesFileContent(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	file := filepath.Join(t.TempDir(), "ips.txt")
	if err := os.WriteFile(file, []byte("1.1.1.1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile initial: %v", err)
	}
	opts := SourceContentLoadOptions{ContentCache: NewMemorySourceContentCache()}
	first, err := LoadSourceContent(Source{Kind: "file", Path: file}, cfg, nil, opts)
	if err != nil {
		t.Fatalf("LoadSourceContent first error = %v", err)
	}
	if err := os.WriteFile(file, []byte("1.0.0.1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile update: %v", err)
	}
	second, err := LoadSourceContent(Source{Kind: "file", Path: file}, cfg, nil, opts)
	if err != nil {
		t.Fatalf("LoadSourceContent second error = %v", err)
	}
	if first.Raw != "1.1.1.1\n" || second.Raw != first.Raw {
		t.Fatalf("raw values = %q / %q, want cached first content", first.Raw, second.Raw)
	}
	if !second.Diagnostics.CacheHit || second.Diagnostics.CacheKind != "file" {
		t.Fatalf("second Diagnostics = %#v, want file cache hit", second.Diagnostics)
	}
}

func TestMemorySourceContentCacheReleasesWaitersAfterPanic(t *testing.T) {
	cache := NewMemorySourceContentCache()
	started := make(chan struct{})
	var calls atomic.Int32
	load := func() (SourceContentCacheValue, error) {
		if calls.Add(1) == 1 {
			close(started)
			time.Sleep(25 * time.Millisecond)
			panic("boom")
		}
		return SourceContentCacheValue{Raw: "unexpected"}, nil
	}

	firstDone := make(chan error, 1)
	go func() {
		_, _, err := cache.Load("url:https://example.com/ips.txt", load)
		firstDone <- err
	}()
	<-started

	secondDone := make(chan error, 1)
	go func() {
		_, hit, err := cache.Load("url:https://example.com/ips.txt", load)
		if !hit {
			secondDone <- errors.New("second cache load did not report hit")
			return
		}
		secondDone <- err
	}()

	for name, done := range map[string]chan error{"first": firstDone, "second": secondDone} {
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "输入源读取异常：boom") {
				t.Fatalf("%s err = %v, want panic converted to source read error", name, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s cache load did not finish after panic", name)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("load calls = %d, want one shared load", calls.Load())
	}
}

func TestFetchSourceURLWithOptionsUsesConditionalCache(t *testing.T) {
	var calls int
	var sawConditional bool
	cache := NewFileSourceURLCache(filepath.Join(t.TempDir(), "source-url-cache.json"))
	cfg := probecore.DefaultProbeConfig()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			header := make(http.Header)
			header.Set("ETag", `"v1"`)
			return &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("1.1.1.1\n")),
				Header:     header,
				Request:    req,
			}, nil
		}
		sawConditional = req.Header.Get("If-None-Match") == `"v1"`
		return &http.Response{
			Status:     "304 Not Modified",
			StatusCode: http.StatusNotModified,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	first, err := FetchSourceURLWithOptions("https://example.com/ips.txt", cfg, client, SourceURLFetchOptions{Cache: cache})
	if err != nil {
		t.Fatalf("FetchSourceURLWithOptions first error = %v", err)
	}
	if !first.PersistentCacheWrite {
		t.Fatalf("first = %#v, want persistent cache write", first)
	}
	second, err := FetchSourceURLWithOptions("https://example.com/ips.txt", cfg, client, SourceURLFetchOptions{Cache: cache})
	if err != nil {
		t.Fatalf("FetchSourceURLWithOptions second error = %v", err)
	}
	if !sawConditional {
		t.Fatal("second request did not send If-None-Match")
	}
	if second.Raw != "1.1.1.1\n" || !second.PersistentCacheHit || !second.ConditionalHit || second.StatusCode != http.StatusNotModified {
		t.Fatalf("second = %#v, want cached body from 304", second)
	}
}

func TestFileSourceURLCacheIgnoresCorruptCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source-url-cache.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile corrupt cache: %v", err)
	}
	cache := NewFileSourceURLCache(path)
	var calls int
	cfg := probecore.DefaultProbeConfig()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Header.Get("If-None-Match") != "" {
			t.Fatalf("If-None-Match = %q, want empty after corrupt cache fallback", req.Header.Get("If-None-Match"))
		}
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("1.1.1.1\n")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	result, err := FetchSourceURLWithOptions("https://example.com/ips.txt", cfg, client, SourceURLFetchOptions{Cache: cache})
	if err != nil {
		t.Fatalf("FetchSourceURLWithOptions error = %v", err)
	}
	if calls != 1 || result.Raw != "1.1.1.1\n" {
		t.Fatalf("calls = %d, result = %#v; want direct fetch after corrupt cache", calls, result)
	}
}

func TestNewSourceHTTPClientRespectsOptions(t *testing.T) {
	t.Setenv("CFST_HTTP_PROTOCOL", "tcp")
	client := NewSourceHTTPClient(probecore.DefaultProbeConfig(), SourceHTTPClientOptions{
		Timeout:      12 * time.Second,
		DisableProxy: true,
	})
	if client.Timeout != 12*time.Second {
		t.Fatalf("Timeout = %s, want 12s", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("Proxy is set, want nil when DisableProxy is true")
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
		MCISRunner: func(tokens []string, source Source, cfg probecore.ProbeConfig, limit int) ([]string, []string, error) {
			gotTokens = append([]string(nil), tokens...)
			gotLimit = limit
			return []string{"1.1.1.1"}, []string{"runner-warning"}, nil
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

func TestBuildMCISEngineConfigIgnoresFinalColoFilter(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	cfg.HttpingCFColo = "hkg,nrt LAX hkg zzz"

	mcisCfg := BuildMCISEngineConfig(cfg, 500)

	if len(mcisCfg.ColoAllow) != 0 {
		t.Fatalf("ColoAllow = %#v, want empty because final COLO filter belongs to stage 2 only", mcisCfg.ColoAllow)
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
