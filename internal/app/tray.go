//go:build !webui

package app

import (
	"context"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
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
	if a.window != nil {
		a.window.Hide()
	}
	return true
}

func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx = ctx
	_, _ = appcore.RunStorageMigration(a.core.StorageLayout(), time.Now())
	a.core.StartRuntimeCleanup(ctx)
	a.startTray()
	a.reloadSchedulerFromDisk()
	return nil
}

func (a *App) ServiceShutdown() error {
	a.core.StopRuntimeCleanup()
	a.stopScheduler()
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
