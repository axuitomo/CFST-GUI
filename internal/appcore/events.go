package appcore

import "context"

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
