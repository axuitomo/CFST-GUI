package appcore

import (
	"errors"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func TestPrepareSourcesStage2BuildsPassAnySourceColoFilters(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	cfg.SourceColoFilterPhase = probecore.SourceColoFilterPhaseStage2

	result := PrepareSources(PrepareSourcesOptions{
		Config: cfg,
		ProcessSource: func(source Source) (SourceProcessResult, error) {
			switch strings.TrimSpace(SourceName(source)) {
			case "sjc":
				return SourceProcessResult{
					Entries:     []string{"104.16.0.1", "104.20.0.1"},
					ColoFilter:  "SJC",
					ColoMode:    "allow",
					SourcePorts: map[string]int{"104.16.0.1": 443},
					Status:      SourceStatus{ID: source.ID, StatusText: "ok-sjc"},
					Warnings:    []string{"第二阶段起效"},
				}, nil
			case "lax":
				return SourceProcessResult{
					Entries: []string{"104.16.0.1"},
					Status:  SourceStatus{ID: source.ID, StatusText: "ok-lax"},
					Warnings: []string{
						"第二阶段起效",
					},
					ColoFilter: "LAX",
					ColoMode:   "allow",
				}, nil
			default:
				return SourceProcessResult{
					Entries: []string{"104.20.0.1"},
					Status:  SourceStatus{ID: source.ID, StatusText: "ok-any"},
				}, nil
			}
		},
		Sources: []Source{
			{Enabled: true, ID: "1", Name: "sjc"},
			{Enabled: true, ID: "2", Name: "lax"},
			{Enabled: true, ID: "3", Name: "unrestricted"},
		},
	})

	if result.SourceColoFilters == nil {
		t.Fatal("SourceColoFilters = nil, want stage2 filter map")
	}
	filter := result.SourceColoFilters["104.16.0.1"]
	if filter.Unrestricted || len(filter.Allowed) != 2 {
		t.Fatalf("filter for duplicate allowlisted IP = %#v, want SJC/LAX pass-any", filter)
	}
	if _, ok := filter.Allowed["SJC"]; !ok {
		t.Fatalf("filter for 104.16.0.1 = %#v, missing SJC", filter)
	}
	if _, ok := filter.Allowed["LAX"]; !ok {
		t.Fatalf("filter for 104.16.0.1 = %#v, missing LAX", filter)
	}
	if filter := result.SourceColoFilters["104.20.0.1"]; !filter.Unrestricted {
		t.Fatalf("filter for unrestricted duplicate IP = %#v, want unrestricted", filter)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "第二阶段起效" {
		t.Fatalf("Warnings = %#v, want deduped stage2 warning", result.Warnings)
	}
}

func TestPrepareSourcesCollectsFatalErrorsForMissingColoFile(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	result := PrepareSources(PrepareSourcesOptions{
		Config: cfg,
		ProcessSource: func(source Source) (SourceProcessResult, error) {
			return SourceProcessResult{
				InvalidCount: 1,
				Status:       SourceStatus{ID: source.ID, StatusText: "最近读取失败 · COLO 文件不存在"},
				Warnings:     []string{"额外 warning"},
			}, errors.New("COLO 文件不存在")
		},
		Sources: []Source{
			{Enabled: true, ID: "1", Name: "missing-colo"},
		},
	})

	if result.InvalidCount != 1 {
		t.Fatalf("InvalidCount = %d, want 1", result.InvalidCount)
	}
	if len(result.FatalErrors) != 1 || !strings.Contains(result.FatalErrors[0], "missing-colo") {
		t.Fatalf("FatalErrors = %#v, want missing-colo message", result.FatalErrors)
	}
	if len(result.SourceStatuses) != 1 || result.SourceStatuses[0].StatusText == "" {
		t.Fatalf("SourceStatuses = %#v, want failed status", result.SourceStatuses)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("Warnings = %#v, want failure message plus extra warning", result.Warnings)
	}
}

func TestSourceHelpersNormalizeFields(t *testing.T) {
	if got := SourceName(Source{Label: "  备用来源  "}); got != "备用来源" {
		t.Fatalf("SourceName() = %q, want 备用来源", got)
	}
	if got := SourceKind(Source{Kind: "FILE"}); got != "file" {
		t.Fatalf("SourceKind() = %q, want file", got)
	}
	if !SourceEnabled(Source{}) {
		t.Fatal("SourceEnabled() = false, want true for legacy empty source")
	}
	if got := SourceIPLimit(Source{}, 500); got != 500 {
		t.Fatalf("SourceIPLimit() = %d, want 500", got)
	}
	if got := SourceIPMode(Source{IPMode: "MCIS"}); got != "mcis" {
		t.Fatalf("SourceIPMode() = %q, want mcis", got)
	}
}
