//go:build windows

package main

import "golang.org/x/sys/windows"

func storageFreeBytes(path string) (int64, bool) {
	var freeBytes uint64
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return -1, false
	}
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytes, nil, nil); err != nil {
		return -1, false
	}
	return int64(freeBytes), true
}
