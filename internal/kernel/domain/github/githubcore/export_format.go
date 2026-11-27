package githubcore

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	configvalue "github.com/axuitomo/CFST-GUI/internal/kernel/domain/config"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/utils"
)

func ProbeRowsFromAny(value any) []probecore.ProbeRow {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var rows []probecore.ProbeRow
	if err := json.Unmarshal(raw, &rows); err == nil {
		rows = CompactProbeRows(rows)
		if len(rows) > 0 {
			return rows
		}
	}
	var generic []map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil
	}
	result := make([]probecore.ProbeRow, 0, len(generic))
	for _, row := range generic {
		ip := strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(row["ip"], row["address"]), ""))
		if ip == "" {
			continue
		}
		result = append(result, probecore.ProbeRow{
			Colo:               strings.TrimSpace(configvalue.String(row["colo"], "N/A")),
			DelayMS:            configvalue.Float(configvalue.FirstNonNil(row["delayMs"], row["delay_ms"], row["tcp_latency_ms"], row["tcpLatencyMs"]), 0),
			DownloadSpeedMB:    configvalue.Float(configvalue.FirstNonNil(row["downloadSpeedMb"], row["download_speed_mb"], row["download_mbps"], row["downloadMbps"]), 0),
			IP:                 ip,
			LossRate:           configvalue.Float(configvalue.FirstNonNil(row["lossRate"], row["loss_rate"]), 0),
			MaxDownloadSpeedMB: configvalue.Float(configvalue.FirstNonNil(row["maxDownloadSpeedMb"], row["max_download_speed_mb"], row["max_download_mbps"], row["maxDownloadMbps"]), 0),
			Received:           configvalue.Int(configvalue.FirstNonNil(row["received"], row["Received"]), 0),
			Sended:             configvalue.Int(configvalue.FirstNonNil(row["sended"], row["sent"], row["Sended"]), 0),
			SourcePort:         configvalue.Int(configvalue.FirstNonNil(row["source_port"], row["sourcePort"]), 0),
			TestPort:           configvalue.Int(configvalue.FirstNonNil(row["test_port"], row["testPort"]), 0),
			TraceDelayMS:       configvalue.Float(configvalue.FirstNonNil(row["traceDelayMs"], row["trace_delay_ms"], row["trace_latency_ms"], row["traceLatencyMs"]), 0),
		})
	}
	return result
}

func CompactProbeRows(rows []probecore.ProbeRow) []probecore.ProbeRow {
	result := rows[:0]
	for _, row := range rows {
		if strings.TrimSpace(row.IP) != "" {
			result = append(result, row)
		}
	}
	return result
}

func EncodeProbeRowsCSV(rows []probecore.ProbeRow) ([]byte, error) {
	return EncodeProbeRowsCSVWithEncoding(rows, utils.CSVEncodingUTF8)
}

func EncodeProbeRowsCSVWithEncoding(rows []probecore.ProbeRow, csvEncoding string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	if bom := utils.CSVEncodingBOM(csvEncoding); len(bom) > 0 {
		buffer.Write(bom)
	}
	writer := csv.NewWriter(buffer)
	if err := writer.Write([]string{"IP 地址", "已发送", "已接收", "丢包率", "TCP延迟(ms)", "平均速率(MB/s)", "最高速率(MB/s)", "地区码", "国家/地区", "追踪延迟(ms)", "备注"}); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if strings.TrimSpace(row.IP) == "" {
			continue
		}
		colo := strings.TrimSpace(row.Colo)
		if colo == "" {
			colo = "N/A"
		}
		record := []string{
			row.IP,
			strconv.Itoa(row.Sended),
			strconv.Itoa(row.Received),
			strconv.FormatFloat(row.LossRate, 'f', 2, 64),
			strconv.FormatFloat(row.DelayMS, 'f', 2, 64),
			strconv.FormatFloat(row.DownloadSpeedMB, 'f', 2, 64),
			strconv.FormatFloat(probeRowMaxDownloadSpeedMB(row), 'f', 2, 64),
			colo,
			utils.ColoCountryName(colo),
			strconv.FormatFloat(row.TraceDelayMS, 'f', 2, 64),
			strings.TrimSpace(row.Note),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

func NormalizeGitHubExportFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "txt", "text":
		return "txt"
	default:
		return "csv"
	}
}

func EncodeProbeRowsForGitHub(rows []probecore.ProbeRow, cfg Config) ([]byte, int, error) {
	rows = CompactProbeRows(rows)
	if len(rows) == 0 {
		return nil, 0, errors.New("没有可导出的有效测速结果行")
	}
	format := NormalizeGitHubExportFormat(cfg.Format)
	if format == "txt" {
		return encodeProbeRowsTXTWithTemplate(rows, cfg.TXTRowTemplate), len(rows), nil
	}
	if strings.TrimSpace(cfg.CSVHeaderTemplate) == "" && strings.TrimSpace(cfg.CSVRowTemplate) == "" {
		body, err := EncodeProbeRowsCSVWithEncoding(rows, cfg.CSVEncoding)
		return body, len(rows), err
	}
	return encodeProbeRowsCSVWithTemplate(rows, cfg.CSVHeaderTemplate, cfg.CSVRowTemplate), len(rows), nil
}

