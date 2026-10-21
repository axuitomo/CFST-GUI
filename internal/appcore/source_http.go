package appcore

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/archivecore"
	"github.com/axuitomo/CFST-GUI/internal/doh"
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
	// 来源都是小文本，HTTP/3 没有收益，却可能让 QUIC 握手吃满整个抓取超时，
	// 使 TCP 回退拿到的 context 已经过期；来源抓取固定走 TCP。
	options := httpclient.Options{
		Protocol:     httpclient.ProtocolTCP,
		Profile:      profile,
		Timeout:      opts.Timeout,
		DisableProxy: opts.DisableProxy,
	}
	var resolver *doh.Resolver
	if endpoint := strings.TrimSpace(cfg.DoHURL); endpoint != "" {
		// 配置在归一化阶段已校验；这里仍然忽略非法值，回退到系统解析。
		if parsed, err := doh.New(endpoint); err == nil {
			resolver = parsed
			options.Resolver = parsed.LookupIPAddr
		}
	}
	// 未配置 DoH 时手填的 ECHConfigList 依然可用：ECH 只要求走 HTTPS，域名由
	// 系统 DNS 解析即可，只有自动获取才依赖 DoH 的 HTTPS RR。
	options.ECHConfig = echConfigProvider(cfg, resolver)
	return httpclient.NewClient(options)
}

// echConfigProvider 选出来源请求的 ECH 配置来源。手填的 ECHConfigList 优先，
// 因为这是 DoH 端点不发布 ech 参数时唯一的办法；否则用 DoH 的 HTTPS RR：
// 先按来源主机自己的记录取，主机没发布时回退到 ECH 域名（默认
// cloudflare-ech.com），这样只提供链式配置的站点也能用 ECH。
// ECH 关闭或两处都没有配置时返回 nil，即不启用 ECH。
func echConfigProvider(cfg probecore.ProbeConfig, resolver *doh.Resolver) func(context.Context, string) ([]byte, error) {
	if !cfg.ECH {
		return nil
	}
	if list := strings.TrimSpace(cfg.ECHConfigList); list != "" {
		// 归一化阶段已校验 base64；这里解码失败就按未配置处理。
		if decoded, err := base64.StdEncoding.DecodeString(list); err == nil && len(decoded) > 0 {
			// 同一份配置适用于所有来源主机，所以只解码一次。
			return func(context.Context, string) ([]byte, error) { return decoded, nil }
		}
	}
	if resolver == nil {
		return nil
	}
	fallbackDomain := probecore.NormalizeECHDomain(cfg.ECHDomain)
	if fallbackDomain == "" {
		fallbackDomain = probecore.DefaultECHDomain
	}
	// ECH 域名的配置对所有主机通用，取一次就够；主机记录每次任务只取一次，
	// 由 httpclient 的主机级 transport 缓存。
	var (
		fallbackOnce sync.Once
		fallbackList []byte
	)
	return func(ctx context.Context, host string) ([]byte, error) {
		if list, err := resolver.ECHConfig(ctx, host); err == nil && len(list) > 0 {
			return list, nil
		}
		if strings.EqualFold(fallbackDomain, host) {
			return nil, nil
		}
		fallbackOnce.Do(func() {
			fallbackList, _ = resolver.ECHConfig(ctx, fallbackDomain)
		})
		return fallbackList, nil
	}
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
