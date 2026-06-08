package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewPingReturnsParseCIDRError(t *testing.T) {
	oldIPText := IPText
	oldIPFile := IPFile
	t.Cleanup(func() {
		IPText = oldIPText
		IPFile = oldIPFile
	})

	IPText = "not-an-ip"
	IPFile = "unused.txt"

	if _, err := NewPing(); err == nil || !strings.Contains(err.Error(), "ParseCIDR err") {
		t.Fatalf("NewPing err = %v, want ParseCIDR error", err)
	}
}

func TestNewPingReturnsMissingIPFileError(t *testing.T) {
	oldIPText := IPText
	oldIPFile := IPFile
	t.Cleanup(func() {
		IPText = oldIPText
		IPFile = oldIPFile
	})

	IPText = ""
	IPFile = filepath.Join(t.TempDir(), "missing-ip.txt")

	if _, err := NewPing(); err == nil || !strings.Contains(err.Error(), "读取 IP 数据文件失败") {
		t.Fatalf("NewPing err = %v, want missing file error", err)
	}
}

func TestNewPingBuildsPoolFromIPText(t *testing.T) {
	oldIPText := IPText
	oldIPFile := IPFile
	t.Cleanup(func() {
		IPText = oldIPText
		IPFile = oldIPFile
	})

	IPText = "192.0.2.1"
	IPFile = "unused.txt"

	ping, err := NewPing()
	if err != nil {
		t.Fatalf("NewPing returned error: %v", err)
	}
	if len(ping.ips) != 1 || ping.ips[0].String() != "192.0.2.1" {
		t.Fatalf("ips = %#v, want one 192.0.2.1", ping.ips)
	}
}

func TestNewPingBuildsPoolFromIPFile(t *testing.T) {
	oldIPText := IPText
	oldIPFile := IPFile
	t.Cleanup(func() {
		IPText = oldIPText
		IPFile = oldIPFile
	})

	path := filepath.Join(t.TempDir(), "ip.txt")
	if err := os.WriteFile(path, []byte("192.0.2.2\n"), 0o600); err != nil {
		t.Fatalf("write ip file: %v", err)
	}
	IPText = ""
	IPFile = path

	ping, err := NewPing()
	if err != nil {
		t.Fatalf("NewPing returned error: %v", err)
	}
	if len(ping.ips) != 1 || ping.ips[0].String() != "192.0.2.2" {
		t.Fatalf("ips = %#v, want one 192.0.2.2", ping.ips)
	}
}
