package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// 结果页跨批次历史(近 30 天)的持久化层。前端只保留展示管道(时间范围、去重、
// 筛选、排序),合并、过期清理与上限裁剪都在这里:桌面、WebUI 与 Android 因此共用
// 同一份历史,不再各自锁在宿主自己的 localStorage 里。
const (
	// resultHistoryRetentionDays 必须与前端 RESULT_HISTORY_RETENTION_DAYS 保持一致。
	resultHistoryRetentionDays = 30
	// resultHistoryMaxRecords 是历史记录上限,超出时按测速时间保留最新记录。
	resultHistoryMaxRecords = 12000
	// resultHistoryMaxImportRecords 是单次 history.import 可接受的记录数:旧版
	// localStorage 历史上限就是 12000 条,这里留一倍余量,再多按异常输入拒绝。
	resultHistoryMaxImportRecords = resultHistoryMaxRecords * 2
	// resultHistoryMaxFileBytes 是历史文件的可读上限;满仓(12000 条)约 3.9MiB。
	resultHistoryMaxFileBytes   = MaxTaskResultsBytes
	resultHistorySchemaVersion  = "cfst-gui-result-history-v1"
	resultHistoryQuarantineHint = "历史文件无法读取"
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
	// Dropped 是因为过期、超过上限或与合并键重复而丢掉的记录数。
	Dropped int `json:"dropped"`
	Merged  int `json:"merged"`
	Records int `json:"records"`
	Skipped int `json:"skipped"`
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

// loadResultHistoryRecordsAt 读并归一化历史文件。
//
// 缺文件按空历史处理。读取本身失败(文件被占用、权限、磁盘错误)原样返回错误,由调用方
// 放弃这次写入——把读失败当成空历史,下一次合并就会把整份历史覆盖掉。内容读不回来
// (超过上限或 JSON 坏掉)时,quarantine 为真则把文件改名留档后按空历史重建,返回给用户的
// 告警;读路径传 false,保持查询无副作用。
func (s *Service) loadResultHistoryRecordsAt(path string, quarantine bool) ([]resultHistoryRecord, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", err
	}
	oversize := info.Size() > resultHistoryMaxFileBytes
	if oversize && !quarantine {
		return nil, "", nil
	}
	if oversize {
		cause := fmt.Errorf("历史文件超过 %d 字节上限", resultHistoryMaxFileBytes)
		warning, qErr := s.quarantineResultHistoryFile(path, cause)
		return nil, warning, qErr
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", err
	}
	var file resultHistoryFile
	if _, err := UnmarshalJSONCompat(raw, &file); err != nil {
		if !quarantine {
			return nil, "", nil
		}
		warning, qErr := s.quarantineResultHistoryFile(path, err)
		return nil, warning, qErr
	}

	records := make([]resultHistoryRecord, 0, len(file.Records))
	for _, record := range file.Records {
		if normalized, ok := normalizeResultHistoryRecord(record); ok {
			records = append(records, normalized)
		}
	}
	return records, "", nil
}

// quarantineResultHistoryFile 把读不回来的历史文件改名留档,返回给用户看的告警。
// 改名失败时返回原因,让调用方按原样失败:宁可这次合并不落盘,也不能覆盖掉现场。
func (s *Service) quarantineResultHistoryFile(path string, cause error) (string, error) {
	backupPath := path + ".corrupt-" + s.now().Format("20060102-150405.000")
	if err := os.Rename(path, backupPath); err != nil {
		return "", fmt.Errorf("%s(%v),留档失败:%w", resultHistoryQuarantineHint, cause, err)
	}
	return fmt.Sprintf("%s(%v),已留档为 %s 并重建。", resultHistoryQuarantineHint, cause, filepath.Base(backupPath)), nil
}

func (s *Service) writeResultHistoryRecordsAt(path string, records []resultHistoryRecord) error {
	file := resultHistoryFile{Records: records, SchemaVersion: resultHistorySchemaVersion, UpdatedAt: s.now().Format(time.RFC3339)}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, raw, 0o600)
}

