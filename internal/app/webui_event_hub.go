package app

import "sync"

type webUIEventHub struct {
	mu          sync.Mutex
	subscribers map[chan desktopProbeEventEnvelope]struct{}
}

func newWebUIEventHub() *webUIEventHub {
	return &webUIEventHub{
		subscribers: make(map[chan desktopProbeEventEnvelope]struct{}),
	}
}

func (h *webUIEventHub) publish(event desktopProbeEventEnvelope) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *webUIEventHub) subscribe() (<-chan desktopProbeEventEnvelope, func()) {
	ch := make(chan desktopProbeEventEnvelope, 64)
	if h == nil {
		return ch, func() { close(ch) }
	}
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		if _, ok := h.subscribers[ch]; ok {
			delete(h.subscribers, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}
