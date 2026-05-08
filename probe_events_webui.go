//go:build webui

package main

func (a *App) emitProbeEvent(event desktopProbeEventEnvelope) {
	if a.eventHub != nil {
		a.eventHub.publish(event)
	}
}
