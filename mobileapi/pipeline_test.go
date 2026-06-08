package mobileapi

import (
	"reflect"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
)

func TestMobilePipelineDNSRowsUsesSharedUploadSelectionAndRecordType(t *testing.T) {
	snapshot := map[string]any{
		"cloudflare": map[string]any{
			"record_type": "A",
		},
		"upload": map[string]any{
			"cloudflare": map[string]any{
				"top_n": 1,
			},
			"shared_filter": map[string]any{
				"colo_allow": "HKG",
				"enabled":    true,
				"ip_version": "ipv4",
			},
		},
	}
	rows := []probeRow{
		{Colo: "HKG", DelayMS: 20, DownloadSpeedMB: 5, IP: "203.0.113.10"},
		{Colo: "HKG", DelayMS: 10, DownloadSpeedMB: 15, IP: "203.0.113.20"},
		{Colo: "HKG", DelayMS: 5, DownloadSpeedMB: 30, IP: "2001:db8::1"},
		{Colo: "LAX", DelayMS: 8, DownloadSpeedMB: 25, IP: "203.0.113.30"},
	}

	filtered, warnings, err := mobilePipelineDNSRows(snapshot, rows, "average", colodict.Paths{})
	if err != nil {
		t.Fatalf("mobilePipelineDNSRows returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if got := len(filtered); got != 1 {
		t.Fatalf("filtered length = %d, want 1", got)
	}
	if got := []string{filtered[0].IP}; !reflect.DeepEqual(got, []string{"203.0.113.20"}) {
		t.Fatalf("filtered IPs = %#v", got)
	}
}
