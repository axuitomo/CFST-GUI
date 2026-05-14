//go:build webui

package app

func currentInstallMode() string {
	return "docker_compose"
}
