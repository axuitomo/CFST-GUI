package probecore

import "strings"

// NormalizeSourceGroupIPMode keeps an empty mode for legacy per-source settings.
func NormalizeSourceGroupIPMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "traverse":
		return "traverse"
	case "mcis":
		return "mcis"
	default:
		return ""
	}
}
