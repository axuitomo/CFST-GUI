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

	"github.com/axuitomo/CFST-GUI/internal/appcore"
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
func Install(payload []byte, self string, opts Options, emit Progress) (retErr error) {
	registered, err := registeredInstallDir()
	if err != nil {
		return err
	}
	dir, err := validateInstallDir(opts.Dir, registered)
	if err != nil {
		return err
	}
	if len(payload) == 0 {
		return errNoPayload
	}
	uninstaller, err := os.ReadFile(self)
	if err != nil {
		return fmt.Errorf("read uninstaller: %w", err)
	}
	programs, err := windowsStartMenuDir()
	if err != nil {
		return fmt.Errorf("locate Start Menu: %w", err)
	}
	target := filepath.Join(dir, appExeName)
	legalFiles, legalErr := installerLegalFiles(payloadFS)
	paths := []string{target, filepath.Join(dir, uninstName), filepath.Join(programs, appName+".lnk")}
	if opts.DesktopShortcut {
		desktop, err := windowsDesktopDir()
		if err != nil {
			return fmt.Errorf("locate desktop: %w", err)
		}
		paths = append(paths, filepath.Join(desktop, appName+".lnk"))
	}
	for _, name := range installerLegalFileNames {
		paths = append(paths, filepath.Join(dir, name))
	}
	states, err := appcore.CaptureFileStates(paths...)
	if err != nil {
		return err
	}
	for _, state := range states {
		if state.SymlinkTarget != "" {
			return fmt.Errorf("refusing to replace linked install file: %s", state.Path)
		}
	}
	_, dirErr := os.Stat(dir)
	createdDir := errors.Is(dirErr, os.ErrNotExist)
	var changed []appcore.FileState
	committed := false
	defer func() {
		if retErr == nil || committed {
			return
		}
		retErr = errors.Join(retErr, appcore.RestoreFileStates(changed))
		if createdDir {
			_ = os.Remove(dir) // Only remove an empty directory created by this attempt.
		}
	}()
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
	if err := appcore.WriteFileAtomic(target, payload, 0o755); err != nil {
		return fmt.Errorf("write application: %w", err)
	}
	changed = append(changed, states[0])
	if err := emit(55, "Installing uninstaller"); err != nil {
		return err
	}
	if err := appcore.WriteFileAtomic(paths[1], uninstaller, 0o755); err != nil {
		return fmt.Errorf("install uninstaller: %w", err)
	}
	changed = append(changed, states[1])
	if err := emit(62, "Creating Start Menu shortcut"); err != nil {
		return err
	}
	if legalErr != nil {
		return fmt.Errorf("read installer legal files: %w", legalErr)
	}
	changed = append(changed, states[2])
	if err := os.MkdirAll(programs, 0o755); err != nil {
		return err
	}
	if err := createShortcut(paths[2], target, target); err != nil {
		return fmt.Errorf("create Start Menu shortcut: %w", err)
	}
	if opts.DesktopShortcut {
		if err := emit(70, "Creating desktop shortcut... "); err != nil {
			return err
		}
		changed = append(changed, states[3])
		if err := createShortcut(paths[3], target, target); err != nil {
			return fmt.Errorf("create desktop shortcut: %w", err)
		}
	}
	if err := emit(75, "Installing license notices"); err != nil {
		return err
	}

	legalStateStart := len(states) - len(legalFiles)
	for i, file := range legalFiles {
		if err := appcore.WriteFileAtomic(paths[legalStateStart+i], file.data, 0o644); err != nil {
			return fmt.Errorf("install %s: %w", file.name, err)
		}
		changed = append(changed, states[legalStateStart+i])
	}
	if err := emit(82, "Registering uninstall entry in HKLM... "); err != nil {
		return err
	}
	if err := writeUninstallEntry(dir); err != nil {
		return fmt.Errorf("write uninstall entry: %w", err)
	}
	// Cancellation after the final commit must not undo an installed application.
	committed = true
	_ = emit(96, "Finishing up")
	_ = emit(100, fmt.Sprintf("Done in %.2fs", time.Since(start).Seconds()))
	if opts.Launch && !opts.Silent {
		if err := launch(target); err != nil {
			return fmt.Errorf("installed, but launch application: %w", err)
		}
	}
	return nil
}

// Uninstall 验证注册位置，只删除应用文件；失败时保留系统卸载入口供重试。
func Uninstall(dir, self string, emit Progress) error {
	registered, err := registeredInstallDir()
	if err != nil {
		return err
	}
	if err := validateUninstallLocation(dir, self, registered); err != nil {
		return err
	}
	if err := emit(10, "Removing application files"); err != nil {
		return err
	}
	if err := uninstallFiles(dir, self); err != nil {
		return err
	}
	if err := emit(70, "Unregistering from Apps & Features"); err != nil {
		return err
	}
	if err := removeUninstallEntry(); err != nil {
		return err
	}
	for _, folder := range []*windows.KNOWNFOLDERID{
		windows.FOLDERID_Desktop, windows.FOLDERID_Programs,
		windows.FOLDERID_PublicDesktop, windows.FOLDERID_CommonPrograms,
	} {
		if path, err := windows.KnownFolderPath(folder, 0); err == nil {
			_ = os.Remove(filepath.Join(path, appName+".lnk"))
		}
	}
	return emit(100, "Done")
}

// uninstallFiles 不清空目录，保留用户文件及运行中的卸载器。
func uninstallFiles(dir, self string) error {
	target := filepath.Join(dir, appExeName)
	if strings.EqualFold(target, self) {
		return errors.New("refusing to remove the running application")
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", appExeName, err)
	}
	for _, name := range installerLegalFileNames {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", name, err)
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

func writeUninstallEntry(dir string) (retErr error) {
	key, existed, err := registry.CreateKey(registry.LOCAL_MACHINE, uninstKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() {
		_ = key.Close()
		if retErr != nil && !existed {
			retErr = errors.Join(retErr, registry.DeleteKey(registry.LOCAL_MACHINE, uninstKey))
		}
	}()

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
	return updateUninstallValues(key, values, uint32(sizeKB))
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

func windowsDesktopDir() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_PublicDesktop, 0)
}

func windowsStartMenuDir() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_CommonPrograms, 0)
}
