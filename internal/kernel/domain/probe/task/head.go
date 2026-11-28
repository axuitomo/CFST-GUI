package task

import (
	"fmt"
	"net"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/utils"
)

const (
	// 追踪阶段并发上限。实测一次 1915 颗候选的跑法在 30 并发下被 2s 超时拖成 100 秒，
	// 每颗 IP 打的是不同边缘节点，并发抬到 100 不共享同一个限流桶（429 仍走既有退避）。
	MaxHeadRoutines      = 100
	MaxTraceRoutines     = MaxHeadRoutines
	defaultHeadRoutines  = 6
	defaultHeadTestCount = 512
	defaultHeadTimeout   = time.Second
	defaultTraceURL      = "https://speed.cloudflare.com/cdn-cgi/trace"
	maxTraceBodyBytes    = 64 * 1024

	TraceColoModeStandard = "standard"
	TraceColoModeTraceURL = "trace_url"
)

type TraceDiagnostic struct {
	Colo         string
	Error        string
	IP           string
	Reason       string
	RetryAfterMS int64
	StatusCode   int
	URL          string
}

type traceFailureReason string

const (
	traceFailureNone             traceFailureReason = ""
	traceFailureRequestCreate    traceFailureReason = "request_create_failed"
	traceFailureInterrupted      traceFailureReason = "trace_interrupted"
	traceFailureRequest          traceFailureReason = "trace_error"
	traceFailureRateLimited      traceFailureReason = "rate_limited"
	traceFailureRead             traceFailureReason = "trace_read_error"
	traceFailureStatus           traceFailureReason = "status_mismatch"
	traceFailureLatencyLimit     traceFailureReason = "trace_latency_limit"
	traceFailureColoFilter       traceFailureReason = "colo_filter"
	traceFailureSourceColoFilter traceFailureReason = "source_colo_filter"
	// traceFailureEdgeRestricted 与文件测速的 403/1034 共用同一个 reason，日志和界面只需认一种值。
	traceFailureEdgeRestricted traceFailureReason = downloadReasonEdgeRestricted
)

type traceProbeResult struct {
	delay      time.Duration
	errorText  string
	colo       string
	ok         bool
	reason     traceFailureReason
	retryAfter time.Duration
	statusCode int
	url        string
	ignored    bool
}

// edgeIgnored 表示这颗 IP 在追踪阶段被 1034 拦下，且已按配置屏蔽（跳过、不重试、不计冷却）。
func (r traceProbeResult) edgeIgnored() bool {
	return r.reason == traceFailureEdgeRestricted && r.ignored
}

func NormalizeHeadRoutines(value int) int {
	return NormalizeTraceRoutines(value)
}

func NormalizeTraceRoutines(value int) int {
	if value <= 0 {
		return defaultHeadRoutines
	}
	if value > MaxTraceRoutines {
		return MaxTraceRoutines
	}
	return value
}

func (e *Engine) EstimateTraceProbeCount(candidateCount int) int {
	if candidateCount <= 0 {
		return 0
	}
	limit := e.config.HeadTestCount
	if limit <= 0 {
		return candidateCount
	}
	if candidateCount < limit {
		return candidateCount
	}
	return limit
}

