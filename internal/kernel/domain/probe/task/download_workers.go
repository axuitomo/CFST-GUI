package task

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpcfg"
)

// isEdgeRestrictedBody 判断 403 正文是不是 Cloudflare 的边缘 IP 受限页。追踪阶段读到的是
// 纯文本短句，文件测速读到的是品牌错误页，两种都要认出来。
func isEdgeRestrictedBody(body []byte) bool {
	lower := strings.ToLower(string(body))
	for _, marker := range downloadEdgeRestrictedMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// cloudflare403Outcome 给 403 响应定性，只读少量正文，响应体仍由调用方关闭。
// 403 一律按屏蔽处理（不重试、不计入连续失败冷却）：正文含 1034 标记记为边缘 IP 限制，
// 其余记为普通 403。两种情况都不保留 IP：没测到速率的候选一律丢弃。
func (e *Engine) cloudflare403Outcome(response *http.Response) (string, bool) {
	if response == nil || response.StatusCode != http.StatusForbidden || response.Body == nil {
		return "", false
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, downloadEdgeRestrictedSniffBytes))
	if err == nil && isEdgeRestrictedBody(body) {
		return downloadReasonEdgeRestricted, true
	}
	return downloadReasonForbidden, true
}

func (e *Engine) probeDownloadRange(ctx context.Context, client *http.Client, profile httpcfg.Profile) (downloadRangeProbe, error) {
	probeRange := "bytes=0-" + strconv.Itoa(downloadContentSniffBytes-1)
	req, err := e.newDownloadRequest(ctx, profile, e.downloadURLWithNonce("probe"), probeRange)
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
		start, end, total, ok := parseContentRange(response.Header.Get("Content-Range"))
		if !ok || start != 0 || total <= 0 {
			return downloadRangeProbe{supported: false}, nil
		}
		expectedPrefix := min(total, int64(downloadContentSniffBytes))
		prefixLength, reason := inspectDownloadProbeContent(response)
		if reason != "" {
			return downloadRangeProbe{}, downloadContentError{reason: reason, statusCode: response.StatusCode}
		}
		if end-start+1 < expectedPrefix || int64(prefixLength) < expectedPrefix {
			return downloadRangeProbe{supported: false}, nil
		}
		return downloadRangeProbe{supported: true, totalSize: total}, nil
	}
	if response.StatusCode == http.StatusOK {
		if _, reason := inspectDownloadProbeContent(response); reason != "" {
			return downloadRangeProbe{}, downloadContentError{reason: reason, statusCode: response.StatusCode}
		}
		return downloadRangeProbe{supported: false}, nil
	}
	if response.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return downloadRangeProbe{supported: false}, nil
	}
	if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusNotFound {
		// Some download endpoints reject Range probes while accepting a normal GET.
		return downloadRangeProbe{supported: false}, nil
	}
	return downloadRangeProbe{}, downloadStatusError{
		statusCode: response.StatusCode,
		retryAfter: retryAfterDelay(response.Header.Get("Retry-After"), time.Now()),
	}
}

func inspectDownloadProbeContent(response *http.Response) (int, string) {
	if response == nil {
		return 0, ""
	}
	prefix, _ := io.ReadAll(io.LimitReader(response.Body, downloadContentSniffBytes))
	return len(prefix), unexpectedDownloadContentReason(response.Header.Get("Content-Type"), prefix)
}

func unexpectedDownloadContentReason(contentType string, prefix []byte) string {
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		return "unexpected_html_response"
	}
	if len(prefix) > 0 {
		detected := strings.ToLower(strings.TrimSpace(strings.SplitN(http.DetectContentType(prefix), ";", 2)[0]))
		if detected == "text/html" || detected == "application/xhtml+xml" {
			return "unexpected_html_response"
		}
	}
	return ""
}

func (e *Engine) runDownloadRangeWorkers(ctx context.Context, client *http.Client, profile httpcfg.Profile, ip *net.IPAddr, measurement *downloadMeasurement, probe downloadRangeProbe) {
	workerCount := e.config.DownloadGetConcurrency
	if workerCount <= 0 {
		workerCount = defaultDownloadGetConcurrency
	}
	if probe.totalSize > 0 && probe.totalSize <= int64(math.MaxInt) && int64(workerCount) > probe.totalSize {
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
				if !measurement.hasValidMeasurement() {
					if edgeReason, ignored := e.cloudflare403Outcome(response); edgeReason != "" {
						measurement.markRejected(edgeReason, ignored)
						reason = edgeReason
					}
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
			if !measurement.hasValidMeasurement() {
				if edgeReason, ignored := e.cloudflare403Outcome(response); edgeReason != "" {
					measurement.markRejected(edgeReason, ignored)
					reason = edgeReason
				}
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
				if reason := integrity.streamFailureReason(); reason != "" {
					return offset, reason
				}
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
