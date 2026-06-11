package githubdownload

import (
	"strings"
	"testing"
)

func TestCandidatesUsesGitHubMirrorChain(t *testing.T) {
	got := Candidates("https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe")
	want := []string{
		"https://ghproxy.vip/https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
		"https://gh.3w.pm/https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
		"https://gh.ddlc.top/https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
		"https://github.com/axuitomo/CFST-GUI/releases/download/v1.5/cfst-gui-windows-amd64.exe",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Candidates() = %#v, want %#v", got, want)
	}
}

func TestCandidatesKeepsNonGitHubURL(t *testing.T) {
	got := Candidates("https://example.invalid/asset")
	if len(got) != 1 || got[0] != "https://example.invalid/asset" {
		t.Fatalf("non-GitHub URL changed to %#v", got)
	}
}
