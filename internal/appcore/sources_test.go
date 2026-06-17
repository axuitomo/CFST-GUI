package appcore

import (
	"errors"
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
			{Content: "sjc", Enabled: true, ID: "1", Kind: "inline", Name: "sjc"},
			{Content: "lax", Enabled: true, ID: "2", Kind: "inline", Name: "lax"},
			{Content: "unrestricted", Enabled: true, ID: "3", Kind: "inline", Name: "unrestricted"},
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
			{Content: "missing-colo", Enabled: true, ID: "1", Kind: "inline", Name: "missing-colo"},
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

func TestPrepareSourcesSkipsEnabledSourcesWithoutInput(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	result := PrepareSources(PrepareSourcesOptions{
		Config: cfg,
		ProcessSource: func(source Source) (SourceProcessResult, error) {
			t.Fatalf("ProcessSource called for empty source %#v", source)
			return SourceProcessResult{}, nil
		},
		Sources: []Source{
			{Enabled: true, ID: "inline-empty", Kind: "inline", StatusText: "keep-inline"},
			{Enabled: true, ID: "file-empty", Kind: "file", StatusText: "keep-file"},
			{Enabled: true, ID: "url-empty", Kind: "url", StatusText: "keep-url"},
		},
	})

	if result.Text != "" {
		t.Fatalf("Text = %q, want empty", result.Text)
	}
	if len(result.Warnings) != 0 || len(result.FatalErrors) != 0 {
		t.Fatalf("Warnings/FatalErrors = %#v/%#v, want none", result.Warnings, result.FatalErrors)
	}
	if len(result.SourceStatuses) != 3 {
		t.Fatalf("SourceStatuses = %d, want 3", len(result.SourceStatuses))
	}
	for index, want := range []string{"keep-inline", "keep-file", "keep-url"} {
		if got := result.SourceStatuses[index].StatusText; got != want {
			t.Fatalf("SourceStatuses[%d].StatusText = %q, want %q", index, got, want)
		}
	}
}

func TestPrepareSourcesProcessesEnabledSourcesConcurrentlyInStableOrder(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	started := make(chan string, 2)
	release := make(chan struct{})

	result := make(chan PreparedSources, 1)
	go func() {
		result <- PrepareSources(PrepareSourcesOptions{
			Config: cfg,
			ProcessSource: func(source Source) (SourceProcessResult, error) {
				started <- source.ID
				if source.ID == "slow-1" {
					<-release
				}
				return SourceProcessResult{
					Entries: []string{source.ID},
					Status:  SourceStatus{ID: source.ID, StatusText: "ok-" + source.ID},
				}, nil
			},
			Sources: []Source{
				{Content: "slow-1", Enabled: true, ID: "slow-1", Kind: "inline", Name: "slow-1"},
				{Content: "fast-2", Enabled: true, ID: "fast-2", Kind: "inline", Name: "fast-2"},
			},
		})
	}()

	first := waitForSourceStart(t, started)
	second := waitForSourceStart(t, started)
	startedSources := map[string]bool{first: true, second: true}
	if !startedSources["slow-1"] || !startedSources["fast-2"] || len(startedSources) != 2 {
		t.Fatalf("started = [%s %s], want slow-1 and fast-2 before release", first, second)
	}
	close(release)

	select {
	case prepared := <-result:
		if prepared.Text != "slow-1\nfast-2" {
			t.Fatalf("Text = %q, want stable source order", prepared.Text)
		}
		if len(prepared.SourceStatuses) != 2 || prepared.SourceStatuses[0].ID != "slow-1" || prepared.SourceStatuses[1].ID != "fast-2" {
			t.Fatalf("SourceStatuses = %#v, want stable source order", prepared.SourceStatuses)
		}
	case <-time.After(time.Second):
		t.Fatal("PrepareSources did not complete after releasing blocked source")
	}
}

func TestPrepareSourcesLimitsConcurrentWork(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	started := make(chan string, defaultPrepareSourcesConcurrency+2)
	release := make(chan struct{})
	sources := make([]Source, 0, defaultPrepareSourcesConcurrency+2)
	for index := range defaultPrepareSourcesConcurrency + 2 {
		sourceID := string(rune('a' + index))
		sources = append(sources, Source{Content: sourceID, Enabled: true, ID: sourceID, Kind: "inline", Name: sourceID})
	}

	result := make(chan PreparedSources, 1)
	go func() {
		result <- PrepareSources(PrepareSourcesOptions{
			Config: cfg,
			ProcessSource: func(source Source) (SourceProcessResult, error) {
				started <- source.ID
				<-release
				return SourceProcessResult{
					Entries: []string{source.ID},
					Status:  SourceStatus{ID: source.ID, StatusText: "ok-" + source.ID},
				}, nil
			},
			Sources: sources,
		})
	}()

	for range defaultPrepareSourcesConcurrency {
		_ = waitForSourceStart(t, started)
	}
	select {
	case sourceID := <-started:
		t.Fatalf("source %s started before a worker slot was released", sourceID)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)

	select {
	case prepared := <-result:
		if len(prepared.SourceStatuses) != len(sources) {
			t.Fatalf("SourceStatuses = %d, want %d", len(prepared.SourceStatuses), len(sources))
		}
	case <-time.After(time.Second):
		t.Fatal("PrepareSources did not complete after releasing worker slots")
	}
}

func TestPrepareSourcesUsesConfiguredConcurrency(t *testing.T) {
	started := make(chan string, 4)
	release := make(chan struct{})
	result := make(chan PreparedSources, 1)

	sources := []Source{
		{Content: "source-1", Enabled: true, ID: "source-1", Kind: "inline", Name: "source-1"},
		{Content: "source-2", Enabled: true, ID: "source-2", Kind: "inline", Name: "source-2"},
		{Content: "source-3", Enabled: true, ID: "source-3", Kind: "inline", Name: "source-3"},
	}

	go func() {
		result <- PrepareSources(PrepareSourcesOptions{
			Concurrency: 2,
			ProcessSource: func(source Source) (SourceProcessResult, error) {
				started <- source.ID
				<-release
				return SourceProcessResult{
					Entries: []string{source.ID},
					Status:  SourceStatus{ID: source.ID, StatusText: "ok-" + source.ID},
				}, nil
			},
			Sources: sources,
		})
	}()

	for range 2 {
		_ = waitForSourceStart(t, started)
	}
	select {
	case sourceID := <-started:
		t.Fatalf("source %s started before configured worker slots were released", sourceID)
	case <-time.After(30 * time.Millisecond):
	}

	close(release)
	select {
	case prepared := <-result:
		if prepared.Text != "source-1\nsource-2\nsource-3" {
			t.Fatalf("Text = %q, want stable configured-concurrency order", prepared.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("PrepareSources did not complete after releasing configured worker slots")
	}
}

func TestPrepareSourcesConvertsProcessPanicToSourceFailure(t *testing.T) {
	cfg := probecore.DefaultProbeConfig()
	result := PrepareSources(PrepareSourcesOptions{
		Config: cfg,
		ProcessSource: func(source Source) (SourceProcessResult, error) {
			if source.ID == "panic-source" {
				panic("boom")
			}
			return SourceProcessResult{
				Entries: []string{source.ID},
				Status:  SourceStatus{ID: source.ID, StatusText: "ok-" + source.ID},
			}, nil
		},
		Sources: []Source{
			{Content: "panic-source", Enabled: true, ID: "panic-source", Kind: "inline", Name: "panic-source"},
			{Content: "good-source", Enabled: true, ID: "good-source", Kind: "inline", Name: "good-source"},
		},
	})

	if result.Text != "good-source" {
		t.Fatalf("Text = %q, want surviving source result", result.Text)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "输入源 panic-source 读取失败") {
		t.Fatalf("Warnings = %#v, want panic source failure", result.Warnings)
	}
	if len(result.SourceStatuses) != 2 || !strings.Contains(result.SourceStatuses[0].StatusText, "输入源处理异常") || result.SourceStatuses[1].ID != "good-source" {
		t.Fatalf("SourceStatuses = %#v, want panic status then good status", result.SourceStatuses)
	}
}

func waitForSourceStart(t *testing.T, started <-chan string) string {
	t.Helper()
	select {
	case sourceID := <-started:
		return sourceID
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for source processing to start")
		return ""
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
