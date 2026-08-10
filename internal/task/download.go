package task

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const (
	defaultURL                                 = "https://speedtest.xyz9923.dpdns.org/500m"
	defaultTimeout                             = 4 * time.Second
	defaultDisableDownload                     = false
	defaultTestNum                             = 10
	defaultDownloadRoutines                    = 1
	defaultDownloadGetConcurrency              = 4
	defaultDownloadBufferKB                    = 256
	defaultDownloadHTTPProtocol                = string(httpclient.ProtocolAuto)
	defaultDownloadSpeedSampleInterval         = 500 * time.Millisecond
	defaultDownloadWarmupDuration              = time.Second
	MaxDownloadRoutines                        = 1
	MaxDownloadGetConcurrency                  = 32
	MinDownloadBufferKB                        = 64
	MaxDownloadBufferKB                        = 4096
	defaultMinSpeed                    float64 = 0.0
)

var errDownloadInterrupted = errors.New("download interrupted")

type downloadResult struct {
	speed            float64
	maxSpeed         float64
	colo             string
	validMeasurement bool
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

func invalidDownloadResultWithRetryAfter(reason string, retryable bool, retryAfter time.Duration) downloadResult {
	result := invalidDownloadResult(reason, retryable)
	result.retryAfter = retryAfter
	return result
}

func validDownloadResult(speed, maxSpeed float64, colo string, bytesRead, measuredBytes int64, measuredElapsed time.Duration) downloadResult {
	if speed < 0 {
		speed = 0
	}
	if maxSpeed <= 0 {
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
		e.checkPause("stage3_get", ipSet[i].IP.String())
		wg.Add(1)
		control <- struct{}{}
		go func(index int) {
			defer wg.Done()
			defer func() { <-control }()

			item := ipSet[index]
			e.checkPause("stage3_get", item.IP.String())
			result := e.runDownloadHandlerWithRetry(item.IP)
			item.DownloadSpeed = result.speed
			item.MaxDownloadSpeed = result.maxSpeed
			if item.Colo == "" { // 只有当 Colo 是空的时候，才写入，否则代表之前是 httping 测速并获取过了
				item.Colo = result.colo
			}
			thresholdSpeed := utils.DownloadSpeedForMetric(item, config.MinSpeedMetric)
			isQualified := result.validMeasurement && thresholdSpeed >= config.MinSpeed*1024*1024
			if isQualified {
				results[index] = item
				qualified[index] = true
				qualifiedCount.Add(1)
			}
			e.noteStageProbeOutcome("stage3_get", item.IP.String(), isQualified)
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

// 统一的请求报错调试输出
func (e *Engine) printDownloadDebugInfo(ip *net.IPAddr, err error, statusCode int, url, lastRedirectURL string, response *http.Response) {
	finalURL := url // 默认的最终 URL，这样当 response 为空时也能输出
	if lastRedirectURL != "" {
		finalURL = lastRedirectURL // 如果 lastRedirectURL 不是空，说明重定向过，优先输出最后一次要重定向至的目标
	} else if response != nil && response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String() // 如果 response 不为 nil，且 Request 和 URL 都不为 nil，则获取最后一次成功的响应地址
	}
	if url != finalURL { // 如果 URL 和最终地址不一致，说明有重定向，是该重定向后的地址引起的错误
		if statusCode > 0 { // 如果状态码大于 0，说明是后续 HTTP 状态码引起的错误
			e.debugEvent("stage.reject", downloadDebugFields(ip, nil, statusCode, url, finalURL, "status_mismatch", "文件测速状态码不匹配，淘汰该 IP。"))
		} else {
			e.debugEvent("stage.reject", downloadDebugFields(ip, err, statusCode, url, finalURL, "get_error", "文件测速请求失败，淘汰该 IP。"))
		}
	} else { // 如果 URL 和最终地址一致，说明没有重定向
		if statusCode > 0 { // 如果状态码大于 0，说明是后续 HTTP 状态码引起的错误
			e.debugEvent("stage.reject", downloadDebugFields(ip, nil, statusCode, url, "", "status_mismatch", "文件测速状态码不匹配，淘汰该 IP。"))
		} else {
			e.debugEvent("stage.reject", downloadDebugFields(ip, err, statusCode, url, "", "get_error", "文件测速请求失败，淘汰该 IP。"))
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

func (e *Engine) downloadHandlerAttempt(ip *net.IPAddr) downloadResult {
	attempt := 1
	stage := "stage3_get"
	ipText := ip.String()
	for {
		result, err := e.downloadHandlerAttemptOnce(ip, attempt)
		if !errors.Is(err, errDownloadInterrupted) {
			return result
		}
		if e.isCanceled(stage, ipText) {
			return result
		}
		e.checkPause(stage, ipText)
		if e.isCanceled(stage, ipText) {
			return result
		}
		attempt++
	}
}

func (e *Engine) downloadHandlerAttemptOnce(ip *net.IPAddr, attempt int) (downloadResult, error) {
	config := e.config
	profile := e.currentRequestProfile()
	ctx, cancelTimeout := context.WithTimeout(e.context(), config.Timeout)
	defer cancelTimeout()
	var interrupted atomic.Bool
	interrupt := func() {
		interrupted.Store(true)
		cancelTimeout()
	}

	var clearDownloadInterrupt func()
	if e.hooks.DownloadInterrupt != nil {
		clearDownloadInterrupt = e.hooks.DownloadInterrupt("stage3_get", ip.String(), interrupt)
	}
	if clearDownloadInterrupt != nil {
		defer clearDownloadInterrupt()
	}

	var lastRedirectURL string // 用于记录最后一次重定向目标，以便在访问错误时输出
	client := httpclient.NewClient(httpclient.Options{
		Protocol:              httpclient.NormalizeProtocol(config.DownloadHTTPProtocol, httpclient.ProtocolAuto),
		Profile:               profile,
		DialContext:           httpclient.DirectDialContext(ip, config.TCPPort, profile),
		DialAddress:           profile.DialAddress(ip, config.TCPPort),
		DisableProxy:          true,
		ResponseHeaderTimeout: config.Timeout,
		TLSHandshakeTimeout:   config.TCPConnectTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			lastRedirectURL = req.URL.String() // 记录每次重定向的目标，以便在访问错误时输出
			profile.Apply(req)
			httpclient.ApplyNoCache(req)
			if len(via) > 10 { // 限制最多重定向 10 次
				e.debugEvent("stage.reject", downloadDebugFields(ip, nil, 0, config.URL, req.URL.String(), "too_many_redirects", "文件测速重定向次数过多，淘汰该 IP。"))
				return http.ErrUseLastResponse
			}
			if req.Header.Get("Referer") == defaultURL { // 当使用默认下载测速地址时，重定向不携带 Referer
				req.Header.Del("Referer")
			}
			return nil
		},
	})
	defer client.CloseIdleConnections()

	measurement := e.newDownloadMeasurement(ip, attempt)
	measurement.emitSample(0, true)
	stopSampler := measurement.startSampler()
	defer stopSampler()

	rangeProbe, probeErr := e.probeDownloadRange(ctx, client, profile)
	if probeErr != nil {
		if errors.Is(probeErr, errDownloadInterrupted) {
			if interrupted.Load() {
				return invalidDownloadResult("download_interrupted", true), errDownloadInterrupted
			}
			return invalidDownloadResult("download_interrupted", true), nil
		}
		if statusErr := (downloadStatusError{}); errors.As(probeErr, &statusErr) {
			if config.Debug {
				e.printDownloadDebugInfo(ip, nil, statusErr.statusCode, config.URL, lastRedirectURL, nil)
			}
			if statusErr.statusCode == http.StatusTooManyRequests {
				return invalidDownloadResultWithRetryAfter("rate_limited", true, statusErr.retryAfter), nil
			}
			return invalidDownloadResult("status_mismatch", true), nil
		}
		if requestErr := (downloadRequestCreateError{}); errors.As(probeErr, &requestErr) {
			e.debugEvent("stage.reject", downloadDebugFields(ip, requestErr.err, 0, config.URL, "", "request_create_failed", "文件测速请求创建失败，淘汰该 IP。"))
			return invalidDownloadResult("request_create_failed", false), nil
		}
		if ctx.Err() != nil {
			return invalidDownloadResult("download_interrupted", true), errDownloadInterrupted
		}
		if config.Debug {
			e.printDownloadDebugInfo(ip, probeErr, 0, config.URL, lastRedirectURL, nil)
		}
		return invalidDownloadResult("get_error", true), nil
	}

	if rangeProbe.supported {
		e.runDownloadRangeWorkers(ctx, client, profile, ip, measurement, rangeProbe)
	} else {
		e.runDownloadFullWorker(ctx, client, profile, ip, measurement)
	}
	if ctx.Err() != nil && interrupted.Load() {
		return invalidDownloadResult("download_interrupted", true), errDownloadInterrupted
	}

	stopSampler()
	if retryAfter, rateLimited := measurement.rateLimitRetryAfter(); rateLimited && !measurement.hasValidMeasurement() {
		return invalidDownloadResultWithRetryAfter("rate_limited", true, retryAfter), nil
	}
	if reason := measurement.integrityFailureReason(); reason != "" {
		return invalidDownloadResult(reason, true), nil
	}
	elapsed := measurement.elapsed()
	averageBytes, averageElapsed := measurement.measuredBytes(elapsed)
	speed := averageDownloadSpeed(averageBytes, averageElapsed)
	measurement.emitSample(elapsed, true)
	maxSpeed := measurement.maxSampleSpeedSnapshot()
	if averageBytes <= 0 || averageElapsed <= 0 {
		if measurement.bytesReadSnapshot() <= 0 {
			return invalidDownloadResult("download_no_body_read", true), nil
		}
		return invalidDownloadResult("download_no_valid_measurement", true), nil
	}
	return validDownloadResult(speed, maxSpeed, measurement.coloValue(), measurement.bytesReadSnapshot(), averageBytes, averageElapsed), nil
}

type downloadStatusError struct {
	statusCode int
	retryAfter time.Duration
}

func (e downloadStatusError) Error() string {
	return "download status mismatch: " + strconv.Itoa(e.statusCode)
}

type downloadRequestCreateError struct {
	err error
}

func (e downloadRequestCreateError) Error() string {
	return e.err.Error()
}

func (e downloadRequestCreateError) Unwrap() error {
	return e.err
}

type downloadRangeProbe struct {
	supported bool
	totalSize int64
}

type downloadMeasurement struct {
	engine            *Engine
	ip                *net.IPAddr
	attempt           int
	startedAt         time.Time
	mu                sync.Mutex
	bytesRead         int64
	measuredRead      int64
	measuredStartedAt time.Duration
	lastSampleRead    int64
	lastSampleAt      time.Duration
	lastSampleElapsed int64
	lastMeasuredRead  int64
	lastMeasuredAt    time.Duration
	maxSampleSpeed    float64
	transferComplete  bool
	colo              string
	integrityFailure  string
	rateLimited       bool
	rateLimitDelay    time.Duration
}

func (m *downloadMeasurement) probeEngine() *Engine {
	return m.engine
}

func (e *Engine) newDownloadMeasurement(ip *net.IPAddr, attempt int) *downloadMeasurement {
	return &downloadMeasurement{
		engine:    e,
		ip:        ip,
		attempt:   attempt,
		startedAt: time.Now(),
	}
}

func (m *downloadMeasurement) elapsed() time.Duration {
	elapsed := time.Since(m.startedAt)
	if elapsed < 0 {
		return 0
	}
	if m.probeEngine().config.Timeout > 0 && elapsed > m.probeEngine().config.Timeout {
		return m.probeEngine().config.Timeout
	}
	return elapsed
}

func (m *downloadMeasurement) addBytes(n int, readStartedElapsed, readFinishedElapsed time.Duration) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bytesRead += int64(n)
	m.transferComplete = false
	if readFinishedElapsed >= m.probeEngine().config.DownloadWarmupDuration {
		if m.measuredStartedAt <= 0 {
			m.measuredStartedAt = readStartedElapsed
			if m.measuredStartedAt < m.probeEngine().config.DownloadWarmupDuration {
				m.measuredStartedAt = m.probeEngine().config.DownloadWarmupDuration
			}
		}
		m.measuredRead += int64(n)
	}
}

func (m *downloadMeasurement) setTransferComplete(complete bool) {
	m.mu.Lock()
	m.transferComplete = complete
	m.mu.Unlock()
}

func (m *downloadMeasurement) markIntegrityFailure(reason string) {
	if reason == "" {
		return
	}
	m.mu.Lock()
	if m.integrityFailure == "" {
		m.integrityFailure = reason
	}
	m.mu.Unlock()
}

func (m *downloadMeasurement) markRateLimited(retryAfter time.Duration) {
	m.mu.Lock()
	m.rateLimited = true
	if retryAfter > m.rateLimitDelay {
		m.rateLimitDelay = retryAfter
	}
	m.mu.Unlock()
}

func (m *downloadMeasurement) rateLimitRetryAfter() (time.Duration, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rateLimitDelay, m.rateLimited
}

func (m *downloadMeasurement) integrityFailureReason() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.integrityFailure
}

func (m *downloadMeasurement) setColoFromHeader(header http.Header) {
	colo := getHeaderColo(header)
	if colo == "" {
		return
	}
	m.mu.Lock()
	if m.colo == "" {
		m.colo = colo
	}
	m.mu.Unlock()
}

func (m *downloadMeasurement) coloValue() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.colo
}

func (m *downloadMeasurement) bytesReadSnapshot() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bytesRead
}

func (m *downloadMeasurement) measuredBytes(elapsed time.Duration) (int64, time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.measuredBytesLocked(elapsed)
}

func (m *downloadMeasurement) maxSampleSpeedSnapshot() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.maxSampleSpeed
}

