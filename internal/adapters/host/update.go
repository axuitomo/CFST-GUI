package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/github/githubdownload"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/updatecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
)

const (
	githubLatestReleaseAPI = "https://api.github.com/repos/axuitomo/CFST-GUI/releases/latest"
	latestReleaseBase      = updatecore.LatestReleaseBase
	ReleasePageURL         = "https://github.com/axuitomo/CFST-GUI/releases/latest"
	releasePageURL         = ReleasePageURL
	updateManifestName     = updatecore.UpdateManifestName
	updateMetadataTimeout  = 8 * time.Second
	updateDownloadTimeout  = 8 * time.Second
)

var httpClientForUpdates = httpclient.NewClient(httpclient.Options{
	DisableProxy: true,
	Protocol:     httpclient.ProtocolTCP,
	Profile:      httpcfg.Resolve("", "", "", "", false),
	Timeout:      30 * time.Second,
})

func closeUpdateIdleConnections() {
	if httpClientForUpdates != nil {
		httpClientForUpdates.CloseIdleConnections()
	}
}

type AppInfo = updatecore.AppInfo

type UpdateInfo = updatecore.UpdateInfo

type UpdateInstallResult = updatecore.UpdateInstallResult

type githubRelease = updatecore.Release
type githubReleaseAsset = updatecore.ReleaseAsset
type updateManifest = updatecore.Manifest
type updateManifestAsset = updatecore.ManifestAsset

func AppVersion() string {
	if strings.TrimSpace(version) == "" {
		return "1.0"
	}
	return strings.TrimSpace(version)
}

func appVersion() string {
	return AppVersion()
}

func appInfoPayload() AppInfo {
	return appInfoPayloadFor(DefaultInstallMode(runtime.GOOS))
}

func appInfoPayloadFor(installMode string) AppInfo {
	if strings.TrimSpace(installMode) == "" {
		installMode = DefaultInstallMode(runtime.GOOS)
	}
	return AppInfo{
		CurrentVersion: AppVersion(),
		InstallMode:    installMode,
		Platform:       runtime.GOOS + "/" + runtime.GOARCH,
		ReleaseURL:     ReleasePageURL,
	}
}

func checkGitHubReleaseForUpdate(ctx context.Context) (UpdateInfo, error) {
	return checkGitHubReleaseForUpdateWithInstallMode(ctx, DefaultInstallMode(runtime.GOOS))
}

func checkGitHubReleaseForUpdateWithInstallMode(ctx context.Context, installMode string) (UpdateInfo, error) {
	_, info, err := latestReleaseUpdateInfoWithInstallMode(ctx, installMode)
	return info, err
}

func resolveGitHubReleaseUpdate(ctx context.Context) (UpdateInfo, error) {
	return resolveGitHubReleaseUpdateWithInstallMode(ctx, DefaultInstallMode(runtime.GOOS))
}

func resolveGitHubReleaseUpdateWithInstallMode(ctx context.Context, installMode string) (UpdateInfo, error) {
	release, info, err := latestReleaseUpdateInfoWithInstallMode(ctx, installMode)
	if err != nil {
		return UpdateInfo{}, err
	}
	if !info.UpdateAvailable {
		return info, nil
	}
	asset, err := selectReleaseAssetWithInstallMode(ctx, release, installMode)
	if err != nil {
		return info, err
	}
	info.AssetName = asset.Name
	info.DownloadURL = asset.DownloadURL
	info.DockerImage = asset.DockerImage
	info.SHA256 = asset.SHA256
	info.InstallMode = FirstNonEmpty(asset.InstallMode, info.InstallMode)
	return info, nil
}

func latestReleaseUpdateInfoWithInstallMode(ctx context.Context, installMode string) (githubRelease, UpdateInfo, error) {
	release, err := fetchLatestGitHubRelease(ctx)
	if err != nil {
		return githubRelease{}, UpdateInfo{}, err
	}
	return release, updateInfoFromReleaseWithInstallMode(release, installMode), nil
}

func updateInfoFromReleaseWithInstallMode(release githubRelease, installMode string) UpdateInfo {
	if strings.TrimSpace(installMode) == "" {
		installMode = DefaultInstallMode(runtime.GOOS)
	}
	return updatecore.UpdateInfoFromRelease(release, AppVersion(), ReleasePageURL, runtime.GOOS+"/"+runtime.GOARCH, installMode)
}

func fetchLatestGitHubRelease(ctx context.Context) (githubRelease, error) {
	requestCtx, cancel := context.WithTimeout(ctx, updateMetadataTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, githubLatestReleaseAPI, nil)
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
	return selectReleaseAssetWithInstallMode(ctx, release, DefaultInstallMode(runtime.GOOS))
}

