//go:build !tray || darwin

package main

func trayIconResources() ([]byte, []byte) {
	return nil, nil
}
