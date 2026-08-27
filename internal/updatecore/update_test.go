package updatecore

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"v1.0.0", "1.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"v2.0", "1.9.9", 1},
		{"1.0.0", "1.1", -1},
	}
	for _, tc := range cases {
		if got := CompareVersions(tc.left, tc.right); got != tc.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestMatchManifestAssetSupportsGoAndPlatformFields(t *testing.T) {
	manifest := Manifest{Assets: []ManifestAsset{
		{Name: "amd64", GoOS: "linux", GoArch: "amd64", InstallMode: "docker_compose"},
		{Name: "arm64", Platform: "linux/arm64"},
	}}

	amd64, ok := MatchManifestAsset(manifest, "linux", "amd64", DefaultInstallMode("linux"))
	if !ok || amd64.Platform != "linux/amd64" || amd64.InstallMode != "docker_compose" {
		t.Fatalf("unexpected amd64 asset: %#v", amd64)
	}

	arm64, ok := MatchManifestAsset(manifest, "linux", "arm64", DefaultInstallMode("linux"))
	if !ok || arm64.GoOS != "linux" || arm64.GoArch != "arm64" || arm64.InstallMode != "replace_binary" {
		t.Fatalf("unexpected arm64 asset: %#v", arm64)
	}
}

func TestUpdateInfoFromRelease(t *testing.T) {
	info := UpdateInfoFromRelease(Release{
		TagName: "v1.2.0",
		Name:    "CFST-GUI 1.2.0",
		HTMLURL: "https://example.invalid/release",
	}, "1.1.0", "https://example.invalid/latest", "linux/amd64", "replace_binary")

	if !info.UpdateAvailable || info.LatestVersion != "1.2.0" || info.ReleaseName != "CFST-GUI 1.2.0" {
		t.Fatalf("unexpected update info: %#v", info)
	}
	if info.Platform != "linux/amd64" || info.InstallMode != "replace_binary" {
		t.Fatalf("platform metadata was not preserved: %#v", info)
	}
}
