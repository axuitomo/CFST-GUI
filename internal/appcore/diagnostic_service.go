package appcore

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func (s *Service) invokeDebugExport(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("DEBUG_LOG_EXPORT_INVALID", nil, err.Error(), false, nil, nil)
	}
	layout := s.StorageLayout()
	sourcePath := layout.DebugLogPath()
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewCommandResult("DEBUG_LOG_EXPORT_NOT_FOUND", nil, "Debug log not found.", false, nil, nil)
		}
		return NewCommandResult("DEBUG_LOG_EXPORT_READ_FAILED", nil, err.Error(), false, nil, nil)
	}
	raw = []byte(utils.RedactSensitiveText(string(raw)))
	fileName := diagnosticExportFileName(payload, "cfip-log", ".txt", s.now())
	targetURI, targetPath := s.diagnosticExportTarget(payload, fileName)
	data := map[string]any{
		"file_name":     fileName,
		"log_dir":       layout.LogsRoot(),
		"source_path":   sourcePath,
		"written_bytes": len(raw),
	}
	if tempPath := tempFilePath(payload); tempPath != "" {
		if err := writeDiagnosticExport(tempPath, raw); err != nil {
			return NewCommandResult("DEBUG_LOG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
		}
		data["temp_file_path"] = tempPath
		return NewCommandResult("DEBUG_LOG_EXPORT_OK", data, "Debug log written to temporary file.", true, nil, nil)
	}
	if targetURI != "" {
		data["content_base64"] = base64.StdEncoding.EncodeToString(raw)
		data["target_uri"] = targetURI
		return NewCommandResult("DEBUG_LOG_EXPORT_OK", data, "Debug log prepared for export.", true, nil, nil)
	}
	if targetPath == "" {
		return NewCommandResult("DEBUG_LOG_EXPORT_INVALID", nil, "Missing export target path.", false, nil, nil)
	}
	if err := writeDiagnosticExport(targetPath, raw); err != nil {
		return NewCommandResult("DEBUG_LOG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	data["file_name"] = filepath.Base(targetPath)
	data["path"] = targetPath
	return NewCommandResult("DEBUG_LOG_EXPORT_OK", data, "Debug log exported.", true, nil, nil)
}

func (s *Service) invokeDiagnosticExport(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("DIAGNOSTIC_PACKAGE_INVALID", nil, err.Error(), false, nil, nil)
	}
	fileName := diagnosticExportFileName(payload, "cfst-diagnostics", ".zip", s.now())
	body, included, err := s.BuildDiagnosticPackage()
	if err != nil {
		return NewCommandResult("DIAGNOSTIC_PACKAGE_BUILD_FAILED", nil, err.Error(), false, nil, nil)
	}
	targetURI, targetPath := s.diagnosticExportTarget(payload, fileName)
	data := map[string]any{"file_name": fileName, "included": included, "written_bytes": len(body)}
	if tempPath := tempFilePath(payload); tempPath != "" {
		if err := writeDiagnosticExport(tempPath, body); err != nil {
			return NewCommandResult("DIAGNOSTIC_PACKAGE_WRITE_FAILED", nil, err.Error(), false, nil, nil)
		}
		data["temp_file_path"] = tempPath
		return NewCommandResult("DIAGNOSTIC_PACKAGE_EXPORT_OK", data, "Diagnostic package written to temporary file.", true, nil, nil)
	}
	if targetURI != "" {
		data["content_base64"] = base64.StdEncoding.EncodeToString(body)
		data["target_uri"] = targetURI
		return NewCommandResult("DIAGNOSTIC_PACKAGE_EXPORT_OK", data, "Diagnostic package prepared for export.", true, nil, nil)
	}
	if targetPath == "" {
		return NewCommandResult("DIAGNOSTIC_PACKAGE_INVALID", nil, "Missing export target path.", false, nil, nil)
	}
	if err := writeDiagnosticExport(targetPath, body); err != nil {
		return NewCommandResult("DIAGNOSTIC_PACKAGE_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	data["file_name"] = filepath.Base(targetPath)
	data["path"] = targetPath
	return NewCommandResult("DIAGNOSTIC_PACKAGE_EXPORT_OK", data, "Diagnostic package exported.", true, nil, nil)
}

func (s *Service) BuildDiagnosticPackage() ([]byte, []string, error) {
	buffer := bytes.NewBuffer(nil)
	archive := zip.NewWriter(buffer)
	included := make([]string, 0)
	addBytes := func(name string, raw []byte) error {
		raw = []byte(utils.RedactSensitiveText(string(raw)))
		writer, err := archive.Create(name)
		if err != nil {
			return err
		}
		if _, err := writer.Write(raw); err != nil {
			return err
		}
		included = append(included, name)
		return nil
	}
	addJSON := func(name string, value any) error {
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		return addBytes(name, raw)
	}
	layout := s.StorageLayout()
	for _, item := range []struct{ name, path string }{
		{name: "logs/cfip-log.txt", path: filepath.Join(layout.LogsRoot(), "cfip-log.txt")},
		{name: "logs/error-log.txt", path: filepath.Join(layout.LogsRoot(), "error-log.txt")},
	} {
		if raw, err := os.ReadFile(item.path); err == nil {
			if err := addBytes(item.name, raw); err != nil {
				_ = archive.Close()
				return nil, nil, err
			}
		}
	}
	scheduler, _, _ := s.LoadSchedulerStatus()
	if err := addJSON("status/scheduler.json", scheduler); err != nil {
		_ = archive.Close()
		return nil, nil, err
	}
	if err := addJSON("status/runtime.json", s.RuntimeStatusData()); err != nil {
		_ = archive.Close()
		return nil, nil, err
	}
	loaded, err := s.LoadConfig()
	if err != nil {
		s.mu.RLock()
		policy := s.options.ConfigPolicy
		s.mu.RUnlock()
		loaded.Snapshot = sanitizeServiceConfig(nil, policy)
	}
	if err := addJSON("config/config-summary.json", redactDiagnosticConfigSnapshot(loaded.Snapshot)); err != nil {
		_ = archive.Close()
		return nil, nil, err
	}
	if err := addRecentTaskSnapshotsToDiagnosticZip(archive, &included, layout.TasksRoot(), 20); err != nil {
		_ = archive.Close()
		return nil, nil, err
	}
	if err := archive.Close(); err != nil {
		return nil, nil, err
	}
	return buffer.Bytes(), included, nil
}
func tempFilePath(payload map[string]any) string {
	return strings.TrimSpace(stringValue(firstNonNil(payload["temp_file_path"], payload["tempFilePath"]), ""))
}

func (s *Service) diagnosticExportTarget(payload map[string]any, fileName string) (string, string) {
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	exportConfig := mapValue(config["export"])
	targetURI := strings.TrimSpace(stringValue(firstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"], exportConfig["target_uri"], exportConfig["targetUri"]), ""))
	if targetURI != "" {
		return targetURI, ""
	}
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	if targetPath != "" {
		return "", targetPath
	}
	targetDir := strings.TrimSpace(stringValue(firstNonNil(payload["target_dir"], payload["targetDir"], exportConfig["target_dir"], exportConfig["targetDir"]), ""))
	if targetDir == "" {
		s.mu.RLock()
		targetDir = strings.TrimSpace(s.options.DefaultExportDir)
		s.mu.RUnlock()
	}
	if targetDir == "" {
		return "", ""
	}
	return "", filepath.Join(targetDir, filepath.Base(fileName))
}

func diagnosticExportFileName(payload map[string]any, prefix, extension string, now time.Time) string {
	rawName := strings.TrimSpace(stringValue(firstNonNil(payload["file_name"], payload["fileName"], payload["default_file_name"], payload["defaultFileName"]), ""))
	if rawName == "" {
		rawName = fmt.Sprintf("%s-%s%s", prefix, now.Format("20060102-150405"), extension)
	}
	name := probecore.SanitizeTemplateFileName(filepath.Base(rawName))
	if name == "" {
		name = prefix + extension
	}
	if !strings.HasSuffix(strings.ToLower(name), strings.ToLower(extension)) {
		name += extension
	}
	return name
}

func writeDiagnosticExport(targetPath string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, raw, 0o644)
}

func redactDiagnosticConfigSnapshot(snapshot map[string]any) map[string]any {
	raw, _ := json.Marshal(snapshot)
	redacted := map[string]any{}
	_ = json.Unmarshal(raw, &redacted)
	if cloudflare := mapValue(redacted["cloudflare"]); len(cloudflare) > 0 {
		cloudflare["api_token"] = ""
	}
	if github := mapValue(redacted["github"]); len(github) > 0 {
		github["token"] = ""
	}
	if exportConfig := mapValue(redacted["export"]); len(exportConfig) > 0 {
		if github := mapValue(exportConfig["github"]); len(github) > 0 {
			github["token"] = ""
		}
	}
	if backup := mapValue(redacted["backup"]); len(backup) > 0 {
		if webdav := mapValue(backup["webdav"]); len(webdav) > 0 {
			webdav["password"] = ""
			webdav["username"] = ""
		}
	}
	if notifications := mapValue(redacted["notifications"]); len(notifications) > 0 {
		if telegram := mapValue(notifications["telegram"]); len(telegram) > 0 {
			for _, key := range []string{"bot_token", "botToken", "token", "chat_id", "chatId", "personal_chat_id", "personalChatId"} {
				if _, ok := telegram[key]; ok {
					telegram[key] = ""
				}
			}
		}
	}
	return redacted
}

func addRecentTaskSnapshotsToDiagnosticZip(archive *zip.Writer, included *[]string, root string, limit int) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	type taskFile struct {
		name    string
		modTime int64
		raw     []byte
	}
	files := make([]taskFile, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || strings.HasSuffix(entry.Name(), "-results.json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			continue
		}
		var snapshot TaskSnapshot
		if err := json.Unmarshal(raw, &snapshot); err != nil || !isTerminalTaskStatus(snapshot.Status) {
			continue
		}
		files = append(files, taskFile{name: entry.Name(), modTime: info.ModTime().UnixNano(), raw: raw})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime > files[j].modTime })
	for index, file := range files {
		if limit > 0 && index >= limit {
			break
		}
		name := path.Join("tasks", file.name)
		writer, err := archive.Create(name)
		if err != nil {
			return err
		}
		raw := []byte(utils.RedactSensitiveText(string(file.raw)))
		if _, err := writer.Write(raw); err != nil {
			return err
		}
		*included = append(*included, name)
	}
	return nil
}
