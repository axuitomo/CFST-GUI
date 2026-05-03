package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	githubLatestReleaseAPI = "https://api.github.com/repos/axuitomo/CFST-GUI/releases/latest"
	releasePageURL         = "https://github.com/axuitomo/CFST-GUI/releases/latest"
	updateManifestName     = "cfst-gui-update-manifest.json"
)

var httpClientForUpdates = &http.Client{Timeout: 30 * time.Second}

type AppInfo struct {
	CurrentVersion string `json:"current_version"`
	InstallMode    string `json:"install_mode"`
	Platform       string `json:"platform"`
	ReleaseURL     string `json:"release_url"`
}

type UpdateInfo struct {
	AppInfo
	AssetName       string `json:"asset_name"`
	DownloadURL     string `json:"download_url"`
	LatestVersion   string `json:"latest_version"`
	ReleaseName     string `json:"release_name"`
	ReleaseURL      string `json:"release_url"`
	SHA256          string `json:"sha256"`
	UpdateAvailable bool   `json:"update_available"`
}

type UpdateInstallResult struct {
	UpdateInfo
	DownloadedPath string `json:"downloaded_path"`
	InstallStarted bool   `json:"install_started"`
	NextAction     string `json:"next_action"`
}

type githubRelease struct {
	Assets  []githubReleaseAsset `json:"assets"`
	HTMLURL string               `json:"html_url"`
	Name    string               `json:"name"`
	TagName string               `json:"tag_name"`
}

type githubReleaseAsset struct {
	BrowserDownloadURL string `json:"browser_download_url"`
	Name               string `json:"name"`
}

type updateManifest struct {
	Assets []updateManifestAsset `json:"assets"`
}

type updateManifestAsset struct {
	DownloadURL string `json:"download_url"`
	GoArch      string `json:"goarch"`
	GoOS        string `json:"goos"`
	InstallMode string `json:"install_mode"`
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	SHA256      string `json:"sha256"`
}

func appVersion() string {
	if strings.TrimSpace(version) == "" {
		return "1.0"
	}
	return strings.TrimSpace(version)
}

func appInfoPayload() AppInfo {
	return AppInfo{
		CurrentVersion: appVersion(),
		InstallMode:    defaultInstallMode(runtime.GOOS),
		Platform:       runtime.GOOS + "/" + runtime.GOARCH,
		ReleaseURL:     releasePageURL,
	}
}

func checkGitHubReleaseForUpdate(ctx context.Context) (UpdateInfo, error) {
	release, err := fetchLatestGitHubRelease(ctx)
	if err != nil {
		return UpdateInfo{}, err
	}
	available := compareSemver(release.TagName, appVersion()) > 0
	info := UpdateInfo{
		AppInfo:         appInfoPayload(),
		LatestVersion:   normalizeDisplayVersion(release.TagName),
		ReleaseName:     strings.TrimSpace(release.Name),
		ReleaseURL:      firstNonEmpty(release.HTMLURL, releasePageURL),
		UpdateAvailable: available,
	}
	if !available {
		return info, nil
	}
	asset, err := selectReleaseAsset(ctx, release)
	if err != nil {
		return info, err
	}
	info.AssetName = asset.Name
	info.DownloadURL = asset.DownloadURL
	info.SHA256 = asset.SHA256
	info.InstallMode = firstNonEmpty(asset.InstallMode, info.InstallMode)
	return info, nil
}

func fetchLatestGitHubRelease(ctx context.Context) (githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubLatestReleaseAPI, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "CFST-GUI/"+appVersion())
	res, err := httpClientForUpdates.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return githubRelease{}, fmt.Errorf("GitHub Releases 返回状态 %s", res.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(res.Body).Decode(&release); err != nil {
		return githubRelease{}, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubRelease{}, errors.New("GitHub Release 缺少 tag_name")
	}
	if release.HTMLURL == "" {
		release.HTMLURL = releasePageURL
	}
	return release, nil
}

func selectReleaseAsset(ctx context.Context, release githubRelease) (updateManifestAsset, error) {
	assetMap := releaseAssetMap(release.Assets)
	if manifestAsset, ok := assetMap[updateManifestName]; ok && manifestAsset.BrowserDownloadURL != "" {
		manifest, err := fetchUpdateManifest(ctx, manifestAsset.BrowserDownloadURL)
		if err != nil {
			return updateManifestAsset{}, err
		}
		if selected, ok := matchManifestAsset(manifest); ok {
			if selected.DownloadURL == "" {
				if releaseAsset, exists := assetMap[selected.Name]; exists {
					selected.DownloadURL = releaseAsset.BrowserDownloadURL
				}
			}
			if selected.DownloadURL == "" {
				return updateManifestAsset{}, fmt.Errorf("更新 manifest 中的资产 %s 缺少下载地址", selected.Name)
			}
			return selected, nil
		}
		return updateManifestAsset{}, fmt.Errorf("更新 manifest 没有匹配当前平台 %s/%s 的资产", runtime.GOOS, runtime.GOARCH)
	}

	name := defaultReleaseAssetName(runtime.GOOS, runtime.GOARCH)
	asset, ok := assetMap[name]
	if !ok || asset.BrowserDownloadURL == "" {
		return updateManifestAsset{}, fmt.Errorf("GitHub Release 缺少当前平台资产 %s", name)
	}
	return updateManifestAsset{
		DownloadURL: asset.BrowserDownloadURL,
		GoArch:      runtime.GOARCH,
		GoOS:        runtime.GOOS,
		InstallMode: defaultInstallMode(runtime.GOOS),
		Name:        asset.Name,
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
	}, nil
}

