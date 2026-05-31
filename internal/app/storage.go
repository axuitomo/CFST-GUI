package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

const (
	storageBootstrapFileName       = "storage.json"
	storageSchemaVersion           = "cfst-gui-storage-v1"
	desktopDraftFileName           = "desktop-draft.json"
	pipelineProfilesFileName       = "pipeline-profiles.json"
	pipelineProfilesSchemaVersion  = "cfst-gui-pipeline-profiles-v1"
	pipelineWorkspaceFileName      = "pipeline-workspace.json"
	pipelineWorkspaceSchemaVersion = "cfst-gui-pipeline-workspace-v1"
	sourceProfilesFileName         = "source-profiles.json"
	sourceProfilesSchemaVersion    = "cfst-gui-source-profiles-v1"
	defaultSourceProfileID         = "source-profile-default"
)

type storageBootstrap struct {
	DisplayName                     string `json:"display_name,omitempty"`
	LegacyStorageDir                string `json:"legacy_storage_dir,omitempty"`
	LegacyStorageMigrationAttempted bool   `json:"legacy_storage_migration_attempted,omitempty"`
	LegacyStorageMigrationCompleted bool   `json:"legacy_storage_migration_completed,omitempty"`
	LegacyStorageMigrationError     string `json:"legacy_storage_migration_error,omitempty"`
	PortableMode                    bool   `json:"portable_mode"`
	SchemaVersion                   string `json:"schema_version"`
	SetupCompleted                  bool   `json:"setup_completed"`
	StorageDir                      string `json:"storage_dir,omitempty"`
	StorageURI                      string `json:"storage_uri,omitempty"`
	UpdatedAt                       string `json:"updated_at"`
}

type storageHealth struct {
	CheckedAt    string `json:"checked_at"`
	Exists       bool   `json:"exists"`
	FreeBytes    int64  `json:"free_bytes"`
	IsDir        bool   `json:"is_dir"`
	Message      string `json:"message"`
	Path         string `json:"path"`
	PortableMode bool   `json:"portable_mode"`
	Writable     bool   `json:"writable"`
}

type storageStatus struct {
	BootstrapPath                   string        `json:"bootstrap_path"`
	CurrentDir                      string        `json:"current_dir"`
	DefaultDir                      string        `json:"default_dir"`
	DisplayName                     string        `json:"display_name,omitempty"`
	Health                          storageHealth `json:"health"`
	LegacyStorageDir                string        `json:"legacy_storage_dir,omitempty"`
	LegacyStorageMigrationAttempted bool          `json:"legacy_storage_migration_attempted,omitempty"`
	LegacyStorageMigrationCompleted bool          `json:"legacy_storage_migration_completed,omitempty"`
	LegacyStorageMigrationError     string        `json:"legacy_storage_migration_error,omitempty"`
	PortableMode                    bool          `json:"portable_mode"`
	SetupCompleted                  bool          `json:"setup_completed"`
	SetupRequired                   bool          `json:"setup_required"`
	StorageURI                      string        `json:"storage_uri,omitempty"`
	Writable                        bool          `json:"writable"`
}

type storageMigrationSummary struct {
	Copied  []string `json:"copied"`
	Skipped []string `json:"skipped"`
	Failed  []string `json:"failed"`
}

type pipelineProfileItem = appcore.PipelineProfile
type pipelineProfileStore = appcore.PipelineProfileStore
type pipelineTargetItem = appcore.PipelineTarget
type pipelineTemplateItem = appcore.PipelineTemplate
type pipelineWorkspace = appcore.PipelineWorkspace
type sourceProfileItem = appcore.SourceProfileItem
type sourceProfileStore = appcore.SourceProfileStore

func defaultStorageDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		dir = "."
	}
	return filepath.Join(dir, "CFST-GUI")
}

