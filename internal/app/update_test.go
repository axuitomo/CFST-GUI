package app

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		left  string
		want  int
		right string
	}{
		{left: "v1.0.0", want: 0, right: "1.0"},
		{left: "1.0.1", want: 1, right: "1.0.0"},
		{left: "v2.0", want: 1, right: "1.9.9"},
		{left: "1.0.0", want: -1, right: "1.1"},
	}
	for _, tc := range cases {
		if got := compareSemver(tc.left, tc.right); got != tc.want {
			t.Fatalf("compareSemver(%q, %q) = %d, want %d", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestSelectReleaseAssetFromManifest(t *testing.T) {
	oldClient := httpClientForUpdates
	defer func() { httpClientForUpdates = oldClient }()
	httpClientForUpdates = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body: io.NopCloser(strings.NewReader(`{
					"assets": [
						{"goos":"plan9","goarch":"amd64","name":"skip","download_url":"https://example.invalid/skip","sha256":"bad"},
						{"goos":"` + runtime.GOOS + `","goarch":"` + runtime.GOARCH + `","name":"matched","download_url":"https://github.com/axuitomo/CFST-GUI/releases/download/v1.1.0/matched","sha256":"abc","install_mode":"manual"}
					]
				}`)),
				Header: make(http.Header),
			}, nil
		}),
	}

	asset, err := selectReleaseAsset(t.Context(), githubRelease{
		Assets: []githubReleaseAsset{
			{Name: updateManifestName, BrowserDownloadURL: "https://example.invalid/manifest.json"},
			{Name: "matched", BrowserDownloadURL: "https://example.invalid/fallback"},
		},
		TagName: "v1.1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if asset.Name != "matched" || asset.DownloadURL != "https://github.com/axuitomo/CFST-GUI/releases/download/v1.1.0/matched" || asset.SHA256 != "abc" {
		t.Fatalf("unexpected asset: %#v", asset)
	}
}

func TestCheckGitHubReleaseForUpdate(t *testing.T) {
	oldClient := httpClientForUpdates
	oldVersion := version
	defer func() {
		httpClientForUpdates = oldClient
		version = oldVersion
	}()
	version = "1.0"
	httpClientForUpdates = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var body string
			switch req.URL.Path {
			case "/repos/axuitomo/CFST-GUI/releases/latest":
				body = `{"tag_name":"v1.1.0","name":"CFST-GUI 1.1.0","html_url":"https://example.invalid/release","assets":[{"name":"cfst-gui-update-manifest.json","browser_download_url":"https://api.example.invalid/manifest.json"},{"name":"matched","browser_download_url":"https://github.com/axuitomo/CFST-GUI/releases/download/v1.1.0/matched"}]}`
			case "/manifest.json":
				body = `{"assets":[{"goos":"` + runtime.GOOS + `","goarch":"` + runtime.GOARCH + `","name":"matched","sha256":"abc","install_mode":"manual"}]}`
			default:
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	info, err := checkGitHubReleaseForUpdate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !info.UpdateAvailable || info.LatestVersion != "1.1.0" || info.AssetName != "matched" {
		t.Fatalf("unexpected update info: %#v", info)
	}
	if info.DownloadURL != "https://github.com/axuitomo/CFST-GUI/releases/download/v1.1.0/matched" {
		t.Fatalf("manifest asset should inherit release download URL, got %q", info.DownloadURL)
	}
}

func TestGitHubDownloadCandidates(t *testing.T) {
	got := githubDownloadCandidates("https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe")
	want := []string{
		"https://ghproxy.com/https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
		"https://kkgithub.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
		"https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("githubDownloadCandidates() = %#v, want %#v", got, want)
	}
	if got := githubDownloadCandidates("https://example.invalid/asset"); len(got) != 1 || got[0] != "https://example.invalid/asset" {
		t.Fatalf("non-GitHub URL changed to %#v", got)
	}
}

func TestDownloadFileFallsBackAcrossMirrors(t *testing.T) {
	oldClient := httpClientForUpdates
	defer func() { httpClientForUpdates = oldClient }()
	attempts := []string{}
	httpClientForUpdates = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts = append(attempts, req.URL.String())
			switch req.URL.Host {
			case "ghproxy.com":
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Status:     "502 Bad Gateway",
					Body:       io.NopCloser(strings.NewReader("bad gateway")),
					Header:     make(http.Header),
				}, nil
			case "kkgithub.com":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader("cfst")),
					Header:     make(http.Header),
				}, nil
			default:
				t.Fatalf("unexpected host: %s", req.URL.Host)
				return nil, nil
			}
		}),
	}
	path := filepath.Join(t.TempDir(), "asset.bin")
	if err := downloadFile(t.Context(), "https://github.com/axuitomo/CFST-GUI/releases/download/v1.7.1/asset.bin", path); err != nil {
		t.Fatal(err)
	}
	if len(attempts) < 2 || attempts[0] != "https://ghproxy.com/https://github.com/axuitomo/CFST-GUI/releases/download/v1.7.1/asset.bin" || attempts[1] != "https://kkgithub.com/axuitomo/CFST-GUI/releases/download/v1.7.1/asset.bin" {
		t.Fatalf("unexpected attempts: %#v", attempts)
	}
}

