//go:build !windows

package appcore

import "os"

func commitAtomicFile(tmpPath, targetPath string) error {
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return err
	}
	return syncParentDirAfterCommit(targetPath)
}
