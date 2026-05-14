//go:build !webui

package app

import "runtime"

func currentInstallMode() string {
	return defaultInstallMode(runtime.GOOS)
}
