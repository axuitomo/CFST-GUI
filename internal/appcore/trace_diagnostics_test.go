package appcore

import (
	"strings"
	"sync"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
)

func TestTraceDiagnosticsPayloadAndSummary(t *testing.T) {
	diagnostics := NewTraceDiagnostics("trace_url", "https://trace.example.com/cdn-cgi/trace")
	diagnostics.Record(task.TraceDiagnostic{
		IP:           "1.1.1.1",
		Reason:       "rate_limited",
		RetryAfterMS: 1500,
		StatusCode:   429,
		URL:          "https://trace.example.com/cdn-cgi/trace",
	})
	diagnostics.Record(task.TraceDiagnostic{
		Error:      "read timeout",
		IP:         "1.1.1.2",
		Reason:     "trace_error",
		StatusCode: 504,
		URL:        "https://trace.example.com/cdn-cgi/trace",
	})
	diagnostics.Record(task.TraceDiagnostic{
		IP:         "1.1.1.3",
		Reason:     "rate_limited",
		StatusCode: 429,
		URL:        "https://trace.example.com/cdn-cgi/trace",
	})

	payload := diagnostics.Payload()
	if got, _ := payload["trace_colo_mode"].(string); got != "trace_url" {
		t.Fatalf("trace_colo_mode = %q, want trace_url", got)
	}
	if got, _ := payload["trace_url"].(string); got != "https://trace.example.com/cdn-cgi/trace" {
		t.Fatalf("trace_url = %q, want configured trace URL", got)
	}
	reasonCounts, ok := payload["reason_counts"].(map[string]int)
	if !ok {
		t.Fatalf("reason_counts = %#v, want map[string]int", payload["reason_counts"])
	}
	if got := reasonCounts["rate_limited"]; got != 2 {
		t.Fatalf("rate_limited count = %d, want 2", got)
	}
	if got := reasonCounts["trace_error"]; got != 1 {
		t.Fatalf("trace_error count = %d, want 1", got)
	}
	statusCounts, ok := payload["status_counts"].(map[string]int)
	if !ok {
		t.Fatalf("status_counts = %#v, want map[string]int", payload["status_counts"])
	}
	if got := statusCounts["429"]; got != 2 {
		t.Fatalf("status 429 count = %d, want 2", got)
	}
	samples, ok := payload["samples"].([]map[string]any)
	if !ok || len(samples) != 3 {
		t.Fatalf("samples = %#v, want 3 structured samples", payload["samples"])
	}
	summary := diagnostics.Summary()
	if !strings.Contains(summary, "服务端限流 2 次") {
		t.Fatalf("summary = %q, want rate limited summary", summary)
	}
	if !strings.Contains(summary, "HTTP 429 2 次") {
		t.Fatalf("summary = %q, want HTTP 429 summary", summary)
	}
}

// 追踪阶段每个 worker 都会记一条诊断；Record 与读侧必须能被并发调用。
func TestTraceDiagnosticsConcurrentRecord(t *testing.T) {
	diagnostics := NewTraceDiagnostics("standard", "https://trace.example.com/cdn-cgi/trace")
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			diagnostics.Record(task.TraceDiagnostic{IP: "1.1.1.1", Reason: "trace_error", StatusCode: 522})
		}()
	}
	wg.Wait()

	payload := diagnostics.Payload()
	reasonCounts, ok := payload["reason_counts"].(map[string]int)
	if !ok || reasonCounts["trace_error"] != 32 {
		t.Fatalf("reason_counts = %#v, want trace_error=32", payload["reason_counts"])
	}
	if samples, ok := payload["samples"].([]map[string]any); !ok || len(samples) != 3 {
		t.Fatalf("samples = %#v, want 3 structured samples", payload["samples"])
	}
	summary := diagnostics.Summary()
	if !strings.Contains(summary, "追踪请求失败 32 次") {
		t.Fatalf("summary = %q, want trace_error summary", summary)
	}
}

func TestShouldMarkTraceFailureStage(t *testing.T) {
	diagnostics := NewTraceDiagnostics("standard", "https://trace.example.com/cdn-cgi/trace")
	diagnostics.Record(task.TraceDiagnostic{
		IP:         "1.1.1.1",
		Reason:     "rate_limited",
		StatusCode: 429,
	})
	if !ShouldMarkTraceFailureStage([]string{probecore.StageTCP, probecore.StageTrace}, diagnostics, nil) {
		t.Fatal("ShouldMarkTraceFailureStage = false, want true")
	}
	if ShouldMarkTraceFailureStage([]string{probecore.StageTCP, probecore.StageTrace, probecore.StageDownload}, diagnostics, nil) {
		t.Fatal("ShouldMarkTraceFailureStage = true with download stage, want false")
	}
}
