package appcore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/alidnscore"
	"github.com/axuitomo/CFST-GUI/internal/cloudflarecore"
	"github.com/axuitomo/CFST-GUI/internal/dnshecore"
	"github.com/axuitomo/CFST-GUI/internal/dnspodcore"
	"github.com/axuitomo/CFST-GUI/internal/dnspush"
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

	testName, err := dnsSelfTestRecordName(provider, stringValue(firstNonNil(payload["test_name"], payload["testName"]), ""))
	if err != nil {
		return dnsSelfTestConfigFailure(err.Error(), dnsSelfTestProviderData(provider))
	}
	baseTTL, altTTL := dnsSelfTestTTLPair(provider)

	adapter := s.dnsSelfTestAdapter(provider, testName)
	return s.runDNSPlatformSelfTestWithAdapter(ctx, provider, testName, baseTTL, altTTL, adapter)
}

// runDNSPlatformSelfTestWithAdapter is separated from the command entry point
// so the stage flow can be tested with a fake adapter.
func (s *Service) runDNSPlatformSelfTestWithAdapter(ctx context.Context, provider DNSProviderConfig, testName string, baseTTL, altTTL int, adapter dnsSelfTestAdapter) CommandResult {
	runCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	run := &dnsSelfTestRun{
		adapter:  adapter,
		ctx:      runCtx,
		testName: testName,
	}
	run.initStages()
	run.stages[0].Status = dnsSelfTestStatusPassed
	run.stages[0].Message = fmt.Sprintf("%s 配置可用，测试记录名：%s", dnsProviderDisplayName(provider), testName)

	run.execute(baseTTL, altTTL)

	warnings := dedupeStrings(run.warnings)
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
	return NewCommandResult("DNS_SELFTEST_CONFIG_INVALID", data, message, false, nil, nil)
}

func dnsSelfTestProviderData(provider DNSProviderConfig) map[string]any {
	return map[string]any{
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

// dnsSelfTestRecordName derives or validates the dedicated test name. The name
// must be a child of the configured record name, so the test can never touch the
// production target or the zone apex.
func dnsSelfTestRecordName(provider DNSProviderConfig, raw string) (string, error) {
	base := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(provider.RecordName), "."))
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
	base := normalizeDNSProviderTTL(provider.Provider, provider.TTL)
	switch provider.Provider {
	case DNSProviderAliyun, DNSProviderDNSPod, DNSProviderDNSHE:
		// These platforms reject TTLs below 600, so a low configured value
		// would fail the upload before it could test anything.
		if base < 600 {
			base = 600
		}
		if base <= 600 {
			return base, 1200
		}
		return base, 600
	default:
		if base == 600 {
			return base, 300
		}
		return base, 600
	}
}

type dnsSelfTestStageSpec struct {
	ID    string
	Title string
}

var dnsSelfTestStageSpecs = []dnsSelfTestStageSpec{
	{ID: "preflight", Title: "配置与记录名检查"},
	{ID: "baseline", Title: "读取基线"},
	{ID: "reset", Title: "清理残留测试记录"},
	{ID: "upload", Title: "上传 A + AAAA"},
	{ID: "read", Title: "读取与类型过滤校验"},
	{ID: "idempotent", Title: "幂等重推"},
	{ID: "replace", Title: "记录换血"},
	{ID: "update", Title: "属性（TTL）更新"},
	{ID: "cname", Title: "CNAME 阻塞替换"},
	{ID: "cleanup", Title: "清理测试记录"},
	{ID: "verify", Title: "终态复核"},
}

type dnsSelfTestRun struct {
	adapter     dnsSelfTestAdapter
	cleanupOK   bool
	ctx         context.Context
	protected   []dnsSelfTestRecord
	stages      []dnsSelfTestStage
	testName    string
	warnings    []string
	stageIndex  map[string]int
	wroteRecord bool
}

func (r *dnsSelfTestRun) initStages() {
	r.stageIndex = make(map[string]int, len(dnsSelfTestStageSpecs))
	r.stages = make([]dnsSelfTestStage, 0, len(dnsSelfTestStageSpecs))
	for _, definition := range dnsSelfTestStageSpecs {
		r.stageIndex[definition.ID] = len(r.stages)
		r.stages = append(r.stages, dnsSelfTestStage{
			Details: map[string]any{},
			ID:      definition.ID,
			Message: "未执行（前序阶段未通过）",
			Status:  dnsSelfTestStatusSkipped,
			Title:   definition.Title,
		})
	}
}

func (r *dnsSelfTestRun) mark(id, status, message string, details map[string]any) {
	index, ok := r.stageIndex[id]
	if !ok {
		return
	}
	r.stages[index].Status = status
	r.stages[index].Message = message
	if details != nil {
		r.stages[index].Details = details
	}
}

func (r *dnsSelfTestRun) countStatus(status string) int {
	count := 0
	for _, stage := range r.stages {
		if stage.Status == status {
			count++
		}
	}
	return count
}

