package utils

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func parseCSVTestIP(value string) *net.IPAddr {
	return &net.IPAddr{IP: net.ParseIP(value)}
}

func TestExportCsvAppendWritesHeaderOnlyOnce(t *testing.T) {
	oldOutput := Output
	oldAppend := OutputAppend
	t.Cleanup(func() {
		Output = oldOutput
		OutputAppend = oldAppend
	})

	Output = filepath.Join(t.TempDir(), "result.csv")
	OutputAppend = true
	data := []CloudflareIPData{
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
				Colo:     "SJC",
			},
		},
	}

	if err := ExportCsv(data); err != nil {
		t.Fatalf("first ExportCsv returned error: %v", err)
	}
	if err := ExportCsv(data); err != nil {
		t.Fatalf("second ExportCsv returned error: %v", err)
	}

	raw, err := os.ReadFile(Output)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3\n%s", len(lines), string(raw))
	}
	if strings.Count(string(raw), "IP 地址") != 1 {
		t.Fatalf("header count mismatch:\n%s", string(raw))
	}
}

func TestSelectTopWeightedResultsUsesDelayAndSpeed(t *testing.T) {
	data := []CloudflareIPData{
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 1 * 1024 * 1024,
		},
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.2"),
				Sended:   3,
				Received: 3,
				Delay:    50 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		},
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.3"),
				Sended:   3,
				Received: 3,
				Delay:    5 * time.Millisecond,
			},
			DownloadSpeed: 512 * 1024,
		},
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.4"),
				Sended:   3,
				Received: 3,
				Delay:    100 * time.Millisecond,
			},
			DownloadSpeed: 100 * 1024 * 1024,
		},
	}

	selected := SelectTopWeightedResults(data, 2)
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}
	if selected[0].IP.String() != "1.1.1.4" || selected[1].IP.String() != "1.1.1.3" {
		t.Fatalf("selected = %s,%s; want weighted best 1.1.1.4,1.1.1.3", selected[0].IP, selected[1].IP)
	}
}

func TestSelectTopWeightedResultsUsesLossRateAndIPFallbacks(t *testing.T) {
	data := []CloudflareIPData{
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.4"),
				Sended:   20,
				Received: 18,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		},
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.2"),
				Sended:   20,
				Received: 19,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		},
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.1"),
				Sended:   20,
				Received: 19,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		},
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.3"),
				Sended:   20,
				Received: 19,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		},
	}

	selected := SelectTopWeightedResults(data, 3)
	got := []string{selected[0].IP.String(), selected[1].IP.String(), selected[2].IP.String()}
	want := []string{"1.1.1.1", "1.1.1.2", "1.1.1.3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("selected = %v, want %v", got, want)
	}
}

func TestSelectTopWeightedResultsSortsWhenLimitDoesNotTruncate(t *testing.T) {
	data := []CloudflareIPData{
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    100 * time.Millisecond,
			},
			DownloadSpeed: 1024 * 1024,
		},
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.2"),
				Sended:   3,
				Received: 3,
				Delay:    5 * time.Millisecond,
			},
			DownloadSpeed: 100 * 1024 * 1024,
		},
	}

	selected := SelectTopWeightedResults(data, 10)
	got := []string{selected[0].IP.String(), selected[1].IP.String()}
	want := []string{"1.1.1.2", "1.1.1.1"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("selected = %v, want weighted ordering %v", got, want)
	}
}

func TestFilterLossRateCapsAtFifteenPercent(t *testing.T) {
	oldMaxLossRate := InputMaxLossRate
	t.Cleanup(func() {
		InputMaxLossRate = oldMaxLossRate
	})
	InputMaxLossRate = 1
	data := PingDelaySet{
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.1"),
				Sended:   20,
				Received: 18,
				Delay:    10 * time.Millisecond,
			},
		},
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.2"),
				Sended:   20,
				Received: 16,
				Delay:    10 * time.Millisecond,
			},
		},
	}

	filtered := data.FilterLossRate()
	if len(filtered) != 1 || filtered[0].IP.String() != "1.1.1.1" {
		t.Fatalf("filtered = %#v, want only 1.1.1.1", filtered)
	}
}
