package mobileapi

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

const storageSchemaVersion = "cfst-gui-storage-v1"

type mobileStorageBootstrap struct {
	DisplayName    string `json:"display_name,omitempty"`
	PortableMode   bool   `json:"portable_mode"`
	SchemaVersion  string `json:"schema_version"`
	SetupCompleted bool   `json:"setup_completed"`
	StorageDir     string `json:"storage_dir,omitempty"`
	StorageURI     string `json:"storage_uri,omitempty"`
	UpdatedAt      string `json:"updated_at"`
}

type mobileStorageHealth struct {
	CheckedAt    string `json:"checked_at"`
	Exists       bool   `json:"exists"`
	FreeBytes    int64  `json:"free_bytes"`
	IsDir        bool   `json:"is_dir"`
	Message      string `json:"message"`
	Path         string `json:"path"`
	PortableMode bool   `json:"portable_mode"`
	Writable     bool   `json:"writable"`
}

func (s *Service) storageBootstrapPath() string {
	return filepath.Join(s.basePath(), "storage.json")
}

func (s *Service) storageStatus() map[string]any {
	health := checkMobileStorageHealth(s.basePath())
	return map[string]any{
		"backend":         "private",
		"bootstrap_path":  s.storageBootstrapPath(),
		"current_dir":     s.basePath(),
		"default_dir":     s.basePath(),
		"display_name":    "",
		"health":          health,
		"last_sync_at":    "",
		"last_sync_error": "",
		"log_uri":         "",
		"permission_ok":   true,
		"portable_mode":   false,
		"runtime_dir":     s.basePath(),
		"setup_completed": true,
		"setup_required":  false,
		"storage_uri":     "",
		"writable":        health.Writable,
	}
}

func (s *Service) writeStorageBootstrap(bootstrap mobileStorageBootstrap) error {
	bootstrap.SchemaVersion = storageSchemaVersion
	bootstrap.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := os.MkdirAll(filepath.Dir(s.storageBootstrapPath()), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(bootstrap, "", "  ")
	if err != nil {
		return err
	}
	return appcore.WriteFileAtomic(s.storageBootstrapPath(), raw, 0o600)
}

func checkMobileStorageHealth(path string) mobileStorageHealth {
	health := mobileStorageHealth{
		CheckedAt: time.Now().Format(time.RFC3339),
		FreeBytes: -1,
		Path:      path,
	}
	if strings.TrimSpace(path) == "" {
		health.Message = "应用私有目录为空。"
		return health
	}
	info, err := os.Stat(path)
	if err == nil {
		health.Exists = true
		health.IsDir = info.IsDir()
	} else if errors.Is(err, os.ErrNotExist) {
		health.IsDir = true
	} else {
		health.Message = err.Error()
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
	testPath := filepath.Join(path, ".cfst-gui-write-test")
	if err := os.WriteFile(testPath, []byte("ok"), 0o600); err != nil {
		health.Message = err.Error()
		return health
	}
	_ = os.Remove(testPath)
	health.Exists = true
	health.Writable = true
	health.Message = "应用私有目录可用。"
	return health
}

func (s *Service) applyStorageDirectory(_ map[string]any) (appcore.StorageSetResult, error) {
	bootstrap := mobileStorageBootstrap{
		PortableMode:   false,
		SchemaVersion:  storageSchemaVersion,
		SetupCompleted: true,
		StorageDir:     s.basePath(),
	}
	if err := s.writeStorageBootstrap(bootstrap); err != nil {
		return appcore.StorageSetResult{Storage: s.storageStatus()}, err
	}
	return appcore.StorageSetResult{
		Migration: map[string]any{"copied": []string{}, "failed": []string{}, "skipped": []string{}},
		Storage:   s.storageStatus(),
	}, nil
}
