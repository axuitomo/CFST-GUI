package mobileapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraceSinkWritesBridgeDebugLog(t *testing.T) {
	service := NewService()
	baseDir := t.TempDir()
	seedReadyColoDictionaryForTest(t, baseDir)
	if got := service.Init(baseDir); !strings.Contains(got, `"code":"MOBILE_INIT_OK"`) {
		t.Fatalf("unexpected init result: %s", got)
	}
	service.Invoke("task.get", `{"limit":1}`)
	service.Invoke("bridge.trace", `{"event":"probe.check","note":"manual probe"}`)

	logPath := filepath.Join(baseDir, "logs", "bridge-debug.log")
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("bridge-debug.log not written: %v", err)
	}
	content := string(raw)
	for _, want := range []string{`"event":"init"`, `"event":"invoke.out"`, `"event":"probe.check"`, `"command":"task.get"`} {
		if !strings.Contains(content, want) {
			t.Fatalf("trace log missing %s; got:\n%s", want, content)
		}
	}
	if lines := strings.Count(content, "\n"); lines != 3 || strings.Count(content, `"event":"invoke.out"`) != 1 {
		t.Fatalf("expected init, one business outcome and one trace; got:\n%s", content)
	}
}

func TestTraceSinkSkipsWithoutInit(t *testing.T) {
	service := NewService()
	service.Invoke("task.get", "{}")
	baseDir := service.basePath()
	logPath := filepath.Join(baseDir, "logs", "bridge-debug.log")
	if _, err := os.Stat(logPath); err == nil {
		t.Fatalf("trace file must not be created before Init: %s", logPath)
	}
}
