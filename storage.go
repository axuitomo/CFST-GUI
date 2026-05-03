package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	storageBootstrapFileName = "storage.json"
	storageSchemaVersion     = "cfst-gui-storage-v1"
	profilesFileName         = "profiles.json"
	profilesSchemaVersion    = "cfst-gui-profiles-v1"
)

type storageBootstrap struct {
	DisplayName    string `json:"display_name,omitempty"`
	PortableMode   bool   `json:"portable_mode"`
	SchemaVersion  string `json:"schema_version"`
	SetupCompleted bool   `json:"setup_completed"`
	StorageDir     string `json:"storage_dir,omitempty"`
	StorageURI     string `json:"storage_uri,omitempty"`
	UpdatedAt      string `json:"updated_at"`
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
	BootstrapPath  string        `json:"bootstrap_path"`
	CurrentDir     string        `json:"current_dir"`
	DefaultDir     string        `json:"default_dir"`
	DisplayName    string        `json:"display_name,omitempty"`
	Health         storageHealth `json:"health"`
	PortableMode   bool          `json:"portable_mode"`
	SetupCompleted bool          `json:"setup_completed"`
	SetupRequired  bool          `json:"setup_required"`
	StorageURI     string        `json:"storage_uri,omitempty"`
	Writable       bool          `json:"writable"`
}

type storageMigrationSummary struct {
	Copied  []string `json:"copied"`
	Skipped []string `json:"skipped"`
	Failed  []string `json:"failed"`
}

type profileItem struct {
	ConfigSnapshot map[string]any `json:"config_snapshot"`
	CreatedAt      string         `json:"created_at"`
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	UpdatedAt      string         `json:"updated_at"`
}

type profileStore struct {
	ActiveProfileID string        `json:"active_profile_id"`
	Items           []profileItem `json:"items"`
	SchemaVersion   string        `json:"schema_version"`
	UpdatedAt       string        `json:"updated_at"`
}

func defaultStorageDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		dir = "."
	}
	return filepath.Join(dir, "CFST-GUI")
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
	setupCompleted := false
	currentDir := defaultDir
	displayName := ""
	storageURI := ""
	if err == nil {
		if strings.TrimSpace(bootstrap.StorageDir) != "" {
			currentDir = strings.TrimSpace(bootstrap.StorageDir)
		}
		displayName = strings.TrimSpace(bootstrap.DisplayName)
		storageURI = strings.TrimSpace(bootstrap.StorageURI)
		setupCompleted = bootstrap.SetupCompleted
	}
	health := checkStorageHealthForPath(currentDir, false)
	return storageStatus{
		BootstrapPath:  bootstrapPath,
		CurrentDir:     currentDir,
		DefaultDir:     defaultDir,
		DisplayName:    displayName,
		Health:         health,
		PortableMode:   false,
		SetupCompleted: setupCompleted,
		SetupRequired:  !setupCompleted,
		StorageURI:     storageURI,
		Writable:       health.Writable,
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
	return os.WriteFile(storageBootstrapPath(), raw, 0o600)
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
		health.Message = "储存目录为空。"
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
	health.Message = "储存目录可用。"
	if free, ok := storageFreeBytes(path); ok {
		health.FreeBytes = free
	}
	return health
}

func setStorageDirectory(payload map[string]any) (storageStatus, storageMigrationSummary, error) {
	oldRoot := storageRoot()
	target := strings.TrimSpace(stringValue(firstNonNil(payload["storage_dir"], payload["storageDir"], payload["path"], payload["directory"]), ""))
	useDefault := boolValue(firstNonNil(payload["use_default"], payload["useDefault"], payload["reset_default"], payload["resetDefault"]), false)
	if useDefault || target == "" {
		target = defaultStorageDir()
	}
	target = filepath.Clean(target)
	health := checkStorageHealthForPath(target, false)
	if !health.Writable {
		return resolveStorageState(), storageMigrationSummary{}, fmt.Errorf("储存目录不可写：%s", health.Message)
	}
	migrate := boolValue(firstNonNil(payload["migrate"], payload["copy_existing"], payload["copyExisting"]), true)
	summary := storageMigrationSummary{}
	if migrate && !samePath(oldRoot, target) {
		summary = migrateStorageFiles(oldRoot, target)
		if len(summary.Failed) > 0 {
			return resolveStorageState(), summary, fmt.Errorf("迁移部分文件失败，未切换储存目录：%s", strings.Join(summary.Failed, "；"))
		}
	}
	bootstrap := storageBootstrap{
		DisplayName:    strings.TrimSpace(stringValue(firstNonNil(payload["display_name"], payload["displayName"]), "")),
		PortableMode:   false,
		SchemaVersion:  storageSchemaVersion,
		SetupCompleted: true,
		StorageDir:     target,
		StorageURI:     strings.TrimSpace(stringValue(firstNonNil(payload["storage_uri"], payload["storageUri"], payload["uri"]), "")),
	}
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
		"config.json",
			"cfip-log.txt",
			"local-ip-ranges.csv",
			"cloudflare-colos.csv",
			"cloudflare-colo-locations.json",
			"cloudflare-countries.json",
			"result.csv",
		"profiles.json",
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

func profilesPath() string {
	return filepath.Join(storageRoot(), profilesFileName)
}

func loadProfileStore() (profileStore, error) {
	store := profileStore{
		Items:         []profileItem{},
		SchemaVersion: profilesSchemaVersion,
	}
	raw, err := os.ReadFile(profilesPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return store, err
	}
	if err := json.Unmarshal(raw, &store); err != nil {
		return store, err
	}
	if store.Items == nil {
		store.Items = []profileItem{}
	}
	if store.SchemaVersion == "" {
		store.SchemaVersion = profilesSchemaVersion
	}
	return store, nil
}

func saveProfileStore(store profileStore) error {
	store.SchemaVersion = profilesSchemaVersion
	store.UpdatedAt = time.Now().Format(time.RFC3339)
	if store.Items == nil {
		store.Items = []profileItem{}
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(profilesPath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(profilesPath(), raw, 0o600)
}

func activeProfileName() string {
	store, err := loadProfileStore()
	if err != nil || strings.TrimSpace(store.ActiveProfileID) == "" {
		return ""
	}
	for _, item := range store.Items {
		if item.ID == store.ActiveProfileID {
			return item.Name
		}
	}
	return ""
}

func sanitizeTemplateFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	value = replacer.Replace(value)
	value = strings.TrimSpace(value)
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	return value
}

func renderExportFileTemplate(template, taskID, profileName string, now time.Time) string {
	template = strings.TrimSpace(template)
	if template == "" {
		return ""
	}
	if profileName == "" {
		profileName = "default"
	}
	replacements := map[string]string{
		"{date}":    now.Format("20060102"),
		"{profile}": sanitizeTemplateFileName(profileName),
		"{task_id}": sanitizeTemplateFileName(taskID),
		"{time}":    now.Format("150405"),
	}
	for key, value := range replacements {
		template = strings.ReplaceAll(template, key, value)
	}
	return sanitizeTemplateFileName(template)
}
