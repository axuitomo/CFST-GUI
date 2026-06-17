package appcore

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

type SourceHTTPClientOptions struct {
	UserAgent    string
	Timeout      time.Duration
	DisableProxy bool
}

type SourceURLFetchOptions struct {
	Cache SourceURLCache
}

type SourceURLFetchResult struct {
	Raw                  string
	ConditionalHit       bool
	PersistentCacheHit   bool
	PersistentCacheWrite bool
	StatusCode           int
}

func NewSourceHTTPClient(cfg probecore.ProbeConfig, opts SourceHTTPClientOptions) *http.Client {
	userAgent := strings.TrimSpace(opts.UserAgent)
	if userAgent == "" {
		userAgent = cfg.UserAgent
	}
	profile := httpcfg.Resolve(userAgent, "", "", "", true)
	return httpclient.NewClient(httpclient.Options{
		Profile:      profile,
		Timeout:      opts.Timeout,
		DisableProxy: opts.DisableProxy,
	})
}

func FetchSourceURL(targetURL string, cfg probecore.ProbeConfig, client *http.Client) (string, int, error) {
	result, err := FetchSourceURLWithOptions(targetURL, cfg, client, SourceURLFetchOptions{})
	return result.Raw, result.StatusCode, err
}

func FetchSourceURLWithOptions(targetURL string, cfg probecore.ProbeConfig, client *http.Client, opts SourceURLFetchOptions) (SourceURLFetchResult, error) {
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return SourceURLFetchResult{}, err
	}
	httpcfg.Resolve(cfg.UserAgent, "", "", "", true).Apply(req)
	cached, hasCached := SourceURLCacheEntry{}, false
	if opts.Cache != nil {
		cached, hasCached = opts.Cache.Get(targetURL)
		if hasCached {
			if value := strings.TrimSpace(cached.ETag); value != "" {
				req.Header.Set("If-None-Match", value)
			}
			if value := strings.TrimSpace(cached.LastModified); value != "" {
				req.Header.Set("If-Modified-Since", value)
			}
		}
	}
	if client == nil {
		client = NewSourceHTTPClient(cfg, SourceHTTPClientOptions{DisableProxy: true})
	}
	res, err := client.Do(req)
	if err != nil {
		return SourceURLFetchResult{}, err
	}
	if res.StatusCode == http.StatusNotModified && hasCached && strings.TrimSpace(cached.Raw) != "" {
		_ = res.Body.Close()
		return SourceURLFetchResult{
			Raw:                cached.Raw,
			ConditionalHit:     true,
			PersistentCacheHit: true,
			StatusCode:         res.StatusCode,
		}, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_ = res.Body.Close()
		return SourceURLFetchResult{StatusCode: res.StatusCode}, fmt.Errorf("远程来源返回状态 %s", res.Status)
	}
	raw, readErr := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if readErr != nil {
		return SourceURLFetchResult{}, readErr
	}
	result := SourceURLFetchResult{Raw: string(raw), StatusCode: res.StatusCode}
	if opts.Cache != nil {
		if err := opts.Cache.Put(SourceURLCacheEntry{
			ETag:         res.Header.Get("ETag"),
			LastModified: res.Header.Get("Last-Modified"),
			Raw:          result.Raw,
			StatusCode:   res.StatusCode,
			URL:          targetURL,
		}); err == nil {
			result.PersistentCacheWrite = true
		}
	}
	return result, nil
}
