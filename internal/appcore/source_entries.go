package appcore

import (
	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/sourceparse"
)

type SourceMCISRunner func(tokens []string, source Source, cfg probecore.ProbeConfig, limit int) ([]string, []string, error)

type SourceEntryBuildOptions struct {
	Raw                 string
	Source              Source
	Config              probecore.ProbeConfig
	DefaultIPLimit      int
	Resolver            sourceparse.Resolver
	ColoDictionaryPaths colodict.Paths
	MCISRunner          SourceMCISRunner
}

func BuildSourceEntriesWithConfig(options SourceEntryBuildOptions) (probecore.SourceBuildResult, error) {
	limit := SourceIPLimit(options.Source, options.DefaultIPLimit)
	return probecore.BuildSourceEntries(probecore.SourceBuildOptions{
		Raw:                   options.Raw,
		Name:                  SourceName(options.Source),
		Mode:                  SourceIPMode(options.Source),
		Limit:                 limit,
		Resolver:              options.Resolver,
		ColoFilter:            options.Source.ColoFilter,
		ColoMode:              options.Source.ColoFilterMode,
		ColoDictionaryPaths:   options.ColoDictionaryPaths,
		SourceColoFilterPhase: options.Config.SourceColoFilterPhase,
		MCISRunner: func(tokens []string, limit int) ([]string, []string, error) {
			if options.MCISRunner == nil {
				return nil, nil, nil
			}
			return options.MCISRunner(tokens, options.Source, options.Config, limit)
		},
	})
}
