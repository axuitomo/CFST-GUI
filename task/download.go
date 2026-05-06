package task

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/utils"
)

const (
	bufferSize                                 = 64 * 1024
	defaultURL                                 = "https://speed.cloudflare.com/__down?bytes=10000000"
	defaultTimeout                             = 10 * time.Second
	defaultDisableDownload                     = false
	defaultTestNum                             = 10
	defaultDownloadRoutines                    = 1
	defaultDownloadSpeedSampleInterval         = 500 * time.Millisecond
	defaultDownloadWarmupDuration              = 5 * time.Second
	downloadReadDeadlineTick                   = 250 * time.Millisecond
	MaxDownloadRoutines                        = 1
	defaultMinSpeed                    float64 = 0.0
)

var (
	URL     = defaultURL
	Timeout = defaultTimeout
	Disable = defaultDisableDownload

	TestCount                   = defaultTestNum
	MinSpeed                    = defaultMinSpeed
	DownloadRoutines            = defaultDownloadRoutines
	DownloadSpeedSampleInterval = defaultDownloadSpeedSampleInterval
	DownloadWarmupDuration      = defaultDownloadWarmupDuration

	downloadHandlerFunc func(*net.IPAddr) (float64, string)
)

var errDownloadInterrupted = errors.New("download interrupted")

