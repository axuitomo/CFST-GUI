package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
)

// 桌面（ServiceStartup）与 WebUI（startup）共用 prepareRuntime：
// 启动准备必须真的触发一次 COLO 词典自检，缺表时在本地补出来。
func TestPrepareRuntimeWarmsColoDictionary(t *testing.T) {
	root := isolateStorageForTest(t)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", root, err)
	}
	if err := os.WriteFile(filepath.Join(root, colodict.GeofeedFileName), []byte(geofeedForStartupTest), 0o600); err != nil {
		t.Fatalf("write geofeed: %v", err)
	}

	application := NewApp()
	application.prepareRuntime(context.Background())

	paths := colodict.DefaultPaths(root)
	deadline := time.Now().Add(10 * time.Second)
	for {
		status, err := colodict.StatusForPaths(paths)
		if err == nil && status.ColoRows > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("启动后 COLO 词典没有被补出来：rows=%d err=%v", status.ColoRows, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

const geofeedForStartupTest = `ip_prefix,country,region,city,postal_code
104.16.0.0/13,US,CA,San Jose,95113
`
