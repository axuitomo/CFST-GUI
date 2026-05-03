//go:build !tray

package main

func (a *App) startTray() {
	a.setTrayAvailable(false)
}

func (a *App) stopTray() {}
