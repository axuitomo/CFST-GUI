package task

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/utils"
)

func TestTCPCheckConnectionSkipsFirstSample(t *testing.T) {
	oldPingTimes := PingTimes
	oldSkipFirst := SkipFirstLatencySample
	oldHttping := Httping
	t.Cleanup(func() {
		PingTimes = oldPingTimes
		SkipFirstLatencySample = oldSkipFirst
		Httping = oldHttping
	})

	PingTimes = 4
	SkipFirstLatencySample = true
	Httping = false
	delays := []time.Duration{
		999 * time.Millisecond,
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
	}
	index := 0
	ping := &Ping{
		tcpProbe: func(ip *net.IPAddr) (bool, time.Duration) {
			delay := delays[index]
			index++
			return true, delay
		},
	}

	sent, received, totalDelay, _ := ping.checkConnection(parseTestIP("1.1.1.1"))
	if sent != 4 {
		t.Fatalf("sent = %d, want 4 measured samples", sent)
	}
	if received != 4 {
		t.Fatalf("received = %d, want 4", received)
	}
	if totalDelay != 100*time.Millisecond {
		t.Fatalf("totalDelay = %v, want 100ms", totalDelay)
	}
}

func TestPingDefaultRejectsSinglePingTime(t *testing.T) {
	oldPingTimes := PingTimes
	t.Cleanup(func() {
		PingTimes = oldPingTimes
	})

	PingTimes = 1
	checkPingDefault()
	if PingTimes != MinPingTimes {
		t.Fatalf("PingTimes = %d, want minimum %d", PingTimes, MinPingTimes)
	}
}

func TestTraceAvailabilityConcurrencyIsCappedAtSix(t *testing.T) {
	oldHeadRoutines := HeadRoutines
	oldHeadTestCount := HeadTestCount
	oldHeadMaxDelay := HeadMaxDelay
	oldTraceProbe := traceProbeFunc
	oldCFColo := HttpingCFColo
	oldCFColomap := HttpingCFColomap
	t.Cleanup(func() {
		HeadRoutines = oldHeadRoutines
		HeadTestCount = oldHeadTestCount
		HeadMaxDelay = oldHeadMaxDelay
		traceProbeFunc = oldTraceProbe
		HttpingCFColo = oldCFColo
		HttpingCFColomap = oldCFColomap
	})

	HeadRoutines = 99
	HeadTestCount = 20
	HeadMaxDelay = 0
	HttpingCFColo = ""
	HttpingCFColomap = nil
	var current atomic.Int32
	var maxSeen atomic.Int32
	traceProbeFunc = func(ip *net.IPAddr) traceProbeResult {
		active := current.Add(1)
		for {
			observed := maxSeen.Load()
			if active <= observed || maxSeen.CompareAndSwap(observed, active) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		current.Add(-1)
		return traceProbeResult{delay: time.Millisecond, ok: true}
	}

	result := TestTraceAvailability(makeProbeSet(20))
	if len(result) != 20 {
		t.Fatalf("Trace result count = %d, want 20", len(result))
	}
	traceMaxSeen := maxSeen.Load()
	if traceMaxSeen > MaxTraceRoutines {
		t.Fatalf("max Trace concurrency = %d, want <= %d", traceMaxSeen, MaxTraceRoutines)
	}
}

func TestTraceAvailabilityLogsRejectReasons(t *testing.T) {
	oldDebug := utils.Debug
	oldHeadRoutines := HeadRoutines
	oldHeadTestCount := HeadTestCount
	oldHeadMaxDelay := HeadMaxDelay
	oldTraceProbe := traceProbeFunc
	oldCFColo := HttpingCFColo
	oldCFColomap := HttpingCFColomap
	t.Cleanup(func() {
		utils.Debug = oldDebug
		_ = utils.CloseDebugLog()
		HeadRoutines = oldHeadRoutines
		HeadTestCount = oldHeadTestCount
		HeadMaxDelay = oldHeadMaxDelay
		traceProbeFunc = oldTraceProbe
		HttpingCFColo = oldCFColo
		HttpingCFColomap = oldCFColomap
	})

	for _, tc := range []struct {
		name       string
		setup      func()
		wantReason string
	}{
		{
			name: "latency limit",
			setup: func() {
				HeadMaxDelay = time.Millisecond
				HttpingCFColo = ""
				HttpingCFColomap = nil
				traceProbeFunc = func(ip *net.IPAddr) traceProbeResult {
					return traceProbeResult{delay: 10 * time.Millisecond, colo: "SJC", ok: true}
				}
			},
			wantReason: "trace_latency_limit",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			utils.Debug = true
			logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
			if _, err := utils.ConfigureDebugLog(true, logPath); err != nil {
				t.Fatalf("ConfigureDebugLog returned error: %v", err)
			}
			utils.SetDebugLogContext("trace-" + strings.ReplaceAll(tc.name, " ", "-"))
			HeadRoutines = 1
			HeadTestCount = 1
			tc.setup()

			result := TestTraceAvailability(makeProbeSet(1))
			if len(result) != 0 {
				t.Fatalf("Trace result count = %d, want 0", len(result))
			}
			if err := utils.CloseDebugLog(); err != nil {
				t.Fatalf("CloseDebugLog returned error: %v", err)
			}

			if !debugLogHasReason(t, logPath, tc.wantReason) {
				t.Fatalf("debug log missing reason %q", tc.wantReason)
			}
		})
	}
}

