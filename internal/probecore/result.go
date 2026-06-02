package probecore

import (
	"fmt"
	"sort"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

type SourceSummary struct {
	CandidateCount int      `json:"candidateCount"`
	DuplicateCount int      `json:"duplicateCount"`
	Duplicates     []string `json:"duplicates"`
	Invalid        []string `json:"invalid"`
	InvalidCount   int      `json:"invalidCount"`
	RawLineCount   int      `json:"rawLineCount"`
	UniqueCount    int      `json:"uniqueCount"`
	Valid          []string `json:"valid"`
	ValidCount     int      `json:"validCount"`
}

type ProbeRow struct {
	Colo               string  `json:"colo"`
	DelayMS            float64 `json:"delayMs"`
	DownloadSpeedMB    float64 `json:"downloadSpeedMb"`
	IP                 string  `json:"ip"`
	LossRate           float64 `json:"lossRate"`
	MaxDownloadSpeedMB float64 `json:"maxDownloadSpeedMb"`
	Received           int     `json:"received"`
	Sended             int     `json:"sended"`
	SourcePort         int     `json:"source_port,omitempty"`
	TestPort           int     `json:"test_port"`
	TraceDelayMS       float64 `json:"traceDelayMs"`
}

type ProbeSummary struct {
	AverageDelayMS float64 `json:"averageDelayMs"`
	BestIP         string  `json:"bestIp"`
	BestSpeedMB    float64 `json:"bestSpeedMb"`
	Failed         int     `json:"failed"`
	Passed         int     `json:"passed"`
	Total          int     `json:"total"`
}

func ConvertProbeRow(item utils.CloudflareIPData, sourcePort int, testPort int) ProbeRow {
	lossRate := 0.0
	if item.Sended > 0 {
		lossRate = float64(item.Sended-item.Received) / float64(item.Sended)
	}
	colo := item.Colo
	if colo == "" {
		colo = "N/A"
	}
	return ProbeRow{
		Colo:               colo,
		DelayMS:            utils.DurationMilliseconds(item.Delay),
		DownloadSpeedMB:    utils.DownloadSpeedMBPerSecond(item.DownloadSpeed),
		IP:                 item.IP.String(),
		LossRate:           lossRate,
		MaxDownloadSpeedMB: utils.DownloadSpeedMBPerSecond(utils.DownloadSpeedForMetric(item, utils.DownloadSpeedMetricMax)),
		Received:           item.Received,
		Sended:             item.Sended,
		SourcePort:         sourcePort,
		TestPort:           testPort,
		TraceDelayMS:       utils.DurationMilliseconds(item.HeadDelay),
	}
}

func SummarizeProbeRows(rows []ProbeRow, total int) ProbeSummary {
	summary := ProbeSummary{Failed: total - len(rows), Passed: len(rows), Total: total}
	if summary.Failed < 0 {
		summary.Failed = 0
	}
	if len(rows) == 0 {
		return summary
	}
	var delay float64
	for _, row := range rows {
		delay += row.DelayMS
	}
	summary.AverageDelayMS = utils.RoundMetricToTwoDecimals(delay / float64(len(rows)))
	summary.BestIP = rows[0].IP
	summary.BestSpeedMB = rows[0].DownloadSpeedMB
	return summary
}

func LimitFinalResults(data []utils.CloudflareIPData, limit int, metric ...string) []utils.CloudflareIPData {
	if limit <= 0 || len(data) <= 1 {
		return data
	}
	selectedMetric := utils.DownloadSpeedMetricAverage
	if len(metric) > 0 {
		selectedMetric = metric[0]
	}
	return utils.SelectTopWeightedResultsByMetric(data, limit, selectedMetric)
}

func LimitFinalProbeResults(raw []utils.CloudflareIPData, rows []ProbeRow, limit int, metric string) ([]utils.CloudflareIPData, []ProbeRow) {
	if limit <= 0 || len(raw) <= 1 || len(raw) != len(rows) {
		return raw, rows
	}
	type scoredResult struct {
		index int
		item  utils.CloudflareIPData
		score float64
	}
	minDelay, maxDelay := raw[0].Delay, raw[0].Delay
	firstSpeed := utils.DownloadSpeedForMetric(raw[0], metric)
	minSpeed, maxSpeed := firstSpeed, firstSpeed
	for _, item := range raw[1:] {
		if item.Delay < minDelay {
			minDelay = item.Delay
		}
		if item.Delay > maxDelay {
			maxDelay = item.Delay
		}
		itemSpeed := utils.DownloadSpeedForMetric(item, metric)
		if itemSpeed < minSpeed {
			minSpeed = itemSpeed
		}
		if itemSpeed > maxSpeed {
			maxSpeed = itemSpeed
		}
	}
	scored := make([]scoredResult, 0, len(raw))
	for index, item := range raw {
		itemSpeed := utils.DownloadSpeedForMetric(item, metric)
		delayScore := 1.0
		if maxDelay > minDelay {
			delayScore = float64(maxDelay-item.Delay) / float64(maxDelay-minDelay)
		}
		speedScore := 0.0
		if maxSpeed > minSpeed {
			speedScore = (itemSpeed - minSpeed) / (maxSpeed - minSpeed)
		} else if maxSpeed > 0 {
			speedScore = 1.0
		}
		scored = append(scored, scoredResult{index: index, item: item, score: delayScore*0.3 + speedScore*0.7})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		iSpeed, jSpeed := utils.DownloadSpeedForMetric(scored[i].item, metric), utils.DownloadSpeedForMetric(scored[j].item, metric)
		if iSpeed != jSpeed {
			return iSpeed > jSpeed
		}
		if scored[i].item.Delay != scored[j].item.Delay {
			return scored[i].item.Delay < scored[j].item.Delay
		}
		iLossRate := lossRate(scored[i].item)
		jLossRate := lossRate(scored[j].item)
		if iLossRate != jLossRate {
			return iLossRate < jLossRate
		}
		return scored[i].item.IP.String() < scored[j].item.IP.String()
	})
	selectedLimit := len(scored)
	if limit > 0 && limit < selectedLimit {
		selectedLimit = limit
	}
	selectedRaw := make([]utils.CloudflareIPData, 0, selectedLimit)
	selectedRows := make([]ProbeRow, 0, selectedLimit)
	for _, scoredItem := range scored[:selectedLimit] {
		selectedRaw = append(selectedRaw, raw[scoredItem.index])
		selectedRows = append(selectedRows, rows[scoredItem.index])
	}
	return selectedRaw, selectedRows
}