func fetchUpdateManifest(ctx context.Context, manifestURL string) (updateManifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return updateManifest{}, err
	}
	req.Header.Set("User-Agent", "CFST-GUI/"+appVersion())
	res, err := httpClientForUpdates.Do(req)
	if err != nil {
		return updateManifest{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return updateManifest{}, fmt.Errorf("更新 manifest 返回状态 %s", res.Status)
	}
	var manifest updateManifest
	if err := json.NewDecoder(res.Body).Decode(&manifest); err != nil {
		return updateManifest{}, err
	}
	return manifest, nil
}

func matchManifestAsset(manifest updateManifest) (updateManifestAsset, bool) {
	targetOS, targetArch := runtime.GOOS, runtime.GOARCH
	for _, asset := range manifest.Assets {
		if strings.EqualFold(asset.GoOS, targetOS) && strings.EqualFold(asset.GoArch, targetArch) {
			asset.InstallMode = firstNonEmpty(asset.InstallMode, defaultInstallMode(targetOS))
			asset.Platform = firstNonEmpty(asset.Platform, targetOS+"/"+targetArch)
			return asset, true
		}
		if strings.EqualFold(asset.Platform, targetOS+"/"+targetArch) {
			asset.GoOS = firstNonEmpty(asset.GoOS, targetOS)
			asset.GoArch = firstNonEmpty(asset.GoArch, targetArch)
			asset.InstallMode = firstNonEmpty(asset.InstallMode, defaultInstallMode(targetOS))
			return asset, true
		}
	}
	return updateManifestAsset{}, false
}

func releaseAssetMap(assets []githubReleaseAsset) map[string]githubReleaseAsset {
	result := make(map[string]githubReleaseAsset, len(assets))
	for _, asset := range assets {
		result[asset.Name] = asset
	}
	return result
}

func defaultReleaseAssetName(goos, goarch string) string {
	switch goos {
	case "windows":
		return fmt.Sprintf("cfst-gui-windows-%s.exe", goarch)
	case "linux":
		return fmt.Sprintf("cfst-gui-linux-%s.tar.gz", goarch)
	case "darwin":
		return fmt.Sprintf("cfst-gui-darwin-%s.app.zip", goarch)
	case "android":
		return "cfst-gui-android-release.apk"
	default:
		return fmt.Sprintf("cfst-gui-%s-%s", goos, goarch)
	}
}

func defaultInstallMode(goos string) string {
	switch goos {
	case "windows":
		return "replace_exe"
	case "linux":
		return "replace_binary"
	case "darwin":
		return "replace_app"
	case "android":
		return "android_apk"
	default:
		return "manual"
	}
}

func compareSemver(left, right string) int {
	leftParts := parseSemverParts(left)
	rightParts := parseSemverParts(right)
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for len(leftParts) < maxLen {
		leftParts = append(leftParts, 0)
	}
	for len(rightParts) < maxLen {
		rightParts = append(rightParts, 0)
	}
	for index := 0; index < maxLen; index++ {
		if leftParts[index] > rightParts[index] {
			return 1
		}
		if leftParts[index] < rightParts[index] {
			return -1
		}
	}
	return 0
}

func parseSemverParts(value string) []int {
	normalized := normalizeDisplayVersion(value)
	if cut := strings.IndexAny(normalized, "-+"); cut >= 0 {
		normalized = normalized[:cut]
	}
	rawParts := strings.Split(normalized, ".")
	parts := make([]int, 0, len(rawParts))
	for _, part := range rawParts {
		digits := takeLeadingDigits(part)
		if digits == "" {
			parts = append(parts, 0)
			continue
		}
		parsed, err := strconv.Atoi(digits)
		if err != nil {
			parts = append(parts, 0)
			continue
		}
		parts = append(parts, parsed)
	}
	if len(parts) == 0 {
		return []int{0}
	}
	return parts
}

func normalizeDisplayVersion(value string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(value), "v"), "V")
}

