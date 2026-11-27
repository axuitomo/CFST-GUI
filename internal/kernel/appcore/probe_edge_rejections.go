package appcore

import (
	"sync"
	"time"
)

// edgeRejectionBatchWindow 是 403 / 1034 淘汰事件向界面合并的窗口。
//
// 这类事件逐颗 IP 产生：一条几千 IP 的源、且都被 Cloudflare 边缘拒绝时，一次探测
// 会发出几千条 probe.edge_rejected。桌面端每条都要过一次 WebView2 的 eval（Wails 的
// 事件就是把 payload 拼进脚本再执行），WebUI 端每一条是一帧 SSE，代价与条数成正比。
// 合并成窗口内一条事件后，界面仍然能列出每一颗被拒的 IP，只是每条事件带回一批明细。
//
// 合并发生在序号分配之前（publishEdgeRejected 不再直接发布），因此 seq 保持连续，
// 前端不会把合并当成事件丢失（probeEventSequence 的 gap 判定会触发整轮重拉）。
const edgeRejectionBatchWindow = 250 * time.Millisecond

// edgeRejectionEntry 是一条待合并的淘汰明细，taskID 用于把不同任务的明细分开下发。
type edgeRejectionEntry struct {
	taskID  string
	payload map[string]any
}

// edgeRejectionBatch 累积窗口内的淘汰明细，由 publishMu 保护的上层在发布任何事件前
// 取走已满窗口的批次。不另起 goroutine：探测期间进度事件本身就在持续发布，取走时机足够。
type edgeRejectionBatch struct {
	mu      sync.Mutex
	pending []edgeRejectionEntry
	since   time.Time
}

func (b *edgeRejectionBatch) add(taskID string, payload map[string]any, now time.Time) {
	if b == nil || payload == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.pending) == 0 {
		b.since = now
	}
	b.pending = append(b.pending, edgeRejectionEntry{taskID: taskID, payload: payload})
}

// takeReady 返回已满一个窗口的明细；force 供终态事件前强制收尾，否则最后一批明细
// 会一直压到下一次发布才出现，探测结束后界面看起来少了那些 IP。
func (b *edgeRejectionBatch) takeReady(now time.Time, force bool) []edgeRejectionEntry {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.pending) == 0 {
		return nil
	}
	if !force && now.Sub(b.since) < edgeRejectionBatchWindow {
		return nil
	}
	entries := b.pending
	b.pending = nil
	b.since = time.Time{}
	return entries
}

func (b *edgeRejectionBatch) pendingCount() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.pending)
}
