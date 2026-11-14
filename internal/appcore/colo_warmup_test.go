package appcore

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func seedColoGeofeedForTest(t *testing.T, path string) {
	t.Helper()
	raw := strings.Join([]string{
		"ip_prefix,country,region,city,postal_code",
		"104.16.0.0/13,US,CA,San Jose,95113",
		"104.22.1.0/24,US,US-GA,Atlanta,",
	}, "\n")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write geofeed: %v", err)
	}
}

// 启动自检发现词典缺失时必须自己补出来；本地已有 geofeed 时不该再下载。
func TestColoDictionaryWarmupBuildsMissingDictionary(t *testing.T) {
	root := t.TempDir()
	seedColoGeofeedForTest(t, filepath.Join(root, colodict.GeofeedFileName))
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: root}})

	service.warmupColoDictionary(context.Background(), root)

	status, err := colodict.StatusForPaths(colodict.DefaultPaths(root))
	if err != nil {
		t.Fatalf("StatusForPaths: %v", err)
	}
	if status.ColoRows == 0 {
		t.Fatalf("colo rows = %d, want rebuilt dictionary", status.ColoRows)
	}
	if warning := service.ColoDictionaryWarning(); warning != "" {
		t.Fatalf("warning = %q, want empty after successful warmup", warning)
	}
}

// 词典已经在位时启动自检必须原地返回：一次补词典动作都不该发生。
func TestColoDictionaryWarmupSkipsReadyDictionary(t *testing.T) {
	root := t.TempDir()
	paths := colodict.DefaultPaths(root)
	raw, err := colodict.EncodeColoEntries([]colodict.ColoEntry{{
		Prefix: netip.MustParsePrefix("104.16.0.0/13"), Colo: "SJC", Country: "US",
	}})
	if err != nil {
		t.Fatalf("EncodeColoEntries: %v", err)
	}
	// 三份都写齐才算“词典就绪”：主表 + IPv4 分表 + 只有表头的 IPv6 分表。
	emptyIPv6, err := colodict.EncodeColoEntries(nil)
	if err != nil {
		t.Fatalf("EncodeColoEntries(nil): %v", err)
	}
	for path, payload := range map[string][]byte{paths.Colo: raw, paths.ColoIPv4: raw, paths.ColoIPv6: emptyIPv6} {
		if err := os.WriteFile(path, payload, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	refreshes := 0
	service := NewService(ServiceOptions{
		ColoDictionaryRefresh: func(context.Context, colodict.Paths) (colodict.UpdateResult, error) {
			refreshes++
			return colodict.UpdateResult{}, errors.New("must not refresh a ready dictionary")
		},
		Storage: StorageLayout{Root: root},
	})

	service.warmupColoDictionary(context.Background(), root)

	if refreshes != 0 {
		t.Fatalf("refreshes = %d, want 0 for a ready dictionary", refreshes)
	}
	if warning := service.ColoDictionaryWarning(); warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
}

// 主表在、但 IPv4/IPv6 分表缺失时，按地址族的筛选会直接报“COLO 文件不存在”，
// 所以自检必须把分表也算进“词典是否就绪”，并从本地 GEOFEED 重建。
func TestColoDictionaryWarmupRebuildsMissingSplitFiles(t *testing.T) {
	root := t.TempDir()
	paths := colodict.DefaultPaths(root)
	seedColoGeofeedForTest(t, paths.Geofeed)
	raw, err := colodict.EncodeColoEntries([]colodict.ColoEntry{{
		Prefix: netip.MustParsePrefix("104.16.0.0/13"), Colo: "SJC", Country: "US",
	}})
	if err != nil {
		t.Fatalf("EncodeColoEntries: %v", err)
	}
	if err := os.WriteFile(paths.Colo, raw, 0o600); err != nil {
		t.Fatalf("write colo dictionary: %v", err)
	}
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: root}})

	service.warmupColoDictionary(context.Background(), root)

	for _, path := range []string{paths.ColoIPv4, paths.ColoIPv6} {
		if _, err := colodict.LoadColoEntries(path); err != nil {
			t.Fatalf("LoadColoEntries(%s) 失败，分表没有被重建：%v", path, err)
		}
	}
	if warning := service.ColoDictionaryWarning(); warning != "" {
		t.Fatalf("warning = %q, want empty after rebuild", warning)
	}
}

