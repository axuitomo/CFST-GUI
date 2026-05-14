package probecore

import (
	"fmt"
	"net"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/sourceparse"
	"github.com/axuitomo/CFST-GUI/task"
)

const SourceColoFilterPhaseStage2 = "stage2"

type MCISSourceRunner func(tokens []string, limit int) ([]string, []string, error)

type SourceBuildOptions struct {
	Raw                   string
	Name                  string
	Mode                  string
	Limit                 int
	Resolver              sourceparse.Resolver
	ColoFilter            string
	ColoMode              string
	ColoDictionaryPaths   colodict.Paths
	SourceColoFilterPhase string
	MCISRunner            MCISSourceRunner
}

type SourceBuildResult struct {
	Entries      []string
	InvalidCount int
	SourcePorts  map[string]int
	Warnings     []string
}

func BuildSourceEntries(options SourceBuildOptions) (SourceBuildResult, error) {
	limit := options.Limit
	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = "输入源"
	}
	mode := strings.TrimSpace(options.Mode)
	if mode == "" {
		mode = "traverse"
	}
	parseLimit := limit
	sourceColoFilter := strings.TrimSpace(options.ColoFilter)
	sourceColoMode := task.NormalizeColoFilterMode(options.ColoMode)
	if sourceColoFilter != "" {
		if err := colodict.RequireColoFileForAllowList(options.ColoDictionaryPaths, sourceColoFilter); err != nil {
			return SourceBuildResult{}, err
		}
	}
	if sourceColoFilter != "" && options.SourceColoFilterPhase != SourceColoFilterPhaseStage2 {
		parseLimit = 0
	}

	parsed := sourceparse.Parse(options.Raw, sourceparse.Options{Limit: parseLimit, Resolver: options.Resolver})
	normalizedTokens := append([]string(nil), parsed.Valid...)
	sourcePorts := CloneStringIntMap(parsed.Ports)
	invalidCount := len(parsed.Invalid)
	warnings := append([]string(nil), parsed.Warnings...)
	if invalidCount > 0 {
		warnings = append(warnings, fmt.Sprintf("输入源 %s 忽略了 %d 条无效 IP/CIDR/域名。", name, invalidCount))
	}
	if len(normalizedTokens) == 0 {
		return SourceBuildResult{
			InvalidCount: invalidCount,
			SourcePorts:  sourcePorts,
			Warnings:     warnings,
		}, nil
	}

	if sourceColoFilter != "" {
		if options.SourceColoFilterPhase != SourceColoFilterPhaseStage2 {
			coloFilter, err := colodict.NewModeFilterForTokens(options.ColoDictionaryPaths, sourceColoFilter, normalizedTokens, sourceColoMode)
			if err != nil {
				return SourceBuildResult{InvalidCount: invalidCount, SourcePorts: sourcePorts, Warnings: warnings}, err
			}
			if coloFilter != nil {
				filteredTokens := make([]string, 0, len(normalizedTokens))
				for _, token := range normalizedTokens {
					filteredTokens = append(filteredTokens, coloFilter.FilterToken(token)...)
				}
				if len(filteredTokens) == 0 {
					warnings = append(warnings, fmt.Sprintf("输入源 %s 的 COLO 筛选没有匹配候选。", name))
					return SourceBuildResult{InvalidCount: invalidCount, SourcePorts: sourcePorts, Warnings: DedupeStrings(warnings)}, nil
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
			return SourceBuildResult{InvalidCount: invalidCount, SourcePorts: sourcePorts, Warnings: warnings}, fmt.Errorf("输入源 %s 缺少 MICS 抽样执行器", name)
		}
		entries, mcisWarnings, err := options.MCISRunner(normalizedTokens, limit)
		warnings = append(warnings, mcisWarnings...)
		if err != nil {
			return SourceBuildResult{InvalidCount: invalidCount, Warnings: warnings}, err
		}
		if len(entries) >= limit {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 达到 IP 上限 %d，已截断候选列表。", name, limit))
		}
		if len(sourcePorts) > 0 {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 使用 MICS 抽样时暂不继承源端口，已回退全局测速端口。", name))
		}
		return SourceBuildResult{Entries: entries, InvalidCount: invalidCount, Warnings: DedupeStrings(warnings)}, nil
	}

	entries, truncated := BuildTraverseEntries(normalizedTokens, limit)
	if truncated {
		warnings = append(warnings, fmt.Sprintf("输入源 %s 达到 IP 上限 %d，已截断候选列表。", name, limit))
	}
	sourcePorts = PrunePortsToEntries(sourcePorts, entries)

	return SourceBuildResult{
		Entries:      entries,
		InvalidCount: invalidCount,
		SourcePorts:  sourcePorts,
		Warnings:     DedupeStrings(warnings),
	}, nil
}

func BuildTraverseEntries(tokens []string, limit int) ([]string, bool) {
	entries := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit)
	truncated := false

	for _, token := range tokens {
		if len(entries) >= limit {
			truncated = true
			break
		}

		expanded, tokenTruncated := ExpandTraverseToken(token, limit-len(entries))
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
	return entries, truncated
}

func ExpandTraverseToken(token string, limit int) ([]string, bool) {
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
