// Package main 实现 CFST-GUI 的安装器。
//
// 结构：
//   - install.go   纯安装逻辑（解压、注册表、启动），可单测
//   - shortcut.go  .lnk 创建（WScript.Shell COM）
//   - precheck.go  WebView2 运行时检测与原生提示
//   - main.go      参数解析、静默路径、Wails 窗口与 HTTP 接口
//
// 设计约束：
//   - 安装器自身依赖 WebView2 渲染界面，而它的职责包含检测 WebView2，
//     所以检测与提示必须在创建窗口之前用原生 MessageBox 完成。
//   - /S 与 --silent 走完全无窗口路径，保证 winget/choco 批量部署可用。
//   - 同一份二进制既是安装器也是卸载器：装好时复制自身为 uninstall.exe，
//     带 --uninstall 启动即执行卸载，UI 与安装器完全一致。
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	appName    = "CFST-GUI"
	appExeName = "cfst-gui.exe"
	uninstName = "uninstall.exe"
	publisher  = "axuitomo"
	uninstKey  = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\axuitomoCFST-GUI`
)

// Options 是一次安装的全部输入，来自界面或命令行。
type Options struct {
	Dir             string
	DesktopShortcut bool
	Launch          bool
	Silent          bool
}

// Progress 由一个事件回调消费；percent 为 0-100，line 为控制台输出。
type Progress func(percent int, line string) error

// ErrCancelled 表示用户在中途取消。
var ErrCancelled = errors.New("cancelled")

func defaultInstallDir() string {
	base := os.Getenv("ProgramFiles")
	if base == "" {
		base = `C:\Program Files`
	}
	return filepath.Join(base, publisher, appName)
}

// 安装流程：落盘 → 快捷方式 → 卸载项 → 可选启动。
// payload 是内嵌的主程序字节；self 是安装器自身路径，用来放卸载器副本。
func Install(payload []byte, self string, opts Options, emit Progress) error {
	if strings.TrimSpace(opts.Dir) == "" {
		return errors.New("install dir is empty")
	}
	dir := filepath.Clean(opts.Dir)
	start := time.Now()

	if err := emit(2, "Preparing "+dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	if err := emit(10, "Extracting "+appExeName+" -> "+dir+`\`); err != nil {
		return err
	}
	target := filepath.Join(dir, appExeName)
	if err := os.WriteFile(target, payload, 0o755); err != nil {
		return fmt.Errorf("write application: %w", err)
	}

	if err := emit(55, "Installing uninstaller"); err != nil {
		return err
	}
	if err := copySelf(self, filepath.Join(dir, uninstName)); err != nil {
		return fmt.Errorf("install uninstaller: %w", err)
	}

	if err := emit(62, "Creating Start Menu shortcut"); err != nil {
		return err
	}
	if programs, err := windowsStartMenuDir(); err == nil {
		if err := os.MkdirAll(programs, 0o755); err == nil {
			if err := createShortcut(filepath.Join(programs, appName+".lnk"), target, target); err != nil {
				return fmt.Errorf("create Start Menu shortcut: %w", err)
			}
		}
	}

	if opts.DesktopShortcut {
		if err := emit(70, "Creating desktop shortcut... "); err != nil {
			return err
		}
		desktop, err := windowsDesktopDir()
		if err != nil {
			return fmt.Errorf("locate desktop: %w", err)
		}
		if err := createShortcut(filepath.Join(desktop, appName+".lnk"), target, target); err != nil {
			return fmt.Errorf("create desktop shortcut: %w", err)
		}
	}

	if err := emit(82, "Registering uninstall entry in HKLM... "); err != nil {
		return err
	}
	if err := writeUninstallEntry(dir); err != nil {
		return fmt.Errorf("write uninstall entry: %w", err)
	}

	if err := emit(96, "Finishing up"); err != nil {
		return err
	}
	if err := emit(100, fmt.Sprintf("Done in %.2fs", time.Since(start).Seconds())); err != nil {
		return err
	}

	if opts.Launch && !opts.Silent {
		if err := launch(target); err != nil {
			return fmt.Errorf("launch application: %w", err)
		}
	}
	return nil
}

// Uninstall 依次删除注册表项、快捷方式、安装目录。
//
// 顺序是有意的：注册表项先删。文件删除可能因为「正在运行的卸载器自身」
// 而失败，如果把注册表留到最后，用户就会在「应用和功能」里看到一个
// 点不掉的条目 —— 实测正是这个顺序问题导致卸载返回 1 且条目残留。
func Uninstall(dir, self string, emit Progress) error {
	dir = filepath.Clean(dir)

	if err := emit(10, "Unregistering from Apps & Features"); err != nil {
		return err
	}
	if err := removeUninstallEntry(); err != nil {
		return err
	}

	if err := emit(40, "Removing shortcuts"); err != nil {
		return err
	}
	if desktop, err := windowsDesktopDir(); err == nil {
		_ = os.Remove(filepath.Join(desktop, appName+".lnk"))
	}
	if programs, err := windowsStartMenuDir(); err == nil {
		_ = os.Remove(filepath.Join(programs, appName+".lnk"))
	}
	// 旧 NSIS 版把快捷方式放在所有用户目录，升级后的卸载也需要清理它们。
	for _, folder := range []*windows.KNOWNFOLDERID{windows.FOLDERID_PublicDesktop, windows.FOLDERID_CommonPrograms} {
		if path, err := windows.KnownFolderPath(folder, 0); err == nil {
			_ = os.Remove(filepath.Join(path, appName+".lnk"))
		}
	}

	if err := emit(70, "Deleting install directory"); err != nil {
		return err
	}
	if err := uninstallFiles(dir, self); err != nil {
		return err
	}

	return emit(100, "Done")
}

// uninstallFiles 删除安装目录里除卸载器自身以外的内容。
// 正在运行的可执行文件删不掉，这是 Windows 的硬限制；
// 目录与它的清理由调用方通过一个短命外部进程延后完成。
func uninstallFiles(dir, self string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read install dir: %w", err)
	}
	for _, entry := range entries {
		target := filepath.Join(dir, entry.Name())
		if self != "" && strings.EqualFold(target, self) {
			continue
		}
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("remove %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func removeUninstallEntry() error {
	if err := registry.DeleteKey(registry.LOCAL_MACHINE, uninstKey); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("delete uninstall entry: %w", err)
	}
	return nil
}

func writeUninstallEntry(dir string) error {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, uninstKey, registry.WRITE)
	if err != nil {
		return err
	}
	defer key.Close()

	uninstExe := filepath.Join(dir, uninstName)
	sizeKB := estimateSizeKB(dir)
	values := map[string]string{
		"Publisher":            publisher,
		"DisplayName":          appName,
		"DisplayVersion":       version,
		"DisplayIcon":          filepath.Join(dir, appExeName),
		"UninstallString":      `"` + uninstExe + `" --uninstall`,
		"QuietUninstallString": `"` + uninstExe + `" --uninstall --silent`,
		"InstallLocation":      dir,
	}
	for name, value := range values {
		if err := key.SetStringValue(name, value); err != nil {
			return err
		}
	}
	return key.SetDWordValue("EstimatedSize", uint32(sizeKB))
}

func estimateSizeKB(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total / 1024
}

func copySelf(self, dst string) error {
	// 卸载器就是安装器自己，避免为卸载单独维护一套二进制。
	if self == "" {
		return errors.New("self path is unknown")
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

func windowsDesktopDir() (string, error) {
	if profile := os.Getenv("USERPROFILE"); profile != "" {
		return filepath.Join(profile, "Desktop"), nil
	}
	return "", errors.New("USERPROFILE is not set")
}

func windowsStartMenuDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", errors.New("APPDATA is not set")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs"), nil
}
