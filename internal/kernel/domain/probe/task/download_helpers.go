package task

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
)

func newDownloadRequest(ctx context.Context, profile httpcfg.Profile, rawURL, rangeHeader string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	profile.Apply(req)
	httpclient.ApplyNoCache(req)
	req.Header.Set("Accept-Encoding", "identity")
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
