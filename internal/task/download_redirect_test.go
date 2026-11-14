package task

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// range worker 共享同一个 client，重定向回调会被多个 goroutine 同时调用。
// 这个测试走真实的并发重定向路径：记录重定向目标时若不加锁，-race 会直接报竞争，
// 且日志里的重定向目标可能对不上实际请求。
func TestDownloadRangeWorkersShareClientAcrossRedirects(t *testing.T) {
	body := []byte(strings.Repeat("x", 8192))
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("hop") == "" {
			http.Redirect(w, r, r.URL.Path+"?"+r.URL.RawQuery+"&hop=1", http.StatusFound)
			return
		}
		start, end, ok := parseRangeHeaderForTest(r.Header.Get("Range"))
		if !ok || start < 0 || end < start || end >= int64(len(body)) {
			http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		active := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			observed := maxInFlight.Load()
			if active <= observed || maxInFlight.CompareAndSwap(observed, active) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(body[start : end+1])
	}))
	defer srv.Close()

	ip, port, downloadURL := probeServerEndpoint(t, srv.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 300 * time.Millisecond
	config.DownloadGetConcurrency = 4
	config.DownloadBufferKB = 64
	config.DownloadHTTPProtocol = "tcp"
	config.DownloadSampleInterval = time.Millisecond
	config.DownloadWarmupDuration = 0
	engine := NewEngine(config, Hooks{})

	if speed := engine.downloadHandlerAttempt(ip).speed; speed <= 0 {
		t.Fatalf("speed = %f, want positive speed through same-origin redirects", speed)
	}
	if maxInFlight.Load() < 2 {
		t.Fatalf("concurrent redirected range requests = %d, want at least 2", maxInFlight.Load())
	}
}
