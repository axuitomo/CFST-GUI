//go:build webui

package app

func (a *App) markQuitting() {
	a.trayMu.Lock()
	a.quitting = true
	a.trayMu.Unlock()
}

func (a *App) setTrayAvailable(available bool) {
	a.trayMu.Lock()
	a.trayAvailable = available
	a.trayMu.Unlock()
}

func (a *App) trayIsAvailable() bool {
	a.trayMu.Lock()
	defer a.trayMu.Unlock()
	return a.trayAvailable
}
