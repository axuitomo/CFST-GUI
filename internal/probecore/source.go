package probecore

import "github.com/axuitomo/CFST-GUI/internal/sourceparse"

func SummarizeSource(raw string, resolver sourceparse.Resolver) SourceSummary {
	return SummarizeParsedSource(sourceparse.Parse(raw, sourceparse.Options{Resolver: resolver}))
}

func SummarizeParsedSource(parsed sourceparse.Result) SourceSummary {
	summary := SourceSummary{
		CandidateCount: parsed.CandidateCount,
		Invalid:        append([]string(nil), parsed.Invalid...),
		RawLineCount:   parsed.RawLineCount,
	}
	seen := map[string]struct{}{}

	for _, normalized := range parsed.Valid {
		if _, exists := seen[normalized]; exists {
			summary.Duplicates = append(summary.Duplicates, normalized)
			continue
		}
		seen[normalized] = struct{}{}
		summary.Valid = append(summary.Valid, normalized)
	}

	summary.ValidCount = len(summary.Valid)
	summary.InvalidCount = len(summary.Invalid)
	summary.DuplicateCount = len(summary.Duplicates)
	summary.UniqueCount = summary.ValidCount
	return summary
}
