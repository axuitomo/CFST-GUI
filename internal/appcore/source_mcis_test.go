package appcore

import (
	"reflect"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
)

func TestMCISEngineConfigScalesBudgetToUniqueCandidateSpace(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()

	if got := BuildMCISEngineConfigForCIDRs(cfg, 20, []string{"198.51.100.10/32", "198.51.100.10/32"}).Budget; got != 1 {
		t.Fatalf("single unique candidate budget = %d, want 1", got)
	}
	if got := BuildMCISEngineConfigForCIDRs(cfg, 20, []string{"198.51.100.0/30"}).Budget; got != 4 {
		t.Fatalf("four candidate budget = %d, want 4", got)
	}
	// limit*3 = 60 is below the /24's 256 candidates, so the budget follows the
	// ratio instead of the fixed 256 floor that predates this change.
	if got := BuildMCISEngineConfigForCIDRs(cfg, 20, []string{"198.51.100.0/24"}).Budget; got != 60 {
		t.Fatalf("/24 budget = %d, want 60 (limit*3)", got)
	}
}

func TestMCISSamplingBudgetHasNoFixedUpperLimit(t *testing.T) {
	if got := mcisSamplingBudget(0, 1, nil); got != 3 {
		t.Fatalf("limit=1 budget = %d, want 3", got)
	}
	if got := mcisSamplingBudget(0, 100, nil); got != 300 {
		t.Fatalf("limit=100 budget = %d, want 300", got)
	}
	if got := mcisSamplingBudget(0, 3000, nil); got != 9000 {
		t.Fatalf("limit=3000 budget = %d, want 9000 without cap", got)
	}
	if got := mcisSamplingBudget(10000, 100, nil); got != 10000 {
		t.Fatalf("manual budget = %d, want 10000", got)
	}
}

func TestMCISSamplingBudgetDeduplicatesOverlappingCIDRs(t *testing.T) {
	// /24 (256) plus its contained /25 (128) still only exposes 256 unique
	// candidates; the ratio budget is 300, so the budget drops to 256.
	if got := mcisSamplingBudget(0, 100, []string{"198.51.100.0/24", "198.51.100.0/25"}); got != 256 {
		t.Fatalf("overlapping budget = %d, want 256 unique", got)
	}
	// duplicate single addresses collapse to one candidate.
	if got := mcisSamplingBudget(0, 100, []string{"198.51.100.10/32", "198.51.100.10/32", "198.51.100.10/32"}); got != 1 {
		t.Fatalf("duplicate single budget = %d, want 1", got)
	}
}

func TestMCISSamplingBudgetHandlesIPv6AndLargeSubnets(t *testing.T) {
	// All addresses in an IPv6 range narrower than /64 collapse to one candidate.
	if got := mcisSamplingBudget(0, 100, []string{"2001:db8::/126"}); got != 1 {
		t.Fatalf("IPv6 /126 budget = %d, want 1 /64 candidate", got)
	}
	// Huge IPv6/IPv4 ranges bail out immediately; the budget stays at the ratio.
	if got := mcisSamplingBudget(0, 100, []string{"2001:db8::/48"}); got != 300 {
		t.Fatalf("IPv6 /48 budget = %d, want 300 (ratio)", got)
	}
	if got := mcisSamplingBudget(0, 100, []string{"10.0.0.0/8"}); got != 300 {
		t.Fatalf("IPv4 /8 budget = %d, want 300 (ratio)", got)
	}
}

func TestBuildMCISEngineConfigKeepsCompatWithoutCIDRs(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	got := BuildMCISEngineConfig(cfg, 100)
	if got.Budget != 300 {
		t.Fatalf("compat budget = %d, want 300 (limit*3)", got.Budget)
	}
	if got.TopN != 100 {
		t.Fatalf("compat TopN = %d, want 100", got.TopN)
	}
}
func TestBuildMCISEngineConfigUsesManualOverrides(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	cfg.MCISBudget = 10000
	cfg.MCISConcurrency = 777
	got := BuildMCISEngineConfig(cfg, 100)
	if got.Budget != 10000 {
		t.Fatalf("manual budget = %d, want 10000", got.Budget)
	}
	if got.Concurrency != 777 {
		t.Fatalf("manual concurrency = %d, want 777", got.Concurrency)
	}
}

