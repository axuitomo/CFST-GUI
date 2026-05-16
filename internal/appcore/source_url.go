package appcore

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func NormalizeSourceURLInput(rawURL string) (string, error) {
	value := probecore.NormalizeProbeURLInput(rawURL)
	if value == "" {
		return "", errors.New("缺少远程 URL")
	}
	if strings.HasPrefix(value, "//") {
		value = "https:" + value
	} else if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("远程 URL 必须包含有效主机")
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("远程 URL 仅支持 http/https：%s", parsed.Scheme)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	return parsed.String(), nil
}
