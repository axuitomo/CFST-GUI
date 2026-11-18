package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func registeredInstallDir() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, uninstKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer key.Close()
	dir, _, err := key.GetStringValue("InstallLocation")
	if err == nil && strings.TrimSpace(dir) != "" {
		return dir, nil
	}
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return "", err
	}
	// 旧 NSIS 安装项没有 InstallLocation，只能从 DisplayIcon 或
	// UninstallString 还原目录；否则已有安装会被误判成陌生非空目录。
	for _, name := range []string{"UninstallString", "DisplayIcon"} {
		value, _, err := key.GetStringValue(name)
		if errors.Is(err, registry.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if dir := installDirFromRegistryPath(value); dir != "" {
			return dir, nil
		}
	}
	return "", nil
}

// installDirFromRegistryPath 从卸载命令或图标路径还原安装目录。
// 只接受本应用自己的可执行文件或图标，避免把任意注册表命令的
// 所在目录当成可信安装位置。
func installDirFromRegistryPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var path string
	if strings.HasPrefix(value, `"`) {
		end := strings.IndexByte(value[1:], '"')
		if end < 0 {
			return ""
		}
		path = value[1 : 1+end]
	} else {
		if comma := strings.IndexByte(value, ','); comma >= 0 {
			value = value[:comma]
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			return ""
		}
		path = fields[0]
	}
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return ""
	}
	switch strings.ToLower(filepath.Base(path)) {
	case strings.ToLower(uninstName), strings.ToLower(appExeName), "icon.ico":
		return filepath.Clean(filepath.Dir(path))
	default:
		return ""
	}
}

func cleanInstallDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" || !filepath.IsAbs(dir) {
		return "", errors.New("install directory must be an absolute path")
	}
	dir = filepath.Clean(dir)
	if filepath.Dir(dir) == dir {
		return "", errors.New("a drive or share root cannot be an install directory")
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if filepath.Dir(dir) == dir {
		return "", errors.New("install directory resolves to a drive or share root")
	}
	return dir, nil
}

// 安装目录可能来自旧 NSIS 版（icon.ico）或当前 Go 安装器
// （LICENSE / THIRD-PARTY-NOTICES.md）；这些文件都属于应用自身。
var installerOwnedFiles = map[string]struct{}{
	appExeName:               {},
	uninstName:               {},
	"icon.ico":               {},
	"license":                {},
	"third-party-notices.md": {},
}

func validateInstallDir(dir, registered string) (string, error) {
	dir, err := cleanInstallDir(dir)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return dir, nil
	}
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return dir, nil
	}
	if trusted, err := cleanInstallDir(registered); err == nil && strings.EqualFold(dir, trusted) {
		return dir, nil
	}
	hasInstallMarker := false
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if _, ok := installerOwnedFiles[name]; !ok || !entry.Type().IsRegular() {
			return "", fmt.Errorf("choose an empty directory or an existing CFST-GUI installation (found %q)", entry.Name())
		}
		if name == strings.ToLower(appExeName) || name == strings.ToLower(uninstName) {
			hasInstallMarker = true
		}
	}
	if !hasInstallMarker {
		return "", errors.New("choose an empty directory or an existing CFST-GUI installation")
	}
	return dir, nil
}

func validateUninstallLocation(dir, self, registered string) error {
	dir, err := cleanInstallDir(dir)
	if err != nil {
		return err
	}
	trusted, err := cleanInstallDir(registered)
	if err != nil || !strings.EqualFold(dir, trusted) {
		return errors.New("refusing to uninstall outside the registered install directory")
	}
	resolvedSelf, err := filepath.EvalSymlinks(self)
	if err != nil || !strings.EqualFold(resolvedSelf, filepath.Join(dir, uninstName)) {
		return errors.New("uninstall must run from the registered uninstall.exe")
	}
	return nil
}

// A failed registration must not leave a mixed-version or partial uninstall entry.
func updateUninstallValues(key registry.Key, values map[string]string, sizeKB uint32) (retErr error) {
	previous := make(map[string]string, len(values))
	for name := range values {
		value, _, err := key.GetStringValue(name)
		if err == nil {
			previous[name] = value
		} else if !errors.Is(err, registry.ErrNotExist) {
			return err
		}
	}
	previousSize, sizeType, sizeErr := key.GetIntegerValue("EstimatedSize")
	if sizeErr != nil && !errors.Is(sizeErr, registry.ErrNotExist) {
		return sizeErr
	}
	defer func() {
		if retErr == nil {
			return
		}
		var restoreErrs []error
		for name := range values {
			var err error
			if value, exists := previous[name]; exists {
				err = key.SetStringValue(name, value)
			} else {
				err = key.DeleteValue(name)
			}
			if err != nil && !errors.Is(err, registry.ErrNotExist) {
				restoreErrs = append(restoreErrs, fmt.Errorf("restore %s: %w", name, err))
			}
		}
		var err error
		if errors.Is(sizeErr, registry.ErrNotExist) {
			err = key.DeleteValue("EstimatedSize")
		} else if sizeType == registry.QWORD {
			err = key.SetQWordValue("EstimatedSize", previousSize)
		} else {
			err = key.SetDWordValue("EstimatedSize", uint32(previousSize))
		}
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			restoreErrs = append(restoreErrs, err)
		}
		retErr = errors.Join(retErr, errors.Join(restoreErrs...))
	}()
	for name, value := range values {
		if err := key.SetStringValue(name, value); err != nil {
			return err
		}
	}
	return key.SetDWordValue("EstimatedSize", sizeKB)
}