// execute runs the staged flow. Once a write has happened, every abort path
// goes through cleanup and verification so a failed run still reports residue.
func (r *dnsSelfTestRun) execute(baseTTL, altTTL int) {
	baseline, err := r.read("baseline")
	if err != nil {
		r.fail("baseline", err)
		r.cleanupOK = true
		r.mark("cleanup", dnsSelfTestStatusSkipped, "读取基线失败，未写入任何记录，无需清理。", nil)
		r.mark("verify", dnsSelfTestStatusSkipped, "读取基线失败，未写入任何记录，无需复核。", nil)
		return
	}
	r.protected = protectedSelfTestRecords(baseline.All)
	r.mark("baseline", dnsSelfTestStatusPassed, fmt.Sprintf("读取到 %d 条记录，其中托管记录 %d 条、非托管记录 %d 条。", len(baseline.All), len(managedSelfTestRecords(baseline.All)), len(r.protected)), map[string]any{
		"records": len(baseline.All), "managed": len(managedSelfTestRecords(baseline.All)), "protected": len(r.protected),
	})

	if _, err := r.reset(); err != nil {
		r.fail("reset", err)
		r.cleanup("残留清理失败，未进入上传阶段。")
		r.verify()
		return
	}

	if err := r.pushStage("upload", "上传 A×2 + AAAA×1", dnsSelfTestUploadIPs, baseTTL, func(summary dnspush.Summary) error {
		if summary.Created != 3 || summary.Updated != 0 || summary.Deleted != 0 {
			return fmt.Errorf("期望 created=3 updated=0 deleted=0，实际 created=%d updated=%d deleted=%d", summary.Created, summary.Updated, summary.Deleted)
		}
		return nil
	}); err != nil {
		r.fail("upload", err)
		r.cleanup("上传阶段失败，已尽力清理。")
		r.verify()
		return
	}

	uploaded, err := r.read("read")
	if err != nil {
		r.fail("read", err)
		r.cleanup("读取校验失败，已尽力清理。")
		r.verify()
		return
	}
	if err := r.checkRead(uploaded, []string{"A:192.0.2.10", "A:192.0.2.11", "AAAA:2001:db8::10"}, baseTTL); err != nil {
		r.fail("read", err)
		r.cleanup("读取校验失败，已尽力清理。")
		r.verify()
		return
	}
	r.mark("read", dnsSelfTestStatusPassed, "读取返回 2 条 A、1 条 AAAA，类型过滤与 TTL 正确。", map[string]any{
		"a": len(uploaded.A), "aaaa": len(uploaded.AAAA),
	})
	uploadIDs := selfTestIDSet(addressSelfTestRecords(uploaded.All))

	if err := r.pushStage("idempotent", "同集合重推", dnsSelfTestUploadIPs, baseTTL, func(summary dnspush.Summary) error {
		if summary.Created != 0 || summary.Updated != 0 || summary.Deleted != 0 {
			return fmt.Errorf("期望 0/0/0，实际 created=%d updated=%d deleted=%d", summary.Created, summary.Updated, summary.Deleted)
		}
		return nil
	}); err != nil {
		r.fail("idempotent", err)
		r.cleanup("幂等重推失败，已尽力清理。")
		r.verify()
		return
	}
	again, err := r.read("idempotent")
	if err != nil {
		r.fail("idempotent", err)
		r.cleanup("幂等重推复核失败，已尽力清理。")
		r.verify()
		return
	}
	if !equalStringSlices(uploadIDs, selfTestIDSet(addressSelfTestRecords(again.All))) {
		r.fail("idempotent", fmt.Errorf("重推后记录 id 集合发生变化：%v → %v", uploadIDs, selfTestIDSet(addressSelfTestRecords(again.All))))
		r.cleanup("幂等重推复核失败，已尽力清理。")
		r.verify()
		return
	}
	r.mark("idempotent", dnsSelfTestStatusPassed, "同集合重推 0/0/0，记录 id 与内容均未变化。", map[string]any{"ids": uploadIDs})

	if err := r.pushStage("replace", "A×1 + AAAA×1 换血", dnsSelfTestReplaceIPs, baseTTL, func(summary dnspush.Summary) error {
		if summary.Created != 2 || summary.Updated != 0 || summary.Deleted != 3 {
			return fmt.Errorf("期望 created=2 updated=0 deleted=3，实际 created=%d updated=%d deleted=%d", summary.Created, summary.Updated, summary.Deleted)
		}
		return nil
	}); err != nil {
		r.fail("replace", err)
		r.cleanup("换血阶段失败，已尽力清理。")
		r.verify()
		return
	}
	replaced, err := r.read("replace")
	if err != nil {
		r.fail("replace", err)
		r.cleanup("换血复核失败，已尽力清理。")
		r.verify()
		return
	}
	if err := r.checkRead(replaced, []string{"A:198.51.100.20", "AAAA:2001:db8::20"}, baseTTL); err != nil {
		r.fail("replace", err)
		r.cleanup("换血复核失败，已尽力清理。")
		r.verify()
		return
	}
	oldIDs := uploadIDs
	if anyIDPresent(oldIDs, selfTestIDSet(addressSelfTestRecords(replaced.All))) {
		r.fail("replace", fmt.Errorf("换血后仍存在旧记录 id"))
		r.cleanup("换血复核失败，已尽力清理。")
		r.verify()
		return
	}
	r.mark("replace", dnsSelfTestStatusPassed, "旧 3 条全部消失，新 2 条写入，符合「换 IP = 新建 + 删除」。", map[string]any{
		"new_ids": selfTestIDSet(addressSelfTestRecords(replaced.All)),
	})

	updateIDs := selfTestIDSet(addressSelfTestRecords(replaced.All))
	if err := r.pushStage("update", fmt.Sprintf("同内容，TTL %d → %d", baseTTL, altTTL), dnsSelfTestReplaceIPs, altTTL, func(summary dnspush.Summary) error {
		if summary.Created != 0 || summary.Updated != 2 || summary.Deleted != 0 {
			return fmt.Errorf("期望 created=0 updated=2 deleted=0，实际 created=%d updated=%d deleted=%d", summary.Created, summary.Updated, summary.Deleted)
		}
		return nil
	}); err != nil {
		r.fail("update", err)
		r.cleanup("属性更新失败，已尽力清理。")
		r.verify()
		return
	}
	updated, err := r.read("update")
	if err != nil {
		r.fail("update", err)
		r.cleanup("属性更新复核失败，已尽力清理。")
		r.verify()
		return
	}
	if err := r.checkRead(updated, []string{"A:198.51.100.20", "AAAA:2001:db8::20"}, altTTL); err != nil {
		r.fail("update", err)
		r.cleanup("属性更新复核失败，已尽力清理。")
		r.verify()
		return
	}
	if !equalStringSlices(updateIDs, selfTestIDSet(addressSelfTestRecords(updated.All))) {
		r.fail("update", fmt.Errorf("属性更新后记录 id 发生变化：%v → %v", updateIDs, selfTestIDSet(addressSelfTestRecords(updated.All))))
		r.cleanup("属性更新复核失败，已尽力清理。")
		r.verify()
		return
	}
	r.mark("update", dnsSelfTestStatusPassed, fmt.Sprintf("原地更新 2 条，id 不变，TTL=%d。", altTTL), map[string]any{"ids": updateIDs, "ttl": altTTL})

	if err := r.runCNAMEStage(baseTTL); err != nil {
		r.fail("cname", err)
		r.cleanup("CNAME 阶段失败，已尽力清理。")
		r.verify()
		return
	}

	residue, err := r.read("cleanup")
	if err == nil {
		if deleted, deleteErr := r.deleteManaged(residue.All); deleteErr != nil {
			err = deleteErr
		} else {
			r.wroteRecord = true
			r.cleanupOK = true
			r.mark("cleanup", dnsSelfTestStatusPassed, fmt.Sprintf("已删除 %d 条测试记录。", deleted), map[string]any{"deleted": deleted})
		}
	}
	if err != nil {
		r.fail("cleanup", err)
		r.verify()
		return
	}
	final, err := r.read("verify")
	if err != nil {
		r.fail("verify", err)
		return
	}
	if err := r.checkFinal(final); err != nil {
		r.fail("verify", err)
		return
	}
	r.mark("verify", dnsSelfTestStatusPassed, fmt.Sprintf("测试记录名已回到基线：托管记录 0 条，保留非托管记录 %d 条。", len(r.protected)), map[string]any{
		"remaining": len(final.All), "protected": len(r.protected),
	})
}

