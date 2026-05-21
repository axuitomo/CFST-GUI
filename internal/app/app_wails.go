//go:build !webui

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) scheduleQuitAfterUpdate() {
	go func() {
		time.Sleep(200 * time.Millisecond)
		a.markQuitting()
		if a.ctx != nil {
			wailsruntime.Quit(a.ctx)
		}
	}()
}

func (a *App) ShowMainWindow() DesktopCommandResult {
	if a.ctx == nil {
		return desktopCommandResult("WINDOW_UNAVAILABLE", nil, "主窗口尚未初始化。", false, nil, nil)
	}
	wailsruntime.WindowShow(a.ctx)
	return desktopCommandResult("WINDOW_SHOWN", nil, "主界面已打开。", true, nil, nil)
}

func (a *App) HideMainWindow() DesktopCommandResult {
	if a.ctx == nil {
		return desktopCommandResult("WINDOW_UNAVAILABLE", nil, "主窗口尚未初始化。", false, nil, nil)
	}
	wailsruntime.WindowHide(a.ctx)
	return desktopCommandResult("WINDOW_HIDDEN", nil, "主界面已隐藏。", true, nil, nil)
}

func (a *App) QuitApplication() DesktopCommandResult {
	a.markQuitting()
	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
	return desktopCommandResult("APP_QUIT_REQUESTED", nil, "已请求关闭软件。", true, nil, nil)
}

func (a *App) OpenPath(targetPath string) error {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return nil
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetPath)
	case "darwin":
		cmd = exec.Command("open", targetPath)
	default:
		cmd = exec.Command("xdg-open", targetPath)
	}
	return cmd.Start()
}

func (a *App) SelectPath(payload map[string]any) DesktopCommandResult {
	if a.ctx == nil {
		return desktopCommandResult("PATH_DIALOG_UNAVAILABLE", nil, "系统文件选择器尚未初始化。", false, nil, nil)
	}

	mode := normalizePathSelectionMode(stringValue(firstNonNil(payload["mode"], payload["kind"]), ""))
	currentPath := strings.TrimSpace(stringValue(firstNonNil(payload["current_path"], payload["currentPath"]), ""))
	defaultFileName := strings.TrimSpace(stringValue(firstNonNil(payload["default_file_name"], payload["defaultFileName"]), ""))
	title := strings.TrimSpace(stringValue(payload["title"], ""))
	defaultDir := selectPathDefaultDirectory(currentPath)

	data := map[string]any{
		"canceled": false,
		"mode":     mode,
	}
	cancel := func(message string) DesktopCommandResult {
		data["canceled"] = true
		return desktopCommandResult("PATH_SELECTION_CANCELED", data, message, true, nil, nil)
	}
	if mode == "storage_dir" {
		data["path"] = storageRoot()
		data["directory"] = storageRoot()
		return desktopCommandResult("PATH_SELECTION_DEPRECATED", data, "当前版本不再支持自定义储存目录，已固定使用应用数据目录。", true, nil, nil)
	}

	switch mode {
	case "export_target", "export_dir", "directory":
		if title == "" {
			title = "选择导出目录"
		}
		selected, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
			Title:            title,
			DefaultDirectory: defaultDir,
		})
		if err != nil {
			return desktopCommandResult("PATH_SELECTION_FAILED", nil, err.Error(), false, nil, nil)
		}
		if strings.TrimSpace(selected) == "" {
			return cancel("已取消选择导出目录。")
		}
		data["path"] = selected
		data["directory"] = selected
		return desktopCommandResult("PATH_SELECTED", data, "已选择导出目录。", true, nil, nil)

	case "config_import", "import_config", "config_archive_import":
		if title == "" {
			if mode == "config_archive_import" {
				title = "加载配置压缩包"
			} else {
				title = "导入配置文件"
			}
		}
		filters := []wailsruntime.FileFilter{
			{DisplayName: "JSON 配置文件 (*.json)", Pattern: "*.json"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		}
		if mode == "config_archive_import" {
			filters = []wailsruntime.FileFilter{
				{DisplayName: "配置压缩包 (*.zip)", Pattern: "*.zip"},
				{DisplayName: "JSON 配置文件 (*.json)", Pattern: "*.json"},
				{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
			}
		}
		selected, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
			Title:            title,
			DefaultDirectory: defaultDir,
			Filters:          filters,
		})
		if err != nil {
			return desktopCommandResult("PATH_SELECTION_FAILED", nil, err.Error(), false, nil, nil)
		}
		if strings.TrimSpace(selected) == "" {
			return cancel("已取消导入配置。")
		}
		raw, err := os.ReadFile(selected)
		if err != nil {
			return desktopCommandResult("CONFIG_IMPORT_READ_FAILED", nil, err.Error(), false, nil, nil)
		}
		data["path"] = selected
		if mode == "config_archive_import" {
			return desktopCommandResult("PATH_SELECTED", data, "已选择配置压缩包。", true, nil, nil)
		}
		data["content"] = string(raw)
		return desktopCommandResult("PATH_SELECTED", data, "已读取配置文件。", true, nil, nil)

	case "export_file", "save_file", "config_export", "config_archive_export":
		if title == "" {
			if mode == "config_export" || mode == "config_archive_export" {
				title = "导出配置文件"
			} else {
				title = "选择导出文件"
			}
		}
		if defaultFileName == "" {
			if mode == "config_archive_export" {
				defaultFileName = fmt.Sprintf("cfst-gui-config-%s.zip", time.Now().Format("20060102-150405"))
			} else if mode == "config_export" {
				defaultFileName = fmt.Sprintf("cfst-gui-config-%s.json", time.Now().Format("20060102-150405"))
			} else {
				defaultFileName = "result.csv"
			}
		}
		filters := []wailsruntime.FileFilter{
			{DisplayName: "CSV 文件 (*.csv)", Pattern: "*.csv"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		}
		if mode == "config_export" {
			filters = []wailsruntime.FileFilter{
				{DisplayName: "JSON 配置文件 (*.json)", Pattern: "*.json"},
				{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
			}
		} else if mode == "config_archive_export" {
			filters = []wailsruntime.FileFilter{
				{DisplayName: "配置压缩包 (*.zip)", Pattern: "*.zip"},
				{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
			}
		}
		selected, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
			Title:            title,
			DefaultDirectory: defaultDir,
			DefaultFilename:  defaultFileName,
			Filters:          filters,
		})
		if err != nil {
			return desktopCommandResult("PATH_SELECTION_FAILED", nil, err.Error(), false, nil, nil)
		}
		if strings.TrimSpace(selected) == "" {
			return cancel("已取消选择导出文件。")
		}
		data["path"] = selected
		data["directory"] = filepath.Dir(selected)
		data["file_name"] = filepath.Base(selected)
		return desktopCommandResult("PATH_SELECTED", data, "已选择导出文件。", true, nil, nil)

	default:
		if title == "" {
			title = "选择输入源文件"
		}
		selected, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
			Title:            title,
			DefaultDirectory: defaultDir,
			Filters: []wailsruntime.FileFilter{
				{DisplayName: "文本/CSV 文件 (*.txt, *.csv)", Pattern: "*.txt;*.csv"},
				{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
			},
		})
		if err != nil {
			return desktopCommandResult("PATH_SELECTION_FAILED", nil, err.Error(), false, nil, nil)
		}
		if strings.TrimSpace(selected) == "" {
			return cancel("已取消选择输入源文件。")
		}
		data["path"] = selected
		return desktopCommandResult("PATH_SELECTED", data, "已选择输入源文件。", true, nil, nil)
	}
}
