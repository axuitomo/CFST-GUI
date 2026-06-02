package appcore

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
)

type SourceContentResult struct {
	Raw      string
	Warnings []string
}

type SourceProcessResult struct {
	Entries      []string
	InvalidCount int
	SourcePorts  map[string]int
	ColoFilter   string
	ColoMode     string
	Status       SourceStatus
	Warnings     []string
}

type PreparedSources struct {
	Text              string
	FatalErrors       []string
	InvalidCount      int
	SourcePorts       map[string]int
	SourceColoFilters task.SourceColoFilterMap
	SourceStatuses    []SourceStatus
	Warnings          []string
}

type PrepareSourcesOptions struct {
	Config        probecore.ProbeConfig
	ProcessSource func(Source) (SourceProcessResult, error)
	Sources       []Source
}

func HasSourceInput(source Source) bool {
	switch SourceKind(source) {
	case "inline":
		return strings.TrimSpace(source.Content) != ""
	case "file":
		return strings.TrimSpace(source.Path) != ""
	default:
		return strings.TrimSpace(source.URL) != ""
	}
}

func SourceName(source Source) string {
	if name := strings.TrimSpace(source.Name); name != "" {
		return name
	}
	if label := strings.TrimSpace(source.Label); label != "" {
		return label
	}
	switch SourceKind(source) {
	case "file":
		return "本地文件来源"
	case "inline":
		return "手动输入来源"
	default:
		return "远程来源"
	}
}

func SourceKind(source Source) string {
	switch strings.ToLower(strings.TrimSpace(source.Kind)) {
	case "inline", "file":
		return strings.ToLower(strings.TrimSpace(source.Kind))
	default:
		return "url"
	}
}

func SourceEnabled(source Source) bool {
	if source.Enabled {
		return true
	}
	return source.ID == "" && source.Name == "" && source.IPLimit == 0 && source.IPMode == ""
}

func SourceIPLimit(source Source, fallback int) int {
	if source.IPLimit <= 0 {
		return fallback
	}
	return source.IPLimit
}

func SourceIPMode(source Source) string {
	if strings.EqualFold(strings.TrimSpace(source.IPMode), "mcis") {
		return "mcis"
	}
	return "traverse"
}

func ProcessSource(
	source Source,
	cfg probecore.ProbeConfig,
	client *http.Client,
	now time.Time,
	loadContent func(Source, probecore.ProbeConfig, *http.Client) (SourceContentResult, error),
	buildEntries func(string, Source, probecore.ProbeConfig) ([]string, map[string]int, []string, int, error),
) (SourceProcessResult, error) {
	status := SourceStatus{
		ID:               strings.TrimSpace(source.ID),
		LastFetchedAt:    strings.TrimSpace(source.LastFetchedAt),
		LastFetchedCount: source.LastFetchedCount,
		StatusText:       strings.TrimSpace(source.StatusText),
	}

	content, err := loadContent(source, cfg, client)
	if err != nil {
		status.LastFetchedAt = now.Format(time.RFC3339)
		status.LastFetchedCount = 0
		status.StatusText = fmt.Sprintf("最近读取失败 · %s", err.Error())
		return SourceProcessResult{Status: status}, err
	}

	entries, sourcePorts, warnings, invalidCount, err := buildEntries(content.Raw, source, cfg)
	warnings = append(content.Warnings, warnings...)
	if err != nil {
		status.LastFetchedAt = now.Format(time.RFC3339)
		status.LastFetchedCount = 0
		status.StatusText = fmt.Sprintf("最近读取失败 · %s", err.Error())
		return SourceProcessResult{
			InvalidCount: invalidCount,
			Status:       status,
			Warnings:     warnings,
		}, err
	}

	action := "载入"
	if SourceKind(source) == "url" {
		action = "抓取"
	}
	status.LastFetchedAt = now.Format(time.RFC3339)
	status.LastFetchedCount = len(entries)
	if len(entries) > 0 {
		status.StatusText = fmt.Sprintf("最近%s成功 · %s · %d 条", action, now.Format("2006/1/2 15:04:05"), len(entries))
	} else {
		status.StatusText = fmt.Sprintf("最近%s完成 · %s · 0 条", action, now.Format("2006/1/2 15:04:05"))
	}

	return SourceProcessResult{
		Entries:      entries,
		InvalidCount: invalidCount,
		SourcePorts:  sourcePorts,
		ColoFilter:   strings.TrimSpace(source.ColoFilter),
		ColoMode:     task.NormalizeColoFilterMode(source.ColoFilterMode),
		Status:       status,
		Warnings:     warnings,
	}, nil
}

func PrepareSources(options PrepareSourcesOptions) PreparedSources {
	parts := make([]string, 0)
	statuses := make([]SourceStatus, 0, len(options.Sources))
	warnings := make([]string, 0)
	fatalErrors := make([]string, 0)
	invalidCount := 0
	sourcePorts := make(map[string]int)
	var sourceColoFilters task.SourceColoFilterMap
	if options.Config.SourceColoFilterPhase == probecore.SourceColoFilterPhaseStage2 {
		sourceColoFilters = make(task.SourceColoFilterMap)
	}

	for index, source := range options.Sources {
		name := strings.TrimSpace(SourceName(source))
		if name == "" {
			name = fmt.Sprintf("输入源 %d", index+1)
		}

		status := SourceStatus{
			ID:               strings.TrimSpace(source.ID),
			LastFetchedAt:    strings.TrimSpace(source.LastFetchedAt),
			LastFetchedCount: source.LastFetchedCount,
			StatusText:       strings.TrimSpace(source.StatusText),
		}

		if !SourceEnabled(source) {
			if status.StatusText == "" {
				status.StatusText = "已停用，启动任务时不会读取该输入源。"
			}
			statuses = append(statuses, status)
			continue
		}

		result, err := options.ProcessSource(source)
		if err != nil {
			statuses = append(statuses, result.Status)
			invalidCount += result.InvalidCount
			message := fmt.Sprintf("输入源 %s 读取失败：%v", name, err)
			warnings = append(warnings, message)
			if isMissingColoFileError(err) {
				fatalErrors = append(fatalErrors, message)
			}
			warnings = append(warnings, result.Warnings...)
			continue
		}

		warnings = append(warnings, result.Warnings...)
		invalidCount += result.InvalidCount
		for token, port := range result.SourcePorts {
			sourcePorts[token] = port
		}
		if len(result.Entries) > 0 {
			parts = append(parts, strings.Join(result.Entries, "\n"))
			if sourceColoFilters != nil {
				task.MergeSourceColoFiltersWithMode(sourceColoFilters, result.Entries, result.ColoFilter, result.ColoMode)
			}
		}
		statuses = append(statuses, result.Status)
	}

	return PreparedSources{
		Text:              strings.Join(parts, "\n"),
		FatalErrors:       dedupeSourceStrings(fatalErrors),
		InvalidCount:      invalidCount,
		SourcePorts:       probecore.CloneStringIntMap(sourcePorts),
		SourceColoFilters: sourceColoFilters,
		SourceStatuses:    statuses,
		Warnings:          dedupeSourceStrings(warnings),
	}
}

func isMissingColoFileError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "COLO 文件不存在")
}

func dedupeSourceStrings(values []string) []string {
	if len(values) <= 1 {
		return values
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
