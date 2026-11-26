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

func mergeHistoryBatch(t *testing.T, service *Service, batchID string, at int64, rows []ProbeResultRow) resultHistoryMergeResult {
	t.Helper()
	payload, err := json.Marshal(resultHistoryMergeRequest{At: at, BatchID: batchID, Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	result := service.Invoke("history.merge_batch", string(payload))
	if !result.OK {
		t.Fatalf("history.merge_batch = %#v", result)
	}
	outcome, ok := result.Data.(resultHistoryMergeResult)
	if !ok {
		t.Fatalf("history.merge_batch data = %#v", result.Data)
	}
	return outcome
}

func importHistoryRecords(t *testing.T, service *Service, records []resultHistoryRecord) resultHistoryMergeResult {
	t.Helper()
	payload, err := json.Marshal(resultHistoryImportRequest{Records: records})
	if err != nil {
		t.Fatal(err)
	}
	result := service.Invoke("history.import", string(payload))
	if !result.OK {
		t.Fatalf("history.import = %#v", result)
	}
	outcome, ok := result.Data.(resultHistoryMergeResult)
	if !ok {
		t.Fatalf("history.import data = %#v", result.Data)
	}
	return outcome
}

func queryHistoryPage(t *testing.T, service *Service) resultHistoryPage {
	t.Helper()
	result := service.Invoke("history.query", "{}")
	if !result.OK {
		t.Fatalf("history.query = %#v", result)
	}
	page, ok := result.Data.(resultHistoryPage)
	if !ok {
		t.Fatalf("history.query data = %#v", result.Data)
	}
	return page
}

func queryHistoryRecords(t *testing.T, service *Service) []resultHistoryRecord {
	t.Helper()
	return queryHistoryPage(t, service).Records
}

func historyFileUpdatedAt(t *testing.T, service *Service) string {
	t.Helper()
	raw, err := os.ReadFile(service.resultHistoryPath())
	if err != nil {
		t.Fatal(err)
	}
	var file resultHistoryFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	return file.UpdatedAt
}

func TestMergeResultHistoryBatchReplacesSameKeyWithoutDuplicating(t *testing.T) {
	service, clock := newResultHistoryService(t)
	at := clock.now().UnixMilli()
	first := mergeHistoryBatch(t, service, "batch-1", at, []ProbeResultRow{
		{Address: "1.1.1.1", StageStatus: "completed", TCPLatencyMS: historyValue(30.0), TestPort: historyValue(443)},
		{Address: "2.2.2.2", StageStatus: "tcp_failed"},
		{Address: "   ", StageStatus: "completed"},
	})
	if !first.Changed || first.Merged != 1 || first.Skipped != 2 {
		t.Fatalf("first merge = %#v", first)
	}

	second := mergeHistoryBatch(t, service, "batch-1", at, []ProbeResultRow{
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

func TestMergeResultHistoryBatchSkipsRewriteWhenRowsAreUnchanged(t *testing.T) {
	service, clock := newResultHistoryService(t)
	at := clock.now().UnixMilli()
	rows := []ProbeResultRow{{Address: "1.1.1.1", StageStatus: "completed", DownloadMbps: historyValue(120.5)}}
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
	outcome := mergeHistoryBatch(t, service, "batch-1", clock.now().UnixMilli(), []ProbeResultRow{
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
	mergeHistoryBatch(t, service, " batch-1 ", clock.now().UnixMilli(), []ProbeResultRow{{
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
	expired := at - int64(resultHistoryRetentionDays+1)*int64(24*time.Hour/time.Millisecond)
	if err := service.writeResultHistoryRecords([]resultHistoryRecord{
		{Address: "1.1.1.1", StageStatus: "completed", Timestamp: expired},
		{Address: "2.2.2.2", StageStatus: "completed", Timestamp: at},
	}); err != nil {
		t.Fatal(err)
	}

	page := queryHistoryPage(t, service)
	if page.TotalCount != 2 || len(page.Records) != 1 || page.Records[0].Address != "2.2.2.2" {
		t.Fatalf("读路径应按保留期过滤过期记录: %#v", page)
	}

	outcome := mergeHistoryBatch(t, service, "batch-new", at, []ProbeResultRow{{Address: "3.3.3.3", StageStatus: "completed"}})
	if outcome.Dropped != 1 || outcome.Records != 2 {
		t.Fatalf("写入应物理清理过期记录: %#v", outcome)
	}
	if page := queryHistoryPage(t, service); page.TotalCount != 2 || len(page.Records) != 2 {
		t.Fatalf("过期记录应已被删除: %#v", page)
	}
}

func TestPruneResultHistoryRecordsKeepsNewestByTimestamp(t *testing.T) {
	records := make([]resultHistoryRecord, 0, resultHistoryMaxRecords+2)
	for index := range resultHistoryMaxRecords + 2 {
		records = append(records, resultHistoryRecord{
			Address:     fmt.Sprintf("10.%d.%d.1", index/65536, (index/256)%256),
			StageStatus: "completed",
			Timestamp:   resultHistoryTestNow.UnixMilli() - int64(index),
		})
	}
	kept, dropped := pruneResultHistoryRecords(records, resultHistoryTestNow)
	if dropped != 2 || len(kept) != resultHistoryMaxRecords {
		t.Fatalf("dropped = %d, kept = %d", dropped, len(kept))
	}
	// records 按时间从新到旧构造,保留最新 12000 条等于留下 index 0..11999,丢掉最旧的两条。
	if kept[0].Timestamp != records[resultHistoryMaxRecords-1].Timestamp || kept[len(kept)-1].Timestamp != records[0].Timestamp {
		t.Fatalf("应按时间保留最新记录: first = %#v, last = %#v", kept[0], kept[len(kept)-1])
	}
}

func TestImportResultHistoryKeepsNewerStoredRecord(t *testing.T) {
	service, clock := newResultHistoryService(t)
	at := clock.now().UnixMilli()
	mergeHistoryBatch(t, service, "batch-1", at, []ProbeResultRow{{
		Address:      "1.1.1.1",
		StageStatus:  "completed",
		DownloadMbps: historyValue(120.0),
	}})

	outcome := importHistoryRecords(t, service, []resultHistoryRecord{
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
	outcome = importHistoryRecords(t, service, []resultHistoryRecord{{
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

func TestResultHistoryToleratesCorruptFile(t *testing.T) {
	service, clock := newResultHistoryService(t)
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
	outcome := mergeHistoryBatch(t, service, "batch-1", clock.now().UnixMilli(), []ProbeResultRow{{Address: "1.1.1.1", StageStatus: "completed"}})
	if outcome.Merged != 1 || outcome.Records != 1 {
		t.Fatalf("坏文件之后仍应能写入: %#v", outcome)
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
			records := resultHistoryRecordsFromRows(fmt.Sprintf("batch-%d", index), at, []ProbeResultRow{{
				Address:     fmt.Sprintf("10.1.%d.1", index),
				StageStatus: "completed",
			}})
			if _, err := service.mergeResultHistoryRecords(records, false); err != nil {
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
	raw, err := json.Marshal(resultHistoryRecord{})
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