func (m *downloadMeasurement) measuredBytesLocked(elapsed time.Duration) (int64, time.Duration) {
	if m.measuredRead <= 0 || elapsed < m.probeEngine().config.DownloadWarmupDuration {
		return 0, 0
	}
	start := m.measuredStartedAt
	if start <= 0 {
		start = m.probeEngine().config.DownloadWarmupDuration
	}
	if elapsed <= start {
		return 0, 0
	}
	return m.measuredRead, elapsed - start
}

func (m *downloadMeasurement) hasValidMeasurement() bool {
	averageBytes, averageElapsed := m.measuredBytes(m.elapsed())
	return averageBytes > 0 && averageElapsed > 0
}

func (m *downloadMeasurement) startSampler() func() {
	if m.probeEngine().config.DownloadSampleInterval <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(m.probeEngine().config.DownloadSampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.emitSample(m.elapsed(), false)
			case <-done:
				return
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(done)
			wg.Wait()
		})
	}
}

func (m *downloadMeasurement) emitSample(elapsed time.Duration, force bool) {
	if elapsed < 0 || (!force && elapsed <= 0) {
		return
	}
	elapsedMS := elapsed.Milliseconds()
	m.mu.Lock()
	if !force && elapsedMS == m.lastSampleElapsed {
		m.mu.Unlock()
		return
	}
	sampleElapsed := elapsed - m.lastSampleAt
	if sampleElapsed < 0 {
		sampleElapsed = 0
	}
	currentReady := sampleElapsed > 0 && m.bytesRead > m.lastSampleRead
	currentSpeed := 0.0
	if currentReady {
		currentSpeed = float64(m.bytesRead-m.lastSampleRead) / sampleElapsed.Seconds()
	}
	averageBytes, averageElapsed := m.measuredBytesLocked(elapsed)
	averageSpeed := averageDownloadSpeed(averageBytes, averageElapsed)
	measuredSampleBytes := averageBytes - m.lastMeasuredRead
	measuredSampleElapsed := averageElapsed - m.lastMeasuredAt
	measuredSampleSpeed := averageDownloadSpeed(measuredSampleBytes, measuredSampleElapsed)
	if measuredSampleSpeed > m.maxSampleSpeed {
		m.maxSampleSpeed = measuredSampleSpeed
	}
	sample := DownloadSpeedSample{
		Stage:             DownloadSpeedSampleStage,
		IP:                m.ip.String(),
		CurrentSpeedMBs:   currentSpeed / 1024 / 1024,
		CurrentReady:      currentReady,
		AverageSpeedMBs:   averageSpeed / 1024 / 1024,
		AverageReady:      averageBytes > 0 && averageElapsed > 0,
		BodyRead:          m.bytesRead > 0,
		BytesRead:         m.bytesRead,
		ElapsedMS:         elapsedMS,
		Colo:              m.colo,
		SampleBytes:       m.bytesRead - m.lastSampleRead,
		SampleElapsedMS:   sampleElapsed.Milliseconds(),
		MeasuredBytes:     averageBytes,
		MeasuredElapsedMS: averageElapsed.Milliseconds(),
		TransferComplete:  m.transferComplete,
		Attempt:           m.attempt,
	}
	m.lastSampleRead = m.bytesRead
	m.lastSampleAt = elapsed
	m.lastSampleElapsed = elapsedMS
	m.lastMeasuredRead = averageBytes
	m.lastMeasuredAt = averageElapsed
	m.mu.Unlock()

	if m.probeEngine().hooks.DownloadSpeedSample != nil {
		m.probeEngine().hooks.DownloadSpeedSample(sample)
	}
}

