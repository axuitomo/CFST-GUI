//go:build webui

package app

import (
	"runtime"
	"testing"
)

func TestCurrentInstallModeWebUI(t *testing.T) {
	got := currentInstallMode()
	switch runtime.GOOS {
	case "linux":
		if got != "docker_compose" {
			t.Fatalf("currentInstallMode() on %s webui = %q, want docker_compose", runtime.GOOS, got)
		}
	default:
		if want := defaultInstallMode(runtime.GOOS); got != want {
			t.Fatalf("currentInstallMode() on %s webui = %q, want %q", runtime.GOOS, got, want)
		}
	}
}