func (e *Engine) TestTraceAvailability(ipSet utils.PingDelaySet) (traceSet utils.PingDelaySet) {
	total := e.EstimateTraceProbeCount(len(ipSet))
	if total <= 0 {
		return traceSet
	}

	candidates := ipSet
	if len(candidates) > total {
		candidates = candidates[:total]
	}

	results := make([]utils.CloudflareIPData, len(candidates))
	passed := make([]bool, len(candidates))
	fallbackResults := make([]utils.CloudflareIPData, len(candidates))
	fallbackPassed := make([]bool, len(candidates))
	control := makeBoundedChan[struct{}](e.config.HeadRoutines, defaultHeadRoutines, MaxTraceRoutines)
	var wg sync.WaitGroup
	var processedCount atomic.Int32
	var passedCount atomic.Int32
	var fallbackCount atomic.Int32

	for index, item := range candidates {
		e.checkPause("stage2_trace", item.IP.String())
		wg.Add(1)
		control <- struct{}{}
		go func(index int, item utils.CloudflareIPData) {
			defer wg.Done()
			defer func() { <-control }()
			setWorkerLabels("stage2_trace", item.IP.String())
			defer func() {
				if recovered := recover(); recovered != nil {
					e.debugEvent("probe.failed", map[string]any{
						"ip":      item.IP.String(),
						"level":   "error",
						"message": fmt.Sprintf("追踪探测 worker 异常退出：%v", recovered),
						"reason":  "trace_worker_panic",
						"stage":   "stage2_trace",
					})
					e.workerPanics.Add(1)
					processed := processedCount.Add(1)
					qualified := passedCount.Load()
					e.emitTraceProgress(int(processed), int(qualified), int(processed-qualified), total)
				}
			}()

			e.checkPause("stage2_trace", item.IP.String())
			probeStartedAt := time.Now()
			probe := e.runTraceProbeWithRetry(item.IP)
			traceDelay, colo, ok := probe.delay, probe.colo, probe.ok
			if ok && e.config.HeadMaxDelay > 0 && traceDelay > e.config.HeadMaxDelay {
				ok = false
				e.debugEvent("stage.reject", map[string]any{
					"ip":      item.IP.String(),
					"message": "追踪延迟超过阈值，淘汰该 IP。",
					"reason":  "trace_latency_limit",
					"stage":   "stage2_trace",
					"trace": map[string]any{
						"delay_ms":     traceDelay.Seconds() * 1000,
						"max_delay_ms": e.config.HeadMaxDelay.Seconds() * 1000,
					},
				})
				e.emitTraceDiagnostic(TraceDiagnostic{
					IP:         item.IP.String(),
					Reason:     string(traceFailureLatencyLimit),
					StatusCode: probe.statusCode,
					URL:        probe.url,
				})
			}
			if ok {
				originalColo := colo
				ok = e.sourceAllowsColo(item.IP, colo)
				if !ok {
					e.debugEvent("stage.reject", map[string]any{
						"colo":    originalColo,
						"ip":      item.IP.String(),
						"message": "追踪地区码不匹配输入源 COLO 白名单，淘汰该 IP。",
						"reason":  "source_colo_filter",
						"stage":   "stage2_trace",
					})
					e.emitTraceDiagnostic(TraceDiagnostic{
						Colo:       originalColo,
						IP:         item.IP.String(),
						Reason:     string(traceFailureSourceColoFilter),
						StatusCode: probe.statusCode,
						URL:        probe.url,
					})
				}
			}
			if ok && e.config.TraceCFColo != "" {
				originalColo := colo
				colo, ok = e.configuredColoAllowed(colo)
				if !ok {
					e.debugEvent("stage.reject", map[string]any{
						"colo":    originalColo,
						"ip":      item.IP.String(),
						"message": "追踪地区码不匹配，淘汰该 IP。",
						"reason":  "colo_filter",
						"stage":   "stage2_trace",
						"trace": map[string]any{
							"expected_colo": e.config.TraceCFColo,
						},
					})
					e.emitTraceDiagnostic(TraceDiagnostic{
						Colo:       originalColo,
						IP:         item.IP.String(),
						Reason:     string(traceFailureColoFilter),
						StatusCode: probe.statusCode,
						URL:        probe.url,
					})
				}
			}
			if ok {
				item.Colo = colo
				item.HeadDelay = traceDelay
				results[index] = item
				passed[index] = true
				passedCount.Add(1)
			} else if e.traceSoftPassAllowedFor(probe) {
				results[index] = item
				passed[index] = true
				passedCount.Add(1)
				ok = true
				e.debugEvent("stage.detail", map[string]any{
					"ip":      item.IP.String(),
					"message": "追踪探测遇到临时异常，保留该 IP 进入后续文件测速。",
					"reason":  "trace_soft_pass",
					"stage":   "stage2_trace",
					"trace": map[string]any{
						"failure_reason": probe.reason,
						"retry_after_ms": probe.retryAfter.Milliseconds(),
						"status_code":    probe.statusCode,
						"url":            e.config.TraceURL,
					},
				})
			} else if traceFallbackAllowedFor(probe.reason) {
				fallbackResults[index] = item
				fallbackPassed[index] = true
				fallbackCount.Add(1)
			}
			// 被屏蔽的 1034 是明确跳过，不计入连续失败冷却；放行的按普通失败计账。
			if !probe.edgeIgnored() {
				e.noteStageProbeOutcome("stage2_trace", item.IP.String(), classifyProbeOutcome(ok, time.Since(probeStartedAt), e.config.HeadTimeout))
			}

			processed := processedCount.Add(1)
			qualified := passedCount.Load()
			e.emitTraceProgress(int(processed), int(qualified), int(processed-qualified), total)
		}(index, item)
	}

	wg.Wait()
	for index, ok := range passed {
		if ok {
			traceSet = append(traceSet, results[index])
		}
	}
	if len(traceSet) == 0 && e.canFallbackToTCPCandidates() && int(fallbackCount.Load()) == len(candidates) {
		for index, ok := range fallbackPassed {
			if ok {
				traceSet = append(traceSet, fallbackResults[index])
			}
		}
		e.debugEvent("stage.fallback", map[string]any{
			"counts": map[string]any{
				"fallback": len(traceSet),
				"total":    len(candidates),
			},
			"message": "追踪请求全部失败，未启用追踪硬筛选，保留 TCP 通过候选进入后续测速。",
			"reason":  "trace_transport_all_failed",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"url": e.config.TraceURL,
			},
		})
		e.emitTraceProgress(total, len(traceSet), total-len(traceSet), total)
	}
	sort.Sort(traceSet)
	return traceSet
}

