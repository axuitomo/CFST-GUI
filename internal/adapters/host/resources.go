package host

import (
	"io/fs"
	"runtime"
)

type Resources struct {
	AppPNGIcon      []byte
	FrontendAssets  fs.FS
	TrayPNGIcon     []byte
	TrayWindowsIcon []byte
}

var runtimeResources Resources

func SetResources(resources Resources) {
	runtimeResources = resources
}

func RuntimeResources() Resources {
	return runtimeResources
}

func (r Resources) TrayIconBytes() []byte {
	if runtime.GOOS == "windows" {
		return r.TrayWindowsIcon
	}
	return r.TrayPNGIcon
}
