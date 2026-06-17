package probecore

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/sourceparse"
	"github.com/axuitomo/CFST-GUI/internal/task"
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
	Entries          []string
	InvalidCount     int
	MCISDuration     time.Duration
	SourcePorts      map[string]int
	ColoFilterActive bool
	ColoFilterColos  []string
	Warnings         []string
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
		parseLimit = 0
	}

	parsed := sourceparse.Parse(options.Raw, sourceparse.Options{Limit: parseLimit, Resolver: options.Resolver})
	normalizedTokens := append([]string(nil), parsed.Valid...)
	sourcePorts := CloneStringIntMap(parsed.Ports)
	invalidCount := len(parsed.Invalid)
	warnings := append([]string(nil), parsed.Warnings...)
	warnings = append(warnings, sourceColoWarnings...)
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
		mcisStart := time.Now()
		entries, mcisWarnings, err := options.MCISRunner(normalizedTokens, limit)
		mcisDuration := time.Since(mcisStart)
		warnings = append(warnings, mcisWarnings...)
		if err != nil {
			return SourceBuildResult{InvalidCount: invalidCount, MCISDuration: mcisDuration, ColoFilterActive: sourceColoActive, ColoFilterColos: sourceColos, Warnings: warnings}, err
		}
		if len(entries) >= limit {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 达到 IP 上限 %d，已截断候选列表。", name, limit))
		}
		if len(sourcePorts) > 0 {
			warnings = append(warnings, fmt.Sprintf("输入源 %s 使用 MICS 抽样时暂不继承源端口，已回退全局测速端口。", name))
		}
		return SourceBuildResult{Entries: entries, InvalidCount: invalidCount, MCISDuration: mcisDuration, ColoFilterActive: sourceColoActive, ColoFilterColos: sourceColos, Warnings: DedupeStrings(warnings)}, nil
	}

	entries, truncated := BuildTraverseEntries(normalizedTokens, limit)
	if truncated {
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
