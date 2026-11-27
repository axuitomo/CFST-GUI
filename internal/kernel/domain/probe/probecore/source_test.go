package probecore

import "testing"

func TestSummarizeSourceDedupesAndCountsInvalidEntries(t *testing.T) {
	summary := SummarizeSource("1.1.1.1\n1.1.1.1\nnot-an-ip", nil)

	if summary.RawLineCount != 3 {
		t.Fatalf("RawLineCount = %d, want 3", summary.RawLineCount)
	}
	if summary.ValidCount != 1 || summary.DuplicateCount != 1 || summary.InvalidCount != 1 {
		t.Fatalf("summary counts = %#v, want 1 valid, 1 duplicate, 1 invalid", summary)
	}
	if len(summary.Valid) != 1 || summary.Valid[0] != "1.1.1.1" {
		t.Fatalf("valid = %#v, want deduped 1.1.1.1", summary.Valid)
	}
}

func TestSummarizeEntriesMatchesParsedCandidates(t *testing.T) {
	parsed := SummarizeSource("104.16.0.1\n104.16.0.1\n104.16.0.2\n1.1.1.0/30\n2001:db8:1:2::9\n2001:db8:1:2::a\n", nil)

	// 阶段 0 之后候选已经是 IP：再次汇总只做网络键去重，结果必须与按文本解析一致。
	direct := SummarizeEntries(parsed.Valid)
	if len(direct.Valid) != len(parsed.Valid) {
		t.Fatalf("valid = %#v, want %#v", direct.Valid, parsed.Valid)
	}
	for i := range direct.Valid {
		if direct.Valid[i] != parsed.Valid[i] {
			t.Fatalf("valid[%d] = %q, want %q", i, direct.Valid[i], parsed.Valid[i])
		}
	}
	if direct.ValidCount != len(direct.Valid) || direct.UniqueCount != direct.ValidCount {
		t.Fatalf("counts = %#v, want ValidCount/UniqueCount to match Valid", direct)
	}
	if direct.DuplicateCount != 0 {
		t.Fatalf("DuplicateCount = %d, want 0 for already deduped candidates", direct.DuplicateCount)
	}
	if direct.InvalidCount != 0 || len(direct.Invalid) != 0 {
		t.Fatalf("invalid = %#v, want none", direct.Invalid)
	}
}

func TestSummarizeEntriesKeepsFirstAddressPer64(t *testing.T) {
	summary := SummarizeEntries([]string{"2001:db8::1", "2001:db8::ffff", "2001:db8:0:1::1"})
	if summary.ValidCount != 2 || summary.DuplicateCount != 1 {
		t.Fatalf("summary counts = %#v, want two /64 networks", summary)
	}
	if summary.Valid[0] != "2001:db8::1" || summary.Valid[1] != "2001:db8:0:1::1" {
		t.Fatalf("valid = %#v, want first address from each /64", summary.Valid)
	}
}

func TestSummarizeSourceDeduplicatesIPv6By64(t *testing.T) {
	summary := SummarizeSource("2001:db8::1\n2001:db8::ffff\n2001:db8:0:1::1", nil)
	if summary.ValidCount != 2 || summary.DuplicateCount != 1 {
		t.Fatalf("summary counts = %#v, want two /64 networks", summary)
	}
	if summary.Valid[0] != "2001:db8::1" || summary.Valid[1] != "2001:db8:0:1::1" {
		t.Fatalf("valid = %#v, want first address from each /64", summary.Valid)
	}
}

func TestCapSourceSummaryTruncatesAggregatePool(t *testing.T) {
	summary := SummarizeSource("1.1.1.1\n2.2.2.2\n3.3.3.3", nil)

	capped, truncated := CapSourceSummary(summary, 2)
	if !truncated {
		t.Fatal("truncated = false, want true when the pool exceeds the ceiling")
	}
	if capped.ValidCount != 2 || capped.UniqueCount != 2 || len(capped.Valid) != 2 {
		t.Fatalf("capped = %#v, want two kept candidates", capped)
	}
	if summary.ValidCount != 3 {
		t.Fatalf("input summary was mutated: %#v", summary)
	}

	if exact, truncated := CapSourceSummary(summary, 3); truncated || exact.ValidCount != 3 {
		t.Fatalf("exact ceiling = %#v truncated=%v, want untouched", exact, truncated)
	}
	if fallback, truncated := CapSourceSummary(summary, 0); truncated || fallback.ValidCount != 3 {
		t.Fatalf("zero ceiling = %#v truncated=%v, want MaxSourcePoolLimit fallback", fallback, truncated)
	}
}
