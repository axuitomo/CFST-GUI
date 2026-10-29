package task

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func edgeRestrictedTestEngine(t *testing.T, port int, ignore bool) *Engine {
	t.Helper()
	config := DefaultConfig()
	config.Disable = false
	config.TestCount = 1
	config.URL = "http://127.0.0.1:" + strconv.Itoa(port) + "/500m"
	config.Timeout = 2 * time.Second
	config.TCPPort = port
	config.DownloadWarmupDuration = 0
	config.DownloadSampleInterval = time.Second
	config.Ignore1034 = ignore
	return NewEngine(config, Hooks{})
}

func TestDownloadIgnoresCloudflareEdgeRestricted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("error code: 1034\n"))
	}))
	defer srv.Close()
	port, err := strconv.Atoi(srv.URL[strings.LastIndex(srv.URL, ":")+1:])
	if err != nil {
		t.Fatal(err)
	}
	ip := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}

	engine := edgeRestrictedTestEngine(t, port, true)
	result, err := engine.downloadHandlerAttemptOnce(ip, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.reason != downloadReasonEdgeRestricted || !result.ignored || result.retryable {
		t.Fatalf("result = %#v, want the ignored edge_ip_restricted outcome", result)
	}

	engine = edgeRestrictedTestEngine(t, port, false)
	result, err = engine.downloadHandlerAttemptOnce(ip, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.reason != "download_no_body_read" || result.ignored {
		t.Fatalf("result = %#v, want the unchanged no-body-read outcome when the option is off", result)
	}
}