func (e *Engine) probeDownloadRange(ctx context.Context, client *http.Client, profile httpcfg.Profile) (downloadRangeProbe, error) {
	req, err := e.newDownloadRequest(ctx, profile, e.downloadURLWithNonce("probe"), "bytes=0-0")
	if err != nil {
		return downloadRangeProbe{}, downloadRequestCreateError{err: err}
	}
	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return downloadRangeProbe{}, errDownloadInterrupted
		}
		return downloadRangeProbe{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusPartialContent {
		_, _, total, ok := parseContentRange(response.Header.Get("Content-Range"))
		if !ok || total <= 0 {
			return downloadRangeProbe{supported: false}, nil
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1))
		return downloadRangeProbe{supported: true, totalSize: total}, nil
	}
	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return downloadRangeProbe{supported: false}, nil
	}
	return downloadRangeProbe{}, downloadStatusError{
		statusCode: response.StatusCode,
		retryAfter: retryAfterDelay(response.Header.Get("Retry-After"), time.Now()),
	}
}

func (e *Engine) runDownloadRangeWorkers(ctx context.Context, client *http.Client, profile httpcfg.Profile, ip *net.IPAddr, measurement *downloadMeasurement, probe downloadRangeProbe) {
	workerCount := e.config.DownloadGetConcurrency
	if workerCount <= 0 {
		workerCount = defaultDownloadGetConcurrency
	}
	if probe.totalSize > 0 && int64(workerCount) > probe.totalSize {
		workerCount = int(probe.totalSize)
	}
	if workerCount <= 0 {
		workerCount = 1
	}
	chunkSize := (probe.totalSize + int64(workerCount) - 1) / int64(workerCount)
	bufferSize := e.downloadBufferSize()

	var wg sync.WaitGroup
	for workerID := 0; workerID < workerCount; workerID++ {
		rangeStart := int64(workerID) * chunkSize
		if rangeStart >= probe.totalSize {
			break
		}
		rangeEnd := rangeStart + chunkSize - 1
		if rangeEnd >= probe.totalSize {
			rangeEnd = probe.totalSize - 1
		}
		wg.Add(1)
		go func(segmentID int, start, end int64) {
			defer wg.Done()
			e.downloadRangeWorker(ctx, client, profile, ip, measurement, segmentID+1, start, end, probe.totalSize, bufferSize)
		}(workerID, rangeStart, rangeEnd)
	}
	wg.Wait()
}

