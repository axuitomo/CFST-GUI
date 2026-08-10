package utils

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultOutput      = "result.csv"
	DefaultMaxDelay    = 9999 * time.Millisecond
	DefaultMinDelay    = 0 * time.Millisecond
	DefaultPrintNum    = 10
	DefaultMaxLossRate = float32(0.15)
	MaxAllowedLossRate = float32(1)

	DownloadSpeedMetricAverage = "average"
	DownloadSpeedMetricMax     = "max"

	CSVEncodingUTF8    = "utf-8"
	CSVEncodingUTF8BOM = "utf-8-bom"
	utf8BOM            = "\xEF\xBB\xBF"
)

type PingData struct {
	IP       *net.IPAddr
	Sended   int
	Received int
	Delay    time.Duration
	Colo     string
}

type CloudflareIPData struct {
	*PingData
	lossRate         float32
	HeadDelay        time.Duration
	DownloadSpeed    float64
	MaxDownloadSpeed float64
}

type FilterConfig struct {
	MaxDelay    time.Duration
	MinDelay    time.Duration
	MaxLossRate float32
	DebugEvent  func(event string, payload map[string]any)
}

type CSVWriter struct {
	Path     string
	Append   bool
	Encoding string
}

// 计算丢包率
func (cf *CloudflareIPData) getLossRate() float32 {
	if cf.Sended <= 0 {
		return 1
	}
	pingLost := cf.Sended - cf.Received
	if pingLost < 0 {
		pingLost = 0
	}
	cf.lossRate = float32(pingLost) / float32(cf.Sended)
	return cf.lossRate
}

func (cf *CloudflareIPData) toString() []string {
	result := make([]string, 7)
	result[0] = cf.IP.String()
	result[1] = strconv.Itoa(cf.Sended)
	result[2] = strconv.Itoa(cf.Received)
	result[3] = strconv.FormatFloat(float64(cf.getLossRate()), 'f', 2, 32)
	result[4] = strconv.FormatFloat(cf.Delay.Seconds()*1000, 'f', 2, 32)
	result[5] = strconv.FormatFloat(cf.DownloadSpeed/1024/1024, 'f', 2, 32)
	// 如果 Colo 为空，则使用 "N/A" 表示
	if cf.Colo == "" {
		result[6] = "N/A"
	} else {
		result[6] = cf.Colo
	}
	return result
}

func (cf *CloudflareIPData) toCSVString() []string {
	result := make([]string, 8)
	result[0] = cf.IP.String()
	result[1] = strconv.Itoa(cf.Sended)
	result[2] = strconv.Itoa(cf.Received)
	result[3] = strconv.FormatFloat(float64(cf.getLossRate()), 'f', 2, 32)
	result[4] = strconv.FormatFloat(cf.Delay.Seconds()*1000, 'f', 2, 32)
	result[5] = strconv.FormatFloat(cf.DownloadSpeed/1024/1024, 'f', 2, 32)
	result[6] = strconv.FormatFloat(cf.maxDownloadSpeed()/1024/1024, 'f', 2, 32)
	if cf.Colo == "" {
		result[7] = "N/A"
	} else {
		result[7] = cf.Colo
	}
	return result
}

func (cf *CloudflareIPData) maxDownloadSpeed() float64 {
	if cf.MaxDownloadSpeed > 0 {
		return cf.MaxDownloadSpeed
	}
	if cf.DownloadSpeed > 0 {
		return cf.DownloadSpeed
	}
	return 0
}

func NormalizeDownloadSpeedMetric(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DownloadSpeedMetricMax, "peak", "highest":
		return DownloadSpeedMetricMax
	case DownloadSpeedMetricAverage, "avg", "mean":
		return DownloadSpeedMetricAverage
	default:
		return DownloadSpeedMetricAverage
	}
}

