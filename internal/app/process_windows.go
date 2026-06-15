//go:build windows

package app

import "syscall"

const (
	processQueryLimitedInformation = 0x1000
	processSynchronize             = 0x00100000
	waitObject0                    = 0x00000000
	waitTimeout                    = 0x00000102
)

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := syscall.OpenProcess(processQueryLimitedInformation|processSynchronize, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	status, err := syscall.WaitForSingleObject(handle, 0)
	if err != nil {
		return true
	}
	switch status {
	case waitTimeout:
		return true
	case waitObject0:
		return false
	default:
		return true
	}
}
