package appcore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspush"
)

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
	allowCNAME  bool
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
	createCtx, cancelCreate := r.stageTimeout()
	_, err = r.adapter.CreateCNAME(createCtx, dnsSelfTestCNAMETarget, ttl)
	cancelCreate()
	if err != nil {
		r.mark("cname", dnsSelfTestStatusSkipped, "平台拒绝在测试记录名上创建 CNAME，已跳过："+err.Error(), nil)
		r.warnings = append(r.warnings, "CNAME 阻塞替换已跳过："+err.Error())
		return nil
	}
	r.wroteRecord = true
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
	if !r.allowCNAME {
		if err == nil {
			r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
			return fmt.Errorf("未启用「允许修改 CNAME」，但存在 CNAME 时推送未被拒绝（created=%d deleted=%d updated=%d）", summary.Created, summary.Deleted, summary.Updated)
		}
		if !errors.Is(err, dnspush.ErrCNAMEProtected) {
			r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
			return fmt.Errorf("未启用「允许修改 CNAME」，但返回了非拦截错误：%w", err)
		}
		after, readErr := r.read("cname")
		if readErr != nil {
			r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
			return fmt.Errorf("CNAME 被拒绝后读取失败：%w", readErr)
		}
		if len(managedSelfTestRecords(after.All)) != 1 || len(recordsBySelfTestType(after.All, dnspush.RecordTypeCNAME)) != 1 {
			r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
			return fmt.Errorf("未启用「允许修改 CNAME」时 CNAME 应保持原样，实际托管记录为 %s", selfTestRecordList(managedSelfTestRecords(after.All)))
		}
		r.mark("cname", dnsSelfTestStatusPassed, "未启用「允许修改 CNAME」：推送被拒绝，CNAME 未被删除。", map[string]any{
			"allow_cname": false, "cname_preserved": true, "error": err.Error(),
		})
		r.stages[r.stageIndex["cname"]].DurationMS = time.Since(started).Milliseconds()
		return nil
	}
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
