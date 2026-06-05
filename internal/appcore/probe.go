package appcore

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

func ReadProbeResultRowsFromCSV(path string) ([]ProbeResultRow, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("结果文件路径为空。")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("读取结果文件失败：%w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析结果 CSV 失败：%w", err)
	}
	if len(records) == 0 {
		return nil, errors.New("结果文件为空。")
	}
	header := csvHeaderIndex(records[0])
	if len(header) == 0 {
		return nil, errors.New("结果文件缺少表头。")
	}

	rows := make([]ProbeResultRow, 0, len(records)-1)
	for _, record := range records[1:] {
		address := csvField(record, header, "IP 地址", "ip", "address")
		if strings.TrimSpace(address) == "" {
			continue
		}
		downloadSpeed := csvFloatPtr(csvField(record, header, "平均速率(MB/s)", "下载速度(MB/s)", "downloadSpeedMb", "download_mbps"))
		maxDownloadSpeed := csvFloatPtr(csvField(record, header, "最高速率(MB/s)", "maxDownloadSpeedMb", "max_download_mbps", "maxDownloadMbps"))
		if maxDownloadSpeed == nil {
			maxDownloadSpeed = downloadSpeed
		}
		rows = append(rows, ProbeResultRow{
			Address:         strings.TrimSpace(address),
			Colo:            stringPtrOrNil(strings.TrimSpace(csvField(record, header, "地区码", "colo"))),
			DownloadMbps:    downloadSpeed,
			ExportStatus:    "exported",
			StageStatus:     "completed",
			MaxDownloadMbps: maxDownloadSpeed,
			SourcePort:      csvIntPtr(csvField(record, header, "输入源端口", "source_port", "sourcePort")),
			TCPLatencyMS:    csvFloatPtr(csvField(record, header, "TCP延迟(ms)", "平均延迟", "delayMs", "tcp_latency_ms")),
			TestPort:        csvIntPtr(csvField(record, header, "测试端口", "实际测速端口", "test_port", "testPort")),
			TraceLatencyMS:  csvFloatPtr(csvField(record, header, "追踪延迟(ms)", "traceDelayMs", "trace_latency_ms")),
		})
	}
	if len(rows) == 0 {
		return nil, errors.New("结果文件没有可读取的结果行。")
	}
	return rows, nil
}

func FilterSortProbeResultRows(rows []ProbeResultRow, sortBy, order, filter, ipFilter string) []ProbeResultRow {
	filtered := make([]ProbeResultRow, 0, len(rows))
	for _, row := range rows {
		if !matchesProbeResultFilter(row, filter) {
			continue
		}
		if !matchesProbeIPFilter(row, ipFilter) {
			continue
		}
		filtered = append(filtered, row)
	}

	desc := strings.EqualFold(strings.TrimSpace(order), "desc")
	sort.SliceStable(filtered, func(i, j int) bool {
		compare := compareProbeResultRows(filtered[i], filtered[j], sortBy)
		if desc {
			return compare > 0
		}
		return compare < 0
	})
	return filtered
}

func PaginateProbeResultRows(rows []ProbeResultRow, limit, offset int) []ProbeResultRow {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		return append([]ProbeResultRow(nil), rows...)
	}
	if offset >= len(rows) {
		return []ProbeResultRow{}
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return append([]ProbeResultRow(nil), rows[offset:end]...)
}

func matchesProbeResultFilter(row ProbeResultRow, filter string) bool {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "exported":
		return row.ExportStatus == "exported"
	case "failed":
		return row.StageStatus == "failed" || (row.LastErrorCode != nil && strings.TrimSpace(*row.LastErrorCode) != "")
	case "pending":
		return row.ExportStatus != "exported" && row.StageStatus != "failed"
	default:
		return true
	}
}

func matchesProbeIPFilter(row ProbeResultRow, ipFilter string) bool {
	address := strings.TrimSpace(row.Address)
	switch strings.ToLower(strings.TrimSpace(ipFilter)) {
	case "ipv4":
		return strings.Count(address, ".") == 3 && !strings.Contains(address, ":")
	case "ipv6":
		return strings.Contains(address, ":")
	default:
		return true
	}
}

func compareProbeResultRows(left, right ProbeResultRow, sortBy string) int {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "stage":
		return strings.Compare(left.StageStatus, right.StageStatus)
	case "tcp":
		return compareFloat64(probeResultNumber(left.TCPLatencyMS, 1<<30), probeResultNumber(right.TCPLatencyMS, 1<<30))
	case "trace":
		return compareFloat64(probeResultNumber(left.TraceLatencyMS, 1<<30), probeResultNumber(right.TraceLatencyMS, 1<<30))
	case "download":
		return compareFloat64(probeResultNumber(left.DownloadMbps, -1), probeResultNumber(right.DownloadMbps, -1))
	case "max_download":
		return compareFloat64(probeResultNumber(left.MaxDownloadMbps, -1), probeResultNumber(right.MaxDownloadMbps, -1))
	case "export_status":
		return strings.Compare(left.ExportStatus, right.ExportStatus)
	default:
		return strings.Compare(left.Address, right.Address)
	}
}

func probeResultNumber(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func compareFloat64(left, right float64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}
