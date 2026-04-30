package httpcfg

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0"

type Profile struct {
	UserAgent          string
	HostHeader         string
	SNI                string
	CaptureAddress     string
	InsecureSkipVerify bool
}

func Resolve(userAgent, hostHeader, sni, captureAddress string, insecureSkipVerify bool) Profile {
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	hostHeader = strings.TrimSpace(hostHeader)
	sni = strings.TrimSpace(sni)
	if sni == "" && hostHeader != "" {
		sni = hostHeader
	}

	return Profile{
		UserAgent:          userAgent,
		HostHeader:         hostHeader,
		SNI:                sni,
		CaptureAddress:     normalizeCaptureAddress(captureAddress),
		InsecureSkipVerify: insecureSkipVerify,
	}
}

func URLHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Hostname())
}

func (p Profile) Apply(req *http.Request) {
	req.Header.Set("User-Agent", p.UserAgent)
	if p.HostHeader != "" {
		req.Host = p.HostHeader
	}
}

func (p Profile) DialAddress(ip *net.IPAddr, port int) string {
	if p.CaptureAddress != "" {
		return p.CaptureAddress
	}
	if ip == nil {
		return ""
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(port))
}

func (p Profile) HasCustomHostHeader() bool {
	return p.HostHeader != ""
}

func (p Profile) HasCustomSNI() bool {
	return p.SNI != ""
}

func normalizeCaptureAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if isDigits(value) {
		return net.JoinHostPort("127.0.0.1", value)
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return value
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
