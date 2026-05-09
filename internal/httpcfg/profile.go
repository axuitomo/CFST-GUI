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
	RequestHeaders     []RequestHeader
}

type RequestHeader struct {
	Name  string
	Value string
}

func Resolve(userAgent, hostHeader, sni, captureAddress string, insecureSkipVerify bool) Profile {
	return ResolveWithHeaders(userAgent, hostHeader, sni, captureAddress, insecureSkipVerify, "")
}

func ResolveWithHeaders(userAgent, hostHeader, sni, captureAddress string, insecureSkipVerify bool, requestHeaders string) Profile {
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
		RequestHeaders:     ParseRequestHeaders(requestHeaders),
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
	for _, header := range p.RequestHeaders {
		req.Header.Set(header.Name, header.Value)
	}
	req.Header.Set("User-Agent", p.UserAgent)
	if p.HostHeader != "" {
		req.Host = p.HostHeader
	}
}

func NormalizeRequestHeaders(raw string) (string, []string) {
	headers, warnings := parseRequestHeaders(raw, true)
	if len(headers) == 0 {
		return "", warnings
	}
	lines := make([]string, 0, len(headers))
	for _, header := range headers {
		lines = append(lines, header.Name+": "+header.Value)
	}
	return strings.Join(lines, "\n"), warnings
}

func ParseRequestHeaders(raw string) []RequestHeader {
	headers, _ := parseRequestHeaders(raw, false)
	return headers
}

func RequestHeadersCount(raw string) int {
	return len(ParseRequestHeaders(raw))
}

func parseRequestHeaders(raw string, collectWarnings bool) ([]RequestHeader, []string) {
	var headers []RequestHeader
	var warnings []string
	warn := func(message string) {
		if collectWarnings {
			warnings = append(warnings, message)
		}
	}
	for index, line := range strings.Split(raw, "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		name, value, ok := strings.Cut(trimmed, ":")
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if !ok || name == "" || !isValidHeaderName(name) {
			warn("请求 Header 第 " + strconv.Itoa(index+1) + " 行格式无效，已忽略。")
			continue
		}
		if !isValidHeaderValue(value) {
			warn("请求 Header " + strconv.Quote(name) + " 包含非法控制字符，已忽略。")
			continue
		}
		canonicalName := http.CanonicalHeaderKey(name)
		if isReservedRequestHeader(canonicalName) {
			warn("请求 Header " + strconv.Quote(canonicalName) + " 为保留字段，已忽略。")
			continue
		}
		headers = append(headers, RequestHeader{Name: canonicalName, Value: value})
	}
	return headers, warnings
}

func isReservedRequestHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Host", "User-Agent", "Range", "Content-Length", "Connection", "Transfer-Encoding", "Accept-Encoding":
		return true
	default:
		return false
	}
}

func isValidHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !isHeaderTokenRune(r) {
			return false
		}
	}
	return true
}

func isHeaderTokenRune(r rune) bool {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
		return true
	}
	switch r {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	default:
		return false
	}
}

func isValidHeaderValue(value string) bool {
	for _, r := range value {
		if r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
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
