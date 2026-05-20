package task

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"github.com/axuitomo/CFST-GUI/utils"
)

const (
	MaxHeadRoutines      = 30
	MaxTraceRoutines     = MaxHeadRoutines
	defaultHeadRoutines  = 6
	defaultHeadTestCount = 512
	defaultHeadTimeout   = time.Second
	defaultTraceURL      = "https://speed.cloudflare.com/cdn-cgi/trace"
	maxTraceBodyBytes    = 64 * 1024

	TraceColoModeStandard = "standard"
	TraceColoModeTraceURL = "trace_url"
)

var (
	HeadRoutines        = defaultHeadRoutines
	HeadTestCount       = defaultHeadTestCount
	HeadMaxDelay        time.Duration
	HeadTimeout         = defaultHeadTimeout
	TraceURL            = defaultTraceURL
	TraceColoMode       = TraceColoModeStandard
	TraceDiagnosticHook func(TraceDiagnostic)
	ColoDictionaryPath  string
	SourceColoFilters   SourceColoFilterMap

	traceProbeFunc = traceProbe

	coloDictionaryCache = struct {
		sync.Mutex
		path    string
		entries []colodict.ColoEntry
		modTime time.Time
		size    int64
	}{}
)

type SourceColoFilter struct {
	Mode         string
	Unrestricted bool
	Allowed      map[string]struct{}
	Denied       map[string]struct{}
}

type SourceColoFilterMap map[string]SourceColoFilter

type TraceDiagnostic struct {
	Colo         string
	Error        string
	IP           string
	Reason       string
	RetryAfterMS int64
	StatusCode   int
	URL          string
}

type traceEndpointResult struct {
	delay      time.Duration
	errorText  string
	colo       string
	ok         bool
	reason     traceFailureReason
	retryAfter time.Duration
	statusCode int
	url        string
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
	if HeadMaxDelay < 0 {
		HeadMaxDelay = 0
	}
	if HeadTimeout <= 0 {
		HeadTimeout = defaultHeadTimeout
	}
	if TraceURL == "" {
		TraceURL = defaultTraceURL
	}
	switch strings.ToLower(strings.TrimSpace(TraceColoMode)) {
	case "", TraceColoModeStandard:
		TraceColoMode = TraceColoModeStandard
	case TraceColoModeTraceURL, "trace-url", "traceurl":
		TraceColoMode = TraceColoModeTraceURL
	default:
		TraceColoMode = TraceColoModeStandard
	}
	if HttpingCFColo != "" && HttpingCFColomap == nil {
		HttpingCFColomap = MapColoMap()
	}
	HttpingCFColoMode = NormalizeColoFilterMode(HttpingCFColoMode)
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
		return candidateCount
	}
	if candidateCount < limit {
		return candidateCount
	}
	return limit
}

func NewSourceColoFilter(allowRaw string) SourceColoFilter {
	return NewSourceColoFilterWithMode(allowRaw, ColoFilterModeAllow)
}

func NewSourceColoFilterWithMode(raw string, mode string) SourceColoFilter {
	codes := ParseColoAllowList(raw)
	mode = NormalizeColoFilterMode(mode)
	if len(codes) == 0 {
		return SourceColoFilter{Unrestricted: true}
	}
	values := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		values[code] = struct{}{}
	}
	if mode == ColoFilterModeDeny {
		return SourceColoFilter{Mode: mode, Denied: values}
	}
	return SourceColoFilter{Mode: ColoFilterModeAllow, Allowed: values}
}

func MergeSourceColoFilters(target SourceColoFilterMap, ips []string, allowRaw string) {
	MergeSourceColoFiltersWithMode(target, ips, allowRaw, ColoFilterModeAllow)
}

