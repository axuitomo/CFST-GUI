package appcore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type FileState struct {
	Data          []byte
	Exists        bool
	Mode          os.FileMode
	Path          string
	SymlinkTarget string
}

var syncParentDirAfterCommit = syncParentDir

func WriteFileAtomic(path string, raw []byte, perm os.FileMode) (retErr error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("写入路径不能为空")
	}
	resolvedPath, err := resolveAtomicWritePath(path)
	if err != nil {
		return err
	}
	path = resolvedPath
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	defer func() {
		if file != nil {
			if closeErr := file.Close(); retErr == nil && closeErr != nil {
				retErr = closeErr
			}
		}
	}()
	if err := file.Chmod(perm); err != nil {
		return err
	}
	if _, err := file.Write(raw); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		file = nil
		return err
	}
	file = nil
	if err := commitAtomicFile(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func resolveAtomicWritePath(path string) (string, error) {
	resolved := filepath.Clean(path)
	seen := map[string]struct{}{}
	for {
		if _, exists := seen[resolved]; exists {
			return "", fmt.Errorf("检测到符号链接循环：%s", path)
		}
		seen[resolved] = struct{}{}
		info, err := os.Lstat(resolved)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return resolved, nil
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			if info.IsDir() {
				return "", fmt.Errorf("%s 是目录，不能作为文件写入", resolved)
			}
			return resolved, nil
		}
		nextPath, err := os.Readlink(resolved)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(nextPath) {
			nextPath = filepath.Join(filepath.Dir(resolved), nextPath)
		}
		resolved = filepath.Clean(nextPath)
	}
}

func CaptureFileStates(paths ...string) ([]FileState, error) {
	states := make([]FileState, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		info, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				states = append(states, FileState{Path: path})
				continue
			}
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return nil, err
			}
			targetInfo, err := os.Stat(path)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					states = append(states, FileState{
						Path:          path,
						SymlinkTarget: linkTarget,
					})
					continue
				}
				return nil, err
			}
			if targetInfo.IsDir() {
				return nil, fmt.Errorf("%s 不是文件", path)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			states = append(states, FileState{
				Data:          raw,
				Exists:        true,
				Mode:          targetInfo.Mode().Perm(),
				Path:          path,
				SymlinkTarget: linkTarget,
			})
			continue
		}
		if info.IsDir() {
			return nil, fmt.Errorf("%s 不是文件", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		states = append(states, FileState{
			Data:   raw,
			Exists: true,
			Mode:   info.Mode().Perm(),
			Path:   path,
		})
	}
	return states, nil
}

func RestoreFileStates(states []FileState) error {
	var restoreErrs []error
	for _, state := range states {
		if strings.TrimSpace(state.Path) == "" {
			continue
		}
		if state.SymlinkTarget != "" {
			if err := restoreSymlinkPath(state.Path, state.SymlinkTarget); err != nil {
				restoreErrs = append(restoreErrs, fmt.Errorf("恢复 %s 符号链接失败：%w", state.Path, err))
				continue
			}
			if state.Exists {
				if err := WriteFileAtomic(state.Path, state.Data, state.Mode); err != nil {
					restoreErrs = append(restoreErrs, fmt.Errorf("恢复 %s 失败：%w", state.Path, err))
				}
				continue
			}
			if err := removeResolvedWriteTarget(state.Path); err != nil {
				restoreErrs = append(restoreErrs, fmt.Errorf("恢复 %s 断链符号链接目标失败：%w", state.Path, err))
			}
			continue
		}
		if state.Exists {
			if err := WriteFileAtomic(state.Path, state.Data, state.Mode); err != nil {
				restoreErrs = append(restoreErrs, fmt.Errorf("恢复 %s 失败：%w", state.Path, err))
			}
			continue
		}
		if err := os.Remove(state.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			restoreErrs = append(restoreErrs, fmt.Errorf("删除 %s 失败：%w", state.Path, err))
		}
	}
	return errors.Join(restoreErrs...)
}

func restoreSymlinkPath(path string, linkTarget string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			currentTarget, readErr := os.Readlink(path)
			if readErr != nil {
				return readErr
			}
			if currentTarget == linkTarget {
				return nil
			}
		}
		if info.IsDir() {
			return fmt.Errorf("%s 是目录，不能恢复为符号链接", path)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Symlink(linkTarget, path)
}

func removeResolvedWriteTarget(path string) error {
	resolvedPath, err := resolveAtomicWritePath(path)
	if err != nil {
		return err
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s 是目录，不能删除以恢复断链符号链接", resolvedPath)
	}
	return os.Remove(resolvedPath)
}

func syncParentDir(path string) error {
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	err = dir.Sync()
	if shouldIgnoreParentDirSyncError(err) {
		return nil
	}
	return err
}

func shouldIgnoreParentDirSyncError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EOPNOTSUPP) ||
		errors.Is(err, syscall.ENOSYS) ||
		errors.Is(err, syscall.EINVAL)
}