func (e *Engine) downloadRangeWorker(ctx context.Context, client *http.Client, profile httpcfg.Profile, ip *net.IPAddr, measurement *downloadMeasurement, segmentID int, baseStart, baseEnd, totalSize int64, bufferSize int) {
	buffer := make([]byte, bufferSize)
	rangeStart, rangeEnd := baseStart, baseEnd
	sequence := 0
	for ctx.Err() == nil && measurement.elapsed() < e.config.Timeout {
		if sequence > 0 {
			rangeStart, rangeEnd = randomDownloadRange(totalSize, baseEnd-baseStart+1, baseStart, baseEnd)
		}
		offset := rangeStart
		for offset <= rangeEnd && ctx.Err() == nil && measurement.elapsed() < e.config.Timeout {
			req, err := e.newDownloadRequest(ctx, profile, e.downloadURLWithNonce("range-"+strconv.Itoa(segmentID)), "bytes="+strconv.FormatInt(offset, 10)+"-"+strconv.FormatInt(rangeEnd, 10))
			if err != nil {
				e.logDownloadReconnect(ip, segmentID, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), "request_create_failed", rangeStart, rangeEnd)
				return
			}
			response, err := client.Do(req)
			if err != nil {
				if ctx.Err() != nil || measurement.hasValidMeasurement() {
					return
				}
				e.logDownloadReconnect(ip, segmentID, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), "range_request_error", rangeStart, rangeEnd)
				return
			}
			if response.StatusCode != http.StatusPartialContent {
				reason := "range_status_mismatch"
				if response.StatusCode == http.StatusTooManyRequests {
					measurement.markRateLimited(retryAfterDelay(response.Header.Get("Retry-After"), time.Now()))
					reason = "rate_limited"
				}
				_ = response.Body.Close()
				if measurement.hasValidMeasurement() {
					return
				}
				e.logDownloadReconnect(ip, segmentID, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), reason, rangeStart, rangeEnd)
				return
			}
			contentStart, contentEnd, _, ok := parseContentRange(response.Header.Get("Content-Range"))
			if !ok || contentStart != offset || contentEnd < contentStart {
				_ = response.Body.Close()
				if measurement.hasValidMeasurement() {
					return
				}
				e.logDownloadReconnect(ip, segmentID, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), "range_header_mismatch", rangeStart, rangeEnd)
				return
			}
			integrity := newDownloadIntegrity(response, contentEnd-contentStart+1)
			measurement.setColoFromHeader(response.Header)
			nextOffset, reason := e.readDownloadBody(ctx, response.Body, buffer, measurement, offset, rangeEnd, integrity)
			_ = response.Body.Close()
			offset = nextOffset
			if reason == "" {
				continue
			}
			if reason == "segment_complete" {
				measurement.setTransferComplete(true)
				break
			}
			if isDownloadIntegrityFailure(reason) {
				measurement.markIntegrityFailure(reason)
				e.logDownloadReconnect(ip, segmentID, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), reason, rangeStart, rangeEnd)
				return
			}
			if ctx.Err() != nil || measurement.elapsed() >= e.config.Timeout {
				return
			}
			e.logDownloadReconnect(ip, segmentID, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), reason, offset, rangeEnd)
			if offset <= rangeStart {
				return
			}
		}
		sequence++
		measurement.setTransferComplete(true)
		e.logDownloadReconnect(ip, segmentID, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), "segment_complete", rangeStart, rangeEnd)
	}
}

