package desktop

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (a *App) hideOnClose() bool {
	a.trayMu.Lock()
	quitting := a.quitting
	trayAvailable := a.trayAvailable
	a.trayMu.Unlock()
	if quitting || !trayAvailable {
		return false
	}
	if desktopWindow != nil {
		desktopWindow.Hide()
	}
	return true
}

func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx = ctx
	a.host.PrepareRuntime(ctx)
	a.startTray()
	a.host.ReloadSchedulerFromDisk()
	return nil
}

func (a *App) ServiceShutdown() error {
	a.host.Shutdown()
	a.stopTray()
	return nil
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
