//go:build !webui

package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// WebView2 的 profile 目录不能再叫 <exe 名>（Windows 上带 .exe）：它必须和应用数据目录
// 一样挂在 %APPDATA%\CFST-GUI 下。
func TestWebviewUserDataPathLivesUnderAppDataDir(t *testing.T) {
	isolateStorageForTest(t)

	got := webviewUserDataPath()
	if base := filepath.Base(filepath.Dir(got)); base != "CFST-GUI" {
		t.Fatalf("webview data root = %q, want parent CFST-GUI", filepath.Dir(got))
	}
	if strings.Contains(strings.ToLower(filepath.Base(filepath.Dir(got))), ".exe") {
		t.Fatalf("webview data root must not carry the exe suffix: %q", got)
	}
}

// 老目录（%APPDATA%\<exe 名>，Windows 上带 .exe）要被一次性搬到新目录，且已有 profile 时
// 不再搬迁；否则每次启动都会把老目录塞回新目录，覆盖正在使用的 profile。
func TestMigrateLegacyWebviewUserData(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("legacy dir is the Wails Windows default: %AppData%\\<exe name>")
	}
	root := isolateStorageForTest(t)
	target := filepath.Join(root, "webview2")

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	legacy := filepath.Join(os.Getenv("AppData"), filepath.Base(exe))
	if !strings.HasSuffix(strings.ToLower(legacy), ".exe") {
		t.Fatalf("legacy dir should keep the exe suffix: %q", legacy)
	}
	if err := os.MkdirAll(filepath.Join(legacy, "EBWebView"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "EBWebView", "marker"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := webviewUserDataPath(); got != target {
		t.Fatalf("webviewUserDataPath() = %q, want %q", got, target)
	}
	if _, err := os.Stat(filepath.Join(target, "EBWebView", "marker")); err != nil {
		t.Fatalf("legacy profile not migrated: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy dir should be moved away, stat err = %v", err)
	}

	// 再来一次：目标已在，老目录（重建）必须原地不动。
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	migrateLegacyWebviewUserData(target)
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy dir should be left in place when target exists: %v", err)
	}
}
