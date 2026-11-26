package appcore

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// 结果页跨批次历史(近 30 天)的持久化层。前端只保留展示管道(时间范围、去重、
// 筛选、排序),合并、过期清理与上限裁剪都在这里:桌面、WebUI 与 Android 因此共用
// 同一份历史,不再各自锁在宿主自己的 localStorage 里。
const (
	// resultHistoryRetentionDays 必须与前端 RESULT_HISTORY_RETENTION_DAYS 保持一致。
	resultHistoryRetentionDays = 30
	// resultHistoryMaxRecords 是历史记录上限,超出时按测速时间保留最新记录。
	resultHistoryMaxRecords    = 12000
	resultHistorySchemaVersion = "cfst-gui-result-history-v1"
)

// resultHistoryRecord 是一条可用结果;字段名与前端 ResultHistoryRecord 同名同义。
type resultHistoryRecord struct {
	Address         string   `json:"address"`
	BatchID         string   `json:"batch_id"`
	Colo            string   `json:"colo"`
	DownloadMbps    *float64 `json:"download_mbps"`
	MaxDownloadMbps *float64 `json:"max_download_mbps"`
	SourcePort      *int     `json:"source_port"`
	StageStatus     string   `json:"stage_status"`
	TCPLatencyMS    *float64 `json:"tcp_latency_ms"`
	TestPort        *int     `json:"test_port"`
	Timestamp       int64    `json:"timestamp"`
	TraceLatencyMS  *float64 `json:"trace_latency_ms"`
}

type resultHistoryMergeResult struct {
	Changed bool `json:"changed"`
	Dropped int  `json:"dropped"`
	Merged  int  `json:"merged"`
	Records int  `json:"records"`
	Skipped int  `json:"skipped"`
}

type resultHistoryPage struct {
	Records []resultHistoryRecord `json:"records"`
	// TotalCount 是文件内记录总数,含读路径按保留期过滤掉的过期记录。
	TotalCount int `json:"total_count"`
}

type resultHistoryMergeRequest struct {
	At      int64            `json:"at"`
	BatchID string           `json:"batch_id"`
	Rows    []ProbeResultRow `json:"rows"`
}

type resultHistoryImportRequest struct {
	Records []resultHistoryRecord `json:"records"`
}

type resultHistoryFile struct {
	Records       []resultHistoryRecord `json:"records"`
	SchemaVersion string                `json:"schema_version"`
	UpdatedAt     string                `json:"updated_at"`
}

// resultHistoryKey 是合并键:批次 + IP + 阶段 + 端口。用结构体而不是拼接字符串,
// 避免分隔符出现在 IP 或批次号里造成误合并。
type resultHistoryKey struct {
	Address     string
	BatchID     string
	SourcePort  int
	StageStatus string
	TestPort    int
}

func (s *Service) resultHistoryPath() string {
	return s.StorageLayout().ResultHistoryPath()
}

// readResultHistoryRecords 读历史文件:缺文件、坏 JSON 或结构不符一律按空历史处理,
// 与任务快照读取同一取向——单个坏文件不能让整个结果页不可用。
func (s *Service) readResultHistoryRecords() []resultHistoryRecord {
	raw, err := os.ReadFile(s.resultHistoryPath())
	if err != nil {
		return nil
	}
	var file resultHistoryFile
	if _, err := UnmarshalJSONCompat(raw, &file); err != nil {
		return nil
	}
	records := make([]resultHistoryRecord, 0, len(file.Records))
	for _, record := range file.Records {
		if normalized, ok := normalizeResultHistoryRecord(record); ok {
			records = append(records, normalized)
		}
	}
	return records
}

func (s *Service) writeResultHistoryRecords(records []resultHistoryRecord) error {
	file := resultHistoryFile{Records: records, SchemaVersion: resultHistorySchemaVersion, UpdatedAt: s.now().Format(time.RFC3339)}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(s.resultHistoryPath(), raw, 0o600)
}

// queryResultHistory 返回保留期内的全部历史记录。读路径没有副作用:过期记录只在
// 写入时物理清理,这里只按保留期过滤,不让一次查询把文件改成半清理的中间状态。
func (s *Service) queryResultHistory() resultHistoryPage {
	s.resultHistoryMu.Lock()
	stored := s.readResultHistoryRecords()
	s.resultHistoryMu.Unlock()
	cutoff := s.now().AddDate(0, 0, -resultHistoryRetentionDays).UnixMilli()
	page := resultHistoryPage{Records: make([]resultHistoryRecord, 0, len(stored)), TotalCount: len(stored)}
	for _, record := range stored {
		if record.Timestamp >= cutoff {
			page.Records = append(page.Records, record)
		}
	}
	return page
}

