package colodict

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
)

type Filter struct {
	entries []ColoEntry
	mode    string
}

func NewFilterForTokens(paths Paths, allowRaw string, tokens []string) (*Filter, error) {
	return NewModeFilterForTokens(paths, allowRaw, tokens, "allow")
}

func NewModeFilterForTokens(paths Paths, raw string, tokens []string, mode string) (*Filter, error) {
	if err := RequireColoFileForAllowList(paths, raw); err != nil {
		return nil, err
	}
	path := ColoPathForTokens(paths, tokens)
	if path != paths.Colo && !fileExists(path) && fileExists(paths.Colo) {
		path = paths.Colo
	}
	colos, _, err := ResolveTokensToColos(paths, raw)
	if err != nil {
		return nil, err
	}
	return NewModeFilterForColos(path, colos, raw, mode)
}

func RequireColoFileForAllowList(paths Paths, allowRaw string) error {
	if !HasCountryToken(allowRaw) {
		return nil
	}
	if !fileExists(paths.Colo) {
		return fmt.Errorf("输入源设置了 COLO 筛选，但 COLO 文件不存在：%s，请先处理 COLO 词典", paths.Colo)
	}
	return nil
}

func LookupColo(entries []ColoEntry, rawIP string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(rawIP))
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.Prefix.Contains(addr) {
			return entry.Colo
		}
	}
	return ""
}

func ResolveTokensToColos(paths Paths, raw string) (map[string]struct{}, []string, error) {
	tokens := parseCountryColoTokens(raw)
	if len(tokens) == 0 {
		return nil, nil, nil
	}
	colos := make(map[string]struct{})
	countries := make(map[string]string)
	unmatched := make([]string, 0)
	for token := range tokens {
		normalized := strings.ToUpper(strings.TrimSpace(token))
		if normalized == "" {
			continue
		}
		if colo := normalizeColo(normalized); colo != "" {
			colos[colo] = struct{}{}
			continue
		}
		if country := normalizeCountryCode(normalized); country != "" {
			countries[country] = normalized
			continue
		}
		unmatched = append(unmatched, normalized)
	}

	if len(countries) > 0 {
		if !fileExists(paths.Colo) {
			return nil, nil, fmt.Errorf("COLO 文件不存在：%s，请先处理 COLO 词典", paths.Colo)
		}
		entries, err := LoadColoEntries(paths.Colo)
		if err != nil {
			return nil, nil, err
		}
		countryColos := make(map[string]map[string]struct{})
		for _, entry := range entries {
			colo := normalizeColo(entry.Colo)
			country := normalizeCountryCode(entry.Country)
			if colo == "" || country == "" {
				continue
			}
			if countryColos[country] == nil {
				countryColos[country] = make(map[string]struct{})
			}
			countryColos[country][colo] = struct{}{}
		}
		for country, original := range countries {
			matched := countryColos[country]
			if len(matched) == 0 {
				unmatched = append(unmatched, original)
				continue
			}
			for colo := range matched {
				colos[colo] = struct{}{}
			}
		}
	}
	if len(colos) == 0 {
		return nil, unmatched, nil
	}
	return colos, unmatched, nil
}

func HasCountryToken(value string) bool {
	tokens := parseCountryColoTokens(value)
	for token := range tokens {
		if normalizeColo(token) != "" {
			continue
		}
		if normalizeCountryCode(token) != "" {
			return true
		}
	}
	return false
}

func parseCountryColoTokens(value string) map[string]struct{} {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == '/' || r == '\\' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	tokens := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		token := strings.ToUpper(strings.TrimSpace(field))
		if token != "" {
			tokens[token] = struct{}{}
		}
	}
	return tokens
}

func ColoPathForTokens(paths Paths, tokens []string) string {
	hasIPv4 := false
	hasIPv6 := false
	for _, token := range tokens {
		prefix, ok := tokenPrefix(token)
		if !ok {
			continue
		}
		if prefix.Addr().Is4() {
			hasIPv4 = true
		} else {
			hasIPv6 = true
		}
		if hasIPv4 && hasIPv6 {
			return paths.Colo
		}
	}
	if hasIPv4 && !hasIPv6 && strings.TrimSpace(paths.ColoIPv4) != "" {
		return paths.ColoIPv4
	}
	if hasIPv6 && !hasIPv4 && strings.TrimSpace(paths.ColoIPv6) != "" {
		return paths.ColoIPv6
	}
	return paths.Colo
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func NewFilter(path string, allowRaw string) (*Filter, error) {
	return NewModeFilter(path, allowRaw, "allow")
}

func NewModeFilter(path string, raw string, mode string) (*Filter, error) {
	return NewModeFilterForColos(path, parseColoAllowList(raw), raw, mode)
}

func NewModeFilterForColos(path string, allowed map[string]struct{}, raw string, mode string) (*Filter, error) {
	if len(allowed) == 0 {
		return nil, nil
	}
	entries, err := LoadColoEntries(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("输入源设置了 COLO 筛选，但 COLO 文件不存在：%s，请先更新 COLO 词典", path)
		}
		return nil, err
	}
	filtered := make([]ColoEntry, 0, len(entries))
	for _, entry := range entries {
		if _, ok := allowed[entry.Colo]; ok {
			filtered = append(filtered, entry)
		}
	}
	mode = normalizeFilterMode(mode)
	if len(filtered) == 0 && mode != "deny" {
		return nil, fmt.Errorf("COLO 文件中没有匹配的筛选地区码：%s", raw)
	}
	if mode == "deny" {
		return &Filter{entries: filtered, mode: mode}, nil
	}
	return &Filter{entries: filtered, mode: "allow"}, nil
}

func (f *Filter) FilterToken(token string) []string {
	if f == nil {
		return []string{token}
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	prefix, ok := tokenPrefix(token)
	if !ok {
		return nil
	}
	if normalizeFilterMode(f.mode) == "deny" {
		return f.filterTokenDeny(token, prefix)
	}
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, entry := range f.entries {
		intersection, ok := intersectPrefixes(prefix, entry.Prefix)
		if !ok {
			continue
		}
		value := prefixStringForToken(intersection, token)
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (f *Filter) filterTokenDeny(token string, prefix netip.Prefix) []string {
	if len(f.entries) == 0 {
		return []string{token}
	}
	remaining := []netip.Prefix{prefix}
	for _, entry := range f.entries {
		next := make([]netip.Prefix, 0, len(remaining))
		for _, current := range remaining {
			next = append(next, subtractPrefix(current, entry.Prefix)...)
		}
		remaining = next
		if len(remaining) == 0 {
			return nil
		}
	}
	result := make([]string, 0, len(remaining))
	seen := make(map[string]struct{}, len(remaining))
	for _, item := range remaining {
		value := prefixStringForToken(item, token)
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeFilterMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "deny", "blacklist", "black-list", "black_list", "blocklist", "block-list", "block_list":
		return "deny"
	default:
		return "allow"
	}
}

func parseColoAllowList(value string) map[string]struct{} {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == '/' || r == '\\' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if colo := normalizeColo(field); colo != "" {
			allowed[colo] = struct{}{}
		}
	}
	return allowed
}

func normalizeColo(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 3 {
		return ""
	}
	for _, r := range value {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return value
}

func normalizeCountryCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "UK" {
		return "GB"
	}
	if len(value) != 2 {
		return ""
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return ""
		}
	}
	return value
}
