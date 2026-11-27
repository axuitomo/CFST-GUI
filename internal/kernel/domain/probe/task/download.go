// Copyright (C) 2026 axuitomo (modifications)
// SPDX-License-Identifier: GPL-3.0-only
// Derived from XIU2/CloudflareSpeedTest task/download.go; see THIRD-PARTY-NOTICES.md.

package task

import (
	"errors"
	"net"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/utils"
)

const (
	defaultURL                         = "https://speed.cloudflare.com/__down?bytes=99999999"
	defaultTimeout                     = 4 * time.Second
	defaultDisableDownload             = false
	defaultDownloadRoutines            = 1
	defaultDownloadGetConcurrency      = 4
	defaultDownloadBufferKB            = 256
	defaultDownloadHTTPProtocol        = string(httpclient.ProtocolAuto)
	defaultDownloadSpeedSampleInterval = time.Second
	defaultDownloadWarmupDuration      = time.Second
	// downloadSpeedWindow 是实时速率的滑动窗口：单次采样间隔内的字节数会被缓冲和重连打散，
	// 用它做除数会得到 0 和尖峰交替的读数，因此固定按最近一个窗口的字节数计算。
	downloadSpeedWindow = time.Second
	// MaxDownloadRoutines caps how many IPs download at once. The default stays 1
	// so a measurement is not shared with other downloads, but the ceiling allows
	// trading accuracy for wall-clock time on large lists.
	MaxDownloadRoutines               = 16
	MaxDownloadGetConcurrency         = 32
	MinDownloadBufferKB               = 64
	MaxDownloadBufferKB               = 4096
	downloadContentSniffBytes         = 512
	defaultMinSpeed           float64 = 0.0
	// downloadEdgeRestrictedSniffBytes 是判定 403 正文时读取的字节上限。品牌错误页里
	// "errorCode: 1034" 落在 1 KB 之后（实测 offset 1042），因此取 8 KB；追踪阶段不受此限，
	// 仍读完整正文。
	downloadEdgeRestrictedSniffBytes = 8 * 1024
	downloadReasonEdgeRestricted     = "edge_ip_restricted"
	downloadReasonForbidden          = "forbidden"
	// EdgeNote403 / EdgeNote1034 是日志里标记边缘拒绝类型的短标识。
	EdgeNote403  = "403"
	EdgeNote1034 = "1034"
)

// downloadEdgeRestrictedMarkers 是 Cloudflare"边缘 IP 受限"403 正文的标记，大小写不敏感：
// /cdn-cgi/trace 回 17 字节纯文本 "error code: 1034"；浏览器式请求（带 Accept: text/html）
// 回品牌错误页，页面里没有"error code: 1034"这个字面量，只有 "Edge IP Restricted"（标题与
// 正文）和 JS 里的 "errorCode: 1034"。
var downloadEdgeRestrictedMarkers = []string{
	"error code: 1034",
	"errorcode: 1034",
	"edge ip restricted",
}

var errDownloadInterrupted = errors.New("download interrupted")

type downloadResult struct {
	speed            float64
	maxSpeed         float64
	colo             string
	validMeasurement bool
	ignored          bool
	retryable        bool
	reason           string
	retryAfter       time.Duration
	bytesRead        int64
	measuredBytes    int64
	measuredElapsed  time.Duration
}

func invalidDownloadResult(reason string, retryable bool) downloadResult {
	return downloadResult{
		retryable: retryable,
		reason:    reason,
	}
}

// ignoredDownloadResult 表示这颗 IP 被边缘拒绝之类的原因跳过：不重试，也不计入连续失败冷却。
func ignoredDownloadResult(reason string) downloadResult {
	result := invalidDownloadResult(reason, false)
	result.ignored = true
	return result
}

func invalidDownloadResultWithRetryAfter(reason string, retryable bool, retryAfter time.Duration) downloadResult {
	result := invalidDownloadResult(reason, retryable)
	result.retryAfter = retryAfter
	return result
}

