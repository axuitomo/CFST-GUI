//go:build !tray || darwin

package desktop

func (a *App) startTray() {
	a.setTrayAvailable(false)
}

func (a *App) stopTray() {}
