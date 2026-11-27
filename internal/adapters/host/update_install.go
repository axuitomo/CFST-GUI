package host

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func startInstallStrategy(mode, downloadedPath string) (string, error) {
	switch strings.TrimSpace(mode) {
	case "windows_msix":
		return startWindowsInstaller(downloadedPath)
	case "windows_exe":
		return startWindowsInstaller(downloadedPath)
	case "replace_binary":
		return startLinuxReplacement(downloadedPath)
	case "replace_app":
		return startDarwinReplacement(downloadedPath)
	case "docker_compose":
		return "manual", nil
	case "android_apk":
		return "manual", errors.New("Android APK 安装必须由 Android 原生插件触发")
	default:
		return "manual", OpenPathDetached(downloadedPath)
	}
}

func startWindowsInstaller(downloadedPath string) (string, error) {
	if runtime.GOOS != "windows" {
		return "manual", OpenPathDetached(downloadedPath)
	}
	if err := exec.Command(
		"powershell",
		"-NoProfile",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		buildWindowsInstallerCleanupCommand(downloadedPath, os.Getpid()),
	).Start(); err != nil {
		return "", err
	}
	return "windows_package_opened", nil
}

func buildWindowsInstallerCleanupCommand(downloadedPath string, ownerPID int) string {
	quotedPath := powerShellSingleQuote(downloadedPath)
	return fmt.Sprintf("$ErrorActionPreference = 'SilentlyContinue'; "+
		"$installer = Start-Process -FilePath %s -PassThru; "+
		"if ($installer) { Wait-Process -Id $installer.Id -ErrorAction SilentlyContinue }; "+
		"Wait-Process -Id %d -ErrorAction SilentlyContinue; "+
		"for ($i = 0; $i -lt 30; $i++) { "+
		"Remove-Item -LiteralPath %s -Force -ErrorAction SilentlyContinue; "+
		"if (-not (Test-Path -LiteralPath %s)) { break }; "+
		"Start-Sleep -Seconds 2 "+
		"}",
		quotedPath,
		ownerPID,
		quotedPath,
		quotedPath,
	)
}

func powerShellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func startLinuxReplacement(downloadedPath string) (string, error) {
	currentExe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := ensureWritableTarget(currentExe); err != nil {
		return "manual", OpenPathDetached(downloadedPath)
	}
	stamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	extractedBinary, cleanup, err := extractLinuxBinary(downloadedPath)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return "", err
	}
	replacementPath := filepath.Join(filepath.Dir(downloadedPath), "cfst-gui-update-binary-"+stamp)
	if err := copyUpdateFile(extractedBinary, replacementPath, 0o755); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return "", err
	}
	if cleanup != nil {
		cleanup()
	}
	scriptPath := filepath.Join(filepath.Dir(downloadedPath), "cfst-gui-update-"+stamp+".sh")
	script := buildUnixReplaceScript(currentExe, replacementPath, downloadedPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		_ = os.Remove(replacementPath)
		return "", err
	}
	if err := exec.Command("sh", scriptPath).Start(); err != nil {
		_ = os.Remove(replacementPath)
		return "", err
	}
	return "restart_pending", nil
}

func buildUnixReplaceScript(currentExe, replacementPath, downloadedPath string) string {
	return fmt.Sprintf("#!/usr/bin/env sh\nset -e\nsleep 1\nchmod +x %s\nmv -f %s %s\nchmod +x %s\n%s >/dev/null 2>&1 &\nrm -f %s\nrm -- \"$0\"\n",
		shellQuote(replacementPath),
		shellQuote(replacementPath),
		shellQuote(currentExe),
		shellQuote(currentExe),
		shellQuote(currentExe),
		shellQuote(downloadedPath),
	)
}

func startDarwinReplacement(downloadedPath string) (string, error) {
	currentExe, err := os.Executable()
	if err != nil {
		return "", err
	}
	appRoot := findDarwinAppRoot(currentExe)
	if appRoot == "" {
		return "manual", OpenPathDetached(downloadedPath)
	}
	if err := ensureWritableTarget(appRoot); err != nil {
		return "manual", OpenPathDetached(downloadedPath)
	}
	extractDir, err := os.MkdirTemp(filepath.Dir(downloadedPath), "cfst-darwin-update-*")
	if err != nil {
		return "", err
	}
	if err := unzip(downloadedPath, extractDir); err != nil {
		_ = os.RemoveAll(extractDir)
		return "", err
	}
	replacementApp, err := findFirstAppBundle(extractDir)
	if err != nil {
		_ = os.RemoveAll(extractDir)
		return "", err
	}
	scriptPath := filepath.Join(filepath.Dir(downloadedPath), "cfst-gui-update-"+strconv.FormatInt(time.Now().UnixNano(), 10)+".sh")
	script := fmt.Sprintf("#!/usr/bin/env sh\nsleep 1\nrm -rf %s\ncp -R %s %s\nopen %s >/dev/null 2>&1 &\nrm -rf %s\nrm -f %s\nrm -- \"$0\"\n",
		shellQuote(appRoot),
		shellQuote(replacementApp),
		shellQuote(appRoot),
		shellQuote(appRoot),
		shellQuote(extractDir),
		shellQuote(downloadedPath),
	)
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		_ = os.RemoveAll(extractDir)
		return "", err
	}
	if err := exec.Command("sh", scriptPath).Start(); err != nil {
		return "", err
	}
	return "restart_pending", nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func OpenExternalURL(rawURL string) error {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return err
	}
	return OpenPathDetached(rawURL)
}

func OpenPathDetached(targetPath string) error {
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
