package appcore

import "path/filepath"

type StorageLayout struct {
	Root               string
	ConfigFileName     string
	DraftFileName      string
	SchedulerFileName  string
	SourceProfilesFile string
}

func (layout StorageLayout) ConfigPath() string {
	return filepath.Join(layout.Root, layout.ConfigFileName)
}

func (layout StorageLayout) DraftPath() string {
	return filepath.Join(layout.Root, layout.DraftFileName)
}

func (layout StorageLayout) SchedulerPath() string {
	return filepath.Join(layout.Root, layout.SchedulerFileName)
}

func (layout StorageLayout) SourceProfilesPath() string {
	return filepath.Join(layout.Root, layout.SourceProfilesFile)
}

func (layout StorageLayout) TasksRoot() string {
	return filepath.Join(layout.Root, "tasks")
}

func (layout StorageLayout) LogsRoot() string {
	return filepath.Join(layout.Root, "logs")
}

func (layout StorageLayout) ExportsRoot() string {
	return filepath.Join(layout.Root, "exports")
}

func (layout StorageLayout) BackupsRoot() string {
	return filepath.Join(layout.Root, "backups")
}

type ConfigPolicy struct {
	LegacySchemaVersions []string
	DefaultSnapshot      func() map[string]any
	SanitizeSnapshot     func(map[string]any) map[string]any
}
