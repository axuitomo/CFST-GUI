package colodict

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
)

func defaultUpdateHTTPClient() *http.Client {
	return httpclient.NewClient(httpclient.Options{
		DisableProxy: true,
		Profile:      httpcfg.Resolve("", "", "", "", false),
		Protocol:     httpclient.ProtocolTCP,
	})
}

// updateAltSourceTimeout bounds the optional alternate naming source download.
const updateAltSourceTimeout = 20 * time.Second

type updateSourceDownload struct {
	key     string
	label   string
	raw     []byte
	url     string
	err     error
	timeout time.Duration
}

func fetchUpdateSources(ctx context.Context, client *http.Client, urlCandidates func(string) []string, sourceURL, locationsURL, locationsAltURL, countryURL string) ([]byte, []byte, []byte, []byte, []string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	sources := []updateSourceDownload{
		{key: "geofeed", label: "GEOFEED", url: sourceURL},
		{key: "locations", label: "COLO locations", url: locationsURL},
		{key: "country", label: "country", url: countryURL},
	}
	if strings.TrimSpace(locationsAltURL) != "" {
		// A third-party mirror must not stall the whole update, so bound this one.
		sources = append(sources, updateSourceDownload{key: "locations_alt", label: "备用 COLO locations", url: locationsAltURL, timeout: updateAltSourceTimeout})
	}
	results := make(chan updateSourceDownload, len(sources))
	for _, source := range sources {
		source := source
		go func() {
			requestCtx := ctx
			if source.timeout > 0 {
				var cancel context.CancelFunc
				requestCtx, cancel = context.WithTimeout(ctx, source.timeout)
				defer cancel()
			}
			raw, err := fetchURL(requestCtx, client, urlCandidates, source.url, source.label)
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
	var locationAltRaw []byte
	var countryRaw []byte
	warnings := make([]string, 0, 2)
	for range sources {
		result := <-results
		if result.err != nil {
			if result.key == "geofeed" {
				return nil, nil, nil, nil, nil, result.err
			}
			warnings = append(warnings, fmt.Sprintf("%v，处理词典时会使用上次本地文件或内置映射。", result.err))
			continue
		}
		switch result.key {
		case "geofeed":
			geofeedRaw = result.raw
		case "locations":
			locationRaw = result.raw
		case "locations_alt":
			locationAltRaw = result.raw
		case "country":
			countryRaw = result.raw
		}
	}
	if (len(locationRaw) == 0 && len(locationAltRaw) == 0) || len(countryRaw) == 0 {
		warnings = append(warnings, "辅助 COLO 映射源未完整拉取；处理词典时会使用上次本地文件或内置映射。")
	}
	return geofeedRaw, locationRaw, locationAltRaw, countryRaw, dedupeWarnings(warnings), nil
}

// mergeLocationSources unions the primary and alternate COLO location lists so a
// geofeed city name matches whichever naming style the source uses (Ashburn vs
// Dulles). Records are deduped by country and normalized city, and the primary
// list wins when the two sources disagree.
func mergeLocationSources(primary, alt []byte) ([]byte, []string, error) {
	warnings := make([]string, 0, 2)
	primaryRecords, primaryErr := parseLocationRecords(primary)
	altRecords, altErr := parseLocationRecords(alt)
	if altErr != nil {
		warnings = append(warnings, fmt.Sprintf("备用 COLO locations 解析失败，已仅使用主源：%v", altErr))
		altRecords = nil
	}
	if primaryErr != nil {
		if len(bytes.TrimSpace(primary)) > 0 {
			// Keep the primary file untouched so Process reports its parse error as before.
			return primary, warnings, nil
		}
		primaryRecords = nil
	}

	merged := make([]map[string]any, 0, len(primaryRecords)+len(altRecords))
	seen := make(map[string]string, cap(merged))
	for _, records := range [][]map[string]any{primaryRecords, altRecords} {
		for _, record := range records {
			iata := normalizeColo(jsonStringField(record, "iata"))
			key := strings.ToUpper(jsonStringField(record, "cca2")) + "|" + normalizeName(jsonStringField(record, "city"))
			if previous, ok := seen[key]; ok {
				if previous != "" && iata != "" && previous != iata {
					warnings = append(warnings, fmt.Sprintf("COLO locations 命名冲突：%s 同时对应 %s 与 %s，已保留 %s。", key, previous, iata, previous))
				}
				continue
			}
			seen[key] = iata
			merged = append(merged, record)
		}
	}
	if len(merged) == 0 {
		return nil, warnings, nil
	}
	raw, err := json.MarshalIndent(merged, "", "    ")
	if err != nil {
		return nil, warnings, err
	}
	return append(raw, '\n'), warnings, nil
}

func parseLocationRecords(raw []byte) ([]map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var records []map[string]any
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func jsonStringField(record map[string]any, key string) string {
	value, _ := record[key].(string)
	return strings.TrimSpace(value)
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

func fetchURL(ctx context.Context, client *http.Client, urlCandidates func(string) []string, sourceURL, label string) ([]byte, error) {
	candidates := urlCandidates(sourceURL)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%s 缺少下载地址", label)
	}
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	attempts := make(chan updateSourceFetchAttempt, len(candidates))
	for _, candidate := range candidates {
		candidate := candidate
		go func() {
			raw, err := fetchURLCandidate(raceCtx, client, candidate, label)
			attempts <- updateSourceFetchAttempt{raw: raw, err: err}
		}()
	}

	errs := make([]error, 0, len(candidates))
	for range candidates {
		attempt := <-attempts
		if attempt.err != nil {
			errs = append(errs, attempt.err)
			continue
		}
		cancel()
		return attempt.raw, nil
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s 下载失败", label)
}

type updateSourceFetchAttempt struct {
	err error
	raw []byte
}

func fetchURLCandidate(ctx context.Context, client *http.Client, candidate, label string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("%s 更新返回状态 %s (%s)", label, response.Status, candidate)
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