func (r *dnsSelfTestRun) runCNAMEStage(ttl int) error {
	started := time.Now()
	before, err := r.read("cname")
	if err != nil {
		r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
		return err
	}
	if _, err := r.deleteManaged(before.All); err != nil {
		r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
		return fmt.Errorf("为 CNAME 测试清理地址记录失败：%w", err)
	}
	r.wroteRecord = true
	createCtx, cancelCreate := r.stageTimeout()
	_, err = r.adapter.CreateCNAME(createCtx, dnsSelfTestCNAMETarget, ttl)
	cancelCreate()
	if err != nil {
		r.mark("cname", dnsSelfTestStatusSkipped, "平台拒绝在测试记录名上创建 CNAME，已跳过："+err.Error(), nil)
		r.warnings = append(r.warnings, "CNAME 阻塞替换已跳过："+err.Error())
		return nil
	}
	afterCreate, err := r.read("cname")
	if err != nil {
		r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
		return fmt.Errorf("创建 CNAME 后读取失败：%w", err)
	}
	if len(recordsBySelfTestType(afterCreate.All, dnspush.RecordTypeCNAME)) != 1 {
		r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
		return fmt.Errorf("创建 CNAME 后未读到唯一 CNAME 记录")
	}
	pushCtx, cancelPush := r.stageTimeout()
	summary, warnings, err := r.adapter.Push(pushCtx, strings.Join(dnsSelfTestCNAMEIPs, ","), ttl)
	cancelPush()
	r.warnings = append(r.warnings, warnings...)
	if err != nil {
		r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
		return fmt.Errorf("存在 CNAME 时推送失败：%w", err)
	}
	if summary.Created != 2 || summary.Deleted != 1 || summary.Updated != 0 {
		r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
		return fmt.Errorf("期望 created=2 deleted=1 updated=0，实际 created=%d deleted=%d updated=%d", summary.Created, summary.Deleted, summary.Updated)
	}
	after, err := r.read("cname")
	if err != nil {
		r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
		return fmt.Errorf("CNAME 替换后读取失败：%w", err)
	}
	if len(recordsBySelfTestType(after.All, dnspush.RecordTypeCNAME)) != 0 {
		r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
		return fmt.Errorf("CNAME 替换后仍存在 CNAME 记录")
	}
	if err := r.checkRead(after, []string{"A:203.0.113.30", "AAAA:2001:db8::30"}, ttl); err != nil {
		r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
		return fmt.Errorf("CNAME 替换后地址记录不正确：%w", err)
	}
	r.mark("cname", dnsSelfTestStatusPassed, "CNAME 先被删除，再写入 A/AAAA；终态无 CNAME。", map[string]any{
		"created": 2, "deleted": 1,
	})
	r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
	return nil
}

