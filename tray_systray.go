//go:build tray && !darwin

package main

import (
	"fmt"

	"github.com/getlantern/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startTray() {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				fmt.Println("托盘初始化失败:", recovered)
				a.setTrayAvailable(false)
			}
		}()
		systray.Run(func() { a.onTrayReady() }, func() { a.setTrayAvailable(false) })
	}()
}

func (a *App) onTrayReady() {
	a.setTrayAvailable(true)
	systray.SetTitle("CFST-GUI")
	systray.SetTooltip("CFST-GUI 正在后台运行")
	openItem := systray.AddMenuItem("打开主界面", "打开 CFST-GUI 主界面")
	quitItem := systray.AddMenuItem("关闭软件", "退出 CFST-GUI")
	go func() {
		for {
			select {
			case <-openItem.ClickedCh:
				if a.ctx != nil {
					wailsruntime.WindowShow(a.ctx)
				}
			case <-quitItem.ClickedCh:
				a.markQuitting()
				if a.ctx != nil {
					wailsruntime.Quit(a.ctx)
				}
				return
			}
		}
	}()
}

func (a *App) stopTray() {
	if a.trayIsAvailable() {
		systray.Quit()
	}
}
