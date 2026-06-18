package mobileapi

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func (s *Service) ExportDebugLog(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("DEBUG_LOG_EXPORT_INVALID", nil, err.Error(), false, nil, nil))
	}
	sourcePath := s.debugLogPath()
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return encodeCommand(commandResultFor("DEBUG_LOG_EXPORT_NOT_FOUND", nil, "调试日志不存在，请先启用调试日志并运行一次任务。", false, nil, nil))
		}
		return encodeCommand(commandResultFor("DEBUG_LOG_EXPORT_READ_FAILED", nil, err.Error(), false, nil, nil))
	}

	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	exportCfg := mapValue(config["export"])
	fileName := mobileDebugLogExportFileName(payload, "cfip-log.txt")
	targetURI := strings.TrimSpace(stringValue(firstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"], exportCfg["target_uri"], exportCfg["targetUri"]), ""))
	if targetURI != "" {
		return encodeCommand(commandResultFor("DEBUG_LOG_EXPORT_OK", map[string]any{
			"content_base64": base64.StdEncoding.EncodeToString(raw),
			"file_name":      fileName,
			"target_uri":     targetURI,
			"written_bytes":  len(raw),
		}, "调试日志已准备导出。", true, nil, nil))
	}

	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	if targetPath == "" {
		targetDir := strings.TrimSpace(stringValue(firstNonNil(payload["target_dir"], payload["targetDir"], exportCfg["target_dir"], exportCfg["targetDir"]), ""))
		if targetDir != "" {
			targetPath = filepath.Join(targetDir, filepath.Base(fileName))
		}
	}
	if targetPath == "" {
		return encodeCommand(commandResultFor("DEBUG_LOG_EXPORT_INVALID", nil, "缺少导出目标路径。", false, nil, nil))
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return encodeCommand(commandResultFor("DEBUG_LOG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if err := os.WriteFile(targetPath, raw, 0o644); err != nil {
		return encodeCommand(commandResultFor("DEBUG_LOG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("DEBUG_LOG_EXPORT_OK", map[string]any{
		"file_name":     filepath.Base(targetPath),
		"log_dir":       s.logDirectoryPath(),
		"logDir":        s.logDirectoryPath(),
		"path":          targetPath,
		"source_path":   sourcePath,
		"written_bytes": len(raw),
	}, "调试日志已导出。", true, nil, nil))
}

func mobileDebugLogExportFileName(payload map[string]any, fallback string) string {
	rawName := strings.TrimSpace(stringValue(firstNonNil(payload["file_name"], payload["fileName"], payload["default_file_name"], payload["defaultFileName"]), ""))
	if rawName == "" {
		rawName = fmt.Sprintf("cfip-log-%s.txt", time.Now().Format("20060102-150405"))
	}
	name := probecore.SanitizeTemplateFileName(filepath.Base(rawName))
	if name == "" {
		return fallback
	}
	if !strings.HasSuffix(strings.ToLower(name), ".txt") {
		name += ".txt"
	}
	return name
}
