package mobileapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

var (
	cloudflareAPIBaseURL   = appcore.CloudflareAPIBaseURL
	mobileGitHubAPIBaseURL = "https://api.github.com"
)

func NewService() *Service {
	service := &Service{}
	service.core = appcore.NewService(appcore.ServiceOptions{
		AppVersion: "mobile",
		SchedulerDefaults: appcore.SchedulerConfig{
			AutoDNSPush:                true,
			AutoGitHubExport:           true,
			SkipIfActive:               true,
			ConfigSource:               "saved",
			PostRunSourceProfileAction: appcore.SchedulerSourceProfileActionUpdate,
		},
		CloudflareAPIBaseURL: cloudflareAPIBaseURL,
		GitHubAPIBaseURL:     mobileGitHubAPIBaseURL,
		Storage: appcore.StorageLayout{
			ConfigFileName:     "mobile-config.json",
			DraftFileName:      "mobile-draft.json",
			SchedulerFileName:  "scheduler-status.json",
			SourceProfilesFile: "source-profiles.json",
		},
		ConfigPolicy: appcore.ConfigPolicy{
			LegacySchemaVersions: []string{legacyMobileSchemaVersion},
			DefaultSnapshot: func() map[string]any {
				return probecore.DefaultConfigSnapshot(mobileConfigSnapshotOptions())
			},
			SanitizeSnapshot: func(input map[string]any) map[string]any {
				return probecore.SanitizeConfigSnapshot(input, mobileConfigSnapshotOptions())
			},
		},
		ProbeConfigOptions: mobileConfigSnapshotOptions(),
		SourceResolver:     sourceParseResolver,
		SourceHTTPTimeout:  20 * time.Second,
	})
	service.core.SetStorageCommandHooks(appcore.StorageCommandHooks{
		Set: func(payload map[string]any) (appcore.StorageSetResult, error) {
			return service.applyStorageDirectory(payload)
		},
		Health: func(map[string]any) (appcore.StorageHealthResult, error) {
			return appcore.StorageHealthResult{
				Health:  checkMobileStorageHealth(service.basePath()),
				Storage: service.storageStatus(),
			}, nil
		},
	})
	service.core.SetArchiveStorageStateProvider(func() any { return service.storageStatus() })
	return service
}

func (s *Service) SetEventSink(sink EventSink) {
	s.stateMu.Lock()
	s.eventSink = sink
	s.stateMu.Unlock()
	s.core.SetEventSink(appcore.EventSinkFunc(func(_ context.Context, event appcore.ProbeEvent) {
		s.deliverProbeEvent(event)
	}))
}

func (s *Service) Init(baseDir string) string {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = defaultBaseDir()
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return encodeCommand(appcore.NewCommandResult("MOBILE_INIT_FAILED", nil, err.Error(), false, nil, nil))
	}
	s.stateMu.Lock()
	s.baseDir = baseDir
	s.stateMu.Unlock()
	s.core.SetStorageLayout(appcore.StorageLayout{
		Root:               baseDir,
		ConfigFileName:     "mobile-config.json",
		DraftFileName:      "mobile-draft.json",
		SchedulerFileName:  "scheduler-status.json",
		SourceProfilesFile: "source-profiles.json",
	})
	s.core.SetColoPaths(s.coloDictionaryPaths())
	s.core.StartRuntimeCleanup(context.Background())
	return encodeCommand(appcore.NewCommandResult("MOBILE_INIT_OK", map[string]any{
		"base_dir": baseDir, "config_path": s.configPath(),
	}, "Android mobile API 已初始化。", true, nil, nil))
}

func (s *Service) basePath() string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if strings.TrimSpace(s.baseDir) != "" {
		return s.baseDir
	}
	return defaultBaseDir()
}

func defaultBaseDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		dir = "."
	}
	return filepath.Join(dir, "CFST-GUI", "mobile")
}

func (s *Service) configPath() string       { return filepath.Join(s.basePath(), "mobile-config.json") }
func (s *Service) errorLogPath() string     { return filepath.Join(s.logDirectoryPath(), "error-log.txt") }
func (s *Service) logDirectoryPath() string { return filepath.Join(s.basePath(), "logs") }
