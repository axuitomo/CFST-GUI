//go:build windows

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func TestParseArgsAcceptsNSISStyleFlags(t *testing.T) {
	opts, silent, uninstall, err := parseArgs([]string{"/S", "/D=C:\\tmp\\cfst", "--no-desktop"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !silent {
		t.Fatal("silent flag not recognised")
	}
	if uninstall {
		t.Fatal("uninstall flag should be off")
	}
	if opts.Dir != `C:\tmp\cfst` {
		t.Fatalf("dir = %q", opts.Dir)
	}
	if opts.DesktopShortcut {
		t.Fatal("--no-desktop should clear DesktopShortcut")
	}
	if !opts.Launch {
		t.Fatal("launch should default to true")
	}
}

func TestParseArgsRejectsUnknownFlag(t *testing.T) {
	if _, _, _, err := parseArgs([]string{"--nope"}); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestScheduleQuitAfterSuccessfulInstall(t *testing.T) {
	done := make(chan struct{})
	srv := &server{quit: func() { close(done) }}
	if timer := srv.scheduleQuit(15 * time.Millisecond); timer == nil {
		t.Fatal("expected a quit timer")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("installer did not request quit after successful install")
	}
}

func TestRunningProcessIDsMatchesExactPath(t *testing.T) {
	// 复制一份 cmd.exe 到临时目录，避免同名进程干扰：
	// 检测必须按完整路径匹配，不能只看 exe 文件名。
	source := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Skipf("read %s: %v", source, err)
	}
	target := filepath.Join(t.TempDir(), "cfst-test-app.exe")
	if err := os.WriteFile(target, raw, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(target, "/c", "ping -n 6 127.0.0.1 >nul")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	var pids []uint32
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pids, err = runningProcessIDs(target)
		if err != nil {
			t.Fatalf("runningProcessIDs: %v", err)
		}
		if len(pids) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(pids) != 1 || pids[0] != uint32(cmd.Process.Pid) {
		t.Fatalf("detected pids = %v, want [%d]", pids, cmd.Process.Pid)
	}
	if err := closeProcesses(pids); err != nil {
		t.Fatalf("closeProcesses: %v", err)
	}
	for deadline = time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		remaining, err := runningProcessIDs(target)
		if err != nil {
			t.Fatalf("runningProcessIDs after close: %v", err)
		}
		if len(remaining) == 0 {
			return
		}
	}
	t.Fatal("process was not closed")
}

func TestRenderUIFillsEveryPlaceholder(t *testing.T) {
	page := renderUI(Options{Dir: `C:\Program Files\axuitomo\CFST-GUI`, DesktopShortcut: true, Launch: true}, true)
	if strings.Contains(page, "{{") {
		t.Fatalf("template placeholder left in output: %s", firstPlaceholder(page))
	}
	if !strings.Contains(page, "WebView2 运行时：已就绪") {
		t.Fatal("expected detected badge text")
	}
	if !strings.Contains(page, version) {
		t.Fatal("expected version to be injected")
	}

	missing := renderUI(Options{Dir: `C:\x`}, false)
	if !strings.Contains(missing, "WebView2 运行时：未安装") {
		t.Fatal("expected missing badge text")
	}
	if !strings.Contains(missing, `data-state="missing"`) {
		t.Fatal("expected missing badge state")
	}
}

func TestPayloadRejectsPlaceholderOnly(t *testing.T) {
	// 仓库里只有占位文件，开发期必须明确报错而不是装出一个空壳。
	if _, err := payloadBytes(payloadFS); err == nil {
		t.Fatal("expected error when only the placeholder is embedded")
	}
}

func TestInstallerLegalFilesReadsBothNotices(t *testing.T) {
	source := fstest.MapFS{
		"payload/LICENSE":                &fstest.MapFile{Data: []byte("license")},
		"payload/THIRD-PARTY-NOTICES.md": &fstest.MapFile{Data: []byte("notices")},
	}
	files, err := installerLegalFiles(source)
	if err != nil {
		t.Fatalf("installerLegalFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	if files[0].name != "LICENSE" || string(files[0].data) != "license" {
		t.Fatalf("unexpected license: %+v", files[0])
	}
	if files[1].name != "THIRD-PARTY-NOTICES.md" || string(files[1].data) != "notices" {
		t.Fatalf("unexpected notices: %+v", files[1])
	}
	if _, err := installerLegalFiles(fstest.MapFS{"payload/LICENSE": &fstest.MapFile{Data: []byte("license")}}); err == nil {
		t.Fatal("expected missing third-party notices to fail")
	}
}

func TestInstallerLegalFilesWriteAndRollback(t *testing.T) {
	files, err := installerLegalFiles(fstest.MapFS{
		"payload/LICENSE":                {Data: []byte("new license")},
		"payload/THIRD-PARTY-NOTICES.md": {Data: []byte("new notices")},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, existing := range []bool{false, true} {
		name := "fresh"
		if existing {
			name = "existing"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			paths := make([]string, len(files))
			for i, file := range files {
				paths[i] = filepath.Join(dir, file.name)
				if existing {
					if err := os.WriteFile(paths[i], []byte("previous "+file.name), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			states, err := appcore.CaptureFileStates(paths...)
			if err != nil {
				t.Fatal(err)
			}
			for i, file := range files {
				if err := appcore.WriteFileAtomic(paths[i], file.data, 0o644); err != nil {
					t.Fatal(err)
				}
				if raw, err := os.ReadFile(paths[i]); err != nil || string(raw) != string(file.data) {
					t.Fatalf("installed %s = %q, %v", file.name, raw, err)
				}
			}
			if err := appcore.RestoreFileStates(states); err != nil {
				t.Fatalf("rollback legal files: %v", err)
			}
			for i, file := range files {
				raw, err := os.ReadFile(paths[i])
				if existing {
					if err != nil || string(raw) != "previous "+file.name {
						t.Fatalf("previous %s = %q, %v", file.name, raw, err)
					}
				} else if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("rollback left new %s: %v", file.name, err)
				}
			}
		})
	}
}

func TestCreateShortcutWritesFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cfst-gui.exe")
	if err := os.WriteFile(target, []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	lnk := filepath.Join(dir, "CFST-GUI.lnk")
	if err := createShortcut(lnk, target, target); err != nil {
		t.Fatalf("createShortcut: %v", err)
	}
	info, err := os.Stat(lnk)
	if err != nil {
		t.Fatalf("shortcut not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("shortcut file is empty")
	}
}

func TestUninstallFilesKeepsRunningExe(t *testing.T) {
	// 卸载移除主程序与法律文件，保留运行中的卸载器和用户文件。
	dir := t.TempDir()
	self := filepath.Join(dir, uninstName)
	for name, body := range map[string]string{
		uninstName:     "self",
		appExeName:     "app",
		"icon.ico":     "icon",
		"leftover.tmp": "tmp",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range installerLegalFileNames {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("notice"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := uninstallFiles(dir, self); err != nil {
		t.Fatalf("uninstallFiles: %v", err)
	}
	if _, err := os.Stat(self); err != nil {
		t.Fatalf("running exe should be left for the delayed delete: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected uninstaller and user files left, got %v", names)
	}
}

func TestScheduleDeleteHandlesWindowsPaths(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "CFST test & %PATH% !")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, uninstName), []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	scheduleDelete(dir)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("delayed cleanup left the installation directory behind")
}

func firstPlaceholder(s string) string {
	i := strings.Index(s, "{{")
	if i < 0 || i+40 > len(s) {
		return s
	}
	return s[i : i+40]
}

func TestLegalHandlerServesEmbeddedFiles(t *testing.T) {
	srv := &server{legalFS: fstest.MapFS{
		"payload/LICENSE":                &fstest.MapFile{Data: []byte("GNU GENERAL PUBLIC LICENSE\n")},
		"payload/THIRD-PARTY-NOTICES.md": &fstest.MapFile{Data: []byte("Third-Party Notices\n")},
	}}
	for name, want := range map[string]string{
		"LICENSE":                "GNU GENERAL PUBLIC LICENSE\n",
		"THIRD-PARTY-NOTICES.md": "Third-Party Notices\n",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/legal?name="+name, nil)
		rec := httptest.NewRecorder()
		srv.handleLegal(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", name, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
			t.Fatalf("%s content type = %q", name, got)
		}
		if got := rec.Body.String(); got != want {
			t.Fatalf("%s body = %q", name, got)
		}
	}
}

func TestLegalHandlerRejectsInvalidRequests(t *testing.T) {
	srv := &server{legalFS: fstest.MapFS{
		"payload/LICENSE":                &fstest.MapFile{Data: []byte("license")},
		"payload/THIRD-PARTY-NOTICES.md": &fstest.MapFile{Data: []byte("notices")},
	}}
	cases := []struct {
		method string
		target string
		status int
	}{
		{http.MethodGet, "/api/legal?name=../LICENSE", http.StatusNotFound},
		{http.MethodGet, "/api/legal?name=LICENSE", http.StatusOK},
		{http.MethodPost, "/api/legal?name=LICENSE", http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		srv.handleLegal(rec, httptest.NewRequest(tc.method, tc.target, nil))
		if rec.Code != tc.status {
			t.Fatalf("%s %s = %d, want %d", tc.method, tc.target, rec.Code, tc.status)
		}
	}
	srv.legalFS = fstest.MapFS{}
	rec := httptest.NewRecorder()
	srv.handleLegal(rec, httptest.NewRequest(http.MethodGet, "/api/legal?name=LICENSE", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing legal payload = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
