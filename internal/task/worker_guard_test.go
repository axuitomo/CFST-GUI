package task

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

// reasonCounter 收集 debug 事件里的 reason，用来断言 worker panic 被兜住并记了日志。
type reasonCounter struct {
	mu      sync.Mutex
	reasons map[string]int
}

func newReasonCounter() *reasonCounter {
	return &reasonCounter{reasons: map[string]int{}}
}

func (c *reasonCounter) hook(event string, payload map[string]any) {
	if event != "probe.failed" {
		return
	}
	reason, _ := payload["reason"].(string)
	c.mu.Lock()
	c.reasons[reason]++
	c.mu.Unlock()
}

func (c *reasonCounter) count(reason string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reasons[reason]
}

// 阶段 1 的 worker panic 必须只淘汰那一个 IP：goroutine 里的 panic 不会被父 goroutine 的
// recover 捕获，漏掉就是整个进程闪退；同时信号量必须释放，否则剩下的 IP 会永久阻塞。
func TestPingWorkerPanicIsolatesIP(t *testing.T) {
	config := DefaultConfig()
	config.IPText = "10.10.0.1,10.10.0.2,10.10.0.3"
	config.PingTimes = 2
	reasons := newReasonCounter()
	engine := NewEngine(config, Hooks{DebugEvent: reasons.hook})

	ping, err := engine.NewPing()
	if err != nil {
		t.Fatalf("NewPing: %v", err)
	}
	ping.tcpProbe = func(ip *net.IPAddr) (bool, time.Duration) {
		if ip.String() == "10.10.0.2" {
			panic("tcp worker boom")
		}
		return true, time.Millisecond
	}

	done := make(chan utils.PingDelaySet, 1)
	go func() { done <- ping.Run() }()

	var set utils.PingDelaySet
	select {
	case set = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Ping.Run 未返回：worker panic 后信号量没有释放")
	}

	if len(set) != 2 {
		t.Fatalf("结果数量 = %d, want 2", len(set))
	}
	for _, item := range set {
		if item.IP.String() == "10.10.0.2" {
			t.Fatal("panic 的 IP 混进了结果")
		}
	}
	if got := reasons.count("tcp_worker_panic"); got != 1 {
		t.Fatalf("tcp_worker_panic 事件 = %d, want 1（实际 reason: %v）", got, reasons.reasons)
	}
}

// 阶段 3 同理：下载 worker 的 panic 只淘汰该 IP，其余 IP 照常测完。
func TestDownloadWorkerPanicIsolatesIP(t *testing.T) {
	config := DefaultConfig()
	config.Disable = false
	config.MinSpeed = 0
	config.DownloadRoutines = 2
	config.TestCount = 3
	reasons := newReasonCounter()
	engine := newEngineWithDownloadProbes(config, Hooks{DebugEvent: reasons.hook}, nil,
		func(ip *net.IPAddr) downloadResult {
			if ip.String() == "1.1.1.2" {
				panic("download worker boom")
			}
			return downloadResult{speed: 2 << 20, maxSpeed: 2 << 20, validMeasurement: true, reason: "download_measured"}
		})

	var result utils.DownloadSpeedSet
	done := make(chan struct{})
	go func() {
		defer close(done)
		result = engine.TestDownloadSpeed(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2", "1.1.1.3"))
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("TestDownloadSpeed 未返回：worker panic 后信号量没有释放")
	}

	if len(result) != 2 {
		t.Fatalf("结果数量 = %d, want 2", len(result))
	}
	for _, item := range result {
		if item.IP.String() == "1.1.1.2" {
			t.Fatal("panic 的 IP 混进了结果")
		}
	}
	if got := reasons.count("download_worker_panic"); got != 1 {
		t.Fatalf("download_worker_panic 事件 = %d, want 1（实际 reason: %v）", got, reasons.reasons)
	}
}
