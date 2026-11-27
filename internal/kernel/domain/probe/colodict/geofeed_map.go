package colodict

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

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
	slices.SortStableFunc(result, func(left, right ColoEntry) int {
		if left.Prefix.Addr().Compare(right.Prefix.Addr()) != 0 {
			return left.Prefix.Addr().Compare(right.Prefix.Addr())
		}
		if left.Prefix.Bits() != right.Prefix.Bits() {
			return left.Prefix.Bits() - right.Prefix.Bits()
		}
		return strings.Compare(left.Colo, right.Colo)
	})
	return result, unmatched
}

type coloLookup struct {
	cityColos        map[string][]string
	coloCountries    map[string]string
	countryCityColos map[string][]string
	countryNames     map[string]string
}

func newColoLookup(locations []LocationEntry, countries map[string]string) *coloLookup {
	lookup := &coloLookup{
		cityColos:        make(map[string][]string),
		coloCountries:    make(map[string]string),
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
		lookup.coloCountries[colo] = country
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
		slices.Sort(colos)
	}
	for _, colos := range lookup.cityColos {
		slices.Sort(colos)
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

func (lookup *coloLookup) colosForCountry(colos []string, country string) []string {
	if country == "" {
		return colos
	}
	kept := make([]string, 0, len(colos))
	for _, colo := range colos {
		if known, ok := lookup.coloCountries[colo]; ok && known != "" && known != country {
			continue
		}
		kept = append(kept, colo)
	}
	return kept
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
	// A city name alone can point at a homonym in another country (Richmond BC ->
	// RIC, London ON -> LHR), so keep only the colos that sit in the geofeed
	// country; unknown countries stay because the dictionary has no answer.
	for _, city := range geofeedCityCandidates(country, region, entry.City) {
		if colos := lookup.colosForCountry(lookup.cityColos[normalizeName(city)], country); len(colos) > 0 {
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

var geofeedColoMap = map[string][]string{
	mappingKey("AU", "", "Sydney"):          {"SYD"},
	mappingKey("CA", "", "Toronto"):         {"YYZ"},
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
