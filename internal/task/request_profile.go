package task

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
)

func (e *Engine) currentRequestProfile() httpcfg.Profile {
	hostHeader, sni := e.customAuthority(e.config.HostHeader, e.config.SNI)
	return e.requestProfile(e.config.TraceURL, hostHeader, sni)
}

func (e *Engine) downloadRequestProfile() httpcfg.Profile {
	hostHeader, sni := e.customAuthority(e.config.DownloadHostHeader, e.config.DownloadSNI)
	if sameRequestAuthority(e.config.URL, e.config.TraceURL) {
		genericHostHeader, genericSNI := e.customAuthority(e.config.HostHeader, e.config.SNI)
		if hostHeader == "" {
			hostHeader = genericHostHeader
		}
		if sni == "" {
			sni = genericSNI
		}
	}
	return e.requestProfile(e.config.URL, hostHeader, sni)
}

// customAuthority 返回生效的自定义 Host/SNI。自定义功能默认关闭：关闭时忽略填入的值，
// 由 requestProfile 回落成 URL 自己的域名，行为和只填 URL 完全一致。
func (e *Engine) customAuthority(hostHeader, sni string) (string, string) {
	if !e.config.CustomHostSNI {
		return "", ""
	}
	return strings.TrimSpace(hostHeader), strings.TrimSpace(sni)
}

func (e *Engine) requestProfile(rawURL, hostHeader, sni string) httpcfg.Profile {
	captureAddress := ""
	if e.config.Debug {
		captureAddress = e.config.CaptureAddress
	}
	hostHeader = strings.TrimSpace(hostHeader)
	sni = strings.TrimSpace(sni)
	if hostHeader == "" {
		hostHeader = httpcfg.URLHostHeader(rawURL)
	}
	if sni == "" {
		sni = httpcfg.URLHost(rawURL)
	}
	return httpcfg.ResolveWithHeaders(e.config.UserAgent, hostHeader, sni, captureAddress, e.config.InsecureSkipVerify, e.config.RequestHeaders)
}

func sameRequestAuthority(firstURL, secondURL string) bool {
	_, firstHost, firstPort, firstOK := requestURLIdentity(firstURL)
	_, secondHost, secondPort, secondOK := requestURLIdentity(secondURL)
	return firstOK && secondOK && strings.EqualFold(firstHost, secondHost) && firstPort == secondPort
}

func sameRequestOrigin(firstURL, secondURL string) bool {
	firstScheme, _, _, firstOK := requestURLIdentity(firstURL)
	secondScheme, _, _, secondOK := requestURLIdentity(secondURL)
	return firstOK && secondOK && strings.EqualFold(firstScheme, secondScheme) && sameRequestAuthority(firstURL, secondURL)
}

func requestURLIdentity(rawURL string) (scheme, host, port string, ok bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return "", "", "", false
	}
	scheme = strings.ToLower(parsed.Scheme)
	host = parsed.Hostname()
	port = parsed.Port()
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	return scheme, host, port, true
}

func (e *Engine) getDialContext(ip *net.IPAddr, profile httpcfg.Profile) func(ctx context.Context, network, address string) (net.Conn, error) {
	dialAddress := profile.DialAddress(ip, e.config.TCPPort)
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, dialAddress)
	}
}
