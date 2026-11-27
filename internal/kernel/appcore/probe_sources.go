package appcore

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/sources"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/doh"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/colodict"
	mcisengine "github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/mcis/engine"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/sourceparse"
)

func (s *Service) prepareProbeSources(ctx context.Context, payload contracts.ProbePayload, cfg probecore.ProbeConfig, taskID string) (sources.PreparedSources, probecore.SourceSummary, int, error) {
	started := s.now()
	s.waitProbePaused(taskID, "stage0_pool", "")
	client := s.sourceHTTPClient(cfg)
	// 阶段 0 读源期间没有其它事件，每读完一个源发一次进度，避免界面长时间无反馈。
	// 输入源并行读取，计数必须是原子的；进度值表示"已完成数"，不再表示"正在读第几个"。
	var processed atomic.Int64
	total := len(payload.Sources)
	prepared := sources.PrepareSources(sources.PrepareSourcesOptions{
		Context: ctx,
		Config:  cfg,
		ProcessSource: func(source contracts.Source) (sources.SourceProcessResult, error) {
			s.waitProbePaused(taskID, "stage0_pool", "")
			sourceStarted := s.now()
			result, err := s.processProbeSource(ctx, cfg, source, client, started, taskID)
			s.logSourceObservation(taskID, source, result, err, s.now().Sub(sourceStarted))
			// 取消后不再发进度：PrepareSources 可能在取消瞬间就返回，
			// 慢源要到超时才退出，此时再发进度会在界面上盖掉「已终止」。
			if !s.probeCancelled(ctx, taskID) {
				s.emitProbeProgress(taskID, "stage0_pool", int(processed.Add(1)), 0, 0, total, 0)
			}
			return result, err
		},
		Sources: payload.Sources,
	})
	if prepared.Error != nil || s.probeCancelled(ctx, taskID) {
		s.publishProbeCancelled(taskID, "stage0_pool", "")
		return prepared, probecore.SourceSummary{}, 0, probecore.ErrProbeCanceled
	}
	if err := s.PersistSourceStatuses(prepared.SourceStatuses); err != nil {
		prepared.Warnings = append(prepared.Warnings, fmt.Sprintf("更新输入源状态失败：%v", err))
	}
	// 阶段 0 的候选已经展开成 IP：这里只做网络键去重，不再重新解析整份文本。
	summary, poolTruncated := probecore.CapSourceSummary(probecore.SummarizeEntries(prepared.Entries), probecore.MaxSourcePoolLimit)
	// 单源内部重复在构池时已丢弃，不补回来会让"过滤"永远只统计跨源重复。
	summary.DuplicateCount += prepared.DuplicateCount
	prepared.Text = strings.Join(summary.Valid, "\n")
	if poolTruncated {
		prepared.Warnings = append(prepared.Warnings, fmt.Sprintf("聚合候选池超过绝对上限 %d 条，已截断；请降低输入源 ip_limit 或 source_pool_limit。", probecore.MaxSourcePoolLimit))
	}
	invalidCount := summary.InvalidCount + prepared.InvalidCount
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.preprocessed", TaskID: taskID, Payload: map[string]any{
		"accepted": summary.ValidCount, "filtered": summary.DuplicateCount, "invalid": invalidCount,
		// total 必须等于 accepted+filtered+invalid，否则界面会显示"候选数小于接受数+过滤数"。
		"source_statuses": prepared.SourceStatuses, "stage": "stage0_pool", "total": summary.ValidCount + summary.DuplicateCount + invalidCount,
	}})
	if len(prepared.FatalErrors) > 0 || (strings.TrimSpace(prepared.Text) == "" && len(prepared.Warnings) > 0) {
		messages := prepared.FatalErrors
		if len(messages) == 0 {
			messages = prepared.Warnings
		}
		err := errors.New(strings.Join(messages, "；"))
		s.logProbePreparationFailure(cfg, taskID, summary, invalidCount, prepared.SourceStatuses, s.now().Sub(started), err)
		s.publishProbeFailure(taskID, err, "stage0_pool", s.probeDebugLogPath(cfg))
		return prepared, summary, invalidCount, err
	}
	return prepared, summary, invalidCount, nil
}

