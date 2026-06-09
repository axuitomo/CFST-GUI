package app

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestLatestAssetDownloadURL(t *testing.T) {
	got := latestAssetDownloadURL("cfst-gui-linux-amd64.tar.gz")
	want := "https://github.com/axuitomo/CFST-GUI/releases/latest/download/cfst-gui-linux-amd64.tar.gz"
	if got != want {
		t.Fatalf("latestAssetDownloadURL() = %q, want %q", got, want)
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
					"docker_image": "ghcr.io/axuitomo/cfst-gui:1.1.0",
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
	if asset.DockerImage != "ghcr.io/axuitomo/cfst-gui:1.1.0" {
		t.Fatalf("manifest docker image was not preserved: %#v", asset)
	}
}

func TestCheckGitHubReleaseForUpdate(t *testing.T) {
	oldClient := httpClientForUpdates
	oldVersion := version
	requests := []string{}
	defer func() {
		httpClientForUpdates = oldClient
		version = oldVersion
	}()
	version = "1.0"
	httpClientForUpdates = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.URL.Path)
			var body string
			switch req.URL.Path {
			case "/repos/axuitomo/CFST-GUI/releases/latest":
				body = `{"tag_name":"v1.1.0","name":"CFST-GUI 1.1.0","html_url":"https://example.invalid/release","assets":[{"name":"cfst-gui-update-manifest.json","browser_download_url":"https://api.example.invalid/manifest.json"},{"name":"matched","browser_download_url":"https://github.com/axuitomo/CFST-GUI/releases/download/v1.1.0/matched"}]}`
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
	if !info.UpdateAvailable || info.LatestVersion != "1.1.0" {
		t.Fatalf("unexpected update info: %#v", info)
	}
	if info.AssetName != "" || info.DownloadURL != "" || info.SHA256 != "" {
		t.Fatalf("check should not resolve install asset: %#v", info)
	}
	if len(requests) != 1 || requests[0] != "/repos/axuitomo/CFST-GUI/releases/latest" {
		t.Fatalf("check should only request latest release, got %#v", requests)
	}
}

func TestResolveGitHubReleaseUpdate(t *testing.T) {
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
				body = `{"docker_image":"ghcr.io/axuitomo/cfst-gui:1.1.0","assets":[{"goos":"` + runtime.GOOS + `","goarch":"` + runtime.GOARCH + `","name":"matched","sha256":"abc","install_mode":"manual"}]}`
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

	info, err := resolveGitHubReleaseUpdate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !info.UpdateAvailable || info.LatestVersion != "1.1.0" || info.AssetName != "matched" {
		t.Fatalf("unexpected update info: %#v", info)
	}
	if info.DownloadURL != "https://github.com/axuitomo/CFST-GUI/releases/download/v1.1.0/matched" {
		t.Fatalf("manifest asset should inherit release download URL, got %q", info.DownloadURL)
	}
	if info.DockerImage != "ghcr.io/axuitomo/cfst-gui:1.1.0" {
		t.Fatalf("manifest docker image should be returned, got %q", info.DockerImage)
	}
}

func TestUpdateMetadataRequestsUseShortTimeout(t *testing.T) {
	oldClient := httpClientForUpdates
	defer func() { httpClientForUpdates = oldClient }()

	assertMetadataDeadline := func(t *testing.T, req *http.Request) {
		t.Helper()
		deadline, ok := req.Context().Deadline()
		if !ok {
			t.Fatalf("metadata request for %s has no context deadline", req.URL.String())
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > updateMetadataTimeout {
			t.Fatalf("metadata request timeout = %s, want within %s", remaining, updateMetadataTimeout)
		}
	}

	t.Run("latest release", func(t *testing.T) {
		httpClientForUpdates = &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				assertMetadataDeadline(t, req)
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v1.0.0","html_url":"https://example.invalid/release"}`)),
					Header:     make(http.Header),
				}, nil
			}),
		}
		if _, err := fetchLatestGitHubRelease(t.Context()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("manifest", func(t *testing.T) {
		httpClientForUpdates = &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				assertMetadataDeadline(t, req)
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(`{"assets":[]}`)),
					Header:     make(http.Header),
				}, nil
			}),
		}
		if _, err := fetchUpdateManifest(t.Context(), "https://example.invalid/manifest.json"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGitHubDownloadCandidates(t *testing.T) {
	got := githubDownloadCandidates("https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe")
	want := []string{
		"https://ghproxy.vip/https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
		"https://gh.3w.pm/https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
		"https://gh.ddlc.top/https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
		"https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("githubDownloadCandidates() = %#v, want %#v", got, want)
	}
	if got := githubDownloadCandidates("https://example.invalid/asset"); len(got) != 1 || got[0] != "https://example.invalid/asset" {
		t.Fatalf("non-GitHub URL changed to %#v", got)
	}
}

func TestDownloadFileRacesAcrossMirrors(t *testing.T) {
	oldClient := httpClientForUpdates
	defer func() { httpClientForUpdates = oldClient }()
	body := []byte("cfst")
	sum := sha256.Sum256(body)
	attempts := make(chan string, 4)
	httpClientForUpdates = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts <- req.URL.Host
			switch req.URL.Host {
			case "ghproxy.vip":
				<-req.Context().Done()
				return nil, req.Context().Err()
			case "gh.3w.pm":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			default:
				<-req.Context().Done()
				return nil, req.Context().Err()
			}
		}),
	}
	path := filepath.Join(t.TempDir(), "asset.bin")
	if err := downloadFile(t.Context(), "https://github.com/axuitomo/CFST-GUI/releases/download/v1.7.1/asset.bin", path, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("downloaded body = %q, want %q", got, body)
	}
	assertNoUpdatePartFiles(t, filepath.Dir(path))
	seen := map[string]bool{}
	for len(seen) < 4 {
		select {
		case host := <-attempts:
			seen[host] = true
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for racing attempts, saw %#v", seen)
		}
	}
	for _, host := range []string{"ghproxy.vip", "gh.3w.pm", "gh.ddlc.top", "github.com"} {
		if !seen[host] {
			t.Fatalf("missing racing attempt for %s: %#v", host, seen)
		}
	}
}

