package appcore

import (
	"context"
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
	Entries          []string
	MCISCandidates   []probecore.MCISCandidate
	InvalidCount     int
	SourcePorts      map[string]int
	ColoFilter       string
	ColoFilterActive bool
	ColoFilterColos  []string
	ColoMode         string
	Status           SourceStatus
	Warnings         []string
}

type PreparedSources struct {
	Error             error
	Entries           []string
	Text              string
	FatalErrors       []string
	InvalidCount      int
	MCISCandidates    map[string]probecore.MCISCandidate
	SourcePorts       map[string]int
	SourceColoFilters task.SourceColoFilterMap
	SourceStatuses    []SourceStatus
	Warnings          []string
}

type PrepareSourcesOptions struct {
	Context       context.Context
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

// SourceEnabled 判断输入源是否参与任务。SourcesFromAny 会把缺少 enabled 字段的
// 旧配置补成启用，这里保留全空（旧版单源）也视为启用的兜底。
func SourceEnabled(source Source) bool {
	if source.Enabled {
		return true
	}
	return source.ID == "" && source.Name == "" && source.IPLimit == 0 && source.IPMode == ""
}

// SourceIPLimit 把输入源配置的 ip_limit 翻译成具体的候选池容量。
// 0 表示使用 fallback；负值表示"不限量"，原样下传，由构建器按可配置的池上限执行。
func SourceIPLimit(source Source, fallback int) int {
	if source.IPLimit < 0 {
		return source.IPLimit
	}
	if source.IPLimit == 0 {
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
	status := SourceStatus{
		ID:               strings.TrimSpace(source.ID),
		LastFetchedAt:    strings.TrimSpace(source.LastFetchedAt),
		LastFetchedCount: source.LastFetchedCount,
		StatusText:       strings.TrimSpace(source.StatusText),
	}

	content, err := loadContent(source, cfg, client)
	if err != nil {
		// 失败只改状态文本：last_fetched_at/count 表示最后一次成功读取，
		// 失败时覆盖会让一次网络抖动抹掉历史可用信息。
		status.StatusText = fmt.Sprintf("最近读取失败 · %s", err.Error())
		return SourceProcessResult{Status: status}, err
	}

	buildResult, err := buildEntries(content.Raw, source, cfg)
	entries := buildResult.Entries
	sourcePorts := buildResult.SourcePorts
	warnings := buildResult.Warnings
	invalidCount := buildResult.InvalidCount
	warnings = append(content.Warnings, warnings...)
	if err != nil {
		status.StatusText = fmt.Sprintf("最近读取失败 · %s", err.Error())
		return SourceProcessResult{
			InvalidCount:     invalidCount,
			ColoFilterActive: buildResult.ColoFilterActive,
			ColoFilterColos:  append([]string(nil), buildResult.ColoFilterColos...),
			Status:           status,
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
		MCISCandidates:   append([]probecore.MCISCandidate(nil), buildResult.MCISCandidates...),
		InvalidCount:     invalidCount,
		SourcePorts:      sourcePorts,
		ColoFilter:       strings.TrimSpace(source.ColoFilter),
		ColoFilterActive: buildResult.ColoFilterActive,
		ColoFilterColos:  append([]string(nil), buildResult.ColoFilterColos...),
		ColoMode:         task.NormalizeColoFilterMode(source.ColoFilterMode),
		Status:           status,
		Warnings:         warnings,
	}, nil
}

func PrepareSources(options PrepareSourcesOptions) PreparedSources {
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	entries := make([]string, 0)
	attempted, failed := 0, 0
	statuses := make([]SourceStatus, 0, len(options.Sources))
	warnings := make([]string, 0)
	fatalErrors := make([]string, 0)
	invalidCount := 0
	sourcePorts := make(map[string]int)
	mcisCandidates := make(map[string]probecore.MCISCandidate)
	var sourceColoFilters task.SourceColoFilterMap
	if options.Config.SourceColoFilterPhase == probecore.SourceColoFilterPhaseStage2 {
		sourceColoFilters = make(task.SourceColoFilterMap)
	}

	for index, source := range options.Sources {
		if err := ctx.Err(); err != nil {
			return PreparedSources{Error: err, SourceStatuses: statuses}
		}
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

		attempted++
		result, err := options.ProcessSource(source)
		if err != nil {
			failed++
			if ctx.Err() != nil {
				return PreparedSources{Error: ctx.Err(), SourceStatuses: statuses}
			}
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
		for _, candidate := range result.MCISCandidates {
			existing, ok := mcisCandidates[candidate.IP]
			if !ok || candidate.TotalMS < existing.TotalMS {
				mcisCandidates[candidate.IP] = candidate
			}
		}
		if len(result.Entries) > 0 {
			entries = append(entries, result.Entries...)
			if sourceColoFilters != nil {
				task.MergeSourceColoFiltersWithResolvedColos(sourceColoFilters, result.Entries, result.ColoFilterColos, result.ColoMode, result.ColoFilterActive)
			}
		}
		statuses = append(statuses, result.Status)
	}
	if err := ctx.Err(); err != nil {
		return PreparedSources{Error: err, SourceStatuses: statuses}
	}
	if attempted > 0 && failed == attempted && len(fatalErrors) == 0 {
		// 输入源直连读取、不走环境代理，全部失败时给出这条线索，省得只看到若干网络错误。
		warnings = append(warnings, "所有启用的输入源都读取失败：输入源为直连读取，不使用 HTTP_PROXY/HTTPS_PROXY 等环境代理变量，请检查网络与来源可达性。")
	}

	return PreparedSources{
		Entries:           entries,
		Text:              strings.Join(entries, "\n"),
		FatalErrors:       dedupeSourceStrings(fatalErrors),
		InvalidCount:      invalidCount,
		MCISCandidates:    mcisCandidates,
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
