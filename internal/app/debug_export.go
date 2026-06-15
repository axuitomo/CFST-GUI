package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func (a *App) ExportDebugLog(payload map[string]any) DesktopCommandResult {
	sourcePath := debugLogFilePath()
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return desktopCommandResult("DEBUG_LOG_EXPORT_NOT_FOUND", nil, "调试日志不存在，请先启用调试日志并运行一次任务。", false, nil, nil)
		}
		return desktopCommandResult("DEBUG_LOG_EXPORT_READ_FAILED", nil, err.Error(), false, nil, nil)
	}

	fileName := debugLogExportFileName(payload, "cfip-log.txt")
	targetURI := strings.TrimSpace(stringValue(firstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"]), ""))
	if targetURI != "" {
		return desktopCommandResult("DEBUG_LOG_EXPORT_OK", map[string]any{
			"content_base64": base64.StdEncoding.EncodeToString(raw),
			"file_name":      fileName,
			"log_dir":        logDirectoryPath(),
			"logDir":         logDirectoryPath(),
			"source_path":    sourcePath,
			"target_uri":     targetURI,
			"written_bytes":  len(raw),
		}, "调试日志已准备导出。", true, nil, nil)
	}

	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	if targetPath == "" {
		targetPath = configuredDebugLogExportPath(payload, fileName)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return desktopCommandResult("DEBUG_LOG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if err := os.WriteFile(targetPath, raw, 0o644); err != nil {
		return desktopCommandResult("DEBUG_LOG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("DEBUG_LOG_EXPORT_OK", map[string]any{
		"file_name":     filepath.Base(targetPath),
		"log_dir":       logDirectoryPath(),
		"logDir":        logDirectoryPath(),
		"path":          targetPath,
		"source_path":   sourcePath,
		"written_bytes": len(raw),
	}, "调试日志已导出。", true, nil, nil)
}

func (a *App) ExportDiagnosticBundle(payload map[string]any) DesktopCommandResult {
	now := time.Now()
	requestedName := stringValue(firstNonNil(payload["file_name"], payload["fileName"], payload["default_file_name"], payload["defaultFileName"]), "")
	bundle, err := appcore.BuildDiagnosticBundle(logDirectoryPath(), runtime.GOOS+"/"+runtime.GOARCH, now, requestedName)
	if err != nil {
		if errors.Is(err, appcore.ErrDiagnosticBundleEmpty) {
			return desktopCommandResult("DIAGNOSTIC_BUNDLE_EMPTY", map[string]any{
				"log_dir": logDirectoryPath(),
				"logDir":  logDirectoryPath(),
				"missing": bundle.Missing,
			}, "诊断包没有可用日志，请先运行任务或开启日志/调试。", false, nil, nil)
		}
		return desktopCommandResult("DIAGNOSTIC_BUNDLE_BUILD_FAILED", nil, err.Error(), false, nil, nil)
	}

	targetURI := strings.TrimSpace(stringValue(firstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"]), ""))
	if targetURI != "" {
		return desktopCommandResult("DIAGNOSTIC_BUNDLE_EXPORT_OK", map[string]any{
			"content_base64": base64.StdEncoding.EncodeToString(bundle.Content),
			"file_name":      bundle.FileName,
			"included":       bundle.Included,
			"log_dir":        bundle.LogDirectory,
			"logDir":         bundle.LogDirectory,
			"missing":        bundle.Missing,
			"target_uri":     targetURI,
			"written_bytes":  len(bundle.Content),
		}, "诊断包已准备导出。", true, nil, nil)
	}

	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	if targetPath == "" {
		targetPath = configuredDebugLogExportPath(payload, bundle.FileName)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return desktopCommandResult("DIAGNOSTIC_BUNDLE_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if err := os.WriteFile(targetPath, bundle.Content, 0o600); err != nil {
		return desktopCommandResult("DIAGNOSTIC_BUNDLE_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("DIAGNOSTIC_BUNDLE_EXPORT_OK", map[string]any{
		"file_name":     filepath.Base(targetPath),
		"included":      bundle.Included,
		"log_dir":       bundle.LogDirectory,
		"logDir":        bundle.LogDirectory,
		"missing":       bundle.Missing,
		"path":          targetPath,
		"written_bytes": len(bundle.Content),
	}, "诊断包已导出。", true, nil, nil)
}

func debugLogExportFileName(payload map[string]any, fallback string) string {
	rawName := strings.TrimSpace(stringValue(firstNonNil(payload["file_name"], payload["fileName"], payload["default_file_name"], payload["defaultFileName"]), ""))
	if rawName == "" {
		rawName = fmt.Sprintf("cfip-log-%s.txt", time.Now().Format("20060102-150405"))
	}
	name := sanitizeTemplateFileName(filepath.Base(rawName))
	if name == "" {
		return fallback
	}
	if !strings.HasSuffix(strings.ToLower(name), ".txt") {
		name += ".txt"
	}
	return name
}

func configuredDebugLogExportPath(payload map[string]any, fileName string) string {
	targetDir := strings.TrimSpace(stringValue(firstNonNil(payload["target_dir"], payload["targetDir"]), ""))
	if targetDir == "" {
		config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
		exportCfg := mapValue(config["export"])
		targetDir = strings.TrimSpace(stringValue(firstNonNil(exportCfg["target_dir"], exportCfg["targetDir"]), ""))
	}
	if targetDir == "" {
		targetDir = defaultExportDir()
	}
	return filepath.Join(targetDir, filepath.Base(fileName))
}
