package mobileapi

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func (s *Service) ExportDiagnosticPackage(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("DIAGNOSTIC_PACKAGE_INVALID", nil, err.Error(), false, nil, nil))
	}
	fileName := mobileDiagnosticPackageFileName(payload, "cfst-diagnostics.zip")
	body, included, err := s.buildDiagnosticPackage()
	if err != nil {
		return encodeCommand(commandResultFor("DIAGNOSTIC_PACKAGE_BUILD_FAILED", nil, err.Error(), false, nil, nil))
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	exportCfg := mapValue(config["export"])
	data := map[string]any{
		"file_name":     fileName,
		"included":      included,
		"written_bytes": len(body),
	}
	targetURI := strings.TrimSpace(stringValue(firstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"], exportCfg["target_uri"], exportCfg["targetUri"]), ""))
	if targetURI != "" {
		data["content_base64"] = base64.StdEncoding.EncodeToString(body)
		data["target_uri"] = targetURI
		return encodeCommand(commandResultFor("DIAGNOSTIC_PACKAGE_EXPORT_OK", data, "诊断包已准备导出。", true, nil, nil))
	}
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	if targetPath == "" {
		targetDir := strings.TrimSpace(stringValue(firstNonNil(payload["target_dir"], payload["targetDir"], exportCfg["target_dir"], exportCfg["targetDir"]), ""))
		if targetDir != "" {
			targetPath = filepath.Join(targetDir, filepath.Base(fileName))
		}
	}
	if targetPath == "" {
		return encodeCommand(commandResultFor("DIAGNOSTIC_PACKAGE_INVALID", nil, "缺少导出目标路径。", false, nil, nil))
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return encodeCommand(commandResultFor("DIAGNOSTIC_PACKAGE_WRITE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if err := os.WriteFile(targetPath, body, 0o644); err != nil {
		return encodeCommand(commandResultFor("DIAGNOSTIC_PACKAGE_WRITE_FAILED", nil, err.Error(), false, nil, nil))
	}
	data["file_name"] = filepath.Base(targetPath)
	data["path"] = targetPath
	return encodeCommand(commandResultFor("DIAGNOSTIC_PACKAGE_EXPORT_OK", data, "诊断包已导出。", true, nil, nil))
}

func (s *Service) buildDiagnosticPackage() ([]byte, []string, error) {
	buf := &bytes.Buffer{}
	archive := zip.NewWriter(buf)
	included := []string{}
	addBytes := func(name string, raw []byte) error {
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
	for _, item := range []struct {
		name string
		path string
	}{
		{name: "logs/cfip-log.txt", path: s.debugLogPath()},
		{name: "logs/error-log.txt", path: s.errorLogPath()},
	} {
		raw, err := os.ReadFile(item.path)
		if err == nil {
			if err := addBytes(item.name, raw); err != nil {
				_ = archive.Close()
				return nil, nil, err
			}
		}
	}
	if err := addJSON("status/scheduler.json", s.currentSchedulerStatus()); err != nil {
		_ = archive.Close()
		return nil, nil, err
	}
	if err := addJSON("status/runtime.json", s.runtimeStatusData()); err != nil {
		_ = archive.Close()
		return nil, nil, err
	}
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		snapshot = defaultConfigSnapshot()
	}
	if err := addJSON("config/config-summary.json", redactMobileDiagnosticConfigSnapshot(snapshot)); err != nil {
		_ = archive.Close()
		return nil, nil, err
	}
	if err := addMobileRecentTaskSnapshotsToZip(archive, &included, s.tasksRootPath(), 20); err != nil {
		_ = archive.Close()
		return nil, nil, err
	}
	if err := archive.Close(); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), included, nil
}

func mobileDiagnosticPackageFileName(payload map[string]any, fallback string) string {
	rawName := strings.TrimSpace(stringValue(firstNonNil(payload["file_name"], payload["fileName"], payload["default_file_name"], payload["defaultFileName"]), ""))
	if rawName == "" {
		rawName = fmt.Sprintf("cfst-diagnostics-%s.zip", time.Now().Format("20060102-150405"))
	}
	name := probecore.SanitizeTemplateFileName(filepath.Base(rawName))
	if name == "" {
		return fallback
	}
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		name += ".zip"
	}
	return name
}

func redactMobileDiagnosticConfigSnapshot(snapshot map[string]any) map[string]any {
	redacted := mobileDeepCloneMap(sanitizeMobileConfigSnapshot(snapshot))
	if cloudflare := mapValue(redacted["cloudflare"]); len(cloudflare) > 0 {
		cloudflare["api_token"] = ""
		redacted["cloudflare"] = cloudflare
	}
	if github := mapValue(redacted["github"]); len(github) > 0 {
		github["token"] = ""
		redacted["github"] = github
	}
	if exportCfg := mapValue(redacted["export"]); len(exportCfg) > 0 {
		if github := mapValue(exportCfg["github"]); len(github) > 0 {
			github["token"] = ""
			exportCfg["github"] = github
		}
		redacted["export"] = exportCfg
	}
	if backup := mapValue(redacted["backup"]); len(backup) > 0 {
		if webdav := mapValue(backup["webdav"]); len(webdav) > 0 {
			webdav["password"] = ""
			webdav["username"] = ""
			backup["webdav"] = webdav
		}
		redacted["backup"] = backup
	}
	return redacted
}

func addMobileRecentTaskSnapshotsToZip(archive *zip.Writer, included *[]string, root string, limit int) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	type taskFile struct {
		name    string
		modTime time.Time
		raw     []byte
	}
	files := []taskFile{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || strings.HasSuffix(entry.Name(), "-results.json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		filePath := filepath.Join(root, entry.Name())
		raw, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		var snapshot taskSnapshot
		if err := json.Unmarshal(raw, &snapshot); err != nil || !mobileTerminalTaskSnapshotStatus(snapshot.Status) {
			continue
		}
		files = append(files, taskFile{name: entry.Name(), modTime: info.ModTime(), raw: raw})
	}
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[j].modTime.After(files[i].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
	for index, file := range files {
		if limit > 0 && index >= limit {
			break
		}
		zipName := path.Join("tasks", file.name)
		writer, err := archive.Create(zipName)
		if err != nil {
			return err
		}
		if _, err := writer.Write(file.raw); err != nil {
			return err
		}
		*included = append(*included, zipName)
	}
	return nil
}
