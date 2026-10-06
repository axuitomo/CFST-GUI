//go:build windows

package httpclient

import (
	"net/http"
	"testing"
)

func TestProxyURLForScheme(t *testing.T) {
	cases := []struct {
		name   string
		server string
		scheme string
		want   string
	}{
		{"single", "127.0.0.1:7890", "https", "http://127.0.0.1:7890"},
		{"per-scheme", "http=127.0.0.1:7890;https=127.0.0.1:7891", "https", "http://127.0.0.1:7891"},
		{"unknown-scheme-falls-back", "socks=127.0.0.1:1080", "https", "http://127.0.0.1:1080"},
		{"empty", "  ", "https", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := proxyURLForScheme(tc.server, tc.scheme)
			if got == nil {
				if tc.want != "" {
					t.Fatalf("proxyURLForScheme(%q) = nil, want %q", tc.server, tc.want)
				}
				return
			}
			if got.String() != tc.want {
				t.Fatalf("proxyURLForScheme(%q) = %q, want %q", tc.server, got, tc.want)
			}
		})
	}
}

func TestProxyOverrideMatch(t *testing.T) {
	override := "<local>;*.example.com;api.telegram.org;skip.me"
	for host, want := range map[string]bool{
		"intranet":            true,
		"cdn.example.com":     true,
		"api.telegram.org":    true,
		"skip.me":             true,
		"api.telegram.org.cn": false,
		"example.org":         false,
	} {
		if got := proxyOverrideMatch(override, host); got != want {
			t.Errorf("proxyOverrideMatch(%q, %q) = %v, want %v", override, host, got, want)
		}
	}
}

func TestSystemProxyUsesEnvironmentWithoutWindowsProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:18080")
	req, err := http.NewRequest(http.MethodGet, "https://api.telegram.org/bot", nil)
	if err != nil {
		t.Fatal(err)
	}
	// A machine without a manual Windows proxy must still honor the environment.
	if settings, err := readWindowsProxySettings(); err == nil && settings.enabled && settings.server != "" {
		t.Skip("host has a manual Windows proxy enabled")
	}
	got, err := SystemProxy(req)
	if err != nil {
		t.Fatalf("SystemProxy: %v", err)
	}
	if got == nil || got.Host != "127.0.0.1:18080" {
		t.Fatalf("SystemProxy = %v, want environment proxy", got)
	}
}
