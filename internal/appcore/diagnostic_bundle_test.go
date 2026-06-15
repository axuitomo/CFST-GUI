package appcore

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildDiagnosticBundleIncludesLogsAndManifest(t *testing.T) {
	logDir := t.TempDir()
	writeDiagnosticTestFile(t, filepath.Join(logDir, "cfip-log.txt"), "debug\n")
	writeDiagnosticTestFile(t, filepath.Join(logDir, "error-log.txt"), "error\n")
	writeDiagnosticTestFile(t, filepath.Join(logDir, "app-2026-06-15.jsonl"), "app\n")
	writeDiagnosticTestFile(t, filepath.Join(logDir, "monitor-2026-06-15.jsonl"), "monitor\n")
	writeDiagnosticTestFile(t, filepath.Join(logDir, "main-heartbeat.json"), "{}\n")
	writeDiagnosticTestFile(t, filepath.Join(logDir, "desktop-config.json"), "secret\n")

	now := time.Date(2026, 6, 15, 10, 20, 30, 0, time.UTC)
	bundle, err := BuildDiagnosticBundle(logDir, "linux/amd64", now, "")
	if err != nil {
		t.Fatalf("BuildDiagnosticBundle failed: %v", err)
	}
	if bundle.FileName != "cfst-diagnostics-20260615-102030.zip" {
		t.Fatalf("FileName = %q", bundle.FileName)
	}

	entries := readDiagnosticBundleEntries(t, bundle.Content)
	for _, name := range []string{
		"manifest.json",
		"logs/app-2026-06-15.jsonl",
		"logs/cfip-log.txt",
		"logs/error-log.txt",
		"logs/main-heartbeat.json",
		"logs/monitor-2026-06-15.jsonl",
	} {
		if _, ok := entries[name]; !ok {
			t.Fatalf("bundle missing %s; entries=%v", name, entries)
		}
	}
	if _, ok := entries["logs/desktop-config.json"]; ok {
		t.Fatalf("bundle included config file")
	}

	var manifest diagnosticBundleManifest
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if manifest.Platform != "linux/amd64" || manifest.LogDirectory != logDir {
		t.Fatalf("manifest = %#v", manifest)
	}
	if len(manifest.Included) != 5 {
		t.Fatalf("manifest included = %#v", manifest.Included)
	}
}

func TestBuildDiagnosticBundleEmpty(t *testing.T) {
	_, err := BuildDiagnosticBundle(t.TempDir(), "test", time.Time{}, "")
	if !errors.Is(err, ErrDiagnosticBundleEmpty) {
		t.Fatalf("err = %v, want ErrDiagnosticBundleEmpty", err)
	}
}

func writeDiagnosticTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readDiagnosticBundleEntries(t *testing.T, raw []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	entries := make(map[string][]byte)
	for _, file := range reader.File {
		handle, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		body, err := io.ReadAll(handle)
		_ = handle.Close()
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}
		entries[file.Name] = body
	}
	return entries
}
