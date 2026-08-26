package appcore

import "path/filepath"

const (
	FileConfig         = "config.json"
	FileDesktopDraft   = "desktop-draft.json"
	FileScheduler      = "scheduler.json"
	FileSourceProfiles = "source-profiles.json"
	FileDebugLog       = "cfip-log.txt"
	FileErrorLog       = "error-log.txt"
	FileBootstrap      = "storage.json"
	FileHotDB          = "hot.db"
	DirTasks           = "tasks"
	DirLogs            = "logs"
	DirConfig          = "config"
	DirImports         = "imports"
	DirTaskData        = "task-data"
	DirExports         = "exports"
	DirBackups         = "backups"
	DirDiagnostics     = "diagnostics"
)

type StorageLayout struct {
	Root string
	// Legacy file-name fields remain supported for existing desktop/mobile callers.
	ConfigFileName     string
	DraftFileName      string
	SchedulerFileName  string
	SourceProfilesFile string
	HotDBPath          string
	HotLogsDir         string
	WarmConfigDir      string
	WarmImportsDir     string
	WarmTaskDataDir    string
	ColdExportsDir     string
	ColdBackupsDir     string
	ColdDiagnosticsDir string
}

// NewStorageLayout creates the v1.1 layered layout.
func NewStorageLayout(root string) StorageLayout {
	return StorageLayout{
		Root:               root,
		ConfigFileName:     FileConfig,
		DraftFileName:      FileDesktopDraft,
		SchedulerFileName:  FileScheduler,
		SourceProfilesFile: FileSourceProfiles,
		HotDBPath:          filepath.Join(root, FileHotDB),
		HotLogsDir:         filepath.Join(root, DirLogs),
		WarmConfigDir:      filepath.Join(root, DirConfig),
		WarmImportsDir:     filepath.Join(root, DirImports),
		WarmTaskDataDir:    filepath.Join(root, DirTaskData),
		ColdExportsDir:     filepath.Join(root, DirExports),
		ColdBackupsDir:     filepath.Join(root, DirBackups),
		ColdDiagnosticsDir: filepath.Join(root, DirDiagnostics),
	}
}

func (layout StorageLayout) configDir() string {
	if layout.WarmConfigDir != "" {
		return layout.WarmConfigDir
	}
	return layout.Root
}

func (layout StorageLayout) hotLogsDir() string {
	if layout.HotLogsDir != "" {
		return layout.HotLogsDir
	}
	return layout.LogsRoot()
}

func (layout StorageLayout) ConfigPath() string {
	name := layout.ConfigFileName
	if name == "" {
		name = FileConfig
	}
	return filepath.Join(layout.configDir(), name)
}

func (layout StorageLayout) DraftPath() string {
	name := layout.DraftFileName
	if name == "" {
		name = FileDesktopDraft
	}
	return filepath.Join(layout.configDir(), name)
}

func (layout StorageLayout) SchedulerPath() string {
	name := layout.SchedulerFileName
	if name == "" {
		name = FileScheduler
	}
	return filepath.Join(layout.configDir(), name)
}

func (layout StorageLayout) SourceProfilesPath() string {
	name := layout.SourceProfilesFile
	if name == "" {
		name = FileSourceProfiles
	}
	return filepath.Join(layout.configDir(), name)
}

func (layout StorageLayout) TasksRoot() string {
	return filepath.Join(layout.Root, DirTasks)
}

func (layout StorageLayout) LogsRoot() string {
	return filepath.Join(layout.Root, DirLogs)
}

func (layout StorageLayout) ExportsRoot() string {
	if layout.ColdExportsDir != "" {
		return layout.ColdExportsDir
	}
	return filepath.Join(layout.Root, DirExports)
}

func (layout StorageLayout) BackupsRoot() string {
	if layout.ColdBackupsDir != "" {
		return layout.ColdBackupsDir
	}
	return filepath.Join(layout.Root, DirBackups)
}

func (layout StorageLayout) HotDB() string {
	if layout.HotDBPath != "" {
		return layout.HotDBPath
	}
	return filepath.Join(layout.Root, FileHotDB)
}

func (layout StorageLayout) DebugLogPath() string {
	return filepath.Join(layout.hotLogsDir(), FileDebugLog)
}

func (layout StorageLayout) ErrorLogPath() string {
	return filepath.Join(layout.hotLogsDir(), FileErrorLog)
}

func (layout StorageLayout) TaskDataPath(taskID string) string {
	dir := layout.WarmTaskDataDir
	if dir == "" {
		dir = filepath.Join(layout.Root, DirTaskData)
	}
	return filepath.Join(dir, taskID+".json")
}

// LegacyPath returns a pre-tiering path during migration.
// Deprecated: use a semantic path method after migration.
func (layout StorageLayout) LegacyPath(name string) string {
	return filepath.Join(layout.Root, name)
}

type ConfigPolicy struct {
	LegacySchemaVersions []string
	DefaultSnapshot      func() map[string]any
	SanitizeSnapshot     func(map[string]any) map[string]any
}