// read assigns the stage duration and returns a fresh snapshot.
func (r *dnsSelfTestRun) read(stageID string) (dnsSelfTestReadResult, error) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.ctx, dnsSelfTestStageTimeout)
	defer cancel()
	result, err := r.adapter.Read(ctx)
	if index, ok := r.stageIndex[stageID]; ok {
		r.stages[index].DurationMS = time.Since(started).Milliseconds()
	}
	return result, err
}

func (r *dnsSelfTestRun) reset() (int, error) {
	started := time.Now()
	snapshot, err := r.read("reset")
	if err == nil {
		if len(managedSelfTestRecords(snapshot.All)) > 0 {
			r.wroteRecord = true
		}
		deleted, deleteErr := r.deleteManaged(snapshot.All)
		if deleteErr != nil {
			err = deleteErr
		} else {
			r.mark("reset", dnsSelfTestStatusPassed, fmt.Sprintf("测试记录名起始状态清理完成，删除 %d 条残留托管记录。", deleted), map[string]any{
				"deleted": deleted, "protected": len(r.protected),
			})
		}
	}
	if err != nil {
		r.stages[r.stageIndex["reset"]].DurationMS = time.Since(started).Milliseconds()
		return 0, err
	}
	r.stages[r.stageIndex["reset"]].DurationMS = time.Since(started).Milliseconds()
	return 0, nil
}

func (r *dnsSelfTestRun) pushStage(id, message string, ips []string, ttl int, check func(dnspush.Summary) error) error {
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.ctx, dnsSelfTestStageTimeout)
	defer cancel()
	summary, warnings, err := r.adapter.Push(ctx, strings.Join(ips, ","), ttl)
	r.warnings = append(r.warnings, warnings...)
	r.wroteRecord = true
	if index, ok := r.stageIndex[id]; ok {
		r.stages[index].DurationMS = time.Since(started).Milliseconds()
	}
	if err != nil {
		return err
	}
	if check != nil {
		if err := check(summary); err != nil {
			return err
		}
	}
	r.mark(id, dnsSelfTestStatusPassed, message, map[string]any{
		"created": summary.Created, "updated": summary.Updated, "deleted": summary.Deleted, "ignored": summary.Ignored,
	})
	return nil
}

func (r *dnsSelfTestRun) checkRead(snapshot dnsSelfTestReadResult, want []string, ttl int) error {
	if got := selfTestContents(snapshot.A); !equalStringSlices(got, wantOfTypes(want, dnspush.RecordTypeA)) {
		return fmt.Errorf("A 记录内容不符：期望 %v，实际 %v", wantOfTypes(want, dnspush.RecordTypeA), got)
	}
	if got := selfTestContents(snapshot.AAAA); !equalStringSlices(got, wantOfTypes(want, dnspush.RecordTypeAAAA)) {
		return fmt.Errorf("AAAA 记录内容不符：期望 %v，实际 %v", wantOfTypes(want, dnspush.RecordTypeAAAA), got)
	}
	if len(snapshot.All) != len(want)+len(r.protected) {
		return fmt.Errorf("测试记录名总条数不符：期望 %d（含非托管 %d），实际 %d", len(want)+len(r.protected), len(r.protected), len(snapshot.All))
	}
	for _, record := range addressSelfTestRecords(snapshot.All) {
		if record.TTL != ttl {
			return fmt.Errorf("%s 记录 %s 的 TTL 期望 %d，实际 %d", record.Type, record.Content, ttl, record.TTL)
		}
	}
	return nil
}

func (r *dnsSelfTestRun) checkFinal(snapshot dnsSelfTestReadResult) error {
	if managed := managedSelfTestRecords(snapshot.All); len(managed) > 0 {
		return fmt.Errorf("测试记录名仍残留 %d 条托管记录：%s", len(managed), selfTestRecordList(managed))
	}
	if got, want := selfTestFingerprint(protectedSelfTestRecords(snapshot.All)), selfTestFingerprint(r.protected); got != want {
		return fmt.Errorf("测试记录名上的非托管记录发生变化：%s → %s", want, got)
	}
	return nil
}

