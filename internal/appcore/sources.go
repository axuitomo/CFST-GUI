package appcore

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
)

type SourceContentResult struct {
	Raw         string
	Diagnostics SourceContentDiagnostics
	Warnings    []string
}

type SourceContentDiagnostics struct {
	CacheHit             bool
	CacheKey             string
	CacheKind            string
	ConditionalHit       bool
	PersistentCacheHit   bool
	PersistentCacheWrite bool
	StatusCode           int
	UsedURL              string
}

type SourceProcessTimings struct {
	FetchDuration time.Duration
	BuildDuration time.Duration
	MCISDuration  time.Duration
	TotalDuration time.Duration
}

type SourceProcessDiagnostics struct {
	CacheHit             bool   `json:"cache_hit"`
	CacheKind            string `json:"cache_kind,omitempty"`
	ConditionalHit       bool   `json:"conditional_hit,omitempty"`
	FetchDurationMS      int64  `json:"fetch_duration_ms"`
	ID                   string `json:"id,omitempty"`
	Kind                 string `json:"kind"`
	BuildDurationMS      int64  `json:"build_duration_ms"`
	MCISDurationMS       int64  `json:"mcis_duration_ms,omitempty"`
	Name                 string `json:"name"`
	PersistentCacheHit   bool   `json:"persistent_cache_hit,omitempty"`
	PersistentCacheWrite bool   `json:"persistent_cache_write,omitempty"`
	StatusCode           int    `json:"status_code,omitempty"`
	TotalDurationMS      int64  `json:"total_duration_ms"`
	UsedURL              string `json:"used_url,omitempty"`
}

type SourceProcessResult struct {
	Entries          []string
	InvalidCount     int
	SourcePorts      map[string]int
	ColoFilter       string
	ColoFilterActive bool
	ColoFilterColos  []string
	ColoMode         string
	Diagnostics      SourceProcessDiagnostics
	Status           SourceStatus
	Timings          SourceProcessTimings
	Warnings         []string
}

type PreparedSources struct {
	Text              string
	FatalErrors       []string
	InvalidCount      int
	SourcePorts       map[string]int
	SourceColoFilters task.SourceColoFilterMap
	SourceDiagnostics []SourceProcessDiagnostics
	SourceStatuses    []SourceStatus
	Warnings          []string
}

type PrepareSourcesOptions struct {
	Config        probecore.ProbeConfig
	Concurrency   int
	ProcessSource func(Source) (SourceProcessResult, error)
	Sources       []Source
}

