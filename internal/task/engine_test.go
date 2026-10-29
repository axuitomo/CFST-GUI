package task

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestDefaultConfigPreservesCLIOutputAndFilterDefaults(t *testing.T) {
	config := DefaultConfig()
	if config.InputMaxDelay != utils.DefaultMaxDelay || config.InputMinDelay != utils.DefaultMinDelay {
		t.Fatalf("delay defaults = %v..%v, want %v..%v", config.InputMinDelay, config.InputMaxDelay, utils.DefaultMinDelay, utils.DefaultMaxDelay)
	}
	if config.InputMaxLossRate != utils.DefaultMaxLossRate {
		t.Fatalf("loss default = %v, want %v", config.InputMaxLossRate, utils.DefaultMaxLossRate)
	}
	if config.PrintNum != utils.DefaultPrintNum || config.Output != utils.DefaultOutput || config.OutputCSVEncoding != utils.CSVEncodingUTF8 {
		t.Fatalf("output defaults = print %d path %q encoding %q", config.PrintNum, config.Output, config.OutputCSVEncoding)
	}
}

func TestEngineCopiesMutableConfig(t *testing.T) {
	config := DefaultConfig()
	config.IPText = "1.1.1.1"
	config.HttpingCFColos = []string{"HKG"}
	config.SourceColoFilters = SourceColoFilterMap{
		"1.1.1.1": NewSourceColoFilter("HKG"),
	}
	engine := NewEngine(config, Hooks{})

	config.HttpingCFColos[0] = "SJC"
	delete(config.SourceColoFilters, "1.1.1.1")
	snapshot := engine.Config()
	if snapshot.HttpingCFColos[0] != "HKG" {
		t.Fatalf("HttpingCFColos = %v, want copied HKG", snapshot.HttpingCFColos)
	}
	if _, ok := snapshot.SourceColoFilters["1.1.1.1"]; !ok {
		t.Fatal("SourceColoFilters was not copied")
	}
}

func TestEnginesUseIndependentInputsAndHooksConcurrently(t *testing.T) {
	t.Parallel()
	configs := []Config{DefaultConfig(), DefaultConfig()}
	configs[0].IPText = "1.1.1.1"
	configs[0].TCPPort = 2053
	configs[1].IPText = "2606:4700:4700::1111"
	configs[1].TCPPort = 8443

	var wg sync.WaitGroup
	for index := range configs {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			engine := NewEngine(configs[index], Hooks{ProbeContext: func() context.Context { return ctx }})
			ping, err := engine.NewPing()
			if err != nil {
				t.Errorf("NewPing(%d): %v", index, err)
				return
			}
			if len(ping.ips) != 1 {
				t.Errorf("engine %d ips = %d, want 1", index, len(ping.ips))
			}
			if ping.engine.config.TCPPort != configs[index].TCPPort {
				t.Errorf("engine %d port = %d, want %d", index, ping.engine.config.TCPPort, configs[index].TCPPort)
			}
		}()
	}
	wg.Wait()
}

func TestEngineWaitUsesOwnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	engine := NewEngine(DefaultConfig(), Hooks{ProbeContext: func() context.Context { return ctx }})
	cancel()
	started := time.Now()
	engine.waitProbeDelay("test", "1.1.1.1", time.Second)
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("wait did not stop when the engine context was canceled")
	}
}

func TestEnginesUseIndependentColoDictionaryCaches(t *testing.T) {
	t.Parallel()
	paths := []string{
		filepath.Join(t.TempDir(), "colo-a.csv"),
		filepath.Join(t.TempDir(), "colo-b.csv"),
	}
	contents := []string{
		"ip_prefix,colo,country,region,city\n198.51.100.0/24,HKG,HK,,Hong Kong\n",
		"ip_prefix,colo,country,region,city\n198.51.100.0/24,NRT,JP,,Tokyo\n",
	}
	for index := range paths {
		if err := os.WriteFile(paths[index], []byte(contents[index]), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	engines := make([]*Engine, len(paths))
	for index := range paths {
		config := DefaultConfig()
		config.ColoDictionaryPath = paths[index]
		engines[index] = NewEngine(config, Hooks{})
	}

	want := []string{"HKG", "NRT"}
	var wg sync.WaitGroup
	for index := range engines {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				if got := engines[index].lookupColoFromDictionary(&net.IPAddr{IP: net.ParseIP("198.51.100.10")}); got != want[index] {
					t.Errorf("engine %d colo = %q, want %q", index, got, want[index])
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestColoDictionaryCacheRechecksOnAWindowNotPerLookup(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "colo.csv")
	// The replacement is larger, which is what a stat-based check can see: a
	// same-size rewrite inside the same timestamp tick is invisible to size and
	// modtime, so it must not be what this test stands on.
	writeColo := func(document string) {
		if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeColo("ip_prefix,colo,country,region,city\n198.51.100.0/24,HKG,HK,,Hong Kong\n")

	config := DefaultConfig()
	config.ColoDictionaryPath = path
	engine := NewEngine(config, Hooks{})
	ip := &net.IPAddr{IP: net.ParseIP("198.51.100.10")}

	if got := engine.lookupColoFromDictionary(ip); got != "HKG" {
		t.Fatalf("colo = %q, want HKG", got)
	}

	writeColo("ip_prefix,colo,country,region,city\n198.51.100.0/24,NRT,JP,,Tokyo\n198.51.101.0/24,NRT,JP,,Tokyo\n")
	// Inside the window the stat is skipped on purpose: the lookup runs once per
	// traced IP, and one stat per IP is exactly the cost this avoids.
	if got := engine.lookupColoFromDictionary(ip); got != "HKG" {
		t.Fatalf("colo = %q, want the cached HKG inside the recheck window", got)
	}

	engine.coloCache.Lock()
	engine.coloCache.checkedAt = time.Now().Add(-coloDictionaryRecheckInterval - time.Second)
	engine.coloCache.Unlock()
	if got := engine.lookupColoFromDictionary(ip); got != "NRT" {
		t.Fatalf("colo = %q, want the replaced NRT after the recheck window", got)
	}
}

// 阶段 1 的 TCP 预算常被调小来筛快 IP，TLS 握手不能跟着缩。
func TestTLSHandshakeTimeoutKeepsTLSBudget(t *testing.T) {
	cases := []struct {
		connect time.Duration
		want    time.Duration
	}{
		{connect: 0, want: defaultTLSHandshakeTimeout},
		{connect: time.Second, want: defaultTLSHandshakeTimeout},
		{connect: 2 * time.Second, want: 4 * time.Second},
		{connect: 5 * time.Second, want: 10 * time.Second},
	}
	for _, testCase := range cases {
		if got := tlsHandshakeTimeout(testCase.connect); got != testCase.want {
			t.Fatalf("tlsHandshakeTimeout(%v) = %v, want %v", testCase.connect, got, testCase.want)
		}
	}
}
