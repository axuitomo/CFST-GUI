package probecore

import (
	"context"
	"strings"
	"testing"
)

// benchSourceCIDRText 模拟上游风格的输入源：二十几行大段 CIDR。
var benchSourceCIDRText = strings.Repeat(strings.Join([]string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/12",
}, "\n")+"\n", 2)

var benchPoolEntries = mustEnumeratePool(65536)

func mustEnumeratePool(limit int) []string {
	entries, _, err := buildTraverseEntriesContext(context.Background(), []string{"104.16.0.0/12"}, limit)
	if err != nil || len(entries) != limit {
		panic("CIDR 展开失败")
	}
	return entries
}

func BenchmarkEnumerateCIDR(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if entries, _, err := buildTraverseEntriesContext(context.Background(), []string{"104.16.0.0/12"}, 65536); err != nil || len(entries) != 65536 {
			b.Fatalf("entries=%d err=%v", len(entries), err)
		}
	}
}

func BenchmarkSummarizeEntries(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if got := SummarizeEntries(benchPoolEntries); got.ValidCount != len(benchPoolEntries) {
			b.Fatalf("valid=%d", got.ValidCount)
		}
	}
}

func BenchmarkBuildSourceEntries(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		res, err := BuildSourceEntries(SourceBuildOptions{
			Context: context.Background(),
			Raw:     benchSourceCIDRText,
			Limit:   65536,
		})
		if err != nil || len(res.Entries) == 0 {
			b.Fatalf("entries=%d err=%v", len(res.Entries), err)
		}
	}
}