// mergeResultHistoryRecords 是唯一的写入实现:history.merge_batch(探测行)与
// history.import(旧 localStorage 迁移)共用它,合并键、过期清理与上限规则只此一份。
// preferNewer 为真时,已存记录的测速时间更新就保留已存记录,迁移不会盖掉新版写入的历史。
func (s *Service) mergeResultHistoryRecords(incoming []resultHistoryRecord, preferNewer bool) (resultHistoryMergeResult, error) {
	s.resultHistoryMu.Lock()
	defer s.resultHistoryMu.Unlock()

	stored := s.readResultHistoryRecords()
	positions := make(map[resultHistoryKey]int, len(stored))
	for position, record := range stored {
		positions[resultHistoryRecordKey(record)] = position
	}
	result := resultHistoryMergeResult{}
	changed := false
	for _, candidate := range incoming {
		record, ok := normalizeResultHistoryRecord(candidate)
		if !ok {
			result.Skipped++
			continue
		}
		key := resultHistoryRecordKey(record)
		position, exists := positions[key]
		if !exists {
			positions[key] = len(stored)
			stored = append(stored, record)
			result.Merged++
			changed = true
			continue
		}
		existing := stored[position]
		// 内容没变就不重写整份文件:同一次任务运行中快照时间一直在变。
		if sameResultHistoryRecord(existing, record) || (preferNewer && existing.Timestamp > record.Timestamp) {
			result.Skipped++
			continue
		}
		stored[position] = record
		result.Merged++
		changed = true
	}
	retained, dropped := pruneResultHistoryRecords(stored, s.now())
	result.Dropped = dropped
	result.Records = len(retained)
	result.Changed = changed || dropped > 0
	if !result.Changed {
		return result, nil
	}
	if err := s.writeResultHistoryRecords(retained); err != nil {
		return result, err
	}
	return result, nil
}

// pruneResultHistoryRecords 丢掉超过保留期的记录;仍超过上限时按测速时间保留最新的,
// 而不是按存储位置。返回丢弃条数,让调用方判断是否需要整份重写。
func pruneResultHistoryRecords(records []resultHistoryRecord, now time.Time) ([]resultHistoryRecord, int) {
	cutoff := now.AddDate(0, 0, -resultHistoryRetentionDays).UnixMilli()
	kept := make([]resultHistoryRecord, 0, len(records))
	for _, record := range records {
		if record.Timestamp >= cutoff {
			kept = append(kept, record)
		}
	}
	if len(kept) > resultHistoryMaxRecords {
		sort.SliceStable(kept, func(left, right int) bool {
			return kept[left].Timestamp < kept[right].Timestamp
		})
		kept = kept[len(kept)-resultHistoryMaxRecords:]
	}
	return kept, len(records) - len(kept)
}

func resultHistoryRecordKey(record resultHistoryRecord) resultHistoryKey {
	return resultHistoryKey{
		Address:     record.Address,
		BatchID:     record.BatchID,
		SourcePort:  optionalPortValue(record.SourcePort),
		StageStatus: record.StageStatus,
		TestPort:    optionalPortValue(record.TestPort),
	}
}

// sameResultHistoryRecord 比较记录内容,时间戳故意不参与:同批次分页、排序、筛选
// 只会重复送来同样的行,比时间会让每次刷新都整份重写。
func sameResultHistoryRecord(left, right resultHistoryRecord) bool {
	return left.Address == right.Address &&
		left.BatchID == right.BatchID &&
		left.Colo == right.Colo &&
		sameOptionalFloat(left.DownloadMbps, right.DownloadMbps) &&
		sameOptionalFloat(left.MaxDownloadMbps, right.MaxDownloadMbps) &&
		optionalPortValue(left.SourcePort) == optionalPortValue(right.SourcePort) &&
		left.StageStatus == right.StageStatus &&
		sameOptionalFloat(left.TCPLatencyMS, right.TCPLatencyMS) &&
		optionalPortValue(left.TestPort) == optionalPortValue(right.TestPort) &&
		sameOptionalFloat(left.TraceLatencyMS, right.TraceLatencyMS)
}