func defaultExportDir() string {
	if portableDir, ok := portableDataDir(); ok {
		return filepath.Join(portableDir, "exports")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, "Downloads", "CFST-GUI")
	}
	return filepath.Join(defaultStorageDir(), "exports")
}

func storageBootstrapPath() string {
	return filepath.Join(defaultStorageDir(), storageBootstrapFileName)
}

func storageRoot() string {
	state := resolveStorageState()
	return state.CurrentDir
}

func resolveStorageState() storageStatus {
	defaultDir := defaultStorageDir()
	bootstrapPath := storageBootstrapPath()
	if portableDir, ok := portableDataDir(); ok {
		health := checkStorageHealthForPath(portableDir, true)
		if health.Writable {
			return storageStatus{
				BootstrapPath:  bootstrapPath,
				CurrentDir:     portableDir,
				DefaultDir:     defaultDir,
				Health:         health,
				PortableMode:   true,
				SetupCompleted: true,
				SetupRequired:  false,
				Writable:       true,
			}
		}
	}

	bootstrap, err := readStorageBootstrap()
	setupCompleted := true
	currentDir := defaultDir
	displayName := ""
	legacyStorageDir := ""
	legacyMigrationAttempted := false
	legacyMigrationCompleted := false
	legacyMigrationError := ""
	if err == nil {
		bootstrap = migrateLegacyStorageBootstrap(bootstrap, defaultDir)
		displayName = strings.TrimSpace(bootstrap.DisplayName)
		legacyStorageDir = strings.TrimSpace(bootstrap.LegacyStorageDir)
		legacyMigrationAttempted = bootstrap.LegacyStorageMigrationAttempted
		legacyMigrationCompleted = bootstrap.LegacyStorageMigrationCompleted
		legacyMigrationError = strings.TrimSpace(bootstrap.LegacyStorageMigrationError)
		setupCompleted = bootstrap.SetupCompleted
	}
	health := checkStorageHealthForPath(currentDir, false)
	if legacyMigrationError != "" {
		health.Message = "旧储存目录迁移失败：" + legacyMigrationError
	}
	return storageStatus{
		BootstrapPath:                   bootstrapPath,
		CurrentDir:                      currentDir,
		DefaultDir:                      defaultDir,
		DisplayName:                     displayName,
		Health:                          health,
		LegacyStorageDir:                legacyStorageDir,
		LegacyStorageMigrationAttempted: legacyMigrationAttempted,
		LegacyStorageMigrationCompleted: legacyMigrationCompleted,
		LegacyStorageMigrationError:     legacyMigrationError,
		PortableMode:                    false,
		SetupCompleted:                  setupCompleted,
		SetupRequired:                   !setupCompleted,
		Writable:                        health.Writable,
	}
}

func readStorageBootstrap() (storageBootstrap, error) {
	raw, err := os.ReadFile(storageBootstrapPath())
	if err != nil {
		return storageBootstrap{}, err
	}
	var bootstrap storageBootstrap
	if err := json.Unmarshal(raw, &bootstrap); err != nil {
		return storageBootstrap{}, err
	}
	return bootstrap, nil
}

