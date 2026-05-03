package mobileapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/internal/colodict"
	"github.com/XIU2/CloudflareSpeedTest/internal/httpcfg"
	mcisengine "github.com/XIU2/CloudflareSpeedTest/internal/mcis/engine"
	mcisprobe "github.com/XIU2/CloudflareSpeedTest/internal/mcis/probe"
)

type sourceProcessResult struct {
	Entries      []string
	InvalidCount int
	Status       desktopSourceStatus
	Warnings     []string
}

func (s *Service) persistSourceStatuses(statuses []desktopSourceStatus) error {
	if len(statuses) == 0 {
		return nil
	}
	raw, err := os.ReadFile(s.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		return err
	}
	snapshot := mapValue(saved["config_snapshot"])
	if len(snapshot) == 0 {
		snapshot = saved
	}
	sourceItems, ok := snapshot["sources"].([]any)
	if !ok {
		return nil
	}
	statusMap := make(map[string]desktopSourceStatus, len(statuses))
	for _, status := range statuses {
		if id := strings.TrimSpace(status.ID); id != "" {
			statusMap[id] = status
		}
	}
	if len(statusMap) == 0 {
		return nil
	}
	for index, item := range sourceItems {
		sourceMap := mapValue(item)
		id := strings.TrimSpace(stringValue(sourceMap["id"], ""))
		status, exists := statusMap[id]
		if !exists {
			continue
		}
		sourceMap["last_fetched_at"] = status.LastFetchedAt
		sourceMap["last_fetched_count"] = status.LastFetchedCount
		sourceMap["status_text"] = status.StatusText
		sourceItems[index] = sourceMap
	}
	snapshot["sources"] = sourceItems
	body := map[string]any{
		"config_snapshot": snapshot,
		"saved_at":        nowRFC3339(),
		"schema_version":  schemaVersion,
	}
	encoded, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath(), encoded, 0o600)
}

type preparedSources struct {
	Text           string
	InvalidCount   int
	SourceStatuses []desktopSourceStatus
	Warnings       []string
}

func (s *Service) PreviewSource(payloadJSON string) string {
	return s.inspectSource(payloadJSON, false)
}

func (s *Service) FetchSource(payloadJSON string) string {
	return s.inspectSource(payloadJSON, true)
}

func (s *Service) inspectSource(payloadJSON string, persist bool) string {
	var payload sourcePreviewPayload
	if err := decodeInto(payloadJSON, &payload); err != nil {
		return encodeCommand(commandResultFor("SOURCE_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	if !hasSourceInput(payload.Source) {
		return encodeCommand(commandResultFor("SOURCE_INPUT_EMPTY", nil, "输入源缺少可读取的内容。", false, nil, nil))
	}
	cfg, _ := configToProbeConfig(payload.Config)
	result, err := s.processSource(cfg, payload.Source, newSourceHTTPClient(cfg), time.Now())
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_READ_FAILED", nil, err.Error(), false, nil, result.Warnings))
	}
	if persist || payload.PersistState {
		if err := s.persistSourceStatuses([]desktopSourceStatus{result.Status}); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("更新输入源状态失败：%v", err))
		}
	}
	previewLimit := payload.PreviewLimit
	if previewLimit <= 0 {
		previewLimit = 16
	}
	previewEntries := result.Entries
	if len(previewEntries) > previewLimit {
		previewEntries = previewEntries[:previewLimit]
	}
	actionLabel := "预览"
	if persist || payload.PersistState {
		actionLabel = "抓取"
	}
	return encodeCommand(commandResultFor("SOURCE_PREVIEW_READY", map[string]any{
		"preview_entries": previewEntries,
		"source_status":   result.Status,
		"summary": map[string]any{
			"action":        actionLabel,
			"invalid_count": result.InvalidCount,
			"mode":          sourceIPMode(payload.Source),
			"name":          sourceName(payload.Source),
			"total_count":   len(result.Entries),
		},
	}, fmt.Sprintf("%s已完成，可预览 %d 条候选。", actionLabel, len(previewEntries)), true, nil, result.Warnings))
}

func hasSourceInput(source desktopSource) bool {
	switch sourceKind(source) {
	case "inline":
		return strings.TrimSpace(source.Content) != ""
	case "file":
		return strings.TrimSpace(source.Path) != ""
	default:
		return strings.TrimSpace(source.URL) != ""
	}
}

