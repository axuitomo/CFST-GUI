package appcore

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

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
					Entries:          []string{"104.16.0.1", "104.20.0.1"},
					ColoFilter:       "SJC",
					ColoFilterActive: true,
					ColoFilterColos:  []string{"SJC"},
					ColoMode:         "allow",
					SourcePorts:      map[string]int{"104.16.0.1": 443},
					Status:           SourceStatus{ID: source.ID, StatusText: "ok-sjc"},
					Warnings:         []string{"第二阶段起效"},
				}, nil
			case "lax":
				return SourceProcessResult{
					Entries:          []string{"104.16.0.1"},
					ColoFilter:       "LAX",
					ColoFilterActive: true,
					ColoFilterColos:  []string{"LAX"},
					ColoMode:         "allow",
					Status:           SourceStatus{ID: source.ID, StatusText: "ok-lax"},
					Warnings: []string{
						"第二阶段起效",
					},
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

func TestPrepareSourcesWarnsWhenEverySourceFailed(t *testing.T) {
	build := func(failFirst bool) PreparedSources {
		return PrepareSources(PrepareSourcesOptions{
			Config: probecore.DefaultProbeConfig(),
			ProcessSource: func(source Source) (SourceProcessResult, error) {
				if source.ID == "1" {
					return SourceProcessResult{Status: SourceStatus{ID: source.ID}}, errors.New("dial tcp: i/o timeout")
				}
				if failFirst {
					return SourceProcessResult{Status: SourceStatus{ID: source.ID}, Entries: []string{"1.1.1.1"}}, nil
				}
				return SourceProcessResult{Status: SourceStatus{ID: source.ID}}, errors.New("dial tcp: i/o timeout")
			},
			Sources: []Source{{Enabled: true, ID: "1", Name: "a"}, {Enabled: true, ID: "2", Name: "b"}},
		})
	}

	const hint = "不使用 HTTP_PROXY/HTTPS_PROXY"
	allFailed := build(false)
	if len(allFailed.Entries) != 0 {
		t.Fatalf("Entries = %#v, want none", allFailed.Entries)
	}
	if !strings.Contains(strings.Join(allFailed.Warnings, "\n"), hint) {
		t.Fatalf("Warnings = %#v, want direct-connection hint", allFailed.Warnings)
	}
	partial := build(true)
	if strings.Contains(strings.Join(partial.Warnings, "\n"), hint) {
		t.Fatalf("Warnings = %#v, want no hint when a source succeeded", partial.Warnings)
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
	if got := SourceIPLimit(Source{IPLimit: -1}, 500); got != -1 {
		t.Fatalf("SourceIPLimit(-1) = %d, want -1 passed through to the pool builder", got)
	}
	if got := SourceIPMode(Source{IPMode: "MCIS"}); got != "mcis" {
		t.Fatalf("SourceIPMode() = %q, want mcis", got)
	}
}

func TestSourcesFromAnyDefaultsMissingEnabledToEnabled(t *testing.T) {
	sources := SourcesFromAny([]any{
		map[string]any{"id": "source-1", "name": "named", "url": "https://example.com/a.txt"},
		map[string]any{"id": "source-2", "name": "paused", "enabled": false, "url": "https://example.com/b.txt"},
		map[string]any{"id": "source-3", "name": "active", "enabled": true, "url": "https://example.com/c.txt"},
	})
	if len(sources) != 3 {
		t.Fatalf("sources = %#v, want 3", sources)
	}
	if !SourceEnabled(sources[0]) {
		t.Fatal("source without an enabled key must stay enabled")
	}
	if SourceEnabled(sources[1]) {
		t.Fatal("explicit enabled:false must stay disabled")
	}
	if !SourceEnabled(sources[2]) {
		t.Fatal("explicit enabled:true must stay enabled")
	}
}

func TestProcessSourceFailureKeepsLastSuccessMetadata(t *testing.T) {
	now := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	result, err := ProcessSource(Source{
		ID: "source-1", LastFetchedAt: "2026-02-28T08:00:00Z", LastFetchedCount: 42,
	}, probecore.DefaultProbeConfig(), nil, now,
		func(Source, probecore.ProbeConfig, *http.Client) (SourceContentResult, error) {
			return SourceContentResult{}, errors.New("network down")
		}, nil)
	if err == nil {
		t.Fatal("ProcessSource() error = nil, want failure")
	}
	if result.Status.LastFetchedAt != "2026-02-28T08:00:00Z" || result.Status.LastFetchedCount != 42 {
		t.Fatalf("status = %#v, want last successful fetch preserved", result.Status)
	}
	if !strings.Contains(result.Status.StatusText, "network down") {
		t.Fatalf("status text = %q, want failure reason", result.Status.StatusText)
	}
}
