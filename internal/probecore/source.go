package probecore

import (
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/sourceparse"
)

func SummarizeSource(raw string, resolver sourceparse.Resolver) SourceSummary {
	return SummarizeParsedSource(sourceparse.Parse(raw, sourceparse.Options{Resolver: resolver}))
}

func SummarizeParsedSource(parsed sourceparse.Result) SourceSummary {
	summary := SummarizeEntries(parsed.Valid)
	summary.CandidateCount = parsed.CandidateCount
	summary.Invalid = append([]string(nil), parsed.Invalid...)
	summary.InvalidCount = len(summary.Invalid)
	summary.RawLineCount = parsed.RawLineCount
	return summary
}

// SummarizeEntries 汇总一批已经展开的候选（只剩 IP），只做网络键去重，
// 不再重新扫描文本：阶段 0 拼好的候选池可直接复用，省掉一次全量解析。
func SummarizeEntries(entries []string) SourceSummary {
	summary := SourceSummary{CandidateCount: len(entries), RawLineCount: len(entries)}
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		key := candidateNetworkKey(entry)
		if _, exists := seen[key]; exists {
			summary.Duplicates = append(summary.Duplicates, entry)
			continue
		}
		seen[key] = struct{}{}
		summary.Valid = append(summary.Valid, entry)
	}
	summary.ValidCount = len(summary.Valid)
	summary.DuplicateCount = len(summary.Duplicates)
	summary.UniqueCount = summary.ValidCount
	return summary
}

// CapSourceSummary 把跨输入源的聚合候选池收敛到绝对上限，避免多源叠加把候选池
// 撑成无界内存和无法完成的测速时长。返回是否发生了截断。
func CapSourceSummary(summary SourceSummary, ceiling int) (SourceSummary, bool) {
	if ceiling <= 0 {
		ceiling = MaxSourcePoolLimit
	}
	if len(summary.Valid) <= ceiling {
		return summary, false
	}
	summary.Valid = append([]string(nil), summary.Valid[:ceiling]...)
	summary.ValidCount = len(summary.Valid)
	summary.UniqueCount = summary.ValidCount
	return summary, true
}
