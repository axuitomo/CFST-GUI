//go:build webui

package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

// TestWebUISPAHandlerServesAssets 回归测试：fs.FS 使用 "/" 分隔路径，
// handler 不得使用 filepath.Clean（Windows 下会产生 "\"）导致所有静态资源
// fallback 到 index.html。详见 webUISPAHandler 内注释。
func TestWebUISPAHandlerServesAssets(t *testing.T) {
	staticFS := fstest.MapFS{
		"index.html":                {Data: []byte("index-page")},
		"favicon.png":               {Data: []byte("png-data")},
		"assets/index-RiAyM-Vd.js":  {Data: []byte("js-data")},
		"assets/index-ChSV6Hvv.css": {Data: []byte("css-data")},
	}
	handler := webUISPAHandler(staticFS)

	cases := []struct {
		name       string
		path       string
		wantBody   string
		wantStatus int
	}{
		{name: "root", path: "/", wantBody: "index-page", wantStatus: http.StatusOK},
		{name: "single segment", path: "/favicon.png", wantBody: "png-data", wantStatus: http.StatusOK},
		{name: "nested asset js", path: "/assets/index-RiAyM-Vd.js", wantBody: "js-data", wantStatus: http.StatusOK},
		{name: "nested asset css", path: "/assets/index-ChSV6Hvv.css", wantBody: "css-data", wantStatus: http.StatusOK},
		{name: "unknown path falls back", path: "/nope.js", wantBody: "index-page", wantStatus: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			body, err := io.ReadAll(rec.Result().Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if string(body) != tc.wantBody {
				t.Fatalf("body = %q, want %q (asset fell back to index.html?)", string(body), tc.wantBody)
			}
		})
	}
}

// TestWebUIHostIsLoopback 保证“无令牌时必须拒绝非回环绑定”的判断不会把
// 0.0.0.0、空主机名或 IPv6 通配地址误判成本机。
func TestWebUIHostIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:34115":    true,
		"127.0.0.1":          true,
		"localhost:34115":    true,
		"[::1]:34115":        true,
		"0.0.0.0:34115":      false,
		":34115":             false,
		"[::]:34115":         false,
		"192.168.1.10:34115": false,
	}
	for addr, want := range cases {
		if got := webUIHostIsLoopback(addr); got != want {
			t.Errorf("webUIHostIsLoopback(%q) = %v, want %v", addr, got, want)
		}
	}
}

// TestWebUIAuthRequiresTokenWhenConfigured 覆盖 webUIAuth：未配置令牌时放行，配置令牌后
// 必须携带 Bearer 头或 token 查询参数，否则返回 401。绑定非回环地址时的安全边界依赖它。
func TestWebUIAuthRequiresTokenWhenConfigured(t *testing.T) {
	handler := NewApp().webUIAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	cases := []struct {
		name   string
		token  string
		header string
		query  string
		want   int
	}{
		{name: "no token configured", want: http.StatusOK},
		{name: "missing credentials", token: "secret", want: http.StatusUnauthorized},
		{name: "wrong bearer token", token: "secret", header: "Bearer nope", want: http.StatusUnauthorized},
		{name: "bearer token", token: "secret", header: "Bearer secret", want: http.StatusOK},
		{name: "query token", token: "secret", query: "?token=secret", want: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CFST_WEBUI_TOKEN", tc.token)
			req := httptest.NewRequest(http.MethodPost, "/api/command/runtime.status"+tc.query, strings.NewReader(`{}`))
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// TestWebUIInvokeRoutesSharedCommand 回归测试：WebUI 的 /api/command 路由必须复用共享 Invoke 调度。
func TestWebUIInvokeRoutesSharedCommand(t *testing.T) {
	setConfigHomeForTest(t, t.TempDir())
	t.Setenv("CFST_GUI_PORTABLE_ROOT", "")
	app := NewApp()
	if err := app.core.WriteTaskSnapshot(appcore.TaskSnapshot{Status: "completed", TaskID: "webui-history-task"}); err != nil {
		t.Fatal(err)
	}

	result := decodeWebUICommandForTest(t, app.Invoke("task.list", `{"limit":1}`))
	if !result.OK || result.Code != "TASK_SNAPSHOT_LIST" {
		t.Fatalf("task.list = %#v", result)
	}
}

// TestWebUIRuntimeStatusRejectsRemoteByDefault 回归测试：runtime.status 默认只允许本机访问。
func TestWebUIRuntimeStatusRejectsRemoteByDefault(t *testing.T) {
	t.Setenv("CFST_RUNTIME_DIAGNOSTICS", "1")
	t.Setenv("CFST_RUNTIME_DIAGNOSTICS_REMOTE", "")
	t.Setenv("CFST_WEBUI_TOKEN", "")
	app := NewApp()
	request := httptest.NewRequest(http.MethodPost, "/api/command/runtime.status", strings.NewReader(`{}`))
	request.RemoteAddr = "203.0.113.10:12345"
	recorder := httptest.NewRecorder()
	app.handleWebUICommand(recorder, request)
	result := decodeWebUICommandForTest(t, recorder.Body.String())
	if result.OK || result.Code != "RUNTIME_DIAGNOSTICS_LOCAL_ONLY" {
		t.Fatalf("runtime.status = %#v, want local-only rejection", result)
	}
}

// TestWebUIRuntimeStatusAllowsAuthenticatedRemoteDiagnostics 覆盖显式开启远程诊断后的放行路径。
func TestWebUIRuntimeStatusAllowsAuthenticatedRemoteDiagnostics(t *testing.T) {
	t.Setenv("CFST_RUNTIME_DIAGNOSTICS", "1")
	t.Setenv("CFST_RUNTIME_DIAGNOSTICS_REMOTE", "1")
	t.Setenv("CFST_WEBUI_TOKEN", "test-token")
	app := NewApp()
	request := httptest.NewRequest(http.MethodPost, "/api/command/runtime.status", strings.NewReader(`{}`))
	request.RemoteAddr = "203.0.113.10:12345"
	recorder := httptest.NewRecorder()
	app.handleWebUICommand(recorder, request)
	result := decodeWebUICommandForTest(t, recorder.Body.String())
	if !result.OK || result.Code != "RUNTIME_STATUS_READY" {
		t.Fatalf("runtime.status = %#v", result)
	}
}

func decodeWebUICommandForTest(t *testing.T, raw string) appcore.CommandResult {
	t.Helper()
	var result appcore.CommandResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode command result: %v", err)
	}
	return result
}
