package updatecore

import (
	"fmt"
	"net/url"
	"strings"
)

const UpdateManifestName = "cfst-gui-update-manifest.json"
const LatestReleaseBase = "https://github.com/axuitomo/CFST-GUI/releases/latest/download/"

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
	DockerImage     string `json:"docker_image"`
	LatestVersion   string `json:"latest_version"`
	ReleaseName     string `json:"release_name"`
	ReleaseURL      string `json:"release_url"`
	SHA256          string `json:"sha256"`
	UpdateAvailable bool   `json:"update_available"`
}

type Release struct {
	Assets  []ReleaseAsset `json:"assets"`
	HTMLURL string         `json:"html_url"`
	Name    string         `json:"name"`
	TagName string         `json:"tag_name"`
}

type ReleaseAsset struct {
	BrowserDownloadURL string `json:"browser_download_url"`
	Name               string `json:"name"`
}

type Manifest struct {
	Assets      []ManifestAsset `json:"assets"`
	DockerImage string          `json:"docker_image"`
}

type ManifestAsset struct {
	DownloadURL string `json:"download_url"`
	DockerImage string `json:"-"`
	GoArch      string `json:"goarch"`
	GoOS        string `json:"goos"`
	InstallMode string `json:"install_mode"`
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	SHA256      string `json:"sha256"`
}

type UpdateInstallResult struct {
	UpdateInfo
	DownloadedPath string `json:"downloaded_path"`
	InstallStarted bool   `json:"install_started"`
	NextAction     string `json:"next_action"`
}

func UpdateInfoFromRelease(release Release, currentVersion, releaseURL, platform, installMode string) UpdateInfo {
	available := CompareVersions(release.TagName, currentVersion) > 0
	return UpdateInfo{
		AppInfo:         AppInfo{CurrentVersion: strings.TrimSpace(currentVersion), InstallMode: strings.TrimSpace(installMode), Platform: strings.TrimSpace(platform), ReleaseURL: strings.TrimSpace(releaseURL)},
		LatestVersion:   NormalizeVersion(release.TagName),
		ReleaseName:     strings.TrimSpace(release.Name),
		ReleaseURL:      firstNonEmpty(release.HTMLURL, releaseURL),
		UpdateAvailable: available,
	}
}

func MatchManifestAsset(manifest Manifest, targetOS, targetArch, fallbackInstallMode string) (ManifestAsset, bool) {
	fallbackInstallMode = firstNonEmpty(fallbackInstallMode, DefaultInstallMode(targetOS))
	for _, asset := range manifest.Assets {
		if strings.EqualFold(asset.GoOS, targetOS) && strings.EqualFold(asset.GoArch, targetArch) {
			asset.InstallMode = firstNonEmpty(asset.InstallMode, fallbackInstallMode)
			asset.Platform = firstNonEmpty(asset.Platform, targetOS+"/"+targetArch)
			return asset, true
		}
		if strings.EqualFold(asset.Platform, targetOS+"/"+targetArch) {
			asset.GoOS = firstNonEmpty(asset.GoOS, targetOS)
			asset.GoArch = firstNonEmpty(asset.GoArch, targetArch)
			asset.InstallMode = firstNonEmpty(asset.InstallMode, fallbackInstallMode)
			return asset, true
		}
	}
	return ManifestAsset{}, false
}

func ReleaseAssetMap(assets []ReleaseAsset) map[string]ReleaseAsset {
	result := make(map[string]ReleaseAsset, len(assets))
	for _, asset := range assets {
		result[asset.Name] = asset
	}
	return result
}

func DefaultReleaseAssetName(goos, goarch string) string {
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

func LatestAssetDownloadURL(assetName string) string {
	return LatestReleaseBase + url.PathEscape(strings.TrimSpace(assetName))
}

func DefaultInstallMode(goos string) string {
	switch goos {
	case "windows":
		return "windows_exe"
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

func CompareVersions(left, right string) int {
	leftParts := parseVersionParts(left)
	rightParts := parseVersionParts(right)
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

func parseVersionParts(value string) []int {
	normalized := NormalizeVersion(value)
	if cut := strings.IndexAny(normalized, "-+"); cut >= 0 {
		normalized = normalized[:cut]
	}
	rawParts := strings.Split(normalized, ".")
	parts := make([]int, 0, len(rawParts))
	for _, part := range rawParts {
		digits := leadingDigits(part)
		if digits == "" {
			parts = append(parts, 0)
			continue
		}
		var parsed int
		for _, digit := range digits {
			parsed = parsed*10 + int(digit-'0')
		}
		parts = append(parts, parsed)
	}
	if len(parts) == 0 {
		return []int{0}
	}
	return parts
}

func NormalizeVersion(value string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(value), "v"), "V")
}

func leadingDigits(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if char < '0' || char > '9' {
			break
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
