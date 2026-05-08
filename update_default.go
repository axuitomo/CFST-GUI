//go:build !webui

package main

import "runtime"

func currentInstallMode() string {
	return defaultInstallMode(runtime.GOOS)
}
