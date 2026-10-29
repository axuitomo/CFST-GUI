package appcore

import "testing"

// TestShouldLogProgress 覆盖进度落盘的十分位闸门：每阶段约 10 条，且首尾必写。
func TestShouldLogProgress(t *testing.T) {
	cases := []struct {
		name      string
		processed int
		total     int
		want      bool
	}{
		{name: "unknown total", processed: 5, total: 0, want: false},
		{name: "decile boundary", processed: 10, total: 100, want: true},
		{name: "inside decile", processed: 15, total: 100, want: false},
		{name: "final step", processed: 100, total: 100, want: true},
		{name: "tiny total logs each step", processed: 2, total: 3, want: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := shouldLogProgress(testCase.processed, testCase.total); got != testCase.want {
				t.Fatalf("shouldLogProgress(%d, %d) = %v, want %v", testCase.processed, testCase.total, got, testCase.want)
			}
		})
	}
}
