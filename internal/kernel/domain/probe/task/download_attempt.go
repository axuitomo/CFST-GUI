package task

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
)

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

// newEdgeRejection 把边缘拒绝原因翻译成共用事实。reason 不属于 403 / 1034 时返回 false，
// 调用方不要为普通状态码不匹配发拒绝事件。
func newEdgeRejection(stage, ip, reason string, ignored bool) (EdgeRejection, bool) {
	note := edgeNoteForReason(reason)
	if note == "" {
		return EdgeRejection{}, false
	}
	return EdgeRejection{Stage: stage, IP: ip, Note: note, Reason: reason, Ignored: ignored}, true
}

// edgeRejectMessage 给界面日志流用的中文说明。
func edgeRejectMessage(reason EdgeRejection) string {
	scope := "文件测速"
	switch reason.Stage {
	case "stage1_tcp":
		scope = "HTTPing 探测"
	case "stage2_trace":
		scope = "追踪探测"
	}
	message := scope + "被 HTTP 403 拒绝"
	if reason.Reason == downloadReasonEdgeRestricted {
		message = scope + "被 Cloudflare 边缘 IP 限制拒绝（403 error code 1034）"
	}
	return message + "，直接淘汰：不重试、不计入连续失败冷却。"
}

// edgeRejectFields 是调试日志的 stage.reject 字段。
func edgeRejectFields(url string, rejection EdgeRejection) map[string]any {
	ip := net.ParseIP(rejection.IP)
	fields := downloadDebugFields(&net.IPAddr{IP: ip}, nil, http.StatusForbidden, url, "", rejection.Reason, edgeRejectMessage(rejection))
	fields["ignored"] = rejection.Ignored
	fields["note"] = rejection.Note
	return fields
}

// edgeNoteForReason 把边缘拒绝原因翻译成日志里的短标识，其他原因返回空串。
func edgeNoteForReason(reason string) string {
	switch reason {
	case downloadReasonEdgeRestricted:
		return EdgeNote1034
	case downloadReasonForbidden:
		return EdgeNote403
	}
	return ""
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

// downloadRedirectRecorder 记录最后一次重定向目标，只用于调试输出。
// range worker 共享同一个 client，CheckRedirect 会被多个 goroutine 并发调用，
// 直接写字符串是数据竞争，可能让日志里的重定向目标对不上。
type downloadRedirectRecorder struct {
	mu  sync.Mutex
	url string
}

func (r *downloadRedirectRecorder) set(value string) {
	r.mu.Lock()
	r.url = value
	r.mu.Unlock()
}

func (r *downloadRedirectRecorder) get() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.url
}

func (e *Engine) downloadHandlerAttemptOnce(ip *net.IPAddr, attempt int) (downloadResult, error) {
	config := e.config
	profile := e.downloadRequestProfile()
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

	var lastRedirectURL downloadRedirectRecorder // 用于记录最后一次重定向目标，以便在访问错误时输出
	client := httpclient.NewClient(httpclient.Options{
		Protocol:              httpclient.NormalizeProtocol(config.DownloadHTTPProtocol, httpclient.ProtocolAuto),
		Profile:               profile,
		DialContext:           httpclient.DirectDialContext(ip, config.TCPPort, profile),
		DialAddress:           profile.DialAddress(ip, config.TCPPort),
		DisableProxy:          true,
		ResponseHeaderTimeout: config.Timeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout(config.TCPConnectTimeout),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			lastRedirectURL.set(req.URL.String()) // 记录每次重定向的目标，以便在访问错误时输出
			if !sameRequestOrigin(config.URL, req.URL.String()) {
				return http.ErrUseLastResponse
			}
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
	defer httpclient.Close(client)

	measurement := e.newDownloadMeasurement(ip, attempt)
	measurement.giveUp = cancelTimeout
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
				e.printDownloadDebugInfo(ip, nil, statusErr.statusCode, config.URL, lastRedirectURL.get(), nil)
			}
			if statusErr.statusCode == http.StatusTooManyRequests {
				return invalidDownloadResultWithRetryAfter("rate_limited", true, statusErr.retryAfter), nil
			}
			return invalidDownloadResult("status_mismatch", true), nil
		}
		if contentErr := (downloadContentError{}); errors.As(probeErr, &contentErr) {
			e.debugEvent("stage.reject", downloadDebugFields(ip, nil, contentErr.statusCode, config.URL, lastRedirectURL.get(), contentErr.reason, "文件测速响应内容异常，淘汰该 IP。"))
			return invalidDownloadResult(contentErr.reason, true), nil
		}
		if requestErr := (downloadRequestCreateError{}); errors.As(probeErr, &requestErr) {
			e.debugEvent("stage.reject", downloadDebugFields(ip, requestErr.err, 0, config.URL, "", "request_create_failed", "文件测速请求创建失败，淘汰该 IP。"))
			return invalidDownloadResult("request_create_failed", false), nil
		}
		if ctx.Err() != nil {
			return invalidDownloadResult("download_interrupted", true), errDownloadInterrupted
		}
		if config.Debug {
			e.printDownloadDebugInfo(ip, probeErr, 0, config.URL, lastRedirectURL.get(), nil)
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
	if reason, ignored := measurement.rejectedOutcome(); reason != "" {
		// 调试日志记详细字段，界面事件流同时收到一条不受 debug 开关约束的拒绝事件。
		if rejection, isEdge := newEdgeRejection("stage3_get", ip.String(), reason, ignored); isEdge {
			e.debugEvent("stage.reject", edgeRejectFields(e.config.URL, rejection))
			e.noteEdgeRejected(rejection)
		}
		if ignored {
			return ignoredDownloadResult(reason), nil
		}
		// 未屏蔽的 403/1034 按普通失败处理：可重试，并计入连续失败冷却。
		return invalidDownloadResult(reason, true), nil
	}
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

type downloadContentError struct {
	reason     string
	statusCode int
}

func (e downloadContentError) Error() string {
	return "unexpected download response content: " + e.reason
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
