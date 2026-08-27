package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/mcis/bandit"
	"github.com/axuitomo/CFST-GUI/internal/mcis/probe"
)

func TestPassColoFilter(t *testing.T) {
	tests := []struct {
		name  string
		allow []string
		block []string
		colo  string
		want  bool
	}{
		{name: "no filter accepts empty", colo: "", want: true},
		{name: "allow is case insensitive", allow: []string{"SJC"}, colo: " sjc ", want: true},
		{name: "allow rejects another colo", allow: []string{"SJC"}, colo: "HKG", want: false},
		{name: "allow rejects empty colo", allow: []string{"SJC"}, colo: "", want: false},
		{name: "block is case insensitive", block: []string{"sjc"}, colo: "SJC", want: false},
		{name: "block accepts another colo", block: []string{"SJC"}, colo: "HKG", want: true},
		{name: "block accepts empty colo", block: []string{"SJC"}, colo: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &Engine{cfg: Config{ColoAllow: tt.allow, ColoBlock: tt.block}}
			if got := engine.passColoFilter(tt.colo); got != tt.want {
				t.Fatalf("passColoFilter(%q) = %v, want %v", tt.colo, got, tt.want)
			}
		})
	}
}

func TestProcessOneResultRejectsFailedCandidateAfterBanditUpdate(t *testing.T) {
	prefix := netip.MustParsePrefix("198.51.100.0/24")
	engine := &Engine{
		cfg:  Config{},
		tree: bandit.NewArmTree([]netip.Prefix{prefix}, bandit.DefaultTreeConfig()),
		topN: NewTopNCollector(5),
	}

	engine.processOneResult(probeDone{
		task: probeTask{prefix: prefix, ip: netip.MustParseAddr("198.51.100.10")},
		result: probe.Result{
			OK:      false,
			Error:   "tls: certificate verification failed",
			TotalMS: 50,
		},
	}, 3000)

	stats := engine.tree.GetNode(prefix).Stats()
	if stats.Samples != 1 || stats.Failures != 1 {
		t.Fatalf("arm stats = samples %d, failures %d; want 1, 1", stats.Samples, stats.Failures)
	}
	if engine.topN.Len() != 0 {
		t.Fatalf("TopN length = %d, want 0 for failed probe", engine.topN.Len())
	}
}

func TestProcessOneResultAcceptsSuccessfulNormalizedColo(t *testing.T) {
	prefix := netip.MustParsePrefix("198.51.100.0/24")
	engine := &Engine{
		cfg:  Config{ColoAllow: []string{"sjc"}},
		tree: bandit.NewArmTree([]netip.Prefix{prefix}, bandit.DefaultTreeConfig()),
		topN: NewTopNCollector(5),
	}

	engine.processOneResult(probeDone{
		task: probeTask{prefix: prefix, ip: netip.MustParseAddr("198.51.100.10")},
		result: probe.Result{
			OK:      true,
			Status:  http.StatusOK,
			TotalMS: 25,
			Trace:   map[string]string{"colo": "SJC"},
		},
	}, 3000)

	results := engine.topN.Snapshot()
	if len(results) != 1 {
		t.Fatalf("TopN length = %d, want 1", len(results))
	}
	if !results[0].OK || results[0].Trace["colo"] != "SJC" {
		t.Fatalf("TopN result = %#v, want successful SJC candidate", results[0])
	}
}

func TestGetExploitationPrefixesUsesDeterministicWeightedOrder(t *testing.T) {
	prefixA := netip.MustParsePrefix("198.51.100.0/24")
	prefixB := netip.MustParsePrefix("198.51.101.0/24")
	prefixC := netip.MustParsePrefix("198.51.102.0/24")
	prefixExcluded := netip.MustParsePrefix("198.51.103.0/24")
	engine := &Engine{topN: NewTopNCollector(10)}
	for _, result := range []TopResult{
		{IP: netip.MustParseAddr("198.51.101.10"), Prefix: prefixB, ScoreMS: 100},
		{IP: netip.MustParseAddr("198.51.100.10"), Prefix: prefixA, ScoreMS: 100},
		{IP: netip.MustParseAddr("198.51.102.10"), Prefix: prefixC, ScoreMS: 140},
		{IP: netip.MustParseAddr("198.51.103.10"), Prefix: prefixExcluded, ScoreMS: 200},
	} {
		engine.topN.Consider(result)
	}

	want := []netip.Prefix{prefixA, prefixA, prefixA, prefixB, prefixB, prefixB, prefixC}
	for i := 0; i < 20; i++ {
		if got := engine.getExploitationPrefixes(); !reflect.DeepEqual(got, want) {
			t.Fatalf("getExploitationPrefixes() = %v, want %v", got, want)
		}
	}
}

func TestEngineRunRejectsRootsAboveNodeLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTreeNodes = 2
	engine := New(cfg, probe.Config{})

	_, err := engine.Run(context.Background(), Request{
		CIDRs: []string{"198.51.100.1/32", "198.51.100.2/32", "198.51.100.3/32"},
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds max tree nodes") {
		t.Fatalf("Run() error = %v, want root node limit error", err)
	}
}

func TestEngineRunCanBeReusedSequentially(t *testing.T) {
	t.Setenv("CFST_HTTP_PROTOCOL", "h1")
	var requestCount atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		_, _ = io.WriteString(w, "colo=SJC\n")
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.Budget = 1
	cfg.TopN = 1
	cfg.Concurrency = 1
	cfg.Heads = 1
	cfg.Beam = 1
	cfg.Seed = 42
	probeCfg := probe.Config{
		Timeout:            2 * time.Second,
		SNI:                "trace.example.com",
		HostHeader:         "trace.example.com",
		DialAddress:        strings.TrimPrefix(server.URL, "https://"),
		InsecureSkipVerify: true,
		Rounds:             1,
		SkipFirst:          0,
	}
	request := Request{CIDRs: []string{"198.51.100.10/32"}, Probe: probeCfg}

	reused := New(cfg, probeCfg)
	first, err := reused.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	sentinel := netip.MustParseAddr("203.0.113.1")
	reused.seenIPs.Store(ipToKey(sentinel), struct{}{})

	second, err := reused.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if reused.submitted.Load() != 1 || reused.completed.Load() != 1 {
		t.Fatalf("second Run() submitted/completed = %d/%d, want 1/1", reused.submitted.Load(), reused.completed.Load())
	}
	if _, exists := reused.seenIPs.Load(ipToKey(sentinel)); exists {
		t.Fatal("second Run() retained a stale seen IP")
	}

	fresh, err := New(cfg, probeCfg).Run(context.Background(), request)
	if err != nil {
		t.Fatalf("fresh Run() error = %v", err)
	}
	for name, response := range map[string]Response{"first": first, "second": second, "fresh": fresh} {
		if len(response.Top) != 1 {
			t.Fatalf("%s Run() Top length = %d, want 1", name, len(response.Top))
		}
		candidate := response.Top[0]
		if !candidate.OK || candidate.IP != netip.MustParseAddr("198.51.100.10") || candidate.Trace["colo"] != "SJC" {
			t.Fatalf("%s Run() candidate = %#v, want successful SJC fixture IP", name, candidate)
		}
	}
	if got := requestCount.Load(); got != 3 {
		t.Fatalf("request count = %d, want 3", got)
	}
}
