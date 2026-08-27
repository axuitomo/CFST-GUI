package task

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestTCPCheckConnectionSkipsFirstSample(t *testing.T) {
	config := DefaultConfig()
	config.PingTimes = 4
	config.SkipFirstLatencySample = true
	config.Httping = false
	delays := []time.Duration{
		999 * time.Millisecond,
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
	}
	index := 0
	ping := &Ping{
		engine: NewEngine(config, Hooks{}),
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

func TestNormalizeConfigRejectsSinglePingTime(t *testing.T) {
	config := DefaultConfig()
	config.PingTimes = 1
	config = normalizeConfig(config)
	if config.PingTimes != MinPingTimes {
		t.Fatalf("PingTimes = %d, want minimum %d", config.PingTimes, MinPingTimes)
	}
}

func TestResetStageCooldownCountersClearsPartialFailures(t *testing.T) {
	engine := NewEngine(Config{
		CooldownFailures: 2,
		CooldownDuration: time.Millisecond,
	}, Hooks{})
	const stage = "stage-reset-test"

	engine.noteStageProbeOutcome(stage, "1.1.1.1", false)
	engine.ResetCooldownCounters()
	engine.noteStageProbeOutcome(stage, "1.1.1.2", false)

	engine.cooldownMu.Lock()
	got := engine.cooldownFails[stage]
	engine.cooldownMu.Unlock()
	if got != 1 {
		t.Fatalf("consecutive failures after reset = %d, want 1", got)
	}
}

func TestTraceAvailabilityConcurrencyIsCappedAtSix(t *testing.T) {
	config := DefaultConfig()
	config.HeadRoutines = 99
	config.HeadTestCount = 20
	config.HeadMaxDelay = 0
	config.HttpingCFColo = ""
	var current atomic.Int32
	var maxSeen atomic.Int32
	engine := newEngineWithTraceProbe(config, Hooks{}, func(ip *net.IPAddr) traceProbeResult {
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
	})

	result := engine.TestTraceAvailability(makeProbeSet(20))
	if len(result) != 20 {
		t.Fatalf("Trace result count = %d, want 20", len(result))
	}
	traceMaxSeen := maxSeen.Load()
	if traceMaxSeen > MaxTraceRoutines {
		t.Fatalf("max Trace concurrency = %d, want <= %d", traceMaxSeen, MaxTraceRoutines)
	}
}

func TestTraceAvailabilityLogsRejectReasons(t *testing.T) {
	for _, tc := range []struct {
		name       string
		setup      func(*Config)
		probe      func(*net.IPAddr) traceProbeResult
		wantReason string
	}{
		{
			name: "latency limit",
			setup: func(config *Config) {
				config.HeadMaxDelay = time.Millisecond
				config.HttpingCFColo = ""
			},
			probe: func(ip *net.IPAddr) traceProbeResult {
				return traceProbeResult{delay: 10 * time.Millisecond, colo: "SJC", ok: true}
			},
			wantReason: "trace_latency_limit",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultConfig()
			config.HeadRoutines = 1
			config.HeadTestCount = 1
			tc.setup(&config)
			var reasons []string
			engine := newEngineWithTraceProbe(config, Hooks{DebugEvent: func(_ string, payload map[string]any) {
				if reason, _ := payload["reason"].(string); reason != "" {
					reasons = append(reasons, reason)
				}
			}}, tc.probe)

			result := engine.TestTraceAvailability(makeProbeSet(1))
			if len(result) != 0 {
				t.Fatalf("Trace result count = %d, want 0", len(result))
			}
			if !slicesContainString(reasons, tc.wantReason) {
				t.Fatalf("debug events missing reason %q: %#v", tc.wantReason, reasons)
			}
		})
	}
}

func TestTraceProbeEmitsStatusMismatchDiagnostic(t *testing.T) {
	var captured []TraceDiagnostic
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.TraceColoMode = TraceColoModeTraceURL
	config.HttpingStatusCode = http.StatusOK
	engine := NewEngine(config, Hooks{TraceDiagnostic: func(diagnostic TraceDiagnostic) {
		captured = append(captured, diagnostic)
	}})

	result := engine.traceProbeIP(ip)
	if result.ok || result.reason != traceFailureStatus {
		t.Fatalf("trace result = %#v, want status_mismatch", result)
	}
	if len(captured) != 1 {
		t.Fatalf("captured diagnostics = %#v, want 1", captured)
	}
	if captured[0].Reason != string(traceFailureStatus) {
		t.Fatalf("diagnostic reason = %q, want %q", captured[0].Reason, traceFailureStatus)
	}
	if captured[0].StatusCode != http.StatusNotFound {
		t.Fatalf("diagnostic status = %d, want 404", captured[0].StatusCode)
	}
	if captured[0].URL == "" {
		t.Fatalf("diagnostic URL = empty, want request URL")
	}
}

func TestTraceAvailabilityEmitsLatencyLimitDiagnostic(t *testing.T) {
	var captured []TraceDiagnostic
	config := DefaultConfig()
	config.HeadRoutines = 1
	config.HeadTestCount = 1
	config.HeadMaxDelay = time.Millisecond
	config.HttpingStatusCode = 0
	config.HttpingCFColo = ""
	engine := newEngineWithTraceProbe(config, Hooks{TraceDiagnostic: func(diagnostic TraceDiagnostic) {
		captured = append(captured, diagnostic)
	}}, func(ip *net.IPAddr) traceProbeResult {
		return traceProbeResult{
			delay:      10 * time.Millisecond,
			colo:       "SJC",
			ok:         true,
			statusCode: http.StatusOK,
			url:        "https://trace.example.com/cdn-cgi/trace",
		}
	})

	result := engine.TestTraceAvailability(makeProbeSet(1))
	if len(result) != 0 {
		t.Fatalf("trace result count = %d, want 0", len(result))
	}
	if len(captured) != 1 {
		t.Fatalf("captured diagnostics = %#v, want 1", captured)
	}
	if captured[0].Reason != string(traceFailureLatencyLimit) {
		t.Fatalf("diagnostic reason = %q, want %q", captured[0].Reason, traceFailureLatencyLimit)
	}
	if captured[0].StatusCode != http.StatusOK {
		t.Fatalf("diagnostic status = %d, want 200", captured[0].StatusCode)
	}
	if captured[0].URL != "https://trace.example.com/cdn-cgi/trace" {
		t.Fatalf("diagnostic URL = %q, want trace URL", captured[0].URL)
	}
}

func TestTraceAvailabilityFiltersByConfiguredColoAfterGETTrace(t *testing.T) {
	config := DefaultConfig()
	config.HeadRoutines = 1
	config.HeadTestCount = 1
	config.HeadMaxDelay = 0
	config.HttpingStatusCode = 0
	config.HttpingCFColo = "LAX"
	engine := newEngineWithTraceProbe(config, Hooks{}, func(ip *net.IPAddr) traceProbeResult {
		return traceProbeResult{delay: time.Millisecond, colo: "SJC", ok: true}
	})

	result := engine.TestTraceAvailability(makeProbeSet(1))
	if len(result) != 0 {
		t.Fatalf("Trace result count = %d, want 0", len(result))
	}
}

func TestTraceAvailabilityAllowsConfiguredColoAfterGETTraceMatch(t *testing.T) {
	config := DefaultConfig()
	config.HeadRoutines = 1
	config.HeadTestCount = 1
	config.HeadMaxDelay = 0
	config.HttpingStatusCode = 0
	config.HttpingCFColo = "HKG"
	engine := newEngineWithTraceProbe(config, Hooks{}, func(ip *net.IPAddr) traceProbeResult {
		return traceProbeResult{delay: time.Millisecond, colo: "HKG", ok: true}
	})

	result := engine.TestTraceAvailability(makeProbeSet(1))
	if len(result) != 1 {
		t.Fatalf("Trace result count = %d, want 1", len(result))
	}
	if result[0].Colo != "HKG" {
		t.Fatalf("colo = %q, want HKG from GET trace response", result[0].Colo)
	}
}

func TestTraceAvailabilityFallsBackToTCPCandidatesWhenAllTraceRequestsFailWithoutColoWhitelist(t *testing.T) {
	config := DefaultConfig()
	config.HeadRoutines = 1
	config.HeadTestCount = 2
	config.HeadMaxDelay = 0
	config.HttpingStatusCode = 0
	config.HttpingCFColo = ""
	engine := newEngineWithTraceProbe(config, Hooks{}, func(ip *net.IPAddr) traceProbeResult {
		return traceProbeResult{reason: traceFailureRequest}
	})

	result := engine.TestTraceAvailability(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2"))
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

func TestTraceAvailabilitySoftPassesTransientFailures(t *testing.T) {
	config := DefaultConfig()
	config.HeadRoutines = 1
	config.HeadTestCount = 4
	config.HeadMaxDelay = 0
	config.HttpingStatusCode = http.StatusOK
	config.HttpingCFColo = ""
	config.RetryMaxAttempts = 0
	var reasons []string
	engine := newEngineWithTraceProbe(config, Hooks{DebugEvent: func(_ string, payload map[string]any) {
		if reason, _ := payload["reason"].(string); reason != "" {
			reasons = append(reasons, reason)
		}
	}}, func(ip *net.IPAddr) traceProbeResult {
		switch ip.String() {
		case "1.1.1.1":
			return traceProbeResult{delay: 10 * time.Millisecond, colo: "SJC", ok: true}
		case "1.1.1.2":
			return traceProbeResult{reason: traceFailureRequest}
		case "1.1.1.3":
			return traceProbeResult{reason: traceFailureRead, statusCode: http.StatusOK}
		case "1.1.1.4":
			return traceProbeResult{reason: traceFailureRateLimited, retryAfter: time.Second, statusCode: http.StatusTooManyRequests}
		default:
			return traceProbeResult{reason: traceFailureRequestCreate}
		}
	})

	result := engine.TestTraceAvailability(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2", "1.1.1.3", "1.1.1.4"))
	if len(result) != 4 {
		t.Fatalf("Trace result count = %d, want 4", len(result))
	}
	for _, item := range result {
		if item.IP.String() == "1.1.1.1" {
			if item.HeadDelay <= 0 || item.Colo != "SJC" {
				t.Fatalf("successful trace item = delay %v colo %q, want positive/SJC", item.HeadDelay, item.Colo)
			}
			continue
		}
		if item.HeadDelay != 0 {
			t.Fatalf("soft-pass trace delay for %s = %v, want 0", item.IP.String(), item.HeadDelay)
		}
		if item.Colo != "" {
			t.Fatalf("soft-pass colo for %s = %q, want empty", item.IP.String(), item.Colo)
		}
	}
	if !slicesContainString(reasons, "trace_soft_pass") {
		t.Fatalf("debug events missing trace_soft_pass reason: %#v", reasons)
	}
}

func TestTraceAvailabilityDoesNotFallbackWhenTraceHardFilterConfigured(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*Config)
		probe func(*net.IPAddr) traceProbeResult
	}{
		{
			name: "non transient status mismatch",
			setup: func(config *Config) {
				config.HeadMaxDelay = 0
				config.HttpingStatusCode = http.StatusOK
				config.HttpingCFColo = ""
			},
			probe: func(ip *net.IPAddr) traceProbeResult {
				return traceProbeResult{reason: traceFailureStatus, statusCode: http.StatusNotFound}
			},
		},
		{
			name: "colo whitelist",
			setup: func(config *Config) {
				config.HeadMaxDelay = 0
				config.HttpingStatusCode = 0
				config.HttpingCFColo = "HKG"
			},
			probe: func(ip *net.IPAddr) traceProbeResult {
				return traceProbeResult{reason: traceFailureRequest}
			},
		},
		{
			name: "trace latency filter",
			setup: func(config *Config) {
				config.HeadMaxDelay = time.Second
				config.HttpingStatusCode = 0
				config.HttpingCFColo = ""
			},
			probe: func(ip *net.IPAddr) traceProbeResult {
				return traceProbeResult{reason: traceFailureRequest}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultConfig()
			config.HeadRoutines = 1
			config.HeadTestCount = 1
			tc.setup(&config)
			engine := newEngineWithTraceProbe(config, Hooks{}, tc.probe)

			result := engine.TestTraceAvailability(makeProbeSet(1))
			if len(result) != 0 {
				t.Fatalf("Trace result count = %d, want 0", len(result))
			}
		})
	}
}

func TestTraceAvailabilitySoftPassesTransientStatusCodes(t *testing.T) {
	config := DefaultConfig()
	config.HeadRoutines = 1
	config.HeadTestCount = 5
	config.HeadMaxDelay = 0
	config.HttpingStatusCode = http.StatusOK
	config.HttpingCFColo = ""
	config.RetryMaxAttempts = 0
	statusByIP := map[string]int{
		"1.1.1.1": http.StatusRequestTimeout,
		"1.1.1.2": http.StatusTooEarly,
		"1.1.1.3": http.StatusInternalServerError,
		"1.1.1.4": http.StatusServiceUnavailable,
		"1.1.1.5": http.StatusNotFound,
	}
	engine := newEngineWithTraceProbe(config, Hooks{}, func(ip *net.IPAddr) traceProbeResult {
		return traceProbeResult{reason: traceFailureStatus, statusCode: statusByIP[ip.String()]}
	})

	result := engine.TestTraceAvailability(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2", "1.1.1.3", "1.1.1.4", "1.1.1.5"))
	if len(result) != 4 {
		t.Fatalf("Trace result count = %d, want 4 transient status soft-passed", len(result))
	}
	for _, item := range result {
		if item.IP.String() == "1.1.1.5" {
			t.Fatal("non-transient 404 status should not soft-pass")
		}
	}
}

func TestTraceAvailabilityRetriesBeforeSoftPass(t *testing.T) {
	config := DefaultConfig()
	config.HeadRoutines = 1
	config.HeadTestCount = 2
	config.HeadMaxDelay = 0
	config.HttpingStatusCode = http.StatusOK
	config.HttpingCFColo = ""
	config.RetryMaxAttempts = 1
	config.RetryBackoff = 0
	calls := map[string]int{}
	var callsMu sync.Mutex
	engine := newEngineWithTraceProbe(config, Hooks{}, func(ip *net.IPAddr) traceProbeResult {
		callsMu.Lock()
		calls[ip.String()]++
		call := calls[ip.String()]
		callsMu.Unlock()
		if ip.String() == "1.1.1.1" && call == 2 {
			return traceProbeResult{delay: 8 * time.Millisecond, colo: "HKG", ok: true}
		}
		return traceProbeResult{reason: traceFailureRequest}
	})

	result := engine.TestTraceAvailability(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2"))
	if len(result) != 2 {
		t.Fatalf("Trace result count = %d, want 2", len(result))
	}
	callsMu.Lock()
	firstCalls, secondCalls := calls["1.1.1.1"], calls["1.1.1.2"]
	callsMu.Unlock()
	if firstCalls != 2 || secondCalls != 2 {
		t.Fatalf("trace calls = (%d,%d), want both retried once", firstCalls, secondCalls)
	}
	for _, item := range result {
		switch item.IP.String() {
		case "1.1.1.1":
			if item.HeadDelay <= 0 || item.Colo != "HKG" {
				t.Fatalf("retried success = delay %v colo %q, want positive/HKG", item.HeadDelay, item.Colo)
			}
		case "1.1.1.2":
			if item.HeadDelay != 0 || item.Colo != "" {
				t.Fatalf("final transient soft pass = delay %v colo %q, want zero/empty", item.HeadDelay, item.Colo)
			}
		}
	}
}

func TestTraceAvailabilityUsesGETTraceAndExtractsColo(t *testing.T) {
	var seenMethod, seenPath, seenCustomHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenCustomHeader = r.Header.Get("X-CFST-Test")
		w.Header().Set("cf-ray", "8f00abcdef-LAX")
		_, _ = w.Write([]byte("fl=1\ncolo=HKG\n"))
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.HeadRoutines = 1
	config.HeadTestCount = 1
	config.HeadMaxDelay = 0
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.HttpingStatusCode = 0
	config.HttpingCFColo = ""
	config.RequestHeaders = "X-CFST-Test: trace"
	engine := NewEngine(config, Hooks{})

	result := engine.TestTraceAvailability(utils.PingDelaySet{{
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
	if seenCustomHeader != "trace" {
		t.Fatalf("X-CFST-Test = %q, want trace", seenCustomHeader)
	}
	if result[0].Colo != "HKG" {
		t.Fatalf("colo = %q, want HKG from trace body", result[0].Colo)
	}
	if result[0].HeadDelay <= 0 {
		t.Fatalf("trace delay = %v, want positive", result[0].HeadDelay)
	}
}

func TestTraceProbeStatusCodeZeroAcceptsAnyNonRateLimitedStatus(t *testing.T) {
	for _, statusCode := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(statusCode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				_, _ = w.Write([]byte("colo=HKG\n"))
			}))
			defer server.Close()

			ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
			config := DefaultConfig()
			config.HeadTimeout = time.Second
			config.TCPPort = port
			config.TraceURL = traceURL
			config.HttpingStatusCode = 0
			engine := NewEngine(config, Hooks{})

			result := engine.traceProbeIP(ip)
			if !result.ok {
				t.Fatalf("trace result = %#v, want accepted status when HttpingStatusCode is 0", result)
			}
			if result.colo != "HKG" {
				t.Fatalf("colo = %q, want HKG", result.colo)
			}
		})
	}
}

func TestTraceProbeStatusCodeZeroStillRateLimits429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.HttpingStatusCode = 0
	engine := NewEngine(config, Hooks{})

	result := engine.traceProbeIP(ip)
	if result.ok || result.reason != traceFailureRateLimited {
		t.Fatalf("trace result = %#v, want rate_limited", result)
	}
	if result.retryAfter != 2*time.Second {
		t.Fatalf("retryAfter = %v, want 2s", result.retryAfter)
	}
}

func TestTraceProbeStandardAcceptsForbiddenCFRayBeforeStatusFilter(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("cf-ray", "8f00abcdef-LAX")
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.TraceColoMode = TraceColoModeStandard
	config.HttpingStatusCode = http.StatusOK
	engine := NewEngine(config, Hooks{})

	result := engine.traceProbeIP(ip)
	if !result.ok || result.colo != "LAX" || result.statusCode != http.StatusForbidden {
		t.Fatalf("trace result = %#v, want accepted 403 CF-RAY LAX", result)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want no fallback after 403 CF-RAY", requests.Load())
	}
}

func TestTraceProbeStandardUsesConfiguredTraceURLHostForIPLiteral(t *testing.T) {
	var seenHosts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHosts = append(seenHosts, r.Host)
		_, _ = w.Write([]byte("colo=HKG\n"))
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.TraceColoMode = TraceColoModeStandard
	config.HttpingStatusCode = 0
	engine := NewEngine(config, Hooks{})

	result := engine.traceProbeIP(ip)
	if !result.ok || result.colo != "HKG" {
		t.Fatalf("trace result = %#v, want HKG using configured trace URL host", result)
	}
	if len(seenHosts) != 1 || !strings.Contains(seenHosts[0], ":") {
		t.Fatalf("seen hosts = %#v, want configured trace URL host for IP literal request", seenHosts)
	}
}

func TestTraceProbeTraceURLModeSkipsIPLiteralAndUsesConfiguredBody(t *testing.T) {
	var seenHosts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHosts = append(seenHosts, r.Host)
		_, _ = w.Write([]byte("colo=NRT\n"))
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.TraceColoMode = TraceColoModeTraceURL
	config.HttpingStatusCode = 0
	engine := NewEngine(config, Hooks{})

	result := engine.traceProbeIP(ip)
	if !result.ok || result.colo != "NRT" {
		t.Fatalf("trace result = %#v, want NRT from configured trace URL body", result)
	}
	if len(seenHosts) != 1 || !strings.Contains(seenHosts[0], ":") {
		t.Fatalf("seen hosts = %#v, want only configured trace URL request", seenHosts)
	}
}

func TestTraceProbeTraceURLModeAcceptsForbiddenCFRay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("cf-ray", "8f00abcdef-SJC")
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.TraceColoMode = TraceColoModeTraceURL
	config.HttpingStatusCode = http.StatusOK
	engine := NewEngine(config, Hooks{})

	result := engine.traceProbeIP(ip)
	if !result.ok || result.colo != "SJC" || result.statusCode != http.StatusForbidden {
		t.Fatalf("trace result = %#v, want accepted 403 CF-RAY SJC", result)
	}
}

