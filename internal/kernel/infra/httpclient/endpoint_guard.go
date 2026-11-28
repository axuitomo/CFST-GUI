package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"syscall"
)

var awsIPv6MetadataIP = net.ParseIP("fd00:ec2::254")

// ValidateEndpointURL validates a user-configured HTTP(S) endpoint URL before
// issuing outbound requests. It rejects control characters, embedded userinfo,
// non-HTTP(S) schemes (or plain HTTP when allowHTTP is false), empty hosts,
// and cloud metadata / link-local / unspecified IP literals.
func ValidateEndpointURL(rawURL string, allowHTTP bool) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, errors.New("请求地址不能为空")
	}
	if hasControlOrWhitespace(trimmed) {
		return nil, errors.New("请求地址不能包含空白或控制字符")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("请求地址无效：%w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && (!allowHTTP || scheme != "http") {
		if allowHTTP {
			return nil, errors.New("请求地址必须以 http:// 或 https:// 开头")
		}
		return nil, errors.New("请求地址必须以 https:// 开头")
	}
	if parsed.User != nil {
		return nil, errors.New("请求地址不能包含内嵌凭据")
	}
	host := parsed.Hostname()
	if host == "" {
		return nil, errors.New("请求地址缺少有效主机名")
	}
	if IsDisallowedEndpointHost(host) {
		return nil, fmt.Errorf("请求地址不允许访问受限主机或链路本地地址：%s", host)
	}
	parsed.Scheme = scheme
	return parsed, nil
}

// IsDisallowedEndpointHost reports whether host is a known cloud-metadata
// hostname or a disallowed IP literal (link-local, unspecified, or multicast).
func IsDisallowedEndpointHost(host string) bool {
	trimmed := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if trimmed == "" {
		return true
	}
	switch trimmed {
	case "metadata.google.internal", "metadata.goog", "metadata.azure.internal", "instance-data":
		return true
	}
	if ip := net.ParseIP(trimmed); ip != nil {
		return IsDisallowedEndpointIP(ip)
	}
	return false
}

// IsDisallowedEndpointIP blocks link-local (including 169.254.169.254 metadata),
// AWS IPv6 metadata, unspecified, and multicast targets while keeping loopback
// and LAN addresses usable for local services and tests.
func IsDisallowedEndpointIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	return awsIPv6MetadataIP != nil && ip.Equal(awsIPv6MetadataIP)
}

func hasControlOrWhitespace(value string) bool {
	for _, r := range value {
		if r <= 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func blockDisallowedEndpointControl(_ string, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	if ip := net.ParseIP(host); ip != nil && IsDisallowedEndpointIP(ip) {
		return fmt.Errorf("拒绝访问受限或链路本地地址：%s", host)
	}
	return nil
}

func wrapBlockLinkLocalDialContext(next func(ctx context.Context, network, address string) (net.Conn, error)) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err == nil && IsDisallowedEndpointHost(host) {
			return nil, fmt.Errorf("拒绝访问受限或链路本地地址：%s", host)
		}
		return next(ctx, network, address)
	}
}
