package probecore

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
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

func TestBuildSourceEntriesPureColoPrecheckDoesNotRequireDictionary(t *testing.T) {
	result, err := BuildSourceEntries(SourceBuildOptions{
		Raw:                 "8.8.8.8\n1.1.1.1",
		Name:                "Pure COLO",
		Mode:                "traverse",
		Limit:               10,
		ColoFilter:          "HKG,NRT",
		ColoDictionaryPaths: colodict.Paths{Colo: filepath.Join(t.TempDir(), "missing-cloudflare-colos.csv")},
	})
	if err != nil {
		t.Fatalf("BuildSourceEntries returned error: %v", err)
	}
	if got, want := len(result.Entries), 2; got != want {
		t.Fatalf("entries = %#v, want %d original candidates", result.Entries, want)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "已保留原始候选") {
		t.Fatalf("warnings = %#v, want dictionary fallback warning", result.Warnings)
	}
}

func TestBuildSourceEntriesCountryPrecheckStillRequiresDictionary(t *testing.T) {
	_, err := BuildSourceEntries(SourceBuildOptions{
		Raw:                 "8.8.8.8",
		Name:                "Country COLO",
		Mode:                "traverse",
		Limit:               10,
		ColoFilter:          "JP",
		ColoDictionaryPaths: colodict.Paths{Colo: filepath.Join(t.TempDir(), "missing-cloudflare-colos.csv")},
	})
	if err == nil || !strings.Contains(err.Error(), "COLO 文件不存在") {
		t.Fatalf("err = %v, want missing COLO dictionary error", err)
	}
}
