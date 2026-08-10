package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewPingReturnsParseCIDRError(t *testing.T) {
	config := DefaultConfig()
	config.IPText = "not-an-ip"
	config.IPFile = "unused.txt"

	if _, err := NewEngine(config, Hooks{}).NewPing(); err == nil || !strings.Contains(err.Error(), "ParseCIDR err") {
		t.Fatalf("NewPing err = %v, want ParseCIDR error", err)
	}
}

func TestNewPingReturnsMissingIPFileError(t *testing.T) {
	config := DefaultConfig()
	config.IPText = ""
	config.IPFile = filepath.Join(t.TempDir(), "missing-ip.txt")

	if _, err := NewEngine(config, Hooks{}).NewPing(); err == nil || !strings.Contains(err.Error(), "读取 IP 数据文件失败") {
		t.Fatalf("NewPing err = %v, want missing file error", err)
	}
}

func TestNewPingBuildsPoolFromIPText(t *testing.T) {
	config := DefaultConfig()
	config.IPText = "192.0.2.1"
	config.IPFile = "unused.txt"

	ping, err := NewEngine(config, Hooks{}).NewPing()
	if err != nil {
		t.Fatalf("NewPing returned error: %v", err)
	}
	if len(ping.ips) != 1 || ping.ips[0].String() != "192.0.2.1" {
		t.Fatalf("ips = %#v, want one 192.0.2.1", ping.ips)
	}
}

func TestNewPingBuildsPoolFromIPFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ip.txt")
	if err := os.WriteFile(path, []byte("192.0.2.2\n"), 0o600); err != nil {
		t.Fatalf("write ip file: %v", err)
	}
	config := DefaultConfig()
	config.IPText = ""
	config.IPFile = path

	ping, err := NewEngine(config, Hooks{}).NewPing()
	if err != nil {
		t.Fatalf("NewPing returned error: %v", err)
	}
	if len(ping.ips) != 1 || ping.ips[0].String() != "192.0.2.2" {
		t.Fatalf("ips = %#v, want one 192.0.2.2", ping.ips)
	}
}
