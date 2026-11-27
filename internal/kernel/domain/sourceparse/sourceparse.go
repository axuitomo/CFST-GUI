package sourceparse

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const DefaultLookupTimeout = 3 * time.Second

// DefaultResolveDuration 限制一次解析中所有域名查询的累计耗时。
// 域名解析不受候选数上限约束（解析失败不会占用 Limit），因此必须另设预算，
// 否则一份大量失效域名的输入会让阶段 0 长时间无响应。
const DefaultResolveDuration = 30 * time.Second

const (
	minDomainLookupBudget = 1024
	maxDomainLookupBudget = 20000
)

var (
	ipCandidatePattern     = regexp.MustCompile(`(?i)(?:\b(?:\d{1,3}\.){3}\d{1,3}(?:/\d{1,2})?\b|\[?[0-9a-f]{0,4}(?::[0-9a-f]{0,4}){2,}\]?(?:/\d{1,3})?)`)
	urlCandidatePattern    = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s,;]+`)
	domainCandidatePattern = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.?\b`)
	cidrPortPattern        = regexp.MustCompile(`(?i)^(\[?[0-9a-f:.]+\]?(?:/\d{1,3})):(\d{1,5})$`)
	hashPortPattern        = regexp.MustCompile(`^(.*?)(?:#)(\d{1,5})$`)
)

type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type Options struct {
	Context       context.Context
	Limit         int
	LookupTimeout time.Duration
	Resolver      Resolver
	// MaxDomainLookups 覆盖域名查询次数预算；<=0 时按 Limit 推导（1024~20000）。
	MaxDomainLookups int
	// MaxResolveDuration 覆盖域名查询累计耗时预算；<=0 时使用 DefaultResolveDuration。
	MaxResolveDuration time.Duration
	// Now 仅用于测试预算判定。
	Now func() time.Time
}

type Result struct {
	CandidateCount int
	Invalid        []string
	Ports          map[string]int
	RawLineCount   int
	Valid          []string
	Warnings       []string
	// Truncated 表示因为 Limit 而跳过了仍在输入的候选，用于上层给出准确的截断提示。
	Truncated bool
}

type parseState struct {
	domainCache map[string][]string
	invalidSeen map[string]struct{}
	opts        Options

	domainLookupBudget    int
	domainLookups         int
	resolveDeadline       time.Time
	lookupBudgetExceeded  bool
	resolveBudgetExceeded bool
	truncated             bool
}

func Parse(raw string, opts Options) Result {
	normalized := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(raw)
	lines := strings.Split(normalized, "\n")
	result := Result{RawLineCount: len(lines)}
	state := parseState{
		domainCache:        make(map[string][]string),
		invalidSeen:        make(map[string]struct{}),
		opts:               opts,
		domainLookupBudget: resolveDomainLookupBudget(opts),
	}

	for _, line := range lines {
		if state.contextCanceled() {
			break
		}
		line = cleanLine(line)
		if line == "" {
			continue
		}
		// 先剔除空行/注释行再判断上限：文件末尾的换行不应触发假的截断告警。
		if state.limitReached(result) {
			state.truncated = true
			break
		}

		lineValid, lineInvalid, linePorts, lineWarnings, candidateCount := parseLine(line, &state, result)
		result.CandidateCount += candidateCount
		result.Valid = append(result.Valid, lineValid...)
		result.Invalid = append(result.Invalid, lineInvalid...)
		result.Warnings = append(result.Warnings, lineWarnings...)
		if len(linePorts) > 0 {
			if result.Ports == nil {
				result.Ports = make(map[string]int)
			}
			for token, port := range linePorts {
				result.Ports[token] = port
			}
		}
	}

	result.Truncated = state.truncated
	result.Warnings = append(result.Warnings, state.resolveBudgetWarnings()...)
	return result
}

func (state *parseState) contextCanceled() bool {
	return state.opts.Context != nil && state.opts.Context.Err() != nil
}

func NormalizeIPToken(token string) (string, bool) {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, "[]")
	if token == "" {
		return "", false
	}
	if strings.Contains(token, "/") {
		ip, ipNet, err := net.ParseCIDR(token)
		if err != nil {
			return "", false
		}
		ones, _ := ipNet.Mask.Size()
		return fmt.Sprintf("%s/%d", ip.String(), ones), true
	}
	ip := net.ParseIP(token)
	if ip == nil {
		return "", false
	}
	return ip.String(), true
}

func cleanLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if hasTerminalHashPort(line) {
		return line
	}
	if idx := strings.IndexByte(line, '#'); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func parseLine(line string, state *parseState, result Result) ([]string, []string, map[string]int, []string, int) {
	valid := make([]string, 0)
	invalid := make([]string, 0)
	// ports 只在行内真的带端口时才建：make(map) 一定会分配，平铺 IP 列表下这是每行一次的纯浪费。
	var ports map[string]int
	warnings := make([]string, 0)
	candidateCount := 0

	line, sourcePort, portWarning := extractLinePort(line)
	if portWarning != "" {
		warnings = append(warnings, portWarning)
	}
	ipCandidates := findIPCandidates(line)
	validIPCount := 0
	invalidIPLike := make(map[string]struct{}, len(ipCandidates))
	for _, candidate := range ipCandidates {
		candidateCount++
		normalized, ok := NormalizeIPToken(candidate)
		if !ok {
			invalid = append(invalid, state.addInvalid(candidate)...)
			invalidIPLike[candidate] = struct{}{}
			continue
		}
		valid = append(valid, normalized)
		if sourcePort > 0 && !strings.Contains(normalized, "/") {
			if ports == nil {
				ports = make(map[string]int, 1)
			}
			ports[normalized] = sourcePort
		}
		validIPCount++
	}
	if validIPCount > 0 {
		return valid, invalid, ports, warnings, candidateCount
	}

	domainCandidates := domainCandidates(line)
	if len(domainCandidates) == 0 {
		return nil, []string{line}, nil, warnings, 1
	}
	for _, candidate := range domainCandidates {
		if _, exists := invalidIPLike[candidate]; exists {
			continue
		}
		candidateCount++
		domain, ok := NormalizeDomain(candidate)
		if !ok {
			invalid = append(invalid, state.addInvalid(candidate)...)
			continue
		}
		ips := state.resolveDomain(domain)
		if len(ips) == 0 {
			invalid = append(invalid, state.addInvalid(domain)...)
			continue
		}
		for _, ip := range ips {
			if state.limitReachedWith(result, len(valid)) {
				state.truncated = true
				break
			}
			valid = append(valid, ip)
			if sourcePort > 0 {
				if ports == nil {
					ports = make(map[string]int, len(ips))
				}
				ports[ip] = sourcePort
			}
		}
		if state.limitReachedWith(result, len(valid)) {
			state.truncated = true
			break
		}
	}

	return valid, invalid, ports, warnings, candidateCount
}

func extractLinePort(line string) (string, int, string) {
	value := strings.TrimSpace(line)
	if value == "" {
		return line, 0, ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err == nil {
			port, ok := parsePort(parsed.Port())
			if ok {
				parsed.Host = parsed.Hostname()
				return parsed.String(), port, ""
			}
		}
		return line, 0, ""
	}
	if normalized, ok := stripCIDRPort(value); ok {
		return normalized, 0, "CIDR 输入暂不支持携带端口，已回退全局测速端口。"
	}
	if normalized, port, ok := stripHashPort(value); ok {
		if cidrValue, matched := stripCIDRHashPort(normalized, port); matched {
			return cidrValue, 0, "CIDR 输入暂不支持携带端口，已回退全局测速端口。"
		}
		return normalized, port, ""
	}
	if strings.HasPrefix(value, "[") {
		host, portText, err := net.SplitHostPort(value)
		if err == nil {
			port, ok := parsePort(portText)
			if ok {
				return strings.Trim(host, "[]"), port, ""
			}
		}
		return line, 0, ""
	}
	// 没有冒号就不可能是 host:port：先判一次，省掉每个纯 IP 行一次必然失败的 SplitHostPort 错误分配。
	if strings.IndexByte(value, ':') >= 0 {
		host, portText, err := net.SplitHostPort(value)
		if err == nil {
			port, ok := parsePort(portText)
			if ok {
				return host, port, ""
			}
		}
	}
	lastColon := strings.LastIndex(value, ":")
	if lastColon > 0 && strings.Count(value, ":") == 1 {
		port, ok := parsePort(value[lastColon+1:])
		if ok {
			return value[:lastColon], port, ""
		}
	}
	return line, 0, ""
}

func hasTerminalHashPort(value string) bool {
	_, _, ok := stripHashPort(strings.TrimSpace(value))
	return ok
}

func stripHashPort(value string) (string, int, bool) {
	matches := hashPortPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return "", 0, false
	}
	port, ok := parsePort(matches[2])
	if !ok {
		return "", 0, false
	}
	host := strings.TrimSpace(matches[1])
	if host == "" {
		return "", 0, false
	}
	return host, port, true
}

func stripCIDRHashPort(value string, port int) (string, bool) {
	if port <= 0 {
		return "", false
	}
	candidate := strings.Trim(strings.TrimSpace(value), "[]")
	if _, ok := NormalizeIPToken(candidate); !ok || !strings.Contains(candidate, "/") {
		return "", false
	}
	return candidate, true
}

func stripCIDRPort(value string) (string, bool) {
	matches := cidrPortPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return "", false
	}
	if _, ok := parsePort(matches[2]); !ok {
		return "", false
	}
	candidate := strings.Trim(matches[1], "[]")
	if _, ok := NormalizeIPToken(candidate); !ok || !strings.Contains(candidate, "/") {
		return "", false
	}
	return candidate, true
}

func parsePort(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return port, port > 0 && port <= 65535
}

