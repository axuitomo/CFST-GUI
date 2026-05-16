package probecore

import (
	"strings"
	"testing"
)

func TestBuildSourceEntriesExpandsCIDRAndPreservesSourcePorts(t *testing.T) {
	result, err := BuildSourceEntries(SourceBuildOptions{
		Raw:   "8.8.8.8:2053\n1.1.1.0/30",
		Name:  "Test",
		Mode:  "traverse",
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("BuildSourceEntries returned error: %v", err)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("entries = %#v, want 3 limited entries", result.Entries)
	}
	if result.SourcePorts["8.8.8.8"] != 2053 {
		t.Fatalf("source ports = %#v, want 8.8.8.8:2053", result.SourcePorts)
	}
	if strings.Join(result.Warnings, "\n") == "" || !strings.Contains(strings.Join(result.Warnings, "\n"), "达到 IP 上限") {
		t.Fatalf("warnings = %#v, want truncation warning", result.Warnings)
	}
}

func TestBuildSourceEntriesMCISDropsSourcePortsWithWarning(t *testing.T) {
	result, err := BuildSourceEntries(SourceBuildOptions{
		Raw:   "8.8.8.8:2053",
		Name:  "MICS",
		Mode:  "mcis",
		Limit: 10,
		MCISRunner: func(tokens []string, limit int) ([]string, []string, error) {
			return []string{"8.8.8.8"}, nil, nil
		},
	})
	if err != nil {
		t.Fatalf("BuildSourceEntries returned error: %v", err)
	}
	if len(result.SourcePorts) != 0 {
		t.Fatalf("source ports = %#v, want nil/empty for MICS", result.SourcePorts)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "暂不继承源端口") {
		t.Fatalf("warnings = %#v, want MICS port fallback warning", result.Warnings)
	}
}
