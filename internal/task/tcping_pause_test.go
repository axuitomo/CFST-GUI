package task

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestPingRunStopsImmediatelyAfterPauseCancel(t *testing.T) {
	oldPauseHook := ProbePauseHook
	oldCancelHook := ProbeCancelHook
	t.Cleanup(func() {
		ProbePauseHook = oldPauseHook
		ProbeCancelHook = oldCancelHook
	})

	ip := &net.IPAddr{IP: net.ParseIP("1.1.1.1")}
	var pauses atomic.Int32
	var probes atomic.Int32
	var canceled atomic.Bool
	pauseCh := make(chan struct{})
	resumeCh := make(chan struct{})

	ProbePauseHook = func(stage, pauseIP string) {
		if stage != "stage1_tcp" || pauseIP != ip.String() {
			return
		}
		if pauses.Add(1) == 2 {
			close(pauseCh)
			<-resumeCh
		}
	}
	ProbeCancelHook = func(stage, cancelIP string) bool {
		return stage == "stage1_tcp" && cancelIP == ip.String() && canceled.Load()
	}

	ping := &Ping{
		wg:      &sync.WaitGroup{},
		m:       &sync.Mutex{},
		ips:     []*net.IPAddr{ip},
		csv:     make(utils.PingDelaySet, 0),
		control: make(chan bool, 1),
		bar:     utils.NewBar(1, "可用:", ""),
		total:   1,
		tcpProbe: func(_ *net.IPAddr) (bool, time.Duration) {
			probes.Add(1)
			return true, time.Millisecond
		},
	}

	done := make(chan struct{})
	go func() {
		_ = ping.Run()
		close(done)
	}()

	select {
	case <-pauseCh:
	case <-time.After(time.Second):
		t.Fatal("pause hook did not block stage1 tcp run")
	}

	canceled.Store(true)
	close(resumeCh)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ping run did not stop after cancel")
	}

	if got := probes.Load(); got != 0 {
		t.Fatalf("tcp probes = %d, want 0 after cancel while paused", got)
	}
}
