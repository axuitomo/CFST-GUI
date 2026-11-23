package utils

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportCsvContextCanceledDoesNotTouchExistingFile(t *testing.T) {
	output := filepath.Join(t.TempDir(), "result.csv")
	writer := CSVWriter{Path: output, Encoding: CSVEncodingUTF8}
	if err := os.WriteFile(output, []byte("existing-content"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := writer.ExportContext(ctx, []CloudflareIPData{csvTestData("1.1.1.1")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ExportCsvContext error = %v, want context canceled", err)
	}
	raw, readErr := os.ReadFile(output)
	if readErr != nil {
		t.Fatalf("ReadFile returned error: %v", readErr)
	}
	if string(raw) != "existing-content" {
		t.Fatalf("existing output changed after cancellation: %q", raw)
	}
}

func parseCSVTestIP(value string) *net.IPAddr {
	return &net.IPAddr{IP: net.ParseIP(value)}
}

func TestExportCsvAppendWritesHeaderOnlyOnce(t *testing.T) {
	output := filepath.Join(t.TempDir(), "result.csv")
	writer := CSVWriter{Path: output, Append: true, Encoding: CSVEncodingUTF8}
	data := []CloudflareIPData{
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
				Colo:     "SJC",
			},
			DownloadSpeed:    12 * 1024 * 1024,
			MaxDownloadSpeed: 24 * 1024 * 1024,
		},
	}

	if err := writer.ExportContext(context.Background(), data); err != nil {
		t.Fatalf("first ExportCsv returned error: %v", err)
	}
	if err := writer.ExportContext(context.Background(), data); err != nil {
		t.Fatalf("second ExportCsv returned error: %v", err)
	}

	raw, err := os.ReadFile(output)
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
	if !strings.Contains(string(raw), "平均速率(MB/s),最高速率(MB/s)") {
		t.Fatalf("header missing average/max speed columns:\n%s", string(raw))
	}
	if !strings.Contains(string(raw), "12.00,24.00,SJC") {
		t.Fatalf("row missing average/max speed values:\n%s", string(raw))
	}
}

// 备注是 CSV 的最后一列，追踪延迟在它前面；列名和顺序决定旧文件还能不能按表头解析。
func TestExportCsvWritesTraceDelayAndNoteColumns(t *testing.T) {
	output := filepath.Join(t.TempDir(), "result.csv")
	writer := CSVWriter{Path: output, Encoding: CSVEncodingUTF8}
	data := []CloudflareIPData{{
		PingData: &PingData{
			IP:       parseCSVTestIP("1.1.1.1"),
			Sended:   3,
			Received: 3,
			Delay:    10 * time.Millisecond,
			Colo:     "SJC",
			Note:     "1034",
		},
		HeadDelay: 34560 * time.Microsecond,
	}}

	if err := writer.ExportContext(context.Background(), data); err != nil {
		t.Fatalf("ExportContext returned error: %v", err)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2\n%s", len(lines), string(raw))
	}
	if !strings.HasSuffix(strings.TrimSpace(lines[0]), "地区码,国家/地区,追踪延迟(ms),备注") {
		t.Fatalf("header does not end with the trace delay and note columns:\n%s", lines[0])
	}
	if !strings.HasSuffix(strings.TrimSpace(lines[1]), ",SJC,美国,34.56,1034") {
		t.Fatalf("row does not end with colo, country, trace delay and note:\n%s", lines[1])
	}
}

func TestExportCsvDefaultUTF8DoesNotWriteBOM(t *testing.T) {
	output := filepath.Join(t.TempDir(), "result.csv")
	writer := CSVWriter{Path: output, Encoding: CSVEncodingUTF8}
	if err := writer.ExportContext(context.Background(), []CloudflareIPData{csvTestData("1.1.1.1")}); err != nil {
		t.Fatalf("ExportCsv returned error: %v", err)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if strings.HasPrefix(string(raw), utf8BOM) {
		t.Fatalf("CSV unexpectedly starts with BOM: %q", string(raw[:3]))
	}
}

func TestExportCsvUTF8BOMWritesBOMOnlyForNewContent(t *testing.T) {
	output := filepath.Join(t.TempDir(), "result.csv")
	writer := CSVWriter{Path: output, Append: true, Encoding: CSVEncodingUTF8BOM}
	if err := writer.ExportContext(context.Background(), []CloudflareIPData{csvTestData("1.1.1.1")}); err != nil {
		t.Fatalf("first ExportCsv returned error: %v", err)
	}
	if err := writer.ExportContext(context.Background(), []CloudflareIPData{csvTestData("1.1.1.2")}); err != nil {
		t.Fatalf("second ExportCsv returned error: %v", err)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.HasPrefix(string(raw), utf8BOM) {
		t.Fatalf("CSV does not start with BOM: %q", string(raw[:3]))
	}
	if strings.Count(string(raw), utf8BOM) != 1 {
		t.Fatalf("BOM count mismatch:\n%s", string(raw))
	}
	if strings.Count(string(raw), "IP 地址") != 1 {
		t.Fatalf("header count mismatch:\n%s", string(raw))
	}
}

func csvTestData(ip string) CloudflareIPData {
	return CloudflareIPData{
		PingData: &PingData{
			IP:       parseCSVTestIP(ip),
			Sended:   3,
			Received: 3,
			Delay:    10 * time.Millisecond,
			Colo:     "SJC",
		},
		DownloadSpeed:    12 * 1024 * 1024,
		MaxDownloadSpeed: 24 * 1024 * 1024,
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

func TestSelectTopWeightedResultsByMetricUsesMaxSpeed(t *testing.T) {
	data := []CloudflareIPData{
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed:    5 * 1024 * 1024,
			MaxDownloadSpeed: 100 * 1024 * 1024,
		},
		{
			PingData: &PingData{
				IP:       parseCSVTestIP("1.1.1.2"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed:    50 * 1024 * 1024,
			MaxDownloadSpeed: 10 * 1024 * 1024,
		},
	}

	averageSelected := SelectTopWeightedResultsByMetric(data, 1, DownloadSpeedMetricAverage)
	if averageSelected[0].IP.String() != "1.1.1.2" {
		t.Fatalf("average selected = %s, want 1.1.1.2", averageSelected[0].IP)
	}
	maxSelected := SelectTopWeightedResultsByMetric(data, 1, DownloadSpeedMetricMax)
	if maxSelected[0].IP.String() != "1.1.1.1" {
		t.Fatalf("max selected = %s, want 1.1.1.1", maxSelected[0].IP)
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

func TestFilterLossRateKeepsDefaultFifteenPercent(t *testing.T) {
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

	filtered := FilterPingLossRate(data, FilterConfig{MaxLossRate: DefaultMaxLossRate})
	if len(filtered) != 1 || filtered[0].IP.String() != "1.1.1.1" {
		t.Fatalf("filtered = %#v, want only 1.1.1.1", filtered)
	}
}

func TestFilterLossRateAllowsHundredPercent(t *testing.T) {
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

	filtered := FilterPingLossRate(data, FilterConfig{MaxLossRate: MaxAllowedLossRate})
	if len(filtered) != 2 {
		t.Fatalf("filtered = %#v, want both IPs", filtered)
	}
}

func TestColoCountryName(t *testing.T) {
	for _, tc := range []struct{ colo, want string }{
		{"HKG", "中国香港"},
		{"hkg", "中国香港"},
		{"NRT", "日本"},
		{"SJC", "美国"},
		{"N/A", "N/A"},
		{"", "N/A"},
		{"ZZZ", "N/A"},
	} {
		if got := ColoCountryName(tc.colo); got != tc.want {
			t.Fatalf("ColoCountryName(%q) = %q, want %q", tc.colo, got, tc.want)
		}
	}
}

func TestExportCsvAppendAlignsLegacyRowWithoutCountryColumn(t *testing.T) {
	output := filepath.Join(t.TempDir(), "legacy.csv")
	legacy := "\xEF\xBB\xBFIP 地址,已发送,已接收,丢包率,TCP延迟(ms),平均速率(MB/s),最高速率(MB/s),地区码,追踪延迟(ms),备注\n" +
		"1.1.1.1,4,4,0.00,10.00,20.00,25.00,SJC,5.00,旧备注\n"
	if err := os.WriteFile(output, []byte(legacy), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	item := CloudflareIPData{
		PingData: &PingData{
			IP: parseCSVTestIP("104.16.9.9"), Sended: 4, Received: 4,
			Delay: 99 * time.Millisecond, Colo: "HKG",
		},
		DownloadSpeed: 50 * 1024 * 1024, MaxDownloadSpeed: 60 * 1024 * 1024,
		HeadDelay: 12 * time.Millisecond,
	}
	writer := CSVWriter{Path: output, Append: true, Encoding: CSVEncodingUTF8}
	if err := writer.ExportContext(context.Background(), []CloudflareIPData{item}); err != nil {
		t.Fatalf("ExportContext returned error: %v", err)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.HasPrefix(string(raw), "\xEF\xBB\xBF") {
		t.Fatalf("BOM was dropped when aligning legacy CSV")
	}
	lines := strings.Split(strings.TrimSpace(TrimUTF8BOM(string(raw))), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3\n%s", len(lines), string(raw))
	}
	if !strings.HasSuffix(lines[0], "地区码,国家/地区,追踪延迟(ms),备注") {
		t.Fatalf("header was not aligned:\n%s", lines[0])
	}
	if want := "1.1.1.1,4,4,0.00,10.00,20.00,25.00,SJC,美国,5.00,旧备注"; lines[1] != want {
		t.Fatalf("legacy row = %q, want %q", lines[1], want)
	}
	if want := "104.16.9.9,4,4,0.00,99.00,50.00,60.00,HKG,中国香港,12.00,"; lines[2] != want {
		t.Fatalf("appended row = %q, want %q", lines[2], want)
	}
}

func TestExportCsvAppendKeepsAlignedFileUntouched(t *testing.T) {
	output := filepath.Join(t.TempDir(), "result.csv")
	writer := CSVWriter{Path: output, Append: true, Encoding: CSVEncodingUTF8}
	if err := writer.ExportContext(context.Background(), []CloudflareIPData{csvTestData("1.1.1.1")}); err != nil {
		t.Fatalf("first ExportContext returned error: %v", err)
	}
	if err := writer.ExportContext(context.Background(), []CloudflareIPData{csvTestData("1.1.1.2")}); err != nil {
		t.Fatalf("second ExportContext returned error: %v", err)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3\n%s", len(lines), string(raw))
	}
	if strings.Count(lines[0], "国家/地区") != 1 {
		t.Fatalf("country column duplicated:\n%s", lines[0])
	}
	if !strings.Contains(lines[2], ",SJC,美国,") {
		t.Fatalf("appended row lost its country column:\n%s", lines[2])
	}
}

func TestAlignCSVCountryColumnSkipsAlignedFile(t *testing.T) {
	output := filepath.Join(t.TempDir(), "result.csv")
	writer := CSVWriter{Path: output, Encoding: CSVEncodingUTF8}
	if err := writer.ExportContext(context.Background(), []CloudflareIPData{csvTestData("1.1.1.1")}); err != nil {
		t.Fatalf("ExportContext returned error: %v", err)
	}
	before, err := os.Stat(output)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	rawBefore, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}

	if err := alignCSVCountryColumn(output); err != nil {
		t.Fatalf("alignCSVCountryColumn returned error: %v", err)
	}
	rawAfter, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	after, err := os.Stat(output)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if string(rawBefore) != string(rawAfter) {
		t.Fatalf("aligned file was rewritten:\n%s\n---\n%s", rawBefore, rawAfter)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("aligned file mtime changed: %v -> %v", before.ModTime(), after.ModTime())
	}
}

func TestAlignCSVCountryColumnLeavesUnparsableFileAlone(t *testing.T) {
	output := filepath.Join(t.TempDir(), "broken.csv")
	broken := []byte("\"unterminated quote,1,2\n")
	if err := os.WriteFile(output, broken, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := alignCSVCountryColumn(output); err != nil {
		t.Fatalf("alignCSVCountryColumn returned error: %v", err)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(raw) != string(broken) {
		t.Fatalf("unparsable file was modified: %q", raw)
	}
}
