package archivecore

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZipSingleFileAndParseConfigArchive(t *testing.T) {
	body := map[string]any{"config_snapshot": map[string]any{"probe": map[string]any{"tcp_port": 443}}}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	archive, err := ZipSingleFile(ConfigArchiveEntryName, raw, time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ZipSingleFile returned error: %v", err)
	}
	parsed, err := ParseConfigArchive(archive)
	if err != nil {
		t.Fatalf("ParseConfigArchive returned error: %v", err)
	}
	if _, ok := parsed["config_snapshot"]; !ok {
		t.Fatalf("parsed = %#v, want config_snapshot", parsed)
	}
}

func TestParseConfigArchiveAcceptsJSONAndFallbackEntry(t *testing.T) {
	parsed, err := ParseConfigArchive([]byte(`{"schema_version":"test"}`))
	if err != nil {
		t.Fatalf("ParseConfigArchive JSON returned error: %v", err)
	}
	if parsed["schema_version"] != "test" {
		t.Fatalf("parsed = %#v", parsed)
	}

	buffer := bytes.NewBuffer(nil)
	writer := zip.NewWriter(buffer)
	entry, err := writer.Create("fallback.json")
	if err != nil {
		t.Fatalf("create fallback entry: %v", err)
	}
	if _, err := entry.Write([]byte(`{"fallback":true}`)); err != nil {
		t.Fatalf("write fallback entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	parsed, err = ParseConfigArchive(buffer.Bytes())
	if err != nil {
		t.Fatalf("ParseConfigArchive fallback returned error: %v", err)
	}
	if parsed["fallback"] != true {
		t.Fatalf("parsed = %#v, want fallback true", parsed)
	}
}

func TestArchivePayloadBytesSupportsBase64ContentAndPath(t *testing.T) {
	raw, name, err := ArchivePayloadBytes(map[string]any{
		"content_base64": base64.StdEncoding.EncodeToString([]byte(`{"a":1}`)),
	})
	if err != nil {
		t.Fatalf("ArchivePayloadBytes base64 returned error: %v", err)
	}
	if string(raw) != `{"a":1}` || name != DefaultConfigArchiveName {
		t.Fatalf("raw=%q name=%q", string(raw), name)
	}

	raw, name, err = ArchivePayloadBytes(map[string]any{"content": `{"b":2}`})
	if err != nil {
		t.Fatalf("ArchivePayloadBytes content returned error: %v", err)
	}
	if string(raw) != `{"b":2}` || name != ConfigArchiveEntryName {
		t.Fatalf("raw=%q name=%q", string(raw), name)
	}

	targetPath := filepath.Join(t.TempDir(), "config.zip")
	if err := os.WriteFile(targetPath, []byte("zip-bytes"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	raw, name, err = ArchivePayloadBytes(map[string]any{"path": targetPath})
	if err != nil {
		t.Fatalf("ArchivePayloadBytes path returned error: %v", err)
	}
	if string(raw) != "zip-bytes" || name != "config.zip" {
		t.Fatalf("raw=%q name=%q", string(raw), name)
	}

	if _, _, err := ArchivePayloadBytes(map[string]any{"path": "content://config.zip"}); err == nil {
		t.Fatal("ArchivePayloadBytes content URI returned nil error, want unsupported path error")
	}
}

func TestWebDAVConfigTargetURLRequestAndErrors(t *testing.T) {
	cfg, err := ParseWebDAVConfig(map[string]any{
		"password":        "pass",
		"remote_path":     "backups/cfst.zip",
		"server_url":      "https://example.com/dav/root",
		"timeout_seconds": 0,
		"username":        "user",
	})
	if err != nil {
		t.Fatalf("ParseWebDAVConfig returned error: %v", err)
	}
	if cfg.TimeoutSeconds != DefaultWebDAVTimeoutSeconds || cfg.RemotePath != "backups/cfst.zip" {
		t.Fatalf("cfg = %#v", cfg)
	}
	targetURL, err := WebDAVTargetURL(cfg)
	if err != nil {
		t.Fatalf("WebDAVTargetURL returned error: %v", err)
	}
	if targetURL != "https://example.com/dav/root/backups/cfst.zip" {
		t.Fatalf("targetURL = %q", targetURL)
	}

	var sawAuth bool
	var sawUserAgent bool
	var sawContentType bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		sawAuth = user == "user" && pass == "pass"
		sawUserAgent = r.Header.Get("User-Agent") == "CFST-GUI/test"
		sawContentType = r.Header.Get("Content-Type") == "application/zip"
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	defer server.Close()
	cfg.ServerURL = server.URL
	targetURL, err = WebDAVTargetURL(cfg)
	if err != nil {
		t.Fatalf("WebDAVTargetURL test server returned error: %v", err)
	}
	status, body, err := WebDAVRequest(nil, cfg, http.MethodPut, targetURL, []byte("archive"), "CFST-GUI/test")
	if err != nil {
		t.Fatalf("WebDAVRequest returned error: %v", err)
	}
	if status != http.StatusCreated || string(body) != "created" || !sawAuth || !sawUserAgent || !sawContentType {
		t.Fatalf("status=%d body=%q auth=%v ua=%v contentType=%v", status, string(body), sawAuth, sawUserAgent, sawContentType)
	}

	message := WebDAVHTTPErrorMessage("WebDAV 备份失败", 500, []byte(strings.Repeat("x", 300)))
	if !strings.Contains(message, "HTTP 500") || len(message) > 280 {
		t.Fatalf("message = %q", message)
	}
}

func TestSetWebDAVTimestampAndSensitiveWarnings(t *testing.T) {
	snapshot := map[string]any{}
	SetWebDAVTimestamp(snapshot, "last_backup_at", "2026-05-09T12:00:00Z")
	backup, ok := snapshot["backup"].(map[string]any)
	if !ok {
		t.Fatalf("snapshot = %#v, want backup map", snapshot)
	}
	webdav, ok := backup["webdav"].(map[string]any)
	if !ok || webdav["last_backup_at"] != "2026-05-09T12:00:00Z" {
		t.Fatalf("webdav = %#v", webdav)
	}
	if got := SensitiveArchiveWarnings(); len(got) != 1 || !strings.Contains(got[0], "Cloudflare Token") {
		t.Fatalf("warnings = %#v", got)
	}
}