func TestTraceAvailabilityFiltersByConfiguredColoAfterGETTrace(t *testing.T) {
	oldHeadRoutines := HeadRoutines
	oldHeadTestCount := HeadTestCount
	oldHeadMaxDelay := HeadMaxDelay
	oldTraceProbe := traceProbeFunc
	oldStatusCode := HttpingStatusCode
	oldCFColo := HttpingCFColo
	oldCFColomap := HttpingCFColomap
	t.Cleanup(func() {
		HeadRoutines = oldHeadRoutines
		HeadTestCount = oldHeadTestCount
		HeadMaxDelay = oldHeadMaxDelay
		traceProbeFunc = oldTraceProbe
		HttpingStatusCode = oldStatusCode
		HttpingCFColo = oldCFColo
		HttpingCFColomap = oldCFColomap
	})

	HeadRoutines = 1
	HeadTestCount = 1
	HeadMaxDelay = 0
	HttpingStatusCode = 0
	HttpingCFColo = "LAX"
	HttpingCFColomap = MapColoMap()
	traceProbeFunc = func(ip *net.IPAddr) traceProbeResult {
		return traceProbeResult{delay: time.Millisecond, colo: "SJC", ok: true}
	}

	result := TestTraceAvailability(makeProbeSet(1))
	if len(result) != 0 {
		t.Fatalf("Trace result count = %d, want 0", len(result))
	}
}

func TestTraceAvailabilityAllowsConfiguredColoAfterGETTraceMatch(t *testing.T) {
	oldHeadRoutines := HeadRoutines
	oldHeadTestCount := HeadTestCount
	oldHeadMaxDelay := HeadMaxDelay
	oldTraceProbe := traceProbeFunc
	oldStatusCode := HttpingStatusCode
	oldCFColo := HttpingCFColo
	oldCFColomap := HttpingCFColomap
	t.Cleanup(func() {
		HeadRoutines = oldHeadRoutines
		HeadTestCount = oldHeadTestCount
		HeadMaxDelay = oldHeadMaxDelay
		traceProbeFunc = oldTraceProbe
		HttpingStatusCode = oldStatusCode
		HttpingCFColo = oldCFColo
		HttpingCFColomap = oldCFColomap
	})

	HeadRoutines = 1
	HeadTestCount = 1
	HeadMaxDelay = 0
	HttpingStatusCode = 0
	HttpingCFColo = "HKG"
	HttpingCFColomap = MapColoMap()
	traceProbeFunc = func(ip *net.IPAddr) traceProbeResult {
		return traceProbeResult{delay: time.Millisecond, colo: "HKG", ok: true}
	}

	result := TestTraceAvailability(makeProbeSet(1))
	if len(result) != 1 {
		t.Fatalf("Trace result count = %d, want 1", len(result))
	}
	if result[0].Colo != "HKG" {
		t.Fatalf("colo = %q, want HKG from GET trace response", result[0].Colo)
	}
}