func (r *dnsSelfTestRun) fail(id string, err error) {
	r.mark(id, dnsSelfTestStatusFailed, err.Error(), nil)
}

func (r *dnsSelfTestRun) cleanup(message string) {
	started := time.Now()
	ctx, cancel := r.cleanupStageContext()
	defer cancel()
	snapshot, err := r.adapter.Read(ctx)
	if err == nil {
		var deleted int
		deleted, err = r.deleteManagedWith(ctx, snapshot.All)
		if err == nil {
			r.cleanupOK = true
			r.mark("cleanup", dnsSelfTestStatusPassed, fmt.Sprintf("%s 已删除 %d 条测试记录。", message, deleted), map[string]any{"deleted": deleted})
		}
	}
	if err != nil {
		r.cleanupOK = false
		r.mark("cleanup", dnsSelfTestStatusFailed, message+" 清理失败："+err.Error(), nil)
	}
	r.stages[r.stageIndex["cleanup"]].DurationMS = time.Since(started).Milliseconds()
}

func (r *dnsSelfTestRun) verify() {
	started := time.Now()
	ctx, cancel := r.cleanupStageContext()
	defer cancel()
	snapshot, err := r.adapter.Read(ctx)
	if err == nil {
		err = r.checkFinal(snapshot)
	}
	if err != nil {
		r.mark("verify", dnsSelfTestStatusFailed, err.Error(), nil)
	} else {
		r.mark("verify", dnsSelfTestStatusPassed, fmt.Sprintf("测试记录名已回到基线：托管记录 0 条，保留非托管记录 %d 条。", len(r.protected)), map[string]any{
			"remaining": len(snapshot.All), "protected": len(r.protected),
		})
		if r.cleanupOK {
			// Keep an explicit success signal for the final report.
			r.stages[r.stageIndex["verify"]].Details["cleanup_ok"] = true
		}
	}
	r.stages[r.stageIndex["verify"]].DurationMS = time.Since(started).Milliseconds()
}

func (r *dnsSelfTestRun) stageTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.ctx, dnsSelfTestStageTimeout)
}

// cleanupStageContext deliberately drops the run cancellation signal: if the
// overall self-test deadline expires mid-flow, cleanup must still be able to
// delete the records it created.
func (r *dnsSelfTestRun) cleanupStageContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(r.ctx), dnsSelfTestStageTimeout)
}

func (r *dnsSelfTestRun) deleteManaged(records []dnsSelfTestRecord) (int, error) {
	ctx, cancel := r.stageTimeout()
	defer cancel()
	return r.deleteManagedWith(ctx, records)
}

func (r *dnsSelfTestRun) deleteManagedWith(ctx context.Context, records []dnsSelfTestRecord) (int, error) {
	deleted := 0
	var errs []error
	for _, record := range managedSelfTestRecords(records) {
		if err := r.adapter.Delete(ctx, record); err != nil {
			// Keep deleting the rest: one permission or API error should not
			// leave every other test record behind.
			errs = append(errs, fmt.Errorf("删除 %s %s（id=%s）失败：%w", record.Type, record.Content, record.ID, err))
			continue
		}
		deleted++
	}
	return deleted, errors.Join(errs...)
}

func managedSelfTestRecords(records []dnsSelfTestRecord) []dnsSelfTestRecord {
	result := make([]dnsSelfTestRecord, 0, len(records))
	for _, record := range records {
		switch strings.ToUpper(strings.TrimSpace(record.Type)) {
		case dnspush.RecordTypeA, dnspush.RecordTypeAAAA, dnspush.RecordTypeCNAME:
			result = append(result, record)
		}
	}
	return result
}

func addressSelfTestRecords(records []dnsSelfTestRecord) []dnsSelfTestRecord {
	result := make([]dnsSelfTestRecord, 0, len(records))
	for _, record := range records {
		switch strings.ToUpper(strings.TrimSpace(record.Type)) {
		case dnspush.RecordTypeA, dnspush.RecordTypeAAAA:
			result = append(result, record)
		}
	}
	return result
}

func protectedSelfTestRecords(records []dnsSelfTestRecord) []dnsSelfTestRecord {
	result := make([]dnsSelfTestRecord, 0, len(records))
	for _, record := range records {
		switch strings.ToUpper(strings.TrimSpace(record.Type)) {
		case dnspush.RecordTypeA, dnspush.RecordTypeAAAA, dnspush.RecordTypeCNAME:
		default:
			result = append(result, record)
		}
	}
	return result
}

func recordsBySelfTestType(records []dnsSelfTestRecord, recordType string) []dnsSelfTestRecord {
	result := make([]dnsSelfTestRecord, 0)
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Type), recordType) {
			result = append(result, record)
		}
	}
	return result
}

func selfTestContents(records []dnsSelfTestRecord) []string {
	result := make([]string, 0, len(records))
	for _, record := range records {
		result = append(result, record.Content)
	}
	sort.Strings(result)
	return result
}