func takeLeadingDigits(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if char < '0' || char > '9' {
			break
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func downloadAndInstallUpdate(ctx context.Context, info UpdateInfo, downloadDir string) (UpdateInstallResult, error) {
	result := UpdateInstallResult{UpdateInfo: info}
	downloadDir = strings.TrimSpace(downloadDir)
	if downloadDir == "" {
		downloadDir = filepath.Join(storageRoot(), "updates")
	}
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return result, err
	}
	targetPath := filepath.Join(downloadDir, info.AssetName)
	if err := downloadFile(ctx, info.DownloadURL, targetPath); err != nil {
		return result, err
	}
	result.DownloadedPath = targetPath
	if strings.TrimSpace(info.SHA256) != "" {
		if err := verifySHA256(targetPath, info.SHA256); err != nil {
			return result, err
		}
	}
	nextAction, err := startInstallStrategy(info.InstallMode, targetPath)
	if err != nil {
		return result, err
	}
	result.InstallStarted = nextAction != "manual"
	result.NextAction = nextAction
	return result, nil
}

func downloadFile(ctx context.Context, sourceURL, targetPath string) error {
	if strings.TrimSpace(sourceURL) == "" {
		return errors.New("缺少更新包下载地址")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "CFST-GUI/"+appVersion())
	res, err := httpClientForUpdates.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("下载更新包返回状态 %s", res.Status)
	}
	tempPath := targetPath + ".part"
	output, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, res.Body)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return closeErr
	}
	return os.Rename(tempPath, targetPath)
}

func verifySHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("SHA256 校验失败：期望 %s，实际 %s", expected, actual)
	}
	return nil
}

func startInstallStrategy(mode, downloadedPath string) (string, error) {
	switch strings.TrimSpace(mode) {
	case "replace_exe":
		return startWindowsReplacement(downloadedPath)
	case "replace_binary":
		return startLinuxReplacement(downloadedPath)
	case "replace_app":
		return startDarwinReplacement(downloadedPath)
	case "android_apk":
		return "manual", errors.New("Android APK 安装必须由 Android 原生插件触发")
	default:
		return "manual", openPathDetached(downloadedPath)
	}
}

func startWindowsReplacement(downloadedPath string) (string, error) {
	currentExe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := ensureWritableTarget(currentExe); err != nil {
		return "manual", openPathDetached(downloadedPath)
	}
	scriptPath := filepath.Join(filepath.Dir(downloadedPath), "cfst-gui-update-"+strconv.FormatInt(time.Now().UnixNano(), 10)+".cmd")
	script := buildWindowsReplaceScript(currentExe, downloadedPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return "", err
	}
	if err := exec.Command("cmd", "/C", "start", "", scriptPath).Start(); err != nil {
		return "", err
	}
	return "restart_pending", nil
}

func buildWindowsReplaceScript(currentExe, downloadedPath string) string {
	return fmt.Sprintf("@echo off\r\nping 127.0.0.1 -n 3 > nul\r\ncopy /Y %s %s\r\nstart \"\" %s\r\ndel \"%%~f0\"\r\n",
		windowsQuote(downloadedPath),
		windowsQuote(currentExe),
		windowsQuote(currentExe),
	)
}

func windowsQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func startLinuxReplacement(downloadedPath string) (string, error) {
	currentExe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := ensureWritableTarget(currentExe); err != nil {
		return "manual", openPathDetached(downloadedPath)
	}
	stamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	extractedBinary, cleanup, err := extractLinuxBinary(downloadedPath)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return "", err
	}
	replacementPath := filepath.Join(filepath.Dir(downloadedPath), "cfst-gui-update-binary-"+stamp)
	if err := copyUpdateFile(extractedBinary, replacementPath, 0o755); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return "", err
	}
	if cleanup != nil {
		cleanup()
	}
	scriptPath := filepath.Join(filepath.Dir(downloadedPath), "cfst-gui-update-"+stamp+".sh")
	script := buildUnixReplaceScript(currentExe, replacementPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		_ = os.Remove(replacementPath)
		return "", err
	}
	if err := exec.Command("sh", scriptPath).Start(); err != nil {
		_ = os.Remove(replacementPath)
		return "", err
	}
	return "restart_pending", nil
}

func buildUnixReplaceScript(currentExe, replacementPath string) string {
	return fmt.Sprintf("#!/usr/bin/env sh\nset -e\nsleep 1\nchmod +x %s\nmv -f %s %s\nchmod +x %s\n%s >/dev/null 2>&1 &\nrm -- \"$0\"\n",
		shellQuote(replacementPath),
		shellQuote(replacementPath),
		shellQuote(currentExe),
		shellQuote(currentExe),
		shellQuote(currentExe),
	)
}

func copyUpdateFile(sourcePath, targetPath string, mode os.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		_ = os.Remove(targetPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(targetPath)
		return closeErr
	}
	return nil
}

