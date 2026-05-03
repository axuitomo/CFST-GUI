package task

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/utils"
)

const (
	MaxHeadRoutines      = 20
	MaxTraceRoutines     = MaxHeadRoutines
	defaultHeadRoutines  = 6
	defaultHeadTestCount = 64
	defaultHeadTimeout   = time.Second
	defaultTraceURL      = "https://speed.cloudflare.com/cdn-cgi/trace"
	maxTraceBodyBytes    = 64 * 1024
)

var (
	HeadRoutines  = defaultHeadRoutines
	HeadTestCount = defaultHeadTestCount
	HeadMaxDelay  time.Duration
	HeadTimeout   = defaultHeadTimeout
	TraceURL      = defaultTraceURL

	traceProbeFunc = traceProbe
)

type traceFailureReason string

const (
	traceFailureNone          traceFailureReason = ""
	traceFailureRequestCreate traceFailureReason = "request_create_failed"
	traceFailureRequest       traceFailureReason = "trace_error"
	traceFailureRead          traceFailureReason = "trace_read_error"
	traceFailureStatus        traceFailureReason = "status_mismatch"
)

type traceProbeResult struct {
	delay  time.Duration
	colo   string
	ok     bool
	reason traceFailureReason
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

func checkHeadDefault() {
	checkTraceDefault()
}

func checkTraceDefault() {
	HeadRoutines = NormalizeTraceRoutines(HeadRoutines)
	if HeadTestCount <= 0 {
		HeadTestCount = defaultHeadTestCount
	}
	if HeadMaxDelay < 0 {
		HeadMaxDelay = 0
	}
	if HeadTimeout <= 0 {
		HeadTimeout = defaultHeadTimeout
	}
	if TraceURL == "" {
		TraceURL = defaultTraceURL
	}
	if HttpingCFColo != "" && HttpingCFColomap == nil {
		HttpingCFColomap = MapColoMap()
	}
}

func EstimateHeadProbeCount(candidateCount int) int {
	return EstimateTraceProbeCount(candidateCount)
}

func EstimateTraceProbeCount(candidateCount int) int {
	if candidateCount <= 0 {
		return 0
	}
	limit := HeadTestCount
	if limit <= 0 {
		limit = defaultHeadTestCount
	}
	if candidateCount < limit {
		return candidateCount
	}
	return limit
}

func TestHeadAvailability(ipSet utils.PingDelaySet) utils.PingDelaySet {
	return TestTraceAvailability(ipSet)
}

func TestTraceAvailability(ipSet utils.PingDelaySet) (traceSet utils.PingDelaySet) {
	checkTraceDefault()
	total := EstimateTraceProbeCount(len(ipSet))
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
	control := make(chan struct{}, HeadRoutines)
	var wg sync.WaitGroup
	var processedCount atomic.Int32
	var passedCount atomic.Int32
	var fallbackCount atomic.Int32

	for index, item := range candidates {
		CheckProbePause("stage2_trace", item.IP.String())
		wg.Add(1)
		control <- struct{}{}
		go func(index int, item utils.CloudflareIPData) {
			defer wg.Done()
			defer func() { <-control }()

			CheckProbePause("stage2_trace", item.IP.String())
			probe := runTraceProbeWithRetry(item.IP)
			traceDelay, colo, ok := probe.delay, probe.colo, probe.ok
			if ok && HeadMaxDelay > 0 && traceDelay > HeadMaxDelay {
				ok = false
				utils.DebugEvent("stage.reject", map[string]any{
					"ip":      item.IP.String(),
					"message": "追踪延迟超过阈值，淘汰该 IP。",
					"reason":  "trace_latency_limit",
					"stage":   "stage2_trace",
					"trace": map[string]any{
						"delay_ms":     traceDelay.Seconds() * 1000,
						"max_delay_ms": HeadMaxDelay.Seconds() * 1000,
					},
				})
			}
			if ok && HttpingCFColo != "" {
				originalColo := colo
				colo = filterConfiguredColo(colo)
				ok = colo != ""
				if !ok {
					utils.DebugEvent("stage.reject", map[string]any{
						"colo":    originalColo,
						"ip":      item.IP.String(),
						"message": "追踪地区码不匹配，淘汰该 IP。",
						"reason":  "colo_filter",
						"stage":   "stage2_trace",
						"trace": map[string]any{
							"expected_colo": HttpingCFColo,
						},
					})
				}
			}
			if ok {
				item.Colo = colo
				item.HeadDelay = traceDelay
				results[index] = item
				passed[index] = true
				passedCount.Add(1)
			} else if traceFallbackAllowedFor(probe.reason) {
				fallbackResults[index] = item
				fallbackPassed[index] = true
				fallbackCount.Add(1)
			}
			noteStageProbeOutcome("stage2_trace", item.IP.String(), ok)

			processed := processedCount.Add(1)
			qualified := passedCount.Load()
			emitTraceProgress(int(processed), int(qualified), int(processed-qualified), total)
		}(index, item)
	}

	wg.Wait()
	for index, ok := range passed {
		if ok {
			traceSet = append(traceSet, results[index])
		}
	}
	if len(traceSet) == 0 && canFallbackToTCPCandidates() && int(fallbackCount.Load()) == len(candidates) {
		for index, ok := range fallbackPassed {
			if ok {
				traceSet = append(traceSet, fallbackResults[index])
			}
		}
		utils.DebugEvent("stage.fallback", map[string]any{
			"counts": map[string]any{
				"fallback": len(traceSet),
				"total":    len(candidates),
			},
			"message": "追踪请求全部失败，未启用追踪硬筛选，保留 TCP 通过候选进入后续测速。",
			"reason":  "trace_transport_all_failed",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"url": TraceURL,
			},
		})
		emitTraceProgress(total, len(traceSet), total-len(traceSet), total)
	}
	sort.Sort(traceSet)
	return traceSet
}

