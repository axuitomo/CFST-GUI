package app

import (
	"encoding/base64"
	"strconv"
	"strings"
	"sync"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

const webUIEventHistoryLimit = 512

type webUIEventHub struct {
	mu          sync.Mutex
	subscribers map[chan appcore.ProbeEvent]struct{}
	history     []appcore.ProbeEvent
}

func newWebUIEventHub() *webUIEventHub {
	return &webUIEventHub{
		subscribers: make(map[chan appcore.ProbeEvent]struct{}),
	}
}

func (h *webUIEventHub) publish(event appcore.ProbeEvent) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.history = append(h.history, event)
	if len(h.history) > webUIEventHistoryLimit {
		h.history = append([]appcore.ProbeEvent(nil), h.history[len(h.history)-webUIEventHistoryLimit:]...)
	}
	for ch := range h.subscribers {
		enqueueWebUIEvent(ch, event)
	}
}

func enqueueWebUIEvent(ch chan appcore.ProbeEvent, event appcore.ProbeEvent) {
	select {
	case ch <- event:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- event:
	default:
	}
}

func (h *webUIEventHub) subscribeAfter(lastEventID string) (<-chan appcore.ProbeEvent, []appcore.ProbeEvent, func()) {
	ch := make(chan appcore.ProbeEvent, 256)
	if h == nil {
		return ch, nil, func() { close(ch) }
	}
	h.mu.Lock()
	replay := h.eventsAfterLocked(lastEventID)
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	return ch, replay, func() {
		h.mu.Lock()
		if _, ok := h.subscribers[ch]; ok {
			delete(h.subscribers, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

func (h *webUIEventHub) eventsAfterLocked(lastEventID string) []appcore.ProbeEvent {
	taskID, seq, ok := decodeWebUIEventID(lastEventID)
	if !ok {
		return nil
	}
	replay := make([]appcore.ProbeEvent, 0)
	for _, event := range h.history {
		if event.TaskID == taskID && event.Seq > seq {
			replay = append(replay, event)
		}
	}
	return replay
}

func encodeWebUIEventID(event appcore.ProbeEvent) string {
	taskID := base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(event.TaskID)))
	return taskID + "." + strconv.Itoa(event.Seq)
}

func decodeWebUIEventID(raw string) (string, int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) != 2 || parts[0] == "" {
		return "", 0, false
	}
	taskID, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", 0, false
	}
	seq, err := strconv.Atoi(parts[1])
	if err != nil || seq < 0 {
		return "", 0, false
	}
	return string(taskID), seq, true
}
