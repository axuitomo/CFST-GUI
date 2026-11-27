package contracts

import (
	"context"
	"time"
)

const (
	EventSchemaVersion = "cfst-gui-event-v2"
	ProbeEventChannel  = "probe:event"
)

type Clock func() time.Time

// UploadNotificationEvent 是 App 层补发系统通知的事件名，载荷为 UploadNotification。
const UploadNotificationEvent = "upload.notification"

type ProbeEvent struct {
	Event         string         `json:"event"`
	Payload       map[string]any `json:"payload"`
	SchemaVersion string         `json:"schema_version"`
	Seq           int            `json:"seq"`
	TaskID        string         `json:"task_id"`
	TS            string         `json:"ts"`
}

type EventSink interface {
	EmitProbeEvent(context.Context, ProbeEvent)
}

type EventSinkFunc func(context.Context, ProbeEvent)

func (fn EventSinkFunc) EmitProbeEvent(ctx context.Context, event ProbeEvent) {
	fn(ctx, event)
}
