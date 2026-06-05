package appcore

import (
	"net"
	"slices"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
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
	cloudflareRecordTypeAll  = "ALL"
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

type UploadCloudflareRoutingConfig struct {
	Enabled bool                          `json:"enabled"`
	Rules   []UploadCloudflareRoutingRule `json:"rules"`
}

type UploadCloudflareRoutingRule struct {
	Enabled      bool   `json:"enabled"`
	FilterMode   string `json:"filter_mode"`
	FilterTokens string `json:"filter_tokens"`
	Name         string `json:"name"`
	RecordName   string `json:"record_name"`
	RecordType   string `json:"record_type"`
	TopN         int    `json:"top_n"`
}

type UploadCloudflareRouteSelection struct {
	FilteredCount int                         `json:"filtered_count"`
	InputCount    int                         `json:"input_count"`
	Rows          []probecore.ProbeRow        `json:"rows"`
	Rule          UploadCloudflareRoutingRule `json:"rule"`
	Skipped       bool                        `json:"skipped"`
	Warnings      []string                    `json:"warnings"`
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

func CloudflareRoutingConfigFromSnapshot(snapshot map[string]any) UploadCloudflareRoutingConfig {
	upload := mapValue(snapshot["upload"])
	legacyCloudflare := mapValue(upload["cloudflare"])
	cloudflare := mapValue(snapshot["cloudflare"])
	if len(cloudflare) == 0 {
		cloudflare = legacyCloudflare
	}
	return UploadCloudflareRoutingConfig{
		Enabled: boolValue(firstNonNil(cloudflare["routing_enabled"], cloudflare["routingEnabled"], legacyCloudflare["routing_enabled"], legacyCloudflare["routingEnabled"]), false),
		Rules:   uploadCloudflareRoutingRulesFromAny(firstNonNil(cloudflare["routing_rules"], cloudflare["routingRules"], legacyCloudflare["routing_rules"], legacyCloudflare["routingRules"])),
	}
}

func BuildCloudflareRouteSelections(snapshot map[string]any, rows []probecore.ProbeRow, metric string, paths colodict.Paths) ([]UploadCloudflareRouteSelection, []string) {
	cfg := CloudflareRoutingConfigFromSnapshot(snapshot)
	if !cfg.Enabled || len(cfg.Rules) == 0 {
		return nil, nil
	}
	result := make([]UploadCloudflareRouteSelection, 0, len(cfg.Rules))
	warnings := make([]string, 0)
	for _, rule := range cfg.Rules {
		if !rule.Enabled {
			continue
		}
		selection := UploadCloudflareRouteSelection{InputCount: len(rows), Rule: rule}
		if strings.TrimSpace(rule.RecordName) == "" {
			selection.Skipped = true
			selection.Warnings = append(selection.Warnings, uploadRouteWarning(rule, "目标 DNS 记录名为空，已跳过。"))
			result = append(result, selection)
			warnings = append(warnings, selection.Warnings...)
			continue
		}
		routeRows, routeWarnings := filterUploadRowsForCloudflareRoute(rows, rule, paths)
		selection.Warnings = append(selection.Warnings, routeWarnings...)
		selection.FilteredCount = len(routeRows)
		selection.Rows = limitUploadRows(routeRows, rule.TopN, metric)
		if len(selection.Rows) == 0 {
			selection.Skipped = true
			selection.Warnings = append(selection.Warnings, uploadRouteWarning(rule, "筛选后无匹配 IP，已跳过。"))
		}
		result = append(result, selection)
		warnings = append(warnings, selection.Warnings...)
	}
	return result, dedupeStrings(warnings)
}

func FilterRowsForCloudflareRecordType(rows []probecore.ProbeRow, recordType string) []probecore.ProbeRow {
	recordType = normalizeCloudflareRecordType(recordType)
	filtered := make([]probecore.ProbeRow, 0, len(rows))
	for _, row := range rows {
		ip := net.ParseIP(strings.TrimSpace(row.IP))
		if ip == nil {
			continue
		}
		if recordType == cloudflareRecordTypeAll {
			filtered = append(filtered, row)
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

func uploadCloudflareRoutingRulesFromAny(value any) []UploadCloudflareRoutingRule {
	items := make([]any, 0)
	switch typed := value.(type) {
	case []any:
		items = typed
	case []map[string]any:
		for _, item := range typed {
			items = append(items, item)
		}
	default:
		return nil
	}
	rules := make([]UploadCloudflareRoutingRule, 0, len(items))
	for _, item := range items {
		raw := mapValue(item)
		if len(raw) == 0 {
			continue
		}
		rules = append(rules, UploadCloudflareRoutingRule{
			Enabled:      boolValue(raw["enabled"], true),
			FilterMode:   normalizeUploadRouteFilterMode(stringValue(firstNonNil(raw["filter_mode"], raw["filterMode"]), "allow")),
			FilterTokens: strings.TrimSpace(stringValue(firstNonNil(raw["filter_tokens"], raw["filterTokens"]), "")),
			Name:         strings.TrimSpace(stringValue(raw["name"], "")),
			RecordName:   strings.TrimSpace(stringValue(firstNonNil(raw["record_name"], raw["recordName"]), "")),
			RecordType:   normalizeCloudflareRecordType(stringValue(firstNonNil(raw["record_type"], raw["recordType"]), cloudflareRecordTypeA)),
			TopN:         max(0, intValue(firstNonNil(raw["top_n"], raw["topN"]), 0)),
		})
	}
	return rules
}

func filterUploadRowsForCloudflareRoute(rows []probecore.ProbeRow, rule UploadCloudflareRoutingRule, paths colodict.Paths) ([]probecore.ProbeRow, []string) {
	rawTokens := strings.TrimSpace(rule.FilterTokens)
	if rawTokens == "" {
		return cloneProbeRows(rows), nil
	}
	colos, unmatched, err := colodict.ResolveTokensToColos(paths, rawTokens)
	if err != nil {
		return nil, []string{uploadRouteWarning(rule, err.Error())}
	}
	warnings := make([]string, 0)
	if len(unmatched) > 0 {
		warnings = append(warnings, uploadRouteWarning(rule, "国家/COLO 筛选词未匹配："+strings.Join(unmatched, ", ")))
	}
	if len(colos) == 0 {
		return nil, warnings
	}
	entries, err := colodict.LoadColoEntries(paths.Colo)
	if err != nil {
		return nil, []string{uploadRouteWarning(rule, err.Error())}
	}
	mode := normalizeUploadRouteFilterMode(rule.FilterMode)
	filtered := make([]probecore.ProbeRow, 0, len(rows))
	for _, row := range rows {
		matched := uploadRouteRowMatchesColos(row, entries, colos)
		if (mode == "deny" && !matched) || (mode == "allow" && matched) {
			filtered = append(filtered, row)
		}
	}
	return filtered, warnings
}

func uploadRouteRowMatchesColos(row probecore.ProbeRow, entries []colodict.ColoEntry, colos map[string]struct{}) bool {
	for _, colo := range uploadRouteRowColos(row, entries) {
		if _, matched := colos[colo]; matched {
			return true
		}
	}
	return false
}

func uploadRouteRowColos(row probecore.ProbeRow, entries []colodict.ColoEntry) []string {
	result := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)
	add := func(raw string) {
		colo := strings.ToUpper(strings.TrimSpace(raw))
		if colo == "" || colo == "N/A" {
			return
		}
		if _, exists := seen[colo]; exists {
			return
		}
		seen[colo] = struct{}{}
		result = append(result, colo)
	}
	colo := strings.ToUpper(strings.TrimSpace(row.Colo))
	add(colo)
	add(colodict.LookupColo(entries, row.IP))
	return result
}

func uploadRouteRowColo(row probecore.ProbeRow, entries []colodict.ColoEntry) string {
	colos := uploadRouteRowColos(row, entries)
	if len(colos) == 0 {
		return ""
	}
	return colos[0]
}

func uploadRouteWarning(rule UploadCloudflareRoutingRule, message string) string {
	label := strings.TrimSpace(rule.Name)
	if label == "" {
		label = strings.TrimSpace(rule.RecordName)
	}
	if label == "" {
		label = "未命名规则"
	}
	return "Cloudflare 分流规则「" + label + "」：" + message
}

func normalizeUploadRouteFilterMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "deny", "blacklist", "exclude":
		return "deny"
	default:
		return "allow"
	}
}

func normalizeCloudflareRecordType(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case cloudflareRecordTypeAll:
		return cloudflareRecordTypeAll
	case cloudflareRecordTypeAAAA:
		return cloudflareRecordTypeAAAA
	default:
		return cloudflareRecordTypeA
	}
}

func uploadSelectionConfigFromSnapshot(snapshot map[string]any) UploadSelectionConfig {
	upload := mapValue(snapshot["upload"])
	shared := mapValue(upload["shared_filter"])
	legacyCloudflare := mapValue(upload["cloudflare"])
	legacyGithub := mapValue(upload["github"])
	cloudflare := mapValue(snapshot["cloudflare"])
	github := mapValue(snapshot["github"])

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
		CloudflareTopN: max(0, intValue(firstNonNil(cloudflare["top_n"], cloudflare["topN"], legacyCloudflare["top_n"], legacyCloudflare["topN"]), 0)),
		GitHubTopN:     max(0, intValue(firstNonNil(github["top_n"], github["topN"], legacyGithub["top_n"], legacyGithub["topN"]), 0)),
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