func selfTestIDSet(records []dnsSelfTestRecord) []string {
	result := make([]string, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.ID) != "" {
			result = append(result, record.ID)
		}
	}
	sort.Strings(result)
	return result
}

func selfTestFingerprint(records []dnsSelfTestRecord) string {
	lines := make([]string, 0, len(records))
	for _, record := range records {
		lines = append(lines, fmt.Sprintf("%s|%s|%s|%d|%s", record.ID, strings.ToUpper(record.Type), record.Name, record.TTL, record.Content))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func selfTestRecordList(records []dnsSelfTestRecord) string {
	parts := make([]string, 0, len(records))
	for _, record := range records {
		parts = append(parts, fmt.Sprintf("%s %s (id=%s)", record.Type, record.Content, record.ID))
	}
	return strings.Join(parts, ", ")
}

func wantOfTypes(want []string, recordType string) []string {
	result := make([]string, 0)
	prefix := strings.ToUpper(recordType) + ":"
	for _, item := range want {
		if strings.HasPrefix(item, prefix) {
			result = append(result, strings.TrimPrefix(item, prefix))
		}
	}
	sort.Strings(result)
	return result
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func anyIDPresent(needles, haystack []string) bool {
	for _, needle := range needles {
		if slices.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

// --- adapters -------------------------------------------------------------

func (s *Service) dnsSelfTestAdapter(provider DNSProviderConfig, testName string) dnsSelfTestAdapter {
	switch provider.Provider {
	case DNSProviderAliyun:
		return &aliyunSelfTestAdapter{
			client: s.newAliyunClient(provider),
			name:   testName,
		}
	case DNSProviderDNSPod:
		return &dnspodSelfTestAdapter{
			client: s.newDNSPodClient(provider),
			name:   testName,
		}
	case DNSProviderDNSHE:
		return &dnsheSelfTestAdapter{
			client:      s.newDNSHEClient(provider),
			name:        testName,
			subdomainID: provider.SubdomainID,
			sourceName:  provider.RecordName,
		}
	default:
		return &cloudflareSelfTestAdapter{
			client: s.newCloudflareClient(provider.APIToken),
			cfg: cloudflarecore.Config{
				APIToken:   provider.APIToken,
				Comment:    dnsSelfTestComment,
				Proxied:    false,
				RecordName: testName,
				TTL:        normalizeDNSProviderTTL(DNSProviderCloudflare, provider.TTL),
				ZoneID:     provider.ZoneID,
			},
			name: testName,
		}
	}
}

type cloudflareSelfTestAdapter struct {
	client *cloudflarecore.Client
	cfg    cloudflarecore.Config
	name   string
}

func (a *cloudflareSelfTestAdapter) Read(ctx context.Context) (dnsSelfTestReadResult, error) {
	all, err := a.client.ListRecordsWithOptions(ctx, a.cfg, cloudflarecore.ListOptions{Name: a.name})
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	aRecords, err := a.client.ListRecordsWithOptions(ctx, a.cfg, cloudflarecore.ListOptions{Name: a.name, Type: dnspush.RecordTypeA})
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	aaaaRecords, err := a.client.ListRecordsWithOptions(ctx, a.cfg, cloudflarecore.ListOptions{Name: a.name, Type: dnspush.RecordTypeAAAA})
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	return dnsSelfTestReadResult{
		All:  cloudflareSelfTestRecords(all),
		A:    cloudflareSelfTestRecords(aRecords),
		AAAA: cloudflareSelfTestRecords(aaaaRecords),
	}, nil
}

func (a *cloudflareSelfTestAdapter) Push(ctx context.Context, ips string, ttl int) (dnspush.Summary, []string, error) {
	cfg := a.cfg
	cfg.TTL = ttl
	result, err := cloudflarecore.PushRecords(ctx, a.client, cfg, ips)
	return result.Summary, result.Warnings, err
}

func (a *cloudflareSelfTestAdapter) CreateCNAME(ctx context.Context, target string, ttl int) (dnsSelfTestRecord, error) {
	created, err := a.client.CreateRecord(ctx, a.cfg, cloudflarecore.Record{
		Comment: dnsSelfTestComment,
		Content: target,
		Name:    a.name,
		Proxied: false,
		TTL:     ttl,
		Type:    dnspush.RecordTypeCNAME,
	})
	if err != nil {
		return dnsSelfTestRecord{}, err
	}
	return dnsSelfTestRecord{
		Content: created.Content,
		ID:      created.ID,
		Name:    created.Name,
		TTL:     created.TTL,
		Type:    created.Type,
	}, nil
}

func (a *cloudflareSelfTestAdapter) Delete(ctx context.Context, record dnsSelfTestRecord) error {
	return a.client.DeleteRecord(ctx, a.cfg, record.ID)
}

func cloudflareSelfTestRecords(records []cloudflarecore.Record) []dnsSelfTestRecord {
	result := make([]dnsSelfTestRecord, 0, len(records))
	for _, record := range records {
		result = append(result, dnsSelfTestRecord{
			Content: record.Content,
			ID:      record.ID,
			Name:    record.Name,
			TTL:     record.TTL,
			Type:    record.Type,
		})
	}
	return result
}

type aliyunSelfTestAdapter struct {
	client *alidnscore.Client
	name   string
	zone   alidnscore.Zone
	ready  bool
}

func (a *aliyunSelfTestAdapter) ensureZone(ctx context.Context) error {
	if a.ready {
		return nil
	}
	zone, err := a.client.ResolveZone(ctx, a.name)
	if err != nil {
		return err
	}
	a.zone = zone
	a.ready = true
	return nil
}

func (a *aliyunSelfTestAdapter) Read(ctx context.Context) (dnsSelfTestReadResult, error) {
	if err := a.ensureZone(ctx); err != nil {
		return dnsSelfTestReadResult{}, err
	}
	records, err := a.client.ListRecords(ctx, a.zone)
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	all := filterSelfTestRecords(records, a.name)
	return dnsSelfTestReadResult{
		All:  all,
		A:    recordsBySelfTestType(all, dnspush.RecordTypeA),
		AAAA: recordsBySelfTestType(all, dnspush.RecordTypeAAAA),
	}, nil
}

func (a *aliyunSelfTestAdapter) Push(ctx context.Context, ips string, ttl int) (dnspush.Summary, []string, error) {
	result, err := alidnscore.Push(ctx, a.client, alidnscore.Config{
		RecordName: a.name,
		TTL:        ttl,
	}, ips)
	return result.Summary, result.Warnings, err
}

func (a *aliyunSelfTestAdapter) CreateCNAME(ctx context.Context, target string, ttl int) (dnsSelfTestRecord, error) {
	if err := a.ensureZone(ctx); err != nil {
		return dnsSelfTestRecord{}, err
	}
	created, err := a.client.CreateRecord(ctx, a.zone, dnspush.Record{
		Content: target,
		Name:    a.name,
		TTL:     ttl,
		Type:    dnspush.RecordTypeCNAME,
	})
	if err != nil {
		return dnsSelfTestRecord{}, err
	}
	return dnsSelfTestRecord{Content: created.Content, ID: created.ID, Name: created.Name, TTL: created.TTL, Type: created.Type}, nil
}

func (a *aliyunSelfTestAdapter) Delete(ctx context.Context, record dnsSelfTestRecord) error {
	return a.client.DeleteRecord(ctx, a.zone, dnspush.Record{
		Content: record.Content,
		ID:      record.ID,
		Name:    record.Name,
		TTL:     record.TTL,
		Type:    record.Type,
	})
}

type dnspodSelfTestAdapter struct {
	client *dnspodcore.Client
	name   string
	zone   dnspodcore.Zone
	ready  bool
}

func (a *dnspodSelfTestAdapter) ensureZone(ctx context.Context) error {
	if a.ready {
		return nil
	}
	zone, err := a.client.ResolveZone(ctx, a.name)
	if err != nil {
		return err
	}
	a.zone = zone
	a.ready = true
	return nil
}

func (a *dnspodSelfTestAdapter) Read(ctx context.Context) (dnsSelfTestReadResult, error) {
	if err := a.ensureZone(ctx); err != nil {
		return dnsSelfTestReadResult{}, err
	}
	records, err := a.client.ListRecords(ctx, a.zone)
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	all := filterSelfTestRecords(records, a.name)
	return dnsSelfTestReadResult{
		All:  all,
		A:    recordsBySelfTestType(all, dnspush.RecordTypeA),
		AAAA: recordsBySelfTestType(all, dnspush.RecordTypeAAAA),
	}, nil
}

func (a *dnspodSelfTestAdapter) Push(ctx context.Context, ips string, ttl int) (dnspush.Summary, []string, error) {
	result, err := dnspodcore.Push(ctx, a.client, dnspodcore.Config{
		RecordName: a.name,
		TTL:        ttl,
	}, ips)
	return result.Summary, result.Warnings, err
}

func (a *dnspodSelfTestAdapter) CreateCNAME(ctx context.Context, target string, ttl int) (dnsSelfTestRecord, error) {
	if err := a.ensureZone(ctx); err != nil {
		return dnsSelfTestRecord{}, err
	}
	created, err := a.client.CreateRecord(ctx, a.zone, dnspush.Record{
		Content: target,
		Name:    a.name,
		TTL:     ttl,
		Type:    dnspush.RecordTypeCNAME,
	})
	if err != nil {
		return dnsSelfTestRecord{}, err
	}
	return dnsSelfTestRecord{Content: created.Content, ID: created.ID, Name: created.Name, TTL: created.TTL, Type: created.Type}, nil
}

func (a *dnspodSelfTestAdapter) Delete(ctx context.Context, record dnsSelfTestRecord) error {
	return a.client.DeleteRecord(ctx, a.zone, dnspush.Record{
		Content: record.Content,
		ID:      record.ID,
		Name:    record.Name,
		TTL:     record.TTL,
		Type:    record.Type,
	})
}

type dnsheSelfTestAdapter struct {
	client      *dnshecore.Client
	name        string
	sourceName  string
	subdomain   dnshecore.Subdomain
	subdomainID int
	ready       bool
}

func (a *dnsheSelfTestAdapter) ensureSubdomain(ctx context.Context) error {
	if a.ready {
		return nil
	}
	var (
		subdomain dnshecore.Subdomain
		err       error
	)
	if a.subdomainID > 0 {
		subdomain, err = a.client.GetSubdomain(ctx, a.subdomainID)
	} else {
		subdomain, err = a.client.ResolveSubdomain(ctx, a.sourceName)
	}
	if err != nil {
		return err
	}
	a.subdomain = subdomain
	a.ready = true
	return nil
}

func (a *dnsheSelfTestAdapter) Read(ctx context.Context) (dnsSelfTestReadResult, error) {
	if err := a.ensureSubdomain(ctx); err != nil {
		return dnsSelfTestReadResult{}, err
	}
	records, err := a.client.ListRecords(ctx, a.subdomain.ID)
	if err != nil {
		return dnsSelfTestReadResult{}, err
	}
	all := make([]dnsSelfTestRecord, 0, len(records))
	for _, record := range records {
		full := dnsheSelfTestFullName(a.subdomain.FullDomain, record.Name)
		if !strings.EqualFold(full, a.name) {
			continue
		}
		all = append(all, dnsSelfTestRecord{
			Content:          record.Content,
			ID:               record.ID,
			Name:             full,
			ProviderRecordID: record.ProviderRecordID,
			TTL:              record.TTL,
			Type:             record.Type,
		})
	}
	return dnsSelfTestReadResult{
		All:  all,
		A:    recordsBySelfTestType(all, dnspush.RecordTypeA),
		AAAA: recordsBySelfTestType(all, dnspush.RecordTypeAAAA),
	}, nil
}

func (a *dnsheSelfTestAdapter) Push(ctx context.Context, ips string, ttl int) (dnspush.Summary, []string, error) {
	if err := a.ensureSubdomain(ctx); err != nil {
		return dnspush.Summary{}, nil, err
	}
	result, err := dnshecore.Push(ctx, a.client, dnshecore.Config{
		RecordName:  a.name,
		SubdomainID: a.subdomain.ID,
		TTL:         ttl,
	}, ips)
	return result.Summary, result.Warnings, err
}

func (a *dnsheSelfTestAdapter) CreateCNAME(ctx context.Context, target string, ttl int) (dnsSelfTestRecord, error) {
	if err := a.ensureSubdomain(ctx); err != nil {
		return dnsSelfTestRecord{}, err
	}
	created, err := a.client.CreateRecord(ctx, a.subdomain.ID, dnshecore.Record{
		Content: target,
		Name:    dnsheSelfTestRelativeName(a.subdomain.FullDomain, a.name),
		TTL:     ttl,
		Type:    dnspush.RecordTypeCNAME,
	})
	if err != nil {
		return dnsSelfTestRecord{}, err
	}
	return dnsSelfTestRecord{
		Content:          created.Content,
		ID:               created.ID,
		Name:             a.name,
		ProviderRecordID: created.ProviderRecordID,
		TTL:              created.TTL,
		Type:             created.Type,
	}, nil
}

func (a *dnsheSelfTestAdapter) Delete(ctx context.Context, record dnsSelfTestRecord) error {
	return a.client.DeleteRecord(ctx, dnshecore.Record{
		Content:          record.Content,
		ID:               record.ID,
		Name:             dnsheSelfTestRelativeName(a.subdomain.FullDomain, a.name),
		ProviderRecordID: record.ProviderRecordID,
		TTL:              record.TTL,
		Type:             record.Type,
	})
}

func filterSelfTestRecords(records []dnspush.Record, name string) []dnsSelfTestRecord {
	normalized := normalizeSelfTestName(name)
	result := make([]dnsSelfTestRecord, 0)
	for _, record := range records {
		if normalizeSelfTestName(record.Name) != normalized {
			continue
		}
		result = append(result, dnsSelfTestRecord{
			Content: record.Content,
			ID:      record.ID,
			Name:    record.Name,
			TTL:     record.TTL,
			Type:    record.Type,
		})
	}
	return result
}

func normalizeSelfTestName(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

func dnsheSelfTestRelativeName(subdomainFullDomain, recordName string) string {
	subdomain := normalizeSelfTestName(subdomainFullDomain)
	name := normalizeSelfTestName(recordName)
	if subdomain == "" || name == "" || name == subdomain {
		return "@"
	}
	if strings.HasSuffix(name, "."+subdomain) {
		if prefix := strings.TrimSuffix(name, "."+subdomain); prefix != "" {
			return prefix
		}
	}
	return "@"
}

func dnsheSelfTestFullName(subdomainFullDomain, name string) string {
	subdomain := normalizeSelfTestName(subdomainFullDomain)
	normalized := normalizeSelfTestName(name)
	switch {
	case normalized == "" || normalized == "@":
		return subdomain
	case normalized == subdomain || strings.HasSuffix(normalized, "."+subdomain):
		return normalized
	default:
		// DNSHE returns direct labels as bare names; a multi-label relative
		// name such as "cfst-selftest.www" must still be expanded.
		return normalized + "." + subdomain
	}
}
