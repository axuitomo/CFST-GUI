package colodict

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseGeofeedAndBuildColoEntries(t *testing.T) {
	raw := []byte(strings.Join([]string{
		"ip_prefix,country,region,city,postal_code",
		"104.16.0.0/13,US,CA,San Jose,95113",
		"104.22.1.0/24,US,US-GA,Atlanta,",
		"104.22.2.0/24,US,US-CA,San Jose,",
		"104.22.3.0/24,JP,JP-12,Narita,",
		"104.24.0.0/14,JP,,Tokyo,",
		"bad,US,CA,Los Angeles,90001",
		"104.28.0.0/15,ZZ,,Unknown City,",
		"",
	}, "\n"))

	entries, invalid, err := ParseGeofeed(raw)
	if err != nil {
		t.Fatalf("ParseGeofeed returned error: %v", err)
	}
	if invalid != 1 {
		t.Fatalf("invalid = %d, want 1", invalid)
	}
	if len(entries) != 6 {
		t.Fatalf("entries = %d, want 6", len(entries))
	}

	coloEntries, unmatched := BuildColoEntries(entries)
	if unmatched != 1 {
		t.Fatalf("unmatched = %d, want 1", unmatched)
	}
	got := make([]string, 0, len(coloEntries))
	for _, entry := range coloEntries {
		got = append(got, entry.Prefix.String()+"|"+entry.Colo)
	}
	want := []string{
		"104.16.0.0/13|SJC",
		"104.22.1.0/24|ATL",
		"104.22.2.0/24|SJC",
		"104.22.3.0/24|NRT",
		"104.24.0.0/14|NRT",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("colo entries = %#v, want %#v", got, want)
	}
}

func TestColoEntriesRoundTripAndFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ColoFileName)
	raw, err := EncodeColoEntries([]ColoEntry{
		mustColoEntryForTest(t, "104.16.0.0/14", "SJC", "US", "CA", "San Jose"),
		mustColoEntryForTest(t, "104.20.0.0/14", "LAX", "US", "CA", "Los Angeles"),
		mustColoEntryForTest(t, "2400:cb00::/32", "NRT", "JP", "", "Tokyo"),
	})
	if err != nil {
		t.Fatalf("EncodeColoEntries returned error: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}

	entries, err := LoadColoEntries(path)
	if err != nil {
		t.Fatalf("LoadColoEntries returned error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("loaded entries = %d, want 3", len(entries))
	}

	filter, err := NewFilter(path, "sjc,nrt")
	if err != nil {
		t.Fatalf("NewFilter returned error: %v", err)
	}
	for _, tc := range []struct {
		name  string
		token string
		want  []string
	}{
		{name: "single ip allowed", token: "104.16.1.1", want: []string{"104.16.1.1"}},
		{name: "single ip blocked", token: "104.20.1.1", want: nil},
		{name: "cidr intersects allowed prefix", token: "104.0.0.0/8", want: []string{"104.16.0.0/14"}},
		{name: "ipv6 intersects allowed prefix", token: "2400:cb00::/31", want: []string{"2400:cb00::/32"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := filter.FilterToken(tc.token)
			if len(got) != len(tc.want) || (len(got) > 0 && !reflect.DeepEqual(got, tc.want)) {
				t.Fatalf("FilterToken(%q) = %#v, want %#v", tc.token, got, tc.want)
			}
		})
	}
}

func TestUpdateDownloadsRawDictionaryFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		switch r.URL.Path {
		case "/geofeed":
			_, _ = w.Write([]byte(strings.Join([]string{
				"ip_prefix,country,region,city,postal_code",
				"104.16.0.0/13,United States,US-CA,San Jose,95113",
				"104.22.1.0/24,US,US-GA,Atlanta,",
				"104.22.2.0/24,JP,JP-12,Narita,",
				"104.22.3.0/24,GB,GB-HNS,Hounslow,",
				"203.0.113.0/24,Exampleland,,Reference City,",
				"104.28.0.0/15,ZZ,,Unknown City,",
			}, "\n")))
		case "/locations":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"cca2":"US","city":"San Jose","iata":"SJC","region":"North America"},
				{"cca2":"US","city":"Atlanta","iata":"ATL","region":"North America"},
				{"cca2":"JP","city":"Tokyo","iata":"NRT","region":"Asia Pacific"},
				{"cca2":"GB","city":"London","iata":"LHR","region":"Europe"},
				{"cca2":"EX","city":"Reference City","iata":"TST","region":"Test"}
			]`))
		case "/countries":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"US":"United States","JP":"Japan","GB":"United Kingdom","EX":"Exampleland"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	result, err := Update(context.Background(), UpdateOptions{
		Client:       server.Client(),
		Paths:        DefaultPaths(dir),
		SourceURL:    server.URL + "/geofeed",
		LocationsURL: server.URL + "/locations",
		CountryURL:   server.URL + "/countries",
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if result.GeofeedRows != 6 || result.ColoRows != 0 || result.UnmatchedRows != 6 {
		t.Fatalf("status = %#v, want only raw geofeed updated", result.Status)
	}
	for _, name := range []string{GeofeedFileName, LocationsFileName, CountryFileName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("raw dictionary file %s missing: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ColoFileName)); !os.IsNotExist(err) {
		t.Fatalf("colo file stat error = %v, want not exist before processing", err)
	}
}

func TestProcessUsesLocalDictionaryFilesAndWritesDerivedColoFile(t *testing.T) {
	dir := t.TempDir()
	paths := DefaultPaths(dir)
	if err := os.WriteFile(paths.Geofeed, []byte(strings.Join([]string{
		"ip_prefix,country,region,city,postal_code",
		"104.16.0.0/13,United States,US-CA,San Jose,95113",
		"104.22.1.0/24,US,US-GA,Atlanta,",
		"104.22.2.0/24,JP,JP-12,Narita,",
		"104.22.3.0/24,GB,GB-HNS,Hounslow,",
		"203.0.113.0/24,Exampleland,,Reference City,",
		"104.28.0.0/15,ZZ,,Unknown City,",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("geofeed file missing: %v", err)
	}
	if err := os.WriteFile(paths.Locations, []byte(`[
		{"cca2":"US","city":"San Jose","iata":"SJC","region":"North America"},
		{"cca2":"US","city":"Atlanta","iata":"ATL","region":"North America"},
		{"cca2":"JP","city":"Tokyo","iata":"NRT","region":"Asia Pacific"},
		{"cca2":"GB","city":"London","iata":"LHR","region":"Europe"},
		{"cca2":"EX","city":"Reference City","iata":"TST","region":"Test"}
	]`), 0o600); err != nil {
		t.Fatalf("write locations file: %v", err)
	}
	if err := os.WriteFile(paths.Country, []byte(`{"US":"United States","JP":"Japan","GB":"United Kingdom","EX":"Exampleland"}`), 0o600); err != nil {
		t.Fatalf("write country file: %v", err)
	}

	result, err := Process(UpdateOptions{Paths: paths})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.GeofeedRows != 6 || result.ColoRows != 5 || result.UnmatchedRows != 1 {
		t.Fatalf("status = %#v, want geofeed=6 colo=5 unmatched=1", result.Status)
	}
	entries, err := LoadColoEntries(paths.Colo)
	if err != nil {
		t.Fatalf("LoadColoEntries returned error: %v", err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Prefix.String()+"|"+entry.Colo)
	}
	want := []string{
		"104.16.0.0/13|SJC",
		"104.22.1.0/24|ATL",
		"104.22.2.0/24|NRT",
		"104.22.3.0/24|LHR",
		"203.0.113.0/24|TST",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %#v, want %#v", got, want)
	}
}

func TestUpdateDownloadsReferenceSourcesConcurrently(t *testing.T) {
	var mu sync.Mutex
	seen := make(map[string]bool)
	var allStartedOnce sync.Once
	allStarted := make(chan struct{})
	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/geofeed", "/locations", "/countries":
			mu.Lock()
			seen[r.URL.Path] = true
			if len(seen) == 3 {
				allStartedOnce.Do(func() { close(allStarted) })
			}
			mu.Unlock()
			<-release
		default:
			http.NotFound(w, r)
			return
		}

		switch r.URL.Path {
		case "/geofeed":
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte(strings.Join([]string{
				"ip_prefix,country,region,city,postal_code",
				"104.16.0.0/13,US,CA,San Jose,95113",
			}, "\n")))
		case "/locations":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"cca2":"US","city":"San Jose","iata":"SJC","region":"North America"}]`))
		case "/countries":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"US":"United States"}`))
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	errCh := make(chan error, 1)
	go func() {
		_, err := Update(context.Background(), UpdateOptions{
			Client:       server.Client(),
			Paths:        DefaultPaths(dir),
			SourceURL:    server.URL + "/geofeed",
			LocationsURL: server.URL + "/locations",
			CountryURL:   server.URL + "/countries",
		})
		errCh <- err
	}()

	select {
	case <-allStarted:
		close(release)
	case <-time.After(500 * time.Millisecond):
		close(release)
		if err := <-errCh; err != nil {
			t.Fatalf("Update returned error after non-concurrent fetch: %v", err)
		}
		t.Fatal("Update did not start all three downloads concurrently")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
}

func TestDefaultUpdateHTTPClientHasNoFixedTimeout(t *testing.T) {
	if client := defaultUpdateHTTPClient(); client == nil || client.Timeout != 0 {
		t.Fatalf("default update HTTP client timeout = %v, want no fixed timeout", client.Timeout)
	}
}

func TestUpdateKeepsRawGeofeedWhenReferenceFetchFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/geofeed":
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte(strings.Join([]string{
				"ip_prefix,country,region,city,postal_code",
				"104.16.0.0/13,US,CA,San Jose,95113",
			}, "\n")))
		case "/locations":
			http.Error(w, "no locations", http.StatusBadGateway)
		case "/countries":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"US":"United States"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	result, err := Update(context.Background(), UpdateOptions{
		Client:       server.Client(),
		Paths:        DefaultPaths(dir),
		SourceURL:    server.URL + "/geofeed",
		LocationsURL: server.URL + "/locations",
		CountryURL:   server.URL + "/countries",
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if result.GeofeedRows != 1 || result.ColoRows != 0 {
		t.Fatalf("status = %#v, want raw geofeed without derived COLO", result.Status)
	}
	if !warningsContainForTest(result.Warnings, "COLO locations 下载失败") {
		t.Fatalf("warnings = %#v, want reference fetch warning", result.Warnings)
	}
	if _, err := os.Stat(filepath.Join(dir, GeofeedFileName)); err != nil {
		t.Fatalf("geofeed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ColoFileName)); !os.IsNotExist(err) {
		t.Fatalf("colo file stat error = %v, want not exist before processing", err)
	}
}

func TestProcessFallsBackToBuiltInMappingWhenLocalReferenceFilesAreMissing(t *testing.T) {
	paths := DefaultPaths(t.TempDir())
	if err := os.WriteFile(paths.Geofeed, []byte(strings.Join([]string{
		"ip_prefix,country,region,city,postal_code",
		"104.16.0.0/13,US,CA,San Jose,95113",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write geofeed: %v", err)
	}

	result, err := Process(UpdateOptions{Paths: paths})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.ColoRows != 1 {
		t.Fatalf("ColoRows = %d, want built-in fallback mapping", result.ColoRows)
	}
	if !warningsContainForTest(result.Warnings, "本地 COLO locations 不可用") {
		t.Fatalf("warnings = %#v, want local reference warning", result.Warnings)
	}
}

func TestStatusForPathsRecomputesUnmatchedRowsFromGeofeed(t *testing.T) {
	dir := t.TempDir()
	paths := DefaultPaths(dir)
	geofeed := strings.Join([]string{
		"ip_prefix,country,region,city,postal_code",
		"104.16.0.0/13,US,CA,San Jose,95113",
		"104.28.0.0/15,ZZ,,Unknown City,",
	}, "\n")
	if err := os.WriteFile(paths.Geofeed, []byte(geofeed), 0o600); err != nil {
		t.Fatalf("write geofeed: %v", err)
	}
	coloRaw, err := EncodeColoEntries([]ColoEntry{
		mustColoEntryForTest(t, "104.16.0.0/13", "SJC", "US", "CA", "San Jose"),
		mustColoEntryForTest(t, "104.16.0.0/13", "SFO", "US", "CA", "San Jose"),
	})
	if err != nil {
		t.Fatalf("EncodeColoEntries returned error: %v", err)
	}
	if err := os.WriteFile(paths.Colo, coloRaw, 0o600); err != nil {
		t.Fatalf("write colo: %v", err)
	}

	status, err := StatusForPaths(paths)
	if err != nil {
		t.Fatalf("StatusForPaths returned error: %v", err)
	}
	if status.GeofeedRows != 2 || status.ColoRows != 2 || status.MatchedRows != 1 || status.UnmatchedRows != 1 {
		t.Fatalf("status = %#v, want geofeed=2 colo=2 matched geofeed rows=1 unmatched=1", status)
	}
}

func mustColoEntryForTest(t *testing.T, prefix, colo, country, region, city string) ColoEntry {
	t.Helper()
	parsed, err := netip.ParsePrefix(prefix)
	if err != nil {
		t.Fatalf("ParsePrefix(%q): %v", prefix, err)
	}
	return ColoEntry{
		Prefix:  parsed.Masked(),
		Colo:    colo,
		Country: country,
		Region:  region,
		City:    city,
	}
}

func warningsContainForTest(warnings []string, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}
