package mobileapi

import (
	"sync"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

const legacyMobileSchemaVersion = "cfst-gui-mobile-v1"

type EventSink interface {
	OnProbeEvent(eventJSON string)
}

type Service struct {
	core  *appcore.Service
	runMu sync.Mutex

	stateMu   sync.Mutex
	baseDir   string
	eventSink EventSink
}
