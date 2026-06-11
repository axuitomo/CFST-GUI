package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	mcisengine "github.com/axuitomo/CFST-GUI/internal/mcis/engine"
	mcisprobe "github.com/axuitomo/CFST-GUI/internal/mcis/probe"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

type DesktopSourcePreviewPayload struct {
	Config       map[string]any `json:"config"`
	PersistState bool           `json:"persist_state"`
	PreviewLimit int            `json:"preview_limit"`
	Source       DesktopSource  `json:"source"`
}

func (a *App) PreviewDesktopSource(payload DesktopSourcePreviewPayload) DesktopCommandResult {
	return a.inspectDesktopSource(payload, false)
}

func (a *App) FetchDesktopSource(payload DesktopSourcePreviewPayload) DesktopCommandResult {
	return a.inspectDesktopSource(payload, true)
}

func (a *App) inspectDesktopSource(payload DesktopSourcePreviewPayload, persist bool) DesktopCommandResult {
	source := payload.Source
	if !appcore.HasSourceInput(source) {
		return desktopCommandResult("SOURCE_INPUT_EMPTY", nil, "输入源缺少可读取的内容。", false, nil, nil)
	}

	cfg, _ := desktopConfigToProbeConfig(payload.Config)
	now := time.Now()
	result, err := processDesktopSource(cfg, source, newDesktopSourceHTTPClient(cfg), now)
	if err != nil {
		return desktopCommandResult("SOURCE_READ_FAILED", nil, err.Error(), false, nil, result.Warnings)
	}

	if persist || payload.PersistState {
		if err := persistDesktopSourceStatuses([]DesktopSourceStatus{result.Status}); err != nil {
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

	return desktopCommandResult("SOURCE_PREVIEW_READY", map[string]any{
		"preview_entries": previewEntries,
		"port_summary":    probecore.PortSummary(result.Entries, result.SourcePorts, cfg.TCPPort, cfg.PortPolicy),
		"source_status":   result.Status,
		"summary": map[string]any{
			"action":        actionLabel,
			"invalid_count": result.InvalidCount,
			"mode":          appcore.SourceIPMode(source),
			"name":          appcore.SourceName(source),
			"total_count":   len(result.Entries),
		},
	}, fmt.Sprintf("%s已完成，可预览 %d 条候选。", actionLabel, len(previewEntries)), true, nil, dedupeStrings(result.Warnings))
}

func processDesktopSource(cfg ProbeConfig, source DesktopSource, client *http.Client, now time.Time) (appcore.SourceProcessResult, error) {
	return appcore.ProcessSource(
		source,
		cfg,
		client,
		now,
		func(source DesktopSource, cfg ProbeConfig, client *http.Client) (appcore.SourceContentResult, error) {
			content, err := loadDesktopSourceContent(source, cfg, client)
			return appcore.SourceContentResult(content), err
		},
		func(raw string, source DesktopSource, cfg ProbeConfig) (probecore.SourceBuildResult, error) {
			return buildDesktopSourceEntriesResultWithConfig(raw, source, cfg)
		},
	)
}

func buildDesktopSourceEntriesWithConfig(raw string, source DesktopSource, cfg ProbeConfig) ([]string, map[string]int, []string, int, error) {
	result, err := buildDesktopSourceEntriesResultWithConfig(raw, source, cfg)
	return result.Entries, result.SourcePorts, result.Warnings, result.InvalidCount, err
}

func buildDesktopSourceEntriesResultWithConfig(raw string, source DesktopSource, cfg ProbeConfig) (probecore.SourceBuildResult, error) {
	result, err := appcore.BuildSourceEntriesWithConfig(appcore.SourceEntryBuildOptions{
		Raw:                 raw,
		Source:              source,
		Config:              cfg,
		DefaultIPLimit:      defaultDesktopSourceIPLimit,
		Resolver:            sourceParseResolver,
		ColoDictionaryPaths: desktopColoDictionaryPaths(),
		MCISRunner: func(tokens []string, source appcore.Source, cfg probecore.ProbeConfig, limit int) ([]string, []string, error) {
			return desktopMCISSearchRunner(tokens, source, cfg, limit)
		},
	})
	return result, err
}

var desktopMCISSearchRunner = runDesktopMCISSearch

func runDesktopMCISSearch(tokens []string, source DesktopSource, cfg ProbeConfig, limit int) ([]string, []string, error) {
	return appcore.RunMCISSearch(tokens, source, cfg, limit)
}

func buildDesktopMCISEngineConfig(cfg ProbeConfig, limit int) mcisengine.Config {
	return appcore.BuildMCISEngineConfig(cfg, limit)
}

func buildDesktopMCISProbeConfig(cfg ProbeConfig) (mcisprobe.Config, []string) {
	return appcore.BuildMCISProbeConfig(cfg)
}

func newDesktopSourceHTTPClient(cfg ProbeConfig) *http.Client {
	return appcore.NewSourceHTTPClient(cfg, appcore.SourceHTTPClientOptions{
		Timeout:      30 * time.Second,
		DisableProxy: true,
	})
}
