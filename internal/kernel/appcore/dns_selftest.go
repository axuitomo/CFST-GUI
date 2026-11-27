package appcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/notify"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspush"
)

const (
	dnsSelfTestPrefix       = "cfst-selftest"
	dnsSelfTestComment      = "CFST DNS self-test"
	dnsSelfTestCNAMETarget  = "example.com"
	dnsSelfTestStageTimeout = 90 * time.Second

	dnsSelfTestStatusPassed  = "passed"
	dnsSelfTestStatusFailed  = "failed"
	dnsSelfTestStatusSkipped = "skipped"
)

// DNS self-test fixtures. Documentation ranges only: nothing here should ever
// point at a real host if a cleanup step fails.
var (
	dnsSelfTestUploadIPs  = []string{"192.0.2.10", "192.0.2.11", "2001:db8::10"}
	dnsSelfTestReplaceIPs = []string{"198.51.100.20", "2001:db8::20"}
	dnsSelfTestCNAMEIPs   = []string{"203.0.113.30", "2001:db8::30"}
)

type dnsSelfTestRecord struct {
	Content          string `json:"content"`
	ID               string `json:"id"`
	Name             string `json:"name"`
	ProviderRecordID string `json:"-"`
	TTL              int    `json:"ttl"`
	Type             string `json:"type"`
}

type dnsSelfTestReadResult struct {
	AAAA []dnsSelfTestRecord
	All  []dnsSelfTestRecord
	A    []dnsSelfTestRecord
}

type dnsSelfTestStage struct {
	Details    map[string]any `json:"details,omitempty"`
	DurationMS int64          `json:"duration_ms"`
	ID         string         `json:"id"`
	Message    string         `json:"message"`
	Status     string         `json:"status"`
	Title      string         `json:"title"`
}

// dnsSelfTestAdapter hides the per-platform client differences: read snapshots,
// one reconcile push, CNAME creation for the blocking-record case, and delete.
type dnsSelfTestAdapter interface {
	// CreateCNAME creates a CNAME on the test name so the next push has to
	// remove the blocking record before it can write A/AAAA.
	CreateCNAME(ctx context.Context, target string, ttl int) (dnsSelfTestRecord, error)
	Delete(ctx context.Context, record dnsSelfTestRecord) error
	// Push runs the same reconcile path the upload feature uses and returns the
	// provider summary plus any warnings.
	Push(ctx context.Context, ips string, ttl int) (dnspush.Summary, []string, error)
	Read(ctx context.Context) (dnsSelfTestReadResult, error)
}

// RunDNSPlatformSelfTest is the command entry point. It validates provider
// config and the test record name before touching any records, then runs the
// staged flow on the dedicated test name only.
func (s *Service) RunDNSPlatformSelfTest(ctx context.Context, payload map[string]any) CommandResult {
	if ctx == nil {
		ctx = context.Background()
	}
	if !s.dnsSelfTestMu.TryLock() {
		return dnsSelfTestBusyFailure()
	}
	defer s.dnsSelfTestMu.Unlock()
	snapshot := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	providers := DNSProvidersFromSnapshot(snapshot)
	providerID := strings.TrimSpace(stringValue(firstNonNil(payload["provider_id"], payload["providerId"]), ""))
	provider, ok := selectDNSProvider(providers, providerID)
	if !ok {
		return dnsSelfTestConfigFailure("没有可用的 DNS 推送平台。", map[string]any{})
	}
	if !provider.Enabled {
		return dnsSelfTestConfigFailure("平台未启用，请先在设置页启用该平台。", dnsSelfTestProviderData(provider))
	}
	if problem := dnsProviderConfigProblem(provider); problem != "" {
		return dnsSelfTestConfigFailure("平台配置未完成："+problem, dnsSelfTestProviderData(provider))
	}

	var dnsheSubdomain *dnshecore.Subdomain
	testBase := provider.RecordName
	if provider.Provider == DNSProviderDNSHE {
		subdomain, recordName, err := resolveDNSHETarget(ctx, s.newDNSHESelfTestClient(provider), provider)
		if err != nil {
			return dnsSelfTestConfigFailure("DNSHE 子域名解析失败："+err.Error(), dnsSelfTestProviderData(provider))
		}
		provider.RecordName = recordName
		dnsheSubdomain = &subdomain
		// DNSHE 自检固定使用主域名（子域 zone 顶点）下的 CFST 子域，避免跟随
		// 推送记录名漂移到别的记录。
		testBase = subdomain.FullDomain
	}

	testName, err := dnsSelfTestRecordName(testBase, stringValue(firstNonNil(payload["test_name"], payload["testName"]), ""))
	if err != nil {
		return dnsSelfTestConfigFailure(err.Error(), dnsSelfTestProviderData(provider))
	}
	baseTTL, altTTL := dnsSelfTestTTLPair(provider)

	adapter := s.dnsSelfTestAdapter(provider, testName, dnsheSubdomain)
	return s.runDNSPlatformSelfTestWithAdapter(ctx, provider, testName, baseTTL, altTTL, adapter)
}

