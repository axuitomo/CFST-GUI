package host

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// 更新包解压安全上限：单文件 512 MiB、解压总量 1 GiB，作为解压炸弹的纵深防御。
// 正常发布产物（Windows NSIS 安装器 / macOS .app / Linux tar.gz）远小于该上限。
// 使用 var 而非 const，便于测试覆盖超限路径。
var (
	maxArchiveEntryBytes int64 = 512 << 20
	maxArchiveTotalBytes int64 = 1 << 30
)

func copyUpdateFile(sourcePath, targetPath string, mode os.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		_ = os.Remove(targetPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(targetPath)
		return closeErr
	}
	return nil
}

func extractLinuxBinary(downloadedPath string) (string, func(), error) {
	if !strings.HasSuffix(downloadedPath, ".tar.gz") {
		return downloadedPath, nil, nil
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(downloadedPath), "cfst-linux-update-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	file, err := os.Open(downloadedPath)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	defer gzipReader.Close()
	entries, err := untarRegularFiles(gzipReader, tempDir)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if len(entries) == 0 {
		cleanup()
		return "", nil, errors.New("Linux 更新包中没有可执行文件")
	}
	slices.Sort(entries)
	return entries[0], cleanup, nil
}

func untarRegularFiles(reader io.Reader, targetDir string) ([]string, error) {
	tarReader := tar.NewReader(reader)
	entries := make([]string, 0)
	var totalBytes int64
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if header.Size > maxArchiveEntryBytes {
			return nil, fmt.Errorf("更新包解压超限：%s 超过单文件上限 %d MiB", header.Name, maxArchiveEntryBytes>>20)
		}
		name := filepath.Clean(strings.ReplaceAll(header.Name, "\\", "/"))
		if name == "." {
			continue
		}
		// 解压目标必须落在 targetDir 内（tar slip）。这里保持 filepath.IsLocal 正向判断：
		// 它是 Go 官方的逃逸检查，也是安全扫描（CodeQL go/zipslip）识别为净化的形式，
		// 改成 `if !filepath.IsLocal(name) { continue }` 会让告警重新出现。
		if filepath.IsLocal(name) {
			targetPath := filepath.Join(targetDir, name)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return nil, err
			}
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return nil, err
			}
			copied, copyErr := io.Copy(file, io.LimitReader(tarReader, maxArchiveEntryBytes+1))
			closeErr := file.Close()
			if copyErr != nil {
				return nil, copyErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
			if copied > maxArchiveEntryBytes {
				return nil, fmt.Errorf("更新包解压超限：%s 超过单文件上限 %d MiB", header.Name, maxArchiveEntryBytes>>20)
			}
			totalBytes += copied
			if totalBytes > maxArchiveTotalBytes {
				return nil, fmt.Errorf("更新包解压超过总大小上限 %d MiB", maxArchiveTotalBytes>>20)
			}
			if header.FileInfo().Mode()&0o111 != 0 || strings.Contains(strings.ToLower(filepath.Base(targetPath)), "cfst-gui") {
				entries = append(entries, targetPath)
			}
		}
	}
	return entries, nil
}

func findDarwinAppRoot(currentExe string) string {
	cleaned := filepath.Clean(currentExe)
	for {
		if strings.HasSuffix(cleaned, ".app") {
			return cleaned
		}
		parent := filepath.Dir(cleaned)
		if parent == cleaned {
			return ""
		}
		cleaned = parent
	}
}

func ensureWritableTarget(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	dir := path
	if !info.IsDir() {
		dir = filepath.Dir(path)
	}
	temp, err := os.CreateTemp(dir, ".cfst-write-check-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	closeErr := temp.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

func unzip(sourcePath, targetDir string) error {
	reader, err := zip.OpenReader(sourcePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	var totalBytes int64
	for _, file := range reader.File {
		// #nosec G115 -- maxArchiveEntryBytes 恒为正且远小于 uint64 上限，转换无溢出风险
		if file.UncompressedSize64 > uint64(maxArchiveEntryBytes) {
			return fmt.Errorf("更新包解压超限：%s 超过单文件上限 %d MiB", file.Name, maxArchiveEntryBytes>>20)
		}
		name := filepath.Clean(strings.ReplaceAll(file.Name, "\\", "/"))
		if name == "." {
			continue
		}
		// 解压目标必须落在 targetDir 内（zip slip）。这里保持 filepath.IsLocal 正向判断：
		// 它是 Go 官方的逃逸检查，也是安全扫描（CodeQL go/zipslip）识别为净化的形式，
		// 改成 `if !filepath.IsLocal(name) { continue }` 会让告警重新出现。
		if filepath.IsLocal(name) {
			targetPath := filepath.Join(targetDir, name)
			if file.FileInfo().IsDir() {
				if err := os.MkdirAll(targetPath, file.FileInfo().Mode()); err != nil {
					return err
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			input, err := file.Open()
			if err != nil {
				return err
			}
			output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.FileInfo().Mode())
			if err != nil {
				_ = input.Close()
				return err
			}
			copied, copyErr := io.Copy(output, io.LimitReader(input, maxArchiveEntryBytes+1))
			closeInputErr := input.Close()
			closeOutputErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeInputErr != nil {
				return closeInputErr
			}
			if closeOutputErr != nil {
				return closeOutputErr
			}
			if copied > maxArchiveEntryBytes {
				return fmt.Errorf("更新包解压超限：%s 超过单文件上限 %d MiB", file.Name, maxArchiveEntryBytes>>20)
			}
			totalBytes += copied
			if totalBytes > maxArchiveTotalBytes {
				return fmt.Errorf("更新包解压超过总大小上限 %d MiB", maxArchiveTotalBytes>>20)
			}
		}
	}
	return nil
}

func findFirstAppBundle(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errors.New("macOS 更新包中没有 .app")
	}
	return found, nil
}
