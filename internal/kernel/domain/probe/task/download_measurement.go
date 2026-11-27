package task

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type downloadRangeProbe struct {
	supported bool
	totalSize int64
}

// downloadRateSnapshot 记录一次采样时刻的累计字节数，用于计算固定时间窗口内的速率。
type downloadRateSnapshot struct {
	at    time.Duration
	bytes int64
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
	rateHistory       []downloadRateSnapshot
	maxSampleSpeed    float64
	transferComplete  bool
	colo              string
	integrityFailure  string
	rejectedReason    string
	rejectedIgnored   bool
	rateLimited       bool
	rateLimitDelay    time.Duration
	// giveUp 由引擎注入，用来在确定达不到 MinSpeed 下限时提前收工（取消本次测量的 ctx）。
	giveUp func()
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

// markRejected 记录这颗 IP 被边缘拒绝（403 / 1034），以及是否按配置屏蔽。
// reason 无论是否屏蔽都会保留，日志才能区分 1034 和普通 403。
func (m *downloadMeasurement) markRejected(reason string, ignored bool) {
	m.mu.Lock()
	if m.rejectedReason == "" {
		m.rejectedReason = reason
		m.rejectedIgnored = ignored
	}
	m.mu.Unlock()
}

func (m *downloadMeasurement) rejectedOutcome() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rejectedReason, m.rejectedIgnored
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

func (m *downloadMeasurement) measuredReadSnapshot() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.measuredRead
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

// rateWindow 返回实时速率的滑动窗口；采样间隔比窗口更长时按采样间隔计算，否则会重复读取同一个快照。
func (m *downloadMeasurement) rateWindow() time.Duration {
	if interval := m.probeEngine().config.DownloadSampleInterval; interval > downloadSpeedWindow {
		return interval
	}
	return downloadSpeedWindow
}

// windowSpeedLocked 用最近一个窗口内新增的字节数计算实时速率，调用方必须持有 m.mu。
func (m *downloadMeasurement) windowSpeedLocked(elapsed time.Duration) (bool, float64) {
	window := m.rateWindow()
	ref := downloadRateSnapshot{}
	if len(m.rateHistory) > 0 {
		// 优先取恰好一个窗口之前的快照；样本还不够时退回到最早的一条。
		ref = m.rateHistory[0]
		for i := len(m.rateHistory) - 1; i >= 0; i-- {
			if elapsed-m.rateHistory[i].at >= window {
				ref = m.rateHistory[i]
				break
			}
		}
	}
	windowElapsed := elapsed - ref.at
	if windowElapsed <= 0 || m.bytesRead <= ref.bytes {
		return false, 0
	}
	return true, float64(m.bytesRead-ref.bytes) / windowElapsed.Seconds()
}

func (m *downloadMeasurement) hasValidMeasurement() bool {
	averageBytes, averageElapsed := m.measuredBytes(m.elapsed())
	return averageBytes > 0 && averageElapsed > 0
}

func (m *downloadMeasurement) startSampler() func() {
	done := make(chan struct{})
	var once sync.Once
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 按滑动窗口节奏刷新：比窗口更密的采样只是重复同一个窗口值，读数更快反而更难看清。
		ticker := time.NewTicker(m.rateWindow())
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
	currentReady, currentSpeed := m.windowSpeedLocked(elapsed)
	averageBytes, averageElapsed := m.measuredBytesLocked(elapsed)
	averageSpeed := averageDownloadSpeed(averageBytes, averageElapsed)
	averageReady := averageBytes > 0 && averageElapsed > 0
	// 最高速率同样取滑动窗口速率，避免把一次突发的极短采样窗当成峰值。
	if averageReady && currentSpeed > m.maxSampleSpeed {
		m.maxSampleSpeed = currentSpeed
	}
	peakSpeed := m.maxSampleSpeed
	complete := m.transferComplete
	sample := DownloadSpeedSample{
		Stage:             DownloadSpeedSampleStage,
		IP:                m.ip.String(),
		CurrentSpeedMBs:   currentSpeed / 1024 / 1024,
		CurrentReady:      currentReady,
		AverageSpeedMBs:   averageSpeed / 1024 / 1024,
		AverageReady:      averageReady,
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
	m.rateHistory = append(m.rateHistory, downloadRateSnapshot{at: elapsed, bytes: m.bytesRead})
	for len(m.rateHistory) > 2 && elapsed-m.rateHistory[1].at >= m.rateWindow() {
		m.rateHistory = m.rateHistory[1:]
	}
	m.mu.Unlock()

	if m.probeEngine().hooks.DownloadSpeedSample != nil {
		m.probeEngine().hooks.DownloadSpeedSample(sample)
	}
	if m.giveUp != nil && !force && m.lowSpeedGiveUp(elapsed, complete, peakSpeed) {
		m.giveUp()
	}
}

// lowSpeedGiveUp 判定这颗 IP 是否已注定拿不到结果：预热结束后再留两个采样窗口避开
// TCP 慢启动爬坡，累计峰值速率仍低于 MinSpeed 下限就直接收工。下限为 0（默认）时不触发。
// 只比较峰值：峰值低于下限时平均值必然也低于下限，两种速率口径下这颗 IP 都会被淘汰。
func (m *downloadMeasurement) lowSpeedGiveUp(elapsed time.Duration, complete bool, peakSpeed float64) bool {
	config := m.probeEngine().config
	threshold := config.MinSpeed * 1024 * 1024
	if threshold <= 0 || complete {
		return false
	}
	return elapsed >= config.DownloadWarmupDuration+2*m.rateWindow() && peakSpeed < threshold
}
