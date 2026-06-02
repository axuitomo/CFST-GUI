package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShouldRunWebUIHealthcheck(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"--healthcheck"}, want: true},
		{args: []string{"--cli", "--healthcheck"}, want: false},
		{args: []string{"--healthcheck", "--verbose"}, want: false},
		{args: nil, want: false},
	}
	for _, tc := range tests {
		if got := shouldRunWebUIHealthcheck(tc.args); got != tc.want {
			t.Fatalf("shouldRunWebUIHealthcheck(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestWebUIHealthcheckURL(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "default", addr: "", want: "http://127.0.0.1:34115/api/health"},
		{name: "wildcard", addr: "0.0.0.0:34115", want: "http://127.0.0.1:34115/api/health"},
		{name: "wildcard with scheme", addr: "http://0.0.0.0:34115", want: "http://127.0.0.1:34115/api/health"},
		{name: "port only", addr: ":34115", want: "http://127.0.0.1:34115/api/health"},
		{name: "loopback", addr: "127.0.0.1:34116", want: "http://127.0.0.1:34116/api/health"},
		{name: "hostname", addr: "localhost:34115", want: "http://localhost:34115/api/health"},
		{name: "ipv6 wildcard", addr: "[::]:34115", want: "http://127.0.0.1:34115/api/health"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := webUIHealthcheckURL(tc.addr); got != tc.want {
				t.Fatalf("webUIHealthcheckURL(%q) = %q, want %q", tc.addr, got, tc.want)
			}
		})
	}
}

func TestRunWebUIHealthcheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	if got := runWebUIHealthcheck(context.Background(), addr, server.Client()); got != 0 {
		t.Fatalf("runWebUIHealthcheck() = %d, want 0", got)
	}
}

func TestRunWebUIHealthcheckFailsOnNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	if got := runWebUIHealthcheck(context.Background(), addr, server.Client()); got == 0 {
		t.Fatalf("runWebUIHealthcheck() = %d, want non-zero", got)
	}
}