func TestSelectReleaseAssetNoMatch(t *testing.T) {
	_, err := selectReleaseAsset(t.Context(), githubRelease{
		Assets:  []githubReleaseAsset{},
		TagName: "v1.1.0",
	})
	if err == nil || !strings.Contains(err.Error(), "缺少当前平台资产") {
		t.Fatalf("expected no asset error, got %v", err)
	}
}

func TestMatchManifestAssetForTargetLinuxArchitectures(t *testing.T) {
	manifest := updateManifest{
		Assets: []updateManifestAsset{
			{Name: "cfst-gui-linux-amd64.tar.gz", GoOS: "linux", GoArch: "amd64", SHA256: "amd", InstallMode: "docker_compose"},
			{Name: "cfst-gui-linux-arm64.tar.gz", Platform: "linux/arm64", SHA256: "arm"},
		},
	}

	amd64Asset, ok := matchManifestAssetForTarget(manifest, "linux", "amd64")
	if !ok {
		t.Fatal("expected amd64 asset match")
	}
	if amd64Asset.Name != "cfst-gui-linux-amd64.tar.gz" || amd64Asset.Platform != "linux/amd64" || amd64Asset.InstallMode != "docker_compose" {
		t.Fatalf("unexpected amd64 asset: %#v", amd64Asset)
	}

	arm64Asset, ok := matchManifestAssetForTarget(manifest, "linux", "arm64")
	if !ok {
		t.Fatal("expected arm64 asset match")
	}
	if arm64Asset.Name != "cfst-gui-linux-arm64.tar.gz" || arm64Asset.GoOS != "linux" || arm64Asset.GoArch != "arm64" || arm64Asset.InstallMode != "replace_binary" {
		t.Fatalf("unexpected arm64 asset: %#v", arm64Asset)
	}
}

func TestMatchManifestAssetForTargetWithInstallModeUsesRuntimeFallback(t *testing.T) {
	manifest := updateManifest{
		Assets: []updateManifestAsset{
			{Name: "cfst-gui-linux-arm64.tar.gz", Platform: "linux/arm64", SHA256: "arm"},
		},
	}

	asset, ok := matchManifestAssetForTargetWithInstallMode(manifest, "linux", "arm64", "docker_compose")
	if !ok {
		t.Fatal("expected linux arm64 asset match")
	}
	if asset.InstallMode != "docker_compose" {
		t.Fatalf("expected runtime install mode fallback, got %#v", asset)
	}
}

func TestMatchManifestAssetForTargetAndroidArchitectures(t *testing.T) {
	manifest := updateManifest{
		Assets: []updateManifestAsset{
			{Name: "cfst-gui-android-release.apk", GoOS: "android", GoArch: "universal", Platform: "android", SHA256: "uni"},
			{Name: "cfst-gui-android-arm64-v8a-release.apk", GoOS: "android", GoArch: "arm64", Platform: "android", SHA256: "arm64"},
			{Name: "cfst-gui-android-armeabi-v7a-release.apk", GoOS: "android", GoArch: "arm", Platform: "android", SHA256: "armv7"},
		},
	}

	androidAsset, ok := matchManifestAssetForTarget(manifest, "android", "arm64")
	if !ok {
		t.Fatal("expected android arm64 asset match")
	}
	if androidAsset.Name != "cfst-gui-android-arm64-v8a-release.apk" {
		t.Fatalf("unexpected android asset: %#v", androidAsset)
	}
}

