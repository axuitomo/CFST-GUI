//go:build !webui

package main

import wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

func (a *App) emitProbeEvent(event desktopProbeEventEnvelope) {
	if a.eventHub != nil {
		a.eventHub.publish(event)
	}
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, desktopProbeEventName, event)
}
