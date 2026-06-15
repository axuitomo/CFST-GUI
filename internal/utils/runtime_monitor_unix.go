//go:build !windows

package utils

import "os"

func commitSyncedJSONFile(tmpPath string, targetPath string) error {
	return os.Rename(tmpPath, targetPath)
}
