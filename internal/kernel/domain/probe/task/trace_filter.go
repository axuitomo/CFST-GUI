package task

import (
	"net/http"
	"regexp"
	"strings"
)

// DefaultTraceStatusCode 是追踪阶段默认接受的 HTTP 状态码。
const DefaultTraceStatusCode = http.StatusOK

var (
	regexpColoIATACode    = regexp.MustCompile(`[A-Z]{3}`)
	regexpColoCountryCode = regexp.MustCompile(`[A-Z]{2}`)
	regexpColoGcore       = regexp.MustCompile(`^[a-z]{2}`)
)

func (e *Engine) isAcceptedTraceStatusCode(statusCode int) bool {
	expectedStatusCode := e.config.TraceStatusCode
	if expectedStatusCode < 100 || expectedStatusCode > 599 {
		expectedStatusCode = 0
	}
	if expectedStatusCode == 0 {
		return true
	}
	return statusCode == expectedStatusCode
}

func (e *Engine) configuredColoAllowed(colo string) (string, bool) {
	if strings.TrimSpace(e.config.TraceCFColo) == "" && len(e.config.TraceCFColos) == 0 {
		return colo, true
	}
	mode := NormalizeColoFilterMode(e.config.TraceCFColoMode)
	colo = normalizeColoCode(colo)
	if colo == "" {
		return "", mode == ColoFilterModeDeny
	}
	configured := e.config.TraceCFColos
	if len(configured) == 0 {
		configured = ParseColoAllowList(e.config.TraceCFColo)
	}
	matched := false
	for _, expected := range configured {
		if normalizeColoCode(expected) == colo {
			matched = true
			break
		}
	}
	if mode == ColoFilterModeDeny {
		if matched {
			return "", false
		}
		return colo, true
	}
	if matched {
		return colo, true
	}
	return "", false
}

// 从响应头中获取 地区码 值
func getHeaderColo(header http.Header) (colo string) {
	return ExtractColo(header, nil)
}
