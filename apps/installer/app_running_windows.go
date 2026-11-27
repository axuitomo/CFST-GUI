//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	th32csSnapProcess       = 0x00000002
	processQueryLimitedInfo = 0x1000
	processTerminate        = 0x0001
)

// runningProcessIDs 返回正在运行、且可执行文件完整路径等于 exePath 的进程。
// 按完整路径匹配是刻意的：便携版或开发版即使同名，也不该被安装器误关。
func runningProcessIDs(exePath string) ([]uint32, error) {
	exePath = filepath.Clean(exePath)
	base := filepath.Base(exePath)

	snapshot, err := windows.CreateToolhelp32Snapshot(th32csSnapProcess, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return nil, nil
		}
		return nil, err
	}

	self := uint32(os.Getpid())
	var pids []uint32
	for {
		if id := entry.ProcessID; id != 0 && id != self && strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), base) {
			if path, ok := processImagePath(id); ok && strings.EqualFold(filepath.Clean(path), exePath) {
				pids = append(pids, id)
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return pids, nil
			}
			return nil, err
		}
	}
}

func processImagePath(pid uint32) (string, bool) {
	handle, err := windows.OpenProcess(processQueryLimitedInfo, false, pid)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(handle)

	// 安装目录可能很长，缓冲区给足，避免 QueryFullProcessImageName 因缓冲不足失败。
	buffer := make([]uint16, 1024)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return "", false
	}
	return windows.UTF16ToString(buffer[:size]), true
}

// closeProcesses 强制结束已获得用户同意的应用进程。
// CFST-GUI 关闭窗口只会退到托盘，发送关闭消息不可靠，所以这里直接结束进程。
func closeProcesses(pids []uint32) error {
	var errs []error
	for _, pid := range pids {
		handle, err := windows.OpenProcess(processTerminate, false, pid)
		if err != nil {
			if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
				continue // 进程已经自行退出
			}
			errs = append(errs, fmt.Errorf("打开进程 %d 失败：%w", pid, err))
			continue
		}
		err = windows.TerminateProcess(handle, 1)
		_ = windows.CloseHandle(handle)
		if err != nil && !errors.Is(err, windows.ERROR_INVALID_PARAMETER) && !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			errs = append(errs, fmt.Errorf("结束进程 %d 失败：%w", pid, err))
		}
	}
	return errors.Join(errs...)
}

func appRunningError(pids []uint32) error {
	return fmt.Errorf("检测到 CFST-GUI 正在运行（%d 个进程），请先退出应用后再继续", len(pids))
}