func (s *Service) processProbeSource(ctx context.Context, cfg probecore.ProbeConfig, source contracts.Source, client *http.Client, now time.Time, taskID string) (sources.SourceProcessResult, error) {
	return sources.ProcessSource(source, cfg, client, now,
		func(source contracts.Source, cfg probecore.ProbeConfig, client *http.Client) (sources.SourceContentResult, error) {
			return sources.LoadSourceContentContext(ctx, source, cfg, client, sharedSourceContentLoadOptions())
		},
		func(raw string, source contracts.Source, cfg probecore.ProbeConfig) (probecore.SourceBuildResult, error) {
			s.mu.RLock()
			limit := s.options.ProbeConfigOptions.DefaultSourceIPLimit
			layout := s.options.Storage
			paths := s.options.ColoPaths
			s.mu.RUnlock()
			if limit <= 0 {
				limit = probecore.DefaultConfigSnapshotSourceIPLimit
			}
			if paths.Colo == "" {
				paths = colodict.DefaultPaths(layout.Root)
			}
			resolver := s.sourceResolver(cfg)
			return sources.BuildSourceEntriesWithConfig(sources.SourceEntryBuildOptions{
				Context: ctx, Raw: raw, Source: source, Config: cfg, DefaultIPLimit: limit,
				Resolver: resolver, ColoDictionaryPaths: paths,
				MCISRunner: func(tokens []string, source contracts.Source, cfg probecore.ProbeConfig, limit int) ([]probecore.MCISCandidate, []string, error) {
					return sources.RunMCISSearchCandidatesContextWithProgress(ctx, tokens, source, cfg, limit, func(progress mcisengine.Progress) {
						s.emitMCISProgress(taskID, source, progress)
					})
				},
			})
		},
	)
}

func (s *Service) PersistSourceStatuses(statuses []contracts.SourceStatus) error {
	if len(statuses) == 0 {
		return nil
	}
	byID := make(map[string]contracts.SourceStatus, len(statuses))
	for _, status := range statuses {
		if id := strings.TrimSpace(status.ID); id != "" {
			byID[id] = status
		}
	}
	if len(byID) == 0 {
		return nil
	}
	// 读-改-写必须串行，否则与并发的 config.save 互相覆盖（SaveConfig 只是原子写文件）。
	s.configMu.Lock()
	defer s.configMu.Unlock()
	loaded, err := s.LoadConfig()
	if err != nil || !loaded.Existed {
		return err
	}
	if !applySourceStatuses(loaded.Snapshot, byID) {
		return nil
	}
	_, err = s.saveConfigLocked(loaded.Snapshot)
	return err
}

// applySourceStatuses 只就地更新状态字段。不能经过 []Source 结构体往返，
// 否则 sources 里本版本不认识的键会在每次写入时被静默丢掉。
func applySourceStatuses(snapshot map[string]any, byID map[string]contracts.SourceStatus) bool {
	changed := false
	for _, entry := range sourceSnapshotItems(snapshot["sources"]) {
		status, ok := byID[strings.TrimSpace(stringValue(entry["id"], ""))]
		if !ok {
			continue
		}
		entry["last_fetched_at"] = status.LastFetchedAt
		entry["last_fetched_count"] = status.LastFetchedCount
		entry["status_text"] = status.StatusText
		changed = true
	}
	return changed
}

// sourceSnapshotItems 返回 sources 数组里的 map 元素（用于原地修改），
// 兼容快照里可能出现的 []any 与 []map[string]any 两种形状。
func sourceSnapshotItems(value any) []map[string]any {
	switch items := value.(type) {
	case []any:
		result := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if entry, ok := item.(map[string]any); ok {
				result = append(result, entry)
			}
		}
		return result
	case []map[string]any:
		return items
	default:
		return nil
	}
}

