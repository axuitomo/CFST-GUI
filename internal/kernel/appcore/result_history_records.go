package appcore

import (
	"math"
	"sort"
	"strings"
	"time"
)

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
