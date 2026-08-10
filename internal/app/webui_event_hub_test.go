package app

import (
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func TestWebUIEventIDRoundTrip(t *testing.T) {
	event := appcore.ProbeEvent{TaskID: "task/with spaces", Seq: 17}
	raw := encodeWebUIEventID(event)
	taskID, seq, ok := decodeWebUIEventID(raw)
	if !ok || taskID != event.TaskID || seq != event.Seq {
		t.Fatalf("decodeWebUIEventID(%q) = (%q, %d, %v), want (%q, %d, true)", raw, taskID, seq, ok, event.TaskID, event.Seq)
	}
}

func TestWebUIEventHubReplaysOnlyMissingTaskEvents(t *testing.T) {
	hub := newWebUIEventHub()
	hub.publish(appcore.ProbeEvent{TaskID: "task-a", Seq: 1})
	hub.publish(appcore.ProbeEvent{TaskID: "task-b", Seq: 1})
	hub.publish(appcore.ProbeEvent{TaskID: "task-a", Seq: 2})
	hub.publish(appcore.ProbeEvent{TaskID: "task-a", Seq: 3})

	_, replay, unsubscribe := hub.subscribeAfter(encodeWebUIEventID(appcore.ProbeEvent{TaskID: "task-a", Seq: 1}))
	defer unsubscribe()
	if len(replay) != 2 || replay[0].Seq != 2 || replay[1].Seq != 3 {
		t.Fatalf("replay = %#v, want task-a seq 2 and 3", replay)
	}
	for _, event := range replay {
		if event.TaskID != "task-a" {
			t.Fatalf("replayed unrelated task event: %#v", event)
		}
	}
}