// 补词典失败要留成用户可见的说明：状态可从 Service 读到，探测结果也会带上。
func TestColoDictionaryWarmupFailureReachesProbeWarnings(t *testing.T) {
	root := t.TempDir()
	service := NewService(ServiceOptions{
		EventSink: EventSinkFunc(func(context.Context, ProbeEvent) {}),
		ProbeHooks: ProbeExecutionHooks{
			RunTCP:   func(*task.Engine) (utils.PingDelaySet, error) { return probeSampleForTest(), nil },
			RunTrace: func(_ *task.Engine, input utils.PingDelaySet) utils.PingDelaySet { return input },
		},
		Storage: StorageLayout{Root: root},
		ColoDictionaryRefresh: func(context.Context, colodict.Paths) (colodict.UpdateResult, error) {
			return colodict.UpdateResult{}, errors.New("network unreachable")
		},
	})

	service.warmupColoDictionary(context.Background(), root)
	if warning := service.ColoDictionaryWarning(); !strings.Contains(warning, "network unreachable") {
		t.Fatalf("warning = %q, want recorded failure", warning)
	}

	payload := probePayloadForCoreTest("task-colo-warmup", t.TempDir())
	payload.AndroidExportURI = ""
	result, err := service.RunProbe(payload)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range result.Warnings {
		if strings.Contains(entry, "COLO 词典缺失且自动更新失败") {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %#v, want colo dictionary warning", result.Warnings)
	}
}

// 同一个数据目录只自检一次；重复调用不再起新的 goroutine。
func TestStartColoDictionaryWarmupDedupesSameRoot(t *testing.T) {
	ran := make(chan struct{}, 2)
	service := NewService(ServiceOptions{
		ColoDictionaryRefresh: func(context.Context, colodict.Paths) (colodict.UpdateResult, error) {
			ran <- struct{}{}
			return colodict.UpdateResult{}, errors.New("stop after first attempt")
		},
		Storage: StorageLayout{Root: t.TempDir()},
	})

	if !service.StartColoDictionaryWarmup() {
		t.Fatal("first StartColoDictionaryWarmup = false, want true")
	}
	if service.StartColoDictionaryWarmup() {
		t.Fatal("second StartColoDictionaryWarmup = true, want false")
	}
	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("warmup goroutine did not run")
	}
	select {
	case <-ran:
		t.Fatal("warmup ran twice")
	case <-time.After(50 * time.Millisecond):
	}
}

func waitColoWarmupRefresh(t *testing.T, refreshes <-chan string) string {
	t.Helper()
	select {
	case path := <-refreshes:
		return path
	case <-time.After(5 * time.Second):
		t.Fatal("补词典动作没有发生")
		return ""
	}
}

