//go:build !tray || darwin

package buildassets

// TrayIconResources returns empty tray icons when tray support is disabled.
func TrayIconResources() ([]byte, []byte) {
	return nil, nil
}
