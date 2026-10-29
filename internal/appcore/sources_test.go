package appcore

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
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

	const hint = "跟随系统代理"
	allFailed := build(false)
	if len(allFailed.Entries) != 0 {
		t.Fatalf("Entries = %#v, want none", allFailed.Entries)
	}
	if !strings.Contains(strings.Join(allFailed.Warnings, "\n"), hint) {
		t.Fatalf("Warnings = %#v, want system-proxy hint", allFailed.Warnings)
	}
	partial := build(true)
	if strings.Contains(strings.Join(partial.Warnings, "\n"), hint) {
		t.Fatalf("Warnings = %#v, want no hint when a source succeeded", partial.Warnings)
	}
}

// 源之间互不依赖，串行读取会让阶段 0 吃满各源耗时之和。这个用例固定两个契约：
// 1) 确实并行（总耗时应接近最长单源，而不是各源之和）；
// 2) 合并结果仍按输入源顺序，警告与条目顺序和串行实现一致。
func TestPrepareSourcesReadsSourcesConcurrentlyAndKeepsOrder(t *testing.T) {
	const sources = 8

	// 用会合点代替计时：同时到达 sourcePrepareConcurrency 个说明并发生效；串行执行时凑不齐，
	// 观察者超时放行并留下较小峰值。比"总耗时 < N×delay"稳，不受机器负载影响。
	arrived := make(chan struct{}, sources)
	release := make(chan struct{})
	var peak atomic.Int64
	go func() {
		defer close(release)
		for count := 1; count <= sourcePrepareConcurrency; count++ {
			select {
			case <-arrived:
				peak.Store(int64(count))
			case <-time.After(2 * time.Second):
				return
			}
		}
	}()

	list := make([]Source, 0, sources)
	for index := 0; index < sources; index++ {
		list = append(list, Source{Enabled: true, ID: strconv.Itoa(index), Name: "源" + strconv.Itoa(index)})
	}

	result := PrepareSources(PrepareSourcesOptions{
		Config:  probecore.DefaultProbeConfig(),
		Sources: list,
		ProcessSource: func(source Source) (SourceProcessResult, error) {
			arrived <- struct{}{}
			<-release
			return SourceProcessResult{
				Entries:  []string{"10.0.0." + source.ID},
				Warnings: []string{"warn-" + source.ID},
				Status:   SourceStatus{ID: source.ID, StatusText: "ok-" + source.ID},
			}, nil
		},
	})

	if peak.Load() < 2 {
		t.Fatalf("并发峰值 = %d, want >= 2（仍然串行）", peak.Load())
	}
	if len(result.Entries) != sources || len(result.SourceStatuses) != sources || len(result.Warnings) != sources {
		t.Fatalf("entries=%d statuses=%d warnings=%d, want %d 各一份",
			len(result.Entries), len(result.SourceStatuses), len(result.Warnings), sources)
	}
	for index := 0; index < sources; index++ {
		if want := "ok-" + strconv.Itoa(index); result.SourceStatuses[index].StatusText != want {
			t.Fatalf("SourceStatuses[%d] = %q, want %q（顺序被打乱）", index, result.SourceStatuses[index].StatusText, want)
		}
		if want := "10.0.0." + strconv.Itoa(index); result.Entries[index] != want {
			t.Fatalf("Entries[%d] = %q, want %q（顺序被打乱）", index, result.Entries[index], want)
		}
		if want := "warn-" + strconv.Itoa(index); result.Warnings[index] != want {
			t.Fatalf("Warnings[%d] = %q, want %q（顺序被打乱）", index, result.Warnings[index], want)
		}
	}
}

// 禁用的源不该被读取，也不该占用并发槽位。
func TestPrepareSourcesSkipsDisabledSourcesConcurrently(t *testing.T) {
	var calls atomic.Int64
	result := PrepareSources(PrepareSourcesOptions{
		Config: probecore.DefaultProbeConfig(),
		Sources: []Source{
			{Enabled: true, ID: "a"},
			{Enabled: false, ID: "b"},
			{Enabled: true, ID: "c"},
		},
		ProcessSource: func(source Source) (SourceProcessResult, error) {
			calls.Add(1)
			return SourceProcessResult{Status: SourceStatus{ID: source.ID, StatusText: "ok"}}, nil
		},
	})
	if calls.Load() != 2 {
		t.Fatalf("ProcessSource 调用次数 = %d, want 2", calls.Load())
	}
	if len(result.SourceStatuses) != 3 {
		t.Fatalf("SourceStatuses = %#v, want 3 条（禁用源也要有状态）", result.SourceStatuses)
	}
	if disabled := result.SourceStatuses[1]; disabled.ID != "b" || !strings.Contains(disabled.StatusText, "已停用") {
		t.Fatalf("禁用源状态 = %#v, want 已停用提示", disabled)
	}
}

// 并发读取把 ProcessSource 挪到了子 goroutine，而子 goroutine 的 panic 不会被调用方的
// recover 接住：不兜住就是整个进程闪退。这个用例固定「单个源崩溃只影响它自己」。
func TestPrepareSourcesIsolatesSourcePanic(t *testing.T) {
	result := PrepareSources(PrepareSourcesOptions{
		Config: probecore.DefaultProbeConfig(),
		Sources: []Source{
			{Enabled: true, ID: "boom"},
			{Enabled: true, ID: "fine"},
		},
		ProcessSource: func(source Source) (SourceProcessResult, error) {
			if source.ID == "boom" {
				panic("source boom")
			}
			return SourceProcessResult{
				Entries: []string{"1.1.1.1"},
				Status:  SourceStatus{ID: source.ID, StatusText: "ok"},
			}, nil
		},
	})

	if len(result.Entries) != 1 || result.Entries[0] != "1.1.1.1" {
		t.Fatalf("Entries = %#v, want 正常源的结果保留", result.Entries)
	}
	if len(result.SourceStatuses) != 2 {
		t.Fatalf("SourceStatuses = %#v, want 两个源都有状态", result.SourceStatuses)
	}
	if got := result.SourceStatuses[0].StatusText; !strings.Contains(got, "读取异常中断") {
		t.Fatalf("崩溃源状态 = %q, want 读取异常中断", got)
	}
	if joined := strings.Join(result.Warnings, "\n"); !strings.Contains(joined, "输入源读取异常") {
		t.Fatalf("Warnings = %#v, want 异常说明", result.Warnings)
	}
}

// 取消不该等慢源：单个源的 HTTP 超时默认 30 秒，用户在界面上按「终止」后立刻返回
// 才是可接受的。这里用一个不会响应 ctx 的“卡死源”模拟最坏情况。
func TestPrepareSourcesReturnsPromptlyOnCancelDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	blocked := make(chan struct{})
	defer close(blocked)

	done := make(chan PreparedSources, 1)
	go func() {
		done <- PrepareSources(PrepareSourcesOptions{
			Context: ctx,
			Config:  probecore.DefaultProbeConfig(),
			Sources: []Source{
				{Enabled: true, ID: "slow-1"},
				{Enabled: true, ID: "slow-2"},
			},
			ProcessSource: func(source Source) (SourceProcessResult, error) {
				<-blocked
				return SourceProcessResult{Status: SourceStatus{ID: source.ID}}, nil
			},
		})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case result := <-done:
		if !errors.Is(result.Error, context.Canceled) {
			t.Fatalf("Error = %v, want context.Canceled", result.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("取消后 PrepareSources 仍在等待慢源")
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