func TestTraceAvailabilityFallsBackToTCPCandidatesWhenAllTraceRequestsFailWithoutColoWhitelist(t *testing.T) {
	oldHeadRoutines := HeadRoutines
	oldHeadTestCount := HeadTestCount
	oldHeadMaxDelay := HeadMaxDelay
	oldTraceProbe := traceProbeFunc
	oldStatusCode := HttpingStatusCode
	oldCFColo := HttpingCFColo
	oldCFColomap := HttpingCFColomap
	t.Cleanup(func() {
		HeadRoutines = oldHeadRoutines
		HeadTestCount = oldHeadTestCount
		HeadMaxDelay = oldHeadMaxDelay
		traceProbeFunc = oldTraceProbe
		HttpingStatusCode = oldStatusCode
		HttpingCFColo = oldCFColo
		HttpingCFColomap = oldCFColomap
	})

	HeadRoutines = 1
	HeadTestCount = 2
	HeadMaxDelay = 0
	HttpingStatusCode = 0
	HttpingCFColo = ""
	HttpingCFColomap = nil
	traceProbeFunc = func(ip *net.IPAddr) traceProbeResult {
		return traceProbeResult{reason: traceFailureRequest}
	}

	result := TestTraceAvailability(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2"))
	if len(result) != 2 {
		t.Fatalf("Trace fallback result count = %d, want 2", len(result))
	}
	for _, item := range result {
		if item.Colo != "" {
			t.Fatalf("fallback colo = %q, want empty", item.Colo)
		}
		if item.HeadDelay != 0 {
			t.Fatalf("fallback trace delay = %v, want 0", item.HeadDelay)
		}
	}
}

func TestTraceAvailabilityDoesNotFallbackWhenTraceHardFilterConfigured(t *testing.T) {
	oldHeadRoutines := HeadRoutines
	oldHeadTestCount := HeadTestCount
	oldHeadMaxDelay := HeadMaxDelay
	oldTraceProbe := traceProbeFunc
	oldStatusCode := HttpingStatusCode
	oldCFColo := HttpingCFColo
	oldCFColomap := HttpingCFColomap
	t.Cleanup(func() {
		HeadRoutines = oldHeadRoutines
		HeadTestCount = oldHeadTestCount
		HeadMaxDelay = oldHeadMaxDelay
		traceProbeFunc = oldTraceProbe
		HttpingStatusCode = oldStatusCode
		HttpingCFColo = oldCFColo
		HttpingCFColomap = oldCFColomap
	})

	for _, tc := range []struct {
		name  string
		setup func()
	}{
		{
			name: "status code filter",
			setup: func() {
				HeadMaxDelay = 0
				HttpingStatusCode = http.StatusOK
				HttpingCFColo = ""
				HttpingCFColomap = nil
			},
		},
		{
			name: "colo whitelist",
			setup: func() {
				HeadMaxDelay = 0
				HttpingStatusCode = 0
				HttpingCFColo = "HKG"
				HttpingCFColomap = MapColoMap()
			},
		},
		{
			name: "trace latency filter",
			setup: func() {
				HeadMaxDelay = time.Second
				HttpingStatusCode = 0
				HttpingCFColo = ""
				HttpingCFColomap = nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			HeadRoutines = 1
			HeadTestCount = 1
			traceProbeFunc = func(ip *net.IPAddr) traceProbeResult {
				return traceProbeResult{reason: traceFailureRequest}
			}
			tc.setup()

			result := TestTraceAvailability(makeProbeSet(1))
			if len(result) != 0 {
				t.Fatalf("Trace result count = %d, want 0", len(result))
			}
		})
	}
}

func TestTraceAvailabilityUsesGETTraceAndExtractsColo(t *testing.T) {
	oldHeadRoutines := HeadRoutines
	oldHeadTestCount := HeadTestCount
	oldHeadMaxDelay := HeadMaxDelay
	oldHeadTimeout := HeadTimeout
	oldTraceURL := TraceURL
	oldTCPPort := TCPPort
	oldStatusCode := HttpingStatusCode
	oldCFColo := HttpingCFColo
	oldCFColomap := HttpingCFColomap
	t.Cleanup(func() {
		HeadRoutines = oldHeadRoutines
		HeadTestCount = oldHeadTestCount
		HeadMaxDelay = oldHeadMaxDelay
		HeadTimeout = oldHeadTimeout
		TraceURL = oldTraceURL
		TCPPort = oldTCPPort
		HttpingStatusCode = oldStatusCode
		HttpingCFColo = oldCFColo
		HttpingCFColomap = oldCFColomap
	})

	var seenMethod, seenPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		w.Header().Set("cf-ray", "8f00abcdef-LAX")
		_, _ = w.Write([]byte("fl=1\ncolo=HKG\n"))
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/cdn-cgi/trace")
	HeadRoutines = 1
	HeadTestCount = 1
	HeadMaxDelay = 0
	HeadTimeout = time.Second
	TCPPort = port
	HttpingStatusCode = 0
	HttpingCFColo = ""
	HttpingCFColomap = nil

	result := TestTraceAvailability(utils.PingDelaySet{{
		PingData: &utils.PingData{
			IP:       ip,
			Sended:   3,
			Received: 3,
			Delay:    time.Millisecond,
		},
	}})
	if len(result) != 1 {
		t.Fatalf("Trace result count = %d, want 1", len(result))
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", seenMethod)
	}
	if seenPath != "/cdn-cgi/trace" {
		t.Fatalf("path = %q, want /cdn-cgi/trace", seenPath)
	}
	if result[0].Colo != "HKG" {
		t.Fatalf("colo = %q, want HKG from trace body", result[0].Colo)
	}
	if result[0].HeadDelay <= 0 {
		t.Fatalf("trace delay = %v, want positive", result[0].HeadDelay)
	}
}

func TestExtractColoFallbackChain(t *testing.T) {
	header := http.Header{}
	header.Set("cf-ray", "8f00abcdef-lax")
	if got := ExtractColo(header, []byte("colo=HKG\n")); got != "HKG" {
		t.Fatalf("ExtractColo body priority = %q, want HKG", got)
	}
	if got := ExtractColo(header, nil); got != "LAX" {
		t.Fatalf("ExtractColo cf-ray fallback = %q, want LAX", got)
	}
	header.Set("cf-ray", "8f00abcdef-zzz")
	if got := ExtractColo(header, nil); got != "ZZZ" {
		t.Fatalf("ExtractColo unknown cf-ray = %q, want ZZZ", got)
	}
	header = http.Header{}
	header.Set("x-served-by", "cache-fra-etou8220141-FRA, cache-hhr-khhr2060043-HHR")
	if got := ExtractColo(header, nil); got != "HHR" {
		t.Fatalf("ExtractColo existing CDN header = %q, want HHR", got)
	}
}

func TestParseColoAllowListNormalizesAndDedupes(t *testing.T) {
	got := ParseColoAllowList("hkg,nrt LAX hkg;sea bad-code zzz")
	want := []string{"HKG", "NRT", "LAX", "SEA", "ZZZ"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ParseColoAllowList = %#v, want %#v", got, want)
	}
	if _, ok := ColoInfoFor("khh"); !ok {
		t.Fatal("ColoInfoFor(KHH) = false, want built-in IATA info")
	}
}

func TestDownloadSpeedForcesSerialConcurrency(t *testing.T) {
	oldHandler := downloadHandlerFunc
	oldDisable := Disable
	oldTestCount := TestCount
	oldMinSpeed := MinSpeed
	oldDownloadRoutines := DownloadRoutines
	oldDebug := utils.Debug
	t.Cleanup(func() {
		downloadHandlerFunc = oldHandler
		Disable = oldDisable
		TestCount = oldTestCount
		MinSpeed = oldMinSpeed
		DownloadRoutines = oldDownloadRoutines
		utils.Debug = oldDebug
		_ = utils.CloseDebugLog()
	})

	utils.Debug = true
	logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
	if _, err := utils.ConfigureDebugLog(true, logPath); err != nil {
		t.Fatalf("ConfigureDebugLog returned error: %v", err)
	}
	utils.SetDebugLogContext("get-concurrent")
	Disable = false
	TestCount = 1
	MinSpeed = 0
	DownloadRoutines = 3
	var current atomic.Int32
	var maxSeen atomic.Int32
	downloadHandlerFunc = func(ip *net.IPAddr) (float64, string) {
		active := current.Add(1)
		for {
			observed := maxSeen.Load()
			if active <= observed || maxSeen.CompareAndSwap(observed, active) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		current.Add(-1)
		return 1024 * 1024, ""
	}

	result := TestDownloadSpeed(makeProbeSet(5))
	if len(result) != 5 {
		t.Fatalf("download result count = %d, want 5", len(result))
	}
	getMaxSeen := maxSeen.Load()
	if getMaxSeen != 1 {
		t.Fatalf("max GET concurrency = %d, want serial 1", getMaxSeen)
	}
	if err := utils.CloseDebugLog(); err != nil {
		t.Fatalf("CloseDebugLog returned error: %v", err)
	}
	if count := debugLogCountStageDetails(t, logPath, "stage3_get"); count != 5 {
		t.Fatalf("stage3_get detail log count = %d, want 5", count)
	}
}

func TestDownloadSpeedAllowsValidZeroAtZeroThreshold(t *testing.T) {
	oldHandler := downloadHandlerFunc
	oldDisable := Disable
	oldTestCount := TestCount
	oldMinSpeed := MinSpeed
	oldDownloadRoutines := DownloadRoutines
	oldRetryMaxAttempts := RetryMaxAttempts
	t.Cleanup(func() {
		downloadHandlerFunc = oldHandler
		Disable = oldDisable
		TestCount = oldTestCount
		MinSpeed = oldMinSpeed
		DownloadRoutines = oldDownloadRoutines
		RetryMaxAttempts = oldRetryMaxAttempts
	})

	Disable = false
	TestCount = 1
	MinSpeed = 0
	DownloadRoutines = 1
	RetryMaxAttempts = 3
	calls := map[string]int{}
	downloadHandlerFunc = func(ip *net.IPAddr) (float64, string) {
		calls[ip.String()]++
		if ip.String() == "1.1.1.1" {
			return 0, "SJC"
		}
		return 2 * 1024 * 1024, "HKG"
	}

	result := TestDownloadSpeed(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2"))
	if len(result) != 2 {
		t.Fatalf("download result count = %d, want 2", len(result))
	}
	if calls["1.1.1.1"] != 1 {
		t.Fatalf("zero-speed valid measurement calls = %d, want no retry", calls["1.1.1.1"])
	}
	foundZero := false
	for _, item := range result {
		if item.IP.String() == "1.1.1.1" {
			foundZero = true
			if item.DownloadSpeed != 0 {
				t.Fatalf("zero-speed result = %f, want 0", item.DownloadSpeed)
			}
			if item.Colo != "SJC" {
				t.Fatalf("zero-speed colo = %q, want SJC", item.Colo)
			}
		}
	}
	if !foundZero {
		t.Fatal("zero-speed valid measurement was not included in results")
	}
}

func TestDownloadSpeedFiltersBelowThreshold(t *testing.T) {
	oldHandler := downloadHandlerFunc
	oldDisable := Disable
	oldTestCount := TestCount
	oldMinSpeed := MinSpeed
	oldDownloadRoutines := DownloadRoutines
	t.Cleanup(func() {
		downloadHandlerFunc = oldHandler
		Disable = oldDisable
		TestCount = oldTestCount
		MinSpeed = oldMinSpeed
		DownloadRoutines = oldDownloadRoutines
	})

	Disable = false
	TestCount = 1
	MinSpeed = 2
	DownloadRoutines = 2
	downloadHandlerFunc = func(ip *net.IPAddr) (float64, string) {
		switch ip.String() {
		case "1.1.1.1":
			return 0, ""
		case "1.1.1.2":
			return 1 * 1024 * 1024, ""
		default:
			return 3 * 1024 * 1024, "HKG"
		}
	}

	result := TestDownloadSpeed(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2", "1.1.1.3", "1.1.1.4"))
	if len(result) != 2 {
		t.Fatalf("download result count = %d, want 2", len(result))
	}
	for _, item := range result {
		if item.DownloadSpeed < 2*1024*1024 {
			t.Fatalf("returned speed = %f, want >= threshold", item.DownloadSpeed)
		}
		if item.Colo != "HKG" {
			t.Fatalf("colo = %q, want HKG", item.Colo)
		}
	}
}

func TestDownloadSpeedRejectsNonOKResponseAtZeroThreshold(t *testing.T) {
	oldHandler := downloadHandlerFunc
	oldDisable := Disable
	oldTestCount := TestCount
	oldMinSpeed := MinSpeed
	oldDownloadRoutines := DownloadRoutines
	oldRetryMaxAttempts := RetryMaxAttempts
	oldURL := URL
	oldTraceURL := TraceURL
	oldTCPPort := TCPPort
	oldTimeout := Timeout
	t.Cleanup(func() {
		downloadHandlerFunc = oldHandler
		Disable = oldDisable
		TestCount = oldTestCount
		MinSpeed = oldMinSpeed
		DownloadRoutines = oldDownloadRoutines
		RetryMaxAttempts = oldRetryMaxAttempts
		URL = oldURL
		TraceURL = oldTraceURL
		TCPPort = oldTCPPort
		Timeout = oldTimeout
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	downloadHandlerFunc = nil
	Disable = false
	TestCount = 1
	MinSpeed = 0
	DownloadRoutines = 1
	RetryMaxAttempts = 0
	TCPPort = port
	Timeout = time.Second

	result := TestDownloadSpeed(utils.PingDelaySet{
		{
			PingData: &utils.PingData{
				IP:       ip,
				Sended:   3,
				Received: 3,
				Delay:    time.Millisecond,
			},
		},
	})
	if len(result) != 0 {
		t.Fatalf("download result count = %d, want 0 for non-200 response", len(result))
	}
}

func TestDownloadSpeedRejectsNoValidMeasurementAtZeroThreshold(t *testing.T) {
	oldHandler := downloadHandlerFunc
	oldDisable := Disable
	oldTestCount := TestCount
	oldMinSpeed := MinSpeed
	oldDownloadRoutines := DownloadRoutines
	oldRetryMaxAttempts := RetryMaxAttempts
	oldURL := URL
	oldTraceURL := TraceURL
	oldTCPPort := TCPPort
	oldTimeout := Timeout
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		downloadHandlerFunc = oldHandler
		Disable = oldDisable
		TestCount = oldTestCount
		MinSpeed = oldMinSpeed
		DownloadRoutines = oldDownloadRoutines
		RetryMaxAttempts = oldRetryMaxAttempts
		URL = oldURL
		TraceURL = oldTraceURL
		TCPPort = oldTCPPort
		Timeout = oldTimeout
		DownloadWarmupDuration = oldWarmup
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(80 * time.Millisecond)
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	downloadHandlerFunc = nil
	Disable = false
	TestCount = 1
	MinSpeed = 0
	DownloadRoutines = 1
	RetryMaxAttempts = 0
	TCPPort = port
	Timeout = 20 * time.Millisecond
	DownloadWarmupDuration = 0

	result := TestDownloadSpeed(utils.PingDelaySet{
		{
			PingData: &utils.PingData{
				IP:       ip,
				Sended:   3,
				Received: 3,
				Delay:    time.Millisecond,
			},
		},
	})
	if len(result) != 0 {
		t.Fatalf("download result count = %d, want 0 without a valid measurement", len(result))
	}
}

func TestDownloadHandlerEmitsSpeedSamplesAndReturnsAverage(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldHook := DownloadSpeedSampleHook
	oldInterval := DownloadSpeedSampleInterval
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadSpeedSampleHook = oldHook
		DownloadSpeedSampleInterval = oldInterval
		DownloadWarmupDuration = oldWarmup
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("cf-ray", "8f00abcdef-SJC")
		for i := 0; i < 4; i++ {
			_, _ = w.Write([]byte(strings.Repeat("a", 1024)))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(2 * time.Millisecond)
		}
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 80 * time.Millisecond
	DownloadSpeedSampleInterval = time.Millisecond
	DownloadWarmupDuration = 0

	samples := make([]DownloadSpeedSample, 0)
	DownloadSpeedSampleHook = func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}

	speed, colo := downloadHandler(ip)
	if speed <= 0 {
		t.Fatalf("speed = %f, want positive average speed", speed)
	}
	if colo != "SJC" {
		t.Fatalf("colo = %q, want SJC", colo)
	}
	if len(samples) == 0 {
		t.Fatal("expected at least one speed sample")
	}
	last := samples[len(samples)-1]
	if last.Stage != "stage3_get" || last.IP != ip.String() {
		t.Fatalf("sample identity = (%q,%q), want stage3_get/%s", last.Stage, last.IP, ip.String())
	}
	if last.BytesRead < 4096 {
		t.Fatalf("sample bytes = %d, want at least one downloaded segment", last.BytesRead)
	}
	if last.AverageSpeedMBs <= 0 || last.CurrentSpeedMBs < 0 {
		t.Fatalf("sample speeds = current %.4f average %.4f, want positive average", last.CurrentSpeedMBs, last.AverageSpeedMBs)
	}
	if diff := speed/1024/1024 - last.AverageSpeedMBs; diff < -0.001 || diff > 0.001 {
		t.Fatalf("returned speed %.6f MB/s differs from final sample average %.6f MB/s", speed/1024/1024, last.AverageSpeedMBs)
	}
}

func TestDownloadHandlerUsesRangeConcurrencyAndNoCacheHeaders(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldGetConcurrency := DownloadGetConcurrency
	oldBufferKB := DownloadBufferKB
	oldProtocol := DownloadHTTPProtocol
	oldInterval := DownloadSpeedSampleInterval
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadGetConcurrency = oldGetConcurrency
		DownloadBufferKB = oldBufferKB
		DownloadHTTPProtocol = oldProtocol
		DownloadSpeedSampleInterval = oldInterval
		DownloadWarmupDuration = oldWarmup
	})

	body := []byte(strings.Repeat("a", 4096))
	seenRanges := map[string]bool{}
	var seenMu sync.Mutex
	var current atomic.Int32
	var maxSeen atomic.Int32
	var badCacheHeader atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cache-Control") != "no-store" || r.Header.Get("Pragma") != "no-cache" {
			badCacheHeader.Store(true)
		}
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			http.Error(w, "range required", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		start, end, ok := parseRangeHeaderForTest(rangeHeader)
		if !ok || start < 0 || end < start || end >= int64(len(body)) {
			http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		seenMu.Lock()
		seenRanges[rangeHeader] = true
		seenMu.Unlock()
		active := current.Add(1)
		for {
			observed := maxSeen.Load()
			if active <= observed || maxSeen.CompareAndSwap(observed, active) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(body[start : end+1])
		current.Add(-1)
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 60 * time.Millisecond
	DownloadGetConcurrency = 4
	DownloadBufferKB = 64
	DownloadHTTPProtocol = "auto"
	DownloadSpeedSampleInterval = time.Millisecond
	DownloadWarmupDuration = 0

	speed, _ := downloadHandler(ip)
	if speed <= 0 {
		t.Fatalf("speed = %f, want positive range download speed", speed)
	}
	if badCacheHeader.Load() {
		t.Fatal("download request missing no-store/no-cache headers")
	}
	if maxSeen.Load() < 2 {
		t.Fatalf("max concurrent range requests = %d, want at least 2", maxSeen.Load())
	}
	for _, want := range []string{"bytes=0-1023", "bytes=1024-2047", "bytes=2048-3071", "bytes=3072-4095"} {
		if !seenRanges[want] {
			t.Fatalf("seen ranges = %#v, missing %s", seenRanges, want)
		}
	}
}

func TestDownloadHandlerAcceptsIntegrityHeaders(t *testing.T) {
	body := []byte(strings.Repeat("integrity", 256))
	tests := []struct {
		name      string
		setHeader func(http.Header)
	}{
		{
			name: "digest sha256",
			setHeader: func(header http.Header) {
				sum := sha256.Sum256(body)
				header.Set("Digest", "sha-256="+base64.StdEncoding.EncodeToString(sum[:]))
			},
		},
		{
			name: "content md5",
			setHeader: func(header http.Header) {
				sum := md5.Sum(body)
				header.Set("Content-MD5", base64.StdEncoding.EncodeToString(sum[:]))
			},
		},
		{
			name: "checksum sha256 hex",
			setHeader: func(header http.Header) {
				sum := sha256.Sum256(body)
				header.Set("X-Checksum-SHA256", hex.EncodeToString(sum[:]))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldURL := URL
			oldTraceURL := TraceURL
			oldTimeout := Timeout
			oldTCPPort := TCPPort
			oldWarmup := DownloadWarmupDuration
			t.Cleanup(func() {
				URL = oldURL
				TraceURL = oldTraceURL
				Timeout = oldTimeout
				TCPPort = oldTCPPort
				DownloadWarmupDuration = oldWarmup
			})

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.setHeader(w.Header())
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				_, _ = w.Write(body)
			}))
			defer server.Close()

			ip, port := configureProbeServer(t, server.URL, "/download.bin")
			TCPPort = port
			Timeout = 30 * time.Millisecond
			DownloadWarmupDuration = 0

			speed, _ := downloadHandler(ip)
			if speed <= 0 {
				t.Fatalf("speed = %f, want positive download with valid integrity header", speed)
			}
		})
	}
}

func TestDownloadHandlerRejectsIntegrityMismatch(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadWarmupDuration = oldWarmup
	})

	body := []byte(strings.Repeat("integrity", 256))
	wrongSum := sha256.Sum256([]byte("wrong body"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Digest", "sha-256="+base64.StdEncoding.EncodeToString(wrongSum[:]))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 30 * time.Millisecond
	DownloadWarmupDuration = 0

	speed, _ := downloadHandler(ip)
	if speed != 0 {
		t.Fatalf("speed = %f, want rejected digest mismatch", speed)
	}
}

func TestDownloadHandlerExcludesWarmupFromAverage(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldHook := DownloadSpeedSampleHook
	oldInterval := DownloadSpeedSampleInterval
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadSpeedSampleHook = oldHook
		DownloadSpeedSampleInterval = oldInterval
		DownloadWarmupDuration = oldWarmup
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("cf-ray", "8f00abcdef-SJC")
		_, _ = w.Write([]byte(strings.Repeat("a", 8*1024)))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(15 * time.Millisecond)
		_, _ = w.Write([]byte(strings.Repeat("b", 2*1024)))
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 80 * time.Millisecond
	DownloadSpeedSampleInterval = time.Millisecond
	DownloadWarmupDuration = 10 * time.Millisecond

	samples := make([]DownloadSpeedSample, 0)
	DownloadSpeedSampleHook = func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}

	speed, _ := downloadHandler(ip)
	if speed <= 0 {
		t.Fatalf("speed = %f, want positive post-warmup speed", speed)
	}
	if len(samples) == 0 {
		t.Fatal("expected at least one speed sample")
	}
	last := samples[len(samples)-1]
	if last.BytesRead < 10*1024 {
		t.Fatalf("sample bytes = %d, want cumulative bytes including warmup", last.BytesRead)
	}
	if last.AverageSpeedMBs <= 0 {
		t.Fatalf("final average speed = %.4f MB/s, want positive post-warmup speed", last.AverageSpeedMBs)
	}
	if diff := speed/1024/1024 - last.AverageSpeedMBs; diff < -0.001 || diff > 0.001 {
		t.Fatalf("returned speed %.6f MB/s differs from final sample average %.6f MB/s", speed/1024/1024, last.AverageSpeedMBs)
	}
}

func TestDownloadHandlerReconnectsWhenTransferCompletesDuringWarmup(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldHook := DownloadSpeedSampleHook
	oldInterval := DownloadSpeedSampleInterval
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadSpeedSampleHook = oldHook
		DownloadSpeedSampleInterval = oldInterval
		DownloadWarmupDuration = oldWarmup
	})

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("cf-ray", "8f00abcdef-SJC")
		_, _ = w.Write([]byte(strings.Repeat("a", 4*1024)))
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 80 * time.Millisecond
	DownloadSpeedSampleInterval = time.Millisecond
	DownloadWarmupDuration = 20 * time.Millisecond

	samples := make([]DownloadSpeedSample, 0)
	DownloadSpeedSampleHook = func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}

	speed, _ := downloadHandler(ip)
	if speed <= 0 {
		t.Fatalf("speed = %f, want positive average after reconnecting past warmup", speed)
	}
	if requests.Load() < 2 {
		t.Fatalf("requests = %d, want reconnects while transfer completes during warmup", requests.Load())
	}
	if len(samples) == 0 {
		t.Fatal("expected final speed sample")
	}
	last := samples[len(samples)-1]
	if last.BytesRead <= 4*1024 {
		t.Fatalf("sample bytes = %d, want cumulative bytes across reconnects", last.BytesRead)
	}
	if !last.AverageReady || last.MeasuredBytes <= 0 {
		t.Fatalf("final measurement = ready %v bytes %d elapsed %dms, want ready measured transfer", last.AverageReady, last.MeasuredBytes, last.MeasuredElapsedMS)
	}
	if last.AverageSpeedMBs <= 0 {
		t.Fatalf("final average speed = %.4f MB/s, want positive whole-transfer average", last.AverageSpeedMBs)
	}
	if diff := speed/1024/1024 - last.AverageSpeedMBs; diff < -0.001 || diff > 0.001 {
		t.Fatalf("returned speed %.6f MB/s differs from final sample average %.6f MB/s", speed/1024/1024, last.AverageSpeedMBs)
	}
}

