package mobileapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	mcisengine "github.com/axuitomo/CFST-GUI/internal/mcis/engine"
	mcisprobe "github.com/axuitomo/CFST-GUI/internal/mcis/probe"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func (s *Service) persistSourceStatuses(statuses []desktopSourceStatus) error {
	if len(statuses) == 0 {
		return nil
	}
	raw, err := os.ReadFile(s.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var saved map[string]any
	if _, err := appcore.UnmarshalJSONCompat(raw, &saved); err != nil {
		return err
	}
	snapshot := mapValue(saved["config_snapshot"])
	if len(snapshot) == 0 {
		snapshot = saved
	}
	sourceItems, ok := snapshot["sources"].([]any)
	if !ok {
		return nil
	}
	statusMap := make(map[string]desktopSourceStatus, len(statuses))
	for _, status := range statuses {
		if id := strings.TrimSpace(status.ID); id != "" {
			statusMap[id] = status
		}
	}
	if len(statusMap) == 0 {
		return nil
	}
	for index, item := range sourceItems {
		sourceMap := mapValue(item)
		id := strings.TrimSpace(stringValue(sourceMap["id"], ""))
		status, exists := statusMap[id]
		if !exists {
			continue
		}
		sourceMap["last_fetched_at"] = status.LastFetchedAt
		sourceMap["last_fetched_count"] = status.LastFetchedCount
		sourceMap["status_text"] = status.StatusText
		sourceItems[index] = sourceMap
	}
	snapshot["sources"] = sourceItems
	snapshot = sanitizeMobileConfigSnapshot(snapshot)
	body := map[string]any{
		"config_snapshot": snapshot,
		"saved_at":        nowRFC3339(),
		"schema_version":  schemaVersion,
	}
	encoded, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	return appcore.WriteFileAtomic(s.configPath(), encoded, 0o600)
}

type preparedSources = appcore.PreparedSources

func (s *Service) PreviewSource(payloadJSON string) string {
	return s.inspectSource(payloadJSON, false)
}

func (s *Service) FetchSource(payloadJSON string) string {
	return s.inspectSource(payloadJSON, true)
}

func (s *Service) inspectSource(payloadJSON string, persist bool) string {
	var payload sourcePreviewPayload
	if err := decodeInto(payloadJSON, &payload); err != nil {
		return encodeCommand(commandResultFor("SOURCE_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	if !appcore.HasSourceInput(payload.Source) {
		return encodeCommand(commandResultFor("SOURCE_INPUT_EMPTY", nil, "输入源缺少可读取的内容。", false, nil, nil))
	}
	cfg, _ := configToProbeConfig(payload.Config)
	result, err := s.processSource(cfg, payload.Source, newSourceHTTPClient(cfg), time.Now())
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_READ_FAILED", nil, err.Error(), false, nil, result.Warnings))
	}
	if persist || payload.PersistState {
		if err := s.persistSourceStatuses([]desktopSourceStatus{result.Status}); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("更新输入源状态失败：%v", err))
		}
	}
	previewLimit := payload.PreviewLimit
	if previewLimit <= 0 {
		previewLimit = 16
	}
	previewEntries := result.Entries
	if len(previewEntries) > previewLimit {
		previewEntries = previewEntries[:previewLimit]
	}
	actionLabel := "预览"
	if persist || payload.PersistState {
		actionLabel = "抓取"
	}
	return encodeCommand(commandResultFor("SOURCE_PREVIEW_READY", map[string]any{
		"preview_entries": previewEntries,
		"port_summary":    probecore.PortSummary(result.Entries, result.SourcePorts, cfg.TCPPort, cfg.PortPolicy),
		"source_status":   result.Status,
		"summary": map[string]any{
			"action":        actionLabel,
			"invalid_count": result.InvalidCount,
			"mode":          appcore.SourceIPMode(payload.Source),
			"name":          appcore.SourceName(payload.Source),
			"total_count":   len(result.Entries),
		},
	}, fmt.Sprintf("%s已完成，可预览 %d 条候选。", actionLabel, len(previewEntries)), true, nil, result.Warnings))
}

func (s *Service) processSource(cfg probeConfig, source desktopSource, client *http.Client, now time.Time) (appcore.SourceProcessResult, error) {
	return appcore.ProcessSource(
		source,
		cfg,
		client,
		now,
		func(source desktopSource, cfg probeConfig, client *http.Client) (appcore.SourceContentResult, error) {
			return appcore.LoadSourceContent(source, cfg, client, mobileSourceContentLoadOptions())
		},
		func(raw string, source desktopSource, cfg probeConfig) ([]string, map[string]int, []string, int, error) {
			return s.buildSourceEntriesWithConfig(raw, source, cfg)
		},
	)
}

func (s *Service) buildSourceEntriesWithConfig(raw string, source desktopSource, cfg probeConfig) ([]string, map[string]int, []string, int, error) {
	result, err := appcore.BuildSourceEntriesWithConfig(appcore.SourceEntryBuildOptions{
		Raw:                 raw,
		Source:              source,
		Config:              cfg,
		DefaultIPLimit:      defaultMobileSourceIPLimit,
		Resolver:            sourceParseResolver,
		ColoDictionaryPaths: s.coloDictionaryPaths(),
		MCISRunner: func(tokens []string, source appcore.Source, cfg probecore.ProbeConfig, limit int) ([]string, []string, error) {
			return mobileMCISSearchRunner(tokens, source, cfg, limit)
		},
	})
	return result.Entries, result.SourcePorts, result.Warnings, result.InvalidCount, err
}

var mobileMCISSearchRunner = runMCISSearch

func runMCISSearch(tokens []string, source desktopSource, cfg probeConfig, limit int) ([]string, []string, error) {
	return appcore.RunMCISSearch(tokens, source, cfg, limit)
}

func buildMCISEngineConfig(cfg probeConfig, limit int) mcisengine.Config {
	return appcore.BuildMCISEngineConfig(cfg, limit)
}

func buildMCISProbeConfig(cfg probeConfig) (mcisprobe.Config, []string) {
	return appcore.BuildMCISProbeConfig(cfg)
}

func newSourceHTTPClient(cfg probeConfig) *http.Client {
	return appcore.NewSourceHTTPClient(cfg, appcore.SourceHTTPClientOptions{
		Timeout:      20 * time.Second,
		DisableProxy: true,
	})
}

func (s *Service) prepareSources(cfg probeConfig, sources []desktopSource) preparedSources {
	client := newSourceHTTPClient(cfg)
	now := time.Now()
	return appcore.PrepareSources(appcore.PrepareSourcesOptions{
		Config: cfg,
		ProcessSource: func(source desktopSource) (appcore.SourceProcessResult, error) {
			return s.processSource(cfg, source, client, now)
		},
		Sources: sources,
	})
}

func loadSourceContent(source desktopSource, cfg probeConfig, client *http.Client) (string, error) {
	content, err := appcore.LoadSourceContent(source, cfg, client, mobileSourceContentLoadOptions())
	return content.Raw, err
}

func normalizeMobileSourceURLInput(rawURL string) (string, error) {
	return appcore.NormalizeSourceURLInput(rawURL)
}

func mobileSourceContentLoadOptions() appcore.SourceContentLoadOptions {
	return appcore.SourceContentLoadOptions{
		BuildAttempts: func(primaryURL string, source appcore.Source) []appcore.RemoteSourceAttempt {
			return []appcore.RemoteSourceAttempt{{URL: primaryURL}}
		},
		ShouldRetry: func(statusCode int, err error) bool {
			return false
		},
	}
}
