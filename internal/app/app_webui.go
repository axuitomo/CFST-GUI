//go:build webui

package app

import "strings"

func (a *App) scheduleQuitAfterUpdate() {}

func (a *App) ShowMainWindow() DesktopCommandResult {
	return desktopCommandResult("WINDOW_WEBUI_UNAVAILABLE", nil, "WebUI 模式不使用桌面窗口。", true, nil, nil)
}

func (a *App) HideMainWindow() DesktopCommandResult {
	return desktopCommandResult("WINDOW_WEBUI_UNAVAILABLE", nil, "WebUI 模式不使用桌面窗口。", true, nil, nil)
}

func (a *App) QuitApplication() DesktopCommandResult {
	a.markQuitting()
	return desktopCommandResult("APP_QUIT_REQUESTED", nil, "WebUI 模式已收到关闭请求，请通过 Docker Compose 管理服务生命周期。", true, nil, nil)
}

func (a *App) OpenPath(targetPath string) error {
	_ = strings.TrimSpace(targetPath)
	return nil
}

func (a *App) SelectPath(payload map[string]any) DesktopCommandResult {
	mode := normalizePathSelectionMode(stringValue(firstNonNil(payload["mode"], payload["kind"]), ""))
	return desktopCommandResult("PATH_SELECTION_WEBUI", map[string]any{
		"canceled": true,
		"mode":     mode,
	}, "WebUI 使用浏览器文件选择和服务端文件浏览。", true, nil, nil)
}