func TestSafeMCISConcurrencyMatchesTCP(t *testing.T) {
	for _, routines := range []int{1, 8, 200, 1000} {
		if got := safeMCISConcurrency(routines); got != routines {
			t.Fatalf("automatic concurrency = %d, want TCP concurrency %d", got, routines)
		}
	}
}

func TestBuildMCISProbeConfigUsesTraceURLAndTLSSetting(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	cfg.URL = "https://download.example.com/500m"
	cfg.TraceURL = "https://trace.example.com/cdn-cgi/trace"
	cfg.HostHeader = ""
	cfg.SNI = ""
	cfg.VerifyTLSCertificate = true

	probeCfg, warnings := BuildMCISProbeConfig(cfg)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if probeCfg.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = true, want false when certificate verification is enabled")
	}
	if probeCfg.HostHeader != "trace.example.com" || probeCfg.SNI != "trace.example.com" {
		t.Fatalf("HostHeader/SNI = %q/%q, want trace.example.com", probeCfg.HostHeader, probeCfg.SNI)
	}
	if probeCfg.Path != "/cdn-cgi/trace" {
		t.Fatalf("Path = %q, want /cdn-cgi/trace", probeCfg.Path)
	}

	cfg.VerifyTLSCertificate = false
	probeCfg, _ = BuildMCISProbeConfig(cfg)
	if !probeCfg.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = false, want true when certificate verification is disabled")
	}
}

func TestBuildMCISProbeConfigTargetPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		traceURL   string
		url        string
		hostHeader string
		sni        string
		wantHost   string
		wantSNI    string
	}{
		{
			name:     "trace URL before download URL",
			traceURL: "https://trace.example.com/cdn-cgi/trace",
			url:      "https://download.example.com/500m",
			wantHost: "trace.example.com",
			wantSNI:  "trace.example.com",
		},
		{
			name:     "download URL fallback",
			url:      "https://download.example.com/500m",
			wantHost: "download.example.com",
			wantSNI:  "download.example.com",
		},
		{
			name:       "explicit host and SNI",
			traceURL:   "https://trace.example.com/cdn-cgi/trace",
			url:        "https://download.example.com/500m",
			hostHeader: "host.example.net",
			sni:        "sni.example.net",
			wantHost:   "host.example.net",
			wantSNI:    "sni.example.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := probecore.DefaultProbeConfig()
			cfg.TraceURL = tt.traceURL
			cfg.URL = tt.url
			cfg.HostHeader = tt.hostHeader
			cfg.SNI = tt.sni

			probeCfg, warnings := BuildMCISProbeConfig(cfg)
			if len(warnings) != 0 {
				t.Fatalf("warnings = %v, want none", warnings)
			}
			if probeCfg.HostHeader != tt.wantHost || probeCfg.SNI != tt.wantSNI {
				t.Fatalf("HostHeader/SNI = %q/%q, want %q/%q", probeCfg.HostHeader, probeCfg.SNI, tt.wantHost, tt.wantSNI)
			}
		})
	}
}

func TestBuildMCISEngineConfigMapsColoFilters(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		mode      string
		wantAllow []string
		wantBlock []string
	}{
		{
			name:      "allow mixed case and separators",
			raw:       "sjc, HkG; nrt sjc",
			mode:      task.ColoFilterModeAllow,
			wantAllow: []string{"SJC", "HKG", "NRT"},
		},
		{
			name:      "deny",
			raw:       "sjc,hkg",
			mode:      task.ColoFilterModeDeny,
			wantBlock: []string{"SJC", "HKG"},
		},
		{
			name: "empty",
			mode: task.ColoFilterModeAllow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := probecore.DefaultProbeConfig()
			cfg.HttpingCFColo = tt.raw
			cfg.HttpingCFColoMode = tt.mode

			engineCfg := BuildMCISEngineConfig(cfg, 20)
			if !reflect.DeepEqual(engineCfg.ColoAllow, tt.wantAllow) {
				t.Fatalf("ColoAllow = %v, want %v", engineCfg.ColoAllow, tt.wantAllow)
			}
			if !reflect.DeepEqual(engineCfg.ColoBlock, tt.wantBlock) {
				t.Fatalf("ColoBlock = %v, want %v", engineCfg.ColoBlock, tt.wantBlock)
			}
		})
	}
}
