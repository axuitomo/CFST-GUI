package colodict

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
)

const (
	DefaultGeofeedURL   = "https://api.cloudflare.com/local-ip-ranges.csv"
	DefaultLocationsURL = "https://cdn.jsdelivr.net/gh/Netrvin/cloudflare-colo-list@main/locations.json"
	DefaultCountryURL   = "https://cdn.jsdelivr.net/gh/Netrvin/cloudflare-colo-list@main/country.json"
	GeofeedFileName     = "local-ip-ranges.csv"
	ColoFileName        = "cloudflare-colos.csv"
	ColoIPv4FileName    = "cloudflare-colos-ipv4.csv"
	ColoIPv6FileName    = "cloudflare-colos-ipv6.csv"
	LocationsFileName   = "cloudflare-colo-locations.json"
	CountryFileName     = "cloudflare-countries.json"
)

type Paths struct {
	Colo      string
	ColoIPv4  string
	ColoIPv6  string
	Country   string
	Geofeed   string
	Locations string
}

type Status struct {
	ColoIPv4Path  string `json:"colo_ipv4_path"`
	ColoIPv4Rows  int    `json:"colo_ipv4_rows"`
	ColoIPv6Path  string `json:"colo_ipv6_path"`
	ColoIPv6Rows  int    `json:"colo_ipv6_rows"`
	ColoPath      string `json:"colo_path"`
	ColoRows      int    `json:"colo_rows"`
	GeofeedPath   string `json:"geofeed_path"`
	GeofeedRows   int    `json:"geofeed_rows"`
	LastUpdatedAt string `json:"last_updated_at"`
	MatchedRows   int    `json:"matched_rows"`
	MissingRows   int    `json:"missing_rows"`
	SourceURL     string `json:"source_url"`
	Updated       bool   `json:"updated"`
	UnmatchedRows int    `json:"unmatched_rows"`
}

type UpdateOptions struct {
	Client       *http.Client
	CountryURL   string
	LocationsURL string
	Paths        Paths
	SourceURL    string
}

type GeofeedEntry struct {
	Prefix  netip.Prefix
	Country string
	Region  string
	City    string
	Postal  string
}

type ColoEntry struct {
	Prefix  netip.Prefix
	Colo    string
	Country string
	Region  string
	City    string
}

type UpdateResult struct {
	Status
	Warnings []string
}

type LocationEntry struct {
	Country string
	City    string
	Colo    string
	Region  string
}

type Filter struct {
	entries []ColoEntry
	allowed map[string]struct{}
	denied  map[string]struct{}
	mode    string
}

func DefaultPaths(baseDir string) Paths {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = "."
	}
	return Paths{
		Colo:      filepath.Join(baseDir, ColoFileName),
		ColoIPv4:  filepath.Join(baseDir, ColoIPv4FileName),
		ColoIPv6:  filepath.Join(baseDir, ColoIPv6FileName),
		Country:   filepath.Join(baseDir, CountryFileName),
		Geofeed:   filepath.Join(baseDir, GeofeedFileName),
		Locations: filepath.Join(baseDir, LocationsFileName),
	}
}

