//go:build windows

package httpclient

import (
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

type windowsProxySettings struct {
	enabled  bool
	server   string
	override string
}

// SystemProxy resolves the proxy from the current user's Windows Internet
// Settings, honoring the manual proxy and its bypass list, and falls back to
// the environment when Windows reports no usable proxy.
// ponytail: AutoConfigURL (PAC) is not evaluated; add a PAC resolver only if a
// deployment actually ships proxy auto-configuration without a manual server.
func SystemProxy(req *http.Request) (*url.URL, error) {
	if req == nil || req.URL == nil {
		return nil, nil
	}
	settings, err := readWindowsProxySettings()
	if err != nil {
		return http.ProxyFromEnvironment(req)
	}
	if !settings.enabled || strings.TrimSpace(settings.server) == "" {
		return http.ProxyFromEnvironment(req)
	}
	if proxyOverrideMatch(settings.override, req.URL.Hostname()) {
		return nil, nil
	}
	proxyURL := proxyURLForScheme(settings.server, req.URL.Scheme)
	if proxyURL == nil {
		return http.ProxyFromEnvironment(req)
	}
	return proxyURL, nil
}

func readWindowsProxySettings() (windowsProxySettings, error) {
	var settings windowsProxySettings
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return settings, err
	}
	defer key.Close()
	if enabled, _, err := key.GetIntegerValue("ProxyEnable"); err == nil {
		settings.enabled = enabled != 0
	}
	settings.server, _, _ = key.GetStringValue("ProxyServer")
	settings.override, _, _ = key.GetStringValue("ProxyOverride")
	return settings, nil
}

// proxyURLForScheme parses a ProxyServer value, which is either a bare
// "host:port" or a "scheme=host:port;..." list.
func proxyURLForScheme(server, scheme string) *url.URL {
	entries := strings.Split(server, ";")
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	first := ""
	for _, entry := range entries {
		name, value, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if !ok {
			if first == "" {
				first = strings.TrimSpace(entry)
			}
			continue
		}
		if strings.ToLower(strings.TrimSpace(name)) == scheme {
			return parseProxyURL(value)
		}
		if first == "" {
			first = strings.TrimSpace(value)
		}
	}
	if first == "" {
		return nil
	}
	return parseProxyURL(first)
}

func parseProxyURL(value string) *url.URL {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return nil
	}
	return parsed
}

// proxyOverrideMatch reports whether host is excluded by a ProxyOverride list.
// ponytail: "<local>" only matches single-label names; refine if intranet
// split-DNS deployments need it.
func proxyOverrideMatch(override, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, entry := range strings.Split(override, ";") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		switch {
		case entry == "":
		case entry == "<local>":
			if !strings.Contains(host, ".") {
				return true
			}
		case strings.HasPrefix(entry, "*."):
			if strings.HasSuffix(host, entry[1:]) {
				return true
			}
		case entry == host:
			return true
		}
	}
	return false
}
