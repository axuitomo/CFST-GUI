package appcore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/archivecore"
	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

const (
	MaxSourceContentBytes  = archivecore.MaxConfigArchiveBytes
	MaxSourceHTTPBodyBytes = MaxSourceContentBytes
	MaxTaskResultsBytes    = MaxSourceContentBytes
)

type SourceHTTPClientOptions struct {
	UserAgent    string
	Timeout      time.Duration
	DisableProxy bool
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
	return FetchSourceURLContext(context.Background(), targetURL, cfg, client)
}

func FetchSourceURLContext(ctx context.Context, targetURL string, cfg probecore.ProbeConfig, client *http.Client) (string, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", 0, err
	}
	httpcfg.Resolve(cfg.UserAgent, "", "", "", true).Apply(req)
	if client == nil {
		client = NewSourceHTTPClient(cfg, SourceHTTPClientOptions{DisableProxy: true})
	}
	res, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_ = res.Body.Close()
		return "", res.StatusCode, fmt.Errorf("远程来源返回状态 %s", res.Status)
	}
	limited := io.LimitReader(res.Body, MaxSourceHTTPBodyBytes+1)
	raw, readErr := io.ReadAll(limited)
	_ = res.Body.Close()
	if readErr != nil {
		return "", 0, readErr
	}
	if int64(len(raw)) > MaxSourceHTTPBodyBytes {
		return "", res.StatusCode, fmt.Errorf("远程来源超过 %d 字节上限", MaxSourceHTTPBodyBytes)
	}
	return string(raw), res.StatusCode, nil
}