// 换数据目录只在应用启动过自检之后才补：库用法与测试不该因为 SetStorageLayout 发起下载。
// 同一目录重复设置不重复检查。
func TestSetStorageLayoutWarmsColoDictionaryAfterStartup(t *testing.T) {
	refreshes := make(chan string, 4)
	service := NewService(ServiceOptions{
		ColoDictionaryRefresh: func(_ context.Context, paths colodict.Paths) (colodict.UpdateResult, error) {
			refreshes <- paths.Colo
			return colodict.UpdateResult{}, errors.New("offline")
		},
		Storage: StorageLayout{Root: t.TempDir()},
	})

	service.SetStorageLayout(StorageLayout{Root: t.TempDir()})
	select {
	case path := <-refreshes:
		t.Fatalf("未启动过自检就换目录，不应补词典（处理了 %s）", path)
	case <-time.After(100 * time.Millisecond):
	}

	armedRoot := t.TempDir()
	service.SetStorageLayout(StorageLayout{Root: armedRoot})
	if !service.StartColoDictionaryWarmup() {
		t.Fatal("StartColoDictionaryWarmup = false, want true")
	}
	if got := waitColoWarmupRefresh(t, refreshes); got != colodict.DefaultPaths(armedRoot).Colo {
		t.Fatalf("首次自检处理了 %s，want %s", got, colodict.DefaultPaths(armedRoot).Colo)
	}

	nextRoot := t.TempDir()
	service.SetStorageLayout(StorageLayout{Root: nextRoot})
	if got := waitColoWarmupRefresh(t, refreshes); got != colodict.DefaultPaths(nextRoot).Colo {
		t.Fatalf("换目录后处理了 %s，want %s", got, colodict.DefaultPaths(nextRoot).Colo)
	}

	service.SetStorageLayout(StorageLayout{Root: nextRoot})
	select {
	case path := <-refreshes:
		t.Fatalf("同一目录重复设置不应重复补词典（处理了 %s）", path)
	case <-time.After(100 * time.Millisecond):
	}
	if service.StartColoDictionaryWarmup() {
		t.Fatal("同一目录的 StartColoDictionaryWarmup = true, want false")
	}
}

// 断网启动时自检失败并留下说明；用户随后手动“处理词典”成功，说明必须撤回，
// 否则本次会话每次测速都在提示词典缺失，而词典其实已经在位。
func TestManualColoProcessClearsWarmupWarning(t *testing.T) {
	root := t.TempDir()
	seedColoGeofeedForTest(t, filepath.Join(root, colodict.GeofeedFileName))
	service := NewService(ServiceOptions{
		ColoDictionaryRefresh: func(context.Context, colodict.Paths) (colodict.UpdateResult, error) {
			return colodict.UpdateResult{}, errors.New("network unreachable")
		},
		Storage: StorageLayout{Root: root},
	})
	service.SetColoPaths(colodict.DefaultPaths(root))

	service.warmupColoDictionary(context.Background(), root)
	if warning := service.ColoDictionaryWarning(); !strings.Contains(warning, "network unreachable") {
		t.Fatalf("warning = %q, want recorded failure", warning)
	}

	if result := service.invokeColoProcess(`{}`); !result.OK {
		t.Fatalf("手动处理词典失败：%+v", result)
	}
	if warning := service.ColoDictionaryWarning(); warning != "" {
		t.Fatalf("手动处理成功之后仍留着失败说明：%q", warning)
	}
}

// 切换数据目录时旧目录的自检可能还在跑；它之后才失败，不能盖掉新目录的结论。
func TestColoDictionaryWarmupDropsLateFailureFromOldRoot(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	service := NewService(ServiceOptions{
		ColoDictionaryRefresh: func(_ context.Context, paths colodict.Paths) (colodict.UpdateResult, error) {
			if paths.Colo == colodict.DefaultPaths(rootA).Colo {
				close(entered)
				<-release
				return colodict.UpdateResult{}, errors.New("root A offline")
			}
			return colodict.UpdateResult{}, errors.New("root B offline")
		},
		Storage: StorageLayout{Root: rootA},
	})

	go func() {
		defer close(done)
		service.warmupColoDictionary(context.Background(), rootA)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("旧目录的自检没有开始")
	}

	service.SetStorageLayout(StorageLayout{Root: rootB})
	service.warmupColoDictionary(context.Background(), rootB)
	if warning := service.ColoDictionaryWarning(); !strings.Contains(warning, "root B offline") {
		t.Fatalf("warning = %q, want the new root's failure", warning)
	}

	close(release) // 旧目录到这里才失败
	<-done
	if warning := service.ColoDictionaryWarning(); strings.Contains(warning, "root A offline") {
		t.Fatalf("旧目录的失败说明覆盖了新目录：%q", warning)
	}
}