func MergeSourceColoFiltersWithMode(target SourceColoFilterMap, ips []string, raw string, mode string) {
	if target == nil || len(ips) == 0 {
		return
	}
	incoming := NewSourceColoFilterWithMode(raw, mode)
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		existing := target[ip]
		if existing.Unrestricted || incoming.Unrestricted {
			target[ip] = SourceColoFilter{Unrestricted: true}
			continue
		}
		if existing.Mode == "" {
			existing.Mode = incoming.Mode
		}
		if existing.Mode != incoming.Mode {
			target[ip] = SourceColoFilter{Unrestricted: true}
			continue
		}
		if incoming.Mode == ColoFilterModeDeny {
			if existing.Denied == nil {
				existing.Denied = make(map[string]struct{})
			}
			for code := range incoming.Denied {
				existing.Denied[code] = struct{}{}
			}
		} else {
			if existing.Allowed == nil {
				existing.Allowed = make(map[string]struct{})
			}
			for code := range incoming.Allowed {
				existing.Allowed[code] = struct{}{}
			}
		}
		target[ip] = existing
	}
}

func CloneSourceColoFilterMap(source SourceColoFilterMap) SourceColoFilterMap {
	if len(source) == 0 {
		return nil
	}
	cloned := make(SourceColoFilterMap, len(source))
	for ip, filter := range source {
		next := SourceColoFilter{Mode: NormalizeColoFilterMode(filter.Mode), Unrestricted: filter.Unrestricted}
		if len(filter.Allowed) > 0 {
			next.Allowed = make(map[string]struct{}, len(filter.Allowed))
			for code := range filter.Allowed {
				next.Allowed[code] = struct{}{}
			}
		}
		if len(filter.Denied) > 0 {
			next.Denied = make(map[string]struct{}, len(filter.Denied))
			for code := range filter.Denied {
				next.Denied[code] = struct{}{}
			}
		}
		cloned[ip] = next
	}
	return cloned
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
				emitTraceDiagnostic(TraceDiagnostic{
					IP:         item.IP.String(),
					Reason:     string(traceFailureLatencyLimit),
					StatusCode: probe.statusCode,
					URL:        probe.url,
				})
			}
			if ok {
				originalColo := colo
				ok = sourceAllowsColo(item.IP, colo)
				if !ok {
					utils.DebugEvent("stage.reject", map[string]any{
						"colo":    originalColo,
						"ip":      item.IP.String(),
						"message": "追踪地区码不匹配输入源 COLO 白名单，淘汰该 IP。",
						"reason":  "source_colo_filter",
						"stage":   "stage2_trace",
					})
					emitTraceDiagnostic(TraceDiagnostic{
						Colo:       originalColo,
						IP:         item.IP.String(),
						Reason:     string(traceFailureSourceColoFilter),
						StatusCode: probe.statusCode,
						URL:        probe.url,
					})
				}
			}
			if ok && HttpingCFColo != "" {
				originalColo := colo
				colo, ok = configuredColoAllowed(colo)
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
					emitTraceDiagnostic(TraceDiagnostic{
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
			} else if traceSoftPassAllowedFor(probe) {
				results[index] = item
				passed[index] = true
				passedCount.Add(1)
				ok = true
				utils.DebugEvent("stage.detail", map[string]any{
					"ip":      item.IP.String(),
					"message": "追踪探测遇到临时异常，保留该 IP 进入后续文件测速。",
					"reason":  "trace_soft_pass",
					"stage":   "stage2_trace",
					"trace": map[string]any{
						"failure_reason": probe.reason,
						"retry_after_ms": probe.retryAfter.Milliseconds(),
						"status_code":    probe.statusCode,
						"url":            TraceURL,
					},
				})
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

func emitTraceDiagnostic(diagnostic TraceDiagnostic) {
	if TraceDiagnosticHook == nil {
		return
	}
	TraceDiagnosticHook(diagnostic)
}

func runTraceProbeWithRetry(ip *net.IPAddr) traceProbeResult {
	var result traceProbeResult
	attempt := 1
	stage := "stage2_trace"
	ipText := ip.String()
	for attempt <= retryAttemptLimit() {
		CheckProbePause(stage, ipText)
		if IsProbeCanceled(stage, ipText) {
			return traceProbeResult{errorText: "任务已取消", reason: traceFailureInterrupted}
		}
		result = traceProbeFunc(ip)
		if result.reason == traceFailureInterrupted {
			// 暂停打断不应被计作真实失败，恢复后重试当前 IP。
			CheckProbePause(stage, ipText)
			if IsProbeCanceled(stage, ipText) {
				return result
			}
			continue
		}
		if result.ok {
			return result
		}
		if attempt < retryAttemptLimit() {
			if result.reason == traceFailureRateLimited {
				sleepBeforeRateLimitRetry("stage2_trace", ip.String(), attempt, result.retryAfter)
			} else {
				sleepBeforeRetry("stage2_trace", ip.String(), attempt)
			}
		}
		attempt++
	}
	return result
}

func traceProbe(ip *net.IPAddr) traceProbeResult {
	ctx, cancel := context.WithTimeout(context.Background(), HeadTimeout)
	defer cancel()

	var clearTraceInterrupt func()
	if TraceInterruptHook != nil {
		clearTraceInterrupt = TraceInterruptHook("stage2_trace", ip.String(), cancel)
	}
	if clearTraceInterrupt != nil {
		defer clearTraceInterrupt()
	}

	profile := currentRequestProfile()
	client := httpclient.NewClient(httpclient.Options{
		Profile:               profile,
		DialContext:           httpclient.DirectDialContext(ip, TCPPort, profile),
		DialAddress:           profile.DialAddress(ip, TCPPort),
		DisableProxy:          true,
		Timeout:               HeadTimeout,
		ResponseHeaderTimeout: HeadTimeout,
		TLSHandshakeTimeout:   TCPConnectTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	})
	defer client.CloseIdleConnections()

	endpoints := traceEndpointsForIP(ip)
	var firstOK traceEndpointResult
	var hasFirstOK bool
	var last traceEndpointResult
	for _, endpoint := range endpoints {
		result := requestTraceEndpoint(ctx, client, profile, ip, endpoint)
		last = result
		if result.reason == traceFailureInterrupted {
			return traceProbeResult{
				errorText: result.errorText,
				reason:    result.reason,
				url:       result.url,
			}
		}
		if result.reason == traceFailureRateLimited {
			return traceProbeResult{
				errorText:  result.errorText,
				reason:     result.reason,
				retryAfter: result.retryAfter,
				statusCode: result.statusCode,
				url:        result.url,
			}
		}
		if !result.ok {
			continue
		}
		if result.colo != "" {
			return traceResultForColo(ip, result.delay, result.colo, result.statusCode, result.url)
		}
		if !hasFirstOK {
			firstOK = result
			hasFirstOK = true
		}
	}

	if colo := lookupColoFromDictionary(ip); colo != "" {
		delay := time.Duration(0)
		statusCode := 0
		if hasFirstOK {
			delay = firstOK.delay
			statusCode = firstOK.statusCode
		}
		return traceResultForColo(ip, delay, colo, statusCode, firstOK.url)
	}
	if hasFirstOK {
		if sourceRequiresColo(ip) {
			emitTraceDiagnostic(TraceDiagnostic{
				IP:         ip.String(),
				Reason:     string(traceFailureSourceColoFilter),
				StatusCode: firstOK.statusCode,
				URL:        firstOK.url,
			})
			return traceProbeResult{reason: traceFailureSourceColoFilter, statusCode: firstOK.statusCode, url: firstOK.url}
		}
		if HttpingCFColo != "" {
			if _, allowed := configuredColoAllowed(""); allowed {
				return traceProbeResult{delay: firstOK.delay, ok: true, statusCode: firstOK.statusCode, url: firstOK.url}
			}
			emitTraceDiagnostic(TraceDiagnostic{
				IP:         ip.String(),
				Reason:     string(traceFailureColoFilter),
				StatusCode: firstOK.statusCode,
				URL:        firstOK.url,
			})
			return traceProbeResult{reason: traceFailureColoFilter, statusCode: firstOK.statusCode, url: firstOK.url}
		}
		return traceProbeResult{delay: firstOK.delay, ok: true, statusCode: firstOK.statusCode, url: firstOK.url}
	}
	if last.reason != "" {
		return traceProbeResult{
			errorText:  last.errorText,
			reason:     last.reason,
			retryAfter: last.retryAfter,
			statusCode: last.statusCode,
			url:        last.url,
		}
	}
	return traceProbeResult{reason: traceFailureRequest}
}

type traceEndpoint struct {
	url        string
	allowCFRay bool
}

func traceEndpointsForIP(ip *net.IPAddr) []traceEndpoint {
	if TraceColoMode == TraceColoModeTraceURL {
		return []traceEndpoint{{url: TraceURL, allowCFRay: true}}
	}
	endpoints := []traceEndpoint{{url: traceIPLiteralURL(ip), allowCFRay: true}}
	if strings.TrimSpace(TraceURL) != "" && TraceURL != endpoints[0].url {
		endpoints = append(endpoints, traceEndpoint{url: TraceURL})
	}
	return endpoints
}

func traceIPLiteralURL(ip *net.IPAddr) string {
	scheme := "https"
	if parsed, err := url.Parse(strings.TrimSpace(TraceURL)); err == nil && strings.EqualFold(parsed.Scheme, "http") {
		scheme = "http"
	}
	host := ip.String()
	if addr, err := netip.ParseAddr(ip.String()); err == nil && addr.Is6() {
		host = "[" + addr.String() + "]"
	}
	return (&url.URL{Scheme: scheme, Host: host, Path: "/cdn-cgi/trace"}).String()
}

func requestTraceEndpoint(ctx context.Context, client *http.Client, profile httpcfg.Profile, ip *net.IPAddr, endpoint traceEndpoint) traceEndpointResult {
	request, err := http.NewRequest(http.MethodGet, endpoint.url, nil)
	if err != nil {
		utils.DebugEvent("stage.reject", map[string]any{
			"error":   err.Error(),
			"ip":      ip.String(),
			"message": "追踪请求创建失败，淘汰该 IP。",
			"reason":  "request_create_failed",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"url": endpoint.url,
			},
		})
		emitTraceDiagnostic(TraceDiagnostic{
			Error:  err.Error(),
			IP:     ip.String(),
			Reason: string(traceFailureRequestCreate),
			URL:    endpoint.url,
		})
		return traceEndpointResult{errorText: err.Error(), reason: traceFailureRequestCreate, url: endpoint.url}
	}
	request = request.WithContext(ctx)
	profile.Apply(request)
	request.Header.Set("Connection", "close")
	request.Close = true

	startTime := time.Now()
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return traceEndpointResult{
				errorText: ctx.Err().Error(),
				reason:    traceFailureInterrupted,
				url:       endpoint.url,
			}
		}
		utils.DebugEvent("stage.reject", map[string]any{
			"error":   err.Error(),
			"ip":      ip.String(),
			"message": "追踪请求失败，淘汰该 IP。",
			"reason":  "trace_error",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"url": endpoint.url,
			},
		})
		emitTraceDiagnostic(TraceDiagnostic{
			Error:  err.Error(),
			IP:     ip.String(),
			Reason: string(traceFailureRequest),
			URL:    endpoint.url,
		})
		return traceEndpointResult{errorText: err.Error(), reason: traceFailureRequest, url: endpoint.url}
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxTraceBodyBytes))
	duration := time.Since(startTime)
	if readErr != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return traceEndpointResult{
				errorText:  ctx.Err().Error(),
				reason:     traceFailureInterrupted,
				statusCode: response.StatusCode,
				url:        endpoint.url,
			}
		}
		utils.DebugEvent("stage.reject", map[string]any{
			"error":   readErr.Error(),
			"ip":      ip.String(),
			"message": "追踪响应读取失败，淘汰该 IP。",
			"reason":  "trace_read_error",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"status_code": response.StatusCode,
				"url":         endpoint.url,
			},
		})
		emitTraceDiagnostic(TraceDiagnostic{
			Error:      readErr.Error(),
			IP:         ip.String(),
			Reason:     string(traceFailureRead),
			StatusCode: response.StatusCode,
			URL:        endpoint.url,
		})
		return traceEndpointResult{errorText: readErr.Error(), reason: traceFailureRead, statusCode: response.StatusCode, url: endpoint.url}
	}
	if response.StatusCode == http.StatusTooManyRequests {
		retryAfter := retryAfterDelay(response.Header.Get("Retry-After"), time.Now())
		utils.DebugEvent("stage.reject", map[string]any{
			"ip":      ip.String(),
			"message": "追踪请求触发服务端限流，淘汰该 IP。",
			"reason":  "rate_limited",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"retry_after_ms": retryAfter.Milliseconds(),
				"status_code":    response.StatusCode,
				"url":            endpoint.url,
			},
		})
		emitTraceDiagnostic(TraceDiagnostic{
			IP:           ip.String(),
			Reason:       string(traceFailureRateLimited),
			RetryAfterMS: retryAfter.Milliseconds(),
			StatusCode:   response.StatusCode,
			URL:          endpoint.url,
		})
		return traceEndpointResult{reason: traceFailureRateLimited, retryAfter: retryAfter, statusCode: response.StatusCode, url: endpoint.url}
	}

	bodyColo := ExtractColoFromTraceBody(body)
	rayColo := ""
	if endpoint.allowCFRay {
		rayColo = ExtractColoFromCFRay(response.Header.Get("cf-ray"))
	}
	if response.StatusCode == http.StatusForbidden && rayColo != "" {
		return traceEndpointResult{delay: duration, colo: rayColo, ok: true, statusCode: response.StatusCode, url: endpoint.url}
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
				"url":                  endpoint.url,
			},
		})
		emitTraceDiagnostic(TraceDiagnostic{
			IP:         ip.String(),
			Reason:     string(traceFailureStatus),
			StatusCode: response.StatusCode,
			URL:        endpoint.url,
		})
		return traceEndpointResult{reason: traceFailureStatus, statusCode: response.StatusCode, url: endpoint.url}
	}
	if bodyColo != "" {
		return traceEndpointResult{delay: duration, colo: bodyColo, ok: true, statusCode: response.StatusCode, url: endpoint.url}
	}
	return traceEndpointResult{delay: duration, colo: rayColo, ok: true, statusCode: response.StatusCode, url: endpoint.url}
}