func encodeProbeRowsCSVWithTemplate(rows []probecore.ProbeRow, headerTemplate, rowTemplate string) []byte {
	lines := make([]string, 0, len(rows))
	if header := strings.TrimSpace(headerTemplate); header != "" {
		lines = append(lines, header)
	}
	rowTemplate = strings.TrimSpace(rowTemplate)
	if rowTemplate == "" {
		rowTemplate = "{ip},{sended},{received},{loss_rate},{tcp_latency_ms},{download_mbps},{max_download_mbps},{colo},{trace_latency_ms},{source_port},{test_port}"
	}
	for index, row := range rows {
		lines = append(lines, renderProbeRowTemplate(rowTemplate, row, index+1))
	}
	return []byte(strings.Join(lines, "\n"))
}

func encodeProbeRowsTXTWithTemplate(rows []probecore.ProbeRow, rowTemplate string) []byte {
	rowTemplate = strings.TrimSpace(rowTemplate)
	if rowTemplate == "" {
		rowTemplate = "{ip}"
	}
	lines := make([]string, 0, len(rows))
	for index, row := range rows {
		lines = append(lines, renderProbeRowTemplate(rowTemplate, row, index+1))
	}
	return []byte(strings.Join(lines, "\n"))
}

func renderProbeRowTemplate(template string, row probecore.ProbeRow, index int) string {
	replacements := map[string]string{
		"{index}":             strconv.Itoa(index),
		"{ip}":                row.IP,
		"{colo}":              rowColo(row),
		"{sended}":            strconv.Itoa(row.Sended),
		"{received}":          strconv.Itoa(row.Received),
		"{loss_rate}":         formatMetric(row.LossRate),
		"{tcp_latency_ms}":    formatMetric(row.DelayMS),
		"{trace_latency_ms}":  formatMetric(row.TraceDelayMS),
		"{download_mbps}":     formatMetric(row.DownloadSpeedMB),
		"{max_download_mbps}": formatMetric(probeRowMaxDownloadSpeedMB(row)),
		"{source_port}":       formatOptionalInt(row.SourcePort),
		"{test_port}":         formatOptionalInt(row.TestPort),
		"{note}":              strings.TrimSpace(row.Note),
	}
	for key, value := range replacements {
		template = strings.ReplaceAll(template, key, value)
	}
	return template
}

func CountCSVDataRows(raw []byte) int {
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) <= 1 {
		return 0
	}
	count := 0
	for _, record := range records[1:] {
		if len(record) > 0 && strings.TrimSpace(record[0]) != "" {
			count++
		}
	}
	return count
}

func ExportCSVTargetFileName(payload map[string]any, targetValue string, fallback string) string {
	if fileName := SanitizeTemplateFileName(configvalue.String(configvalue.FirstNonNil(payload["file_name"], payload["fileName"]), "")); fileName != "" {
		return fileName
	}
	targetValue = strings.TrimSpace(targetValue)
	if strings.HasPrefix(targetValue, "browser-download:") {
		if fileName := SanitizeTemplateFileName(strings.TrimPrefix(targetValue, "browser-download:")); fileName != "" {
			return fileName
		}
	}
	if targetValue != "" {
		if fileName := SanitizeTemplateFileName(filepath.Base(targetValue)); fileName != "" && fileName != "." {
			return fileName
		}
		if parsed, err := url.Parse(targetValue); err == nil {
			if fileName := SanitizeTemplateFileName(path.Base(parsed.Path)); fileName != "" && fileName != "." {
				return fileName
			}
		}
	}
	return fallback
}

func SanitizeTemplateFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return strings.TrimSpace(replacer.Replace(value))
}

func RenderTemplate(template string, taskID string, now time.Time) string {
	if template == "" {
		template = DefaultPathTemplate
	}
	values := map[string]string{
		"{date}":      now.Format("2006-01-02"),
		"{time}":      now.Format("150405"),
		"{task_id}":   sanitizePathPart(taskID),
		"{taskId}":    sanitizePathPart(taskID),
		"{timestamp}": now.Format("20060102-150405"),
	}
	result := template
	for key, value := range values {
		result = strings.ReplaceAll(result, key, value)
	}
	result = strings.ReplaceAll(result, "\\", "/")
	result = path.Clean(strings.TrimLeft(result, "/"))
	if result == "." || strings.HasPrefix(result, "../") || result == ".." {
		return DefaultPathTemplate
	}
	return result
}

func EscapeContentPath(targetPath string) string {
	parts := strings.Split(strings.Trim(targetPath, "/"), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func probeRowMaxDownloadSpeedMB(row probecore.ProbeRow) float64 {
	if row.MaxDownloadSpeedMB > 0 {
		return row.MaxDownloadSpeedMB
	}
	return row.DownloadSpeedMB
}

func rowColo(row probecore.ProbeRow) string {
	colo := strings.TrimSpace(row.Colo)
	if colo == "" {
		return ""
	}
	return colo
}

func formatMetric(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func formatOptionalInt(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func sanitizePathPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "manual"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-")
	return replacer.Replace(value)
}
