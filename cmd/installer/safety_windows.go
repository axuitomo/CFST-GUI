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
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	return dir, err
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
	if trusted, err := cleanInstallDir(registered); err == nil && strings.EqualFold(dir, trusted) {
		return dir, nil
	}
	for _, entry := range entries {
		owned := strings.EqualFold(entry.Name(), appExeName) || strings.EqualFold(entry.Name(), uninstName)
		if !owned || !entry.Type().IsRegular() {
			return "", errors.New("choose an empty directory or an existing CFST-GUI installation")
		}
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
