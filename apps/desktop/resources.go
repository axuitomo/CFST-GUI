package main

import (
	buildassets "github.com/axuitomo/CFST-GUI/build"
	"github.com/axuitomo/CFST-GUI/internal/adapters/host"
	frontendassets "github.com/axuitomo/CFST-GUI/packages/frontend"
)

func runtimeResources() host.Resources {
	trayPNGIcon, trayWindowsIcon := buildassets.TrayIconResources()
	return host.Resources{
		AppPNGIcon:      buildassets.AppIcon,
		FrontendAssets:  frontendassets.Dist,
		TrayPNGIcon:     trayPNGIcon,
		TrayWindowsIcon: trayWindowsIcon,
	}
}