func SelectTopProbeRowsByMetric(rows []ProbeRow, limit int, metric string) []ProbeRow {
	if limit <= 0 || len(rows) <= 1 {
		return rows
	}
	type scoredRow struct {
		index int
		row   ProbeRow
		score float64
	}
	selectedMetric := utils.DownloadSpeedMetricAverage
	if strings.TrimSpace(metric) != "" {
		selectedMetric = metric
	}
	minDelay, maxDelay := rows[0].DelayMS, rows[0].DelayMS
	firstSpeed := probeRowSpeedForMetric(rows[0], selectedMetric)
	minSpeed, maxSpeed := firstSpeed, firstSpeed
	for _, row := range rows[1:] {
		if row.DelayMS < minDelay {
			minDelay = row.DelayMS
		}
		if row.DelayMS > maxDelay {
			maxDelay = row.DelayMS
		}
		speed := probeRowSpeedForMetric(row, selectedMetric)
		if speed < minSpeed {
			minSpeed = speed
		}
		if speed > maxSpeed {
			maxSpeed = speed
		}
	}
	scored := make([]scoredRow, 0, len(rows))
	for index, row := range rows {
		speed := probeRowSpeedForMetric(row, selectedMetric)
		delayScore := 1.0
		if maxDelay > minDelay {
			delayScore = (maxDelay - row.DelayMS) / (maxDelay - minDelay)
		}
		speedScore := 0.0
		if maxSpeed > minSpeed {
			speedScore = (speed - minSpeed) / (maxSpeed - minSpeed)
		} else if maxSpeed > 0 {
			speedScore = 1.0
		}
		scored = append(scored, scoredRow{index: index, row: row, score: delayScore*0.3 + speedScore*0.7})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		iSpeed := probeRowSpeedForMetric(scored[i].row, selectedMetric)
		jSpeed := probeRowSpeedForMetric(scored[j].row, selectedMetric)
		if iSpeed != jSpeed {
			return iSpeed > jSpeed
		}
		if scored[i].row.DelayMS != scored[j].row.DelayMS {
			return scored[i].row.DelayMS < scored[j].row.DelayMS
		}
		if scored[i].row.LossRate != scored[j].row.LossRate {
			return scored[i].row.LossRate < scored[j].row.LossRate
		}
		return scored[i].row.IP < scored[j].row.IP
	})
	selectedLimit := len(scored)
	if limit > 0 && limit < selectedLimit {
		selectedLimit = limit
	}
	selected := make([]ProbeRow, 0, selectedLimit)
	for _, item := range scored[:selectedLimit] {
		selected = append(selected, rows[item.index])
	}
	return selected
}

func BuildProbeWarnings(source SourceSummary) []string {
	warnings := make([]string, 0)
	if source.InvalidCount > 0 {
		warnings = append(warnings, fmt.Sprintf("已忽略 %d 条非法 IP/CIDR/域名。", source.InvalidCount))
	}
	if source.DuplicateCount > 0 {
		warnings = append(warnings, fmt.Sprintf("已忽略 %d 条重复候选。", source.DuplicateCount))
	}
	return warnings
}

func lossRate(item utils.CloudflareIPData) float64 {
	if item.Sended <= 0 {
		return 1.0
	}
	return float64(item.Sended-item.Received) / float64(item.Sended)
}

func probeRowSpeedForMetric(row ProbeRow, metric string) float64 {
	if metric == utils.DownloadSpeedMetricMax {
		return row.MaxDownloadSpeedMB
	}
	return row.DownloadSpeedMB
}