func validDownloadResult(speed, maxSpeed float64, colo string, bytesRead, measuredBytes int64, measuredElapsed time.Duration) downloadResult {
	if speed < 0 {
		speed = 0
	}
	if maxSpeed < speed {
		// 滑动窗口峰值可能低于整段平均值（例如窗口跨过预热边界），最高速率不应小于平均速率。
		maxSpeed = speed
	}
	return downloadResult{
		speed:            speed,
		maxSpeed:         maxSpeed,
		colo:             colo,
		validMeasurement: true,
		reason:           "download_measured",
		bytesRead:        bytesRead,
		measuredBytes:    measuredBytes,
		measuredElapsed:  measuredElapsed,
	}
}

func (e *Engine) TestDownloadSpeed(ipSet utils.PingDelaySet) (speedSet utils.DownloadSpeedSet) {
	config := e.config
	if config.Disable {
		return utils.DownloadSpeedSet(ipSet)
	}
	if len(ipSet) <= 0 { // IP 数组长度(IP数量) 大于 0 时才会继续下载测速
		utils.Yellow.Println("[信息] 延迟测速结果 IP 数量为 0，跳过下载测速。")
		return
	}
	testNum := len(ipSet)
	utils.Cyan.Printf("开始下载测速（下限：%.2f MB/s, 数量：全部 %d, 并发线程：%d）\n", config.MinSpeed, testNum, config.DownloadRoutines)
	// 控制 下载测速进度条 与 延迟测速进度条 长度一致（强迫症）
	bar_a := len(strconv.Itoa(len(ipSet)))
	bar_b := "     "
	for i := 0; i < bar_a; i++ {
		bar_b += " "
	}
	bar := utils.NewBar(testNum, bar_b, "")
	results := make([]utils.CloudflareIPData, testNum)
	qualified := make([]bool, testNum)
	control := make(chan struct{}, config.DownloadRoutines)
	var wg sync.WaitGroup
	var processedCount atomic.Int32
	var qualifiedCount atomic.Int32

	for i := 0; i < testNum; i++ {
		if config.DownloadSuccessLimit > 0 && int(qualifiedCount.Load()) >= config.DownloadSuccessLimit {
			break
		}
		e.checkPause("stage3_get", ipSet[i].IP.String())
		if e.isCanceled("stage3_get", ipSet[i].IP.String()) {
			break
		}
		control <- struct{}{}
		if config.DownloadSuccessLimit > 0 && int(qualifiedCount.Load()) >= config.DownloadSuccessLimit {
			<-control
			break
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			defer func() { <-control }()

			item := ipSet[index]
			ipText := item.IP.String()
			defer e.recoverWorkerPanic("stage3_get", ipText, "download_worker_panic")
			setWorkerLabels("stage3_get", ipText)
			e.checkPause("stage3_get", ipText)
			if e.isCanceled("stage3_get", ipText) {
				return
			}
			result := e.runDownloadHandlerWithRetry(item.IP)
			if e.isCanceled("stage3_get", item.IP.String()) {
				return
			}
			item.DownloadSpeed = result.speed
			item.MaxDownloadSpeed = result.maxSpeed
			if item.Colo == "" { // 只有当 Colo 是空的时候，才写入，否则代表追踪阶段已经获取过了
				item.Colo = result.colo
			}
			thresholdSpeed := utils.DownloadSpeedForMetric(item, config.MinSpeedMetric)
			isQualified := result.validMeasurement && thresholdSpeed >= config.MinSpeed*1024*1024
			if !isQualified {
				// 阶段汇总的淘汰原因按候选项计一次：重试链的中间原因不算，这颗 IP 若走
				// 403/1034 分支会由 noteEdgeRejected 记账，这里跳过避免重复计数。
				if !result.ignored {
					e.noteStageReject("stage3_get", downloadResultReason(result, false))
				}
			}
			if isQualified {
				results[index] = item
				qualified[index] = true
				qualifiedCount.Add(1)
			}
			// 被屏蔽的 403/1034 是明确跳过，不计入连续失败冷却；不屏蔽的按普通失败计账。
			if !result.ignored {
				// budget 传 0：阶段 3 的失败全是候选侧问题（速度不够 / 边缘拒绝 / 连接被重置），
				// 冷却在这里既救不了远端也省不了时间，只按候选侧记账。
				e.noteStageProbeOutcome("stage3_get", item.IP.String(), classifyProbeOutcome(isQualified, 0, 0))
			}
			if e.debugEnabled() {
				e.debugEvent("stage.detail", map[string]any{
					"colo": item.Colo,
					"get": map[string]any{
						"bytes_read":           result.bytesRead,
						"concurrency":          config.DownloadRoutines,
						"duration_ms":          config.Timeout.Milliseconds(),
						"get_concurrency":      config.DownloadGetConcurrency,
						"measured_bytes":       result.measuredBytes,
						"measured_elapsed_ms":  result.measuredElapsed.Milliseconds(),
						"min_speed_mb_s":       config.MinSpeed,
						"min_speed_metric":     utils.NormalizeDownloadSpeedMetric(config.MinSpeedMetric),
						"max_speed_mb_s":       result.maxSpeed / 1024 / 1024,
						"protocol":             config.DownloadHTTPProtocol,
						"qualified":            isQualified,
						"sequence":             index + 1,
						"speed_mb_s":           result.speed / 1024 / 1024,
						"threshold_speed_mb_s": thresholdSpeed / 1024 / 1024,
						"total":                testNum,
						"valid_measurement":    result.validMeasurement,
					},
					"ip":      item.IP.String(),
					"message": "文件测速完成。",
					"reason":  downloadResultReason(result, isQualified),
					"stage":   "stage3_get",
				})
			}
			processed := processedCount.Add(1)
			currentQualified := qualifiedCount.Load()
			bar.Grow(1, strconv.Itoa(int(currentQualified)))
			if e.hooks.DownloadProgress != nil {
				e.hooks.DownloadProgress(int(processed), int(currentQualified), testNum)
			}
		}(i)
	}
	wg.Wait()
	bar.Done()
	for index, item := range results {
		if qualified[index] {
			speedSet = append(speedSet, item)
		}
	}
	// 按速度排序
	sort.Sort(speedSet)
	return
}