func TestDownloadFileSkipsChecksumMismatch(t *testing.T) {
	oldClient := httpClientForUpdates
	defer func() { httpClientForUpdates = oldClient }()
	body := []byte("correct")
	sum := sha256.Sum256(body)
	httpClientForUpdates = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Host {
			case "ghproxy.vip":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader("wrong")),
					Header:     make(http.Header),
				}, nil
			case "gh.3w.pm":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			default:
				<-req.Context().Done()
				return nil, req.Context().Err()
			}
		}),
	}
	path := filepath.Join(t.TempDir(), "asset.bin")
	if err := downloadFile(t.Context(), "https://github.com/axuitomo/CFST-GUI/releases/download/v1.7.1/asset.bin", path, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("downloaded body = %q, want %q", got, body)
	}
	assertNoUpdatePartFiles(t, filepath.Dir(path))
}

func TestDownloadFileAllFailuresCleansTempFiles(t *testing.T) {
	oldClient := httpClientForUpdates
	defer func() { httpClientForUpdates = oldClient }()
	httpClientForUpdates = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Status:     "502 Bad Gateway",
				Body:       io.NopCloser(strings.NewReader("bad gateway")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "asset.bin")
	if err := downloadFile(t.Context(), "https://github.com/axuitomo/CFST-GUI/releases/download/v1.7.1/asset.bin", path, ""); err == nil {
		t.Fatal("expected download failure")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target path exists after failure: %v", err)
	}
	assertNoUpdatePartFiles(t, dir)
}

func assertNoUpdatePartFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.part"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files were not cleaned: %#v", matches)
	}
}

func TestSelectReleaseAssetFallsBackToLatestRedirect(t *testing.T) {
	asset, err := selectReleaseAsset(t.Context(), githubRelease{
		Assets:  []githubReleaseAsset{},
		TagName: "v1.1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	name := defaultReleaseAssetName(runtime.GOOS, runtime.GOARCH)
	if asset.Name != name || asset.DownloadURL != latestAssetDownloadURL(name) {
		t.Fatalf("unexpected latest fallback asset: %#v", asset)
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

func TestDefaultReleaseAssetNameWindowsUsesEXE(t *testing.T) {
	if got := defaultReleaseAssetName("windows", "amd64"); got != "cfst-gui-windows-amd64.exe" {
		t.Fatalf("defaultReleaseAssetName(windows, amd64) = %q", got)
	}
	if got := defaultInstallMode("windows"); got != "windows_exe" {
		t.Fatalf("defaultInstallMode(windows) = %q", got)
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
	if got := powerShellSingleQuote(`C:\Temp\O'Hara\cfst.exe`); got != `'C:\Temp\O''Hara\cfst.exe'` {
		t.Fatalf("powerShellSingleQuote returned %s", got)
	}
	windowsCommand := buildWindowsInstallerCleanupCommand(`C:\Temp\O'Hara\cfst.exe`, 1234)
	if !strings.Contains(windowsCommand, "Start-Process") || !strings.Contains(windowsCommand, "Remove-Item") || !strings.Contains(windowsCommand, "Wait-Process -Id 1234") {
		t.Fatalf("unexpected windows cleanup command: %s", windowsCommand)
	}
	unixScript := buildUnixReplaceScript("/opt/cfst-gui/cfst-gui", "/tmp/cfst-gui", "/tmp/cfst-gui-linux-amd64.tar.gz")
	if !strings.Contains(unixScript, "chmod +x") || !strings.Contains(unixScript, "mv -f") || !strings.Contains(unixScript, "rm -f '/tmp/cfst-gui-linux-amd64.tar.gz'") {
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
	downloadedPackage := filepath.Join(dir, "cfst-gui-linux-amd64.tar.gz")
	if err := os.WriteFile(downloadedPackage, []byte("package"), 0o600); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "update.sh")
	if err := os.WriteFile(scriptPath, []byte(buildUnixReplaceScript(currentExe, replacement, downloadedPackage)), 0o700); err != nil {
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
	if _, err := os.Stat(downloadedPackage); !os.IsNotExist(err) {
		t.Fatalf("downloaded package should be removed after script, stat err=%v", err)
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
