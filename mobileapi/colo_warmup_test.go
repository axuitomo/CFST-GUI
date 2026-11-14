package mobileapi

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
)

// writeColoGeofeedForTest 让启动自检只走本地重建，不联网。
func writeColoGeofeedForTest(t *testing.T, path string) {
	t.Helper()
	raw := "ip_prefix,country,region,city,postal_code\n104.16.0.0/13,US,CA,San Jose,95113\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write geofeed: %v", err)
	}
}

// seedReadyColoDictionaryForTest 预置一份可加载的词典，让 Init 触发的自检原地返回。
// 其它 Init 用例只关心自己的主题，不应该因为升起的后台自检去访问网络。
func seedReadyColoDictionaryForTest(t *testing.T, baseDir string) {
	t.Helper()
	paths := colodict.DefaultPaths(baseDir)
	ipv4, err := colodict.EncodeColoEntries([]colodict.ColoEntry{{
		Prefix: netip.MustParsePrefix("104.16.0.0/13"), Colo: "SJC", Country: "US",
	}})
	if err != nil {
		t.Fatalf("EncodeColoEntries: %v", err)
	}
	ipv6, err := colodict.EncodeColoEntries(nil)
	if err != nil {
		t.Fatalf("EncodeColoEntries(nil): %v", err)
	}
	for path, payload := range map[string][]byte{
		paths.Colo: ipv4, paths.ColoIPv4: ipv4, paths.ColoIPv6: ipv6,
	} {
		if err := os.WriteFile(path, payload, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// Android 的“打开应用”就是 Init：它必须真的触发一次 COLO 词典自检。
func TestInitWarmsColoDictionary(t *testing.T) {
	baseDir := t.TempDir()
	writeColoGeofeedForTest(t, filepath.Join(baseDir, colodict.GeofeedFileName))

	service := NewService()
	decodeCommandForTest(t, service.Init(baseDir))

	paths := colodict.DefaultPaths(baseDir)
	deadline := time.Now().Add(10 * time.Second)
	for {
		status, err := colodict.StatusForPaths(paths)
		if err == nil && status.ColoRows > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Init 后 COLO 词典没有被补出来：rows=%d err=%v", status.ColoRows, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// 运行中换数据目录（旧版 self-hosted 存储的迁移路径会再调一次 Init）要立刻对新目录补一次，
// 而不是等下次启动。
func TestInitWarmsColoDictionaryAfterStorageChange(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	seedReadyColoDictionaryForTest(t, rootA)
	writeColoGeofeedForTest(t, filepath.Join(rootB, colodict.GeofeedFileName))

	service := NewService()
	// 第一次 Init 是“打开应用”，第二次是运行中换数据目录（旧版存储迁移路径）。
	decodeCommandForTest(t, service.Init(rootA))
	decodeCommandForTest(t, service.Init(rootB))

	paths := colodict.DefaultPaths(rootB)
	deadline := time.Now().Add(10 * time.Second)
	for {
		status, err := colodict.StatusForPaths(paths)
		if err == nil && status.ColoRows > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("切换数据目录后 COLO 词典没有被补出来：rows=%d err=%v", status.ColoRows, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