func findIPCandidates(line string) []string {
	matches := ipCandidatePattern.FindAllStringIndex(line, -1)
	if len(matches) == 0 {
		return nil
	}

	candidates := make([]string, 0, len(matches))
	for _, match := range matches {
		if !hasIPCandidateBoundary(line, match[0], match[1]) {
			continue
		}
		candidates = append(candidates, line[match[0]:match[1]])
	}
	return candidates
}

func hasIPCandidateBoundary(line string, start, end int) bool {
	if start > 0 && isIPTokenChar(line[start-1]) {
		return false
	}
	if end < len(line) && isIPTokenChar(line[end]) {
		return false
	}
	return true
}

func isIPTokenChar(b byte) bool {
	return (b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		b == '_' ||
		b == '-'
}

func domainCandidates(line string) []string {
	urlMatches := urlCandidatePattern.FindAllString(line, -1)
	if len(urlMatches) > 0 {
		candidates := make([]string, 0, len(urlMatches))
		for _, rawURL := range urlMatches {
			parsed, err := url.Parse(rawURL)
			if err != nil {
				continue
			}
			host := strings.TrimSpace(parsed.Hostname())
			if host != "" {
				candidates = append(candidates, host)
			}
		}
		if len(candidates) > 0 {
			return candidates
		}
	}
	return domainCandidatePattern.FindAllString(line, -1)
}

func NormalizeDomain(domain string) (string, bool) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" || len(domain) > 253 || !strings.Contains(domain, ".") {
		return "", false
	}
	for _, r := range domain {
		if r > unicode.MaxASCII {
			return "", false
		}
	}

	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if !validDomainLabel(label) {
			return "", false
		}
	}
	if isAllDigits(labels[len(labels)-1]) {
		return "", false
	}
	return domain, true
}

func validDomainLabel(label string) bool {
	if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (state *parseState) addInvalid(token string) []string {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if _, exists := state.invalidSeen[token]; exists {
		return nil
	}
	state.invalidSeen[token] = struct{}{}
	return []string{token}
}

func (state *parseState) limitReached(result Result) bool {
	return state.limitReachedWith(result, 0)
}

func (state *parseState) limitReachedWith(result Result, pending int) bool {
	return state.opts.Limit > 0 && len(result.Valid)+pending >= state.opts.Limit
}

func (state *parseState) resolveDomain(domain string) []string {
	if cached, ok := state.domainCache[domain]; ok {
		return cached
	}
	if !state.allowDomainLookup() {
		return nil
	}
	resolver := state.opts.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	timeout := state.opts.LookupTimeout
	if timeout <= 0 {
		timeout = DefaultLookupTimeout
	}

	parentCtx := state.opts.Context
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	addrs, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		state.domainCache[domain] = nil
		return nil
	}

	seen := make(map[string]struct{}, len(addrs))
	ips := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if addr.IP == nil {
			continue
		}
		ip := addr.IP.String()
		if _, exists := seen[ip]; exists {
			continue
		}
		seen[ip] = struct{}{}
		ips = append(ips, ip)
	}
	state.domainCache[domain] = ips
	return ips
}

// resolveDomainLookupBudget 把候选上限换算成域名查询次数预算。
// 解析失败不占用 Limit，因此必须单独限次，否则失效域名列表可以无限循环。
func resolveDomainLookupBudget(opts Options) int {
	if opts.MaxDomainLookups > 0 {
		return opts.MaxDomainLookups
	}
	budget := opts.Limit * 4
	if budget < minDomainLookupBudget {
		budget = minDomainLookupBudget
	}
	if budget > maxDomainLookupBudget {
		budget = maxDomainLookupBudget
	}
	return budget
}

func (state *parseState) resolveDuration() time.Duration {
	if state.opts.MaxResolveDuration > 0 {
		return state.opts.MaxResolveDuration
	}
	return DefaultResolveDuration
}

func (state *parseState) now() time.Time {
	if state.opts.Now != nil {
		return state.opts.Now()
	}
	return time.Now()
}

// allowDomainLookup 在次数或累计耗时预算耗尽后拒绝继续解析域名，
// 调用方把该域名计入无效候选，解析流程本身继续处理 IP/CIDR。
func (state *parseState) allowDomainLookup() bool {
	if state.domainLookups >= state.domainLookupBudget {
		state.lookupBudgetExceeded = true
		return false
	}
	now := state.now()
	if state.resolveDeadline.IsZero() {
		state.resolveDeadline = now.Add(state.resolveDuration())
	}
	if !now.Before(state.resolveDeadline) {
		state.resolveBudgetExceeded = true
		return false
	}
	state.domainLookups++
	return true
}

func (state *parseState) resolveBudgetWarnings() []string {
	warnings := make([]string, 0, 2)
	if state.lookupBudgetExceeded {
		warnings = append(warnings, fmt.Sprintf("输入源域名解析次数达到上限 %d，剩余域名已跳过。", state.domainLookupBudget))
	}
	if state.resolveBudgetExceeded {
		warnings = append(warnings, fmt.Sprintf("输入源域名解析累计耗时超过 %s 上限，剩余域名已跳过。", state.resolveDuration()))
	}
	return warnings
}
