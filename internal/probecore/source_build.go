package probecore

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/sourceparse"
	"github.com/axuitomo/CFST-GUI/internal/task"
)

const SourceColoFilterPhaseStage2 = "stage2"

const (
	// DefaultSourcePoolLimit 是单个输入源候选池的默认上限，可由 probe.source_pool_limit 覆盖。
	DefaultSourcePoolLimit = 65536
	// MaxSourcePoolLimit 是允许配置的绝对上限。IPv4/IPv6 CIDR 一律按地址展开，
	// 超过该值的配置会被收敛，避免误配把候选池撑到内存无法承受的量级。
	MaxSourcePoolLimit = 1000000
	// sourcePrefilterParseFactor 决定 COLO 预筛阶段的解析预算：预筛必须先展开再过滤，
	// 但解析本身也要受池上限约束，否则超大输入会把候选池阶段撑成无界内存。
	sourcePrefilterParseFactor = 8
)

type MCISCandidate struct {
	IP            string
	Prefix        string
	Colo          string
	ConnectMS     int64
	TLSMS         int64
	TTFBMS        int64
	TotalMS       int64
	PrefixSamples int
	PrefixOK      int
	PrefixFail    int
}

type MCISSourceRunner func(tokens []string, limit int) ([]MCISCandidate, []string, error)
type SourceBuildOptions struct {
	Context               context.Context
	Raw                   string
	Name                  string
	Mode                  string
	Limit                 int // 候选池容量；负值表示不限量，按 MaxPoolSize 执行
	MaxPoolSize           int // 候选池上限；<=0 用 DefaultSourcePoolLimit
	Resolver              sourceparse.Resolver
	ColoFilter            string
	ColoMode              string
	ColoDictionaryPaths   colodict.Paths
	SourceColoFilterPhase string
	MCISRunner            MCISSourceRunner
}

type SourceBuildResult struct {
	Entries          []string
	MCISCandidates   []MCISCandidate
	InvalidCount     int
	SourcePorts      map[string]int
	ColoFilterActive bool
	ColoFilterColos  []string
	Warnings         []string
}

