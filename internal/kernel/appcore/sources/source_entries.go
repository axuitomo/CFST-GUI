package sources

import (
	"context"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/colodict"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/sourceparse"
)

type SourceMCISRunner func(tokens []string, source contracts.Source, cfg probecore.ProbeConfig, limit int) ([]probecore.MCISCandidate, []string, error)

type SourceEntryBuildOptions struct {
	Context             context.Context
	Raw                 string
	Source              contracts.Source
	Config              probecore.ProbeConfig
	DefaultIPLimit      int
	Resolver            sourceparse.Resolver
	ColoDictionaryPaths colodict.Paths
	MCISRunner          SourceMCISRunner
}

func BuildSourceEntriesWithConfig(options SourceEntryBuildOptions) (probecore.SourceBuildResult, error) {
	limit := SourceIPLimit(options.Source, options.DefaultIPLimit)
	return probecore.BuildSourceEntries(probecore.SourceBuildOptions{
		Context:               options.Context,
		Raw:                   options.Raw,
		Name:                  SourceName(options.Source),
		Mode:                  SourceIPMode(options.Source),
		Limit:                 limit,
		MaxPoolSize:           options.Config.SourcePoolLimit,
		Resolver:              options.Resolver,
		ColoFilter:            options.Source.ColoFilter,
		ColoMode:              options.Source.ColoFilterMode,
		ColoDictionaryPaths:   options.ColoDictionaryPaths,
		SourceColoFilterPhase: options.Config.SourceColoFilterPhase,
		MCISRunner: func(tokens []string, limit int) ([]probecore.MCISCandidate, []string, error) {
			if options.MCISRunner == nil {
				return nil, nil, nil
			}
			return options.MCISRunner(tokens, options.Source, options.Config, limit)
		},
	})
}