type downloadResult struct {
	speed            float64
	colo             string
	validMeasurement bool
	retryable        bool
	reason           string
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

func validDownloadResult(speed float64, colo string, bytesRead, measuredBytes int64, measuredElapsed time.Duration) downloadResult {
	if speed < 0 {
		speed = 0
	}
	return downloadResult{
		speed:            speed,
		colo:             colo,
		validMeasurement: true,
		reason:           "download_measured",
		bytesRead:        bytesRead,
		measuredBytes:    measuredBytes,
		measuredElapsed:  measuredElapsed,
	}
}

func checkDownloadDefault() {
	if URL == "" {
		URL = defaultURL
	}
	if Timeout <= 0 {
		Timeout = defaultTimeout
	}
	if TestCount <= 0 {
		TestCount = defaultTestNum
	}
	if MinSpeed <= 0.0 {
		MinSpeed = defaultMinSpeed
	}
	if DownloadRoutines <= 0 {
		DownloadRoutines = defaultDownloadRoutines
	}
	if DownloadRoutines > MaxDownloadRoutines {
		DownloadRoutines = MaxDownloadRoutines
	}
	if DownloadSpeedSampleInterval <= 0 {
		DownloadSpeedSampleInterval = defaultDownloadSpeedSampleInterval
	}
	if DownloadWarmupDuration < 0 {
		DownloadWarmupDuration = defaultDownloadWarmupDuration
	}
}

func TestDownloadSpeed(ipSet utils.PingDelaySet) (speedSet utils.DownloadSpeedSet) {
	checkDownloadDefault()
	if Disable {
		return utils.DownloadSpeedSet(ipSet)
	}
	if len(ipSet) <= 0 { // IP 数组长度(IP数量) 大于 0 时才会继续下载测速
		utils.Yellow.Println("[信息] 延迟测速结果 IP 数量为 0，跳过下载测速。")
		return
	}
	testNum := len(ipSet)
	utils.Cyan.Printf("开始下载测速（下限：%.2f MB/s, 数量：全部 %d, 并发线程：%d）\n", MinSpeed, testNum, DownloadRoutines)
	// 控制 下载测速进度条 与 延迟测速进度条 长度一致（强迫症）
	bar_a := len(strconv.Itoa(len(ipSet)))
	bar_b := "     "
	for i := 0; i < bar_a; i++ {
		bar_b += " "
	}
	bar := utils.NewBar(testNum, bar_b, "")
	results := make([]utils.CloudflareIPData, testNum)
	qualified := make([]bool, testNum)
	control := make(chan struct{}, DownloadRoutines)
	var wg sync.WaitGroup
	var processedCount atomic.Int32
	var qualifiedCount atomic.Int32

	for i := 0; i < testNum; i++ {
		CheckProbePause("stage3_get", ipSet[i].IP.String())
		wg.Add(1)
		control <- struct{}{}
		go func(index int) {
			defer wg.Done()
			defer func() { <-control }()

			item := ipSet[index]
			CheckProbePause("stage3_get", item.IP.String())
			result := runDownloadHandlerWithRetry(item.IP)
			item.DownloadSpeed = result.speed
			if item.Colo == "" { // 只有当 Colo 是空的时候，才写入，否则代表之前是 httping 测速并获取过了
				item.Colo = result.colo
			}
			isQualified := result.validMeasurement && result.speed >= MinSpeed*1024*1024
			if isQualified {
				results[index] = item
				qualified[index] = true
				qualifiedCount.Add(1)
			}
			noteStageProbeOutcome("stage3_get", item.IP.String(), isQualified)
			utils.DebugEvent("stage.detail", map[string]any{
				"colo": item.Colo,
				"get": map[string]any{
					"bytes_read":          result.bytesRead,
					"concurrency":         DownloadRoutines,
					"duration_ms":         Timeout.Milliseconds(),
					"measured_bytes":      result.measuredBytes,
					"measured_elapsed_ms": result.measuredElapsed.Milliseconds(),
					"min_speed_mb_s":      MinSpeed,
					"qualified":           isQualified,
					"sequence":            index + 1,
					"speed_mb_s":          result.speed / 1024 / 1024,
					"total":               testNum,
					"valid_measurement":   result.validMeasurement,
				},
				"ip":      item.IP.String(),
				"message": "文件测速完成。",
				"reason":  downloadResultReason(result, isQualified),
				"stage":   "stage3_get",
			})
			processed := processedCount.Add(1)
			currentQualified := qualifiedCount.Load()
			bar.Grow(1, strconv.Itoa(int(currentQualified)))
			if DownloadProgressHook != nil {
				DownloadProgressHook(int(processed), int(currentQualified), testNum)
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

func runDownloadHandlerWithRetry(ip *net.IPAddr) downloadResult {
	var result downloadResult
	for attempt := 1; attempt <= retryAttemptLimit(); attempt++ {
		CheckProbePause("stage3_get", ip.String())
		if downloadHandlerFunc != nil {
			speed, colo := downloadHandlerFunc(ip)
			result = downloadResult{
				speed:            speed,
				colo:             colo,
				validMeasurement: true,
				reason:           "download_measured",
			}
		} else {
			result = downloadHandlerAttempt(ip)
		}
		if result.validMeasurement || !result.retryable {
			return result
		}
		if attempt < retryAttemptLimit() {
			sleepBeforeRetry("stage3_get", ip.String(), attempt)
		}
	}
	return result
}

// 统一的请求报错调试输出
func printDownloadDebugInfo(ip *net.IPAddr, err error, statusCode int, url, lastRedirectURL string, response *http.Response) {
	finalURL := url // 默认的最终 URL，这样当 response 为空时也能输出
	if lastRedirectURL != "" {
		finalURL = lastRedirectURL // 如果 lastRedirectURL 不是空，说明重定向过，优先输出最后一次要重定向至的目标
	} else if response != nil && response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String() // 如果 response 不为 nil，且 Request 和 URL 都不为 nil，则获取最后一次成功的响应地址
	}
	if url != finalURL { // 如果 URL 和最终地址不一致，说明有重定向，是该重定向后的地址引起的错误
		if statusCode > 0 { // 如果状态码大于 0，说明是后续 HTTP 状态码引起的错误
			utils.DebugEvent("stage.reject", downloadDebugFields(ip, nil, statusCode, url, finalURL, "status_mismatch", "文件测速状态码不匹配，淘汰该 IP。"))
		} else {
			utils.DebugEvent("stage.reject", downloadDebugFields(ip, err, statusCode, url, finalURL, "get_error", "文件测速请求失败，淘汰该 IP。"))
		}
	} else { // 如果 URL 和最终地址一致，说明没有重定向
		if statusCode > 0 { // 如果状态码大于 0，说明是后续 HTTP 状态码引起的错误
			utils.DebugEvent("stage.reject", downloadDebugFields(ip, nil, statusCode, url, "", "status_mismatch", "文件测速状态码不匹配，淘汰该 IP。"))
		} else {
			utils.DebugEvent("stage.reject", downloadDebugFields(ip, err, statusCode, url, "", "get_error", "文件测速请求失败，淘汰该 IP。"))
		}
	}
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

func downloadDebugFields(ip *net.IPAddr, err error, statusCode int, url, finalURL, reason, message string) map[string]any {
	fields := map[string]any{
		"get": map[string]any{
			"final_url":   finalURL,
			"status_code": statusCode,
			"url":         url,
		},
		"ip":      ip.String(),
		"message": message,
		"reason":  reason,
		"stage":   "stage3_get",
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	return fields
}

func downloadHandler(ip *net.IPAddr) (float64, string) {
	result := downloadHandlerAttempt(ip)
	return result.speed, result.colo
}

func downloadHandlerAttempt(ip *net.IPAddr) downloadResult {
	attempt := 1
	for {
		result, err := downloadHandlerAttemptOnce(ip, attempt)
		if !errors.Is(err, errDownloadInterrupted) {
			return result
		}
		CheckProbePause("stage3_get", ip.String())
		attempt++
	}
}

func downloadHandlerAttemptOnce(ip *net.IPAddr, attempt int) (downloadResult, error) {
	var lastRedirectURL string // 用于记录最后一次重定向目标，以便在访问错误时输出
	profile := currentRequestProfile()
	tlsConfig := &tls.Config{InsecureSkipVerify: profile.InsecureSkipVerify}
	if profile.HasCustomSNI() {
		tlsConfig.ServerName = profile.SNI
	}
	var responseConn net.Conn
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           captureDialContext(getDialContext(ip, profile), &responseConn),
			TLSClientConfig:       tlsConfig,
			TLSHandshakeTimeout:   TCPConnectTimeout,
			ResponseHeaderTimeout: Timeout,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			lastRedirectURL = req.URL.String() // 记录每次重定向的目标，以便在访问错误时输出
			profile.Apply(req)
			if len(via) > 10 { // 限制最多重定向 10 次
				utils.DebugEvent("stage.reject", downloadDebugFields(ip, nil, 0, URL, req.URL.String(), "too_many_redirects", "文件测速重定向次数过多，淘汰该 IP。"))
				return http.ErrUseLastResponse
			}
			if req.Header.Get("Referer") == defaultURL { // 当使用默认下载测速地址时，重定向不携带 Referer
				req.Header.Del("Referer")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
	if err != nil {
		utils.DebugEvent("stage.reject", downloadDebugFields(ip, err, 0, URL, "", "request_create_failed", "文件测速请求创建失败，淘汰该 IP。"))
		return invalidDownloadResult("request_create_failed", false), nil
	}

	profile.Apply(req)

	var clearDownloadInterrupt func()
	if DownloadInterruptHook != nil {
		clearDownloadInterrupt = DownloadInterruptHook("stage3_get", ip.String(), cancel)
	}
	if clearDownloadInterrupt != nil {
		defer clearDownloadInterrupt()
	}

	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return invalidDownloadResult("download_interrupted", true), errDownloadInterrupted
		}
		if utils.Debug { // 调试模式下，输出更多信息
			printDownloadDebugInfo(ip, err, 0, URL, lastRedirectURL, response)
		}
		return invalidDownloadResult("get_error", true), nil
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		if utils.Debug { // 调试模式下，输出更多信息
			printDownloadDebugInfo(ip, nil, response.StatusCode, URL, lastRedirectURL, response)
		}
		return invalidDownloadResult("status_mismatch", true), nil
	}

	// 通过头部参数获取地区码
	colo := getHeaderColo(response.Header)

	timeStart := time.Now()
	contentLength := response.ContentLength
	buffer := make([]byte, bufferSize)

	var (
		contentRead         int64
		measuredRead        int64
		lastSampleRead      int64
		lastSampleAt        time.Duration
		nextSampleAt        = DownloadSpeedSampleInterval
		lastSampleElapsedMS int64
		activeElapsed       time.Duration
		lastActiveTick      = timeStart
		measuredStartedAt   time.Duration
		transferComplete    = contentLength == 0
	)

	measuredBytes := func(elapsed time.Duration, allowFallback bool) (int64, time.Duration) {
		if measuredRead <= 0 {
			if allowFallback && contentRead > 0 && elapsed > 0 {
				return contentRead, elapsed
			}
			return 0, 0
		}
		if elapsed < DownloadWarmupDuration {
			return 0, 0
		}
		start := measuredStartedAt
		if start <= 0 {
			start = DownloadWarmupDuration
		}
		return measuredRead, elapsed - start
	}

	snapshotMeasurement := func(elapsed time.Duration, force bool) (float64, bool, float64, bool, int64, int64, time.Duration, int64, int64) {
		sampleElapsed := elapsed - lastSampleAt
		if sampleElapsed < 0 {
			sampleElapsed = 0
		}
		currentReady := sampleElapsed > 0
		if force && (transferComplete || elapsed > 0) {
			currentReady = true
		}
		currentSpeed := 0.0
		if sampleElapsed > 0 {
			currentSpeed = float64(contentRead-lastSampleRead) / sampleElapsed.Seconds()
		}
		averageBytes, averageElapsed := measuredBytes(elapsed, force && transferComplete)
		averageSpeed := averageDownloadSpeed(averageBytes, averageElapsed)
		hasBodySignal := contentRead > 0 || transferComplete
		averageReady := hasBodySignal && averageElapsed > 0
		if hasBodySignal && elapsed >= DownloadWarmupDuration && elapsed > 0 {
			averageReady = true
		}
		if force && transferComplete && (contentRead == 0 || averageElapsed > 0) {
			averageReady = true
		}
		return currentSpeed / 1024 / 1024, currentReady, averageSpeed / 1024 / 1024, averageReady, contentRead, elapsed.Milliseconds(), averageElapsed, averageBytes, sampleElapsed.Milliseconds()
	}

	emitSample := func(elapsed time.Duration, force bool) {
		if elapsed < 0 || (!force && elapsed <= 0) {
			return
		}
		elapsedMS := elapsed.Milliseconds()
		if !force && elapsedMS == lastSampleElapsedMS {
			return
		}
		currentSpeed, currentReady, averageSpeed, averageReady, bytesRead, elapsedMS, measuredElapsed, measuredBytesValue, sampleElapsedMS := snapshotMeasurement(elapsed, force)
		if DownloadSpeedSampleHook != nil {
			DownloadSpeedSampleHook(DownloadSpeedSample{
				Stage:             DownloadSpeedSampleStage,
				IP:                ip.String(),
				CurrentSpeedMBs:   currentSpeed,
				CurrentReady:      currentReady,
				AverageSpeedMBs:   averageSpeed,
				AverageReady:      averageReady,
				BodyRead:          bytesRead > 0,
				BytesRead:         bytesRead,
				ElapsedMS:         elapsedMS,
				Colo:              colo,
				SampleBytes:       bytesRead - lastSampleRead,
				SampleElapsedMS:   sampleElapsedMS,
				MeasuredBytes:     measuredBytesValue,
				MeasuredElapsedMS: measuredElapsed.Milliseconds(),
				TransferComplete:  transferComplete,
				Attempt:           attempt,
			})
		}
		lastSampleRead = contentRead
		lastSampleAt = elapsed
		lastSampleElapsedMS = elapsedMS
	}

	emitSample(0, true)
	for contentLength != contentRead {
		pauseCheckAt := time.Now()
		if pauseCheckAt.After(lastActiveTick) {
			activeElapsed += pauseCheckAt.Sub(lastActiveTick)
		}
		CheckProbePause("stage3_get", ip.String())
		lastActiveTick = time.Now()
		if activeElapsed >= Timeout {
			break
		}
		if responseConn != nil {
			_ = responseConn.SetReadDeadline(time.Now().Add(downloadReadDeadlineTick))
		}
		readStartedElapsed := activeElapsed
		readStartedAt := time.Now()
		bufferRead, err := response.Body.Read(buffer)
		readFinishedAt := time.Now()
		if readFinishedAt.After(readStartedAt) {
			activeElapsed += readFinishedAt.Sub(readStartedAt)
		}
		lastActiveTick = readFinishedAt
		if err != nil && ctx.Err() != nil {
			return invalidDownloadResult("download_interrupted", true), errDownloadInterrupted
		}
		if bufferRead > 0 {
			contentRead += int64(bufferRead)
			if activeElapsed >= DownloadWarmupDuration {
				if measuredStartedAt <= 0 {
					measuredStartedAt = readStartedElapsed
					if measuredStartedAt < DownloadWarmupDuration {
						measuredStartedAt = DownloadWarmupDuration
					}
				}
				measuredRead += int64(bufferRead)
			}
			if contentLength >= 0 && contentRead >= contentLength {
				transferComplete = true
			}
		}
		if isNetTimeout(err) {
			if activeElapsed >= nextSampleAt {
				emitSample(activeElapsed, false)
				for nextSampleAt <= activeElapsed {
					nextSampleAt += DownloadSpeedSampleInterval
				}
			}
			continue
		}
		if activeElapsed >= nextSampleAt {
			emitSample(activeElapsed, false)
			for nextSampleAt <= activeElapsed {
				nextSampleAt += DownloadSpeedSampleInterval
			}
		}
		if err != nil {
			if err != io.EOF {
				break
			}
			transferComplete = contentLength < 0 || contentRead >= contentLength
			break
		}
	}

	elapsed := activeElapsed
	averageBytes, averageElapsed := measuredBytes(elapsed, transferComplete)
	speed := averageDownloadSpeed(averageBytes, averageElapsed)
	emitSample(elapsed, true)
	result := validDownloadResult(speed, colo, contentRead, averageBytes, averageElapsed)
	if contentLength > 0 && contentRead == 0 && !transferComplete {
		return invalidDownloadResult("download_no_body_read", true), nil
	}
	return result, nil
}

func captureDialContext(dialContext func(ctx context.Context, network, address string) (net.Conn, error), captured *net.Conn) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dialContext(ctx, network, address)
		if err == nil && captured != nil {
			*captured = conn
		}
		return conn, err
	}
}

func isNetTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func averageDownloadSpeed(bytesRead int64, elapsed time.Duration) float64 {
	if bytesRead <= 0 || elapsed <= 0 {
		return 0
	}
	return float64(bytesRead) / elapsed.Seconds()
}
