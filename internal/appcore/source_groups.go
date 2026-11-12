package appcore

import (
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
)

func sourceGroupIPMode(mode string, sources []Source) string {
	if normalized := probecore.NormalizeSourceGroupIPMode(mode); normalized != "" {
		return normalized
	}
	if len(sources) == 0 {
		return "traverse"
	}
	mode = SourceIPMode(sources[0])
	for _, source := range sources[1:] {
		if SourceIPMode(source) != mode {
			return ""
		}
	}
	return mode
}

func normalizeSourceGroupColoFilterMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case task.ColoFilterModeAllow, task.ColoFilterModeDeny:
		return task.NormalizeColoFilterMode(mode)
	default:
		return ""
	}
}

func sourceGroupColoFilterSettings(sources []Source) (string, string) {
	if len(sources) == 0 {
		return "", task.ColoFilterModeAllow
	}
	filter := strings.TrimSpace(sources[0].ColoFilter)
	mode := task.NormalizeColoFilterMode(sources[0].ColoFilterMode)
	for _, source := range sources[1:] {
		if strings.TrimSpace(source.ColoFilter) != filter || task.NormalizeColoFilterMode(source.ColoFilterMode) != mode {
			return "", ""
		}
	}
	return filter, mode
}

func sourcesWithGroupColoFilter(sources []Source, filter, mode string) []Source {
	cloned := CloneSources(sources)
	if mode = normalizeSourceGroupColoFilterMode(mode); mode != "" {
		for index := range cloned {
			cloned[index].ColoFilter = strings.TrimSpace(filter)
			cloned[index].ColoFilterMode = mode
		}
	}
	return cloned
}

func normalizeSourceProfileColoFilters(store *SourceProfileStore) {
	for index := range store.Items {
		profile := &store.Items[index]
		profile.ColoFilterMode = normalizeSourceGroupColoFilterMode(profile.ColoFilterMode)
		if profile.ColoFilterMode == "" {
			profile.ColoFilter, profile.ColoFilterMode = sourceGroupColoFilterSettings(profile.Sources)
		}
		if profile.ColoFilterMode != "" {
			profile.Sources = sourcesWithGroupColoFilter(profile.Sources, profile.ColoFilter, profile.ColoFilterMode)
		}
	}
}

func requestedSourceGroupColoFilter(filter, mode string, store SourceProfileStore, profileID string, sources []Source) (string, string) {
	if normalized := normalizeSourceGroupColoFilterMode(mode); normalized != "" {
		return strings.TrimSpace(filter), normalized
	}
	for _, profile := range store.Items {
		if profile.ID == profileID && normalizeSourceGroupColoFilterMode(profile.ColoFilterMode) != "" {
			return strings.TrimSpace(profile.ColoFilter), normalizeSourceGroupColoFilterMode(profile.ColoFilterMode)
		}
	}
	return sourceGroupColoFilterSettings(sources)
}

func sourceGroupModeFromSnapshot(snapshot map[string]any) string {
	return probecore.NormalizeSourceGroupIPMode(stringValue(firstNonNil(snapshot["source_ip_mode"], snapshot["sourceIpMode"]), ""))
}

func sourcesWithGroupIPMode(sources []Source, mode string) []Source {
	cloned := CloneSources(sources)
	if mode = probecore.NormalizeSourceGroupIPMode(mode); mode != "" {
		for index := range cloned {
			cloned[index].IPMode = mode
		}
	}
	return cloned
}

func setSnapshotSourceGroup(snapshot map[string]any, sources []Source, mode string) {
	snapshot["source_ip_mode"] = sourceGroupIPMode(mode, sources)
	snapshot["sources"] = sourcesWithGroupIPMode(sources, stringValue(snapshot["source_ip_mode"], ""))
}

func requestedSourceGroupIPMode(requested *string, store SourceProfileStore, profileID string, sources []Source) string {
	if requested != nil {
		return sourceGroupIPMode(*requested, sources)
	}
	for _, profile := range store.Items {
		if profile.ID == profileID {
			return sourceGroupIPMode(profile.IPMode, sources)
		}
	}
	return sourceGroupIPMode("", sources)
}

func normalizeSourceProfileIPModes(store *SourceProfileStore) {
	for index := range store.Items {
		profile := &store.Items[index]
		profile.IPMode = sourceGroupIPMode(profile.IPMode, profile.Sources)
		profile.Sources = sourcesWithGroupIPMode(profile.Sources, profile.IPMode)
	}
}

func activeSourceProfileIPMode(store SourceProfileStore) string {
	for _, profile := range store.Items {
		if profile.ID == store.ActiveProfileID {
			return sourceGroupIPMode(profile.IPMode, profile.Sources)
		}
	}
	return "traverse"
}

func sourceGroupWarnings(snapshot map[string]any, sources []Source) []string {
	if sourceGroupModeFromSnapshot(snapshot) == "" && sourceGroupIPMode("", sources) == "" {
		return []string{"旧输入组包含不同 IP 模式，暂按原来源设置运行；请在输入组中选择统一模式。"}
	}
	return nil
}
