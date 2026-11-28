package httpclient

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestValidateEndpointURL(t *testing.T) {
	validHTTPS := []string{
		"https://dns.alidns.com/dns-query",
		"https://1.1.1.1/dns-query",
		"https://127.0.0.1:8443/webhook",
		"https://192.168.1.20:5244/dav",
	}
	for _, raw := range validHTTPS {
		if _, err := ValidateEndpointURL(raw, false); err != nil {
			t.Fatalf("ValidateEndpointURL(%q, false) error = %v", raw, err)
		}
	}
	if _, err := ValidateEndpointURL("http://127.0.0.1:8080/hook", true); err != nil {
		t.Fatalf("ValidateEndpointURL(http loopback, true) error = %v", err)
	}

	invalid := []struct {
		raw       string
		allowHTTP bool
	}{
		{"", true},
		{"http://dns.example/dns-query", false},
		{"ftp://example.com/file", true},
		{"https://user:pass@example.com/dav", true},
		{"https://api.telegram.org@evil.example/bot", true},
		{"https://169.254.169.254/latest/meta-data/", true},
		{"http://[fe80::1]/meta", true},
		{"http://[fd00:ec2::254]/latest/meta-data/", true},
		{"http://0.0.0.0:8080/", true},
		{"http://metadata.google.internal/computeMetadata/v1/", true},
		{"https://example.com/a\rb", true},
		{"https://example.com/a b", true},
		{"https:///missing-host", true},
	}
	for _, tc := range invalid {
		if _, err := ValidateEndpointURL(tc.raw, tc.allowHTTP); err == nil {
			t.Fatalf("ValidateEndpointURL(%q, %v) = nil, want rejection", tc.raw, tc.allowHTTP)
		}
	}
}

func TestBlockLinkLocalRejectsDisallowedTargetBeforeDial(t *testing.T) {
	client := NewClient(Options{
		Protocol:       ProtocolTCP,
		DisableProxy:   true,
		BlockLinkLocal: true,
		Resolver: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
		},
	})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://rebind.example/latest", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(req)
	if err == nil || !strings.Contains(err.Error(), "拒绝访问") {
		t.Fatalf("Do() error = %v, want link-local block error", err)
	}
}
