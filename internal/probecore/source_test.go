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
