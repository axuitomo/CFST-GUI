package appcore

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	mcisengine "github.com/axuitomo/CFST-GUI/internal/mcis/engine"
	mcisprobe "github.com/axuitomo/CFST-GUI/internal/mcis/probe"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
)

func RunMCISSearch(tokens []string, source Source, cfg probecore.ProbeConfig, limit int) ([]string, []string, error) {
	candidates, warnings, err := RunMCISSearchCandidates(tokens, source, cfg, limit)
	return mcisCandidateIPs(candidates), warnings, err
}

func RunMCISSearchContext(ctx context.Context, tokens []string, source Source, cfg probecore.ProbeConfig, limit int) ([]string, []string, error) {
	candidates, warnings, err := RunMCISSearchCandidatesContext(ctx, tokens, source, cfg, limit)
	return mcisCandidateIPs(candidates), warnings, err
}

func RunMCISSearchContextWithProgress(ctx context.Context, tokens []string, source Source, cfg probecore.ProbeConfig, limit int, onProgress func(mcisengine.Progress)) ([]string, []string, error) {
	candidates, warnings, err := RunMCISSearchCandidatesContextWithProgress(ctx, tokens, source, cfg, limit, onProgress)
	return mcisCandidateIPs(candidates), warnings, err
}

func RunMCISSearchCandidates(tokens []string, source Source, cfg probecore.ProbeConfig, limit int) ([]probecore.MCISCandidate, []string, error) {
	return RunMCISSearchCandidatesContext(context.Background(), tokens, source, cfg, limit)
}

func RunMCISSearchCandidatesContext(ctx context.Context, tokens []string, source Source, cfg probecore.ProbeConfig, limit int) ([]probecore.MCISCandidate, []string, error) {
	return RunMCISSearchCandidatesContextWithProgress(ctx, tokens, source, cfg, limit, nil)
}

func RunMCISSearchCandidatesContextWithProgress(ctx context.Context, tokens []string, source Source, cfg probecore.ProbeConfig, limit int, onProgress func(mcisengine.Progress)) ([]probecore.MCISCandidate, []string, error) {
	if limit <= 0 {
		return nil, nil, nil
	}
	cidrs := normalizeMCISTokens(tokens)
	if len(cidrs) == 0 {
		return nil, nil, errors.New("MICS抽样没有可用的 CIDR/IP 输入")
	}

	mcisCfg := BuildMCISEngineConfigForCIDRs(cfg, limit, cidrs)
	mcisCfg.OnProgress = onProgress
	probeCfg, warnings := BuildMCISProbeConfig(cfg)
	engine := mcisengine.New(mcisCfg, probeCfg)

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	response, err := engine.Run(ctx, mcisengine.Request{
		CIDRs: cidrs,
		Probe: probeCfg,
	})
	if err != nil {
		return nil, warnings, err
	}

	candidates := make([]probecore.MCISCandidate, 0, minInt(limit, len(response.Top)))
	for _, item := range response.Top {
		colo := ""
		if item.Trace != nil {
			colo = item.Trace["colo"]
		}
		candidates = append(candidates, probecore.MCISCandidate{
			IP: item.IP.String(), Prefix: item.Prefix.String(), Colo: colo,
			ConnectMS: item.ConnectMS, TLSMS: item.TLSMS, TTFBMS: item.TTFBMS, TotalMS: item.TotalMS,
			PrefixSamples: item.PrefixSamples, PrefixOK: item.PrefixOK, PrefixFail: item.PrefixFail,
		})
		if len(candidates) >= limit {
			break
		}
	}

	warnings = append(warnings, fmt.Sprintf("输入源 %s 的 MICS抽样结果将复用已有延迟与 COLO，跳过重复 TCP 测速。", SourceName(source)))
	return candidates, probecore.DedupeStrings(warnings), nil
}

func BuildMCISEngineConfig(cfg probecore.ProbeConfig, limit int) mcisengine.Config {
	return BuildMCISEngineConfigForCIDRs(cfg, limit, nil)
}

func BuildMCISEngineConfigForCIDRs(cfg probecore.ProbeConfig, limit int, cidrs []string) mcisengine.Config {
	mcisCfg := mcisengine.DefaultConfig()
	mcisCfg.TopN = limit
	mcisCfg.Budget = mcisSamplingBudget(limit, cidrs)
	mcisCfg.Concurrency = safeMCISConcurrency(cfg.Routines, runtime.GOOS, runtime.GOARCH)
	mcisCfg.Heads = clampInt(maxInt(limit/256, 4), 4, 8)
	mcisCfg.Beam = clampInt(maxInt(limit/64, 24), 24, 48)
	colos := task.ParseColoAllowList(cfg.HttpingCFColo)
	if task.NormalizeColoFilterMode(cfg.HttpingCFColoMode) == task.ColoFilterModeDeny {
		mcisCfg.ColoBlock = colos
	} else {
		mcisCfg.ColoAllow = colos
	}
	mcisCfg.Verbose = false
	return mcisCfg
}