// queryResultHistory 返回保留期内的全部历史记录。读路径没有副作用:过期记录只在
// 写入时物理清理,坏文件也不在这里留档,只按空历史返回。
func (s *Service) queryResultHistory() resultHistoryPage {
	s.resultHistoryMu.Lock()
	path := s.resultHistoryPath()
	stored, _, _ := s.loadResultHistoryRecordsAt(path, false)
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
//
// 只有内容不同且测速时间不更旧的记录才会替换已存记录:同批次分页、排序、筛选会重复
// 送来同样的行,而迟到的旧快照不该把真正较新的测速结果改回去。返回的告警用于把
// 「原文件已留档重建」这类需要用户知道的事带出去。
func (s *Service) mergeResultHistoryRecords(incoming []resultHistoryRecord) (resultHistoryMergeResult, []string, error) {
	s.resultHistoryMu.Lock()
	defer s.resultHistoryMu.Unlock()

	now := s.now()
	// 路径在临界区里解析一次:换数据目录时读和写必须落在同一个文件上。
	path := s.resultHistoryPath()
	stored, warning, err := s.loadResultHistoryRecordsAt(path, true)
	if err != nil {
		return resultHistoryMergeResult{}, nil, err
	}
	warnings := []string{}
	if warning != "" {
		warnings = append(warnings, warning)
	}

	// 读进来时就按合并键折叠:旧版本或外部改过的文件可能已经存在重复键,不折叠就会
	// 每次合并都把重复项原样写回去。
	loaded := len(stored)
	deduped := make([]resultHistoryRecord, 0, len(stored))
	positions := make(map[resultHistoryKey]int, len(stored))
	for _, record := range stored {
		key := resultHistoryRecordKey(record)
		if position, exists := positions[key]; exists {
			deduped[position] = record
			continue
		}
		positions[key] = len(deduped)
		deduped = append(deduped, record)
	}
	stored = deduped

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
		if sameResultHistoryRecord(existing, record) || existing.Timestamp > record.Timestamp {
			result.Skipped++
			continue
		}
		stored[position] = record
		result.Merged++
		changed = true
	}

	retained, expired := pruneResultHistoryRecords(stored, now)
	result.Dropped = expired + (loaded - len(deduped))
	result.Records = len(retained)
	result.Changed = changed || result.Dropped > 0
	if !result.Changed {
		return result, warnings, nil
	}
	if err := s.writeResultHistoryRecordsAt(path, retained); err != nil {
		return resultHistoryMergeResult{}, warnings, err
	}
	return result, warnings, nil
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
		return s.mergeResultHistoryCommand("HISTORY_MERGED", resultHistoryRecordsFromRows(request.BatchID, at, request.Rows), "历史记录已合并。")
	case "history.import":
		request, err := decodeCommandPayload[resultHistoryImportRequest](payloadJSON)
		if err != nil {
			return NewCommandResult("RESULT_HISTORY_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		}
		if len(request.Records) > resultHistoryMaxImportRecords {
			return NewCommandResult("RESULT_HISTORY_PAYLOAD_INVALID", nil, fmt.Sprintf("导入记录数超过上限 %d", resultHistoryMaxImportRecords), false, nil, nil)
		}
		return s.mergeResultHistoryCommand("HISTORY_IMPORTED", request.Records, "历史记录已导入。")
	case "history.query":
		if _, err := decodeCommandPayload[struct{}](payloadJSON); err != nil {
			return NewCommandResult("RESULT_HISTORY_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		}
		return NewCommandResult("HISTORY_LISTED", s.queryResultHistory(), "历史记录已读取。", true, nil, nil)
	default:
		return NewCommandResult("COMMAND_UNKNOWN", nil, fmt.Sprintf("unknown command: %s", command), false, nil, nil)
	}
}

func (s *Service) mergeResultHistoryCommand(code string, records []resultHistoryRecord, message string) CommandResult {
	result, warnings, err := s.mergeResultHistoryRecords(records)
	if err != nil {
		// 未落盘时不带回计数:调用方不该从 data 里读到「合并成功」的样子。
		return NewCommandResult("RESULT_HISTORY_FAILED", nil, err.Error(), false, nil, warnings)
	}
	return NewCommandResult(code, result, message, true, nil, warnings)
}
