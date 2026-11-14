package appcore

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
)

// coloDictionaryWarmupTimeout 给启动补词典的下载兜个上限：网络卡住时不至于
// 让后台 goroutine 和它的连接留一整天。写文件是原子的，超时中断也不会写坏词典。
const coloDictionaryWarmupTimeout = 3 * time.Minute

// StartColoDictionaryWarmup 要求对当前数据目录做一次 COLO 词典自检：缺了就自动补一份。
//
// 异步执行、失败只记录、不阻塞启动。按数据目录去重：同一个目录只检查一次，
// 换目录（Android 旧版 self-hosted 存储的迁移路径会调 SetStorageLayout）会立刻再检查。
// 首次调用之后的目录变化会自动重跑，未调用过则完全不参与，测试与库用法不受影响。
func (s *Service) StartColoDictionaryWarmup() bool {
	root := strings.TrimSpace(s.StorageLayout().Root)
	s.coloWarmupMu.Lock()
	s.coloWarmupArmed = true
	if s.coloWarmupRoot == root {
		s.coloWarmupMu.Unlock()
		return false
	}
	s.coloWarmupRoot = root
	s.coloWarmupFailure = "" // 换了目录，上一个目录的失败说明不再适用
	s.coloWarmupMu.Unlock()
	go s.warmupColoDictionary(context.Background(), root)
	return true
}

// warmupColoDictionary 同步执行一次「检查 → 需要就补」；测试直接调用它。
// root 由调用方固定，检查过程中目录再变也不会写错位置。
func (s *Service) warmupColoDictionary(ctx context.Context, root string) {
	paths := probeColoPaths(root)
	if paths.Colo == "" {
		return
	}

	if coloDictionaryReady(paths) {
		return // 词典已就绪，每次启动只读三份小 CSV
	}
	started := time.Now()
	warmupCtx, cancel := context.WithTimeout(ctx, coloDictionaryWarmupTimeout)
	defer cancel()
	result, err := s.coloDictionaryRefresh()(warmupCtx, paths)
	if err != nil {
		message := fmt.Sprintf("COLO 词典缺失且自动更新失败：%v", err)
		s.setColoDictionaryFailure(root, message)
		s.DebugLogger().Event("colo.dictionary.warmup", map[string]any{
			"colo_path": paths.Colo, "duration_ms": time.Since(started).Milliseconds(),
			"error": err.Error(), "geofeed_path": paths.Geofeed, "level": "error",
			"message": message, "reason": "colo_dictionary_warmup_failed",
		})
		return
	}
	s.setColoDictionaryFailure(root, "")
	s.DebugLogger().Event("colo.dictionary.warmup", map[string]any{
		"colo_path":   paths.Colo,
		"counts":      map[string]any{"colo_rows": result.ColoRows, "geofeed_rows": result.GeofeedRows},
		"duration_ms": time.Since(started).Milliseconds(),
		"message":     "启动时补齐 COLO 词典。", "reason": "colo_dictionary_warmup_done",
	})
}

// ColoDictionaryWarning 返回启动自检留下的失败说明；成功或未执行时为空。
// 探测结果会带上它，用户不开调试日志也能知道 COLO 为什么是空的。
func (s *Service) ColoDictionaryWarning() string {
	s.coloWarmupMu.Lock()
	defer s.coloWarmupMu.Unlock()
	return s.coloWarmupFailure
}

// setColoDictionaryFailure 记录失败说明；root 已不是当前数据目录时丢弃。
// 切换目录时旧目录的自检可能还在跑（上限 3 分钟），它的结论不该覆盖新目录。
func (s *Service) setColoDictionaryFailure(root, message string) {
	if root != strings.TrimSpace(s.StorageLayout().Root) {
		return
	}
	s.coloWarmupMu.Lock()
	s.coloWarmupFailure = strings.TrimSpace(message)
	s.coloWarmupMu.Unlock()
}

// clearColoDictionaryFailure 在词典被别的路径修好后清掉旧说明。
// 手动“更新词典 / 处理”成功即算修好：否则断网启动过一次之后，本次会话每次测速都会
// 继续提示词典缺失，而词典其实已经在位。
func (s *Service) clearColoDictionaryFailure() {
	s.coloWarmupMu.Lock()
	s.coloWarmupFailure = ""
	s.coloWarmupMu.Unlock()
}

func (s *Service) coloDictionaryRefresh() func(context.Context, colodict.Paths) (colodict.UpdateResult, error) {
	s.mu.RLock()
	refresh := s.options.ColoDictionaryRefresh
	s.mu.RUnlock()
	if refresh != nil {
		return refresh
	}
	client := s.nonStageHTTPClient()
	return func(ctx context.Context, paths colodict.Paths) (colodict.UpdateResult, error) {
		return refreshColoDictionary(ctx, client, paths)
	}
}

// probeColoPaths 返回探测实际读取的 COLO 词典路径，必须与 NewProbeEngine 传的是同一份。
func probeColoPaths(root string) colodict.Paths {
	root = strings.TrimSpace(root)
	if root == "" {
		return colodict.Paths{}
	}
	return colodict.DefaultPaths(root)
}

// maybeRewarmColoDictionary 在数据目录变化后补一次自检。
// 只在应用启动过自检（armed）时才动作：库用法和测试不会因为换目录突然发起下载。
func (s *Service) maybeRewarmColoDictionary() {
	s.coloWarmupMu.Lock()
	armed := s.coloWarmupArmed
	s.coloWarmupMu.Unlock()
	if armed {
		s.StartColoDictionaryWarmup()
	}
}

// coloDictionaryReady 判定词典是不是真的能加载：主表供 IP 查 COLO，
// IPv4/IPv6 分表供按地址族的筛选使用（ColoPathForTokens 只看路径非空，
// 缺表时筛选会直接报“COLO 文件不存在”）。任何一份读不出来都算没就绪。
func coloDictionaryReady(paths colodict.Paths) bool {
	for _, path := range []string{paths.Colo, paths.ColoIPv4, paths.ColoIPv6} {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if _, err := colodict.LoadColoEntries(path); err != nil {
			return false
		}
	}
	return strings.TrimSpace(paths.Colo) != ""
}

// refreshColoDictionary 本地已有 geofeed 时只重建表，省掉一次下载；
// geofeed 坏掉或缺失时重新下载再重建。
func refreshColoDictionary(ctx context.Context, client *http.Client, paths colodict.Paths) (colodict.UpdateResult, error) {
	if _, err := os.Stat(paths.Geofeed); err == nil {
		if result, processErr := colodict.Process(colodict.UpdateOptions{Paths: paths}); processErr == nil {
			return result, nil
		}
	}
	updated, err := colodict.Update(ctx, colodict.UpdateOptions{Client: client, Paths: paths})
	if err != nil {
		return colodict.UpdateResult{}, err
	}
	processed, err := colodict.Process(colodict.UpdateOptions{Paths: paths})
	if err != nil {
		return colodict.UpdateResult{}, err
	}
	processed.Warnings = append(updated.Warnings, processed.Warnings...)
	return processed, nil
}
