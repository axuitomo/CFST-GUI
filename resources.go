package main

import "github.com/axuitomo/CFST-GUI/internal/app"

func runtimeResources() app.Resources {
	trayPNGIcon, trayWindowsIcon := trayIconResources()
	return app.Resources{
		FrontendAssets:  frontendAssets,
		TrayPNGIcon:     trayPNGIcon,
		TrayWindowsIcon: trayWindowsIcon,
	}
}
