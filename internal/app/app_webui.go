//go:build webui

package app

import (
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/configvalue"
)

func (a *App) scheduleQuitAfterUpdate() {}

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
	logDir := logDirectoryPath()
	return appcore.NewCommandResult("LOG_DIRECTORY_WEBUI", map[string]any{
		"directory": logDir,
		"path":      logDir,
	}, "WebUI 模式请在服务端日志目录查看。", true, nil, nil)

}

func (a *App) SelectPath(payload map[string]any) appcore.CommandResult {
	mode := normalizePathSelectionMode(configvalue.String(configvalue.FirstNonNil(payload["mode"], payload["kind"]), ""))
	return appcore.NewCommandResult("PATH_SELECTION_WEBUI", map[string]any{
		"canceled": true,
		"mode":     mode,
	}, "WebUI 使用浏览器文件选择和服务端文件浏览。", true, nil, nil)

}