func StatusForPaths(paths Paths) (Status, error) {
	status := Status{
		ColoIPv4Path: paths.ColoIPv4,
		ColoIPv6Path: paths.ColoIPv6,
		ColoPath:     paths.Colo,
		GeofeedPath:  paths.Geofeed,
		SourceURL:    DefaultGeofeedURL,
	}
	var geofeedEntries []GeofeedEntry
	if info, err := os.Stat(paths.Geofeed); err == nil {
		status.LastUpdatedAt = info.ModTime().Format(time.RFC3339)
		status.Updated = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return status, err
	}

	if raw, err := os.ReadFile(paths.Geofeed); err == nil {
		entries, _, parseErr := ParseGeofeed(raw)
		if parseErr != nil {
			return status, parseErr
		}
		geofeedEntries = entries
		status.GeofeedRows = len(entries)
	} else if !errors.Is(err, os.ErrNotExist) {
		return status, err
	}

	var coloEntries []ColoEntry
	if rows, err := LoadColoEntries(paths.Colo); err == nil {
		coloEntries = rows
		status.ColoRows = len(rows)
		status.MatchedRows = len(rows)
	} else if errors.Is(err, os.ErrNotExist) {
		status.MissingRows = status.GeofeedRows
		status.UnmatchedRows = status.GeofeedRows
	} else {
		return status, err
	}
	if rows, err := loadOptionalColoRowCount(paths.ColoIPv4); err == nil {
		status.ColoIPv4Rows = rows
	} else {
		return status, err
	}
	if rows, err := loadOptionalColoRowCount(paths.ColoIPv6); err == nil {
		status.ColoIPv6Rows = rows
	} else {
		return status, err
	}

	if len(geofeedEntries) > 0 {
		if len(coloEntries) > 0 {
			status.MatchedRows = countMatchedGeofeedRows(geofeedEntries, coloEntries)
			status.MissingRows = len(geofeedEntries) - status.MatchedRows
			status.UnmatchedRows = status.MissingRows
		}
	}
	return status, nil
}

func Update(ctx context.Context, options UpdateOptions) (UpdateResult, error) {
	sourceURL := strings.TrimSpace(options.SourceURL)
	if sourceURL == "" {
		sourceURL = DefaultGeofeedURL
	}
	locationsURL := strings.TrimSpace(options.LocationsURL)
	if locationsURL == "" {
		locationsURL = DefaultLocationsURL
	}
	countryURL := strings.TrimSpace(options.CountryURL)
	if countryURL == "" {
		countryURL = DefaultCountryURL
	}
	client := options.Client
	if client == nil {
		client = defaultUpdateHTTPClient()
	}

	raw, locationRaw, countryRaw, warnings, err := fetchUpdateSources(ctx, client, sourceURL, locationsURL, countryURL)
	if err != nil {
		return UpdateResult{}, err
	}

	entries, _, err := ParseGeofeed(raw)
	if err != nil {
		return UpdateResult{}, err
	}

	if err := os.MkdirAll(filepath.Dir(options.Paths.Geofeed), 0o755); err != nil {
		return UpdateResult{}, err
	}
	if err := writeFileAtomic(options.Paths.Geofeed, raw, 0o600); err != nil {
		return UpdateResult{}, err
	}
	if len(locationRaw) > 0 {
		if err := writeFileAtomic(options.Paths.Locations, locationRaw, 0o600); err != nil {
			return UpdateResult{}, err
		}
	}
	if len(countryRaw) > 0 {
		if err := writeFileAtomic(options.Paths.Country, countryRaw, 0o600); err != nil {
			return UpdateResult{}, err
		}
	}
	status, err := StatusForPaths(options.Paths)
	if err != nil {
		return UpdateResult{}, err
	}
	status.SourceURL = sourceURL
	status.GeofeedRows = len(entries)
	status.Updated = true
	if info, err := os.Stat(options.Paths.Geofeed); err == nil {
		status.LastUpdatedAt = info.ModTime().Format(time.RFC3339)
	}
	return UpdateResult{Status: status, Warnings: dedupeWarnings(warnings)}, nil
}

