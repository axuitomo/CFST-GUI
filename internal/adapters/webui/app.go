package webui

import (
	"context"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/axuitomo/CFST-GUI/internal/adapters/host"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore"
	configvalue "github.com/axuitomo/CFST-GUI/internal/kernel/domain/config"
)

type App struct {
	host     *host.Host
	eventHub *webUIEventHub
	quitting atomic.Bool
}

func NewApp() *App {
	app := &App{
		eventHub: newWebUIEventHub(),
	}
	app.host = host.NewHost(host.Options{
		InstallMode: currentInstallMode,
		OnEmitEvent: app.emitProbeEvent,
	})
	return app
}

func (a *App) Host() *host.Host {
	return a.host
}

func (a *App) startup(ctx context.Context) {
	a.host.PrepareRuntime(ctx)
	a.host.ReloadSchedulerFromDisk()
}

func (a *App) shutdown() {
	a.host.Shutdown()
}

func (a *App) markQuitting() {
	a.quitting.Store(true)
}

func currentInstallMode() string {
	if runtime.GOOS == "linux" {
		return "docker_compose"
	}
	return host.DefaultInstallMode(runtime.GOOS)
}

func (a *App) Invoke(command, payloadJSON string) string {
	return a.host.Invoke(command, payloadJSON)
}

func (a *App) GetHealth() host.HealthResult {
	return a.host.GetHealth()
}

func (a *App) GetAppInfo() appcore.CommandResult {
	return a.host.GetAppInfo()
}

func (a *App) CheckForUpdates(payload map[string]any) appcore.CommandResult {
	return a.host.CheckForUpdates(payload)
}

func (a *App) DownloadAndInstallUpdate(payload map[string]any) appcore.CommandResult {
	return a.host.DownloadAndInstallUpdate(payload)
}

func (a *App) OpenReleasePage() appcore.CommandResult {
	return a.host.OpenReleasePage()
}

func (a *App) ShowMainWindow() appcore.CommandResult {
	return appcore.NewCommandResult("WINDOW_WEBUI_UNAVAILABLE", nil, "WebUI 模式不使用桌面窗口。", true, nil, nil)
}

func (a *App) HideMainWindow() appcore.CommandResult {
	return appcore.NewCommandResult("WINDOW_WEBUI_UNAVAILABLE", nil, "WebUI 模式不使用桌面窗口。", true, nil, nil)
}

func (a *App) QuitApplication() appcore.CommandResult {
	a.markQuitting()
	return appcore.NewCommandResult("APP_QUIT_REQUESTED", nil, "WebUI 模式已收到关闭请求，请通过 Docker Compose 管理服务生命周期。", true, nil, nil)
}

func (a *App) OpenPath(targetPath string) error {
	_ = strings.TrimSpace(targetPath)
	return nil
}

func (a *App) OpenLogDirectory(payload map[string]any) appcore.CommandResult {
	_ = payload
	logDir := host.LogDirectoryPath()
	return appcore.NewCommandResult("LOG_DIRECTORY_WEBUI", map[string]any{
		"directory": logDir,
		"path":      logDir,
	}, "WebUI 模式请在服务端日志目录查看。", true, nil, nil)

}

func (a *App) SelectPath(payload map[string]any) appcore.CommandResult {
	mode := host.NormalizePathSelectionMode(configvalue.String(configvalue.FirstNonNil(payload["mode"], payload["kind"]), ""))
	return appcore.NewCommandResult("PATH_SELECTION_WEBUI", map[string]any{
		"canceled": true,
		"mode":     mode,
	}, "WebUI 使用浏览器文件选择和服务端文件浏览。", true, nil, nil)

}