func TestDownloadHandlerKeepsAverageNotReadyBeforeWarmup(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldHook := DownloadSpeedSampleHook
	oldInterval := DownloadSpeedSampleInterval
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadSpeedSampleHook = oldHook
		DownloadSpeedSampleInterval = oldInterval
		DownloadWarmupDuration = oldWarmup
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		_, _ = w.Write([]byte(strings.Repeat("a", 4*1024)))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(300 * time.Millisecond)
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 40 * time.Millisecond
	DownloadSpeedSampleInterval = time.Millisecond
	DownloadWarmupDuration = 500 * time.Millisecond

	samples := make([]DownloadSpeedSample, 0)
	DownloadSpeedSampleHook = func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}

	speed, _ := downloadHandler(ip)
	if speed != 0 {
		t.Fatalf("speed = %f, want 0 when transfer stalls before warmup completes", speed)
	}
	if len(samples) == 0 {
		t.Fatal("expected final speed sample")
	}
	last := samples[len(samples)-1]
	if last.AverageReady {
		t.Fatalf("final average ready = true for elapsed %dms, want false before warmup", last.ElapsedMS)
	}
	if last.MeasuredBytes != 0 || last.MeasuredElapsedMS != 0 {
		t.Fatalf("measured window = %d/%dms, want empty before warmup", last.MeasuredBytes, last.MeasuredElapsedMS)
	}
}

