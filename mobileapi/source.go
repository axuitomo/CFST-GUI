package mobileapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	mcisengine "github.com/axuitomo/CFST-GUI/internal/mcis/engine"
	mcisprobe "github.com/axuitomo/CFST-GUI/internal/mcis/probe"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/task"
)

type sourceProcessResult struct {
	Entries      []string
	InvalidCount int
	SourcePorts  map[string]int
	ColoFilter   string
	ColoMode     string
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
	snapshot = sanitizeMobileConfigSnapshot(snapshot)
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
	Text              string
	FatalErrors       []string
	InvalidCount      int
	SourcePorts       map[string]int
	SourceColoFilters task.SourceColoFilterMap
	SourceStatuses    []desktopSourceStatus
	Warnings          []string
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
		"port_summary":    probecore.PortSummary(result.Entries, result.SourcePorts, cfg.TCPPort),
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
	entries, sourcePorts, warnings, invalidCount, err := s.buildSourceEntriesWithConfig(raw, source, cfg)
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
	return sourceProcessResult{
		Entries:      entries,
		InvalidCount: invalidCount,
		SourcePorts:  sourcePorts,
		ColoFilter:   strings.TrimSpace(source.ColoFilter),
		ColoMode:     task.NormalizeColoFilterMode(source.ColoFilterMode),
		Status:       status,
		Warnings:     warnings,
	}, nil
}

func (s *Service) buildSourceEntriesWithConfig(raw string, source desktopSource, cfg probeConfig) ([]string, map[string]int, []string, int, error) {
	limit := sourceIPLimit(source)
	result, err := probecore.BuildSourceEntries(probecore.SourceBuildOptions{
		Raw:                   raw,
		Name:                  sourceName(source),
		Mode:                  sourceIPMode(source),
		Limit:                 limit,
		Resolver:              sourceParseResolver,
		ColoFilter:            source.ColoFilter,
		ColoMode:              source.ColoFilterMode,
		ColoDictionaryPaths:   s.coloDictionaryPaths(),
		SourceColoFilterPhase: cfg.SourceColoFilterPhase,
		MCISRunner: func(tokens []string, limit int) ([]string, []string, error) {
			return mobileMCISSearchRunner(tokens, source, cfg, limit)
		},
	})
	return result.Entries, result.SourcePorts, result.Warnings, result.InvalidCount, err
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
	if captureAddress := effectiveDebugCaptureAddress(cfg); captureAddress != "" {
		probeCfg.DialAddress = captureAddress
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
	return httpclient.NewClient(httpclient.Options{
		Profile:      profile,
		Timeout:      20 * time.Second,
		DisableProxy: true,
	})
}

func (s *Service) prepareSources(cfg probeConfig, sources []desktopSource) preparedSources {
	client := newSourceHTTPClient(cfg)
	now := time.Now()
	parts := make([]string, 0)
	statuses := make([]desktopSourceStatus, 0, len(sources))
	warnings := make([]string, 0)
	fatalErrors := make([]string, 0)
	invalidCount := 0
	sourcePorts := make(map[string]int)
	var sourceColoFilters task.SourceColoFilterMap
	if cfg.SourceColoFilterPhase == sourceColoFilterPhaseStage2 {
		sourceColoFilters = make(task.SourceColoFilterMap)
	}
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
			message := fmt.Sprintf("输入源 %s 读取失败：%v", name, err)
			warnings = append(warnings, message)
			if isMissingColoFileError(err) {
				fatalErrors = append(fatalErrors, message)
			}
			warnings = append(warnings, result.Warnings...)
			continue
		}
		warnings = append(warnings, result.Warnings...)
		invalidCount += result.InvalidCount
		if len(result.Entries) > 0 {
			parts = append(parts, strings.Join(result.Entries, "\n"))
			for token, port := range result.SourcePorts {
				sourcePorts[token] = port
			}
			if sourceColoFilters != nil {
				task.MergeSourceColoFiltersWithMode(sourceColoFilters, result.Entries, result.ColoFilter, result.ColoMode)
			}
		}
		statuses = append(statuses, result.Status)
	}
	return preparedSources{
		Text:              strings.Join(parts, "\n"),
		FatalErrors:       dedupeStrings(fatalErrors),
		InvalidCount:      invalidCount,
		SourcePorts:       probecore.CloneStringIntMap(sourcePorts),
		SourceColoFilters: sourceColoFilters,
		SourceStatuses:    statuses,
		Warnings:          dedupeStrings(warnings),
	}
}

func isMissingColoFileError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "COLO 文件不存在")
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
		sourceURL, err := normalizeMobileSourceURLInput(source.URL)
		if err != nil {
			return "", err
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

func normalizeMobileSourceURLInput(rawURL string) (string, error) {
	value := normalizeProbeURLInput(rawURL)
	if value == "" {
		return "", errors.New("缺少远程 URL")
	}
	if strings.HasPrefix(value, "//") {
		value = "https:" + value
	} else if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("远程 URL 必须包含有效主机")
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("远程 URL 仅支持 http/https：%s", parsed.Scheme)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	return parsed.String(), nil
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
