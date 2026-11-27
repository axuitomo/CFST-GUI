package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/history"
)

var resultHistoryTestNow = time.Date(2026, 1, 20, 12, 0, 0, 0, time.UTC)

// historyTestClock 让过期与上限的判定可复现,同时允许测试推进时间验证"没变就不重写"。
type historyTestClock struct {
	at time.Time
}

func (clock *historyTestClock) now() time.Time {
	return clock.at
}

func newResultHistoryService(t *testing.T) (*Service, *historyTestClock) {
	t.Helper()
	clock := &historyTestClock{at: resultHistoryTestNow}
	service := NewService(ServiceOptions{
		Clock:   clock.now,
		Storage: StorageLayout{Root: filepath.Join(t.TempDir(), "state")},
	})
	return service, clock
}

func historyValue[T any](value T) *T {
	return &value
}

func mergeHistoryBatchCommand(t *testing.T, service *Service, batchID string, at int64, rows []contracts.ProbeResultRow) CommandResult {
	t.Helper()
	payload, err := json.Marshal(history.MergeRequest{At: at, BatchID: batchID, Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	return service.Invoke("history.merge_batch", string(payload))
}

func mergeHistoryBatch(t *testing.T, service *Service, batchID string, at int64, rows []contracts.ProbeResultRow) history.MergeResult {
	t.Helper()
	result := mergeHistoryBatchCommand(t, service, batchID, at, rows)
	if !result.OK {
		t.Fatalf("history.merge_batch = %#v", result)
	}
	outcome, ok := result.Data.(history.MergeResult)
	if !ok {
		t.Fatalf("history.merge_batch data = %#v", result.Data)
	}
	return outcome
}

func importHistoryRecords(t *testing.T, service *Service, records []history.Record) history.MergeResult {
	t.Helper()
	payload, err := json.Marshal(history.ImportRequest{Records: records})
	if err != nil {
		t.Fatal(err)
	}
	result := service.Invoke("history.import", string(payload))
	if !result.OK {
		t.Fatalf("history.import = %#v", result)
	}
	outcome, ok := result.Data.(history.MergeResult)
	if !ok {
		t.Fatalf("history.import data = %#v", result.Data)
	}
	return outcome
}

func queryHistoryPage(t *testing.T, service *Service) history.Page {
	t.Helper()
	result := service.Invoke("history.query", "{}")
	if !result.OK {
		t.Fatalf("history.query = %#v", result)
	}
	page, ok := result.Data.(history.Page)
	if !ok {
		t.Fatalf("history.query data = %#v", result.Data)
	}
	return page
}

func queryHistoryRecords(t *testing.T, service *Service) []history.Record {
	t.Helper()
	return queryHistoryPage(t, service).Records
}

func historyFileUpdatedAt(t *testing.T, service *Service) string {
	t.Helper()
	raw, err := os.ReadFile(service.resultHistoryPath())
	if err != nil {
		t.Fatal(err)
	}
	var file history.File
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	return file.UpdatedAt
}

func historyQuarantineFiles(t *testing.T, service *Service) []string {
	t.Helper()
	matches, err := filepath.Glob(service.resultHistoryPath() + ".corrupt-*")
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func TestMergeResultHistoryBatchReplacesSameKeyWithoutDuplicating(t *testing.T) {
	service, clock := newResultHistoryService(t)
	at := clock.now().UnixMilli()
	first := mergeHistoryBatch(t, service, "batch-1", at, []contracts.ProbeResultRow{
		{Address: "1.1.1.1", StageStatus: "completed", TCPLatencyMS: historyValue(30.0), TestPort: historyValue(443)},
		{Address: "2.2.2.2", StageStatus: "tcp_failed"},
		{Address: "   ", StageStatus: "completed"},
	})
	if !first.Changed || first.Merged != 1 || first.Skipped != 2 {
		t.Fatalf("first merge = %#v", first)
	}

	second := mergeHistoryBatch(t, service, "batch-1", at, []contracts.ProbeResultRow{
		{Address: "1.1.1.1", StageStatus: "completed", TCPLatencyMS: historyValue(42.0), TestPort: historyValue(443)},
	})
	if !second.Changed || second.Merged != 1 || second.Records != 1 {
		t.Fatalf("second merge = %#v", second)
	}
	records := queryHistoryRecords(t, service)
	if len(records) != 1 || records[0].Address != "1.1.1.1" || records[0].TCPLatencyMS == nil || *records[0].TCPLatencyMS != 42 {
		t.Fatalf("records = %#v", records)
	}
}

func TestMergeResultHistoryBatchKeepsNewerStoredRow(t *testing.T) {
	service, clock := newResultHistoryService(t)
	at := clock.now().UnixMilli()
	newer := []contracts.ProbeResultRow{{Address: "1.1.1.1", StageStatus: "completed", DownloadMbps: historyValue(120.0)}}
	mergeHistoryBatch(t, service, "batch-1", at, newer)

	// 迟到的旧快照:内容不同但测速时间更早,不能把较新的结果改回去。
	stale := mergeHistoryBatch(t, service, "batch-1", at-int64(time.Hour/time.Millisecond), []contracts.ProbeResultRow{
		{Address: "1.1.1.1", StageStatus: "completed", DownloadMbps: historyValue(80.0)},
	})
	if stale.Merged != 0 || stale.Skipped != 1 || stale.Changed {
		t.Fatalf("stale merge = %#v", stale)
	}
	if records := queryHistoryRecords(t, service); records[0].DownloadMbps == nil || *records[0].DownloadMbps != 120 {
		t.Fatalf("较新的记录应保留: %#v", records[0])
	}

	// 同一次快照内分页/筛选重复送来的行(时间相同、内容不同)仍然以最后到达的为准。
	tie := mergeHistoryBatch(t, service, "batch-1", at, []contracts.ProbeResultRow{
		{Address: "1.1.1.1", StageStatus: "completed", DownloadMbps: historyValue(150.0)},
	})
	if tie.Merged != 1 {
		t.Fatalf("同一时间的更新应生效: %#v", tie)
	}
	if records := queryHistoryRecords(t, service); records[0].DownloadMbps == nil || *records[0].DownloadMbps != 150 {
		t.Fatalf("records = %#v", records[0])
	}
}

func TestMergeResultHistoryBatchSkipsRewriteWhenRowsAreUnchanged(t *testing.T) {
	service, clock := newResultHistoryService(t)
	at := clock.now().UnixMilli()
	rows := []contracts.ProbeResultRow{{Address: "1.1.1.1", StageStatus: "completed", DownloadMbps: historyValue(120.5)}}
	mergeHistoryBatch(t, service, "batch-1", at, rows)
	first := historyFileUpdatedAt(t, service)

	clock.at = clock.at.Add(time.Minute)
	outcome := mergeHistoryBatch(t, service, "batch-1", clock.now().UnixMilli(), rows)
	if outcome.Changed || outcome.Merged != 0 || outcome.Skipped != 1 {
		t.Fatalf("unchanged merge = %#v", outcome)
	}
	if updatedAt := historyFileUpdatedAt(t, service); updatedAt != first {
		t.Fatalf("updated_at = %q, want unchanged %q", updatedAt, first)
	}
}

func TestMergeResultHistoryBatchDoesNotCreateFileForRejectedRows(t *testing.T) {
	service, clock := newResultHistoryService(t)
	outcome := mergeHistoryBatch(t, service, "batch-1", clock.now().UnixMilli(), []contracts.ProbeResultRow{
		{Address: "2.2.2.2", StageStatus: "tcp_failed"},
		{Address: "", StageStatus: "completed"},
	})
	if outcome.Changed || outcome.Merged != 0 || outcome.Skipped != 2 || outcome.Records != 0 {
		t.Fatalf("rejected merge = %#v", outcome)
	}
	if _, err := os.Stat(service.resultHistoryPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("没有可写入记录时不应创建历史文件: %v", err)
	}
}

func TestMergeResultHistoryBatchNormalizesRows(t *testing.T) {
	service, clock := newResultHistoryService(t)
	colo := " hkg "
	mergeHistoryBatch(t, service, " batch-1 ", clock.now().UnixMilli(), []contracts.ProbeResultRow{{
		Address:      " 1.1.1.1 ",
		Colo:         &colo,
		SourcePort:   historyValue(0),
		StageStatus:  " completed ",
		TestPort:     historyValue(-1),
		DownloadMbps: historyValue(12.5),
	}})
	records := queryHistoryRecords(t, service)
	if len(records) != 1 {
		t.Fatalf("records = %#v", records)
	}
	record := records[0]
	if record.Address != "1.1.1.1" || record.BatchID != "batch-1" || record.Colo != "HKG" || record.StageStatus != "completed" {
		t.Fatalf("record = %#v", record)
	}
	if record.SourcePort != nil || record.TestPort != nil {
		t.Fatalf("非正数端口应归一为 null: %#v", record)
	}
}

func TestResultHistoryPrunesExpiredRecordsOnWrite(t *testing.T) {
	service, clock := newResultHistoryService(t)
	at := clock.now().UnixMilli()
	expired := at - int64(history.RetentionDays+1)*int64(24*time.Hour/time.Millisecond)
	if err := service.writeResultHistoryRecordsAt(service.resultHistoryPath(), []history.Record{
		{Address: "1.1.1.1", StageStatus: "completed", Timestamp: expired},
		{Address: "2.2.2.2", StageStatus: "completed", Timestamp: at},
	}); err != nil {
		t.Fatal(err)
	}

	page := queryHistoryPage(t, service)
	if page.TotalCount != 2 || len(page.Records) != 1 || page.Records[0].Address != "2.2.2.2" {
		t.Fatalf("读路径应按保留期过滤过期记录: %#v", page)
	}

	outcome := mergeHistoryBatch(t, service, "batch-new", at, []contracts.ProbeResultRow{{Address: "3.3.3.3", StageStatus: "completed"}})
	if outcome.Dropped != 1 || outcome.Records != 2 {
		t.Fatalf("写入应物理清理过期记录: %#v", outcome)
	}
	if page := queryHistoryPage(t, service); page.TotalCount != 2 || len(page.Records) != 2 {
		t.Fatalf("过期记录应已被删除: %#v", page)
	}
}

func TestPruneResultHistoryRecordsKeepsNewestByTimestamp(t *testing.T) {
	records := make([]history.Record, 0, history.MaxRecords+2)
	for index := range history.MaxRecords + 2 {
		records = append(records, history.Record{
			Address:     fmt.Sprintf("10.%d.%d.1", index/65536, (index/256)%256),
			StageStatus: "completed",
			Timestamp:   resultHistoryTestNow.UnixMilli() - int64(index),
		})
	}
	kept, dropped := history.PruneRecords(records, resultHistoryTestNow)
	if dropped != 2 || len(kept) != history.MaxRecords {
		t.Fatalf("dropped = %d, kept = %d", dropped, len(kept))
	}
	// records 按时间从新到旧构造,保留最新 12000 条等于留下 index 0..11999,丢掉最旧的两条。
	if kept[0].Timestamp != records[history.MaxRecords-1].Timestamp || kept[len(kept)-1].Timestamp != records[0].Timestamp {
		t.Fatalf("应按时间保留最新记录: first = %#v, last = %#v", kept[0], kept[len(kept)-1])
	}
}

func TestMergeResultHistoryRecordsCollapsesDuplicateKeys(t *testing.T) {
	service, clock := newResultHistoryService(t)
	at := clock.now().UnixMilli()
	if err := service.writeResultHistoryRecordsAt(service.resultHistoryPath(), []history.Record{
		{Address: "1.1.1.1", BatchID: "batch-1", StageStatus: "completed", Timestamp: at},
		{Address: "1.1.1.1", BatchID: "batch-1", StageStatus: "completed", Timestamp: at + 1, DownloadMbps: historyValue(99.0)},
	}); err != nil {
		t.Fatal(err)
	}

	outcome := mergeHistoryBatch(t, service, "batch-1", at, []contracts.ProbeResultRow{{Address: "1.1.1.1", StageStatus: "completed"}})
	if outcome.Dropped != 1 || outcome.Records != 1 {
		t.Fatalf("重复键应被折叠: %#v", outcome)
	}
	records := queryHistoryRecords(t, service)
	if len(records) != 1 || records[0].DownloadMbps == nil || *records[0].DownloadMbps != 99 {
		t.Fatalf("后出现的重复记录胜出: %#v", records)
	}
}

func TestImportResultHistoryKeepsNewerStoredRecord(t *testing.T) {
	service, clock := newResultHistoryService(t)
	at := clock.now().UnixMilli()
	mergeHistoryBatch(t, service, "batch-1", at, []contracts.ProbeResultRow{{
		Address:      "1.1.1.1",
		StageStatus:  "completed",
		DownloadMbps: historyValue(120.0),
	}})

	outcome := importHistoryRecords(t, service, []history.Record{
		{Address: "1.1.1.1", BatchID: "batch-1", StageStatus: "completed", DownloadMbps: historyValue(80.0), Timestamp: at - int64(time.Hour/time.Millisecond)},
		{Address: "9.9.9.9", StageStatus: "completed", DownloadMbps: historyValue(31.0), Timestamp: at - 2*int64(time.Hour/time.Millisecond)},
	})
	if outcome.Merged != 1 || outcome.Skipped != 1 {
		t.Fatalf("import = %#v", outcome)
	}
	records := queryHistoryRecords(t, service)
	if len(records) != 2 {
		t.Fatalf("records = %#v", records)
	}
	for _, record := range records {
		if record.Address != "1.1.1.1" {
			continue
		}
		if record.DownloadMbps == nil || *record.DownloadMbps != 120 {
			t.Fatalf("导入不应盖掉更新的已存记录: %#v", record)
		}
	}

	// 导入记录比已存记录更新时仍应生效(旧版历史里这条比新版写入的那条更晚)。
	imported := at + 2*int64(time.Minute/time.Millisecond)
	outcome = importHistoryRecords(t, service, []history.Record{{
		Address:      "1.1.1.1",
		BatchID:      "batch-1",
		StageStatus:  "completed",
		DownloadMbps: historyValue(90.0),
		Timestamp:    imported,
	}})
	if outcome.Merged != 1 {
		t.Fatalf("更新的导入记录应替换已存记录: %#v", outcome)
	}
	for _, record := range queryHistoryRecords(t, service) {
		if record.Address != "1.1.1.1" {
			continue
		}
		if record.Timestamp != imported || record.DownloadMbps == nil || *record.DownloadMbps != 90 {
			t.Fatalf("导入记录未生效: %#v", record)
		}
	}
}

func TestImportResultHistoryRejectsOversizedPayload(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	records := make([]history.Record, history.MaxImportRecords+1)
	for index := range records {
		records[index] = history.Record{Address: fmt.Sprintf("10.0.%d.%d", index/256, index%256), StageStatus: "completed", Timestamp: resultHistoryTestNow.UnixMilli()}
	}
	payload, err := json.Marshal(history.ImportRequest{Records: records})
	if err != nil {
		t.Fatal(err)
	}
	result := service.Invoke("history.import", string(payload))
	if result.OK || result.Code != "RESULT_HISTORY_PAYLOAD_INVALID" {
		t.Fatalf("history.import = %#v", result)
	}
}

func TestQueryResultHistoryToleratesCorruptFileWithoutTouchingIt(t *testing.T) {
	service, _ := newResultHistoryService(t)
	path := service.resultHistoryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if page := queryHistoryPage(t, service); page.TotalCount != 0 || len(page.Records) != 0 {
		t.Fatalf("坏文件应按空历史处理: %#v", page)
	}
	if quarantined := historyQuarantineFiles(t, service); len(quarantined) != 0 {
		t.Fatalf("读路径不应改动历史文件: %#v", quarantined)
	}
}

func TestMergeResultHistoryBatchQuarantinesUnreadableFile(t *testing.T) {
	service, clock := newResultHistoryService(t)
	path := service.resultHistoryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	at := clock.now().UnixMilli()
	result := mergeHistoryBatchCommand(t, service, "batch-1", at, []contracts.ProbeResultRow{{Address: "1.1.1.1", StageStatus: "completed"}})
	if !result.OK {
		t.Fatalf("history.merge_batch = %#v", result)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("应带出留档告警: %#v", result.Warnings)
	}
	quarantined := historyQuarantineFiles(t, service)
	if len(quarantined) != 1 {
		t.Fatalf("坏文件应被留档: %#v", quarantined)
	}
	raw, err := os.ReadFile(quarantined[0])
	if err != nil || string(raw) != "not json" {
		t.Fatalf("留档内容 = %q, err = %v", raw, err)
	}
	if records := queryHistoryRecords(t, service); len(records) != 1 || records[0].Address != "1.1.1.1" {
		t.Fatalf("坏文件之后应重建历史: %#v", records)
	}
}

func TestMergeResultHistoryBatchQuarantinesOversizedFile(t *testing.T) {
	service, clock := newResultHistoryService(t)
	path := service.resultHistoryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// 稀疏文件:只占一个上限,却能让读取路径按异常文件处理,不必真写 32MiB。
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, history.MaxFileBytes+1); err != nil {
		t.Fatal(err)
	}

	result := mergeHistoryBatchCommand(t, service, "batch-1", clock.now().UnixMilli(), []contracts.ProbeResultRow{{Address: "1.1.1.1", StageStatus: "completed"}})
	if !result.OK || len(result.Warnings) != 1 {
		t.Fatalf("history.merge_batch = %#v", result)
	}
	if quarantined := historyQuarantineFiles(t, service); len(quarantined) != 1 {
		t.Fatalf("超限文件应被留档: %#v", quarantined)
	}
	if records := queryHistoryRecords(t, service); len(records) != 1 {
		t.Fatalf("超限文件之后应重建历史: %#v", records)
	}
}

func TestMergeResultHistoryRecordsAbortsWhenHistoryFileIsUnreadable(t *testing.T) {
	service, clock := newResultHistoryService(t)
	path := service.resultHistoryPath()
	// 用目录占住历史文件路径:Stat 成功、ReadFile 必然失败,模拟真实读取错误(占用/权限)。
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}

	outcome, _, err := service.mergeResultHistoryRecords(history.RecordsFromRows("batch-1", clock.now().UnixMilli(), []contracts.ProbeResultRow{{Address: "1.1.1.1", StageStatus: "completed"}}))
	if err == nil || outcome.Changed {
		t.Fatalf("读失败必须中止写入: outcome = %#v, err = %v", outcome, err)
	}
	info, statErr := os.Stat(path)
	if statErr != nil || !info.IsDir() {
		t.Fatalf("读失败时不应改动原路径: %v, %v", info, statErr)
	}
	if quarantined := historyQuarantineFiles(t, service); len(quarantined) != 0 {
		t.Fatalf("读失败不是留档场景: %#v", quarantined)
	}
}

func TestMergeResultHistoryRecordsSerializesConcurrentWrites(t *testing.T) {
	service, clock := newResultHistoryService(t)
	at := clock.now().UnixMilli()
	var waitGroup sync.WaitGroup
	for index := range 8 {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			records := history.RecordsFromRows(fmt.Sprintf("batch-%d", index), at, []contracts.ProbeResultRow{{
				Address:     fmt.Sprintf("10.1.%d.1", index),
				StageStatus: "completed",
			}})
			if _, _, err := service.mergeResultHistoryRecords(records); err != nil {
				t.Errorf("merge: %v", err)
			}
		}(index)
	}
	waitGroup.Wait()
	if records := queryHistoryRecords(t, service); len(records) != 8 {
		t.Fatalf("并发写入丢记录: %d", len(records))
	}
}

func TestResultHistoryCommandsRejectInvalidPayloads(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	for _, command := range []string{"history.merge_batch", "history.import", "history.query"} {
		result := service.Invoke(command, "{")
		if result.OK || result.Code != "RESULT_HISTORY_PAYLOAD_INVALID" {
			t.Fatalf("%s = %#v", command, result)
		}
	}
}

// TestResultHistoryRecordJSONMatchesFrontendContract 把 wire 字段名钉在测试里:
// 前端的 ResultHistoryRecord 直接消费这些键,改名等于静默丢字段。
func TestResultHistoryRecordJSONMatchesFrontendContract(t *testing.T) {
	raw, err := json.Marshal(history.Record{})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	want := []string{"address", "batch_id", "colo", "download_mbps", "max_download_mbps", "source_port", "stage_status", "tcp_latency_ms", "test_port", "timestamp", "trace_latency_ms"}
	for key := range decoded {
		if !slices.Contains(want, key) {
			t.Fatalf("未登记的历史字段 %q", key)
		}
	}
	for _, key := range want {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("缺少历史字段 %q", key)
		}
	}
}