func NormalizeCSVEncoding(value string) string {
	switch normalizedCSVEncodingKey(value) {
	case "", "utf8", "utf-8":
		return CSVEncodingUTF8
	case "utf8-bom", "utf-8-bom", "utf8-with-bom", "utf-8-with-bom", "utf-8-sig", "bom":
		return CSVEncodingUTF8BOM
	default:
		return CSVEncodingUTF8
	}
}

func IsKnownCSVEncoding(value string) bool {
	switch normalizedCSVEncodingKey(value) {
	case "", "utf8", "utf-8", "utf8-bom", "utf-8-bom", "utf8-with-bom", "utf-8-with-bom", "utf-8-sig", "bom":
		return true
	default:
		return false
	}
}

func CSVEncodingBOM(value string) []byte {
	if NormalizeCSVEncoding(value) != CSVEncodingUTF8BOM {
		return nil
	}
	return []byte(utf8BOM)
}

func TrimUTF8BOM(value string) string {
	return strings.TrimPrefix(value, "\ufeff")
}

func normalizedCSVEncodingKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, " ", "-")
	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}
	return normalized
}

func DownloadSpeedForMetric(item CloudflareIPData, metric string) float64 {
	if NormalizeDownloadSpeedMetric(metric) == DownloadSpeedMetricMax {
		return item.maxDownloadSpeed()
	}
	if item.DownloadSpeed > 0 {
		return item.DownloadSpeed
	}
	return 0
}

func (writer CSVWriter) ExportContext(ctx context.Context, data []CloudflareIPData, checkpoints ...func()) error {
	if strings.TrimSpace(writer.Path) == "" || len(data) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := os.O_CREATE | os.O_WRONLY
	if writer.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	writeHeader := true
	if writer.Append {
		if info, statErr := os.Stat(writer.Path); statErr == nil && info.Size() > 0 {
			writeHeader = false
		}
	}
	raw, err := encodeCSVContextWithEncoding(ctx, data, writeHeader, writer.Encoding, checkpoints)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	fp, err := os.OpenFile(writer.Path, flags, 0o644)
	if err != nil {
		return fmt.Errorf("创建文件[%s]失败：%w", writer.Path, err)
	}
	defer fp.Close()
	if _, err := fp.Write(raw); err != nil {
		return fmt.Errorf("写入 CSV 失败：%w", err)
	}
	return nil
}