func (s *Service) processSource(cfg probeConfig, source desktopSource, client *http.Client, now time.Time) (sourceProcessResult, error) {
	status := desktopSourceStatus{
		ID:               strings.TrimSpace(source.ID),
		LastFetchedAt:    strings.TrimSpace(source.LastFetchedAt),
		LastFetchedCount: source.LastFetchedCount,
		StatusText:       strings.TrimSpace(source.StatusText),
	}
	raw, err := loadSourceContent(source, cfg, client)
	if err != nil {
		status.LastFetchedAt = now.Format(time.RFC3339)
		status.LastFetchedCount = 0
		status.StatusText = fmt.Sprintf("最近读取失败 · %s", err.Error())
		return sourceProcessResult{Status: status}, err
	}
	entries, warnings, invalidCount, err := s.buildSourceEntriesWithConfig(raw, source, cfg)
	if err != nil {
		status.LastFetchedAt = now.Format(time.RFC3339)
		status.LastFetchedCount = 0
		status.StatusText = fmt.Sprintf("最近读取失败 · %s", err.Error())
		return sourceProcessResult{InvalidCount: invalidCount, Status: status, Warnings: warnings}, err
	}
	action := "载入"
	if sourceKind(source) == "url" {
		action = "抓取"
	}
	status.LastFetchedAt = now.Format(time.RFC3339)
	status.LastFetchedCount = len(entries)
	if len(entries) > 0 {
		status.StatusText = fmt.Sprintf("最近%s成功 · %s · %d 条", action, now.Format("2006/1/2 15:04:05"), len(entries))
	} else {
		status.StatusText = fmt.Sprintf("最近%s完成 · %s · 0 条", action, now.Format("2006/1/2 15:04:05"))
	}
	return sourceProcessResult{Entries: entries, InvalidCount: invalidCount, Status: status, Warnings: warnings}, nil
}

func (s *Service) buildSourceEntriesWithConfig(raw string, source desktopSource, cfg probeConfig) ([]string, []string, int, error) {
	limit := sourceIPLimit(source)
	mode := sourceIPMode(source)
	name := sourceName(source)
	normalizedTokens := make([]string, 0, limit)
	invalidCount := 0
	for _, token := range sourceTokens(raw) {
		normalized, ok := normalizeIPToken(token)
		if !ok {
			invalidCount++
			continue
		}
		normalizedTokens = append(normalizedTokens, normalized)
	}
	warnings := make([]string, 0)
	if invalidCount > 0 {
		warnings = append(warnings, fmt.Sprintf("输入源 %s 忽略了 %d 条无效 IP/CIDR。", name, invalidCount))
	}
	if len(normalizedTokens) == 0 {
		return nil, warnings, invalidCount, nil
	}
	coloFilter, err := colodict.NewFilter(s.coloDictionaryPaths().Colo, source.ColoFilter)
	if err != nil {
		return nil, warnings, invalidCount, err
	}
	if coloFilter != nil {
		filteredTokens := make([]string, 0, len(normalizedTokens))
		for _, token := range normalizedTokens {
			filteredTokens = append(filteredTokens, coloFilter.FilterToken(token)...)
		}
		if len(filteredTokens) == 0 {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 的 COLO 筛选没有匹配候选。", name))
			return nil, dedupeStrings(warnings), invalidCount, nil
		}
		normalizedTokens = filteredTokens
		warnings = append(warnings, fmt.Sprintf("输入源 %s 已按 COLO 白名单 %s 预筛候选。", name, strings.TrimSpace(source.ColoFilter)))
	}
	if mode == "mcis" {
		entries, mcisWarnings, err := mobileMCISSearchRunner(normalizedTokens, source, cfg, limit)
		warnings = append(warnings, mcisWarnings...)
		if err != nil {
			return nil, warnings, invalidCount, err
		}
		if len(entries) >= limit {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 达到 IP 上限 %d，已截断候选列表。", name, limit))
		}
		return entries, dedupeStrings(warnings), invalidCount, nil
	}

	entries := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit)
	truncated := false
	for _, token := range normalizedTokens {
		if len(entries) >= limit {
			truncated = true
			break
		}
		expanded, tokenTruncated := expandTraverseToken(token, limit-len(entries))
		if tokenTruncated {
			truncated = true
		}
		for _, entry := range expanded {
			if _, exists := seen[entry]; exists {
				continue
			}
			seen[entry] = struct{}{}
			entries = append(entries, entry)
			if len(entries) >= limit {
				truncated = true
				break
			}
		}
	}
	if truncated {
		warnings = append(warnings, fmt.Sprintf("输入源 %s 达到 IP 上限 %d，已截断候选列表。", name, limit))
	}
	return entries, dedupeStrings(warnings), invalidCount, nil
}

func expandTraverseToken(token string, limit int) ([]string, bool) {
	if limit <= 0 {
		return nil, true
	}
	if !strings.Contains(token, "/") {
		return []string{token}, false
	}
	_, ipNet, err := net.ParseCIDR(token)
	if err != nil {
		return nil, false
	}
	return enumerateCIDRIPs(ipNet, limit)
}

