package appcore

import (
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/taskstate"
)

// TestShouldLogProgress 覆盖进度落盘的十分位闸门：每阶段约 10 条，且首条必写。
// 进度回调已被 100ms 节流，落盘判断必须按十分位档位前进而不是精确整除。
func TestShouldLogProgress(t *testing.T) {
	runtime := taskstate.NewProbeRuntime()
	if runtime.ShouldLogProgress("stage1_tcp", 5, 0) {
		t.Fatal("unknown total should not log")
	}
	if !runtime.ShouldLogProgress("stage1_tcp", 0, 4096) {
		t.Fatal("first progress of a stage should log")
	}
	if runtime.ShouldLogProgress("stage1_tcp", 17, 4096) {
		t.Fatal("same decile should not log twice")
	}
	logged := 0
	for processed := 1; processed <= 4096; processed++ {
		if runtime.ShouldLogProgress("stage1_tcp", processed, 4096) {
			logged++
		}
	}
	if logged != 10 {
		t.Fatalf("decile logs = %d, want 10", logged)
	}
	if !runtime.ShouldLogProgress("stage2_trace", 0, 100) {
		t.Fatal("stage switch should log again")
	}
	if !runtime.ShouldLogProgress("stage3_get", 1, 3) {
		t.Fatal("tiny total should log the first decile")
	}
	runtime.ConfigureProgressThrottle(0)
	if !runtime.ShouldLogProgress("stage3_get", 1, 3) {
		t.Fatal("new run should log again")
	}
}
