package appcore

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/task"
	"github.com/axuitomo/CFST-GUI/utils"
)

type traceDiagnosticSample struct {
	Colo         string `json:"colo,omitempty"`
	Error        string `json:"error,omitempty"`
	IP           string `json:"ip,omitempty"`
	Reason       string `json:"reason"`
	RetryAfterMS int64  `json:"retry_after_ms,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
	URL          string `json:"url,omitempty"`
}

type TraceDiagnosticsCollector struct {
	reasonCounts  map[string]int
	samples       []traceDiagnosticSample
	statusCounts  map[string]int
	traceColoMode string
	traceURL      string
}

func NewTraceDiagnostics(traceColoMode, traceURL string) *TraceDiagnosticsCollector {
	return &TraceDiagnosticsCollector{
		reasonCounts:  make(map[string]int),
		statusCounts:  make(map[string]int),
		traceColoMode: strings.TrimSpace(traceColoMode),
		traceURL:      strings.TrimSpace(traceURL),
	}
}

func (d *TraceDiagnosticsCollector) Record(entry task.TraceDiagnostic) {
	if d == nil {
		return
	}
	reason := strings.TrimSpace(entry.Reason)
	if reason == "" {
		return
	}
	d.reasonCounts[reason]++
	if entry.StatusCode > 0 {
		d.statusCounts[strconv.Itoa(entry.StatusCode)]++
	}
	if len(d.samples) >= 3 {
		return
	}
	d.samples = append(d.samples, traceDiagnosticSample{
		Colo:         strings.TrimSpace(entry.Colo),
		Error:        strings.TrimSpace(entry.Error),
		IP:           strings.TrimSpace(entry.IP),
		Reason:       reason,
		RetryAfterMS: entry.RetryAfterMS,
		StatusCode:   entry.StatusCode,
		URL:          strings.TrimSpace(entry.URL),
	})
}

func (d *TraceDiagnosticsCollector) Empty() bool {
	return d == nil || (len(d.reasonCounts) == 0 && len(d.samples) == 0 && len(d.statusCounts) == 0)
}

func (d *TraceDiagnosticsCollector) Payload() map[string]any {
	if d.Empty() {
		return nil
	}
	samples := make([]map[string]any, 0, len(d.samples))
	for _, sample := range d.samples {
		row := map[string]any{
			"reason": sample.Reason,
		}
		if sample.Colo != "" {
			row["colo"] = sample.Colo
		}
		if sample.Error != "" {
			row["error"] = sample.Error
		}
		if sample.IP != "" {
			row["ip"] = sample.IP
		}
		if sample.RetryAfterMS > 0 {
			row["retry_after_ms"] = sample.RetryAfterMS
		}
		if sample.StatusCode > 0 {
			row["status_code"] = sample.StatusCode
		}
		if sample.URL != "" {
			row["url"] = sample.URL
		}
		samples = append(samples, row)
	}
	return map[string]any{
		"reason_counts":   cloneStringIntMap(d.reasonCounts),
		"samples":         samples,
		"status_counts":   cloneStringIntMap(d.statusCounts),
		"trace_colo_mode": d.traceColoMode,
		"trace_url":       d.traceURL,
	}
}

func (d *TraceDiagnosticsCollector) Summary() string {
	if d.Empty() {
		return ""
	}
	reason, count := d.topReason()
	parts := make([]string, 0, 3)
	if reason != "" && count > 0 {
		parts = append(parts, fmt.Sprintf("%s %d 次", traceReasonLabel(reason), count))
	}
	if code, count := d.topStatus(); code != "" && count > 0 {
		parts = append(parts, fmt.Sprintf("HTTP %s %d 次", code, count))
	}
	if len(d.samples) > 0 {
		sample := d.samples[0]
		switch {
		case sample.Error != "":
			parts = append(parts, sample.Error)
		case sample.IP != "" || sample.URL != "":
			parts = append(parts, strings.Join(filterNonEmpty(sample.IP, sample.URL), " · "))
		}
	}
	return strings.Join(parts, "；")
}

func StageTraceFailureMessage(summary string, fallback string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fallback
	}
	return "追踪阶段失败：" + summary
}

func ShouldMarkTraceFailureStage(completedStages []string, diagnostics *TraceDiagnosticsCollector, resultData []utils.CloudflareIPData) bool {
	if diagnostics.Empty() || len(resultData) > 0 {
		return false
	}
	sawTrace := false
	for _, stage := range completedStages {
		switch stage {
		case probecore.StageDownload:
			return false
		case probecore.StageTrace:
			sawTrace = true
		}
	}
	return sawTrace
}

func (d *TraceDiagnosticsCollector) topReason() (string, int) {
	if len(d.reasonCounts) == 0 {
		return "", 0
	}
	type reasonCount struct {
		count  int
		reason string
	}
	values := make([]reasonCount, 0, len(d.reasonCounts))
	for reason, count := range d.reasonCounts {
		values = append(values, reasonCount{reason: reason, count: count})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].count == values[j].count {
			return values[i].reason < values[j].reason
		}
		return values[i].count > values[j].count
	})
	return values[0].reason, values[0].count
}

func (d *TraceDiagnosticsCollector) topStatus() (string, int) {
	if len(d.statusCounts) == 0 {
		return "", 0
	}
	type statusCount struct {
		code  string
		count int
	}
	values := make([]statusCount, 0, len(d.statusCounts))
	for code, count := range d.statusCounts {
		values = append(values, statusCount{code: code, count: count})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].count == values[j].count {
			return values[i].code < values[j].code
		}
		return values[i].count > values[j].count
	})
	return values[0].code, values[0].count
}

func traceReasonLabel(reason string) string {
	switch strings.TrimSpace(reason) {
	case "colo_filter":
		return "地区码不匹配"
	case "rate_limited":
		return "服务端限流"
	case "request_create_failed":
		return "追踪请求创建失败"
	case "source_colo_filter":
		return "输入源 COLO 过滤未通过"
	case "status_mismatch":
		return "状态码不匹配"
	case "trace_error":
		return "追踪请求失败"
	case "trace_latency_limit":
		return "追踪延迟超阈值"
	case "trace_read_error":
		return "追踪响应读取失败"
	default:
		if strings.TrimSpace(reason) == "" {
			return "未知原因"
		}
		return strings.TrimSpace(reason)
	}
}

func cloneStringIntMap(source map[string]int) map[string]int {
	if len(source) == 0 {
		return map[string]int{}
	}
	cloned := make(map[string]int, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func filterNonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