func encodeCSVContextWithEncoding(ctx context.Context, data []CloudflareIPData, writeHeader bool, encoding string, checkpoints []func()) ([]byte, error) {
	var buffer bytes.Buffer
	if writeHeader {
		buffer.Write(CSVEncodingBOM(encoding))
	}
	w := csv.NewWriter(&buffer)
	if writeHeader {
		if err := w.Write([]string{"IP 地址", "已发送", "已接收", "丢包率", "TCP延迟(ms)", "平均速率(MB/s)", "最高速率(MB/s)", "地区码"}); err != nil {
			return nil, err
		}
	}
	for _, item := range data {
		for _, checkpoint := range checkpoints {
			if checkpoint != nil {
				checkpoint()
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := w.Write(item.toCSVString()); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func convertToCSVString(data []CloudflareIPData) [][]string {
	result := make([][]string, 0)
	for _, v := range data {
		result = append(result, v.toCSVString())
	}
	return result
}

func convertToString(data []CloudflareIPData) [][]string {
	result := make([][]string, 0)
	for _, v := range data {
		result = append(result, v.toString())
	}
	return result
}

func SelectTopWeightedResults(data []CloudflareIPData, limit int) []CloudflareIPData {
	return SelectTopWeightedResultsByMetric(data, limit, DownloadSpeedMetricAverage)
}

func SelectTopWeightedResultsByMetric(data []CloudflareIPData, limit int, metric string) []CloudflareIPData {
	if len(data) <= 1 {
		return data
	}

	minDelay, maxDelay := data[0].Delay, data[0].Delay
	firstSpeed := DownloadSpeedForMetric(data[0], metric)
	minSpeed, maxSpeed := firstSpeed, firstSpeed
	for _, item := range data[1:] {
		if item.Delay < minDelay {
			minDelay = item.Delay
		}
		if item.Delay > maxDelay {
			maxDelay = item.Delay
		}
		itemSpeed := DownloadSpeedForMetric(item, metric)
		if itemSpeed < minSpeed {
			minSpeed = itemSpeed
		}
		if itemSpeed > maxSpeed {
			maxSpeed = itemSpeed
		}
	}

	type scoredResult struct {
		item  CloudflareIPData
		score float64
	}
	scored := make([]scoredResult, 0, len(data))
	for _, item := range data {
		itemSpeed := DownloadSpeedForMetric(item, metric)
		delayScore := 1.0
		if maxDelay > minDelay {
			delayScore = float64(maxDelay-item.Delay) / float64(maxDelay-minDelay)
		}
		speedScore := 0.0
		if maxSpeed > minSpeed {
			speedScore = (itemSpeed - minSpeed) / (maxSpeed - minSpeed)
		} else if maxSpeed > 0 {
			speedScore = 1.0
		}
		scored = append(scored, scoredResult{
			item:  item,
			score: delayScore*0.3 + speedScore*0.7,
		})
	}

	slices.SortStableFunc(scored, func(a, b scoredResult) int {
		if a.score != b.score {
			if a.score > b.score {
				return -1
			}
			return 1
		}
		aSpeed, bSpeed := DownloadSpeedForMetric(a.item, metric), DownloadSpeedForMetric(b.item, metric)
		if aSpeed != bSpeed {
			if aSpeed > bSpeed {
				return -1
			}
			return 1
		}
		if a.item.Delay != b.item.Delay {
			if a.item.Delay < b.item.Delay {
				return -1
			}
			return 1
		}
		aLossRate, bLossRate := a.item.getLossRate(), b.item.getLossRate()
		if aLossRate != bLossRate {
			if aLossRate < bLossRate {
				return -1
			}
			return 1
		}
		return strings.Compare(a.item.IP.String(), b.item.IP.String())
	})

	selectedLimit := len(scored)
	if limit > 0 && limit < selectedLimit {
		selectedLimit = limit
	}
	selected := make([]CloudflareIPData, 0, selectedLimit)
	for _, item := range scored[:selectedLimit] {
		selected = append(selected, item.item)
	}
	return selected
}

// 延迟丢包排序
type PingDelaySet []CloudflareIPData

// 延迟条件过滤
func FilterPingDelay(s PingDelaySet, config FilterConfig) (data PingDelaySet) {
	if config.DebugEvent == nil {
		config.DebugEvent = func(string, map[string]any) {}
	}
	if config.MaxDelay > DefaultMaxDelay || config.MinDelay < DefaultMinDelay { // 当输入的延迟条件不在默认范围内时，不进行过滤
		return s
	}
	if config.MaxDelay == DefaultMaxDelay && config.MinDelay == DefaultMinDelay { // 当输入的延迟条件为默认值时，不进行过滤
		return s
	}
	for _, v := range s {
		if v.Delay > config.MaxDelay { // 平均延迟上限，延迟大于条件最大值时，后面的数据都不满足条件，直接跳出循环
			config.DebugEvent("stage.reject", map[string]any{
				"ip":      v.IP.String(),
				"message": "TCP 平均延迟超过上限，淘汰该 IP。",
				"reason":  "tcp_delay_above_limit",
				"stage":   "stage1_tcp",
				"tcp": map[string]any{
					"delay_ms":     v.Delay.Seconds() * 1000,
					"max_delay_ms": config.MaxDelay.Seconds() * 1000,
				},
			})
			break
		}
		if v.Delay < config.MinDelay { // 平均延迟下限，延迟小于条件最小值时，不满足条件，跳过
			config.DebugEvent("stage.reject", map[string]any{
				"ip":      v.IP.String(),
				"message": "TCP 平均延迟低于下限，淘汰该 IP。",
				"reason":  "tcp_delay_below_min",
				"stage":   "stage1_tcp",
				"tcp": map[string]any{
					"delay_ms":     v.Delay.Seconds() * 1000,
					"min_delay_ms": config.MinDelay.Seconds() * 1000,
				},
			})
			continue
		}
		data = append(data, v) // 延迟满足条件时，添加到新数组中
	}
	return
}

// 丢包条件过滤
func FilterPingLossRate(s PingDelaySet, config FilterConfig) (data PingDelaySet) {
	if config.DebugEvent == nil {
		config.DebugEvent = func(string, map[string]any) {}
	}
	maxLossRate := config.MaxLossRate
	if maxLossRate < 0 || maxLossRate > MaxAllowedLossRate {
		maxLossRate = MaxAllowedLossRate
	}
	for _, v := range s {
		lossRate := v.getLossRate()
		if lossRate > maxLossRate { // 丢包几率上限
			config.DebugEvent("stage.reject", map[string]any{
				"ip":      v.IP.String(),
				"message": "TCP 丢包率超过上限，淘汰该 IP。",
				"reason":  "tcp_loss_above_limit",
				"stage":   "stage1_tcp",
				"tcp": map[string]any{
					"loss_rate":     lossRate,
					"max_loss_rate": maxLossRate,
					"received":      v.Received,
					"sent":          v.Sended,
				},
			})
			continue
		}
		data = append(data, v) // 丢包率满足条件时，添加到新数组中
	}
	return
}

func (s PingDelaySet) Len() int {
	return len(s)
}
func (s PingDelaySet) Less(i, j int) bool {
	iRate, jRate := s[i].getLossRate(), s[j].getLossRate()
	if iRate != jRate {
		return iRate < jRate
	}
	return s[i].Delay < s[j].Delay
}
func (s PingDelaySet) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// 下载速度排序
type DownloadSpeedSet []CloudflareIPData

func (s DownloadSpeedSet) Len() int {
	return len(s)
}
func (s DownloadSpeedSet) Less(i, j int) bool {
	return s[i].DownloadSpeed > s[j].DownloadSpeed
}
func (s DownloadSpeedSet) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s DownloadSpeedSet) PrintLimit(limit int, outputPath string) {
	if limit == 0 {
		return
	}
	if len(s) <= 0 { // IP数组长度(IP数量) 大于 0 时继续
		fmt.Println("\n[信息] 完整测速结果 IP 数量为 0，跳过输出结果。")
		return
	}
	dateString := convertToString(s)          // 转为多维数组 [][]String
	if limit < 0 || len(dateString) < limit { // 如果IP数组长度(IP数量) 小于打印次数，则次数改为IP数量
		limit = len(dateString)
	}
	headFormat := "%-16s%-5s%-5s%-5s%-6s%-12s%-5s\n"
	dataFormat := "%-18s%-8s%-8s%-8s%-10s%-16s%-8s\n"
	for i := 0; i < limit; i++ { // 如果要输出的 IP 中包含 IPv6，那么就需要调整一下间隔
		if len(dateString[i][0]) > 15 {
			headFormat = "%-40s%-5s%-5s%-5s%-6s%-12s%-5s\n"
			dataFormat = "%-42s%-8s%-8s%-8s%-10s%-16s%-8s\n"
			break
		}
	}
	Cyan.Printf(headFormat, "IP 地址", "已发送", "已接收", "丢包率", "平均延迟", "下载速度(MB/s)", "地区码")
	for i := 0; i < limit; i++ {
		fmt.Printf(dataFormat, dateString[i][0], dateString[i][1], dateString[i][2], dateString[i][3], dateString[i][4], dateString[i][5], dateString[i][6])
	}
	if strings.TrimSpace(outputPath) != "" {
		fmt.Printf("\n完整测速结果已写入 %v 文件，可使用记事本/表格软件查看。\n", outputPath)
	}
}
