package main

import (
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

const desktopSingleInstanceID = "io.github.axuitomo.cfst-gui"

func runGUI() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "CFST-GUI",
		Width:     1180,
		Height:    760,
		MinWidth:  960,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: desktopSingleInstanceID,
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				app.ShowMainWindow()
			},
		},
		Bind: []any{
			app,
		},
	})
	if err != nil {
		fmt.Println("Wails 启动失败:", err)
	}
}
