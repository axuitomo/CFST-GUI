package appcore

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	mcisengine "github.com/axuitomo/CFST-GUI/internal/mcis/engine"
	mcisprobe "github.com/axuitomo/CFST-GUI/internal/mcis/probe"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func RunMCISSearch(tokens []string, source Source, cfg probecore.ProbeConfig, limit int) ([]string, []string, error) {
	if limit <= 0 {
		return nil, nil, nil
	}
	cidrs := normalizeMCISTokens(tokens)
	if len(cidrs) == 0 {
		return nil, nil, errors.New("MICS抽样没有可用的 CIDR/IP 输入")
	}

	mcisCfg := BuildMCISEngineConfig(cfg, limit)
	probeCfg, warnings := BuildMCISProbeConfig(cfg)
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

	warnings = append(warnings, fmt.Sprintf("输入源 %s 的 MICS抽样模式已先通过独立搜索引擎筛选候选，再交由当前 CFST 流程做最终测速。", SourceName(source)))
	return entries, probecore.DedupeStrings(warnings), nil
}

func BuildMCISEngineConfig(cfg probecore.ProbeConfig, limit int) mcisengine.Config {
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

func BuildMCISProbeConfig(cfg probecore.ProbeConfig) (mcisprobe.Config, []string) {
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
		targetURL = probecore.DefaultProbeConfig().URL
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

func normalizeMCISTokens(tokens []string) []string {
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
	return cidrs
}

func effectiveDebugCaptureAddress(cfg probecore.ProbeConfig) string {
	if !cfg.Debug || !cfg.DebugCaptureEnabled || strings.TrimSpace(cfg.DebugCaptureAddress) == "" {
		return ""
	}
	return httpcfg.Resolve("", "", "", cfg.DebugCaptureAddress, true).CaptureAddress
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