const defaultPrepareSourcesConcurrency = 4

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
	buildEntries func(string, Source, probecore.ProbeConfig) (probecore.SourceBuildResult, error),
) (SourceProcessResult, error) {
	start := time.Now()
	status := SourceStatus{
		ID:               strings.TrimSpace(source.ID),
		LastFetchedAt:    strings.TrimSpace(source.LastFetchedAt),
		LastFetchedCount: source.LastFetchedCount,
		StatusText:       strings.TrimSpace(source.StatusText),
	}

	fetchStart := time.Now()
	content, err := loadContent(source, cfg, client)
	fetchDuration := time.Since(fetchStart)
	if err != nil {
		status.LastFetchedAt = now.Format(time.RFC3339)
		status.LastFetchedCount = 0
		status.StatusText = fmt.Sprintf("最近读取失败 · %s", err.Error())
		timings := SourceProcessTimings{FetchDuration: fetchDuration, TotalDuration: time.Since(start)}
		return SourceProcessResult{Diagnostics: sourceProcessDiagnostics(source, content.Diagnostics, timings, 0), Status: status, Timings: timings}, err
	}

	buildStart := time.Now()
	buildResult, err := buildEntries(content.Raw, source, cfg)
	buildDuration := time.Since(buildStart)
	entries := buildResult.Entries
	sourcePorts := buildResult.SourcePorts
	warnings := buildResult.Warnings
	invalidCount := buildResult.InvalidCount
	warnings = append(content.Warnings, warnings...)
	timings := SourceProcessTimings{
		FetchDuration: fetchDuration,
		BuildDuration: buildDuration,
		MCISDuration:  buildResult.MCISDuration,
		TotalDuration: time.Since(start),
	}
	if err != nil {
		status.LastFetchedAt = now.Format(time.RFC3339)
		status.LastFetchedCount = 0
		status.StatusText = fmt.Sprintf("最近读取失败 · %s", err.Error())
		return SourceProcessResult{
			InvalidCount:     invalidCount,
			ColoFilterActive: buildResult.ColoFilterActive,
			ColoFilterColos:  append([]string(nil), buildResult.ColoFilterColos...),
			Diagnostics:      sourceProcessDiagnostics(source, content.Diagnostics, timings, buildResult.MCISDuration),
			Status:           status,
			Timings:          timings,
			Warnings:         warnings,
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
		Entries:          entries,
		InvalidCount:     invalidCount,
		SourcePorts:      sourcePorts,
		ColoFilter:       strings.TrimSpace(source.ColoFilter),
		ColoFilterActive: buildResult.ColoFilterActive,
		ColoFilterColos:  append([]string(nil), buildResult.ColoFilterColos...),
		ColoMode:         task.NormalizeColoFilterMode(source.ColoFilterMode),
		Diagnostics:      sourceProcessDiagnostics(source, content.Diagnostics, timings, buildResult.MCISDuration),
		Status:           status,
		Timings:          timings,
		Warnings:         warnings,
	}, nil
}

func PrepareSources(options PrepareSourcesOptions) PreparedSources {
	parts := make([]string, 0)
	statuses := make([]SourceStatus, 0, len(options.Sources))
	warnings := make([]string, 0)
	fatalErrors := make([]string, 0)
	invalidCount := 0
	sourcePorts := make(map[string]int)
	diagnostics := make([]SourceProcessDiagnostics, 0, len(options.Sources))
	results := processEnabledSources(options)
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
		if !HasSourceInput(source) {
			statuses = append(statuses, status)
			continue
		}

		result := results[index]
		if result.Err != nil {
			statuses = append(statuses, result.Result.Status)
			diagnostics = append(diagnostics, result.Result.Diagnostics)
			invalidCount += result.Result.InvalidCount
			message := fmt.Sprintf("输入源 %s 读取失败：%v", name, result.Err)
			warnings = append(warnings, message)
			if isMissingColoFileError(result.Err) {
				fatalErrors = append(fatalErrors, message)
			}
			warnings = append(warnings, result.Result.Warnings...)
			continue
		}

		warnings = append(warnings, result.Result.Warnings...)
		diagnostics = append(diagnostics, result.Result.Diagnostics)
		invalidCount += result.Result.InvalidCount
		for token, port := range result.Result.SourcePorts {
			sourcePorts[token] = port
		}
		if len(result.Result.Entries) > 0 {
			parts = append(parts, strings.Join(result.Result.Entries, "\n"))
			if sourceColoFilters != nil {
				task.MergeSourceColoFiltersWithResolvedColos(sourceColoFilters, result.Result.Entries, result.Result.ColoFilterColos, result.Result.ColoMode, result.Result.ColoFilterActive)
			}
		}
		statuses = append(statuses, result.Result.Status)
	}

	return PreparedSources{
		Text:              strings.Join(parts, "\n"),
		FatalErrors:       dedupeSourceStrings(fatalErrors),
		InvalidCount:      invalidCount,
		SourcePorts:       probecore.CloneStringIntMap(sourcePorts),
		SourceColoFilters: sourceColoFilters,
		SourceDiagnostics: trimEmptySourceDiagnostics(diagnostics),
		SourceStatuses:    statuses,
		Warnings:          dedupeSourceStrings(warnings),
	}
}

