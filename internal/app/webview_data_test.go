//go:build !webui

package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// WebView2 的 profile 属于本机缓存，必须落在本机目录（Windows 上是 %LOCALAPPDATA%），
// 不能再跟着漫游的 %APPDATA% 走：域账号登录/注销会整份同步 HTTP 缓存与 GPUCache。
func TestWebviewUserDataPathLivesUnderLocalAppData(t *testing.T) {
	isolateStorageForTest(t)

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("os.UserCacheDir: %v", err)
	}
	got := webviewUserDataPath()
	if want := filepath.Join(cacheDir, "CFST-GUI", "webview2"); got != want {
		t.Fatalf("webviewUserDataPath() = %q, want %q", got, want)
	}
	if strings.Contains(strings.ToLower(filepath.Base(filepath.Dir(filepath.Dir(got)))), ".exe") {
		t.Fatalf("webview data root must not carry the exe suffix: %q", got)
	}
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		roaming := filepath.Join(configDir, "CFST-GUI")
		if strings.HasPrefix(got, roaming+string(filepath.Separator)) {
			t.Fatalf("webview data must not live under the roaming config dir %q: %q", roaming, got)
		}
	}
}

// 上一版把 profile 放在应用数据目录下（%APPDATA%\CFST-GUI\webview2），换到本机目录时
// 必须一次性搬走：那里存着 localStorage，不搬等于让老用户丢掉侧栏折叠、结果页偏好。
func TestMigrateLegacyWebviewUserDataMovesRoamingProfile(t *testing.T) {
	isolateStorageForTest(t)

	legacy := filepath.Join(defaultStorageDir(), "webview2")
	target := webviewUserDataPath()
	if legacy == target {
		t.Fatalf("legacy and target must differ: %q", legacy)
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
}

// 老目录（%APPDATA%\<exe 名>，Windows 上带 .exe）也要能被搬；目标已存在时不再搬迁，
// 否则每次启动都会把老目录塞回新目录，覆盖正在使用的 profile。
func TestMigrateLegacyWebviewUserData(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("legacy dir is the Wails Windows default: %AppData%\\<exe name>")
	}
	isolateStorageForTest(t)
	target := webviewUserDataPath()

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