// runDNSPlatformSelfTestWithAdapter is separated from the command entry point
// so the stage flow can be tested with a fake adapter.
func (s *Service) runDNSPlatformSelfTestWithAdapter(ctx context.Context, provider DNSProviderConfig, testName string, baseTTL, altTTL int, adapter dnsSelfTestAdapter) CommandResult {
	runCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	run := &dnsSelfTestRun{
		adapter:    adapter,
		allowCNAME: provider.AllowCNAME,
		ctx:        runCtx,
		testName:   testName,
	}
	run.initStages()
	run.stages[0].Status = dnsSelfTestStatusPassed
	run.stages[0].Message = fmt.Sprintf("%s 配置可用，测试记录名：%s", dnsProviderDisplayName(provider), testName)

	run.execute(baseTTL, altTTL)

	warnings := notify.DedupeStrings(run.warnings)
	data := dnsSelfTestProviderData(provider)
	data["test_name"] = testName
	data["base_ttl"] = baseTTL
	data["alt_ttl"] = altTTL
	data["stages"] = run.stages
	data["warnings"] = warnings
	data["passed"] = run.countStatus(dnsSelfTestStatusPassed)
	data["failed"] = run.countStatus(dnsSelfTestStatusFailed)
	data["skipped"] = run.countStatus(dnsSelfTestStatusSkipped)
	data["cleanup_ok"] = run.cleanupOK
	data["wrote_records"] = run.wroteRecord

	switch {
	case run.countStatus(dnsSelfTestStatusFailed) > 0:
		message := fmt.Sprintf("DNS 自检失败：%d 项失败、%d 项通过、%d 项跳过。", data["failed"], data["passed"], data["skipped"])
		if !run.cleanupOK {
			message += " 测试记录可能残留，请按阶段详情手动清理。"
		}
		return NewCommandResult("DNS_SELFTEST_FAILED", data, message, false, nil, warnings)
	case run.countStatus(dnsSelfTestStatusSkipped) > 0:
		return NewCommandResult("DNS_SELFTEST_PARTIAL", data, fmt.Sprintf("DNS 自检完成：%d 项通过、%d 项跳过。", data["passed"], data["skipped"]), true, nil, warnings)
	default:
		return NewCommandResult("DNS_SELFTEST_COMPLETED", data, fmt.Sprintf("DNS 自检通过：%d 项全部通过，测试记录已清理。", data["passed"]), true, nil, warnings)
	}
}

func dnsSelfTestConfigFailure(message string, data map[string]any) CommandResult {
	return dnsSelfTestPreflightFailure("DNS_SELFTEST_CONFIG_INVALID", message, data)
}

func dnsSelfTestBusyFailure() CommandResult {
	return dnsSelfTestPreflightFailure("DNS_SELFTEST_BUSY", "已有 DNS 自检正在运行，请等待当前任务结束。", map[string]any{})
}

func dnsSelfTestPreflightFailure(code, message string, data map[string]any) CommandResult {
	stages := make([]dnsSelfTestStage, 0, len(dnsSelfTestStageSpecs))
	for _, definition := range dnsSelfTestStageSpecs {
		status, detail := dnsSelfTestStatusSkipped, "配置检查未通过，未执行。"
		if definition.ID == "preflight" {
			status, detail = dnsSelfTestStatusFailed, message
		}
		stages = append(stages, dnsSelfTestStage{
			Details: map[string]any{"id": definition.ID},
			ID:      definition.ID,
			Message: detail,
			Status:  status,
			Title:   definition.Title,
		})
	}
	data["stages"] = stages
	data["passed"] = 0
	data["failed"] = 1
	data["skipped"] = len(stages) - 1
	data["cleanup_ok"] = true
	data["wrote_records"] = false
	return NewCommandResult(code, data, message, false, nil, nil)
}

