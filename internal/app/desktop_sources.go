package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	mcisengine "github.com/axuitomo/CFST-GUI/internal/mcis/engine"
	mcisprobe "github.com/axuitomo/CFST-GUI/internal/mcis/probe"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/task"
)

type DesktopSourcePreviewPayload struct {
	Config       map[string]any `json:"config"`
	PersistState bool           `json:"persist_state"`
	PreviewLimit int            `json:"preview_limit"`
	Source       DesktopSource  `json:"source"`
}

type desktopSourceProcessResult struct {
	Entries      []string
	InvalidCount int
	SourcePorts  map[string]int
	ColoFilter   string
	ColoMode     string
	Status       DesktopSourceStatus
	Warnings     []string
}

func (a *App) PreviewDesktopSource(payload DesktopSourcePreviewPayload) DesktopCommandResult {
	return a.inspectDesktopSource(payload, false)
}

func (a *App) FetchDesktopSource(payload DesktopSourcePreviewPayload) DesktopCommandResult {
	return a.inspectDesktopSource(payload, true)
}

func (a *App) inspectDesktopSource(payload DesktopSourcePreviewPayload, persist bool) DesktopCommandResult {
	source := payload.Source
	if !hasDesktopSourceInput(source) {
		return desktopCommandResult("SOURCE_INPUT_EMPTY", nil, "输入源缺少可读取的内容。", false, nil, nil)
	}

	cfg, _ := desktopConfigToProbeConfig(payload.Config)
	now := time.Now()
	result, err := processDesktopSource(cfg, source, newDesktopSourceHTTPClient(cfg), now)
	if err != nil {
		return desktopCommandResult("SOURCE_READ_FAILED", nil, err.Error(), false, nil, result.Warnings)
	}

	if persist || payload.PersistState {
		if err := persistDesktopSourceStatuses([]DesktopSourceStatus{result.Status}); err != nil {
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

	return desktopCommandResult("SOURCE_PREVIEW_READY", map[string]any{
		"preview_entries": previewEntries,
		"port_summary":    probecore.PortSummary(result.Entries, result.SourcePorts, cfg.TCPPort),
		"source_status":   result.Status,
		"summary": map[string]any{
			"action":        actionLabel,
			"invalid_count": result.InvalidCount,
			"mode":          desktopSourceIPMode(source),
			"name":          desktopSourceName(source),
			"total_count":   len(result.Entries),
		},
	}, fmt.Sprintf("%s已完成，可预览 %d 条候选。", actionLabel, len(previewEntries)), true, nil, dedupeStrings(result.Warnings))
}

func hasDesktopSourceInput(source DesktopSource) bool {
	switch desktopSourceKind(source) {
	case "inline":
		return strings.TrimSpace(source.Content) != ""
	case "file":
		return strings.TrimSpace(source.Path) != ""
	default:
		return strings.TrimSpace(source.URL) != ""
	}
}

func processDesktopSource(cfg ProbeConfig, source DesktopSource, client *http.Client, now time.Time) (desktopSourceProcessResult, error) {
	status := DesktopSourceStatus{
		ID:               strings.TrimSpace(source.ID),
		LastFetchedAt:    strings.TrimSpace(source.LastFetchedAt),
		LastFetchedCount: source.LastFetchedCount,
		StatusText:       strings.TrimSpace(source.StatusText),
	}

	content, err := loadDesktopSourceContent(source, cfg, client)
	if err != nil {
		status.LastFetchedAt = now.Format(time.RFC3339)
		status.LastFetchedCount = 0
		status.StatusText = fmt.Sprintf("最近读取失败 · %s", err.Error())
		return desktopSourceProcessResult{Status: status}, err
	}

	entries, sourcePorts, warnings, invalidCount, err := buildDesktopSourceEntriesWithConfig(content.Raw, source, cfg)
	warnings = append(content.Warnings, warnings...)
	if err != nil {
		status.LastFetchedAt = now.Format(time.RFC3339)
		status.LastFetchedCount = 0
		status.StatusText = fmt.Sprintf("最近读取失败 · %s", err.Error())
		return desktopSourceProcessResult{
			InvalidCount: invalidCount,
			Status:       status,
			Warnings:     warnings,
		}, err
	}

	action := "载入"
	if desktopSourceKind(source) == "url" {
		action = "抓取"
	}
	status.LastFetchedAt = now.Format(time.RFC3339)
	status.LastFetchedCount = len(entries)
	if len(entries) > 0 {
		status.StatusText = fmt.Sprintf("最近%s成功 · %s · %d 条", action, now.Format("2006/1/2 15:04:05"), len(entries))
	} else {
		status.StatusText = fmt.Sprintf("最近%s完成 · %s · 0 条", action, now.Format("2006/1/2 15:04:05"))
	}

	return desktopSourceProcessResult{
		Entries:      entries,
		InvalidCount: invalidCount,
		SourcePorts:  sourcePorts,
		ColoFilter:   strings.TrimSpace(source.ColoFilter),
		ColoMode:     task.NormalizeColoFilterMode(source.ColoFilterMode),
		Status:       status,
		Warnings:     warnings,
	}, nil
}

func buildDesktopSourceEntriesWithConfig(raw string, source DesktopSource, cfg ProbeConfig) ([]string, map[string]int, []string, int, error) {
	limit := desktopSourceIPLimit(source)
	result, err := probecore.BuildSourceEntries(probecore.SourceBuildOptions{
		Raw:                   raw,
		Name:                  desktopSourceName(source),
		Mode:                  desktopSourceIPMode(source),
		Limit:                 limit,
		Resolver:              sourceParseResolver,
		ColoFilter:            source.ColoFilter,
		ColoMode:              source.ColoFilterMode,
		ColoDictionaryPaths:   desktopColoDictionaryPaths(),
		SourceColoFilterPhase: cfg.SourceColoFilterPhase,
		MCISRunner: func(tokens []string, limit int) ([]string, []string, error) {
			return desktopMCISSearchRunner(tokens, source, cfg, limit)
		},
	})
	return result.Entries, result.SourcePorts, result.Warnings, result.InvalidCount, err
}

var desktopMCISSearchRunner = runDesktopMCISSearch

func runDesktopMCISSearch(tokens []string, source DesktopSource, cfg ProbeConfig, limit int) ([]string, []string, error) {
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

	mcisCfg := buildDesktopMCISEngineConfig(cfg, limit)

	probeCfg, warnings := buildDesktopMCISProbeConfig(cfg)
	engine := mcisengine.New(mcisCfg, probeCfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	response, err := engine.Run(ctx, mcisengine.Request{
		CIDRs: cidrs,
		Probe: probeCfg,
	})
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

	name := desktopSourceName(source)
	warnings = append(warnings, fmt.Sprintf("输入源 %s 的 MICS抽样模式已先通过独立搜索引擎筛选候选，再交由当前 CFST 流程做最终测速。", name))
	return entries, dedupeStrings(warnings), nil
}

func buildDesktopMCISEngineConfig(cfg ProbeConfig, limit int) mcisengine.Config {
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

func buildDesktopMCISProbeConfig(cfg ProbeConfig) (mcisprobe.Config, []string) {
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

func newDesktopSourceHTTPClient(cfg ProbeConfig) *http.Client {
	profile := httpcfg.Resolve(cfg.UserAgent, "", "", "", true)
	return httpclient.NewClient(httpclient.Options{
		Profile: profile,
		Timeout: 30 * time.Second,
	})
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
