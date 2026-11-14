package appcore

import (
	"context"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/task"
)

// 一次探测里逐颗 IP 产生的 403 淘汰必须合并成窗口内的一条事件：逐条发布在桌面端是
// 一条一次 WebView2 eval，几千颗 IP 就是几千次。这里同时锁住两件事——窗口内不发布、
// 窗口到期后一批一起发布，且顺序在触发发布的那条事件之前。
func TestEdgeRejectionsMergeIntoOneEventAfterBatchWindow(t *testing.T) {
	events := make([]ProbeEvent, 0, 8)
	now := time.Date(2026, time.November, 20, 10, 0, 0, 0, time.UTC)
	service := NewService(ServiceOptions{
		Clock:   func() time.Time { return now },
		Storage: StorageLayout{Root: t.TempDir()},
		EventSink: EventSinkFunc(func(_ context.Context, event ProbeEvent) {
			events = append(events, event)
		}),
	})

	service.publishEdgeRejected("task-1", task.EdgeRejection{Stage: "stage3_get", IP: "1.1.1.1", Note: "403", Reason: "forbidden"})
	service.publishEdgeRejected("task-1", task.EdgeRejection{Stage: "stage3_get", IP: "2.2.2.2", Note: "1034", Reason: "edge_ip_restricted", Ignored: true})
	if len(events) != 0 {
		t.Fatalf("edge rejections must wait for the batch window, got %d immediate events", len(events))
	}

	now = now.Add(edgeRejectionBatchWindow + time.Millisecond)
	if err := service.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.progress", TaskID: "task-1", Payload: map[string]any{"stage": "stage3_get", "processed": 2, "total": 10}}); err != nil {
		t.Fatal(err)
	}

	if len(events) != 2 {
		t.Fatalf("events = %d, want batched rejection + progress", len(events))
	}
	batch := events[0]
	if batch.Event != "probe.edge_rejected" {
		t.Fatalf("first event = %q, want the flushed rejection batch before the triggering event", batch.Event)
	}
	items, ok := batch.Payload["items"].([]map[string]any)
	if !ok || len(items) != 2 {
		t.Fatalf("batched items = %#v, want 2 items", batch.Payload["items"])
	}
	if batch.Payload["count"] != 2 || batch.Payload["batched"] != true {
		t.Fatalf("batch metadata = %#v", batch.Payload)
	}
	// 旧形态字段保留最后一条，兼容只读单条字段的消费方。
	if batch.Payload["ip"] != "2.2.2.2" || batch.Payload["note"] != "1034" {
		t.Fatalf("legacy single-item fields = %#v", batch.Payload)
	}
	if items[0]["ip"] != "1.1.1.1" || items[1]["ignored"] != true {
		t.Fatalf("item order/details = %#v", items)
	}
	// 合并发生在序号分配之前：seq 必须连续，否则前端按 gap 判定会整轮重拉。
	if batch.Seq != 1 || events[1].Seq != 2 {
		t.Fatalf("seqs = %d,%d, want contiguous 1,2", batch.Seq, events[1].Seq)
	}
}

func TestEdgeRejectionsWaitForTheWindow(t *testing.T) {
	events := make([]ProbeEvent, 0, 4)
	now := time.Date(2026, time.November, 20, 10, 0, 0, 0, time.UTC)
	service := NewService(ServiceOptions{
		Clock:     func() time.Time { return now },
		Storage:   StorageLayout{Root: t.TempDir()},
		EventSink: EventSinkFunc(func(_ context.Context, event ProbeEvent) { events = append(events, event) }),
	})

	service.publishEdgeRejected("task-2", task.EdgeRejection{Stage: "stage3_get", IP: "3.3.3.3", Note: "403", Reason: "forbidden"})
	now = now.Add(edgeRejectionBatchWindow / 2)
	_ = service.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.progress", TaskID: "task-2", Payload: map[string]any{"stage": "stage3_get"}})

	if len(events) != 1 || events[0].Event != "probe.progress" {
		t.Fatalf("events inside the window = %#v, want only the progress event", events)
	}
	if got := service.edgeRejections.pendingCount(); got != 1 {
		t.Fatalf("pending rejections = %d, want 1 still queued", got)
	}
}

// 探测以终态事件收尾时，最后一批明细必须强制下发，否则这些被拒 IP 永远不上界面。
func TestTerminalEventFlushesPendingEdgeRejections(t *testing.T) {
	events := make([]ProbeEvent, 0, 4)
	now := time.Date(2026, time.November, 20, 10, 0, 0, 0, time.UTC)
	service := NewService(ServiceOptions{
		Clock:     func() time.Time { return now },
		Storage:   StorageLayout{Root: t.TempDir()},
		EventSink: EventSinkFunc(func(_ context.Context, event ProbeEvent) { events = append(events, event) }),
	})

	service.publishEdgeRejected("task-3", task.EdgeRejection{Stage: "stage3_get", IP: "4.4.4.4", Note: "403", Reason: "forbidden"})
	_ = service.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.completed", TaskID: "task-3", Payload: map[string]any{"passed": 0, "failed": 1}})

	if len(events) != 2 || events[0].Event != "probe.edge_rejected" || events[1].Event != "probe.completed" {
		t.Fatalf("events = %#v, want rejection batch then probe.completed", events)
	}
	if got := service.edgeRejections.pendingCount(); got != 0 {
		t.Fatalf("pending rejections after terminal event = %d, want 0", got)
	}
}

// 淘汰事件不改变任务快照内容，却会按默认分支写一次 status/updated_at。落盘必须跟着
// 进度类事件一起被节流，否则几千颗被拒 IP 就是几千次带 fsync 的快照重写。
func TestEdgeRejectionSnapshotWriteIsCoalesced(t *testing.T) {
	if !isCoalescableTaskEvent("probe.edge_rejected") {
		t.Fatal("probe.edge_rejected must be coalescable, otherwise every rejection rewrites the task snapshot")
	}
}
