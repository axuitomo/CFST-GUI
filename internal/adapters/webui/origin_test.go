package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebUIWriteRequestBoundaries(t *testing.T) {
	setConfigHomeForTest(t, t.TempDir())
	t.Setenv("CFST_GUI_PORTABLE_ROOT", "")
	app := NewApp()
	for _, endpoint := range []string{"command/config.save", "platform/GetAppInfo"} {
		for _, tc := range []struct {
			name, host, origin, site, contentType, token string
			status                                       int
		}{
			{name: "cross-site form", origin: "https://attacker.invalid", site: "cross-site", contentType: "text/plain", status: http.StatusForbidden},
			{name: "cross-origin JSON", origin: "https://attacker.invalid", contentType: "application/json", status: http.StatusForbidden},
			{name: "same-origin form", origin: "http://localhost:34115", contentType: "text/plain", status: http.StatusUnsupportedMediaType},
			{name: "missing type", status: http.StatusUnsupportedMediaType},
			{name: "same-origin JSON", origin: "http://localhost:34115", site: "same-origin", contentType: "application/json; charset=utf-8", status: http.StatusOK},
			{name: "local automation", contentType: "application/json", status: http.StatusOK},
			{name: "untrusted host", host: "rebound.invalid:34115", contentType: "application/json", status: http.StatusForbidden},
			{name: "authenticated automation", host: "webui.example:443", token: "fixture", contentType: "application/json", status: http.StatusOK},
			{name: "TLS proxy same-origin", host: "webui.example:443", origin: "https://webui.example:443", token: "fixture", contentType: "application/json", status: http.StatusOK},
		} {
			t.Run(endpoint+"/"+tc.name, func(t *testing.T) {
				t.Setenv("CFST_WEBUI_TOKEN", tc.token)
				host := tc.host
				if host == "" {
					host = "localhost:34115"
				}
				request := httptest.NewRequest(http.MethodPost, "http://"+host+"/api/"+endpoint, strings.NewReader(`{"config_snapshot":{"sources":[]},"padding":"="}`))
				request.Header.Set("Origin", tc.origin)
				request.Header.Set("Sec-Fetch-Site", tc.site)
				request.Header.Set("Content-Type", tc.contentType)
				if tc.token != "" {
					request.Header.Set("Authorization", "Bearer "+tc.token)
				}
				handler := app.handleWebUICommand
				if strings.HasPrefix(endpoint, "platform/") {
					handler = app.handleWebUIPlatformCommand
				}
				response := httptest.NewRecorder()
				app.webUIAuth(http.HandlerFunc(handler)).ServeHTTP(response, request)
				if response.Code != tc.status {
					t.Fatalf("HTTP status = %d, want %d", response.Code, tc.status)
				}
				if tc.status == http.StatusOK {
					result := decodeWebUICommandForTest(t, response.Body.String())
					if !result.OK {
						t.Fatalf("valid request failed: %s", result.Code)
					}
				}
			})
		}
	}
}

func TestWebUIUnauthenticatedReadRejectsUntrustedHost(t *testing.T) {
	t.Setenv("CFST_WEBUI_TOKEN", "")
	handler := NewApp().webUIAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "http://rebound.invalid/api/files/list", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatal("unauthenticated read accepted an untrusted Host")
	}
}