func TestDownloadHandlerKeepsAverageNotReadyForNoBodyRead(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldHook := DownloadSpeedSampleHook
	oldInterval := DownloadSpeedSampleInterval
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadSpeedSampleHook = oldHook
		DownloadSpeedSampleInterval = oldInterval
		DownloadWarmupDuration = oldWarmup
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(80 * time.Millisecond)
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 20 * time.Millisecond
	DownloadSpeedSampleInterval = time.Millisecond
	DownloadWarmupDuration = 5 * time.Millisecond

	samples := make([]DownloadSpeedSample, 0)
	DownloadSpeedSampleHook = func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}

	speed, _ := downloadHandler(ip)
	if speed != 0 {
		t.Fatalf("speed = %f, want 0 for no-body invalid download", speed)
	}
	if len(samples) == 0 {
		t.Fatal("expected final speed sample")
	}
	last := samples[len(samples)-1]
	if last.BytesRead != 0 || last.BodyRead || last.TransferComplete {
		t.Fatalf("body state = bytes %d bodyRead %v transferComplete %v, want no body and incomplete", last.BytesRead, last.BodyRead, last.TransferComplete)
	}
	if last.AverageReady {
		t.Fatalf("final average ready = true for no-body invalid download at elapsed %dms, want false", last.ElapsedMS)
	}
}

