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

// webviewUserDataPath 返回显式的 WebView2 用户数据目录（%APPDATA%\CFST-GUI\webview2）。
//
// Wails v3 的默认值是 filepath.Join(os.Getenv("AppData"), exeName)
// （wails v3 internal/webview2/pkg/edge/chromium.go 的 Embed），exeName 取自
// filepath.Base(os.Executable())，在 Windows 上带 .exe 后缀，数据目录因此落成
// %APPDATA%\CFST-GUI.exe，与应用数据目录 %APPDATA%\CFST-GUI 不同名、同一次安装下
// 出现两个目录；而且 Wails 对拼接结果不做任何校验：进程环境里没有 APPDATA 时（部分
// 启动器、计划任务、CI、agent shell 会剥掉它），拼接结果退化成裸文件名，WebView2 会按
// 「exe 所在目录」解析它，最终指向 exe 文件自身——于是弹出「Microsoft Edge 无法读取和
// 写入其数据目录」，controller 建不出来、窗口永远不出现。Wails 自己对这条路径的注释写
// 得很直白：「If the path is not valid, a messagebox will be displayed with the error
// and the app will exit with error code.」
//
// 实测（本机 Go / Windows）：APPDATA 缺失时 os.UserConfigDir() 直接返回错误
// 「%AppData% is not defined」，并不回退到 %USERPROFILE%；APPDATA 是未展开的
// %USERPROFILE%\AppData\Roaming 字面量时 UserConfigDir 会原样返回它（Go 只判空、
// 不判 %），这种路径交给 WebView2 同样会失败。三种情况统一交给 storage.go 的目录兜底，
// 桌面数据目录与 WebView2 profile 都挂在同一个 %APPDATA%\CFST-GUI 下（开发版
// build/bin/cfst-gui-dev.exe 因此与发行版共用同一 profile，和它们本来就共用该目录下的
// 配置一致；两者同时常驻时按 WebView2 自身的用户数据目录规则共享同一浏览器进程）。
func webviewUserDataPath() string {
	target := filepath.Join(defaultStorageDir(), "webview2")
	migrateLegacyWebviewUserData(target)
	return target
}

// migrateLegacyWebviewUserData 把 Wails 默认目录（%APPDATA%\<exe 名>，带 .exe 后缀）
// 一次性搬到新的 webview2 目录：那里存着 WebView2 的 localStorage 与缓存，换目录不搬
// 等于让老用户回到首次运行状态（侧栏折叠、结果页偏好等都要重设）。目标目录已存在、
// 老目录不存在、或搬迁失败（跨卷、目录被占用）时都保持现状，让 WebView2 新建 profile。
func migrateLegacyWebviewUserData(target string) {
	if _, err := os.Stat(target); err == nil {
		return
	}
	appData := strings.TrimSpace(os.Getenv("AppData"))
	if appData == "" || strings.Contains(appData, "%") {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	legacy := filepath.Join(appData, filepath.Base(exe))
	if legacy == target {
		return
	}
	info, err := os.Stat(legacy)
	if err != nil || !info.IsDir() {
		return
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return
	}
	_ = os.Rename(legacy, target)
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
