package appcore

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServiceDebugExportSupportsFileAndSAFTargets(t *testing.T) {
	root := t.TempDir()
	exportDir := filepath.Join(root, "published")
	service := NewService(ServiceOptions{
		Clock:            func() time.Time { return time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC) },
		DefaultExportDir: exportDir,
		Storage:          StorageLayout{Root: root},
	})
	if err := os.MkdirAll(service.StorageLayout().LogsRoot(), 0o755); err != nil {
		t.Fatal(err)
	}
	telegramToken := "123456789:" + "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi"
	logBody := []byte("shared debug log https://api.telegram.org/bot" + telegramToken + "/sendMessage\n")
	redactedLogBody := []byte("shared debug log https://api.telegram.org/bot<redacted>/sendMessage\n")
	if err := os.WriteFile(filepath.Join(service.StorageLayout().LogsRoot(), "cfip-log.txt"), logBody, 0o600); err != nil {
		t.Fatal(err)
	}

	fileResult := service.Invoke("debug.export", `{"file_name":"debug.txt"}`)
	if !fileResult.OK || fileResult.Code != "DEBUG_LOG_EXPORT_OK" {
		t.Fatalf("file export = %#v", fileResult)
	}
	fileData := mapValue(fileResult.Data)
	targetPath := filepath.Join(exportDir, "debug.txt")
	if fileData["path"] != targetPath {
		t.Fatalf("path = %#v, want %q", fileData["path"], targetPath)
	}
	if raw, err := os.ReadFile(targetPath); err != nil || !bytes.Equal(raw, redactedLogBody) {
		t.Fatalf("exported log = %q, %v", raw, err)
	}

	safResult := service.Invoke("debug.export", `{"target_uri":"content://exports/debug.txt"}`)
	if !safResult.OK || safResult.Code != "DEBUG_LOG_EXPORT_OK" {
		t.Fatalf("SAF export = %#v", safResult)
	}
	safData := mapValue(safResult.Data)
	decoded, err := base64.StdEncoding.DecodeString(stringValue(safData["content_base64"], ""))
	if err != nil || !bytes.Equal(decoded, redactedLogBody) {
		t.Fatalf("SAF content = %q, %v", decoded, err)
	}
	if safData["target_uri"] != "content://exports/debug.txt" {
		t.Fatalf("target_uri = %#v", safData["target_uri"])
	}
}

func TestServiceDiagnosticExportIncludesArtifactsAndRedactsSecrets(t *testing.T) {
	root := t.TempDir()
	layout := StorageLayout{
		Root:               root,
		ConfigFileName:     "desktop-config.json",
		SchedulerFileName:  "scheduler-status.json",
		SourceProfilesFile: "source-profiles.json",
	}
	service := NewService(ServiceOptions{Storage: layout})
	if err := os.MkdirAll(layout.LogsRoot(), 0o755); err != nil {
		t.Fatal(err)
	}
	telegramToken := "123456789:" + "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi"
	for name, body := range map[string]string{
		"cfip-log.txt":  "debug https://api.telegram.org/bot" + telegramToken + "/sendMessage\n",
		"error-log.txt": `{"message":"telegram failed","bot_token":"` + telegramToken + `"}` + "\n",
	} {
		if err := os.WriteFile(filepath.Join(layout.LogsRoot(), name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := map[string]any{
		"cloudflare": map[string]any{"api_token": "cloudflare-secret"},
		"github":     map[string]any{"token": "github-secret"},
		"export": map[string]any{
			"github": map[string]any{"token": "export-github-secret"},
		},
		"backup": map[string]any{
			"webdav": map[string]any{"username": "private-user", "password": "webdav-secret"},
		},
		"notifications": map[string]any{
			"telegram": map[string]any{
				"bot_token": telegramToken, "chat_id": "private-chat", "personal_chat_id": "private-personal-chat",
			},
		},
	}
	if _, err := service.SaveConfig(snapshot); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Format(time.RFC3339)
	if err := service.WriteTaskSnapshot(TaskSnapshot{
		TaskID: "diagnostic-task", Status: "completed", CompletedAt: now, UpdatedAt: now,
		FailureSummary: map[string]any{"message": "https://api.telegram.org/bot" + telegramToken + "/sendMessage"},
	}); err != nil {
		t.Fatal(err)
	}

	result := service.Invoke("diagnostics.export", `{"target_uri":"content://exports/diagnostics.zip"}`)
	if !result.OK || result.Code != "DIAGNOSTIC_PACKAGE_EXPORT_OK" {
		t.Fatalf("diagnostic export = %#v", result)
	}
	data := mapValue(result.Data)
	body, err := base64.StdEncoding.DecodeString(stringValue(data["content_base64"], ""))
	if err != nil {
		t.Fatal(err)
	}
	entries := unzipDiagnosticEntries(t, body)
	for _, name := range []string{
		"logs/cfip-log.txt",
		"logs/error-log.txt",
		"status/scheduler.json",
		"status/runtime.json",
		"config/config-summary.json",
		"tasks/diagnostic-task.json",
	} {
		if _, ok := entries[name]; !ok {
			t.Fatalf("missing %s; entries=%v", name, diagnosticEntryNames(entries))
		}
	}
	for name, raw := range entries {
		if strings.Contains(string(raw), telegramToken) {
			t.Fatalf("%s leaked Telegram bot token: %s", name, entries[name])
		}
	}
	var config map[string]any
	if err := json.Unmarshal(entries["config/config-summary.json"], &config); err != nil {
		t.Fatal(err)
	}
	if got := stringValue(mapValue(config["cloudflare"])["api_token"], "missing"); got != "" {
		t.Fatalf("cloudflare token = %q", got)
	}
	if got := stringValue(mapValue(config["github"])["token"], "missing"); got != "" {
		t.Fatalf("github token = %q", got)
	}
	exportGitHub := mapValue(mapValue(config["export"])["github"])
	if got := stringValue(exportGitHub["token"], "missing"); got != "" {
		t.Fatalf("export github token = %q", got)
	}
	webdav := mapValue(mapValue(config["backup"])["webdav"])
	if got := stringValue(webdav["username"], "missing"); got != "" {
		t.Fatalf("webdav username = %q", got)
	}
	if got := stringValue(webdav["password"], "missing"); got != "" {
		t.Fatalf("webdav password = %q", got)
	}
	telegram := mapValue(mapValue(config["notifications"])["telegram"])
	for _, key := range []string{"bot_token", "chat_id", "personal_chat_id"} {
		if got := stringValue(telegram[key], "missing"); got != "" {
			t.Fatalf("telegram %s = %q, want empty", key, got)
		}
	}
}

func unzipDiagnosticEntries(t *testing.T, body []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		raw, err := func() ([]byte, error) {
			stream, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer stream.Close()
			buffer := bytes.NewBuffer(nil)
			_, err = buffer.ReadFrom(stream)
			return buffer.Bytes(), err
		}()
		if err != nil {
			t.Fatal(err)
		}
		entries[file.Name] = raw
	}
	return entries
}

func diagnosticEntryNames(entries map[string][]byte) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	return names
}