func TestTraceProbeFallsBackToColoDictionaryWhenTraceHasNoColo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fl=trace\n"))
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	dictionaryPath := filepath.Join(t.TempDir(), "cloudflare-colos.csv")
	prefixSuffix := "/32"
	if ip.IP.To4() == nil {
		prefixSuffix = "/128"
	}
	raw := strings.Join([]string{
		"ip_prefix,colo,country,region,city",
		ip.String() + prefixSuffix + ",SJC,US,CA,San Jose",
	}, "\n")
	if err := os.WriteFile(dictionaryPath, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", dictionaryPath, err)
	}
	config := DefaultConfig()
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.TraceColoMode = TraceColoModeStandard
	config.HttpingStatusCode = 0
	config.ColoDictionaryPath = dictionaryPath
	engine := NewEngine(config, Hooks{})

	result := engine.traceProbeIP(ip)
	if !result.ok || result.colo != "SJC" {
		t.Fatalf("trace result = %#v, want SJC from dictionary fallback", result)
	}
}

func TestTraceProbeDoesNotFallbackAfterColoMismatch(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("colo=LAX\n"))
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.TraceColoMode = TraceColoModeStandard
	config.HttpingStatusCode = 0
	config.HttpingCFColo = "HKG"
	engine := NewEngine(config, Hooks{})

	result := engine.traceProbeIP(ip)
	if result.ok || result.reason != traceFailureColoFilter {
		t.Fatalf("trace result = %#v, want direct colo_filter rejection", result)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want no fallback after mismatched COLO", requests.Load())
	}
}