func (e *Engine) runDownloadFullWorker(ctx context.Context, client *http.Client, profile httpcfg.Profile, ip *net.IPAddr, measurement *downloadMeasurement) {
	buffer := make([]byte, e.downloadBufferSize())
	for segment := 1; ctx.Err() == nil && measurement.elapsed() < e.config.Timeout; segment++ {
		req, err := e.newDownloadRequest(ctx, profile, e.downloadURLWithNonce("full-"+strconv.Itoa(segment)), "")
		if err != nil {
			e.logDownloadReconnect(ip, segment, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), "request_create_failed", -1, -1)
			return
		}
		response, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil || measurement.hasValidMeasurement() {
				return
			}
			e.logDownloadReconnect(ip, segment, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), "get_error", -1, -1)
			return
		}
		if response.StatusCode != http.StatusOK {
			reason := "status_mismatch"
			if response.StatusCode == http.StatusTooManyRequests {
				measurement.markRateLimited(retryAfterDelay(response.Header.Get("Retry-After"), time.Now()))
				reason = "rate_limited"
			}
			_ = response.Body.Close()
			if measurement.hasValidMeasurement() {
				return
			}
			e.logDownloadReconnect(ip, segment, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), reason, -1, -1)
			return
		}
		measurement.setColoFromHeader(response.Header)
		contentLength := response.ContentLength
		integrity := newDownloadIntegrity(response, contentLength)
		nextOffset, reason := e.readDownloadBody(ctx, response.Body, buffer, measurement, 0, contentLength-1, integrity)
		_ = response.Body.Close()
		if isDownloadIntegrityFailure(reason) {
			measurement.markIntegrityFailure(reason)
			e.logDownloadReconnect(ip, segment, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), reason, -1, -1)
			return
		}
		if reason == "segment_complete" || (contentLength >= 0 && nextOffset >= contentLength) {
			measurement.setTransferComplete(true)
			e.logDownloadReconnect(ip, segment, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), "segment_complete", -1, -1)
			continue
		}
		if ctx.Err() != nil || measurement.elapsed() >= e.config.Timeout {
			return
		}
		if nextOffset <= 0 && !measurement.hasValidMeasurement() {
			return
		}
		if reason == "" {
			reason = "body_disconnected"
		}
		e.logDownloadReconnect(ip, segment, measurement.elapsed(), measurement.bytesReadSnapshot(), measurement.measuredReadSnapshot(), reason, -1, -1)
	}
}

