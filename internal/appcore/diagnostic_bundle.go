package appcore

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

var ErrDiagnosticBundleEmpty = errors.New("diagnostic bundle has no log files")

type DiagnosticBundle struct {
	Content      []byte
	FileName     string
	GeneratedAt  string
	Included     []string
	LogDirectory string
	Missing      []string
}

type diagnosticBundleManifest struct {
	GeneratedAt  string   `json:"generated_at"`
	Included     []string `json:"included"`
	LogDirectory string   `json:"log_dir"`
	Missing      []string `json:"missing"`
	Platform     string   `json:"platform"`
}

type diagnosticBundleEntry struct {
	archiveName string
	path        string
}

func BuildDiagnosticBundle(logDir string, platform string, now time.Time, requestedName string) (DiagnosticBundle, error) {
	if now.IsZero() {
		now = time.Now()
	}
	generatedAt := now.Format(time.RFC3339)
	entries, included, missing := diagnosticBundleEntries(logDir)
	if len(entries) == 0 {
		return DiagnosticBundle{
			FileName:     DiagnosticBundleFileName(requestedName, now),
			GeneratedAt:  generatedAt,
			Included:     included,
			LogDirectory: logDir,
			Missing:      missing,
		}, ErrDiagnosticBundleEmpty
	}

	manifest := diagnosticBundleManifest{
		GeneratedAt:  generatedAt,
		Included:     included,
		LogDirectory: logDir,
		Missing:      missing,
		Platform:     strings.TrimSpace(platform),
	}
	manifestRaw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return DiagnosticBundle{}, err
	}

	buffer := bytes.NewBuffer(nil)
	writer := zip.NewWriter(buffer)
	if err := writeDiagnosticBundleEntry(writer, "manifest.json", manifestRaw, now); err != nil {
		_ = writer.Close()
		return DiagnosticBundle{}, err
	}
	for _, entry := range entries {
		raw, err := os.ReadFile(entry.path)
		if err != nil {
			_ = writer.Close()
			return DiagnosticBundle{}, err
		}
		if err := writeDiagnosticBundleEntry(writer, entry.archiveName, raw, now); err != nil {
			_ = writer.Close()
			return DiagnosticBundle{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return DiagnosticBundle{}, err
	}

	return DiagnosticBundle{
		Content:      buffer.Bytes(),
		FileName:     DiagnosticBundleFileName(requestedName, now),
		GeneratedAt:  generatedAt,
		Included:     included,
		LogDirectory: logDir,
		Missing:      missing,
	}, nil
}

func DiagnosticBundleFileName(requestedName string, now time.Time) string {
	name := strings.TrimSpace(requestedName)
	if name == "" {
		if now.IsZero() {
			now = time.Now()
		}
		name = "cfst-diagnostics-" + now.Format("20060102-150405") + ".zip"
	}
	name = probecore.SanitizeTemplateFileName(filepath.Base(name))
	if name == "" {
		return "cfst-diagnostics.zip"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		name += ".zip"
	}
	return name
}

func diagnosticBundleEntries(logDir string) ([]diagnosticBundleEntry, []string, []string) {
	exactNames := []string{"cfip-log.txt", "error-log.txt", "main-heartbeat.json"}
	patterns := []string{"app-*.jsonl", "monitor-*.jsonl"}
	entries := make([]diagnosticBundleEntry, 0)
	included := make([]string, 0)
	missing := make([]string, 0)

	for _, name := range exactNames {
		path := filepath.Join(logDir, name)
		if diagnosticReadableFile(path) {
			archiveName := filepath.ToSlash(filepath.Join("logs", name))
			entries = append(entries, diagnosticBundleEntry{archiveName: archiveName, path: path})
			included = append(included, archiveName)
			continue
		}
		missing = append(missing, filepath.ToSlash(filepath.Join("logs", name)))
	}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(logDir, pattern))
		readable := make([]string, 0, len(matches))
		for _, match := range matches {
			if diagnosticReadableFile(match) {
				readable = append(readable, match)
			}
		}
		sort.Strings(readable)
		if len(readable) == 0 {
			missing = append(missing, filepath.ToSlash(filepath.Join("logs", pattern)))
			continue
		}
		for _, path := range readable {
			archiveName := filepath.ToSlash(filepath.Join("logs", filepath.Base(path)))
			entries = append(entries, diagnosticBundleEntry{archiveName: archiveName, path: path})
			included = append(included, archiveName)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].archiveName < entries[j].archiveName
	})
	sort.Strings(included)
	sort.Strings(missing)
	return entries, included, missing
}

func diagnosticReadableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func writeDiagnosticBundleEntry(writer *zip.Writer, name string, raw []byte, modTime time.Time) error {
	header := &zip.FileHeader{
		Name:   filepath.ToSlash(name),
		Method: zip.Deflate,
	}
	header.SetModTime(modTime)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(raw)
	return err
}