func TestTraceProbeIPLiteralURLFormatsIPv4AndIPv6(t *testing.T) {
	config := DefaultConfig()
	config.TraceURL = "http://example.com/cdn-cgi/trace"
	engine := NewEngine(config, Hooks{})
	if got := engine.traceIPLiteralURL(parseTestIP("1.1.1.1")); got != "http://1.1.1.1/cdn-cgi/trace" {
		t.Fatalf("IPv4 literal trace URL = %q", got)
	}
	config.TraceURL = "https://example.com/cdn-cgi/trace"
	engine = NewEngine(config, Hooks{})
	if got := engine.traceIPLiteralURL(parseTestIP("2400:cb00::1")); got != "https://[2400:cb00::1]/cdn-cgi/trace" {
		t.Fatalf("IPv6 literal trace URL = %q", got)
	}
}

func TestTraceProbeIgnoresTLSServerCertificate(t *testing.T) {
	t.Setenv("CFST_HTTP_PROTOCOL", "h1")

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("colo=HKG\n"))
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.TraceColoMode = TraceColoModeStandard
	config.HttpingStatusCode = 0
	config.InsecureSkipVerify = true
	engine := NewEngine(config, Hooks{})

	result := engine.traceProbeIP(ip)
	if !result.ok || result.colo != "HKG" {
		t.Fatalf("trace result = %#v, want HTTPS trace accepted with certificate verification disabled", result)
	}
}

