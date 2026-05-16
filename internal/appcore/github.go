package appcore

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/githubcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

type GitHubExportConfig = githubcore.Config
type GitHubExportResult = githubcore.ExportResult
type GitHubContentsResponse = githubcore.ContentsResponse
type GitHubPutContentsResponse = githubcore.PutContentsResponse
type GitHubContentsPutRequest = githubcore.ContentsPutRequest
type GitHubExportClient = githubcore.Client

func GitHubExportConfigFromPayload(payload map[string]any, defaults githubcore.ConfigDefaults) (GitHubExportConfig, []string, error) {
	return githubcore.ParseConfigFromPayload(payload, defaults)
}

func GitHubExportConfigFromSnapshot(config map[string]any, defaults githubcore.ConfigDefaults) (GitHubExportConfig, []string, error) {
	return githubcore.ParseConfigFromSnapshot(config, defaults)
}

func GitHubCSVEncodingFromPayload(payload map[string]any) string {
	return githubcore.CSVEncodingFromPayload(payload)
}

func GitHubExportCSVTargetFileName(payload map[string]any, targetValue string, fallback string) string {
	return githubcore.ExportCSVTargetFileName(payload, targetValue, fallback)
}

func GitHubExportConfigDefaults(owner, repo string) githubcore.ConfigDefaults {
	return githubcore.ConfigDefaults{
		Owner: owner,
		Repo:  repo,
	}
}

func ProbeRowsFromAny(value any) []probecore.ProbeRow {
	return githubcore.ProbeRowsFromAny(value)
}

func CompactProbeRows(rows []probecore.ProbeRow) []probecore.ProbeRow {
	return githubcore.CompactProbeRows(rows)
}

func EncodeProbeRowsCSV(rows []probecore.ProbeRow) ([]byte, error) {
	return githubcore.EncodeProbeRowsCSV(rows)
}

func EncodeProbeRowsCSVWithEncoding(rows []probecore.ProbeRow, csvEncoding string) ([]byte, error) {
	return githubcore.EncodeProbeRowsCSVWithEncoding(rows, csvEncoding)
}

func EncodeProbeRowsForGitHub(rows []probecore.ProbeRow, cfg GitHubExportConfig) ([]byte, int, error) {
	return githubcore.EncodeProbeRowsForGitHub(rows, cfg)
}

func CountCSVDataRows(raw []byte) int {
	return githubcore.CountCSVDataRows(raw)
}

func ReadProbeRowsForGitHubFromCSV(path string) ([]probecore.ProbeRow, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("结果文件路径为空")
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
		return nil, errors.New("结果文件为空")
	}
	header := csvHeaderIndex(records[0])
	if len(header) == 0 {
		return nil, errors.New("结果文件缺少表头")
	}
	result := make([]probecore.ProbeRow, 0, len(records)-1)
	for _, record := range records[1:] {
		address := csvField(record, header, "IP 地址", "ip", "address")
		if strings.TrimSpace(address) == "" {
			continue
		}
		result = append(result, probecore.ProbeRow{
			Colo:               strings.TrimSpace(csvField(record, header, "地区码", "colo")),
			DelayMS:            valueOrDefaultFloat(csvFloatPtr(csvField(record, header, "TCP延迟(ms)", "平均延迟", "delayMs", "tcp_latency_ms"))),
			DownloadSpeedMB:    valueOrDefaultFloat(csvFloatPtr(csvField(record, header, "平均速率(MB/s)", "下载速度(MB/s)", "downloadSpeedMb", "download_mbps"))),
			IP:                 strings.TrimSpace(address),
			MaxDownloadSpeedMB: valueOrDefaultFloat(csvFloatPtr(csvField(record, header, "最高速率(MB/s)", "maxDownloadSpeedMb", "max_download_mbps", "maxDownloadMbps"))),
			SourcePort:         valueOrDefaultInt(csvIntPtr(csvField(record, header, "输入源端口", "source_port", "sourcePort"))),
			TestPort:           valueOrDefaultInt(csvIntPtr(csvField(record, header, "测试端口", "实际测速端口", "test_port", "testPort"))),
			TraceDelayMS:       valueOrDefaultFloat(csvFloatPtr(csvField(record, header, "追踪延迟(ms)", "traceDelayMs", "trace_latency_ms"))),
		})
	}
	if len(result) == 0 {
		return nil, errors.New("结果 CSV 没有可导出的数据行")
	}
	return result, nil
}

func ExportCSVToGitHub(ctx context.Context, cfg GitHubExportConfig, taskID string, body []byte, rowCount int, now time.Time, client *GitHubExportClient) (GitHubExportResult, error) {
	return githubcore.ExportCSV(ctx, client, cfg, taskID, body, rowCount, now)
}

func NewGitHubExportClient(token string, baseURL string) *GitHubExportClient {
	return githubcore.NewClientWithOptions(githubcore.ClientOptions{
		BaseURL: baseURL,
		Token:   token,
	})
}

func RenderGitHubTemplate(template string, taskID string, now time.Time) string {
	return githubcore.RenderTemplate(template, taskID, now)
}

func EscapeGitHubContentPath(targetPath string) string {
	return githubcore.EscapeContentPath(targetPath)
}

func csvHeaderIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, name := range header {
		key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))
		key = strings.ReplaceAll(key, " ", "")
		index[key] = i
	}
	return index
}

func csvField(record []string, header map[string]int, names ...string) string {
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		key = strings.ReplaceAll(key, " ", "")
		if index, ok := header[key]; ok && index >= 0 && index < len(record) {
			return record[index]
		}
	}
	return ""
}

func csvFloatPtr(value string) *float64 {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "N/A") {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return nil
	}
	return &parsed
}

func csvIntPtr(value string) *int {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "N/A") {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return nil
	}
	return &parsed
}

func stringPtrOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "N/A") {
		return nil
	}
	return &value
}

func valueOrDefaultString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func valueOrDefaultFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func valueOrDefaultInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
