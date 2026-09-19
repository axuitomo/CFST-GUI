//go:build webui

package app

import "runtime"

// Linux WebUI 的发行形态是 Docker Compose bundle，不能按裸二进制的 replace_binary
// 上报；其余平台（如 Windows 本地调试的 WebUI）按默认发行形态上报，避免把自己
// 误报成 docker_compose。
func currentInstallMode() string {
	if runtime.GOOS == "linux" {
		return "docker_compose"
	}
	return defaultInstallMode(runtime.GOOS)
}