func dnsSelfTestProviderData(provider DNSProviderConfig) map[string]any {
	return map[string]any{
		"allow_cname": provider.AllowCNAME,
		"provider":    dnsProviderPlatformName(provider.Provider),
		"provider_id": provider.ID,
		"record_name": provider.RecordName,
	}
}

func selectDNSProvider(providers []DNSProviderConfig, providerID string) (DNSProviderConfig, bool) {
	if len(providers) == 0 {
		return DNSProviderConfig{}, false
	}
	if providerID == "" {
		for _, provider := range providers {
			if provider.Enabled {
				return provider, true
			}
		}
		return providers[0], true
	}
	for _, provider := range providers {
		if provider.ID == providerID {
			return provider, true
		}
	}
	return DNSProviderConfig{}, false
}

// resolveDNSHETarget mirrors dnshecore.resolveTarget precedence so the
// self-test and the DNS read page resolve the same zone and record name a push
// would target.
func resolveDNSHETarget(ctx context.Context, client *dnshecore.Client, provider DNSProviderConfig) (dnshecore.Subdomain, string, error) {
	var (
		subdomain  dnshecore.Subdomain
		recordName string
		err        error
	)
	switch {
	case strings.TrimSpace(provider.Subdomain) != "":
		subdomain, recordName, err = client.ResolveSubdomainTarget(ctx, provider.Subdomain, provider.RecordName)
	case provider.SubdomainID > 0:
		subdomain, err = client.GetSubdomain(ctx, provider.SubdomainID)
		recordName = provider.RecordName
	default:
		subdomain, err = client.ResolveSubdomain(ctx, provider.RecordName)
		recordName = provider.RecordName
	}
	if err != nil {
		return dnshecore.Subdomain{}, "", err
	}
	fullDomain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(subdomain.FullDomain), "."))
	if fullDomain == "" {
		return dnshecore.Subdomain{}, "", fmt.Errorf("DNSHE 子域 %d 缺少完整域名", subdomain.ID)
	}
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(recordName), "."))
	if name == "" {
		name = fullDomain
	}
	if name != fullDomain && !strings.HasSuffix(name, "."+fullDomain) {
		return dnshecore.Subdomain{}, "", fmt.Errorf("DNSHE 记录名 %s 不属于子域名 %s", name, fullDomain)
	}
	subdomain.FullDomain = fullDomain
	return subdomain, name, nil
}

// dnsSelfTestRecordName derives or validates the dedicated test name. The name
// must be a child of base: the push record name for most platforms, the
// subdomain apex (主域名) for DNSHE. The cfst-selftest prefix keeps the test off
// production names.
func dnsSelfTestRecordName(base, raw string) (string, error) {
	base = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(base), "."))
	if base == "" {
		return "", fmt.Errorf("当前平台没有填写推送记录名，无法确定测试记录名")
	}
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
	if name == "" {
		name = dnsSelfTestPrefix + "." + base
	}
	if strings.Contains(name, "*") {
		return "", fmt.Errorf("测试记录名不能包含通配符")
	}
	if name == base {
		return "", fmt.Errorf("测试记录名不能等于当前推送记录名 %s", base)
	}
	if !strings.HasSuffix(name, "."+base) {
		return "", fmt.Errorf("测试记录名必须是 %s 的子域名（例如 %s.%s）", base, dnsSelfTestPrefix, base)
	}
	label := strings.TrimSuffix(name, "."+base)
	if label != dnsSelfTestPrefix && !strings.HasPrefix(label, dnsSelfTestPrefix+".") {
		return "", fmt.Errorf("测试记录名必须以 %s 开头，避免误用生产子域名", dnsSelfTestPrefix)
	}
	if label == "" || strings.Contains(label, "..") || strings.HasPrefix(label, ".") || strings.HasSuffix(label, ".") {
		return "", fmt.Errorf("测试记录名格式无效")
	}
	return name, nil
}

// dnsSelfTestTTLPair picks two TTLs the platform is likely to accept, so the
// attribute-update stage exercises a real change instead of an API rejection.
func dnsSelfTestTTLPair(provider DNSProviderConfig) (int, int) {
	return dnsProviderModuleOrDefault(provider.Provider).ttlPair(provider.TTL)
}
