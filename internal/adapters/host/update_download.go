package host

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/github/githubdownload"
)

func downloadAndInstallUpdate(ctx context.Context, info UpdateInfo, downloadDir string) (UpdateInstallResult, error) {
	result := UpdateInstallResult{UpdateInfo: info}
	downloadDir = strings.TrimSpace(downloadDir)
	if downloadDir == "" {
		downloadDir = filepath.Join(storageRoot(), "updates")
	}
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return result, err
	}
	targetPath := filepath.Join(downloadDir, info.AssetName)
	if err := downloadFile(ctx, info.DownloadURL, targetPath, info.SHA256); err != nil {
		return result, err
	}
	result.DownloadedPath = targetPath
	if strings.TrimSpace(info.SHA256) != "" {
		if err := verifySHA256(targetPath, info.SHA256); err != nil {
			return result, err
		}
	}
	nextAction, err := startInstallStrategy(info.InstallMode, targetPath)
	if err != nil {
		return result, err
	}
	result.InstallStarted = nextAction != "manual"
	result.NextAction = nextAction
	return result, nil
}

func downloadFile(ctx context.Context, sourceURL, targetPath, expectedSHA256 string) error {
	candidates := githubdownload.Candidates(sourceURL)
	if len(candidates) == 0 {
		return errors.New("缺少更新包下载地址")
	}
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan updateDownloadResult, len(candidates))
	for index, candidate := range candidates {
		tempPath := fmt.Sprintf("%s.%d.part", targetPath, index)
		go func(candidate, tempPath string) {
			err := downloadCandidateFile(raceCtx, candidate, tempPath, expectedSHA256)
			results <- updateDownloadResult{candidate: candidate, tempPath: tempPath, err: err}
		}(candidate, tempPath)
	}

	var errs []error
	for range candidates {
		result := <-results
		if result.err != nil {
			errs = append(errs, result.err)
			_ = os.Remove(result.tempPath)
			continue
		}
		cancel()
		if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = os.Remove(result.tempPath)
			return err
		}
		if err := os.Rename(result.tempPath, targetPath); err != nil {
			_ = os.Remove(result.tempPath)
			return err
		}
		for remaining := 0; remaining < len(candidates)-len(errs)-1; remaining++ {
			loser := <-results
			_ = os.Remove(loser.tempPath)
		}
		return nil
	}
	return errors.Join(errs...)
}

type updateDownloadResult struct {
	candidate string
	tempPath  string
	err       error
}

func downloadCandidateFile(ctx context.Context, candidate, tempPath, expectedSHA256 string) error {
	requestCtx, cancel := context.WithTimeout(ctx, updateDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, candidate, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "CFST-GUI/"+appVersion())
	res, err := httpClientForUpdates.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", candidate, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s: 下载更新包返回状态 %s", candidate, res.Status)
	}
	output, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, res.Body)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("%s: %w", candidate, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("%s: %w", candidate, closeErr)
	}
	if strings.TrimSpace(expectedSHA256) != "" {
		if err := verifySHA256(tempPath, expectedSHA256); err != nil {
			_ = os.Remove(tempPath)
			return fmt.Errorf("%s: %w", candidate, err)
		}
	}
	return nil
}

func verifySHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("SHA256 校验失败：期望 %s，实际 %s", expected, actual)
	}
	return nil
}
