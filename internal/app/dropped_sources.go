package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore"
)

const (
	droppedSourcesCommand = "sources.dropped.consume"
	droppedSourcesEvent   = "sources:dropped"
	maxDroppedSourceFiles = 8
	// 与输入源文件读取上限一致，超过这个大小的文件按旧参数处理并提示。
	maxDroppedSourceBytes = 32 << 20
)

// startupDroppedSources 由 Run 在创建 GUI 前写入，NewApp 取走后清空。
var startupDroppedSources []string

// droppedSourceFiles 只在参数全部是普通文件时返回绝对路径；
// 任何 flag、目录、空文件或超限文件都返回 nil，让调用方走旧参数拒绝。
func droppedSourceFiles(args []string) []string {
	if len(args) == 0 || len(args) > maxDroppedSourceFiles {
		return nil
	}
	paths := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return nil
		}
		info, err := os.Stat(arg)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxDroppedSourceBytes {
			return nil
		}
		absolute, err := filepath.Abs(arg)
		if err != nil {
			return nil
		}
		if _, exists := seen[absolute]; exists {
			continue
		}
		seen[absolute] = struct{}{}
		paths = append(paths, absolute)
	}
	return paths
}

// secondInstanceDroppedSources 从 Wails 转发的完整 os.Args 里取第二实例的文件参数。
func secondInstanceDroppedSources(args []string) []string {
	if len(args) <= 1 {
		return nil
	}
	return droppedSourceFiles(args[1:])
}

// droppedSourcesState 保存等待前端领取的拖放文件，前端领取后清空。
type droppedSourcesState struct {
	mu    sync.Mutex
	paths []string
}

func (state *droppedSourcesState) add(paths []string) {
	if len(paths) == 0 {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	seen := make(map[string]struct{}, len(state.paths))
	for _, path := range state.paths {
		seen[path] = struct{}{}
	}
	for _, path := range paths {
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		state.paths = append(state.paths, path)
	}
}

func (state *droppedSourcesState) consume() []string {
	state.mu.Lock()
	defer state.mu.Unlock()
	paths := state.paths
	state.paths = nil
	if paths == nil {
		return []string{}
	}
	return paths
}

func (a *App) consumeDroppedSourcesResult() appcore.CommandResult {
	return appcore.NewCommandResult("SOURCES_DROPPED_CONSUMED", map[string]any{
		"paths": a.droppedSources.consume(),
	}, "", true, nil, nil)
}
