package appcore

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/doh"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

// defaultDoHTestHost 是连通性测试的默认域名。DoH 的真实职责是解析输入源域名和
// 来源 URL 主机名，所以测试也解析一个真实主机，而不只看端点有没有回 200。
const defaultDoHTestHost = "cloudflare.com"

const dohTestTimeout = 8 * time.Second

// dohResolver 只暴露连通性测试用到的解析能力，*doh.Resolver 实现它。
// 抽成接口是为了让单测不依赖真实 DNS 查询。
type dohResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
	ECHConfig(ctx context.Context, host string) ([]byte, error)
}

type dohTestRequest struct {
	Endpoint  string
	Host      string
	ECH       bool
	ECHDomain string
	ECHConfig []byte
}

func (s *Service) invokeDoHTest(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	request, warnings := dohTestRequestFromPayload(payload)
	resolver, err := doh.New(request.Endpoint)
	if err != nil {
		return NewCommandResult("DOH_TEST_INVALID", nil, err.Error(), false, nil, warnings)
	}
	// 端点自身失败时不能回退系统 DNS，否则一个坏端点会被测成可用。
	resolver.Fallback = func(context.Context, string) ([]net.IPAddr, error) {
		return nil, errors.New("DoH 端点没有返回有效答案")
	}
	ctx, cancel := context.WithTimeout(context.Background(), dohTestTimeout)
	defer cancel()
	data, code, message, ok := runDoHTest(ctx, resolver, request)
	return NewCommandResult(code, data, message, ok, nil, warnings)
}

// dohTestRequestFromPayload 复用与探测配置相同的归一化规则（默认端点、默认 ECH
// 域名、base64 校验），非法值只降级并给出警告，与配置保存时的行为保持一致。
func dohTestRequestFromPayload(payload map[string]any) (dohTestRequest, []string) {
	var warnings []string
	endpoint := strings.TrimSpace(stringValue(firstNonNil(payload["doh_url"], payload["dohUrl"], payload["endpoint"]), ""))
	if endpoint == "" {
		endpoint = probecore.DefaultDoHURL
	}
	host := strings.TrimSuffix(strings.TrimSpace(stringValue(firstNonNil(payload["host"], payload["test_host"], payload["testHost"]), "")), ".")
	if host == "" {
		host = defaultDoHTestHost
	}
	var config []byte
	if raw := strings.TrimSpace(stringValue(firstNonNil(payload["ech_config"], payload["echConfig"]), "")); raw != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(decoded) == 0 {
			warnings = append(warnings, "ECH 配置不是有效的 base64 ECHConfigList；已改为从 DoH 的 HTTPS RR 自动获取。")
		} else {
			config = decoded
		}
	}
	domainValue := strings.TrimSpace(stringValue(firstNonNil(payload["ech_domain"], payload["echDomain"]), ""))
	domain := probecore.NormalizeECHDomain(domainValue)
	if domain == "" {
		if domainValue != "" {
			warnings = append(warnings, fmt.Sprintf("ECH 域名 %q 无效，必须是裸域名（不带协议和路径）；已改用 %s。", domainValue, probecore.DefaultECHDomain))
		}
		domain = probecore.DefaultECHDomain
	}
	return dohTestRequest{
		Endpoint:  endpoint,
		Host:      host,
		ECH:       boolValue(firstNonNil(payload["ech"], payload["ECH"]), true),
		ECHDomain: domain,
		ECHConfig: config,
	}, warnings
}

func runDoHTest(ctx context.Context, resolver dohResolver, request dohTestRequest) (map[string]any, string, string, bool) {
	ipv4 := []string{}
	ipv6 := []string{}
	data := map[string]any{
		"endpoint":    request.Endpoint,
		"host":        request.Host,
		"ech_enabled": request.ECH,
		"ech_domain":  request.ECHDomain,
	}
	notFound := false
	addrs, err := resolver.LookupIPAddr(ctx, request.Host)
	if err != nil {
		var dnsErr *net.DNSError
		if !errors.As(err, &dnsErr) || !dnsErr.IsNotFound {
			data["ipv4"], data["ipv6"] = ipv4, ipv6
			return data, "DOH_TEST_FAILED", fmt.Sprintf("DoH 端点测试失败：%v", err), false
		}
		// 有效 NXDOMAIN 证明端点工作正常，只是这个域名本身没有记录。
		notFound = true
	}
	for _, addr := range addrs {
		if ip := addr.IP.To4(); ip != nil {
			ipv4 = append(ipv4, ip.String())
			continue
		}
		ipv6 = append(ipv6, addr.IP.String())
	}
	data["ipv4"], data["ipv6"] = ipv4, ipv6

	source, configBytes, note := resolveECHForTest(ctx, resolver, request)
	data["ech_source"], data["ech_config_bytes"] = source, configBytes
	if note != "" {
		data["ech_note"] = note
	}

	if notFound {
		return data, "DOH_TEST_OK", fmt.Sprintf("DoH 可用，但 %s 没有解析结果。%s", request.Host, note), true
	}
	message := fmt.Sprintf("DoH 可用：%s 解析出 %d 个 IPv4、%d 个 IPv6。", request.Host, len(ipv4), len(ipv6))
	if request.ECH {
		switch source {
		case "manual":
			message += fmt.Sprintf(" 使用手填 ECH 配置（%d 字节）。", configBytes)
		case "host":
			message += fmt.Sprintf(" ECH 配置来自 %s（%d 字节）。", request.Host, configBytes)
		case "domain":
			message += fmt.Sprintf(" ECH 配置来自 %s（%d 字节）。", request.ECHDomain, configBytes)
		default:
			message += " 未取到 ECH 配置：" + note
		}
	}
	return data, "DOH_TEST_OK", message, true
}

// resolveECHForTest 复刻来源请求的 ECH 配置挑选顺序：手填优先，其次来源主机自己
// 的 HTTPS RR，最后回退到 ECH 域名。返回来源、字节数和取不到时的说明。
func resolveECHForTest(ctx context.Context, resolver dohResolver, request dohTestRequest) (string, int, string) {
	if !request.ECH {
		return "none", 0, ""
	}
	if len(request.ECHConfig) > 0 {
		return "manual", len(request.ECHConfig), ""
	}
	if list, err := resolver.ECHConfig(ctx, request.Host); err == nil && len(list) > 0 {
		return "host", len(list), ""
	}
	if !strings.EqualFold(request.ECHDomain, request.Host) {
		if list, err := resolver.ECHConfig(ctx, request.ECHDomain); err == nil && len(list) > 0 {
			return "domain", len(list), ""
		}
	}
	return "none", 0, fmt.Sprintf("%s 和 %s 都没有发布 ech 记录。", request.Host, request.ECHDomain)
}