func TestDownloadHandlerReconnectsWhenTransferDisconnectsAfterWarmup(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldHook := DownloadSpeedSampleHook
	oldInterval := DownloadSpeedSampleInterval
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadSpeedSampleHook = oldHook
		DownloadSpeedSampleInterval = oldInterval
		DownloadWarmupDuration = oldWarmup
	})

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(15 * time.Millisecond)
		_, _ = w.Write([]byte(strings.Repeat("a", 4*1024)))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 70 * time.Millisecond
	DownloadSpeedSampleInterval = time.Millisecond
	DownloadWarmupDuration = 10 * time.Millisecond

	samples := make([]DownloadSpeedSample, 0)
	DownloadSpeedSampleHook = func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}

	speed, _ := downloadHandler(ip)
	if speed <= 0 {
		t.Fatalf("speed = %f, want positive speed after reconnecting post-warmup disconnect", speed)
	}
	if requests.Load() < 2 {
		t.Fatalf("requests = %d, want reconnect after post-warmup disconnect", requests.Load())
	}
	if len(samples) == 0 {
		t.Fatal("expected final speed sample")
	}
	last := samples[len(samples)-1]
	if !last.AverageReady {
		t.Fatal("final average ready = false, want true after reconnecting post-warmup disconnect")
	}
	for _, sample := range samples {
		if sample.ElapsedMS >= DownloadWarmupDuration.Milliseconds() && sample.CurrentReady && sample.CurrentSpeedMBs == 0 {
			t.Fatalf("samples = %#v, want no ready zero current-speed sample during reconnects", samples)
		}
	}
}

