//go:build tray && !darwin

package app

import (
	"fmt"
	"runtime"

	"github.com/getlantern/systray"
)

func (a *App) startTray() {
	a.trayStartOnce.Do(func() {
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer func() {
				if recovered := recover(); recovered != nil {
					fmt.Println("托盘初始化失败:", recovered)
					a.setTrayAvailable(false)
				}
			}()

			systray.Run(func() { a.onTrayReady() }, func() { a.setTrayAvailable(false) })
		}()
	})
}

func (a *App) onTrayReady() {
	a.setTrayAvailable(true)
	if icon := runtimeResources.trayIconBytes(); len(icon) > 0 {
		systray.SetIcon(icon)
	}
	systray.SetTitle("CFST-GUI")
	systray.SetTooltip("CFST-GUI 正在后台运行")
	openItem := systray.AddMenuItem("打开主界面", "打开 CFST-GUI 主界面")
	quitItem := systray.AddMenuItem("关闭软件", "退出 CFST-GUI")
	go func() {
		for {
			select {
			case <-openItem.ClickedCh:
				go a.ShowMainWindow()
			case <-quitItem.ClickedCh:
				go a.QuitApplication()
				return
			}
		}
	}()
}

func (a *App) stopTray() {
	a.trayStopOnce.Do(func() {
		if a.trayIsAvailable() {
			systray.Quit()
		}
	})
}
