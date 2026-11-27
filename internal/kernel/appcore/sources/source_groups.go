package sources

import (
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	configvalue "github.com/axuitomo/CFST-GUI/internal/kernel/domain/config"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/task"
)

func SourceGroupIPMode(mode string, sources []contracts.Source) string {
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

func SourceGroupColoFilterSettings(sources []contracts.Source) (string, string) {
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

func SourcesWithGroupColoFilter(sources []contracts.Source, filter, mode string) []contracts.Source {
	cloned := CloneSources(sources)
	if mode = normalizeSourceGroupColoFilterMode(mode); mode != "" {
		for index := range cloned {
			cloned[index].ColoFilter = strings.TrimSpace(filter)
			cloned[index].ColoFilterMode = mode
		}
	}
	return cloned
}

func NormalizeSourceProfileColoFilters(store *contracts.SourceProfileStore) {
	for index := range store.Items {
		profile := &store.Items[index]
		profile.ColoFilterMode = normalizeSourceGroupColoFilterMode(profile.ColoFilterMode)
		if profile.ColoFilterMode == "" {
			profile.ColoFilter, profile.ColoFilterMode = SourceGroupColoFilterSettings(profile.Sources)
		}
		if profile.ColoFilterMode != "" {
			profile.Sources = SourcesWithGroupColoFilter(profile.Sources, profile.ColoFilter, profile.ColoFilterMode)
		}
	}
}

func RequestedSourceGroupColoFilter(filter, mode string, store contracts.SourceProfileStore, profileID string, sources []contracts.Source) (string, string) {
	if normalized := normalizeSourceGroupColoFilterMode(mode); normalized != "" {
		return strings.TrimSpace(filter), normalized
	}
	for _, profile := range store.Items {
		if profile.ID == profileID && normalizeSourceGroupColoFilterMode(profile.ColoFilterMode) != "" {
			return strings.TrimSpace(profile.ColoFilter), normalizeSourceGroupColoFilterMode(profile.ColoFilterMode)
		}
	}
	return SourceGroupColoFilterSettings(sources)
}

func SourceGroupModeFromSnapshot(snapshot map[string]any) string {
	return probecore.NormalizeSourceGroupIPMode(configvalue.String(configvalue.FirstNonNil(snapshot["source_ip_mode"], snapshot["sourceIpMode"]), ""))
}

func SourcesWithGroupIPMode(sources []contracts.Source, mode string) []contracts.Source {
	cloned := CloneSources(sources)
	if mode = probecore.NormalizeSourceGroupIPMode(mode); mode != "" {
		for index := range cloned {
			cloned[index].IPMode = mode
		}
	}
	return cloned
}

func SetSnapshotSourceGroup(snapshot map[string]any, sources []contracts.Source, mode string) {
	snapshot["source_ip_mode"] = SourceGroupIPMode(mode, sources)
	snapshot["sources"] = SourcesWithGroupIPMode(sources, configvalue.String(snapshot["source_ip_mode"], ""))
}

func RequestedSourceGroupIPMode(requested *string, store contracts.SourceProfileStore, profileID string, sources []contracts.Source) string {
	if requested != nil {
		return SourceGroupIPMode(*requested, sources)
	}
	for _, profile := range store.Items {
		if profile.ID == profileID {
			return SourceGroupIPMode(profile.IPMode, sources)
		}
	}
	return SourceGroupIPMode("", sources)
}

func NormalizeSourceProfileIPModes(store *contracts.SourceProfileStore) {
	for index := range store.Items {
		profile := &store.Items[index]
		profile.IPMode = SourceGroupIPMode(profile.IPMode, profile.Sources)
		profile.Sources = SourcesWithGroupIPMode(profile.Sources, profile.IPMode)
	}
}

func ActiveSourceProfileIPMode(store contracts.SourceProfileStore) string {
	for _, profile := range store.Items {
		if profile.ID == store.ActiveProfileID {
			return SourceGroupIPMode(profile.IPMode, profile.Sources)
		}
	}
	return "traverse"
}

func SourceGroupWarnings(snapshot map[string]any, sources []contracts.Source) []string {
	if SourceGroupModeFromSnapshot(snapshot) == "" && SourceGroupIPMode("", sources) == "" {
		return []string{"旧输入组包含不同 IP 模式，暂按原来源设置运行；请在输入组中选择统一模式。"}
	}
	return nil
}