func TestDefaultReleaseAssetNameLinuxArchitectures(t *testing.T) {
	if got := defaultReleaseAssetName("linux", "amd64"); got != "cfst-gui-linux-amd64.tar.gz" {
		t.Fatalf("defaultReleaseAssetName(linux, amd64) = %q", got)
	}
	if got := defaultReleaseAssetName("linux", "arm64"); got != "cfst-gui-linux-arm64.tar.gz" {
		t.Fatalf("defaultReleaseAssetName(linux, arm64) = %q", got)
	}
}

func TestDefaultReleaseAssetNameAndroid(t *testing.T) {
	if got := defaultReleaseAssetName("android", "arm64"); got != "cfst-gui-android-release.apk" {
		t.Fatalf("defaultReleaseAssetName(android, arm64) = %q", got)
	}
}

func TestVerifySHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asset.bin")
	body := []byte("cfst-gui")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if err := verifySHA256(path, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	if err := verifySHA256(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestBuildInstallScripts(t *testing.T) {
	windowsScript := buildWindowsReplaceScript(`C:\Program Files\CFST-GUI\cfst-gui.exe`, `C:\Temp\cfst-gui.exe`)
	if !strings.Contains(windowsScript, "copy /Y") || !strings.Contains(windowsScript, "start") {
		t.Fatalf("unexpected windows script: %s", windowsScript)
	}
	unixScript := buildUnixReplaceScript("/opt/cfst-gui/cfst-gui", "/tmp/cfst-gui")
	if !strings.Contains(unixScript, "chmod +x") || !strings.Contains(unixScript, "mv -f") {
		t.Fatalf("unexpected unix script: %s", unixScript)
	}
}

func TestEnsureWritableTarget(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cfst-gui")
	if err := os.WriteFile(filePath, []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureWritableTarget(filePath); err != nil {
		t.Fatal(err)
	}
	if err := ensureWritableTarget(dir); err != nil {
		t.Fatal(err)
	}
}

func TestBuildUnixReplaceScriptKeepsReplacementUntilCopy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script execution is only checked on Unix-like hosts")
	}
	dir := t.TempDir()
	currentExe := filepath.Join(dir, "current")
	replacement := filepath.Join(dir, "replacement")
	if err := os.WriteFile(currentExe, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacement, []byte("#!/usr/bin/env sh\n# updated\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "update.sh")
	if err := os.WriteFile(scriptPath, []byte(buildUnixReplaceScript(currentExe, replacement)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("sh", scriptPath).Run(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "#!/usr/bin/env sh\n# updated\nexit 0\n" {
		t.Fatalf("replacement did not copy expected content: %q", string(body))
	}
	if _, err := os.Stat(replacement); !os.IsNotExist(err) {
		t.Fatalf("replacement should be removed after script, stat err=%v", err)
	}
}

func TestArchiveHelpers(t *testing.T) {
	dir := t.TempDir()
	tarGzPath := filepath.Join(dir, "linux.tar.gz")
	if err := writeTestTarGz(tarGzPath, "cfst-gui", []byte("binary")); err != nil {
		t.Fatal(err)
	}
	extracted, cleanup, err := extractLinuxBinary(tarGzPath)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(extracted) != "cfst-gui" {
		t.Fatalf("unexpected extracted binary: %s", extracted)
	}

	zipPath := filepath.Join(dir, "darwin.zip")
	if err := writeTestZip(zipPath, "CFST-GUI.app/Contents/MacOS/cfst-gui", []byte("binary")); err != nil {
		t.Fatal(err)
	}
	unzipDir := filepath.Join(dir, "zip")
	if err := unzip(zipPath, unzipDir); err != nil {
		t.Fatal(err)
	}
	app, err := findFirstAppBundle(unzipDir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(app) != "CFST-GUI.app" {
		t.Fatalf("unexpected app bundle: %s", app)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func writeTestTarGz(path, name string, body []byte) error {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(body)),
	}); err != nil {
		return err
	}
	if _, err := tarWriter.Write(body); err != nil {
		return err
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o600)
}

func writeTestZip(path, name string, body []byte) error {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create(name)
	if err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o600)
}