func extractLinuxBinary(downloadedPath string) (string, func(), error) {
	if !strings.HasSuffix(downloadedPath, ".tar.gz") {
		return downloadedPath, nil, nil
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(downloadedPath), "cfst-linux-update-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	file, err := os.Open(downloadedPath)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	defer gzipReader.Close()
	entries, err := untarRegularFiles(gzipReader, tempDir)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if len(entries) == 0 {
		cleanup()
		return "", nil, errors.New("Linux 更新包中没有可执行文件")
	}
	sort.Strings(entries)
	return entries[0], cleanup, nil
}

func untarRegularFiles(reader io.Reader, targetDir string) ([]string, error) {
	tarReader := tar.NewReader(reader)
	entries := make([]string, 0)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		targetPath, ok := safeArchiveTargetPath(targetDir, header.Name)
		if !ok {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode())
		if err != nil {
			return nil, err
		}
		_, copyErr := io.Copy(file, tarReader)
		closeErr := file.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if header.FileInfo().Mode()&0o111 != 0 || strings.Contains(strings.ToLower(filepath.Base(targetPath)), "cfst-gui") {
			entries = append(entries, targetPath)
		}
	}
	return entries, nil
}

func startDarwinReplacement(downloadedPath string) (string, error) {
	currentExe, err := os.Executable()
	if err != nil {
		return "", err
	}
	appRoot := findDarwinAppRoot(currentExe)
	if appRoot == "" {
		return "manual", openPathDetached(downloadedPath)
	}
	if err := ensureWritableTarget(appRoot); err != nil {
		return "manual", openPathDetached(downloadedPath)
	}
	extractDir, err := os.MkdirTemp(filepath.Dir(downloadedPath), "cfst-darwin-update-*")
	if err != nil {
		return "", err
	}
	if err := unzip(downloadedPath, extractDir); err != nil {
		_ = os.RemoveAll(extractDir)
		return "", err
	}
	replacementApp, err := findFirstAppBundle(extractDir)
	if err != nil {
		_ = os.RemoveAll(extractDir)
		return "", err
	}
	scriptPath := filepath.Join(filepath.Dir(downloadedPath), "cfst-gui-update-"+strconv.FormatInt(time.Now().UnixNano(), 10)+".sh")
	script := fmt.Sprintf("#!/usr/bin/env sh\nsleep 1\nrm -rf %s\ncp -R %s %s\nopen %s >/dev/null 2>&1 &\nrm -rf %s\nrm -- \"$0\"\n",
		shellQuote(appRoot),
		shellQuote(replacementApp),
		shellQuote(appRoot),
		shellQuote(appRoot),
		shellQuote(extractDir),
	)
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		_ = os.RemoveAll(extractDir)
		return "", err
	}
	if err := exec.Command("sh", scriptPath).Start(); err != nil {
		return "", err
	}
	return "restart_pending", nil
}

func findDarwinAppRoot(currentExe string) string {
	cleaned := filepath.Clean(currentExe)
	for {
		if strings.HasSuffix(cleaned, ".app") {
			return cleaned
		}
		parent := filepath.Dir(cleaned)
		if parent == cleaned {
			return ""
		}
		cleaned = parent
	}
}

func ensureWritableTarget(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	dir := path
	if !info.IsDir() {
		dir = filepath.Dir(path)
	}
	temp, err := os.CreateTemp(dir, ".cfst-write-check-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	closeErr := temp.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

func unzip(sourcePath, targetDir string) error {
	reader, err := zip.OpenReader(sourcePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		targetPath, ok := safeArchiveTargetPath(targetDir, file.Name)
		if !ok {
			continue
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		input, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.FileInfo().Mode())
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeInputErr := input.Close()
		closeOutputErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeInputErr != nil {
			return closeInputErr
		}
		if closeOutputErr != nil {
			return closeOutputErr
		}
	}
	return nil
}

func findFirstAppBundle(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errors.New("macOS 更新包中没有 .app")
	}
	return found, nil
}

func safeArchiveTargetPath(baseDir, name string) (string, bool) {
	cleanName := filepath.Clean(strings.ReplaceAll(name, "\\", "/"))
	cleanName = strings.TrimPrefix(cleanName, string(filepath.Separator))
	if cleanName == "." || strings.HasPrefix(cleanName, "..") {
		return "", false
	}
	targetPath := filepath.Join(baseDir, cleanName)
	rel, err := filepath.Rel(baseDir, targetPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return targetPath, true
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func openExternalURL(rawURL string) error {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return err
	}
	return openPathDetached(rawURL)
}

func openPathDetached(targetPath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetPath)
	case "darwin":
		cmd = exec.Command("open", targetPath)
	default:
		cmd = exec.Command("xdg-open", targetPath)
	}
	return cmd.Start()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