func Process(options UpdateOptions) (UpdateResult, error) {
	raw, err := os.ReadFile(options.Paths.Geofeed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return UpdateResult{}, fmt.Errorf("GEOFEED 文件不存在：%s，请先更新词典", options.Paths.Geofeed)
		}
		return UpdateResult{}, err
	}
	entries, _, err := ParseGeofeed(raw)
	if err != nil {
		return UpdateResult{}, err
	}

	coloEntries, unmatched, warnings := buildColoEntriesFromLocalSources(entries, options.Paths)
	if len(coloEntries) == 0 {
		return UpdateResult{}, errors.New("GEOFEED 未能映射出任何 COLO 记录")
	}
	coloIPv4Entries, coloIPv6Entries := splitColoEntriesByAddressFamily(coloEntries)
	if err := writeColoEntries(options.Paths.Colo, coloEntries); err != nil {
		return UpdateResult{}, err
	}
	if err := writeColoEntries(options.Paths.ColoIPv4, coloIPv4Entries); err != nil {
		return UpdateResult{}, err
	}
	if err := writeColoEntries(options.Paths.ColoIPv6, coloIPv6Entries); err != nil {
		return UpdateResult{}, err
	}

	status, err := StatusForPaths(options.Paths)
	if err != nil {
		return UpdateResult{}, err
	}
	status.GeofeedRows = len(entries)
	status.ColoRows = len(coloEntries)
	status.ColoIPv4Rows = len(coloIPv4Entries)
	status.ColoIPv6Rows = len(coloIPv6Entries)
	status.MatchedRows = len(entries) - unmatched
	status.MissingRows = unmatched
	status.UnmatchedRows = unmatched
	status.Updated = true
	if info, err := os.Stat(options.Paths.Geofeed); err == nil {
		status.LastUpdatedAt = info.ModTime().Format(time.RFC3339)
	}
	return UpdateResult{Status: status, Warnings: dedupeWarnings(warnings)}, nil
}

func defaultUpdateHTTPClient() *http.Client {
	return httpclient.NewClient(httpclient.Options{
		Profile: httpcfg.Resolve("", "", "", "", true),
	})
}

type updateSourceDownload struct {
	key   string
	label string
	raw   []byte
	url   string
	err   error
}

func fetchUpdateSources(ctx context.Context, client *http.Client, sourceURL, locationsURL, countryURL string) ([]byte, []byte, []byte, []string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	sources := []updateSourceDownload{
		{key: "geofeed", label: "GEOFEED", url: sourceURL},
		{key: "locations", label: "COLO locations", url: locationsURL},
		{key: "country", label: "country", url: countryURL},
	}
	results := make(chan updateSourceDownload, len(sources))
	for _, source := range sources {
		source := source
		go func() {
			raw, err := fetchURL(ctx, client, source.url, source.label)
			if err != nil {
				err = fmt.Errorf("%s 下载失败：%w", source.label, err)
			}
			source.raw = raw
			source.err = err
			results <- source
		}()
	}

	var geofeedRaw []byte
	var locationRaw []byte
	var countryRaw []byte
	warnings := make([]string, 0, 2)
	for range sources {
		result := <-results
		if result.err != nil {
			if result.key == "geofeed" {
				return nil, nil, nil, nil, result.err
			}
			warnings = append(warnings, fmt.Sprintf("%v，处理词典时会使用上次本地文件或内置映射。", result.err))
			continue
		}
		switch result.key {
		case "geofeed":
			geofeedRaw = result.raw
		case "locations":
			locationRaw = result.raw
		case "country":
			countryRaw = result.raw
		}
	}
	if len(locationRaw) == 0 || len(countryRaw) == 0 {
		warnings = append(warnings, "辅助 COLO 映射源未完整拉取；处理词典时会使用上次本地文件或内置映射。")
	}
	return geofeedRaw, locationRaw, countryRaw, dedupeWarnings(warnings), nil
}

func buildColoEntriesFromLocalSources(entries []GeofeedEntry, paths Paths) ([]ColoEntry, int, []string) {
	locationRaw, locationErr := os.ReadFile(paths.Locations)
	countryRaw, countryErr := os.ReadFile(paths.Country)
	warnings := make([]string, 0, 2)
	if locationErr == nil && countryErr == nil {
		locations, parseLocationErr := ParseLocations(locationRaw)
		countries, parseCountryErr := ParseCountries(countryRaw)
		if parseLocationErr == nil && parseCountryErr == nil {
			coloEntries, unmatched := BuildColoEntriesWithLocations(entries, locations, countries)
			return coloEntries, unmatched, nil
		}
		if parseLocationErr != nil {
			warnings = append(warnings, fmt.Sprintf("本地 COLO locations 解析失败，已使用内置映射降级：%v", parseLocationErr))
		}
		if parseCountryErr != nil {
			warnings = append(warnings, fmt.Sprintf("本地 country 数据解析失败，已使用内置映射降级：%v", parseCountryErr))
		}
	} else {
		if locationErr != nil {
			warnings = append(warnings, fmt.Sprintf("本地 COLO locations 不可用，已使用内置映射降级：%v", locationErr))
		}
		if countryErr != nil {
			warnings = append(warnings, fmt.Sprintf("本地 country 数据不可用，已使用内置映射降级：%v", countryErr))
		}
	}
	coloEntries, unmatched := BuildColoEntries(entries)
	return coloEntries, unmatched, warnings
}

func dedupeWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	result := make([]string, 0, len(warnings))
	seen := make(map[string]struct{}, len(warnings))
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		if _, exists := seen[warning]; exists {
			continue
		}
		seen[warning] = struct{}{}
		result = append(result, warning)
	}
	return result
}

func fetchURL(ctx context.Context, client *http.Client, sourceURL, label string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("%s 更新返回状态 %s", label, response.Status)
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func ParseGeofeed(raw []byte) ([]GeofeedEntry, int, error) {
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	entries := make([]GeofeedEntry, 0)
	invalid := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return entries, invalid, err
		}
		if len(record) == 0 {
			continue
		}
		first := strings.TrimSpace(record[0])
		if first == "" || strings.HasPrefix(first, "#") {
			continue
		}
		if strings.EqualFold(first, "ip_prefix") {
			continue
		}
		if len(record) < 4 {
			invalid++
			continue
		}
		prefix, err := netip.ParsePrefix(first)
		if err != nil {
			invalid++
			continue
		}
		entry := GeofeedEntry{
			Prefix:  prefix.Masked(),
			Country: strings.TrimSpace(record[1]),
			Region:  strings.TrimSpace(record[2]),
			City:    strings.TrimSpace(record[3]),
		}
		if len(record) > 4 {
			entry.Postal = strings.TrimSpace(record[4])
		}
		entries = append(entries, entry)
	}
	return entries, invalid, nil
}

func ParseLocations(raw []byte) ([]LocationEntry, error) {
	var records []struct {
		CCA2   string `json:"cca2"`
		City   string `json:"city"`
		IATA   string `json:"iata"`
		Region string `json:"region"`
	}
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, err
	}
	entries := make([]LocationEntry, 0, len(records))
	for _, record := range records {
		colo := normalizeColo(record.IATA)
		country := strings.ToUpper(strings.TrimSpace(record.CCA2))
		city := strings.TrimSpace(record.City)
		if colo == "" || country == "" || city == "" {
			continue
		}
		entries = append(entries, LocationEntry{
			Country: country,
			City:    city,
			Colo:    colo,
			Region:  strings.TrimSpace(record.Region),
		})
	}
	if len(entries) == 0 {
		return nil, errors.New("COLO locations 未包含可用记录")
	}
	return entries, nil
}

func ParseCountries(raw []byte) (map[string]string, error) {
	var countries map[string]string
	if err := json.Unmarshal(raw, &countries); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(countries))
	for code, name := range countries {
		code = strings.ToUpper(strings.TrimSpace(code))
		name = strings.TrimSpace(name)
		if len(code) != 2 || name == "" {
			continue
		}
		result[code] = name
	}
	if len(result) == 0 {
		return nil, errors.New("country 数据未包含可用记录")
	}
	return result, nil
}

func BuildColoEntries(entries []GeofeedEntry) ([]ColoEntry, int) {
	return buildColoEntries(entries, nil)
}

func BuildColoEntriesWithLocations(entries []GeofeedEntry, locations []LocationEntry, countries map[string]string) ([]ColoEntry, int) {
	return buildColoEntries(entries, newColoLookup(locations, countries))
}