func BuildSourceEntries(options SourceBuildOptions) (SourceBuildResult, error) {
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return SourceBuildResult{}, err
	}
	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = "输入源"
	}
	ceiling := options.MaxPoolSize
	if ceiling <= 0 {
		ceiling = DefaultSourcePoolLimit
	}
	if ceiling > MaxSourcePoolLimit {
		ceiling = MaxSourcePoolLimit
	}
	limit := options.Limit
	limitWarnings := []string(nil)
	if limit < 0 {
		// 负值表示"不限量"，在池物化边界收敛为用户可配置的池上限。
		limit = ceiling
	} else if limit > ceiling {
		limitWarnings = append(limitWarnings, fmt.Sprintf("输入源 %s 的候选上限 %d 超过池上限 %d，已按上限执行。", name, limit, ceiling))
		limit = ceiling
	}
	mode := strings.TrimSpace(options.Mode)
	if mode == "" {
		mode = "traverse"
	}
	parseLimit := limit
	prefilterParseBudget := 0
	sourceColoFilter := strings.TrimSpace(options.ColoFilter)
	sourceColoMode := task.NormalizeColoFilterMode(options.ColoMode)
	sourceColos := []string(nil)
	sourceColoActive := sourceColoFilter != ""
	sourceColoWarnings := []string(nil)
	if sourceColoFilter != "" {
		resolvedColos, unmatched, err := colodict.ResolveTokensToColos(options.ColoDictionaryPaths, sourceColoFilter)
		if err != nil {
			phase := "预检查阶段"
			if options.SourceColoFilterPhase == SourceColoFilterPhaseStage2 {
				phase = "第二阶段"
			}
			return SourceBuildResult{}, fmt.Errorf("输入源 %s 设置了 COLO 筛选（%s），需要先更新/处理 COLO 词典：%w", name, phase, err)
		}
		sourceColos = sortedStringKeys(resolvedColos)
		if len(unmatched) > 0 {
			slices.Sort(unmatched)
			phase := "预检查阶段"
			if options.SourceColoFilterPhase == SourceColoFilterPhaseStage2 {
				phase = "第二阶段"
			}
			sourceColoWarnings = append(sourceColoWarnings, fmt.Sprintf("输入源 %s 设置了 COLO 筛选（%s），但筛选词未匹配：%s。", name, phase, strings.Join(unmatched, ", ")))
		}
	}
	if sourceColoFilter != "" && options.SourceColoFilterPhase != SourceColoFilterPhaseStage2 {
		// 预筛要在展开前看到尽量多的候选，但解析不能无界：按池上限的倍数给预算，
		// 超出预算的候选会被截断并告警（可调大 probe.source_pool_limit）。
		prefilterParseBudget = ceiling * sourcePrefilterParseFactor
		parseLimit = prefilterParseBudget
	}

	parsed := sourceparse.Parse(options.Raw, sourceparse.Options{Context: ctx, Limit: parseLimit, Resolver: options.Resolver})
	if err := ctx.Err(); err != nil {
		return SourceBuildResult{}, err
	}
	normalizedTokens := append([]string(nil), parsed.Valid...)
	sourcePorts := CloneStringIntMap(parsed.Ports)
	invalidCount := len(parsed.Invalid)
	warnings := append([]string(nil), parsed.Warnings...)
	warnings = append(warnings, sourceColoWarnings...)
	warnings = append(warnings, limitWarnings...)
	if invalidCount > 0 {
		warnings = append(warnings, fmt.Sprintf("输入源 %s 忽略了 %d 条无效 IP/CIDR/域名。", name, invalidCount))
	}
	if len(normalizedTokens) == 0 {
		return SourceBuildResult{
			InvalidCount:     invalidCount,
			SourcePorts:      sourcePorts,
			ColoFilterActive: sourceColoActive,
			ColoFilterColos:  sourceColos,
			Warnings:         warnings,
		}, nil
	}

	if sourceColoFilter != "" {
		if options.SourceColoFilterPhase != SourceColoFilterPhaseStage2 {
			if len(sourceColos) == 0 && sourceColoMode != task.ColoFilterModeDeny {
				warnings = append(warnings, fmt.Sprintf("输入源 %s 的 COLO 筛选没有匹配候选。", name))
				return SourceBuildResult{
					InvalidCount:     invalidCount,
					SourcePorts:      sourcePorts,
					ColoFilterActive: sourceColoActive,
					ColoFilterColos:  sourceColos,
					Warnings:         DedupeStrings(warnings),
				}, nil
			}
			coloFilter, err := colodict.NewModeFilterForTokens(options.ColoDictionaryPaths, sourceColoFilter, normalizedTokens, sourceColoMode)
			if err != nil {
				if colodict.HasCountryToken(sourceColoFilter) {
					return SourceBuildResult{InvalidCount: invalidCount, SourcePorts: sourcePorts, ColoFilterActive: sourceColoActive, ColoFilterColos: sourceColos, Warnings: warnings}, err
				}
				warnings = append(warnings, fmt.Sprintf("输入源 %s 的 COLO 预筛需要本地 COLO 词典，已保留原始候选：%v", name, err))
				coloFilter = nil
			}
			if err == nil && coloFilter != nil {
				filteredTokens := make([]string, 0, len(normalizedTokens))
				for _, token := range normalizedTokens {
					if err := ctx.Err(); err != nil {
						return SourceBuildResult{}, err
					}
					filteredTokens = append(filteredTokens, coloFilter.FilterToken(token)...)
				}
				if len(filteredTokens) == 0 {
					warnings = append(warnings, fmt.Sprintf("输入源 %s 的 COLO 筛选没有匹配候选。", name))
					return SourceBuildResult{InvalidCount: invalidCount, SourcePorts: sourcePorts, ColoFilterActive: sourceColoActive, ColoFilterColos: sourceColos, Warnings: DedupeStrings(warnings)}, nil
				}
				normalizedTokens = filteredTokens
				warnings = append(warnings, fmt.Sprintf("输入源 %s 已按 COLO %s %s 预筛候选。", name, ColoModeLabel(sourceColoMode), sourceColoFilter))
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 的 COLO %s %s 将在第二阶段起效。", name, ColoModeLabel(sourceColoMode), sourceColoFilter))
		}
	}

	if mode == "mcis" {
		if options.MCISRunner == nil {
			return SourceBuildResult{InvalidCount: invalidCount, SourcePorts: sourcePorts, ColoFilterActive: sourceColoActive, ColoFilterColos: sourceColos, Warnings: warnings}, fmt.Errorf("输入源 %s 缺少 MICS 抽样执行器", name)
		}
		candidates, mcisWarnings, err := options.MCISRunner(normalizedTokens, limit)
		warnings = append(warnings, mcisWarnings...)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return SourceBuildResult{}, ctxErr
		}
		if err != nil {
			return SourceBuildResult{InvalidCount: invalidCount, ColoFilterActive: sourceColoActive, ColoFilterColos: sourceColos, Warnings: warnings}, err
		}
		entries := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			if strings.TrimSpace(candidate.IP) != "" {
				entries = append(entries, candidate.IP)
			}
		}
		if len(entries) >= limit {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 达到 IP 上限 %d，已截断候选列表。", name, limit))
		}
		if len(sourcePorts) > 0 {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 使用 MICS 抽样时暂不继承源端口，已回退全局测速端口。", name))
		}
		return SourceBuildResult{Entries: entries, MCISCandidates: candidates, InvalidCount: invalidCount, ColoFilterActive: sourceColoActive, ColoFilterColos: sourceColos, Warnings: DedupeStrings(warnings)}, nil
	}

	entries, builderTruncated, err := buildTraverseEntriesContext(ctx, normalizedTokens, limit)
	if err != nil {
		return SourceBuildResult{}, err
	}
	if parsed.Truncated {
		if prefilterParseBudget > 0 {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 的 COLO 预筛解析达到预算 %d 条，剩余候选未解析；可调大 probe.source_pool_limit。", name, prefilterParseBudget))
		} else {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 达到 IP 上限 %d，已截断候选列表。", name, limit))
		}
	}
	if builderTruncated {
		warnings = append(warnings, fmt.Sprintf("输入源 %s 达到 IP 上限 %d，已截断候选列表。", name, limit))
	}
	sourcePorts = PrunePortsToEntries(sourcePorts, entries)

	return SourceBuildResult{
		Entries:          entries,
		InvalidCount:     invalidCount,
		SourcePorts:      sourcePorts,
		ColoFilterActive: sourceColoActive,
		ColoFilterColos:  sourceColos,
		Warnings:         DedupeStrings(warnings),
	}, nil
}

func sortedStringKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func BuildTraverseEntries(tokens []string, limit int) ([]string, bool) {
	entries, truncated, _ := buildTraverseEntriesContext(context.Background(), tokens, limit)
	return entries, truncated
}

func buildTraverseEntriesContext(ctx context.Context, tokens []string, limit int) ([]string, bool, error) {
	entries := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit)

	for _, token := range tokens {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		// 池已满且还有 token 未展开：确实有候选被丢弃。
		if len(entries) >= limit {
			return entries, true, nil
		}

		expanded, tokenTruncated, err := expandTraverseTokenContext(ctx, token, limit-len(entries))
		if err != nil {
			return nil, false, err
		}
		for _, entry := range expanded {
			key := candidateNetworkKey(entry)
			if _, exists := seen[key]; exists {
				continue
			}
			if len(entries) >= limit {
				return entries, true, nil
			}
			seen[key] = struct{}{}
			entries = append(entries, entry)
		}
		// CIDR 展开被当前剩余容量截断：段内仍有未枚举的地址。
		if tokenTruncated {
			return entries, true, nil
		}
	}
	return entries, false, nil
}

// candidateNetworkKey 把 IPv6 候选归一到 /64：同一 /64 只保留首个候选，
// 因此细于 /64 的输入（如 2001:db8::9/64）最终只贡献该 /64 的网络地址。
func candidateNetworkKey(value string) string {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil || ip.To4() != nil {
		return value
	}
	ip = ip.To16()
	network := make(net.IP, net.IPv6len)
	copy(network[:8], ip[:8])
	return network.String() + "/64"
}

func ExpandTraverseToken(token string, limit int) ([]string, bool) {
	entries, truncated, _ := expandTraverseTokenContext(context.Background(), token, limit)
	return entries, truncated
}

func expandTraverseTokenContext(ctx context.Context, token string, limit int) ([]string, bool, error) {
	if limit <= 0 {
		return nil, true, nil
	}
	if !strings.Contains(token, "/") {
		return []string{token}, false, nil
	}

	_, ipNet, err := net.ParseCIDR(token)
	if err != nil {
		return nil, false, nil
	}

	return enumerateCIDRIPsContext(ctx, ipNet, limit)
}

func ColoModeLabel(mode string) string {
	if task.NormalizeColoFilterMode(mode) == task.ColoFilterModeDeny {
		return "黑名单"
	}
	return "白名单"
}

func DedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func enumerateCIDRIPs(ipNet *net.IPNet, limit int) ([]string, bool) {
	entries, truncated, _ := enumerateCIDRIPsContext(context.Background(), ipNet, limit)
	return entries, truncated
}

func enumerateCIDRIPsContext(ctx context.Context, ipNet *net.IPNet, limit int) ([]string, bool, error) {
	if limit <= 0 {
		return nil, true, nil
	}
	_, bits := ipNet.Mask.Size()
	current := cloneIPForBits(ipNet.IP, bits)
	entries := make([]string, 0, limit)

	for len(entries) < limit && ipNet.Contains(current) {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		entries = append(entries, current.String())
		if bits == 128 {
			incrementIPv6Network64(current)
		} else {
			incrementIP(current)
		}
	}

	return entries, ipNet.Contains(current), nil
}

func incrementIPv6Network64(ip net.IP) {
	for i := 7; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			return
		}
	}
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
