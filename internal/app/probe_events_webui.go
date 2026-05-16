//go:build webui

package app

func (a *App) emitProbeEvent(event desktopProbeEventEnvelope) {
	if a.eventHub != nil {
		a.eventHub.publish(event)
	}
}
