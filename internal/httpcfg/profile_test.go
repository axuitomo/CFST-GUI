package httpcfg

import (
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeRequestHeadersFiltersInvalidAndReserved(t *testing.T) {
	normalized, warnings := NormalizeRequestHeaders(strings.Join([]string{
		"Accept: */*",
		"accept-language: zh-CN",
		"Host: example.com",
		"User-Agent: bad",
		"Range: bytes=0-1",
		"Bad Header: nope",
		"X-Unsafe: ok\rbad",
		"X-Test: value",
	}, "\n"))

	want := strings.Join([]string{
		"Accept: */*",
		"Accept-Language: zh-CN",
		"X-Test: value",
	}, "\n")
	if normalized != want {
		t.Fatalf("normalized headers = %q, want %q", normalized, want)
	}
	if len(warnings) != 5 {
		t.Fatalf("warnings = %#v, want 5 invalid/reserved warnings", warnings)
	}
}

func TestProfileApplyUsesCustomHeadersAndDedicatedFieldsWin(t *testing.T) {
	profile := ResolveWithHeaders(
		"CFST-Test-UA",
		"origin.example",
		"",
		"",
		true,
		"Accept: text/plain\nUser-Agent: ignored\nX-Test: ok",
	)
	req, err := http.NewRequest(http.MethodGet, "https://example.test/path", nil)
	if err != nil {
		t.Fatal(err)
	}

	profile.Apply(req)

	if got := req.Header.Get("Accept"); got != "text/plain" {
		t.Fatalf("Accept = %q, want text/plain", got)
	}
	if got := req.Header.Get("X-Test"); got != "ok" {
		t.Fatalf("X-Test = %q, want ok", got)
	}
	if got := req.Header.Get("User-Agent"); got != "CFST-Test-UA" {
		t.Fatalf("User-Agent = %q, want dedicated UA", got)
	}
	if req.Host != "origin.example" {
		t.Fatalf("Host = %q, want origin.example", req.Host)
	}
}
