package task

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
)

const DefaultHTTPingStatusCode = http.StatusOK

var (
	regexpColoIATACode    = regexp.MustCompile(`[A-Z]{3}`)
	regexpColoCountryCode = regexp.MustCompile(`[A-Z]{2}`)
	regexpColoGcore       = regexp.MustCompile(`^[a-z]{2}`)
)

// pingReceived pingTotalTime
func (p *Ping) httping(ip *net.IPAddr) (int, time.Duration, string) {
	engine := p.probeEngine()
	config := engine.config
	profile := engine.currentRequestProfile()
	hc := httpclient.NewClient(httpclient.Options{
		Profile:               profile,
		DialContext:           httpclient.DirectDialContext(ip, config.TCPPort, profile),
		DialAddress:           profile.DialAddress(ip, config.TCPPort),
		DisableProxy:          true,
		Timeout:               time.Second * 2,
		ResponseHeaderTimeout: time.Second * 2,
		TLSHandshakeTimeout:   config.TCPConnectTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 阻止重定向
		},
	})
	defer hc.CloseIdleConnections()

	// 先访问一次获得 HTTP 状态码 及 地区码
	var colo string
	{
		statusCode, _, header, err := engine.httpingRequest(hc, profile, false)
		if err != nil {
			engine.debugEvent("stage.reject", map[string]any{
				"error": err.Error(),
				"head": map[string]any{
					"url": config.URL,
				},
				"ip":      ip.String(),
				"message": "HTTPing 延迟测速请求失败，淘汰该 IP。",
				"reason":  "httping_error",
				"stage":   "stage1_tcp",
			})
			return 0, 0, ""
		}
		if !engine.isAcceptedHTTPingStatusCode(statusCode) {
			engine.debugEvent("stage.reject", map[string]any{
				"head": map[string]any{
					"accepted_status_code": config.HttpingStatusCode,
					"status_code":          statusCode,
					"url":                  config.URL,
				},
				"ip":      ip.String(),
				"message": "HTTPing 状态码不匹配，淘汰该 IP。",
				"reason":  "status_mismatch",
				"stage":   "stage1_tcp",
			})
			return 0, 0, ""
		}

		// 通过头部参数获取地区码
		colo = getHeaderColo(header)

		// 只有指定了地区才匹配机场地区码
		if config.HttpingCFColo != "" {
			// 判断是否匹配指定的地区码
			originalColo := colo
			filteredColo, allowed := engine.configuredColoAllowed(colo)
			colo = filteredColo
			if !allowed { // 没有匹配到地区码或不符合指定地区则直接结束该 IP 测试
				engine.debugEvent("stage.reject", map[string]any{
					"colo": originalColo,
					"head": map[string]any{
						"expected_colo": config.HttpingCFColo,
					},
					"ip":      ip.String(),
					"message": "HTTPing 地区码不匹配，淘汰该 IP。",
					"reason":  "colo_filter",
					"stage":   "stage1_tcp",
				})
				return 0, 0, ""
			}
		}
	}

	// 循环测速计算延迟
	if config.SkipFirstLatencySample {
		_, _, _, _ = engine.httpingRequest(hc, profile, false)
	}

	success := 0
	var delay time.Duration
	for i := 0; i < config.PingTimes; i++ {
		_, duration, _, err := engine.httpingRequest(hc, profile, i == config.PingTimes-1)
		if err != nil {
			continue
		}
		success++
		delay += duration
	}

	return success, delay, colo
}

func (e *Engine) httpingRequest(hc *http.Client, profile httpcfg.Profile, closeConnection bool) (int, time.Duration, http.Header, error) {
	request, err := http.NewRequest(http.MethodHead, e.config.URL, nil)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("创建 HTTPing 请求失败: %w", err)
	}
	profile.Apply(request)
	if closeConnection {
		request.Header.Set("Connection", "close")
		request.Close = true
	}
	startTime := time.Now()
	response, err := hc.Do(request)
	if err != nil {
		return 0, 0, nil, err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, response.Body)
	return response.StatusCode, time.Since(startTime), response.Header.Clone(), nil
}

func (e *Engine) isAcceptedHTTPingStatusCode(statusCode int) bool {
	expectedStatusCode := e.config.HttpingStatusCode
	if expectedStatusCode < 100 || expectedStatusCode > 599 {
		expectedStatusCode = 0
	}
	if expectedStatusCode == 0 {
		return true
	}
	return statusCode == expectedStatusCode
}

func (e *Engine) configuredColoAllowed(colo string) (string, bool) {
	if strings.TrimSpace(e.config.HttpingCFColo) == "" && len(e.config.HttpingCFColos) == 0 {
		return colo, true
	}
	mode := NormalizeColoFilterMode(e.config.HttpingCFColoMode)
	colo = normalizeColoCode(colo)
	if colo == "" {
		return "", mode == ColoFilterModeDeny
	}
	configured := e.config.HttpingCFColos
	if len(configured) == 0 {
		configured = ParseColoAllowList(e.config.HttpingCFColo)
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

// 处理地区码
func (p *Ping) filterColo(colo string) string {
	filtered, ok := p.probeEngine().configuredColoAllowed(colo)
	if ok {
		return filtered
	}
	return ""
}
