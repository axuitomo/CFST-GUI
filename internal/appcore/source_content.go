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
	ShouldRetry       func(statusCode int, err error) bool
	OnFallbackSuccess func(primaryURL string, used RemoteSourceAttempt, source Source) []string
}

func LoadSourceContent(source Source, cfg probecore.ProbeConfig, client *http.Client, opts SourceContentLoadOptions) (SourceContentResult, error) {
	switch SourceKind(source) {
	case "inline":
		return SourceContentResult{Raw: strings.TrimSpace(source.Content)}, nil
	case "file":
		path := strings.TrimSpace(source.Path)
		if path == "" {
			return SourceContentResult{}, errors.New("缺少文件路径")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return SourceContentResult{}, err
		}
		return SourceContentResult{Raw: string(raw)}, nil
	default:
		return loadRemoteSourceContent(source, cfg, client, opts)
	}
}

func loadRemoteSourceContent(source Source, cfg probecore.ProbeConfig, client *http.Client, opts SourceContentLoadOptions) (SourceContentResult, error) {
	primaryURL, err := NormalizeSourceURLInput(source.URL)
	if err != nil {
		return SourceContentResult{}, err
	}
	attempts := []RemoteSourceAttempt{{URL: primaryURL}}
	if opts.BuildAttempts != nil {
		if built := opts.BuildAttempts(primaryURL, source); len(built) > 0 {
			attempts = built
		}
	}

	var firstErr error
	for index, attempt := range attempts {
		raw, statusCode, err := FetchSourceURL(attempt.URL, cfg, client)
		if err == nil {
			result := SourceContentResult{Raw: raw}
			if index > 0 && opts.OnFallbackSuccess != nil {
				result.Warnings = append(result.Warnings, opts.OnFallbackSuccess(primaryURL, attempt, source)...)
			}
			result.Warnings = probecore.DedupeStrings(result.Warnings)
			return result, nil
		}
		if index == 0 {
			firstErr = err
		}
		if opts.ShouldRetry == nil || !opts.ShouldRetry(statusCode, err) {
			return SourceContentResult{}, err
		}
	}

	if firstErr != nil {
		return SourceContentResult{}, firstErr
	}
	return SourceContentResult{}, errors.New("远程来源读取失败")
}
