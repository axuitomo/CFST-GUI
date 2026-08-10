//go:build webui

package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

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
