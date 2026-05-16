package appcore

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
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