func TestTraceProbeRejectsInvalidTLSServerCertificate(t *testing.T) {
	t.Setenv("CFST_HTTP_PROTOCOL", "h1")

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("colo=HKG\n"))
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.HeadTimeout = time.Second
	config.TCPPort = port
	config.TraceURL = traceURL
	config.TraceColoMode = TraceColoModeStandard
	config.HttpingStatusCode = 0
	config.InsecureSkipVerify = false
	engine := NewEngine(config, Hooks{})

	result := engine.traceProbeIP(ip)
	if result.ok || result.reason != traceFailureRequest {
		t.Fatalf("trace result = %#v, want invalid TLS certificate rejection", result)
	}
}

func TestTraceAvailabilityAppliesSourceColoFiltersPassAnyAndUnrestricted(t *testing.T) {
	config := DefaultConfig()
	config.HeadRoutines = 1
	config.HeadTestCount = 3
	config.HeadMaxDelay = 0
	config.HttpingStatusCode = 0
	config.HttpingCFColo = ""
	config.SourceColoFilters = SourceColoFilterMap{
		"1.1.1.1": {Allowed: map[string]struct{}{"HKG": {}, "LAX": {}}},
		"1.1.1.2": {Allowed: map[string]struct{}{"HKG": {}}},
		"1.1.1.3": {Unrestricted: true},
	}
	engine := newEngineWithTraceProbe(config, Hooks{}, func(ip *net.IPAddr) traceProbeResult {
		return traceProbeResult{delay: time.Millisecond, colo: "LAX", ok: true}
	})

	result := engine.TestTraceAvailability(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2", "1.1.1.3"))
	got := make([]string, 0, len(result))
	for _, item := range result {
		got = append(got, item.IP.String())
	}
	want := []string{"1.1.1.1", "1.1.1.3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("passed IPs = %#v, want %#v", got, want)
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
	config := DefaultConfig()
	config.Disable = false
	config.TestCount = 1
	config.MinSpeed = 0
	config.DownloadRoutines = 3
	var current atomic.Int32
	var maxSeen atomic.Int32
	var detailCount atomic.Int32
	engine := newEngineWithDownloadProbes(config, Hooks{DebugEvent: func(event string, payload map[string]any) {
		if event == "stage.detail" && payload["stage"] == "stage3_get" {
			detailCount.Add(1)
		}
	}}, func(ip *net.IPAddr) (float64, string) {
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
	}, nil)

	result := engine.TestDownloadSpeed(makeProbeSet(5))
	if len(result) != 5 {
		t.Fatalf("download result count = %d, want 5", len(result))
	}
	getMaxSeen := maxSeen.Load()
	if getMaxSeen != 1 {
		t.Fatalf("max GET concurrency = %d, want serial 1", getMaxSeen)
	}
	if count := detailCount.Load(); count != 5 {
		t.Fatalf("stage3_get detail log count = %d, want 5", count)
	}
}

func TestDownloadSpeedAllowsValidZeroAtZeroThreshold(t *testing.T) {
	config := DefaultConfig()
	config.Disable = false
	config.TestCount = 1
	config.MinSpeed = 0
	config.DownloadRoutines = 1
	config.RetryMaxAttempts = 3
	calls := map[string]int{}
	engine := newEngineWithDownloadProbes(config, Hooks{}, func(ip *net.IPAddr) (float64, string) {
		calls[ip.String()]++
		if ip.String() == "1.1.1.1" {
			return 0, "SJC"
		}
		return 2 * 1024 * 1024, "HKG"
	}, nil)

	result := engine.TestDownloadSpeed(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2"))
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

func TestDownloadSpeedCancellationStopsCandidatesWithoutFailureDetails(t *testing.T) {
	config := DefaultConfig()
	config.Disable = false
	config.DownloadRoutines = 1
	var canceled atomic.Bool
	var calls atomic.Int32
	var details atomic.Int32
	var progress atomic.Int32
	engine := newEngineWithDownloadProbes(config, Hooks{
		ProbeCancel: func(stage, ip string) bool {
			return stage == "stage3_get" && canceled.Load()
		},
		DebugEvent: func(event string, payload map[string]any) {
			if event == "stage.detail" && payload["stage"] == "stage3_get" {
				details.Add(1)
			}
		},
		DownloadProgress: func(processed, qualified, total int) {
			progress.Add(1)
		},
	}, nil, func(ip *net.IPAddr) downloadResult {
		calls.Add(1)
		canceled.Store(true)
		return invalidDownloadResult("download_invalid", true)
	})

	result := engine.TestDownloadSpeed(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2", "1.1.1.3"))
	if len(result) != 0 || calls.Load() != 1 {
		t.Fatalf("result count = %d, calls = %d; want cancellation after first candidate", len(result), calls.Load())
	}
	if details.Load() != 0 || progress.Load() != 0 {
		t.Fatalf("details = %d, progress = %d; canceled candidate must not update failure statistics", details.Load(), progress.Load())
	}
}

func TestDownloadSpeedFiltersBelowThreshold(t *testing.T) {
	config := DefaultConfig()
	config.Disable = false
	config.TestCount = 1
	config.MinSpeed = 2
	config.DownloadRoutines = 2
	engine := newEngineWithDownloadProbes(config, Hooks{}, func(ip *net.IPAddr) (float64, string) {
		switch ip.String() {
		case "1.1.1.1":
			return 0, ""
		case "1.1.1.2":
			return 1 * 1024 * 1024, ""
		default:
			return 3 * 1024 * 1024, "HKG"
		}
	}, nil)

	result := engine.TestDownloadSpeed(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2", "1.1.1.3", "1.1.1.4"))
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

func TestDownloadSpeedFiltersThresholdByMaxMetric(t *testing.T) {
	config := DefaultConfig()
	config.Disable = false
	config.TestCount = 1
	config.MinSpeed = 10
	config.MinSpeedMetric = utils.DownloadSpeedMetricMax
	config.DownloadRoutines = 1
	engine := newEngineWithDownloadProbes(config, Hooks{}, nil, func(ip *net.IPAddr) downloadResult {
		if ip.String() == "1.1.1.1" {
			return validDownloadResult(5*1024*1024, 20*1024*1024, "SJC", 1, 1, time.Second)
		}
		return validDownloadResult(5*1024*1024, 8*1024*1024, "HKG", 1, 1, time.Second)
	})

	result := engine.TestDownloadSpeed(makeProbeSetWithIPs("1.1.1.1", "1.1.1.2"))
	if len(result) != 1 {
		t.Fatalf("download result count = %d, want 1", len(result))
	}
	if result[0].IP.String() != "1.1.1.1" {
		t.Fatalf("selected IP = %s, want max-speed qualified 1.1.1.1", result[0].IP)
	}
	if result[0].DownloadSpeed != 5*1024*1024 || result[0].MaxDownloadSpeed != 20*1024*1024 {
		t.Fatalf("speeds = avg %.0f max %.0f, want avg 5MiB/s max 20MiB/s", result[0].DownloadSpeed, result[0].MaxDownloadSpeed)
	}
}

func TestDownloadSpeedRejectsNonOKResponseAtZeroThreshold(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := DefaultConfig()
	config.URL = downloadURL
	config.TraceURL = downloadURL
	config.Disable = false
	config.TestCount = 1
	config.MinSpeed = 0
	config.DownloadRoutines = 1
	config.RetryMaxAttempts = 0
	config.TCPPort = port
	config.Timeout = time.Second
	engine := NewEngine(config, Hooks{})

	result := engine.TestDownloadSpeed(utils.PingDelaySet{
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(80 * time.Millisecond)
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := DefaultConfig()
	config.URL = downloadURL
	config.TraceURL = downloadURL
	config.Disable = false
	config.TestCount = 1
	config.MinSpeed = 0
	config.DownloadRoutines = 1
	config.RetryMaxAttempts = 0
	config.TCPPort = port
	config.Timeout = 20 * time.Millisecond
	config.DownloadWarmupDuration = 0
	engine := NewEngine(config, Hooks{})

	result := engine.TestDownloadSpeed(utils.PingDelaySet{
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

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := DefaultConfig()
	config.URL = downloadURL
	config.TraceURL = downloadURL
	config.TCPPort = port
	config.Timeout = 80 * time.Millisecond
	config.DownloadSampleInterval = time.Millisecond
	config.DownloadWarmupDuration = 0
	samples := make([]DownloadSpeedSample, 0)
	engine := NewEngine(config, Hooks{DownloadSpeedSample: func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}})

	result := engine.downloadHandlerAttempt(ip)
	speed, colo := result.speed, result.colo
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

func TestDownloadHandlerAttemptReportsRateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := DefaultConfig()
	config.URL = downloadURL
	config.TraceURL = downloadURL
	config.TCPPort = port
	config.Timeout = time.Second
	config.DownloadHTTPProtocol = "auto"
	engine := NewEngine(config, Hooks{})

	result := engine.downloadHandlerAttempt(ip)
	if result.reason != "rate_limited" || !result.retryable {
		t.Fatalf("download result = %#v, want retryable rate_limited", result)
	}
	if result.retryAfter != 2*time.Second {
		t.Fatalf("retryAfter = %v, want 2s", result.retryAfter)
	}
}

func TestDownloadHandlerUsesDownloadAuthorityWhenTraceDiffers(t *testing.T) {
	var seenHost string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHost = r.Host
		_, _ = w.Write([]byte(strings.Repeat("download", 1024)))
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.TraceURL = "http://trace.example.com/cdn-cgi/trace"
	config.HostHeader = "trace.example.com"
	config.SNI = "trace.example.com"
	config.Timeout = 30 * time.Millisecond
	config.DownloadWarmupDuration = 0
	engine := NewEngine(config, Hooks{})

	result := engine.downloadHandlerAttempt(ip)
	expectedHost := httpcfg.URLHostHeader(downloadURL)
	if !result.validMeasurement || result.speed <= 0 {
		t.Fatalf("download result = %#v, want valid measurement", result)
	}
	if seenHost != expectedHost {
		t.Fatalf("request Host = %q, want download authority %q", seenHost, expectedHost)
	}
}

func TestDownloadHandlerRejectsUnexpectedHTML(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
	}{
		{name: "declared html", contentType: "text/html; charset=utf-8"},
		{name: "html disguised as binary", contentType: "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = w.Write([]byte("<!DOCTYPE html><html><head><title>nginx panel</title></head><body>login</body></html>"))
			}))
			defer server.Close()

			ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
			config := downloadTestConfig(downloadURL, port)
			config.Timeout = 30 * time.Millisecond
			config.DownloadWarmupDuration = 0
			engine := NewEngine(config, Hooks{})

			result := engine.downloadHandlerAttempt(ip)
			if result.validMeasurement || result.speed != 0 || result.reason != "unexpected_html_response" {
				t.Fatalf("download result = %#v, want unexpected_html_response rejection", result)
			}
		})
	}
}

func TestDownloadHandlerRejectsRangeHTMLDisguisedAsBinary(t *testing.T) {
	body := []byte("<!DOCTYPE html><html><head><title>nginx panel</title></head><body>login</body></html>")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if got := r.Header.Get("Range"); got != "bytes=0-511" {
			t.Fatalf("Range = %q, want bytes=0-511", got)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(body)-1, len(body)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 30 * time.Millisecond
	config.DownloadWarmupDuration = 0
	engine := NewEngine(config, Hooks{})

	result := engine.downloadHandlerAttempt(ip)
	if result.validMeasurement || result.speed != 0 || result.reason != "unexpected_html_response" {
		t.Fatalf("download result = %#v, want unexpected_html_response rejection", result)
	}
	if requests.Load() != 1 {
		t.Fatalf("request count = %d, want rejection during range probe", requests.Load())
	}
}

func TestDownloadHandlerFallsBackToFullGetAfterRangeForbidden(t *testing.T) {
	body := []byte(strings.Repeat("a", 4096))
	var rangeRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			rangeRequests.Add(1)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 80 * time.Millisecond
	config.DownloadWarmupDuration = 0
	engine := NewEngine(config, Hooks{})

	result := engine.downloadHandlerAttempt(ip)
	if !result.validMeasurement || result.speed <= 0 {
		t.Fatalf("download result = %#v, want full GET fallback measurement", result)
	}
	if rangeRequests.Load() != 1 {
		t.Fatalf("range request count = %d, want 1", rangeRequests.Load())
	}
}

func TestDownloadHandlerRejectsCrossAuthorityRedirect(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "http://panel.example.test/login", http.StatusFound)
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 30 * time.Millisecond
	engine := NewEngine(config, Hooks{})

	result := engine.downloadHandlerAttempt(ip)
	if result.reason != "status_mismatch" || result.validMeasurement {
		t.Fatalf("download result = %#v, want cross-authority redirect rejection", result)
	}
	if requests.Load() != 1 {
		t.Fatalf("request count = %d, want redirect target not followed", requests.Load())
	}
}

func TestDownloadHandlerUsesRangeConcurrencyAndNoCacheHeaders(t *testing.T) {
	body := []byte(strings.Repeat("a", 4096))
	seenRanges := map[string]bool{}
	var seenMu sync.Mutex
	var current atomic.Int32
	var maxSeen atomic.Int32
	var badCacheHeader atomic.Bool
	var badCustomHeader atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cache-Control") != "no-store" || r.Header.Get("Pragma") != "no-cache" {
			badCacheHeader.Store(true)
		}
		if r.Header.Get("X-CFST-Test") != "download" {
			badCustomHeader.Store(true)
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

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 60 * time.Millisecond
	config.DownloadGetConcurrency = 4
	config.DownloadBufferKB = 64
	config.DownloadHTTPProtocol = "auto"
	config.DownloadSampleInterval = time.Millisecond
	config.DownloadWarmupDuration = 0
	config.RequestHeaders = "X-CFST-Test: download\nCache-Control: max-age=3600\nRange: bytes=1-2"
	engine := NewEngine(config, Hooks{})

	speed := engine.downloadHandlerAttempt(ip).speed
	if speed <= 0 {
		t.Fatalf("speed = %f, want positive range download speed", speed)
	}
	if badCacheHeader.Load() {
		t.Fatal("download request missing no-store/no-cache headers")
	}
	if badCustomHeader.Load() {
		t.Fatal("download request missing custom request header")
	}
	if maxSeen.Load() < 2 {
		t.Fatalf("max concurrent range requests = %d, want at least 2", maxSeen.Load())
	}
	seenMu.Lock()
	seenRangesSnapshot := make(map[string]bool, len(seenRanges))
	for value, seen := range seenRanges {
		seenRangesSnapshot[value] = seen
	}
	seenMu.Unlock()
	for _, want := range []string{"bytes=0-1023", "bytes=1024-2047", "bytes=2048-3071", "bytes=3072-4095"} {
		if !seenRangesSnapshot[want] {
			t.Fatalf("seen ranges = %#v, missing %s", seenRangesSnapshot, want)
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.setHeader(w.Header())
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				_, _ = w.Write(body)
			}))
			defer server.Close()

			ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
			config := downloadTestConfig(downloadURL, port)
			config.Timeout = 30 * time.Millisecond
			config.DownloadWarmupDuration = 0
			engine := NewEngine(config, Hooks{})

			speed := engine.downloadHandlerAttempt(ip).speed
			if speed <= 0 {
				t.Fatalf("speed = %f, want positive download with valid integrity header", speed)
			}
		})
	}
}

func TestDownloadHandlerRejectsIntegrityMismatch(t *testing.T) {
	body := []byte(strings.Repeat("integrity", 256))
	wrongSum := sha256.Sum256([]byte("wrong body"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Digest", "sha-256="+base64.StdEncoding.EncodeToString(wrongSum[:]))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 30 * time.Millisecond
	config.DownloadWarmupDuration = 0
	engine := NewEngine(config, Hooks{})

	speed := engine.downloadHandlerAttempt(ip).speed
	if speed != 0 {
		t.Fatalf("speed = %f, want rejected digest mismatch", speed)
	}
}

func TestDownloadHandlerExcludesWarmupFromAverage(t *testing.T) {
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

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 80 * time.Millisecond
	config.DownloadSampleInterval = time.Millisecond
	config.DownloadWarmupDuration = 10 * time.Millisecond
	samples := make([]DownloadSpeedSample, 0)
	engine := NewEngine(config, Hooks{DownloadSpeedSample: func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}})

	speed := engine.downloadHandlerAttempt(ip).speed
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
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("cf-ray", "8f00abcdef-SJC")
		_, _ = w.Write([]byte(strings.Repeat("a", 4*1024)))
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 80 * time.Millisecond
	config.DownloadSampleInterval = time.Millisecond
	config.DownloadWarmupDuration = 20 * time.Millisecond
	samples := make([]DownloadSpeedSample, 0)
	engine := NewEngine(config, Hooks{DownloadSpeedSample: func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}})

	speed := engine.downloadHandlerAttempt(ip).speed
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		_, _ = w.Write([]byte(strings.Repeat("a", 4*1024)))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(300 * time.Millisecond)
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 40 * time.Millisecond
	config.DownloadSampleInterval = time.Millisecond
	config.DownloadWarmupDuration = 500 * time.Millisecond
	samples := make([]DownloadSpeedSample, 0)
	engine := NewEngine(config, Hooks{DownloadSpeedSample: func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}})

	speed := engine.downloadHandlerAttempt(ip).speed
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(80 * time.Millisecond)
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 20 * time.Millisecond
	config.DownloadSampleInterval = time.Millisecond
	config.DownloadWarmupDuration = 5 * time.Millisecond
	samples := make([]DownloadSpeedSample, 0)
	engine := NewEngine(config, Hooks{DownloadSpeedSample: func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}})

	speed := engine.downloadHandlerAttempt(ip).speed
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

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 70 * time.Millisecond
	config.DownloadSampleInterval = time.Millisecond
	config.DownloadWarmupDuration = 10 * time.Millisecond
	samples := make([]DownloadSpeedSample, 0)
	engine := NewEngine(config, Hooks{DownloadSpeedSample: func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}})

	speed := engine.downloadHandlerAttempt(ip).speed
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
		if sample.ElapsedMS >= config.DownloadWarmupDuration.Milliseconds() && sample.CurrentReady && sample.CurrentSpeedMBs == 0 {
			t.Fatalf("samples = %#v, want no ready zero current-speed sample during reconnects", samples)
		}
	}
}

