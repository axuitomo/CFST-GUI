package task

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
)

type traceEndpointResult struct {
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

type traceEndpoint struct {
	url        string
	allowCFRay bool
}

func (e *Engine) traceProbeIP(ip *net.IPAddr) traceProbeResult {
	ctx, cancel := context.WithTimeout(e.context(), e.config.HeadTimeout)
	defer cancel()

	var clearTraceInterrupt func()
	if e.hooks.TraceInterrupt != nil {
		clearTraceInterrupt = e.hooks.TraceInterrupt("stage2_trace", ip.String(), cancel)
	}
	if clearTraceInterrupt != nil {
		defer clearTraceInterrupt()
	}

	profile := e.currentRequestProfile()
	client := httpclient.NewClient(httpclient.Options{
		// 追踪是小文本请求：QUIC 握手失败（UDP 443 被封或被代理截走）会吃满
		// HeadTimeout，TCP 回退拿到的 context 已经过期，整批 IP 按超时淘汰。
		// 与来源抓取一致，固定走 TCP，不让 auto 的 H3 抢预算。
		Protocol:              httpclient.ProtocolTCP,
		Profile:               profile,
		DialContext:           httpclient.DirectDialContext(ip, e.config.TCPPort, profile),
		DialAddress:           profile.DialAddress(ip, e.config.TCPPort),
		DisableProxy:          true,
		Timeout:               e.config.HeadTimeout,
		ResponseHeaderTimeout: e.config.HeadTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout(e.config.TCPConnectTimeout),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	})
	defer httpclient.Close(client)

	endpoints := e.traceEndpointsForIP(ip)
	var firstOK traceEndpointResult
	var hasFirstOK bool
	var last traceEndpointResult
	for _, endpoint := range endpoints {
		result := e.requestTraceEndpoint(ctx, client, profile, ip, endpoint)
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
		if result.reason == traceFailureEdgeRestricted {
			// 1034 是这个 IP 在边缘被限制，换端点还是同一个边缘，直接收口省掉剩余预算。
			return traceProbeResult{
				reason:     result.reason,
				ignored:    result.ignored,
				statusCode: result.statusCode,
				url:        result.url,
			}
		}
		if !result.ok {
			continue
		}
		if result.colo != "" {
			return e.traceResultForColo(ip, result.delay, result.colo, result.statusCode, result.url)
		}
		if !hasFirstOK {
			firstOK = result
			hasFirstOK = true
		}
	}

	if colo := e.lookupColoFromDictionary(ip); colo != "" {
		delay := time.Duration(0)
		statusCode := 0
		if hasFirstOK {
			delay = firstOK.delay
			statusCode = firstOK.statusCode
		}
		return e.traceResultForColo(ip, delay, colo, statusCode, firstOK.url)
	}
	if hasFirstOK {
		if e.sourceRequiresColo(ip) {
			e.emitTraceDiagnostic(TraceDiagnostic{
				IP:         ip.String(),
				Reason:     string(traceFailureSourceColoFilter),
				StatusCode: firstOK.statusCode,
				URL:        firstOK.url,
			})
			return traceProbeResult{reason: traceFailureSourceColoFilter, statusCode: firstOK.statusCode, url: firstOK.url}
		}
		if e.config.TraceCFColo != "" {
			if _, allowed := e.configuredColoAllowed(""); allowed {
				return traceProbeResult{delay: firstOK.delay, ok: true, statusCode: firstOK.statusCode, url: firstOK.url}
			}
			e.emitTraceDiagnostic(TraceDiagnostic{
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

func (e *Engine) traceEndpointsForIP(ip *net.IPAddr) []traceEndpoint {
	if e.config.TraceColoMode == TraceColoModeTraceURL {
		return []traceEndpoint{{url: e.config.TraceURL, allowCFRay: true}}
	}
	endpoints := []traceEndpoint{{url: e.traceIPLiteralURL(ip), allowCFRay: true}}
	if strings.TrimSpace(e.config.TraceURL) != "" && e.config.TraceURL != endpoints[0].url {
		endpoints = append(endpoints, traceEndpoint{url: e.config.TraceURL})
	}
	return endpoints
}

func (e *Engine) traceIPLiteralURL(ip *net.IPAddr) string {
	scheme := "https"
	if parsed, err := url.Parse(strings.TrimSpace(e.config.TraceURL)); err == nil && strings.EqualFold(parsed.Scheme, "http") {
		scheme = "http"
	}
	host := ip.String()
	if addr, err := netip.ParseAddr(ip.String()); err == nil && addr.Is6() {
		host = "[" + addr.String() + "]"
	}
	return (&url.URL{Scheme: scheme, Host: host, Path: "/cdn-cgi/trace"}).String()
}

func (e *Engine) requestTraceEndpoint(ctx context.Context, client *http.Client, profile httpcfg.Profile, ip *net.IPAddr, endpoint traceEndpoint) traceEndpointResult {
	request, err := http.NewRequest(http.MethodGet, endpoint.url, nil)
	if err != nil {
		e.debugEvent("stage.reject", map[string]any{
			"error":   err.Error(),
			"ip":      ip.String(),
			"message": "追踪请求创建失败，淘汰该 IP。",
			"reason":  "request_create_failed",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"url": endpoint.url,
			},
		})
		e.emitTraceDiagnostic(TraceDiagnostic{
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
		e.debugEvent("stage.reject", map[string]any{
			"error":   err.Error(),
			"ip":      ip.String(),
			"message": "追踪请求失败，淘汰该 IP。",
			"reason":  "trace_error",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"url": endpoint.url,
			},
		})
		e.emitTraceDiagnostic(TraceDiagnostic{
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
	if duration <= 0 {
		duration = time.Nanosecond
	}
	if readErr != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return traceEndpointResult{
				errorText:  ctx.Err().Error(),
				reason:     traceFailureInterrupted,
				statusCode: response.StatusCode,
				url:        endpoint.url,
			}
		}
		e.debugEvent("stage.reject", map[string]any{
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
		e.emitTraceDiagnostic(TraceDiagnostic{
			Error:      readErr.Error(),
			IP:         ip.String(),
			Reason:     string(traceFailureRead),
			StatusCode: response.StatusCode,
			URL:        endpoint.url,
		})
		return traceEndpointResult{errorText: readErr.Error(), reason: traceFailureRead, statusCode: response.StatusCode, url: endpoint.url}
	}
	// 追踪阶段就认出 1034：CF 边缘 IP 受限是 403 + 正文标记（纯文本 "error code: 1034"，
	// 或品牌错误页的 "Edge IP Restricted" / "errorCode: 1034"），与普通 403 只差正文，
	// 正文已经读在这里，判定完直接淘汰，不用等文件测速再发现。
	if response.StatusCode == http.StatusForbidden && isEdgeRestrictedBody(body) {
		rejection := EdgeRejection{
			Stage:   "stage2_trace",
			IP:      ip.String(),
			Note:    EdgeNote1034,
			Reason:  downloadReasonEdgeRestricted,
			Ignored: true,
		}
		e.debugEvent("stage.reject", edgeRejectFields(endpoint.url, rejection))
		e.noteEdgeRejected(rejection)
		e.emitTraceDiagnostic(TraceDiagnostic{
			IP:         ip.String(),
			Reason:     string(traceFailureEdgeRestricted),
			StatusCode: response.StatusCode,
			URL:        endpoint.url,
		})
		return traceEndpointResult{
			reason:     traceFailureEdgeRestricted,
			ignored:    rejection.Ignored,
			statusCode: response.StatusCode,
			url:        endpoint.url,
		}
	}
	if response.StatusCode == http.StatusTooManyRequests {
		retryAfter := retryAfterDelay(response.Header.Get("Retry-After"), time.Now())
		e.debugEvent("stage.reject", map[string]any{
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
		e.emitTraceDiagnostic(TraceDiagnostic{
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
	if !e.isAcceptedTraceStatusCode(response.StatusCode) {
		e.debugEvent("stage.reject", map[string]any{
			"ip":      ip.String(),
			"message": "追踪状态码不匹配，淘汰该 IP。",
			"reason":  "status_mismatch",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"accepted_status_code": e.config.TraceStatusCode,
				"status_code":          response.StatusCode,
				"url":                  endpoint.url,
			},
		})
		e.emitTraceDiagnostic(TraceDiagnostic{
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

func (e *Engine) traceResultForColo(ip *net.IPAddr, delay time.Duration, colo string, statusCode int, rawURL string) traceProbeResult {
	colo = normalizeColoCode(colo)
	if colo == "" {
		return traceProbeResult{delay: delay, ok: true, statusCode: statusCode, url: rawURL}
	}
	if !e.sourceAllowsColo(ip, colo) {
		e.debugEvent("stage.reject", map[string]any{
			"colo":    colo,
			"ip":      ip.String(),
			"message": "追踪地区码不匹配输入源 COLO 白名单，淘汰该 IP。",
			"reason":  "source_colo_filter",
			"stage":   "stage2_trace",
		})
		e.emitTraceDiagnostic(TraceDiagnostic{
			Colo:       colo,
			IP:         ip.String(),
			Reason:     string(traceFailureSourceColoFilter),
			StatusCode: statusCode,
			URL:        rawURL,
		})
		return traceProbeResult{delay: delay, colo: colo, reason: traceFailureSourceColoFilter, statusCode: statusCode, url: rawURL}
	}
	filteredColo, allowed := e.configuredColoAllowed(colo)
	if !allowed {
		e.debugEvent("stage.reject", map[string]any{
			"colo":    colo,
			"ip":      ip.String(),
			"message": "追踪地区码不匹配，淘汰该 IP。",
			"reason":  "colo_filter",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"expected_colo": e.config.TraceCFColo,
			},
		})
		e.emitTraceDiagnostic(TraceDiagnostic{
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