func buildColoEntries(entries []GeofeedEntry, lookup *coloLookup) ([]ColoEntry, int) {
	result := make([]ColoEntry, 0, len(entries))
	unmatched := 0
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		colos := colosForGeofeedEntryWithLookup(entry, lookup)
		if len(colos) == 0 {
			unmatched++
			continue
		}
		for _, colo := range colos {
			key := entry.Prefix.String() + "|" + colo
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, ColoEntry{
				Prefix:  entry.Prefix,
				Colo:    colo,
				Country: entry.Country,
				Region:  entry.Region,
				City:    entry.City,
			})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if left.Prefix.Addr().Compare(right.Prefix.Addr()) != 0 {
			return left.Prefix.Addr().Compare(right.Prefix.Addr()) < 0
		}
		if left.Prefix.Bits() != right.Prefix.Bits() {
			return left.Prefix.Bits() < right.Prefix.Bits()
		}
		return left.Colo < right.Colo
	})
	return result, unmatched
}

func EncodeColoEntries(entries []ColoEntry) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{"ip_prefix", "colo", "country", "region", "city"}); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if err := writer.Write([]string{
			entry.Prefix.String(),
			entry.Colo,
			entry.Country,
			entry.Region,
			entry.City,
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func LoadColoEntries(path string) ([]ColoEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	entries := make([]ColoEntry, 0)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return entries, err
		}
		if len(record) == 0 {
			continue
		}
		first := strings.TrimSpace(record[0])
		if first == "" || strings.HasPrefix(first, "#") || strings.EqualFold(first, "ip_prefix") {
			continue
		}
		if len(record) < 2 {
			continue
		}
		prefix, err := netip.ParsePrefix(first)
		if err != nil {
			continue
		}
		colo := normalizeColo(strings.TrimSpace(record[1]))
		if colo == "" {
			continue
		}
		entry := ColoEntry{
			Prefix: prefix.Masked(),
			Colo:   colo,
		}
		if len(record) > 2 {
			entry.Country = strings.TrimSpace(record[2])
		}
		if len(record) > 3 {
			entry.Region = strings.TrimSpace(record[3])
		}
		if len(record) > 4 {
			entry.City = strings.TrimSpace(record[4])
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func loadOptionalColoRowCount(path string) (int, error) {
	if strings.TrimSpace(path) == "" {
		return 0, nil
	}
	rows, err := LoadColoEntries(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	return len(rows), nil
}

func splitColoEntriesByAddressFamily(entries []ColoEntry) ([]ColoEntry, []ColoEntry) {
	ipv4Entries := make([]ColoEntry, 0, len(entries))
	ipv6Entries := make([]ColoEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Prefix.Addr().Is4() {
			ipv4Entries = append(ipv4Entries, entry)
			continue
		}
		ipv6Entries = append(ipv6Entries, entry)
	}
	return ipv4Entries, ipv6Entries
}

func writeColoEntries(path string, entries []ColoEntry) error {
	raw, err := EncodeColoEntries(entries)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, raw, 0o600)
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
	return NewModeFilter(path, raw, mode)
}

func HasColoAllowList(allowRaw string) bool {
	return len(parseColoAllowList(allowRaw)) > 0
}

func RequireColoFileForAllowList(paths Paths, allowRaw string) error {
	if !HasColoAllowList(allowRaw) {
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

func LookupColoInFile(path string, rawIP string) (string, error) {
	entries, err := LoadColoEntries(path)
	if err != nil {
		return "", err
	}
	return LookupColo(entries, rawIP), nil
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
	allowed := parseColoAllowList(raw)
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
		return &Filter{entries: filtered, denied: allowed, mode: mode}, nil
	}
	return &Filter{entries: filtered, allowed: allowed, mode: "allow"}, nil
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

func tokenPrefix(token string) (netip.Prefix, bool) {
	if strings.Contains(token, "/") {
		prefix, err := netip.ParsePrefix(token)
		if err != nil {
			return netip.Prefix{}, false
		}
		return prefix.Masked(), true
	}
	addr, err := netip.ParseAddr(token)
	if err != nil {
		return netip.Prefix{}, false
	}
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32), true
	}
	return netip.PrefixFrom(addr, 128), true
}

func intersectPrefixes(left, right netip.Prefix) (netip.Prefix, bool) {
	left = left.Masked()
	right = right.Masked()
	if left.Addr().Is4() != right.Addr().Is4() {
		return netip.Prefix{}, false
	}
	if left.Bits() >= right.Bits() {
		if right.Contains(left.Addr()) {
			return left, true
		}
		return netip.Prefix{}, false
	}
	if left.Contains(right.Addr()) {
		return right, true
	}
	return netip.Prefix{}, false
}

func subtractPrefix(source, remove netip.Prefix) []netip.Prefix {
	source = source.Masked()
	remove = remove.Masked()
	if !prefixesOverlap(source, remove) {
		return []netip.Prefix{source}
	}
	if remove.Bits() <= source.Bits() && remove.Contains(source.Addr()) {
		return nil
	}
	maxBits := prefixMaxBits(source)
	if source.Bits() >= maxBits {
		return []netip.Prefix{source}
	}
	left, right := splitPrefix(source)
	result := make([]netip.Prefix, 0, 2)
	result = append(result, subtractPrefix(left, remove)...)
	result = append(result, subtractPrefix(right, remove)...)
	return result
}

func prefixesOverlap(left, right netip.Prefix) bool {
	left = left.Masked()
	right = right.Masked()
	if left.Addr().Is4() != right.Addr().Is4() {
		return false
	}
	return left.Contains(right.Addr()) || right.Contains(left.Addr())
}

func splitPrefix(prefix netip.Prefix) (netip.Prefix, netip.Prefix) {
	prefix = prefix.Masked()
	nextBits := prefix.Bits() + 1
	step := new(big.Int).Lsh(big.NewInt(1), uint(prefixMaxBits(prefix)-nextBits))
	secondAddr := bigToAddr(new(big.Int).Add(addrToBig(prefix.Addr()), step), prefix.Addr().Is4())
	return netip.PrefixFrom(prefix.Addr(), nextBits).Masked(), netip.PrefixFrom(secondAddr, nextBits).Masked()
}

func prefixMaxBits(prefix netip.Prefix) int {
	if prefix.Addr().Is4() {
		return 32
	}
	return 128
}

func addrToBig(addr netip.Addr) *big.Int {
	if addr.Is4() {
		raw := addr.As4()
		return new(big.Int).SetBytes(raw[:])
	}
	raw := addr.As16()
	return new(big.Int).SetBytes(raw[:])
}

func bigToAddr(value *big.Int, ipv4 bool) netip.Addr {
	size := 16
	if ipv4 {
		size = 4
	}
	raw := value.Bytes()
	if len(raw) > size {
		raw = raw[len(raw)-size:]
	}
	padded := make([]byte, size)
	copy(padded[size-len(raw):], raw)
	if ipv4 {
		return netip.AddrFrom4([4]byte{padded[0], padded[1], padded[2], padded[3]})
	}
	return netip.AddrFrom16([16]byte{
		padded[0], padded[1], padded[2], padded[3],
		padded[4], padded[5], padded[6], padded[7],
		padded[8], padded[9], padded[10], padded[11],
		padded[12], padded[13], padded[14], padded[15],
	})
}

func prefixStringForToken(prefix netip.Prefix, original string) string {
	prefix = prefix.Masked()
	if !strings.Contains(original, "/") {
		return prefix.Addr().String()
	}
	return prefix.String()
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

type coloLookup struct {
	cityColos        map[string][]string
	countryCityColos map[string][]string
	countryNames     map[string]string
}

func newColoLookup(locations []LocationEntry, countries map[string]string) *coloLookup {
	lookup := &coloLookup{
		cityColos:        make(map[string][]string),
		countryCityColos: make(map[string][]string),
		countryNames:     make(map[string]string),
	}
	for code, name := range countries {
		code = strings.ToUpper(strings.TrimSpace(code))
		name = normalizeName(name)
		if len(code) == 2 && name != "" {
			lookup.countryNames[name] = code
		}
	}

	cityCountryKeys := make(map[string]map[string]struct{})
	for _, location := range locations {
		country := strings.ToUpper(strings.TrimSpace(location.Country))
		city := normalizeName(location.City)
		colo := normalizeColo(location.Colo)
		if country == "" || city == "" || colo == "" {
			continue
		}
		countryCityKey := country + "|" + city
		appendUniqueColo(lookup.countryCityColos, countryCityKey, colo)
		if cityCountryKeys[city] == nil {
			cityCountryKeys[city] = make(map[string]struct{})
		}
		cityCountryKeys[city][countryCityKey] = struct{}{}
	}

	for city, countryKeys := range cityCountryKeys {
		if len(countryKeys) != 1 {
			continue
		}
		for countryKey := range countryKeys {
			lookup.cityColos[city] = append([]string(nil), lookup.countryCityColos[countryKey]...)
		}
	}
	for _, colos := range lookup.countryCityColos {
		sort.Strings(colos)
	}
	for _, colos := range lookup.cityColos {
		sort.Strings(colos)
	}
	return lookup
}

func appendUniqueColo(target map[string][]string, key, colo string) {
	for _, existing := range target[key] {
		if existing == colo {
			return
		}
	}
	target[key] = append(target[key], colo)
}

func colosForGeofeedEntry(entry GeofeedEntry) []string {
	return colosForGeofeedEntryWithLookup(entry, nil)
}

func colosForGeofeedEntryWithLookup(entry GeofeedEntry, lookup *coloLookup) []string {
	country := normalizeGeofeedCountry(entry.Country, lookup)
	region := normalizeGeofeedRegion(country, entry.Region)
	for _, city := range geofeedCityCandidates(country, region, entry.City) {
		if colos := staticColosForGeofeed(country, region, city); len(colos) > 0 {
			return colos
		}
	}
	if lookup == nil {
		return nil
	}
	for _, city := range geofeedCityCandidates(country, region, entry.City) {
		cityKey := normalizeName(city)
		if cityKey == "" {
			continue
		}
		if colos := lookup.countryCityColos[country+"|"+cityKey]; len(colos) > 0 {
			return colos
		}
	}
	for _, city := range geofeedCityCandidates(country, region, entry.City) {
		if colos := lookup.cityColos[normalizeName(city)]; len(colos) > 0 {
			return colos
		}
	}
	return nil
}

func staticColosForGeofeed(country, region, city string) []string {
	key := mappingKey(country, region, city)
	if colos, ok := geofeedColoMap[key]; ok {
		return colos
	}
	key = mappingKey(country, "", city)
	if colos, ok := geofeedColoMap[key]; ok {
		return colos
	}
	key = mappingKey("", "", city)
	return geofeedColoMap[key]
}

func normalizeGeofeedCountry(country string, lookup *coloLookup) string {
	country = strings.TrimSpace(country)
	upper := strings.ToUpper(country)
	if len(upper) == 2 {
		return upper
	}
	if lookup != nil {
		if code := lookup.countryNames[normalizeName(country)]; code != "" {
			return code
		}
	}
	return upper
}

func normalizeGeofeedRegion(country, region string) string {
	country = strings.ToUpper(strings.TrimSpace(country))
	region = strings.TrimSpace(region)
	upperRegion := strings.ToUpper(region)
	prefix := country + "-"
	if country != "" && strings.HasPrefix(upperRegion, prefix) {
		return strings.TrimSpace(upperRegion[len(prefix):])
	}
	return upperRegion
}

func geofeedCityCandidates(country, region, city string) []string {
	candidates := make([]string, 0, 3)
	addCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range candidates {
			if normalizeName(existing) == normalizeName(value) {
				return
			}
		}
		candidates = append(candidates, value)
	}
	addCandidate(city)
	for _, key := range []string{
		mappingKey(country, region, city),
		mappingKey(country, "", city),
		mappingKey("", "", city),
	} {
		for _, alias := range geofeedCityAliases[key] {
			addCandidate(alias)
		}
	}
	return candidates
}

func mappingKey(country, region, city string) string {
	return strings.ToUpper(strings.Join([]string{
		strings.TrimSpace(country),
		strings.TrimSpace(region),
		strings.TrimSpace(city),
	}, "|"))
}

func normalizeName(value string) string {
	return strings.Join(strings.Fields(strings.ToUpper(strings.TrimSpace(value))), " ")
}

func countMatchedGeofeedRows(geofeedEntries []GeofeedEntry, coloEntries []ColoEntry) int {
	mappedPrefixes := make(map[string]struct{}, len(coloEntries))
	for _, entry := range coloEntries {
		mappedPrefixes[entry.Prefix.String()] = struct{}{}
	}
	matched := 0
	for _, entry := range geofeedEntries {
		if _, ok := mappedPrefixes[entry.Prefix.String()]; ok {
			matched++
		}
	}
	return matched
}

func writeFileAtomic(path string, raw []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if _, err := temp.Write(raw); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(perm); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

var geofeedColoMap = map[string][]string{
	mappingKey("AU", "", "Sydney"):          {"SYD"},
	mappingKey("CA", "", "Toronto"):         {"YYZ"},
	mappingKey("DE", "", "Frankfurt"):       {"FRA"},
	mappingKey("ES", "", "Madrid"):          {"MAD"},
	mappingKey("FR", "", "Paris"):           {"CDG"},
	mappingKey("GB", "", "London"):          {"LHR"},
	mappingKey("HK", "", "Hong Kong"):       {"HKG"},
	mappingKey("IN", "", "Mumbai"):          {"BOM"},
	mappingKey("JP", "", "Osaka"):           {"KIX"},
	mappingKey("JP", "", "Tokyo"):           {"NRT"},
	mappingKey("KR", "", "Seoul"):           {"ICN"},
	mappingKey("NL", "", "Amsterdam"):       {"AMS"},
	mappingKey("SG", "", "Singapore"):       {"SIN"},
	mappingKey("TW", "", "Kaohsiung City"):  {"KHH"},
	mappingKey("TW", "", "Kaohsiung"):       {"KHH"},
	mappingKey("TW", "", "Taipei"):          {"TPE"},
	mappingKey("US", "GA", "Atlanta"):       {"ATL"},
	mappingKey("US", "IL", "Chicago"):       {"ORD"},
	mappingKey("US", "TX", "Dallas"):        {"DFW"},
	mappingKey("US", "VA", "Ashburn"):       {"IAD"},
	mappingKey("US", "CA", "Los Angeles"):   {"LAX"},
	mappingKey("US", "FL", "Miami"):         {"MIA"},
	mappingKey("US", "NJ", "Newark"):        {"EWR"},
	mappingKey("US", "CA", "San Francisco"): {"SFO"},
	mappingKey("US", "CA", "San Jose"):      {"SJC"},
	mappingKey("US", "WA", "Seattle"):       {"SEA"},
}

var geofeedCityAliases = map[string][]string{
	mappingKey("CA", "ON", "Mississauga"):       {"Toronto"},
	mappingKey("DE", "HE", "Dreieich"):          {"Frankfurt", "Frankfurt-am-Main"},
	mappingKey("FR", "IDF", "Aulnay-sous-Bois"): {"Paris"},
	mappingKey("GB", "HNS", "Hounslow"):         {"London"},
	mappingKey("JP", "12", "Narita"):            {"Tokyo"},
	mappingKey("US", "CA", "East Los Angeles"):  {"Los Angeles"},
	mappingKey("US", "TX", "Dallas"):            {"Dallas-Fort Worth"},
	mappingKey("US", "WA", "Kent"):              {"Seattle"},
	mappingKey("", "", "East Los Angeles"):      {"Los Angeles"},
	mappingKey("", "", "Frankfurt-am-Main"):     {"Frankfurt"},
}
