//go:build !webui

package app

import (
	"fmt"

	wailsruntime "github.com/axuitomo/CFST-GUI/internal/app/wailsruntime"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const desktopSingleInstanceID = "io.github.axuitomo.cfst-gui"

func runGUI() {
	app := NewApp()
	wailsApp := application.New(application.Options{
		Name:        "CFST-GUI",
		Description: "Cloudflare/CDN IP 测速工具",
		Icon:        runtimeResources.AppPNGIcon,
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(runtimeResources.FrontendAssets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: desktopSingleInstanceID,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				app.ShowMainWindow()
			},
		},
	})
	app.wailsApp = wailsApp
	desktopWailsApp = wailsApp
	wailsApp.RegisterService(application.NewService(app))

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:       "main",
		Title:      "CFST-GUI",
		Frameless:  true,
		Width:      1180,
		Height:     760,
		MinWidth:   360,
		MinHeight:  640,
		StartState: application.WindowStateMaximised,
		URL:        "/",
	})
	app.window = window
	desktopWindow = window
	wailsruntime.SetApplication(wailsApp, window)
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if app.hideOnClose() {
			event.Cancel()
		}
	})

	if err := wailsApp.Run(); err != nil {
		fmt.Println("Wails 启动失败:", err)
	}
}
