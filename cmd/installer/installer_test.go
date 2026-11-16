package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestRenderUIFillsEveryPlaceholder(t *testing.T) {
	page := renderUI(`C:\Program Files\axuitomo\CFST-GUI`, true)
	if strings.Contains(page, "{{") {
		t.Fatalf("template placeholder left in output: %s", firstPlaceholder(page))
	}
	if !strings.Contains(page, "WEBVIEW2: DETECTED") {
		t.Fatal("expected detected badge text")
	}
	if !strings.Contains(page, version) {
		t.Fatal("expected version to be injected")
	}

	missing := renderUI(`C:\x`, false)
	if !strings.Contains(missing, "WEBVIEW2: MISSING") {
		t.Fatal("expected missing badge text")
	}
	if !strings.Contains(missing, `data-state="missing"`) {
		t.Fatal("expected missing badge state")
	}
}

func TestPayloadRejectsPlaceholderOnly(t *testing.T) {
	// 仓库里只有占位文件，开发期必须明确报错而不是装出一个空壳。
	if _, err := payloadBytes(); err == nil {
		t.Fatal("expected error when only the placeholder is embedded")
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
	// 正在运行的卸载器删不掉，这是 Windows 的硬限制。
	// 这里断言它被跳过而其余内容被清空，且不返回错误 —— 之前正是这里
	// 让整个卸载返回 1，进而把注册表项留在「应用和功能」里。
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
	if len(entries) != 1 || entries[0].Name() != uninstName {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected only %s left, got %v", uninstName, names)
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
