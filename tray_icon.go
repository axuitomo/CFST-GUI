//go:build tray && !darwin

package main

import (
	_ "embed"
	"runtime"
)

//go:embed build/appicon.png
var trayPNGIcon []byte

//go:embed build/windows/icon.ico
var trayWindowsIcon []byte

func trayIconBytes() []byte {
	if runtime.GOOS == "windows" {
		return trayWindowsIcon
	}
	return trayPNGIcon
}
