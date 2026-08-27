package task

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
)

func (e *Engine) currentRequestProfile() httpcfg.Profile {
	return e.requestProfile(e.config.TraceURL, e.config.HostHeader, e.config.SNI)
}

func (e *Engine) downloadRequestProfile() httpcfg.Profile {
	hostHeader := strings.TrimSpace(e.config.DownloadHostHeader)
	sni := strings.TrimSpace(e.config.DownloadSNI)
	if sameRequestAuthority(e.config.URL, e.config.TraceURL) {
		if hostHeader == "" {
			hostHeader = e.config.HostHeader
		}
		if sni == "" {
			sni = e.config.SNI
		}
	}
	return e.requestProfile(e.config.URL, hostHeader, sni)
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
