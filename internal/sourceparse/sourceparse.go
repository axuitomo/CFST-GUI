package sourceparse

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const DefaultLookupTimeout = 3 * time.Second

var (
	ipCandidatePattern     = regexp.MustCompile(`(?i)(?:\b(?:\d{1,3}\.){3}\d{1,3}(?:/\d{1,2})?\b|\[?[0-9a-f]{0,4}(?::[0-9a-f]{0,4}){2,}\]?(?:/\d{1,3})?)`)
	urlCandidatePattern    = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s,;]+`)
	domainCandidatePattern = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.?\b`)
)

type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type Options struct {
	Limit         int
	LookupTimeout time.Duration
	Resolver      Resolver
}

type Result struct {
	CandidateCount int
	Invalid        []string
	RawLineCount   int
	Valid          []string
}

type parseState struct {
	domainCache map[string][]string
	invalidSeen map[string]struct{}
	opts        Options
}

func Parse(raw string, opts Options) Result {
	normalized := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(raw)
	lines := strings.Split(normalized, "\n")
	result := Result{RawLineCount: len(lines)}
	state := parseState{
		domainCache: make(map[string][]string),
		invalidSeen: make(map[string]struct{}),
		opts:        opts,
	}

	for _, line := range lines {
		if state.limitReached(result) {
			break
		}
		line = cleanLine(line)
		if line == "" {
			continue
		}

		lineValid, lineInvalid, candidateCount := parseLine(line, &state, result)
		result.CandidateCount += candidateCount
		result.Valid = append(result.Valid, lineValid...)
		result.Invalid = append(result.Invalid, lineInvalid...)
	}

	return result
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
	if idx := strings.IndexByte(line, '#'); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func parseLine(line string, state *parseState, result Result) ([]string, []string, int) {
	valid := make([]string, 0)
	invalid := make([]string, 0)
	candidateCount := 0

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
		validIPCount++
	}
	if validIPCount > 0 {
		return valid, invalid, candidateCount
	}

	domainCandidates := domainCandidates(line)
	if len(domainCandidates) == 0 {
		return nil, []string{line}, 1
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
				break
			}
			valid = append(valid, ip)
		}
		if state.limitReachedWith(result, len(valid)) {
			break
		}
	}

	return valid, invalid, candidateCount
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

	resolver := state.opts.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	timeout := state.opts.LookupTimeout
	if timeout <= 0 {
		timeout = DefaultLookupTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