func (e *Engine) readDownloadBody(ctx context.Context, body io.Reader, buffer []byte, measurement *downloadMeasurement, offset, rangeEnd int64, integrity *downloadIntegrity) (int64, string) {
	for ctx.Err() == nil && measurement.elapsed() < e.config.Timeout {
		readStartedElapsed := measurement.elapsed()
		n, err := body.Read(buffer)
		readFinishedElapsed := measurement.elapsed()
		if n > 0 {
			offset += int64(n)
			if integrity != nil {
				integrity.write(buffer[:n])
			}
			measurement.addBytes(n, readStartedElapsed, readFinishedElapsed)
		}
		if err == nil {
			if rangeEnd >= 0 && offset > rangeEnd {
				if reason := integrityFailureReason(integrity); reason != "" {
					return offset, reason
				}
				return offset, "segment_complete"
			}
			continue
		}
		if ctx.Err() != nil {
			return offset, "download_interrupted"
		}
		if errors.Is(err, io.EOF) {
			if rangeEnd < 0 || offset > rangeEnd {
				if reason := integrityFailureReason(integrity); reason != "" {
					return offset, reason
				}
				return offset, "segment_complete"
			}
			if n > 0 {
				return offset, "body_disconnected"
			}
			return offset, "body_read_error"
		}
		if isReconnectableDownloadBodyError(err) && offset > 0 {
			return offset, "body_disconnected"
		}
		if isNetTimeout(err) {
			return offset, "read_timeout"
		}
		return offset, "body_read_error"
	}
	if ctx.Err() != nil {
		return offset, "download_interrupted"
	}
	return offset, ""
}

