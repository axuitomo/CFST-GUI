package appcore

import (
	"context"
	"fmt"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func (s *Service) inspectSource(request SourcePreviewRequest, fetch bool) CommandResult {
	if !HasSourceInput(request.Source) {
		return NewCommandResult("SOURCE_INPUT_EMPTY", nil, "输入源缺少可读取的内容。", false, nil, nil)
	}

	s.mu.RLock()
	options := s.options.ProbeConfigOptions
	s.mu.RUnlock()
	options.Now = s.now()
	cfg, configWarnings := probecore.ConfigSnapshotToProbeConfig(cloneServiceMap(request.Config), options)
	result, err := s.processProbeSource(context.Background(), cfg, request.Source, s.sourceHTTPClient(cfg), s.now(), "")
	warnings := append(configWarnings, result.Warnings...)
	if err != nil {
		return NewCommandResult("SOURCE_READ_FAILED", nil, err.Error(), false, nil, warnings)
	}

	persist := fetch || request.PersistState
	if persist {
		if err := s.PersistSourceStatuses([]SourceStatus{result.Status}); err != nil {
			warnings = append(warnings, fmt.Sprintf("更新输入源状态失败：%v", err))
		}
	}

	previewLimit := request.PreviewLimit
	if previewLimit <= 0 {
		previewLimit = 16
	}
	previewEntries := result.Entries
	if len(previewEntries) > previewLimit {
		previewEntries = previewEntries[:previewLimit]
	}
	action := "预览"
	if persist {
		action = "抓取"
	}

	return NewCommandResult("SOURCE_PREVIEW_READY", map[string]any{
		"preview_entries": previewEntries,
		"port_summary":    probecore.PortSummary(result.Entries, result.SourcePorts, cfg.TCPPort, cfg.PortPolicy),
		"source_status":   result.Status,
		"summary": map[string]any{
			"action":        action,
			"invalid_count": result.InvalidCount,
			"mode":          SourceIPMode(request.Source),
			"name":          SourceName(request.Source),
			"total_count":   len(result.Entries),
		},
	}, fmt.Sprintf("%s已完成，可预览 %d 条候选。", action, len(previewEntries)), true, nil, warnings)
}
