//go:build !windows

package main

import "golang.org/x/sys/unix"

func storageFreeBytes(path string) (int64, bool) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return -1, false
	}
	return int64(stat.Bavail) * int64(stat.Bsize), true
}