func writeStorageBootstrap(bootstrap storageBootstrap) error {
	if strings.TrimSpace(bootstrap.SchemaVersion) == "" {
		bootstrap.SchemaVersion = storageSchemaVersion
	}
	bootstrap.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := os.MkdirAll(filepath.Dir(storageBootstrapPath()), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(bootstrap, "", "  ")
	if err != nil {
		return err
	}
	return appcore.WriteFileAtomic(storageBootstrapPath(), raw, 0o600)
}

func migrateLegacyStorageBootstrap(bootstrap storageBootstrap, fixedRoot string) storageBootstrap {
	fixedRoot = strings.TrimSpace(fixedRoot)
	legacyRoot := strings.TrimSpace(bootstrap.StorageDir)
	if legacyRoot == "" || samePath(legacyRoot, fixedRoot) {
		shouldWrite := !bootstrap.SetupCompleted || strings.TrimSpace(bootstrap.StorageDir) != fixedRoot || strings.TrimSpace(bootstrap.StorageURI) != ""
		bootstrap.StorageDir = fixedRoot
		bootstrap.StorageURI = ""
		bootstrap.SetupCompleted = true
		if shouldWrite {
			_ = writeStorageBootstrap(bootstrap)
		}
		return bootstrap
	}

	previousLegacy := strings.TrimSpace(bootstrap.LegacyStorageDir)
	alreadyAttemptedForRoot := previousLegacy != "" && samePath(previousLegacy, legacyRoot) && bootstrap.LegacyStorageMigrationAttempted
	if !alreadyAttemptedForRoot {
		summary := migrateStorageFiles(legacyRoot, fixedRoot)
		bootstrap.LegacyStorageDir = legacyRoot
		bootstrap.LegacyStorageMigrationAttempted = true
		bootstrap.LegacyStorageMigrationCompleted = len(summary.Failed) == 0
		bootstrap.LegacyStorageMigrationError = strings.Join(summary.Failed, "；")
	}
	bootstrap.DisplayName = ""
	bootstrap.SetupCompleted = true
	bootstrap.StorageDir = fixedRoot
	bootstrap.StorageURI = ""
	if err := writeStorageBootstrap(bootstrap); err != nil {
		bootstrap.LegacyStorageMigrationError = firstNonEmpty(bootstrap.LegacyStorageMigrationError, err.Error())
	}
	return bootstrap
}

func portableDataDir() (string, bool) {
	root := strings.TrimSpace(os.Getenv("CFST_GUI_PORTABLE_ROOT"))
	if root != "" {
		return filepath.Join(root, "data"), true
	}
	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		return "", false
	}
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, "portable.json")); err != nil {
		return "", false
	}
	return filepath.Join(dir, "data"), true
}

func checkStorageHealthForPath(path string, portable bool) storageHealth {
	path = strings.TrimSpace(path)
	health := storageHealth{
		CheckedAt:    time.Now().Format(time.RFC3339),
		FreeBytes:    -1,
		Path:         path,
		PortableMode: portable,
	}
	if path == "" {
		health.Message = "应用数据目录为空。"
		return health
	}
	info, statErr := os.Stat(path)
	if statErr == nil {
		health.Exists = true
		health.IsDir = info.IsDir()
	} else if errors.Is(statErr, os.ErrNotExist) {
		health.Exists = false
		health.IsDir = true
	} else {
		health.Message = statErr.Error()
		return health
	}
	if !health.IsDir {
		health.Message = "目标路径不是目录。"
		return health
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		health.Message = err.Error()
		return health
	}
	probePath := filepath.Join(path, ".cfst-gui-write-test")
	if err := os.WriteFile(probePath, []byte("ok"), 0o600); err != nil {
		health.Message = err.Error()
		return health
	}
	_ = os.Remove(probePath)
	health.Exists = true
	health.Writable = true
	health.Message = "应用数据目录可用。"
	if free, ok := storageFreeBytes(path); ok {
		health.FreeBytes = free
	}
	return health
}

func setStorageDirectory(payload map[string]any) (storageStatus, storageMigrationSummary, error) {
	summary := storageMigrationSummary{}
	bootstrap, err := readStorageBootstrap()
	if err != nil {
		bootstrap = storageBootstrap{}
	}
	bootstrap.DisplayName = ""
	bootstrap.PortableMode = false
	bootstrap.SchemaVersion = storageSchemaVersion
	bootstrap.SetupCompleted = true
	bootstrap.StorageDir = defaultStorageDir()
	bootstrap.StorageURI = ""
	if err := writeStorageBootstrap(bootstrap); err != nil {
		return resolveStorageState(), summary, err
	}
	return resolveStorageState(), summary, nil
}