type preparedSourceProcessOutcome struct {
	Result SourceProcessResult
	Err    error
}

func processEnabledSources(options PrepareSourcesOptions) []preparedSourceProcessOutcome {
	outcomes := make([]preparedSourceProcessOutcome, len(options.Sources))
	if options.ProcessSource == nil || len(options.Sources) == 0 {
		return outcomes
	}

	workerCount := prepareSourcesConcurrency(options)
	if workerCount > enabledSourceCount(options.Sources) {
		workerCount = enabledSourceCount(options.Sources)
	}
	if workerCount <= 0 {
		return outcomes
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for workerIndex := 0; workerIndex < workerCount; workerIndex++ {
		go func() {
			defer wg.Done()
			for index := range jobs {
				outcomes[index] = processSourceSafely(options.Sources[index], options.ProcessSource)
			}
		}()
	}

	for index, source := range options.Sources {
		if shouldProcessSource(source) {
			jobs <- index
		}
	}
	close(jobs)
	wg.Wait()
	return outcomes
}

func processSourceSafely(source Source, processSource func(Source) (SourceProcessResult, error)) (outcome preparedSourceProcessOutcome) {
	defer func() {
		if recovered := recover(); recovered != nil {
			outcome = preparedSourceProcessOutcome{
				Result: SourceProcessResult{Status: SourceStatus{
					ID:               strings.TrimSpace(source.ID),
					LastFetchedAt:    strings.TrimSpace(source.LastFetchedAt),
					LastFetchedCount: source.LastFetchedCount,
					StatusText:       fmt.Sprintf("最近读取失败 · 输入源处理异常：%v", recovered),
				}},
				Err: fmt.Errorf("输入源处理异常：%v", recovered),
			}
		}
	}()

	result, err := processSource(source)
	return preparedSourceProcessOutcome{Result: result, Err: err}
}

func enabledSourceCount(sources []Source) int {
	count := 0
	for _, source := range sources {
		if shouldProcessSource(source) {
			count++
		}
	}
	return count
}

func shouldProcessSource(source Source) bool {
	return SourceEnabled(source) && HasSourceInput(source)
}

func prepareSourcesConcurrency(options PrepareSourcesOptions) int {
	workerCount := options.Concurrency
	if workerCount <= 0 {
		workerCount = defaultPrepareSourcesConcurrency
	}
	if workerCount > defaultPrepareSourcesConcurrency {
		workerCount = defaultPrepareSourcesConcurrency
	}
	return workerCount
}

func sourceProcessDiagnostics(source Source, content SourceContentDiagnostics, timings SourceProcessTimings, mcisDuration time.Duration) SourceProcessDiagnostics {
	return SourceProcessDiagnostics{
		CacheHit:             content.CacheHit,
		CacheKind:            content.CacheKind,
		ConditionalHit:       content.ConditionalHit,
		FetchDurationMS:      timings.FetchDuration.Milliseconds(),
		ID:                   strings.TrimSpace(source.ID),
		Kind:                 SourceKind(source),
		BuildDurationMS:      timings.BuildDuration.Milliseconds(),
		MCISDurationMS:       mcisDuration.Milliseconds(),
		Name:                 SourceName(source),
		PersistentCacheHit:   content.PersistentCacheHit,
		PersistentCacheWrite: content.PersistentCacheWrite,
		StatusCode:           content.StatusCode,
		TotalDurationMS:      timings.TotalDuration.Milliseconds(),
		UsedURL:              strings.TrimSpace(content.UsedURL),
	}
}

func trimEmptySourceDiagnostics(values []SourceProcessDiagnostics) []SourceProcessDiagnostics {
	result := make([]SourceProcessDiagnostics, 0, len(values))
	for _, value := range values {
		if value.ID == "" && value.Name == "" && value.Kind == "" && value.TotalDurationMS == 0 {
			continue
		}
		result = append(result, value)
	}
	return result
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