func TestDownloadSpeedSampleIntervalDefault(t *testing.T) {
	config := DefaultConfig()
	config.DownloadSampleInterval = 0
	got := NewEngine(config, Hooks{}).Config().DownloadSampleInterval
	if got != 500*time.Millisecond {
		t.Fatalf("DownloadSampleInterval = %v, want 500ms", got)
	}
}

func TestDownloadHandlerSamplesOnIntervalAndFinal(t *testing.T) {
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

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 90 * time.Millisecond
	config.DownloadSampleInterval = 25 * time.Millisecond
	config.DownloadWarmupDuration = 0
	samples := make([]DownloadSpeedSample, 0)
	engine := NewEngine(config, Hooks{DownloadSpeedSample: func(sample DownloadSpeedSample) {
		samples = append(samples, sample)
	}})

	speed := engine.downloadHandlerAttempt(ip).speed
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
		w.Header().Set("Content-Length", "32768")
		for range 4 {
			if _, err := w.Write([]byte(strings.Repeat("b", 8*1024))); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(time.Millisecond)
		}
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.DownloadHTTPProtocol = "auto"
	config.RequestHeaders = ""
	config.DownloadGetConcurrency = 1
	config.DownloadBufferKB = defaultDownloadBufferKB
	config.Timeout = time.Second
	config.DownloadSampleInterval = time.Millisecond
	config.DownloadWarmupDuration = 0
	config.RetryMaxAttempts = 1

	var pauses atomic.Int32
	var registeredInterrupts atomic.Int32
	pauseCh := make(chan struct{})
	resumeCh := make(chan struct{})
	hooks := Hooks{}
	hooks.ProbePause = func(stage, pauseIP string) {
		if stage != "stage3_get" || pauseIP != ip.String() {
			return
		}
		if pauses.Add(1) == 1 {
			close(pauseCh)
			<-resumeCh
		}
	}
	hooks.DownloadInterrupt = func(stage, interruptIP string, interrupt func()) func() {
		if stage == "stage3_get" && interruptIP == ip.String() && registeredInterrupts.Add(1) == 1 {
			go func() {
				<-firstRequestStarted
				interrupt()
			}()
		}
		return func() {}
	}
	engine := NewEngine(config, hooks)

	resumed := make(chan struct{})
	go func() {
		<-pauseCh
		close(resumeCh)
		close(resumed)
	}()

	result := engine.downloadHandlerAttempt(ip)
	speed, colo := result.speed, result.colo
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

func TestDownloadHandlerCancelInterruptStopsWithoutRetry(t *testing.T) {
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

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.DownloadHTTPProtocol = "auto"
	config.RequestHeaders = ""
	config.DownloadGetConcurrency = 1
	config.DownloadBufferKB = defaultDownloadBufferKB
	config.Timeout = time.Second
	config.DownloadSampleInterval = time.Millisecond
	config.DownloadWarmupDuration = 0
	config.RetryMaxAttempts = 3

	var canceled atomic.Bool
	hooks := Hooks{}
	hooks.ProbeCancel = func(stage, cancelIP string) bool {
		return stage == "stage3_get" && cancelIP == ip.String() && canceled.Load()
	}
	hooks.DownloadInterrupt = func(stage, interruptIP string, interrupt func()) func() {
		if stage == "stage3_get" && interruptIP == ip.String() {
			go func() {
				<-firstRequestStarted
				canceled.Store(true)
				interrupt()
			}()
		}
		return func() {}
	}
	engine := NewEngine(config, hooks)

	speed := engine.downloadHandlerAttempt(ip).speed
	if speed != 0 {
		t.Fatalf("speed = %f, want canceled download without measurement", speed)
	}
	select {
	case <-firstRequestInterrupted:
	case <-time.After(time.Second):
		t.Fatal("first download request was not interrupted")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want cancel interrupt to stop without retry", got)
	}
}

func TestTraceProbeInterruptRestartsSameIPWithoutConsumingRetry(t *testing.T) {
	var requests atomic.Int32
	firstRequestStarted := make(chan struct{})
	firstRequestInterrupted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestNo := requests.Add(1)
		w.Header().Set("cf-ray", "8f00abcdef-SJC")
		if requestNo == 1 {
			close(firstRequestStarted)
			_, _ = w.Write([]byte("colo=SJC\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			<-r.Context().Done()
			close(firstRequestInterrupted)
			return
		}
		_, _ = w.Write([]byte("colo=SJC\n"))
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.TCPPort = port
	config.TraceURL = traceURL
	config.HeadTimeout = time.Second
	config.RetryMaxAttempts = 0
	config.RetryBackoff = 0

	var pauses atomic.Int32
	var registeredInterrupts atomic.Int32
	pauseCh := make(chan struct{})
	resumeCh := make(chan struct{})
	hooks := Hooks{}
	hooks.ProbePause = func(stage, pauseIP string) {
		if stage != "stage2_trace" || pauseIP != ip.String() {
			return
		}
		if pauses.Add(1) == 1 {
			close(pauseCh)
			<-resumeCh
		}
	}
	hooks.TraceInterrupt = func(stage, interruptIP string, interrupt func()) func() {
		if stage == "stage2_trace" && interruptIP == ip.String() && registeredInterrupts.Add(1) == 1 {
			go func() {
				<-firstRequestStarted
				interrupt()
			}()
		}
		return func() {}
	}
	engine := NewEngine(config, hooks)

	resumed := make(chan struct{})
	go func() {
		<-pauseCh
		close(resumeCh)
		close(resumed)
	}()

	result := engine.runTraceProbeWithRetry(ip)
	if !result.ok {
		t.Fatalf("trace result = %#v, want resumed trace success", result)
	}
	if result.colo != "SJC" {
		t.Fatalf("colo = %q, want SJC", result.colo)
	}
	select {
	case <-firstRequestInterrupted:
	case <-time.After(time.Second):
		t.Fatal("first trace request was not interrupted")
	}
	select {
	case <-resumed:
	case <-time.After(time.Second):
		t.Fatal("trace pause hook did not resume")
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want same IP restarted after interruption", got)
	}
	if got := pauses.Load(); got < 1 {
		t.Fatalf("pause count = %d, want at least 1", got)
	}
}

func TestTraceProbeCancelInterruptStopsWithoutRetry(t *testing.T) {
	var requests atomic.Int32
	firstRequestStarted := make(chan struct{})
	firstRequestInterrupted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestNo := requests.Add(1)
		w.Header().Set("cf-ray", "8f00abcdef-SJC")
		if requestNo == 1 {
			close(firstRequestStarted)
			_, _ = w.Write([]byte("colo=SJC\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			<-r.Context().Done()
			close(firstRequestInterrupted)
			return
		}
		_, _ = w.Write([]byte("colo=SJC\n"))
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.TCPPort = port
	config.TraceURL = traceURL
	config.HeadTimeout = time.Second
	config.RetryMaxAttempts = 3
	config.RetryBackoff = 0

	var canceled atomic.Bool
	hooks := Hooks{}
	hooks.ProbeCancel = func(stage, cancelIP string) bool {
		return stage == "stage2_trace" && cancelIP == ip.String() && canceled.Load()
	}
	hooks.TraceInterrupt = func(stage, interruptIP string, interrupt func()) func() {
		if stage == "stage2_trace" && interruptIP == ip.String() {
			go func() {
				<-firstRequestStarted
				canceled.Store(true)
				interrupt()
			}()
		}
		return func() {}
	}
	engine := NewEngine(config, hooks)

	result := engine.runTraceProbeWithRetry(ip)
	if result.reason != traceFailureInterrupted {
		t.Fatalf("trace result reason = %q, want %q", result.reason, traceFailureInterrupted)
	}
	select {
	case <-firstRequestInterrupted:
	case <-time.After(time.Second):
		t.Fatal("first trace request was not interrupted")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want cancel interrupt to stop without retry", got)
	}
}

func TestTraceAvailabilityRecoversWorkerPanic(t *testing.T) {
	ipA := parseTestIP("1.1.1.1")
	ipB := parseTestIP("1.1.1.2")
	config := DefaultConfig()
	config.HeadRoutines = 1
	config.RetryMaxAttempts = 0
	config.RetryBackoff = 0
	config.TraceColoMode = TraceColoModeTraceURL
	config.TraceURL = "https://example.com/cdn-cgi/trace"
	engine := newEngineWithTraceProbe(config, Hooks{}, func(ip *net.IPAddr) traceProbeResult {
		if ip.String() == ipA.String() {
			panic("boom")
		}
		return traceProbeResult{ok: true, colo: "SJC"}
	})

	got := engine.TestTraceAvailability(utils.PingDelaySet{
		{PingData: &utils.PingData{IP: ipA}},
		{PingData: &utils.PingData{IP: ipB}},
	})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 surviving candidate", len(got))
	}
	if got[0].IP.String() != ipB.String() {
		t.Fatalf("surviving IP = %s, want %s", got[0].IP.String(), ipB.String())
	}
}

func TestTraceProbeTimeoutConsumesRetryBudget(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		<-r.Context().Done()
	}))
	defer server.Close()

	ip, port, traceURL := probeServerEndpoint(t, server.URL, "/cdn-cgi/trace")
	config := DefaultConfig()
	config.TCPPort = port
	config.TraceURL = traceURL
	config.HeadTimeout = 20 * time.Millisecond
	config.RetryMaxAttempts = 1
	config.RetryBackoff = 0
	engine := NewEngine(config, Hooks{})

	result := engine.runTraceProbeWithRetry(ip)
	if result.ok {
		t.Fatalf("trace result = %#v, want timeout failure", result)
	}
	if result.reason != traceFailureRequest {
		t.Fatalf("reason = %q, want %q", result.reason, traceFailureRequest)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2 attempts bounded by retry budget", got)
	}
}

func TestDownloadHandlerTimeoutsStalledBodyRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(300 * time.Millisecond)
	}))
	defer server.Close()

	ip, port, downloadURL := probeServerEndpoint(t, server.URL, "/download.bin")
	config := downloadTestConfig(downloadURL, port)
	config.Timeout = 40 * time.Millisecond
	config.DownloadWarmupDuration = 0
	engine := NewEngine(config, Hooks{})

	done := make(chan struct{})
	go func() {
		_ = engine.downloadHandlerAttempt(ip)
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

func newEngineWithTraceProbe(config Config, hooks Hooks, probe func(*net.IPAddr) traceProbeResult) *Engine {
	engine := NewEngine(config, hooks)
	if probe != nil {
		engine.traceProbe = func(_ *Engine, ip *net.IPAddr) traceProbeResult {
			return probe(ip)
		}
	}
	return engine
}

func newEngineWithDownloadProbes(
	config Config,
	hooks Hooks,
	probe func(*net.IPAddr) (float64, string),
	resultProbe func(*net.IPAddr) downloadResult,
) *Engine {
	engine := NewEngine(config, hooks)
	if probe != nil {
		engine.downloadHandler = func(_ *Engine, ip *net.IPAddr) (float64, string) {
			return probe(ip)
		}
	}
	if resultProbe != nil {
		engine.downloadHandlerResult = func(_ *Engine, ip *net.IPAddr) downloadResult {
			return resultProbe(ip)
		}
	}
	return engine
}

func downloadTestConfig(downloadURL string, port int) Config {
	config := DefaultConfig()
	config.URL = downloadURL
	config.TraceURL = downloadURL
	config.TCPPort = port
	return config
}

func slicesContainString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func probeServerEndpoint(t *testing.T, serverURL, path string) (*net.IPAddr, int, string) {
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
	return &net.IPAddr{IP: ip}, port, parsed.String()
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