func selectReleaseAssetWithInstallMode(ctx context.Context, release githubRelease, installMode string) (updateManifestAsset, error) {
	if strings.TrimSpace(installMode) == "" {
		installMode = DefaultInstallMode(runtime.GOOS)
	}
	assetMap := releaseAssetMap(release.Assets)
	if manifestAsset, ok := assetMap[updatecore.UpdateManifestName]; ok && manifestAsset.BrowserDownloadURL != "" {
		manifest, err := fetchUpdateManifest(ctx, manifestAsset.BrowserDownloadURL)
		if err != nil {
			return updateManifestAsset{}, err
		}
		if selected, ok := MatchManifestAssetForTargetWithInstallMode(manifest, runtime.GOOS, runtime.GOARCH, installMode); ok {
			selected.DockerImage = strings.TrimSpace(manifest.DockerImage)
			if selected.DownloadURL == "" {
				if releaseAsset, exists := assetMap[selected.Name]; exists {
					selected.DownloadURL = releaseAsset.BrowserDownloadURL
				}
			}
			if selected.DownloadURL == "" {
				selected.DownloadURL = latestAssetDownloadURL(selected.Name)
			}
			return selected, nil
		}
		return updateManifestAsset{}, fmt.Errorf("更新 manifest 没有匹配当前平台 %s/%s 的资产", runtime.GOOS, runtime.GOARCH)
	}

	name := defaultReleaseAssetName(runtime.GOOS, runtime.GOARCH)
	asset, ok := assetMap[name]
	downloadURL := ""
	if ok {
		downloadURL = strings.TrimSpace(asset.BrowserDownloadURL)
	}
	if downloadURL == "" {
		downloadURL = latestAssetDownloadURL(name)
	}
	return updateManifestAsset{
		DownloadURL: downloadURL,
		GoArch:      runtime.GOARCH,
		GoOS:        runtime.GOOS,
		InstallMode: installMode,
		Name:        FirstNonEmpty(asset.Name, name),
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
	}, nil
}

func fetchUpdateManifest(ctx context.Context, manifestURL string) (updateManifest, error) {
	var lastErr error
	for _, candidate := range githubdownload.Candidates(manifestURL) {
		requestCtx, cancel := context.WithTimeout(ctx, updateMetadataTimeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, candidate, nil)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "CFST-GUI/"+appVersion())
		res, err := httpClientForUpdates.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		var manifest updateManifest
		func() {
			defer cancel()
			defer res.Body.Close()
			if res.StatusCode < 200 || res.StatusCode >= 300 {
				lastErr = fmt.Errorf("更新 manifest 返回状态 %s (%s)", res.Status, candidate)
				return
			}
			if err := json.NewDecoder(res.Body).Decode(&manifest); err != nil {
				lastErr = err
				return
			}
			lastErr = nil
		}()
		if lastErr == nil {
			return manifest, nil
		}
	}
	return updateManifest{}, lastErr
}

func matchManifestAsset(manifest updateManifest) (updateManifestAsset, bool) {
	return MatchManifestAssetForTargetWithInstallMode(manifest, runtime.GOOS, runtime.GOARCH, DefaultInstallMode(runtime.GOOS))
}

func matchManifestAssetForTarget(manifest updateManifest, targetOS, targetArch string) (updateManifestAsset, bool) {
	return MatchManifestAssetForTargetWithInstallMode(manifest, targetOS, targetArch, updatecore.DefaultInstallMode(targetOS))
}

func MatchManifestAssetForTargetWithInstallMode(manifest updateManifest, targetOS, targetArch, fallbackInstallMode string) (updateManifestAsset, bool) {
	return updatecore.MatchManifestAsset(manifest, targetOS, targetArch, fallbackInstallMode)
}

func matchManifestAssetForTargetWithInstallMode(manifest updateManifest, targetOS, targetArch, fallbackInstallMode string) (updateManifestAsset, bool) {
	return MatchManifestAssetForTargetWithInstallMode(manifest, targetOS, targetArch, fallbackInstallMode)
}

func releaseAssetMap(assets []githubReleaseAsset) map[string]githubReleaseAsset {
	return updatecore.ReleaseAssetMap(assets)
}

func defaultReleaseAssetName(goos, goarch string) string {
	return updatecore.DefaultReleaseAssetName(goos, goarch)
}

func latestAssetDownloadURL(assetName string) string {
	return updatecore.LatestAssetDownloadURL(assetName)
}

func DefaultInstallMode(goos string) string {
	return updatecore.DefaultInstallMode(goos)
}

func defaultInstallMode(goos string) string {
	return DefaultInstallMode(goos)
}

func compareSemver(left, right string) int {
	return updatecore.CompareVersions(left, right)
}

func normalizeDisplayVersion(value string) string {
	return updatecore.NormalizeVersion(value)
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
