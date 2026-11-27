package history

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
)

const (
	// RetentionDays must stay aligned with frontend RESULT_HISTORY_RETENTION_DAYS.
	RetentionDays = 30
	// MaxRecords is the history record cap, retaining newest entries on prune.
	MaxRecords       = 12000
	MaxImportRecords = MaxRecords * 2
	MaxFileBytes     = contracts.MaxTaskResultsBytes
	SchemaVersion    = "cfst-gui-result-history-v1"
	QuarantineHint   = "历史文件无法读取"
)

type Record struct {
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

type MergeResult struct {
	Changed bool `json:"changed"`
	Dropped int  `json:"dropped"`
	Merged  int  `json:"merged"`
	Records int  `json:"records"`
	Skipped int  `json:"skipped"`
}

type Page struct {
	Records    []Record `json:"records"`
	TotalCount int      `json:"total_count"`
}

type MergeRequest struct {
	At      int64                      `json:"at"`
	BatchID string                     `json:"batch_id"`
	Rows    []contracts.ProbeResultRow `json:"rows"`
}

type ImportRequest struct {
	Records []Record `json:"records"`
}

type File struct {
	Records       []Record `json:"records"`
	SchemaVersion string   `json:"schema_version"`
	UpdatedAt     string   `json:"updated_at"`
}

type Key struct {
	Address     string
	BatchID     string
	SourcePort  int
	StageStatus string
	TestPort    int
}

// PruneRecords drops entries past the retention window and caps at MaxRecords
// sorted by timestamp ascending (retaining the newest entries).
func PruneRecords(records []Record, now time.Time) ([]Record, int) {
	cutoff := now.AddDate(0, 0, -RetentionDays).UnixMilli()
	kept := make([]Record, 0, len(records))
	for _, record := range records {
		if record.Timestamp >= cutoff {
			kept = append(kept, record)
		}
	}
	if len(kept) > MaxRecords {
		sort.SliceStable(kept, func(left, right int) bool {
			return kept[left].Timestamp < kept[right].Timestamp
		})
		kept = kept[len(kept)-MaxRecords:]
	}
	return kept, len(records) - len(kept)
}

func RecordKey(record Record) Key {
	return Key{
		Address:     record.Address,
		BatchID:     record.BatchID,
		SourcePort:  optionalPortValue(record.SourcePort),
		StageStatus: record.StageStatus,
		TestPort:    optionalPortValue(record.TestPort),
	}
}

func SameRecord(left, right Record) bool {
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

func NormalizeRecord(record Record) (Record, bool) {
	address := strings.TrimSpace(record.Address)
	stageStatus := strings.TrimSpace(record.StageStatus)
	if address == "" || record.Timestamp <= 0 || strings.Contains(stageStatus, "failed") {
		return Record{}, false
	}
	if stageStatus == "" {
		stageStatus = "completed"
	}
	return Record{
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

func RecordsFromRows(batchID string, at int64, rows []contracts.ProbeResultRow) []Record {
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		records = append(records, Record{
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
