package appcore

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
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

func TestBuildUploadSelectionUsesRootProviderTopNBeforeLegacyUpload(t *testing.T) {
	snapshot := map[string]any{
		"cloudflare": map[string]any{
			"top_n": 2,
		},
		"github": map[string]any{
			"top_n": 1,
		},
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"top_n": 1,
			},
			"github": map[string]any{
				"top_n": 3,
			},
		},
	}
	rows := []probecore.ProbeRow{
		{DownloadSpeedMB: 10, IP: "203.0.113.1"},
		{DownloadSpeedMB: 30, IP: "203.0.113.2"},
		{DownloadSpeedMB: 20, IP: "203.0.113.3"},
	}

	result, err := BuildUploadSelection(snapshot, rows, "average")
	if err != nil {
		t.Fatalf("BuildUploadSelection returned error: %v", err)
	}
	if got := []string{result.CloudflareRows[0].IP, result.CloudflareRows[1].IP}; !reflect.DeepEqual(got, []string{"203.0.113.2", "203.0.113.3"}) {
		t.Fatalf("CloudflareRows = %#v, want root top_n=2", got)
	}
	if got := []string{result.GitHubRows[0].IP}; !reflect.DeepEqual(got, []string{"203.0.113.2"}) {
		t.Fatalf("GitHubRows = %#v, want root top_n=1", got)
	}
}

func TestBuildUploadSelectionFallsBackToLegacyProviderTopN(t *testing.T) {
	snapshot := map[string]any{
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"top_n": 1,
			},
			"github": map[string]any{
				"top_n": 2,
			},
		},
	}
	rows := []probecore.ProbeRow{
		{DownloadSpeedMB: 10, IP: "203.0.113.1"},
		{DownloadSpeedMB: 30, IP: "203.0.113.2"},
		{DownloadSpeedMB: 20, IP: "203.0.113.3"},
	}

	result, err := BuildUploadSelection(snapshot, rows, "average")
	if err != nil {
		t.Fatalf("BuildUploadSelection returned error: %v", err)
	}
	if got := []string{result.CloudflareRows[0].IP}; !reflect.DeepEqual(got, []string{"203.0.113.2"}) {
		t.Fatalf("CloudflareRows = %#v, want legacy top_n=1", got)
	}
	if got := []string{result.GitHubRows[0].IP, result.GitHubRows[1].IP}; !reflect.DeepEqual(got, []string{"203.0.113.2", "203.0.113.3"}) {
		t.Fatalf("GitHubRows = %#v, want legacy top_n=2", got)
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

func TestBuildUploadSelectionSharedFilterMatchesCountryCodes(t *testing.T) {
	dir := t.TempDir()
	coloPath := filepath.Join(dir, "cloudflare-colos.csv")
	if err := os.WriteFile(coloPath, []byte("ip_prefix,colo,country,region,city\n198.51.100.0/24,NRT,JP,,Tokyo\n203.0.113.0/24,KIX,JP,,Osaka\n192.0.2.0/24,LAX,US,CA,Los Angeles\n"), 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}
	snapshot := map[string]any{
		"upload": map[string]any{
			"shared_filter": map[string]any{
				"colo_allow": "JP",
				"enabled":    true,
			},
		},
	}
	rows := []probecore.ProbeRow{
		{DownloadSpeedMB: 30, IP: "198.51.100.10"},
		{DownloadSpeedMB: 20, IP: "203.0.113.10"},
		{DownloadSpeedMB: 40, IP: "192.0.2.10"},
	}

	result, err := BuildUploadSelectionWithColoPaths(snapshot, rows, "average", colodict.Paths{Colo: coloPath})
	if err != nil {
		t.Fatalf("BuildUploadSelectionWithColoPaths returned error: %v", err)
	}
	if got := []string{result.FilteredRows[0].IP, result.FilteredRows[1].IP}; !reflect.DeepEqual(got, []string{"198.51.100.10", "203.0.113.10"}) {
		t.Fatalf("FilteredRows = %#v, want JP rows", got)
	}
}

func TestBuildUploadSelectionSharedFilterDenyMatchesCountryCodes(t *testing.T) {
	dir := t.TempDir()
	coloPath := filepath.Join(dir, "cloudflare-colos.csv")
	if err := os.WriteFile(coloPath, []byte("ip_prefix,colo,country,region,city\n198.51.100.0/24,NRT,JP,,Tokyo\n192.0.2.0/24,LAX,US,CA,Los Angeles\n"), 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}
	snapshot := map[string]any{
		"upload": map[string]any{
			"shared_filter": map[string]any{
				"colo_deny": "US",
				"enabled":   true,
			},
		},
	}
	rows := []probecore.ProbeRow{
		{DownloadSpeedMB: 30, IP: "198.51.100.10"},
		{DownloadSpeedMB: 40, IP: "192.0.2.10"},
	}

	result, err := BuildUploadSelectionWithColoPaths(snapshot, rows, "average", colodict.Paths{Colo: coloPath})
	if err != nil {
		t.Fatalf("BuildUploadSelectionWithColoPaths returned error: %v", err)
	}
	if got := []string{result.FilteredRows[0].IP}; !reflect.DeepEqual(got, []string{"198.51.100.10"}) {
		t.Fatalf("FilteredRows = %#v, want non-US rows", got)
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

	allRows := FilterRowsForCloudflareRecordType(rows, "ALL")
	if got := []string{allRows[0].IP, allRows[1].IP}; !reflect.DeepEqual(got, []string{"203.0.113.1", "2001:db8::1"}) {
		t.Fatalf("allRows = %#v", got)
	}
}

func TestBuildCloudflareRouteSelectionsTreatsUKAsGBAlias(t *testing.T) {
	dir := t.TempDir()
	coloPath := filepath.Join(dir, "cloudflare-colos.csv")
	if err := os.WriteFile(coloPath, []byte("ip_prefix,colo,country,region,city\n203.0.113.0/24,LHR,GB,,London\n198.51.100.0/24,NRT,JP,,Tokyo\n"), 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}
	snapshot := map[string]any{
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"routing_enabled": true,
				"routing_rules": []map[string]any{
					{"enabled": true, "name": "gb", "record_name": "gb.example.com", "filter_tokens": "GB"},
					{"enabled": true, "name": "uk", "record_name": "uk.example.com", "filter_tokens": "UK"},
				},
			},
		},
	}
	rows := []probecore.ProbeRow{
		{DownloadSpeedMB: 20, IP: "203.0.113.10"},
		{DownloadSpeedMB: 30, IP: "198.51.100.10"},
	}

	routes, warnings := BuildCloudflareRouteSelections(snapshot, rows, "average", colodict.Paths{Colo: coloPath})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if got := len(routes); got != 2 {
		t.Fatalf("routes = %d, want 2", got)
	}
	for _, route := range routes {
		if got := []string{route.Rows[0].IP}; !reflect.DeepEqual(got, []string{"203.0.113.10"}) {
			t.Fatalf("%s route rows = %#v, want GB rows", route.Rule.Name, got)
		}
	}
}

