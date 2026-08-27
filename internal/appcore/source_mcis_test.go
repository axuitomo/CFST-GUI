package appcore

import (
	"reflect"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
)

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