func traceResultForColo(ip *net.IPAddr, delay time.Duration, colo string, statusCode int, rawURL string) traceProbeResult {
	colo = normalizeColoCode(colo)
	if colo == "" {
		return traceProbeResult{delay: delay, ok: true, statusCode: statusCode, url: rawURL}
	}
	if !sourceAllowsColo(ip, colo) {
		utils.DebugEvent("stage.reject", map[string]any{
			"colo":    colo,
			"ip":      ip.String(),
			"message": "追踪地区码不匹配输入源 COLO 白名单，淘汰该 IP。",
			"reason":  "source_colo_filter",
			"stage":   "stage2_trace",
		})
		emitTraceDiagnostic(TraceDiagnostic{
			Colo:       colo,
			IP:         ip.String(),
			Reason:     string(traceFailureSourceColoFilter),
			StatusCode: statusCode,
			URL:        rawURL,
		})
		return traceProbeResult{delay: delay, colo: colo, reason: traceFailureSourceColoFilter, statusCode: statusCode, url: rawURL}
	}
	filteredColo, allowed := configuredColoAllowed(colo)
	if !allowed {
		utils.DebugEvent("stage.reject", map[string]any{
			"colo":    colo,
			"ip":      ip.String(),
			"message": "追踪地区码不匹配，淘汰该 IP。",
			"reason":  "colo_filter",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"expected_colo": HttpingCFColo,
			},
		})
		emitTraceDiagnostic(TraceDiagnostic{
			Colo:       colo,
			IP:         ip.String(),
			Reason:     string(traceFailureColoFilter),
			StatusCode: statusCode,
			URL:        rawURL,
		})
		return traceProbeResult{delay: delay, colo: colo, reason: traceFailureColoFilter, statusCode: statusCode, url: rawURL}
	}
	return traceProbeResult{delay: delay, colo: filteredColo, ok: true, statusCode: statusCode, url: rawURL}
}