var mobileMCISSearchRunner = runMCISSearch

func runMCISSearch(tokens []string, source desktopSource, cfg probeConfig, limit int) ([]string, []string, error) {
	if limit <= 0 {
		return nil, nil, nil
	}
	cidrs := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if strings.Contains(token, "/") {
			cidrs = append(cidrs, token)
			continue
		}
		addr, err := netip.ParseAddr(token)
		if err != nil {
			continue
		}
		if addr.Is4() {
			cidrs = append(cidrs, addr.String()+"/32")
		} else {
			cidrs = append(cidrs, addr.String()+"/128")
		}
	}
	if len(cidrs) == 0 {
		return nil, nil, errors.New("MICS抽样没有可用的 CIDR/IP 输入")
	}
	mcisCfg := buildMCISEngineConfig(cfg, limit)

	probeCfg, warnings := buildMCISProbeConfig(cfg)
	engine := mcisengine.New(mcisCfg, probeCfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	response, err := engine.Run(ctx, mcisengine.Request{CIDRs: cidrs, Probe: probeCfg})
	if err != nil {
		return nil, warnings, err
	}
	entries := make([]string, 0, minInt(limit, len(response.Top)))
	seen := make(map[string]struct{}, limit)
	for _, item := range response.Top {
		ip := item.IP.String()
		if _, exists := seen[ip]; exists {
			continue
		}
		seen[ip] = struct{}{}
		entries = append(entries, ip)
		if len(entries) >= limit {
			break
		}
	}
	warnings = append(warnings, fmt.Sprintf("输入源 %s 的 MICS抽样模式已先通过独立搜索引擎筛选候选，再交由当前 CFST 流程做最终测速。", sourceName(source)))
	return entries, dedupeStrings(warnings), nil
}

func buildMCISEngineConfig(cfg probeConfig, limit int) mcisengine.Config {
	mcisCfg := mcisengine.DefaultConfig()
	mcisCfg.TopN = limit
	mcisCfg.Budget = clampInt(maxInt(limit*3, 256), limit, 8192)
	mcisCfg.Concurrency = clampInt(maxInt(cfg.Routines/2, 32), 16, 128)
	mcisCfg.Heads = clampInt(maxInt(limit/256, 4), 4, 8)
	mcisCfg.Beam = clampInt(maxInt(limit/64, 24), 24, 48)
	mcisCfg.ColoAllow = nil
	mcisCfg.Verbose = false
	return mcisCfg
}

func buildMCISProbeConfig(cfg probeConfig) (mcisprobe.Config, []string) {
	probeCfg := mcisprobe.Config{
		Path:               "/cdn-cgi/trace",
		Rounds:             maxInt(cfg.PingTimes+1, 4),
		SkipFirst:          1,
		Timeout:            time.Duration(clampInt(cfg.MaxDelayMS, 1000, 3000)) * time.Millisecond,
		UserAgent:          strings.TrimSpace(cfg.UserAgent),
		InsecureSkipVerify: true,
	}
	warnings := make([]string, 0, 1)
	if cfg.Debug {
		probeCfg.DialAddress = httpcfg.Resolve("", "", "", cfg.DebugCaptureAddress, true).CaptureAddress
	}
	targetURL := strings.TrimSpace(cfg.URL)
	if targetURL == "" {
		targetURL = defaultProbeConfig().URL
	}
	if parsed, err := url.Parse(targetURL); err == nil {
		host := strings.TrimSpace(parsed.Hostname())
		if hostHeader := strings.TrimSpace(cfg.HostHeader); hostHeader != "" {
			probeCfg.HostHeader = hostHeader
		} else if host != "" {
			probeCfg.HostHeader = host
		}
		if sni := strings.TrimSpace(cfg.SNI); sni != "" {
			probeCfg.SNI = sni
		} else if probeCfg.HostHeader != "" {
			probeCfg.SNI = probeCfg.HostHeader
		}
		if path := strings.TrimSpace(parsed.EscapedPath()); path == "/cdn-cgi/trace" {
			probeCfg.Path = path
		}
	}
	if probeCfg.SNI == "" {
		probeCfg.SNI = "cf.xiu2.xyz"
		probeCfg.HostHeader = probeCfg.SNI
		warnings = append(warnings, "MICS抽样未能从测速 URL 解析 Host，已回退到默认 Host。")
	}
	return probeCfg, warnings
}

func newSourceHTTPClient(cfg probeConfig) *http.Client {
	profile := httpcfg.Resolve(cfg.UserAgent, "", "", "", true)
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: profile.InsecureSkipVerify},
		},
	}
}

