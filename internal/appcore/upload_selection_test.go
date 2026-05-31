package appcore

import (
	"reflect"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func TestBuildUploadSelectionAppliesSharedFilterAndTopN(t *testing.T) {
	snapshot := map[string]any{
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"top_n": 1,
			},
			"github": map[string]any{
				"top_n": 2,
			},
			"shared_filter": map[string]any{
				"colo_allow":         "HKG",
				"enabled":            true,
				"ip_version":         "ipv4",
				"max_tcp_latency_ms": 20,
				"min_download_mbps":  5,
			},
		},
	}
	rows := []probecore.ProbeRow{
		{Colo: "HKG", DelayMS: 12, DownloadSpeedMB: 8, IP: "203.0.113.1"},
		{Colo: "HKG", DelayMS: 10, DownloadSpeedMB: 20, IP: "203.0.113.2"},
		{Colo: "NRT", DelayMS: 8, DownloadSpeedMB: 30, IP: "203.0.113.3"},
		{Colo: "HKG", DelayMS: 5, DownloadSpeedMB: 15, IP: "2001:db8::1"},
	}

	result, err := BuildUploadSelection(snapshot, rows, "average")
	if err != nil {
		t.Fatalf("BuildUploadSelection returned error: %v", err)
	}
	if got := len(result.InputRows); got != 4 {
		t.Fatalf("InputRows = %d, want 4", got)
	}
	if got := len(result.FilteredRows); got != 2 {
		t.Fatalf("FilteredRows = %d, want 2", got)
	}
	if got := []string{result.FilteredRows[0].IP, result.FilteredRows[1].IP}; !reflect.DeepEqual(got, []string{"203.0.113.1", "203.0.113.2"}) {
		t.Fatalf("FilteredRows IPs = %#v", got)
	}
	if got := len(result.CloudflareRows); got != 1 {
		t.Fatalf("CloudflareRows = %d, want 1", got)
	}
	if got := result.CloudflareRows[0].IP; got != "203.0.113.2" {
		t.Fatalf("CloudflareRows[0].IP = %q, want 203.0.113.2", got)
	}
	if got := len(result.GitHubRows); got != 2 {
		t.Fatalf("GitHubRows = %d, want 2", got)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %#v, want none", result.Warnings)
	}
}

func TestBuildUploadSelectionWarnsWhenSharedFilterRemovesAllRows(t *testing.T) {
	snapshot := map[string]any{
		"upload": map[string]any{
			"shared_filter": map[string]any{
				"colo_allow": "LAX",
				"enabled":    true,
			},
		},
	}
	rows := []probecore.ProbeRow{
		{Colo: "HKG", IP: "203.0.113.1"},
	}

	result, err := BuildUploadSelection(snapshot, rows, "average")
	if err != nil {
		t.Fatalf("BuildUploadSelection returned error: %v", err)
	}
	if len(result.FilteredRows) != 0 {
		t.Fatalf("FilteredRows = %#v, want empty", result.FilteredRows)
	}
	if !reflect.DeepEqual(result.Warnings, []string{"共享上传筛选后没有剩余结果。"}) {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
}

func TestFilterRowsForCloudflareRecordType(t *testing.T) {
	rows := []probecore.ProbeRow{
		{IP: "203.0.113.1"},
		{IP: "2001:db8::1"},
		{IP: "not-an-ip"},
	}

	ipv4Rows := FilterRowsForCloudflareRecordType(rows, "A")
	if got := []string{ipv4Rows[0].IP}; !reflect.DeepEqual(got, []string{"203.0.113.1"}) {
		t.Fatalf("ipv4Rows = %#v", got)
	}

	ipv6Rows := FilterRowsForCloudflareRecordType(rows, "AAAA")
	if got := []string{ipv6Rows[0].IP}; !reflect.DeepEqual(got, []string{"2001:db8::1"}) {
		t.Fatalf("ipv6Rows = %#v", got)
	}
}