const mcisSamplingBudgetCap = 8192

// mcisSamplingBudget computes the MICS sampling budget.
//
// The budget no longer has the fixed 256 floor: it is the ratio limit*3
// (capped at mcisSamplingBudgetCap). When the input CIDR/IP list contains
// fewer deduplicated candidate addresses than the ratio, the budget is
// lowered to that unique candidate count so small inputs are not re-probed
// just to fill a fixed budget.
func mcisSamplingBudget(limit int, cidrs []string) int {
	base := clampInt(limit*3, limit, mcisSamplingBudgetCap)
	capacity := uniqueMCISCandidateCapacity(cidrs, base)
	if capacity > 0 && capacity < base {
		return capacity
	}
	return base
}

// uniqueMCISCandidateCapacity counts the number of deduplicated candidate
// addresses across all input CIDR/IP tokens, stopping as soon as stopAt
// unique addresses are found. Duplicate IPs and overlapping CIDRs are
// counted once. Huge ranges (whose address count alone exceeds stopAt or
// whose host bits overflow the shift) bail out immediately with stopAt+1 so
// no large subnet is ever enumerated beyond the budget cap.
func uniqueMCISCandidateCapacity(cidrs []string, stopAt int) int {
	if len(cidrs) == 0 || stopAt <= 0 {
		return 0
	}
	seen := make(map[netip.Prefix]struct{}, stopAt)
	for _, token := range cidrs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(token))
		if err != nil {
			continue
		}
		prefix = prefix.Masked()
		bits := 32 - prefix.Bits()
		if prefix.Addr().Is6() {
			bits = 64 - minInt(prefix.Bits(), 64)
		}
		if bits >= 63 || (bits >= 0 && (uint64(1)<<uint(bits)) > uint64(stopAt)) {
			return stopAt + 1
		}
		count := 1 << uint(bits)
		addr := prefix.Addr()
		for offset := 0; offset < count; offset++ {
			seen[mcisNetworkKey(addr)] = struct{}{}
			if len(seen) >= stopAt {
				return len(seen)
			}
			addr = nextMCISNetwork(addr)
		}
	}
	return len(seen)
}

func mcisNetworkKey(addr netip.Addr) netip.Prefix {
	bits := 32
	if addr.Is6() {
		bits = 64
	}
	return netip.PrefixFrom(addr, bits).Masked()
}

func nextMCISNetwork(addr netip.Addr) netip.Addr {
	if addr.Is4() {
		return addr.Next()
	}
	value := addr.As16()
	for i := 7; i >= 0; i-- {
		value[i]++
		if value[i] != 0 {
			break
		}
	}
	return netip.AddrFrom16(value)
}

// safeMCISConcurrency returns the MICS probe concurrency: half of the TCP
// concurrent thread count, floored at 8 so the search engine stays usable on
// low-concurrency configs, and capped by a per-platform safety limit.
func safeMCISConcurrency(routines int, goos string, goarch string) int {
	maxConcurrency := 64
	if goos == "android" {
		maxConcurrency = 16
	} else if goos == "linux" && (goarch == "arm" || goarch == "arm64") {
		maxConcurrency = 32
	}
	return clampInt(routines/2, 8, maxConcurrency)
}

func BuildMCISProbeConfig(cfg probecore.ProbeConfig) (mcisprobe.Config, []string) {
	probeCfg := mcisprobe.Config{
		Path:               "/cdn-cgi/trace",
		Rounds:             maxInt(cfg.PingTimes+1, 4),
		SkipFirst:          1,
		Timeout:            time.Duration(clampInt(cfg.MaxDelayMS, 1000, 3000)) * time.Millisecond,
		UserAgent:          strings.TrimSpace(cfg.UserAgent),
		InsecureSkipVerify: !cfg.VerifyTLSCertificate,
	}
	warnings := make([]string, 0, 1)
	if captureAddress := effectiveDebugCaptureAddress(cfg); captureAddress != "" {
		probeCfg.DialAddress = captureAddress
	}

	targetURL := strings.TrimSpace(cfg.TraceURL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(cfg.URL)
	}
	if targetURL == "" {
		defaults := probecore.DefaultProbeConfig()
		targetURL = strings.TrimSpace(defaults.TraceURL)
		if targetURL == "" {
			targetURL = strings.TrimSpace(defaults.URL)
		}
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

func mcisCandidateIPs(candidates []probecore.MCISCandidate) []string {
	if candidates == nil {
		return nil
	}
	ips := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if ip := strings.TrimSpace(candidate.IP); ip != "" {
			ips = append(ips, ip)
		}
	}
	return ips
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