func (e *Engine) runDownloadHandlerWithRetry(ip *net.IPAddr) downloadResult {
	var result downloadResult
	stage := "stage3_get"
	ipText := ip.String()
	for attempt := 1; attempt <= e.retryAttemptLimit(); attempt++ {
		e.checkPause(stage, ipText)
		if e.isCanceled(stage, ipText) {
			return result
		}
		if e.downloadHandlerResult != nil {
			result = e.downloadHandlerResult(e, ip)
		} else if e.downloadHandler != nil {
			speed, colo := e.downloadHandler(e, ip)
			result = downloadResult{
				speed:            speed,
				maxSpeed:         speed,
				colo:             colo,
				validMeasurement: true,
				reason:           "download_measured",
			}
		} else {
			result = e.downloadHandlerAttempt(ip)
		}
		if e.isCanceled(stage, ipText) {
			return result
		}
		if result.validMeasurement || !result.retryable {
			return result
		}
		if attempt < e.retryAttemptLimit() {
			if result.reason == "rate_limited" {
				e.sleepBeforeRateLimitRetry("stage3_get", ip.String(), attempt, result.retryAfter)
			} else {
				e.sleepBeforeRetry("stage3_get", ip.String(), attempt)
			}
		}
	}
	return result
}

func downloadResultReason(result downloadResult, qualified bool) string {
	if qualified {
		return "download_qualified"
	}
	if !result.validMeasurement {
		if result.reason != "" {
			return result.reason
		}
		return "download_invalid"
	}
	return "download_speed_below_min"
}