func emitTraceProgress(processed, passed, failed, total int) {
	if TraceProgressHook != nil {
		TraceProgressHook(processed, passed, failed, total)
		return
	}
	if HeadProgressHook != nil {
		HeadProgressHook(processed, passed, failed, total)
	}
}

func runTraceProbeWithRetry(ip *net.IPAddr) traceProbeResult {
	var result traceProbeResult
	for attempt := 1; attempt <= retryAttemptLimit(); attempt++ {
		CheckProbePause("stage2_trace", ip.String())
		result = traceProbeFunc(ip)
		if result.ok {
			return result
		}
		if attempt < retryAttemptLimit() {
			sleepBeforeRetry("stage2_trace", ip.String(), attempt)
		}
	}
	return result
}

func traceProbe(ip *net.IPAddr) traceProbeResult {
	profile := currentRequestProfile()
	tlsConfig := &tls.Config{InsecureSkipVerify: profile.InsecureSkipVerify}
	if profile.HasCustomSNI() {
		tlsConfig.ServerName = profile.SNI
	}
	client := http.Client{
		Timeout: HeadTimeout,
		Transport: &http.Transport{
			DialContext:     getDialContext(ip, profile),
			TLSClientConfig: tlsConfig,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	request, err := http.NewRequest(http.MethodGet, TraceURL, nil)
	if err != nil {
		utils.DebugEvent("stage.reject", map[string]any{
			"error":   err.Error(),
			"ip":      ip.String(),
			"message": "追踪请求创建失败，淘汰该 IP。",
			"reason":  "request_create_failed",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"url": TraceURL,
			},
		})
		return traceProbeResult{reason: traceFailureRequestCreate}
	}
	profile.Apply(request)
	request.Header.Set("Connection", "close")
	request.Close = true

	startTime := time.Now()
	response, err := client.Do(request)
	if err != nil {
		utils.DebugEvent("stage.reject", map[string]any{
			"error":   err.Error(),
			"ip":      ip.String(),
			"message": "追踪请求失败，淘汰该 IP。",
			"reason":  "trace_error",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"url": TraceURL,
			},
		})
		return traceProbeResult{reason: traceFailureRequest}
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxTraceBodyBytes))
	duration := time.Since(startTime)
	if readErr != nil {
		utils.DebugEvent("stage.reject", map[string]any{
			"error":   readErr.Error(),
			"ip":      ip.String(),
			"message": "追踪响应读取失败，淘汰该 IP。",
			"reason":  "trace_read_error",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"status_code": response.StatusCode,
				"url":         TraceURL,
			},
		})
		return traceProbeResult{reason: traceFailureRead}
	}
	if !isAcceptedHTTPingStatusCode(response.StatusCode) {
		utils.DebugEvent("stage.reject", map[string]any{
			"ip":      ip.String(),
			"message": "追踪状态码不匹配，淘汰该 IP。",
			"reason":  "status_mismatch",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"accepted_status_code": HttpingStatusCode,
				"status_code":          response.StatusCode,
				"url":                  TraceURL,
			},
		})
		return traceProbeResult{reason: traceFailureStatus}
	}
	return traceProbeResult{delay: duration, colo: ExtractColo(response.Header, body), ok: true}
}

func canFallbackToTCPCandidates() bool {
	return HeadMaxDelay <= 0 && HttpingStatusCode == 0 && HttpingCFColo == ""
}

func traceFallbackAllowedFor(reason traceFailureReason) bool {
	return reason == traceFailureRequest || reason == traceFailureRead
}

func filterConfiguredColo(colo string) string {
	if colo == "" {
		return ""
	}
	if HttpingCFColo == "" {
		return colo
	}
	if HttpingCFColomap == nil {
		HttpingCFColomap = MapColoMap()
	}
	if HttpingCFColomap == nil {
		return colo
	}
	_, ok := HttpingCFColomap.Load(colo)
	if ok {
		return colo
	}
	return ""
}