func TestDownloadSpeedSampleIntervalDefault(t *testing.T) {
	oldInterval := DownloadSpeedSampleInterval
	t.Cleanup(func() {
		DownloadSpeedSampleInterval = oldInterval
	})

	DownloadSpeedSampleInterval = 0
	checkDownloadDefault()

	if DownloadSpeedSampleInterval != 500*time.Millisecond {
		t.Fatalf("DownloadSpeedSampleInterval = %v, want 500ms", DownloadSpeedSampleInterval)
	}
}

func TestDownloadHandlerSamplesOnIntervalAndFinal(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldHook := DownloadSpeedSampleHook
	oldInterval := DownloadSpeedSampleInterval
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadSpeedSampleHook = oldHook
		DownloadSpeedSampleInterval = oldInterval
		DownloadWarmupDuration = oldWarmup
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("cf-ray", "8f00abcdef-SJC")
		for i := 0; i < 5; i++ {
			_, _ = w.Write([]byte(strings.Repeat("a", 2*1024)))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(12 * time.Millisecond)
		}
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 90 * time.Millisecond
	DownloadSpeedSampleInterval = 25 * time.Millisecond
	DownloadWarmupDuration = 0

	samples := make([]DownloadSpeedSample, 0)
	DownloadSpeedSampleHook = func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}

	speed, _ := downloadHandler(ip)
	if speed <= 0 {
		t.Fatalf("speed = %f, want positive average speed", speed)
	}
	if len(samples) < 3 {
		t.Fatalf("samples = %d, want initial, interval, and final samples", len(samples))
	}
	if samples[0].ElapsedMS != 0 {
		t.Fatalf("first sample elapsed = %dms, want 0", samples[0].ElapsedMS)
	}
	foundIntervalSample := false
	for _, sample := range samples[1 : len(samples)-1] {
		if sample.ElapsedMS >= 20 && sample.SampleElapsedMS >= 20 && sample.CurrentSpeedMBs > 0 {
			foundIntervalSample = true
			break
		}
	}
	if !foundIntervalSample {
		t.Fatalf("samples = %#v, want interval sample with current speed based on recent interval", samples)
	}
	last := samples[len(samples)-1]
	if last.SampleBytes < 0 || last.SampleElapsedMS < 0 {
		t.Fatalf("final sample delta = %d/%dms, want non-negative", last.SampleBytes, last.SampleElapsedMS)
	}
}