// normalizeResultHistoryRecord 是记录的唯一入口校验:失败行没有可比较的测速数据,
// 留着只会挤占上限;空地址与没有时间戳的记录无法参与时间范围与去重。
func normalizeResultHistoryRecord(record resultHistoryRecord) (resultHistoryRecord, bool) {
	address := strings.TrimSpace(record.Address)
	stageStatus := strings.TrimSpace(record.StageStatus)
	if address == "" || record.Timestamp <= 0 || strings.Contains(stageStatus, "failed") {
		return resultHistoryRecord{}, false
	}
	if stageStatus == "" {
		stageStatus = "completed"
	}
	return resultHistoryRecord{
		Address:         address,
		BatchID:         strings.TrimSpace(record.BatchID),
		Colo:            strings.ToUpper(strings.TrimSpace(record.Colo)),
		DownloadMbps:    optionalFiniteFloat(record.DownloadMbps),
		MaxDownloadMbps: optionalFiniteFloat(record.MaxDownloadMbps),
		SourcePort:      optionalPositivePort(record.SourcePort),
		StageStatus:     stageStatus,
		TCPLatencyMS:    optionalFiniteFloat(record.TCPLatencyMS),
		TestPort:        optionalPositivePort(record.TestPort),
		Timestamp:       record.Timestamp,
		TraceLatencyMS:  optionalFiniteFloat(record.TraceLatencyMS),
	}, true
}

func resultHistoryRecordsFromRows(batchID string, at int64, rows []ProbeResultRow) []resultHistoryRecord {
	records := make([]resultHistoryRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, resultHistoryRecord{
			Address:         row.Address,
			BatchID:         batchID,
			Colo:            optionalStringValue(row.Colo),
			DownloadMbps:    row.DownloadMbps,
			MaxDownloadMbps: row.MaxDownloadMbps,
			SourcePort:      row.SourcePort,
			StageStatus:     row.StageStatus,
			TCPLatencyMS:    row.TCPLatencyMS,
			TestPort:        row.TestPort,
			Timestamp:       at,
			TraceLatencyMS:  row.TraceLatencyMS,
		})
	}
	return records
}

func (s *Service) invokeResultHistory(command, payloadJSON string) CommandResult {
	switch command {
	case "history.merge_batch":
		request, err := decodeCommandPayload[resultHistoryMergeRequest](payloadJSON)
		if err != nil {
			return NewCommandResult("RESULT_HISTORY_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		}
		at := request.At
		if at <= 0 {
			at = s.now().UnixMilli()
		}
		records := resultHistoryRecordsFromRows(request.BatchID, at, request.Rows)
		return s.mergeResultHistoryCommand("HISTORY_MERGED", records, false, "历史记录已合并。")
	case "history.import":
		request, err := decodeCommandPayload[resultHistoryImportRequest](payloadJSON)
		if err != nil {
			return NewCommandResult("RESULT_HISTORY_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		}
		return s.mergeResultHistoryCommand("HISTORY_IMPORTED", request.Records, true, "历史记录已导入。")
	case "history.query":
		if _, err := decodeCommandPayload[struct{}](payloadJSON); err != nil {
			return NewCommandResult("RESULT_HISTORY_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		}
		return NewCommandResult("HISTORY_LISTED", s.queryResultHistory(), "历史记录已读取。", true, nil, nil)
	default:
		return NewCommandResult("COMMAND_UNKNOWN", nil, fmt.Sprintf("unknown command: %s", command), false, nil, nil)
	}
}

func (s *Service) mergeResultHistoryCommand(code string, records []resultHistoryRecord, preferNewer bool, message string) CommandResult {
	result, err := s.mergeResultHistoryRecords(records, preferNewer)
	if err != nil {
		return NewCommandResult("RESULT_HISTORY_WRITE_FAILED", result, err.Error(), false, nil, nil)
	}
	return NewCommandResult(code, result, message, true, nil, nil)
}

func optionalPortValue(port *int) int {
	if port == nil || *port <= 0 {
		return 0
	}
	return *port
}

func optionalPositivePort(port *int) *int {
	if port == nil || *port <= 0 {
		return nil
	}
	copied := *port
	return &copied
}

func optionalFiniteFloat(value *float64) *float64 {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return nil
	}
	copied := *value
	return &copied
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func sameOptionalFloat(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
