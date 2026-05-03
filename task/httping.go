package task

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/internal/httpcfg"
	"github.com/XIU2/CloudflareSpeedTest/utils"
)

var (
	Httping               bool
	HttpingStatusCode     int
	HttpingCFColo         string
	HttpingCFColomap      *sync.Map
	RegexpColoIATACode    = regexp.MustCompile(`[A-Z]{3}`)  // 匹配 IATA 机场地区码（俗称 机场三字码）的正则表达式
	RegexpColoCountryCode = regexp.MustCompile(`[A-Z]{2}`)  // 匹配国家地区码的正则表达式（如 US、CN、UK 等）
	RegexpColoGcore       = regexp.MustCompile(`^[a-z]{2}`) // 匹配城市地区码的正则表达式（小写，如 us、cn、uk 等）
)

// pingReceived pingTotalTime
func (p *Ping) httping(ip *net.IPAddr) (int, time.Duration, string) {
	profile := currentRequestProfile()
	tlsConfig := &tls.Config{InsecureSkipVerify: profile.InsecureSkipVerify}
	if profile.HasCustomSNI() {
		tlsConfig.ServerName = profile.SNI
	}
	hc := http.Client{
		Timeout: time.Second * 2,
		Transport: &http.Transport{
			DialContext:     getDialContext(ip, profile),
			TLSClientConfig: tlsConfig,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 阻止重定向
		},
	}

	// 先访问一次获得 HTTP 状态码 及 地区码
	var colo string
	{
		statusCode, _, header, err := httpingRequest(&hc, profile, false)
		if err != nil {
			utils.DebugEvent("stage.reject", map[string]any{
				"error": err.Error(),
				"head": map[string]any{
					"url": URL,
				},
				"ip":      ip.String(),
				"message": "HTTPing 延迟测速请求失败，淘汰该 IP。",
				"reason":  "httping_error",
				"stage":   "stage1_tcp",
			})
			return 0, 0, ""
		}
		if !isAcceptedHTTPingStatusCode(statusCode) {
			utils.DebugEvent("stage.reject", map[string]any{
				"head": map[string]any{
					"accepted_status_code": HttpingStatusCode,
					"status_code":          statusCode,
					"url":                  URL,
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
		if HttpingCFColo != "" {
			// 判断是否匹配指定的地区码
			originalColo := colo
			colo = p.filterColo(colo)
			if colo == "" { // 没有匹配到地区码或不符合指定地区则直接结束该 IP 测试
				utils.DebugEvent("stage.reject", map[string]any{
					"colo": originalColo,
					"head": map[string]any{
						"expected_colo": HttpingCFColo,
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
	if SkipFirstLatencySample {
		_, _, _, _ = httpingRequest(&hc, profile, false)
	}

	success := 0
	var delay time.Duration
	for i := 0; i < PingTimes; i++ {
		_, duration, _, err := httpingRequest(&hc, profile, i == PingTimes-1)
		if err != nil {
			continue
		}
		success++
		delay += duration
	}

	return success, delay, colo
}

func httpingRequest(hc *http.Client, profile httpcfg.Profile, closeConnection bool) (int, time.Duration, http.Header, error) {
	request, err := http.NewRequest(http.MethodHead, URL, nil)
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

func isAcceptedHTTPingStatusCode(statusCode int) bool {
	expectedStatusCode := HttpingStatusCode
	if expectedStatusCode < 100 || expectedStatusCode > 599 {
		expectedStatusCode = 0
	}
	if expectedStatusCode == 0 {
		return statusCode == 200 || statusCode == 301 || statusCode == 302
	}
	return statusCode == expectedStatusCode
}

func MapColoMap() *sync.Map {
	if HttpingCFColo == "" {
		return nil
	}
	colos := ParseColoAllowList(HttpingCFColo)
	if len(colos) == 0 {
		return nil
	}
	colomap := &sync.Map{}
	for _, colo := range colos {
		colomap.Store(colo, colo)
	}
	return colomap
}

// 从响应头中获取 地区码 值
func getHeaderColo(header http.Header) (colo string) {
	return ExtractColo(header, nil)
}

// 处理地区码
func (p *Ping) filterColo(colo string) string {
	if colo == "" {
		return ""
	}
	// 如果没有指定 -cfcolo 参数，则直接返回
	if HttpingCFColomap == nil {
		return colo
	}
	// 匹配 机场地区码 是否为指定的地区
	_, ok := HttpingCFColomap.Load(colo)
	if ok {
		return colo
	}
	return ""
}
