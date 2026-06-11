package githubdownload

import (
	"net/url"
	"strings"
)

var githubMirrorPrefixes = []string{
	"https://ghproxy.vip/",
	"https://gh.3w.pm/",
	"https://gh.ddlc.top/",
}

func Candidates(value string) []string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return []string{raw}
	}
	if !strings.EqualFold(parsed.Host, "github.com") {
		return uniqueURLs([]string{raw})
	}

	candidates := make([]string, 0, len(githubMirrorPrefixes)+1)
	for _, prefix := range githubMirrorPrefixes {
		candidates = append(candidates, prefix+raw)
	}
	candidates = append(candidates, raw)
	return uniqueURLs(candidates)
}

func uniqueURLs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
