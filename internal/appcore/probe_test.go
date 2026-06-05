package appcore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadProbeResultRowsFromCSVHandlesBOMHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.csv")
	raw := "\xEF\xBB\xBFIP 地址,已发送,已接收,丢包率,TCP延迟(ms),平均速率(MB/s),最高速率(MB/s),地区码,追踪延迟(ms)\n1.1.1.1,3,3,0.00,12.34,56.78,78.90,HKG,34.56\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	rows, err := ReadProbeResultRowsFromCSV(path)
	if err != nil {
		t.Fatalf("ReadProbeResultRowsFromCSV returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Address != "1.1.1.1" {
		t.Fatalf("Address = %q, want 1.1.1.1", rows[0].Address)
	}
	if rows[0].Colo == nil || *rows[0].Colo != "HKG" {
		t.Fatalf("Colo = %#v, want HKG", rows[0].Colo)
	}
}

func TestReadProbeRowsForGitHubFromCSVParsesRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.csv")
	raw := "\xEF\xBB\xBFIP 地址,已发送,已接收,丢包率,TCP延迟(ms),平均速率(MB/s),最高速率(MB/s),地区码,追踪延迟(ms),输入源端口,测试端口\n1.1.1.1,4,4,0.00,12.34,56.78,78.90,HKG,34.56,8443,443\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	rows, err := ReadProbeRowsForGitHubFromCSV(path)
	if err != nil {
		t.Fatalf("ReadProbeRowsForGitHubFromCSV returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.IP != "1.1.1.1" || row.Colo != "HKG" {
		t.Fatalf("row = %#v, want parsed ip/colo", row)
	}
	if row.DelayMS != 12.34 || row.DownloadSpeedMB != 56.78 || row.MaxDownloadSpeedMB != 78.90 {
		t.Fatalf("row metrics = %#v", row)
	}
	if row.SourcePort != 8443 || row.TestPort != 443 || row.TraceDelayMS != 34.56 {
		t.Fatalf("row ports/trace = %#v", row)
	}
}

func TestFilterSortProbeResultRowsFiltersIPVersionAndSorts(t *testing.T) {
	rows := []ProbeResultRow{
		{Address: "2606:4700:4700::1111", DownloadMbps: probeTestFloatPtr(20), ExportStatus: "exported", StageStatus: "completed"},
		{Address: "1.1.1.1", DownloadMbps: probeTestFloatPtr(10), ExportStatus: "exported", StageStatus: "completed"},
		{Address: "2.2.2.2", DownloadMbps: probeTestFloatPtr(30), ExportStatus: "pending", StageStatus: "completed"},
	}

	filtered := FilterSortProbeResultRows(rows, "download", "desc", "exported", "ipv4")
	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1: %#v", len(filtered), filtered)
	}
	if filtered[0].Address != "1.1.1.1" {
		t.Fatalf("filtered[0].Address = %q, want 1.1.1.1", filtered[0].Address)
	}
}

func TestPaginateProbeResultRowsReturnsCopyWindow(t *testing.T) {
	rows := []ProbeResultRow{
		{Address: "1.1.1.1"},
		{Address: "2.2.2.2"},
		{Address: "3.3.3.3"},
	}

	paged := PaginateProbeResultRows(rows, 1, 1)
	if len(paged) != 1 || paged[0].Address != "2.2.2.2" {
		t.Fatalf("paged = %#v, want only 2.2.2.2", paged)
	}

	paged[0].Address = "changed"
	if rows[1].Address != "2.2.2.2" {
		t.Fatalf("source row changed to %q, want copy window", rows[1].Address)
	}
}

func probeTestFloatPtr(value float64) *float64 {
	return &value
}