type downloadIntegrity struct {
	expectedLength int64
	headerFailure  string
	read           int64
	hashes         []downloadHashCheck
}

type downloadHashCheck struct {
	name     string
	hash     hash.Hash
	expected []byte
}

func newDownloadIntegrity(response *http.Response, expectedLength int64) *downloadIntegrity {
	if expectedLength < 0 && response != nil && response.ContentLength >= 0 {
		expectedLength = response.ContentLength
	}
	integrity := &downloadIntegrity{
		expectedLength: expectedLength,
	}
	if response == nil {
		return integrity
	}
	if response.ContentLength >= 0 && expectedLength >= 0 && response.ContentLength != expectedLength {
		integrity.headerFailure = "content_length_mismatch"
	}
	integrity.hashes = downloadHashChecks(response.Header)
	return integrity
}

func (v *downloadIntegrity) write(data []byte) {
	if v == nil || len(data) == 0 {
		return
	}
	v.read += int64(len(data))
	for index := range v.hashes {
		_, _ = v.hashes[index].hash.Write(data)
	}
}

func (v *downloadIntegrity) validate() string {
	if v == nil {
		return ""
	}
	if v.headerFailure != "" {
		return v.headerFailure
	}
	if v.expectedLength >= 0 && v.read != v.expectedLength {
		return "content_length_mismatch"
	}
	for _, check := range v.hashes {
		if !bytes.Equal(check.hash.Sum(nil), check.expected) {
			return check.name + "_mismatch"
		}
	}
	return ""
}

func integrityFailureReason(integrity *downloadIntegrity) string {
	if integrity == nil {
		return ""
	}
	return integrity.validate()
}

func isDownloadIntegrityFailure(reason string) bool {
	return reason == "content_length_mismatch" || strings.HasSuffix(reason, "_mismatch")
}

func downloadHashChecks(header http.Header) []downloadHashCheck {
	if header == nil {
		return nil
	}
	checks := make([]downloadHashCheck, 0, 2)
	checks = appendHashCheck(checks, "content_md5", "md5", header.Get("Content-MD5"))
	checks = appendHashCheck(checks, "x_checksum_md5", "md5", header.Get("X-Checksum-MD5"))
	for _, headerName := range []string{
		"X-Checksum-SHA256",
		"X-Checksum-Sha256",
		"X-Content-SHA256",
		"X-Content-Sha256",
		"X-Amz-Content-Sha256",
	} {
		checks = appendHashCheck(checks, strings.ToLower(strings.ReplaceAll(headerName, "-", "_")), "sha256", header.Get(headerName))
	}
	for _, value := range header.Values("Digest") {
		checks = appendDigestHashChecks(checks, value)
	}
	return checks
}

func appendDigestHashChecks(checks []downloadHashCheck, value string) []downloadHashCheck {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, encoded, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		algorithm := normalizeHashAlgorithm(name)
		if algorithm == "" {
			continue
		}
		checkName := "digest_" + strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "-", "_")
		checks = appendHashCheck(checks, checkName, algorithm, encoded)
	}
	return checks
}

func appendHashCheck(checks []downloadHashCheck, name, algorithm, encoded string) []downloadHashCheck {
	hasher, size := newDownloadHasher(algorithm)
	if hasher == nil {
		return checks
	}
	expected, ok := decodeHashValue(encoded, size)
	if !ok {
		return checks
	}
	return append(checks, downloadHashCheck{
		name:     name,
		hash:     hasher,
		expected: expected,
	})
}

func newDownloadHasher(algorithm string) (hash.Hash, int) {
	switch normalizeHashAlgorithm(algorithm) {
	case "md5":
		return md5.New(), md5.Size
	case "sha256":
		return sha256.New(), sha256.Size
	default:
		return nil, 0
	}
}

func normalizeHashAlgorithm(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	switch normalized {
	case "md5", "contentmd5":
		return "md5"
	case "sha256", "sha2256":
		return "sha256"
	default:
		return ""
	}
}

