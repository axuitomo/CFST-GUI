package task

import (
	"net"
	"testing"
	"time"
)

func newGiveUpTestMeasurement(t *testing.T, minSpeed float64) *downloadMeasurement {
	t.Helper()
	config := DefaultConfig()
	config.MinSpeed = minSpeed
	config.DownloadWarmupDuration = time.Second
	config.DownloadSampleInterval = time.Second
	engine := NewEngine(config, Hooks{})
	return engine.newDownloadMeasurement(&net.IPAddr{IP: net.ParseIP("1.1.1.1")}, 1)
}

// 早退点必须落在「预热 + 两个采样窗口」之后，否则 TCP 慢启动爬坡会被误判成低速。
func TestDownloadLowSpeedGiveUpWaitsForWarmupAndTwoWindows(t *testing.T) {
	measurement := newGiveUpTestMeasurement(t, 15)
	below := 1.0 * 1024 * 1024
	above := 20.0 * 1024 * 1024

	if measurement.lowSpeedGiveUp(2*time.Second, false, below) {
		t.Fatalf("预热后仅一个窗口就早退，会误伤慢启动中的 IP")
	}
	if !measurement.lowSpeedGiveUp(3*time.Second, false, below) {
		t.Fatalf("预热 + 两个窗口后仍低速，应当早退")
	}
	if measurement.lowSpeedGiveUp(3*time.Second, false, above) {
		t.Fatalf("峰值高于下限时不应早退")
	}
	if measurement.lowSpeedGiveUp(3*time.Second, true, below) {
		t.Fatalf("传输已完成时不应早退")
	}
}

func TestDownloadLowSpeedGiveUpIgnoresDisabledThreshold(t *testing.T) {
	measurement := newGiveUpTestMeasurement(t, 0)
	if measurement.lowSpeedGiveUp(5*time.Second, false, 0) {
		t.Fatalf("下限为 0 时永不早退")
	}
}

// emitSample 是唯一注入点：确认它在下限判定成立时真的调用 giveUp（取消本次测量的 ctx）。
func TestDownloadEmitSampleTriggersGiveUp(t *testing.T) {
	measurement := newGiveUpTestMeasurement(t, 15)
	measurement.maxSampleSpeed = 1.0 * 1024 * 1024
	calls := 0
	measurement.giveUp = func() { calls++ }

	measurement.emitSample(3*time.Second, false)
	if calls != 1 {
		t.Fatalf("giveUp 调用次数 = %d, want 1", calls)
	}

	measurement.emitSample(4*time.Second, true)
	if calls != 1 {
		t.Fatalf("强制收尾采样不应再次触发 giveUp，调用次数 = %d, want 1", calls)
	}
}
