package task

import (
	"net/http"
	"regexp"
	"strings"
	"unicode"
)

type ColoInfo struct {
	Code    string
	City    string
	Country string
	Region  string
}

var (
	regexpTraceBodyColo = regexp.MustCompile(`(?im)^colo=([a-z0-9]{3})\s*$`)
	regexpCFRayColo     = regexp.MustCompile(`(?i)-([a-z0-9]{3})(?:$|[^a-z0-9])`)
)

var iataPlusColos = map[string]ColoInfo{
	"AMS": {Code: "AMS", City: "Amsterdam", Country: "Netherlands", Region: "Europe"},
	"ATL": {Code: "ATL", City: "Atlanta", Country: "United States", Region: "North America"},
	"BOM": {Code: "BOM", City: "Mumbai", Country: "India", Region: "Asia Pacific"},
	"CDG": {Code: "CDG", City: "Paris", Country: "France", Region: "Europe"},
	"DFW": {Code: "DFW", City: "Dallas", Country: "United States", Region: "North America"},
	"EWR": {Code: "EWR", City: "Newark", Country: "United States", Region: "North America"},
	"FRA": {Code: "FRA", City: "Frankfurt", Country: "Germany", Region: "Europe"},
	"HKG": {Code: "HKG", City: "Hong Kong", Country: "Hong Kong", Region: "Asia Pacific"},
	"IAD": {Code: "IAD", City: "Ashburn", Country: "United States", Region: "North America"},
	"ICN": {Code: "ICN", City: "Seoul", Country: "South Korea", Region: "Asia Pacific"},
	"KHH": {Code: "KHH", City: "Kaohsiung", Country: "Taiwan", Region: "Asia Pacific"},
	"KIX": {Code: "KIX", City: "Osaka", Country: "Japan", Region: "Asia Pacific"},
	"LAX": {Code: "LAX", City: "Los Angeles", Country: "United States", Region: "North America"},
	"LHR": {Code: "LHR", City: "London", Country: "United Kingdom", Region: "Europe"},
	"MAD": {Code: "MAD", City: "Madrid", Country: "Spain", Region: "Europe"},
	"MIA": {Code: "MIA", City: "Miami", Country: "United States", Region: "North America"},
	"NRT": {Code: "NRT", City: "Tokyo", Country: "Japan", Region: "Asia Pacific"},
	"ORD": {Code: "ORD", City: "Chicago", Country: "United States", Region: "North America"},
	"SEA": {Code: "SEA", City: "Seattle", Country: "United States", Region: "North America"},
	"SFO": {Code: "SFO", City: "San Francisco", Country: "United States", Region: "North America"},
	"SIN": {Code: "SIN", City: "Singapore", Country: "Singapore", Region: "Asia Pacific"},
	"SJC": {Code: "SJC", City: "San Jose", Country: "United States", Region: "North America"},
	"SYD": {Code: "SYD", City: "Sydney", Country: "Australia", Region: "Asia Pacific"},
	"TPE": {Code: "TPE", City: "Taipei", Country: "Taiwan", Region: "Asia Pacific"},
	"YYZ": {Code: "YYZ", City: "Toronto", Country: "Canada", Region: "North America"},
}

func ExtractColo(header http.Header, body []byte) string {
	if colo := ExtractColoFromTraceBody(body); colo != "" {
		return colo
	}
	if header == nil {
		return ""
	}
	if colo := ExtractColoFromCFRay(header.Get("cf-ray")); colo != "" {
		return colo
	}
	return extractColoFromCDNHeaders(header)
}

func ExtractColoFromTraceBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	matches := regexpTraceBodyColo.FindSubmatch(body)
	if len(matches) < 2 {
		return ""
	}
	return normalizeColoCode(string(matches[1]))
}

func ExtractColoFromCFRay(value string) string {
	matches := regexpCFRayColo.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) < 2 {
		return ""
	}
	return normalizeColoCode(matches[1])
}

func ColoInfoFor(code string) (ColoInfo, bool) {
	normalized := normalizeColoCode(code)
	if normalized == "" {
		return ColoInfo{}, false
	}
	info, ok := iataPlusColos[normalized]
	if ok {
		return info, true
	}
	return ColoInfo{Code: normalized}, false
}

func ParseColoAllowList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || unicode.IsSpace(r)
	})
	if len(fields) == 0 {
		return nil
	}

	result := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		code := normalizeColoCode(field)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func normalizeColoCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 3 {
		return ""
	}
	for _, r := range value {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return ""
		}
	}
	if info, ok := iataPlusColos[value]; ok {
		return info.Code
	}
	return value
}

func extractColoFromCDNHeaders(header http.Header) (colo string) {
	if header.Get("server") != "" {
		if header.Get("server") == "CDN77-Turbo" {
			if colo = header.Get("x-77-pop"); colo != "" {
				return RegexpColoCountryCode.FindString(colo)
			}
		}
		if colo = header.Get("server"); strings.Contains(colo, "BunnyCDN-") {
			return RegexpColoCountryCode.FindString(strings.TrimPrefix(colo, "BunnyCDN-"))
		}
	}
	if colo = header.Get("x-amz-cf-pop"); colo != "" {
		return normalizeColoCode(RegexpColoIATACode.FindString(colo))
	}
	if colo = header.Get("x-served-by"); colo != "" {
		if matches := RegexpColoIATACode.FindAllString(colo, -1); len(matches) > 0 {
			return normalizeColoCode(matches[len(matches)-1])
		}
	}
	if colo = header.Get("x-id-fe"); colo != "" {
		if colo = RegexpColoGcore.FindString(colo); colo != "" {
			return strings.ToUpper(colo)
		}
	}
	return ""
}