func decodeHashValue(value string, expectedSize int) ([]byte, bool) {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	if strings.HasPrefix(value, ":") && strings.HasSuffix(value, ":") && len(value) >= 2 {
		value = strings.TrimPrefix(strings.TrimSuffix(value, ":"), ":")
	}
	if value == "" || strings.EqualFold(value, "UNSIGNED-PAYLOAD") {
		return nil, false
	}
	for _, decoder := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := decoder.DecodeString(value); err == nil && len(decoded) == expectedSize {
			return decoded, true
		}
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == expectedSize {
		return decoded, true
	}
	return nil, false
}

func (m *downloadMeasurement) measuredReadSnapshot() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.measuredRead
}

func newDownloadRequest(ctx context.Context, profile httpcfg.Profile, rawURL, rangeHeader string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	profile.Apply(req)
	httpclient.ApplyNoCache(req)
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	return req, nil
}

func (e *Engine) newDownloadRequest(ctx context.Context, profile httpcfg.Profile, rawURL, rangeHeader string) (*http.Request, error) {
	return newDownloadRequest(ctx, profile, rawURL, rangeHeader)
}

func (e *Engine) downloadBufferSize() int {
	kb := e.config.DownloadBufferKB
	if kb <= 0 {
		kb = defaultDownloadBufferKB
	}
	if kb < MinDownloadBufferKB {
		kb = MinDownloadBufferKB
	}
	if kb > MaxDownloadBufferKB {
		kb = MaxDownloadBufferKB
	}
	return kb * 1024
}

func (e *Engine) downloadURLWithNonce(block string) string {
	parsed, err := url.Parse(e.config.URL)
	if err != nil {
		return e.config.URL
	}
	query := parsed.Query()
	query.Set("cfst_nonce", randomHex(8))
	if block != "" {
		query.Set("cfst_block", block)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func randomHex(byteCount int) string {
	if byteCount <= 0 {
		byteCount = 8
	}
	raw := make([]byte, byteCount)
	if _, err := rand.Read(raw); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(raw)
}

func parseContentRange(value string) (int64, int64, int64, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "bytes ") {
		return 0, 0, 0, false
	}
	value = strings.TrimSpace(value[len("bytes "):])
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, 0, 0, false
	}
	rangeParts := strings.Split(parts[0], "-")
	if len(rangeParts) != 2 || strings.TrimSpace(parts[1]) == "*" {
		return 0, 0, 0, false
	}
	start, startErr := strconv.ParseInt(strings.TrimSpace(rangeParts[0]), 10, 64)
	end, endErr := strconv.ParseInt(strings.TrimSpace(rangeParts[1]), 10, 64)
	total, totalErr := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if startErr != nil || endErr != nil || totalErr != nil || start < 0 || end < start || total <= 0 {
		return 0, 0, 0, false
	}
	return start, end, total, true
}

func randomDownloadRange(totalSize, length, fallbackStart, fallbackEnd int64) (int64, int64) {
	if totalSize <= 0 || length <= 0 || length >= totalSize {
		return fallbackStart, fallbackEnd
	}
	maxStart := totalSize - length
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return fallbackStart, fallbackEnd
	}
	value := int64(0)
	for _, b := range randomBytes {
		value = (value << 8) | int64(b)
	}
	if value < 0 {
		value = -value
	}
	start := value % (maxStart + 1)
	return start, start + length - 1
}

func isReconnectableDownloadBodyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func (e *Engine) logDownloadReconnect(ip *net.IPAddr, segment int, elapsed time.Duration, bytesRead, measuredBytes int64, reason string, rangeStart, rangeEnd int64) {
	e.debugEvent("stage.detail", map[string]any{
		"get": map[string]any{
			"bytes_read":           bytesRead,
			"elapsed_ms":           elapsed.Milliseconds(),
			"get_concurrency":      e.config.DownloadGetConcurrency,
			"measured_bytes":       measuredBytes,
			"protocol":             e.config.DownloadHTTPProtocol,
			"range_end":            rangeEnd,
			"range_start":          rangeStart,
			"reconnect_sequence":   segment,
			"reconnect_reason":     reason,
			"segment_id":           segment,
			"timeout_remaining_ms": (e.config.Timeout - elapsed).Milliseconds(),
		},
		"ip":      ip.String(),
		"message": "文件测速连接中断，继续对同一 IP 发起下载测速。",
		"reason":  "download_reconnect",
		"stage":   "stage3_get",
	})
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
