package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) beforeClose(ctx context.Context) bool {
	a.trayMu.Lock()
	quitting := a.quitting
	trayAvailable := a.trayAvailable
	a.trayMu.Unlock()

	if quitting || !trayAvailable {
		return false
	}
	wailsruntime.WindowHide(ctx)
	return true
}

func (a *App) shutdown(ctx context.Context) {
	_ = ctx
	a.stopTray()
}

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
