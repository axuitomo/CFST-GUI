//go:build !webui

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wailsruntime "github.com/axuitomo/CFST-GUI/internal/app/wailsruntime"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const desktopSingleInstanceID = "io.github.axuitomo.cfst-gui"

// devSingleInstanceIDPrefix 是 `wails3 dev` 拉起的进程使用的单实例标识前缀。
//
// 发行版与开发版必须分开，否则 Windows 上会撞命名互斥体：
// Wails v3 的单实例锁是 `wails-app-<UniqueID>-sim` 命名互斥体，第二个进程
// acquire 失败后只会通知第一个实例 ShowMainWindow 并 os.Exit(0)
// （见 wails v3 pkg/application/single_instance_windows.go 与 application.go 的
// alreadyRunningError 分支）。开发进程在 application.New 阶段就退出，根本走不到
// assetserver 的 dev server 检查，于是 `wails3 dev` 看着像没生效——窗口里还是老实例
// 那份内嵌的 frontend/dist。托盘常驻的发行版会让这个现象每次必现。
//
// 判定依据沿用 Wails 自己的开发模式契约：`wails3 dev` 会向被拉起的进程导出
// FRONTEND_DEVSERVER_URL，assetserver 正是用它决定是否把资源请求代理到 Vite
// （wails v3 internal/assetserver/build_dev.go）。用同一个信号，开发/发行判定不会跑偏。
const devSingleInstanceIDPrefix = desktopSingleInstanceID + "-dev"

// singleInstanceID 返回本次进程要占用的单实例标识。
//
// 开发态还要再带上 PID：只按「有 FRONTEND_DEVSERVER_URL」区分，只能把开发进程和
// 发行版/手工启动的实例分开，分不开两个开发进程。上一次 `wails3 dev` 被强杀时进程
// 未必立刻退出（窗口关闭走 hideOnClose，进程可能继续常驻并锁住 build/bin 下的 exe），
// 此时固定的开发标识会让新进程同样在 application.New 阶段退出，只把那个旧窗口抬到
// 前台——现象和撞发行版一模一样，都是「wails3 dev 显示旧前端」。带上 PID 后，新进程
// 总是自己开窗口，旧实例只可能是一个正在运行但不再更新的孤儿。
func singleInstanceID() string {
	if os.Getenv("FRONTEND_DEVSERVER_URL") != "" {
		return fmt.Sprintf("%s-%d", devSingleInstanceIDPrefix, os.Getpid())
	}
	return desktopSingleInstanceID
}

// webviewUserDataPath 返回显式的 WebView2 用户数据目录；返回空串表示沿用 Wails 的默认值。
//
// Wails v3 的默认值是 filepath.Join(os.Getenv("AppData"), exeName)
// （wails v3 internal/webview2/pkg/edge/chromium.go 的 Embed），且对拼接结果不做任何校验。
// 进程环境里没有 APPDATA 时（部分启动器、计划任务、CI、agent shell 会剥掉它），
// 拼接结果退化成裸文件名，WebView2 会按「exe 所在目录」解析它，最终指向 exe 文件自身
// ——于是弹出「Microsoft Edge 无法读取和写入其数据目录」，controller 建不出来、
// 窗口永远不出现。Wails 自己对这条路径的注释写得很直白：
// 「If the path is not valid, a messagebox will be displayed with the error and the app
// will exit with error code.」
//
// 实测（本机 Go / Windows）：
//   - APPDATA 缺失时 os.UserConfigDir() 直接返回错误「%AppData% is not defined」，
//     并不回退到 %USERPROFILE%；
//   - APPDATA 是未展开的 %USERPROFILE%\AppData\Roaming 字面量时，UserConfigDir 会原样
//     返回它（Go 只判空、不判 %），这种路径交给 WebView2 同样会失败。
//
// 所以两个条件都要挡。只在默认值必然坏掉时才兜底：正常路径（有可用的 APPDATA）返回空串，
// 行为完全不变，避免换目录把 WebView2 里已有的 localStorage 与缓存重置掉。
func webviewUserDataPath() string {
	dir := os.Getenv("AppData")
	if strings.TrimSpace(dir) != "" && !strings.Contains(dir, "%") {
		return ""
	}
	return filepath.Join(fallbackDataRoot(), "webview2")
}

// fallbackDataRoot 在 %APPDATA% 不可用时挑一个绝对且可写的根目录。
//
// 不锚在进程当前目录上：`wails3 dev` 下 exe 的 cwd 由 refresh 引擎决定，双击 exe 时是
// exe 所在目录，计划任务里可能是 system32——锚在 cwd 上会让 WebView2 的 profile 随启动
// 方式漂移，每次都像首次运行；在项目根启动还会在仓库里造出一个数据目录。用户主目录
// （USERPROFILE，实测在 APPDATA 缺失时仍可用）是稳定的锚点；连它也拿不到时退到系统
// 临时目录，那是最后的可用位置。
func fallbackDataRoot() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".cfst-gui")
	}
	if tmp := os.TempDir(); strings.TrimSpace(tmp) != "" {
		return filepath.Join(tmp, "cfst-gui")
	}
	root := "cfst-gui"
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return root
}

func runGUI() {
	// 先标明本次进程用的是 Vite 实时源码还是内嵌快照，避免把「内嵌回退」误判成热更新失效。
	logFrontendSource()
	// dev 模式反代 Vite，内嵌 dist 允许只是占位；只有真正依赖内嵌资源（发行/`go run`）时
	// 才校验入口是否存在，缺失时给出可操作的构建指引，而不是渲染空白或陈旧页面。
	if !frontendUsesDevServer() {
		if err := verifyEmbeddedFrontend(runtimeResources.FrontendAssets); err != nil {
			fmt.Println("前端资源校验失败:", err)
		}
	}

	app := NewApp()
	wailsApp := application.New(application.Options{
		Name:        "CFST-GUI",
		Description: "Cloudflare/CDN IP 测速工具",
		Icon:        runtimeResources.AppPNGIcon,
		Windows: application.WindowsOptions{
			WebviewUserDataPath: webviewUserDataPath(),
		},
		Assets: application.AssetOptions{
			// 外层补缓存策略：index.html no-cache、内容哈希的 /assets/* immutable，
			// 防止覆盖安装新版本后 WebView2 仍读旧入口而显示旧前端。
			Handler: withFrontendCacheHeaders(
				application.BundledAssetFileServer(runtimeResources.FrontendAssets),
			),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: singleInstanceID(),
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				app.ShowMainWindow()
			},
		},
	})
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
