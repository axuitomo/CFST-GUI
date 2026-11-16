//go:build webui

package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWebUIFilesStayInsideAllowedRoots(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CFST_WEBUI_ALLOWED_ROOTS", root)
	filePath := filepath.Join(root, "result.csv")
	const contents = "address\n1.1.1.1\n"
	if err := os.WriteFile(filePath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	opened, path, err := openWebUIAllowedPath(filePath)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(opened)
	opened.Close()
	if readErr != nil || path != filePath || string(body) != contents {
		t.Fatalf("allowed open = %q, %q, %v", path, body, readErr)
	}
	app := NewApp()
	recorder := httptest.NewRecorder()
	app.handleWebUIFileDownload(recorder, httptest.NewRequest(http.MethodGet, "/api/files/download?path="+url.QueryEscape(filePath), nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != contents || !strings.Contains(recorder.Header().Get("Content-Disposition"), "result.csv") {
		t.Fatalf("download = %d, %q, %v", recorder.Code, recorder.Body.String(), recorder.Header())
	}
	recorder = httptest.NewRecorder()
	app.handleWebUIFileList(recorder, httptest.NewRequest(http.MethodGet, "/api/files/list?path="+url.QueryEscape(root), nil))
	var listing struct {
		Entries []webUIFileEntry `json:"entries"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || len(listing.Entries) != 1 || listing.Entries[0].Name != "result.csv" {
		t.Fatalf("list = %d, %s", recorder.Code, recorder.Body.String())
	}
	outside := filepath.Join(base, "secret.txt")
	if err := os.WriteFile(outside, []byte("outside-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertRejected := func(t *testing.T, path string) {
		t.Helper()
		if file, _, err := openWebUIAllowedPath(path); err == nil {
			file.Close()
			t.Fatalf("opened outside path: %q", path)
		}
		for _, handler := range []http.HandlerFunc{app.handleWebUIFileList, app.handleWebUIFileDownload} {
			recorder := httptest.NewRecorder()
			handler(recorder, httptest.NewRequest(http.MethodGet, "/?path="+url.QueryEscape(path), nil))
			if recorder.Code == http.StatusOK || strings.Contains(recorder.Body.String(), "outside-secret") {
				t.Fatalf("outside handler = %d, %s", recorder.Code, recorder.Body.String())
			}
		}
	}
	assertRejected(t, outside)
	assertRejected(t, root+string(os.PathSeparator)+".."+string(os.PathSeparator)+"secret.txt")
	t.Run("symlink_escape", func(t *testing.T) {
		link := filepath.Join(root, "link")
		if err := os.Symlink(base, link); err != nil {
			if runtime.GOOS != "windows" {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			if output, junctionErr := exec.Command("cmd", "/c", "mklink", "/J", link, base).CombinedOutput(); junctionErr != nil {
				t.Skipf("symlink and junction creation unavailable: %v; %v: %s", err, junctionErr, output)
			}
		}
		assertRejected(t, filepath.Join(link, "secret.txt"))
	})
}
