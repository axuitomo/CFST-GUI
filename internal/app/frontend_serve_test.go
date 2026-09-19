package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestFrontendCachePolicy(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{"root entry", "/", "no-cache"},
		{"html entry", "/index.html", "no-cache"},
		{"nested html", "/some/route.html", "no-cache"},
		{"hashed js asset", "/assets/index-BtndABzC.js", "public, max-age=31536000, immutable"},
		{"hashed css asset", "/assets/index-BUgz_WY2.css", "public, max-age=31536000, immutable"},
		{"wails runtime passthrough", "/wails/runtime.js", ""},
		{"api passthrough", "/api/health", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := frontendCachePolicy(tc.path); got != tc.want {
				t.Fatalf("frontendCachePolicy(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestVerifyEmbeddedFrontend(t *testing.T) {
	t.Run("nil assets", func(t *testing.T) {
		if err := verifyEmbeddedFrontend(nil); err == nil {
			t.Fatal("expected error for nil embedded assets, got nil")
		}
	})

	t.Run("only gitkeep placeholder", func(t *testing.T) {
		fsys := fstest.MapFS{
			embeddedDistRoot + "/.gitkeep": {},
		}
		if err := verifyEmbeddedFrontend(fsys); err == nil {
			t.Fatal("expected error when dist only contains .gitkeep, got nil")
		}
	})

	t.Run("built dist", func(t *testing.T) {
		fsys := fstest.MapFS{
			embeddedDistRoot + "/index.html":        {Data: []byte("<html></html>")},
			embeddedDistRoot + "/assets/index-x.js": {Data: []byte("{}")},
			embeddedDistRoot + "/.gitkeep":          {},
		}
		if err := verifyEmbeddedFrontend(fsys); err != nil {
			t.Fatalf("expected nil for a built dist, got %v", err)
		}
	})
}

func TestWithFrontendCacheHeaders(t *testing.T) {
	noop := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withFrontendCacheHeaders(noop)

	t.Run("entry gets no-cache", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("entry Cache-Control = %q, want no-cache", got)
		}
	})

	t.Run("asset gets immutable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/index-x.js", nil))
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Fatalf("asset Cache-Control = %q, want immutable policy", got)
		}
	})

	t.Run("other path untouched", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wails/runtime.js", nil))
		if got := rec.Header().Get("Cache-Control"); got != "" {
			t.Fatalf("passthrough Cache-Control = %q, want empty", got)
		}
	})
}