func TestBuildCloudflareRouteSelectionsMatchesColoAndCountryTokens(t *testing.T) {
	dir := t.TempDir()
	coloPath := filepath.Join(dir, "cloudflare-colos.csv")
	if err := os.WriteFile(coloPath, []byte("ip_prefix,colo,country,region,city\n203.0.113.0/24,HKG,HK,,Hong Kong\n198.51.100.0/24,NRT,JP,,Tokyo\n192.0.2.0/24,LAX,US,CA,Los Angeles\n"), 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}
	snapshot := map[string]any{
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"routing_enabled": true,
				"routing_rules": []map[string]any{
					{"enabled": true, "name": "asia", "record_name": "asia.example.com", "filter_tokens": "HKG,JP", "top_n": 1},
					{"enabled": true, "name": "not-us", "record_name": "not-us.example.com", "filter_mode": "deny", "filter_tokens": "US", "top_n": 0},
					{"enabled": true, "name": "empty", "record_name": "empty.example.com", "filter_tokens": "ZZZ"},
				},
			},
		},
	}
	rows := []probecore.ProbeRow{
		{Colo: "HKG", DownloadSpeedMB: 10, IP: "203.0.113.10"},
		{Colo: "NRT", DownloadSpeedMB: 30, IP: "198.51.100.10"},
		{Colo: "LAX", DownloadSpeedMB: 40, IP: "192.0.2.10"},
	}

	routes, warnings := BuildCloudflareRouteSelections(snapshot, rows, "average", colodict.Paths{Colo: coloPath})
	if got := len(routes); got != 3 {
		t.Fatalf("routes = %d, want 3", got)
	}
	if got := []string{routes[0].Rows[0].IP}; !reflect.DeepEqual(got, []string{"198.51.100.10"}) {
		t.Fatalf("asia route rows = %#v", got)
	}
	if got := []string{routes[1].Rows[0].IP, routes[1].Rows[1].IP}; !reflect.DeepEqual(got, []string{"203.0.113.10", "198.51.100.10"}) {
		t.Fatalf("deny route rows = %#v", got)
	}
	if !routes[2].Skipped {
		t.Fatalf("empty route should be skipped")
	}
	if stringsContainForTest(warnings, "ZZZ") {
		t.Fatalf("warnings = %#v, want pure three-character COLO token accepted without unmatched warning", warnings)
	}
}

func TestBuildCloudflareRouteSelectionsMatchesCountryByIPColoLookup(t *testing.T) {
	dir := t.TempDir()
	coloPath := filepath.Join(dir, "cloudflare-colos.csv")
	if err := os.WriteFile(coloPath, []byte("ip_prefix,colo,country,region,city\n198.51.100.0/24,NRT,JP,,Tokyo\n203.0.113.0/24,KIX,JP,,Osaka\n192.0.2.0/24,LAX,US,CA,Los Angeles\n"), 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}
	snapshot := map[string]any{
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"routing_enabled": true,
				"routing_rules": []map[string]any{
					{"enabled": true, "name": "jp", "record_name": "jp.example.com", "filter_tokens": "JP"},
				},
			},
		},
	}
	rows := []probecore.ProbeRow{
		{DownloadSpeedMB: 30, IP: "198.51.100.10"},
		{DownloadSpeedMB: 20, IP: "203.0.113.10"},
		{DownloadSpeedMB: 40, IP: "192.0.2.10"},
	}

	routes, warnings := BuildCloudflareRouteSelections(snapshot, rows, "average", colodict.Paths{Colo: coloPath})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if got := len(routes); got != 1 {
		t.Fatalf("routes = %d, want 1", got)
	}
	if got := []string{routes[0].Rows[0].IP, routes[0].Rows[1].IP}; !reflect.DeepEqual(got, []string{"198.51.100.10", "203.0.113.10"}) {
		t.Fatalf("country route rows = %#v, want both JP COLO IPs", got)
	}
}

func stringsContainForTest(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
