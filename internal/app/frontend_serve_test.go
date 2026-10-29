package app

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

func TestAcceptsGzip(t *testing.T) {
	cases := []struct {
		header string
		want   bool
	}{
		{"", false},
		{"identity", false},
		{"gzip", true},
		{"gzip, deflate, br", true},
		{"br;q=1.0, gzip;q=0.8", true},
		{"gzip;q=0", false},
		{"gzip;q=0.0, deflate", false},
		{"*", false},
	}
	for _, tc := range cases {
		if got := acceptsGzip(tc.header); got != tc.want {
			t.Fatalf("acceptsGzip(%q) = %v, want %v", tc.header, got, tc.want)
		}
	}
}

func TestFrontendCompressible(t *testing.T) {
	for _, path := range []string{"/assets/app-abc.js", "/assets/app.css", "/index.html", "/logo.svg"} {
		if !frontendCompressible(path) {
			t.Fatalf("frontendCompressible(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"/assets/icon.png", "/assets/font.woff2", "/api/health", "/"} {
		if frontendCompressible(path) {
			t.Fatalf("frontendCompressible(%q) = true, want false", path)
		}
	}
}

// 中间件本身的行为：压缩后必须能解回原文，Content-Length 必须消失。
// 真实 SPA handler 的联调用例在 webui_test.go（webui 构建标签下）。
func TestWithFrontendCompression(t *testing.T) {
	body := strings.Repeat("console.log(1);", 200)
	save := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = io.WriteString(w, body)
	}
	handler := withFrontendCompression(http.HandlerFunc(save))

	t.Run("gzip accepted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app-x.js", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("Content-Encoding = %q, want gzip", got)
		}
		if got := rec.Header().Get("Content-Length"); got != "" {
			t.Fatalf("Content-Length = %q, want removed", got)
		}
		if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
			t.Fatalf("Vary = %q, want Accept-Encoding", got)
		}
		reader, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatalf("gzip.NewReader: %v", err)
		}
		decoded, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("gunzip: %v", err)
		}
		if string(decoded) != body {
			t.Fatalf("解压后长度 = %d, want %d", len(decoded), len(body))
		}
	})

	t.Run("no Accept-Encoding passes through", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app-x.js", nil))
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want empty", got)
		}
		if rec.Body.String() != body {
			t.Fatalf("响应体长度 = %d, want %d", rec.Body.Len(), len(body))
		}
	})

	t.Run("range request not compressed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app-x.js", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Range", "bytes=0-9")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want empty for range request", got)
		}
		if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(body)) {
			t.Fatalf("Content-Length = %q, want original length preserved", got)
		}
	})

	t.Run("binary asset not compressed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/icon-x.png", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want empty for binary asset", got)
		}
	})
}
