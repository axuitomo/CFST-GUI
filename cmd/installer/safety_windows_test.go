package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"golang.org/x/sys/windows/registry"
)

func TestInstallDirectoryBoundaries(t *testing.T) {
	dir := t.TempDir()
	for _, invalid := range []string{"", ".", filepath.VolumeName(dir) + `\`, `\\server\share\`} {
		if _, err := validateInstallDir(invalid, ""); err == nil {
			t.Errorf("accepted unsafe install directory %q", invalid)
		}
	}
	if _, err := validateInstallDir(dir, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "user-document.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := validateInstallDir(dir, ""); err == nil {
		t.Fatal("accepted an unrelated nonempty directory")
	}
	if _, err := validateInstallDir(dir, dir); err != nil {
		t.Fatalf("registered install must remain repairable: %v", err)
	}
}

func TestUninstallRequiresRegisteredLocationAndIdentity(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, uninstName)
	if err := os.WriteFile(self, []byte("installer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateUninstallLocation(dir, self, dir); err != nil {
		t.Fatal(err)
	}
	for _, registered := range []string{"", t.TempDir()} {
		if err := validateUninstallLocation(dir, self, registered); err == nil {
			t.Fatal("accepted an unregistered uninstall directory")
		}
	}
	download := filepath.Join(dir, "downloaded-installer.exe")
	if err := os.WriteFile(download, []byte("installer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateUninstallLocation(dir, download, dir); err == nil {
		t.Fatal("accepted the downloaded installer as the registered uninstaller")
	}
}

func TestInstallCancellationRestoresPreviousFiles(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, stopAt := range []int{55, 62} {
			t.Run(fmt.Sprintf("existing=%v/percent=%d", existing, stopAt), func(t *testing.T) {
				dir := t.TempDir()
				old := []byte("old-version")
				if existing {
					for _, name := range []string{appExeName, uninstName} {
						if err := os.WriteFile(filepath.Join(dir, name), old, 0o755); err != nil {
							t.Fatal(err)
						}
					}
				}
				self := filepath.Join(t.TempDir(), "setup.exe")
				if err := os.WriteFile(self, []byte("new-uninstaller"), 0o755); err != nil {
					t.Fatal(err)
				}
				err := Install([]byte("new-app"), self, Options{Dir: dir}, func(percent int, _ string) error {
					if percent == stopAt {
						return ErrCancelled
					}
					return nil
				})
				if !errors.Is(err, ErrCancelled) {
					t.Fatalf("cancellation = %v", err)
				}
				for _, name := range []string{appExeName, uninstName} {
					raw, err := os.ReadFile(filepath.Join(dir, name))
					if existing && (err != nil || !bytes.Equal(raw, old)) {
						t.Fatalf("previous %s was not restored: %v", name, err)
					}
					if !existing && !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("fresh cancelled install left %s: %v", name, err)
					}
				}
			})
		}
	}
}

func TestInstallerPayloadAndUIBoundaries(t *testing.T) {
	payload, err := payloadBytes(fstest.MapFS{
		"payload/cfst-gui.exe": {Data: []byte("app")},
		"payload/old.exe":      {Data: bytes.Repeat([]byte("old"), 20)},
	})
	if err != nil || string(payload) != "app" {
		t.Fatalf("selected payload = %q, %v", payload, err)
	}
	page := renderUI(Options{Dir: `C:\tools&copy;`}, true)
	if !strings.Contains(page, `value="C:\tools&amp;copy;"`) {
		t.Fatal("install path was not HTML escaped")
	}
	for _, option := range []string{"desktop", "launch"} {
		if strings.Contains(page, `id="`+option+`" checked`) {
			t.Fatalf("disabled option %s was checked", option)
		}
	}
}

func TestCloseDuringInstallRequestsCancellation(t *testing.T) {
	server := &server{}
	server.running.Store(true)
	response := httptest.NewRecorder()
	server.handleClose(response, httptest.NewRequest("POST", "/api/close", nil))
	if !server.cancelled.Load() || !strings.Contains(response.Body.String(), `"closing":false`) {
		t.Fatal("close did not wait for controlled cancellation")
	}
}

func TestDelayedCleanupPreservesUserFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{uninstName, "user-document.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	scheduleDelete(dir)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, uninstName)); errors.Is(err, os.ErrNotExist) {
			if raw, err := os.ReadFile(filepath.Join(dir, "user-document.txt")); err != nil || string(raw) != "keep" {
				t.Fatal("delayed cleanup removed an unrelated file")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("delayed cleanup did not remove the uninstaller")
}

func TestUninstallRegistrationRollback(t *testing.T) {
	path := fmt.Sprintf(`Software\CFST-GUI-Installer-Tests\%d-%d`, os.Getpid(), time.Now().UnixNano())
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = key.Close()
		_ = registry.DeleteKey(registry.CURRENT_USER, path)
	})
	if err := key.SetStringValue("DisplayName", "previous"); err != nil {
		t.Fatal(err)
	}
	if err := key.SetDWordValue("EstimatedSize", 123); err != nil {
		t.Fatal(err)
	}
	if err := updateUninstallValues(key, map[string]string{"DisplayName": "new", "InvalidValue": "invalid\x00value"}, 456); err == nil {
		t.Fatal("expected a failed registry write")
	}
	if value, _, err := key.GetStringValue("DisplayName"); err != nil || value != "previous" {
		t.Fatal("uninstall metadata was not restored")
	}
	if size, _, err := key.GetIntegerValue("EstimatedSize"); err != nil || size != 123 {
		t.Fatal("previous size was not restored")
	}
}
