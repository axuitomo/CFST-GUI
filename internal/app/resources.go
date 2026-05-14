package app

import (
	"io/fs"
	"runtime"
)

type Resources struct {
	FrontendAssets  fs.FS
	TrayPNGIcon     []byte
	TrayWindowsIcon []byte
}

var runtimeResources Resources

func setResources(resources Resources) {
	runtimeResources = resources
}

func (r Resources) trayIconBytes() []byte {
	if runtime.GOOS == "windows" {
		return r.TrayWindowsIcon
	}
	return r.TrayPNGIcon
}