func (e *Engine) emitTraceProgress(processed, passed, failed, total int) {
	if e.hooks.TraceProgress != nil {
		e.hooks.TraceProgress(processed, passed, failed, total)
		return
	}
	if e.hooks.HeadProgress != nil {
		e.hooks.HeadProgress(processed, passed, failed, total)
	}
}

func (e *Engine) emitTraceDiagnostic(diagnostic TraceDiagnostic) {
	if e.hooks.TraceDiagnostic != nil {
		e.hooks.TraceDiagnostic(diagnostic)
	}
}

func (e *Engine) runTraceProbeWithRetry(ip *net.IPAddr) traceProbeResult {
	var result traceProbeResult
	attempt := 1
	stage := "stage2_trace"
	ipText := ip.String()
	for attempt <= e.retryAttemptLimit() {
		e.checkPause(stage, ipText)
		if e.isCanceled(stage, ipText) {
			return traceProbeResult{errorText: "任务已取消", reason: traceFailureInterrupted}
		}
		if e.traceProbe != nil {
			result = e.traceProbe(e, ip)
		} else {
			result = e.traceProbeIP(ip)
		}
		if result.reason == traceFailureInterrupted {
			// 暂停打断不应被计作真实失败，恢复后重试当前 IP。
			e.checkPause(stage, ipText)
			if e.isCanceled(stage, ipText) {
				return result
			}
			continue
		}
		if result.edgeIgnored() {
			// 1034 是明确跳过：不重试、不计入连续失败冷却。
			return result
		}
		if result.ok {
			return result
		}
		if attempt < e.retryAttemptLimit() {
			if result.reason == traceFailureRateLimited {
				e.sleepBeforeRateLimitRetry("stage2_trace", ip.String(), attempt, result.retryAfter)
			} else {
				e.sleepBeforeRetry("stage2_trace", ip.String(), attempt)
			}
		}
		attempt++
	}
	return result
}

func (e *Engine) canFallbackToTCPCandidates() bool {
	return e.config.HeadMaxDelay <= 0 && e.config.TraceStatusCode == 0 && e.config.TraceCFColo == "" && len(e.config.SourceColoFilters) == 0
}

func traceFallbackAllowedFor(reason traceFailureReason) bool {
	return reason == traceFailureRequest || reason == traceFailureRead
}

func (e *Engine) traceSoftPassAllowedFor(result traceProbeResult) bool {
	if e.config.HeadMaxDelay > 0 || e.config.TraceCFColo != "" || len(e.config.SourceColoFilters) > 0 {
		return false
	}
	switch result.reason {
	case traceFailureRequest, traceFailureRead, traceFailureRateLimited:
		return true
	case traceFailureStatus:
		return isTransientTraceStatus(result.statusCode)
	default:
		return false
	}
}

func isTransientTraceStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooEarly ||
		(statusCode >= http.StatusInternalServerError && statusCode <= 599)
}
