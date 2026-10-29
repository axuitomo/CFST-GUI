package sourceparse

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// benchFlatSource 模拟用户从测速结果里粘贴过来的平铺 IP 列表（每行一个 IP）。
// 这是阶段 0 最贵的一种输入形态：每行都要走一次正则匹配和 IP 归一化。
func benchFlatSource(lines int) string {
	var b strings.Builder
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&b, "104.%d.%d.%d\n", 16+i/65536%8, i/256%256, i%256)
	}
	return b.String()
}

var benchFlatRaw = benchFlatSource(65536)

func BenchmarkParseFlatIPs(b *testing.B) {
	b.SetBytes(int64(len(benchFlatRaw)))
	b.ReportAllocs()
	for b.Loop() {
		if got := Parse(benchFlatRaw, Options{Context: context.Background(), Limit: 1 << 20}); len(got.Valid) == 0 {
			b.Fatal("没有解析出候选")
		}
	}
}
