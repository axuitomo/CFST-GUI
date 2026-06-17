package appcore

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

type RemoteSourceAttempt struct {
	URL string
}

type SourceContentLoadOptions struct {
	BuildAttempts     func(primaryURL string, source Source) []RemoteSourceAttempt
	ContentCache      SourceContentCache
	URLCache          SourceURLCache
	ShouldRetry       func(statusCode int, err error) bool
	OnFallbackSuccess func(primaryURL string, used RemoteSourceAttempt, source Source) []string
}

func LoadSourceContent(source Source, cfg probecore.ProbeConfig, client *http.Client, opts SourceContentLoadOptions) (SourceContentResult, error) {
	cacheKey := SourceContentCacheKey(source)
	cacheKind := SourceKind(source)
	if opts.ContentCache != nil && cacheKey != "" {
		value, hit, err := opts.ContentCache.Load(cacheKey, func() (SourceContentCacheValue, error) {
			return loadSourceContentValue(source, cfg, client, opts)
		})
		return sourceContentResultFromValue(source, value, hit, cacheKey, cacheKind, opts), err
	}
	value, err := loadSourceContentValue(source, cfg, client, opts)
	return sourceContentResultFromValue(source, value, false, cacheKey, cacheKind, opts), err
}

func loadSourceContentValue(source Source, cfg probecore.ProbeConfig, client *http.Client, opts SourceContentLoadOptions) (SourceContentCacheValue, error) {
	switch SourceKind(source) {
	case "inline":
		return SourceContentCacheValue{Raw: strings.TrimSpace(source.Content)}, nil
	case "file":
		path := strings.TrimSpace(source.Path)
		if path == "" {
			return SourceContentCacheValue{}, errors.New("缺少文件路径")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return SourceContentCacheValue{}, err
		}
		return SourceContentCacheValue{Raw: string(raw)}, nil
	default:
		return loadRemoteSourceContent(source, cfg, client, opts)
	}
}

func loadRemoteSourceContent(source Source, cfg probecore.ProbeConfig, client *http.Client, opts SourceContentLoadOptions) (SourceContentCacheValue, error) {
	primaryURL, err := NormalizeSourceURLInput(source.URL)
	if err != nil {
		return SourceContentCacheValue{}, err
	}
	attempts := []RemoteSourceAttempt{{URL: primaryURL}}
	if opts.BuildAttempts != nil {
		if built := opts.BuildAttempts(primaryURL, source); len(built) > 0 {
			attempts = built
		}
	}

	var firstErr error
	for index, attempt := range attempts {
		fetchResult, err := FetchSourceURLWithOptions(attempt.URL, cfg, client, SourceURLFetchOptions{Cache: opts.URLCache})
		statusCode := fetchResult.StatusCode
		if err == nil {
			return SourceContentCacheValue{
				Raw:                  fetchResult.Raw,
				ConditionalHit:       fetchResult.ConditionalHit,
				PersistentCacheHit:   fetchResult.PersistentCacheHit,
				PersistentCacheWrite: fetchResult.PersistentCacheWrite,
				StatusCode:           statusCode,
				UsedURL:              attempt.URL,
			}, nil
		}
		if index == 0 {
			firstErr = err
		}
		if opts.ShouldRetry == nil || !opts.ShouldRetry(statusCode, err) {
			return SourceContentCacheValue{}, err
		}
	}

	if firstErr != nil {
		return SourceContentCacheValue{}, firstErr
	}
	return SourceContentCacheValue{}, errors.New("远程来源读取失败")
}

func sourceContentResultFromValue(source Source, value SourceContentCacheValue, cacheHit bool, cacheKey string, cacheKind string, opts SourceContentLoadOptions) SourceContentResult {
	result := SourceContentResult{
		Raw: value.Raw,
		Diagnostics: SourceContentDiagnostics{
			CacheHit:             cacheHit,
			CacheKey:             strings.TrimSpace(cacheKey),
			CacheKind:            strings.TrimSpace(cacheKind),
			ConditionalHit:       value.ConditionalHit,
			PersistentCacheHit:   value.PersistentCacheHit,
			PersistentCacheWrite: value.PersistentCacheWrite,
			StatusCode:           value.StatusCode,
			UsedURL:              strings.TrimSpace(value.UsedURL),
		},
	}
	if SourceKind(source) != "url" {
		return result
	}
	primaryURL, err := NormalizeSourceURLInput(source.URL)
	if err != nil {
		return result
	}
	if value.UsedURL != "" && value.UsedURL != primaryURL && opts.OnFallbackSuccess != nil {
		result.Warnings = append(result.Warnings, opts.OnFallbackSuccess(primaryURL, RemoteSourceAttempt{URL: value.UsedURL}, source)...)
	}
	result.Warnings = probecore.DedupeStrings(result.Warnings)
	return result
}
