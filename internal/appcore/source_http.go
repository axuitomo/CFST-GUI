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
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return "", 0, err
	}
	httpcfg.Resolve(cfg.UserAgent, "", "", "", true).Apply(req)
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_ = res.Body.Close()
		return "", res.StatusCode, fmt.Errorf("远程来源返回状态 %s", res.Status)
	}
	raw, readErr := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if readErr != nil {
		return "", 0, readErr
	}
	return string(raw), res.StatusCode, nil
}
