package utils

import (
	"encoding/csv"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"time"
)

const (
	defaultOutput      = "result.csv"
	maxDelay           = 9999 * time.Millisecond
	minDelay           = 0 * time.Millisecond
	MaxAllowedLossRate = float32(0.15)
)

var (
	InputMaxDelay    = maxDelay
	InputMinDelay    = minDelay
	InputMaxLossRate = MaxAllowedLossRate
	Output           = defaultOutput
	OutputAppend     = false
	PrintNum         = 10
	Debug            = false // 是否开启调试模式
)

// 是否打印测试结果
func NoPrintResult() bool {
	return PrintNum == 0
}

// 是否输出到文件
func noOutput() bool {
	return Output == "" || Output == " "
}

type PingData struct {
	IP       *net.IPAddr
	Sended   int
	Received int
	Delay    time.Duration
	Colo     string
}

type CloudflareIPData struct {
	*PingData
	lossRate      float32
	HeadDelay     time.Duration
	DownloadSpeed float64
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

func ExportCsv(data []CloudflareIPData) error {
	if noOutput() || len(data) == 0 {
		return nil
	}
	flags := os.O_CREATE | os.O_WRONLY
	if OutputAppend {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	writeHeader := true
	if OutputAppend {
		if info, statErr := os.Stat(Output); statErr == nil && info.Size() > 0 {
			writeHeader = false
		}
	}
	fp, err := os.OpenFile(Output, flags, 0o644)
	if err != nil {
		return fmt.Errorf("创建文件[%s]失败：%w", Output, err)
	}
	defer fp.Close()
	w := csv.NewWriter(fp) //创建一个新的写入文件流
	if writeHeader {
		_ = w.Write([]string{"IP 地址", "已发送", "已接收", "丢包率", "TCP延迟(ms)", "下载速度(MB/s)", "地区码"})
	}
	_ = w.WriteAll(convertToString(data))
	w.Flush()
	return w.Error()
}

func convertToString(data []CloudflareIPData) [][]string {
	result := make([][]string, 0)
	for _, v := range data {
		result = append(result, v.toString())
	}
	return result
}

func SelectTopWeightedResults(data []CloudflareIPData, limit int) []CloudflareIPData {
	if len(data) <= 1 {
		return data
	}

	minDelay, maxDelay := data[0].Delay, data[0].Delay
	minSpeed, maxSpeed := data[0].DownloadSpeed, data[0].DownloadSpeed
	for _, item := range data[1:] {
		if item.Delay < minDelay {
			minDelay = item.Delay
		}
		if item.Delay > maxDelay {
			maxDelay = item.Delay
		}
		if item.DownloadSpeed < minSpeed {
			minSpeed = item.DownloadSpeed
		}
		if item.DownloadSpeed > maxSpeed {
			maxSpeed = item.DownloadSpeed
		}
	}

	type scoredResult struct {
		item  CloudflareIPData
		score float64
	}
	scored := make([]scoredResult, 0, len(data))
	for _, item := range data {
		delayScore := 1.0
		if maxDelay > minDelay {
			delayScore = float64(maxDelay-item.Delay) / float64(maxDelay-minDelay)
		}
		speedScore := 0.0
		if maxSpeed > minSpeed {
			speedScore = (item.DownloadSpeed - minSpeed) / (maxSpeed - minSpeed)
		} else if maxSpeed > 0 {
			speedScore = 1.0
		}
		scored = append(scored, scoredResult{
			item:  item,
			score: delayScore*0.3 + speedScore*0.7,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].item.DownloadSpeed != scored[j].item.DownloadSpeed {
			return scored[i].item.DownloadSpeed > scored[j].item.DownloadSpeed
		}
		if scored[i].item.Delay != scored[j].item.Delay {
			return scored[i].item.Delay < scored[j].item.Delay
		}
		iLossRate, jLossRate := scored[i].item.getLossRate(), scored[j].item.getLossRate()
		if iLossRate != jLossRate {
			return iLossRate < jLossRate
		}
		return scored[i].item.IP.String() < scored[j].item.IP.String()
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
func (s PingDelaySet) FilterDelay() (data PingDelaySet) {
	if InputMaxDelay > maxDelay || InputMinDelay < minDelay { // 当输入的延迟条件不在默认范围内时，不进行过滤
		return s
	}
	if InputMaxDelay == maxDelay && InputMinDelay == minDelay { // 当输入的延迟条件为默认值时，不进行过滤
		return s
	}
	for _, v := range s {
		if v.Delay > InputMaxDelay { // 平均延迟上限，延迟大于条件最大值时，后面的数据都不满足条件，直接跳出循环
			DebugEvent("stage.reject", map[string]any{
				"ip":      v.IP.String(),
				"message": "TCP 平均延迟超过上限，淘汰该 IP。",
				"reason":  "tcp_delay_above_limit",
				"stage":   "stage1_tcp",
				"tcp": map[string]any{
					"delay_ms":     v.Delay.Seconds() * 1000,
					"max_delay_ms": InputMaxDelay.Seconds() * 1000,
				},
			})
			break
		}
		if v.Delay < InputMinDelay { // 平均延迟下限，延迟小于条件最小值时，不满足条件，跳过
			DebugEvent("stage.reject", map[string]any{
				"ip":      v.IP.String(),
				"message": "TCP 平均延迟低于下限，淘汰该 IP。",
				"reason":  "tcp_delay_below_min",
				"stage":   "stage1_tcp",
				"tcp": map[string]any{
					"delay_ms":     v.Delay.Seconds() * 1000,
					"min_delay_ms": InputMinDelay.Seconds() * 1000,
				},
			})
			continue
		}
		data = append(data, v) // 延迟满足条件时，添加到新数组中
	}
	return
}

// 丢包条件过滤
func (s PingDelaySet) FilterLossRate() (data PingDelaySet) {
	maxLossRate := InputMaxLossRate
	if maxLossRate < 0 || maxLossRate > MaxAllowedLossRate {
		maxLossRate = MaxAllowedLossRate
	}
	for _, v := range s {
		lossRate := v.getLossRate()
		if lossRate > maxLossRate { // 丢包几率上限
			DebugEvent("stage.reject", map[string]any{
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

func (s DownloadSpeedSet) Print() {
	if NoPrintResult() {
		return
	}
	if len(s) <= 0 { // IP数组长度(IP数量) 大于 0 时继续
		fmt.Println("\n[信息] 完整测速结果 IP 数量为 0，跳过输出结果。")
		return
	}
	dateString := convertToString(s) // 转为多维数组 [][]String
	if len(dateString) < PrintNum {  // 如果IP数组长度(IP数量) 小于  打印次数，则次数改为IP数量
		PrintNum = len(dateString)
	}
	headFormat := "%-16s%-5s%-5s%-5s%-6s%-12s%-5s\n"
	dataFormat := "%-18s%-8s%-8s%-8s%-10s%-16s%-8s\n"
	for i := 0; i < PrintNum; i++ { // 如果要输出的 IP 中包含 IPv6，那么就需要调整一下间隔
		if len(dateString[i][0]) > 15 {
			headFormat = "%-40s%-5s%-5s%-5s%-6s%-12s%-5s\n"
			dataFormat = "%-42s%-8s%-8s%-8s%-10s%-16s%-8s\n"
			break
		}
	}
	Cyan.Printf(headFormat, "IP 地址", "已发送", "已接收", "丢包率", "平均延迟", "下载速度(MB/s)", "地区码")
	for i := 0; i < PrintNum; i++ {
		fmt.Printf(dataFormat, dateString[i][0], dateString[i][1], dateString[i][2], dateString[i][3], dateString[i][4], dateString[i][5], dateString[i][6])
	}
	if !noOutput() {
		fmt.Printf("\n完整测速结果已写入 %v 文件，可使用记事本/表格软件查看。\n", Output)
	}
}