func (s *Service) sourceHTTPClient(cfg probecore.ProbeConfig) *http.Client {
	s.mu.RLock()
	client, timeout := s.options.HTTPClient, s.options.SourceHTTPTimeout
	s.mu.RUnlock()
	if client != nil && client != http.DefaultClient {
		return client
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return sources.NewSourceHTTPClient(cfg, sources.SourceHTTPClientOptions{Timeout: timeout})
}

func resolveServiceProbeSource(cfg probecore.ProbeConfig, raw string, resolver sourceparse.Resolver) (string, probecore.SourceSummary, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		text = strings.TrimSpace(cfg.IPText)
	}
	if text == "" {
		loaded, err := sources.LoadLocalSourceFile(context.Background(), cfg.IPFile, contracts.MaxSourceContentBytes)
		if err != nil {
			return "", probecore.SourceSummary{}, fmt.Errorf("读取 IP 数据文件失败：%w", err)
		}
		text = loaded.Raw
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return text, probecore.SummarizeSource(text, resolver), nil
}

// sourceResolver prefers the configured DoH endpoint and falls back to the
// injected resolver (the system one by default).
func (s *Service) sourceResolver(cfg probecore.ProbeConfig) sourceparse.Resolver {
	s.mu.RLock()
	fallback := s.options.SourceResolver
	s.mu.RUnlock()
	if endpoint := strings.TrimSpace(cfg.DoHURL); endpoint != "" {
		if resolver, err := doh.New(endpoint); err == nil {
			// DoH 端点本身失败时用注入的本地解析器兜底，默认就是系统 DNS。
			if fallback != nil {
				resolver.Fallback = fallback.LookupIPAddr
			}
			return resolver
		}
	}
	if fallback == nil {
		return net.DefaultResolver
	}
	return fallback
}

func sharedSourceContentLoadOptions() sources.SourceContentLoadOptions {
	return sources.SourceContentLoadOptions{
		BuildAttempts: func(primary string, source contracts.Source) []sources.RemoteSourceAttempt {
			if cdn, ok := githubRawToJSDelivrURL(primary); ok && cdn != primary {
				return []sources.RemoteSourceAttempt{{URL: primary}, {URL: cdn}}
			}
			return []sources.RemoteSourceAttempt{{URL: primary}}
		},
		ShouldRetry: func(status int, err error) bool {
			return err != nil && (status == 0 || status == http.StatusTooManyRequests || status >= 500)
		},
		OnFallbackSuccess: func(primary string, used sources.RemoteSourceAttempt, source contracts.Source) []string {
			if used.URL == primary {
				return nil
			}
			return []string{fmt.Sprintf("输入源 %s 已通过 jsDelivr CDN 兜底读取。", sources.SourceName(source))}
		},
	}
}

func githubRawToJSDelivrURL(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Host, "raw.githubusercontent.com") {
		return "", false
	}
	segments := splitURLPath(parsed.Path)
	if len(segments) < 4 {
		return "", false
	}
	branchIndex := 2
	if len(segments) >= 6 && segments[2] == "refs" && segments[3] == "heads" {
		branchIndex = 4
	}
	owner, repo, branch := segments[0], segments[1], segments[branchIndex]
	files := segments[branchIndex+1:]
	if owner == "" || repo == "" || branch == "" || len(files) == 0 {
		return "", false
	}
	return (&url.URL{Scheme: "https", Host: "cdn.jsdelivr.net", Path: "/gh/" + strings.Join(append([]string{owner, repo + "@" + branch}, files...), "/")}).String(), true
}

func splitURLPath(value string) []string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

var _ sourceparse.Resolver = net.DefaultResolver