func TestDownloadHandlerInterruptRestartsSameIPWithoutConsumingRetry(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldSpeedHook := DownloadSpeedSampleHook
	oldPauseHook := ProbePauseHook
	oldInterruptHook := DownloadInterruptHook
	oldInterval := DownloadSpeedSampleInterval
	oldWarmup := DownloadWarmupDuration
	oldRetryMaxAttempts := RetryMaxAttempts
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadSpeedSampleHook = oldSpeedHook
		ProbePauseHook = oldPauseHook
		DownloadInterruptHook = oldInterruptHook
		DownloadSpeedSampleInterval = oldInterval
		DownloadWarmupDuration = oldWarmup
		RetryMaxAttempts = oldRetryMaxAttempts
	})

	var requests atomic.Int32
	firstRequestStarted := make(chan struct{})
	firstRequestInterrupted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestNo := requests.Add(1)
		w.Header().Set("cf-ray", "8f00abcdef-SJC")
		if requestNo == 1 {
			close(firstRequestStarted)
			w.Header().Set("Content-Length", "1048576")
			_, _ = w.Write([]byte(strings.Repeat("a", 4*1024)))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			<-r.Context().Done()
			close(firstRequestInterrupted)
			return
		}
		_, _ = w.Write([]byte(strings.Repeat("b", 8*1024)))
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 50 * time.Millisecond
	DownloadSpeedSampleInterval = time.Millisecond
	DownloadWarmupDuration = 0
	RetryMaxAttempts = 1

	var pauses atomic.Int32
	var registeredInterrupts atomic.Int32
	pauseCh := make(chan struct{})
	resumeCh := make(chan struct{})
	ProbePauseHook = func(stage, pauseIP string) {
		if stage != "stage3_get" || pauseIP != ip.String() {
			return
		}
		if pauses.Add(1) == 1 {
			close(pauseCh)
			<-resumeCh
		}
	}
	DownloadInterruptHook = func(stage, interruptIP string, interrupt func()) func() {
		if stage == "stage3_get" && interruptIP == ip.String() && registeredInterrupts.Add(1) == 1 {
			go func() {
				<-firstRequestStarted
				interrupt()
			}()
		}
		return func() {}
	}

	resumed := make(chan struct{})
	go func() {
		<-pauseCh
		close(resumeCh)
		close(resumed)
	}()

	speed, colo := downloadHandler(ip)
	if speed <= 0 {
		t.Fatalf("speed = %f, want successful retry after pause interrupt", speed)
	}
	if colo != "SJC" {
		t.Fatalf("colo = %q, want SJC", colo)
	}
	select {
	case <-firstRequestInterrupted:
	case <-time.After(time.Second):
		t.Fatal("first request was not interrupted")
	}
	select {
	case <-resumed:
	case <-time.After(time.Second):
		t.Fatal("pause hook did not resume")
	}
	if requests.Load() < 2 {
		t.Fatalf("requests = %d, want same IP restarted after interruption", requests.Load())
	}
}

func TestDownloadHandlerTimeoutsStalledBodyRead(t *testing.T) {
	oldURL := URL
	oldTraceURL := TraceURL
	oldTimeout := Timeout
	oldTCPPort := TCPPort
	oldWarmup := DownloadWarmupDuration
	t.Cleanup(func() {
		URL = oldURL
		TraceURL = oldTraceURL
		Timeout = oldTimeout
		TCPPort = oldTCPPort
		DownloadWarmupDuration = oldWarmup
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(300 * time.Millisecond)
	}))
	defer server.Close()

	ip, port := configureProbeServer(t, server.URL, "/download.bin")
	TCPPort = port
	Timeout = 40 * time.Millisecond
	DownloadWarmupDuration = 0

	done := make(chan struct{})
	go func() {
		_, _ = downloadHandler(ip)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("downloadHandler hung on a stalled response body")
	}
}

func makeProbeSet(count int) utils.PingDelaySet {
	result := make(utils.PingDelaySet, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, utils.CloudflareIPData{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    time.Duration(i+1) * time.Millisecond,
			},
		})
	}
	return result
}

func makeProbeSetWithIPs(values ...string) utils.PingDelaySet {
	result := make(utils.PingDelaySet, 0, len(values))
	for index, value := range values {
		result = append(result, utils.CloudflareIPData{
			PingData: &utils.PingData{
				IP:       parseTestIP(value),
				Sended:   3,
				Received: 3,
				Delay:    time.Duration(index+1) * time.Millisecond,
			},
		})
	}
	return result
}

func configureProbeServer(t *testing.T, serverURL, path string) (*net.IPAddr, int) {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) returned error: %v", serverURL, err)
	}
	host, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("SplitHostPort(%q) returned error: %v", parsed.Host, err)
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		t.Fatalf("LookupPort(%q) returned error: %v", portText, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		addrs, err := net.LookupIP(host)
		if err != nil || len(addrs) == 0 {
			t.Fatalf("could not resolve test server host %q: %v", host, err)
		}
		ip = addrs[0]
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	TraceURL = parsed.String()
	URL = parsed.String()
	return &net.IPAddr{IP: ip}, port
}

func parseRangeHeaderForTest(value string) (int64, int64, bool) {
	if !strings.HasPrefix(value, "bytes=") {
		return 0, 0, false
	}
	parts := strings.Split(strings.TrimPrefix(value, "bytes="), "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, startErr := strconv.ParseInt(parts[0], 10, 64)
	end, endErr := strconv.ParseInt(parts[1], 10, 64)
	return start, end, startErr == nil && endErr == nil
}

func parseTestIP(value string) *net.IPAddr {
	return &net.IPAddr{IP: net.ParseIP(value)}
}

func readTaskDebugLogEntries(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("debug log line is not JSON: %v\n%s", err, line)
		}
		entries = append(entries, entry)
	}
	return entries
}

func debugLogHasReason(t *testing.T, path, reason string) bool {
	t.Helper()
	for _, entry := range readTaskDebugLogEntries(t, path) {
		if entry["reason"] == reason {
			return true
		}
	}
	return false
}

func debugLogCountStageDetails(t *testing.T, path, stage string) int {
	t.Helper()
	count := 0
	for _, entry := range readTaskDebugLogEntries(t, path) {
		if entry["event"] == "stage.detail" && entry["stage"] == stage {
			count++
		}
	}
	return count
}
