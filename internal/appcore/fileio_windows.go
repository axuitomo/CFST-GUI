//go:build windows

package appcore

import (
	"errors"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32DLL      = windows.NewLazySystemDLL("kernel32.dll")
	procReplaceFileW = kernel32DLL.NewProc("ReplaceFileW")
)

func commitAtomicFile(tmpPath, targetPath string) error {
	replacement, err := windows.UTF16PtrFromString(tmpPath)
	if err != nil {
		return err
	}
	replaced, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return err
	}
	// ReplaceFileW provides the dedicated Windows file-replacement path for
	// existing targets, which is safer than treating overwrite as a plain rename.
	if err := replaceFileWindows(replaced, replacement); err == nil {
		return nil
	} else if !isMissingWindowsPathError(err) {
		return err
	}
	moveErr := windows.MoveFileEx(replacement, replaced, windows.MOVEFILE_WRITE_THROUGH)
	if moveErr == nil {
		return nil
	}
	if _, err := os.Stat(targetPath); err == nil {
		return replaceFileWindows(replaced, replacement)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return moveErr
}

func replaceFileWindows(replaced, replacement *uint16) error {
	r1, _, e1 := procReplaceFileW.Call(
		uintptr(unsafe.Pointer(replaced)),
		uintptr(unsafe.Pointer(replacement)),
		0,
		0,
		0,
		0,
	)
	if r1 != 0 {
		return nil
	}
	if e1 != nil {
		return os.NewSyscallError("ReplaceFileW", e1)
	}
	return os.NewSyscallError("ReplaceFileW", syscall.EINVAL)
}

func isMissingWindowsPathError(err error) bool {
	return errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND)
}
