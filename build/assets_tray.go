//go:build tray && !darwin

package buildassets

import (
	_ "embed"
	"runtime"
)

//go:embed windows/icon.ico
var trayWindowsIcon []byte

// TrayIconResources returns platform tray icon byte slices for desktop hosts.
func TrayIconResources() ([]byte, []byte) {
	if runtime.GOOS == "windows" {
		return AppIcon, trayWindowsIcon
	}
	return AppIcon, nil
}
