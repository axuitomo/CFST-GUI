package appcore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/dnspush"
)

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
	if summary.Created > 0 || summary.Updated > 0 {
		r.wroteRecord = true
	}
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
