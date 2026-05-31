package appcore

import (
	"net"
	"slices"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

const (
	uploadFilterStatusAll    = "all"
	uploadFilterStatusPassed = "passed"
	uploadFilterIPVersionAny = "any"
	uploadFilterIPVersionV4  = "ipv4"
	uploadFilterIPVersionV6  = "ipv6"

	cloudflareRecordTypeA    = "A"
	cloudflareRecordTypeAAAA = "AAAA"
)

type UploadSelectionConfig struct {
	SharedFilter   UploadSharedFilterConfig `json:"shared_filter"`
	CloudflareTopN int                      `json:"cloudflare_top_n"`
	GitHubTopN     int                      `json:"github_top_n"`
}

type UploadSharedFilterConfig struct {
	Enabled           bool     `json:"enabled"`
	Status            string   `json:"status"`
	IPVersion         string   `json:"ip_version"`
	ColoAllow         []string `json:"colo_allow"`
	ColoDeny          []string `json:"colo_deny"`
	MaxTCPLatencyMS   *float64 `json:"max_tcp_latency_ms"`
	MaxTraceLatencyMS *float64 `json:"max_trace_latency_ms"`
	MinDownloadMBPS   float64  `json:"min_download_mbps"`
	MaxLossRate       *float64 `json:"max_loss_rate"`
}

type UploadSelectionResult struct {
	InputRows      []probecore.ProbeRow `json:"input_rows"`
	FilteredRows   []probecore.ProbeRow `json:"filtered_rows"`
	CloudflareRows []probecore.ProbeRow `json:"cloudflare_rows"`
	GitHubRows     []probecore.ProbeRow `json:"github_rows"`
	Warnings       []string             `json:"warnings"`
}

func BuildUploadSelection(snapshot map[string]any, rows []probecore.ProbeRow, metric string) (UploadSelectionResult, error) {
	cfg := uploadSelectionConfigFromSnapshot(snapshot)
	inputRows := cloneProbeRows(rows)
	filteredRows := cloneProbeRows(rows)
	warnings := make([]string, 0)

	if cfg.SharedFilter.Enabled {
		filteredRows = make([]probecore.ProbeRow, 0, len(rows))
		for _, row := range rows {
			if uploadRowMatchesSharedFilter(row, cfg.SharedFilter) {
				filteredRows = append(filteredRows, row)
			}
		}
	}

	cloudflareRows := limitUploadRows(filteredRows, cfg.CloudflareTopN, metric)
	githubRows := limitUploadRows(filteredRows, cfg.GitHubTopN, metric)

	if cfg.SharedFilter.Enabled && len(filteredRows) == 0 {
		warnings = append(warnings, "共享上传筛选后没有剩余结果。")
	}

	return UploadSelectionResult{
		InputRows:      inputRows,
		FilteredRows:   filteredRows,
		CloudflareRows: cloudflareRows,
		GitHubRows:     githubRows,
		Warnings:       dedupeStrings(warnings),
	}, nil
}

func FilterRowsForCloudflareRecordType(rows []probecore.ProbeRow, recordType string) []probecore.ProbeRow {
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	if recordType != cloudflareRecordTypeAAAA {
		recordType = cloudflareRecordTypeA
	}
	filtered := make([]probecore.ProbeRow, 0, len(rows))
	for _, row := range rows {
		ip := net.ParseIP(strings.TrimSpace(row.IP))
		if ip == nil {
			continue
		}
		if recordType == cloudflareRecordTypeA && ip.To4() != nil {
			filtered = append(filtered, row)
		}
		if recordType == cloudflareRecordTypeAAAA && ip.To4() == nil {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func uploadSelectionConfigFromSnapshot(snapshot map[string]any) UploadSelectionConfig {
	upload := mapValue(snapshot["upload"])
	shared := mapValue(upload["shared_filter"])
	cloudflare := mapValue(upload["cloudflare"])
	github := mapValue(upload["github"])

	return UploadSelectionConfig{
		SharedFilter: UploadSharedFilterConfig{
			Enabled:           boolValue(shared["enabled"], false),
			Status:            normalizeUploadFilterStatus(stringValue(shared["status"], uploadFilterStatusPassed)),
			IPVersion:         normalizeUploadFilterIPVersion(stringValue(firstNonNil(shared["ip_version"], shared["ipVersion"]), uploadFilterIPVersionAny)),
			ColoAllow:         normalizeUploadFilterTokens(stringValue(firstNonNil(shared["colo_allow"], shared["coloAllow"]), "")),
			ColoDeny:          normalizeUploadFilterTokens(stringValue(firstNonNil(shared["colo_deny"], shared["coloDeny"]), "")),
			MaxTCPLatencyMS:   uploadOptionalFloat(shared["max_tcp_latency_ms"]),
			MaxTraceLatencyMS: uploadOptionalFloat(firstNonNil(shared["max_trace_latency_ms"], shared["maxTraceLatencyMs"])),
			MinDownloadMBPS:   floatValue(firstNonNil(shared["min_download_mbps"], shared["minDownloadMbps"]), 0),
			MaxLossRate:       uploadOptionalFloat(firstNonNil(shared["max_loss_rate"], shared["maxLossRate"])),
		},
		CloudflareTopN: max(0, intValue(firstNonNil(cloudflare["top_n"], cloudflare["topN"]), 0)),
		GitHubTopN:     max(0, intValue(firstNonNil(github["top_n"], github["topN"]), 0)),
	}
}

func limitUploadRows(rows []probecore.ProbeRow, topN int, metric string) []probecore.ProbeRow {
	if len(rows) == 0 {
		return nil
	}
	raw := make([]probecore.ProbeRow, len(rows))
	copy(raw, rows)
	selected := probecore.SelectTopProbeRowsByMetric(raw, topN, metric)
	return cloneProbeRows(selected)
}

func uploadRowMatchesSharedFilter(row probecore.ProbeRow, cfg UploadSharedFilterConfig) bool {
	if normalizeUploadFilterStatus(cfg.Status) == uploadFilterStatusPassed {
		// ProbeRow currently only represents successful/exportable rows.
	}

	if !uploadRowMatchesIPVersion(row, cfg.IPVersion) {
		return false
	}

	colo := strings.ToUpper(strings.TrimSpace(row.Colo))
	if colo == "N/A" {
		colo = ""
	}
	if len(cfg.ColoDeny) > 0 && colo != "" && slices.Contains(cfg.ColoDeny, colo) {
		return false
	}
	if len(cfg.ColoAllow) > 0 {
		if colo == "" || !slices.Contains(cfg.ColoAllow, colo) {
			return false
		}
	}

	if cfg.MaxTCPLatencyMS != nil && row.DelayMS > *cfg.MaxTCPLatencyMS {
		return false
	}
	if cfg.MaxTraceLatencyMS != nil && row.TraceDelayMS > *cfg.MaxTraceLatencyMS {
		return false
	}
	if row.DownloadSpeedMB < cfg.MinDownloadMBPS {
		return false
	}
	if cfg.MaxLossRate != nil && row.LossRate > *cfg.MaxLossRate {
		return false
	}
	return true
}

func uploadRowMatchesIPVersion(row probecore.ProbeRow, version string) bool {
	if normalizeUploadFilterIPVersion(version) == uploadFilterIPVersionAny {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(row.IP))
	if ip == nil {
		return false
	}
	switch normalizeUploadFilterIPVersion(version) {
	case uploadFilterIPVersionV4:
		return ip.To4() != nil
	case uploadFilterIPVersionV6:
		return ip.To4() == nil
	default:
		return true
	}
}

func cloneProbeRows(rows []probecore.ProbeRow) []probecore.ProbeRow {
	if len(rows) == 0 {
		return nil
	}
	cloned := make([]probecore.ProbeRow, len(rows))
	copy(cloned, rows)
	return cloned
}

func normalizeUploadFilterTokens(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' ' || r == ';'
	})
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		token := strings.ToUpper(strings.TrimSpace(part))
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}

func normalizeUploadFilterStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case uploadFilterStatusAll:
		return uploadFilterStatusAll
	default:
		return uploadFilterStatusPassed
	}
}

func normalizeUploadFilterIPVersion(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case uploadFilterIPVersionV4:
		return uploadFilterIPVersionV4
	case uploadFilterIPVersionV6:
		return uploadFilterIPVersionV6
	default:
		return uploadFilterIPVersionAny
	}
}

func uploadOptionalFloat(value any) *float64 {
	if value == nil {
		return nil
	}
	if text := strings.TrimSpace(stringValue(value, "")); text == "" {
		return nil
	}
	floatVal := floatValue(value, 0)
	if floatVal < 0 {
		return nil
	}
	return &floatVal
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