func canFallbackToTCPCandidates() bool {
	return HeadMaxDelay <= 0 && HttpingStatusCode == 0 && HttpingCFColo == "" && len(SourceColoFilters) == 0
}

func traceFallbackAllowedFor(reason traceFailureReason) bool {
	return reason == traceFailureRequest || reason == traceFailureRead
}

func traceSoftPassAllowedFor(result traceProbeResult) bool {
	if HeadMaxDelay > 0 || HttpingCFColo != "" || len(SourceColoFilters) > 0 {
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

func filterConfiguredColo(colo string) string {
	filtered, ok := configuredColoAllowed(colo)
	if ok {
		return filtered
	}
	return ""
}

func configuredColoAllowed(colo string) (string, bool) {
	if HttpingCFColo == "" {
		return colo, true
	}
	if HttpingCFColomap == nil {
		HttpingCFColomap = MapColoMap()
	}
	if HttpingCFColomap == nil {
		return colo, true
	}
	mode := NormalizeColoFilterMode(HttpingCFColoMode)
	colo = normalizeColoCode(colo)
	if colo == "" {
		if mode == ColoFilterModeDeny {
			return "", true
		}
		return "", false
	}
	_, ok := HttpingCFColomap.Load(colo)
	if mode == ColoFilterModeDeny {
		if ok {
			return "", false
		}
		return colo, true
	}
	if ok {
		return colo, true
	}
	return "", false
}

func sourceAllowsColo(ip *net.IPAddr, colo string) bool {
	filter, ok := SourceColoFilters[ip.String()]
	if !ok || filter.Unrestricted {
		return true
	}
	mode := NormalizeColoFilterMode(filter.Mode)
	colo = normalizeColoCode(colo)
	if colo == "" {
		return mode == ColoFilterModeDeny || len(filter.Allowed) == 0
	}
	if mode == ColoFilterModeDeny {
		if len(filter.Denied) == 0 {
			return true
		}
		_, ok = filter.Denied[colo]
		return !ok
	}
	if len(filter.Allowed) == 0 {
		return true
	}
	_, ok = filter.Allowed[colo]
	return ok
}

func sourceRequiresColo(ip *net.IPAddr) bool {
	filter, ok := SourceColoFilters[ip.String()]
	if !ok || filter.Unrestricted {
		return false
	}
	if NormalizeColoFilterMode(filter.Mode) == ColoFilterModeDeny {
		return false
	}
	return len(filter.Allowed) > 0
}

func lookupColoFromDictionary(ip *net.IPAddr) string {
	path := strings.TrimSpace(ColoDictionaryPath)
	if path == "" {
		return ""
	}
	entries, err := cachedColoDictionaryEntries(path)
	if err != nil {
		utils.DebugEvent("stage.detail", map[string]any{
			"error":   err.Error(),
			"ip":      ip.String(),
			"message": "COLO 词典兜底不可用。",
			"reason":  "colo_dictionary_unavailable",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"colo_dictionary_path": path,
			},
		})
		return ""
	}
	return normalizeColoCode(colodict.LookupColo(entries, ip.String()))
}

func cachedColoDictionaryEntries(path string) ([]colodict.ColoEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	coloDictionaryCache.Lock()
	if coloDictionaryCache.path == path &&
		coloDictionaryCache.size == info.Size() &&
		coloDictionaryCache.modTime.Equal(info.ModTime()) &&
		coloDictionaryCache.entries != nil {
		entries := coloDictionaryCache.entries
		coloDictionaryCache.Unlock()
		return entries, nil
	}
	coloDictionaryCache.Unlock()

	entries, err := colodict.LoadColoEntries(path)
	if err != nil {
		return nil, err
	}
	coloDictionaryCache.Lock()
	coloDictionaryCache.path = path
	coloDictionaryCache.size = info.Size()
	coloDictionaryCache.modTime = info.ModTime()
	coloDictionaryCache.entries = entries
	coloDictionaryCache.Unlock()
	return entries, nil
}
