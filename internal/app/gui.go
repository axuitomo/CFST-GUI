//go:build !webui

package app

import (
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

const desktopSingleInstanceID = "io.github.axuitomo.cfst-gui"

func runGUI() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "CFST-GUI",
		Frameless:        true,
		Width:            1180,
		Height:           760,
		MinWidth:         360,
		MinHeight:        640,
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets: runtimeResources.FrontendAssets,
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