func (s *Service) prepareSources(cfg probeConfig, sources []desktopSource) preparedSources {
	client := newSourceHTTPClient(cfg)
	now := time.Now()
	parts := make([]string, 0)
	statuses := make([]desktopSourceStatus, 0, len(sources))
	warnings := make([]string, 0)
	invalidCount := 0
	for index, source := range sources {
		name := sourceName(source)
		if name == "" {
			name = fmt.Sprintf("输入源 %d", index+1)
		}
		status := desktopSourceStatus{
			ID:               strings.TrimSpace(source.ID),
			LastFetchedAt:    strings.TrimSpace(source.LastFetchedAt),
			LastFetchedCount: source.LastFetchedCount,
			StatusText:       strings.TrimSpace(source.StatusText),
		}
		if !sourceEnabled(source) {
			if status.StatusText == "" {
				status.StatusText = "已停用，启动任务时不会读取该输入源。"
			}
			statuses = append(statuses, status)
			continue
		}
		result, err := s.processSource(cfg, source, client, now)
		if err != nil {
			statuses = append(statuses, result.Status)
			invalidCount += result.InvalidCount
			warnings = append(warnings, fmt.Sprintf("输入源 %s 读取失败：%v", name, err))
			warnings = append(warnings, result.Warnings...)
			continue
		}
		warnings = append(warnings, result.Warnings...)
		invalidCount += result.InvalidCount
		if len(result.Entries) > 0 {
			parts = append(parts, strings.Join(result.Entries, "\n"))
		}
		statuses = append(statuses, result.Status)
	}
	return preparedSources{
		Text:           strings.Join(parts, "\n"),
		InvalidCount:   invalidCount,
		SourceStatuses: statuses,
		Warnings:       dedupeStrings(warnings),
	}
}

func loadSourceContent(source desktopSource, cfg probeConfig, client *http.Client) (string, error) {
	switch sourceKind(source) {
	case "inline":
		return strings.TrimSpace(source.Content), nil
	case "file":
		path := strings.TrimSpace(source.Path)
		if path == "" {
			return "", errors.New("缺少文件路径")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	default:
		sourceURL := strings.TrimSpace(source.URL)
		if sourceURL == "" {
			return "", errors.New("缺少远程 URL")
		}
		req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
		if err != nil {
			return "", err
		}
		httpcfg.Resolve(cfg.UserAgent, "", "", "", true).Apply(req)
		res, err := client.Do(req)
		if err != nil {
			return "", err
		}
		raw, readErr := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if readErr != nil {
			return "", readErr
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return "", fmt.Errorf("远程来源返回状态 %s", res.Status)
		}
		return string(raw), nil
	}
}

func sourceName(source desktopSource) string {
	if name := strings.TrimSpace(source.Name); name != "" {
		return name
	}
	if label := strings.TrimSpace(source.Label); label != "" {
		return label
	}
	switch sourceKind(source) {
	case "file":
		return "本地文件来源"
	case "inline":
		return "手动输入来源"
	default:
		return "远程来源"
	}
}

func sourceKind(source desktopSource) string {
	switch strings.ToLower(strings.TrimSpace(source.Kind)) {
	case "inline", "file":
		return strings.ToLower(strings.TrimSpace(source.Kind))
	default:
		return "url"
	}
}

func sourceEnabled(source desktopSource) bool {
	if source.Enabled {
		return true
	}
	return source.ID == "" && source.Name == "" && source.IPLimit == 0 && source.IPMode == ""
}

func sourceIPLimit(source desktopSource) int {
	if source.IPLimit <= 0 {
		return defaultMobileSourceIPLimit
	}
	return source.IPLimit
}

func sourceIPMode(source desktopSource) string {
	if strings.EqualFold(strings.TrimSpace(source.IPMode), "mcis") {
		return "mcis"
	}
	return "traverse"
}

func enumerateCIDRIPs(ipNet *net.IPNet, limit int) ([]string, bool) {
	if limit <= 0 {
		return nil, true
	}
	_, bits := ipNet.Mask.Size()
	current := cloneIPForBits(ipNet.IP, bits)
	entries := make([]string, 0, limit)
	for len(entries) < limit && ipNet.Contains(current) {
		entries = append(entries, current.String())
		incrementIP(current)
	}
	return entries, ipNet.Contains(current)
}

func cloneIPForBits(ip net.IP, bits int) net.IP {
	if bits == 32 {
		return append(net.IP(nil), ip.To4()...)
	}
	return append(net.IP(nil), ip.To16()...)
}

func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			return
		}
	}
}