func migrateStorageFiles(oldRoot, newRoot string) storageMigrationSummary {
	summary := storageMigrationSummary{}
	oldRoot = strings.TrimSpace(oldRoot)
	newRoot = strings.TrimSpace(newRoot)
	if oldRoot == "" || newRoot == "" || samePath(oldRoot, newRoot) {
		return summary
	}
	entries := []string{
		"desktop-config.json",
		desktopDraftFileName,
		"config.json",
		"cfip-log.txt",
		"local-ip-ranges.csv",
		"cloudflare-colos.csv",
		"cloudflare-colos-ipv4.csv",
		"cloudflare-colos-ipv6.csv",
		"cloudflare-colo-locations.json",
		"cloudflare-countries.json",
		"result.csv",
		pipelineProfilesFileName,
		sourceProfilesFileName,
		"exports",
		"imports",
		"backups",
	}
	for _, name := range entries {
		src := filepath.Join(oldRoot, name)
		dst := filepath.Join(newRoot, name)
		if _, err := os.Stat(src); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			summary.Failed = append(summary.Failed, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			summary.Skipped = append(summary.Skipped, name)
			continue
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			summary.Failed = append(summary.Failed, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if err := copyPath(src, dst); err != nil {
			summary.Failed = append(summary.Failed, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		summary.Copied = append(summary.Copied, name)
	}
	return summary
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			target := filepath.Join(dst, rel)
			if entry.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			if _, err := os.Stat(target); err == nil {
				return nil
			}
			return copyFile(path, target)
		})
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer output.Close()
	_, err = io.Copy(output, input)
	return err
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func pipelineProfilesPath() string {
	return filepath.Join(storageRoot(), pipelineProfilesFileName)
}

func pipelineWorkspacePath() string {
	return filepath.Join(storageRoot(), pipelineWorkspaceFileName)
}

func desktopDraftFilePath() string {
	return filepath.Join(storageRoot(), desktopDraftFileName)
}

func removeDesktopDraft() error {
	err := os.Remove(desktopDraftFilePath())
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func loadPipelineProfileStore() (pipelineProfileStore, error) {
	return appcore.LoadPipelineProfileStore(pipelineProfilesPath(), pipelineProfilesSchemaVersion, sanitizeDesktopConfigSnapshot)
}

func savePipelineProfileStore(store pipelineProfileStore) error {
	return appcore.SavePipelineProfileStore(pipelineProfilesPath(), store, pipelineProfilesSchemaVersion, sanitizeDesktopConfigSnapshot)
}

func loadPipelineWorkspace() (pipelineWorkspace, bool, error) {
	return appcore.LoadPipelineWorkspace(
		pipelineWorkspacePath(),
		pipelineProfilesPath(),
		pipelineWorkspaceSchemaVersion,
		time.Now().Format(time.RFC3339),
		sanitizeDesktopConfigSnapshot,
	)
}

func savePipelineWorkspace(workspace pipelineWorkspace) error {
	return appcore.SavePipelineWorkspace(
		pipelineWorkspacePath(),
		workspace,
		pipelineWorkspaceSchemaVersion,
		time.Now().Format(time.RFC3339),
		sanitizeDesktopConfigSnapshot,
	)
}

func sourceProfilesPath() string {
	return filepath.Join(storageRoot(), sourceProfilesFileName)
}

func loadSourceProfileStore() (sourceProfileStore, error) {
	return appcore.LoadSourceProfileStore(sourceProfilesPath(), sourceProfilesSchemaVersion)
}

func saveSourceProfileStore(store sourceProfileStore) error {
	return appcore.SaveSourceProfileStore(sourceProfilesPath(), store, sourceProfilesSchemaVersion)
}

func sanitizeTemplateFileName(value string) string {
	return probecore.SanitizeTemplateFileName(value)
}

func renderExportFileTemplate(template, taskID, profileName string, now time.Time) string {
	return probecore.RenderExportFileTemplate(template, taskID, profileName, now)
}
