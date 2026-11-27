package colodict

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/github/githubdownload"
)

const (
	DefaultGeofeedURL   = "https://api.cloudflare.com/local-ip-ranges.csv"
	DefaultLocationsURL = "https://github.com/Netrvin/cloudflare-colo-list/raw/main/locations.json"
	// DefaultLocationsAltURL uses city names instead of airport-town names, so merging it
	// with the primary list matches more geofeed rows (Ashburn vs Dulles, Dallas vs
	// Dallas-Fort Worth). Empty disables the merge.
	DefaultLocationsAltURL = "https://www.baipiao.eu.org/cloudflare/locations"
	DefaultCountryURL      = "https://github.com/Netrvin/cloudflare-colo-list/raw/main/country.json"
	GeofeedFileName        = "local-ip-ranges.csv"
	ColoFileName           = "cloudflare-colos.csv"
	ColoIPv4FileName       = "cloudflare-colos-ipv4.csv"
	ColoIPv6FileName       = "cloudflare-colos-ipv6.csv"
	LocationsFileName      = "cloudflare-colo-locations.json"
	CountryFileName        = "cloudflare-countries.json"
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
	Client          *http.Client
	CountryURL      string
	LocationsAltURL string
	LocationsURL    string
	Paths           Paths
	SourceURL       string
	URLCandidates   func(string) []string
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
	locationsAltURL := strings.TrimSpace(options.LocationsAltURL)
	if locationsAltURL == "" {
		locationsAltURL = DefaultLocationsAltURL
	}
	countryURL := strings.TrimSpace(options.CountryURL)
	if countryURL == "" {
		countryURL = DefaultCountryURL
	}
	client := options.Client
	if client == nil {
		client = defaultUpdateHTTPClient()
	}
	urlCandidates := options.URLCandidates
	if urlCandidates == nil {
		urlCandidates = githubdownload.Candidates
	}

	raw, locationRaw, locationAltRaw, countryRaw, warnings, err := fetchUpdateSources(ctx, client, urlCandidates, sourceURL, locationsURL, locationsAltURL, countryURL)
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
	mergedLocationRaw, mergeWarnings, err := mergeLocationSources(locationRaw, locationAltRaw)
	if err != nil {
		return UpdateResult{}, err
	}
	warnings = append(warnings, mergeWarnings...)
	if len(mergedLocationRaw) > 0 {
		if err := writeFileAtomic(options.Paths.Locations, mergedLocationRaw, 0o600); err != nil {
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
